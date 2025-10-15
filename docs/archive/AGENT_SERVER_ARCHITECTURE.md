# Agent-Server 架构详细设计方案

## 📋 概述

Agent-Server 架构是 Process Tracker 项目的**长期演进方向**，旨在将单机监控工具扩展为**多服务器集中管理平台**。这是一种经典的分布式监控架构模式，被 Prometheus、Telegraf、Datadog 等主流监控系统广泛采用。

## 🎯 核心价值

### 解决的问题
1. **多服务器管理困境**: 当前需要逐个登录服务器查看监控数据
2. **数据分散**: 每台服务器的数据孤立，难以做横向对比
3. **运维效率低**: 无法统一配置、统一告警、统一展示
4. **扩展性受限**: 单机模式难以适应集群环境

### 带来的收益
✅ **统一管理**: 一个界面管理所有服务器  
✅ **集中存储**: 所有数据汇聚到中心节点  
✅ **横向对比**: 轻松对比多台服务器的资源使用  
✅ **集中告警**: 统一配置告警规则  
✅ **服务发现**: 自动发现和注册新节点  

## 🏗️ 架构设计

### 整体架构图

```
                    ┌─────────────────────────────────┐
                    │       Web Dashboard             │
                    │   (浏览器访问，可视化界面)        │
                    └────────────────┬────────────────┘
                                     │ HTTP/WebSocket
                    ┌────────────────▼────────────────┐
                    │         Hub Server              │
                    │      (中心管理节点)              │
                    │                                 │
                    │  ┌──────────────────────────┐  │
                    │  │ API Gateway              │  │
                    │  │ - REST API               │  │
                    │  │ - WebSocket (实时推送)    │  │
                    │  └──────────────────────────┘  │
                    │                                 │
                    │  ┌──────────────────────────┐  │
                    │  │ Data Aggregator          │  │
                    │  │ - 数据接收和聚合          │  │
                    │  │ - 时间序列存储            │  │
                    │  └──────────────────────────┘  │
                    │                                 │
                    │  ┌──────────────────────────┐  │
                    │  │ Alert Engine             │  │
                    │  │ - 规则评估               │  │
                    │  │ - 告警发送               │  │
                    │  └──────────────────────────┘  │
                    │                                 │
                    │  ┌──────────────────────────┐  │
                    │  │ Service Registry         │  │
                    │  │ - Agent 注册和心跳        │  │
                    │  │ - 健康检查               │  │
                    │  └──────────────────────────┘  │
                    └────────────────┬────────────────┘
                                     │ gRPC/HTTP
                    ┌────────────────┴────────────────┐
                    │                                  │
         ┌──────────▼──────────┐         ┌───────────▼──────────┐
         │   Agent (Server1)   │         │   Agent (Server2)    │
         │                     │         │                      │
         │ ┌─────────────────┐ │   ...   │ ┌─────────────────┐  │
         │ │ Data Collector  │ │         │ │ Data Collector  │  │
         │ │ - 进程监控       │ │         │ │ - 进程监控       │  │
         │ │ - 系统指标       │ │         │ │ - 系统指标       │  │
         │ │ - Docker监控     │ │         │ │ - Docker监控     │  │
         │ └─────────────────┘ │         │ └─────────────────┘  │
         │                     │         │                      │
         │ ┌─────────────────┐ │         │ ┌─────────────────┐  │
         │ │ Data Reporter   │ │         │ │ Data Reporter   │  │
         │ │ - 批量上报       │ │         │ │ - 批量上报       │  │
         │ │ - 断线重连       │ │         │ │ - 断线重连       │  │
         │ │ - 本地缓存       │ │         │ │ - 本地缓存       │  │
         │ └─────────────────┘ │         │ └─────────────────┘  │
         └─────────────────────┘         └──────────────────────┘
              生产服务器                        开发服务器
```

## 🔧 核心组件详解

### 1. Agent (数据采集器)

**职责**: 部署在每台被监控服务器上，负责数据采集和上报

