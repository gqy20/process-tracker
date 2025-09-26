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

配置文件位于 `~/.process-tracker.yaml`，首次运行会自动创建默认配置：

```yaml
statistics_granularity: detailed  # simple|detailed|full
show_commands: true               # 显示完整命令
show_working_dirs: true           # 显示工作目录
use_smart_categories: true        # 使用智能分类
max_command_length: 100           # 最大命令长度
max_dir_length: 50               # 最大目录长度

# 存储管理配置
storage:
  max_file_size_mb: 100          # 最大文件大小(MB)
  max_files: 10                  # 最大保留文件数
  compress_after_days: 3         # 压缩天数
  cleanup_after_days: 30         # 清理天数
  auto_cleanup: true              # 自动清理
```

## 📈 监控指标

- **CPU使用率**: 进程CPU占用百分比
- **内存使用**: 进程内存占用(MB)
- **线程数**: 进程线程数量
- **磁盘I/O**: 读取和写入数据量(MB)
- **网络流量**: 发送和接收数据量(KB)
- **活跃状态**: 基于资源使用的活动检测

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
go build -ldflags="-X main.Version=0.3.0" -o process-tracker .
```

### 项目结构

```
process-tracker/
├── main.go              # 主程序入口
├── core/                 # 核心功能模块
│   ├── app.go           # 应用核心逻辑
│   ├── types.go         # 数据类型定义
│   └── storage_manager.go # 存储管理
├── build.sh             # 构建脚本
├── CLAUDE.md           # 开发文档
└── README.md           # 项目说明
```

## 📄 许可证

[添加您的许可证信息]

## 🤝 贡献

欢迎提交Issue和Pull Request！

## 📞 联系方式

[添加您的联系信息]