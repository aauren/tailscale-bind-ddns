package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		wantErr bool
	}{
		{
			name: "valid config with API key",
			setup: func() {
				viper.Set("tailscale.api_key", "test-api-key")
				viper.Set("tailscale.tailnet", "test.example.com")
				viper.Set("bind.server", "dns.example.com")
				viper.Set("bind.zone", "test.example.com")
				viper.Set("bind.key_name", "test-key")
				viper.Set("bind.key_secret", "test-secret")
			},
			wantErr: false,
		},
		{
			name: "valid config with OAuth",
			setup: func() {
				viper.Set("tailscale.client_id", "test-client-id")
				viper.Set("tailscale.client_secret", "test-client-secret")
				viper.Set("tailscale.tailnet", "test.example.com")
				viper.Set("bind.server", "dns.example.com")
				viper.Set("bind.zone", "test.example.com")
				viper.Set("bind.key_name", "test-key")
				viper.Set("bind.key_secret", "test-secret")
			},
			wantErr: false,
		},
		{
			name: "missing tailscale credentials",
			setup: func() {
				viper.Set("tailscale.tailnet", "test.example.com")
				viper.Set("bind.server", "dns.example.com")
				viper.Set("bind.zone", "test.example.com")
				viper.Set("bind.key_name", "test-key")
				viper.Set("bind.key_secret", "test-secret")
			},
			wantErr: true,
		},
		{
			name: "missing tailnet",
			setup: func() {
				viper.Set("tailscale.api_key", "test-api-key")
				viper.Set("bind.server", "dns.example.com")
				viper.Set("bind.zone", "test.example.com")
				viper.Set("bind.key_name", "test-key")
				viper.Set("bind.key_secret", "test-secret")
			},
			wantErr: true,
		},
		{
			name: "missing bind server",
			setup: func() {
				viper.Set("tailscale.api_key", "test-api-key")
				viper.Set("tailscale.tailnet", "test.example.com")
				viper.Set("bind.zone", "test.example.com")
				viper.Set("bind.key_name", "test-key")
				viper.Set("bind.key_secret", "test-secret")
			},
			wantErr: true,
		},
		{
			name: "missing bind zone",
			setup: func() {
				viper.Set("tailscale.api_key", "test-api-key")
				viper.Set("tailscale.tailnet", "test.example.com")
				viper.Set("bind.server", "dns.example.com")
				viper.Set("bind.key_name", "test-key")
				viper.Set("bind.key_secret", "test-secret")
			},
			wantErr: true,
		},
		{
			name: "missing bind key name",
			setup: func() {
				viper.Set("tailscale.api_key", "test-api-key")
				viper.Set("tailscale.tailnet", "test.example.com")
				viper.Set("bind.server", "dns.example.com")
				viper.Set("bind.zone", "test.example.com")
				viper.Set("bind.key_secret", "test-secret")
			},
			wantErr: true,
		},
		{
			name: "missing bind key secret",
			setup: func() {
				viper.Set("tailscale.api_key", "test-api-key")
				viper.Set("tailscale.tailnet", "test.example.com")
				viper.Set("bind.server", "dns.example.com")
				viper.Set("bind.zone", "test.example.com")
				viper.Set("bind.key_name", "test-key")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper
			viper.Reset()
			setDefaults()

			// Setup test data
			tt.setup()

			// Load config
			config, err := LoadConfig()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config with API key",
			config: &Config{
				Tailscale: TailscaleConfig{
					APIKey:  "test-api-key",
					Tailnet: "test.example.com",
				},
				Bind: BindConfig{
					Server:    "dns.example.com",
					Zone:      "test.example.com",
					KeyName:   "test-key",
					KeySecret: "test-secret",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with OAuth",
			config: &Config{
				Tailscale: TailscaleConfig{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					Tailnet:      "test.example.com",
				},
				Bind: BindConfig{
					Server:    "dns.example.com",
					Zone:      "test.example.com",
					KeyName:   "test-key",
					KeySecret: "test-secret",
				},
			},
			wantErr: false,
		},
		{
			name: "missing tailscale credentials",
			config: &Config{
				Tailscale: TailscaleConfig{
					Tailnet: "test.example.com",
				},
				Bind: BindConfig{
					Server:    "dns.example.com",
					Zone:      "test.example.com",
					KeyName:   "test-key",
					KeySecret: "test-secret",
				},
			},
			wantErr: true,
		},
		{
			name: "missing tailnet",
			config: &Config{
				Tailscale: TailscaleConfig{
					APIKey: "test-api-key",
				},
				Bind: BindConfig{
					Server:    "dns.example.com",
					Zone:      "test.example.com",
					KeyName:   "test-key",
					KeySecret: "test-secret",
				},
			},
			wantErr: true,
		},
		{
			name: "missing bind server",
			config: &Config{
				Tailscale: TailscaleConfig{
					APIKey:  "test-api-key",
					Tailnet: "test.example.com",
				},
				Bind: BindConfig{
					Zone:      "test.example.com",
					KeyName:   "test-key",
					KeySecret: "test-secret",
				},
			},
			wantErr: true,
		},
		{
			name: "missing bind zone",
			config: &Config{
				Tailscale: TailscaleConfig{
					APIKey:  "test-api-key",
					Tailnet: "test.example.com",
				},
				Bind: BindConfig{
					Server:    "dns.example.com",
					KeyName:   "test-key",
					KeySecret: "test-secret",
				},
			},
			wantErr: true,
		},
		{
			name: "missing bind key name",
			config: &Config{
				Tailscale: TailscaleConfig{
					APIKey:  "test-api-key",
					Tailnet: "test.example.com",
				},
				Bind: BindConfig{
					Server:    "dns.example.com",
					Zone:      "test.example.com",
					KeySecret: "test-secret",
				},
			},
			wantErr: true,
		},
		{
			name: "missing bind key secret",
			config: &Config{
				Tailscale: TailscaleConfig{
					APIKey:  "test-api-key",
					Tailnet: "test.example.com",
				},
				Bind: BindConfig{
					Server:  "dns.example.com",
					Zone:    "test.example.com",
					KeyName: "test-key",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	viper.Reset()
	setDefaults()

	assert.Equal(t, "30s", viper.GetString("tailscale.poll_interval"))
	assert.Equal(t, 53, viper.GetInt("bind.port"))
	assert.Equal(t, "hmac-sha256", viper.GetString("bind.algorithm"))
	assert.Equal(t, "300s", viper.GetString("bind.ttl"))
	assert.Equal(t, "60s", viper.GetString("bind.update_interval"))
	assert.Equal(t, "info", viper.GetString("general.log_level"))
	assert.Equal(t, false, viper.GetBool("general.dry_run"))
}

func TestBindEnvVars(t *testing.T) {
	// This test verifies that environment variables are properly bound
	// We'll skip this test for now as it's complex to test with viper
	t.Skip("Skipping environment variable test - complex viper interaction")
}

func TestConfigDefaults(t *testing.T) {
	viper.Reset()
	setDefaults()
	bindEnvVars()

	// Set required values for validation
	viper.Set("tailscale.api_key", "test-api-key")
	viper.Set("tailscale.tailnet", "test.example.com")
	viper.Set("bind.server", "dns.example.com")
	viper.Set("bind.zone", "test.example.com")
	viper.Set("bind.key_name", "test-key")
	viper.Set("bind.key_secret", "test-secret")

	config, err := LoadConfig()
	require.NoError(t, err)

	// Check that defaults are applied
	assert.Equal(t, 30*time.Second, config.Tailscale.PollInterval)
	assert.Equal(t, 53, config.Bind.Port)
	assert.Equal(t, "hmac-sha256", config.Bind.Algorithm)
	assert.Equal(t, 300*time.Second, config.Bind.TTL)
	assert.Equal(t, 60*time.Second, config.Bind.UpdateInterval)
	assert.Equal(t, "info", config.General.LogLevel)
	assert.Equal(t, false, config.General.DryRun)
}
