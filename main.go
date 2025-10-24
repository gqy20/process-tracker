package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/yourusername/process-tracker/api"
	"github.com/yourusername/process-tracker/core"
)

// Version is set during build
var Version = "0.4.1"

// GlobalOptions represents command line global options
type GlobalOptions struct {
	Port        int
	Interval    int
	Format      string
	Filter      string
	Sort        string
	Limit       int
	Offset      int
	Help        bool
	Version     bool
	Quiet       bool
}

// LoadDefaultConfig returns default configuration
func LoadDefaultConfig() core.Config {
	return core.Config{
		EnableSmartCategories: true,
		Storage: core.StorageConfig{
			MaxSizeMB:    100,
			KeepDays:     7,
			Type:         "csv",
			SQLitePath:   "",
			SQLiteWAL:    true,
			SQLiteCacheSize: 2000,
		},
		Web: core.WebConfig{
			Enabled: true,
			Host:    "localhost",
			Port:    "9999",
		},
	}
}

// getMonitoringConfig returns monitoring configuration
func getMonitoringConfig() MonitoringConfig {
	homeDir, _ := os.UserHomeDir()
	return MonitoringConfig{
		Interval: 5,
		DataFile: fmt.Sprintf("%s/.process-tracker/process-tracker.log", homeDir),
	}
}

// MonitoringConfig represents monitoring configuration
type MonitoringConfig struct {
	Interval int
	DataFile string
}

// parseCommandLine parses command line arguments
func parseCommandLine() (command string, options GlobalOptions) {
	args := os.Args[1:]

	if len(args) == 0 {
		options.Help = true
		return
	}

	command = args[0]

	// Parse options
	i := 1
	for i < len(args) {
		switch args[i] {
		case "-p":
			if i+1 < len(args) {
				if port, err := strconv.Atoi(args[i+1]); err == nil {
					options.Port = port
				}
				i++
			}
		case "-i":
			if i+1 < len(args) {
				if interval, err := strconv.Atoi(args[i+1]); err == nil {
					options.Interval = interval
				}
				i++
			}
		case "-f":
			if i+1 < len(args) {
				options.Format = args[i+1]
				i++
			}
		case "--filter":
			if i+1 < len(args) {
				options.Filter = args[i+1]
				i++
			}
		case "--sort":
			if i+1 < len(args) {
				options.Sort = args[i+1]
				i++
			}
		case "--limit":
			if i+1 < len(args) {
				if limit, err := strconv.Atoi(args[i+1]); err == nil {
					options.Limit = limit
				}
				i++
			}
		case "--offset":
			if i+1 < len(args) {
				if offset, err := strconv.Atoi(args[i+1]); err == nil {
					options.Offset = offset
				}
				i++
			}
		case "-h", "--help":
			options.Help = true
		case "-v", "--version":
			options.Version = true
		case "-q", "--quiet":
			options.Quiet = true
		}
		i++
	}

	// Set defaults
	if options.Port == 0 {
		options.Port = 9999
	}
	if options.Interval == 0 {
		options.Interval = 5
	}
	if options.Format == "" {
		options.Format = "table"
	}

	return command, options
}

// printHelp prints help information
func printHelp() {
	fmt.Printf(`Process Tracker v%s - 系统进程监控工具

用法:
  process-tracker <命令> [选项]

命令:
  start    启动进程监控
  stop     停止进程监控
  status   显示监控状态
  stats    显示统计信息
  web      启动Web界面

选项:
  -p <端口>       设置Web服务器端口 (默认: 9999)
  -i <秒数>       设置监控间隔 (默认: 5)
  -f <格式>       输出格式: table, json (默认: table)
  --filter <条件>  过滤条件
  --sort <字段>    排序字段
  --limit <数量>   限制结果数量
  --offset <数量>  偏移结果数量
  -h, --help       显示帮助信息
  -v, --version    显示版本信息
  -q, --quiet      静默模式

示例:
  process-tracker start -i 10          # 启动监控，间隔10秒
  process-tracker web -p 8080           # 启动Web界面，端口8080
  process-tracker stats --format json  # 以JSON格式显示统计
  process-tracker status --filter running # 显示运行中的任务

`, Version)
}

// formatOutput formats output according to format option
func formatOutput(data interface{}, format string) {
	switch format {
	case "json":
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(jsonData))
	default:
		// Default table format
		fmt.Printf("%+v\n", data)
	}
}

// handleStart starts process monitoring
func handleStart(options GlobalOptions) {
	config := LoadDefaultConfig()
	monitoringConfig := getMonitoringConfig()
	if options.Interval > 0 {
		monitoringConfig.Interval = options.Interval
	}

	interval := time.Duration(monitoringConfig.Interval) * time.Second
	app := core.NewApp(monitoringConfig.DataFile, interval, config)

	// Initialize app
	if err := app.Initialize(); err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	// Create daemon manager
	dataDir := filepath.Dir(monitoringConfig.DataFile)
	daemon := core.NewDaemonManager(dataDir)

	// Check if already running
	if running, pid, _ := daemon.IsRunning(); running {
		fmt.Printf("❌ 进程已在运行 (PID: %d)\n", pid)
		fmt.Println("💡 使用 'process-tracker stop' 停止")
		return
	}

	// Write PID file
	if err := daemon.WritePID(); err != nil {
		log.Printf("Warning: Failed to write PID file: %v", err)
	}

	if !options.Quiet {
		fmt.Printf("✅ 监控已启动 (间隔: %ds)\n", monitoringConfig.Interval)
		fmt.Printf("📁 数据文件: %s\n", monitoringConfig.DataFile)
	}

	// Start monitoring loop
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			if err := app.CollectAndSaveData(); err != nil {
				log.Printf("Error collecting data: %v", err)
			}
		case <-sigChan:
			fmt.Println("\n🛑 收到停止信号，正在关闭...")
			daemon.RemovePID()
			return
		}
	}
}

