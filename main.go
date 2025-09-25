package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// Version is set during build
var Version = "0.2.1"

// Config represents application configuration
type Config struct {
	StatisticsGranularity string  // "simple", "detailed", "full"
	ShowCommands         bool    // Whether to show full commands
	ShowWorkingDirs      bool    // Whether to show working directories
	UseSmartCategories   bool    // Whether to use smart application categorization
	MaxCommandLength     int     // Maximum command length to display
	MaxDirLength         int     // Maximum directory length to display
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() Config {
	return Config{
		StatisticsGranularity: "detailed",  // simple, detailed, full
		ShowCommands:         true,
		ShowWorkingDirs:      true,
		UseSmartCategories:   true,
		MaxCommandLength:     100,
		MaxDirLength:         50,
	}
}

// loadConfig loads configuration from file or returns default
func loadConfig(configPath string) (Config, error) {
	config := getDefaultConfig()
	
	expandedPath := os.ExpandEnv(configPath)
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return config, nil // No config file, use defaults
	}
	
	// TODO: Implement YAML config parsing in future versions
	// For now, just use defaults
	return config, nil
}

// App represents the application with dependency injection
// No global variables - following Dave Cheney's principles
type App struct {
	dataFile string
	interval time.Duration
	config   Config
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
	DiskReadMB  float64   // Disk read in MB
	DiskWriteMB float64   // Disk write in MB
	NetSentKB   float64   // Network sent in KB
	NetRecvKB   float64   // Network received in KB
	IsActive    bool      // Process is actively being used
	Command     string    // Full command line
	WorkingDir  string    // Working directory
	Category    string    // Application category for better classification
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
	Name         string
	Samples      int     // Number of samples
	ActiveSamples int     // Number of active samples
	CPUAvg       float64 // Average CPU percentage
	CPUMax       float64 // Maximum CPU percentage
	MemoryAvg    float64 // Average memory in MB
	MemoryMax    float64 // Maximum memory in MB
	ThreadsAvg   float64 // Average thread count
	DiskReadAvg  float64 // Average disk read in MB
	DiskWriteAvg float64 // Average disk write in MB
	NetSentAvg   float64 // Average network sent in KB
	NetRecvAvg   float64 // Average network received in KB
	ActiveTime   time.Duration // Total active time based on interval
	// New fields for v0.2.1
	Category     string  // Application category
	Command      string  // Command line (first sample)
	WorkingDir   string  // Working directory (first sample)
}

// NewApp creates a new App instance with dependency injection
// Explicit dependencies, no hidden state
func NewApp(dataFile string, interval time.Duration, config Config) *App {
	return &App{
		dataFile: dataFile,
		interval: interval,
		config:   config,
	}
}

// ActivityConfig defines thresholds for active detection
type ActivityConfig struct {
	CPUThreshold    float64 // Minimum CPU percentage to be considered active
	MemoryThreshold float64 // Minimum memory change in MB
	DiskThreshold   float64 // Minimum disk I/O in MB
	NetThreshold    float64 // Minimum network activity in KB
}

// Data format versions
const (
	DataFormatV1 = 1 // v0.1.2 format: 9 fields (timestamp,name,cpu,memory,threads,diskRead,diskWrite,netSent,netRecv)
	DataFormatV2 = 2 // v0.1.3+ format: 10 fields (adds isActive)
	DataFormatV3 = 3 // v0.2.1+ format: 13 fields (adds command, workingDir, category)
)

// Default activity thresholds
func (a *App) getDefaultActivityConfig() ActivityConfig {
	return ActivityConfig{
		CPUThreshold:    1.0,    // 1% CPU usage
		MemoryThreshold: 0.5,    // 0.5MB memory change
		DiskThreshold:   0.1,    // 0.1MB disk I/O
		NetThreshold:    1.0,    // 1KB network activity
	}
}

// isActive determines if a process is actively being used
func (a *App) isActive(resource ResourceRecord, config ActivityConfig) bool {
	// Check CPU usage
	if resource.CPUPercent >= config.CPUThreshold {
		return true
	}
	
	// Check disk I/O activity
	if resource.DiskReadMB >= config.DiskThreshold || resource.DiskWriteMB >= config.DiskThreshold {
		return true
	}
	
	// Check network activity
	if resource.NetSentKB >= config.NetThreshold || resource.NetRecvKB >= config.NetThreshold {
		return true
	}
	
	// Note: Memory threshold comparison would require tracking previous values
	// For now, we consider significant memory usage as active
	if resource.MemoryMB >= 50.0 { // Processes using >50MB memory
		return true
	}
	
	return false
}

