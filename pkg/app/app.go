package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/aauren/tailscale-bind-ddns/pkg/bind"
	"github.com/aauren/tailscale-bind-ddns/pkg/config"
	"github.com/aauren/tailscale-bind-ddns/pkg/tailscale"
	"k8s.io/klog/v2"
)

// App represents the main application
type App struct {
	config          *config.Config
	tailscaleClient *tailscale.Client
	bindClient      *bind.Client
	machineChan     chan []tailscale.Machine
	recordChan      chan []bind.DNSRecord
	wg              sync.WaitGroup
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config) (*App, error) {
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
	}, nil
}

// Run starts the application
func (a *App) Run(ctx context.Context) error {
	klog.Info("Starting Tailscale-Bind DDNS application")

	// Validate Bind connection
	if err := a.bindClient.ValidateConnection(ctx); err != nil {
		return fmt.Errorf("bind connection validation failed: %w", err)
	}

	// Start the machine-to-record converter
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.convertMachinesToRecords(ctx)
	}()

	// Start Tailscale polling
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.tailscaleClient.StartPolling(ctx, a.config.Tailscale.PollInterval, a.machineChan)
	}()

	// Start Bind DDNS updating
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.bindClient.StartUpdating(ctx, a.config.Bind.UpdateInterval, a.recordChan, a.config.General.DryRun)
	}()

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
			if len(records) > 0 {
				select {
				case a.recordChan <- records:
				case <-ctx.Done():
					return
				}
			}

		case <-ctx.Done():
			klog.Info("Machine-to-record converter stopped")
			return
		}
	}
}

// machinesToRecords converts a list of machines to DNS records
func (a *App) machinesToRecords(machines []tailscale.Machine) []bind.DNSRecord {
	var records []bind.DNSRecord

	for _, machine := range machines {
		// Only create records for online machines with IPv4 addresses
		if !machine.Online || machine.IPv4Address == "" {
			continue
		}

		// Use machine name as DNS record name
		recordName := machine.Name
		if recordName == "" {
			recordName = machine.ID
		}

		// Sanitize record name for DNS (replace invalid characters)
		recordName = sanitizeDNSName(recordName)

		record := bind.DNSRecord{
			Name:  recordName,
			Value: machine.IPv4Address,
			TTL:   uint32(a.config.Bind.TTL.Seconds()),
		}

		records = append(records, record)
		klog.V(2).Infof("Converted machine %s (%s) to DNS record %s -> %s",
			machine.Name, machine.ID, recordName, machine.IPv4Address)
	}

	klog.V(1).Infof("Converted %d machines to %d DNS records", len(machines), len(records))
	return records
}

// GetStatus returns the current status of the application
func (a *App) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"tailscale_tailnet": a.config.Tailscale.Tailnet,
		"bind_server":       a.config.Bind.Server,
		"bind_zone":         a.config.Bind.Zone,
		"dry_run":           a.config.General.DryRun,
		"log_level":         a.config.General.LogLevel,
	}
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
