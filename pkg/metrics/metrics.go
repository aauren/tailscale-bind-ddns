package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aauren/tailscale-bind-ddns/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"k8s.io/klog/v2"
)

// MachineInfo represents the data required for machine metrics.
type MachineInfo struct {
	ID         string
	Name       string
	IPv4       string
	IPv6       string
	Online     bool
	Authorized bool
}

// RecordInfo represents the data required for DNS record metrics.
type RecordInfo struct {
	Name  string
	Type  string
	Value string
}

// Manager coordinates Prometheus metrics.
type Manager struct {
	cfg        config.MetricsConfig
	enabled    bool
	perMachine bool

	registry *prometheus.Registry

	endpointGauge           prometheus.Gauge
	endpointOnlineGauge     prometheus.Gauge
	endpointAuthorizedGauge prometheus.Gauge
	endpointIPv4Gauge       prometheus.Gauge
	endpointIPv6Gauge       prometheus.Gauge

	endpointAddCounter    prometheus.Counter
	endpointRemoveCounter prometheus.Counter
	endpointDeleteCounter prometheus.Counter

	pollDurationHistogram prometheus.Histogram
	pollSuccessCounter    prometheus.Counter
	pollFailureCounter    prometheus.Counter

	updateDurationHistogram prometheus.Histogram
	updateSuccessCounter    prometheus.Counter
	updateFailureCounter    prometheus.Counter

	recordCreatedCounter prometheus.Counter
	recordUpdatedCounter prometheus.Counter
	recordRemovedCounter prometheus.Counter

	recordQueueDepthGauge prometheus.Gauge
	lastUpdateGauge       prometheus.Gauge

	perEndpointAuthorizedGauge *prometheus.GaugeVec
	perEndpointConnectedGauge  *prometheus.GaugeVec

	inventoryDurationHistogram prometheus.Histogram
	inventoryRunCounter        prometheus.Counter
	inventoryFailureCounter    prometheus.Counter

	mu              sync.Mutex
	server          *http.Server
	machineSnapshot map[string]machineState
	recordSnapshot  map[string]string
}

type machineState struct {
	online bool
	labels prometheus.Labels
}

// NewManager creates a metrics manager based on configuration.
func NewManager(cfg config.MetricsConfig) (*Manager, error) {
	manager := &Manager{
		cfg:        cfg,
		enabled:    cfg.Enabled,
		perMachine: cfg.Enabled && cfg.PerEndpointMetric,
	}

	if !cfg.Enabled {
		return manager, nil
	}

	registry := prometheus.NewRegistry()
	manager.registry = registry

	// Register default collectors for Go, process, and build info metrics.
	if err := registry.Register(collectors.NewGoCollector()); err != nil {
		return nil, fmt.Errorf("registering Go collector: %w", err)
	}
	if err := registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return nil, fmt.Errorf("registering process collector: %w", err)
	}
	if err := registry.Register(collectors.NewBuildInfoCollector()); err != nil {
		return nil, fmt.Errorf("registering build info collector: %w", err)
	}

	manager.machineSnapshot = make(map[string]machineState)
	manager.recordSnapshot = make(map[string]string)

	manager.initializeCollectors()
	return manager, nil
}

// Enabled returns true when metrics are enabled.
func (m *Manager) Enabled() bool {
	return m.enabled
}

