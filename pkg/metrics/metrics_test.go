package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/aauren/tailscale-bind-ddns/pkg/config"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManagerDisabled(t *testing.T) {
	manager, err := NewManager(config.MetricsConfig{})
	require.NoError(t, err)
	assert.False(t, manager.Enabled())
	assert.Nil(t, manager.Handler())
}

func TestObservePoll(t *testing.T) {
	manager := mustManager(t, config.MetricsConfig{Enabled: true})

	machines := []MachineInfo{
		{
			ID:         "machine-1",
			Name:       "host1.example.com",
			IPv4:       "100.64.0.1",
			Online:     true,
			Authorized: true,
		},
		{
			ID:         "machine-2",
			Name:       "host2.example.com",
			IPv6:       "fd7a:115c:a1e0::1",
			Online:     false,
			Authorized: true,
		},
	}

	manager.ObservePoll(machines, 150*time.Millisecond, nil)

	assert.Equal(t, 1.0, testutil.ToFloat64(manager.pollSuccessCounter))
	assert.Equal(t, 0.0, testutil.ToFloat64(manager.pollFailureCounter))
	assert.Equal(t, 2.0, testutil.ToFloat64(manager.endpointGauge))
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.endpointOnlineGauge))
	assert.Equal(t, 2.0, testutil.ToFloat64(manager.endpointAuthorizedGauge))
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.endpointIPv4Gauge))
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.endpointIPv6Gauge))

	manager.ObservePoll(nil, 0, errors.New("boom"))
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.pollFailureCounter))
}

func TestObserveBindUpdate(t *testing.T) {
	manager := mustManager(t, config.MetricsConfig{Enabled: true})

	initialRecords := []RecordInfo{
		{Name: "host1", Type: "A", Value: "100.64.0.1"},
		{Name: "host1", Type: "AAAA", Value: "fd7a:115c:a1e0::1"},
	}

	manager.ObserveBindUpdate(initialRecords, 220*time.Millisecond, nil)
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.updateSuccessCounter))
	assert.Equal(t, 0.0, testutil.ToFloat64(manager.updateFailureCounter))
	assert.Equal(t, 2.0, testutil.ToFloat64(manager.recordCreatedCounter))

	updatedRecords := []RecordInfo{
		{Name: "host1", Type: "A", Value: "100.64.0.2"}, // updated
	}

	manager.ObserveBindUpdate(updatedRecords, 90*time.Millisecond, nil)
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.recordUpdatedCounter))
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.recordRemovedCounter))

	manager.ObserveBindUpdate(updatedRecords, 0, errors.New("update failed"))
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.updateFailureCounter))
}

func TestPerEndpointMetrics(t *testing.T) {
	manager := mustManager(t, config.MetricsConfig{
		Enabled:           true,
		PerEndpointMetric: true,
	})

	machines := []MachineInfo{
		{
			ID:         "machine-1",
			Name:       "host1.example.com",
			IPv4:       "100.64.0.1",
			Online:     true,
			Authorized: true,
		},
	}

	manager.ObservePoll(machines, 0, nil)
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.endpointAddCounter))

	authorized := testutil.ToFloat64(manager.perEndpointAuthorizedGauge.WithLabelValues("host1.example.com", "100.64.0.1"))
	connected := testutil.ToFloat64(manager.perEndpointConnectedGauge.WithLabelValues("host1.example.com", "100.64.0.1"))
	assert.Equal(t, 1.0, authorized)
	assert.Equal(t, 1.0, connected)

	// Transition to offline should set gauge to 0 and increment removal counter.
	machines[0].Online = false
	manager.ObservePoll(machines, 0, nil)
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.endpointRemoveCounter))
	authorized = testutil.ToFloat64(manager.perEndpointAuthorizedGauge.WithLabelValues("host1.example.com", "100.64.0.1"))
	connected = testutil.ToFloat64(manager.perEndpointConnectedGauge.WithLabelValues("host1.example.com", "100.64.0.1"))
	assert.Equal(t, 1.0, authorized)
	assert.Equal(t, 0.0, connected)

	// Removal from list increments delete counter.
	manager.ObservePoll(nil, 0, nil)
	assert.Equal(t, 1.0, testutil.ToFloat64(manager.endpointDeleteCounter))
	assert.Empty(t, manager.machineSnapshot)
}

func mustManager(t *testing.T, cfg config.MetricsConfig) *Manager {
	t.Helper()
	manager, err := NewManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, manager)
	return manager
}
