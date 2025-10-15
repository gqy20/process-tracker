# 兼容性说明

## GLIBC依赖问题（已解决）

### 问题描述

早期版本（v0.3.9及之前）使用动态链接编译，依赖系统GLIBC版本。在较旧系统上运行会遇到：

```bash
$ ./process-tracker version
process-tracker: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.34' not found
process-tracker: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.32' not found
```

### 解决方案（v0.4.0+）

**从v0.4.0开始，所有二进制文件都使用静态编译**，完全不依赖系统库。

**技术细节**：
- 编译选项：`CGO_ENABLED=0`
- 链接方式：`statically linked`
- 优势：
  - ✅ 可在任何Linux发行版运行（不限GLIBC版本）
  - ✅ 无需安装依赖库
  - ✅ 文件更小（压缩后约3-4MB）
  - ✅ 更易分发和部署

**验证静态链接**：
```bash
$ ldd process-tracker
不是动态可执行文件  # 说明是纯静态二进制

$ file process-tracker
process-tracker: ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked, stripped
```

---

## 平台兼容性

### Linux

**最低要求**：
- 架构：x86_64 (AMD64) 或 ARM64
- 内核：Linux 2.6.32+（任何发行版）
- GLIBC：无要求（静态编译）

**测试通过的发行版**：
- ✅ Ubuntu 16.04+
- ✅ Debian 8+
- ✅ CentOS 7+
- ✅ RHEL 7+
- ✅ Fedora 任何版本
- ✅ Alpine Linux
- ✅ Arch Linux

### macOS

**最低要求**：
- macOS 10.13+ (High Sierra)
- Intel (x86_64) 或 Apple Silicon (ARM64)

**已知问题**：
- UPX压缩在macOS上可能失败（不影响功能）
- macOS构建未压缩，文件较大（约9MB）

### Windows

**最低要求**：
- Windows 7+
- 架构：x86_64 (AMD64)

**已知问题**：
- Windows Defender可能误报（可添加白名单）
- 需要管理员权限访问某些进程信息

---

## Docker支持

**容器监控需求**：
- Docker API访问权限
- 用户需在`docker`组中

**启用Docker监控**：
```bash
# 添加用户到docker组
sudo usermod -aG docker $USER

# 重新登录或刷新组
newgrp docker

# 验证权限
docker ps
```

**无Docker环境**：
- 监控程序自动检测Docker环境
- 无Docker时自动禁用容器监控
- 不影响普通进程监控功能

---

## 权限要求

### Linux/macOS

**普通权限**（推荐）：
- ✅ 监控当前用户进程
- ✅ 基本系统统计（CPU/内存总量）
- ❌ 无法监控其他用户进程
- ❌ 无法读取某些进程详细信息

**Root权限**：
- ✅ 监控所有进程
- ✅ 读取所有进程详细信息（命令行、工作目录）
- ⚠️ 安全风险：谨慎使用

**建议**：
```bash
# 开发环境：普通权限足够
./process-tracker start

# 生产环境：考虑使用sudo
sudo ./process-tracker start
```

### Windows

**普通权限**：
- ✅ 监控当前用户进程
- ❌ 无法监控系统服务

**管理员权限**：
- ✅ 监控所有进程和服务
- ✅ 读取完整进程信息

---

## 网络要求

**基础功能**：
- ❌ 无需网络连接
- 所有数据本地存储

**可选功能**：
- 🌐 Web界面需要开放端口（默认8080）
- 🔔 告警通知需要网络访问（飞书/钉钉/企业微信）

**防火墙配置**（如需Web界面）：
```bash
# Ubuntu/Debian
sudo ufw allow 8080/tcp

# CentOS/RHEL
sudo firewall-cmd --add-port=8080/tcp --permanent
sudo firewall-cmd --reload
```

---

## 存储要求

**最小磁盘空间**：
- 程序本身：3-9 MB（取决于平台和压缩）
- 数据存储：可配置（默认100MB限制）
- 推荐预留：500 MB

**数据目录**：
- Linux/macOS: `~/.process-tracker/`
- Windows: `%USERPROFILE%\.process-tracker\`

**自动清理**：
- 默认保留7天数据
- 自动轮转和压缩
- 配置：`storage.keep_days`

---

## 性能影响

**CPU占用**：
- 采集时：<1% (5秒间隔)
- 空闲时：0%

**内存占用**：
- 基础：10-20 MB
- 缓冲：+5 MB（100条记录）
- Web界面：+10 MB

**磁盘I/O**：
- 写入频率：每5秒或缓冲满时
- 批量写入减少I/O压力
- 压缩后存储节省90%空间

---

## 升级兼容性

### 数据格式

**向后兼容**：
- v0.4.0可读取v0.3.x的所有数据
- 自动识别数据格式版本
- 无需数据迁移

**数据格式演进**：
- v5 (16字段)：基础格式
- v6 (17字段)：添加MemoryPercent
- v7 (18字段)：添加CPUPercentNormalized

### 配置文件

**兼容性**：
- v0.4.0兼容v0.3.x配置文件
- 新增配置项使用默认值
- 弃用配置项自动忽略

**建议**：
- 升级后检查配置：`./process-tracker config`
- 参考：`config-example.yaml`

---

## 故障排查

### 问题1：GLIBC版本错误

```bash
# 错误信息
process-tracker: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.34' not found

# 解决方案
# 升级到v0.4.0+（静态编译版本）
wget https://github.com/yourusername/process-tracker/releases/download/v0.4.0/process-tracker-linux-amd64
chmod +x process-tracker-linux-amd64
./process-tracker-linux-amd64 version
```

### 问题2：权限不足

```bash
# 症状：某些进程无法监控

# 临时解决（单次运行）
sudo ./process-tracker start

# 永久解决（systemd服务）
sudo cp process-tracker /usr/local/bin/
sudo systemctl edit --force --full process-tracker.service
```

### 问题3：Docker监控失败

```bash
# 检查Docker权限
docker ps

# 如果失败，添加用户到docker组
sudo usermod -aG docker $USER
newgrp docker

# 验证
docker ps
./process-tracker start
```

### 问题4：Web界面无法访问

```bash
# 检查监控是否运行
./process-tracker status

# 检查端口占用
netstat -tuln | grep 8080

# 启动Web界面
./process-tracker web --port 8080

# 访问（替换为实际IP）
curl http://localhost:8080/api/health
```

---

## 技术规格

| 项目 | 规格 |
|-----|------|
| **编程语言** | Go 1.21+ |
| **编译方式** | 静态编译（CGO_ENABLED=0） |
| **依赖库** | 无（纯静态） |
| **支持架构** | x86_64, ARM64 |
| **支持系统** | Linux, macOS, Windows |
| **最小内核** | Linux 2.6.32+ |
| **文件大小** | 3-9 MB（压缩后） |
| **启动时间** | <1秒 |
| **采集间隔** | 5秒（可配置） |
| **数据格式** | CSV（兼容Excel） |
| **配置格式** | YAML |

---

## 获取帮助

- 📖 文档：[README.md](../README.md)
- 🐛 问题反馈：[GitHub Issues](https://github.com/yourusername/process-tracker/issues)
- 💬 讨论：[GitHub Discussions](https://github.com/yourusername/process-tracker/discussions)

---

**最后更新**: v0.4.0 (2025-01)
