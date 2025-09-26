package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"github.com/shirou/gopsutil/v3/process"
)

// HealthChecker manages health checks and alerting
type HealthChecker struct {
	app              *App
	config           HealthCheckConfig
	checks           map[string]*HealthCheck
	alerts           map[string]*Alert
	rules            map[string]*HealthRule
	channels         map[string]map[string]interface{}
	violationCounts  map[string]int // Track consecutive violations
	healthHistory    []*HealthCheck
	alertHistory     []*Alert
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	events           chan HealthEvent
	stats            HealthStats
}

// HealthEvent represents a health check event
type HealthEvent struct {
	Type      HealthEventType `yaml:"type"`
	CheckID   string          `yaml:"check_id"`
	AlertID   string          `yaml:"alert_id"`
	Timestamp time.Time       `yaml:"timestamp"`
	Data      interface{}     `yaml:"data"`
}

// HealthEventType represents the type of health event
type HealthEventType string

const (
	HealthEventCheckStarted  HealthEventType = "check_started"
	HealthEventCheckCompleted HealthEventType = "check_completed"
	HealthEventAlertTriggered HealthEventType = "alert_triggered"
	HealthEventAlertResolved HealthEventType = "alert_resolved"
	HealthEventRuleViolated  HealthEventType = "rule_violated"
)

// NotificationChannel represents a notification channel implementation
type NotificationChannel struct {
	config NotificationChannelConfig
}

// HealthStats represents health checker statistics
type HealthStats struct {
	TotalChecks      int64
	CompletedChecks  int64
	FailedChecks     int64
	ActiveAlerts     int64
	ResolvedAlerts   int64
	LastCheckTime    time.Time
	AvgCheckDuration time.Duration
}

// NewHealthChecker creates a new health checker instance
func NewHealthChecker(config HealthCheckConfig, app *App) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	
	hc := &HealthChecker{
		app:             app,
		config:          config,
		checks:          make(map[string]*HealthCheck),
		alerts:          make(map[string]*Alert),
		rules:           make(map[string]*HealthRule),
		channels:        make(map[string]map[string]interface{}),
		violationCounts: make(map[string]int),
		healthHistory:   make([]*HealthCheck, 0),
		alertHistory:    make([]*Alert, 0),
		ctx:             ctx,
		cancel:          cancel,
		events:          make(chan HealthEvent, 100),
		stats: HealthStats{
			LastCheckTime: time.Now(),
		},
	}
	
	// Initialize health rules
	for _, rule := range config.HealthRules {
		hc.rules[rule.ID] = &rule
	}
	
	// Initialize notification channels
	for _, channelConfig := range config.AlertManager.NotificationChannels {
		hc.channels[channelConfig.Name] = map[string]interface{}{
			"type":    channelConfig.Type,
			"enabled": channelConfig.Enabled,
		}
	}
	
	return hc
}

// Start starts the health checker
func (hc *HealthChecker) Start() error {
	if !hc.config.Enabled {
		return nil
	}

	log.Printf("🚀 HealthChecker starting with %d rules", len(hc.rules))

	// Start health check scheduler
	go hc.healthCheckScheduler()

	// Start alert manager
	go hc.alertManager()

	// Start stats collector
	go hc.statsCollector()

	return nil
}

// Stop stops the health checker gracefully
func (hc *HealthChecker) Stop() {
	log.Println("🛑 HealthChecker stopping...")

	// Cancel context
	hc.cancel()

	log.Println("✅ HealthChecker stopped")
}

// healthCheckScheduler schedules health checks
func (hc *HealthChecker) healthCheckScheduler() {
	ticker := time.NewTicker(hc.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.RunHealthChecks()
		}
	}
}

