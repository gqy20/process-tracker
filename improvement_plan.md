# 进程统计改进方案

## 当前问题
1. 进程名称过于简化，无法区分相同名称的不同应用
2. 缺乏应用上下文信息
3. 统计结果不够精确

## 改进方案

### 方案1: 完整命令行路径
```go
func (a *App) getProcessName(p *process.Process) (string, error) {
    cmd, err := p.Cmdline()
    if err != nil {
        return p.Name()
    }
    
    // 提取有意义的路径信息
    if len(cmd) > 100 {
        // 截取前100个字符，避免过长
        return cmd[:100]
    }
    return cmd
}
```

### 方案2: 基于工作目录的应用分类
```go
func (a *App) categorizeByWorkingDir(p *process.Process) (string, error) {
    cwd, err := p.Cwd()
    if err != nil {
        return p.Name()
    }
    
    // 提取项目名作为分类
    parts := strings.Split(cwd, "/")
    if len(parts) > 0 {
        projectName := parts[len(parts)-1]
        return fmt.Sprintf("%s (%s)", p.Name(), projectName)
    }
    return p.Name()
}
```

### 方案3: 分层统计体系
```go
type ProcessCategory struct {
    PrimaryName   string // 主要名称 (python, java, node)
    SecondaryName string // 次要名称 (项目名、包名)
    FullPath      string // 完整路径
    Pid           int32  // 进程ID
}

func (a *App) createProcessCategory(p *process.Process) ProcessCategory {
    name, _ := p.Name()
    cmd, _ := p.Cmdline()
    cwd, _ := p.Cwd()
    
    return ProcessCategory{
        PrimaryName:   name,
        SecondaryName: a.extractProjectName(cmd, cwd),
        FullPath:      cmd,
        Pid:           p.Pid,
    }
}
```

### 方案4: 智能应用识别
```go
func (a *App) identifyApplication(p *process.Process) string {
    cmd, _ := p.Cmdline()
    
    // 识别特定应用
    switch {
    case strings.Contains(cmd, "tika-server"):
        return "Tika Server"
    case strings.Contains(cmd, "@modelcontextprotocol"):
        return "MCP Sequential Thinking"
    case strings.Contains(cmd, "npm exec"):
        return extractNpmPackageName(cmd)
    case strings.Contains(cmd, "uv run"):
        return extractPythonProjectName(cmd)
    default:
        return p.Name()
    }
}
```

## 实施建议

1. **渐进式改进**: 先实现方案1，再逐步添加其他方案
2. **用户配置**: 允许用户选择统计粒度
3. **性能考虑**: 缓存进程信息，避免频繁系统调用
4. **兼容性**: 保持向后兼容，支持多种统计方式