# Process Tracker 功能详解

## 📊 CPU归一化显示

### 什么是CPU归一化？

**传统显示问题**：
- 多核系统中，进程CPU可以超过100%
- 例如：72核系统，8核满载显示800%
- 用户难以判断系统整体负载

**归一化解决方案**：
```
归一化CPU = 原始CPU ÷ 系统总核心数

例如在72核系统：
- 1核满载 = 100% → 1.39%
- 8核满载 = 800% → 11.11%
- 36核满载 = 3600% → 50%
- 72核满载 = 7200% → 100%
```

### 实现细节

**后端计算**：
```go
// core/app.go
func calculateCPUPercentNormalized(cpuPercent float64) float64 {
    totalCores := getTotalCPUCores()  // 获取系统核心数
    return cpuPercent / float64(totalCores)
}
```

**数据存储**：
- v7格式：18字段（包含CPUPercentNormalized）
- 向后兼容v6（17字段）和v5（16字段）

**前端显示**：
- Dashboard：显示归一化CPU (0-100%)
- 趋势图：Y轴0-100%，带百分号刻度
- Tooltip：显示精确到2位小数

### 使用场景

**系统负载监控**：
```
平均CPU (归一化): 2.5%
解读：系统CPU使用率2.5%，资源充足

平均CPU (归一化): 85%
解读：系统CPU使用率85%，接近满载
```

**进程资源占用**：
```
chrome: 1.39% CPU
解读：占用约1个核心（100/72 ≈ 1.39%）

python: 11.11% CPU  
解读：占用约8个核心（800/72 ≈ 11.11%）
```

---

## 💾 内存百分比显示

### 为什么需要百分比？

**传统显示问题**：
- 只显示绝对值：2048 MB
- 用户需要心算：2048 ÷ 32768 ≈ 6.25%
- 无法快速判断内存压力

**百分比解决方案**：
```
内存百分比 = (进程内存 ÷ 系统总内存) × 100%

显示格式：2048 MB (6.25%)
```

### 实现细节

**后端计算**：
```go
// cmd/web.go
func calculateStats(...) DashboardStats {
    totalMemPercent := (totalMem / totalMemoryMB) * 100
    maxMemPercent := (maxMem / totalMemoryMB) * 100
    
    return DashboardStats{
        TotalMemory:        totalMem,
        TotalMemoryPercent: totalMemPercent,  // 新增
        MaxMemory:          maxMem,
        MaxMemoryPercent:   maxMemPercent,    // 新增
    }
}
```

**前端显示**：
```javascript
// cmd/static/js/app.js
const totalMemStr = `${formatMemory(total)} (${percent.toFixed(1)}%)`;
```

### 内存压力判断

| 百分比 | 状态 | 建议 |
|-------|------|------|
| 0-60% | 充足 🟢 | 无需操作 |
| 60-80% | 适中 🟡 | 关注高内存进程 |
| 80-95% | 紧张 🟠 | 考虑优化或扩容 |
| >95% | 严重 🔴 | 立即释放内存 |

---

## 🎛️ 守护进程管理

### 进程生命周期

```bash
# 启动
./process-tracker start --web
→ 后台运行，写入PID文件

# 查看状态
./process-tracker status
→ 读取PID，检查进程存在

# 停止
./process-tracker stop
→ 发送SIGTERM，等待退出（最多5秒）

# 重启
./process-tracker restart --web
→ stop + start
```

### PID文件管理

**位置**：`~/.process-tracker/process-tracker.pid`

**内容**：
```
12345
```

**作用**：
- 防止重复启动
- 进程状态查询
- 优雅关闭

### 信号处理

**SIGTERM处理**：
```go
// cmd/commands.go
func StartMonitoring() error {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    
    // 等待信号
    <-sigCh
    
    // 停止监控循环
    close(stopCh)
    
    // 清理资源
    app.CloseFile()
    
    return nil
}
```

**超时保护**：
```go
// main.go
func handleStop(daemon *DaemonManager) {
    daemon.Stop()  // 发送SIGTERM
    
    // 轮询检查（最多5秒）
    for elapsed := 0; elapsed < 5s; elapsed += 100ms {
        if !daemon.IsRunning() {
            return  // 成功退出
        }
    }
    
    // 超时提示
    fmt.Println("进程未在5秒内停止，可能需要强制终止")
    fmt.Printf("使用: kill -9 %d\n", pid)
}
```

