#!/bin/bash

# Build script for process-tracker

set -e

PROJECT_NAME="process-tracker"
VERSION="1.0.0"
BUILD_DIR="build"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building process-tracker...${NC}"

# Create build directory
mkdir -p $BUILD_DIR

# Function to build for specific platform
build() {
    local os=$1
    local arch=$2
    local output="$BUILD_DIR/${PROJECT_NAME}-${os}-${arch}"
    
    if [ "$os" = "windows" ]; then
        output="${output}.exe"
    fi
    
    echo -e "${YELLOW}Building for $os/$arch...${NC}"
    GOOS=$os GOARCH=$arch go build -ldflags="-X main.version=$VERSION" \
        -o "$output" ./cmd/process-tracker
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Built $output${NC}"
    else
        echo -e "${RED}✗ Failed to build for $os/$arch${NC}"
        exit 1
    fi
}

# Build for current platform
build $(go env GOOS) $(go env GOARCH)

# Build for common platforms
build linux amd64
build linux arm64
build darwin amd64
build darwin arm64
build windows amd64

echo -e "${GREEN}Build completed successfully!${NC}"
echo -e "${YELLOW}Binaries are available in the $BUILD_DIR directory${NC}"