# Process Tracker - 进程监控工具

一个简单高效的Linux进程监控工具，提供实时进程资源使用统计和Web界面。

## 🚀 快速开始

### 安装

```bash
# 编译
go build -o process-tracker main.go

# 或者使用构建脚本
./build.sh
```

### 基本使用

```bash
# 查看帮助
./process-tracker help

# 启动监控
./process-tracker start

# 查看今日统计
./process-tracker stats

# 启动Web界面
./process-tracker web

# 查看运行状态
./process-tracker status

# 停止监控
./process-tracker stop
```

## 📋 核心命令

Process Tracker 遵循简洁设计原则，只提供5个核心命令：

### 1. start - 启动监控
```bash
./process-tracker start [选项]

选项:
  -i N  监控间隔(秒) [默认: 5]
  -w    同时启动Web界面
  -p PORT  Web端口

示例:
  ./process-tracker start                    # 默认5秒间隔启动
  ./process-tracker start -i 10              # 10秒间隔
  ./process-tracker start -w                 # 启动Web界面
  ./process-tracker start -i 10 -w -p 9090   # 10秒间隔，Web在9090端口
```

### 2. stop - 停止监控
```bash
./process-tracker stop
```

### 3. status - 查看状态
```bash
./process-tracker status
```

### 4. stats - 查看统计
```bash
./process-tracker stats [选项]

选项:
  -d  显示今日统计 (默认)
  -w  显示本周统计
  -m  显示本月统计

示例:
  ./process-tracker stats      # 今日统计
  ./process-tracker stats -w   # 本周统计
  ./process-tracker stats -m   # 本月统计
```

### 5. web - 启动Web界面
```bash
./process-tracker web [选项]

选项:
  -p PORT  Web端口 [默认: 8080]
  -h HOST  Web主机 [默认: 0.0.0.0]

示例:
  ./process-tracker web           # 默认配置启动Web
  ./process-tracker web -p 9090   # Web在9090端口
```

## ⚙️ 配置

配置文件位置：`~/.process-tracker/config.yaml`

```yaml
# 存储配置
storage:
  type: "sqlite"              # 存储类型: csv/sqlite
  sqlite_path: "~/.process-tracker/process-tracker.db"
  max_file_size_mb: 50        # 最大文件大小 (CSV)
  keep_days: 7                # 保留天数

# Web界面配置
web:
  enabled: true               # 启用Web界面
  host: "0.0.0.0"            # 绑定主机
  port: "8080"               # 端口号

# 监控配置
monitoring:
  interval: "5s"              # 监控间隔
```

## 📊 数据存储

支持两种存储方式：

### CSV存储 (默认)
- 文件路径：`~/.process-tracker/process-tracker.log`
- 简单易读，兼容性好
- 适合小规模监控

### SQLite存储 (推荐)
- 数据库路径：`~/.process-tracker/process-tracker.db`
- 高性能，支持复杂查询
- 适合长期监控和大数据量

从CSV迁移到SQLite：
```bash
# 迁移数据（备份原始CSV文件）
./process-tracker migrate-to-sqlite

# 指定自定义路径
./process-tracker migrate-to-sqlite --sqlite-path /path/to/database.db
```

## 🌐 Web界面

Web界面提供：
- 实时进程监控
- 历史数据统计
- 进程资源使用图表
- 系统概览

默认地址：http://localhost:8080

## 📈 统计功能

统计信息包括：
- CPU使用率
- 内存使用量
- 磁盘I/O
- 网络流量
- 进程活跃时间
- 进程分类统计

## 🔧 系统要求

- Linux操作系统
- Go 1.19+
- 超级用户权限 (读取进程信息)

## 📝 设计原则

遵循以下设计原则：
- **简洁性**：只提供核心功能，避免过度设计
- **实用性**：专注于实际监控需求
- **性能**：高效的资源监控和数据存储
- **兼容性**：支持多种存储方式

## 🤝 贡献

欢迎提交Issue和Pull Request来改进这个工具。

## 📄 许可证

MIT License