# Makefile for Tailscale-Bind DDNS

# Variables
BINARY_NAME=tailscale-bind-ddns
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD)
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build clean test deps lint install-tools release snapshot

# Default target
all: test build

# Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -cover ./...

# Generate coverage report
coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Install development tools
install-tools:
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/goreleaser/goreleaser@latest

# Run linter
lint:
	golangci-lint run

# Run linter with fix
lint-fix:
	golangci-lint run --fix

# Build and test (CI)
ci: deps lint test

# Create a release using goreleaser
release:
	goreleaser release

# Create a snapshot release using goreleaser
snapshot:
	goreleaser release --snapshot

# Run goreleaser locally (without publishing)
release-local:
	goreleaser release --snapshot --skip-publish

# Install the binary to GOPATH/bin
install:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .
	mv $(BINARY_NAME) $(GOPATH)/bin/

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary for current platform"
	@echo "  build-all     - Build binaries for multiple platforms"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  coverage      - Generate HTML coverage report"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  install-tools - Install development tools (golangci-lint, goreleaser)"
	@echo "  lint          - Run linter"
	@echo "  lint-fix      - Run linter with auto-fix"
	@echo "  ci            - Run CI pipeline (deps, lint, test)"
	@echo "  release       - Create a release using goreleaser"
	@echo "  snapshot      - Create a snapshot release using goreleaser"
	@echo "  release-local - Run goreleaser locally without publishing"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  help          - Show this help message"