**核心功能**:
```go
type Agent struct {
    config       AgentConfig
    collector    *DataCollector    // 数据采集
    reporter     *DataReporter     // 数据上报
    localCache   *LocalCache       // 本地缓存
    heartbeat    *HeartbeatManager // 心跳管理
}

type AgentConfig struct {
    ServerName      string        // 服务器标识名称
    HubURL          string        // Hub服务器地址
    ReportInterval  time.Duration // 上报间隔 (默认10秒)
    BufferSize      int           // 缓冲大小
    EnableLocalLog  bool          // 启用本地日志备份
    LocalLogPath    string        // 本地日志路径
    
    // 采集配置
    EnableDocker    bool          // Docker监控
    EnableNetwork   bool          // 网络监控
    SampleInterval  time.Duration // 采样间隔
}
```

**关键特性**:
- ✅ **轻量级**: 资源占用 < 20MB (复用现有core包)
- ✅ **本地缓存**: 断线时缓存数据，恢复后补发
- ✅ **压缩传输**: gzip压缩减少带宽占用
- ✅ **断线重连**: 自动重连机制，支持指数退避
- ✅ **本地备份**: 可选的本地CSV备份

**数据上报协议**:
```go
// Agent -> Hub 数据上报
type MetricReport struct {
    ServerName  string              `json:"server_name"`
    Timestamp   time.Time           `json:"timestamp"`
    Metrics     []ResourceRecord    `json:"metrics"`
    SystemInfo  SystemInfo          `json:"system_info"`
}

type SystemInfo struct {
    Hostname    string  `json:"hostname"`
    OS          string  `json:"os"`
    Arch        string  `json:"arch"`
    CPUCores    int     `json:"cpu_cores"`
    TotalMemMB  float64 `json:"total_mem_mb"`
    AgentVersion string `json:"agent_version"`
}
```

---

### 2. Hub Server (中心管理节点)

**职责**: 接收所有Agent数据，提供统一管理和查询接口

**核心模块**:

#### 2.1 API Gateway (API网关)
```go
type APIGateway struct {
    router *gin.Engine  // 使用Gin框架
}

// REST API 路由
func (g *APIGateway) SetupRoutes() {
    // Agent数据上报
    g.router.POST("/api/v1/report", g.handleReport)
    
    // Agent注册
    g.router.POST("/api/v1/register", g.handleRegister)
    
    // Agent心跳
    g.router.POST("/api/v1/heartbeat", g.handleHeartbeat)
    
    // 查询接口
    g.router.GET("/api/v1/servers", g.listServers)
    g.router.GET("/api/v1/servers/:name/stats", g.getServerStats)
    g.router.GET("/api/v1/stats/compare", g.compareServers)
    
    // 告警配置
    g.router.GET("/api/v1/alerts", g.listAlerts)
    g.router.POST("/api/v1/alerts", g.createAlert)
    
    // WebSocket 实时推送
    g.router.GET("/ws", g.handleWebSocket)
}
```

#### 2.2 Data Aggregator (数据聚合器)
```go
type DataAggregator struct {
    storage TimeSeriesDB  // 时间序列数据库
    cache   *StatsCache   // 统计缓存
}

// 存储选型
type TimeSeriesDB interface {
    Write(serverName string, metrics []ResourceRecord) error
    Query(query TimeSeriesQuery) ([]ResourceRecord, error)
    Aggregate(query AggregateQuery) (AggregateResult, error)
}

// 支持的存储后端
// - SQLite (默认，单机部署)
// - PostgreSQL + TimescaleDB (高性能)
// - InfluxDB (专业时序数据库)
// - ClickHouse (大规模部署)
```

**数据聚合示例**:
```go
// 横向对比多台服务器
type CompareQuery struct {
    ServerNames []string
    TimeRange   TimeRange
    Metric      string  // "cpu", "memory", "disk"
}

// 返回结果
type CompareResult struct {
    Servers map[string]ServerStats
    Chart   ChartData  // 前端图表数据
}
```

