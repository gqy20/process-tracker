# Web Dashboard + 告警系统 快速开始

## 🎉 新功能概览

**v0.4.0 新增功能**:
- ✅ Web Dashboard - 实时可视化监控界面
- ✅ 告警系统 - 支持CPU/内存阈值告警
- ✅ 多渠道通知 - 钉钉/企业微信/Webhook

---

## 🚀 快速开始 (5分钟)

### 步骤1: 配置告警 (可选)

复制配置示例:
```bash
mkdir -p ~/.process-tracker
cp config.example.yaml ~/.process-tracker/config.yaml
```

编辑配置文件 `~/.process-tracker/config.yaml`:
```yaml
# 启用Web界面
web:
  enabled: true
  port: "8080"

# 启用告警
alerts:
  enabled: true
  rules:
    - name: high_cpu
      metric: cpu_percent
      threshold: 80
      duration: 300  # 5分钟
      channels: ["dingtalk"]

# 配置钉钉通知
notifiers:
  dingtalk:
    webhook_url: "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN"
```

### 步骤2: 启动监控

**方式1: Web + 监控一体**
```bash
./process-tracker web
```

**方式2: 命令行参数**
```bash
./process-tracker start --web --web-port 8080
```

### 步骤3: 访问Dashboard

打开浏览器访问: **http://localhost:8080**

---

## 📊 Web Dashboard 功能

### 概览卡片
- 活跃进程数 / 总进程数
- 平均CPU使用率 / 峰值
- 总内存使用 / 峰值
- 系统状态 (正常/中等/高负载)

### 趋势图表
- CPU使用率趋势 (可选今日/本周/本月)
- 内存使用趋势
- 交互式Chart.js图表

### Top进程列表
- 实时进程信息
- 支持按CPU/内存/名称排序
- 显示PID、CPU%、内存、运行时长、状态

---

## 🔔 配置告警通知

### 钉钉机器人

1. **创建机器人**
   - 打开钉钉群 → 群设置 → 智能群助手
   - 添加机器人 → 自定义机器人
   - 安全设置: 选择"加签" (推荐) 或 "自定义关键词"

2. **复制Webhook URL**
   ```
   https://oapi.dingtalk.com/robot/send?access_token=xxx
   ```

3. **配置到config.yaml**
   ```yaml
   notifiers:
     dingtalk:
       webhook_url: "你的Webhook URL"
       secret: "你的加签密钥"  # 如果启用了加签
   ```

### 企业微信机器人

1. **创建机器人**
   - 打开企业微信群 → 添加群机器人
   - 复制Webhook地址

2. **配置到config.yaml**
   ```yaml
   notifiers:
     wechat:
       webhook_url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
   ```

### 自定义Webhook

```yaml
notifiers:
  webhook:
    url: "https://your-api-endpoint"
    method: "POST"
    headers:
      Content-Type: "application/json"
      Authorization: "Bearer YOUR_TOKEN"
```

**Webhook Payload格式**:
```json
{
  "title": "告警标题",
  "content": "告警内容",
  "timestamp": 1234567890,
  "source": "process-tracker"
}
```

---

## 📝 告警规则配置

### 示例1: CPU高负载告警

```yaml
alerts:
  rules:
    - name: high_cpu
      enabled: true
      metric: cpu_percent
      threshold: 80           # CPU超过80%
      duration: 300           # 持续5分钟
      channels: ["dingtalk"]
```

### 示例2: 内存高占用告警

```yaml
alerts:
  rules:
    - name: high_memory
      enabled: true
      metric: memory_mb
      threshold: 2048         # 内存超过2GB
      duration: 300
      channels: ["wechat", "webhook"]
```

### 示例3: 进程级别告警

```yaml
alerts:
  rules:
    - name: docker_high_cpu
      enabled: true
      metric: cpu_percent
      threshold: 50
      duration: 180
      process: "docker"       # 只监控docker进程
      channels: ["dingtalk"]
```

---

## 🛠️ 命令行使用

### 启动Web服务器
```bash
# 默认端口8080
./process-tracker web

# 自定义端口
./process-tracker web --port 8081

# 自定义主机和端口
./process-tracker web --host 0.0.0.0 --port 8080
```

### 启动监控+Web
```bash
# 监控 + Web一起启动
./process-tracker start --web

# 自定义Web端口
./process-tracker start --web --web-port 8081

# 自定义监控间隔
./process-tracker start --web --interval 10
```

