package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/yourusername/process-tracker/cmd"
	"github.com/yourusername/process-tracker/core"
)

// Version is set during build
var Version = "0.4.1"

// App wraps the core.App with CLI-specific functionality
type App struct {
	*core.App
}

// NewApp creates a new App instance
func NewApp(dataFile string, interval time.Duration, config core.Config) *App {
	return &App{
		App: core.NewApp(dataFile, interval, config),
	}
}

func main() {
	// 简化的命令行参数处理
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	// 加载配置
	config, err := cmd.LoadConfig(os.ExpandEnv("$HOME/.process-tracker/config.yaml"))
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
		config = core.GetDefaultConfig()
	}

	// 初始化daemon管理器
	dataDir := os.ExpandEnv("$HOME/.process-tracker")
	daemon := core.NewDaemonManager(dataDir)

	// 处理5个核心命令
	switch command {
	case "start":
		handleStart(daemon, config)
	case "stop":
		handleStop(daemon)
	case "status":
		handleStatus(daemon)
	case "stats":
		handleStats(config)
	case "web":
		handleWeb(config)
	case "help", "-h":
		printUsage()
	case "version", "-v":
		fmt.Printf("Process Tracker v%s\n", Version)
	default:
		fmt.Printf("❌ 未知命令: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

// printUsage prints simplified usage information
func printUsage() {
	fmt.Printf("Process Tracker v%s\n\n", Version)
	fmt.Printf("用法:\n")
	fmt.Printf("  process-tracker <command> [options]\n\n")
	fmt.Printf("命令:\n")
	fmt.Printf("  start   启动监控\n")
	fmt.Printf("  stop    停止监控\n")
	fmt.Printf("  status  状态\n")
	fmt.Printf("  stats   统计\n")
	fmt.Printf("  web     Web界面\n")
	fmt.Printf("  help    帮助\n")
	fmt.Printf("  version 版本\n\n")
	fmt.Printf("选项:\n")
	fmt.Printf("  -i N    间隔(秒)\n")
	fmt.Printf("  -w      启动Web\n")
	fmt.Printf("  -p N    端口\n")
	fmt.Printf("  -d      今日统计\n")
	fmt.Printf("  -w      本周统计\n")
	fmt.Printf("  -m      本月统计\n\n")
	fmt.Printf("示例:\n")
	fmt.Printf("  process-tracker start\n")
	fmt.Printf("  process-tracker start -i 10 -w\n")
	fmt.Printf("  process-tracker stats -w\n")
	fmt.Printf("  process-tracker web -p 9090\n")
}

// handleStart handles the start command
func handleStart(daemon *core.DaemonManager, config core.Config) {
	// Check if already running
	if running, pid, _ := daemon.IsRunning(); running {
		fmt.Printf("❌ 进程已在运行 (PID: %d)\n", pid)
		fmt.Println("💡 使用 'process-tracker stop' 停止")
		os.Exit(1)
	}

	// Write PID file
	if err := daemon.WritePID(); err != nil {
		log.Printf("Warning: Failed to write PID file: %v", err)
	}

	// Parse flags for start command
	interval := 5 * time.Second
	webEnabled := false
	webPort := config.Web.Port

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-i":
			if i+1 < len(os.Args) {
				if sec, err := strconv.Atoi(os.Args[i+1]); err == nil && sec > 0 {
					interval = time.Duration(sec) * time.Second
					i++
				}
			}
		case "-w":
			webEnabled = true
		case "-p":
			if i+1 < len(os.Args) {
				webPort = os.Args[i+1]
				i++
			}
		}
	}

	// Create app
	dataFile := os.ExpandEnv("$HOME/.process-tracker/process-tracker.log")
	app := NewApp(dataFile, interval, config)
	monitorCmd := cmd.NewMonitoringCommands(app.App)

	// Start web server if enabled
	if webEnabled || config.Web.Enabled {
		port := webPort
		webServer := cmd.NewWebServer(app.App, config.Web.Host, port)
		go func() {
			if err := webServer.Start(); err != nil {
				log.Printf("Web服务器错误: %v", err)
			}
		}()
		fmt.Printf("🚀 启动进程监控 (间隔: %v, Web: http://%s:%s)\n", interval, config.Web.Host, webPort)
	} else {
		fmt.Printf("🚀 启动进程监控 (间隔: %v)\n", interval)
	}

	if err := monitorCmd.StartMonitoring(); err != nil {
		daemon.RemovePID()
		log.Fatalf("启动监控失败: %v", err)
	}
}

