package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/yourusername/process-tracker/cmd"
	"github.com/yourusername/process-tracker/core"
)

// Version is set during build
var Version = "0.3.9"

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
	// Parse command line flags
	var (
		configPath  = flag.String("config", os.ExpandEnv("$HOME/.process-tracker/config.yaml"), "配置文件路径")
		dataFile    = flag.String("data-file", os.ExpandEnv("$HOME/.process-tracker/process-tracker.log"), "数据文件路径")
		intervalSec = flag.Int("interval", 5, "监控间隔(秒)")
		granularity = flag.String("granularity", "simple", "统计粒度 (simple/detailed/full)")
		help        = flag.Bool("help", false, "显示帮助信息")
	)

	flag.Parse()

	// Show help if requested
	if *help {
		cmd.PrintUsage(Version)
		return
	}

	// Load configuration
	config, err := cmd.LoadConfig(*configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
		config = core.GetDefaultConfig()
	}

	// Create app with dependency injection
	interval := time.Duration(*intervalSec) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second // default interval
	}
	app := NewApp(*dataFile, interval, config)

	// Create command handlers
	monitorCmd := cmd.NewMonitoringCommands(app.App)

	// Handle commands
	if flag.NArg() == 0 {
		cmd.PrintUsage(Version)
		return
	}

	command := flag.Arg(0)
	switch command {
	case "version":
		fmt.Printf("进程跟踪器版本 %s\n", Version)
	case "start":
		if err := monitorCmd.StartMonitoring(); err != nil {
			log.Fatalf("启动监控失败: %v", err)
		}
	case "today":
		if err := monitorCmd.ShowTodayStats(*granularity); err != nil {
			log.Fatalf("显示今日统计失败: %v", err)
		}
	case "week":
		if err := monitorCmd.ShowWeekStats(*granularity); err != nil {
			log.Fatalf("显示本周统计失败: %v", err)
		}
	case "month":
		if err := monitorCmd.ShowMonthStats(*granularity); err != nil {
			log.Fatalf("显示本月统计失败: %v", err)
		}
	case "details":
		if err := monitorCmd.ShowDetailedStats(*granularity); err != nil {
			log.Fatalf("显示详细统计失败: %v", err)
		}
	case "export":
		filename := "process-tracker-export.json"
		if flag.NArg() > 1 {
			filename = flag.Arg(1)
		}
		if err := monitorCmd.ExportData(filename); err != nil {
			log.Fatalf("导出数据失败: %v", err)
		}
	case "help":
		cmd.PrintUsage(Version)
	default:
		fmt.Printf("❌ 未知命令: %s\n\n", command)
		cmd.PrintUsage(Version)
		os.Exit(1)
	}
}
