# 局域网访问 + 飞书告警使用指南

## 📡 一、局域网访问配置

### 当前状态

**默认配置**:
```yaml
web:
  host: "localhost"  # 仅本地访问
  port: "18080"
```

**访问限制**: ❌ 局域网内其他设备无法访问

---

### ✅ 启用局域网访问

#### 方法1: 修改配置文件（推荐）

```yaml
# ~/.process-tracker/config.yaml
web:
  enabled: true
  host: "0.0.0.0"    # 监听所有网卡，允许局域网访问
  port: "18080"
```

然后启动：
```bash
./process-tracker web
```

#### 方法2: 命令行参数

```bash
# 直接指定host和port
./process-tracker web --host 0.0.0.0 --port 18080
```

#### 方法3: 启动监控时同时启用Web

```bash
./process-tracker start --web --web-port 18080
# 需要修改配置文件中的host为0.0.0.0
```

---

### 🌐 局域网访问步骤

**1. 启动服务（监听所有网卡）**
```bash
./process-tracker web --host 0.0.0.0 --port 18080
```

**输出示例**:
```
Web服务器启动: http://0.0.0.0:18080
```

**2. 查看服务器IP地址**
```bash
# 查看本机IP
ip addr show | grep "inet " | grep -v "127.0.0.1"
# 或
hostname -I
```

假设输出: `192.168.1.100`

**3. 局域网内其他设备访问**
```
浏览器打开: http://192.168.1.100:18080
```

**4. 验证端口监听**
```bash
# 确认端口正在监听所有接口
ss -tlnp | grep 18080
# 或
netstat -tlnp | grep 18080
```

应该看到：
```
tcp   0   0  0.0.0.0:18080    0.0.0.0:*    LISTEN
```

---

### 🔒 安全建议

⚠️ **警告**: 使用 `0.0.0.0` 会让局域网内所有设备都能访问

**建议安全措施**:

1. **防火墙配置**
   ```bash
   # Ubuntu/Debian
   sudo ufw allow from 192.168.1.0/24 to any port 18080
   
   # CentOS/RHEL
   sudo firewall-cmd --permanent --add-rich-rule='rule family="ipv4" source address="192.168.1.0/24" port protocol="tcp" port="18080" accept'
   sudo firewall-cmd --reload
   ```

2. **使用非标准端口**
   - 已使用18080，避免常见攻击

3. **仅内网环境使用**
   - 不要暴露到公网
   - 生产环境建议使用Nginx反向代理 + SSL

4. **添加基础认证**（未来功能）
   - 计划在v0.5.0添加HTTP Basic Auth

---

## 🔔 二、飞书告警触发机制

### 告警触发条件

**飞书通知会在以下情况下发送**:

```
条件1: 告警系统已启用 (alerts.enabled = true)
  ↓
条件2: 配置了告警规则
  ↓
条件3: 监控指标超过阈值 (如CPU > 80%)
  ↓
条件4: 持续超过指定时间 (如持续5分钟)
  ↓
条件5: 不在抑制期内 (默认30分钟抑制期)
  ↓
触发: 发送飞书通知 ✅
```

---

### 完整配置示例

```yaml
# ~/.process-tracker/config.yaml

# Web配置 - 局域网访问
web:
  enabled: true
  host: "0.0.0.0"     # 允许局域网访问
  port: "18080"

# 告警配置
alerts:
  enabled: true       # ⚠️ 必须设置为true
  suppress_duration: 30  # 抑制时长(分钟)
  
  rules:
    # 规则1: CPU高负载告警
    - name: high_cpu_alert
      enabled: true
      metric: cpu_percent      # 监控CPU使用率
      threshold: 80            # 阈值80%
      duration: 300            # 持续5分钟(300秒)
      channels: ["feishu"]     # 通过飞书通知
    
    # 规则2: 内存高占用告警
    - name: high_memory_alert
      enabled: true
      metric: memory_mb        # 监控内存(MB)
      threshold: 1024          # 阈值1GB
      duration: 300            # 持续5分钟
      channels: ["feishu"]
    
    # 规则3: 特定进程告警
    - name: docker_high_cpu
      enabled: true
      metric: cpu_percent
      threshold: 50
      duration: 180            # 持续3分钟
      process: "docker"        # 只监控docker进程
      channels: ["feishu"]

# 通知器配置
notifiers:
  feishu:
    webhook_url: "${WEBHOOK_URL}"  # 从环境变量读取
```