func (m *Manager) initializeCollectors() {
	if !m.enabled {
		return
	}

	m.endpointGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tailscale_bind_endpoints_total",
		Help: "Total number of Tailscale endpoints discovered.",
	})
	m.endpointOnlineGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tailscale_bind_endpoints_online_total",
		Help: "Number of Tailscale endpoints currently online.",
	})
	m.endpointAuthorizedGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tailscale_bind_endpoints_authorized_total",
		Help: "Number of Tailscale endpoints authorized in the tailnet.",
	})
	m.endpointIPv4Gauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tailscale_bind_endpoints_ipv4_total",
		Help: "Number of Tailscale endpoints possessing an IPv4 address.",
	})
	m.endpointIPv6Gauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tailscale_bind_endpoints_ipv6_total",
		Help: "Number of Tailscale endpoints possessing an IPv6 address.",
	})

	m.endpointAddCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_endpoint_adds_total",
		Help: "Total number of endpoint additions observed.",
	})
	m.endpointRemoveCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_endpoint_removals_total",
		Help: "Total number of endpoint removals (online to offline transitions).",
	})
	m.endpointDeleteCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_endpoint_deletes_total",
		Help: "Total number of endpoint deletions from the tailnet.",
	})

	m.pollDurationHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "tailscale_bind_tailscale_poll_duration_seconds",
		Help:    "Histogram of durations for Tailscale polling calls.",
		Buckets: prometheus.DefBuckets,
	})
	m.pollSuccessCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_tailscale_poll_success_total",
		Help: "Total number of successful Tailscale polling attempts.",
	})
	m.pollFailureCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_tailscale_poll_failure_total",
		Help: "Total number of failed Tailscale polling attempts.",
	})

	m.updateDurationHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "tailscale_bind_bind_update_duration_seconds",
		Help:    "Histogram of durations for Bind DNS update operations.",
		Buckets: prometheus.DefBuckets,
	})
	m.updateSuccessCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_bind_update_success_total",
		Help: "Total number of successful Bind DNS update operations.",
	})
	m.updateFailureCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_bind_update_failure_total",
		Help: "Total number of failed Bind DNS update operations.",
	})

	m.recordCreatedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_records_created_total",
		Help: "Total number of DNS records newly created.",
	})
	m.recordUpdatedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_records_updated_total",
		Help: "Total number of DNS records updated with new values.",
	})
	m.recordRemovedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_records_removed_total",
		Help: "Total number of DNS records removed.",
	})

	m.recordQueueDepthGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tailscale_bind_record_queue_depth",
		Help: "Current depth of the DNS record update queue.",
	})
	m.lastUpdateGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tailscale_bind_last_successful_update_timestamp_seconds",
		Help: "Unix timestamp of the last successful Bind update.",
	})
	m.inventoryDurationHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "tailscale_bind_inventory_refresh_duration_seconds",
		Help:    "Histogram of durations for inventory refresh operations.",
		Buckets: prometheus.DefBuckets,
	})
	m.inventoryRunCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_inventory_refresh_total",
		Help: "Total number of inventory refresh attempts.",
	})
	m.inventoryFailureCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tailscale_bind_inventory_refresh_failures_total",
		Help: "Total number of failed inventory refresh attempts.",
	})

	if m.perMachine {
		m.perEndpointAuthorizedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tailscale_bind_endpoint_authorized",
			Help: "Endpoint authorization indicator (1 when the node is authorized, 0 otherwise).",
		}, []string{"fqdn", "ip"})
		m.perEndpointConnectedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tailscale_bind_endpoint_connected",
			Help: "Endpoint connectivity indicator (1 when the node is actively connected to Tailscale, 0 when disconnected).",
		}, []string{"fqdn", "ip"})
	}

	collectorList := []prometheus.Collector{
		m.endpointGauge,
		m.endpointOnlineGauge,
		m.endpointAuthorizedGauge,
		m.endpointIPv4Gauge,
		m.endpointIPv6Gauge,
		m.endpointAddCounter,
		m.endpointRemoveCounter,
		m.endpointDeleteCounter,
		m.pollDurationHistogram,
		m.pollSuccessCounter,
		m.pollFailureCounter,
		m.updateDurationHistogram,
		m.updateSuccessCounter,
		m.updateFailureCounter,
		m.recordCreatedCounter,
		m.recordUpdatedCounter,
		m.recordRemovedCounter,
		m.recordQueueDepthGauge,
		m.lastUpdateGauge,
		m.inventoryDurationHistogram,
		m.inventoryRunCounter,
		m.inventoryFailureCounter,
	}

	for _, collector := range collectorList {
		if err := m.registry.Register(collector); err != nil {
			klog.Errorf("Failed to register collector %T: %v", collector, err)
		}
	}

	if m.perEndpointAuthorizedGauge != nil {
		if err := m.registry.Register(m.perEndpointAuthorizedGauge); err != nil {
			klog.Errorf("Failed to register per-endpoint authorized gauge: %v", err)
		}
	}
	if m.perEndpointConnectedGauge != nil {
		if err := m.registry.Register(m.perEndpointConnectedGauge); err != nil {
			klog.Errorf("Failed to register per-endpoint connected gauge: %v", err)
		}
	}
}

