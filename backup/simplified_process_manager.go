package core

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"github.com/shirou/gopsutil/v3/process"
)

// SimplifiedProcessManager 简化的进程管理器
type SimplifiedProcessManager struct {
	app                 *App
	config              ProcessManagerConfig
	monitor             *UnifiedMonitor
	resourceCollector   UnifiedResourceCollector
	mutex               sync.RWMutex
	ctx                 context.Context
	cancel              context.CancelFunc
}


// ExtendedProcessInfo 扩展的进程信息（包含管理器特有字段）
type ExtendedProcessInfo struct {
	ProcessInfo  // 嵌入app.go中的ProcessInfo
	Status          string              `json:"status"`
	StartTime       time.Time           `json:"start_time"`
	LastSeen        time.Time           `json:"last_seen"`
	ResourceUsage   *ResourceUsage      `json:"resource_usage"`
	Health          *HealthStatus       `json:"health"`
	Tags            map[string]string   `json:"tags"`
	Group           string              `json:"group"`
	Priority        string              `json:"priority"`
}

// ProcessGroup 简化的进程组定义
type SimplifiedProcessGroup struct {
	Name        string   `yaml:"name"`
	Pattern     string   `yaml:"pattern"`
	Description string   `yaml:"description"`
	PIDs        []int32  `yaml:"pids"`
	AutoManage  bool     `yaml:"auto_manage"`
	QuotaName   string   `yaml:"quota_name"`
	Tags        []string `yaml:"tags"`
	Priority    string   `yaml:"priority"`
}

