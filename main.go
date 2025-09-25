package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// Version is set during build
var Version = "0.1.1"

// App represents the application with dependency injection
// No global variables - following Dave Cheney's principles
type App struct {
	dataFile string
	interval time.Duration
}

// ProcessRecord represents a simple process record
// Minimal structure, only essential fields
type ProcessRecord struct {
	Name      string
	Timestamp time.Time
}

// ResourceRecord represents detailed process resource usage
type ResourceRecord struct {
	Name        string
	Timestamp   time.Time
	CPUPercent  float64   // CPU usage percentage
	MemoryMB    float64   // Memory usage in MB
	Threads     int32     // Number of threads
}

// ProcessStats represents accumulated process statistics
type ProcessStats struct {
	Name      string
	Count     int
	CPUTotal  float64 // Total CPU usage percentage
	MemoryAvg float64 // Average memory usage in MB
}

// ResourceStats represents accumulated resource statistics
type ResourceStats struct {
	Name        string
	Samples     int     // Number of samples
	CPUAvg      float64 // Average CPU percentage
	CPUMax      float64 // Maximum CPU percentage
	MemoryAvg   float64 // Average memory in MB
	MemoryMax   float64 // Maximum memory in MB
	ThreadsAvg  float64 // Average thread count
}

// NewApp creates a new App instance with dependency injection
// Explicit dependencies, no hidden state
func NewApp(dataFile string, interval time.Duration) *App {
	return &App{
		dataFile: dataFile,
		interval: interval,
	}
}

func main() {
	// Hardcoded configuration - simple and explicit
	const dataFile = "$HOME/.process-tracker.log"
	const interval = 5 * time.Second

	// Create app with dependency injection
	app := NewApp(dataFile, interval)

	if len(os.Args) < 2 {
		app.printUsage()
		return
	}

	command := os.Args[1]
	switch command {
	case "version":
		fmt.Printf("进程跟踪器版本 %s\n", Version)
	case "start":
		app.startMonitoring()
	case "today":
		app.showTodayStats()
	case "week":
		app.showWeekStats()
	case "help":
		app.printUsage()
	default:
		app.printUsage()
	}
}

func (a *App) printUsage() {
	fmt.Println("进程跟踪器 - 简单的进程监控工具")
	fmt.Println()
	fmt.Println("使用方法:")
	fmt.Println("  process-tracker <命令>")
	fmt.Println()
	fmt.Println("命令:")
	fmt.Println("  start    开始监控进程")
	fmt.Println("  today    显示今日使用统计")
	fmt.Println("  week     显示本周使用统计")
	fmt.Println("  version  显示版本信息")
	fmt.Println("  help     显示此帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  process-tracker start")
	fmt.Println("  process-tracker today")
}

func (a *App) startMonitoring() {
	log.Printf("开始监控进程...")
	log.Printf("数据文件: %s", a.dataFile)
	log.Printf("监控间隔: %v", a.interval)

	// Simple monitoring loop - explicit and clear
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := a.monitorAndSave(); err != nil {
			log.Printf("Error: %v", err)
		}
	}
}

func (a *App) monitorAndSave() error {
	// Get current processes with resource usage
	resources, err := a.getCurrentResources()
	if err != nil {
		return fmt.Errorf("failed to get resources: %w", err)
	}

	// Save to file
	if err := a.saveResources(resources); err != nil {
		return fmt.Errorf("failed to save resources: %w", err)
	}

	return nil
}

func (a *App) getCurrentProcesses() ([]ProcessRecord, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var records []ProcessRecord
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue // Skip processes we can't read
		}

		name = strings.TrimSpace(name)
		if name == "" || p.Pid <= 0 {
			continue // Skip invalid processes
		}

		// Skip obvious system processes
		if a.isSystemProcess(name) {
			continue
		}

		// Normalize process name
		name = a.normalizeProcessName(name)

		records = append(records, ProcessRecord{
			Name:      name,
			Timestamp: time.Now(),
		})
	}

	return records, nil
}

func (a *App) getCurrentResources() ([]ResourceRecord, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var records []ResourceRecord
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue // Skip processes we can't read
		}

		name = strings.TrimSpace(name)
		if name == "" || p.Pid <= 0 {
			continue // Skip invalid processes
		}

		// Skip obvious system processes
		if a.isSystemProcess(name) {
			continue
		}

		// Normalize process name
		name = a.normalizeProcessName(name)

		// Get CPU percentage
		cpuPercent, err := p.Percent(0)
		if err != nil {
			cpuPercent = 0.0
		}

		// Get memory information
		var memoryMB float64 = 0.0
		if memInfo, err := p.MemoryInfo(); err == nil {
			memoryMB = float64(memInfo.RSS) / 1024 / 1024 // Convert to MB
		}

		// Get thread count
		threads, err := p.NumThreads()
		if err != nil {
			threads = 1
		}

		records = append(records, ResourceRecord{
			Name:       name,
			Timestamp:  time.Now(),
			CPUPercent: cpuPercent,
			MemoryMB:   memoryMB,
			Threads:    threads,
		})
	}

	return records, nil
}

