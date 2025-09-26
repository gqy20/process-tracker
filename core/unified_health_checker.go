package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"github.com/shirou/gopsutil/v3/process"
)

// UnifiedHealthChecker 统一健康检查器
type UnifiedHealthChecker struct {
	app          *App
	config       HealthCheckerConfig
	rules        map[string]HealthCheckRule
	healthHistory map[int32][]*HealthStatus
	eventHandler EventHandler
	mutex        sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}


// AutoRestartConfig 自动重启配置
type AutoRestartConfig struct {
	Enabled           bool          `yaml:"enabled"`
	MaxRestartCount   int           `yaml:"max_restart_count"`
	RestartDelay      time.Duration `yaml:"restart_delay"`
	BackoffMultiplier float64       `yaml:"backoff_multiplier"`
	MaxBackoffTime    time.Duration `yaml:"max_backoff_time"`
}


// HealthReport 健康检查报告
type HealthReport struct {
	Timestamp        time.Time                    `json:"timestamp"`
	SystemHealth     *HealthStatus                 `json:"system_health"`
	ProcessHealth     map[int32]*HealthStatus      `json:"process_health"`
	ServiceHealth    map[string]*HealthStatus      `json:"service_health"`
	Summary          HealthSummary                 `json:"summary"`
	Recommendations  []string                     `json:"recommendations"`
}

// HealthSummary 健康摘要
type HealthSummary struct {
	TotalChecks      int     `json:"total_checks"`
	HealthyCount     int     `json:"healthy_count"`
	WarningCount     int     `json:"warning_count"`
	ErrorCount       int     `json:"error_count"`
	CriticalCount    int     `json:"critical_count"`
	OverallScore     float64 `json:"overall_score"`
	OverallStatus    string  `json:"overall_status"`
}

// HealthTrend 健康趋势
type HealthTrend struct {
	ProcessID     int32           `json:"process_id"`
	Period        time.Duration   `json:"period"`
	DataPoints    []*HealthStatus `json:"data_points"`
	ScoreTrend    float64         `json:"score_trend"`
	IssuesTrend   int             `json:"issues_trend"`
	Stability     float64         `json:"stability"` // 0-1 稳定性评分
	Prediction    string          `json:"prediction"`  // "improving", "stable", "degrading"
}

// NewUnifiedHealthChecker 创建统一健康检查器
func NewUnifiedHealthChecker(config HealthCheckerConfig, app *App) *UnifiedHealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &UnifiedHealthChecker{
		app:            app,
		config:         config,
		rules:          make(map[string]HealthCheckRule),
		healthHistory:  make(map[int32][]*HealthStatus),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动健康检查器
func (uhc *UnifiedHealthChecker) Start() error {
	if !uhc.config.Enabled {
		return nil
	}
	
	log.Println("🚀 启动统一健康检查器...")
	
	// 初始化默认规则
	uhc.initializeDefaultRules()
	
	// 启动健康检查循环
	go uhc.healthCheckLoop()
	
	// 启动趋势分析循环
	go uhc.trendAnalysisLoop()
	
	return nil
}

// CheckHealth 执行健康检查 (实现HealthChecker接口)
func (uhc *UnifiedHealthChecker) CheckHealth() *HealthStatus {
	// 检查系统健康状态
	systemHealth := uhc.checkSystemHealth()
	
	return systemHealth
}

// Stop 停止健康检查器
func (uhc *UnifiedHealthChecker) Stop() {
	uhc.cancel()
	log.Println("🛑 统一健康检查器已停止")
}

// healthCheckLoop 健康检查循环
func (uhc *UnifiedHealthChecker) healthCheckLoop() {
	ticker := time.NewTicker(uhc.config.CheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-uhc.ctx.Done():
			return
		case <-ticker.C:
			uhc.performHealthChecks()
		}
	}
}

// trendAnalysisLoop 趋势分析循环
func (uhc *UnifiedHealthChecker) trendAnalysisLoop() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟分析一次趋势
	defer ticker.Stop()
	
	for {
		select {
		case <-uhc.ctx.Done():
			return
		case <-ticker.C:
			uhc.analyzeHealthTrends()
		}
	}
}

