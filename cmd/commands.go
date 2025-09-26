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

// NewMonitoringCommands creates a new MonitoringCommands instance
func NewMonitoringCommands(app *core.App) *MonitoringCommands {
	return &MonitoringCommands{app: app}
}

// ShowTodayStats shows today's process statistics
func (mc *MonitoringCommands) ShowTodayStats(granularity string) error {
	stats, err := mc.app.CalculateResourceStats(24 * time.Hour)
	if err != nil {
		return fmt.Errorf("获取今日统计失败: %w", err)
	}

	DisplayStats("今日统计", stats, granularity)
	return nil
}

// ShowWeekStats shows this week's process statistics
func (mc *MonitoringCommands) ShowWeekStats(granularity string) error {
	stats, err := mc.app.CalculateResourceStats(7 * 24 * time.Hour)
	if err != nil {
		return fmt.Errorf("获取本周统计失败: %w", err)
	}

	DisplayStats("本周统计", stats, granularity)
	return nil
}

// ShowMonthStats shows this month's process statistics
func (mc *MonitoringCommands) ShowMonthStats(granularity string) error {
	stats, err := mc.app.CalculateResourceStats(30 * 24 * time.Hour)
	if err != nil {
		return fmt.Errorf("获取本月统计失败: %w", err)
	}

	DisplayStats("本月统计", stats, granularity)
	return nil
}

// ShowDetailedStats shows detailed process statistics
func (mc *MonitoringCommands) ShowDetailedStats(granularity string) error {
	stats, err := mc.app.CalculateResourceStats(24 * time.Hour)
	if err != nil {
		return fmt.Errorf("获取详细统计失败: %w", err)
	}

	DisplayStats("详细统计", stats, granularity)
	return nil
}

// ExportData exports process data to JSON format
func (mc *MonitoringCommands) ExportData(filename string) error {
	// Calculate stats from data file
	stats, err := mc.app.CalculateResourceStats(24 * time.Hour)
	if err != nil {
		return fmt.Errorf("计算统计数据失败: %w", err)
	}

	return ExportData(stats, filename)
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