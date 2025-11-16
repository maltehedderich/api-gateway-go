.PHONY: build test clean run dev deps lint fmt docker-build

# Binary name
BINARY_NAME=gateway
BINARY_PATH=bin/$(BINARY_NAME)

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Linker flags
LDFLAGS=-ldflags="-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	go build $(LDFLAGS) -o $(BINARY_PATH) ./cmd/gateway
	@echo "Build complete: $(BINARY_PATH)"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Run the application in development mode
dev:
	@echo "Running in development mode..."
	go run ./cmd/gateway -config configs/config.dev.yaml

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_PATH) -config configs/config.dev.yaml

# Install/update dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated"

# Lint the code
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/" && exit 1)
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t api-gateway:$(VERSION) .
	docker tag api-gateway:$(VERSION) api-gateway:latest
	@echo "Docker image built: api-gateway:$(VERSION)"

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 -p 9090:9090 \
		-v $(PWD)/configs:/etc/gateway/configs \
		api-gateway:latest -config /etc/gateway/configs/config.dev.yaml

# Display help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application binary"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Remove build artifacts"
	@echo "  dev            - Run in development mode"
	@echo "  run            - Build and run the application"
	@echo "  deps           - Download and update dependencies"
	@echo "  lint           - Run code linters"
	@echo "  fmt            - Format code"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  help           - Display this help message"

# Default target
.DEFAULT_GOAL := build
