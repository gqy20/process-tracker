# Process Tracker

一个智能的进程监控工具，用于跟踪和分析系统进程的资源使用情况。

## ✨ 主要特性

- 🔍 **实时监控**: 监控CPU、内存、磁盘I/O、网络使用情况
- 📊 **智能统计**: 支持简单、详细、完整三种统计粒度
- 🗂️ **智能分类**: 自动识别应用程序类型（Java、Node.js、Python等）
- 💾 **存储优化**: 自动文件轮转和压缩，节省90%存储空间
- 📁 **多平台支持**: Linux、macOS、Windows
- 🎛️ **灵活配置**: YAML配置文件支持
- 📤 **数据导出**: JSON格式数据导出和分析

## 🚀 快速开始

### 安装

```bash
# 克隆仓库
git clone <repository-url>
cd process-tracker

# 构建（会自动在主目录生成可执行文件）
./build.sh
```

### 基本使用

```bash
# 开始监控
./process-tracker start

# 查看今日统计
./process-tracker today

# 查看本周统计  
./process-tracker week

# 查看详细统计
./process-tracker details

# 导出数据
./process-tracker export

# 清理旧数据
./process-tracker cleanup

# 查看版本
./process-tracker version
```

## ⚙️ 配置

配置文件位于 `~/.process-tracker.yaml`，首次运行会自动创建默认配置。

### 极简配置（仅3个参数！）

```yaml
# 智能分类（可选，默认启用）
enable_smart_categories: true  # 自动识别进程类型

# 存储管理（自动轮转和压缩）
storage:
  max_size_mb: 100    # 最大存储空间(MB)，自动轮转
  keep_days: 7        # 保留天数，0=永久保留

# Docker监控（可选，自动检测）
docker:
  enabled: true       # 启用Docker容器监控
```

**零配置运行**：不需要配置文件，直接运行即可！所有参数都有智能默认值。

**智能特性**：
- ✅ 自动文件轮转（每个文件 ~20MB）
- ✅ 自动压缩旧文件（1天后）
- ✅ 自动清理过期数据（根据keep_days）
- ✅ 自动检测Docker环境
- ✅ 智能进程分类

> **设计理念**: 遵循"简单优先"原则，只保留最必要的配置。高级功能（如进程控制、资源配额等）请使用专门工具（systemd、supervisor等）配合使用。

## 📈 监控指标

- **CPU使用率**: 进程CPU占用百分比
- **内存使用**: 进程内存占用(MB)
- **线程数**: 进程线程数量
- **磁盘I/O**: 读取和写入数据量(MB)
- **网络流量**: ⚠️ 当前版本未实现进程级网络监控（数据始终为0）
- **活跃状态**: 基于CPU和内存使用的活动检测
- **Docker容器**: 容器级别的资源使用统计（需启用Docker监控）

## 🏗️ 架构特点

- **高效缓冲**: 批量写入减少I/O操作
- **向后兼容**: 支持多种数据格式版本
- **优雅关闭**: 完整的信号处理
- **资源友好**: 低系统开销
- **可扩展**: 模块化设计便于扩展

## 📦 版本发布

每个版本都会为以下平台构建：

- Linux AMD64
- Linux ARM64
- macOS Intel (AMD64)
- macOS ARM64 (Apple Silicon)
- Windows AMD64

## 🛠️ 开发

### 构建项目

```bash
# 构建所有平台版本
./build.sh

# 手动构建当前平台
go build -ldflags="-X main.Version=0.3.7" -o process-tracker .
```

### 项目结构

```
process-tracker/
├── main.go              # 主程序入口
├── core/                 # 核心功能模块
│   ├── app.go           # 应用核心逻辑
│   ├── types.go         # 数据类型定义
│   ├── unified_monitor.go  # 统一监控器
│   └── storage_manager.go # 存储管理
├── tests/                # 测试文件
│   ├── unit/            # 单元测试
│   │   ├── app_test.go
│   │   ├── unified_monitor_test.go
│   │   └── bio_tools_manager_test.go
│   └── README.md        # 测试文档
├── releases/             # 自动构建版本
│   └── v0.3.7/          # 版本目录
├── .git/hooks/          # Git hooks
├── build.sh             # 构建脚本
├── CLAUDE.md           # 开发文档
└── README.md           # 项目说明
```

### 自动构建

项目配置了 Git post-commit hook，每次提交后会自动：

1. **自动版本检测**: 从 `main.go` 中读取当前版本
2. **多平台构建**: 为支持的平台构建可执行文件
3. **文件组织**: 构建文件存储在 `releases/v{VERSION}/` 目录
4. **构建报告**: 显示构建状态和文件大小

#### 构建的平台版本
- Linux AMD64 (当前平台)
- macOS Intel (AMD64)
- macOS ARM64 (Apple Silicon)
- Linux ARM64

#### 自动构建文件位置
```
releases/v0.3.7/
├── process-tracker           # Linux AMD64
├── process-tracker-macos      # macOS Intel
├── process-tracker-macos-arm64 # macOS ARM64
└── process-tracker-linux-arm64 # Linux ARM64
```

## 📄 许可证

[添加您的许可证信息]

## 🤝 贡献

欢迎提交Issue和Pull Request！

## 📞 联系方式

[添加您的联系信息]