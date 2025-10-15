# Process Tracker 开发指南

> 本项目遵循 Dave Cheney 的 Go 编程理念

## 🎯 开发哲学

### 核心理念
1. **简单优先** - 选择最简单的解决方案
2. **可读性即正确性** - 代码应该像散文一样易读  
3. **错误是值** - 优雅处理每一个错误
4. **少即是多** - 更少的代码意味着更少的bug
5. **接口最小化** - 小接口，大实现

### 编程原则
- 依赖抽象，而不是具体实现
- 避免包的全局状态
- 使用channel协调goroutine
- 让调用者处理并发
- 测试是公共API的一部分

### 代码风格
- 使用短变量名（作用域越短，名字越简单）
- 优先使用 `:=` 而非 `var`
- 处理每一个错误，不使用 `_`
- 优先使用defer清理资源

### 架构思想
- 分层架构：依赖向下，信息向上
- main包只做协调，不包含业务逻辑
- 显式初始化，避免init函数
- 可测试性是设计目标

---

## 🏗️ 项目架构

### 目录结构
```
process-tracker/
├── main.go              # 主程序入口（协调层）
├── cmd/                 # 命令行接口层
│   ├── commands.go      # 监控命令
│   ├── config.go        # 配置管理
│   ├── web.go           # Web服务器
│   └── static/          # 静态资源
├── core/                # 核心业务逻辑层
│   ├── app.go           # 应用核心
│   ├── types.go         # 数据类型
│   ├── storage.go       # 存储抽象
│   ├── storage_manager.go  # 存储管理
│   ├── daemon.go        # 守护进程管理
│   ├── docker.go        # Docker监控
│   └── categories.go    # 进程分类
├── tests/               # 测试
│   └── unit/            # 单元测试
└── docs/                # 文档
```

### 层次关系
```
main.go (协调)
    ↓
cmd/ (命令接口)
    ↓
core/ (核心逻辑)
    ↓
gopsutil (系统API)
```

---

## 🔧 开发环境

### 依赖
```bash
go version  # >= 1.19
```

### 主要库
- `github.com/shirou/gopsutil/v3` - 系统信息采集
- `gopkg.in/yaml.v2` - 配置文件解析
- `github.com/docker/docker` - Docker客户端

### 安装依赖
```bash
go mod download
```

---

## 🛠️ 构建和测试

### 本地构建
```bash
# 构建当前平台
go build -o process-tracker main.go

# 带版本信息构建
go build -ldflags="-X main.Version=0.3.9" -o process-tracker main.go
```

### 多平台构建
```bash
# 使用构建脚本
./build.sh

# 手动指定平台
GOOS=linux GOARCH=amd64 go build -o process-tracker-linux-amd64
GOOS=darwin GOARCH=arm64 go build -o process-tracker-macos-arm64
```

### 运行测试
```bash
# 所有测试
go test ./...

# 单个包
go test ./core

# 带覆盖率
go test -cover ./...

# 详细输出
go test -v ./tests/unit/
```

---

## 📝 代码规范

### 命名约定
```go
// 包名：小写，单数
package core

// 类型：PascalCase
type ResourceRecord struct {}

// 函数：PascalCase（导出）或 camelCase（私有）
func CalculateStats() {}
func calculateAverage() {}

// 变量：camelCase
var maxCPU float64

// 常量：PascalCase或UPPER_CASE
const BufferSize = 100
```

### 错误处理
```go
// ✅ 好的做法
records, err := storage.ReadRecords()
if err != nil {
    return fmt.Errorf("failed to read records: %w", err)
}

// ❌ 避免
records, _ := storage.ReadRecords()  // 忽略错误
```

### 并发安全
```go
// ✅ 使用mutex保护共享数据
type Cache struct {
    mu    sync.RWMutex
    data  map[string]interface{}
}

func (c *Cache) Get(key string) interface{} {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.data[key]
}

// ✅ 使用channel通信
go func() {
    for record := range recordCh {
        process(record)
    }
}()
```

---

## 🔍 调试技巧

### 日志级别
```go
// 开发时启用详细日志
log.SetFlags(log.LstdFlags | log.Lshortfile)

// 关键路径添加日志
log.Printf("Processing %d records", len(records))
```

### 性能分析
```go
// CPU profiling
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// 访问 http://localhost:6060/debug/pprof/
```

### 内存泄漏检测
```bash
# 运行一段时间后
go tool pprof http://localhost:6060/debug/pprof/heap
```

---

## 📦 发布流程

### 版本号规则
- **主版本(Major)**: 不兼容的API更改
- **次版本(Minor)**: 向后兼容的功能新增
- **补丁(Patch)**: 向后兼容的bug修复

例如：v0.3.9 → 0主版本.3次版本.9补丁

### 发布步骤
1. 更新版本号（main.go中的Version）
2. 更新docs/release-notes.md
3. 提交代码
4. Git hook自动构建多平台版本
5. 打tag：`git tag v0.3.9`
6. 推送：`git push --tags`

---

## 🧪 测试指南

### 单元测试
```go
// tests/unit/app_test.go
func TestCalculateStats(t *testing.T) {
    app := core.NewApp("test.log", 5*time.Second, config)
    stats, err := app.CalculateResourceStats(24 * time.Hour)
    
    if err != nil {
        t.Fatalf("Failed: %v", err)
    }
    
    if len(stats) == 0 {
        t.Error("Expected stats, got none")
    }
}
```

### 表驱动测试
```go
func TestNormalizeProcessName(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"python3.9", "python"},
        {"node", "node"},
        {"/usr/bin/go", "go"},
    }
    
    for _, tt := range tests {
        got := normalizeProcessName(tt.input)
        if got != tt.expected {
            t.Errorf("got %s, want %s", got, tt.expected)
        }
    }
}
```

---

## 🐛 常见问题

### Q: 编译错误 "cannot find package"
```bash
# 解决：更新依赖
go mod tidy
go mod download
```

### Q: 测试失败 "permission denied"
```bash
# 解决：以root运行或加入docker组
sudo usermod -aG docker $USER
newgrp docker
```

### Q: Web界面无法访问
```bash
# 检查端口占用
lsof -i :18080

# 检查防火墙
sudo firewall-cmd --list-ports
```

---

## 🤝 贡献指南

### 提交PR前
1. 阅读本文档和[CLAUDE.md](../CLAUDE.md)
2. 运行所有测试：`go test ./...`
3. 确保代码通过：`go vet ./...`
4. 格式化代码：`go fmt ./...`
5. 更新文档（如果需要）

### PR要求
- ✅ 清晰的提交消息
- ✅ 相关测试用例
- ✅ 更新文档
- ✅ 没有破坏性更改（或明确说明）

### Commit消息格式
```
<type>: <subject>

<body>

<footer>
```

类型：
- `feat`: 新功能
- `fix`: Bug修复
- `docs`: 文档更新
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建/工具更改

示例：
```
feat: 添加CPU归一化显示

- 后端计算归一化CPU百分比
- 前端图表使用归一化值
- 更新存储格式为v7

Closes #123
```

---

## 📚 参考资料

### Go相关
- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Dave Cheney's Blog](https://dave.cheney.net/)

### 项目文档
- [README.md](../README.md) - 用户文档
- [CLAUDE.md](../CLAUDE.md) - 项目架构
- [QUICKSTART.md](QUICKSTART.md) - Web快速开始
- [FEATURES.md](FEATURES.md) - 功能详解

---

**欢迎贡献！让Process Tracker更好！** 🚀
