package app

import (
	"testing"
	"time"

	"github.com/aauren/tailscale-bind-ddns/pkg/config"
	"github.com/aauren/tailscale-bind-ddns/pkg/tailscale"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewApp(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid config with API key",
			config: &config.Config{
				Tailscale: config.TailscaleConfig{
					APIKey:  "test-api-key",
					Tailnet: "test.example.com",
				},
				Bind: config.BindConfig{
					Server:    "dns.example.com",
					Port:      53,
					Zone:      "test.example.com",
					KeyName:   "test-key",
					KeySecret: "test-secret",
					Algorithm: "hmac-sha256",
					TTL:       300 * time.Second,
				},
				General: config.GeneralConfig{
					LogLevel: "info",
					DryRun:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with OAuth",
			config: &config.Config{
				Tailscale: config.TailscaleConfig{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					Tailnet:      "test.example.com",
				},
				Bind: config.BindConfig{
					Server:    "dns.example.com",
					Port:      53,
					Zone:      "test.example.com",
					KeyName:   "test-key",
					KeySecret: "test-secret",
					Algorithm: "hmac-sha256",
					TTL:       300 * time.Second,
				},
				General: config.GeneralConfig{
					LogLevel: "info",
					DryRun:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid Tailscale config",
			config: &config.Config{
				Tailscale: config.TailscaleConfig{
					Tailnet: "test.example.com",
					// Missing API key and OAuth credentials
				},
				Bind: config.BindConfig{
					Server:    "dns.example.com",
					Port:      53,
					Zone:      "test.example.com",
					KeyName:   "test-key",
					KeySecret: "test-secret",
					Algorithm: "hmac-sha256",
					TTL:       300 * time.Second,
				},
				General: config.GeneralConfig{
					LogLevel: "info",
					DryRun:   false,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid Bind config",
			config: &config.Config{
				Tailscale: config.TailscaleConfig{
					APIKey:  "test-api-key",
					Tailnet: "test.example.com",
				},
				Bind: config.BindConfig{
					Server: "dns.example.com",
					Port:   53,
					Zone:   "test.example.com",
					// Missing key name and secret
				},
				General: config.GeneralConfig{
					LogLevel: "info",
					DryRun:   false,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := NewApp(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, app)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, app)
				assert.Equal(t, tt.config, app.config)
				assert.NotNil(t, app.tailscaleClient)
				assert.NotNil(t, app.bindClient)
				assert.NotNil(t, app.machineChan)
				assert.NotNil(t, app.recordChan)
			}
		})
	}
}

func TestMachinesToRecords(t *testing.T) {
	app := &App{
		config: &config.Config{
			Bind: config.BindConfig{
				TTL: 300 * time.Second,
			},
		},
	}

	tests := []struct {
		name     string
		machines []tailscale.Machine
		expected int
	}{
		{
			name: "online machines with IPv4",
			machines: []tailscale.Machine{
				{
					ID:          "machine1",
					Name:        "machine1",
					IPv4Address: "100.64.1.1",
					Online:      true,
				},
				{
					ID:          "machine2",
					Name:        "machine2",
					IPv4Address: "100.64.1.2",
					Online:      true,
				},
			},
			expected: 2,
		},
		{
			name: "mixed online/offline machines",
			machines: []tailscale.Machine{
				{
					ID:          "machine1",
					Name:        "machine1",
					IPv4Address: "100.64.1.1",
					Online:      true,
				},
				{
					ID:          "machine2",
					Name:        "machine2",
					IPv4Address: "100.64.1.2",
					Online:      false,
				},
				{
					ID:          "machine3",
					Name:        "machine3",
					IPv4Address: "100.64.1.3",
					Online:      true,
				},
			},
			expected: 2,
		},
		{
			name: "machines without IPv4 addresses",
			machines: []tailscale.Machine{
				{
					ID:     "machine1",
					Name:   "machine1",
					Online: true,
					// No IPv4 address
				},
				{
					ID:          "machine2",
					Name:        "machine2",
					IPv4Address: "100.64.1.2",
					Online:      true,
				},
			},
			expected: 1,
		},
		{
			name:     "empty machine list",
			machines: []tailscale.Machine{},
			expected: 0,
		},
		{
			name: "machines with empty names",
			machines: []tailscale.Machine{
				{
					ID:          "machine1",
					Name:        "", // Empty name
					IPv4Address: "100.64.1.1",
					Online:      true,
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := app.machinesToRecords(tt.machines)
			assert.Len(t, records, tt.expected)

			// Verify record structure
			for i, record := range records {
				assert.NotEmpty(t, record.Name)
				assert.NotEmpty(t, record.Value)
				assert.Equal(t, uint32(300), record.TTL)

				// Verify that the record name is either the machine name or ID
				found := false
				for _, machine := range tt.machines {
					if machine.Online && machine.IPv4Address != "" {
						expectedName := machine.Name
						if expectedName == "" {
							expectedName = machine.ID
						}
						if record.Name == expectedName {
							found = true
							assert.Equal(t, machine.IPv4Address, record.Value)
							break
						}
					}
				}
				assert.True(t, found, "Record %d should correspond to a valid machine", i)
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	config := &config.Config{
		Tailscale: config.TailscaleConfig{
			Tailnet: "test.example.com",
		},
		Bind: config.BindConfig{
			Server: "dns.example.com",
			Zone:   "test.example.com",
		},
		General: config.GeneralConfig{
			LogLevel: "info",
			DryRun:   true,
		},
	}

	app := &App{
		config: config,
	}

	status := app.GetStatus()

	assert.Equal(t, "test.example.com", status["tailscale_tailnet"])
	assert.Equal(t, "dns.example.com", status["bind_server"])
	assert.Equal(t, "test.example.com", status["bind_zone"])
	assert.Equal(t, true, status["dry_run"])
	assert.Equal(t, "info", status["log_level"])
}

func TestConvertMachinesToRecords(t *testing.T) {
	app := &App{
		config: &config.Config{
			Bind: config.BindConfig{
				TTL: 300 * time.Second,
			},
		},
	}

	// Test the conversion logic
	machines := []tailscale.Machine{
		{
			ID:          "machine1",
			Name:        "test-machine",
			IPv4Address: "100.64.1.1",
			Online:      true,
		},
	}

	records := app.machinesToRecords(machines)

	require.Len(t, records, 1)
	assert.Equal(t, "test-machine", records[0].Name)
	assert.Equal(t, "100.64.1.1", records[0].Value)
	assert.Equal(t, uint32(300), records[0].TTL)
}

func TestAppRun(t *testing.T) {
	// This test would require mocking the clients
	// For now, we'll test the basic structure

	config := &config.Config{
		Tailscale: config.TailscaleConfig{
			APIKey:       "test-api-key",
			Tailnet:      "test.example.com",
			PollInterval: 30 * time.Second,
		},
		Bind: config.BindConfig{
			Server:         "dns.example.com",
			Port:           53,
			Zone:           "test.example.com",
			KeyName:        "test-key",
			KeySecret:      "test-secret",
			Algorithm:      "hmac-sha256",
			TTL:            300 * time.Second,
			UpdateInterval: 60 * time.Second,
		},
		General: config.GeneralConfig{
			LogLevel: "info",
			DryRun:   true,
		},
	}

	app, err := NewApp(config)
	require.NoError(t, err)
	require.NotNil(t, app)

	// Test that the app can be created and has the expected structure
	assert.Equal(t, config, app.config)
	assert.NotNil(t, app.tailscaleClient)
	assert.NotNil(t, app.bindClient)
	assert.NotNil(t, app.machineChan)
	assert.NotNil(t, app.recordChan)
}
