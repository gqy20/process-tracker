package core

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// ProcessInfo represents process information for monitoring
type ProcessInfo struct {
	Pid         int32
	Ppid        int32   // Parent Process ID
	Name        string
	Cmdline     string
	Cwd         string
	CPUPercent  float64
	MemoryMB    float64
	Threads     int32
	DiskReadMB  float64
	DiskWriteMB float64
	NetSentKB   float64
	NetRecvKB   float64
	CreateTime  int64   // Process start time (Unix timestamp in milliseconds)
	CPUTime     float64 // Cumulative CPU time in seconds (User + System)
}

// App represents the simplified application core
type App struct {
	DataFile string
	Interval time.Duration
	Config   Config

	// Storage interface (supports both CSV and SQLite)
	storage Storage

	// Docker monitoring
	dockerMonitor *DockerMonitor

	// Alert manager
	alertManager *AlertManager
}

// NewApp creates a new application instance
func NewApp(dataFile string, interval time.Duration, config Config) *App {
	// Create storage based on configuration
	storage := NewStorage(dataFile, 100, true, config.Storage)

	// Create Docker monitor
	dockerMonitor, err := NewDockerMonitor(config)
	if err != nil {
		log.Printf("Warning: Failed to create Docker monitor: %v", err)
		dockerMonitor = nil
	}

	// Create Alert manager if alerts are enabled
	var alertManager *AlertManager
	if config.Alerts.Enabled {
		alertManager = NewAlertManager(config.Alerts, config.Notifiers)
		log.Printf("Alert manager initialized with %d rules", len(config.Alerts.Rules))
	}

	return &App{
		DataFile:      dataFile,
		Interval:      interval,
		Config:        config,
		storage:       storage,
		dockerMonitor: dockerMonitor,
		alertManager:  alertManager,
	}
}

