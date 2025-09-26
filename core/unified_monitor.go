package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"github.com/shirou/gopsutil/v3/process"
)

// UnifiedMonitor 统一监控器
type UnifiedMonitor struct {
	app                 *App
	config              MonitoringConfig
	processes           map[int32]*MonitoredProcess
	resourceCache       map[int32]*ResourceUsage
	healthCache         map[int32]*HealthStatus
	performanceDB       map[int32][]PerformanceRecord
	eventHandlers       []EventHandler
	metrics             MetricsCollector
	resourceCollector   UnifiedResourceCollector
	mutex               sync.RWMutex
	ctx                 context.Context
	cancel              context.CancelFunc
}


// MonitoredProcess 被监控的进程
type MonitoredProcess struct {
	PID             int32               `json:"pid"`
	Name            string              `json:"name"`
	Cmdline         string              `json:"cmdline"`
	StartTime       time.Time           `json:"start_time"`
	LastSeen        time.Time           `json:"last_seen"`
	Status          string              `json:"status"`
	Tags            map[string]string   `json:"tags"`
	Config          ProcessMonitorConfig `json:"config"`
	Health          *HealthStatus       `json:"health"`
	ResourceUsage   *ResourceUsage      `json:"resource_usage"`
	Performance     []PerformanceRecord `json:"performance"`
	RestartCount    int                 `json:"restart_count"`
	LastRestart     time.Time           `json:"last_restart"`
}

// ProcessMonitorConfig 进程监控配置
type ProcessMonitorConfig struct {
	EnableMonitoring    bool                   `yaml:"enable_monitoring"`
	EnableHealthCheck   bool                   `yaml:"enable_health_check"`
	EnableResourceLimit bool                   `yaml:"enable_resource_limit"`
	CustomRules         []HealthCheckRule      `yaml:"custom_rules"`
	ResourceLimits      map[string]float64     `yaml:"resource_limits"`
	Tags                map[string]string      `yaml:"tags"`
	Priority            string                 `yaml:"priority"` // "high", "medium", "low"
}

// NewUnifiedMonitor 创建统一监控器
func NewUnifiedMonitor(config MonitoringConfig, app *App) *UnifiedMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	// 创建统一资源收集器配置
	collectorConfig := ResourceCollectionConfig{
		EnableCPUMonitoring:    true,
		EnableMemoryMonitoring:  true,
		EnableIOMonitoring:      config.EnableDetailedIO,
		EnableNetworkMonitoring: false, // 默认不启用网络监控
		EnableThreadMonitoring:  true,
		EnableDetailedIO:        config.EnableDetailedIO,
		CollectionInterval:      config.CheckInterval,
		CacheTTL:               config.CheckInterval * 2,
		MaxCacheSize:           1000,
		EnableHistory:          false, // 统一监控器不需要历史记录
		HistoryRetention:       time.Hour,
	}
	
	return &UnifiedMonitor{
		app:               app,
		config:            config,
		processes:         make(map[int32]*MonitoredProcess),
		resourceCache:     make(map[int32]*ResourceUsage),
		healthCache:       make(map[int32]*HealthStatus),
		performanceDB:     make(map[int32][]PerformanceRecord),
		eventHandlers:     make([]EventHandler, 0),
		resourceCollector: NewUnifiedResourceCollector(collectorConfig),
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start 启动统一监控器
func (um *UnifiedMonitor) Start() error {
	if !um.config.Enabled {
		return nil
	}
	
	log.Println("🚀 启动统一监控器...")
	
	// 启动监控循环
	go um.monitoringLoop()
	
	// 启动健康检查循环
	go um.healthCheckLoop()
	
	// 启动清理循环
	go um.cleanupLoop()
	
	return nil
}

// Stop 停止统一监控器
func (um *UnifiedMonitor) Stop() {
	um.cancel()
	
	// 清理资源
	um.mutex.Lock()
	defer um.mutex.Unlock()
	
	um.processes = make(map[int32]*MonitoredProcess)
	um.resourceCache = make(map[int32]*ResourceUsage)
	um.healthCache = make(map[int32]*HealthStatus)
	
	log.Println("🛑 统一监控器已停止")
}

// monitoringLoop 监控循环
func (um *UnifiedMonitor) monitoringLoop() {
	ticker := time.NewTicker(um.config.Interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-um.ctx.Done():
			return
		case <-ticker.C:
			um.updateAllProcesses()
		}
	}
}

