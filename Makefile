.PHONY: all build clean test lint fmt dev install-tools web web-build package-windows package-linux acceptance-linux install-linux uninstall-linux

# Build variables
BINARY_NAME=duorelay
BUILD_DIR=build
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

all: build

.PHONY: build-lite package-linux-lite package-windows-lite
build-lite: web-build
	@mkdir -p $(BUILD_DIR)
	go build -tags lite $(LDFLAGS) -o $(BUILD_DIR)/duorelay-lite ./cmd/duorelay

package-linux-lite:
	bash scripts/package-linux.sh $(VERSION) $(if $(ARCH),$(ARCH),$$(uname -m)) --lite

package-windows-lite:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows.ps1 -Version $(VERSION) -Lite

# Build the binary
build: web-build
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/duorelay

# Build for multiple platforms
build-all: web-build
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/duorelay
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/duorelay

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf web/dist
	@rm -rf web/node_modules

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Run tests with coverage report
test-coverage: test
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint code
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run 'make install-tools'" && exit 1)
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# Development mode (with auto-reload)
dev:
	@which air > /dev/null || (echo "air not installed. Run 'go install github.com/cosmtrek/air@latest'" && exit 1)
	air

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/cosmtrek/air@latest

# Frontend: Install dependencies
web:
	@echo "Installing frontend dependencies..."
	cd web && npm install

# Frontend: Build for production
web-build:
	@echo "Building frontend..."
	@if [ -d "web" ] && [ -f "web/package.json" ]; then \
		cd web && npm install && npm run build; \
	else \
		echo "Frontend not initialized yet, skipping..."; \
		mkdir -p web/dist; \
	fi

# Package a Windows amd64 release (PowerShell required).
package-windows:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows.ps1 -Version $(VERSION)

# Package a Linux release for the current architecture (or pass ARCH=arm64).
package-linux:
	bash scripts/package-linux.sh $(VERSION) $(if $(ARCH),$(ARCH),$$(uname -m))

acceptance-linux:
	bash scripts/acceptance-linux.sh

install-linux:
	bash scripts/install-linux.sh --package $(PACKAGE)

uninstall-linux:
	bash scripts/uninstall-linux.sh

# Frontend: Development mode
web-dev:
	@echo "Starting frontend dev server..."
	cd web && npm run dev

# Run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Show help
help:
	@echo "DuoRelay Makefile Commands:"
	@echo "  make build         - Build the binary"
	@echo "  make build-all     - Build for multiple platforms"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make lint          - Run linters"
	@echo "  make fmt           - Format code"
	@echo "  make dev           - Run in development mode"
	@echo "  make web           - Install frontend dependencies"
	@echo "  make web-build     - Build frontend"
	@echo "  make web-dev       - Start frontend dev server"
	@echo "  make run           - Build and run the application"
	@echo "  make install-tools - Install development tools"
	@echo "  make package-linux VERSION=0.3.0 [ARCH=amd64|arm64]"
	@echo "  make acceptance-linux - Check a running Linux service"
	@echo "  make install-linux PACKAGE=./dist/..."
	@echo "  make uninstall-linux - Remove the Linux system service"