// performHealthChecks 执行健康检查
func (uhc *UnifiedHealthChecker) performHealthChecks() {
	report := HealthReport{
		Timestamp:     time.Now(),
		ProcessHealth: make(map[int32]*HealthStatus),
		ServiceHealth: make(map[string]*HealthStatus),
	}
	
	// 系统健康检查
	systemHealth := uhc.checkSystemHealth()
	report.SystemHealth = systemHealth
	
	// 进程健康检查
	processHealth := uhc.checkAllProcessesHealth()
	for pid, health := range processHealth {
		report.ProcessHealth[pid] = health
	}
	
	// 服务健康检查（如果有）
	serviceHealth := uhc.checkServicesHealth()
	for name, health := range serviceHealth {
		report.ServiceHealth[name] = health
	}
	
	// 生成摘要和推荐
	report.Summary = uhc.generateHealthSummary(report)
	report.Recommendations = uhc.generateRecommendations(report)
	
	// 记录健康报告
	uhc.logHealthReport(report)
	
	// 发送通知（如果需要）
	uhc.sendNotifications(report)
}

// checkSystemHealth 检查系统健康状态
func (uhc *UnifiedHealthChecker) checkSystemHealth() *HealthStatus {
	health := &HealthStatus{
		LastCheck: time.Now(),
		NextCheck: time.Now().Add(uhc.config.CheckInterval),
		Issues:    []string{},
	}
	
	score := 100.0
	
	// CPU负载检查
	if cpuLoad, err := uhc.getCPULoad(); err == nil {
		if cpuLoad > uhc.config.DefaultLoadThreshold {
			health.Issues = append(health.Issues, fmt.Sprintf("系统CPU负载过高: %.2f%%", cpuLoad))
			score -= 20
		}
	}
	
	// 内存使用检查
	if memUsage, err := uhc.getMemoryUsage(); err == nil {
		if memUsage > 90 { // 超过90%内存使用
			health.Issues = append(health.Issues, fmt.Sprintf("系统内存使用率过高: %.2f%%", memUsage))
			score -= 25
		} else if memUsage > 80 {
			health.Issues = append(health.Issues, fmt.Sprintf("系统内存使用率偏高: %.2f%%", memUsage))
			score -= 10
		}
	}
	
	// 磁盘空间检查
	if diskUsage, err := uhc.getDiskUsage(); err == nil {
		if diskUsage > 95 {
			health.Issues = append(health.Issues, fmt.Sprintf("磁盘空间不足: %.2f%%", diskUsage))
			score -= 30
		} else if diskUsage > 90 {
			health.Issues = append(health.Issues, fmt.Sprintf("磁盘空间偏低: %.2f%%", diskUsage))
			score -= 15
		}
	}
	
	// 进程数量检查
	if processCount := uhc.getProcessCount(); processCount > 1000 {
		health.Issues = append(health.Issues, fmt.Sprintf("系统进程数量过多: %d", processCount))
		score -= 10
	}
	
	// 应用健康检查规则
	for _, rule := range uhc.getApplicableRules("system") {
		if uhc.evaluateSystemRule(rule) {
			health.Issues = append(health.Issues, rule.Description)
			score -= uhc.getSeverityPenalty(rule.Severity)
		}
	}
	
	// 确定最终健康状态
	health.Score = score
	uhc.determineHealthStatus(health)
	
	return health
}

// checkAllProcessesHealth 检查所有进程健康状态
func (uhc *UnifiedHealthChecker) checkAllProcessesHealth() map[int32]*HealthStatus {
	healthMap := make(map[int32]*HealthStatus)
	
	// 获取所有进程
	if processes, err := process.Processes(); err == nil {
		for _, p := range processes {
			pid := p.Pid
			
			// 跳过系统进程
			if pid <= 1 {
				continue
			}
			
			// 检查进程健康状态
			if health := uhc.checkProcessHealth(p); health != nil {
				healthMap[pid] = health
				
				// 记录健康历史
				uhc.recordHealthHistory(pid, health)
			}
		}
	}
	
	return healthMap
}

