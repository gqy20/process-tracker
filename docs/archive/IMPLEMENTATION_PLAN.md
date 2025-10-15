# Web界面 + Webhook 实施方案

## 📋 实施概览

**目标**: 实现轻量级Web Dashboard和告警系统  
**工期**: 3-4天  
**技术栈**: Go标准库 + embed + Chart.js + Webhook

---

## 🗂️ 文件结构设计

```
process-tracker/
├── cmd/
│   ├── commands.go           # 现有命令
│   ├── config.go             # 现有配置
│   ├── web.go                # 🆕 Web服务器
│   └── static/               # 🆕 静态文件
│       ├── index.html        # 主页面
│       ├── css/
│       │   └── style.css     # 自定义样式（可选）
│       └── js/
│           └── app.js        # 前端逻辑
├── core/
│   ├── app.go                # 现有核心
│   ├── types.go              # 现有类型
│   ├── alerting.go           # 🆕 告警引擎
│   ├── notifiers.go          # 🆕 通知器接口
│   ├── webhook_notifier.go   # 🆕 Webhook实现
│   ├── dingtalk_notifier.go  # 🆕 钉钉实现
│   └── wechat_notifier.go    # 🆕 企微实现
├── main.go                   # 添加web命令
└── config.example.yaml       # 🆕 配置示例
```

---

## 🎯 实施步骤

### Day 1: Web服务器核心（6-8小时）

#### 1.1 创建Web服务器框架
**文件**: `cmd/web.go`

**功能**:
- HTTP服务器启动
- 静态文件服务（embed）
- API路由注册
- 优雅关闭

#### 1.2 创建基础HTML
**文件**: `cmd/static/index.html`

**功能**:
- 响应式布局（Tailwind CSS）
- 概览卡片（进程数、CPU、内存）
- Chart.js图表容器
- 进程列表表格

#### 1.3 实现API端点
**端点**:
- `GET /api/stats/today` - 今日统计
- `GET /api/stats/week` - 本周统计
- `GET /api/live` - 实时数据
- `GET /api/processes` - 进程列表

---

### Day 2: 前端完善 + 数据可视化（6-8小时）

#### 2.1 实现前端逻辑
**文件**: `cmd/static/js/app.js`

**功能**:
- 数据获取（fetch API）
- Chart.js图表渲染
- 自动刷新（5秒间隔）
- 进程列表更新
- 错误处理

#### 2.2 数据处理优化
**优化点**:
- 仅读取文件末尾（避免全文件加载）
- 数据聚合（按小时/天）
- 缓存机制（5秒缓存）

---

### Day 3: Webhook告警系统（6-8小时）

#### 3.1 实现告警引擎
**文件**: `core/alerting.go`

**功能**:
- 告警规则定义
- 规则评估引擎
- 告警状态管理
- 告警抑制（防止重复）

#### 3.2 实现通知器
**文件**:
- `core/notifiers.go` - 接口定义
- `core/webhook_notifier.go` - 通用Webhook
- `core/dingtalk_notifier.go` - 钉钉机器人
- `core/wechat_notifier.go` - 企业微信

#### 3.3 集成到监控循环
**修改**: `core/app.go`

**功能**:
- 每次采集后评估告警
- 触发通知

---

### Day 4: 集成测试 + 文档（4-6小时）

#### 4.1 集成到main.go
**新命令**:
- `process-tracker web` - 启动Web服务器
- `process-tracker start --web` - 监控+Web

#### 4.2 编写配置示例
**文件**: `config.example.yaml`

#### 4.3 测试
- 单元测试（告警规则）
- 集成测试（Web API）
- 手动测试（浏览器）

#### 4.4 文档
- README更新
- 配置说明
- 快速开始

---

## 💻 核心代码设计

### Web服务器架构

