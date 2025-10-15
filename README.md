# Process Tracker

一个智能的进程监控工具，提供实时Web界面和命令行统计，用于跟踪和分析系统进程的资源使用情况。

## ✨ 主要特性

- 🌐 **Web界面**: 实时可视化仪表板，图表展示CPU/内存趋势
- 🔔 **智能告警**: 系统级/进程级资源监控，支持飞书/钉钉/企业微信通知
- 🔍 **实时监控**: CPU、内存、磁盘I/O、Docker容器监控
- 📊 **智能统计**: 支持简单、详细、完整三种统计粒度
- 🗂️ **智能分类**: 自动识别应用程序类型（Java、Node.js、Python等）
- 💾 **存储优化**: 自动文件轮转和压缩，节省90%存储空间
- 🎛️ **守护进程**: start/stop/restart/status 进程管理
- 📁 **多平台支持**: Linux、macOS、Windows
- 📤 **数据导出**: JSON/CSV格式数据导出

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

#### 启动Web界面（推荐）

```bash
# 启动Web监控界面
./process-tracker start --web

# 访问仪表板
# 浏览器打开任意显示的内网IP地址，例如：
# http://192.168.1.100:18080
```

#### 命令行统计

```bash
# 进程管理
./process-tracker start          # 后台启动监控
./process-tracker stop           # 停止监控
./process-tracker restart --web  # 重启并启用Web
./process-tracker status         # 查看状态

# 查看统计
./process-tracker today          # 今日统计
./process-tracker week           # 本周统计  
./process-tracker month          # 本月统计
./process-tracker details        # 详细统计

# 数据管理
./process-tracker export         # 导出数据
./process-tracker cleanup        # 清理旧数据

# 告警测试
./process-tracker test-alert     # 测试告警通知
```

### 配置告警（可选）

```bash
# 1. 设置webhook环境变量
export WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_TOKEN"

# 2. 使用示例配置启动
./process-tracker --config config-example.yaml start

# 3. 测试告警
./process-tracker --config config-example.yaml test-alert
```

**告警功能**：
- ✅ 系统CPU/内存使用率监控
- ✅ 单个进程异常检测
- ✅ 支持飞书/钉钉/企业微信
- ✅ 智能告警抑制避免风暴

详细配置请参考 [ALERTS.md](docs/ALERTS.md)

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

### 系统级指标
- **CPU使用率（归一化）**: 0-100%表示整个系统的CPU使用率
  - 例如：72核系统，单进程100% CPU显示为1.39%
- **内存使用**: MB和百分比双重显示
  - 例如：24GB (7.52%) - 一目了然系统压力

### 进程级指标
- **CPU**: 原始CPU百分比 + 归一化百分比
- **内存**: 绝对值(MB) + 占系统百分比
- **线程数**: 进程线程数量
- **磁盘I/O**: 读取和写入数据量(MB)
- **进程信息**: PID、启动时间、CPU累积时间
- **活跃状态**: 基于CPU和内存使用的智能检测

### Docker监控
- **容器统计**: CPU、内存、磁盘、网络流量
- **自动检测**: 自动发现并监控运行中的容器
- **分类标识**: docker:容器名 格式显示

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

## 📚 文档

- [告警配置指南](docs/ALERTS.md) - 告警功能完整说明
- [Web快速开始](docs/QUICKSTART.md) - Web界面使用指南
- [功能详解](docs/FEATURES.md) - CPU归一化、内存百分比等功能说明
- [开发指南](docs/development.md) - 贡献代码前请阅读
- [AI开发助手](CLAUDE.md) - AI助手的项目架构说明

## 🆕 最新更新 (v0.3.9+)

- ✅ Web实时监控界面
- ✅ CPU归一化显示（0-100%表示系统整体）
- ✅ 内存百分比显示
- ✅ 守护进程管理（start/stop/restart/status）
- ✅ 自动显示所有内网IP地址
- ✅ 优雅关闭机制（5秒超时保护）
- ✅ Docker容器监控
- ✅ 进程搜索和分类过滤

## 📄 许可证

MIT License

## 🤝 贡献

欢迎提交Issue和Pull Request！

开发前请阅读：
- [CLAUDE.md](CLAUDE.md) - 项目架构和设计理念
- [docs/development.md](docs/development.md) - 开发指南