// checkProcessHealth 检查单个进程健康状态
func (uhc *UnifiedHealthChecker) checkProcessHealth(p *process.Process) *HealthStatus {
	health := &HealthStatus{
		LastCheck: time.Now(),
		NextCheck: time.Now().Add(uhc.config.CheckInterval),
		Issues:    []string{},
	}
	
	score := 100.0
	
	// 进程存在性检查（已经在process.Process中确认存在）
	
	// 进程状态检查
	if status, err := p.Status(); err == nil {
		if len(status) > 0 && status[0] != "running" {
			health.Issues = append(health.Issues, fmt.Sprintf("进程状态异常: %s", status[0]))
			score -= 40
		}
	}
	
	// CPU使用率检查
	if cpuPercent, err := p.CPUPercent(); err == nil {
		if cpuPercent > uhc.config.DefaultCPUThreshold {
			health.Issues = append(health.Issues, fmt.Sprintf("CPU使用率过高: %.2f%%", cpuPercent))
			score -= 20
		}
	}
	
	// 内存使用检查
	if memInfo, err := p.MemoryInfo(); err == nil {
		memoryMB := float64(memInfo.RSS) / 1024 / 1024
		if memoryMB > float64(uhc.config.DefaultMemoryThreshold) {
			health.Issues = append(health.Issues, fmt.Sprintf("内存使用过高: %.2f MB", memoryMB))
			score -= 25
		}
	}
	
	// I/O检查
	if ioCounters, err := p.IOCounters(); err == nil {
		totalIO := float64(ioCounters.ReadCount + ioCounters.WriteCount)
		if totalIO > float64(uhc.config.DefaultIOThreshold) {
			health.Issues = append(health.Issues, fmt.Sprintf("I/O操作频繁: %.0f 次操作", totalIO))
			score -= 15
		}
	}
	
	// 文件描述符检查
	if numFDs, err := p.NumFDs(); err == nil {
		if numFDs > 1000 {
			health.Issues = append(health.Issues, fmt.Sprintf("文件描述符过多: %d", numFDs))
			score -= 10
		}
	}
	
	// 线程数检查
	if numThreads, err := p.NumThreads(); err == nil {
		if numThreads > 100 {
			health.Issues = append(health.Issues, fmt.Sprintf("线程数过多: %d", numThreads))
			score -= 10
		}
	}
	
	// 运行时间检查
	if createTime, err := p.CreateTime(); err == nil {
		uptime := time.Now().Unix() - createTime/1000
		if uptime < 60 { // 运行时间小于1分钟
			health.Issues = append(health.Issues, "进程运行时间过短")
			score -= 5
		}
	}
	
	// 应用进程特定的健康检查规则
	if processName, err := p.Name(); err == nil {
		rules := uhc.getApplicableRules(processName)
		for _, rule := range rules {
			if uhc.evaluateProcessRule(p, rule) {
				health.Issues = append(health.Issues, rule.Description)
				score -= uhc.getSeverityPenalty(rule.Severity)
			}
		}
	}
	
	// 确定最终健康状态
	health.Score = score
	uhc.determineHealthStatus(health)
	
	return health
}

// checkServicesHealth 检查服务健康状态
func (uhc *UnifiedHealthChecker) checkServicesHealth() map[string]*HealthStatus {
	services := make(map[string]*HealthStatus)
	
	// 检查监控服务
	if uhc.app.UnifiedMonitor != nil {
		services["unified_monitor"] = uhc.checkServiceHealth("unified_monitor")
	}
	
	// 检查配额管理服务
	if uhc.app.QuotaManager != nil {
		services["quota_manager"] = uhc.checkServiceHealth("quota_manager")
	}
	
	// 检查任务管理服务
	if uhc.app.TaskManager != nil {
		services["task_manager"] = uhc.checkServiceHealth("task_manager")
	}
	
	return services
}

// checkServiceHealth 检查服务健康状态
func (uhc *UnifiedHealthChecker) checkServiceHealth(serviceName string) *HealthStatus {
	health := &HealthStatus{
		LastCheck: time.Now(),
		NextCheck: time.Now().Add(uhc.config.CheckInterval),
		Issues:    []string{},
	}
	
	score := 100.0
	
	// 根据服务类型进行特定的健康检查
	switch serviceName {
	case "unified_monitor":
		// 检查监控的进程数量
		if uhc.app.UnifiedMonitor != nil {
			// 这里可以添加更具体的检查逻辑
		} else {
			health.Issues = append(health.Issues, "统一监控器未初始化")
			score -= 50
		}
		
	case "quota_manager":
		// 检查配额管理器状态
		if uhc.app.QuotaManager == nil {
			health.Issues = append(health.Issues, "配额管理器未初始化")
			score -= 50
		}
		
	case "task_manager":
		// 检查任务管理器状态
		if uhc.app.TaskManager == nil {
			health.Issues = append(health.Issues, "任务管理器未初始化")
			score -= 50
		}
	}
	
	// 确定最终健康状态
	health.Score = score
	uhc.determineHealthStatus(health)
	
	return health
}