```go
// cmd/web.go
package cmd

import (
    "embed"
    "io/fs"
    "net/http"
    "encoding/json"
    "time"
)

//go:embed static/*
var staticFS embed.FS

type WebServer struct {
    app    *core.App
    port   string
    server *http.Server
    cache  *StatsCache
}

func NewWebServer(app *core.App, port string) *WebServer {
    return &WebServer{
        app:   app,
        port:  port,
        cache: NewStatsCache(5 * time.Second),
    }
}

func (ws *WebServer) Start() error {
    mux := http.NewServeMux()
    
    // 静态文件
    staticSub, _ := fs.Sub(staticFS, "static")
    mux.Handle("/", http.FileServer(http.FS(staticSub)))
    
    // API端点
    mux.HandleFunc("/api/stats/today", ws.handleStatsToday)
    mux.HandleFunc("/api/stats/week", ws.handleStatsWeek)
    mux.HandleFunc("/api/live", ws.handleLive)
    mux.HandleFunc("/api/processes", ws.handleProcesses)
    
    ws.server = &http.Server{
        Addr:    ":" + ws.port,
        Handler: mux,
    }
    
    return ws.server.ListenAndServe()
}

// 缓存机制
type StatsCache struct {
    data   map[string]interface{}
    expiry map[string]time.Time
    ttl    time.Duration
    mu     sync.RWMutex
}
```

### API响应格式

```go
// 统计数据
type DashboardStats struct {
    ProcessCount  int              `json:"process_count"`
    ActiveCount   int              `json:"active_count"`
    AvgCPU        float64          `json:"avg_cpu"`
    MaxCPU        float64          `json:"max_cpu"`
    TotalMemory   float64          `json:"total_memory"` // MB
    Timeline      []TimelinePoint  `json:"timeline"`
    TopProcesses  []ProcessSummary `json:"top_processes"`
}

type TimelinePoint struct {
    Time   string  `json:"time"`   // "15:00"
    CPU    float64 `json:"cpu"`    // 平均CPU%
    Memory float64 `json:"memory"` // 总内存MB
}

type ProcessSummary struct {
    PID        int32   `json:"pid"`
    Name       string  `json:"name"`
    CPUPercent float64 `json:"cpu_percent"`
    MemoryMB   float64 `json:"memory_mb"`
    Status     string  `json:"status"`
}
```

### 告警系统架构

```go
// core/alerting.go
package core

type AlertRule struct {
    Name      string   `yaml:"name"`
    Metric    string   `yaml:"metric"`     // cpu_percent, memory_mb
    Threshold float64  `yaml:"threshold"`
    Duration  int      `yaml:"duration"`   // 秒
    Channels  []string `yaml:"channels"`   // webhook, dingtalk, wechat
}

type AlertManager struct {
    rules     []AlertRule
    notifiers map[string]Notifier
    states    map[string]*AlertState
    mu        sync.RWMutex
}

type AlertState struct {
    Rule       *AlertRule
    StartTime  time.Time
    Count      int
    LastNotify time.Time
    Suppressed bool
}

func (am *AlertManager) Evaluate(records []ResourceRecord) {
    for _, rule := range am.rules {
        value := am.getMetricValue(records, rule.Metric)
        
        if value > rule.Threshold {
            am.handleAlert(rule, value)
        } else {
            am.clearAlert(rule.Name)
        }
    }
}
```

### 通知器接口

```go
// core/notifiers.go
package core

type Notifier interface {
    Send(title, content string) error
}

type NotifierConfig struct {
    Type   string                 `yaml:"type"`
    Config map[string]interface{} `yaml:"config"`
}
```

---

## 🎨 前端设计

### HTML结构（简化版）

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Process Tracker</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0"></script>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100">
    <div class="container mx-auto p-4">
        <h1 class="text-3xl font-bold mb-6">Process Tracker</h1>
        
        <!-- 概览卡片 -->
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
            <div class="card">
                <h3>活跃进程</h3>
                <p id="process-count">-</p>
            </div>
            <!-- 更多卡片... -->
        </div>
        
        <!-- 图表 -->
        <div class="card mb-6">
            <canvas id="cpuChart"></canvas>
        </div>
        
        <!-- 进程列表 -->
        <div class="card">
            <table id="process-table">
                <!-- 动态生成 -->
            </table>
        </div>
    </div>
    
    <script src="js/app.js"></script>
</body>
</html>
```

### JavaScript逻辑

```javascript
// app.js
class Dashboard {
    constructor() {
        this.chart = null;
        this.refreshInterval = 5000; // 5秒
        this.init();
    }
    
    async init() {
        await this.loadStats();
        setInterval(() => this.loadStats(), this.refreshInterval);
    }
    
