package app

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aauren/tailscale-bind-ddns/pkg/bind"
	"github.com/aauren/tailscale-bind-ddns/pkg/config"
	"github.com/aauren/tailscale-bind-ddns/pkg/metrics"
	"github.com/aauren/tailscale-bind-ddns/pkg/tailscale"
	"k8s.io/klog/v2"
)

// App represents the main application
type App struct {
	config           *config.Config
	tailscaleClient  *tailscale.Client
	bindClient       *bind.Client
	machineChan      chan []tailscale.Machine
	recordChan       chan []bind.DNSRecord
	metricsManager   *metrics.Manager
	wg               sync.WaitGroup
	stateMu          sync.Mutex
	currentRecords   map[string]bind.DNSRecord
	desiredRecords   map[string]bind.DNSRecord
	protectedRecords map[string]struct{}
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config, metricsManager *metrics.Manager) (*App, error) {
	// Create Tailscale client
	var tsClient *tailscale.Client
	var err error

	if cfg.Tailscale.APIKey != "" {
		tsClient, err = tailscale.NewClient(cfg.Tailscale.APIKey, cfg.Tailscale.Tailnet)
	} else {
		tsClient, err = tailscale.NewOAuthClient(cfg.Tailscale.ClientID, cfg.Tailscale.ClientSecret, cfg.Tailscale.Tailnet)
	}
	if err != nil {
		return nil, fmt.Errorf("creating tailscale client: %w", err)
	}

	// Create Bind client
	bindClient, err := bind.NewClient(
		cfg.Bind.Server,
		cfg.Bind.Port,
		cfg.Bind.Zone,
		cfg.Bind.KeyName,
		cfg.Bind.KeySecret,
		cfg.Bind.Algorithm,
		cfg.Bind.TTL,
		&cfg.Bind.PTR,
	)
	if err != nil {
		return nil, fmt.Errorf("creating bind client: %w", err)
	}

	return &App{
		config:          cfg,
		tailscaleClient: tsClient,
		bindClient:      bindClient,
		machineChan:     make(chan []tailscale.Machine, 10),
		recordChan:      make(chan []bind.DNSRecord, 10),
		metricsManager:  metricsManager,
	}, nil
}

// Run starts the application
func (a *App) Run(ctx context.Context) error {
	klog.Info("Starting Tailscale-Bind DDNS application")

	// Validate Bind connection
	if err := a.bindClient.ValidateConnection(ctx); err != nil {
		return fmt.Errorf("bind connection validation failed: %w", err)
	}

	if err := a.loadExistingDNSRecords(ctx); err != nil {
		return fmt.Errorf("loading existing DNS records: %w", err)
	}

	// Start the machine-to-record converter
	a.wg.Go(func() {
		a.convertMachinesToRecords(ctx)
	})

	// Start Tailscale polling
	a.wg.Go(func() {
		a.pollTailscale(ctx)
	})

	// Start Bind DDNS updating
	a.wg.Go(func() {
		a.startUpdating(ctx)
	})

	// Periodic inventory refresh
	if a.config.Bind.InventoryRefreshInterval > 0 {
		a.wg.Go(func() {
			a.inventoryRefreshLoop(ctx)
		})
	}

	// Wait for context cancellation
	<-ctx.Done()
	klog.Info("Shutting down application...")

	// Close channels to signal goroutines to stop
	close(a.machineChan)
	close(a.recordChan)

	// Wait for all goroutines to finish
	a.wg.Wait()

	klog.Info("Application stopped")
	return nil
}

// convertMachinesToRecords converts Tailscale machines to DNS records
func (a *App) convertMachinesToRecords(ctx context.Context) {
	klog.Info("Starting machine-to-record converter")

	for {
		select {
		case machines, ok := <-a.machineChan:
			if !ok {
				klog.Info("Machine channel closed, stopping converter")
				return
			}

			records := a.machinesToRecords(machines)
			ptrRecords := a.createPTRRecords(machines)

			//nolint:gocritic // we understand that we're appending to a new array here
			allRecords := append(records, ptrRecords...)
			protected := a.computeProtectedRecordKeys(machines)
			a.setProtectedKeys(protected)
			updates := a.prepareRecordUpdates(allRecords, protected)
			if len(updates) == 0 {
				continue
			}

			select {
			case a.recordChan <- updates:
				if a.metricsManager != nil {
					a.metricsManager.ObserveRecordQueue(len(a.recordChan))
				}
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			klog.Info("Machine-to-record converter stopped")
			return
		}
	}
}

func (a *App) pollTailscale(ctx context.Context) {
	klog.Info("Starting Tailscale polling loop")
	a.fetchAndPublishMachines(ctx)

	ticker := time.NewTicker(a.config.Tailscale.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Tailscale polling stopped")
			return
		case <-ticker.C:
			a.fetchAndPublishMachines(ctx)
		}
	}
}