### 仅启动监控 (不启动Web)
```bash
./process-tracker start
```

---

## 🔍 API端点

Web服务器提供以下API端点:

| 端点 | 说明 | 示例 |
|------|------|------|
| `GET /` | Web Dashboard主页 | http://localhost:8080 |
| `GET /api/stats/today` | 今日统计 | http://localhost:8080/api/stats/today |
| `GET /api/stats/week` | 本周统计 | http://localhost:8080/api/stats/week |
| `GET /api/stats/month` | 本月统计 | http://localhost:8080/api/stats/month |
| `GET /api/live` | 实时数据 | http://localhost:8080/api/live |
| `GET /api/processes?sort=cpu` | 进程列表 | http://localhost:8080/api/processes |
| `GET /api/health` | 健康检查 | http://localhost:8080/api/health |

### API响应示例

**GET /api/stats/today**
```json
{
  "process_count": 142,
  "active_count": 87,
  "avg_cpu": 12.5,
  "max_cpu": 45.2,
  "total_memory": 2048.5,
  "max_memory": 3200.1,
  "timeline": [
    {"time": "14:00", "cpu": 10.5, "memory": 1800.2},
    {"time": "15:00", "cpu": 15.2, "memory": 2100.5}
  ],
  "top_processes": [
    {
      "pid": 12345,
      "name": "chrome",
      "cpu_percent": 25.5,
      "memory_mb": 512.3,
      "status": "active",
      "uptime": "2h30m"
    }
  ]
}
```

---

## 🎨 移动端访问

Web Dashboard是响应式设计，支持移动端访问:

1. 确保手机和服务器在同一网络
2. 修改配置: `web.host: "0.0.0.0"` (允许外部访问)
3. 访问: `http://服务器IP:8080`

**安全提示**: 生产环境建议使用反向代理 (Nginx) + SSL

---

## ⚙️ 高级配置

### 告警抑制

避免频繁通知:
```yaml
alerts:
  suppress_duration: 30  # 30分钟内不重复发送相同告警
```

### 多通道通知

同时发送到多个渠道:
```yaml
alerts:
  rules:
    - name: critical_alert
      channels: ["dingtalk", "wechat", "webhook"]
```

### 禁用某个规则

```yaml
alerts:
  rules:
    - name: some_rule
      enabled: false  # 暂时禁用
```

---

## 🐛 故障排查

### Web界面无法访问

**问题**: 浏览器显示"无法访问此网站"

**解决**:
1. 检查进程是否启动: `ps aux | grep process-tracker`
2. 检查端口是否被占用: `netstat -tlnp | grep 8080`
3. 查看日志: `./process-tracker web` 查看错误信息

### 告警不发送

**问题**: 触发阈值但没收到通知

**解决**:
1. 检查配置: `alerts.enabled = true`
2. 检查Webhook URL是否正确
3. 测试通知器: 在钉钉群发送测试消息
4. 查看日志: 查找"告警已发送"或错误信息

### 钉钉机器人报错

**问题**: "sign not match" 或 "invalid sign"

**解决**:
- 检查secret配置是否正确
- 确保时间同步: `ntpdate time.apple.com`

---

## 📈 性能影响

**资源占用**:
- 内存: 增加 ~25MB (总计 ~40-50MB)
- CPU: 增加 ~0.5% (Web服务器)
- 磁盘: 无额外占用

**建议**:
- Web端口仅监听localhost (默认设置)
- 告警抑制时长设置合理值 (推荐30分钟)
- 定期清理旧数据: `./process-tracker clear-data`

---

## 🎯 下一步

- [ ] 探索Web Dashboard各项功能
- [ ] 配置适合你的告警规则
- [ ] 集成到现有监控体系
- [ ] 提供反馈和建议

---

## 📚 相关文档

- [README.md](README.md) - 项目主文档
- [DEEP_ANALYSIS.md](DEEP_ANALYSIS.md) - 深度技术分析
- [ALERT_COMPARISON.md](ALERT_COMPARISON.md) - 告警方式对比
- [config.example.yaml](config.example.yaml) - 配置文件示例

---

## 💬 获取帮助

遇到问题? 查看:
1. 项目README
2. GitHub Issues
3. 日志输出 (运行时查看控制台)

**祝使用愉快！** 🎉