#### 2.3 Alert Engine (告警引擎)
```go
type AlertEngine struct {
    rules      []AlertRule
    evaluator  *RuleEvaluator
    notifier   *AlertNotifier
}

type AlertRule struct {
    ID          string
    Name        string
    ServerNames []string  // 应用于哪些服务器
    Condition   AlertCondition
    Actions     []AlertAction
}

type AlertCondition struct {
    Metric      string   // cpu_percent, memory_mb
    Operator    string   // >, <, ==, !=
    Threshold   float64
    Duration    time.Duration  // 持续时间
}

// 告警示例
/*
规则: 任何服务器CPU > 80% 持续5分钟 -> 发送钉钉通知
{
    "name": "高CPU告警",
    "server_names": ["*"],  // * 表示所有服务器
    "condition": {
        "metric": "cpu_percent",
        "operator": ">",
        "threshold": 80,
        "duration": "5m"
    },
    "actions": [
        {
            "type": "webhook",
            "url": "https://oapi.dingtalk.com/robot/send?access_token=EXAMPLE_TOKEN_REPLACE_ME"
        }
    ]
}
*/
```

#### 2.4 Service Registry (服务注册中心)
```go
type ServiceRegistry struct {
    agents      sync.Map  // map[serverName]*AgentInfo
    healthCheck *HealthChecker
}

type AgentInfo struct {
    ServerName   string
    HubURL       string
    Version      string
    RegisterTime time.Time
    LastHeartbeat time.Time
    Status       AgentStatus  // online, offline, degraded
    SystemInfo   SystemInfo
}

// 健康检查
type HealthChecker struct {
    interval time.Duration
}

func (h *HealthChecker) Check() {
    // 检查Agent心跳超时 (默认30秒)
    // 更新Agent状态
    // 发送Agent下线告警
}
```

---

### 3. Web Dashboard (可视化界面)

**职责**: 提供统一的Web界面，展示所有服务器监控数据

**核心功能**:

#### 3.1 服务器总览页面
```
┌────────────────────────────────────────────────────────────┐
│  Process Tracker - Multi-Server Dashboard          [🔔] [⚙] │
├────────────────────────────────────────────────────────────┤
│                                                              │
│  📊 集群概览                                                 │
│  ┌──────────┬──────────┬──────────┬──────────┐             │
│  │ 总服务器  │ 在线      │ 离线      │ 总进程    │             │
│  │   12     │   10     │    2     │  1,247   │             │
│  └──────────┴──────────┴──────────┴──────────┘             │
│                                                              │
│  🖥️  服务器列表                                              │
│  ┌────────────┬────────┬─────────┬─────────┬──────────┐    │
│  │ 服务器名称  │ 状态   │ CPU     │ 内存    │ 最后更新  │    │
│  ├────────────┼────────┼─────────┼─────────┼──────────┤    │
│  │ prod-web-01│ 🟢在线 │ 45%     │ 2.3G    │ 2秒前    │    │
│  │ prod-web-02│ 🟢在线 │ 38%     │ 1.8G    │ 3秒前    │    │
│  │ prod-db-01 │ 🟢在线 │ 72%     │ 15.2G   │ 5秒前    │    │
│  │ dev-01     │ 🔴离线 │ -       │ -       │ 5分钟前  │    │
│  └────────────┴────────┴─────────┴─────────┴──────────┘    │
│                                                              │
│  📈 资源使用趋势 (最近24小时)                                 │
│  [═══════════════多服务器对比图表═══════════════]            │
│                                                              │
└────────────────────────────────────────────────────────────┘
```

#### 3.2 服务器详情页面
```
┌────────────────────────────────────────────────────────────┐
│  ← 返回列表    prod-web-01 详情                      [🔔] [⚙] │
├────────────────────────────────────────────────────────────┤
│                                                              │
│  ℹ️  系统信息                                                │
│  主机名: prod-web-01.example.com                            │
│  系统: Linux Ubuntu 22.04 (x86_64)                          │
│  CPU: 8核 Intel Xeon   内存: 32GB                          │
│  Agent版本: v0.4.0     运行时长: 15天3小时                  │
│                                                              │
│  📊 实时指标                                                 │
│  ┌──────────┬──────────┬──────────┬──────────┐             │
│  │ CPU      │ 内存      │ 磁盘I/O  │ 网络     │             │
│  │ 45%      │ 2.3G/32G │ 15MB/s   │ 8Mbps    │             │
│  └──────────┴──────────┴──────────┴──────────┘             │
│                                                              │
│  📈 资源趋势图 (可切换: 1h/6h/24h/7d/30d)                   │
│  [═══════════════交互式图表═══════════════]                 │
│                                                              │
│  📋 进程列表 (Top 10)                                        │
│  ┌────┬──────────────┬─────┬──────┬────────┐               │
│  │PID │ 进程名        │ CPU │ 内存 │ 运行时间│               │
│  ├────┼──────────────┼─────┼──────┼────────┤               │
│  │... │ ...          │ ... │ ...  │ ...    │               │
│  └────┴──────────────┴─────┴──────┴────────┘               │
│                                                              │
└────────────────────────────────────────────────────────────┘
```

