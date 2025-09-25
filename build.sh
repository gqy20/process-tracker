#!/bin/bash

# Cross-platform build script for process-tracker
# Following Dave Cheney's principles: simple and explicit

set -e

PROJECT_NAME="process-tracker"
VERSION="0.2.1"
RELEASE_DIR="releases/v${VERSION}"

echo "🔨 Building process-tracker for multiple platforms..."
echo "📁 Output directory: ${RELEASE_DIR}"

# Create release directory
mkdir -p "${RELEASE_DIR}"

# Build for current platform first
echo "📦 Building for $(go env GOOS)/$(go env GOARCH)..."
go build -ldflags="-X main.Version=${VERSION}" -o "${RELEASE_DIR}/${PROJECT_NAME}" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}"

# Build for Windows
echo "📦 Building for Windows/amd64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-X main.Version=${VERSION}" -o "${RELEASE_DIR}/${PROJECT_NAME}.exe" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}.exe"

# Build for macOS Intel
echo "📦 Building for macOS/amd64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.Version=${VERSION}" -o "${RELEASE_DIR}/${PROJECT_NAME}-macos" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}-macos"

# Build for macOS ARM
echo "📦 Building for macOS/arm64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-X main.Version=${VERSION}" -o "${RELEASE_DIR}/${PROJECT_NAME}-macos-arm64" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}-macos-arm64"

# Build for Linux ARM
echo "📦 Building for Linux/arm64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-X main.Version=${VERSION}" -o "${RELEASE_DIR}/${PROJECT_NAME}-linux-arm64" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}-linux-arm64"

echo ""
echo "🎉 All builds completed successfully!"
echo ""
echo "📋 Generated files:"
ls -la "${RELEASE_DIR}/" 2>/dev/null || true
echo ""
echo "💾 File sizes:"
for file in "${RELEASE_DIR}/"${PROJECT_NAME}*; do
    if [ -f "$file" ]; then
        size=$(du -h "$file" | cut -f1)
        echo "   $(basename "$file"): $size"
    fi
done
echo ""
echo "🚀 Ready to distribute from ${RELEASE_DIR}/"