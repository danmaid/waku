.PHONY: help build clean test install run lint fmt

# Default Go settings
GO := go
GOFLAGS := -v
BINARY_NAME := dynamic-proxy
BIN_DIR := bin
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

help:
	@echo "Dynamic Proxy - Makefile commands"
	@echo ""
	@echo "Usage:"
	@echo "  make build          - Build binaries"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make test           - Run tests"
	@echo "  make run            - Run the application"
	@echo "  make install        - Install development dependencies"
	@echo "  make lint           - Run linter"
	@echo "  make fmt            - Format code"
	@echo ""

build: clean
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) main.go
	@echo "✓ Build complete: $(BIN_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BIN_DIR)/$(BINARY_NAME)
	@rm -f $(BINARY_NAME)
	@$(GO) clean -cache -testcache
	@echo "✓ Clean complete"

test:
	@echo "Running tests..."
	$(GO) test -v ./...

run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BIN_DIR)/$(BINARY_NAME) -port 6002 -dns-port 53 -dns 8.8.8.8:53

install:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
