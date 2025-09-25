#!/bin/bash

# Installation script for process-tracker

set -e

PROJECT_NAME="process-tracker"
CONFIG_DIR="$HOME/.config/process-tracker"
SERVICE_FILE="/etc/systemd/system/process-tracker.service"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Installing process-tracker...${NC}"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Please install Go 1.21 or later"
    exit 1
fi

# Create config directory
mkdir -p "$CONFIG_DIR"

# Build the binary
echo -e "${YELLOW}Building binary...${NC}"
go build -o "$PROJECT_NAME" ./cmd/process-tracker

# Install binary
echo -e "${YELLOW}Installing binary...${NC}"
sudo cp "$PROJECT_NAME" /usr/local/bin/
sudo chmod +x /usr/local/bin/"$PROJECT_NAME"

# Install configuration file
echo -e "${YELLOW}Installing configuration...${NC}"
cp configs/default.yaml "$CONFIG_DIR/config.yaml"

# Install systemd service (Linux only)
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo -e "${YELLOW}Installing systemd service...${NC}"
    sudo cp scripts/process-tracker.service "$SERVICE_FILE"
    sudo systemctl daemon-reload
    sudo systemctl enable process-tracker
    
    echo -e "${GREEN}Installation completed!${NC}"
    echo -e "${YELLOW}To start the service, run:${NC}"
    echo "sudo systemctl start process-tracker"
    echo ""
    echo -e "${YELLOW}To check status, run:${NC}"
    echo "sudo systemctl status process-tracker"
    echo ""
    echo -e "${YELLOW}To view logs, run:${NC}"
    echo "journalctl -u process-tracker -f"
else
    echo -e "${GREEN}Installation completed!${NC}"
    echo -e "${YELLOW}You can now run:${NC}"
    echo "$PROJECT_NAME start"
fi

# Clean up
rm -f "$PROJECT_NAME"

echo ""
echo -e "${GREEN}Process tracker has been installed successfully!${NC}"