// identifyApplication intelligently identifies the application based on name and command
func (a *App) identifyApplication(name, cmd string) string {
	if !a.config.UseSmartCategories {
		return name
	}
	
	cmdLower := strings.ToLower(cmd)
	nameLower := strings.ToLower(name)
	
	// Java applications
	if nameLower == "java" {
		switch {
		case strings.Contains(cmdLower, "tika-server"):
			return "Tika Server"
		case strings.Contains(cmdLower, "spring-boot"):
			return "Spring Boot Application"
		case strings.Contains(cmdLower, "tomcat"):
			return "Tomcat Server"
		case strings.Contains(cmdLower, "jetty"):
			return "Jetty Server"
		default:
			return "Java Application"
		}
	}
	
	// Node.js applications
	if nameLower == "node" || strings.Contains(cmdLower, "npm ") {
		switch {
		case strings.Contains(cmdLower, "@modelcontextprotocol"):
			return "MCP Sequential Thinking"
		case strings.Contains(cmdLower, "playwright-mcp"):
			return "Playwright MCP Server"
		case strings.Contains(cmdLower, "context7-mcp"):
			return "Context7 MCP Server"
		case strings.Contains(cmdLower, "npm exec"):
			return extractNpmPackageName(cmd)
		default:
			return "Node.js Application"
		}
	}
	
	// Python applications
	if nameLower == "python" || nameLower == "python3" {
		switch {
		case strings.Contains(cmdLower, "uv run"):
			return extractPythonProjectName(cmd)
		case strings.Contains(cmdLower, "app.py"):
			return "Python Application (app.py)"
		case strings.Contains(cmdLower, "main.py"):
			return "Python Application (main.py)"
		case strings.Contains(cmdLower, "manage.py"):
			return "Django Application"
		case strings.Contains(cmdLower, "flask"):
			return "Flask Application"
		default:
			return "Python Application"
		}
	}
	
	// Web browsers
	switch nameLower {
	case "chrome", "chromium", "google-chrome":
		return "Chrome Browser"
	case "firefox":
		return "Firefox Browser"
	case "safari":
		return "Safari Browser"
	}
	
	// Development tools
	switch nameLower {
	case "code", "code-insiders":
		return "Visual Studio Code"
	case "vim", "nvim", "vim.basic":
		return "Vim Editor"
	case "emacs":
		return "Emacs Editor"
	case "git":
		return "Git Version Control"
	}
	
	// Default to process name
	return name
}

// extractNpmPackageName extracts the package name from npm exec command
func extractNpmPackageName(cmd string) string {
	parts := strings.Fields(cmd)
	for i, part := range parts {
		if part == "exec" && i+1 < len(parts) {
			pkg := parts[i+1]
			// Remove @scope/ prefix for display
			if strings.HasPrefix(pkg, "@") && strings.Contains(pkg, "/") {
				slashIdx := strings.Index(pkg, "/")
				if slashIdx > 0 {
					return pkg[slashIdx+1:]
				}
			}
			return pkg
		}
	}
	return "npm exec"
}

// extractPythonProjectName extracts project name from uv run command
func extractPythonProjectName(cmd string) string {
	// Extract directory or script name from uv run command
	if strings.Contains(cmd, "uv run") {
		parts := strings.Fields(cmd)
		for i, part := range parts {
			if part == "run" && i+1 < len(parts) {
				script := parts[i+1]
				// Remove path and extension
				if strings.Contains(script, "/") {
					slashIdx := strings.LastIndex(script, "/")
					script = script[slashIdx+1:]
				}
				if strings.HasSuffix(script, ".py") {
					script = script[:len(script)-3]
				}
				return fmt.Sprintf("Python (%s)", script)
			}
		}
	}
	return "Python Application"
}

// extractProjectName extracts project name from working directory
func (a *App) extractProjectName(cwd string) string {
	if cwd == "" {
		return ""
	}
	
	parts := strings.Split(cwd, "/")
	if len(parts) > 0 {
		projectName := parts[len(parts)-1]
		// Skip common directory names
		switch projectName {
		case "src", "bin", "lib", "tmp", "temp", "var", "etc", "usr", "opt":
			if len(parts) > 1 {
				return parts[len(parts)-2]
			}
		}
		return projectName
	}
	return ""
}

// truncateString truncates string to maximum length
func (a *App) truncateString(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// detectDataFormat detects the data format version from a resource file
func (a *App) detectDataFormat(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return DataFormatV2, nil // No file yet, use latest format
		}
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) == 9 {
			return DataFormatV1, nil // v0.1.2 format
		} else if len(fields) == 10 {
			return DataFormatV2, nil // v0.1.3+ format
		}
		// Continue checking other lines - mixed format files
	}

	// If we get here, check if we found any valid format
	// Default to latest format if empty or only invalid lines
	return DataFormatV2, nil
}

// parseResourceLineV1 parses v0.1.2 format (9 fields)
func (a *App) parseResourceLineV1(line string) (ResourceRecord, error) {
	fields := strings.Split(line, ",")
	if len(fields) != 9 {
		return ResourceRecord{}, fmt.Errorf("invalid v0.1.2 format: expected 9 fields, got %d", len(fields))
	}

	timestamp, err := time.Parse(time.RFC3339, fields[0])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid timestamp: %w", err)
	}

	cpuPercent, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid CPU percent: %w", err)
	}

	memoryMB, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid memory MB: %w", err)
	}

	threads, err := strconv.ParseInt(fields[4], 10, 32)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid threads: %w", err)
	}

	diskReadMB, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk read MB: %w", err)
	}

	diskWriteMB, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk write MB: %w", err)
	}

	netSentKB, err := strconv.ParseFloat(fields[7], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net sent KB: %w", err)
	}

	netRecvKB, err := strconv.ParseFloat(fields[8], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net recv KB: %w", err)
	}

	// For v0.1.2 format, determine active status
	resource := ResourceRecord{
		Name:        fields[1],
		Timestamp:   timestamp,
		CPUPercent:  cpuPercent,
		MemoryMB:    memoryMB,
		Threads:     int32(threads),
		DiskReadMB:  diskReadMB,
		DiskWriteMB: diskWriteMB,
		NetSentKB:   netSentKB,
		NetRecvKB:   netRecvKB,
		IsActive:    false, // Will be set below
	}

	// Determine active status for old data
	config := a.getDefaultActivityConfig()
	resource.IsActive = a.isActive(resource, config)

	return resource, nil
}