---

## 🐳 Docker容器监控

### 自动发现

**检测逻辑**：
```go
// core/docker.go
func NewDockerMonitor(config Config) (*DockerMonitor, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return nil, err  // Docker不可用
    }
    
    // 测试连接
    _, err = cli.Ping(context.Background())
    if err != nil {
        return nil, err  // Docker服务未运行
    }
    
    return &DockerMonitor{client: cli}, nil
}
```

### 监控指标

**容器级统计**：
- CPU使用率（归一化）
- 内存使用（MB + 百分比）
- 磁盘I/O（读写MB）
- 网络流量（发送/接收KB）
- 容器状态（运行中/已停止）

**数据收集**：
```go
// 每10秒收集一次
stats, err := cli.ContainerStats(ctx, containerID, false)

// 解析JSON数据
var v *types.StatsJSON
json.NewDecoder(stats.Body).Decode(&v)

// 计算资源使用
cpuPercent := calculateContainerCPU(v)
memoryUsage := v.MemoryStats.Usage
networkRx := v.Networks["eth0"].RxBytes
```

### Web界面显示

**进程名格式**：
```
docker:nginx-proxy
docker:mysql-db
docker:redis-cache
```

**分类过滤**：
- 点击"🐳 Docker"按钮
- 只显示docker:开头的容器
- 实时更新容器状态

---

## 🔍 进程搜索和过滤

### 搜索功能

**实时搜索**：
```javascript
// 搜索进程名和命令
document.getElementById('process-search').addEventListener('input', (e) => {
    const query = e.target.value.toLowerCase();
    filterProcesses(query, currentCategory);
});

// 匹配逻辑
function matchesSearch(process, query) {
    return process.name.toLowerCase().includes(query) ||
           process.command.toLowerCase().includes(query);
}
```

**搜索示例**：
- 输入"chrome" → 显示所有Chrome相关进程
- 输入"python" → 显示所有Python脚本
- 输入"docker" → 显示所有Docker容器

### 分类过滤

**预定义分类**：
- **全部进程**: 显示所有监控的进程
- **🐳 Docker**: docker:开头的容器
- **💻 开发工具**: java, node, python, go等
- **🌐 浏览器**: chrome, firefox, safari等

**智能分类**：
```go
// core/types.go
func IdentifyApplication(name, cmdline string, enableSmart bool) string {
    if strings.HasPrefix(name, "docker:") {
        return "docker"
    }
    
    switch {
    case strings.Contains(cmdline, "/go/"):
        return "development"
    case strings.Contains(name, "chrome"):
        return "browser"
    // ... 更多规则
    }
}
```

### 排序功能

**支持的排序**：
- PID：进程ID
- 进程名：字母顺序
- CPU：归一化CPU使用率
- 内存：内存使用量（MB）
- 状态：活跃/空闲

**排序保持**：
- 用户点击排序后
- 自动刷新保持排序状态
- 不会被自动刷新打断

---

## 🌐 网络IP自动发现

### 问题背景

**旧方式**：
```
Web服务器启动: http://0.0.0.0:18080
```
- 用户无法直接访问0.0.0.0
- 需要手动查找内网IP

**新方式**：
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🌐 Web服务器已启动，可通过以下地址访问：
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  http://192.168.1.102:18080
  http://192.168.1.105:18080
  http://100.65.159.9:18080
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### 实现原理

```go
// cmd/web.go
func getLocalIPs() []string {
    interfaces, _ := net.Interfaces()
    var ips []string
    
    for _, iface := range interfaces {
        // 跳过down和loopback接口
        if iface.Flags&net.FlagUp == 0 || 
           iface.Flags&net.FlagLoopback != 0 {
            continue
        }
        
        addrs, _ := iface.Addrs()
        for _, addr := range addrs {
            if ip := extractIPv4(addr); ip != nil {
                ips = append(ips, ip.String())
            }
        }
    }
    
    return ips
}
```

### 多网卡支持

