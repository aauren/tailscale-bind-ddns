package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aauren/tailscale-bind-ddns/pkg/app"
	"github.com/aauren/tailscale-bind-ddns/pkg/bind"
	"github.com/aauren/tailscale-bind-ddns/pkg/config"
	"github.com/aauren/tailscale-bind-ddns/pkg/tailscale"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

const (
	defaultPollInterval   = 30 * time.Second
	defaultBindPort       = 53
	defaultTTL            = 300 * time.Second
	defaultUpdateInterval = 60 * time.Second
	testTimeout           = 30 * time.Second
)

var (
	cfgFile string
	cfg     *config.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tailscale-bind-ddns",
	Short: "A tool to sync Tailscale machines to Bind DNS records",
	Long: `Tailscale-Bind DDNS is a tool that connects to your Tailscale tailnet,
retrieves the list of online machines, and automatically updates DNS A records
in your Bind server using RFC 2136 dynamic updates with TSIG authentication.

The tool supports configuration via:
- Command line flags
- Environment variables (prefixed with TSBD_)
- YAML configuration files

Example usage:
  tailscale-bind-ddns run --tailscale-api-key YOUR_KEY --bind-server dns.example.com
  tailscale-bind-ddns run --config config.yaml
  TSBD_TAILSCALE_API_KEY=your_key tailscale-bind-ddns run`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading configuration: %w", err)
		}
		return nil
	},
}

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the Tailscale-Bind DDNS sync",
	Long: `Run the main application that continuously syncs Tailscale machines
to Bind DNS records. The application will run until interrupted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging
		if err := setupLogging(); err != nil {
			return fmt.Errorf("setting up logging: %w", err)
		}

		klog.Info("Starting Tailscale-Bind DDNS application")
		klog.V(1).Infof("Configuration: %+v", cfg)

		// Create application
		application, err := app.NewApp(cfg)
		if err != nil {
			return fmt.Errorf("creating application: %w", err)
		}

		// Set up graceful shutdown
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			sig := <-sigChan
			klog.Infof("Received signal %v, shutting down...", sig)
			cancel()
		}()

		// Run the application
		return application.Run(ctx)
	},
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show application status",
	Long:  `Show the current status and configuration of the application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging
		if err := setupLogging(); err != nil {
			return fmt.Errorf("setting up logging: %w", err)
		}

		// Create application to get status
		application, err := app.NewApp(cfg)
		if err != nil {
			return fmt.Errorf("creating application: %w", err)
		}

		status := application.GetStatus()
		fmt.Println("Application Status:")
		for key, value := range status {
			fmt.Printf("  %s: %v\n", key, value)
		}

		return nil
	},
}

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connections to Tailscale and Bind",
	Long: `Test the connections to both Tailscale API and Bind DNS server
to verify configuration is correct.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging
		if err := setupLogging(); err != nil {
			return fmt.Errorf("setting up logging: %w", err)
		}

		klog.Info("Testing connections...")

		// Test Tailscale connection
		klog.Info("Testing Tailscale connection...")
		var tsClient *tailscale.Client
		var err error

		if cfg.Tailscale.APIKey != "" {
			tsClient, err = tailscale.NewClient(cfg.Tailscale.APIKey, cfg.Tailscale.Tailnet)
		} else {
			tsClient, err = tailscale.NewOAuthClient(cfg.Tailscale.ClientID, cfg.Tailscale.ClientSecret, cfg.Tailscale.Tailnet)
		}
		if err != nil {
			return fmt.Errorf("creating Tailscale client: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		machines, err := tsClient.GetMachines(ctx)
		if err != nil {
			return fmt.Errorf("testing Tailscale connection: %w", err)
		}
		klog.Infof("✓ Tailscale connection successful - found %d machines", len(machines))

		// Test Bind connection
		klog.Info("Testing Bind connection...")
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
			return fmt.Errorf("creating Bind client: %w", err)
		}

		if err := bindClient.ValidateConnection(ctx); err != nil {
			return fmt.Errorf("testing Bind connection: %w", err)
		}
		klog.Info("✓ Bind connection successful")

		klog.Info("All connections tested successfully!")
		return nil
	},
}

// setupLogging configures the logging based on the configuration
func setupLogging() error {
	// Set log level based on configuration
	switch cfg.General.LogLevel {
	case "debug":
		klog.InitFlags(nil)
		// Set verbosity to 2 for debug level
		klog.V(2).Info("Debug logging enabled")
	case "info":
		klog.InitFlags(nil)
	case "warn":
		klog.InitFlags(nil)
	case "error":
		klog.InitFlags(nil)
	default:
		return fmt.Errorf("invalid log level: %s", cfg.General.LogLevel)
	}

	return nil
}

// initializeCommands sets up all commands and flags
func initializeCommands() {
	// Add all commands
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(testCmd)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")

	// Run command flags
	runCmd.Flags().String("tailscale-api-key", "", "Tailscale API key")
	runCmd.Flags().String("tailscale-client-id", "", "Tailscale OAuth client ID")
	runCmd.Flags().String("tailscale-client-secret", "", "Tailscale OAuth client secret")
	runCmd.Flags().String("tailscale-tailnet", "", "Tailscale tailnet name")
	runCmd.Flags().Duration("tailscale-poll-interval", defaultPollInterval, "Tailscale polling interval")

	runCmd.Flags().String("bind-server", "", "Bind DNS server address")
	runCmd.Flags().Int("bind-port", defaultBindPort, "Bind DNS server port")
	runCmd.Flags().String("bind-zone", "", "DNS zone to update")
	runCmd.Flags().String("bind-key-name", "", "TSIG key name")
	runCmd.Flags().String("bind-key-secret", "", "TSIG key secret")
	runCmd.Flags().String("bind-algorithm", "hmac-sha256", "TSIG algorithm")
	runCmd.Flags().Duration("bind-ttl", defaultTTL, "DNS record TTL")
	runCmd.Flags().Duration("bind-update-interval", defaultUpdateInterval, "DNS update interval")

	runCmd.Flags().Bool("dry-run", false, "Run in dry-run mode (don't actually update DNS)")

	// Bind flags to viper
	bindFlagsToViper()
}

// bindFlagsToViper binds command line flags to viper configuration
func bindFlagsToViper() {
	// General flags
	if err := viper.BindPFlag("general.log_level", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		klog.Errorf("Failed to bind log-level flag: %v", err)
	}

	// Tailscale flags
	if err := viper.BindPFlag("tailscale.api_key", runCmd.Flags().Lookup("tailscale-api-key")); err != nil {
		klog.Errorf("Failed to bind tailscale-api-key flag: %v", err)
	}
	if err := viper.BindPFlag("tailscale.client_id", runCmd.Flags().Lookup("tailscale-client-id")); err != nil {
		klog.Errorf("Failed to bind tailscale-client-id flag: %v", err)
	}
	if err := viper.BindPFlag("tailscale.client_secret", runCmd.Flags().Lookup("tailscale-client-secret")); err != nil {
		klog.Errorf("Failed to bind tailscale-client-secret flag: %v", err)
	}
	if err := viper.BindPFlag("tailscale.tailnet", runCmd.Flags().Lookup("tailscale-tailnet")); err != nil {
		klog.Errorf("Failed to bind tailscale-tailnet flag: %v", err)
	}
	if err := viper.BindPFlag("tailscale.poll_interval", runCmd.Flags().Lookup("tailscale-poll-interval")); err != nil {
		klog.Errorf("Failed to bind tailscale-poll-interval flag: %v", err)
	}

	// Bind flags
	if err := viper.BindPFlag("bind.server", runCmd.Flags().Lookup("bind-server")); err != nil {
		klog.Errorf("Failed to bind bind-server flag: %v", err)
	}
	if err := viper.BindPFlag("bind.port", runCmd.Flags().Lookup("bind-port")); err != nil {
		klog.Errorf("Failed to bind bind-port flag: %v", err)
	}
	if err := viper.BindPFlag("bind.zone", runCmd.Flags().Lookup("bind-zone")); err != nil {
		klog.Errorf("Failed to bind bind-zone flag: %v", err)
	}
	if err := viper.BindPFlag("bind.key_name", runCmd.Flags().Lookup("bind-key-name")); err != nil {
		klog.Errorf("Failed to bind bind-key-name flag: %v", err)
	}
	if err := viper.BindPFlag("bind.key_secret", runCmd.Flags().Lookup("bind-key-secret")); err != nil {
		klog.Errorf("Failed to bind bind-key-secret flag: %v", err)
	}
	if err := viper.BindPFlag("bind.algorithm", runCmd.Flags().Lookup("bind-algorithm")); err != nil {
		klog.Errorf("Failed to bind bind-algorithm flag: %v", err)
	}
	if err := viper.BindPFlag("bind.ttl", runCmd.Flags().Lookup("bind-ttl")); err != nil {
		klog.Errorf("Failed to bind bind-ttl flag: %v", err)
	}
	if err := viper.BindPFlag("bind.update_interval", runCmd.Flags().Lookup("bind-update-interval")); err != nil {
		klog.Errorf("Failed to bind bind-update-interval flag: %v", err)
	}

	// General flags
	if err := viper.BindPFlag("general.dry_run", runCmd.Flags().Lookup("dry-run")); err != nil {
		klog.Errorf("Failed to bind dry-run flag: %v", err)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	initializeCommands()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
