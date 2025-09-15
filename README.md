# Tailscale-Bind DDNS

A Go application that automatically syncs Tailscale machines to Bind DNS records using RFC 2136 dynamic updates with TSIG authentication.

## Features

- **Tailscale Integration**: Connects to Tailscale using OAuth or API key authentication
- **Dynamic DNS Updates**: Updates Bind DNS server using RFC 2136 with TSIG security
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

- Tailscale account with API access
- Bind DNS server with TSIG key configured

### Pre-built Releases (Recommended)

Download the latest release from [GitHub Releases](https://github.com/aauren/tailscale-bind-ddns/releases):

```bash
# Download and extract the latest release
wget https://github.com/aauren/tailscale-bind-ddns/releases/latest/download/tailscale-bind-ddns_*_linux_amd64.tar.gz
tar -xzf tailscale-bind-ddns_*_linux_amd64.tar.gz
sudo mv tailscale-bind-ddns /usr/local/bin/
```

### Build from Source

#### Using Make (Recommended)

```bash
git clone https://github.com/aauren/tailscale-bind-ddns.git
cd tailscale-bind-ddns

# Install development tools
make install-tools

# Build for current platform
make build

# Or build for all platforms
make build-all
```

#### Using Go directly

```bash
git clone https://github.com/aauren/tailscale-bind-ddns.git
cd tailscale-bind-ddns
go build -o tailscale-bind-ddns .
```

#### Using GoReleaser

```bash
# Install goreleaser
go install github.com/goreleaser/goreleaser@latest

# Build snapshot release
goreleaser release --snapshot --rm-dist

# Or build full release (requires git tag)
goreleaser release --rm-dist
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

general:
  log_level: "info"
  dry_run: false
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
