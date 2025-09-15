package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aauren/tailscale-bind-ddns/pkg/app"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

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
		klog.V(2).Infof("Configuration: %+v", cfg)

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

//nolint:gochecknoinits // This is a command line tool
func init() {
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
	bindRunFlagsToViper()
}

// bindRunFlagsToViper binds run command flags to viper configuration
func bindRunFlagsToViper() {
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