// parseResourceLineV2 parses v0.1.3+ format (10 fields)
func (a *App) parseResourceLineV2(line string) (ResourceRecord, error) {
	fields := strings.Split(line, ",")
	if len(fields) != 10 {
		return ResourceRecord{}, fmt.Errorf("invalid v0.1.3+ format: expected 10 fields, got %d", len(fields))
	}

	timestamp, err := time.Parse(time.RFC3339, fields[0])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid timestamp: %w", err)
	}

	cpuPercent, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid CPU percent: %w", err)
	}

	memoryMB, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid memory MB: %w", err)
	}

	threads, err := strconv.ParseInt(fields[4], 10, 32)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid threads: %w", err)
	}

	diskReadMB, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk read MB: %w", err)
	}

	diskWriteMB, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk write MB: %w", err)
	}

	netSentKB, err := strconv.ParseFloat(fields[7], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net sent KB: %w", err)
	}

	netRecvKB, err := strconv.ParseFloat(fields[8], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net recv KB: %w", err)
	}

	isActive, err := strconv.ParseBool(fields[9])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid active status: %w", err)
	}

	return ResourceRecord{
		Name:        fields[1],
		Timestamp:   timestamp,
		CPUPercent:  cpuPercent,
		MemoryMB:    memoryMB,
		Threads:     int32(threads),
		DiskReadMB:  diskReadMB,
		DiskWriteMB: diskWriteMB,
		NetSentKB:   netSentKB,
		NetRecvKB:   netRecvKB,
		IsActive:    isActive,
	}, nil
}

// parseResourceLineV3 parses v0.2.1+ format (13 fields)
func (a *App) parseResourceLineV3(line string) (ResourceRecord, error) {
	fields := strings.Split(line, ",")
	if len(fields) != 13 {
		return ResourceRecord{}, fmt.Errorf("invalid v0.2.1 format: expected 13 fields, got %d", len(fields))
	}

	timestamp, err := time.Parse(time.RFC3339, fields[0])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid timestamp: %w", err)
	}

	cpuPercent, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid CPU percent: %w", err)
	}

	memoryMB, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid memory MB: %w", err)
	}

	threads, err := strconv.ParseInt(fields[4], 10, 32)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid threads: %w", err)
	}

	diskReadMB, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk read MB: %w", err)
	}

	diskWriteMB, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk write MB: %w", err)
	}

	netSentKB, err := strconv.ParseFloat(fields[7], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net sent KB: %w", err)
	}

	netRecvKB, err := strconv.ParseFloat(fields[8], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net recv KB: %w", err)
	}

	isActive, err := strconv.ParseBool(fields[9])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid active status: %w", err)
	}

	// V3 format includes command, working directory, and category
	command := fields[10]
	if command == "" {
		command = fields[1] // fallback to process name
	}
	workingDir := fields[11]
	category := fields[12]
	if category == "" {
		category = a.identifyApplication(fields[1], command)
	}

	return ResourceRecord{
		Name:        fields[1],
		Timestamp:   timestamp,
		CPUPercent:  cpuPercent,
		MemoryMB:    memoryMB,
		Threads:     int32(threads),
		DiskReadMB:  diskReadMB,
		DiskWriteMB: diskWriteMB,
		NetSentKB:   netSentKB,
		NetRecvKB:   netRecvKB,
		IsActive:    isActive,
		Command:     command,
		WorkingDir:  workingDir,
		Category:    category,
	}, nil
}

// readResourceRecords reads resource records with automatic format detection
func (a *App) readResourceRecords(filePath string) ([]ResourceRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ResourceRecord{}, nil // No data file yet
		}
		return nil, err
	}
	defer file.Close()

	var records []ResourceRecord
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record ResourceRecord
		var parseErr error

		// Try to parse based on field count (handle mixed format files)
		fields := strings.Split(line, ",")
		if len(fields) == 9 {
			record, parseErr = a.parseResourceLineV1(line)
		} else if len(fields) == 10 {
			record, parseErr = a.parseResourceLineV2(line)
		} else if len(fields) == 13 {
			record, parseErr = a.parseResourceLineV3(line)
		} else {
			log.Printf("Warning: line %d has invalid field count: %d", lineNumber, len(fields))
			continue
		}

		if parseErr != nil {
			log.Printf("Warning: failed to parse line %d: %v", lineNumber, parseErr)
			continue
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return records, nil
}

