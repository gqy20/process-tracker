package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start monitoring loop
	stopCh := make(chan struct{})
	go mc.monitoringLoop(stopCh)

	fmt.Println("✅ 监控已启动")

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n🛑 收到停止信号，正在关闭...")
	
	// Signal monitoring loop to stop
	close(stopCh)
	
	// Give it a moment to finish current work
	time.Sleep(500 * time.Millisecond)
	
	// Cleanup
	if err := mc.app.CloseFile(); err != nil {
		fmt.Printf("⚠️  清理资源失败: %v\n", err)
	}
	
	fmt.Println("✅ 监控已停止")
	return nil
}

// monitoringLoop runs the actual monitoring in a goroutine
func (mc *MonitoringCommands) monitoringLoop(stopCh chan struct{}) {
	ticker := time.NewTicker(mc.app.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := mc.app.CollectAndSaveData(); err != nil {
				fmt.Printf("收集数据失败: %v\n", err)
			}
		case <-stopCh:
			return
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

// ClearAllData removes all historical data files
func (mc *MonitoringCommands) ClearAllData(force bool) error {
	dataDir := filepath.Dir(mc.app.DataFile)
	baseFile := filepath.Base(mc.app.DataFile)

	if !force {
		fmt.Printf("\n⚠️  警告: 即将删除所有历史数据文件\n")
		fmt.Printf("📂 目录: %s\n", dataDir)
		fmt.Printf("🗑️  文件模式: %s*\n\n", baseFile)
		fmt.Printf("确认删除? (yes/no): ")

		reader := bufio.NewReader(os.Stdin)
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))

		if confirm != "yes" && confirm != "y" {
			fmt.Println("❌ 已取消操作")
			return nil
		}
	}

	// Find all related files
	pattern := filepath.Join(dataDir, baseFile+"*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("查找数据文件失败: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("ℹ️  未找到数据文件")
		return nil
	}

	// Delete files
	deleted := 0
	var errors []error
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			errors = append(errors, err)
			fmt.Printf("❌ 删除失败: %s - %v\n", filepath.Base(file), err)
		} else {
			deleted++
			if !force {
				fmt.Printf("✅ 已删除: %s\n", filepath.Base(file))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("部分文件删除失败 (%d个错误)", len(errors))
	}

	fmt.Printf("\n✅ 成功删除 %d 个数据文件\n", deleted)
	return nil
}
