# Process Tracker - MVP 简化版预估

## 反思：我之前的过度设计

我确实陷入了"架构师思维"，为MVP设计了过于复杂的结构。真正的MVP应该：

### 🎯 **MVP的本质需求**
1. **监控进程** - 获取进程使用时间
2. **存储数据** - 简单的本地存储
3. **显示报告** - 基本的命令行输出
4. **可以运行** - 下载即用，无需复杂安装

### ❌ **之前的过度设计**
- 太多的目录结构 (`internal/`, `pkg/`, `configs/`)
- 过多的脚本文件 (15个脚本!)
- 复杂的配置系统
- 过度的模块化

## 重新设计：真正的MVP

### 📁 **简化后的项目结构**
```
process-tracker/
├── main.go              # 主程序 (400行)
├── monitor.go           # 监控逻辑 (300行)
├── storage.go           # 数据存储 (200行)
├── config.go            # 配置 (100行)
├── build.sh             # 构建脚本 (30行)
├── install.sh           # 安装脚本 (40行)
├── README.md            # 说明 (50行)
└── go.mod               # 依赖 (5行)
```

### 📊 **重新预估：代码行数**

| 模块 | 原预估 | 简化后 | 说明 |
|------|--------|--------|------|
| 主程序 | 400行 | 400行 | CLI入口，整合所有功能 |
| 监控逻辑 | 600行 | 300行 | 简化为单一文件 |
| 数据存储 | 500行 | 200行 | 基础SQLite操作 |
| 配置管理 | 250行 | 100行 | 简单配置文件 |
| **总计** | **1,750行** | **1,000行** | **减少43%** |

### 📜 **重新预估：脚本文件**

| 脚本 | 原预估 | 简化后 | 说明 |
|------|--------|--------|------|
| 构建脚本 | 6个 | 1个 | 统一构建脚本 |
| 安装脚本 | 3个 | 1个 | 简单安装 |
| 开发脚本 | 5个 | 0个 | 开发者可以手动 |
| 系统服务 | 2个 | 1个 | 只保留systemd |
| **总计** | **16个** | **3个** | **减少81%** |

## 最小可行方案

### 🔧 **核心功能 (1,000行)**

#### `main.go` (400行)
```go
package main

import (
    "fmt"
    "log"
    "os"
    "time"
    
    "github.com/shirou/gopsutil/v3/process"
    "github.com/mattn/go-sqlite3"
)

// 全局变量 - 简化设计
var (
    dbPath = os.ExpandEnv("~/.process-tracker.db")
    interval = 5 * time.Second
)

func main() {
    if len(os.Args) < 2 {
        printUsage()
        return
    }
    
    switch os.Args[1] {
    case "start":
        startMonitoring()
    case "today":
        showTodayReport()
    case "week":
        showWeekReport()
    case "status":
        showStatus()
    default:
        printUsage()
    }
}

// 简单的监控循环
func startMonitoring() {
    log.Println("Starting process monitoring...")
    
    for {
        processes, _ := process.Processes()
        for _, p := range processes {
            name, _ := p.Name()
            cpuPercent, _ := p.CPUPercent()
            createTime, _ := p.CreateTime()
            
            // 简单存储逻辑
            saveProcessData(name, cpuPercent, createTime)
        }
        
        time.Sleep(interval)
    }
}

// 其他函数...
```

#### `monitor.go` (300行)
```go
package main

import (
    "github.com/shirou/gopsutil/v3/process"
    "time"
)

// 进程数据结构
type ProcessRecord struct {
    Name      string
    PID       int32
    CPU       float64
    Memory    uint64
    Timestamp time.Time
}

// 获取进程信息
func getProcessInfo() ([]ProcessRecord, error) {
    processes, err := process.Processes()
    if err != nil {
        return nil, err
    }
    
    var records []ProcessRecord
    for _, p := range processes {
        name, _ := p.Name()
        cpu, _ := p.CPUPercent()
        mem, _ := p.MemoryInfo()
        pid := p.Pid
        
        records = append(records, ProcessRecord{
            Name:      name,
            PID:       pid,
            CPU:       cpu,
            Memory:    mem.RSS,
            Timestamp: time.Now(),
        })
    }
    
    return records, nil
}

// 应用识别
func identifyApp(name string) string {
    appCategories := map[string]string{
        "chrome":    "Browser",
        "firefox":   "Browser",
        "code":      "Development",
        "go":        "Development",
        "python":    "Development",
        "node":      "Development",
        "bash":      "System",
        "zsh":       "System",
    }
    
    if category, exists := appCategories[name]; exists {
        return category
    }
    return "Other"
}
```