// healthCheckLoop 健康检查循环
func (um *UnifiedMonitor) healthCheckLoop() {
	ticker := time.NewTicker(um.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-um.ctx.Done():
			return
		case <-ticker.C:
			um.performHealthChecks()
		}
	}
}

// cleanupLoop 清理循环
func (um *UnifiedMonitor) cleanupLoop() {
	ticker := time.NewTicker(time.Hour) // 每小时清理一次
	defer ticker.Stop()
	
	for {
		select {
		case <-um.ctx.Done():
			return
		case <-ticker.C:
			um.cleanupOldData()
		}
	}
}

// updateAllProcesses 更新所有进程状态
func (um *UnifiedMonitor) updateAllProcesses() {
	um.mutex.Lock()
	defer um.mutex.Unlock()
	
	// 获取当前系统进程
	allProcesses, err := process.Processes()
	if err != nil {
		log.Printf("❌ 获取系统进程失败: %v", err)
		return
	}
	
	currentPIDs := make(map[int32]bool)
	for _, p := range allProcesses {
		pid := p.Pid
		currentPIDs[pid] = true
		
		// 更新已监控的进程
		if monitored, exists := um.processes[pid]; exists {
			um.updateMonitoredProcess(monitored, p)
		}
	}
	
	// 检查丢失的进程
	for pid, monitored := range um.processes {
		if !currentPIDs[pid] {
			um.handleProcessLost(monitored)
		}
	}
	
	// 检查是否超过最大监控进程数
	if len(um.processes) > um.config.MaxMonitoredProcesses {
		um.cleanupLeastImportantProcesses()
	}
}

// updateMonitoredProcess 更新单个被监控进程
func (um *UnifiedMonitor) updateMonitoredProcess(monitored *MonitoredProcess, p *process.Process) {
	// 更新基本信息
	monitored.LastSeen = time.Now()
	
	// 更新资源使用情况
	resourceUsage := um.collectResourceUsage(p)
	monitored.ResourceUsage = resourceUsage
	um.resourceCache[monitored.PID] = resourceUsage
	
	// 记录性能数据
	performance := PerformanceRecord{
		Timestamp:       time.Now(),
		PID:             monitored.PID,
		CPUUsed:         resourceUsage.CPUUsed,
		MemoryUsedMB:    resourceUsage.MemoryUsedMB,
		IOReadBytes:     uint64(resourceUsage.DiskReadMB) * 1024 * 1024, // 转换为字节
		IOWriteBytes:    uint64(resourceUsage.DiskWriteMB) * 1024 * 1024,
		PerformanceScore: resourceUsage.PerformanceScore,
		Status:          monitored.Status,
		Tags:            monitored.Tags,
	}
	
	monitored.Performance = append(monitored.Performance, performance)
	um.performanceDB[monitored.PID] = append(um.performanceDB[monitored.PID], performance)
	
	// 限制历史记录大小
	if len(monitored.Performance) > um.config.PerformanceHistorySize {
		monitored.Performance = monitored.Performance[len(monitored.Performance)-um.config.PerformanceHistorySize:]
	}
	if len(um.performanceDB[monitored.PID]) > um.config.PerformanceHistorySize {
		um.performanceDB[monitored.PID] = um.performanceDB[monitored.PID][len(um.performanceDB[monitored.PID])-um.config.PerformanceHistorySize:]
	}
	
	// 检查资源限制
	if monitored.Config.EnableResourceLimit {
		um.checkResourceLimits(monitored)
	}
}

