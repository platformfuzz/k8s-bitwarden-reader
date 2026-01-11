.PHONY: all all-fast build test docker-build docker-run dev-container run clean help fmt lint deps

.DEFAULT_GOAL := help

# Variables
BINARY_NAME=bitwarden-reader
DOCKER_IMAGE=bitwarden-reader
DOCKER_TAG=latest
VERSION?=1.0.0

# Run full workflow: deps, test, build, and run
all: deps test build
	@echo ""
	@echo "=========================================="
	@echo "Starting application..."
	@echo "Web UI: http://localhost:8080"
	@echo "API Health: http://localhost:8080/api/v1/health"
	@echo "API Secrets: http://localhost:8080/api/v1/secrets"
	@echo "Press Ctrl+C to stop"
	@echo "=========================================="
	@echo ""
	@go run ./cmd/server

# Run workflow without tests (faster iteration)
all-fast: deps build
	@echo ""
	@echo "=========================================="
	@echo "Starting application (tests skipped)..."
	@echo "Web UI: http://localhost:8080"
	@echo "API Health: http://localhost:8080/api/v1/health"
	@echo "API Secrets: http://localhost:8080/api/v1/secrets"
	@echo "Press Ctrl+C to stop"
	@echo "=========================================="
	@echo ""
	@go run ./cmd/server

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	@go build -o bin/$(BINARY_NAME) ./cmd/server
	@echo "Build complete: bin/$(BINARY_NAME)"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -timeout 60s ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -timeout 60s -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 --rm $(DOCKER_IMAGE):$(DOCKER_TAG)

# Run full workflow inside a development container
dev-container:
	@echo "Running full workflow in development container..."
	@docker run --rm -it \
		-v $(PWD):/workspace \
		-w /workspace \
		-p 8080:8080 \
		-e PORT=8080 \
		golang:1.23-alpine \
		sh -c "apk add --no-cache make && make all"

# Run the application locally
run:
	@echo "Running application..."
	@go run ./cmd/server

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run ./... || echo "golangci-lint not installed, skipping..."

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Show help
help:
	@echo "Available targets:"
	@echo "  all            - Full workflow: deps, test, build, and run"
	@echo "  all-fast       - Fast workflow: deps, build, and run (skip tests)"
	@echo "  build          - Build the application"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  dev-container  - Run full workflow in dev container (no local Go needed)"
	@echo "  run            - Run the application locally"
	@echo "  clean          - Clean build artifacts"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  deps           - Download dependencies"
	@echo "  help           - Show this help message"

