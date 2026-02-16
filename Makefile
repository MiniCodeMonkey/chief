# Chief - Autonomous PRD Agent
# https://github.com/minicodemonkey/chief

BINARY_NAME := bin/chief
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BIN_DIR := ./bin
BUILD_DIR := ./build
MAIN_PKG := ./cmd/chief

# Go build flags
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

.PHONY: all build install test lint clean release snapshot help

all: build

## build: Build the binary
build:
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PKG)

## install: Install to $GOPATH/bin
install:
	go install $(LDFLAGS) $(MAIN_PKG)

## test: Run all tests
test:
	go test -v ./...

## test-short: Run tests without verbose output
test-short:
	go test ./...

## lint: Run linters (requires golangci-lint)
lint:
	golangci-lint run ./...

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format code
fmt:
	go fmt ./...

## tidy: Tidy and verify dependencies
tidy:
	go mod tidy
	go mod verify

## clean: Remove build artifacts
clean:
	rm -rf $(BIN_DIR)
	rm -rf $(BUILD_DIR)
	rm -rf dist/

## snapshot: Build snapshot release with goreleaser
snapshot:
	goreleaser release --snapshot --clean

## release: Build release (requires GITHUB_TOKEN)
release:
	goreleaser release --clean

## run: Build and run the TUI
run: build
	$(BIN_DIR)/$(BINARY_NAME)

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