// Initialize initializes the application
func (a *App) Initialize() error {
	// Validate configuration before initialization
	if err := ValidateConfig(a.Config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Initialize storage with rotation support
	if err := a.storage.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Log storage configuration (simplified)
	log.Printf("Storage: max=%dMB total, keep=%d days, auto-rotation enabled",
		a.Config.Storage.MaxSizeMB,
		a.Config.Storage.KeepDays)

	// Initialize total memory cache
	SystemMemoryMB()

	// Start Docker monitoring if enabled
	if a.dockerMonitor != nil {
		if err := a.dockerMonitor.Start(); err != nil {
			log.Printf("Warning: Failed to start Docker monitoring: %v", err)
		}
	}

	return nil
}

// CloseFile closes file handles and cleans up resources
func (a *App) CloseFile() error {
	// Stop Docker monitoring
	if a.dockerMonitor != nil {
		if err := a.dockerMonitor.Stop(); err != nil {
			log.Printf("Warning: Failed to stop Docker monitoring: %v", err)
		}
	}

	return a.storage.Close()
}

// SaveResourceRecords saves multiple resource records
func (a *App) SaveResourceRecords(records []ResourceRecord) error {
	return a.storage.SaveRecords(records)
}

// SaveResourceRecord saves a single resource record
func (a *App) SaveResourceRecord(record ResourceRecord) error {
	return a.storage.SaveRecord(record)
}

// ReadResourceRecords reads resource records from file
func (a *App) ReadResourceRecords(filePath string) ([]ResourceRecord, error) {
	return a.storage.ReadRecords(filePath)
}

// CalculateResourceStats calculates resource statistics for a given time period
func (a *App) CalculateResourceStats(period time.Duration) ([]ResourceStats, error) {
	// Initialize storage if not already initialized
	if err := a.storage.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	records, err := a.ReadResourceRecords(a.DataFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read records: %w", err)
	}

	// Filter records by time period
	cutoff := time.Now().Add(-period)
	var filteredRecords []ResourceRecord
	for _, record := range records {
		if record.Timestamp.After(cutoff) {
			filteredRecords = append(filteredRecords, record)
		}
	}

	return a.storage.CalculateStats(filteredRecords), nil
}

// CompareStats compares statistics between two time periods
func (a *App) CompareStats(period1, period2 time.Duration, name1, name2 string) error {
	stats1, err := a.CalculateResourceStats(period1)
	if err != nil {
		return fmt.Errorf("failed to get stats for %s: %w", name1, err)
	}

	stats2, err := a.CalculateResourceStats(period2)
	if err != nil {
		return fmt.Errorf("failed to get stats for %s: %w", name2, err)
	}

	// Create maps for easier comparison
	statsMap1 := make(map[string]ResourceStats)
	statsMap2 := make(map[string]ResourceStats)

	for _, stat := range stats1 {
		statsMap1[stat.Name] = stat
	}
	for _, stat := range stats2 {
		statsMap2[stat.Name] = stat
	}

	// Display comparison
	fmt.Printf("\n📊 对比分析: %s vs %s\n", name1, name2)
	fmt.Println(strings.Repeat("═", 100))

	// Get all unique process names
	allNames := make(map[string]bool)
	for name := range statsMap1 {
		allNames[name] = true
	}
	for name := range statsMap2 {
		allNames[name] = true
	}

	// Display comparison table
	fmt.Printf("%-25s %18s %18s %15s\n", "进程名称", name1+"(内存)", name2+"(内存)", "变化")
	fmt.Println(strings.Repeat("─", 100))

	for name := range allNames {
		stat1, exists1 := statsMap1[name]
		stat2, exists2 := statsMap2[name]

		if exists1 && exists2 {
			mem1 := stat1.MemoryAvg
			mem2 := stat2.MemoryAvg
			change := ((mem2 - mem1) / mem1) * 100

			var changeStr string
			if change > 0 {
				changeStr = fmt.Sprintf("↑ +%.1f%%", change)
			} else if change < 0 {
				changeStr = fmt.Sprintf("↓ %.1f%%", change)
			} else {
				changeStr = "→ 0.0%"
			}

			fmt.Printf("%-25s %18s %18s %15s\n",
				truncateString(name, 25), formatMemory(mem1), formatMemory(mem2), changeStr)
		} else if exists1 {
			fmt.Printf("%-25s %18s %18s %15s\n",
				truncateString(name, 25), formatMemory(stat1.MemoryAvg), "-", "已消失")
		} else if exists2 {
			fmt.Printf("%-25s %18s %18s %15s\n",
				truncateString(name, 25), "-", formatMemory(stat2.MemoryAvg), "新出现")
		}
	}

	return nil
}

// ShowTrends shows resource usage trends over multiple days
func (a *App) ShowTrends(days int) error {
	if days <= 0 {
		days = 7
	}

	fmt.Printf("\n📈 资源使用趋势 (最近 %d 天)\n", days)
	fmt.Println(strings.Repeat("═", 100))

	// Collect stats for each day
	type DayStats struct {
		Day        int
		TotalProc  int
		AvgCPU     float64
		TotalMem   float64
		TotalDisk  float64
	}

	var dailyStats []DayStats

	for day := 0; day < days; day++ {
		startPeriod := time.Duration(day*24) * time.Hour
		endPeriod := time.Duration((day+1)*24) * time.Hour

		records, err := a.ReadResourceRecords(a.DataFile)
		if err != nil {
			return fmt.Errorf("failed to read records: %w", err)
		}

		// Filter records for this day
		now := time.Now()
		startTime := now.Add(-endPeriod)
		endTime := now.Add(-startPeriod)

		var dayRecords []ResourceRecord
		for _, record := range records {
			if record.Timestamp.After(startTime) && record.Timestamp.Before(endTime) {
				dayRecords = append(dayRecords, record)
			}
		}

		if len(dayRecords) == 0 {
			continue
		}

		stats := a.storage.CalculateStats(dayRecords)

		// Calculate aggregates
		var totalCPU, totalMem, totalDisk float64
		for _, stat := range stats {
			totalCPU += stat.CPUAvg
			totalMem += stat.MemoryAvg
			totalDisk += stat.DiskReadAvg + stat.DiskWriteAvg
		}

		avgCPU := 0.0
		if len(stats) > 0 {
			avgCPU = totalCPU / float64(len(stats))
		}

		dailyStats = append(dailyStats, DayStats{
			Day:       day,
			TotalProc: len(stats),
			AvgCPU:    avgCPU,
			TotalMem:  totalMem,
			TotalDisk: totalDisk,
		})
	}

	// Display trend table
	fmt.Printf("%-15s %12s %12s %18s %18s\n", "日期", "进程数", "平均CPU", "总内存", "总磁盘I/O")
	fmt.Println(strings.Repeat("─", 100))

	for i := len(dailyStats) - 1; i >= 0; i-- {
		stat := dailyStats[i]
		dateStr := fmt.Sprintf("%d天前", stat.Day)
		if stat.Day == 0 {
			dateStr = "今天"
		} else if stat.Day == 1 {
			dateStr = "昨天"
		}

		fmt.Printf("%-15s %12d %11.1f%% %18s %18s\n",
			dateStr, stat.TotalProc, stat.AvgCPU, formatMemory(stat.TotalMem), formatMemory(stat.TotalDisk))
	}

	// Show trend indicators
	if len(dailyStats) >= 2 {
		fmt.Println("\n趋势分析:")
		recent := dailyStats[0]
		older := dailyStats[len(dailyStats)-1]

		memTrend := ((recent.TotalMem - older.TotalMem) / older.TotalMem) * 100
		cpuTrend := ((recent.AvgCPU - older.AvgCPU) / older.AvgCPU) * 100

		if memTrend > 10 {
			fmt.Printf("  ⚠️  内存使用上升 %.1f%%\n", memTrend)
		} else if memTrend < -10 {
			fmt.Printf("  ✅ 内存使用下降 %.1f%%\n", -memTrend)
		}

		if cpuTrend > 10 {
			fmt.Printf("  ⚠️  CPU使用上升 %.1f%%\n", cpuTrend)
		} else if cpuTrend < -10 {
			fmt.Printf("  ✅ CPU使用下降 %.1f%%\n", -cpuTrend)
		}
	}

	return nil
}

// CleanOldData removes old data files
func (a *App) CleanOldData(keepDays int) error {
	return a.storage.CleanOldData(keepDays)
}

// GetTotalRecords returns the total number of records
func (a *App) GetTotalRecords() (int, error) {
	return a.storage.GetRecordCount()
}

// truncateString truncates string to specified length
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// formatMemory formats memory size with appropriate unit (MB/GB/TB)
func formatMemory(mb float64) string {
	if mb >= 1024*1024 { // >= 1TB
		return fmt.Sprintf("%.2fTB", mb/1024/1024)
	} else if mb >= 1024 { // >= 1GB
		return fmt.Sprintf("%.2fGB", mb/1024)
	} else if mb >= 1 {
		return fmt.Sprintf("%.1fMB", mb)
	} else {
		return fmt.Sprintf("%.2fKB", mb*1024)
	}
}

// GetProcessInfo gets detailed process information
// Returns an error only if the process name cannot be retrieved (critical failure)
func (a *App) GetProcessInfo(p *process.Process) (ProcessInfo, error) {
	info := ProcessInfo{Pid: p.Pid}

	// Process name is critical - return error if unavailable
	name, err := p.Name()
	if err != nil {
		return ProcessInfo{}, err
	}
	info.Name = name

	// Other fields are optional - ignore errors
	if ppid, err := p.Ppid(); err == nil {
		info.Ppid = ppid
	}
	if cmdline, err := p.Cmdline(); err == nil {
		info.Cmdline = cmdline
	}
	if cwd, err := p.Cwd(); err == nil {
		info.Cwd = cwd
	}
	if cpuPercent, err := p.CPUPercent(); err == nil {
		info.CPUPercent = cpuPercent
	}
	if memInfo, err := p.MemoryInfo(); err == nil {
		info.MemoryMB = float64(memInfo.RSS) / 1024 / 1024
	}
	if threads, err := p.NumThreads(); err == nil {
		info.Threads = threads
	}

	// Get disk I/O statistics
	if ioCounters, err := p.IOCounters(); err == nil {
		info.DiskReadMB = float64(ioCounters.ReadBytes) / 1024 / 1024
		info.DiskWriteMB = float64(ioCounters.WriteBytes) / 1024 / 1024
	}

	// Get process creation time
	if createTime, err := p.CreateTime(); err == nil {
		info.CreateTime = createTime
	}

	// Get cumulative CPU time
	if times, err := p.Times(); err == nil {
		info.CPUTime = times.User + times.System
	}

	// Get network statistics (not implemented - always returns 0)
	info.NetSentKB, info.NetRecvKB = a.getNetworkStats(p)

	return info, nil
}

// getNetworkStats estimates network usage based on connection patterns
// NOTE: Network monitoring is not implemented in the current version.
// This function always returns 0.0 for both sent and received bytes.
// Implementing accurate per-process network monitoring requires:
// - Parsing /proc/net/tcp, /proc/net/udp for connection tracking
// - Using eBPF or similar kernel-level monitoring
// - Significant performance overhead
// For network monitoring, consider using specialized tools like nethogs or iftop.
func (a *App) getNetworkStats(p *process.Process) (float64, float64) {
	// NOT IMPLEMENTED: Always returns zero
	return 0.0, 0.0
}

// GetCurrentResources gets current resource usage for all processes
func (a *App) GetCurrentResources() ([]ResourceRecord, error) {
	// PHASE 1: Get all processes and establish CPU baseline
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	// Build process map and establish CPU baseline
	processMap := make(map[int32]*process.Process)
	for _, p := range processes {
		p.CPUPercent() // Establish baseline, ignore return value
		processMap[p.Pid] = p
	}

	// PHASE 2: Wait 500ms for CPU sampling
	time.Sleep(500 * time.Millisecond)

	// PHASE 3: Collect accurate CPU values
	var records []ResourceRecord
	for _, p := range processMap {
		info, err := a.GetProcessInfo(p)
		if err != nil {
			continue
		}

		name := strings.TrimSpace(info.Name)

		if name == "" || info.Pid <= 0 {
			continue
		}

		// Skip system processes
		if a.isSystemProcess(name) {
			continue
		}

		// Normalize process name
		name = a.normalizeProcessName(name)

		// Create resource record
		record := ResourceRecord{
			Name:                 name,
			Timestamp:            time.Now(),
			CPUPercent:           info.CPUPercent,
			CPUPercentNormalized: CalculateCPUPercentNormalized(info.CPUPercent),
			MemoryMB:             info.MemoryMB,
			MemoryPercent:        CalculateMemoryPercent(info.MemoryMB),
			Threads:              info.Threads,
			DiskReadMB:           info.DiskReadMB,
			DiskWriteMB:          info.DiskWriteMB,
			NetSentKB:            info.NetSentKB,
			NetRecvKB:            info.NetRecvKB,
			IsActive:             false, // Will be set below
			Command:              info.Cmdline,
			WorkingDir:           info.Cwd,
			Category:             "", // Will be set below
			PID:                  info.Pid,
			PPID:                 info.Ppid,
			CreateTime:           info.CreateTime,
			CPUTime:              info.CPUTime,
		}

		// Determine if process is active
		activityConfig := GetDefaultActivityConfig()
		record.IsActive = IsActive(record, activityConfig)

		// Set application category
		record.Category = IdentifyApplication(name, info.Cmdline, a.Config.EnableSmartCategories)

		records = append(records, record)
	}

	return records, nil
}

// isSystemProcess checks if a process is a system process
func (a *App) isSystemProcess(name string) bool {
	name = strings.ToLower(name)
	// Only filter kernel threads and core system daemons
	// DO NOT filter user shells (bash, sh, zsh) or user programs (ssh, etc)
	systemPrefixes := []string{
		"kworker", "ksoftirqd", "migration", "rcu_", "watchdog",
		"khugepaged", "kthreadd", "kswapd", "cpuhp",
		"irq/", "kdevtmpfs", "netns", "kauditd",
		"khungtaskd", "oom_reaper", "writeback", "kcompactd",
		"md", "jbd2", "ext4-", "xfs-",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	// Only truly low-level system processes
	systemProcesses := map[string]bool{
		"systemd": true,  // init system (too many instances)
		"init":    true,  // legacy init
	}

	return systemProcesses[name]
}

// normalizeProcessName normalizes process name
func (a *App) normalizeProcessName(name string) string {
	name = strings.TrimSuffix(name, ".exe")
	name = strings.TrimSuffix(name, ".so")
	return strings.TrimSpace(name)
}

// GetProcessNameWithContext gets process name with additional context
func (a *App) GetProcessNameWithContext(info ProcessInfo) string {
	if info.Cmdline != "" {
		// Extract meaningful name from command line
		parts := strings.Fields(info.Cmdline)
		if len(parts) > 0 {
			// Get the base name without path
			lastPart := parts[len(parts)-1]
			if strings.Contains(lastPart, "/") {
				parts := strings.Split(lastPart, "/")
				lastPart = parts[len(parts)-1]
			}
			return lastPart
		}
	}
	return info.Name
}

// CollectAndSaveData collects process data and saves it to storage
func (a *App) CollectAndSaveData() error {
	// PHASE 1: Get all processes and establish CPU baseline
	processes, err := process.Processes()
	if err != nil {
		return fmt.Errorf("failed to get processes: %w", err)
	}

	// Build process map and establish CPU baseline
	processMap := make(map[int32]*process.Process)
	for _, p := range processes {
		p.CPUPercent() // Establish baseline, ignore return value
		processMap[p.Pid] = p
	}

	// PHASE 2: Wait 500ms for CPU sampling
	time.Sleep(500 * time.Millisecond)

	// PHASE 3: Collect accurate CPU values
	var records []ResourceRecord
	totalProcesses := len(processes)
	filteredCount := 0
	errorCount := 0

	for _, p := range processMap {
		info, err := a.GetProcessInfo(p)
		if err != nil {
			errorCount++
			continue // Skip processes we can't get info for
		}

		// Skip system processes
		if a.isSystemProcess(info.Name) {
			filteredCount++
			continue
		}

		// Normalize process name
		name := a.normalizeProcessName(info.Name)

		// Create resource record
		record := ResourceRecord{
			Name:                 name,
			Timestamp:            time.Now(),
			CPUPercent:           info.CPUPercent,
			CPUPercentNormalized: CalculateCPUPercentNormalized(info.CPUPercent),
			MemoryMB:             info.MemoryMB,
			MemoryPercent:        CalculateMemoryPercent(info.MemoryMB),
			Threads:              info.Threads,
			DiskReadMB:           info.DiskReadMB,
			DiskWriteMB:          info.DiskWriteMB,
			NetSentKB:            info.NetSentKB,
			NetRecvKB:            info.NetRecvKB,
			IsActive:             false, // Will be set below
			Command:              info.Cmdline,
			WorkingDir:           info.Cwd,
			Category:             IdentifyApplication(name, info.Cmdline, a.Config.EnableSmartCategories),
			PID:                  info.Pid,
			PPID:                 info.Ppid,
			CreateTime:           info.CreateTime,
			CPUTime:              info.CPUTime,
		}

		// Set active status based on thresholds
		config := GetDefaultActivityConfig()
		record.IsActive = IsActive(record, config)

		records = append(records, record)
	}

	// Add Docker container records
	dockerRecords := a.collectDockerContainerRecords()
	records = append(records, dockerRecords...)

	// Log collection statistics (every 12 cycles, i.e., every minute at 5s interval)
	if len(records) == 0 || len(records) < 10 {
		log.Printf("⚠️  Collected %d processes (total=%d, filtered=%d, errors=%d)", 
			len(records), totalProcesses, filteredCount, errorCount)
	}

	// Save all records
	if len(records) > 0 {
		if err := a.storage.SaveRecords(records); err != nil {
			return err
		}
	}

	// Evaluate alert rules if alert manager is enabled
	if a.alertManager != nil && len(records) > 0 {
		a.alertManager.Evaluate(records)
	}

	return nil
}



// collectDockerContainerRecords collects Docker container statistics
func (a *App) collectDockerContainerRecords() []ResourceRecord {
	if a.dockerMonitor == nil {
		return []ResourceRecord{}
	}

	stats, err := a.dockerMonitor.GetContainerStats()
	if err != nil {
		log.Printf("Warning: Failed to collect Docker stats: %v", err)
		return []ResourceRecord{}
	}

	var records []ResourceRecord
	for _, stat := range stats {
		memoryMB := float64(stat.MemoryUsage) / 1024 / 1024 // Convert to MB
		record := ResourceRecord{
			Name:                 fmt.Sprintf("docker:%s", stat.ContainerName),
			Timestamp:            stat.Timestamp,
			CPUPercent:           stat.CPUPercent,
			CPUPercentNormalized: CalculateCPUPercentNormalized(stat.CPUPercent),
			MemoryMB:             memoryMB,
			MemoryPercent:        CalculateMemoryPercent(memoryMB),
			Threads:              0, // Not available for containers
			DiskReadMB:           float64(stat.BlockReadBytes) / 1024 / 1024,
			DiskWriteMB:          float64(stat.BlockWriteBytes) / 1024 / 1024,
			NetSentKB:            float64(stat.NetworkTxBytes) / 1024,
			NetRecvKB:            float64(stat.NetworkRxBytes) / 1024,
			IsActive:             stat.CPUPercent > 1.0 || stat.MemoryPercent > 1.0,
			Command:              fmt.Sprintf("container:%s", stat.Image),
			WorkingDir:           "",
			Category:             "docker",
			PID:                  stat.PID,
			CreateTime:           stat.CreatedTime,
			CPUTime:              stat.CPUTime,
		}
		records = append(records, record)
	}

	return records
}