func (a *App) fetchAndPublishMachines(ctx context.Context) {
	start := time.Now()
	machines, err := a.tailscaleClient.GetMachines(ctx)
	duration := time.Since(start)

	if a.metricsManager != nil {
		a.metricsManager.ObservePoll(toMachineInfos(machines), duration, err)
	}

	if err != nil {
		klog.Errorf("Failed to get machines: %v", err)
		return
	}

	select {
	case a.machineChan <- machines:
	case <-ctx.Done():
	}
}

func (a *App) startUpdating(ctx context.Context) {
	klog.Info("Starting Bind update loop")

	ticker := time.NewTicker(a.config.Bind.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("DDNS updating stopped")
			return
		case records, ok := <-a.recordChan:
			if !ok {
				klog.Info("Record channel closed, stopping updates")
				return
			}
			a.processBindUpdate(ctx, records)
		case <-ticker.C:
			if cont := a.flushPendingUpdates(ctx); !cont {
				return
			}
		}
	}
}

func (a *App) inventoryRefreshLoop(ctx context.Context) {
	interval := a.config.Bind.InventoryRefreshInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	klog.Infof("Starting inventory refresh loop with interval %v", interval)

	for {
		select {
		case <-ctx.Done():
			klog.Info("Inventory refresh loop stopped")
			return
		case <-ticker.C:
			a.refreshInventory(ctx)
		}
	}
}

func (a *App) refreshInventory(ctx context.Context) {
	klog.V(1).Info("Refreshing DNS inventory from Bind")

	start := time.Now()
	var refreshErr error
	defer func() {
		if a.metricsManager != nil {
			a.metricsManager.ObserveInventoryRefresh(time.Since(start), refreshErr)
		}
	}()

	records, err := a.bindClient.FetchInventory(ctx)
	if err != nil {
		refreshErr = err
		klog.Errorf("Inventory refresh failed: %v", err)
		return
	}

	a.stateMu.Lock()
	a.currentRecords = recordsToMap(records)
	protected := cloneProtectedKeys(a.protectedRecords)
	desired := recordsFromMap(a.desiredRecords)
	a.stateMu.Unlock()

	if len(desired) == 0 {
		return
	}

	updates := a.prepareRecordUpdates(desired, protected)
	if len(updates) == 0 {
		return
	}

	select {
	case a.recordChan <- updates:
		if a.metricsManager != nil {
			a.metricsManager.ObserveRecordQueue(len(a.recordChan))
		}
	case <-ctx.Done():
	}
}

func (a *App) flushPendingUpdates(ctx context.Context) bool {
flushLoop:
	for {
		select {
		case records, ok := <-a.recordChan:
			if !ok {
				klog.Info("Record channel closed during flush, stopping updates")
				return false
			}
			a.processBindUpdate(ctx, records)
		default:
			if a.metricsManager != nil {
				a.metricsManager.ObserveRecordQueue(len(a.recordChan))
			}
			break flushLoop
		}
	}
	return true
}

func (a *App) processBindUpdate(ctx context.Context, records []bind.DNSRecord) {
	if len(records) == 0 {
		if a.metricsManager != nil {
			a.metricsManager.ObserveRecordQueue(len(a.recordChan))
		}
		return
	}

	start := time.Now()
	err := a.bindClient.UpdateRecords(ctx, records, a.config.General.DryRun)
	duration := time.Since(start)

	if err != nil {
		klog.Errorf("Failed to update DNS records: %v", err)
	} else {
		a.markStateApplied()
	}

	if a.metricsManager != nil {
		a.metricsManager.ObserveBindUpdate(toRecordInfos(records), duration, err)
		a.metricsManager.ObserveRecordQueue(len(a.recordChan))
	}
}

func toMachineInfos(machines []tailscale.Machine) []metrics.MachineInfo {
	if len(machines) == 0 {
		return nil
	}

	infos := make([]metrics.MachineInfo, 0, len(machines))
	for _, machine := range machines {
		infos = append(infos, metrics.MachineInfo{
			ID:         machine.ID,
			Name:       machine.Name,
			IPv4:       machine.IPv4Address,
			IPv6:       machine.IPv6Address,
			Online:     machine.Online,
			Authorized: machine.Authorized,
		})
	}
	return infos
}

func toRecordInfos(records []bind.DNSRecord) []metrics.RecordInfo {
	if len(records) == 0 {
		return nil
	}

	infos := make([]metrics.RecordInfo, 0, len(records))
	for _, record := range records {
		infos = append(infos, metrics.RecordInfo{
			Name:  record.Name,
			Type:  record.Type,
			Value: record.Value,
		})
	}
	return infos
}

