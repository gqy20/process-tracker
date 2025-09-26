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

### 基础配置

```yaml
# 统计和显示配置
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

### 进程控制配置

```yaml
# 进程控制选项
process_control:
  enabled: true|false             # 启用进程控制
  enable_auto_restart: true|false # 启用自动重启
  max_restarts: 3                 # 最大重启次数
  restart_delay: 5s               # 重启延迟
  check_interval: 10s             # 检查间隔
```

### 资源配额配置

```yaml
# 资源配额管理
resource_quota:
  enabled: true|false             # 启用资源配额
  check_interval: 30s             # 检查间隔
  default_action: warn|throttle|stop|restart|notify  # 默认动作
  max_violations: 5               # 最大违规次数
  violation_window: 5m            # 违规窗口期
```

### 进程发现配置

```yaml
# 进程自动发现
process_discovery:
  enabled: true|false             # 启用进程发现
  discovery_interval: 30s         # 发现间隔
  auto_manage: true|false        # 自动管理
  bio_tools_only: true|false     # 仅生物信息学工具
  process_patterns: [pattern1, pattern2]    # 进程模式
  exclude_patterns: [pattern1, pattern2]   # 排除模式
  max_processes: 100              # 最大进程数
  cpu_threshold: 80.0             # CPU阈值
  memory_threshold_mb: 1024      # 内存阈值(MB)
```

### 生物信息学工具配置

```yaml
# 生物信息学工具管理
bio_tools:
  enabled: true                   # 启用生物信息学工具
  auto_discovery: true            # 自动发现
  custom_tools:                   # 自定义工具列表
    - name: "tool-name"
      executable: "/path/to/tool"
      category: "alignment|assembly|analysis|visualization"
      description: "Tool description"
```

### 监控配置

```yaml
# 统一监控配置
monitoring:
  enabled: true                   # 启用监控
  interval: 1s                    # 监控间隔
  health_check_interval: 30s      # 健康检查间隔
  max_monitored_processes: 100    # 最大监控进程数
  performance_history_size: 1000  # 性能历史大小
  enable_detailed_io: false       # 启用详细IO监控
  auto_restart_attempt: true       # 自动重启尝试
  max_restart_attempts: 3         # 最大重启次数
```

### 健康检查配置

```yaml
# 健康检查规则
health_check_rules:
  - name: "cpu_rule"
    description: "CPU使用率检查"
    metric: "cpu"
    operator: ">"
    threshold: 80.0
    severity: "warning"
    enabled: true
  - name: "memory_rule"
    description: "内存使用检查"
    metric: "memory"
    operator: ">"
    threshold: 1024.0
    severity: "error"
    enabled: true
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