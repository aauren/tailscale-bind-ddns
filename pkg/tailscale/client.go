package tailscale

import (
	"context"
	"fmt"
	"time"

	"github.com/tailscale/tailscale-client-go/tailscale"
	"k8s.io/klog/v2"
)

// Client wraps the Tailscale client with additional functionality
type Client struct {
	client  *tailscale.Client
	tailnet string
}

// Machine represents a Tailscale machine
type Machine struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	IPv4Address string    `json:"ipv4_address"`
	IPv6Address string    `json:"ipv6_address"`
	LastSeen    time.Time `json:"last_seen"`
	Online      bool      `json:"online"`
}

// NewClient creates a new Tailscale client
func NewClient(apiKey, tailnet string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	if tailnet == "" {
		return nil, fmt.Errorf("tailnet is required")
	}

	client, err := tailscale.NewClient(apiKey, tailnet)
	if err != nil {
		return nil, fmt.Errorf("creating Tailscale client: %w", err)
	}

	return &Client{
		client:  client,
		tailnet: tailnet,
	}, nil
}

// NewOAuthClient creates a new Tailscale client using OAuth
func NewOAuthClient(clientID, clientSecret, tailnet string) (*Client, error) {
	if clientID == "" {
		return nil, fmt.Errorf("client ID is required")
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("client secret is required")
	}
	if tailnet == "" {
		return nil, fmt.Errorf("tailnet is required")
	}

	// Create client with OAuth credentials
	client, err := tailscale.NewClient(
		"",
		tailnet,
		tailscale.WithOAuthClientCredentials(clientID, clientSecret, []string{"devices:read"}),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OAuth Tailscale client: %w", err)
	}

	return &Client{
		client:  client,
		tailnet: tailnet,
	}, nil
}

// GetMachines retrieves all machines from the tailnet
func (c *Client) GetMachines(ctx context.Context) ([]Machine, error) {
	klog.V(2).Info("Fetching machines from Tailscale")

	devices, err := c.client.Devices(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching devices: %w", err)
	}

	var machines []Machine
	for _, device := range devices {
		machine := Machine{
			ID:       device.ID,
			Name:     device.Name,
			LastSeen: device.LastSeen.Time,
			Online:   device.Authorized, // Use Authorized as a proxy for online status
		}

		// Extract IPv4 address from the device's IP addresses
		for _, addr := range device.Addresses {
			if len(addr) >= 4 && addr[0] == 100 { // Tailscale IPv4 addresses start with 100.x.x.x
				machine.IPv4Address = addr
				break
			}
		}

		// Extract IPv6 address if available
		for _, addr := range device.Addresses {
			const ipv6AddrLen = 16 // IPv6 addresses are 16 bytes
			if len(addr) >= ipv6AddrLen {
				machine.IPv6Address = addr
				break
			}
		}

		machines = append(machines, machine)
	}

	klog.V(1).Infof("Found %d machines", len(machines))
	return machines, nil
}

// GetOnlineMachines retrieves only online machines from the tailnet
func (c *Client) GetOnlineMachines(ctx context.Context) ([]Machine, error) {
	machines, err := c.GetMachines(ctx)
	if err != nil {
		return nil, err
	}

	var onlineMachines []Machine
	for _, machine := range machines {
		if machine.Online {
			onlineMachines = append(onlineMachines, machine)
		}
	}

	klog.V(1).Infof("Found %d online machines", len(onlineMachines))
	return onlineMachines, nil
}

// StartPolling starts polling for machine updates and sends them to the provided channel
func (c *Client) StartPolling(ctx context.Context, pollInterval time.Duration, machineChan chan<- []Machine) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	klog.Infof("Starting Tailscale polling with interval %v", pollInterval)

	// Send initial data
	machines, err := c.GetOnlineMachines(ctx)
	if err != nil {
		klog.Errorf("Failed to get initial machine list: %v", err)
	} else {
		select {
		case machineChan <- machines:
		case <-ctx.Done():
			return
		}
	}

	for {
		select {
		case <-ticker.C:
			machines, err := c.GetOnlineMachines(ctx)
			if err != nil {
				klog.Errorf("Failed to get machines: %v", err)
				continue
			}

			select {
			case machineChan <- machines:
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			klog.Info("Tailscale polling stopped")
			return
		}
	}
}
