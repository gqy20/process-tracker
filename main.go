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

	// Create app
	app := NewApp(*dataFile, 5*time.Second, config)
	monitorCmd := cmd.NewMonitoringCommands(app.App)

	// Handle commands with subcommand flags
	switch command {
	case "start":
		startFlags := flag.NewFlagSet("start", flag.ExitOnError)
		intervalSec := startFlags.Int("interval", 5, "监控间隔(秒)")
		startFlags.Parse(flag.Args()[1:])

		interval := time.Duration(*intervalSec) * time.Second
		if interval <= 0 {
			interval = 5 * time.Second
		}
		app.Interval = interval

		if err := monitorCmd.StartMonitoring(); err != nil {
			log.Fatalf("启动监控失败: %v", err)
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
