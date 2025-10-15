# Phase 1 实施快速指南

## 🚀 Quick Start - Web Dashboard原型（2-3天）

### Day 1: 基础框架

#### 步骤1: 创建Web服务器结构
```go
// cmd/web.go
package cmd

import (
    "embed"
    "io/fs"
    "net/http"
)

//go:embed static/*
var staticFS embed.FS

type WebServer struct {
    app  *core.App
    port string
}

func NewWebServer(app *core.App, port string) *WebServer {
    return &WebServer{app: app, port: port}
}

func (ws *WebServer) Start() error {
    // 静态文件
    staticSub, _ := fs.Sub(staticFS, "static")
    http.Handle("/", http.FileServer(http.FS(staticSub)))
    
    // API端点
    http.HandleFunc("/api/stats/today", ws.handleToday)
    http.HandleFunc("/api/stats/week", ws.handleWeek)
    http.HandleFunc("/api/live", ws.handleLive)
    
    return http.ListenAndServe(":"+ws.port, nil)
}
```

#### 步骤2: 创建静态文件结构
```
cmd/static/
├── index.html          # 主页面
├── css/
│   └── style.css       # 样式（可选，使用CDN的Tailwind也行）
└── js/
    └── app.js          # 前端逻辑
```

#### 步骤3: 最简单的HTML模板
```html
<!-- cmd/static/index.html -->
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Process Tracker</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100">
    <div class="container mx-auto p-4">
        <h1 class="text-3xl font-bold mb-4">Process Tracker Dashboard</h1>
        
        <!-- 概览卡片 -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
            <div class="bg-white p-4 rounded shadow">
                <h3 class="text-gray-600">活跃进程</h3>
                <p class="text-3xl font-bold" id="process-count">-</p>
            </div>
            <div class="bg-white p-4 rounded shadow">
                <h3 class="text-gray-600">平均CPU</h3>
                <p class="text-3xl font-bold" id="avg-cpu">-</p>
            </div>
            <div class="bg-white p-4 rounded shadow">
                <h3 class="text-gray-600">总内存</h3>
                <p class="text-3xl font-bold" id="total-memory">-</p>
            </div>
        </div>
        
        <!-- 图表 -->
        <div class="bg-white p-4 rounded shadow">
            <canvas id="cpuChart"></canvas>
        </div>
    </div>
    
    <script src="js/app.js"></script>
</body>
</html>
```

#### 步骤4: 前端逻辑
```javascript
// cmd/static/js/app.js
async function fetchStats() {
    const response = await fetch('/api/stats/today');
    const data = await response.json();
    updateDashboard(data);
}

function updateDashboard(data) {
    document.getElementById('process-count').textContent = data.process_count;
    document.getElementById('avg-cpu').textContent = data.avg_cpu.toFixed(1) + '%';
    document.getElementById('total-memory').textContent = 
        (data.total_memory / 1024).toFixed(1) + 'GB';
    
    // 更新图表
    updateChart(data.timeline);
}

// Chart.js配置
let cpuChart = null;
function updateChart(timeline) {
    const ctx = document.getElementById('cpuChart').getContext('2d');
    
    if (cpuChart) {
        cpuChart.destroy();
    }
    
    cpuChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: timeline.map(t => t.time),
            datasets: [{
                label: 'CPU Usage',
                data: timeline.map(t => t.cpu),
                borderColor: 'rgb(59, 130, 246)',
                tension: 0.1
            }]
        },
        options: {
            responsive: true,
            scales: {
                y: {
                    beginAtZero: true,
                    max: 100
                }
            }
        }
    });
}

// 自动刷新
setInterval(fetchStats, 5000);
fetchStats();
```

### Day 2: API实现