// generateHealthSummary 生成健康摘要
func (uhc *UnifiedHealthChecker) generateHealthSummary(report HealthReport) HealthSummary {
	summary := HealthSummary{
		TotalChecks: 0,
		HealthyCount: 0,
		WarningCount: 0,
		ErrorCount: 0,
		CriticalCount: 0,
	}
	
	// 统计系统健康
	if report.SystemHealth != nil {
		summary.TotalChecks++
		switch report.SystemHealth.Status {
		case "healthy":
			summary.HealthyCount++
		case "degraded":
			summary.WarningCount++
		case "unhealthy":
			summary.ErrorCount++
		case "critical":
			summary.CriticalCount++
		}
	}
	
	// 统计进程健康
	for _, health := range report.ProcessHealth {
		summary.TotalChecks++
		switch health.Status {
		case "healthy":
			summary.HealthyCount++
		case "degraded":
			summary.WarningCount++
		case "unhealthy":
			summary.ErrorCount++
		case "critical":
			summary.CriticalCount++
		}
	}
	
	// 统计服务健康
	for _, health := range report.ServiceHealth {
		summary.TotalChecks++
		switch health.Status {
		case "healthy":
			summary.HealthyCount++
		case "degraded":
			summary.WarningCount++
		case "unhealthy":
			summary.ErrorCount++
		case "critical":
			summary.CriticalCount++
		}
	}
	
	// 计算总体评分
	if summary.TotalChecks > 0 {
		summary.OverallScore = (float64(summary.HealthyCount)*100 + 
			float64(summary.WarningCount)*70 + 
			float64(summary.ErrorCount)*40 + 
			float64(summary.CriticalCount)*10) / float64(summary.TotalChecks)
	}
	
	// 确定总体状态
	if summary.OverallScore >= 80 {
		summary.OverallStatus = "healthy"
	} else if summary.OverallScore >= 60 {
		summary.OverallStatus = "degraded"
	} else if summary.OverallScore >= 40 {
		summary.OverallStatus = "unhealthy"
	} else {
		summary.OverallStatus = "critical"
	}
	
	return summary
}

// generateRecommendations 生成建议
func (uhc *UnifiedHealthChecker) generateRecommendations(report HealthReport) []string {
	recommendations := []string{}
	
	// 基于系统健康的建议
	if report.SystemHealth != nil {
		if len(report.SystemHealth.Issues) > 0 {
			recommendations = append(recommendations, "检查系统资源使用情况，考虑优化系统配置")
		}
	}
	
	// 基于进程健康的建议
	unhealthyProcesses := 0
	for _, health := range report.ProcessHealth {
		if !health.IsHealthy {
			unhealthyProcesses++
		}
	}
	
	if unhealthyProcesses > len(report.ProcessHealth)/4 {
		recommendations = append(recommendations, "多个进程存在健康问题，建议检查系统整体状态")
	} else if unhealthyProcesses > 0 {
		recommendations = append(recommendations, fmt.Sprintf("发现 %d 个不健康的进程，建议检查具体原因", unhealthyProcesses))
	}
	
	// 基于服务健康的建议
	unhealthyServices := 0
	for name, health := range report.ServiceHealth {
		if !health.IsHealthy {
			unhealthyServices++
			recommendations = append(recommendations, fmt.Sprintf("服务 %s 存在问题，建议重启或检查配置", name))
		}
	}
	
	// 基于评分的建议
	if report.Summary.OverallScore < 60 {
		recommendations = append(recommendations, "系统整体健康状况较差，建议立即采取措施")
	} else if report.Summary.OverallScore < 80 {
		recommendations = append(recommendations, "系统健康状况一般，建议进行预防性维护")
	}
	
	return recommendations
}

// 辅助方法

func (uhc *UnifiedHealthChecker) initializeDefaultRules() {
	// 系统级默认规则
	uhc.rules["system_cpu_high"] = HealthCheckRule{
		Name:        "system_cpu_high",
		Category:    "system",
		Metric:      "cpu",
		Operator:    ">",
		Threshold:   uhc.config.DefaultLoadThreshold,
		Severity:    "warning",
		Action:      "alert",
		Description: "系统CPU负载过高",
		Enabled:     true,
	}
	
	uhc.rules["system_memory_high"] = HealthCheckRule{
		Name:        "system_memory_high",
		Category:    "system", 
		Metric:      "memory",
		Operator:    ">",
		Threshold:   90,
		Severity:    "warning",
		Action:      "alert",
		Description: "系统内存使用率过高",
		Enabled:     true,
	}
	
	// 进程级默认规则
	uhc.rules["process_cpu_high"] = HealthCheckRule{
		Name:        "process_cpu_high",
		Category:    "process",
		Metric:      "cpu",
		Operator:    ">",
		Threshold:   uhc.config.DefaultCPUThreshold,
		Severity:    "warning",
		Action:      "alert",
		Description: "进程CPU使用率过高",
		Enabled:     true,
	}
}

func (uhc *UnifiedHealthChecker) getApplicableRules(category string) []HealthCheckRule {
	var rules []HealthCheckRule
	
	for _, rule := range uhc.rules {
		if rule.Category == category && rule.Enabled {
			rules = append(rules, rule)
		}
	}
	
	return rules
}

