package cmd

import (
	"fmt"
	"time"

	"github.com/yourusername/process-tracker/core"
)

// MonitoringCommands handles monitoring and statistics commands
type MonitoringCommands struct {
	app *core.App
}

// StatsOptions contains options for displaying statistics
type StatsOptions struct {
	Granularity string
	SortBy      string
	Filter      string
	Category    string
	Top         int
	ShowSummary bool
}

// NewMonitoringCommands creates a new MonitoringCommands instance
func NewMonitoringCommands(app *core.App) *MonitoringCommands {
	return &MonitoringCommands{app: app}
}

// ShowStats shows process statistics with options
func (mc *MonitoringCommands) ShowStats(title string, period time.Duration, opts StatsOptions) error {
	stats, err := mc.app.CalculateResourceStats(period)
	if err != nil {
		return fmt.Errorf("获取统计失败: %w", err)
	}

	// Map title to Chinese
	titleMap := map[string]string{
		"today":   "今日统计",
		"week":    "本周统计",
		"month":   "本月统计",
		"details": "详细统计",
	}
	displayTitle := titleMap[title]
	if displayTitle == "" {
		displayTitle = title
	}

	DisplayStatsWithOptions(displayTitle, stats, opts)
	return nil
}

// CompareStats compares statistics between different time periods
func (mc *MonitoringCommands) CompareStats(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法: compare <period1> <period2>\n例如: compare today yesterday")
	}

	period1, err := parsePeriod(args[0])
	if err != nil {
		return fmt.Errorf("无效的时间段1: %w", err)
	}

	period2, err := parsePeriod(args[1])
	if err != nil {
		return fmt.Errorf("无效的时间段2: %w", err)
	}

	return mc.app.CompareStats(period1, period2, args[0], args[1])
}

// ShowTrends shows resource usage trends over time
func (mc *MonitoringCommands) ShowTrends(days int) error {
	return mc.app.ShowTrends(days)
}

// ExportData exports process data to specified format
func (mc *MonitoringCommands) ExportData(filename string, format string) error {
	stats, err := mc.app.CalculateResourceStats(24 * time.Hour)
	if err != nil {
		return fmt.Errorf("计算统计数据失败: %w", err)
	}

	if format == "csv" {
		return ExportDataCSV(stats, filename)
	}
	return ExportDataJSON(stats, filename)
}

// StartMonitoring starts the process monitoring
func (mc *MonitoringCommands) StartMonitoring() error {
	fmt.Println("🔄 开始进程监控...")
	fmt.Printf("监控间隔: %v\n", mc.app.Interval)
	fmt.Printf("数据文件: %s\n", mc.app.DataFile)
	fmt.Println("按 Ctrl+C 停止监控")

	// Initialize the app
	if err := mc.app.Initialize(); err != nil {
		return fmt.Errorf("初始化失败: %w", err)
	}

	// Start monitoring loop
	go mc.monitoringLoop()

	fmt.Println("✅ 监控已启动")

	// Keep the main process running
	select {} // This blocks forever until interrupted

	return nil // This line will never be reached
}

// monitoringLoop runs the actual monitoring in a goroutine
func (mc *MonitoringCommands) monitoringLoop() {
	ticker := time.NewTicker(mc.app.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := mc.app.CollectAndSaveData(); err != nil {
				fmt.Printf("收集数据失败: %v\n", err)
			}
		}
	}
}

// parsePeriod parses a period string to time.Duration
func parsePeriod(period string) (time.Duration, error) {
	switch period {
	case "today":
		return 24 * time.Hour, nil
	case "yesterday":
		return 48 * time.Hour, nil
	case "week":
		return 7 * 24 * time.Hour, nil
	case "month":
		return 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("不支持的时间段: %s (支持: today, yesterday, week, month)", period)
	}
}
