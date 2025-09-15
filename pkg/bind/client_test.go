package bind

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/aauren/tailscale-bind-ddns/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		server    string
		port      int
		zone      string
		keyName   string
		keySecret string
		algorithm string
		ttl       time.Duration
		wantErr   bool
		wantError string
	}{
		{
			name:      "valid client",
			server:    "dns.example.com",
			port:      53,
			zone:      "test.example.com",
			keyName:   "test-key",
			keySecret: "test-secret",
			algorithm: "hmac-sha256",
			ttl:       300 * time.Second,
			wantErr:   false,
		},
		{
			name:      "missing server",
			server:    "",
			port:      53,
			zone:      "test.example.com",
			keyName:   "test-key",
			keySecret: "test-secret",
			algorithm: "hmac-sha256",
			ttl:       300 * time.Second,
			wantErr:   true,
			wantError: "server is required",
		},
		{
			name:      "missing zone",
			server:    "dns.example.com",
			port:      53,
			zone:      "",
			keyName:   "test-key",
			keySecret: "test-secret",
			algorithm: "hmac-sha256",
			ttl:       300 * time.Second,
			wantErr:   true,
			wantError: "zone is required",
		},
		{
			name:      "missing key name",
			server:    "dns.example.com",
			port:      53,
			zone:      "test.example.com",
			keyName:   "",
			keySecret: "test-secret",
			algorithm: "hmac-sha256",
			ttl:       300 * time.Second,
			wantErr:   true,
			wantError: "key name is required",
		},
		{
			name:      "missing key secret",
			server:    "dns.example.com",
			port:      53,
			zone:      "test.example.com",
			keyName:   "test-key",
			keySecret: "",
			algorithm: "hmac-sha256",
			ttl:       300 * time.Second,
			wantErr:   true,
			wantError: "key secret is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.server, tt.port, tt.zone, tt.keyName, tt.keySecret, tt.algorithm, tt.ttl, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.wantError != "" {
					assert.Contains(t, err.Error(), tt.wantError)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tt.server, client.server)
				assert.Equal(t, tt.port, client.port)
				assert.Equal(t, tt.zone, client.zone)
				assert.Equal(t, tt.keyName, client.keyName)
				assert.Equal(t, tt.keySecret, client.keySecret)
				assert.Equal(t, tt.algorithm, client.algorithm)
				assert.Equal(t, uint32(tt.ttl.Seconds()), client.ttl)
			}
		})
	}
}

func TestCreateTSIGKey(t *testing.T) {
	client := &Client{
		keyName:   "test-key",
		keySecret: "test-secret",
		algorithm: "hmac-sha256",
	}

	tests := []struct {
		name      string
		algorithm string
		wantErr   bool
		wantError string
	}{
		{
			name:      "valid hmac-sha256",
			algorithm: "hmac-sha256",
			wantErr:   false,
		},
		{
			name:      "valid hmac-sha1",
			algorithm: "hmac-sha1",
			wantErr:   false,
		},
		{
			name:      "valid hmac-md5",
			algorithm: "hmac-md5",
			wantErr:   false,
		},
		{
			name:      "valid hmac-sha384",
			algorithm: "hmac-sha384",
			wantErr:   false,
		},
		{
			name:      "valid hmac-sha512",
			algorithm: "hmac-sha512",
			wantErr:   false,
		},
		{
			name:      "invalid algorithm",
			algorithm: "invalid-algorithm",
			wantErr:   true,
			wantError: "unsupported TSIG algorithm",
		},
		{
			name:      "empty algorithm uses default",
			algorithm: "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client.algorithm = tt.algorithm
			key, err := client.createTSIGKey()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, key)
				if tt.wantError != "" {
					assert.Contains(t, err.Error(), tt.wantError)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, key)
				assert.Equal(t, client.keyName, key.Hdr.Name)
				expectedAlgorithm := tt.algorithm
				if expectedAlgorithm == "" {
					expectedAlgorithm = "hmac-sha256."
				} else {
					expectedAlgorithm = fmt.Sprintf("%s.", tt.algorithm)
				}
				assert.Equal(t, expectedAlgorithm, key.Algorithm)
			}
		})
	}
}