// calculateResourceStats calculates detailed resource statistics for a period
func (a *App) calculateResourceStats(period time.Duration) ([]ResourceStats, error) {
	expandedPath := os.ExpandEnv(a.dataFile + ".resources")
	
	// Read resource records with automatic format detection
	records, err := a.readResourceRecords(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ResourceStats{}, nil // No data file yet
		}
		return nil, err
	}

	// Calculate cutoff time
	cutoff := time.Now().Add(-period)

	// Group records by process name
	processRecords := make(map[string][]ResourceRecord)
	for _, record := range records {
		if record.Timestamp.After(cutoff) {
			processRecords[record.Name] = append(processRecords[record.Name], record)
		}
	}

	// Calculate statistics for each process
	var stats []ResourceStats
	for name, records := range processRecords {
		if len(records) == 0 {
			continue
		}

		var stat ResourceStats
		stat.Name = name
		stat.Samples = len(records)
		
		var cpuTotal, memoryTotal, threadsTotal, diskReadTotal, diskWriteTotal, netSentTotal, netRecvTotal float64
		var cpuMax, memoryMax float64
		var activeSamples int

		for _, record := range records {
			cpuTotal += record.CPUPercent
			memoryTotal += record.MemoryMB
			threadsTotal += float64(record.Threads)
			diskReadTotal += record.DiskReadMB
			diskWriteTotal += record.DiskWriteMB
			netSentTotal += record.NetSentKB
			netRecvTotal += record.NetRecvKB

			if record.CPUPercent > cpuMax {
				cpuMax = record.CPUPercent
			}
			if record.MemoryMB > memoryMax {
				memoryMax = record.MemoryMB
			}
			if record.IsActive {
				activeSamples++
			}
		}

		stat.ActiveSamples = activeSamples
		stat.CPUAvg = cpuTotal / float64(len(records))
		stat.CPUMax = cpuMax
		stat.MemoryAvg = memoryTotal / float64(len(records))
		stat.MemoryMax = memoryMax
		stat.ThreadsAvg = threadsTotal / float64(len(records))
		stat.DiskReadAvg = diskReadTotal / float64(len(records))
		stat.DiskWriteAvg = diskWriteTotal / float64(len(records))
		stat.NetSentAvg = netSentTotal / float64(len(records))
		stat.NetRecvAvg = netRecvTotal / float64(len(records))
		stat.ActiveTime = time.Duration(activeSamples) * a.interval

		stats = append(stats, stat)
	}

	// Sort by active time (descending)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].ActiveTime > stats[j].ActiveTime
	})

	return stats, nil
}

func main() {
	// Configuration
	const dataFile = "$HOME/.process-tracker.log"
	const configPath = "$HOME/.process-tracker.yaml"
	const interval = 5 * time.Second

	// Load configuration
	config, err := loadConfig(configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
		config = getDefaultConfig()
	}

	// Create app with dependency injection
	app := NewApp(dataFile, interval, config)

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
	case "month":
		app.showMonthStats()
	case "details":
		app.showDetailedStats()
	case "cleanup":
		app.cleanupOldData()
	case "help":
		app.printUsage()
	default:
		app.printUsage()
	}
}

