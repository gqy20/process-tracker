# 告警功能完整指南

## 🚀 快速开始（5分钟）

### 1. 获取Webhook URL

**飞书群机器人**（推荐）：
1. 打开飞书群聊 → 群设置 → 群机器人 → 添加机器人 → 自定义机器人
2. 复制Webhook地址

**其他平台**：
- 钉钉：群设置 → 智能群助手 → 添加机器人 → 自定义
- 企业微信：群详情 → 群机器人 → 添加机器人

### 2. 配置环境变量

```bash
export WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_TOKEN"
```

### 3. 测试通知

```bash
./process-tracker --config config-example.yaml test-alert
```

看到 `✅ 测试通知已发送` 表示配置成功！

### 4. 启动监控

```bash
./process-tracker --config config-example.yaml start
```

完成！你的系统现在会在异常时自动通知你。

## 📊 告警指标详解

### 系统级指标（推荐默认使用）⭐⭐⭐

最直观、最常用的监控方式！

#### system_cpu_percent - 系统CPU使用率

```yaml
- name: system_cpu_usage
  metric: system_cpu_percent
  threshold: 80                # 系统CPU > 80%触发
  duration: 300
  channels: ["feishu"]
```

**计算公式**：`sum(所有进程CPU%) / CPU核心数`

**示例**：
- 72核服务器
- 10个进程各占50% CPU
- 结果：500 / 72 = **6.94%** ← 直观！

**优点**：
- ✅ 就像top命令看到的系统整体使用率
- ✅ 阈值直观：80就是80%
- ✅ 最常用的监控指标

#### system_memory_percent - 系统内存使用率

```yaml
- name: system_memory_usage
  metric: system_memory_percent
  threshold: 85                # 系统内存 > 85%触发
  duration: 300
  channels: ["feishu"]
```

**计算公式**：`sum(所有进程内存MB) / 系统总内存MB * 100`

**示例**：
- 总内存300GB
- 当前使用260GB
- 结果：260/300 * 100 = **86.67%** ← 直观！

### 进程级指标（配合aggregation使用）

用于检测单个进程异常。

#### cpu_percent + max - 检测单个进程CPU异常

```yaml
- name: single_process_cpu_high
  metric: cpu_percent
  aggregation: max             # 使用最大值
  threshold: 95                # 任意进程CPU > 95%
  duration: 300
  channels: ["feishu"]
```

**示例**：
- 100个进程，1个占100% CPU
- max = 100% ✅ 立即检测到
- avg = 1% ❌ 检测不到

#### memory_mb + max - 检测单个进程内存异常

```yaml
- name: single_process_memory_high
  metric: memory_mb
  aggregation: max
  threshold: 5000              # 任意进程内存 > 5GB
  duration: 300
  channels: ["feishu"]
```

## 🎯 推荐配置方案

### 方案一：基础监控（必须配置）

适用于：所有服务器

```yaml
alerts:
  enabled: true
  suppress_duration: 30
  
  rules:
    # 系统CPU告警
    - name: system_cpu_high
      metric: system_cpu_percent
      threshold: 80
      duration: 300
      channels: ["feishu"]
      enabled: true
    
    # 系统内存告警
    - name: system_memory_high
      metric: system_memory_percent
      threshold: 85
      duration: 300
      channels: ["feishu"]
      enabled: true

notifiers:
  feishu:
    webhook_url: "${WEBHOOK_URL}"
```

### 方案二：完整监控（推荐生产环境）

增加单进程异常检测：

```yaml
alerts:
  enabled: true
  suppress_duration: 30
  
  rules:
    # 第一层：系统监控
    - name: system_cpu_high
      metric: system_cpu_percent
      threshold: 80
      duration: 300
      channels: ["feishu"]
    
    - name: system_memory_high
      metric: system_memory_percent
      threshold: 85
      duration: 300
      channels: ["feishu"]
    
    # 第二层：进程异常监控
    - name: process_cpu_runaway
      metric: cpu_percent
      aggregation: max
      threshold: 95
      duration: 300
      channels: ["feishu"]
    
    - name: process_memory_leak
      metric: memory_mb
      aggregation: max
      threshold: 5000
      duration: 300
      channels: ["feishu"]

notifiers:
  feishu:
    webhook_url: "${WEBHOOK_URL}"
```