#### 3.3 对比分析页面
```
┌────────────────────────────────────────────────────────────┐
│  服务器对比分析                                              │
├────────────────────────────────────────────────────────────┤
│                                                              │
│  选择服务器:                                                 │
│  ☑ prod-web-01   ☑ prod-web-02   ☑ prod-db-01              │
│                                                              │
│  时间范围: [最近24小时 ▼]   指标: [CPU使用率 ▼]            │
│                                                              │
│  📊 对比图表                                                 │
│  ┌────────────────────────────────────────────────────┐    │
│  │  100% ┤                                            │    │
│  │       ┤     ╭─prod-web-01                          │    │
│  │   80% ┤    ╱ ╰─prod-web-02                         │    │
│  │       ┤   ╱   ╭──prod-db-01                        │    │
│  │   60% ┤  ╱   ╱                                     │    │
│  │       ┤ ╱   ╱                                      │    │
│  │   40% ┤╱   ╱                                       │    │
│  │       ┤   ╱                                        │    │
│  │   20% ┤  ╱                                         │    │
│  │    0% └──────────────────────────────────────────  │    │
│  │       0h    6h    12h   18h   24h                  │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
│  📈 统计对比                                                 │
│  ┌───────────┬──────────┬──────────┬──────────┐            │
│  │ 服务器    │ 平均CPU  │ 峰值CPU  │ 平均内存 │            │
│  ├───────────┼──────────┼──────────┼──────────┤            │
│  │prod-web-01│   45%    │   72%    │  2.3G    │            │
│  │prod-web-02│   38%    │   65%    │  1.8G    │            │
│  │prod-db-01 │   68%    │   92%    │  14.5G   │            │
│  └───────────┴──────────┴──────────┴──────────┘            │
│                                                              │
└────────────────────────────────────────────────────────────┘
```

---

## 🚀 实施路线图

### Phase 1: 基础架构 (2-3周)

**目标**: 建立Agent-Hub基本通信

#### Week 1: Agent端开发
- [ ] 创建 `cmd/agent` 包
- [ ] 实现数据采集器 (复用core包)
- [ ] 实现数据上报器 (HTTP/gRPC)
- [ ] 实现本地缓存和断线重连
- [ ] 添加配置文件支持

#### Week 2: Hub端开发
- [ ] 创建 `cmd/hub` 包
- [ ] 实现API Gateway (使用Gin)
- [ ] 实现数据接收端点
- [ ] 实现SQLite存储 (默认)
- [ ] 实现服务注册中心

#### Week 3: 基础Web界面
- [ ] 服务器列表页
- [ ] 基础统计展示
- [ ] 实时数据刷新

**交付成果**:
- ✅ Agent可以上报数据到Hub
- ✅ Hub可以存储和查询数据
- ✅ Web界面可以查看多服务器数据

---

### Phase 2: 功能增强 (3-4周)

#### Week 4-5: 可视化和对比
- [ ] 集成Chart.js图表库
- [ ] 实现时间序列图表
- [ ] 实现多服务器对比功能
- [ ] 添加数据导出功能

#### Week 6: 告警系统
- [ ] 实现告警规则引擎
- [ ] 支持Webhook通知
- [ ] 支持钉钉/企业微信/Slack
- [ ] 告警历史记录

#### Week 7: 高级特性
- [ ] WebSocket实时推送
- [ ] 服务自动发现
- [ ] Agent健康检查
- [ ] 配置热更新

**交付成果**:
- ✅ 完整的可视化界面
- ✅ 告警系统上线
- ✅ 实时数据推送

---

### Phase 3: 生产优化 (2-3周)