func (a *App) printUsage() {
	fmt.Println("进程跟踪器 - 智能进程监控工具 v0.2.1")
	fmt.Println()
	fmt.Println("使用方法:")
	fmt.Println("  process-tracker <命令>")
	fmt.Println()
	fmt.Println("命令:")
	fmt.Println("  start    开始监控进程")
	fmt.Println("  today    显示今日使用统计")
	fmt.Println("  week     显示本周使用统计")
	fmt.Println("  month    显示本月使用统计")
	fmt.Println("  details  显示详细资源使用统计")
	fmt.Println("  cleanup  清理30天前的旧数据")
	fmt.Println("  version  显示版本信息")
	fmt.Println("  help     显示此帮助信息")
	fmt.Println()
	fmt.Println("配置文件:")
	fmt.Println("  ~/.process-tracker.yaml - 控制统计粒度和显示选项")
	fmt.Println("    statistics_granularity: simple|detailed|full")
	fmt.Println("    show_commands: true|false")
	fmt.Println("    show_working_dirs: true|false")
	fmt.Println("    use_smart_categories: true|false")
	fmt.Println()
	fmt.Println("v0.2.1 新特性:")
	fmt.Println("  ✨ 智能应用识别 (Tika Server, Spring Boot等)")
	fmt.Println("  ✨ 基于类别的统计分组")
	fmt.Println("  ✨ 命令行和工作目录跟踪")
	fmt.Println("  ✨ 可配置的统计粒度")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  process-tracker start")
	fmt.Println("  process-tracker today")
	fmt.Println("  process-tracker month")
	fmt.Println("  process-tracker details")
	fmt.Println("  process-tracker cleanup")
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

		// Get I/O statistics
		var diskReadMB, diskWriteMB float64 = 0.0, 0.0
		if ioCounters, err := p.IOCounters(); err == nil {
			diskReadMB = float64(ioCounters.ReadBytes) / 1024 / 1024
			diskWriteMB = float64(ioCounters.WriteBytes) / 1024 / 1024
		}

		// Get network statistics (if available)
		var netSentKB, netRecvKB float64 = 0.0, 0.0
		if connections, err := p.Connections(); err == nil {
			// Count network activity by connections
			for _, conn := range connections {
				if conn.Status == "ESTABLISHED" {
					// This is a simplified approach - real network stats would require more complex analysis
					netSentKB += 1.0  // Placeholder for actual network metrics
					netRecvKB += 1.0
				}
			}
		}

		// Get command line and working directory
		var command, workingDir string
		if cmd, err := p.Cmdline(); err == nil {
			command = a.truncateString(cmd, a.config.MaxCommandLength)
		}
		if cwd, err := p.Cwd(); err == nil {
			workingDir = a.truncateString(cwd, a.config.MaxDirLength)
		}

		// Create resource record
		resource := ResourceRecord{
			Name:        name,
			Timestamp:   time.Now(),
			CPUPercent:  cpuPercent,
			MemoryMB:    memoryMB,
			Threads:     threads,
			DiskReadMB:  diskReadMB,
			DiskWriteMB: diskWriteMB,
			NetSentKB:   netSentKB,
			NetRecvKB:   netRecvKB,
			IsActive:    false, // Will be set below,
			Command:     command,
			WorkingDir:  workingDir,
			Category:    "", // Will be set below
		}

		// Determine if process is active
		config := a.getDefaultActivityConfig()
		resource.IsActive = a.isActive(resource, config)
		
		// Set application category
		resource.Category = a.identifyApplication(name, command)

		records = append(records, resource)
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

	// Write each resource record in CSV format (V3 - 13 fields)
	for _, resource := range resources {
		line := fmt.Sprintf("%s,%s,%.2f,%.2f,%d,%.2f,%.2f,%.2f,%.2f,%t,%s,%s,%s\n",
			resource.Timestamp.Format(time.RFC3339),
			resource.Name,
			resource.CPUPercent,
			resource.MemoryMB,
			resource.Threads,
			resource.DiskReadMB,
			resource.DiskWriteMB,
			resource.NetSentKB,
			resource.NetRecvKB,
			resource.IsActive,
			escapeCSVField(resource.Command),
			escapeCSVField(resource.WorkingDir),
			escapeCSVField(resource.Category))
		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

// escapeCSVField escapes special characters in CSV fields
func escapeCSVField(field string) string {
	if strings.ContainsAny(field, ",\"\n") {
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(field, "\"", "\"\""))
	}
	return field
}

func (a *App) showTodayStats() {
	if a.config.StatisticsGranularity == "simple" {
		stats, err := a.getStatsForPeriod(24 * time.Hour)
		if err != nil {
			log.Fatalf("获取今日统计数据失败: %v", err)
		}
		a.displayStats("今日使用统计", stats)
	} else {
		stats, err := a.getResourceStatsForPeriod(24 * time.Hour)
		if err != nil {
			log.Fatalf("获取今日统计数据失败: %v", err)
		}
		a.displayEnhancedStats("今日使用统计", stats, 24 * time.Hour)
	}
}

func (a *App) showWeekStats() {
	if a.config.StatisticsGranularity == "simple" {
		stats, err := a.getStatsForPeriod(7 * 24 * time.Hour)
		if err != nil {
			log.Fatalf("获取本周统计数据失败: %v", err)
		}
		a.displayStats("本周使用统计", stats)
	} else {
		stats, err := a.getResourceStatsForPeriod(7 * 24 * time.Hour)
		if err != nil {
			log.Fatalf("获取本周统计数据失败: %v", err)
		}
		a.displayEnhancedStats("本周使用统计", stats, 7 * 24 * time.Hour)
	}
}

func (a *App) showMonthStats() {
	if a.config.StatisticsGranularity == "simple" {
		stats, err := a.getStatsForPeriod(30 * 24 * time.Hour)
		if err != nil {
			log.Fatalf("获取本月统计数据失败: %v", err)
		}
		a.displayStats("本月使用统计", stats)
	} else {
		stats, err := a.getResourceStatsForPeriod(30 * 24 * time.Hour)
		if err != nil {
			log.Fatalf("获取本月统计数据失败: %v", err)
		}
		a.displayEnhancedStats("本月使用统计", stats, 30 * 24 * time.Hour)
	}
}

func (a *App) cleanupOldData() {
	fmt.Println("清理30天前的旧数据...")
	
	expandedPath := os.ExpandEnv(a.dataFile + ".resources")
	
	// Read existing records
	records, err := a.readResourceRecords(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("没有找到数据文件。")
			return
		}
		log.Fatalf("读取数据文件失败: %v", err)
	}

	if len(records) == 0 {
		fmt.Println("没有数据需要清理。")
		return
	}

	// Calculate cutoff time (30 days ago)
	cutoff := time.Now().Add(-30 * 24 * time.Hour)

	// Filter records to keep
	var newRecords []ResourceRecord
	removedCount := 0
	for _, record := range records {
		if record.Timestamp.After(cutoff) {
			newRecords = append(newRecords, record)
		} else {
			removedCount++
		}
	}

	fmt.Printf("原始记录数: %d\n", len(records))
	fmt.Printf("保留记录数: %d\n", len(newRecords))
	fmt.Printf("删除记录数: %d\n", removedCount)

	if removedCount == 0 {
		fmt.Println("没有30天前的数据需要清理。")
		return
	}

	// Write filtered records back to file
	if err := a.writeResourceRecords(expandedPath, newRecords); err != nil {
		log.Fatalf("写入清理后的数据失败: %v", err)
	}

	fmt.Println("数据清理完成。")
}