### 方案三：精细化监控（特定场景）

针对关键服务：

```yaml
rules:
  # ... 基础监控规则 ...
  
  # 针对Nginx进程
  - name: nginx_cpu_high
    metric: cpu_percent
    threshold: 80
    duration: 180
    process: nginx             # 只监控nginx进程
    aggregation: max
    channels: ["feishu"]
  
  # 针对MySQL进程
  - name: mysql_memory_high
    metric: memory_mb
    threshold: 10000           # 10GB
    duration: 180
    process: mysqld
    aggregation: max
    channels: ["feishu"]
```

## 📈 告警通知示例

### 系统CPU告警

```
🚨 告警: system_cpu_high

**指标**: system_cpu_percent
**当前值**: 87.50 (聚合:avg)
**阈值**: 80.00
**持续时长**: 305秒

🕐 2025-10-15 18:31:59
```

### 单进程CPU告警

```
🚨 告警: process_cpu_runaway

**指标**: cpu_percent
**当前值**: 99.70 (聚合:max)
**阈值**: 95.00
**持续时长**: 315秒

🕐 2025-10-15 18:35:20
```

### 恢复通知

```
✅ 恢复: system_cpu_high

**指标**: system_cpu_percent
**上次值**: 87.50
**阈值**: 80.00
**状态**: 已恢复正常

🕐 2025-10-15 18:38:45
```

## 🔧 配置参数详解

### 告警规则参数

| 参数 | 类型 | 说明 | 示例 |
|------|------|------|------|
| name | string | 告警规则名称 | `system_cpu_high` |
| metric | string | 监控指标 | `system_cpu_percent` |
| threshold | float | 阈值 | `80` |
| duration | int | 持续时间(秒) | `300` |
| aggregation | string | 聚合方式(可选) | `max` / `avg` / `sum` |
| process | string | 进程名过滤(可选) | `nginx` |
| channels | array | 通知渠道 | `["feishu"]` |
| enabled | bool | 是否启用 | `true` |

### 指标类型

| 指标 | 说明 | 单位 | 使用场景 |
|------|------|------|---------|
| system_cpu_percent | 系统CPU使用率 | % | 系统整体监控 ⭐⭐⭐ |
| system_memory_percent | 系统内存使用率 | % | 系统整体监控 ⭐⭐⭐ |
| cpu_percent | 进程CPU使用率 | % | 单进程监控 ⭐⭐ |
| memory_mb | 进程内存用量 | MB | 单进程监控 ⭐⭐ |

### 聚合方式

| 方式 | 说明 | 适用场景 |
|------|------|---------|
| max | 最大值 | 检测单个进程异常 ⭐ 推荐 |
| avg | 平均值 | 检测平均负载（很少用）|
| sum | 总和 | 不推荐，用system_xxx_percent代替 |

**注意**：
- system_cpu_percent 和 system_memory_percent 不需要aggregation参数
- cpu_percent 和 memory_mb 需要配合aggregation使用

## ⚙️ 支持的通知渠道

### 飞书（已测试）✅

```yaml
notifiers:
  feishu:
    webhook_url: "${WEBHOOK_URL}"
```

### 钉钉

```yaml
notifiers:
  dingtalk:
    webhook_url: "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN"
    secret: "YOUR_SECRET"      # 可选：签名密钥
```

### 企业微信

```yaml
notifiers:
  wechat:
    webhook_url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY"
```

### 通用Webhook

```yaml
notifiers:
  webhook:
    url: "https://your-webhook-url.com/alert"
    method: "POST"             # 默认POST
    headers:
      Authorization: "Bearer YOUR_TOKEN"
```

## 💡 最佳实践

### 1. 分层监控策略

**必须配置**（第一层）：
- system_cpu_percent > 80%
- system_memory_percent > 85%