// handleStop stops process monitoring
func handleStop(options GlobalOptions) {
	monitoringConfig := getMonitoringConfig()
	dataDir := filepath.Dir(monitoringConfig.DataFile)
	daemon := core.NewDaemonManager(dataDir)

	// Check if running
	running, pid, err := daemon.IsRunning()
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	if !running {
		fmt.Println("⚠️  进程未运行")
		return
	}

	// Stop the process
	if err := daemon.Stop(); err != nil {
		fmt.Printf("❌ 停止进程失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 已停止监控进程 (PID: %d)\n", pid)
}

// handleStatus shows monitoring status
func handleStatus(options GlobalOptions) {
	monitoringConfig := getMonitoringConfig()
	dataDir := filepath.Dir(monitoringConfig.DataFile)
	daemon := core.NewDaemonManager(dataDir)

	// Check if monitoring is running
	running, pid, err := daemon.IsRunning()
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	if running {
		fmt.Printf("🔄 监控正在运行 (PID: %d)\n", pid)
	} else {
		fmt.Println("⏸️ 监控未运行")
		fmt.Println("💡 使用 'process-tracker start' 启动监控")
	}

	// Show task status
	config := LoadDefaultConfig()
	interval := time.Duration(monitoringConfig.Interval) * time.Second
	app := core.NewApp(monitoringConfig.DataFile, interval, config)

	tasks, err := app.ListTasks(core.StatusPending)
	if err != nil {
		log.Printf("Error getting tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		fmt.Println("📋 暂无任务")
		return
	}

	fmt.Printf("📊 任务状态: %d 个任务\n", len(tasks))
	for _, task := range tasks {
		status := "⏸️ 等待中"
		if task.Status == core.StatusRunning {
			status = "🔄 运行中"
		} else if task.Status == core.StatusCompleted {
			status = "✅ 已完成"
		}
		fmt.Printf("  [%d] %s - %s\n", task.ID, task.Name, status)
	}
}

// handleStats shows statistics
func handleStats(options GlobalOptions) {
	config := LoadDefaultConfig()
	monitoringConfig := getMonitoringConfig()
	interval := time.Duration(monitoringConfig.Interval) * time.Second
	app := core.NewApp(monitoringConfig.DataFile, interval, config)

	// 获取当前系统资源统计
	records, err := app.GetCurrentResources()
	if err != nil {
		log.Printf("Error getting current resources: %v", err)
		records = []core.ResourceRecord{}
	}

	// 计算统计数据
	totalProcesses := len(records)
	activeProcesses := 0
	totalMemory := 0.0
	totalCPU := 0.0

	for _, record := range records {
		if record.IsActive {
			activeProcesses++
		}
		totalMemory += record.MemoryMB
		totalCPU += record.CPUPercent
	}

	// 简化的统计实现
	stats := map[string]interface{}{
		"total_processes":  totalProcesses,
		"active_processes": activeProcesses,
		"memory_usage":     fmt.Sprintf("%.1f MB", totalMemory),
		"cpu_usage":        fmt.Sprintf("%.1f%%", totalCPU),
		"timestamp":        time.Now().Format("2006-01-02 15:04:05"),
	}

	formatOutput(stats, options.Format)
}

// handleWeb starts web interface
func handleWeb(options GlobalOptions) {
	config := LoadDefaultConfig()
	monitoringConfig := getMonitoringConfig()

	// Handle port option
	port := options.Port
	if port == 0 {
		// Convert default port from string to int
		if defaultPort, err := strconv.Atoi(config.Web.Port); err == nil {
			port = defaultPort
		} else {
			port = 9999
		}
	}

	// Update config with the port as string
	config.Web.Port = strconv.Itoa(port)

	interval := time.Duration(monitoringConfig.Interval) * time.Second
	app := core.NewApp(monitoringConfig.DataFile, interval, config)

	// Initialize app
	if err := app.Initialize(); err != nil {
		log.Printf("Warning: Failed to initialize app: %v", err)
	}

	server := api.NewServer(app, port)

	if !options.Quiet {
		fmt.Printf("🚀 启动Process Tracker API服务器\n")
		fmt.Printf("📖 API文档: http://localhost:%d/v1/docs\n", port)
		fmt.Printf("🌐 Web界面: http://localhost:%d/\n", port)
		fmt.Printf("❤️  Health Check: http://localhost:%d/health\n", port)
		fmt.Printf("\n按 Ctrl+C 停止服务器\n")
	}

	if err := server.Run(); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}
}

func main() {
	command, options := parseCommandLine()

	if options.Help {
		printHelp()
		return
	}

	if options.Version {
		fmt.Printf("Process Tracker v%s\n", Version)
		return
	}

	switch command {
	case "start":
		handleStart(options)
	case "stop":
		handleStop(options)
	case "status":
		handleStatus(options)
	case "stats":
		handleStats(options)
	case "web":
		handleWeb(options)
	default:
		fmt.Printf("未知命令: %s\n", command)
		fmt.Println("使用 -h 查看帮助信息")
		os.Exit(1)
	}
}