package cmd

import (
	"context"
	"fmt"

	"github.com/aauren/tailscale-bind-ddns/pkg/bind"
	"github.com/aauren/tailscale-bind-ddns/pkg/tailscale"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

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