func (a *App) isSystemProcess(name string) bool {
	name = strings.ToLower(name)
	systemPrefixes := []string{
		"kworker", "ksoftirqd", "migration", "rcu_", "watchdog",
		"khugepaged", "kthreadd", "kswapd", "pool", "cpuhp",
		"irq", "migration", "md", "jbd2", "ext4", "xfs",
		"loop", "sr_", "ata_", "scsi_", "usb", "pci",
		"idle_inject", "systemd", "dbus-daemon", "containerd-shim",
		"s6-supervise", "docker-proxy", "pipewire", "pulseaudio",
		"gvfsd", "gnome-keyring", "xdg-desktop-portal",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	// Additional system processes to filter by exact match
	systemProcesses := map[string]bool{
		"system": true,
		"init": true,
		"bash": true,  // Usually shell, not user application
		"ssh": true,   // System service
	}

	if systemProcesses[name] {
		return true
	}

	return false
}

func (a *App) normalizeProcessName(name string) string {
	// Truncate long npm command lines
	if strings.HasPrefix(name, "npm exec ") {
		if len(name) > 30 {
			return "npm exec"
		}
	}
	
	// Truncate other long command lines
	if len(name) > 50 {
		// Try to extract the base command
		parts := strings.Fields(name)
		if len(parts) > 0 {
			base := parts[0]
			if strings.Contains(base, "/") {
				// Extract just the executable name
				parts := strings.Split(base, "/")
				base = parts[len(parts)-1]
			}
			return base
		}
		return name[:50] + "..."
	}
	
	return name
}

func (a *App) saveProcesses(processes []ProcessRecord) error {
	// Expand the data file path
	expandedPath := os.ExpandEnv(a.dataFile)
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(expandedPath), 0755); err != nil {
		return err
	}

	// Open file for appending
	file, err := os.OpenFile(expandedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write each process record
	for _, process := range processes {
		line := fmt.Sprintf("%s,%s\n", process.Timestamp.Format(time.RFC3339), process.Name)
		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) saveResources(resources []ResourceRecord) error {
	// Expand the data file path (use .resources extension for new format)
	expandedPath := os.ExpandEnv(a.dataFile)
	resourcePath := expandedPath + ".resources"
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(resourcePath), 0755); err != nil {
		return err
	}

	// Open file for appending
	file, err := os.OpenFile(resourcePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write each resource record in CSV format
	for _, resource := range resources {
		line := fmt.Sprintf("%s,%s,%.2f,%.2f,%d\n",
			resource.Timestamp.Format(time.RFC3339),
			resource.Name,
			resource.CPUPercent,
			resource.MemoryMB,
			resource.Threads)
		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) showTodayStats() {
	stats, err := a.getStatsForPeriod(24 * time.Hour)
	if err != nil {
		log.Fatalf("获取今日统计数据失败: %v", err)
	}

	a.displayStats("今日使用统计", stats)
}

func (a *App) showWeekStats() {
	stats, err := a.getStatsForPeriod(7 * 24 * time.Hour)
	if err != nil {
		log.Fatalf("获取本周统计数据失败: %v", err)
	}

	a.displayStats("本周使用统计", stats)
}

func (a *App) getStatsForPeriod(period time.Duration) ([]ProcessStats, error) {
	expandedPath := os.ExpandEnv(a.dataFile)
	
	// Open file for reading
	file, err := os.Open(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ProcessStats{}, nil // No data file yet
		}
		return nil, err
	}
	defer file.Close()

	// Calculate cutoff time
	cutoff := time.Now().Add(-period)

	// Read and parse file
	scanner := bufio.NewScanner(file)
	processCount := make(map[string]int)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}

		timestampStr := parts[0]
		processName := parts[1]

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			continue
		}

		// Only count records within the period
		if timestamp.After(cutoff) {
			processCount[processName]++
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Convert to slice for sorting
	var stats []ProcessStats
	for name, count := range processCount {
		stats = append(stats, ProcessStats{Name: name, Count: count})
	}

	// Sort by count (descending)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})

	return stats, nil
}

func (a *App) displayStats(title string, stats []ProcessStats) {
	fmt.Printf("=== %s ===\n", title)
	fmt.Printf("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println()

	if len(stats) == 0 {
		fmt.Println("未找到使用数据。")
		fmt.Println("请确保监控正在运行: 'process-tracker start'")
		return
	}

	// 显示前20个进程
	maxDisplay := 20
	if len(stats) < maxDisplay {
		maxDisplay = len(stats)
	}

	totalCount := 0
	for i := 0; i < maxDisplay; i++ {
		stat := stats[i]
		duration := time.Duration(stat.Count) * a.interval
		fmt.Printf("%-30s %6d 次  %s\n", stat.Name, stat.Count, formatDuration(duration))
		totalCount += stat.Count
	}

	fmt.Println()
	fmt.Printf("总监控间隔数: %d\n", totalCount)
	fmt.Printf("估算总时间: %s\n", formatDuration(time.Duration(totalCount)*a.interval))
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", d/time.Second)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", d/time.Minute, (d%time.Minute)/time.Second)
	}
	return fmt.Sprintf("%dh %dm", d/time.Hour, (d%time.Hour)/time.Minute)
}