// SimplifiedProcessEvent 简化的进程事件
type SimplifiedProcessEvent struct {
	Type      EventType `json:"type"`
	PID       int32     `json:"pid"`
	Name      string    `json:"name"`
	Group     string    `json:"group"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Details   *ExtendedProcessInfo `json:"details,omitempty"`
}

// NewSimplifiedProcessManager 创建简化的进程管理器
func NewSimplifiedProcessManager(config ProcessManagerConfig, app *App) *SimplifiedProcessManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	// 创建统一资源收集器配置
	collectorConfig := ResourceCollectionConfig{
		EnableCPUMonitoring:    true,
		EnableMemoryMonitoring:  true,
		EnableIOMonitoring:      true,
		EnableNetworkMonitoring: false,
		EnableThreadMonitoring:  true,
		EnableDetailedIO:        false,
		CollectionInterval:      config.DiscoveryInterval,
		CacheTTL:               config.DiscoveryInterval * 2,
		MaxCacheSize:           1000,
		EnableHistory:          false,
		HistoryRetention:       time.Hour,
	}
	
	spm := &SimplifiedProcessManager{
		app:               app,
		config:            config,
		resourceCollector: NewUnifiedResourceCollector(collectorConfig),
		ctx:               ctx,
		cancel:            cancel,
	}
	
	// 初始化统一监控器
	if config.EnableUnifiedMonitor {
		spm.monitor = NewUnifiedMonitor(config.MonitoringConfig, app)
	}
	
	return spm
}

// Start 启动进程管理器
func (spm *SimplifiedProcessManager) Start() error {
	if !spm.config.Enabled {
		return nil
	}
	
	log.Println("🚀 启动简化进程管理器...")
	
	// 启动统一监控器
	if spm.monitor != nil {
		if err := spm.monitor.Start(); err != nil {
			return fmt.Errorf("启动统一监控器失败: %w", err)
		}
	}
	
	// 启动进程发现循环
	if spm.config.AutoDiscovery {
		go spm.discoveryLoop()
	}
	
	return nil
}

// Stop 停止进程管理器
func (spm *SimplifiedProcessManager) Stop() {
	spm.cancel()
	
	if spm.monitor != nil {
		spm.monitor.Stop()
	}
	
	// 清理资源收集器
	spm.resourceCollector.InvalidateAllCache()
	
	log.Println("🛑 简化进程管理器已停止")
}

// discoveryLoop 进程发现循环
func (spm *SimplifiedProcessManager) discoveryLoop() {
	ticker := time.NewTicker(spm.config.DiscoveryInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-spm.ctx.Done():
			return
		case <-ticker.C:
			spm.discoverProcesses()
		}
	}
}

// discoverProcesses 发现进程
func (spm *SimplifiedProcessManager) discoverProcesses() {
	// 获取所有系统进程
	processes, err := process.Processes()
	if err != nil {
		log.Printf("❌ 获取系统进程失败: %v", err)
		return
	}
	
	// 统计发现的进程
	discoveredCount := 0
	
	for _, p := range processes {
		pid := p.Pid
		
		// 跳过系统进程和自身
		if pid <= 1 || pid == int32(spm.app.GetPID()) {
			continue
		}
		
		// 获取进程信息
		if info := spm.getProcessInfo(p); info != nil {
			// 检查是否匹配模式
			if group := spm.matchProcessGroup(info); group != "" {
				info.Group = group
				info.Tags = spm.getGroupTags(group)
				
				// 添加到统一监控器
				if spm.monitor != nil {
					spm.addToMonitor(info)
				}
				
				// 添加到配额管理（如果启用）
				if spm.config.EnableQuota && group != "" {
					spm.addToQuota(info, group)
				}
				
				discoveredCount++
			}
		}
	}
	
	// 清理丢失的进程
	if spm.monitor != nil {
		spm.cleanupLostProcesses()
	}
	
	log.Printf("🔍 进程发现完成: 发现 %d 个匹配的进程", discoveredCount)
}

// getProcessInfo 获取进程信息
func (spm *SimplifiedProcessManager) getProcessInfo(p *process.Process) *ExtendedProcessInfo {
	info := &ExtendedProcessInfo{}
	info.Pid = p.Pid
	info.Tags = make(map[string]string)
	
	// 获取进程名
	if name, err := p.Name(); err == nil {
		info.Name = name
	} else {
		return nil
	}
	
	// 获取命令行
	if cmdline, err := p.Cmdline(); err == nil {
		info.Cmdline = cmdline
	}
	
	// 获取启动时间
	if createTime, err := p.CreateTime(); err == nil {
		info.StartTime = time.Unix(createTime/1000, 0)
		info.LastSeen = time.Now()
	}
	
	// 获取状态
	if status, err := p.Status(); err == nil {
		if len(status) > 0 {
			info.Status = status[0] // Take first status string
		} else {
			info.Status = "unknown"
		}
	}
	
	// 获取资源使用情况
	info.ResourceUsage = spm.collectResourceUsage(p)
	
	return info
}

// collectResourceUsage 收集资源使用情况（使用统一收集器）
func (spm *SimplifiedProcessManager) collectResourceUsage(p *process.Process) *ResourceUsage {
	// 使用统一资源收集器，它内置了回退机制
	unifiedUsage, err := spm.resourceCollector.CollectProcessResources(p.Pid)
	if err != nil {
		// 记录错误但返回基本的ResourceUsage
		log.Printf("⚠️  统一资源收集器失败 PID %d: %v", p.Pid, err)
		return &ResourceUsage{
			CPUUsed:        0,
			MemoryUsedMB:   0,
			DiskReadMB:     0,
			DiskWriteMB:    0,
			PerformanceScore: 0,
		}
	}
	
	// 转换为ResourceUsage以保持兼容性
	usage := &ResourceUsage{
		CPUUsed:        unifiedUsage.CPU.UsedPercent,
		MemoryUsedMB:   int64(unifiedUsage.Memory.UsedMB),
		DiskReadMB:     int64(unifiedUsage.Disk.ReadMB),
		DiskWriteMB:    int64(unifiedUsage.Disk.WriteMB),
		PerformanceScore: unifiedUsage.Performance.Score,
	}
	
	// 添加异常信息
	for _, anomaly := range unifiedUsage.Anomalies {
		if usage.AnomalyType == nil {
			usage.AnomalyType = []string{}
		}
		usage.AnomalyType = append(usage.AnomalyType, anomaly.Type)
		if usage.LastAnomaly.IsZero() || anomaly.Timestamp.After(usage.LastAnomaly) {
			usage.LastAnomaly = anomaly.Timestamp
		}
	}
	
	return usage
}


// matchProcessGroup 匹配进程组
func (spm *SimplifiedProcessManager) matchProcessGroup(info *ExtendedProcessInfo) string {
	// 检查排除模式
	for _, pattern := range spm.config.ExcludePatterns {
		if matched, _ := regexp.MatchString(pattern, info.Name); matched {
			return ""
		}
		if matched, _ := regexp.MatchString(pattern, info.Cmdline); matched {
			return ""
		}
	}
	
	// 检查包含模式
	for _, pattern := range spm.config.ProcessPatterns {
		if matched, _ := regexp.MatchString(pattern, info.Name); matched {
			return fmt.Sprintf("pattern_%s", pattern)
		}
		if matched, _ := regexp.MatchString(pattern, info.Cmdline); matched {
			return fmt.Sprintf("pattern_%s", pattern)
		}
	}
	
	// 生物信息学工具识别
	if bioGroup := spm.identifyBioTool(info); bioGroup != "" {
		return bioGroup
	}
	
	return ""
}

// identifyBioTool 识别生物信息学工具
func (spm *SimplifiedProcessManager) identifyBioTool(info *ExtendedProcessInfo) string {
	name := strings.ToLower(info.Name)
	cmdline := strings.ToLower(info.Cmdline)
	
	// BLAST系列
	if strings.Contains(name, "blast") || strings.Contains(cmdline, "blast") {
		return "blast"
	}
	
	// BWA
	if strings.Contains(name, "bwa") || strings.Contains(cmdline, "bwa") {
		return "bwa"
	}
	
	// SAMtools
	if strings.Contains(name, "samtools") || strings.Contains(cmdline, "samtools") {
		return "samtools"
	}
	
	// GATK
	if strings.Contains(name, "gatk") || strings.Contains(cmdline, "gatk") {
		return "gatk"
	}
	
	// FastQC
	if strings.Contains(name, "fastqc") || strings.Contains(cmdline, "fastqc") {
		return "fastqc"
	}
	
	// Trimmomatic
	if strings.Contains(name, "trimmomatic") || strings.Contains(cmdline, "trimmomatic") {
		return "trimmomatic"
	}
	
	// HISAT2
	if strings.Contains(name, "hisat2") || strings.Contains(cmdline, "hisat2") {
		return "hisat2"
	}
	
	// Cufflinks
	if strings.Contains(name, "cufflinks") || strings.Contains(cmdline, "cufflinks") {
		return "cufflinks"
	}
	
	// R脚本
	if strings.Contains(name, "rscript") || strings.Contains(cmdline, "rscript") {
		return "rscript"
	}
	
	// Python生物信息学脚本
	if strings.Contains(name, "python") && (strings.Contains(cmdline, "bio") || 
		strings.Contains(cmdline, "sequenc") || strings.Contains(cmdline, "align")) {
		return "python_bio"
	}
	
	return ""
}

// getGroupTags 获取组标签
func (spm *SimplifiedProcessManager) getGroupTags(group string) map[string]string {
	tags := make(map[string]string)
	
	// 通用标签
	tags["group"] = group
	tags["auto_discovered"] = "true"
	
	// 生物信息学工具特定标签
	switch group {
	case "blast", "bwa", "samtools", "gatk", "fastqc", "trimmomatic", "hisat2", "cufflinks":
		tags["category"] = "bioinformatics"
		tags["tool_type"] = "analysis"
	case "rscript":
		tags["category"] = "bioinformatics"
		tags["tool_type"] = "statistics"
	case "python_bio":
		tags["category"] = "bioinformatics"
		tags["tool_type"] = "scripting"
	}
	
	return tags
}

// addToMonitor 添加到监控器
func (spm *SimplifiedProcessManager) addToMonitor(info *ExtendedProcessInfo) {
	if spm.monitor == nil {
		return
	}
	
	// 创建监控配置
	monitorConfig := ProcessMonitorConfig{
		EnableMonitoring:  true,
		EnableHealthCheck: true,
		Tags:             info.Tags,
		Priority:         info.Priority,
	}
	
	// 根据组设置特定配置
	switch info.Group {
	case "blast", "bwa", "gatk":
		monitorConfig.Priority = "high"
		monitorConfig.EnableResourceLimit = true
		monitorConfig.ResourceLimits = map[string]float64{
			"cpu":    80.0,
			"memory": 8192.0,
		}
	case "samtools", "fastqc", "trimmomatic":
		monitorConfig.Priority = "medium"
		monitorConfig.EnableResourceLimit = true
		monitorConfig.ResourceLimits = map[string]float64{
			"cpu":    60.0,
			"memory": 4096.0,
		}
	default:
		monitorConfig.Priority = "low"
	}
	
	// 将进程添加到统一监控器
	// 这里需要调用统一监控器的相应方法
	// spm.monitor.AddProcess(info.Pid, monitorConfig)
}

// addToQuota 添加到配额管理
func (spm *SimplifiedProcessManager) addToQuota(info *ExtendedProcessInfo, group string) {
	if spm.app.QuotaManager == nil {
		return
	}
	
	// 生物信息学工具的配额组映射
	quotaGroups := map[string]string{
		"blast":       "bio_tools_high",
		"bwa":         "bio_tools_high", 
		"gatk":        "bio_tools_high",
		"samtools":    "bio_tools_medium",
		"fastqc":      "bio_tools_low",
		"trimmomatic": "bio_tools_medium",
		"hisat2":      "bio_tools_high",
		"cufflinks":   "bio_tools_medium",
	}
	
	if quotaName, exists := quotaGroups[group]; exists {
		if err := spm.app.AddProcessToQuota(quotaName, info.Pid); err == nil {
			log.Printf("✅ 进程 %s (PID: %d) 已添加到配额组 %s", info.Name, info.Pid, quotaName)
		}
	}
}

// cleanupLostProcesses 清理丢失的进程
func (spm *SimplifiedProcessManager) cleanupLostProcesses() {
	// 这里需要调用统一监控器的清理功能
	// 统一监控器会自动处理丢失的进程
}

// GetAllProcesses 获取所有管理的进程
func (spm *SimplifiedProcessManager) GetAllProcesses() []*ExtendedProcessInfo {
	// 这里应该从统一监控器获取进程列表
	// 暂时返回空列表
	return []*ExtendedProcessInfo{}
}

// GetAllProcessesAsProcessInfo 获取所有管理的进程，返回ProcessInfo格式
func (spm *SimplifiedProcessManager) GetAllProcessesAsProcessInfo() []*ProcessInfo {
	extendedProcesses := spm.GetAllProcesses()
	processes := make([]*ProcessInfo, len(extendedProcesses))
	
	for i, extProc := range extendedProcesses {
		processInfo := &ProcessInfo{
			Pid:         extProc.Pid,
			Name:        extProc.Name,
			Cmdline:     extProc.Cmdline,
			Cwd:         "", // Not available in ExtendedProcessInfo
			CPUPercent:  0, // Will be filled by resource usage if available
			MemoryMB:    0, // Will be filled by resource usage if available
			Threads:     0, // Will be filled by resource usage if available
			DiskReadMB:  0, // Will be filled by resource usage if available
			DiskWriteMB: 0, // Will be filled by resource usage if available
			NetSentKB:   0, // Will be filled by resource usage if available
			NetRecvKB:   0, // Will be filled by resource usage if available
		}
		
		// Fill in resource usage if available
		if extProc.ResourceUsage != nil {
			processInfo.CPUPercent = extProc.ResourceUsage.CPUUsed
			processInfo.MemoryMB = float64(extProc.ResourceUsage.MemoryUsedMB)
			processInfo.DiskReadMB = float64(extProc.ResourceUsage.DiskReadMB)
			processInfo.DiskWriteMB = float64(extProc.ResourceUsage.DiskWriteMB)
			processInfo.NetSentKB = float64(extProc.ResourceUsage.NetworkInKB)
			processInfo.NetRecvKB = float64(extProc.ResourceUsage.NetworkOutKB)
		}
		
		processes[i] = processInfo
	}
	
	return processes
}

// GetProcessGroupsAsMap 获取进程组，返回map格式
func (spm *SimplifiedProcessManager) GetProcessGroupsAsMap() map[string]*SimplifiedProcessGroup {
	groups := spm.GetProcessGroups()
	groupMap := make(map[string]*SimplifiedProcessGroup)
	
	for i := range groups {
		groupMap[groups[i].Name] = &groups[i]
	}
	
	return groupMap
}

// GetProcessesByGroupAsProcessInfo 按组获取进程，返回ProcessInfo格式
func (spm *SimplifiedProcessManager) GetProcessesByGroupAsProcessInfo(group string) []*ProcessInfo {
	extendedProcesses := spm.GetProcessesByGroup(group)
	processes := make([]*ProcessInfo, len(extendedProcesses))
	
	for i, extProc := range extendedProcesses {
		processInfo := &ProcessInfo{
			Pid:         extProc.Pid,
			Name:        extProc.Name,
			Cmdline:     extProc.Cmdline,
			Cwd:         "", // Not available in ExtendedProcessInfo
			CPUPercent:  0, // Will be filled by resource usage if available
			MemoryMB:    0, // Will be filled by resource usage if available
			Threads:     0, // Will be filled by resource usage if available
			DiskReadMB:  0, // Will be filled by resource usage if available
			DiskWriteMB: 0, // Will be filled by resource usage if available
			NetSentKB:   0, // Will be filled by resource usage if available
			NetRecvKB:   0, // Will be filled by resource usage if available
		}
		
		// Fill in resource usage if available
		if extProc.ResourceUsage != nil {
			processInfo.CPUPercent = extProc.ResourceUsage.CPUUsed
			processInfo.MemoryMB = float64(extProc.ResourceUsage.MemoryUsedMB)
			processInfo.DiskReadMB = float64(extProc.ResourceUsage.DiskReadMB)
			processInfo.DiskWriteMB = float64(extProc.ResourceUsage.DiskWriteMB)
			processInfo.NetSentKB = float64(extProc.ResourceUsage.NetworkInKB)
			processInfo.NetRecvKB = float64(extProc.ResourceUsage.NetworkOutKB)
		}
		
		processes[i] = processInfo
	}
	
	return processes
}

// AddCustomGroupWithQuota 添加自定义进程组（带配额参数）
func (spm *SimplifiedProcessManager) AddCustomGroupWithQuota(name, pattern string, autoManage bool, quotaName string) error {
	// For now, ignore quotaName parameter and call the original method
	return spm.AddCustomGroup(name, pattern, autoManage)
}

// GetProcessesByGroup 按组获取进程
func (spm *SimplifiedProcessManager) GetProcessesByGroup(group string) []*ExtendedProcessInfo {
	var result []*ExtendedProcessInfo
	
	// 这里应该从统一监控器按组过滤进程
	// 暂时返回空列表
	return result
}

// GetProcessStats 获取进程统计
func (spm *SimplifiedProcessManager) GetProcessStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_processes":    0,
		"by_group":           make(map[string]int),
		"by_status":          make(map[string]int),
		"total_cpu_usage":    0.0,
		"total_memory_usage": int64(0),
	}
	
	// 这里应该从统一监控器获取统计数据
	// 暂时返回基本统计
	return stats
}

// ControlProcess 控制进程
func (spm *SimplifiedProcessManager) ControlProcess(pid int32, action string) error {
	if !spm.config.EnableControl {
		return fmt.Errorf("进程控制未启用")
	}
	
	p, err := process.NewProcess(pid)
	if err != nil {
		return fmt.Errorf("进程不存在: %w", err)
	}
	
	switch action {
	case "stop", "kill":
		return p.Kill()
	case "pause", "suspend":
		return p.Suspend()
	case "resume":
		return p.Resume()
	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}
}

// AddCustomGroup 添加自定义进程组
func (spm *SimplifiedProcessManager) AddCustomGroup(name, pattern string, autoManage bool) error {
	// 添加自定义模式到配置
	spm.config.ProcessPatterns = append(spm.config.ProcessPatterns, pattern)
	log.Printf("✅ 添加自定义进程组: %s (模式: %s)", name, pattern)
	return nil
}

// RemoveCustomGroup 移除自定义进程组
func (spm *SimplifiedProcessManager) RemoveCustomGroup(name string) error {
	// 从配置中移除模式
	// 这里需要实现更复杂的逻辑来匹配和移除特定模式
	log.Printf("✅ 移除自定义进程组: %s", name)
	return nil
}

// GetProcessGroups 获取所有进程组
func (spm *SimplifiedProcessManager) GetProcessGroups() []SimplifiedProcessGroup {
	groups := []SimplifiedProcessGroup{}
	
	// 预定义的生物信息学工具组
	bioGroups := []SimplifiedProcessGroup{
		{
			Name:        "blast",
			Pattern:     "blast.*",
			Description: "BLAST sequence alignment tools",
			AutoManage:  true,
			Tags:        []string{"bioinformatics", "alignment"},
			Priority:    "high",
		},
		{
			Name:        "bwa",
			Pattern:     "bwa.*",
			Description: "Burrows-Wheeler Aligner",
			AutoManage:  true,
			Tags:        []string{"bioinformatics", "alignment"},
			Priority:    "high",
		},
		{
			Name:        "samtools",
			Pattern:     "samtools.*",
			Description: "SAM/BAM file manipulation tools",
			AutoManage:  true,
			Tags:        []string{"bioinformatics", "sam"},
			Priority:    "medium",
		},
		{
			Name:        "gatk",
			Pattern:     "gatk.*",
			Description: "Genome Analysis Toolkit",
			AutoManage:  true,
			Tags:        []string{"bioinformatics", "variant"},
			Priority:    "high",
		},
		{
			Name:        "fastqc",
			Pattern:     "fastqc",
			Description: "FastQC quality control tool",
			AutoManage:  true,
			Tags:        []string{"bioinformatics", "quality"},
			Priority:    "low",
		},
	}
	
	groups = append(groups, bioGroups...)
	
	return groups
}

// SearchProcesses 搜索进程
func (spm *SimplifiedProcessManager) SearchProcesses(query string) []*ExtendedProcessInfo {
	var results []*ExtendedProcessInfo
	
	// 这里应该从统一监控器搜索进程
	// 暂时返回空列表
	return results
}

// GetProcessDetails 获取进程详细信息
func (spm *SimplifiedProcessManager) GetProcessDetails(pid int32) (*ExtendedProcessInfo, error) {
	// 这里应该从统一监控器获取进程详情
	// 暂时返回错误
	return nil, fmt.Errorf("进程详情获取功能待实现")
}

// StartProcess starts a new process by name, command, and working directory
func (spm *SimplifiedProcessManager) StartProcess(name string, command []string, workingDir string) error {
	// For compatibility - this would need actual implementation to start a new process
	// The simplified manager currently only discovers existing processes
	return fmt.Errorf("StartProcess not implemented in simplified discovery manager")
}

// StopProcess stops a process by PID
func (spm *SimplifiedProcessManager) StopProcess(pid int32) error {
	return spm.ControlProcess(pid, "stop")
}

// RestartProcess restarts a process by PID
func (spm *SimplifiedProcessManager) RestartProcess(pid int32) error {
	// Try to restart the process
	if err := spm.ControlProcess(pid, "stop"); err != nil {
		return err
	}
	
	// Wait a moment before attempting to restart
	time.Sleep(1 * time.Second)
	
	// Note: In a full implementation, you'd restart with the original command
	// For now, just return that restart is not fully implemented
	return fmt.Errorf("RestartProcess partially implemented - process stopped but restart requires original command info")
}

// GetProcessByName gets process by name
func (spm *SimplifiedProcessManager) GetProcessByName(name string) (*ProcessInfo, error) {
	// Search through all processes for a matching name
	processes := spm.GetAllProcesses()
	for _, extProc := range processes {
		if extProc.Name == name {
			// Convert ExtendedProcessInfo to ProcessInfo
			processInfo := &ProcessInfo{
				Pid:         extProc.Pid,
				Name:        extProc.Name,
				Cmdline:     extProc.Cmdline,
				Cwd:         "", // Not available in ExtendedProcessInfo
				CPUPercent:  0, // Will be filled by resource usage if available
				MemoryMB:    0, // Will be filled by resource usage if available
				Threads:     0, // Will be filled by resource usage if available
				DiskReadMB:  0, // Will be filled by resource usage if available
				DiskWriteMB: 0, // Will be filled by resource usage if available
				NetSentKB:   0, // Will be filled by resource usage if available
				NetRecvKB:   0, // Will be filled by resource usage if available
			}
			
			// Fill in resource usage if available
			if extProc.ResourceUsage != nil {
				processInfo.CPUPercent = extProc.ResourceUsage.CPUUsed
				processInfo.MemoryMB = float64(extProc.ResourceUsage.MemoryUsedMB)
				processInfo.Threads = 0 // Threads not available in unified ResourceUsage
				processInfo.DiskReadMB = float64(extProc.ResourceUsage.DiskReadMB)
				processInfo.DiskWriteMB = float64(extProc.ResourceUsage.DiskWriteMB)
				processInfo.NetSentKB = float64(extProc.ResourceUsage.NetworkInKB)
				processInfo.NetRecvKB = float64(extProc.ResourceUsage.NetworkOutKB)
			}
			
			return processInfo, nil
		}
	}
	return nil, fmt.Errorf("process %s not found", name)
}

// Events returns the event channel for compatibility
func (spm *SimplifiedProcessManager) Events() <-chan SimplifiedProcessEvent {
	// Return a nil channel for now - in a full implementation this would return a real event channel
	return nil
}

// GetStats returns process statistics for compatibility
func (spm *SimplifiedProcessManager) GetStats() map[string]interface{} {
	return spm.GetProcessStats()
}

// GetProcessEvents returns process events (simplified implementation)
func (spm *SimplifiedProcessManager) GetProcessEvents(pid int32) []SimplifiedProcessEvent {
	// Return empty slice for now - would need real event tracking in full implementation
	return []SimplifiedProcessEvent{}
}

// GetDiscoveryEvents returns discovery events (simplified implementation)
func (spm *SimplifiedProcessManager) GetDiscoveryEvents() []SimplifiedProcessEvent {
	// Return empty slice for now - would need real event tracking in full implementation
	return []SimplifiedProcessEvent{}
}