---

### 告警工作流程

#### 1. 监控循环

```
每5秒执行一次:
  ↓
采集进程数据 (CPU, 内存, 等)
  ↓
评估告警规则 ⚠️ (需要集成)
  ↓
如果超过阈值: 更新告警状态
  ↓
如果持续时间满足: 发送通知
```

#### 2. 告警状态机

```
初始状态: 无告警
  ↓
[CPU > 80%] → 开始计时
  ↓
[持续1分钟] → 继续计时
  ↓
[持续5分钟] → 触发告警 → 发送飞书通知 📨
  ↓
[CPU恢复正常] → 发送恢复通知 → 重置状态
```

#### 3. 告警抑制

```
第一次通知: 立即发送 ✅
  ↓
抑制期开始 (30分钟)
  ↓
期间即使再次触发: 不发送 ❌
  ↓
30分钟后: 可以再次发送 ✅
```

---

### ⚠️ 重要：当前限制

**告警系统尚未完全集成**

当前状态：
- ✅ 告警引擎已实现 (`core/alerting.go`)
- ✅ 飞书通知器已实现 (`core/feishu_notifier.go`)
- ✅ 配置加载已实现
- ❌ **未集成到监控循环** ⚠️

**需要添加的代码** (在 `core/app.go` 中):

```go
// 在监控循环中添加 (大约第200行)
func (a *App) Run() error {
    // ... 现有代码 ...
    
    for {
        // 采集进程数据
        records := a.collectProcessData()
        
        // 🆕 添加告警评估
        if a.alertManager != nil {
            a.alertManager.Evaluate(records)
        }
        
        // ... 其他逻辑 ...
    }
}
```

---

## 🧪 完整测试步骤

### 步骤1: 配置文件

```bash
# 创建完整配置
cat > ~/.process-tracker/config.yaml << 'EOF'
web:
  enabled: true
  host: "0.0.0.0"      # 局域网访问
  port: "18080"

storage:
  max_size_mb: 100
  keep_days: 7

alerts:
  enabled: true
  suppress_duration: 30
  
  rules:
    # 低阈值便于测试
    - name: test_cpu_alert
      enabled: true
      metric: cpu_percent
      threshold: 10       # 低阈值10%，容易触发
      duration: 10        # 10秒后触发
      channels: ["feishu"]

notifiers:
  feishu:
    webhook_url: "${WEBHOOK_URL}"
EOF
```

### 步骤2: 启动服务

```bash
# 确保环境变量已设置
echo $WEBHOOK_URL

# 启动Web + 监控
./process-tracker web
```

### 步骤3: 测试局域网访问

**在服务器上**:
```bash
# 查看IP
hostname -I
# 假设输出: 192.168.1.100
```

**在局域网内其他设备**:
```
浏览器打开: http://192.168.1.100:18080
```

应该能看到Dashboard界面

### 步骤4: 测试飞书告警

**方法A: 使用测试工具**
```bash
go run test_webhook.go
# 应该立即收到飞书通知
```

**方法B: 触发实际告警** (需要集成后)
```bash
# 模拟高CPU
stress --cpu 2 --timeout 30s

# 或
yes > /dev/null &
# 记住PID，测试后执行: kill <PID>
```

10秒后应该收到飞书告警

---

## 📊 当前功能状态

### Web局域网访问

| 功能 | 状态 | 配置 |
|------|------|------|
| 本地访问 | ✅ | host: "localhost" |
| 局域网访问 | ✅ | host: "0.0.0.0" |
| 命令行指定 | ✅ | --host 0.0.0.0 |
| 端口配置 | ✅ | --port 18080 |

### 飞书告警