// Handler returns an http.Handler that exposes the metrics.
func (m *Manager) Handler() http.Handler {
	if !m.enabled {
		return nil
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// GatherMetrics exposes the underlying Prometheus metrics, primarily for testing.
func (m *Manager) GatherMetrics() ([]*io_prometheus_client.MetricFamily, error) {
	if !m.enabled {
		return nil, nil
	}
	return m.registry.Gather()
}

// RecordQueueGauge exposes the queue depth gauge; useful for validation in tests.
func (m *Manager) RecordQueueGauge() prometheus.Gauge {
	if !m.enabled {
		return nil
	}
	return m.recordQueueDepthGauge
}

// RunServer starts a metrics HTTP server and blocks until it exits or the context is cancelled.
func (m *Manager) RunServer(ctx context.Context) {
	if !m.enabled {
		return
	}

	handler := m.Handler()
	if handler == nil {
		return
	}

	server := &http.Server{
		Addr:              m.cfg.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	m.server = server

	go func() { //nolint:gosec // G118: parent ctx is done; new context is intentional.
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
			klog.Errorf("Metrics server shutdown error: %v", err)
		}
	}()

	klog.Infof("Prometheus metrics server listening on %s", m.cfg.ListenAddress)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		klog.Errorf("Metrics server ListenAndServe error: %v", err)
	}
}

// ObservePoll records the result of a Tailscale poll, including machine snapshots.
func (m *Manager) ObservePoll(machines []MachineInfo, duration time.Duration, err error) {
	if !m.enabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		m.pollFailureCounter.Inc()
		return
	}

	m.pollSuccessCounter.Inc()
	m.pollDurationHistogram.Observe(duration.Seconds())

	m.endpointGauge.Set(float64(len(machines)))

	var onlineCount, authorizedCount, ipv4Count, ipv6Count float64
	for _, machine := range machines {
		if machine.Online {
			onlineCount++
		}
		if machine.Authorized {
			authorizedCount++
		}
		if machine.IPv4 != "" {
			ipv4Count++
		}
		if machine.IPv6 != "" {
			ipv6Count++
		}
	}

	m.endpointOnlineGauge.Set(onlineCount)
	m.endpointAuthorizedGauge.Set(authorizedCount)
	m.endpointIPv4Gauge.Set(ipv4Count)
	m.endpointIPv6Gauge.Set(ipv6Count)

	m.updateMachineSnapshotsLocked(machines)
}

// ObserveRecordQueue updates the gauge tracking pending record updates.
func (m *Manager) ObserveRecordQueue(depth int) {
	if !m.enabled {
		return
	}
	m.recordQueueDepthGauge.Set(float64(depth))
}

// ObserveBindUpdate records the outcome of a Bind update attempt.
func (m *Manager) ObserveBindUpdate(records []RecordInfo, duration time.Duration, err error) {
	if !m.enabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		m.updateFailureCounter.Inc()
		return
	}

	m.updateSuccessCounter.Inc()
	m.updateDurationHistogram.Observe(duration.Seconds())
	m.lastUpdateGauge.Set(float64(time.Now().Unix()))

	m.updateRecordSnapshotsLocked(records)
}