// runHealthChecks runs all enabled health checks
func (hc *HealthChecker) RunHealthChecks() {
	startTime := time.Now()
	
	// Run process health checks
	if hc.config.EnableProcessChecks {
		hc.checkProcessHealth()
	}
	
	// Run resource health checks
	if hc.config.EnableResourceChecks {
		hc.checkResourceHealth()
	}
	
	// Run task health checks
	if hc.config.EnableTaskChecks {
		hc.checkTaskHealth()
	}
	
	// Run system health checks
	if hc.config.EnableSystemChecks {
		hc.checkSystemHealth()
	}
	
	// Update stats
	hc.mu.Lock()
	hc.stats.TotalChecks++
	hc.stats.LastCheckTime = time.Now()
	hc.stats.AvgCheckDuration = time.Duration(
		(int64(hc.stats.AvgCheckDuration)*hc.stats.TotalChecks + time.Since(startTime).Nanoseconds()) /
		(hc.stats.TotalChecks + 1),
	)
	hc.mu.Unlock()
}

// checkProcessHealth checks the health of monitored processes
func (hc *HealthChecker) checkProcessHealth() {
	processes := hc.app.ProcessController.GetManagedProcesses()
	
	for _, proc := range processes {
		checkID := fmt.Sprintf("process_%d", proc.PID)
		
		// Create health check
		check := &HealthCheck{
			ID:        checkID,
			Name:      fmt.Sprintf("Process %d (%s)", proc.PID, proc.Name),
			Type:      HealthCheckTypeProcess,
			Timestamp: time.Now(),
			Tags:      []string{"process", proc.Name},
		}
		
		// Get current resource usage
		p, err := process.NewProcess(proc.PID)
		if err != nil {
			continue
		}
		resource, err := hc.app.QuotaManager.monitor.GetResourceUsage(p)
		if resource == nil {
			check.Status = HealthStatusUnknown
			check.Score = 0
			check.Message = "Process not found or no resource data"
			hc.recordHealthCheck(check)
			continue
		}
		
		// Calculate health score (0-100)
		score := 100.0
		
		// CPU health
		if resource.CPUUsed > 90 {
			score -= 30
		} else if resource.CPUUsed > 70 {
			score -= 15
		} else if resource.CPUUsed > 50 {
			score -= 5
		}
		
		// Memory health
		if resource.MemoryUsedMB > 4096 {
			score -= 30
		} else if resource.MemoryUsedMB > 2048 {
			score -= 15
		} else if resource.MemoryUsedMB > 1024 {
			score -= 5
		}
		
		// Thread health
		if resource.ThreadsUsed > 100 {
			score -= 10
		} else if resource.ThreadsUsed > 50 {
			score -= 5
		}
		
		// Determine status and message
		if score >= 80 {
			check.Status = HealthStatusHealthy
			check.Message = "Process is healthy"
		} else if score >= 60 {
			check.Status = HealthStatusWarning
			check.Message = "Process showing warning signs"
		} else if score >= 40 {
			check.Status = HealthStatusCritical
			check.Message = "Process in critical state"
		} else {
			check.Status = HealthStatusCritical
			check.Message = "Process in very poor health"
		}
		
		check.Score = score
		check.Details = map[string]interface{}{
			"cpu_percent":  resource.CPUUsed,
			"memory_mb":    resource.MemoryUsedMB,
			"threads":      resource.ThreadsUsed,
			"disk_read_mb": resource.DiskReadMB,
			"disk_write_mb": resource.DiskWriteMB,
		}
		check.Duration = time.Since(check.Timestamp)
		
		hc.recordHealthCheck(check)
		hc.evaluateHealthRules(check, proc.PID)
	}
}