// writeResourceRecords writes resource records to file
func (a *App) writeResourceRecords(filePath string, records []ResourceRecord) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	// Create backup first
	backupPath := filePath + ".backup"
	if err := copyFile(filePath, backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Open file for writing (truncate)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write each resource record in CSV format
	for _, resource := range records {
		line := fmt.Sprintf("%s,%s,%.2f,%.2f,%d,%.2f,%.2f,%.2f,%.2f,%t\n",
			resource.Timestamp.Format(time.RFC3339),
			resource.Name,
			resource.CPUPercent,
			resource.MemoryMB,
			resource.Threads,
			resource.DiskReadMB,
			resource.DiskWriteMB,
			resource.NetSentKB,
			resource.NetRecvKB,
			resource.IsActive)
		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func (a *App) getStatsForPeriod(period time.Duration) ([]ProcessStats, error) {
	// For backward compatibility, use the old simple counting method
	expandedPath := os.ExpandEnv(a.dataFile + ".resources")
	
	// Read resource records with automatic format detection
	records, err := a.readResourceRecords(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ProcessStats{}, nil // No data file yet
		}
		return nil, err
	}

	// Calculate cutoff time
	cutoff := time.Now().Add(-period)

	// Count active processes within the period
	processCount := make(map[string]int)
	for _, record := range records {
		if record.Timestamp.After(cutoff) && record.IsActive {
			processCount[record.Name]++
		}
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

// getResourceStatsForPeriod gets detailed resource statistics for a period
func (a *App) getResourceStatsForPeriod(period time.Duration) ([]ResourceStats, error) {
	expandedPath := os.ExpandEnv(a.dataFile + ".resources")
	
	// Read resource records with automatic format detection
	records, err := a.readResourceRecords(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ResourceStats{}, nil // No data file yet
		}
		return nil, err
	}

	// Calculate cutoff time
	cutoff := time.Now().Add(-period)

	// Group records by aggregation key based on configuration
	var processRecords map[string][]ResourceRecord
	
	switch a.config.StatisticsGranularity {
	case "simple":
		processRecords = make(map[string][]ResourceRecord)
		for _, record := range records {
			if record.Timestamp.After(cutoff) {
				key := record.Name
				processRecords[key] = append(processRecords[key], record)
			}
		}
	case "detailed":
		processRecords = make(map[string][]ResourceRecord)
		for _, record := range records {
			if record.Timestamp.After(cutoff) {
				key := record.Category
				if key == "" {
					key = record.Name
				}
				processRecords[key] = append(processRecords[key], record)
			}
		}
	case "full":
		processRecords = make(map[string][]ResourceRecord)
		for _, record := range records {
			if record.Timestamp.After(cutoff) {
				// Use category + working directory as key for full granularity
				key := record.Category
				if record.WorkingDir != "" && a.config.ShowWorkingDirs {
					key += " [" + record.WorkingDir + "]"
				}
				if key == "" {
					key = record.Name
				}
				processRecords[key] = append(processRecords[key], record)
			}
		}
	default:
		processRecords = make(map[string][]ResourceRecord)
		for _, record := range records {
			if record.Timestamp.After(cutoff) {
				key := record.Name
				processRecords[key] = append(processRecords[key], record)
			}
		}
	}

	// Calculate statistics for each process
	var stats []ResourceStats
	for key, records := range processRecords {
		if len(records) == 0 {
			continue
		}

		var stat ResourceStats
		stat.Name = key
		stat.Samples = len(records)
		
		var cpuTotal, memoryTotal, threadsTotal, diskReadTotal, diskWriteTotal, netSentTotal, netRecvTotal float64
		var cpuMax, memoryMax float64
		var activeSamples int
		var firstCommand, firstWorkingDir, firstCategory string

		for _, record := range records {
			cpuTotal += record.CPUPercent
			memoryTotal += record.MemoryMB
			threadsTotal += float64(record.Threads)
			diskReadTotal += record.DiskReadMB
			diskWriteTotal += record.DiskWriteMB
			netSentTotal += record.NetSentKB
			netRecvTotal += record.NetRecvKB

			if record.CPUPercent > cpuMax {
				cpuMax = record.CPUPercent
			}
			if record.MemoryMB > memoryMax {
				memoryMax = record.MemoryMB
			}
			if record.IsActive {
				activeSamples++
			}
			
			// Capture first sample's additional info
			if firstCommand == "" && record.Command != "" {
				firstCommand = record.Command
			}
			if firstWorkingDir == "" && record.WorkingDir != "" {
				firstWorkingDir = record.WorkingDir
			}
			if firstCategory == "" && record.Category != "" {
				firstCategory = record.Category
			}
		}

		stat.ActiveSamples = activeSamples
		stat.CPUAvg = cpuTotal / float64(len(records))
		stat.CPUMax = cpuMax
		stat.MemoryAvg = memoryTotal / float64(len(records))
		stat.MemoryMax = memoryMax
		stat.ThreadsAvg = threadsTotal / float64(len(records))
		stat.DiskReadAvg = diskReadTotal / float64(len(records))
		stat.DiskWriteAvg = diskWriteTotal / float64(len(records))
		stat.NetSentAvg = netSentTotal / float64(len(records))
		stat.NetRecvAvg = netRecvTotal / float64(len(records))
		stat.ActiveTime = time.Duration(activeSamples) * 5 * time.Second // 5 second intervals
		
		// Set additional fields
		stat.Category = firstCategory
		stat.Command = firstCommand
		stat.WorkingDir = firstWorkingDir

		stats = append(stats, stat)
	}

	// Sort by active time (descending)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].ActiveTime > stats[j].ActiveTime
	})

	return stats, nil
}

