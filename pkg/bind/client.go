package bind

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
	"k8s.io/klog/v2"
)

// Client represents a Bind DDNS client
type Client struct {
	server    string
	port      int
	zone      string
	keyName   string
	keySecret string
	algorithm string
	ttl       uint32
}

// DNSRecord represents a DNS A record
type DNSRecord struct {
	Name  string
	Value string
	TTL   uint32
}

// NewClient creates a new Bind DDNS client
func NewClient(
	server string,
	port int,
	zone, keyName, keySecret, algorithm string,
	ttl time.Duration,
) (*Client, error) {
	if server == "" {
		return nil, fmt.Errorf("server is required")
	}
	if zone == "" {
		return nil, fmt.Errorf("zone is required")
	}
	if keyName == "" {
		return nil, fmt.Errorf("key name is required")
	}
	if keySecret == "" {
		return nil, fmt.Errorf("key secret is required")
	}

	return &Client{
		server:    server,
		port:      port,
		zone:      zone,
		keyName:   keyName,
		keySecret: keySecret,
		algorithm: algorithm,
		ttl:       uint32(ttl.Seconds()),
	}, nil
}

// UpdateRecords updates DNS records for the given machines
func (c *Client) UpdateRecords(ctx context.Context, records []DNSRecord, dryRun bool) error {
	if dryRun {
		klog.Infof("DRY RUN: Would update %d DNS records", len(records))
		for _, record := range records {
			klog.V(1).Infof("DRY RUN: Would create/update A record %s.%s -> %s (TTL: %d)",
				record.Name, c.zone, record.Value, record.TTL)
		}
		return nil
	}

	if len(records) == 0 {
		klog.V(1).Info("No records to update")
		return nil
	}

	klog.Infof("Updating %d DNS records", len(records))

	// Create TSIG key
	key, err := c.createTSIGKey()
	if err != nil {
		return fmt.Errorf("creating TSIG key: %w", err)
	}

	// Create dynamic update message
	msg := new(dns.Msg)
	msg.SetUpdate(dns.Fqdn(c.zone))

	// Add records to the update message
	klog.V(2).Infof("Adding %d records to the update message", len(records))
	for _, record := range records {
		// Remove any existing A record for this name
		klog.V(1).Infof("Removing A record for %s.%s", record.Name, c.zone)
		rrset := &dns.A{
			Hdr: dns.RR_Header{
				Name:   dns.Fqdn(record.Name + "." + c.zone),
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0, // TTL 0 for removal
			},
		}
		msg.RemoveRRset([]dns.RR{rrset})

		// Add new A record
		klog.V(1).Infof("Adding A record for %s.%s -> %s (TTL: %d)", record.Name, c.zone, record.Value, record.TTL)
		aRecord := &dns.A{
			Hdr: dns.RR_Header{
				Name:   dns.Fqdn(record.Name + "." + c.zone),
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    record.TTL,
			},
			A: net.ParseIP(record.Value),
		}
		msg.Insert([]dns.RR{aRecord})
	}

	// Sign the message with TSIG (300 seconds timeout)
	const tsigTimeout = 300
	msg.SetTsig(key.Hdr.Name, key.Algorithm, tsigTimeout, time.Now().Unix())

	klog.V(2).Infof("Sending DNS update message: %s", msg.String())
	klog.V(2).Infof("Message Question Section: %v", msg.Question)
	klog.V(2).Infof("Message NS Section: %s", msg.Ns)
	klog.V(2).Infof("Message Header Section: %s", &msg.MsgHdr)
	klog.V(2).Infof("Message Additional Section: %s", msg.Extra)

	// Send the update
	client := new(dns.Client)
	client.TsigSecret = map[string]string{key.Hdr.Name: c.keySecret}

	response, _, err := client.ExchangeContext(ctx, msg, net.JoinHostPort(c.server, fmt.Sprintf("%d", c.port)))
	if err != nil {
		return fmt.Errorf("sending DNS update: %w", err)
	}

	if response.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("DNS update failed with Rcode %d: %s", response.Rcode, dns.RcodeToString[response.Rcode])
	}

	klog.Infof("Successfully updated %d DNS records", len(records))
	return nil
}

// createTSIGKey creates a TSIG key for authentication
func (c *Client) createTSIGKey() (*dns.TSIG, error) {
	algorithm := c.algorithm
	if algorithm == "" {
		algorithm = "hmac-sha256"
	}

	// Validate algorithm
	switch algorithm {
	case "hmac-md5", "hmac-sha1", "hmac-sha256", "hmac-sha384", "hmac-sha512":
		// Valid algorithms
	default:
		return nil, fmt.Errorf("unsupported TSIG algorithm: %s", algorithm)
	}

	return &dns.TSIG{
		Hdr: dns.RR_Header{
			Name:   c.keyName,
			Rrtype: dns.TypeTSIG,
			Class:  dns.ClassANY,
		},
		Algorithm: fmt.Sprintf("%s.", algorithm),
	}, nil
}

// ValidateConnection tests the connection to the Bind server
func (c *Client) ValidateConnection(ctx context.Context) error {
	klog.V(1).Infof("Validating connection to Bind server %s:%d", c.server, c.port)

	// Create a simple query to test connectivity
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(c.zone), dns.TypeSOA)

	client := new(dns.Client)
	client.Timeout = 5 * time.Second

	response, _, err := client.ExchangeContext(ctx, msg, net.JoinHostPort(c.server, fmt.Sprintf("%d", c.port)))
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	if response.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("connection test failed with Rcode %d: %s", response.Rcode, dns.RcodeToString[response.Rcode])
	}

	klog.V(1).Info("Successfully validated connection to Bind server")
	return nil
}

// StartUpdating starts the DDNS update process
func (c *Client) StartUpdating(
	ctx context.Context,
	updateInterval time.Duration,
	recordChan <-chan []DNSRecord,
	dryRun bool,
) {
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	klog.Infof("Starting DDNS updates with interval %v", updateInterval)

	// Process initial records if available
	select {
	case records := <-recordChan:
		if err := c.UpdateRecords(ctx, records, dryRun); err != nil {
			klog.Errorf("Failed to update initial records: %v", err)
		}
	case <-ctx.Done():
		return
	}

	for {
		select {
		case records := <-recordChan:
			if err := c.UpdateRecords(ctx, records, dryRun); err != nil {
				klog.Errorf("Failed to update records: %v", err)
			}

		case <-ticker.C:
			// Periodic update - check if there are any pending records
			select {
			case records := <-recordChan:
				if err := c.UpdateRecords(ctx, records, dryRun); err != nil {
					klog.Errorf("Failed to update records: %v", err)
				}
			default:
				// No records to process
			}

		case <-ctx.Done():
			klog.Info("DDNS updating stopped")
			return
		}
	}
}