// ObserveInventoryRefresh records stats for a DNS inventory refresh.
func (m *Manager) ObserveInventoryRefresh(duration time.Duration, err error) {
	if !m.enabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.inventoryRunCounter.Inc()
	m.inventoryDurationHistogram.Observe(duration.Seconds())
	if err != nil {
		m.inventoryFailureCounter.Inc()
	}
}

func (m *Manager) updateMachineSnapshotsLocked(machines []MachineInfo) {
	current := make(map[string]machineState, len(machines))

	for _, machine := range machines {
		labels := prometheus.Labels{}
		if m.perMachine {
			labels = buildEndpointLabels(machine)
		}

		prev, exists := m.machineSnapshot[machine.ID]

		if !exists && machine.Online {
			m.endpointAddCounter.Inc()
		} else if exists {
			if !prev.online && machine.Online {
				m.endpointAddCounter.Inc()
			}
			if prev.online && !machine.Online {
				m.endpointRemoveCounter.Inc()
			}

			if m.perMachine && !labelsMatch(prev.labels, labels) {
				m.deletePerEndpointLocked(prev.labels)
			}
		}

		if m.perMachine {
			if m.perEndpointAuthorizedGauge != nil {
				m.perEndpointAuthorizedGauge.WithLabelValues(labels["fqdn"], labels["ip"]).Set(boolToFloat(machine.Authorized))
			}
			if m.perEndpointConnectedGauge != nil {
				m.perEndpointConnectedGauge.WithLabelValues(labels["fqdn"], labels["ip"]).Set(boolToFloat(machine.Online))
			}
		}

		current[machine.ID] = machineState{
			online: machine.Online,
			labels: labels,
		}
	}

	for id, prev := range m.machineSnapshot {
		if _, ok := current[id]; !ok {
			m.endpointDeleteCounter.Inc()
			m.deletePerEndpointLocked(prev.labels)
		}
	}

	m.machineSnapshot = current
}

func (m *Manager) updateRecordSnapshotsLocked(records []RecordInfo) {
	current := make(map[string]string, len(records))

	for _, record := range records {
		key := recordKey(record)
		current[key] = record.Value

		if previousValue, exists := m.recordSnapshot[key]; !exists {
			m.recordCreatedCounter.Inc()
		} else if previousValue != record.Value {
			m.recordUpdatedCounter.Inc()
		}
	}

	for key := range m.recordSnapshot {
		if _, ok := current[key]; !ok {
			m.recordRemovedCounter.Inc()
		}
	}

	m.recordSnapshot = current
}

func (m *Manager) deletePerEndpointLocked(labels prometheus.Labels) {
	if !m.perMachine || labels == nil {
		return
	}
	if fqdn, ok := labels["fqdn"]; ok {
		if ip, ok := labels["ip"]; ok {
			if m.perEndpointAuthorizedGauge != nil {
				m.perEndpointAuthorizedGauge.DeleteLabelValues(fqdn, ip)
			}
			if m.perEndpointConnectedGauge != nil {
				m.perEndpointConnectedGauge.DeleteLabelValues(fqdn, ip)
			}
		}
	}
}

func labelsMatch(lhs, rhs prometheus.Labels) bool {
	if len(lhs) != len(rhs) {
		return false
	}
	for key, left := range lhs {
		if right, ok := rhs[key]; !ok || right != left {
			return false
		}
	}
	return true
}

func buildEndpointLabels(machine MachineInfo) prometheus.Labels {
	fqdn := machine.Name
	if fqdn == "" {
		fqdn = machine.ID
	}
	labels := prometheus.Labels{
		"fqdn": fqdn,
		"ip":   endpointIP(machine),
	}
	return labels
}

func endpointIP(machine MachineInfo) string {
	if machine.IPv4 != "" {
		return machine.IPv4
	}
	if machine.IPv6 != "" {
		return machine.IPv6
	}
	return "unknown"
}

func recordKey(record RecordInfo) string {
	return fmt.Sprintf("%s|%s", record.Type, record.Name)
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
