# 重构代码量预估 (Refactoring Code Changes Estimation)

本文档详细估算Phase 1和Phase 2重构需要修改的代码量。

---

## 预估方法说明

- **新增**: 新创建的代码行数
- **删除**: 删除或移除的代码行数
- **修改**: 原地修改的代码行数
- **净变化**: 新增 - 删除（代码库最终增减）
- **实际变动**: 新增 + 删除 + 修改（更能反映工作量）

---

## Phase 1: 基础设施 (预计1周)

### 任务1: TD2 - 提取系统指标函数 (3小时)

#### 当前状态
- `core/app.go`: 756行，包含系统指标函数
- `cmd/web.go`: 671行，包含重复的系统指标函数

#### 变更详情

**删除代码:**
- `core/app.go`:
  - `getTotalMemoryMB()`: 117-145行 (29行)
  - `getTotalCPUCores()`: 156-182行 (27行)
  - `calculateMemoryPercent()`: 147-154行 (8行)
  - `calculateCPUPercentNormalized()`: 184-191行 (8行)
  - 全局变量和mutex: ~10行
  - **小计**: 82行

- `cmd/web.go`:
  - `getSystemCPUCores()`: ~10行
  - `getSystemTotalMemoryMB()`: ~10行
  - **小计**: 20行

**删除总计: 102行**

**新增代码:**
- 创建 `core/system_metrics.go`:
  ```
  package core              // 包声明和imports: ~15行
  
  var (                     // 全局变量和mutex: ~10行
      cachedTotalMemoryMB float64
      cachedTotalCPUCores int
      memoryMu            sync.RWMutex
      cpuMu               sync.RWMutex
  )
  
  // GetSystemMemoryMB()    // 增强版，带缓存: ~30行
  // GetSystemCPUCores()    // 增强版，带缓存: ~30行
  // CalculateMemoryPercent()  // 计算百分比: ~10行
  // CalculateCPUPercentNormalized() // 归一化: ~10行
  // FormatMemory()         // 格式化显示: ~20行
  // 文档注释               // ~20行
  ```
  - **小计**: 145行

**新增总计: 145行**

**修改代码:**
- `core/app.go`: 更新调用点 (~8处) = 8行
- `cmd/web.go`: 更新调用点 (~3处) = 3行
- `core/alerting.go`: 更新调用点 (~2处) = 2行
- imports更新: ~2行

**修改总计: 15行**

#### TD2 汇总
| 指标 | 行数 |
|------|------|
| 新增 | 145行 |
| 删除 | 102行 |
| 修改 | 15行 |
| **净变化** | **+43行** |
| **实际变动** | **262行** |

---

### 任务2: TD3 - 修复资源浪费 (2小时)

#### 变更详情

**修改代码:**
- `core/storage.go`:
  - `Initialize()` 方法: ~15行修改
    ```go
    func (m *Manager) Initialize() error {
        if m.useStorageMgr {
            // 使用StorageManager时，不初始化自己的file
            sm := NewStorageManager(m.dataFile, m.storageConfig)
            m.storageManager = sm
        } else {
            // 只有不使用StorageManager时，才初始化file
            if err := m.initializeFile(); err != nil {
                return fmt.Errorf("failed to initialize file: %w", err)
            }
        }
        return nil
    }
    ```
  - `flushBuffer()` fallback逻辑调整: ~10行
  - `Close()` 方法调整: ~5行

**修改总计: 30行**

#### TD3 汇总
| 指标 | 行数 |
|------|------|
| 新增 | 0行 |
| 删除 | 0行 |
| 修改 | 30行 |
| **净变化** | **0行** |
| **实际变动** | **30行** |

---

### 任务3: TD1 Phase 1 - 单元测试 (5天)

#### 新增测试文件

**1. core/storage_test.go (~300行)**
```go
- TestNewManager (20行)
- TestManagerInitialize (30行)
- TestSaveRecord (40行)
- TestSaveRecordBuffering (50行)
- TestReadRecords (40行)
- TestParseRecord_V5Format (30行)
- TestParseRecord_V6Format (30行)
- TestParseRecord_V7Format (30行)
- TestCalculateStats (60行)
- 辅助函数和fixtures (20行)
```