// machinesToRecords converts a list of machines to DNS records
func (a *App) machinesToRecords(machines []tailscale.Machine) []bind.DNSRecord {
	var records []bind.DNSRecord

	for _, machine := range machines {
		// Only create records for online machines
		if !machine.Online {
			continue
		}

		// Use machine name as DNS record name
		recordName := machine.Name
		if recordName == "" {
			recordName = machine.ID
		}

		// Sanitize record name for DNS (replace invalid characters)
		recordName = sanitizeDNSName(recordName)

		// Create A record for IPv4 address
		if machine.IPv4Address != "" {
			aRecord := bind.DNSRecord{
				Name:  recordName,
				Value: machine.IPv4Address,
				TTL:   uint32(a.config.Bind.TTL.Seconds()),
				Type:  "A",
			}
			records = append(records, aRecord)
			klog.V(2).Infof("Converted machine %s (%s) to A record %s -> %s",
				machine.Name, machine.ID, recordName, machine.IPv4Address)
		}

		// Create AAAA record for IPv6 address if available
		if machine.IPv6Address != "" {
			aaaaRecord := bind.DNSRecord{
				Name:  recordName,
				Value: machine.IPv6Address,
				TTL:   uint32(a.config.Bind.TTL.Seconds()),
				Type:  "AAAA",
			}
			records = append(records, aaaaRecord)
			klog.V(2).Infof("Converted machine %s (%s) to AAAA record %s -> %s",
				machine.Name, machine.ID, recordName, machine.IPv6Address)
		}
	}

	klog.V(1).Infof("Converted %d machines to %d DNS records", len(machines), len(records))
	return records
}

// createPTRRecords creates PTR records for the given machines
func (a *App) createPTRRecords(machines []tailscale.Machine) []bind.DNSRecord {
	var ptrRecords []bind.DNSRecord

	if !a.config.Bind.PTR.Enabled {
		return ptrRecords
	}

	for _, machine := range machines {
		// Only create PTR records for online machines
		if !machine.Online {
			continue
		}

		// Use machine name as DNS record name
		recordName := machine.Name
		if recordName == "" {
			recordName = machine.ID
		}

		// Sanitize record name for DNS (replace invalid characters)
		recordName = sanitizeDNSName(recordName)

		// Create PTR record for IPv4 address
		if machine.IPv4Address != "" {
			ptrRecord, err := a.bindClient.CreatePTRRecord(machine.IPv4Address, recordName+"."+a.config.Bind.Zone)
			if err != nil {
				klog.Warningf("Failed to create PTR record for IPv4 %s: %v", machine.IPv4Address, err)
				continue
			}
			if ptrRecord != nil {
				ptrRecords = append(ptrRecords, *ptrRecord)
			}
		}

		// Create PTR record for IPv6 address if available
		if machine.IPv6Address != "" {
			ptrRecord, err := a.bindClient.CreatePTRRecord(machine.IPv6Address, recordName+"."+a.config.Bind.Zone)
			if err != nil {
				klog.Warningf("Failed to create PTR record for IPv6 %s: %v", machine.IPv6Address, err)
				continue
			}
			if ptrRecord != nil {
				ptrRecords = append(ptrRecords, *ptrRecord)
			}
		}
	}

	klog.V(1).Infof("Created %d PTR records", len(ptrRecords))
	return ptrRecords
}

// GetStatus returns the current status of the application
func (a *App) GetStatus() map[string]any {
	return map[string]any{
		"tailscale_tailnet": a.config.Tailscale.Tailnet,
		"bind_server":       a.config.Bind.Server,
		"bind_zone":         a.config.Bind.Zone,
		"dry_run":           a.config.General.DryRun,
		"log_level":         a.config.General.LogLevel,
	}
}

func (a *App) loadExistingDNSRecords(ctx context.Context) (err error) {
	start := time.Now()
	defer func() {
		if a.metricsManager != nil {
			a.metricsManager.ObserveInventoryRefresh(time.Since(start), err)
		}
	}()

	records, err := a.bindClient.FetchInventory(ctx)
	if err != nil {
		return err
	}

	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	a.currentRecords = recordsToMap(records)
	a.desiredRecords = cloneRecordMap(a.currentRecords)

	klog.Infof("Initialized DNS state with %d records", len(a.currentRecords))
	return nil
}

