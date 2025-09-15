# Makefile for Tailscale-Bind DDNS

# Variables
BINARY_NAME=tailscale-bind-ddns
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD)
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
DOCKER_IMAGE=aauren/tailscale-bind-ddns
GO_VERSION=1.25

# Containerized Go commands
DOCKER_RUN=docker run --rm -v $(PWD):/app -w /app
GOLANG_IMAGE=golang:$(GO_VERSION)-alpine
GOLANGCI_IMAGE=golangci/golangci-lint:latest

.PHONY: all build clean test deps lint docker-build docker-push ci help

# Default target
all: test build

# Build the binary locally
build:
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o $(BINARY_NAME) .

# Build the binary in container
build-container:
	$(DOCKER_RUN) $(GOLANG_IMAGE) sh -c "apk add --no-cache git ca-certificates && go build -ldflags '-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)' -o $(BINARY_NAME) ."

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o $(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o $(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o $(BINARY_NAME)-windows-amd64.exe .

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*

# Run tests locally
test:
	go test -v ./...

# Run tests in container
test-container:
	$(DOCKER_RUN) $(GOLANG_IMAGE) sh -c "apk add --no-cache git ca-certificates && go test -v ./..."

# Run tests with coverage
test-coverage:
	go test -v -cover ./...

# Generate coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Download dependencies
deps:
	go mod download
	go mod tidy

# Download dependencies in container
deps-container:
	$(DOCKER_RUN) $(GOLANG_IMAGE) sh -c "apk add --no-cache git ca-certificates && go mod download && go mod tidy"

# Run linter locally
lint:
	golangci-lint run

# Run linter in container
lint-container:
	$(DOCKER_RUN) $(GOLANGCI_IMAGE) golangci-lint run

# Run linter with fix
lint-fix:
	golangci-lint run --fix

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest \
		--label "org.opencontainers.image.version=$(VERSION)" \
		--label "org.opencontainers.image.created=$(DATE)" \
		--label "org.opencontainers.image.revision=$(COMMIT)" \
		--label "org.opencontainers.image.source=https://github.com/aauren/tailscale-bind-ddns" \
		.

# Push Docker image
docker-push:
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

# Build and test (CI) - containerized
ci: deps-container lint-container test-container

# Build and test (CI) - local
ci-local: deps lint test

# Show help
help:
	@echo "Available targets:"
	@echo "  build           - Build the binary for current platform (local)"
	@echo "  build-container - Build the binary in container"
	@echo "  build-all       - Build binaries for multiple platforms"
	@echo "  clean           - Clean build artifacts"
	@echo "  test            - Run tests (local)"
	@echo "  test-container  - Run tests in container"
	@echo "  test-coverage   - Run tests with coverage"
	@echo "  coverage        - Generate HTML coverage report"
	@echo "  deps            - Download and tidy dependencies (local)"
	@echo "  deps-container  - Download and tidy dependencies in container"
	@echo "  lint            - Run linter (local)"
	@echo "  lint-container  - Run linter in container"
	@echo "  lint-fix        - Run linter with auto-fix"
	@echo "  docker-build    - Build Docker image"
	@echo "  docker-push     - Push Docker image to registry"
	@echo "  ci              - Run CI pipeline (containerized)"
	@echo "  ci-local        - Run CI pipeline (local)"
	@echo "  help            - Show this help message"