**2. core/storage_manager_test.go (~250行)**
```go
- TestNewStorageManager (20行)
- TestFileRotation (70行)
- TestFileCompression (60行)
- TestCleanup (50行)
- TestRotationPolicy (30行)
- 辅助函数 (20行)
```

**3. core/alerting_test.go (~300行)**
```go
- TestNewAlertManager (20行)
- TestEvaluateRules_Threshold (50行)
- TestAggregation_Max (40行)
- TestAggregation_Avg (40行)
- TestAggregation_Sum (40行)
- TestSystemCPUPercent (30行)
- TestSystemMemoryPercent (30行)
- TestNotifierIntegration (30行)
- Mock notifiers (20行)
```

**4. core/docker_test.go (~200行)**
```go
- TestNewDockerMonitor (30行)
- TestDockerMonitor_Start (40行)
- TestDockerMonitor_Stop (30行)
- TestContainerTracking (50行)
- TestContainerEvents (30行)
- Mock Docker client (20行)
```

**5. cmd/web_test.go (~250行)**
```go
- TestNewWebServer (20行)
- TestWebServer_Start (30行)
- TestHandleStatsToday (40行)
- TestHandleStatsWeek (40行)
- TestHandleLive (40行)
- TestHandleProcesses (40行)
- TestStatsCache (20行)
- httptest helpers (20行)
```

**6. cmd/commands_test.go (~200行)**
```go
- TestLoadConfig (40行)
- TestLoadConfig_MissingFile (30行)
- TestValidateConfig (40行)
- TestValidateConfig_Invalid (30行)
- TestCreateDefaultConfig (30行)
- 测试fixtures (30行)
```

#### TD1 Phase 1 汇总
| 指标 | 行数 |
|------|------|
| 新增 | 1,500行 |
| 删除 | 0行 |
| 修改 | 0行 |
| **净变化** | **+1,500行** |
| **实际变动** | **1,500行** |

---

## Phase 1 总计

| 任务 | 新增 | 删除 | 修改 | 净变化 | 实际变动 |
|------|------|------|------|--------|----------|
| TD2 - 代码重复 | 145 | 102 | 15 | +43 | 262 |
| TD3 - 资源浪费 | 0 | 0 | 30 | 0 | 30 |
| TD1 Phase 1 - 测试 | 1,500 | 0 | 0 | +1,500 | 1,500 |
| **Phase 1 合计** | **1,645** | **102** | **45** | **+1,543** | **1,792** |

**文件数变化**: +6个新文件
- core/system_metrics.go
- core/storage_test.go
- core/storage_manager_test.go
- core/alerting_test.go
- core/docker_test.go
- cmd/web_test.go
- cmd/commands_test.go (实际7个)

---

## Phase 2: 代码重构 (预计1-2周)

### 任务1: TD4 - app.go重构 (3天)

#### 当前状态
- `core/app.go`: 756行，承担多种职责

#### 创建新文件

**1. core/collector.go (~220行)**

从app.go迁移数据采集功能：
```go
package core                     // 包声明和imports: ~20行

type ProcessCollector struct {   // 结构体定义: ~20行
    config Config
    dockerMonitor *DockerMonitor
}

// NewProcessCollector()          // 构造函数: ~15行
// GetProcessInfo()                // 454-510行: ~57行
// GetCurrentResources()           // 517-577行: ~61行  
// getNetworkStats()               // 511-515行: ~5行
// collectDockerContainerRecords() // 718-757行: ~40行
// 辅助函数                        // ~2行
```
**小计: 220行**

**2. core/analyzer.go (~320行)**

从app.go迁移数据分析功能：
```go
package core                        // 包声明和imports: ~15行

type ResourceAnalyzer struct {      // 结构体定义: ~10行
    storage *Manager
}

// NewResourceAnalyzer()            // 构造函数: ~10行
// CalculateResourceStats()         // 220-238行: ~19行
// CompareStats()                   // 239-309行: ~71行
// ShowTrends()                     // 311-420行: ~110行
// CleanOldData()                   // 422-425行: ~4行
// GetTotalRecords()                // 427-430行: ~4行
// 辅助函数和格式化                  // ~77行
```
**小计: 320行**