// handleStats handles the stats command
func handleStats(config core.Config) {
	// Default to today's stats
	period := 24 * time.Hour
	title := "今日统计"

	// Parse stats flags
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-d":
			period = 24 * time.Hour
			title = "今日统计"
		case "-w":
			period = 7 * 24 * time.Hour
			title = "本周统计"
		case "-m":
			period = 30 * 24 * time.Hour
			title = "本月统计"
		}
	}

	// Create app and show stats
	dataFile := os.ExpandEnv("$HOME/.process-tracker/process-tracker.log")
	app := NewApp(dataFile, 5*time.Second, config)
	monitorCmd := cmd.NewMonitoringCommands(app.App)

	opts := cmd.StatsOptions{
		Granularity: "simple",
		ShowSummary: true,
	}

	if err := monitorCmd.ShowStats(title, period, opts); err != nil {
		log.Fatalf("显示统计失败: %v", err)
	}
}

// handleWeb handles the web command
func handleWeb(config core.Config) {
	// Parse web flags
	host := config.Web.Host
	port := config.Web.Port

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-p":
			if i+1 < len(os.Args) {
				port = os.Args[i+1]
				i++
			}
		case "-h":
			if i+1 < len(os.Args) {
				host = os.Args[i+1]
				i++
			}
		}
	}

	// Create app
	dataFile := os.ExpandEnv("$HOME/.process-tracker/process-tracker.log")
	app := NewApp(dataFile, 5*time.Second, config)

	if err := app.Initialize(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// Start monitoring in background
	monitorCmd := cmd.NewMonitoringCommands(app.App)
	go func() {
		if err := monitorCmd.StartMonitoring(); err != nil {
			log.Printf("监控启动失败: %v", err)
		}
	}()

	// Start web server
	webServer := cmd.NewWebServer(app.App, host, port)
	fmt.Printf("🌐 启动Web界面: http://%s:%s\n", host, port)
	if err := webServer.Start(); err != nil {
		log.Fatalf("Web服务器启动失败: %v", err)
	}
}

// handleStop handles the stop command
func handleStop(daemon *core.DaemonManager) {
	status, pid, err := daemon.GetStatus()
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		os.Exit(1)
	}

	if status != "running" {
		fmt.Println("⚠️  进程未运行")
		os.Exit(0)
	}

	fmt.Printf("🛑 正在停止进程 (PID: %d)...\n", pid)
	if err := daemon.Stop(); err != nil {
		fmt.Printf("❌ 停止失败: %v\n", err)
		os.Exit(1)
	}

	// Wait for process to exit with timeout (max 5 seconds)
	maxWait := 5 * time.Second
	checkInterval := 100 * time.Millisecond
	elapsed := time.Duration(0)

	for elapsed < maxWait {
		time.Sleep(checkInterval)
		elapsed += checkInterval

		if running, _, _ := daemon.IsRunning(); !running {
			fmt.Println("✅ 进程已停止")
			return
		}
	}

	// Timeout - process still running
	fmt.Println("⚠️  进程在5秒内未停止，可能需要强制终止")
	fmt.Printf("💡 使用以下命令强制终止: kill -9 %d\n", pid)
}

// handleStatus handles the status command
func handleStatus(daemon *core.DaemonManager) {
	status, pid, err := daemon.GetStatus()
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("📊 Process Tracker 状态")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if status == "running" {
		fmt.Printf("状态: 🟢 运行中\n")
		fmt.Printf("PID:  %d\n", pid)

		// Additional info if available
		dataFile := os.ExpandEnv("$HOME/.process-tracker/process-tracker.log")
		if info, err := os.Stat(dataFile); err == nil {
			sizeMB := float64(info.Size()) / 1024 / 1024
			fmt.Printf("数据: %.2f MB\n", sizeMB)
			fmt.Printf("更新: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
		}
	} else {
		fmt.Printf("状态: 🔴 已停止\n")
	}
}