func (a *App) prepareRecordUpdates(records []bind.DNSRecord, protected map[string]struct{}) []bind.DNSRecord {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	desired := recordsToMap(records)

	var updates []bind.DNSRecord

	for key, desiredRecord := range desired {
		if current, ok := a.currentRecords[key]; !ok ||
			current.Value != desiredRecord.Value ||
			current.TTL != desiredRecord.TTL {
			updates = append(updates, desiredRecord)
		}
	}

	for key, current := range a.currentRecords {
		if _, ok := desired[key]; !ok {
			if protected != nil {
				if _, keep := protected[key]; keep {
					continue
				}
			}
			toDelete := current
			toDelete.Delete = true
			updates = append(updates, toDelete)
		}
	}

	a.desiredRecords = desired
	if len(updates) == 0 && mapsEqual(desired, a.currentRecords) {
		return nil
	}

	return updates
}

func (a *App) markStateApplied() {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.currentRecords = cloneRecordMap(a.desiredRecords)
}

func (a *App) setProtectedKeys(keys map[string]struct{}) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.protectedRecords = keys
}

func (a *App) computeProtectedRecordKeys(machines []tailscale.Machine) map[string]struct{} {
	protected := make(map[string]struct{})

	for _, machine := range machines {
		recordName := machine.Name
		if recordName == "" {
			recordName = machine.ID
		}
		recordName = sanitizeDNSName(recordName)

		if machine.IPv4Address != "" {
			key := recordMapKey(bind.DNSRecord{Name: recordName, Type: "A"})
			protected[key] = struct{}{}
		}
		if machine.IPv6Address != "" {
			key := recordMapKey(bind.DNSRecord{Name: recordName, Type: "AAAA"})
			protected[key] = struct{}{}
		}

		if !a.config.Bind.PTR.Enabled {
			continue
		}

		fqdn := recordName + "." + a.config.Bind.Zone

		if machine.IPv4Address != "" {
			ptrRecord, err := a.bindClient.CreatePTRRecord(machine.IPv4Address, fqdn)
			if err == nil && ptrRecord != nil {
				protected[recordMapKey(*ptrRecord)] = struct{}{}
			}
		}

		if machine.IPv6Address != "" {
			ptrRecord, err := a.bindClient.CreatePTRRecord(machine.IPv6Address, fqdn)
			if err == nil && ptrRecord != nil {
				protected[recordMapKey(*ptrRecord)] = struct{}{}
			}
		}
	}

	return protected
}

func recordsToMap(records []bind.DNSRecord) map[string]bind.DNSRecord {
	if len(records) == 0 {
		return make(map[string]bind.DNSRecord)
	}

	result := make(map[string]bind.DNSRecord, len(records))
	for _, record := range records {
		record.Delete = false
		result[recordMapKey(record)] = record
	}
	return result
}

func cloneRecordMap(src map[string]bind.DNSRecord) map[string]bind.DNSRecord {
	if len(src) == 0 {
		return make(map[string]bind.DNSRecord)
	}

	clone := make(map[string]bind.DNSRecord, len(src))
	maps.Copy(clone, src)
	return clone
}

func mapsEqual(a, b map[string]bind.DNSRecord) bool {
	if len(a) != len(b) {
		return false
	}

	for key, recordA := range a {
		recordB, ok := b[key]
		if !ok {
			return false
		}
		if recordA.Value != recordB.Value || recordA.TTL != recordB.TTL {
			return false
		}
	}

	return true
}

func recordMapKey(record bind.DNSRecord) string {
	return fmt.Sprintf("%s|%s", record.Type, record.Name)
}

func cloneProtectedKeys(src map[string]struct{}) map[string]struct{} {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]struct{}, len(src))
	for key := range src {
		cloned[key] = struct{}{}
	}
	return cloned
}

func recordsFromMap(recordMap map[string]bind.DNSRecord) []bind.DNSRecord {
	if len(recordMap) == 0 {
		return nil
	}

	records := make([]bind.DNSRecord, 0, len(recordMap))
	for _, record := range recordMap {
		record.Delete = false
		records = append(records, record)
	}
	return records
}

// sanitizeDNSName sanitizes a string to be a valid DNS record name
func sanitizeDNSName(name string) string {
	// Extract only the hostname (leftmost part) from FQDN
	// Split by dots and take only the first part
	parts := strings.Split(name, ".")
	hostname := parts[0]

	// Replace invalid DNS characters with hyphens
	// DNS names can only contain letters, digits, hyphens, and dots
	reg := regexp.MustCompile(`[^a-zA-Z0-9.-]`)
	sanitized := reg.ReplaceAllString(hostname, "-")

	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	sanitized = reg.ReplaceAllString(sanitized, "-")

	// Remove leading/trailing hyphens and dots
	sanitized = strings.Trim(sanitized, "-.")

	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "machine"
	}

	// Convert to lowercase for consistency
	return strings.ToLower(sanitized)
}