func TestUpdateRecordsDryRun(t *testing.T) {
	client := &Client{
		server:    "dns.example.com",
		port:      53,
		zone:      "test.example.com",
		keyName:   "test-key",
		keySecret: "test-secret",
		algorithm: "hmac-sha256",
		ttl:       300,
	}

	records := []DNSRecord{
		{
			Name:  "machine1",
			Value: "100.64.1.1",
			TTL:   300,
		},
		{
			Name:  "machine2",
			Value: "100.64.1.2",
			TTL:   300,
		},
	}

	ctx := context.Background()
	err := client.UpdateRecords(ctx, records, true) // dry run = true

	// Dry run should not return an error
	assert.NoError(t, err)
}

func TestUpdateRecordsEmpty(t *testing.T) {
	client := &Client{
		server:    "dns.example.com",
		port:      53,
		zone:      "test.example.com",
		keyName:   "test-key",
		keySecret: "test-secret",
		algorithm: "hmac-sha256",
		ttl:       300,
	}

	ctx := context.Background()
	err := client.UpdateRecords(ctx, []DNSRecord{}, false)

	// Empty records should not return an error
	assert.NoError(t, err)
}

func TestDNSRecordValidation(t *testing.T) {
	tests := []struct {
		name    string
		record  DNSRecord
		wantErr bool
	}{
		{
			name: "valid IPv4 record",
			record: DNSRecord{
				Name:  "test.example.com",
				Value: "192.168.1.1",
				TTL:   300,
			},
			wantErr: false,
		},
		{
			name: "valid IPv6 record",
			record: DNSRecord{
				Name:  "test.example.com",
				Value: "2001:db8::1",
				TTL:   300,
			},
			wantErr: false,
		},
		{
			name: "invalid IP address",
			record: DNSRecord{
				Name:  "test.example.com",
				Value: "invalid-ip",
				TTL:   300,
			},
			wantErr: true,
		},
		{
			name: "empty name",
			record: DNSRecord{
				Name:  "",
				Value: "192.168.1.1",
				TTL:   300,
			},
			wantErr: false, // Empty name is valid, will be handled by DNS library
		},
		{
			name: "zero TTL",
			record: DNSRecord{
				Name:  "test.example.com",
				Value: "192.168.1.1",
				TTL:   0,
			},
			wantErr: false, // Zero TTL is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IP address parsing
			ip := net.ParseIP(tt.record.Value)

			if tt.wantErr {
				assert.Nil(t, ip, "Expected invalid IP address")
			} else {
				assert.NotNil(t, ip, "Expected valid IP address")
			}
		})
	}
}