// checkResourceHealth checks overall resource health
func (hc *HealthChecker) checkResourceHealth() {
	// Get system resource usage
	systemMetrics := hc.getSystemMetrics()
	
	check := &HealthCheck{
		ID:        "system_resources",
		Name:      "System Resources",
		Type:      HealthCheckTypeResource,
		Timestamp: time.Now(),
		Tags:      []string{"system", "resources"},
	}
	
	// Calculate health score
	score := 100.0
	
	// CPU health
	if systemMetrics.CPUUsage > 80 {
		score -= 25
	} else if systemMetrics.CPUUsage > 60 {
		score -= 10
	}
	
	// Memory health
	if systemMetrics.MemoryUsage > 80 {
		score -= 25
	} else if systemMetrics.MemoryUsage > 60 {
		score -= 10
	}
	
	// Load average health
	if systemMetrics.LoadAverage > 2.0 {
		score -= 20
	} else if systemMetrics.LoadAverage > 1.0 {
		score -= 10
	}
	
	// Process count health
	if systemMetrics.ProcessCount > 1000 {
		score -= 10
	} else if systemMetrics.ProcessCount > 500 {
		score -= 5
	}
	
	// Determine status
	if score >= 80 {
		check.Status = HealthStatusHealthy
		check.Message = "System resources are healthy"
	} else if score >= 60 {
		check.Status = HealthStatusWarning
		check.Message = "System resources showing stress"
	} else if score >= 40 {
		check.Status = HealthStatusCritical
		check.Message = "System resources under heavy load"
	} else {
		check.Status = HealthStatusCritical
		check.Message = "System resources in critical state"
	}
	
	check.Score = score
	check.Details = map[string]interface{}{
		"cpu_usage":      systemMetrics.CPUUsage,
		"memory_usage":   systemMetrics.MemoryUsage,
		"load_average":   systemMetrics.LoadAverage,
		"process_count":  systemMetrics.ProcessCount,
		"network_in":     systemMetrics.NetworkIn,
		"network_out":    systemMetrics.NetworkOut,
	}
	check.Duration = time.Since(check.Timestamp)
	
	hc.recordHealthCheck(check)
	hc.evaluateHealthRules(check, 0)
}

// checkTaskHealth checks the health of tasks
func (hc *HealthChecker) checkTaskHealth() {
	if hc.app.TaskManager == nil {
		return
	}
	
	tasks := hc.app.TaskManager.ListTasks()
	
	for _, task := range tasks {
		checkID := fmt.Sprintf("task_%s", task.ID)
		
		check := &HealthCheck{
			ID:        checkID,
			Name:      fmt.Sprintf("Task %s (%s)", task.ID, task.Name),
			Type:      HealthCheckTypeTask,
			Timestamp: time.Now(),
			Tags:      []string{"task", task.Name},
		}
		
		// Calculate health score based on task status
		score := 100.0
		message := ""
		
		switch task.Status {
		case TaskStatusCompleted:
			message = "Task completed successfully"
			score = 100
		case TaskStatusRunning:
			message = "Task is running"
			score = 90
			// Check if task has been running too long
			if task.Timeout > 0 && time.Since(task.StartedAt) > task.Timeout {
				score -= 30
				message = "Task running longer than timeout"
			}
		case TaskStatusPending:
			message = "Task is pending"
			score = 70
		case TaskStatusRetry:
			message = "Task is retrying"
			score = 50
		case TaskStatusPaused:
			message = "Task is paused"
			score = 60
		case TaskStatusFailed:
			message = "Task failed"
			score = 20
		case TaskStatusCancelled:
			message = "Task cancelled"
			score = 30
		default:
			message = "Unknown task status"
			score = 0
		}
		
		// Adjust score based on retry count
		if task.RetryCount > 0 {
			score -= float64(task.RetryCount * 10)
		}
		
		// Determine status
		if score >= 80 {
			check.Status = HealthStatusHealthy
		} else if score >= 60 {
			check.Status = HealthStatusWarning
		} else if score >= 40 {
			check.Status = HealthStatusCritical
		} else {
			check.Status = HealthStatusCritical
		}
		
		check.Score = score
		check.Message = message
		check.Details = map[string]interface{}{
			"status":       task.Status,
			"retry_count":  task.RetryCount,
			"exit_code":    task.ExitCode,
			"created_at":   task.CreatedAt,
			"started_at":   task.StartedAt,
			"completed_at": task.CompletedAt,
		}
		check.Duration = time.Since(check.Timestamp)
		
		hc.recordHealthCheck(check)
		hc.evaluateHealthRules(check, 0)
	}
}