#### Week 8-9: 性能和稳定性
- [ ] 性能测试和优化
- [ ] 支持高可用部署 (Hub集群)
- [ ] 数据备份和恢复
- [ ] 监控指标优化

#### Week 10: 企业特性
- [ ] 用户认证和权限
- [ ] 多租户支持
- [ ] 审计日志
- [ ] API文档 (Swagger)

**交付成果**:
- ✅ 生产级稳定性
- ✅ 企业级特性
- ✅ 完整文档

---

## 📊 技术选型

### 通信协议
| 协议 | 使用场景 | 优势 | 劣势 |
|------|---------|------|------|
| **HTTP/JSON** | Agent数据上报 | 简单、易调试 | 性能一般 |
| **gRPC** | 高频数据传输 | 高性能、类型安全 | 复杂度高 |
| **WebSocket** | 实时推送 | 双向通信 | 连接管理复杂 |

**推荐方案**: 
- Agent -> Hub: HTTP/JSON (简单优先)
- 实时推送: WebSocket
- 后期优化: 可升级为gRPC

### 数据存储
| 存储 | 使用场景 | 优势 | 劣势 |
|------|---------|------|------|
| **SQLite** | 单机/小规模 | 零配置、轻量 | 并发受限 |
| **PostgreSQL + TimescaleDB** | 中等规模 | 强大查询、时序优化 | 需要额外部署 |
| **InfluxDB** | 大规模时序数据 | 专业时序DB | 学习成本 |
| **ClickHouse** | 海量数据分析 | 超高性能 | 复杂度高 |

**推荐方案**:
- 默认: SQLite (开箱即用)
- 可选: PostgreSQL (企业部署)
- 高级: InfluxDB (专业场景)

### Web框架
| 框架 | 优势 | 劣势 |
|------|------|------|
| **Gin** | 高性能、生态好 | 稍重 |
| **Fiber** | 极致性能 | 生态较小 |
| **标准库** | 零依赖 | 开发效率低 |

**推荐**: Gin (性能与易用的平衡)

---

## 🔐 安全考虑

### 1. 认证授权
```go
// Agent认证 (基于Token)
type AgentAuth struct {
    Token     string    // Agent密钥
    ServerName string   // 服务器标识
}

// Web认证 (基于JWT)
type WebAuth struct {
    Username string
    Role     string  // admin, viewer
}
```

### 2. 数据加密
- Agent -> Hub: HTTPS/TLS加密
- 敏感配置: 环境变量或加密存储
- 数据库: 可选加密存储

### 3. 访问控制
```yaml
# Hub配置
security:
  enable_auth: true
  jwt_secret: "EXAMPLE_SECRET_REPLACE_ME"
  
  # Agent认证
  agent_tokens:
    - token: "agent-token-1"
      server_names: ["prod-*"]  # 允许的服务器名称模式
    - token: "agent-token-2"
      server_names: ["dev-*"]
  
  # Web用户
  users:
    - username: admin
      password_hash: "..."
      role: admin
    - username: viewer
      password_hash: "..."
      role: viewer
```

---

## 📈 部署方案

### 单Hub部署 (小规模)
```
       ┌─────────┐
       │   Hub   │  (单节点)
       └────┬────┘
            │
    ┌───────┼───────┐
    │       │       │
 ┌──▼──┐ ┌─▼──┐ ┌─▼──┐
 │Agent│ │Agent│ │Agent│
 └─────┘ └────┘ └────┘
  (1-20台服务器)
```

### 高可用部署 (中规模)
```
     ┌─────────┐     ┌─────────┐
     │  Hub 1  │────▶│  Hub 2  │  (主备模式)
     └────┬────┘     └────┬────┘
          │               │
          └───────┬───────┘
                  │
       ┌──────────┼──────────┐
       │          │          │
    ┌──▼──┐    ┌─▼──┐    ┌─▼──┐
    │Agent│    │Agent│    │Agent│
    └─────┘    └────┘    └────┘
     (20-100台服务器)
```

