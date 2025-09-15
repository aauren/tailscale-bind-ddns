# PTR Records (Reverse DNS)

The application can optionally create PTR records (reverse DNS) for Tailscale machines. This allows you to perform reverse DNS lookups on Tailscale IP addresses.

## How PTR Records Work

PTR records map IP addresses back to hostnames. For example:
- Forward DNS: `machine1.tailscale.example.com` → `100.64.1.1`
- Reverse DNS: `1.1.64.100.in-addr.arpa` → `machine1.tailscale.example.com`

## Dynamic Zone Generation

The application can automatically generates PTR zones based on configurable subnet boundaries. This allows you to organize PTR records into appropriate zones without manual configuration.

**NOTE:** Creating the Bind configuration for a subnet as large as a `100.64.0.0/10` is complex and beyond
the scope of this help guide. If you are new to bind, I would recommend that you make your subnet smaller
rather than deal with it. Most people should be fine with a `/24`.

### IPv4 Subnet Boundaries

For IPv4 addresses, you can configure subnet boundaries of `/8`, `/16`, or `/24`:

- **/8**: Creates zones like `100.in-addr.arpa` (one zone per Class A network)
- **/16**: Creates zones like `64.100.in-addr.arpa` (one zone per Class B network) - **Default**
- **/24**: Creates zones like `1.64.100.in-addr.arpa` (one zone per Class C network)

**Example with /16 boundaries:**
- `100.64.1.1` → Zone: `64.100.in-addr.arpa`
- `100.65.1.1` → Zone: `65.100.in-addr.arpa`
- `100.66.1.1` → Zone: `66.100.in-addr.arpa`

### IPv6 Subnet Boundaries

For IPv6 addresses, you can configure subnet boundaries of `/32`, `/48`, or `/64`:

- **/32**: Creates zones with 8 nibbles (e.g., `8.b.d.0.1.0.0.2.ip6.arpa`)
- **/48**: Creates zones with 12 nibbles (e.g., `0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa`)
- **/64**: Creates zones with 16 nibbles (e.g., `0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa`) - **Default**

## Configuration

To enable PTR records, configure the `ptr` section in your config file:

```yaml
bind:
  ptr:
    enabled: true

    # IPv4 Configuration
    ipv4_zone: "64.100.in-addr.arpa"  # Base IPv4 reverse DNS zone (used for validation)
    ipv4_subnet: "100.64.0.0/10"      # Only create PTR records for IPs in this subnet
    ipv4_subnet_size: 16              # Subnet boundary: /8, /16, or /24 (default: 16)

    # IPv6 Configuration
    ipv6_enabled: false               # Enable IPv6 PTR records
    ipv6_zone: "0.e.1.a.1.c.5.1.1.a.7.d.f.ip6.arpa"  # Base IPv6 reverse DNS zone
    ipv6_subnet: "fd7a:115c:a1e0::/64"  # IPv6 subnet for PTR records
    ipv6_subnet_size: 64              # Subnet boundary: /32, /48, or /64 (default: 64)
```

## Subnet Validation

The application validates that each machine's IP address falls within the configured subnet before creating PTR records:

- **IPv4**: Only machines with IPs in `ipv4_subnet` will get PTR records
- **IPv6**: Only machines with IPs in `ipv6_subnet` will get PTR records (if enabled)
- **Out-of-subnet IPs**: Will be logged as warnings and skipped

## Zone Management

The application automatically groups PTR records by their calculated zones and sends separate DNS update messages for each zone. This ensures proper DNS organization and allows Bind to handle each zone independently.

**Example zone distribution for 100.64.0.0/10 with /16 boundaries:**
- Zone `64.100.in-addr.arpa`: Contains PTR records for 100.64.x.x addresses
- Zone `65.100.in-addr.arpa`: Contains PTR records for 100.65.x.x addresses
- Zone `66.100.in-addr.arpa`: Contains PTR records for 100.66.x.x addresses
- And so on...

## IPv6 PTR Records

For IPv6 PTR records, you need to:

1. Enable IPv6 PTR records in configuration
2. Configure the appropriate IPv6 reverse DNS zone
3. Ensure your Bind server supports IPv6

Example IPv6 configuration:
```yaml
bind:
  ptr:
    enabled: true
    ipv4_zone: "64.100.in-addr.arpa"  # IPv4 reverse zone
    ipv4_subnet: "100.64.0.0/10"
    ipv4_subnet_size: 16
    ipv6_enabled: true
    ipv6_zone: "0.e.1.a.1.c.5.1.1.a.7.d.f.ip6.arpa"  # IPv6 reverse zone for fd7a:115c:a1e0::/64
    ipv6_subnet: "fd7a:115c:a1e0::/64"
    ipv6_subnet_size: 64
```