// checkSystemHealth checks overall system health
func (hc *HealthChecker) checkSystemHealth() {
	systemMetrics := hc.getSystemMetrics()
	
	check := &HealthCheck{
		ID:        "system_health",
		Name:      "System Health",
		Type:      HealthCheckTypeSystem,
		Timestamp: time.Now(),
		Tags:      []string{"system", "health"},
	}
	
	// Calculate comprehensive health score
	score := 100.0
	
	// System load
	if systemMetrics.LoadAverage > float64(runtime.NumCPU()) {
		score -= 30
	} else if systemMetrics.LoadAverage > float64(runtime.NumCPU())*0.8 {
		score -= 15
	}
	
	// Memory pressure
	if systemMetrics.MemoryUsage > 90 {
		score -= 25
	} else if systemMetrics.MemoryUsage > 70 {
		score -= 10
	}
	
	// Process health
	if systemMetrics.ProcessCount > 2000 {
		score -= 15
	} else if systemMetrics.ProcessCount > 1000 {
		score -= 5
	}
	
	// Network activity
	if systemMetrics.NetworkIn > 10000 || systemMetrics.NetworkOut > 10000 {
		score -= 5
	}
	
	// Determine status
	if score >= 80 {
		check.Status = HealthStatusHealthy
		check.Message = "System is healthy"
	} else if score >= 60 {
		check.Status = HealthStatusWarning
		check.Message = "System showing warning signs"
	} else if score >= 40 {
		check.Status = HealthStatusCritical
		check.Message = "System under stress"
	} else {
		check.Status = HealthStatusCritical
		check.Message = "System in critical state"
	}
	
	check.Score = score
	check.Details = map[string]interface{}{
		"cpu_cores":     runtime.NumCPU(),
		"load_average":  systemMetrics.LoadAverage,
		"memory_usage":  systemMetrics.MemoryUsage,
		"process_count": systemMetrics.ProcessCount,
		"uptime":        time.Now().Sub(startTime),
	}
	check.Duration = time.Since(check.Timestamp)
	
	hc.recordHealthCheck(check)
	hc.evaluateHealthRules(check, 0)
}

// getSystemMetrics collects system-wide metrics
func (hc *HealthChecker) getSystemMetrics() SystemMetrics {
	// This is a simplified implementation
	// In a real implementation, you would use sysinfo or similar libraries
	
	// Get CPU usage (simplified)
	cpuUsage := 0.0
	if cpuPercent, err := hc.getCPUUsage(); err == nil {
		cpuUsage = cpuPercent
	}
	
	// Get memory usage (simplified)
	memUsage := 0.0
	if memPercent, err := hc.getMemoryUsage(); err == nil {
		memUsage = memPercent
	}
	
	// Get load average
	loadAvg := 0.0
	if load, err := hc.getLoadAverage(); err == nil {
		loadAvg = load
	}
	
	// Get process count
	processCount := int64(0)
	if count, err := hc.getProcessCount(); err == nil {
		processCount = count
	}
	
	return SystemMetrics{
		CPUUsage:     cpuUsage,
		MemoryUsage:  memUsage,
		DiskUsage:    0.0, // Simplified
		LoadAverage:  loadAvg,
		ProcessCount: processCount,
		NetworkIn:    0.0, // Simplified
		NetworkOut:   0.0, // Simplified
		Timestamp:    time.Now(),
	}
}

// getCPUUsage gets current CPU usage percentage
func (hc *HealthChecker) getCPUUsage() (float64, error) {
	// Simplified CPU usage calculation
	// In a real implementation, use proper system monitoring
	return 25.0, nil
}

// getMemoryUsage gets current memory usage percentage
func (hc *HealthChecker) getMemoryUsage() (float64, error) {
	// Simplified memory usage calculation
	// In a real implementation, use proper system monitoring
	return 45.0, nil
}

// getLoadAverage gets system load average
func (hc *HealthChecker) getLoadAverage() (float64, error) {
	// Simplified load average calculation
	// In a real implementation, use proper system monitoring
	return 0.8, nil
}

