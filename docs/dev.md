# Development

## Project Structure

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

## Development Setup

```bash
# Clone the repository
git clone https://github.com/aauren/tailscale-bind-ddns.git
cd tailscale-bind-ddns

# Download dependencies
make deps
```

## Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Generate HTML coverage report
make coverage
```

## Building

```bash
# Build for current platform
make tailscale-bind-ddns

# Build for all platforms
make build-all

# Clean build artifacts
make clean
```

## Linting

```bash
# Run linter
make lint

# Run linter with auto-fix
make lint-fix
```
