package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/yourusername/process-tracker/cmd"
	"github.com/yourusername/process-tracker/core"
)

// Version is set during build
var Version = "0.4.0"

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
	// Global flags
	configPath := flag.String("config", os.ExpandEnv("$HOME/.process-tracker/config.yaml"), "配置文件路径")
	dataFile := flag.String("data-file", os.ExpandEnv("$HOME/.process-tracker/process-tracker.log"), "数据文件路径")
	help := flag.Bool("help", false, "显示帮助信息")

	flag.Parse()

	if *help || flag.NArg() == 0 {
		cmd.PrintUsage(Version)
		return
	}

	// Load configuration
	config, err := cmd.LoadConfig(*configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
		config = core.GetDefaultConfig()
	}

	command := flag.Arg(0)

	// Handle version command early
	if command == "version" {
		fmt.Printf("进程跟踪器版本 %s\n", Version)
		return
	}

	if command == "help" {
		cmd.PrintUsage(Version)
		return
	}

	// Initialize daemon manager for process control
	dataDir := os.ExpandEnv("$HOME/.process-tracker")
	daemon := core.NewDaemonManager(dataDir)

	// Handle daemon control commands
	switch command {
	case "stop":
		handleStop(daemon)
		return
	case "status":
		handleStatus(daemon)
		return
	case "restart":
		handleRestart(daemon, *dataFile, config)
		return
	}

	// Create app
	app := NewApp(*dataFile, 5*time.Second, config)
	monitorCmd := cmd.NewMonitoringCommands(app.App)

	// Handle commands with subcommand flags
	switch command {
	case "start":
		// Check if already running
		if running, pid, _ := daemon.IsRunning(); running {
			fmt.Printf("❌ 进程已在运行 (PID: %d)\n", pid)
			fmt.Println("💡 使用 'process-tracker stop' 停止，或 'process-tracker restart' 重启")
			os.Exit(1)
		}

		startFlags := flag.NewFlagSet("start", flag.ExitOnError)
		intervalSec := startFlags.Int("interval", 5, "监控间隔(秒)")
		webEnabled := startFlags.Bool("web", false, "启用Web界面")
		webPort := startFlags.String("web-port", "", "Web服务器端口 (默认: 8080)")
		startFlags.Parse(flag.Args()[1:])

		interval := time.Duration(*intervalSec) * time.Second
		if interval <= 0 {
			interval = 5 * time.Second
		}
		app.Interval = interval

		// Write PID file
		if err := daemon.WritePID(); err != nil {
			log.Printf("Warning: Failed to write PID file: %v", err)
		}
		defer daemon.RemovePID()

		// Start web server if enabled
		if *webEnabled || config.Web.Enabled {
			port := config.Web.Port
			if *webPort != "" {
				port = *webPort
			}
			webServer := cmd.NewWebServer(app.App, config.Web.Host, port)
			go func() {
				if err := webServer.Start(); err != nil {
					log.Printf("Web服务器错误: %v", err)
				}
			}()
		}

		if err := monitorCmd.StartMonitoring(); err != nil {
			log.Fatalf("启动监控失败: %v", err)
		}

	case "web":
		webFlags := flag.NewFlagSet("web", flag.ExitOnError)
		webPort := webFlags.String("port", config.Web.Port, "Web服务器端口")
		webHost := webFlags.String("host", config.Web.Host, "Web服务器主机")
		webFlags.Parse(flag.Args()[1:])

		if err := app.Initialize(); err != nil {
			log.Fatalf("初始化失败: %v", err)
		}

		// Start monitoring in background
		go func() {
			if err := monitorCmd.StartMonitoring(); err != nil {
				log.Printf("监控启动失败: %v", err)
			}
		}()

		// Start web server
		webServer := cmd.NewWebServer(app.App, *webHost, *webPort)
		if err := webServer.Start(); err != nil {
			log.Fatalf("Web服务器启动失败: %v", err)
		}

	case "today", "week", "month", "details":
		opts := parseStatsFlags(flag.Args()[1:])
		var period time.Duration
		switch command {
		case "today":
			period = 24 * time.Hour
		case "week":
			period = 7 * 24 * time.Hour
		case "month":
			period = 30 * 24 * time.Hour
		case "details":
			period = 24 * time.Hour
		}

		if err := monitorCmd.ShowStats(command, period, opts); err != nil {
			log.Fatalf("显示统计失败: %v", err)
		}

	case "compare":
		if err := monitorCmd.CompareStats(flag.Args()[1:]); err != nil {
			log.Fatalf("对比统计失败: %v", err)
		}

	case "trends":
		trendsFlags := flag.NewFlagSet("trends", flag.ExitOnError)
		days := trendsFlags.Int("days", 7, "趋势分析天数")
		trendsFlags.Parse(flag.Args()[1:])

		if err := monitorCmd.ShowTrends(*days); err != nil {
			log.Fatalf("显示趋势失败: %v", err)
		}

	case "export":
		exportFlags := flag.NewFlagSet("export", flag.ExitOnError)
		format := exportFlags.String("format", "json", "导出格式 (json/csv)")
		filename := exportFlags.String("output", "", "输出文件名")
		exportFlags.Parse(flag.Args()[1:])

		if *filename == "" {
			if *format == "csv" {
				*filename = "process-tracker-export.csv"
			} else {
				*filename = "process-tracker-export.json"
			}
		}

		if err := monitorCmd.ExportData(*filename, *format); err != nil {
			log.Fatalf("导出数据失败: %v", err)
		}

	case "clear-data", "reset":
		clearFlags := flag.NewFlagSet("clear-data", flag.ExitOnError)
		force := clearFlags.Bool("force", false, "强制清除不提示确认")
		clearFlags.Parse(flag.Args()[1:])

		if err := monitorCmd.ClearAllData(*force); err != nil {
			log.Fatalf("清除数据失败: %v", err)
		}

	case "test-alert":
		// Test alert notification
		testFlags := flag.NewFlagSet("test-alert", flag.ExitOnError)
		channel := testFlags.String("channel", "feishu", "通知渠道: feishu/dingtalk/wechat")
		testFlags.Parse(flag.Args()[1:])

		if err := app.Initialize(); err != nil {
			log.Fatalf("初始化失败: %v", err)
		}

		if app.App.Config.Alerts.Enabled {
			fmt.Printf("🔔 测试告警通知 (渠道: %s)...\n", *channel)
			// Access the alertManager through a public method
			if err := testAlertNotification(app.App, *channel); err != nil {
				log.Fatalf("❌ 测试失败: %v", err)
			}
			fmt.Println("✅ 测试通知已发送")
		} else {
			fmt.Println("⚠️  告警功能未启用，请在配置文件中启用 alerts.enabled")
		}

	default:
		fmt.Printf("❌ 未知命令: %s\n\n", command)
		cmd.PrintUsage(Version)
		os.Exit(1)
	}
}

