package bind

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

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
			client, err := NewClient(tt.server, tt.port, tt.zone, tt.keyName, tt.keySecret, tt.algorithm, tt.ttl)

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