### 分布式部署 (大规模)
```
              ┌─────────────┐
              │  Hub Master │  (管理中心)
              └──────┬──────┘
                     │
         ┌───────────┼───────────┐
         │           │           │
    ┌────▼────┐ ┌───▼────┐ ┌───▼────┐
    │Regional │ │Regional│ │Regional│  (区域Hub)
    │ Hub 1   │ │ Hub 2  │ │ Hub 3  │
    └────┬────┘ └───┬────┘ └───┬────┘
         │          │          │
      (Agents)   (Agents)   (Agents)
       (100+台服务器)
```

---

## 💰 成本分析

### 资源占用估算

**Hub Server** (管理100台服务器):
- CPU: 2核
- 内存: 2GB
- 存储: 100GB (保留30天数据)
- 带宽: 10Mbps

**Agent** (每台):
- CPU: 0.1核
- 内存: 20MB
- 存储: 1GB (本地缓存)
- 带宽: 100KB/s

### 总体成本 (100台服务器场景)
- Hub服务器: 1台 (约$50/月)
- Agent额外开销: 可忽略 (~$0)
- **总成本**: 约$50/月 (vs Datadog ~$1500/月)

---

## 🎯 与竞品对比

| 特性 | Process Tracker | Prometheus | Netdata | Datadog |
|------|----------------|-----------|---------|---------|
| 部署复杂度 | ⭐ 简单 | ⭐⭐⭐ 中等 | ⭐⭐ 较简单 | ⭐⭐⭐⭐ 复杂 |
| 学习曲线 | ⭐ 平缓 | ⭐⭐⭐ 陡峭 | ⭐⭐ 中等 | ⭐⭐⭐ 陡峭 |
| 进程级监控 | ✅ 内置 | ❌ 需Exporter | ✅ 内置 | ✅ 内置 |
| 历史数据 | ✅ 长期 | ✅ 长期 | ✅ 短期 | ✅ 长期 |
| 成本 | 💰 免费 | 💰 免费 | 💰 免费 | 💰💰💰 昂贵 |
| 适用规模 | 1-500台 | 100-10000台 | 1-100台 | 10-10000台 |

**差异化优势**:
- ✅ 专注进程级监控 (vs Prometheus的指标监控)
- ✅ 开箱即用 (vs Prometheus复杂配置)
- ✅ 轻量级 (vs Datadog重量级)
- ✅ 免费开源 (vs Datadog付费)

---

## 🚧 实施建议

### 何时采用Agent-Server架构？

✅ **适合的场景**:
- 管理3台以上服务器
- 需要集中查看和对比
- 需要统一告警
- 团队协作需求

❌ **不适合的场景**:
- 单机使用 (继续用单机模式)
- 临时监控需求
- 资源极度受限

### 渐进式迁移方案

**第一步**: 继续优化单机版本 (Phase 1-2)
- Web Dashboard
- 实时TUI
- 告警系统

**第二步**: 小规模试点 (3-5台服务器)
- 部署Agent-Hub架构
- 验证稳定性和性能
- 收集用户反馈

**第三步**: 全面推广
- 优化性能和稳定性
- 增加企业特性
- 完善文档和工具

---

## 📚 参考资料

### 类似项目
- **Prometheus + Node Exporter**: 指标监控的事实标准
- **Telegraf + InfluxDB**: 轻量级Agent架构
- **Netdata**: 实时监控和可视化
- **Elastic APM**: 应用性能监控

### 技术文档
- [Prometheus Architecture](https://prometheus.io/docs/introduction/overview/)
- [Telegraf Agent](https://docs.influxdata.com/telegraf/)
- [gRPC Go Tutorial](https://grpc.io/docs/languages/go/)
- [TimescaleDB](https://docs.timescale.com/)

---

## 🎯 总结

Agent-Server 架构是 Process Tracker 的**长期战略方向**，将单机监控工具演进为**多服务器集中管理平台**。

**核心价值**:
- 统一管理多台服务器
- 集中存储和分析
- 横向对比和告警
- 降低运维成本

**实施建议**:
- 优先完成单机版优化 (Phase 1-2)
- 逐步引入Agent-Server架构 (Phase 3)
- 保持轻量级和易用性的核心优势
- 参考成熟监控系统的最佳实践

**预期效果**:
打造一个**简单、轻量、强大**的多服务器进程监控平台，成为中小团队的首选监控工具。

---

*文档版本: 1.0*  
*最后更新: 2025-10-15*  
*作者: Process Tracker Team*