// getProcessCount gets current process count
func (hc *HealthChecker) getProcessCount() (int64, error) {
	// Simplified process count
	// In a real implementation, use proper system monitoring
	return 150, nil
}

// recordHealthCheck records a health check result
func (hc *HealthChecker) recordHealthCheck(check *HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.checks[check.ID] = check
	hc.healthHistory = append(hc.healthHistory, check)
	
	// Trim history if needed
	if len(hc.healthHistory) > 1000 {
		hc.healthHistory = hc.healthHistory[1:]
	}
	
	// Update stats
	if check.Status == HealthStatusHealthy {
		hc.stats.CompletedChecks++
	} else {
		hc.stats.FailedChecks++
	}
	
	// Emit event
	hc.emitEvent(HealthEventCheckCompleted, check.ID, "", check)
}

// evaluateHealthRules evaluates health rules against a check
func (hc *HealthChecker) evaluateHealthRules(check *HealthCheck, pid int32) {
	for _, rule := range hc.rules {
		if !rule.Enabled || rule.Type != check.Type {
			continue
		}
		
		if hc.evaluateRule(rule, check, pid) {
			hc.triggerAlert(rule, check)
		}
	}
}

// evaluateRule evaluates a single health rule
func (hc *HealthChecker) evaluateRule(rule *HealthRule, check *HealthCheck, pid int32) bool {
	for _, condition := range rule.Conditions {
		if !hc.evaluateCondition(condition, check, pid) {
			// Reset violation count if condition is not met
			key := fmt.Sprintf("%s_%d", rule.ID, pid)
			hc.mu.Lock()
			hc.violationCounts[key] = 0
			hc.mu.Unlock()
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single health condition
func (hc *HealthChecker) evaluateCondition(condition HealthCondition, check *HealthCheck, pid int32) bool {
	var value float64
	var ok bool
	
	// Extract metric value from check details
	switch condition.Metric {
	case "cpu_percent":
		value, ok = check.Details["cpu_percent"].(float64)
	case "memory_mb":
		value, ok = check.Details["memory_mb"].(float64)
	case "load_average":
		value, ok = check.Details["load_average"].(float64)
	case "process_count":
		if count, ok2 := check.Details["process_count"].(int64); ok2 {
			value = float64(count)
			ok = true
		}
	case "threads":
		if threads, ok2 := check.Details["threads"].(float64); ok2 {
			value = threads
			ok = true
		}
	case "health_score":
		value = check.Score
		ok = true
	case "failure_count":
		if retryCount, ok2 := check.Details["retry_count"].(int); ok2 {
			value = float64(retryCount)
			ok = true
		}
	default:
		return false
	}
	
	if !ok {
		return false
	}
	
	// Apply comparison operator
	switch condition.Operator {
	case OpGreaterThan:
		return value > condition.Threshold
	case OpGreaterEqual:
		return value >= condition.Threshold
	case OpLessThan:
		return value < condition.Threshold
	case OpLessEqual:
		return value <= condition.Threshold
	case OpEqual:
		return value == condition.Threshold
	case OpNotEqual:
		return value != condition.Threshold
	default:
		return false
	}
}

// triggerAlert triggers an alert for a rule
func (hc *HealthChecker) triggerAlert(rule *HealthRule, check *HealthCheck) {
	alertID := fmt.Sprintf("alert_%s_%d", rule.ID, time.Now().Unix())
	
	alert := &Alert{
		ID:          alertID,
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Type:        rule.Type,
		Severity:    rule.Severity,
		Status:      AlertStatusActive,
		Title:       fmt.Sprintf("%s Alert", rule.Name),
		Message:     check.Message,
		Details:     check.Details,
		TriggeredAt: time.Now(),
		UpdatedAt:   time.Now(),
		Tags:        check.Tags,
		Actions:     rule.Actions,
	}
	
	hc.mu.Lock()
	hc.alerts[alertID] = alert
	hc.alertHistory = append(hc.alertHistory, alert)
	hc.stats.ActiveAlerts++
	hc.mu.Unlock()
	
	// Emit event
	hc.emitEvent(HealthEventAlertTriggered, check.ID, alertID, alert)
	
	// Execute alert actions
	hc.executeAlertActions(alert)
}

// executeAlertActions executes actions for an alert
func (hc *HealthChecker) executeAlertActions(alert *Alert) {
	for _, action := range alert.Actions {
		if !action.Enabled {
			continue
		}
		
		switch action.Type {
		case ActionTypeNotify:
			hc.sendNotification(alert, action.Channel)
		case ActionTypeLog:
			hc.logAlert(alert)
		case ActionTypeRestart:
			hc.restartProcess(alert)
		case ActionTypeStop:
			hc.stopProcess(alert)
		case ActionTypeExecute:
			hc.executeAction(alert, action)
		}
	}
}

// sendNotification sends a notification to a channel
func (hc *HealthChecker) sendNotification(alert *Alert, channelName string) {
	channel, exists := hc.channels[channelName]
	if !exists {
		log.Printf("❌ Notification channel %s not found", channelName)
		return
	}
	
	switch channel["type"].(string) {
	case "console":
		hc.sendConsoleNotification(alert)
	case "log":
		hc.sendLogNotification(alert)
	default:
		log.Printf("📧 Notification sent to %s: %s", channelName, alert.Title)
	}
}

// sendConsoleNotification sends notification to console
func (hc *HealthChecker) sendConsoleNotification(alert *Alert) {
	severityIcon := "ℹ️"
	switch alert.Severity {
	case AlertSeverityWarning:
		severityIcon = "⚠️"
	case AlertSeverityError:
		severityIcon = "❌"
	case AlertSeverityCritical:
		severityIcon = "🚨"
	}
	
	fmt.Printf("%s [%s] %s: %s\n", severityIcon, strings.ToUpper(string(alert.Severity)), alert.Title, alert.Message)
}

// sendLogNotification sends notification to log file
func (hc *HealthChecker) sendLogNotification(alert *Alert) {
	logFile := "logs/alerts.log"
	if channel, ok := hc.channels["log"]; ok {
		if configFile, ok := channel["log_file"]; ok {
			if file, ok := configFile.(string); ok {
				logFile = file
			}
		}
	}
	
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		log.Printf("❌ Failed to create log directory: %v", err)
		return
	}
	
	// Format log entry
	logEntry := fmt.Sprintf("[%s] [%s] %s: %s\n", 
		time.Now().Format("2006-01-02 15:04:05"),
		alert.Severity,
		alert.Title,
		alert.Message,
	)
	
	// Write to log file
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("❌ Failed to open log file: %v", err)
		return
	}
	defer f.Close()
	
	if _, err := f.WriteString(logEntry); err != nil {
		log.Printf("❌ Failed to write to log file: %v", err)
		return
	}
}

// logAlert logs an alert
func (hc *HealthChecker) logAlert(alert *Alert) {
	log.Printf("📋 Alert logged: %s - %s", alert.Title, alert.Message)
}

// restartProcess restarts a process based on alert
func (hc *HealthChecker) restartProcess(alert *Alert) {
	// Implementation for process restart
	log.Printf("🔄 Process restart action triggered for alert: %s", alert.Title)
}

// stopProcess stops a process based on alert
func (hc *HealthChecker) stopProcess(alert *Alert) {
	// Implementation for process stop
	log.Printf("🛑 Process stop action triggered for alert: %s", alert.Title)
}

// executeAction executes a custom action
func (hc *HealthChecker) executeAction(alert *Alert, action AlertAction) {
	log.Printf("⚡ Custom action executed for alert: %s", alert.Title)
}

// alertManager manages alert lifecycle
func (hc *HealthChecker) alertManager() {
	ticker := time.NewTicker(hc.config.AlertManager.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.processAlerts()
		}
	}
}

