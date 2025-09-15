package bind

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aauren/tailscale-bind-ddns/pkg/config"
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

	// PTR configuration
	ptrConfig *config.PTRConfig
}

// DNSRecord represents a DNS record (A or PTR)
type DNSRecord struct {
	Name  string
	Value string
	TTL   uint32
	Type  string // "A" or "PTR"
}

// NewClient creates a new Bind DDNS client
func NewClient(
	server string,
	port int,
	zone, keyName, keySecret, algorithm string,
	ttl time.Duration,
	ptrConfig *config.PTRConfig,
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
		ptrConfig: ptrConfig,
	}, nil
}

// UpdateRecords updates DNS records for the given machines
func (c *Client) UpdateRecords(ctx context.Context, records []DNSRecord, dryRun bool) error {
	if dryRun {
		klog.Infof("DRY RUN: Would update %d DNS records", len(records))
		for _, record := range records {
			if record.Type == "PTR" {
				klog.V(1).Infof("DRY RUN: Would create/update PTR record %s -> %s (TTL: %d)",
					record.Name, record.Value, record.TTL)
			} else {
				klog.V(1).Infof("DRY RUN: Would create/update A record %s.%s -> %s (TTL: %d)",
					record.Name, c.zone, record.Value, record.TTL)
			}
		}
		return nil
	}

	if len(records) == 0 {
		klog.V(1).Info("No records to update")
		return nil
	}

	klog.Infof("Updating %d DNS records", len(records))

	// Group records by zone
	recordsByZone := make(map[string][]DNSRecord)
	for _, record := range records {
		var zone string
		if record.Type == "PTR" {
			// For PTR records, determine the zone dynamically based on the record name
			if strings.Contains(record.Name, ".in-addr.arpa.") {
				// IPv4 PTR record - extract zone from the record name
				zone = c.extractIPv4ZoneFromPTRName(record.Name)
			} else if strings.Contains(record.Name, ".ip6.arpa.") {
				// IPv6 PTR record - extract zone from the record name
				zone = c.extractIPv6ZoneFromPTRName(record.Name)
			}
		} else {
			// A/AAAA records go to the main zone
			zone = c.zone
		}

		if zone != "" {
			recordsByZone[zone] = append(recordsByZone[zone], record)
		}
	}

	// Create TSIG key
	key, err := c.createTSIGKey()
	if err != nil {
		return fmt.Errorf("creating TSIG key: %w", err)
	}

	// Send updates for each zone
	for zone, zoneRecords := range recordsByZone {
		klog.V(1).Infof("Sending %d records to zone %s", len(zoneRecords), zone)

		if err := c.sendZoneUpdate(ctx, zone, zoneRecords, key); err != nil {
			return fmt.Errorf("sending update to zone %s: %w", zone, err)
		}
	}

	return nil
}

// sendZoneUpdate sends DNS updates for a specific zone
func (c *Client) sendZoneUpdate(ctx context.Context, zone string, records []DNSRecord, key *dns.TSIG) error {
	// Create dynamic update message for this zone
	msg := new(dns.Msg)
	msg.SetUpdate(dns.Fqdn(zone))

	// Add records to the update message
	klog.V(2).Infof("Adding %d records to zone %s", len(records), zone)
	for _, record := range records {
		if record.Type == "PTR" {
			// Handle PTR records
			klog.V(1).Infof("Processing PTR record: %s -> %s", record.Name, record.Value)

			// Remove any existing PTR record for this name
			rrset := &dns.PTR{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(record.Name),
					Rrtype: dns.TypePTR,
					Class:  dns.ClassINET,
					Ttl:    0, // TTL 0 for removal
				},
			}
			msg.RemoveRRset([]dns.RR{rrset})

			// Add new PTR record
			ptrRecord := &dns.PTR{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(record.Name),
					Rrtype: dns.TypePTR,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				Ptr: dns.Fqdn(record.Value),
			}
			msg.Insert([]dns.RR{ptrRecord})

		} else {
			// Handle A/AAAA records (default)
			klog.V(1).Infof("Processing %s record: %s.%s -> %s", record.Type, record.Name, zone, record.Value)

			// Remove any existing A/AAAA record for this name
			var rrset dns.RR
			if record.Type == "AAAA" {
				rrset = &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(record.Name + "." + zone),
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    0, // TTL 0 for removal
					},
				}
			} else {
				rrset = &dns.A{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(record.Name + "." + zone),
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    0, // TTL 0 for removal
					},
				}
			}
			msg.RemoveRRset([]dns.RR{rrset})

			// Add new A/AAAA record
			var newRecord dns.RR
			if record.Type == "AAAA" {
				newRecord = &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(record.Name + "." + zone),
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    record.TTL,
					},
					AAAA: net.ParseIP(record.Value),
				}
			} else {
				newRecord = &dns.A{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(record.Name + "." + zone),
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    record.TTL,
					},
					A: net.ParseIP(record.Value),
				}
			}
			msg.Insert([]dns.RR{newRecord})
		}
	}

	// Sign the message with TSIG (300 seconds timeout)
	const tsigTimeout = 300
	msg.SetTsig(key.Hdr.Name, key.Algorithm, tsigTimeout, time.Now().Unix())

	// Send the update
	klog.V(2).Infof("Sending DNS update message to zone %s: %s", zone, msg.String())
	klog.V(2).Infof("Message Question Section: %v", msg.Question)

	client := new(dns.Client)
	client.TsigSecret = map[string]string{key.Hdr.Name: c.keySecret}

	response, _, err := client.ExchangeContext(ctx, msg, net.JoinHostPort(c.server, fmt.Sprintf("%d", c.port)))
	if err != nil {
		return fmt.Errorf("sending DNS update: %w", err)
	}

	if response.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("DNS update failed with Rcode %d: %s", response.Rcode, dns.RcodeToString[response.Rcode])
	}

	klog.V(1).Infof("Successfully updated %d records in zone %s", len(records), zone)
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

