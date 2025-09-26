#!/bin/bash

# Test script for GitHub Actions release workflow
# This script simulates the GitHub Actions workflow locally

set -e

echo "🧪 Testing GitHub Actions Release Workflow"
echo "========================================="

# Step 1: Version Check
echo "📋 Step 1: Version Check"
VERSION=$(grep -o 'var Version = "[^"]*"' main.go | sed 's/var Version = "\(.*\)"/\1/')
echo "Current version: $VERSION"

# Check if tag exists
if git tag -l "v$VERSION" | grep -q "v$VERSION"; then
  echo "⚠️  Version v$VERSION already exists"
  IS_NEW_VERSION="false"
else
  echo "✅ New version v$VERSION detected"
  IS_NEW_VERSION="true"
fi

# Step 2: Simulate Build (only for current platform)
echo ""
echo "📋 Step 2: Build Simulation"
echo "Building for current platform..."

go build -ldflags="-X main.Version=$VERSION" -o process-tracker-test .
echo "✅ Built: process-tracker-test"

# Verify build
./process-tracker-test version
echo "✅ Build verified"

# Step 3: Prepare Release Files
echo ""
echo "📋 Step 3: Release Preparation"
mkdir -p test-release-files

# Copy and rename files
cp process-tracker-test test-release-files/process-tracker-linux-amd64
echo "✅ Release files prepared"

# Show file info
echo "📦 Release file:"
ls -la test-release-files/process-tracker-linux-amd64

# Step 4: Generate Release Notes from Commits
echo ""
echo "📋 Step 4: Release Notes from Commits"

# 获取从上一个版本到现在的所有 commit
if git tag -l | grep -q "v"; then
  # 找到上一个版本标签
  PREV_TAG=$(git tag -l "v*" | sort -V | tail -n 2 | head -n 1)
  if [ -n "$PREV_TAG" ]; then
    COMMITS=$(git log --pretty=format:"%h %s" $PREV_TAG..HEAD)
    echo "Commits since $PREV_TAG:"
  else
    COMMITS=$(git log --pretty=format:"%h %s" --reverse | head -n 10)
    echo "Recent commits (first 10):"
  fi
else
  COMMITS=$(git log --pretty=format:"%h %s" --reverse | head -n 10)
  echo "Recent commits (first 10):"
fi

echo "$COMMITS"

# 生成发布说明
cat > test-release-notes.md << EOF
## Process Tracker v$VERSION

🚀 **智能进程监控工具** - 用于跟踪和分析系统进程的资源使用情况

### 📦 下载

选择适合您平台的版本：

- **process-tracker-linux-amd64** - Linux Intel/AMD 64位
- **process-tracker-linux-arm64** - Linux ARM 64位
- **process-tracker-macos-amd64** - macOS Intel 64位
- **process-tracker-macos-arm64** - macOS ARM64 (Apple Silicon)
- **process-tracker-windows-amd64.exe** - Windows Intel/AMD 64位

### 🚀 快速开始

\`\`\`bash
# 赋予执行权限
chmod +x process-tracker-*

# 开始监控
./process-tracker-linux-amd64 start

# 查看版本
./process-tracker-linux-amd64 version

# 查看帮助
./process-tracker-linux-amd64 help
\`\`\`

### 📋 本次更新内容

$COMMITS

### ✨ 主要特性

- 🔍 **实时监控**: 监控CPU、内存、磁盘I/O、网络使用情况
- 📊 **智能统计**: 支持简单、详细、完整三种统计粒度
- 🗂️ **智能分类**: 自动识别应用程序类型
- 💾 **存储优化**: 自动文件轮转和压缩
- 🎛️ **灵活配置**: YAML配置文件支持
- 📤 **数据导出**: JSON格式数据导出和分析

### 📄 完整文档

详细使用说明请参考：[README.md](https://github.com/yourusername/process-tracker/blob/main/README.md)

---

🤖 *此发布由 GitHub Actions 自动生成*
EOF

# Step 5: Summary
echo ""
echo "📋 Step 5: Test Summary"
echo "========================================="
echo "✅ Version Check: v$VERSION"
echo "✅ New Version: $IS_NEW_VERSION"
echo "✅ Build Test: PASSED"
echo "✅ Release Files: Prepared"
echo "✅ Release Notes: Generated"

if [ "$IS_NEW_VERSION" = "true" ]; then
  echo ""
  echo "🎉 Ready for GitHub Actions Release!"
  echo "Next commit will trigger automatic release creation."
else
  echo ""
  echo "ℹ️  Version v$VERSION already released."
  echo "Update version in main.go to trigger new release."
fi

# Cleanup
rm -f process-tracker-test
rm -rf test-release-files

echo ""
echo "🧪 Test completed successfully!"