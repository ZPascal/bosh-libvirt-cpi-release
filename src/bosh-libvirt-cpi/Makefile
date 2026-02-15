.PHONY: all build test test-unit test-integration clean lint fmt check deps vendor help

# Build variables
BINARY_NAME=cpi
BUILD_DIR=../../bin
SRC_DIR=./main
GO=go

all: clean fmt lint test build

build:
	@echo "Building $(BINARY_NAME)..."
	@$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(SRC_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

test: test-unit

test-unit:
	@echo "Running unit tests..."
	@$(GO) test -short -v ./...

test-integration:
	@echo "Running integration tests..."
	@$(GO) test -tags=integration -v ./test/integration/...

test-coverage:
	@echo "Running tests with coverage..."
	@$(GO) test -short -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	@echo "Cleaning..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@$(GO) clean -testcache

lint:
	@echo "Running linter..."
	@$(GO) vet ./...
	@gofmt -l . | grep -v vendor || true

fmt:
	@echo "Formatting code..."
	@gofmt -w .

check: fmt lint test
	@echo "All checks passed!"

deps:
	@echo "Installing dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy

vendor:
	@echo "Vendoring dependencies..."
	@$(GO) mod vendor

help:
	@echo "Available targets:"
	@echo "  build            - Build the CPI binary"
	@echo "  test             - Run all unit tests"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests"
	@echo "  test-coverage    - Generate test coverage report"
	@echo "  clean            - Clean build artifacts"
	@echo "  lint             - Run code linters"
	@echo "  fmt              - Format code"
	@echo "  check            - Run fmt, lint, and test"
	@echo "  deps             - Install dependencies"
	@echo "  vendor           - Vendor dependencies"
	@echo "  all              - Clean, format, lint, test, and build"