#### `storage.go` (200行)
```go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    "time"
    
    _ "github.com/mattn/go-sqlite3"
)

type DB struct {
    conn *sql.DB
}

// 初始化数据库
func initDB() (*DB, error) {
    // 确保数据目录存在
    os.MkdirAll(os.ExpandEnv("~/.process-tracker"), 0755)
    
    conn, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }
    
    // 创建表
    _, err = conn.Exec(`
        CREATE TABLE IF NOT EXISTS process_logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
            process_name TEXT,
            pid INTEGER,
            cpu_time REAL,
            memory_bytes INTEGER,
            category TEXT
        )
    `)
    
    if err != nil {
        return nil, err
    }
    
    return &DB{conn: conn}, nil
}

// 保存进程数据
func (db *DB) SaveProcess(name string, cpu float64, pid int32) error {
    category := identifyApp(name)
    _, err := db.conn.Exec(`
        INSERT INTO process_logs (process_name, pid, cpu_time, category)
        VALUES (?, ?, ?, ?)
    `, name, pid, cpu, category)
    
    return err
}

// 获取今日统计
func (db *DB) GetTodayStats() (map[string]time.Duration, error) {
    rows, err := db.conn.Query(`
        SELECT process_name, COUNT(*) * 5 as seconds
        FROM process_logs 
        WHERE date(timestamp) = date('now')
        GROUP BY process_name
    `)
    
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    stats := make(map[string]time.Duration)
    for rows.Next() {
        var name string
        var seconds int
        rows.Scan(&name, &seconds)
        stats[name] = time.Duration(seconds) * time.Second
    }
    
    return stats, nil
}
```

#### `config.go` (100行)
```go
package main

import (
    "os"
    "time"
    
    "gopkg.in/yaml.v2"
)

type Config struct {
    Interval time.Duration `yaml:"interval"`
    DBPath  string        `yaml:"db_path"`
}

// 默认配置
var defaultConfig = Config{
    Interval: 5 * time.Second,
    DBPath:   os.ExpandEnv("~/.process-tracker.db"),
}

// 加载配置
func loadConfig() Config {
    configPath := os.ExpandEnv("~/.process-tracker/config.yaml")
    
    // 如果配置文件不存在，创建默认配置
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        saveConfig(defaultConfig)
        return defaultConfig
    }
    
    data, err := os.ReadFile(configPath)
    if err != nil {
        return defaultConfig
    }
    
    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return defaultConfig
    }
    
    return config
}

// 保存配置
func saveConfig(config Config) {
    configPath := os.ExpandEnv("~/.process-tracker/config.yaml")
    data, _ := yaml.Marshal(config)
    os.WriteFile(configPath, data, 0644)
}
```

### 📜 **简化的脚本 (3个)**

#### `build.sh` (30行)
```bash
#!/bin/bash
set -e

echo "Building process-tracker..."

# 简单构建
go build -o process-tracker .

# 如果有Go环境，也构建Windows版本
if command -v go &> /dev/null; then
    GOOS=windows GOARCH=amd64 go build -o process-tracker.exe .
    echo "Windows version built: process-tracker.exe"
fi

echo "Build completed!"
```

#### `install.sh` (40行)
```bash
#!/bin/bash
set -e

echo "Installing process-tracker..."

# 构建项目
./build.sh

# 创建目录
mkdir -p ~/.config/process-tracker

# 复制二进制文件
sudo cp process-tracker /usr/local/bin/
sudo chmod +x /usr/local/bin/process-tracker

# 创建systemd服务（仅Linux）
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    cat > /tmp/process-tracker.service << 'EOF'
[Unit]
Description=Process Tracker
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/process-tracker start
Restart=always
User=$USER

[Install]
WantedBy=multi-user.target
EOF

    sudo cp /tmp/process-tracker.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable process-tracker
    echo "Systemd service installed"
fi

echo "Installation completed!"
echo "Run 'process-tracker today' to test"
```

#### `uninstall.sh` (20行)
```bash
#!/bin/bash
set -e

echo "Uninstalling process-tracker..."

# 停止服务
sudo systemctl stop process-tracker 2>/dev/null || true
sudo systemctl disable process-tracker 2>/dev/null || true

# 删除文件
sudo rm -f /usr/local/bin/process-tracker
sudo rm -f /etc/systemd/system/process-tracker.service
sudo systemctl daemon-reload 2>/dev/null || true

# 删除用户数据（可选）
read -p "Remove user data? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    rm -rf ~/.config/process-tracker
    rm -f ~/.process-tracker.db
fi

echo "Uninstall completed!"
```

## 重新预估：开发工作量

### 🎯 **新的开发计划 (2-3周)**

#### 第1周：核心功能
- 实现进程监控 (400行)
- 实现数据存储 (200行)
- 基础CLI功能 (200行)
- **总计**：800行

#### 第2周：完善功能
- 报告生成 (200行)
- 配置管理 (100行)
- 测试和调试 (100行)
- **总计**：400行

#### 第3周：部署和文档
- 构建脚本 (30行)
- 安装脚本 (40行)
- 文档完善 (50行)
- **总计**：120行

### 📈 **最终统计**
- **总代码行数**：1,320行 (原2,400行，减少45%)
- **脚本文件数**：3个 (原16个，减少81%)
- **开发时间**：2-3周 (原5周，减少50%)
- **复杂度**：显著降低

## 关键改进

1. **单文件设计**：每个功能一个文件，避免过度模块化
2. **全局变量**：简化依赖注入
3. **内置配置**：减少配置复杂性
4. **简单脚本**：只保留必要的构建和安装脚本
5. **渐进式开发**：先让功能跑起来，再考虑优化

这样的MVP更加真实，更容易快速实现和验证想法。你觉得这个简化版本如何？