**自动检测**：
- 以太网（eth0）
- WiFi（wlan0）
- VPN（tun0）
- Docker网桥（docker0）
- 虚拟网卡（veth*）

**过滤逻辑**：
- ✅ 只显示IPv4地址
- ✅ 跳过127.0.0.1（loopback）
- ✅ 跳过未启动的网卡
- ✅ 包含所有活跃的内网IP

---

## 💾 数据格式版本管理

### 格式演进

| 版本 | 字段数 | 新增字段 | 发布时间 |
|------|--------|---------|---------|
| v5 | 16 | 基础版本 | v0.3.7 |
| v6 | 17 | MemoryPercent | v0.3.8 |
| v7 | 18 | CPUPercentNormalized | v0.3.9 |

### 向后兼容

**读取逻辑**：
```go
// core/storage.go
func parseRecord(line string) (ResourceRecord, error) {
    fields := strings.Split(line, ",")
    
    switch len(fields) {
    case 18:  // v7
        record.CPUPercentNormalized = parseFloat(fields[3])
        record.MemoryPercent = parseFloat(fields[5])
    case 17:  // v6
        record.CPUPercentNormalized = 0  // 不存在
        record.MemoryPercent = parseFloat(fields[4])
    case 16:  // v5
        record.CPUPercentNormalized = 0
        record.MemoryPercent = 0
    }
    
    return record, nil
}
```

**写入格式**：
- 始终使用最新格式（v7）
- 包含所有字段
- 逗号分隔

**迁移策略**：
- 无需手动迁移
- 旧数据自动识别版本
- 新数据使用新格式
- 混合数据正常工作

---

## 📈 性能优化

### 批量写入

**缓冲机制**：
```go
// core/storage_manager.go
const bufferSize = 100

func (m *Manager) SaveRecords(records []ResourceRecord) error {
    m.buffer = append(m.buffer, records...)
    
    if len(m.buffer) >= bufferSize {
        return m.Flush()
    }
    
    return nil
}
```

**效果**：
- 减少I/O次数：从每5秒1次 → 每8分钟1次
- 降低磁盘负载：90%
- 数据完整性：定期自动刷新

### 缓存机制

**系统信息缓存**：
```go
var (
    cachedTotalMemoryMB float64  // 缓存系统总内存
    cachedTotalCPUCores int      // 缓存CPU核心数
)

func getTotalMemoryMB() float64 {
    if cachedTotalMemoryMB > 0 {
        return cachedTotalMemoryMB  // 返回缓存
    }
    
    // 首次查询系统
    v, _ := mem.VirtualMemory()
    cachedTotalMemoryMB = float64(v.Total) / 1024 / 1024
    return cachedTotalMemoryMB
}
```

**Web缓存**：
```javascript
// 5秒TTL缓存
class StatsCache {
    constructor(ttl) {
        this.cache = {};
        this.ttl = ttl;
    }
    
    get(key) {
        const item = this.cache[key];
        if (item && Date.now() - item.timestamp < this.ttl) {
            return item.data;  // 返回缓存
        }
        return null;
    }
}
```

### 客户端过滤

**搜索和分类在前端处理**：
- ✅ 无服务器负载
- ✅ 即时响应
- ✅ 减少API调用
- ✅ 降低带宽

**实现**：
```javascript
function filterProcesses(query, category) {
    let filtered = allProcesses;  // 使用客户端数据
    
    // 文本搜索
    if (query) {
        filtered = filtered.filter(p => matchesSearch(p, query));
    }
    
    // 分类过滤
    if (category !== 'all') {
        filtered = filtered.filter(p => matchesCategory(p, category));
    }
    
    updateTable(filtered);  // 直接更新DOM
}
```

---

## 🎯 使用建议

### CPU监控
- 关注归一化CPU（0-100%）
- >80%考虑扩容或优化
- 使用趋势图观察模式

### 内存监控
- 关注百分比而非绝对值
- >80%需要释放内存
- 定位内存泄漏进程

### Docker监控
- 使用分类过滤快速定位
- 观察容器资源占用
- 对比容器间资源分配

### 进程管理
- 使用守护进程避免手动管理
- 定期检查status
- 配置自动启动（systemd）

**充分利用这些功能，让监控更高效！** 🚀
