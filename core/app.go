package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// ProcessInfo represents process information for monitoring
type ProcessInfo struct {
	Pid         int32
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
}

// App represents the simplified application core
type App struct {
	DataFile string
	Interval time.Duration
	Config   Config
	
	// Storage manager
	storage *Manager
}

// NewApp creates a new application instance
func NewApp(dataFile string, interval time.Duration, config Config) *App {
	useStorageMgr := config.Storage.MaxFileSizeMB > 0
	storageMgr := NewManager(dataFile, 100, useStorageMgr, config.Storage)
	
	return &App{
		DataFile: dataFile,
		Interval: interval,
		Config:   config,
		storage:  storageMgr,
	}
}

// Initialize initializes the application
func (a *App) Initialize() error {
	return a.storage.Initialize()
}

// CloseFile closes file handles and cleans up resources
func (a *App) CloseFile() error {
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

// DetectDataFormat detects the format version of a data file
func (a *App) DetectDataFormat(filePath string) (int, error) {
	return a.storage.DetectDataFormat(filePath)
}

// ReadResourceRecords reads resource records from file
func (a *App) ReadResourceRecords(filePath string) ([]ResourceRecord, error) {
	return a.storage.ReadRecords(filePath)
}

// CalculateResourceStats calculates resource statistics for a given time period
func (a *App) CalculateResourceStats(period time.Duration) ([]ResourceStats, error) {
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

// CleanOldData removes old data files
func (a *App) CleanOldData(keepDays int) error {
	return a.storage.CleanOldData(keepDays)
}

// GetTotalRecords returns the total number of records
func (a *App) GetTotalRecords() (int, error) {
	return a.storage.GetTotalRecords()
}

// GetProcessInfo gets detailed process information
func (a *App) GetProcessInfo(p *process.Process) (ProcessInfo, error) {
	info := ProcessInfo{Pid: p.Pid}

	if name, err := p.Name(); err == nil {
		info.Name = name
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

	// Get network statistics (estimated based on connections)
	info.NetSentKB, info.NetRecvKB = a.getNetworkStats(p)

	return info, nil
}

// getNetworkStats estimates network usage based on connection patterns
func (a *App) getNetworkStats(p *process.Process) (float64, float64) {
	// This is a simplified implementation
	// In a real scenario, you might want to track network connections more accurately
	return 0.0, 0.0
}

// GetCurrentResources gets current resource usage for all processes
func (a *App) GetCurrentResources() ([]ResourceRecord, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var records []ResourceRecord
	for _, p := range processes {
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
			Name:        name,
			Timestamp:   time.Now(),
			CPUPercent:  info.CPUPercent,
			MemoryMB:    info.MemoryMB,
			Threads:     info.Threads,
			DiskReadMB:  info.DiskReadMB,
			DiskWriteMB: info.DiskWriteMB,
			NetSentKB:   info.NetSentKB,
			NetRecvKB:   info.NetRecvKB,
			IsActive:    false, // Will be set below
			Command:     info.Cmdline,
			WorkingDir:  info.Cwd,
			Category:    "", // Will be set below
		}

		// Determine if process is active
		activityConfig := GetDefaultActivityConfig()
		record.IsActive = IsActive(record, activityConfig)
		
		// Set application category
		record.Category = IdentifyApplication(name, info.Cmdline, a.Config.UseSmartCategories)

		records = append(records, record)
	}

	return records, nil
}

// isSystemProcess checks if a process is a system process
func (a *App) isSystemProcess(name string) bool {
	name = strings.ToLower(name)
	systemPrefixes := []string{
		"kworker", "ksoftirqd", "migration", "rcu_", "watchdog",
		"khugepaged", "kthreadd", "kswapd", "pool", "cpuhp",
		"irq", "migration", "md", "jbd2", "ext4", "xfs",
		"loop", "sr_", "ata_", "scsi_", "usb", "pci",
		"idle_inject", "systemd", "dbus-daemon", "containerd-shim",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	systemProcesses := map[string]bool{
		"system": true,
		"init":   true,
		"bash":   true,
		"ssh":    true,
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