#### API处理器
```go
// cmd/web.go 继续

func (ws *WebServer) handleToday(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    // 读取最近24小时的数据
    records, err := ws.readRecentRecords(24 * time.Hour)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // 计算统计数据
    stats := ws.calculateStats(records)
    
    json.NewEncoder(w).Encode(stats)
}

type DashboardStats struct {
    ProcessCount  int              `json:"process_count"`
    AvgCPU        float64          `json:"avg_cpu"`
    TotalMemory   float64          `json:"total_memory"`
    Timeline      []TimelinePoint  `json:"timeline"`
    TopProcesses  []ProcessSummary `json:"top_processes"`
}

type TimelinePoint struct {
    Time   string  `json:"time"`
    CPU    float64 `json:"cpu"`
    Memory float64 `json:"memory"`
}

func (ws *WebServer) readRecentRecords(duration time.Duration) ([]core.ResourceRecord, error) {
    // 实现：从CSV文件读取最近N小时的数据
    // 可以复用现有的core.ReadRecords函数
    // 优化：只读取文件末尾，不要全文件加载
}

func (ws *WebServer) calculateStats(records []core.ResourceRecord) DashboardStats {
    // 按时间聚合数据
    timelineMap := make(map[string][]core.ResourceRecord)
    for _, r := range records {
        hour := r.Timestamp.Format("15:00")
        timelineMap[hour] = append(timelineMap[hour], r)
    }
    
    // 计算每小时平均值
    var timeline []TimelinePoint
    for hour, recs := range timelineMap {
        var totalCPU, totalMem float64
        for _, r := range recs {
            totalCPU += r.CPUPercent
            totalMem += r.MemoryMB
        }
        timeline = append(timeline, TimelinePoint{
            Time:   hour,
            CPU:    totalCPU / float64(len(recs)),
            Memory: totalMem / float64(len(recs)),
        })
    }
    
    // 排序
    sort.Slice(timeline, func(i, j int) bool {
        return timeline[i].Time < timeline[j].Time
    })
    
    return DashboardStats{
        ProcessCount: len(uniqueProcesses(records)),
        AvgCPU:       averageCPU(records),
        TotalMemory:  totalMemory(records),
        Timeline:     timeline,
    }
}
```

### Day 3: 集成与优化

#### 添加到main.go
```go
// main.go
case "web":
    webFlags := flag.NewFlagSet("web", flag.ExitOnError)
    port := webFlags.String("port", "8080", "Web服务器端口")
    webFlags.Parse(flag.Args()[1:])
    
    if err := app.Initialize(); err != nil {
        log.Fatalf("初始化失败: %v", err)
    }
    
    // 启动监控（后台）
    go func() {
        if err := monitorCmd.StartMonitoring(); err != nil {
            log.Printf("监控启动失败: %v", err)
        }
    }()
    
    // 启动Web服务器
    webServer := cmd.NewWebServer(app.App, *port)
    log.Printf("Web服务器启动: http://localhost:%s", *port)
    if err := webServer.Start(); err != nil {
        log.Fatalf("Web服务器失败: %v", err)
    }

case "start":
    // 原有的纯监控模式
    startFlags := flag.NewFlagSet("start", flag.ExitOnError)
    webEnabled := startFlags.Bool("web", false, "启用Web界面")
    port := startFlags.String("port", "8080", "Web服务器端口")
    // ... 其他参数
    
    if *webEnabled {
        // 启动监控+Web
    } else {
        // 仅启动监控
    }
```

---

## 🔔 告警系统实现（1周）

### 核心结构

```go
// core/alerting.go
package core

type AlertRule struct {
    Name      string  `yaml:"name"`
    Metric    string  `yaml:"metric"`     // cpu_percent, memory_mb
    Threshold float64 `yaml:"threshold"`
    Duration  int     `yaml:"duration"`   // 秒
    Webhook   string  `yaml:"webhook"`
}

type AlertManager struct {
    rules       []AlertRule
    notifiers   map[string]Notifier
    alertStates map[string]*AlertState // 追踪告警状态
    mu          sync.RWMutex
}

type AlertState struct {
    Rule       *AlertRule
    StartTime  time.Time
    Count      int           // 超过阈值的次数
    LastNotify time.Time     // 最后通知时间
    Suppressed bool          // 是否已抑制
}

func NewAlertManager(rules []AlertRule) *AlertManager {
    am := &AlertManager{
        rules:       rules,
        notifiers:   make(map[string]Notifier),
        alertStates: make(map[string]*AlertState),
    }
    
    // 注册通知器
    am.RegisterNotifier("webhook", &WebhookNotifier{})
    am.RegisterNotifier("dingtalk", &DingTalkNotifier{})
    
    return am
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

func (am *AlertManager) handleAlert(rule AlertRule, value float64) {
    am.mu.Lock()
    defer am.mu.Unlock()
    
    state, exists := am.alertStates[rule.Name]
    if !exists {
        state = &AlertState{
            Rule:      &rule,
            StartTime: time.Now(),
        }
        am.alertStates[rule.Name] = state
    }
    
    state.Count++
    duration := time.Since(state.StartTime).Seconds()
    
    // 检查是否满足持续时间
    if duration >= float64(rule.Duration) && !state.Suppressed {
        am.sendAlert(rule, value)
        state.Suppressed = true
        state.LastNotify = time.Now()
    }
}

func (am *AlertManager) sendAlert(rule AlertRule, value float64) {
    notifier := am.notifiers["webhook"]
    message := fmt.Sprintf(
        "告警: %s\n指标: %s\n当前值: %.2f\n阈值: %.2f",
        rule.Name, rule.Metric, value, rule.Threshold,
    )
    
    if err := notifier.Send(rule.Name, message); err != nil {
        log.Printf("发送告警失败: %v", err)
    }
}
```

### 通知器接口