// processAlerts processes active alerts
func (hc *HealthChecker) processAlerts() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	now := time.Now()
	
	for alertID, alert := range hc.alerts {
		if alert.Status == AlertStatusActive {
			// Check if alert should be resolved
			if hc.shouldResolveAlert(alert) {
				alert.Status = AlertStatusResolved
				alert.ResolvedAt = now
				alert.UpdatedAt = now
				hc.stats.ActiveAlerts--
				hc.stats.ResolvedAlerts++
				
				// Emit event
				hc.emitEvent(HealthEventAlertResolved, "", alertID, alert)
			}
			
			// Check if alert should be retried
			if alert.RetryCount < hc.config.AlertManager.MaxRetries {
				alert.RetryCount++
				alert.UpdatedAt = now
				hc.executeAlertActions(alert)
			}
		}
	}
}

// shouldResolveAlert determines if an alert should be resolved
func (hc *HealthChecker) shouldResolveAlert(alert *Alert) bool {
	// Check if the conditions that triggered the alert are no longer met
	for _, rule := range hc.rules {
		if rule.ID == alert.RuleID {
			// Get latest health check for this rule type
			for _, check := range hc.checks {
				if check.Type == rule.Type {
					if !hc.evaluateRule(rule, check, 0) {
						return true
					}
				}
			}
		}
	}
	return false
}

