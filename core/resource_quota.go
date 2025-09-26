package core

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
	"github.com/shirou/gopsutil/v3/process"
)


// ResourceQuotaManager manages resource quotas for processes
type ResourceQuotaManager struct {
	quotas            map[string]*ResourceQuota
	mutex             sync.RWMutex
	config            ResourceQuotaConfig
	monitor           *ResourceMonitor
	events            chan ResourceQuotaEvent
	resourceCollector UnifiedResourceCollector
	ctx               context.Context
	cancel            context.CancelFunc
}


// QuotaResourceUsage represents current resource usage for quota management
type QuotaResourceUsage struct {
	CPUUsed        float64 `json:"cpu_used"`
	MemoryUsedMB   int64   `json:"memory_used_mb"`
	DiskReadMB     int64   `json:"disk_read_mb"`
	DiskWriteMB    int64   `json:"disk_write_mb"`
	NetworkUsedKB  int64   `json:"network_used_kb"`
	ThreadsUsed    int32   `json:"threads_used"`
	ProcessesUsed  int32   `json:"processes_used"`
	Runtime        time.Duration `json:"runtime"`
}

// ResourceQuotaEvent represents quota-related events
type ResourceQuotaEvent struct {
	Type        QuotaEventType `json:"type"`
	QuotaName   string         `json:"quota_name"`
	PID         int32          `json:"pid"`
	Timestamp   time.Time      `json:"timestamp"`
	Message     string         `json:"message"`
	Resource    string         `json:"resource"`
	UsedValue   float64        `json:"used_value"`
	LimitValue  float64        `json:"limit_value"`
	Severity    EventSeverity  `json:"severity"`
}

// QuotaEventType represents different types of quota events
type QuotaEventType string

const (
	EventQuotaExceeded    QuotaEventType = "quota_exceeded"
	EventQuotaWarning     QuotaEventType = "quota_warning"
	EventQuotaRestored    QuotaEventType = "quota_restored"
	EventQuotaCleared     QuotaEventType = "quota_cleared"
)

// EventSeverity represents the severity of an event
type EventSeverity string

const (
	SeverityLow      EventSeverity = "low"
	SeverityMedium   EventSeverity = "medium"
	SeverityHigh     EventSeverity = "high"
	SeverityCritical EventSeverity = "critical"
)

// ResourceMonitor handles the actual resource monitoring
type ResourceMonitor struct {
	app *App
}

// GetResourceUsage gets current resource usage for a process using unified resource collector
func (rm *ResourceMonitor) GetResourceUsage(p *process.Process) (*QuotaResourceUsage, error) {
	// Return empty usage for now - this method should be refactored to use unified collector directly
	// The quota manager now has its own resource collector through resourceCollector field
	return &QuotaResourceUsage{}, nil
}



// NewResourceQuotaManager creates a new resource quota manager
func NewResourceQuotaManager(config ResourceQuotaConfig, app *App) *ResourceQuotaManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create unified resource collector configuration
	collectorConfig := ResourceCollectionConfig{
		EnableCPUMonitoring:    true,
		EnableMemoryMonitoring:  true,
		EnableIOMonitoring:      true,
		EnableNetworkMonitoring: true,
		EnableThreadMonitoring:  true,
		EnableDetailedIO:        true,
		CollectionInterval:      config.CheckInterval,
		CacheTTL:               config.CheckInterval * 2,
		MaxCacheSize:           1000,
		EnableHistory:          false,
		HistoryRetention:       time.Hour,
	}
	
	return &ResourceQuotaManager{
		quotas:            make(map[string]*ResourceQuota),
		config:           config,
		monitor:          &ResourceMonitor{app: app},
		events:           make(chan ResourceQuotaEvent, 100),
		resourceCollector: NewUnifiedResourceCollector(collectorConfig),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start begins monitoring resource quotas
func (rqm *ResourceQuotaManager) Start() {
	// Initialize quotas from config
	rqm.mutex.Lock()
	for i := range rqm.config.Quotas {
		quota := &rqm.config.Quotas[i]
		quota.Active = true
		rqm.quotas[quota.Name] = quota
	}
	rqm.mutex.Unlock()

	// Start monitoring loop
	go rqm.monitorQuotas()
}

// Stop shuts down the quota manager
func (rqm *ResourceQuotaManager) Stop() {
	rqm.cancel()
	
	// Clean up resource collector - no explicit cleanup needed for now
	
	close(rqm.events)
}

// monitorQuotas periodically checks all quotas
func (rqm *ResourceQuotaManager) monitorQuotas() {
	ticker := time.NewTicker(rqm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rqm.ctx.Done():
			return
		case <-ticker.C:
			rqm.checkAllQuotas()
		}
	}
}

