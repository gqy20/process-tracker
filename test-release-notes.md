## Process Tracker v0.3.7

🚀 **智能进程监控工具** - 用于跟踪和分析系统进程的资源使用情况

### 📦 下载

选择适合您平台的版本：

- **process-tracker-linux-amd64** - Linux Intel/AMD 64位
- **process-tracker-linux-arm64** - Linux ARM 64位
- **process-tracker-macos-amd64** - macOS Intel 64位
- **process-tracker-macos-arm64** - macOS ARM64 (Apple Silicon)
- **process-tracker-windows-amd64.exe** - Windows Intel/AMD 64位

### 🚀 快速开始

```bash
# 赋予执行权限
chmod +x process-tracker-*

# 开始监控
./process-tracker-linux-amd64 start

# 查看版本
./process-tracker-linux-amd64 version

# 查看帮助
./process-tracker-linux-amd64 help
```

### 📋 本次更新内容

11dc475 Initial commit: Process Tracker v0.1.0 MVP
bc86051 v0.1.1: Add basic resource monitoring (CPU, memory)
de7ab50 v0.1.2: Add I/O and network monitoring
a195e19 v0.1.3: Add active detection and usage time optimization
9f441b7 v0.1.4: Data format refactoring and compatibility
4357440 v0.1.5: Statistics and reporting improvements
f4330fe v0.2.0: Full feature integration and optimization
20c8b60 Implement v0.2.1: Smart process statistics improvements
c0493a1 Implement v0.2.2: Performance optimization and enhanced features
afe596a Cleanup project structure and complete v0.2.2 release

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