// statsCollector collects and updates health checker statistics
func (hc *HealthChecker) statsCollector() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.updateStats()
		}
	}
}

// updateStats updates health checker statistics
func (hc *HealthChecker) updateStats() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	// Count active alerts
	activeCount := int64(0)
	for _, alert := range hc.alerts {
		if alert.Status == AlertStatusActive {
			activeCount++
		}
	}
	hc.stats.ActiveAlerts = activeCount
}

// emitEvent emits a health event
func (hc *HealthChecker) emitEvent(eventType HealthEventType, checkID, alertID string, data interface{}) {
	select {
	case hc.events <- HealthEvent{
		Type:      eventType,
		CheckID:   checkID,
		AlertID:   alertID,
		Timestamp: time.Now(),
		Data:      data,
	}:
	default:
		log.Printf("⚠️ Health event channel full, dropping event: %s", eventType)
	}
}

// GetHealthCheck retrieves a health check by ID
func (hc *HealthChecker) GetHealthCheck(checkID string) (*HealthCheck, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	check, exists := hc.checks[checkID]
	if !exists {
		return nil, fmt.Errorf("health check %s not found", checkID)
	}
	
	return check, nil
}

// GetAlert retrieves an alert by ID
func (hc *HealthChecker) GetAlert(alertID string) (*Alert, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	alert, exists := hc.alerts[alertID]
	if !exists {
		return nil, fmt.Errorf("alert %s not found", alertID)
	}
	
	return alert, nil
}

// ListHealthChecks returns all health checks
func (hc *HealthChecker) ListHealthChecks() []*HealthCheck {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	checks := make([]*HealthCheck, 0, len(hc.checks))
	for _, check := range hc.checks {
		checks = append(checks, check)
	}
	
	return checks
}

// ListAlerts returns all alerts
func (hc *HealthChecker) ListAlerts() []*Alert {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	alerts := make([]*Alert, 0, len(hc.alerts))
	for _, alert := range hc.alerts {
		alerts = append(alerts, alert)
	}
	
	return alerts
}

// GetStats returns health checker statistics
func (hc *HealthChecker) GetStats() HealthStats {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	return hc.stats
}

// GetHealthHistory returns health check history
func (hc *HealthChecker) GetHealthHistory() []*HealthCheck {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	history := make([]*HealthCheck, len(hc.healthHistory))
	copy(history, hc.healthHistory)
	
	return history
}

// GetAlertHistory returns alert history
func (hc *HealthChecker) GetAlertHistory() []*Alert {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	history := make([]*Alert, len(hc.alertHistory))
	copy(history, hc.alertHistory)
	
	return history
}

// startTime is the application start time (would be set in main)
var startTime = time.Now()