**建议配置**（第二层）：
- cpu_percent (max) > 95%
- memory_mb (max) > 5GB

**可选配置**（第三层）：
- 针对特定服务的process过滤

### 2. 合理设置duration

```yaml
# 紧急告警
duration: 60         # 1分钟

# 常规告警
duration: 300        # 5分钟

# 非关键告警
duration: 600        # 10分钟
```

### 3. 配置抑制期避免告警风暴

```yaml
alerts:
  suppress_duration: 30  # 30分钟内不重复通知同一告警
```

### 4. 生产环境前测试

```bash
# 1. 测试通知
./process-tracker test-alert

# 2. 降低阈值运行几小时观察
threshold: 10  # 临时设置低阈值

# 3. 调整到合理阈值
threshold: 80  # 恢复正常阈值
```

### 5. 告警阈值参考

**CPU使用率**：
- 警告：70-80%
- 严重：80-90%
- 紧急：> 90%

**内存使用率**：
- 警告：75-85%
- 严重：85-95%
- 紧急：> 95%

**单进程CPU**：
- 警告：80%
- 严重：95%

**单进程内存**：
- 根据服务器规格调整
- 建议：总内存的10-20%

## 🐛 常见问题

### Q1: 告警不触发？

**检查点**：
1. 配置文件中 `enabled: true`
2. duration是否太长
3. threshold是否设置过高
4. 环境变量WEBHOOK_URL是否设置

```bash
# 检查配置
cat config-example.yaml | grep -A 5 "alerts:"

# 测试webhook
./process-tracker test-alert

# 查看评估日志
tail -f ~/.process-tracker/process-tracker.log | grep "告警评估"
```

### Q2: 告警太频繁？

调整抑制时长：
```yaml
alerts:
  suppress_duration: 60  # 增加到60分钟
```

或增加duration：
```yaml
duration: 600  # 增加到10分钟
```

### Q3: 想监控特定进程？

添加process过滤：
```yaml
- name: nginx_monitor
  metric: cpu_percent
  threshold: 80
  process: nginx           # 只监控nginx
  aggregation: max
```

### Q4: 如何查看当前告警状态？

```bash
# 查看监控日志
tail -f ~/.process-tracker/process-tracker.log | grep "告警"

# 查看系统状态
./process-tracker status
```

### Q5: system_xxx_percent vs cpu_percent + sum的区别？

| 方式 | 结果示例 | 直观性 |
|------|---------|--------|
| system_cpu_percent | 6.94% | ✅ 直观 |
| cpu_percent + sum | 500% | ❌ 不直观 |

**推荐使用system_cpu_percent**！

## 🔍 指标对比案例

### 场景：72核服务器，10个进程各50% CPU

| 指标配置 | 计算结果 | 说明 |
|---------|---------|------|
| `system_cpu_percent` | **6.94%** | ✅ 直观的系统使用率 |
| `cpu_percent` + `max` | **50%** | 单个进程最大值 |
| `cpu_percent` + `avg` | **50%** | 进程平均值 |
| `cpu_percent` + `sum` | **500%** | ❌ 不直观 |

**结论**：system_cpu_percent最适合系统整体监控！

## 📚 完整配置示例

参考文件：`config-example.yaml`

包含：
- ✅ 系统级监控（必须）
- ✅ 单进程异常监控（建议）
- ✅ 特定服务监控（可选）
- ✅ 多通知渠道配置
- ✅ 详细注释说明

## ✨ 功能特性

1. ✅ **系统级百分比监控** - 最直观的监控方式
2. ✅ **多聚合方式支持** - max/avg/sum
3. ✅ **进程过滤** - 针对特定进程监控
4. ✅ **告警抑制** - 避免告警风暴
5. ✅ **恢复通知** - 问题解决后自动通知
6. ✅ **多通道支持** - 飞书/钉钉/企业微信
7. ✅ **测试命令** - 快速验证配置
8. ✅ **调试日志** - 详细的评估日志

现在开始使用吧！🚀