// isIPInSubnet checks if an IP address is within the specified subnet
func isIPInSubnet(ipStr, subnetStr string) bool {
	if subnetStr == "" {
		return false
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	_, network, err := net.ParseCIDR(subnetStr)
	if err != nil {
		return false
	}

	return network.Contains(ip)
}

// getPTRZoneForIP determines the correct PTR zone for an IP address based on subnet size
func (c *Client) getPTRZoneForIP(ipStr string, isIPv6 bool) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", ipStr)
	}

	if isIPv6 {
		if !c.ptrConfig.IPv6Enabled {
			return "", fmt.Errorf("IPv6 PTR records not enabled")
		}
		return c.generateIPv6PTRZone(ipStr, c.ptrConfig.IPv6SubnetSize)
	} else {
		return c.generateIPv4PTRZone(ipStr, c.ptrConfig.IPv4SubnetSize)
	}
}

// generateIPv4PTRZone generates the PTR zone name for IPv4 based on subnet size
func (c *Client) generateIPv4PTRZone(ipStr string, subnetSize int) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("invalid IPv4 address: %s", ipStr)
	}

	ipv4 := ip.To4()

	switch subnetSize {
	case 8:
		// /8: Use first octet (e.g., 100.64.0.1 -> 100.in-addr.arpa)
		return fmt.Sprintf("%d.in-addr.arpa", ipv4[0]), nil
	case 16:
		// /16: Use first two octets (e.g., 100.64.0.1 -> 64.100.in-addr.arpa)
		return fmt.Sprintf("%d.%d.in-addr.arpa", ipv4[1], ipv4[0]), nil
	case 24:
		// /24: Use first three octets (e.g., 100.64.0.1 -> 0.64.100.in-addr.arpa)
		return fmt.Sprintf("%d.%d.%d.in-addr.arpa", ipv4[2], ipv4[1], ipv4[0]), nil
	default:
		return "", fmt.Errorf("unsupported IPv4 subnet size: %d", subnetSize)
	}
}

// generateIPv6PTRZone generates the PTR zone name for IPv6 based on subnet size
func (c *Client) generateIPv6PTRZone(ipStr string, subnetSize int) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil || ip.To4() != nil {
		return "", fmt.Errorf("invalid IPv6 address: %s", ipStr)
	}

	// Convert IPv6 to nibble format
	nibbles := ipv6ToReverseDNS(ipStr)
	if nibbles == "" {
		return "", fmt.Errorf("failed to convert IPv6 to reverse DNS format")
	}

	// Remove the trailing dot and .ip6.arpa
	nibbles = strings.TrimSuffix(nibbles, ".ip6.arpa.")

	// Calculate how many nibbles to use based on subnet size
	var nibblesToUse int
	switch subnetSize {
	case 32:
		nibblesToUse = 8 // /32 = 8 nibbles
	case 48:
		nibblesToUse = 12 // /48 = 12 nibbles
	case 64:
		nibblesToUse = 16 // /64 = 16 nibbles
	default:
		return "", fmt.Errorf("unsupported IPv6 subnet size: %d", subnetSize)
	}

	// Split nibbles and take the required number from the end (network portion)
	nibbleParts := strings.Split(nibbles, ".")
	if len(nibbleParts) < nibblesToUse {
		return "", fmt.Errorf("not enough nibbles for subnet size %d", subnetSize)
	}

	// Take the last nibblesToUse nibbles (network portion)
	selectedNibbles := nibbleParts[len(nibbleParts)-nibblesToUse:]

	return strings.Join(selectedNibbles, ".") + ".ip6.arpa", nil
}

