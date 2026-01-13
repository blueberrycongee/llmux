.PHONY: build run test lint clean fmt vet coverage

# Build variables
BINARY_NAME=llmux
BUILD_DIR=bin
VERSION?=0.1.0
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=gofmt
GOIMPORTS?=$(shell command -v goimports 2>/dev/null)
ifeq ($(GOIMPORTS),)
GOIMPORTS=$(shell go env GOPATH)/bin/goimports
endif
GOLANGCI_LINT?=$(shell command -v golangci-lint 2>/dev/null)
ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT=$(shell go env GOPATH)/bin/golangci-lint
endif
GOMOD=$(GOCMD) mod

# Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

# Run the server
run: build
	./$(BUILD_DIR)/$(BINARY_NAME) --config config/config.yaml

# Run tests
test:
	$(GOTEST) -v -race ./...

# Run tests with coverage
coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter
lint:
	$(GOLANGCI_LINT) run ./...

# Format code
fmt:
	$(GOFMT) -s -w .
	$(GOIMPORTS) -local github.com/blueberrycongee/llmux -w .

# Run go vet
vet:
	$(GOVET) ./...

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	$(GOMOD) download

# Run all checks before commit
check: fmt vet lint test

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/server
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/server
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/server

# Docker build
docker-build:
	docker build -t llmux:$(VERSION) .

# Help
help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  run        - Build and run the server"
	@echo "  test       - Run tests"
	@echo "  coverage   - Run tests with coverage report"
	@echo "  lint       - Run golangci-lint"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  tidy       - Tidy dependencies"
	@echo "  clean      - Clean build artifacts"
	@echo "  deps       - Download dependencies"
	@echo "  check      - Run all checks (fmt, vet, lint, test)"
	@echo "  build-all  - Build for all platforms"
	@echo "  docker-build - Build Docker image"
