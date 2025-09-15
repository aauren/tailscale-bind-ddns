package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/aauren/tailscale-bind-ddns/pkg/config"
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
	rootCmd.PersistentFlags().String("log-level", "info",
		"log level (debug, verbose, info) (WARNING: debug level may leak secrets)")

	// Bind global flags to viper
	bindGlobalFlagsToViper()
}

// bindGlobalFlagsToViper binds global flags to viper configuration
func bindGlobalFlagsToViper() {
	// General flags
	if err := viper.BindPFlag("general.log_level", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		klog.Errorf("Failed to bind log-level flag: %v", err)
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
