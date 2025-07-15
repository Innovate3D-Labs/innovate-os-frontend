# Makefile for Innovate OS Frontend

# Build settings
BINARY_NAME=innovate-os-frontend
SOURCE_DIR=.
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty)

# Cross-compilation settings for ARM Linux (Raspberry Pi / embedded systems)
GOOS_ARM=linux
GOARCH_ARM=arm64
GOARM=7

# Development build (native platform)
build:
	@echo "Building for native platform..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(SOURCE_DIR)

# Production build for ARM Linux (embedded systems)
build-arm:
	@echo "Building for ARM Linux (embedded systems)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS_ARM) GOARCH=$(GOARCH_ARM) go build -ldflags="-w -s" -o $(BUILD_DIR)/$(BINARY_NAME)-arm $(SOURCE_DIR)

# Build for x86_64 Linux (development/testing)
build-linux:
	@echo "Building for x86_64 Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o $(BUILD_DIR)/$(BINARY_NAME)-linux $(SOURCE_DIR)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)

# Run development version
run:
	@echo "Running development version..."
	go run $(SOURCE_DIR)

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Build all platforms
build-all: build build-arm build-linux

# Install on target system (ARM Linux)
install-arm: build-arm
	@echo "Installing on ARM Linux target..."
	@if [ -z "$(TARGET_HOST)" ]; then \
		echo "Error: TARGET_HOST not set. Usage: make install-arm TARGET_HOST=user@hostname"; \
		exit 1; \
	fi
	scp $(BUILD_DIR)/$(BINARY_NAME)-arm $(TARGET_HOST):/tmp/$(BINARY_NAME)
	ssh $(TARGET_HOST) "sudo mv /tmp/$(BINARY_NAME) /opt/innovate-os/$(BINARY_NAME)"
	ssh $(TARGET_HOST) "sudo chmod +x /opt/innovate-os/$(BINARY_NAME)"
	ssh $(TARGET_HOST) "sudo systemctl restart innovate-os-frontend"

# Create systemd service file
create-service:
	@echo "Creating systemd service file..."
	@mkdir -p $(BUILD_DIR)
	@cat > $(BUILD_DIR)/innovate-os-frontend.service << 'EOF'
[Unit]
Description=Innovate OS Frontend
After=network.target innovate-os-backend.service
Wants=innovate-os-backend.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/innovate-os
ExecStart=/opt/innovate-os/innovate-os-frontend
Restart=always
RestartSec=10
Environment=DISPLAY=:0

[Install]
WantedBy=multi-user.target
EOF
	@echo "Service file created at $(BUILD_DIR)/innovate-os-frontend.service"

# Development setup
dev-setup: deps fmt vet
	@echo "Development setup complete"

# Production deployment package
package: build-arm create-service
	@echo "Creating deployment package..."
	@mkdir -p $(BUILD_DIR)/deploy
	@cp $(BUILD_DIR)/$(BINARY_NAME)-arm $(BUILD_DIR)/deploy/$(BINARY_NAME)
	@cp $(BUILD_DIR)/innovate-os-frontend.service $(BUILD_DIR)/deploy/
	@cat > $(BUILD_DIR)/deploy/install.sh << 'EOF'
#!/bin/bash
# Innovate OS Frontend Installation Script

set -e

echo "Installing Innovate OS Frontend..."

# Create directory
sudo mkdir -p /opt/innovate-os

# Copy binary
sudo cp innovate-os-frontend /opt/innovate-os/
sudo chmod +x /opt/innovate-os/innovate-os-frontend

# Copy service file
sudo cp innovate-os-frontend.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable and start service
sudo systemctl enable innovate-os-frontend
sudo systemctl start innovate-os-frontend

# Check status
sudo systemctl status innovate-os-frontend

echo "Installation complete!"
echo "Frontend is running on the touchscreen display"
EOF
	@chmod +x $(BUILD_DIR)/deploy/install.sh
	@echo "Deployment package created in $(BUILD_DIR)/deploy/"

# Build discovery demo
build-discovery-demo:
	@echo "$(GREEN)Building printer discovery demo...$(NC)"
	go build -o bin/discovery-demo discovery_demo.go printer_discovery.go backend_client.go theme.go

# Run discovery demo
run-discovery-demo: build-discovery-demo
	@echo "$(GREEN)Running printer discovery demo...$(NC)"
	./bin/discovery-demo

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build for native platform"
	@echo "  build-arm      - Build for ARM Linux (embedded systems)"
	@echo "  build-linux    - Build for x86_64 Linux"
	@echo "  build-all      - Build for all platforms"
	@echo "  deps           - Install dependencies"
	@echo "  clean          - Clean build artifacts"
	@echo "  run            - Run development version"
	@echo "  test           - Run tests"
	@echo "  fmt            - Format code"
	@echo "  vet            - Vet code"
	@echo "  install-arm    - Install on ARM Linux target (requires TARGET_HOST)"
	@echo "  create-service - Create systemd service file"
	@echo "  package        - Create deployment package"
	@echo "  dev-setup      - Setup development environment"
	@echo "  help           - Show this help"

.PHONY: build build-arm build-linux build-all deps clean run test fmt vet install-arm create-service package dev-setup help 