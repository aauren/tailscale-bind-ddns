# Configuration

The application supports three configuration methods (in order of precedence):

1. **Command Line Flags**
2. **Environment Variables** (prefixed with `TSBD_`)
3. **YAML Configuration File**

## Configuration Options

### Tailscale Configuration

| Option | CLI Flag | Environment Variable | Description |
|--------|----------|---------------------|-------------|
| API Key | `--tailscale-api-key` | `TSBD_TAILSCALE_API_KEY` | Tailscale API key (recommended) |
| Client ID | `--tailscale-client-id` | `TSBD_TAILSCALE_CLIENT_ID` | OAuth client ID |
| Client Secret | `--tailscale-client-secret` | `TSBD_TAILSCALE_CLIENT_SECRET` | OAuth client secret |
| Tailnet | `--tailscale-tailnet` | `TSBD_TAILSCALE_TAILNET` | Your Tailscale tailnet name |
| Poll Interval | `--tailscale-poll-interval` | `TSBD_TAILSCALE_POLL_INTERVAL` | How often to poll Tailscale (default: 30s) |

### Bind DNS Configuration

| Option | CLI Flag | Environment Variable | Description |
|--------|----------|---------------------|-------------|
| Server | `--bind-server` | `TSBD_BIND_SERVER` | DNS server address |
| Port | `--bind-port` | `TSBD_BIND_PORT` | DNS server port (default: 53) |
| Zone | `--bind-zone` | `TSBD_BIND_ZONE` | DNS zone to update |
| Key Name | `--bind-key-name` | `TSBD_BIND_KEY_NAME` | TSIG key name |
| Key Secret | `--bind-key-secret` | `TSBD_BIND_KEY_SECRET` | TSIG key secret |
| Algorithm | `--bind-algorithm` | `TSBD_BIND_ALGORITHM` | TSIG algorithm (default: hmac-sha256) |
| TTL | `--bind-ttl` | `TSBD_BIND_TTL` | DNS record TTL (default: 300s) |
| Update Interval | `--bind-update-interval` | `TSBD_BIND_UPDATE_INTERVAL` | DNS update interval (default: 60s) |

### PTR Record Configuration

| Option | CLI Flag | Environment Variable | Description |
|--------|----------|---------------------|-------------|
| Enabled | `--ptr-enabled` | `TSBD_PTR_ENABLED` | Enable PTR record creation (default: false) |
| IPv4 Zone | `--ptr-ipv4-zone` | `TSBD_PTR_IPV4_ZONE` | IPv4 PTR zone name (required when enabled) |
| IPv4 Subnet | `--ptr-ipv4-subnet` | `TSBD_PTR_IPV4_SUBNET` | IPv4 subnet for PTR records (default: 100.64.0.0/10) |
| IPv4 Subnet Size | `--ptr-ipv4-subnet-size` | `TSBD_PTR_IPV4_SUBNET_SIZE` | IPv4 subnet boundary: 8, 16, or 24 (default: 16) |
| IPv6 Enabled | `--ptr-ipv6-enabled` | `TSBD_PTR_IPV6_ENABLED` | Enable IPv6 PTR records (default: false) |
| IPv6 Zone | `--ptr-ipv6-zone` | `TSBD_PTR_IPV6_ZONE` | IPv6 PTR zone name (required when IPv6 enabled) |
| IPv6 Subnet | `--ptr-ipv6-subnet` | `TSBD_PTR_IPV6_SUBNET` | IPv6 subnet for PTR records (required when IPv6 enabled) |
| IPv6 Subnet Size | `--ptr-ipv6-subnet-size` | `TSBD_PTR_IPV6_SUBNET_SIZE` | IPv6 subnet boundary: 32, 48, or 64 (default: 64) |

### General Configuration

| Option | CLI Flag | Environment Variable | Description |
|--------|----------|---------------------|-------------|
| Log Level | `--log-level` | `TSBD_LOG_LEVEL` | Log level (debug, verbose, info) (WARNING: debug level may leak secrets) |
| Dry Run | `--dry-run` | `TSBD_DRY_RUN` | Run in dry-run mode |

## Example Configuration File

```yaml
# config.yaml
tailscale:
  # OAuth credentials (recommended approach)
  client_id: "your-oauth-client-id"
  client_secret: "your-oauth-client-secret"
  tailnet: "your-tailnet.example.com"
  poll_interval: "30s"

bind:
  server: "dns.example.com"
  port: 53
  zone: "tailscale.example.com"
  key_name: "tailscale-key"
  key_secret: "your-tsig-key-secret-here"
  algorithm: "hmac-sha256"
  ttl: "300s"
  update_interval: "60s"

  # PTR record configuration (optional)
  ptr:
    enabled: true
    ipv4_zone: "64.100.in-addr.arpa"  # Reverse DNS zone for IPv4
    ipv4_subnet: "100.64.0.0/10"
    ipv4_subnet_size: 16              # Subnet boundary: /8, /16, or /24 (default: 16)
    ipv6_enabled: false               # Enable IPv6 PTR records
    #ipv6_zone: "0.e.1.a.1.c.5.1.1.a.7.d.f.ip6.arpa"  # Reverse DNS zone for IPv6
    #ipv6_subnet: "fd7a:115c:a1e0::/64"
    #ipv6_subnet_size: 64              # Subnet boundary: /32, /48, or /64 (default: 64)

general:
  log_level: "info"
  dry_run: false
```