```go
// core/notifiers.go
type Notifier interface {
    Send(title, content string) error
}

// Webhook通知器
type WebhookNotifier struct {
    URL string
}

func (w *WebhookNotifier) Send(title, content string) error {
    payload := map[string]string{
        "title":   title,
        "content": content,
        "time":    time.Now().Format(time.RFC3339),
    }
    
    data, _ := json.Marshal(payload)
    resp, err := http.Post(w.URL, "application/json", bytes.NewBuffer(data))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("webhook返回非200状态: %d", resp.StatusCode)
    }
    
    return nil
}

// 钉钉通知器
type DingTalkNotifier struct {
    WebhookURL string
    Secret     string
}

func (d *DingTalkNotifier) Send(title, content string) error {
    timestamp := time.Now().UnixMilli()
    sign := d.generateSign(timestamp)
    
    url := fmt.Sprintf("%s&timestamp=%d&sign=%s", 
        d.WebhookURL, timestamp, sign)
    
    payload := map[string]interface{}{
        "msgtype": "markdown",
        "markdown": map[string]string{
            "title": title,
            "text":  content,
        },
    }
    
    data, _ := json.Marshal(payload)
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
    // ... 处理响应
}

func (d *DingTalkNotifier) generateSign(timestamp int64) string {
    stringToSign := fmt.Sprintf("%d\n%s", timestamp, d.Secret)
    h := hmac.New(sha256.New, []byte(d.Secret))
    h.Write([]byte(stringToSign))
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
```

### 配置文件扩展

```yaml
# ~/.process-tracker/config.yaml
alerts:
  enabled: true
  rules:
    - name: high_cpu
      metric: cpu_percent
      threshold: 80
      duration: 300  # 5分钟
      webhook: "https://your-webhook-url"
      
    - name: high_memory
      metric: memory_mb
      threshold: 1024
      duration: 300
      webhook: "https://oapi.dingtalk.com/robot/send?access_token=xxx"
      
  notifiers:
    dingtalk:
      secret: "your-secret-key"
```

---

## 🖥️ TUI实现（3-5天）

### 快速入门示例

```go
// cmd/live.go
package cmd

import (
    "fmt"
    "time"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type model struct {
    processes []ProcessInfo
    sortBy    string // "cpu", "memory", "disk"
    width     int
    height    int
}

type tickMsg time.Time
type processUpdateMsg []ProcessInfo

func (m model) Init() tea.Cmd {
    return tea.Batch(
        tickCmd(),
        fetchProcesses,
    )
}

func tickCmd() tea.Cmd {
    return tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

func fetchProcesses() tea.Msg {
    // 调用core.App获取进程信息
    processes := getCurrentProcesses()
    return processUpdateMsg(processes)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "c":
            m.sortBy = "cpu"
        case "m":
            m.sortBy = "memory"
        case "d":
            m.sortBy = "disk"
        }
        
    case tickMsg:
        return m, tea.Batch(tickCmd(), fetchProcesses)
        
    case processUpdateMsg:
        m.processes = []ProcessInfo(msg)
        m.sortProcesses()
        
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    }
    
    return m, nil
}

func (m model) View() string {
    // 样式定义
    headerStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("39"))
    
    // 表头
    header := fmt.Sprintf("%-8s %-20s %8s %8s %8s\n",
        "PID", "NAME", "CPU%", "MEM", "STATUS")
    
    // 进程列表
    var rows string
    for i, p := range m.processes {
        if i >= 20 { // 只显示Top 20
            break
        }
        rows += fmt.Sprintf("%-8d %-20s %7.1f%% %8s %8s\n",
            p.Pid, p.Name, p.CPUPercent, 
            formatMemory(p.MemoryMB), "●")
    }
    
    // 帮助
    help := "\nPress 'c' (CPU) | 'm' (Memory) | 'd' (Disk) | 'q' (Quit)"
    
    return headerStyle.Render(header) + rows + help
}

func formatMemory(mb float64) string {
    if mb < 1024 {
        return fmt.Sprintf("%.0fMB", mb)
    }
    return fmt.Sprintf("%.1fGB", mb/1024)
}

// 启动TUI
func StartLiveMode(app *core.App) error {
    p := tea.NewProgram(
        model{sortBy: "cpu"},
        tea.WithAltScreen(),
    )
    _, err := p.Run()
    return err
}
```

### 集成到main.go

```go
case "live":
    if err := app.Initialize(); err != nil {
        log.Fatalf("初始化失败: %v", err)
    }
    
    if err := cmd.StartLiveMode(app.App); err != nil {
        log.Fatalf("Live模式失败: %v", err)
    }
```

---

## 🧪 测试策略

### 单元测试

