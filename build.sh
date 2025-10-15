#!/bin/bash

# Cross-platform build script for process-tracker
# Following Dave Cheney's principles: simple and explicit

set -e

PROJECT_NAME="process-tracker"
VERSION="0.4.0"
RELEASE_DIR="releases/v${VERSION}"

# Build flags for optimization
BUILD_FLAGS="-ldflags=\"-s -w -X main.Version=${VERSION}\" -trimpath"

# Static compilation flags (no CGO for maximum portability)
export CGO_ENABLED=0

echo "🔨 Building process-tracker for multiple platforms (static)..."
echo "📁 Output directory: ${RELEASE_DIR}"
echo "🔧 Static compilation enabled (CGO_ENABLED=0)"

# Create release directory
mkdir -p "${RELEASE_DIR}"

# Build for current platform first
echo "📦 Building for $(go env GOOS)/$(go env GOARCH)..."
eval go build $BUILD_FLAGS -o "${RELEASE_DIR}/${PROJECT_NAME}" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}"

# Also build in main directory for easy access
echo "📦 Building current version for main directory..."
eval go build $BUILD_FLAGS -o "${PROJECT_NAME}" .
echo "✅ Built: ./${PROJECT_NAME}"

# Build for Windows
echo "📦 Building for Windows/amd64..."
GOOS=windows GOARCH=amd64 eval go build $BUILD_FLAGS -o "${RELEASE_DIR}/${PROJECT_NAME}.exe" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}.exe"

# Build for macOS Intel
echo "📦 Building for macOS/amd64..."
GOOS=darwin GOARCH=amd64 eval go build $BUILD_FLAGS -o "${RELEASE_DIR}/${PROJECT_NAME}-macos" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}-macos"

# Build for macOS ARM
echo "📦 Building for macOS/arm64..."
GOOS=darwin GOARCH=arm64 eval go build $BUILD_FLAGS -o "${RELEASE_DIR}/${PROJECT_NAME}-macos-arm64" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}-macos-arm64"

# Build for Linux ARM
echo "📦 Building for Linux/arm64..."
GOOS=linux GOARCH=arm64 eval go build $BUILD_FLAGS -o "${RELEASE_DIR}/${PROJECT_NAME}-linux-arm64" .
echo "✅ Built: ${RELEASE_DIR}/${PROJECT_NAME}-linux-arm64"

# Compress builds with UPX
echo "🗜️  Compressing builds with UPX..."
for file in "${RELEASE_DIR}/"${PROJECT_NAME}*; do
    if [ -f "$file" ] && [ -x "$file" ]; then
        echo "   Compressing $(basename "$file")..."
        upx --best --quiet "$file"
    fi
done

# Also compress the main build
if [ -f "${PROJECT_NAME}" ] && [ -x "${PROJECT_NAME}" ]; then
    echo "   Compressing ${PROJECT_NAME}..."
    upx --best --quiet "${PROJECT_NAME}"
fi

echo ""
echo "🎉 All builds completed successfully!"
echo ""
echo "📋 Generated files:"
ls -la "${RELEASE_DIR}/" 2>/dev/null || true
echo ""
echo "💾 File sizes (after compression):"
for file in "${RELEASE_DIR}/"${PROJECT_NAME}*; do
    if [ -f "$file" ]; then
        size=$(du -h "$file" | cut -f1)
        echo "   $(basename "$file"): $size"
    fi
done
if [ -f "${PROJECT_NAME}" ]; then
    size=$(du -h "${PROJECT_NAME}" | cut -f1)
    echo "   ${PROJECT_NAME}: $size"
fi
echo ""
echo "🚀 Ready to distribute from ${RELEASE_DIR}/"