func (a *App) displayStats(title string, stats []ProcessStats) {
	fmt.Printf("=== %s ===\n", title)
	fmt.Printf("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("统计粒度: %s\n", a.config.StatisticsGranularity)
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
	totalActiveTime := time.Duration(0)
	
	// 根据配置显示不同级别的信息
	if a.config.StatisticsGranularity == "simple" {
		fmt.Printf("%-30s %8s %15s\n", "进程名", "次数", "活跃时间")
		fmt.Printf("%s\n", strings.Repeat("-", 55))
		for i := 0; i < maxDisplay; i++ {
			stat := stats[i]
			duration := time.Duration(stat.Count) * a.interval
			fmt.Printf("%-30s %8d %15s\n", stat.Name, stat.Count, formatDuration(duration))
			totalCount += stat.Count
		}
	} else {
		// 使用详细的资源统计
		detailedStats, err := a.getResourceStatsForPeriod(24 * time.Hour)
		if err == nil && len(detailedStats) > 0 {
			fmt.Printf("%-40s %8s %10s %12s\n", "应用/进程", "样本数", "活跃度", "活跃时间")
			fmt.Printf("%s\n", strings.Repeat("-", 75))
			for i := 0; i < maxDisplay && i < len(detailedStats); i++ {
				stat := detailedStats[i]
				activeRate := float64(stat.ActiveSamples) / float64(stat.Samples) * 100
				fmt.Printf("%-40s %8d %9.1f%% %12s\n", 
					stat.Name, 
					stat.Samples, 
					activeRate, 
					formatDuration(stat.ActiveTime))
				totalCount += stat.Samples
				totalActiveTime += stat.ActiveTime
			}
		} else {
			// 回退到简单统计
			for i := 0; i < maxDisplay; i++ {
				stat := stats[i]
				duration := time.Duration(stat.Count) * a.interval
				fmt.Printf("%-30s %8d %15s\n", stat.Name, stat.Count, formatDuration(duration))
				totalCount += stat.Count
			}
		}
	}
	fmt.Println()
	if totalActiveTime > 0 {
		fmt.Printf("总活跃时间: %s\n", formatDuration(totalActiveTime))
	} else {
		fmt.Printf("总监控间隔数: %d\n", totalCount)
		fmt.Printf("估算总时间: %s\n", formatDuration(time.Duration(totalCount)*a.interval))
	}
}

func (a *App) showDetailedStats() {
	stats, err := a.getResourceStatsForPeriod(24 * time.Hour)
	if err != nil {
		log.Fatalf("获取详细统计数据失败: %v", err)
	}

	a.displayDetailedStats("今日详细资源使用统计", stats)
}

func (a *App) displayDetailedStats(title string, stats []ResourceStats) {
	fmt.Printf("=== %s ===\n", title)
	fmt.Printf("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("统计粒度: %s\n", a.config.StatisticsGranularity)
	fmt.Println()

	if len(stats) == 0 {
		fmt.Println("未找到使用数据。")
		fmt.Println("请确保监控正在运行: 'process-tracker start'")
		return
	}

	// 显示前15个进程（详细信息需要更多空间）
	maxDisplay := 15
	if len(stats) < maxDisplay {
		maxDisplay = len(stats)
	}

	// 根据配置显示不同级别的详细信息
	if a.config.StatisticsGranularity == "full" && a.config.ShowCommands {
		fmt.Printf("%-35s %8s %8s %8s %8s %8s %10s %10s\n", 
			"应用/进程", "活跃度", "样本数", "CPU平均", "内存平均", "磁盘读", "磁盘写", "活跃时间")
		fmt.Printf("%s\n", strings.Repeat("-", 110))
		
		totalActiveTime := time.Duration(0)
		for i := 0; i < maxDisplay; i++ {
			stat := stats[i]
			activeRate := float64(stat.ActiveSamples) / float64(stat.Samples) * 100
			
			fmt.Printf("%-35s %7.1f%% %8d %7.1f%% %7.1fMB %8.2fMB %8.2fMB %10s\n",
				stat.Name,
				activeRate,
				stat.Samples,
				stat.CPUAvg,
				stat.MemoryAvg,
				stat.DiskReadAvg,
				stat.DiskWriteAvg,
				formatDuration(stat.ActiveTime))
			
			totalActiveTime += stat.ActiveTime
			
			// 显示额外信息（如果有）
			if a.config.ShowCommands && stat.Command != "" {
				fmt.Printf("  命令: %s\n", a.truncateString(stat.Command, 80))
			}
			if a.config.ShowWorkingDirs && stat.WorkingDir != "" {
				fmt.Printf("  目录: %s\n", a.truncateString(stat.WorkingDir, 60))
			}
			if stat.Category != "" && stat.Category != stat.Name {
				fmt.Printf("  类别: %s\n", stat.Category)
			}
			fmt.Println()
		}
		fmt.Printf("%s\n", strings.Repeat("-", 110))
		fmt.Printf("总活跃时间: %s\n", formatDuration(totalActiveTime))
	} else {
		// 标准详细显示
		fmt.Printf("%-30s %8s %8s %8s %8s %8s %8s %10s %10s %10s\n", 
			"进程名", "活跃度", "样本数", "CPU平均", "CPU峰值", "内存平均", "内存峰值", "磁盘读", "磁盘写", "活跃时间")
		fmt.Printf("%s\n", strings.Repeat("-", 130))

		totalActiveTime := time.Duration(0)
		for i := 0; i < maxDisplay; i++ {
			stat := stats[i]
			activeRate := float64(stat.ActiveSamples) / float64(stat.Samples) * 100
			
			fmt.Printf("%-30s %7.1f%% %8d %7.1f%% %7.1f%% %7.1fMB %7.1fMB %8.2fMB %8.2fMB %10s\n",
				stat.Name,
				activeRate,
				stat.Samples,
				stat.CPUAvg,
				stat.CPUMax,
				stat.MemoryAvg,
				stat.MemoryMax,
				stat.DiskReadAvg,
				stat.DiskWriteAvg,
				formatDuration(stat.ActiveTime))
			
			totalActiveTime += stat.ActiveTime
		}

		fmt.Printf("%s\n", strings.Repeat("-", 130))
		fmt.Printf("总活跃时间: %s\n", formatDuration(totalActiveTime))
	}
}

func (a *App) displayEnhancedStats(title string, stats []ResourceStats, period time.Duration) {
	fmt.Printf("=== %s ===\n", title)
	fmt.Printf("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("统计粒度: %s\n", a.config.StatisticsGranularity)
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

	// 根据配置显示不同级别的信息
	if a.config.StatisticsGranularity == "detailed" {
		fmt.Printf("%-40s %8s %10s %12s %10s %10s\n", 
			"应用/进程", "样本数", "活跃度", "活跃时间", "CPU平均", "内存平均")
		fmt.Printf("%s\n", strings.Repeat("-", 105))
		
		totalActiveTime := time.Duration(0)
		for i := 0; i < maxDisplay; i++ {
			stat := stats[i]
			activeRate := float64(stat.ActiveSamples) / float64(stat.Samples) * 100
			
			fmt.Printf("%-40s %8d %9.1f%% %12s %9.1f%% %9.1fMB\n", 
				stat.Name, 
				stat.Samples, 
				activeRate, 
				formatDuration(stat.ActiveTime),
				stat.CPUAvg,
				stat.MemoryAvg)
			
			totalActiveTime += stat.ActiveTime
		}
		fmt.Printf("%s\n", strings.Repeat("-", 105))
		fmt.Printf("总活跃时间: %s\n", formatDuration(totalActiveTime))
	} else if a.config.StatisticsGranularity == "full" {
		fmt.Printf("%-35s %8s %10s %12s %10s %10s\n", 
			"应用/进程", "样本数", "活跃度", "活跃时间", "CPU平均", "内存平均")
		fmt.Printf("%s\n", strings.Repeat("-", 95))
		
		totalActiveTime := time.Duration(0)
		for i := 0; i < maxDisplay; i++ {
			stat := stats[i]
			activeRate := float64(stat.ActiveSamples) / float64(stat.Samples) * 100
			
			fmt.Printf("%-35s %8d %9.1f%% %12s %9.1f%% %9.1fMB\n", 
				stat.Name, 
				stat.Samples, 
				activeRate, 
				formatDuration(stat.ActiveTime),
				stat.CPUAvg,
				stat.MemoryAvg)
			
			totalActiveTime += stat.ActiveTime
			
			// 显示额外信息
			if a.config.ShowCommands && stat.Command != "" {
				fmt.Printf("  命令: %s\n", a.truncateString(stat.Command, 60))
			}
			if a.config.ShowWorkingDirs && stat.WorkingDir != "" {
				fmt.Printf("  目录: %s\n", a.truncateString(stat.WorkingDir, 50))
			}
			if stat.Category != "" && stat.Category != stat.Name {
				fmt.Printf("  类别: %s\n", stat.Category)
			}
			fmt.Println()
		}
		fmt.Printf("%s\n", strings.Repeat("-", 95))
		fmt.Printf("总活跃时间: %s\n", formatDuration(totalActiveTime))
	}
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