**3. core/process_filter.go (~150行)**

从app.go迁移进程过滤功能：
```go
package core                     // 包声明和imports: ~10行

type ProcessFilter struct {      // 结构体定义: ~10行
    config Config
}

// NewProcessFilter()            // 构造函数: ~10行
// isSystemProcess()             // 580-607行: ~28行
// normalizeProcessName()        // 608-614行: ~7行
// GetProcessNameWithContext()   // 615-632行: ~18行
// shouldFilterProcess()         // 新增: ~20行
// categorizeProcess()           // 新增: ~30行
// 分类规则数据                   // ~17行
```
**小计: 150行**

**4. core/system_metrics.go 补充 (~50行)**

Phase 1已创建，Phase 2补充：
```go
// formatMemory()          // 440-452行: ~20行
// truncateString()        // 432-438行: ~7行
// 其他系统级工具函数       // ~23行
```
**补充: 50行**

**新增总计: 740行** (collector.go + analyzer.go + process_filter.go + 补充)

#### 修改 core/app.go

**删除代码:**
- 数据采集函数: ~160行
- 数据分析函数: ~200行
- 进程过滤函数: ~130行
- 工具函数: ~50行

**删除总计: 540行**

**保留代码:**
- App结构体定义: ~20行
- NewApp() 构造函数: ~30行
- Initialize(), Close(): ~20行
- CollectAndSaveData(): ~80行
- 存储代理方法: ~20行
- imports和包声明: ~10行

**保留总计: ~180行**

**修改代码:**
- App结构体添加新组件字段: ~10行
- NewApp()注入新依赖: ~15行
- CollectAndSaveData()调用新模块: ~20行
- 更新imports: ~5行

**修改总计: 50行**

#### TD4 汇总
| 指标 | 行数 |
|------|------|
| 新增 | 740行 |
| 删除 | 540行 |
| 修改 | 50行 |
| **净变化** | **+200行** |
| **实际变动** | **1,330行** |

**文件变化**:
- 新增: collector.go, analyzer.go, process_filter.go
- 修改: app.go (756行 → ~230行), system_metrics.go

---

### 任务2: TD1 Phase 2 - 集成测试 (3天)

#### 新增集成测试

**1. tests/integration/storage_integration_test.go (~150行)**
```go
- TestFullDataPipeline (50行)
  // 采集 → 存储 → 查询 → 分析完整流程
- TestFileRotationIntegration (50行)
  // 写入大量数据触发轮转
- TestCompressionIntegration (30行)
  // 验证压缩和解压
- Setup/Teardown helpers (20行)
```

**2. tests/integration/alert_integration_test.go (~150行)**
```go
- TestAlertTriggering_EndToEnd (60行)
  // 数据采集 → 规则评估 → 通知发送
- TestMultipleRules_Priority (40行)
  // 多规则并发评估
- TestAlertRecovery (30行)
  // 告警恢复流程
- Mock webhook server (20行)
```

**3. tests/integration/web_integration_test.go (~100行)**
```go
- TestWebServer_FullRequest (40行)
  // 启动服务器 → 发送请求 → 验证响应
- TestWebServer_LiveData (30行)
  // 实时数据推送测试
- TestWebServer_Concurrency (20行)
  // 并发请求测试
- Helper functions (10行)
```

#### 补充单元测试

**4. core/collector_test.go (~100行)**
```go
- TestProcessCollector_GetProcessInfo (40行)
- TestProcessCollector_GetCurrentResources (40行)
- Mock process data (20行)
```

**5. core/analyzer_test.go (~100行)**
```go
- TestResourceAnalyzer_Calculate (40行)
- TestResourceAnalyzer_Compare (40行)
- Test fixtures (20行)
```