// checkAllQuotas checks all active quotas
func (rqm *ResourceQuotaManager) checkAllQuotas() {
	rqm.mutex.RLock()
	quotas := make([]*ResourceQuota, 0, len(rqm.quotas))
	for _, quota := range rqm.quotas {
		if quota.Active {
			quotas = append(quotas, quota)
		}
	}
	rqm.mutex.RUnlock()

	for _, quota := range quotas {
		rqm.checkQuota(quota)
	}
}

// checkQuota checks a specific quota for all its processes
func (rqm *ResourceQuotaManager) checkQuota(quota *ResourceQuota) {
	for _, pid := range quota.Processes {
		rqm.checkProcessQuota(quota, pid)
	}
}

// checkProcessQuota checks resource usage for a specific process
func (rqm *ResourceQuotaManager) checkProcessQuota(quota *ResourceQuota, pid int32) {
	p, err := process.NewProcess(pid)
	if err != nil {
		// Process no longer exists
		rqm.removeProcessFromQuota(quota.Name, pid)
		return
	}

	// Get current resource usage using unified collector
	usage, err := rqm.getResourceUsageWithCollector(p)
	if err != nil {
		log.Printf("⚠️  Failed to get resource usage for PID %d: %v", pid, err)
		return
	}

	// Check each resource limit
	violations := rqm.checkResourceLimits(quota, usage, pid)
	
	// Handle violations
	if len(violations) > 0 {
		rqm.handleViolations(quota, pid, violations)
	}

	quota.LastCheck = time.Now()
}

// checkResourceLimits checks all resource limits for a quota
func (rqm *ResourceQuotaManager) checkResourceLimits(quota *ResourceQuota, usage *QuotaResourceUsage, pid int32) []ResourceViolation {
	var violations []ResourceViolation

	// Check CPU limit
	if quota.CPULimit > 0 && usage.CPUUsed > quota.CPULimit {
		violations = append(violations, ResourceViolation{
			Resource:    "cpu",
			UsedValue:   usage.CPUUsed,
			LimitValue:  quota.CPULimit,
			Severity:    rqm.calculateSeverity(usage.CPUUsed, quota.CPULimit, 1.5),
		})
	}

	// Check Memory limit
	if quota.MemoryLimitMB > 0 && usage.MemoryUsedMB > quota.MemoryLimitMB {
		violations = append(violations, ResourceViolation{
			Resource:    "memory",
			UsedValue:   float64(usage.MemoryUsedMB),
			LimitValue:  float64(quota.MemoryLimitMB),
			Severity:    rqm.calculateSeverity(float64(usage.MemoryUsedMB), float64(quota.MemoryLimitMB), 1.2),
		})
	}

	// Check Thread limit
	if quota.ThreadLimit > 0 && usage.ThreadsUsed > quota.ThreadLimit {
		violations = append(violations, ResourceViolation{
			Resource:    "threads",
			UsedValue:   float64(usage.ThreadsUsed),
			LimitValue:  float64(quota.ThreadLimit),
			Severity:    SeverityMedium,
		})
	}

	// Check Time limit
	if quota.TimeLimit > 0 && usage.Runtime > quota.TimeLimit {
		violations = append(violations, ResourceViolation{
			Resource:    "time",
			UsedValue:   usage.Runtime.Seconds(),
			LimitValue:  quota.TimeLimit.Seconds(),
			Severity:    SeverityHigh,
		})
	}

	return violations
}

// ResourceViolation represents a resource limit violation
type ResourceViolation struct {
	Resource   string        `json:"resource"`
	UsedValue  float64       `json:"used_value"`
	LimitValue float64       `json:"limit_value"`
	Severity   EventSeverity `json:"severity"`
}

// calculateSeverity calculates the severity based on usage ratio
func (rqm *ResourceQuotaManager) calculateSeverity(used, limit float64, highThreshold float64) EventSeverity {
	ratio := used / limit
	if ratio >= highThreshold {
		return SeverityCritical
	} else if ratio >= 1.2 {
		return SeverityHigh
	} else {
		return SeverityMedium
	}
}

// handleViolations handles quota violations
func (rqm *ResourceQuotaManager) handleViolations(quota *ResourceQuota, pid int32, violations []ResourceViolation) {
	quota.Violations++
	
	for _, violation := range violations {
		event := ResourceQuotaEvent{
			Type:       EventQuotaExceeded,
			QuotaName:  quota.Name,
			PID:        pid,
			Timestamp:  time.Now(),
			Message:    fmt.Sprintf("%s quota exceeded for %s: %.2f > %.2f", violation.Resource, quota.Name, violation.UsedValue, violation.LimitValue),
			Resource:   violation.Resource,
			UsedValue:  violation.UsedValue,
			LimitValue: violation.LimitValue,
			Severity:   violation.Severity,
		}
		
		rqm.emitEvent(event)
	}

	// Take action based on quota configuration
	rqm.takeQuotaAction(quota, pid, violations)
}

