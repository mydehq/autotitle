# Build Variables
BINARY_NAME := autotitle
BIN_DIR     := bin
GO          := go
MODULE      := $(shell $(GO) list -m)


# Version Information
VERSION := $(shell git describe --tags --always --dirty)
COMMIT  := $(shell git rev-parse --short HEAD)
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")


# Linker Flags
LDFLAGS_COMMON := -X '$(MODULE)/internal/version.Version=$(VERSION)' \
                  -X '$(MODULE)/internal/version.Commit=$(COMMIT)' \
                  -X '$(MODULE)/internal/version.Date=$(DATE)'

LDFLAGS_DEV     := $(LDFLAGS_COMMON)
LDFLAGS_RELEASE := -s -w $(LDFLAGS_COMMON)


.PHONY: all build release install clean test fmt lint help

# Default target
all: lint test build


# Build development binary (with debug symbols)
build:
	@echo "Building (dev)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS_DEV)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/autotitle


# Build release binary (stripped, optimized)
release:
	@echo "Building (release)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS_RELEASE)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/autotitle


# Build for all common platforms
release-all:
	@echo "Building cross-platform releases..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux   GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS_RELEASE)" -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64       ./cmd/autotitle
	GOOS=linux   GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS_RELEASE)" -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64       ./cmd/autotitle
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS_RELEASE)" -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/autotitle
	GOOS=windows GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS_RELEASE)" -o $(BIN_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/autotitle
	GOOS=darwin  GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS_RELEASE)" -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64      ./cmd/autotitle
	GOOS=darwin  GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS_RELEASE)" -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64      ./cmd/autotitle


# Install optimized binary to GOPATH/bin
install:
	@echo "Installing..."
	$(GO) install -ldflags "$(LDFLAGS_RELEASE)" ./cmd/autotitle


# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	$(GO) clean
	rm -f autotitle # legacy
	rm -f test/*.mkv
	rm -rf test/_backup

# Run tests
test:
	$(GO) test ./...

# Format code
fmt:
	$(GO) fmt ./...

# Lint code (requires golangci-lint)
lint:
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi


# Show help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build       Build development binary"
	@echo "  release     Build optimized production binary"
	@echo "  release-all Build optimized binaries for Linux, Windows (amd64/arm64), macOS"
	@echo "  install     Install optimized binary to GOPATH/bin"
	@echo "  test        Run tests"
	@echo "  clean       Remove build artifacts"
	@echo "  fmt         Format code"
	@echo "  lint        Run linter"