    async loadStats() {
        try {
            const response = await fetch('/api/stats/today');
            const data = await response.json();
            this.updateUI(data);
        } catch (error) {
            console.error('加载数据失败:', error);
        }
    }
    
    updateUI(data) {
        // 更新卡片
        document.getElementById('process-count').textContent = data.process_count;
        
        // 更新图表
        this.updateChart(data.timeline);
        
        // 更新进程列表
        this.updateProcessTable(data.top_processes);
    }
    
    updateChart(timeline) {
        // Chart.js更新逻辑
    }
}

// 启动
new Dashboard();
```

---

## 📝 配置文件设计

```yaml
# ~/.process-tracker/config.yaml

# Web服务器配置
web:
  enabled: true
  port: 8080
  host: "localhost"  # 默认仅本地访问

# 告警配置
alerts:
  enabled: true
  
  # 告警规则
  rules:
    - name: high_cpu
      metric: cpu_percent
      threshold: 80
      duration: 300  # 持续5分钟
      channels: ["dingtalk"]
      
    - name: high_memory
      metric: memory_mb
      threshold: 1024  # 1GB
      duration: 300
      channels: ["wechat", "webhook"]

# 通知器配置
notifiers:
  # 钉钉机器人
  dingtalk:
    webhook_url: "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN"
    secret: ""  # 可选，签名验证
    
  # 企业微信机器人
  wechat:
    webhook_url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY"
    
  # 自定义Webhook
  webhook:
    url: "https://your-webhook-url"
    method: "POST"
    headers:
      Content-Type: "application/json"
```

---

## 🧪 测试计划

### 单元测试

```go
// core/alerting_test.go
func TestAlertEvaluation(t *testing.T) {
    rule := AlertRule{
        Name:      "test_cpu",
        Metric:    "cpu_percent",
        Threshold: 80,
        Duration:  5,
    }
    
    am := NewAlertManager([]AlertRule{rule})
    
    // 测试超阈值
    records := []ResourceRecord{
        {CPUPercent: 85},
    }
    
    am.Evaluate(records)
    
    state := am.states["test_cpu"]
    if state == nil {
        t.Error("应该创建告警状态")
    }
}
```

### 集成测试

```bash
#!/bin/bash
# test_web.sh

# 启动服务器
./process-tracker web --port 8081 &
PID=$!
sleep 2

# 测试API
echo "测试 /api/stats/today"
curl http://localhost:8081/api/stats/today | jq .

echo "测试 /api/live"
curl http://localhost:8081/api/live | jq .

# 清理
kill $PID
```

---

## 📊 里程碑

### Milestone 1: Web基础（Day 1结束）
- [x] Web服务器可访问
- [x] 静态页面显示
- [x] API返回数据

### Milestone 2: 可视化（Day 2结束）
- [x] 图表正常显示
- [x] 数据自动刷新
- [x] 进程列表更新

### Milestone 3: 告警（Day 3结束）
- [x] 规则评估工作
- [x] Webhook发送成功
- [x] 钉钉/企微通知

### Milestone 4: 完成（Day 4结束）
- [x] 所有功能集成
- [x] 测试通过
- [x] 文档完善

---

## 🎯 验收标准

### 功能验收
- [ ] 访问 http://localhost:8080 显示Dashboard
- [ ] 显示今日/本周统计数据
- [ ] 图表实时更新
- [ ] CPU超80%触发告警
- [ ] 钉钉/企微收到通知
- [ ] 移动端布局正常

### 性能验收
- [ ] 内存占用 < 60MB
- [ ] CPU占用 < 3%
- [ ] API响应 < 500ms
- [ ] 页面加载 < 2s

### 代码质量
- [ ] 单元测试覆盖率 > 60%
- [ ] 无明显内存泄漏
- [ ] 错误处理完善
- [ ] 日志输出合理

---

## 🚀 开始实施

准备好开始编码了！接下来的步骤：

1. ✅ 创建 `cmd/web.go`
2. ✅ 创建静态文件目录和HTML
3. ✅ 实现API端点
4. ✅ 实现Webhook通知器
5. ✅ 实现告警引擎
6. ✅ 集成到main.go
7. ✅ 测试和文档

**预计完成时间**: 3-4天  
**当前状态**: 准备就绪

让我们开始吧！