// parseStatsFlags parses statistics command flags
func parseStatsFlags(args []string) cmd.StatsOptions {
	statsFlags := flag.NewFlagSet("stats", flag.ExitOnError)

	opts := cmd.StatsOptions{}
	statsFlags.StringVar(&opts.Granularity, "g", "simple", "统计粒度: simple/detailed/full")
	statsFlags.StringVar(&opts.SortBy, "s", "", "排序: cpu/memory/time/disk/network")
	statsFlags.StringVar(&opts.Filter, "f", "", "按进程名筛选")
	statsFlags.StringVar(&opts.Category, "c", "", "按分类筛选")
	statsFlags.IntVar(&opts.Top, "n", 0, "显示前N条 (0=全部)")
	statsFlags.BoolVar(&opts.ShowSummary, "summary", true, "显示汇总统计")

	statsFlags.Parse(args)
	return opts
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

// testAlertNotification tests alert notification
func testAlertNotification(app *core.App, channel string) error {
	// Use reflection or add a public method to access alertManager
	// For now, we'll create a temporary alert manager with the same config
	if !app.Config.Alerts.Enabled {
		return fmt.Errorf("alerts not enabled in configuration")
	}
	
	testManager := core.NewAlertManager(app.Config.Alerts, app.Config.Notifiers)
	return testManager.TestNotifier(channel)
}

// handleRestart handles the restart command
func handleRestart(daemon *core.DaemonManager, dataFile string, config core.Config) {
	fmt.Println("🔄 重启进程...")
	
	// Stop if running
	if running, pid, _ := daemon.IsRunning(); running {
		fmt.Printf("🛑 停止现有进程 (PID: %d)...\n", pid)
		if err := daemon.Stop(); err != nil {
			fmt.Printf("❌ 停止失败: %v\n", err)
			os.Exit(1)
		}
		time.Sleep(1 * time.Second)
	}

	// Start new instance
	fmt.Println("🚀 启动新进程...")
	
	// Re-exec the current process with start command
	args := []string{os.Args[0], "start"}
	
	// Preserve web flag if it was enabled
	if config.Web.Enabled {
		args = append(args, "--web")
	}
	
	// Execute new process
	if err := syscall.Exec(os.Args[0], args, os.Environ()); err != nil {
		fmt.Printf("❌ 启动失败: %v\n", err)
		os.Exit(1)
	}
}