**6. core/process_filter_test.go (~100行)**
```go
- TestProcessFilter_IsSystem (30行)
- TestProcessFilter_Normalize (30行)
- TestProcessFilter_Categorize (30行)
- Test data (10行)
```

#### TD1 Phase 2 汇总
| 指标 | 行数 |
|------|------|
| 新增 | 700行 |
| 删除 | 0行 |
| 修改 | 0行 |
| **净变化** | **+700行** |
| **实际变动** | **700行** |

**文件数变化**: +6个新测试文件

---

### 任务3: TD5 - web.go重构 (2天)

#### 当前状态
- `cmd/web.go`: 671行

#### 创建新文件

**1. cmd/web_server.go (~170行)**
```go
package cmd                      // 包声明和imports: ~20行

type WebServer struct { ... }    // 结构体定义: ~30行

// NewWebServer()                // 78-87行: ~10行
// Start()                       // 88-128行: ~60行
// handleShutdown()              // 129-177行: ~40行
// printAccessURLs()             // 606-623行: ~25行
```
**小计: 170行**

**2. cmd/web_handlers.go (~280行)**
```go
package cmd                      // 包声明和imports: ~15行

// handleStatsToday()            // 178-182行: ~5行
// handleStatsWeek()             // 183-187行: ~5行
// handleStatsMonth()            // 188-192行: ~5行
// handleStatsPeriod()           // 193-219行: ~30行
// handleLive()                  // 220-240行: ~25行
// handleProcesses()             // 241-283行: ~50行
// handleHealth()                // 284-292行: ~10行
// readRecentRecords()           // 293-316行: ~30行
// getLatestProcesses()          // 520-562行: ~50行
// 辅助函数                       // ~55行
```
**小计: 280行**

**3. cmd/web_stats.go (~220行)**
```go
package cmd                      // 包声明和imports: ~15行

type DashboardStats struct {...} // 数据结构: ~30行
type TimelinePoint struct {...}  // 数据结构: ~10行

// calculateStats()              // 317-417行: ~120行
// generateTimeline()            // 418-484行: ~80行
// getTopProcesses()             // 485-519行: ~40行
```
**小计: 220行**

**4. cmd/web_cache.go (~80行)**
```go
package cmd                      // 包声明和imports: ~10行

type StatsCache struct {...}     // 结构体定义: ~15行

// NewStatsCache()               // 46-54行: ~10行
// Get()                         // 55-68行: ~15行
// Set()                         // 69-77行: ~10行
// cleanup routine               // ~20行
```
**小计: 80行**

**新增总计: 750行**

#### 删除 cmd/web.go

- 删除迁移的代码: ~620行
- 保留共享工具函数: ~50行
  - `getSystemCPUCores()`, `getSystemTotalMemoryMB()` (将在Phase 1移除)
  - `getStatus()`, `formatUptime()`, `getLocalIPs()`

**删除总计: 620行**

#### 修改代码

- 更新imports: ~5行
- 调整函数调用: ~20行
- 路由设置调整: ~5行

**修改总计: 30行**

#### TD5 汇总
| 指标 | 行数 |
|------|------|
| 新增 | 750行 |
| 删除 | 620行 |
| 修改 | 30行 |
| **净变化** | **+130行** |
| **实际变动** | **1,400行** |

**文件变化**:
- 新增: web_server.go, web_handlers.go, web_stats.go, web_cache.go
- 修改: web.go (671行 → ~50行工具函数)

---

## Phase 2 总计

| 任务 | 新增 | 删除 | 修改 | 净变化 | 实际变动 |
|------|------|------|------|--------|----------|
| TD4 - app.go重构 | 740 | 540 | 50 | +200 | 1,330 |
| TD1 Phase 2 - 测试 | 700 | 0 | 0 | +700 | 700 |
| TD5 - web.go重构 | 750 | 620 | 30 | +130 | 1,400 |
| **Phase 2 合计** | **2,190** | **1,160** | **80** | **+1,030** | **3,430** |