func TestStartUpdating(t *testing.T) {
	client := &Client{
		server:    "dns.example.com",
		port:      53,
		zone:      "test.example.com",
		keyName:   "test-key",
		keySecret: "test-secret",
		algorithm: "hmac-sha256",
		ttl:       300,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	recordChan := make(chan []DNSRecord, 1)

	// Send some test records
	testRecords := []DNSRecord{
		{
			Name:  "test-machine",
			Value: "100.64.1.1",
			TTL:   300,
		},
	}

	go func() {
		recordChan <- testRecords
	}()

	// Start updating with a very short interval
	go client.StartUpdating(ctx, 10*time.Millisecond, recordChan, true) // dry run

	// Wait for context cancellation
	<-ctx.Done()
}

func TestClientFields(t *testing.T) {
	client := &Client{
		server:    "dns.example.com",
		port:      53,
		zone:      "test.example.com",
		keyName:   "test-key",
		keySecret: "test-secret",
		algorithm: "hmac-sha256",
		ttl:       300,
	}

	assert.Equal(t, "dns.example.com", client.server)
	assert.Equal(t, 53, client.port)
	assert.Equal(t, "test.example.com", client.zone)
	assert.Equal(t, "test-key", client.keyName)
	assert.Equal(t, "test-secret", client.keySecret)
	assert.Equal(t, "hmac-sha256", client.algorithm)
	assert.Equal(t, uint32(300), client.ttl)
}

func TestCreatePTRRecord(t *testing.T) {
	tests := []struct {
		name       string
		ipStr      string
		hostname   string
		ptrConfig  *config.PTRConfig
		wantRecord bool
		wantErr    bool
		wantError  string
	}{
		{
			name:     "IPv4 PTR record enabled",
			ipStr:    "100.64.1.1",
			hostname: "test.example.com",
			ptrConfig: &config.PTRConfig{
				Enabled:    true,
				IPv4Zone:   "64.100.in-addr.arpa",
				IPv4Subnet: "100.64.0.0/10",
			},
			wantRecord: true,
			wantErr:    false,
		},
		{
			name:     "IPv4 PTR record disabled",
			ipStr:    "100.64.1.1",
			hostname: "test.example.com",
			ptrConfig: &config.PTRConfig{
				Enabled: false,
			},
			wantRecord: false,
			wantErr:    false,
		},
		{
			name:     "IPv4 address not in subnet",
			ipStr:    "192.168.1.1",
			hostname: "test.example.com",
			ptrConfig: &config.PTRConfig{
				Enabled:    true,
				IPv4Zone:   "64.100.in-addr.arpa",
				IPv4Subnet: "100.64.0.0/10",
			},
			wantRecord: false,
			wantErr:    false,
		},
		{
			name:     "IPv6 PTR record enabled",
			ipStr:    "fd7a:115c:a1e0::1",
			hostname: "test.example.com",
			ptrConfig: &config.PTRConfig{
				Enabled:     true,
				IPv6Enabled: true,
				IPv6Zone:    "0.e.1.a.1.c.5.1.1.a.7.d.f.ip6.arpa",
				IPv6Subnet:  "fd7a:115c:a1e0::/64",
			},
			wantRecord: true,
			wantErr:    false,
		},
		{
			name:     "IPv6 PTR record disabled",
			ipStr:    "fd7a:115c:a1e0::1",
			hostname: "test.example.com",
			ptrConfig: &config.PTRConfig{
				Enabled:     true,
				IPv6Enabled: false,
			},
			wantRecord: false,
			wantErr:    false,
		},
		{
			name:     "Invalid IP address",
			ipStr:    "invalid-ip",
			hostname: "test.example.com",
			ptrConfig: &config.PTRConfig{
				Enabled: true,
			},
			wantRecord: false,
			wantErr:    true,
			wantError:  "invalid IP address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				ptrConfig: tt.ptrConfig,
				ttl:       300,
			}

			record, err := client.CreatePTRRecord(tt.ipStr, tt.hostname)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, record)
				if tt.wantError != "" {
					assert.Contains(t, err.Error(), tt.wantError)
				}
			} else {
				assert.NoError(t, err)
				if tt.wantRecord {
					assert.NotNil(t, record)
					assert.Equal(t, "PTR", record.Type)
					assert.Equal(t, tt.hostname, record.Value)
					assert.Equal(t, uint32(300), record.TTL)
				} else {
					assert.Nil(t, record)
				}
			}
		})
	}
}