func (uhc *UnifiedHealthChecker) evaluateSystemRule(rule HealthCheckRule) bool {
	switch rule.Metric {
	case "cpu":
		if load, err := uhc.getCPULoad(); err == nil {
			return uhc.evaluateThreshold(load, rule.Operator, rule.Threshold)
		}
	case "memory":
		if usage, err := uhc.getMemoryUsage(); err == nil {
			return uhc.evaluateThreshold(usage, rule.Operator, rule.Threshold)
		}
	}
	return false
}

func (uhc *UnifiedHealthChecker) evaluateProcessRule(p *process.Process, rule HealthCheckRule) bool {
	switch rule.Metric {
	case "cpu":
		if cpu, err := p.CPUPercent(); err == nil {
			return uhc.evaluateThreshold(cpu, rule.Operator, rule.Threshold)
		}
	case "memory":
		if mem, err := p.MemoryInfo(); err == nil {
			memoryMB := float64(mem.RSS) / 1024 / 1024
			return uhc.evaluateThreshold(memoryMB, rule.Operator, rule.Threshold)
		}
	}
	return false
}

func (uhc *UnifiedHealthChecker) evaluateThreshold(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

func (uhc *UnifiedHealthChecker) getSeverityPenalty(severity string) float64 {
	switch severity {
	case "critical":
		return 25
	case "error":
		return 15
	case "warning":
		return 10
	default:
		return 5
	}
}

func (uhc *UnifiedHealthChecker) determineHealthStatus(health *HealthStatus) {
	if health.Score >= 80 {
		health.IsHealthy = true
		health.Status = "healthy"
	} else if health.Score >= 60 {
		health.IsHealthy = false
		health.Status = "degraded"
	} else if health.Score >= 40 {
		health.IsHealthy = false
		health.Status = "unhealthy"
	} else {
		health.IsHealthy = false
		health.Status = "critical"
	}
}

func (uhc *UnifiedHealthChecker) recordHealthHistory(pid int32, health *HealthStatus) {
	uhc.mutex.Lock()
	defer uhc.mutex.Unlock()
	
	if _, exists := uhc.healthHistory[pid]; !exists {
		uhc.healthHistory[pid] = []*HealthStatus{}
	}
	
	uhc.healthHistory[pid] = append(uhc.healthHistory[pid], health)
	
	// 限制历史记录大小
	if len(uhc.healthHistory[pid]) > uhc.config.MaxHistorySize {
		uhc.healthHistory[pid] = uhc.healthHistory[pid][len(uhc.healthHistory[pid])-uhc.config.MaxHistorySize:]
	}
}

func (uhc *UnifiedHealthChecker) logHealthReport(report HealthReport) {
	if uhc.config.EnableDetailedLogging {
		log.Printf("📊 健康检查报告 - 状态: %s, 评分: %.1f, 问题: %d", 
			report.Summary.OverallStatus, report.Summary.OverallScore, 
			len(report.SystemHealth.Issues))
	}
}

func (uhc *UnifiedHealthChecker) sendNotifications(report HealthReport) {
	if !uhc.config.NotificationConfig.Enabled {
		return
	}
	
	// 只在有问题时发送通知
	if report.Summary.OverallStatus == "healthy" {
		return
	}
	
	// 创建事件
	event := Event{
		Type:      EventHealthCheckFailed,
		Source:    "health_checker",
		Level:     LevelWarning,
		Message:   fmt.Sprintf("系统健康检查报告: %s (评分: %.1f)", report.Summary.OverallStatus, report.Summary.OverallScore),
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"report": report,
		},
		Tags: []string{"health", "system"},
	}
	
	// 发送事件
	if uhc.eventHandler != nil {
		uhc.eventHandler.HandleEvent(event)
	}
}

func (uhc *UnifiedHealthChecker) analyzeHealthTrends() {
	// 实现健康趋势分析逻辑
	// 这里可以分析历史数据，预测潜在问题
}

// 系统信息获取方法（简化实现）
func (uhc *UnifiedHealthChecker) getCPULoad() (float64, error) {
	// 简化实现，返回0
	return 0, nil
}

func (uhc *UnifiedHealthChecker) getMemoryUsage() (float64, error) {
	// 简化实现，返回0
	return 0, nil
}

func (uhc *UnifiedHealthChecker) getDiskUsage() (float64, error) {
	// 简化实现，返回0
	return 0, nil
}

func (uhc *UnifiedHealthChecker) getProcessCount() int {
	// 简化实现，返回0
	return 0
}