// extractIPv4ZoneFromPTRName extracts the zone name from an IPv4 PTR record name
func (c *Client) extractIPv4ZoneFromPTRName(ptrName string) string {
	if c.ptrConfig == nil || !c.ptrConfig.Enabled {
		return ""
	}

	// Remove trailing dot and .in-addr.arpa
	name := strings.TrimSuffix(ptrName, ".in-addr.arpa.")
	name = strings.TrimSuffix(name, ".")

	// Split by dots to get octets
	octets := strings.Split(name, ".")
	if len(octets) != 4 {
		return ""
	}

	// Generate zone based on subnet size
	switch c.ptrConfig.IPv4SubnetSize {
	case 8:
		// /8: Use first octet (e.g., 4.3.2.1 -> 1.in-addr.arpa)
		return fmt.Sprintf("%s.in-addr.arpa", octets[3])
	case 16:
		// /16: Use first two octets (e.g., 4.3.2.1 -> 2.1.in-addr.arpa)
		return fmt.Sprintf("%s.%s.in-addr.arpa", octets[2], octets[3])
	case 24:
		// /24: Use first three octets (e.g., 4.3.2.1 -> 3.2.1.in-addr.arpa)
		return fmt.Sprintf("%s.%s.%s.in-addr.arpa", octets[1], octets[2], octets[3])
	default:
		return ""
	}
}

// extractIPv6ZoneFromPTRName extracts the zone name from an IPv6 PTR record name
func (c *Client) extractIPv6ZoneFromPTRName(ptrName string) string {
	if c.ptrConfig == nil || !c.ptrConfig.Enabled || !c.ptrConfig.IPv6Enabled {
		return ""
	}

	// Remove trailing dot and .ip6.arpa
	name := strings.TrimSuffix(ptrName, ".ip6.arpa.")
	name = strings.TrimSuffix(name, ".")

	// Split by dots to get nibbles
	nibbles := strings.Split(name, ".")
	if len(nibbles) < 8 {
		return ""
	}

	// Calculate how many nibbles to use based on subnet size
	var nibblesToUse int
	switch c.ptrConfig.IPv6SubnetSize {
	case 32:
		nibblesToUse = 8 // /32 = 8 nibbles
	case 48:
		nibblesToUse = 12 // /48 = 12 nibbles
	case 64:
		nibblesToUse = 16 // /64 = 16 nibbles
	default:
		return ""
	}

	if len(nibbles) < nibblesToUse {
		return ""
	}

	// Take the last nibblesToUse nibbles (network portion)
	selectedNibbles := nibbles[len(nibbles)-nibblesToUse:]

	return strings.Join(selectedNibbles, ".") + ".ip6.arpa"
}

// CreatePTRRecord creates a PTR record for the given IP address and hostname
func (c *Client) CreatePTRRecord(ipStr, hostname string) (*DNSRecord, error) {
	if c.ptrConfig == nil || !c.ptrConfig.Enabled {
		return nil, nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	var ptrName string
	var subnet string

	if ip.To4() != nil {
		// IPv4 address
		if !isIPInSubnet(ipStr, c.ptrConfig.IPv4Subnet) {
			klog.Warningf("IPv4 address %s is not in configured subnet %s, skipping PTR record", ipStr, c.ptrConfig.IPv4Subnet)
			return nil, nil
		}

		// Create reverse DNS name for IPv4 (e.g., 1.2.3.4 -> 4.3.2.1.in-addr.arpa.)
		parts := strings.Split(ipStr, ".")
		if len(parts) != 4 {
			return nil, fmt.Errorf("invalid IPv4 address format: %s", ipStr)
		}

		// Reverse the IP address parts
		ptrName = fmt.Sprintf("%s.%s.%s.%s.in-addr.arpa.", parts[3], parts[2], parts[1], parts[0])
		subnet = c.ptrConfig.IPv4Subnet

	} else {
		// IPv6 address
		if !c.ptrConfig.IPv6Enabled {
			klog.V(2).Infof("IPv6 PTR records disabled, skipping IPv6 address %s", ipStr)
			return nil, nil
		}

		if !isIPInSubnet(ipStr, c.ptrConfig.IPv6Subnet) {
			klog.Warningf("IPv6 address %s is not in configured subnet %s, skipping PTR record", ipStr, c.ptrConfig.IPv6Subnet)
			return nil, nil
		}

		// Create reverse DNS name for IPv6
		ptrName = ipv6ToReverseDNS(ipStr)
		subnet = c.ptrConfig.IPv6Subnet
	}

	klog.V(2).Infof("Creating PTR record for %s -> %s (subnet: %s)", ptrName, hostname, subnet)

	return &DNSRecord{
		Name:  ptrName,
		Value: hostname,
		TTL:   c.ttl,
		Type:  "PTR",
	}, nil
}

// ipv6ToReverseDNS converts an IPv6 address to reverse DNS format
func ipv6ToReverseDNS(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	// Convert to 16-byte representation
	ipv6 := ip.To16()
	if ipv6 == nil {
		return ""
	}

	// Convert each byte to two nibbles and reverse the order
	var nibbles []string
	for i := len(ipv6) - 1; i >= 0; i-- {
		nibbles = append(nibbles, fmt.Sprintf("%x", ipv6[i]&0x0f))
		nibbles = append(nibbles, fmt.Sprintf("%x", (ipv6[i]&0xf0)>>4))
	}

	return strings.Join(nibbles, ".") + ".ip6.arpa."
}