// collectResourceUsage 收集资源使用情况（使用统一资源收集器）
func (um *UnifiedMonitor) collectResourceUsage(p *process.Process) *ResourceUsage {
	// 使用统一资源收集器收集信息，它内置了回退机制
	unifiedUsage, err := um.resourceCollector.CollectProcessResources(p.Pid)
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
	
	// 转换UnifiedResourceUsage为ResourceUsage以保持兼容性
	usage := &ResourceUsage{
		CPUUsed:        unifiedUsage.CPU.UsedPercent,
		CPUExpected:    unifiedUsage.CPU.ExpectedPercent,
		MemoryUsedMB:   unifiedUsage.Memory.UsedMB,
		MemoryExpected: unifiedUsage.Memory.ExpectedMB,
		DiskReadMB:     unifiedUsage.Disk.ReadMB,
		DiskWriteMB:    unifiedUsage.Disk.WriteMB,
		LastAnomaly:    unifiedUsage.Performance.LastAnomaly,
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
	
	// 检查异常（保留原有逻辑）
	um.detectResourceAnomalies(usage)
	
	return usage
}



// detectResourceAnomalies 检测资源异常
func (um *UnifiedMonitor) detectResourceAnomalies(usage *ResourceUsage) {
	usage.AnomalyType = []string{}
	
	// CPU异常检测
	if usage.CPUExpected > 0 && usage.CPUUsed > usage.CPUExpected*1.5 {
		usage.AnomalyType = append(usage.AnomalyType, "high_cpu")
		usage.LastAnomaly = time.Now()
	}
	
	// 内存异常检测
	if usage.MemoryExpected > 0 && usage.MemoryUsedMB > usage.MemoryExpected*2 {
		usage.AnomalyType = append(usage.AnomalyType, "high_memory")
		usage.LastAnomaly = time.Now()
	}
	
	// I/O异常检测
	if um.config.EnableDetailedIO && usage.DiskWriteMB > 1000 { // 超过1GB写入
		usage.AnomalyType = append(usage.AnomalyType, "high_io")
		usage.LastAnomaly = time.Now()
	}
}

// performHealthChecks 执行健康检查
func (um *UnifiedMonitor) performHealthChecks() {
	um.mutex.Lock()
	defer um.mutex.Unlock()
	
	for pid, monitored := range um.processes {
		if monitored.Config.EnableHealthCheck {
			health := um.checkProcessHealth(monitored)
			monitored.Health = health
			um.healthCache[pid] = health
			
			// 处理不健康的进程
			if !health.IsHealthy {
				um.handleUnhealthyProcess(monitored, health)
			}
		}
	}
}

// checkProcessHealth 检查进程健康状态
func (um *UnifiedMonitor) checkProcessHealth(monitored *MonitoredProcess) *HealthStatus {
	health := &HealthStatus{
		LastCheck: time.Now(),
		NextCheck: time.Now().Add(um.config.HealthCheckInterval),
		Issues:    []string{},
	}
	
	// 基础进程存在性检查
	if p, err := process.NewProcess(monitored.PID); err != nil {
		health.IsHealthy = false
		health.Status = "not_found"
		health.Issues = append(health.Issues, "进程不存在")
		health.Score = 0
		return health
	} else {
		// 检查进程状态
		if status, err := p.Status(); err == nil {
			if len(status) > 0 && status[0] != "running" {
				health.IsHealthy = false
				health.Status = status[0]
				health.Issues = append(health.Issues, fmt.Sprintf("进程状态异常: %s", status[0]))
			}
		}
		
		p, _ = process.NewProcess(monitored.PID)
	}
	
	// 应用健康检查规则
	score := 100.0
	rules := um.getApplicableRules(monitored)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		
		if um.evaluateHealthRule(monitored, rule) {
			health.Issues = append(health.Issues, rule.Description)
			switch rule.Severity {
			case "critical":
				score -= 25
			case "error":
				score -= 15
			case "warning":
				score -= 10
			case "info":
				score -= 5
			}
		}
	}
	
	// 确定最终健康状态
	health.Score = score
	if score >= 80 {
		health.IsHealthy = true
		health.Status = "healthy"
	} else if score >= 60 {
		health.IsHealthy = false
		health.Status = "degraded"
	} else {
		health.IsHealthy = false
		health.Status = "unhealthy"
	}
	
	return health
}

// getApplicableRules 获取适用的健康检查规则
func (um *UnifiedMonitor) getApplicableRules(monitored *MonitoredProcess) []HealthCheckRule {
	rules := um.config.HealthCheckRules
	
	// 添加进程特定的规则
	if monitored.Config.CustomRules != nil {
		rules = append(rules, monitored.Config.CustomRules...)
	}
	
	return rules
}

// evaluateHealthRule 评估健康检查规则
func (um *UnifiedMonitor) evaluateHealthRule(monitored *MonitoredProcess, rule HealthCheckRule) bool {
	var value float64
	
	switch rule.Metric {
	case "cpu":
		value = monitored.ResourceUsage.CPUUsed
	case "memory":
		value = float64(monitored.ResourceUsage.MemoryUsedMB)
	case "performance_score":
		value = monitored.ResourceUsage.PerformanceScore
	default:
		return false
	}
	
	switch rule.Operator {
	case ">":
		return value > rule.Threshold
	case "<":
		return value < rule.Threshold
	case ">=":
		return value >= rule.Threshold
	case "<=":
		return value <= rule.Threshold
	case "==":
		return value == rule.Threshold
	case "!=":
		return value != rule.Threshold
	default:
		return false
	}
}

