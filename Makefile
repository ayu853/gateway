.PHONY: build run test clean backends bench fmt lint help

# Binary name
BINARY=gateway
BUILD_DIR=bin

# Go parameters
GOCMD=/opt/homebrew/bin/go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build flags
LDFLAGS=-ldflags "-s -w"

## help: Show this help message
help:
	@echo ""
	@echo "╔══════════════════════════════════════════════════════╗"
	@echo "║        HP Gateway — Makefile Commands                ║"
	@echo "╠══════════════════════════════════════════════════════╣"
	@echo "║  make build      Build the gateway binary            ║"
	@echo "║  make run        Build and run the gateway           ║"
	@echo "║  make test       Run all tests                       ║"
	@echo "║  make backends   Start test backend servers          ║"
	@echo "║  make bench      Run benchmarks                      ║"
	@echo "║  make fmt        Format all Go files                 ║"
	@echo "║  make lint       Run go vet                          ║"
	@echo "║  make clean      Remove build artifacts              ║"
	@echo "║  make deps       Download dependencies               ║"
	@echo "║  make all        Format, lint, test, build           ║"
	@echo "╚══════════════════════════════════════════════════════╝"
	@echo ""

## build: Compile the gateway binary
build:
	@echo "🔨 Building gateway..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/gateway/
	@echo "✅ Built: $(BUILD_DIR)/$(BINARY)"

## run: Build and run the gateway
run: build
	@echo "🚀 Starting gateway..."
	./$(BUILD_DIR)/$(BINARY) -config config/gateway.yaml

## test: Run all tests
test:
	@echo "🧪 Running tests..."
	$(GOTEST) -v -race -cover ./...

## backends: Start test backend servers
backends:
	@echo "🖥️  Starting test backends..."
	$(GOCMD) run scripts/test_backends.go

## bench: Run benchmarks (requires 'hey' tool)
bench:
	@echo "📊 Running benchmarks..."
	@if command -v hey > /dev/null; then \
		echo "Testing: 1000 requests, 50 concurrent..."; \
		hey -n 1000 -c 50 http://localhost:8080/; \
	else \
		echo "⚠️  'hey' not installed. Install with: go install github.com/rakyll/hey@latest"; \
	fi

## fmt: Format Go source files
fmt:
	@echo "📝 Formatting code..."
	$(GOFMT) ./...

## lint: Run go vet
lint:
	@echo "🔍 Running linter..."
	$(GOVET) ./...

## deps: Download dependencies
deps:
	@echo "📦 Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## clean: Remove build artifacts
clean:
	@echo "🧹 Cleaning..."
	@rm -rf $(BUILD_DIR)
	@echo "✅ Clean!"

## all: Format, lint, test, and build
all: fmt lint test build
	@echo ""
	@echo "✅ All checks passed. Binary ready at $(BUILD_DIR)/$(BINARY)"