**文件数变化**: +13个新文件
- 核心模块: collector.go, analyzer.go, process_filter.go
- Web模块: web_server.go, web_handlers.go, web_stats.go, web_cache.go
- 测试: 6个新测试文件

---

## 总计 (Phase 1 + Phase 2)

| 阶段 | 新增 | 删除 | 修改 | 净变化 | 实际变动 | 工期 |
|------|------|------|------|--------|----------|------|
| Phase 1 | 1,645 | 102 | 45 | +1,543 | 1,792 | 1周 |
| Phase 2 | 2,190 | 1,160 | 80 | +1,030 | 3,430 | 1-2周 |
| **总计** | **3,835** | **1,262** | **125** | **+2,573** | **5,222** | **2-3周** |

---

## 代码变动分析

### 代码库规模变化

| 项目 | 当前 | Phase 1后 | Phase 2后 | 变化 |
|------|------|-----------|-----------|------|
| 总代码行数 | ~5,000 | ~6,543 | ~7,573 | +51% |
| 测试代码 | 42 | 1,542 | 2,242 | +5,238% |
| 业务代码 | ~4,958 | ~5,001 | ~5,331 | +7.5% |
| 测试覆盖率 | ~1% | ~30% | ~60% | +59% |

### 文件结构变化

**当前文件数**: ~25个
- core/: 10个文件
- cmd/: 4个文件
- tests/: 3个文件（含空目录）

**Phase 1后文件数**: ~32个 (+7)
- core/: 11个（+system_metrics.go）
- tests/: 9个（+6个测试文件）

**Phase 2后文件数**: ~45个 (+13)
- core/: 14个（+collector/analyzer/filter）
- cmd/: 8个（+4个web拆分）
- tests/: 15个（+6个测试文件）

### 平均文件大小

| 阶段 | core/平均 | cmd/平均 | 总体平均 |
|------|-----------|----------|----------|
| 当前 | ~330行 | ~400行 | ~200行 |
| Phase 1后 | ~310行 | ~380行 | ~200行 |
| Phase 2后 | ~240行 | ~210行 | ~170行 |

**结论**: 重构后文件更小、更聚焦，符合单一职责原则。

---

## 代码变动类型分布

### Phase 1 (1,792行变动)

| 类型 | 行数 | 占比 |
|------|------|------|
| 测试代码 | 1,500 | 83.7% |
| 新增功能 | 145 | 8.1% |
| 代码迁移 | 102 | 5.7% |
| 修复调整 | 45 | 2.5% |

### Phase 2 (3,430行变动)

| 类型 | 行数 | 占比 |
|------|------|------|
| 代码重组 | 2,300 | 67.1% |
| 测试代码 | 700 | 20.4% |
| 新增功能 | 430 | 12.5% |

---

## 风险评估

### 高风险区域

1. **core/app.go** - 540行删除
   - 影响范围最大
   - 需要充分测试
   - 建议分步重构

2. **cmd/web.go** - 620行删除
   - 影响Web功能
   - 需要集成测试保护

### 低风险区域

1. **system_metrics.go** - 独立新文件
   - 功能单一
   - 易于测试

2. **测试文件** - 纯新增
   - 无破坏性
   - 增量添加

---

## 建议

### 渐进式重构策略

1. **先测试后重构**
   - Phase 1先建立测试基础
   - 有测试保护后再进行大重构

2. **小步快跑**
   - 每完成一个模块就测试
   - 不要等全部完成才测试

3. **保持主干可用**
   - 使用feature flag控制新旧代码
   - 逐步切换到新实现

### 测试覆盖率目标

| 阶段 | 目标 | 重点 |
|------|------|------|
| Phase 1 | 30% | 核心路径 |
| Phase 2 | 60% | 主要功能 |
| Phase 3 | 80% | 边界条件 |

---

## 总结

**Phase 1 + Phase 2 重构规模:**

- **实际修改代码**: 5,222行
- **净增代码**: 2,573行
- **新增文件**: 20个
- **预计工期**: 2-3周

主要增长来自**测试代码** (~2,200行)，这是提高代码质量的必要投资。业务代码通过重构变得更简洁、更易维护。
