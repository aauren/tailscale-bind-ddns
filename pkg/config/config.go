package config

import (
	"fmt"
	"net"
	"time"

	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

const (
	dnsStandardPort = 53 // DNS standard port
)

type Config struct {
	Tailscale TailscaleConfig `mapstructure:"tailscale"`
	Bind      BindConfig      `mapstructure:"bind"`
	General   GeneralConfig   `mapstructure:"general"`
}

// TailscaleConfig holds Tailscale-specific configuration
type TailscaleConfig struct {
	ClientID     string        `mapstructure:"client_id"`
	ClientSecret string        `mapstructure:"client_secret"`
	APIKey       string        `mapstructure:"api_key"`
	Tailnet      string        `mapstructure:"tailnet"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
}

// BindConfig holds Bind DNS server configuration
type BindConfig struct {
	Server         string        `mapstructure:"server"`
	Port           int           `mapstructure:"port"`
	Zone           string        `mapstructure:"zone"`
	KeyName        string        `mapstructure:"key_name"`
	KeySecret      string        `mapstructure:"key_secret"`
	Algorithm      string        `mapstructure:"algorithm"`
	TTL            time.Duration `mapstructure:"ttl"`
	UpdateInterval time.Duration `mapstructure:"update_interval"`

	// PTR record configuration
	PTR PTRConfig `mapstructure:"ptr"`
}

// PTRConfig holds PTR record configuration
type PTRConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	IPv4Zone       string `mapstructure:"ipv4_zone"`
	IPv4Subnet     string `mapstructure:"ipv4_subnet"`
	IPv4SubnetSize int    `mapstructure:"ipv4_subnet_size"` // /8, /16, or /24
	IPv6Enabled    bool   `mapstructure:"ipv6_enabled"`
	IPv6Zone       string `mapstructure:"ipv6_zone"`
	IPv6Subnet     string `mapstructure:"ipv6_subnet"`
	IPv6SubnetSize int    `mapstructure:"ipv6_subnet_size"` // /32, /48, or /64
}

// GeneralConfig holds general application configuration
type GeneralConfig struct {
	LogLevel string `mapstructure:"log_level"`
	DryRun   bool   `mapstructure:"dry_run"`
}

// LoadConfig loads configuration from multiple sources
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("$HOME/.tailscale-bind-ddns")

	// Set default values
	setDefaults()

	// Enable reading from environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("TSBD")

	// Bind environment variables
	bindEnvVars()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		// Config file not found is OK, we'll use defaults and env vars
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	viper.SetDefault("tailscale.poll_interval", "30s")
	viper.SetDefault("bind.port", dnsStandardPort)
	viper.SetDefault("bind.algorithm", "hmac-sha256")
	viper.SetDefault("bind.ttl", "300s")
	viper.SetDefault("bind.update_interval", "60s")
	viper.SetDefault("general.log_level", "info")
	viper.SetDefault("general.dry_run", false)

	// PTR record defaults
	viper.SetDefault("bind.ptr.enabled", false)
	viper.SetDefault("bind.ptr.ipv4_subnet", "100.64.0.0/10")
	viper.SetDefault("bind.ptr.ipv4_subnet_size", 16) // Default to /16 for IPv4
	viper.SetDefault("bind.ptr.ipv6_enabled", false)
	viper.SetDefault("bind.ptr.ipv6_subnet_size", 64) // Default to /64 for IPv6
}

// bindEnvVars binds environment variables to configuration keys
func bindEnvVars() {
	// Tailscale configuration
	if err := viper.BindEnv("tailscale.client_id", "TSBD_TAILSCALE_CLIENT_ID"); err != nil {
		klog.Errorf("Failed to bind TSBD_TAILSCALE_CLIENT_ID: %v", err)
	}
	if err := viper.BindEnv("tailscale.client_secret", "TSBD_TAILSCALE_CLIENT_SECRET"); err != nil {
		klog.Errorf("Failed to bind TSBD_TAILSCALE_CLIENT_SECRET: %v", err)
	}
	if err := viper.BindEnv("tailscale.api_key", "TSBD_TAILSCALE_API_KEY"); err != nil {
		klog.Errorf("Failed to bind TSBD_TAILSCALE_API_KEY: %v", err)
	}
	if err := viper.BindEnv("tailscale.tailnet", "TSBD_TAILSCALE_TAILNET"); err != nil {
		klog.Errorf("Failed to bind TSBD_TAILSCALE_TAILNET: %v", err)
	}
	if err := viper.BindEnv("tailscale.poll_interval", "TSBD_TAILSCALE_POLL_INTERVAL"); err != nil {
		klog.Errorf("Failed to bind TSBD_TAILSCALE_POLL_INTERVAL: %v", err)
	}

	// Bind configuration
	if err := viper.BindEnv("bind.server", "TSBD_BIND_SERVER"); err != nil {
		klog.Errorf("Failed to bind TSBD_BIND_SERVER: %v", err)
	}
	if err := viper.BindEnv("bind.port", "TSBD_BIND_PORT"); err != nil {
		klog.Errorf("Failed to bind TSBD_BIND_PORT: %v", err)
	}
	if err := viper.BindEnv("bind.zone", "TSBD_BIND_ZONE"); err != nil {
		klog.Errorf("Failed to bind TSBD_BIND_ZONE: %v", err)
	}
	if err := viper.BindEnv("bind.key_name", "TSBD_BIND_KEY_NAME"); err != nil {
		klog.Errorf("Failed to bind TSBD_BIND_KEY_NAME: %v", err)
	}
	if err := viper.BindEnv("bind.key_secret", "TSBD_BIND_KEY_SECRET"); err != nil {
		klog.Errorf("Failed to bind TSBD_BIND_KEY_SECRET: %v", err)
	}
	if err := viper.BindEnv("bind.algorithm", "TSBD_BIND_ALGORITHM"); err != nil {
		klog.Errorf("Failed to bind TSBD_BIND_ALGORITHM: %v", err)
	}
	if err := viper.BindEnv("bind.ttl", "TSBD_BIND_TTL"); err != nil {
		klog.Errorf("Failed to bind TSBD_BIND_TTL: %v", err)
	}
	if err := viper.BindEnv("bind.update_interval", "TSBD_BIND_UPDATE_INTERVAL"); err != nil {
		klog.Errorf("Failed to bind TSBD_BIND_UPDATE_INTERVAL: %v", err)
	}

	// PTR configuration
	if err := viper.BindEnv("bind.ptr.enabled", "TSBD_PTR_ENABLED"); err != nil {
		klog.Errorf("Failed to bind TSBD_PTR_ENABLED: %v", err)
	}
	if err := viper.BindEnv("bind.ptr.ipv4_zone", "TSBD_PTR_IPV4_ZONE"); err != nil {
		klog.Errorf("Failed to bind TSBD_PTR_IPV4_ZONE: %v", err)
	}
	if err := viper.BindEnv("bind.ptr.ipv4_subnet", "TSBD_PTR_IPV4_SUBNET"); err != nil {
		klog.Errorf("Failed to bind TSBD_PTR_IPV4_SUBNET: %v", err)
	}
	if err := viper.BindEnv("bind.ptr.ipv4_subnet_size", "TSBD_PTR_IPV4_SUBNET_SIZE"); err != nil {
		klog.Errorf("Failed to bind TSBD_PTR_IPV4_SUBNET_SIZE: %v", err)
	}
	if err := viper.BindEnv("bind.ptr.ipv6_enabled", "TSBD_PTR_IPV6_ENABLED"); err != nil {
		klog.Errorf("Failed to bind TSBD_PTR_IPV6_ENABLED: %v", err)
	}
	if err := viper.BindEnv("bind.ptr.ipv6_zone", "TSBD_PTR_IPV6_ZONE"); err != nil {
		klog.Errorf("Failed to bind TSBD_PTR_IPV6_ZONE: %v", err)
	}
	if err := viper.BindEnv("bind.ptr.ipv6_subnet", "TSBD_PTR_IPV6_SUBNET"); err != nil {
		klog.Errorf("Failed to bind TSBD_PTR_IPV6_SUBNET: %v", err)
	}
	if err := viper.BindEnv("bind.ptr.ipv6_subnet_size", "TSBD_PTR_IPV6_SUBNET_SIZE"); err != nil {
		klog.Errorf("Failed to bind TSBD_PTR_IPV6_SUBNET_SIZE: %v", err)
	}

	// General configuration
	if err := viper.BindEnv("general.log_level", "TSBD_LOG_LEVEL"); err != nil {
		klog.Errorf("Failed to bind TSBD_LOG_LEVEL: %v", err)
	}
	if err := viper.BindEnv("general.dry_run", "TSBD_DRY_RUN"); err != nil {
		klog.Errorf("Failed to bind TSBD_DRY_RUN: %v", err)
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Tailscale.ClientID == "" && c.Tailscale.APIKey == "" {
		return fmt.Errorf("either tailscale client_id or api_key must be provided")
	}

	if c.Tailscale.ClientSecret == "" && c.Tailscale.APIKey == "" {
		return fmt.Errorf("either tailscale client_secret or api_key must be provided")
	}

	if c.Tailscale.Tailnet == "" {
		return fmt.Errorf("tailscale tailnet must be provided")
	}

	if c.Bind.Server == "" {
		return fmt.Errorf("bind server must be provided")
	}

	if c.Bind.Zone == "" {
		return fmt.Errorf("bind zone must be provided")
	}

	if c.Bind.KeyName == "" {
		return fmt.Errorf("bind key_name must be provided")
	}

	if c.Bind.KeySecret == "" {
		return fmt.Errorf("bind key_secret must be provided")
	}

	// Validate PTR configuration if enabled
	if c.Bind.PTR.Enabled {
		// Validate IPv4 configuration
		if c.Bind.PTR.IPv4Zone == "" {
			return fmt.Errorf("IPv4 PTR zone must be provided when PTR records are enabled")
		}
		if c.Bind.PTR.IPv4Subnet != "" {
			if _, _, err := net.ParseCIDR(c.Bind.PTR.IPv4Subnet); err != nil {
				return fmt.Errorf("invalid IPv4 subnet format: %w", err)
			}
		}
		// Validate IPv4 subnet size
		if c.Bind.PTR.IPv4SubnetSize != 8 && c.Bind.PTR.IPv4SubnetSize != 16 && c.Bind.PTR.IPv4SubnetSize != 24 {
			return fmt.Errorf("IPv4 subnet size must be 8, 16, or 24")
		}

		// Validate IPv6 configuration if IPv6 is enabled
		if c.Bind.PTR.IPv6Enabled {
			if c.Bind.PTR.IPv6Zone == "" {
				return fmt.Errorf("IPv6 PTR zone must be provided when IPv6 PTR records are enabled")
			}
			if c.Bind.PTR.IPv6Subnet == "" {
				return fmt.Errorf("IPv6 subnet must be provided when IPv6 PTR records are enabled")
			}
			if _, _, err := net.ParseCIDR(c.Bind.PTR.IPv6Subnet); err != nil {
				return fmt.Errorf("invalid IPv6 subnet format: %w", err)
			}
			// Validate IPv6 subnet size
			if c.Bind.PTR.IPv6SubnetSize != 32 && c.Bind.PTR.IPv6SubnetSize != 48 && c.Bind.PTR.IPv6SubnetSize != 64 {
				return fmt.Errorf("IPv6 subnet size must be 32, 48, or 64")
			}
		}
	}

	return nil
}