// handleUnhealthyProcess 处理不健康的进程
func (um *UnifiedMonitor) handleUnhealthyProcess(monitored *MonitoredProcess, health *HealthStatus) {
	// 发送事件
	event := Event{
		Type:      EventHealthCheckFailed,
		Source:    "unified_monitor",
		Level:     LevelError,
		Message:   fmt.Sprintf("进程 %s (PID: %d) 健康检查失败: %s", monitored.Name, monitored.PID, health.Status),
		Timestamp: time.Now(),
		PID:       monitored.PID,
		Details: map[string]interface{}{
			"health_status": health,
			"score":        health.Score,
			"issues":       health.Issues,
		},
		Tags: []string{"health", "process", monitored.Name},
	}
	
	um.emitEvent(event)
	
	// 自动重启策略
	if um.config.AutoRestartAttempt && monitored.RestartCount < um.config.MaxRestartAttempts {
		// 这里可以实现重启逻辑
		log.Printf("⚠️ 进程 %s (PID: %d) 不健康，考虑重启策略", monitored.Name, monitored.PID)
	}
}

// emitEvent 发送事件
func (um *UnifiedMonitor) emitEvent(event Event) {
	for _, handler := range um.eventHandlers {
		if err := handler.HandleEvent(event); err != nil {
			log.Printf("❌ 事件处理失败: %v", err)
		}
	}
}

// cleanupOldData 清理旧数据
func (um *UnifiedMonitor) cleanupOldData() {
	um.mutex.Lock()
	defer um.mutex.Unlock()
	
	cutoff := time.Now().Add(-24 * time.Hour) // 清理24小时前的数据
	
	// 清理性能历史
	for pid, records := range um.performanceDB {
		validRecords := records[:0]
		for _, record := range records {
			if record.Timestamp.After(cutoff) {
				validRecords = append(validRecords, record)
			}
		}
		um.performanceDB[pid] = validRecords
	}
	
	// 清理资源缓存
	for pid, usage := range um.resourceCache {
		if usage.LastAnomaly.Before(cutoff) {
			delete(um.resourceCache, pid)
		}
	}
}

// cleanupLeastImportantProcesses 清理最不重要的进程
func (um *UnifiedMonitor) cleanupLeastImportantProcesses() {
	// 实现清理策略，优先保留高优先级的进程
	// 这里可以根据优先级、资源使用率等因素来决定清理哪些进程
}

// checkResourceLimits 检查资源限制
func (um *UnifiedMonitor) checkResourceLimits(monitored *MonitoredProcess) {
	// 实现资源限制检查逻辑
}

// handleProcessLost 处理丢失的进程
func (um *UnifiedMonitor) handleProcessLost(monitored *MonitoredProcess) {
	// 发送进程丢失事件
	event := Event{
		Type:      EventProcessStopped,
		Source:    "unified_monitor",
		Level:     LevelInfo,
		Message:   fmt.Sprintf("进程 %s (PID: %d) 已停止", monitored.Name, monitored.PID),
		Timestamp: time.Now(),
		PID:       monitored.PID,
		Details: map[string]interface{}{
			"last_seen": monitored.LastSeen,
			"uptime":    time.Since(monitored.StartTime),
		},
		Tags: []string{"process", "stopped", monitored.Name},
	}
	
	um.emitEvent(event)
	
	// 从监控列表中移除
	delete(um.processes, monitored.PID)
	delete(um.resourceCache, monitored.PID)
	delete(um.healthCache, monitored.PID)
	delete(um.performanceDB, monitored.PID)
	
	log.Printf("📋 进程 %s (PID: %d) 已从监控中移除", monitored.Name, monitored.PID)
}

// GetStats 获取监控统计信息 (实现ProcessMonitor接口)
func (um *UnifiedMonitor) GetStats() map[string]interface{} {
	um.mutex.RLock()
	defer um.mutex.RUnlock()
	
	stats := map[string]interface{}{
		"monitored_processes": len(um.processes),
		"resource_cache_size": len(um.resourceCache),
		"health_cache_size":   len(um.healthCache),
		"performance_records":  0,
		"enabled":             um.config.Enabled,
		"max_processes":       um.config.MaxMonitoredProcesses,
	}
	
	// 计算总性能记录数
	for _, records := range um.performanceDB {
		stats["performance_records"] = stats["performance_records"].(int) + len(records)
	}
	
	return stats
}