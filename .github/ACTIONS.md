# GitHub Actions 自动发布工作流

## 概述

项目配置了 GitHub Actions 工作流，实现每次提交新版本时自动构建多平台可执行文件并创建 GitHub Release。

## 工作流程

### 1. 触发条件
- **Push 到 main 分支**: 检测版本变化
- **Pull Request 到 main 分支**: 仅构建测试，不创建 release

### 2. 版本检测
- 从 `main.go` 文件中提取版本号
- 检查是否已存在对应 tag
- 仅对新版本创建 release

### 3. 多平台构建
支持以下平台：
- Linux AMD64
- Linux ARM64
- macOS Intel (AMD64)
- macOS ARM64 (Apple Silicon)
- Windows AMD64

### 4. 自动发布
- 创建 GitHub Release
- 上传所有平台构建文件
- 生成发布说明
- 自动创建和推送 tag

## 文件结构

```
.github/
└── workflows/
    └── release.yml                    # GitHub Actions 工作流
test-github-action.sh                  # 本地测试脚本
```

## 工作流详情

### Jobs

#### 1. version-check
- **目的**: 检测版本号和是否为新版本
- **输出**: version, is-new-version
- **逻辑**: 从 main.go 提取版本，检查 git tags

#### 2. build
- **目的**: 为所有支持平台构建可执行文件
- **矩阵**: 多平台交叉编译
- **输出**: 上传构建 artifacts

#### 3. create-release
- **目的**: 创建 GitHub Release
- **条件**: 仅当 is-new-version = true
- **功能**: 
  - 下载所有构建 artifacts
  - 重命名文件包含平台信息
  - 创建 GitHub Release
  - 上传所有平台文件
  - 自动创建 tag

## 使用方法

### 1. 版本更新
更新 `main.go` 中的版本号：

```go
var Version = "0.3.8"  // 修改版本号
```

### 2. 提交更改
```bash
git add main.go
git commit -m "feat: Update version to 0.3.8"
git push origin main
```

### 3. 自动发布
GitHub Actions 会自动：
- 检测到新版本
- 构建所有平台
- 创建 release
- 推送 tag

### 4. 查看结果
- 访问仓库的 Releases 页面
- 查看新创建的 release
- 下载对应平台的文件

## 测试工作流

### 本地测试
使用提供的测试脚本：

```bash
./test-github-action.sh
```

### GitHub Actions 测试
1. 创建 test 分支
2. 修改版本号
3. 提交并推送到 origin
4. 查看 Actions 页面结果

## 发布文件命名约定

```
process-tracker-linux-amd64        # Linux Intel/AMD 64位
process-tracker-linux-arm64        # Linux ARM 64位
process-tracker-macos-amd64        # macOS Intel 64位
process-tracker-macos-arm64        # macOS ARM64 (Apple Silicon)
process-tracker-windows-amd64.exe  # Windows Intel/AMD 64位
```

## 环境要求

### GitHub Actions
- Go 1.21
- Ubuntu, macOS, Windows runners
- GitHub Token (自动提供)

### 本地测试
- Go 1.21+
- Git
- Bash shell

## 故障排除

### 常见问题

#### 1. 构建失败
- 检查 Go 版本兼容性
- 验证代码语法
- 查看构建日志

#### 2. Release 创建失败
- 检查 GitHub Token 权限
- 验证版本号格式
- 确认 tag 不存在

#### 3. 文件上传失败
- 检查文件大小限制
- 验证文件命名
- 确认文件存在

### 调试技巧

#### 本地调试
```bash
# 手动运行版本检测
grep -o 'var Version = "[^"]*"' main.go | sed 's/var Version = "\(.*\)"/\1/'

# 手动构建
go build -ldflags="-X main.Version=0.3.7" -o process-tracker .

# 手动测试
./process-tracker version
```

#### GitHub Actions 调试
- 查看 Actions 页面日志
- 使用 `echo` 输出调试信息
- 分步测试工作流

## 安全考虑

### 权限管理
- 使用默认的 GITHUB_TOKEN
- 不需要额外的密钥
- 仓库级别的操作权限

### 文件验证
- 自动签名（通过 GitHub Actions）
- 文件完整性检查
- 版本号验证

## 维护

### 工作流更新
- 编辑 `.github/workflows/release.yml`
- 提交更改到 main 分支
- 测试新工作流

### 版本管理
- 遵循语义化版本号
- 及时更新版本号
- 清理过时的 releases

---

🤖 *此文档由 Claude Code 自动生成*