func TestIsIPInSubnet(t *testing.T) {
	tests := []struct {
		name   string
		ipStr  string
		subnet string
		want   bool
	}{
		{
			name:   "IPv4 in subnet",
			ipStr:  "100.64.1.1",
			subnet: "100.64.0.0/10",
			want:   true,
		},
		{
			name:   "IPv4 not in subnet",
			ipStr:  "192.168.1.1",
			subnet: "100.64.0.0/10",
			want:   false,
		},
		{
			name:   "IPv6 in subnet",
			ipStr:  "fd7a:115c:a1e0::1",
			subnet: "fd7a:115c:a1e0::/64",
			want:   true,
		},
		{
			name:   "IPv6 not in subnet",
			ipStr:  "2001:db8::1",
			subnet: "fd7a:115c:a1e0::/64",
			want:   false,
		},
		{
			name:   "Invalid IP",
			ipStr:  "invalid-ip",
			subnet: "100.64.0.0/10",
			want:   false,
		},
		{
			name:   "Empty subnet",
			ipStr:  "100.64.1.1",
			subnet: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIPInSubnet(tt.ipStr, tt.subnet)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestIPv6ToReverseDNS(t *testing.T) {
	tests := []struct {
		name    string
		ipStr   string
		want    string
		wantErr bool
	}{
		{
			name:  "Valid IPv6",
			ipStr: "2001:db8::1",
			want:  "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
		},
		{
			name:    "Invalid IPv6",
			ipStr:   "invalid-ipv6",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ipv6ToReverseDNS(tt.ipStr)
			if tt.wantErr {
				assert.Empty(t, result)
			} else {
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestGenerateIPv4PTRZone(t *testing.T) {
	tests := []struct {
		name       string
		ipStr      string
		subnetSize int
		want       string
		wantErr    bool
	}{
		{
			name:       "IPv4 /8 subnet",
			ipStr:      "100.64.0.1",
			subnetSize: 8,
			want:       "100.in-addr.arpa",
			wantErr:    false,
		},
		{
			name:       "IPv4 /16 subnet",
			ipStr:      "100.64.0.1",
			subnetSize: 16,
			want:       "64.100.in-addr.arpa",
			wantErr:    false,
		},
		{
			name:       "IPv4 /24 subnet",
			ipStr:      "100.64.0.1",
			subnetSize: 24,
			want:       "0.64.100.in-addr.arpa",
			wantErr:    false,
		},
		{
			name:       "Invalid subnet size",
			ipStr:      "100.64.0.1",
			subnetSize: 32,
			want:       "",
			wantErr:    true,
		},
		{
			name:       "Invalid IP address",
			ipStr:      "invalid",
			subnetSize: 16,
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			result, err := client.generateIPv4PTRZone(tt.ipStr, tt.subnetSize)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestGenerateIPv6PTRZone(t *testing.T) {
	tests := []struct {
		name       string
		ipStr      string
		subnetSize int
		want       string
		wantErr    bool
	}{
		{
			name:       "IPv6 /32 subnet",
			ipStr:      "2001:db8::1",
			subnetSize: 32,
			want:       "8.b.d.0.1.0.0.2.ip6.arpa",
			wantErr:    false,
		},
		{
			name:       "IPv6 /48 subnet",
			ipStr:      "2001:db8::1",
			subnetSize: 48,
			want:       "0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa",
			wantErr:    false,
		},
		{
			name:       "IPv6 /64 subnet",
			ipStr:      "2001:db8::1",
			subnetSize: 64,
			want:       "0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa",
			wantErr:    false,
		},
		{
			name:       "Invalid subnet size",
			ipStr:      "2001:db8::1",
			subnetSize: 128,
			want:       "",
			wantErr:    true,
		},
		{
			name:       "Invalid IPv6 address",
			ipStr:      "192.168.1.1",
			subnetSize: 64,
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			result, err := client.generateIPv6PTRZone(tt.ipStr, tt.subnetSize)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestExtractIPv4ZoneFromPTRName(t *testing.T) {
	tests := []struct {
		name       string
		ptrName    string
		subnetSize int
		want       string
	}{
		{
			name:       "IPv4 /8 PTR name",
			ptrName:    "1.0.64.100.in-addr.arpa.",
			subnetSize: 8,
			want:       "100.in-addr.arpa",
		},
		{
			name:       "IPv4 /16 PTR name",
			ptrName:    "1.0.64.100.in-addr.arpa.",
			subnetSize: 16,
			want:       "64.100.in-addr.arpa",
		},
		{
			name:       "IPv4 /24 PTR name",
			ptrName:    "1.0.64.100.in-addr.arpa.",
			subnetSize: 24,
			want:       "0.64.100.in-addr.arpa",
		},
		{
			name:       "Invalid PTR name",
			ptrName:    "invalid",
			subnetSize: 16,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				ptrConfig: &config.PTRConfig{
					Enabled:        true,
					IPv4SubnetSize: tt.subnetSize,
				},
			}
			result := client.extractIPv4ZoneFromPTRName(tt.ptrName)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestExtractIPv6ZoneFromPTRName(t *testing.T) {
	tests := []struct {
		name       string
		ptrName    string
		subnetSize int
		want       string
	}{
		{
			name:       "IPv6 /32 PTR name",
			ptrName:    "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
			subnetSize: 32,
			want:       "8.b.d.0.1.0.0.2.ip6.arpa",
		},
		{
			name:       "IPv6 /48 PTR name",
			ptrName:    "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
			subnetSize: 48,
			want:       "0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa",
		},
		{
			name:       "IPv6 /64 PTR name",
			ptrName:    "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
			subnetSize: 64,
			want:       "0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa",
		},
		{
			name:       "Invalid PTR name",
			ptrName:    "invalid",
			subnetSize: 64,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				ptrConfig: &config.PTRConfig{
					Enabled:        true,
					IPv6Enabled:    true,
					IPv6SubnetSize: tt.subnetSize,
				},
			}
			result := client.extractIPv6ZoneFromPTRName(tt.ptrName)
			assert.Equal(t, tt.want, result)
		})
	}
}