```go
// core/alerting_test.go
func TestAlertEvaluation(t *testing.T) {
    rule := AlertRule{
        Name:      "test",
        Metric:    "cpu_percent",
        Threshold: 80,
        Duration:  5,
    }
    
    am := NewAlertManager([]AlertRule{rule})
    
    // 模拟超阈值记录
    records := []ResourceRecord{
        {CPUPercent: 85, Timestamp: time.Now()},
    }
    
    am.Evaluate(records)
    
    // 验证告警状态
    state := am.alertStates["test"]
    if state == nil {
        t.Fatal("应该创建告警状态")
    }
}
```

### 集成测试

```bash
#!/bin/bash
# test_integration.sh

# 启动进程监控（后台）
./process-tracker start &
PID=$!
sleep 5

# 测试Web接口
curl http://localhost:8080/api/stats/today | jq .

# 测试告警
# ... 模拟高负载

# 清理
kill $PID
```

---

## 📦 发布清单

### v0.4.0 发布内容

- [ ] 功能完整（Web + 告警 + TUI）
- [ ] 文档更新（README + Wiki）
- [ ] 跨平台编译（Linux/macOS/Windows）
- [ ] Docker镜像发布
- [ ] 发布说明（CHANGELOG.md）
- [ ] GitHub Release

### 编译脚本

```bash
#!/bin/bash
# build_release.sh

VERSION="0.4.0"

# Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-X main.Version=$VERSION" \
    -o releases/process-tracker-linux-amd64

# macOS
GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.Version=$VERSION" \
    -o releases/process-tracker-darwin-amd64

GOOS=darwin GOARCH=arm64 go build -ldflags="-X main.Version=$VERSION" \
    -o releases/process-tracker-darwin-arm64

# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-X main.Version=$VERSION" \
    -o releases/process-tracker-windows-amd64.exe

# 压缩
cd releases
for file in process-tracker-*; do
    tar -czf $file.tar.gz $file
done
```

---

## ⏱️ 时间规划参考

| 日期 | 任务 | 预计工时 |
|------|------|----------|
| **Week 1** | | |
| Day 1-3 | Web Dashboard原型 | 24h |
| Day 4-5 | 完善API和前端 | 16h |
| **Week 2** | | |
| Day 1-3 | 告警系统核心 | 20h |
| Day 4-5 | 钉钉/Webhook集成 | 12h |
| **Week 3** | | |
| Day 1-3 | TUI基础实现 | 20h |
| Day 4-5 | 集成测试和优化 | 12h |
| **Week 4** | | |
| Day 1-3 | 文档和Docker | 16h |
| Day 4-5 | 发布准备 | 8h |

**总计**: 约128小时（3-4周全职工作）

---

## 💡 实用技巧

### 快速原型技巧

1. **使用在线工具生成HTML模板**
   - [Tailwind UI](https://tailwindui.com/components) - 现成组件
   - [Bootstrap Examples](https://getbootstrap.com/docs/5.3/examples/) - 布局示例

2. **Chart.js快速配置**
   - [Chart.js Samples](https://www.chartjs.org/docs/latest/samples/) - 直接复制

3. **bubbletea学习路径**
   - 官方Tutorial（1-2小时）
   - 示例程序（examples目录）
   - 参考项目：[gh](https://github.com/cli/cli)

### 调试技巧

```go
// 添加调试模式
var Debug = flag.Bool("debug", false, "Enable debug mode")

func debugLog(format string, args ...interface{}) {
    if *Debug {
        log.Printf("[DEBUG] "+format, args...)
    }
}
```

### 性能优化技巧

1. **避免全文件读取**
```go
// 从文件末尾读取最后N行
func ReadLastLines(filename string, n int) ([]string, error) {
    f, _ := os.Open(filename)
    defer f.Close()
    
    stat, _ := f.Stat()
    size := stat.Size()
    
    // 从末尾开始读取
    buf := make([]byte, 8192)
    f.Seek(-int64(len(buf)), io.SeekEnd)
    // ... 解析行
}
```

2. **使用缓存**
```go
type StatsCache struct {
    data      DashboardStats
    expiry    time.Time
    mu        sync.RWMutex
}

func (c *StatsCache) Get() (DashboardStats, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    if time.Now().Before(c.expiry) {
        return c.data, true
    }
    return DashboardStats{}, false
}
```

---

## 🎓 学习资源

### Go Web开发
- [Let's Go](https://lets-go.alexedwards.net/) - 实用教程
- [Go Web Examples](https://gowebexamples.com/) - 代码片段

### TUI开发
- [Bubble Tea Tutorial](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- [Awesome TUIs](https://github.com/rothgar/awesome-tuis) - 优秀案例

### 前端可视化
- [Chart.js Getting Started](https://www.chartjs.org/docs/latest/getting-started/)
- [Tailwind CSS Docs](https://tailwindcss.com/docs)

---

**准备好了吗？** 从Web Dashboard原型开始，3天后见到第一个可用版本！
