# Tailscale-Bind DDNS

A Go application that automatically syncs Tailscale machines to Bind DNS records using RFC 2136 dynamic updates with TSIG authentication.

## Features

- **Tailscale Integration**: Connects to Tailscale using OAuth or API key authentication
- **Dynamic DNS Updates**: Updates Bind DNS server using RFC 2136 with TSIG security
- **PTR Records**: Optional reverse DNS (PTR) record creation with subnet validation
- **Real-time Sync**: Continuously monitors Tailscale machines and updates DNS records
- **Flexible Configuration**: Supports CLI flags, environment variables, and YAML configuration files
- **Goroutine-based Architecture**: Uses separate goroutines for Tailscale polling and DNS updates
- **Comprehensive Testing**: Achieves 42.4% test coverage with unit tests

## Architecture

The application consists of several key components:

- **Configuration Management**: Uses Viper and Cobra for flexible configuration
- **Tailscale Client**: Handles OAuth and API key authentication, machine listing
- **Bind DDNS Client**: Manages RFC 2136 dynamic updates with TSIG authentication
- **Application Coordinator**: Orchestrates communication between components using channels

## Installation

### Prerequisites

- Tailscale account with API access (See `Setup` section below for more details)
- Bind DNS server (or any RFC 2136 compliant DNS server) with TSIG key configured (See `Setup` below for more details)
- A tailscale-bind-ddns configuration file (see [config.yaml.example](./config.yaml.example) and the `Configuration` section below for more
  details)

### Pre-built Releases (Recommended)