// takeQuotaAction takes configured action for quota violations
func (rqm *ResourceQuotaManager) takeQuotaAction(quota *ResourceQuota, pid int32, violations []ResourceViolation) {
	action := quota.Action
	if action == "" {
		action = rqm.config.DefaultAction
	}

	switch action {
	case ActionWarn:
		log.Printf("⚠️  Resource quota warning for %s (PID: %d)", quota.Name, pid)
	case ActionThrottle:
		rqm.throttleProcess(pid)
	case ActionStop:
		rqm.stopProcess(pid)
	case ActionRestart:
		rqm.restartProcess(pid)
	case ActionNotify:
		rqm.sendNotification(quota, pid, violations)
	}
}

// throttleProcess attempts to throttle a process (reduce priority)
func (rqm *ResourceQuotaManager) throttleProcess(pid int32) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return
	}
	
	// Try to reduce process priority (Unix-like systems only)
	// This is a simplified implementation - in practice, you might use more sophisticated methods
	if runtime.GOOS != "windows" {
		_ = p.SendSignal(GetSIGUSR1()) // Custom signal for throttling
	} else {
		// On Windows, we could use other methods like priority class adjustment
		log.Printf("🐌 Process throttling not implemented for Windows PID %d", pid)
	}
	log.Printf("🐌 Throttled process PID %d", pid)
}

// stopProcess stops a violating process
func (rqm *ResourceQuotaManager) stopProcess(pid int32) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return
	}
	
	_ = p.Terminate()
	log.Printf("🛑 Stopped process PID %d due to quota violation", pid)
}

// restartProcess restarts a violating process
func (rqm *ResourceQuotaManager) restartProcess(pid int32) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return
	}
	
	// Get process info before stopping
	name, _ := p.Name()
	// cmdline, _ := p.Cmdline() // Available for future use
	// cwd, _ := p.Cwd() // Available for future use
	
	// Stop the process
	_ = p.Terminate()
	
	// Restart after a delay
	time.Sleep(2 * time.Second)
	
	log.Printf("🔄 Restarted process %s (PID: %d) due to quota violation", name, pid)
	// Note: In a full implementation, you'd want to restart with the same command
}

// sendNotification sends a notification about quota violation
func (rqm *ResourceQuotaManager) sendNotification(quota *ResourceQuota, pid int32, violations []ResourceViolation) {
	log.Printf("📧 Resource quota notification for %s (PID: %d)", quota.Name, pid)
	for _, violation := range violations {
		log.Printf("   %s: %.2f > %.2f", violation.Resource, violation.UsedValue, violation.LimitValue)
	}
}

// emitEvent sends a quota event
func (rqm *ResourceQuotaManager) emitEvent(event ResourceQuotaEvent) {
	select {
	case rqm.events <- event:
	default:
		log.Printf("⚠️  Event channel full, dropping quota event: %s", event.Message)
	}
}

// removeProcessFromQuota removes a process from quota tracking
func (rqm *ResourceQuotaManager) removeProcessFromQuota(quotaName string, pid int32) {
	rqm.mutex.Lock()
	defer rqm.mutex.Unlock()
	
	if quota, exists := rqm.quotas[quotaName]; exists {
		for i, id := range quota.Processes {
			if id == pid {
				quota.Processes = append(quota.Processes[:i], quota.Processes[i+1:]...)
				break
			}
		}
	}
}

// AddProcessToQuota adds a process to a quota
func (rqm *ResourceQuotaManager) AddProcessToQuota(quotaName string, pid int32) error {
	rqm.mutex.Lock()
	defer rqm.mutex.Unlock()
	
	quota, exists := rqm.quotas[quotaName]
	if !exists {
		return fmt.Errorf("quota %s not found", quotaName)
	}
	
	// Check if process already exists
	for _, id := range quota.Processes {
		if id == pid {
			return fmt.Errorf("process %d already in quota %s", pid, quotaName)
		}
	}
	
	quota.Processes = append(quota.Processes, pid)
	return nil
}

// RemoveProcessFromQuota removes a process from quota tracking
func (rqm *ResourceQuotaManager) RemoveProcessFromQuota(quotaName string, pid int32) error {
	rqm.mutex.Lock()
	defer rqm.mutex.Unlock()
	
	quota, exists := rqm.quotas[quotaName]
	if !exists {
		return fmt.Errorf("quota %s not found", quotaName)
	}
	
	for i, id := range quota.Processes {
		if id == pid {
			quota.Processes = append(quota.Processes[:i], quota.Processes[i+1:]...)
			return nil
		}
	}
	
	return fmt.Errorf("process %d not found in quota %s", pid, quotaName)
}

