package cmd

import (
	"fmt"

	"github.com/aauren/tailscale-bind-ddns/pkg/app"
	"github.com/spf13/cobra"
)

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