Download the latest release from [GitHub Releases](https://github.com/aauren/tailscale-bind-ddns/releases)

```bash
# Ensure that you have a working configuration & connectivity to DNS and Tailscale first
./tailscale-bind-ddns --config=config.yaml test

# If everything above succeeds, then you can continue with:
./tailscale-bind-ddns --config=config.yaml run
```

### Container

You can get the latest container release at: [aauren/tailscale-bind-ddns](https://hub.docker.com/repository/docker/aauren/tailscale-bind-ddns/general)

#### Docker Run Example

```bash
# Ensure that you have a working configuration & connectivity to DNS and Tailscale first
docker run -ti --rm -v $(pwd)/config.yaml:/config/config.yaml aauren/tailscale-bind-ddns:latest test --config=/config/config.yaml

# If everything above succeeds, then you can continue with:
docker run -ti --rm -v $(pwd)/config.yaml:/config/config.yaml aauren/tailscale-bind-ddns:latest run --config=/config/config.yaml
```

#### Docker Compose Example

Download the [Docker Compose](./compose.yaml)

```bash
docker compose up -d
```

### Build from Source

#### Using Make (Recommended)

For this set of instructions you must have [Docker](https://www.docker.com/) installed.

```bash
git clone https://github.com/aauren/tailscale-bind-ddns.git
cd tailscale-bind-ddns

# Build for current platform
make tailscale-bind-ddns

# Or build for all platforms
make build-all
```

#### Using Go directly

```bash
git clone https://github.com/aauren/tailscale-bind-ddns.git
cd tailscale-bind-ddns
go build -o tailscale-bind-ddns .
```

## Configuration

The application supports three configuration methods (in order of precedence):

1. **Command Line Flags**
2. **Environment Variables** (prefixed with `TSBD_`)
3. **YAML Configuration File**

### Configuration Options

#### Tailscale Configuration

| Option | CLI Flag | Environment Variable | Description |
|--------|----------|---------------------|-------------|
| API Key | `--tailscale-api-key` | `TSBD_TAILSCALE_API_KEY` | Tailscale API key (recommended) |
| Client ID | `--tailscale-client-id` | `TSBD_TAILSCALE_CLIENT_ID` | OAuth client ID |
| Client Secret | `--tailscale-client-secret` | `TSBD_TAILSCALE_CLIENT_SECRET` | OAuth client secret |
| Tailnet | `--tailscale-tailnet` | `TSBD_TAILSCALE_TAILNET` | Your Tailscale tailnet name |
| Poll Interval | `--tailscale-poll-interval` | `TSBD_TAILSCALE_POLL_INTERVAL` | How often to poll Tailscale (default: 30s) |

#### Bind DNS Configuration

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

#### PTR Record Configuration

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

#### General Configuration

| Option | CLI Flag | Environment Variable | Description |
|--------|----------|---------------------|-------------|
| Log Level | `--log-level` | `TSBD_LOG_LEVEL` | Log level (debug, verbose, info) (WARNING: debug level may leak secrets) |
| Dry Run | `--dry-run` | `TSBD_DRY_RUN` | Run in dry-run mode |

### Example Configuration File

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

## PTR Records (Reverse DNS)

The application can optionally create PTR records (reverse DNS) for Tailscale machines. This allows you to perform reverse DNS lookups on Tailscale IP addresses.

### How PTR Records Work

PTR records map IP addresses back to hostnames. For example:
- Forward DNS: `machine1.tailscale.example.com` → `100.64.1.1`
- Reverse DNS: `1.1.64.100.in-addr.arpa` → `machine1.tailscale.example.com`

### Dynamic Zone Generation

The application can automatically generates PTR zones based on configurable subnet boundaries. This allows you to organize PTR records into appropriate zones without manual configuration.

**NOTE:** Creating the Bind configuration for a subnet as large as a `100.64.0.0/10` is complex and beyond
the scope of this help guide. If you are new to bind, I would recommend that you make your subnet smaller
rather than deal with it. Most people should be fine with a `/24`.

#### IPv4 Subnet Boundaries

For IPv4 addresses, you can configure subnet boundaries of `/8`, `/16`, or `/24`:

- **/8**: Creates zones like `100.in-addr.arpa` (one zone per Class A network)
- **/16**: Creates zones like `64.100.in-addr.arpa` (one zone per Class B network) - **Default**
- **/24**: Creates zones like `1.64.100.in-addr.arpa` (one zone per Class C network)

**Example with /16 boundaries:**
- `100.64.1.1` → Zone: `64.100.in-addr.arpa`
- `100.65.1.1` → Zone: `65.100.in-addr.arpa`
- `100.66.1.1` → Zone: `66.100.in-addr.arpa`

#### IPv6 Subnet Boundaries

For IPv6 addresses, you can configure subnet boundaries of `/32`, `/48`, or `/64`:

- **/32**: Creates zones with 8 nibbles (e.g., `8.b.d.0.1.0.0.2.ip6.arpa`)
- **/48**: Creates zones with 12 nibbles (e.g., `0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa`)
- **/64**: Creates zones with 16 nibbles (e.g., `0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa`) - **Default**

### Configuration

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

### Subnet Validation

The application validates that each machine's IP address falls within the configured subnet before creating PTR records:

- **IPv4**: Only machines with IPs in `ipv4_subnet` will get PTR records
- **IPv6**: Only machines with IPs in `ipv6_subnet` will get PTR records (if enabled)
- **Out-of-subnet IPs**: Will be logged as warnings and skipped

### Zone Management

The application automatically groups PTR records by their calculated zones and sends separate DNS update messages for each zone. This ensures proper DNS organization and allows Bind to handle each zone independently.

**Example zone distribution for 100.64.0.0/10 with /16 boundaries:**
- Zone `64.100.in-addr.arpa`: Contains PTR records for 100.64.x.x addresses
- Zone `65.100.in-addr.arpa`: Contains PTR records for 100.65.x.x addresses
- Zone `66.100.in-addr.arpa`: Contains PTR records for 100.66.x.x addresses
- And so on...

### IPv6 PTR Records

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

## Usage

### Basic Usage

```bash
# Using command line flags
./tailscale-bind-ddns run \
  --tailscale-api-key "your-api-key" \
  --tailscale-tailnet "your-tailnet.example.com" \
  --bind-server "dns.example.com" \
  --bind-zone "tailscale.example.com" \
  --bind-key-name "tailscale-key" \
  --bind-key-secret "your-tsig-secret"

# Using configuration file
./tailscale-bind-ddns run --config config.yaml

# Using environment variables
export TSBD_TAILSCALE_API_KEY="your-api-key"
export TSBD_TAILSCALE_TAILNET="your-tailnet.example.com"
export TSBD_BIND_SERVER="dns.example.com"
export TSBD_BIND_ZONE="tailscale.example.com"
export TSBD_BIND_KEY_NAME="tailscale-key"
export TSBD_BIND_KEY_SECRET="your-tsig-secret"
./tailscale-bind-ddns run
```

### Commands

#### `run`
Starts the main application that continuously syncs Tailscale machines to DNS records.

```bash
./tailscale-bind-ddns run [flags]
```

#### `test`
Tests connections to both Tailscale API and Bind DNS server.

```bash
./tailscale-bind-ddns test [flags]
```

#### `status`
Shows the current status and configuration of the application.

```bash
./tailscale-bind-ddns status [flags]
```

### Dry Run Mode

Test the application without making actual DNS changes:

```bash
./tailscale-bind-ddns run --dry-run
```

## Setup Instructions

### 1. Tailscale Setup

#### Option A: OAuth Client (Recommended)

1. Go to [Tailscale Admin Console](https://login.tailscale.com/admin/settings/oauth)
2. Create a new OAuth client with `devices:read` scope
3. Use the client ID and secret in your configuration

#### Option B: API Key

1. Go to [Tailscale Admin Console](https://login.tailscale.com/admin/settings/keys)
2. Generate a new API key with appropriate permissions
3. Use the API key in your configuration

#### Obtain Your Tailnet Name

1. Go to [Tailscale Admin Console](https://login.tailscale.com/admin/dns)
2. Find the section that says `Tailnet Name`
3. Fill that into the configuration

### 2. Bind DNS Setup

1. **Generate TSIG Key**:

```bash
tsig-keygen -a HMAC-SHA256 tailscale-bind-ddns-key
```

2. **Configure Bind** (`/etc/named.conf`):

```
key "tailscale-bind-ddns-key" {
    algorithm hmac-sha256;
    secret "your-generated-secret";
};

zone "tailscale.example.com" {
    type master;
    file "tailscale.example.com.zone";
    allow-update { key "tailscale-bind-ddns-key"; };
};

// OPTIONAL: if you are enabling reverse DNS functionality (note this would only cover addresses
// 100.64.0.0-100.64.255.255, if you want to cover the entire /10 subnet you would need ~64 more
// of these declarations and a matching 64 zone files)
zone "64.100.in-addr.arpa" {
    type master;
    file "/etc/bind/zones/64.100.in-addr.arpa.zone";
    allow-update { key "tailscale-bind-ddns-key"; };
};
```

3. **Create Zone File** (`/var/lib/bind/tailscale.example.com.zone`):

This may be in a different location depending on the settings of your Linux package distributor or
the `options` -> `directory` setting in your `/etc/named.conf` file.

```
$TTL 60
@   IN  SOA ns1.example.com. admin.example.com (
    2025091401  ; serial
    300         ; refresh
    60          ; retry
    3600        ; expire
    60          ; minimum
)
    IN  NS  ns1.example.com.
```

Replace all instances of example.com with the primary name of your DNS server.

**Optionally Create Reverse Zone File** (`/var/lib/bind/64.100.in-addr.arpa`):

```
$TTL 300
@ IN SOA ns1.example.com. admin.example.com. (
    2025091401  ; Serial
    300         ; Refresh
    60          ; Retry
    3600        ; Expire
    60          ; Minimum TTL
)

  IN NS ns1.example.com.
```

4. **Restart Bind**:

```bash
systemctl restart named
```

## Development

### Project Structure

```
.
├── .github/                 # GitHub Actions workflows
│   └── workflows/
│       ├── ci.yml          # Continuous Integration
│       └── release.yml     # Automated releases
├── cmd/                     # CLI commands and main entry point
│   └── root.go
├── pkg/                     # Public library code
│   ├── app/                # Main application logic
│   ├── bind/               # Bind DDNS client
│   ├── config/             # Configuration management
│   └── tailscale/          # Tailscale client
├── internal/               # Internal packages (not used yet)
├── test/                   # Test utilities
├── docs/                   # Documentation
├── .goreleaser.yml         # GoReleaser configuration
├── Makefile               # Build automation
├── main.go                # Application entry point
├── config.yaml.example    # Example configuration
└── go.mod                 # Go module definition
```

### Development Setup

```bash
# Clone the repository
git clone https://github.com/aauren/tailscale-bind-ddns.git
cd tailscale-bind-ddns

# Install development tools
make install-tools

# Download dependencies
make deps
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Generate HTML coverage report
make coverage
```

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Clean build artifacts
make clean
```

### Linting

```bash
# Run linter
make lint

# Run linter with auto-fix
make lint-fix
```

### Creating Releases

```bash
# Create a snapshot release (for testing)
make snapshot

# Create a full release (requires git tag)
make release

# Test release locally without publishing
make release-local
```

## Automated Releases

This project uses [GoReleaser](https://goreleaser.com/) for automated releases and [GitHub Actions](https://github.com/features/actions) for CI/CD.

### Release Process

1. **Tag a release**: Create a git tag (e.g., `v1.0.0`)
2. **Push the tag**: `git push origin v1.0.0`
3. **Automated build**: GitHub Actions automatically builds binaries for multiple platforms
4. **Release creation**: GoReleaser creates a GitHub release with:
   - Pre-built binaries for Linux (amd64, arm64), macOS (amd64, arm64), and Windows (amd64)
   - Checksums for verification
   - Release notes with changelog

### Supported Platforms

- **Linux**: amd64, arm64
- **macOS**: amd64, arm64
- **Windows**: amd64

### CI/CD Pipeline

The project includes two GitHub Actions workflows:

- **CI** (`.github/workflows/ci.yml`): Runs on every push and PR
  - Tests on multiple Go versions (1.24, 1.25)
  - Runs linter (golangci-lint)
  - Generates test coverage reports
  - Tests GoReleaser build process

- **Release** (`.github/workflows/release.yml`): Runs on tag pushes
  - Builds binaries for all supported platforms
  - Creates GitHub release with assets
  - Generates checksums for verification

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass (`make ci`)
6. Submit a pull request

### Development Workflow

```bash
# Set up development environment
make install-tools
make deps

# Make your changes...

# Run tests and linting
make ci

# Test the build
make build

# Create a snapshot release to test
make snapshot
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Troubleshooting

### Common Issues

1. **Authentication Errors**: Verify your Tailscale API key or OAuth credentials
2. **DNS Update Failures**: Check TSIG key configuration and Bind server permissions
3. **Connection Issues**: Ensure network connectivity to both Tailscale API and DNS server

### Debug Mode

Enable debug logging for detailed information:

```bash
./tailscale-bind-ddns run --log-level debug
```

### Testing Connections

Test your configuration before running:

```bash
./tailscale-bind-ddns test
```

## Security Considerations

- Store API keys and TSIG secrets securely
- Use environment variables or secure configuration files
- Regularly rotate API keys and TSIG secrets
- Monitor DNS updates for unauthorized changes
- Use appropriate firewall rules for DNS server access
