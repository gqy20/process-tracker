# Process Tracker 快速开始

## 🚀 5分钟快速上手

### 1. 安装和编译

```bash
# 克隆项目
git clone <repository-url>
cd process-tracker

# 编译
go build -o process-tracker main.go
```

### 2. 启动监控

```bash
# 启动后台监控
./process-tracker start

# 检查状态
./process-tracker status
```

### 3. 查看统计

```bash
# 查看今日统计
./process-tracker stats

# 查看本周统计
./process-tracker stats -w
```

### 4. 启动Web界面

```bash
# 启动Web界面
./process-tracker web

# 浏览器访问 http://localhost:8080
```

### 5. 停止监控

```bash
./process-tracker stop
```

## 📋 命令参考

### 核心命令

| 命令 | 功能 | 示例 |
|------|------|------|
| `start` | 启动监控 | `./process-tracker start -i 10` |
| `stop` | 停止监控 | `./process-tracker stop` |
| `status` | 查看状态 | `./process-tracker status` |
| `stats` | 查看统计 | `./process-tracker stats -w` |
| `web` | Web界面 | `./process-tracker web -p 9090` |

### 常用选项

- `-i N` - 设置监控间隔（秒）
- `-w` - 同时启动Web界面
- `-p PORT` - 设置端口
- `-d` - 今日统计
- `-w` - 本周统计
- `-m` - 本月统计

## ⚙️ 配置

配置文件位置：`~/.process-tracker/config.yaml`

```yaml
storage:
  type: "sqlite"
  sqlite_path: "~/.process-tracker/process-tracker.db"
  keep_days: 7

web:
  enabled: true
  host: "0.0.0.0"
  port: "8080"
```

## 🔧 故障排除

### 权限问题
```bash
# 确保有读取进程信息的权限
sudo ./process-tracker start
```

### 端口占用
```bash
# 使用其他端口
./process-tracker web -p 9090
```

### 数据迁移
```bash
# 从CSV迁移到SQLite
./process-tracker migrate-to-sqlite
```

## 📞 获取帮助

```bash
# 查看完整帮助
./process-tracker help

# 查看版本
./process-tracker version
```