| 功能 | 状态 | 说明 |
|------|------|------|
| 飞书通知器 | ✅ | 已实现 |
| 环境变量支持 | ✅ | ${WEBHOOK_URL} |
| 告警引擎 | ✅ | 已实现 |
| 规则配置 | ✅ | YAML配置 |
| 测试工具 | ✅ | test_webhook.go |
| **监控集成** | ⚠️ | **需要30分钟集成** |

---

## 🚀 立即可用命令

### 局域网Web访问

```bash
# 启动Web服务器（局域网可访问）
./process-tracker web --host 0.0.0.0 --port 18080

# 访问地址（替换为实际IP）
# http://YOUR_SERVER_IP:18080
```

### 测试飞书通知

```bash
# 直接测试飞书通知
go run test_webhook.go

# 或使用脚本
./test_webhook.sh
```

---

## 💡 实用配置模板

### 开发环境（本地访问）

```yaml
web:
  enabled: true
  host: "localhost"
  port: "18080"

alerts:
  enabled: false  # 开发时关闭告警
```

### 测试环境（局域网访问+告警）

```yaml
web:
  enabled: true
  host: "0.0.0.0"
  port: "18080"

alerts:
  enabled: true
  suppress_duration: 5  # 短抑制期便于测试
  
  rules:
    - name: test_alert
      metric: cpu_percent
      threshold: 10      # 低阈值
      duration: 10       # 10秒
      channels: ["feishu"]

notifiers:
  feishu:
    webhook_url: "${WEBHOOK_URL}"
```

### 生产环境（安全配置）

```yaml
web:
  enabled: true
  host: "127.0.0.1"    # 仅本地，通过Nginx代理
  port: "18080"

alerts:
  enabled: true
  suppress_duration: 30
  
  rules:
    - name: critical_cpu
      metric: cpu_percent
      threshold: 80
      duration: 300      # 5分钟
      channels: ["feishu"]
    
    - name: critical_memory
      metric: memory_mb
      threshold: 4096    # 4GB
      duration: 300
      channels: ["feishu"]

notifiers:
  feishu:
    webhook_url: "${WEBHOOK_URL}"
```

---

## 🔧 故障排查

### 局域网无法访问

**问题**: 其他设备访问不了Dashboard

**检查清单**:
```bash
# 1. 确认host配置
cat ~/.process-tracker/config.yaml | grep "host:"
# 应该是: host: "0.0.0.0"

# 2. 确认端口监听
ss -tlnp | grep 18080
# 应该看到: 0.0.0.0:18080

# 3. 测试本地访问
curl http://localhost:18080/api/health

# 4. 测试局域网访问（在另一台机器）
curl http://SERVER_IP:18080/api/health

# 5. 检查防火墙
sudo ufw status | grep 18080
# 或
sudo firewall-cmd --list-ports | grep 18080
```

### 飞书通知不发送

**问题**: 没有收到飞书通知

**检查清单**:
```bash
# 1. 测试Webhook URL
go run test_webhook.go

# 2. 确认环境变量
echo $WEBHOOK_URL

# 3. 确认告警已启用
cat ~/.process-tracker/config.yaml | grep -A2 "alerts:"

# 4. 确认规则配置
cat ~/.process-tracker/config.yaml | grep -A5 "rules:"

# 5. 查看日志
./process-tracker start
# 查看是否有"告警已发送"的日志
```

---

## 📝 总结

### 局域网访问

✅ **支持**: 配置 `host: "0.0.0.0"`  
✅ **方法**: 配置文件或命令行参数  
✅ **安全**: 建议配置防火墙  

### 飞书告警触发时机

✅ **条件1**: alerts.enabled = true  
✅ **条件2**: 配置了告警规则  
✅ **条件3**: CPU/内存超过阈值  
✅ **条件4**: 持续超过指定时间  
⚠️ **限制**: 需要集成到监控循环（30分钟工作）

### 立即可测试

```bash
# 1. 局域网Web访问
./process-tracker web --host 0.0.0.0 --port 18080

# 2. 飞书通知测试
go run test_webhook.go
```

---

*更新时间: 2025年1月*  
*状态: ✅ 局域网支持完整*  
*状态: ⚠️ 告警需30分钟集成*