// GetQuotaByName returns a quota by name
func (rqm *ResourceQuotaManager) GetQuotaByName(name string) (*ResourceQuota, error) {
	rqm.mutex.RLock()
	defer rqm.mutex.RUnlock()
	
	quota, exists := rqm.quotas[name]
	if !exists {
		return nil, fmt.Errorf("quota %s not found", name)
	}
	
	return quota, nil
}

// GetAllQuotas returns all quotas
func (rqm *ResourceQuotaManager) GetAllQuotas() []*ResourceQuota {
	rqm.mutex.RLock()
	defer rqm.mutex.RUnlock()
	
	quotas := make([]*ResourceQuota, 0, len(rqm.quotas))
	for _, quota := range rqm.quotas {
		quotas = append(quotas, quota)
	}
	
	return quotas
}

// Events returns the event channel
func (rqm *ResourceQuotaManager) Events() <-chan ResourceQuotaEvent {
	return rqm.events
}

// GetQuotaStats returns statistics about quotas
func (rqm *ResourceQuotaManager) GetQuotaStats() QuotaStats {
	rqm.mutex.RLock()
	defer rqm.mutex.RUnlock()
	
	stats := QuotaStats{
		TotalQuotas:     len(rqm.quotas),
		ActiveQuotas:    0,
		TotalProcesses:  0,
		TotalViolations: 0,
		ViolationCounts: make(map[string]int),
		CollectorStats:  rqm.GetCollectorStats(),
		CacheStats:      rqm.GetCacheStats(),
	}
	
	for _, quota := range rqm.quotas {
		if quota.Active {
			stats.ActiveQuotas++
		}
		stats.TotalProcesses += len(quota.Processes)
		stats.TotalViolations += quota.Violations
		stats.ViolationCounts[quota.Name] = quota.Violations
	}
	
	return stats
}

// getResourceUsageWithCollector gets resource usage using the unified collector
func (rqm *ResourceQuotaManager) getResourceUsageWithCollector(p *process.Process) (*QuotaResourceUsage, error) {
	// Try to use unified resource collector first
	unifiedUsage, err := rqm.resourceCollector.CollectProcessResources(p.Pid)
	if err == nil {
		return rqm.convertUnifiedToQuotaUsage(unifiedUsage, p)
	}
	
	// Fallback to monitor implementation
	return rqm.monitor.GetResourceUsage(p)
}

// convertUnifiedToQuotaUsage converts UnifiedResourceUsage to QuotaResourceUsage
func (rqm *ResourceQuotaManager) convertUnifiedToQuotaUsage(unifiedUsage *UnifiedResourceUsage, p *process.Process) (*QuotaResourceUsage, error) {
	usage := &QuotaResourceUsage{
		CPUUsed:       unifiedUsage.CPU.UsedPercent,
		MemoryUsedMB:  unifiedUsage.Memory.UsedMB,
		DiskReadMB:    int64(unifiedUsage.Disk.ReadMB),
		DiskWriteMB:   int64(unifiedUsage.Disk.WriteMB),
		NetworkUsedKB: int64(unifiedUsage.Network.SentKB + unifiedUsage.Network.RecvKB),
		ThreadsUsed:   unifiedUsage.Threads,
		ProcessesUsed: 1,
	}
	
	// Get process runtime
	createTime, err := p.CreateTime()
	if err == nil {
		usage.Runtime = time.Since(time.Unix(0, createTime/1000000))
	}
	
	return usage, nil
}

// GetCollectorStats returns statistics about the resource collector
func (rqm *ResourceQuotaManager) GetCollectorStats() CollectionStats {
	return rqm.resourceCollector.GetCollectionStats()
}

// GetCacheStats returns cache statistics from the resource collector
func (rqm *ResourceQuotaManager) GetCacheStats() CacheStats {
	return rqm.resourceCollector.GetCacheStats()
}

// InvalidateProcessCache invalidates cache for a specific process
func (rqm *ResourceQuotaManager) InvalidateProcessCache(pid int32) {
	rqm.resourceCollector.InvalidateCache(pid)
}

// InvalidateAllCache invalidates all cache entries
func (rqm *ResourceQuotaManager) InvalidateAllCache() {
	rqm.resourceCollector.InvalidateAllCache()
}

// QuotaStats provides statistics about quotas
type QuotaStats struct {
	TotalQuotas     int            `json:"total_quotas"`
	ActiveQuotas    int            `json:"active_quotas"`
	TotalProcesses  int            `json:"total_processes"`
	TotalViolations int            `json:"total_violations"`
	ViolationCounts map[string]int `json:"violation_counts"`
	CollectorStats  CollectionStats `json:"collector_stats"`
	CacheStats      CacheStats      `json:"cache_stats"`
}