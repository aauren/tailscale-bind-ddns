# Tailscale-Bind DDNS

A Go application that automatically syncs Tailscale machines to Bind DNS records using RFC 2136 dynamic updates with TSIG authentication.

## Why???

### Subnet Router Usecase

While best practice states that every endpoint in your Tailnet should be connected directly via the Tailscale client, not every device can
be connected that way. Some people may also just want to use the [Subnet Router](https://tailscale.com/kb/1019/subnets) configuration for
other reasons.

In the event that you do this, you will no longer get the benfit of the Tailscale client automagically resolving your devices via
Tailscale DNS.

If the network where you run the subnet router has an RFC 2136 compliant DNS server on it though, you can use this application to
resolve the names of the tailnet devices to their tailnet IPs the same as if you were running the client directly.

### Reverse DNS Usecase

Sometimes when logging information all you have is an IP address. However, via reverse DNS it is possible to look up an endpoint's name
from its IP address. This gives you a much more helpful understanding of the devices on your network than an IP address would.

If you run this application with [PTR](./docs/ptr.md) (Reverse DNS) enabled you'll get exactly such functionality.

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
- A tailscale-bind-ddns configuration file (see [config.yaml.example](./config.yaml.example) and the [Configuration](./docs/config.md)
  docs for more details)

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

#### Docker Compose Example (Recommended)

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

## Usage

While you can use environment variables or CLI parameters to configure tailscale-bind-ddns, the project recommends that you use a
configuration file as most users will find it simpler.

For more details see:

* [config.yaml.example](./config.yaml.example) - An example configuration file with all options in it
* [Configuration Documentation](./docs/config.md) - Docs with in-depth information about configuration specification

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

## License

This project is licensed under the Apache 2.0 - see the LICENSE file for details.

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
