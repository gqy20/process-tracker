package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourusername/process-tracker/core"
	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from file or returns default
func LoadConfig(configPath string) (core.Config, error) {
	config := core.GetDefaultConfig()

	expandedPath := os.ExpandEnv(configPath)
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		// No config file, create default one
		if err := createDefaultConfigFile(expandedPath, config); err != nil {
			log.Printf("Warning: Failed to create default config file: %v", err)
		}
		return config, nil
	}

	// Read and parse YAML config file
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		log.Printf("Warning: Failed to read config file: %v", err)
		return config, nil
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Printf("Warning: Failed to parse config file: %v", err)
		return config, nil
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		log.Printf("Warning: Invalid config, using defaults: %v", err)
		return core.GetDefaultConfig(), nil
	}

	return config, nil
}

// createDefaultConfigFile creates a default configuration file
func createDefaultConfigFile(configPath string, config core.Config) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// Create default YAML config
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	// Write config file with header
	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header comment
	if _, err := file.WriteString("# 进程跟踪器配置文件\n# Process Tracker Configuration File\n\n"); err != nil {
		return err
	}

	// Write YAML data
	if _, err := file.Write(data); err != nil {
		return err
	}

	return nil
}

// validateConfig validates the configuration
func validateConfig(config core.Config) error {
	// Use the comprehensive validation from core package
	return core.ValidateConfig(config)
}

// PrintUsage displays command usage information
func PrintUsage(version string) {
	fmt.Printf("进程跟踪器 v%s - 增强版\n\n", version)
	fmt.Println("用法: process-tracker <命令> [选项]")
	fmt.Println("")
	fmt.Println("进程控制:")
	fmt.Println("  start              开始监控")
	fmt.Println("  stop               停止监控")
	fmt.Println("  restart            重启监控")
	fmt.Println("  status             查看运行状态")
	fmt.Println("  web                启动Web界面")
	fmt.Println("")
	fmt.Println("统计查询:")
	fmt.Println("  today/week/month   显示统计 (支持排序、筛选)")
	fmt.Println("  details            详细统计")
	fmt.Println("  compare            对比两个时间段 (today/yesterday/week/month)")
	fmt.Println("  trends             显示资源使用趋势")
	fmt.Println("  export             导出数据 (JSON/CSV)")
	fmt.Println("")
	fmt.Println("其他:")
	fmt.Println("  version            版本信息")
	fmt.Println("  help               帮助信息")
	fmt.Println("")
	fmt.Println("全局选项:")
	fmt.Println("  --config           配置文件路径 (默认: ~/.process-tracker/config.yaml)")
	fmt.Println("  --data-file        数据文件路径 (默认: ~/.process-tracker/process-tracker.log)")
	fmt.Println("")
	fmt.Println("统计选项 (用于today/week/month/details):")
	fmt.Println("  -g <mode>          统计粒度: simple/detailed/full")
	fmt.Println("  -s <field>         排序: cpu/memory/time/disk/network")
	fmt.Println("  -f <name>          按进程名筛选 (支持部分匹配)")
	fmt.Println("  -c <category>      按分类筛选")
	fmt.Println("  -n <number>        显示前N条 (0=全部)")
	fmt.Println("")
	fmt.Println("  提示: 所有参数都是单字母短参数，简洁高效")
	fmt.Println("")
	fmt.Println("导出选项:")
	fmt.Println("  --format <type>    导出格式: json/csv (默认: json)")
	fmt.Println("  --output <file>    输出文件名")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  # 基本使用")
	fmt.Println("  process-tracker start            # 开始监控（零配置）")
	fmt.Println("  process-tracker start --web      # 启动监控+Web界面")
	fmt.Println("  process-tracker status           # 查看状态")
	fmt.Println("  process-tracker stop             # 停止监控")
	fmt.Println("  process-tracker restart          # 重启监控")
	fmt.Println("")
	fmt.Println("  # 统计查询")
	fmt.Println("  process-tracker today            # 今日统计")
	fmt.Println("  process-tracker week -s cpu      # 本周按CPU排序")
	fmt.Println("")
	fmt.Println("  # 排序和筛选")
	fmt.Println("  process-tracker today -s memory -n 10")
	fmt.Println("  process-tracker week -f chrome")
	fmt.Println("  process-tracker today -c development -g detailed")
	fmt.Println("")
	fmt.Println("  # 对比和趋势")
	fmt.Println("  process-tracker compare today yesterday")
	fmt.Println("  process-tracker trends --days 7")
	fmt.Println("")
	fmt.Println("  # 组合使用")
	fmt.Println("  process-tracker today -s cpu -n 5 -g full")
	fmt.Println("  process-tracker week -f docker -s memory")
	fmt.Println("")
	fmt.Println("  # 导出数据")
	fmt.Println("  process-tracker export --format csv --output stats.csv")
	fmt.Println("  process-tracker export --format json")
	fmt.Println("")
	fmt.Println("默认目录结构:")
	fmt.Println("  ~/.process-tracker/")
	fmt.Println("  ├── config.yaml              # 配置文件")
	fmt.Println("  ├── process-tracker.log      # 主日志文件")
	fmt.Println("  ├── process-tracker.log.1    # 轮转日志文件")
	fmt.Println("  └── process-tracker.log.2    # 轮转日志文件")
	fmt.Println("")
	fmt.Println("功能特性:")
	fmt.Println("  ✓ 实时进程监控 (CPU、内存、磁盘I/O)")
	fmt.Println("  ✓ Docker容器监控 (容器级别资源统计)")
	fmt.Println("  ✓ 智能排序和筛选")
	fmt.Println("  ✓ 汇总统计分析")
	fmt.Println("  ✓ 时间段对比")
	fmt.Println("  ✓ 趋势分析")
	fmt.Println("  ✓ 多格式导出 (JSON/CSV)")
}

// DisplayStatsWithOptions displays statistics with filtering, sorting, and summary
func DisplayStatsWithOptions(title string, stats []core.ResourceStats, opts StatsOptions) {
	// Apply filters
	stats = filterStats(stats, opts)

	// Apply sorting
	stats = sortStats(stats, opts.SortBy)

	// Apply top N limit
	originalCount := len(stats)
	if opts.Top > 0 && opts.Top < len(stats) {
		stats = stats[:opts.Top]
	}

	// Show summary if requested
	if opts.ShowSummary {
		displaySummary(title, stats, originalCount)
		fmt.Println()
	}

	// Display stats based on granularity
	switch opts.Granularity {
	case "simple":
		displaySimpleStatsEnhanced(stats, originalCount)
	case "detailed":
		displayDetailedStatsEnhanced(stats, originalCount)
	case "full":
		displayFullStatsEnhanced(stats, originalCount)
	default:
		displaySimpleStatsEnhanced(stats, originalCount)
	}
}

// displaySimpleStatsEnhanced displays simplified statistics with enhancements
func displaySimpleStatsEnhanced(stats []core.ResourceStats, totalCount int) {
	if len(stats) == 0 {
		fmt.Println("📊 暂无数据")
		return
	}

	fmt.Println("📊 进程使用统计")
	fmt.Println(strings.Repeat("─", 150))
	fmt.Printf("%-20s %8s %16s %12s %12s %12s %12s %12s\n",
		"进程名称", "PID", "启动时间", "观察时长", "CPU时间", "内存占比", "平均内存", "平均CPU")
	fmt.Println(strings.Repeat("─", 150))

	totalMem := calculateTotalMemory(stats)

	for _, stat := range stats {
		pidStr := formatPIDs(stat.PIDs)
		startTimeStr := formatStartTime(stat.ProcessStartTime)
		uptimeStr := formatDuration(stat.TotalUptime)
		cpuTimeStr := formatDuration(stat.TotalCPUTime)
		memPercent := fmt.Sprintf("%.1f%%", (stat.MemoryAvg/totalMem)*100)
		memFormatted := formatBytes(stat.MemoryAvg)
		cpuPercent := fmt.Sprintf("%.1f%%", stat.CPUAvg)

		fmt.Printf("%-20s %8s %16s %12s %12s %12s %12s %12s\n",
			truncateString(stat.Name, 20), pidStr, startTimeStr, uptimeStr, cpuTimeStr,
			memPercent, memFormatted, cpuPercent)
	}

	if totalCount > len(stats) {
		fmt.Printf("\n显示 %d 条，共 %d 条记录\n", len(stats), totalCount)
	}
}

// displayDetailedStatsEnhanced displays detailed statistics with enhancements
func displayDetailedStatsEnhanced(stats []core.ResourceStats, totalCount int) {
	if len(stats) == 0 {
		fmt.Println("📊 暂无数据")
		return
	}

	fmt.Println("📊 进程使用详细统计")
	fmt.Println(strings.Repeat("─", 115))
	fmt.Printf("%-25s %12s %12s %12s %15s %15s %12s\n",
		"进程名称", "活跃时间", "内存占比", "平均CPU", "平均内存", "平均磁盘", "样本数")
	fmt.Println(strings.Repeat("─", 115))

	totalMem := calculateTotalMemory(stats)

	for _, stat := range stats {
		activeTime := formatDuration(stat.ActiveTime)
		memPercent := fmt.Sprintf("%.1f%%", (stat.MemoryAvg/totalMem)*100)
		cpuPercent := fmt.Sprintf("%.1f%%", stat.CPUAvg)
		memFormatted := formatBytes(stat.MemoryAvg)
		diskFormatted := formatBytes(stat.DiskReadAvg + stat.DiskWriteAvg)
		samples := fmt.Sprintf("%d", stat.Samples)

		fmt.Printf("%-25s %12s %12s %12s %15s %15s %12s\n",
			truncateString(stat.Name, 25), activeTime, memPercent, cpuPercent, memFormatted, diskFormatted, samples)
	}

	if totalCount > len(stats) {
		fmt.Printf("\n显示 %d 条，共 %d 条记录\n", len(stats), totalCount)
	}
}

// displayFullStatsEnhanced displays comprehensive statistics with enhancements
func displayFullStatsEnhanced(stats []core.ResourceStats, totalCount int) {
	if len(stats) == 0 {
		fmt.Println("📊 暂无数据")
		return
	}

	fmt.Println("📊 进程使用完整统计")
	fmt.Println(strings.Repeat("═", 100))

	totalMem := calculateTotalMemory(stats)

	for i, stat := range stats {
		memPercent := (stat.MemoryAvg / totalMem) * 100

		fmt.Printf("进程: %s\n", stat.Name)
		if stat.Command != "" {
			fmt.Printf("  命令: %s\n", truncateString(stat.Command, 80))
		}
		if stat.WorkingDir != "" {
			fmt.Printf("  目录: %s\n", truncateString(stat.WorkingDir, 60))
		}
		fmt.Printf("  类型: %s\n", stat.Category)
		fmt.Printf("  活跃时间: %s\n", formatDuration(stat.ActiveTime))
		fmt.Printf("  平均内存: %s (%.1f%%)\n", formatBytes(stat.MemoryAvg), memPercent)
		fmt.Printf("  最大内存: %s\n", formatBytes(stat.MemoryMax))
		fmt.Printf("  平均CPU: %.1f%%\n", stat.CPUAvg)
		fmt.Printf("  最大CPU: %.1f%%\n", stat.CPUMax)
		fmt.Printf("  平均磁盘读: %s\n", formatBytes(stat.DiskReadAvg))
		fmt.Printf("  平均磁盘写: %s\n", formatBytes(stat.DiskWriteAvg))
		fmt.Printf("  样本数: %d\n", stat.Samples)
		fmt.Printf("  活跃样本: %d\n", stat.ActiveSamples)
		fmt.Println(strings.Repeat("─", 80))

		if i+1 >= len(stats) && totalCount > len(stats) {
			fmt.Printf("\n显示 %d 条，共 %d 条记录\n", len(stats), totalCount)
		}
	}
}

// formatDuration formats time duration in human readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}

// formatPIDs formats PID list in compact form
func formatPIDs(pids []int32) string {
	if len(pids) == 0 {
		return "-"
	}
	if len(pids) == 1 {
		return fmt.Sprintf("%d", pids[0])
	}
	// Show first PID + count for multiple PIDs
	return fmt.Sprintf("%d+%d", pids[0], len(pids)-1)
}

// formatStartTime formats process start time
func formatStartTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	
	now := time.Now()
	
	// If today, show only time
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	
	// If this year, show month-day and time
	if t.Year() == now.Year() {
		return t.Format("01-02 15:04")
	}
	
	// If different year, show full date with year
	return t.Format("2006-01-02 15:04")
}

// truncateString truncates string to specified length
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// formatBytes formats bytes/MB value with appropriate unit (MB/GB/TB)
func formatBytes(mb float64) string {
	if mb >= 1024*1024 { // >= 1TB
		return fmt.Sprintf("%.2fTB", mb/1024/1024)
	} else if mb >= 1024 { // >= 1GB
		return fmt.Sprintf("%.2fGB", mb/1024)
	} else if mb >= 1 {
		return fmt.Sprintf("%.1fMB", mb)
	} else {
		return fmt.Sprintf("%.2fKB", mb*1024)
	}
}

// formatBytesSimple formats bytes/MB value with appropriate unit, less precision
func formatBytesSimple(mb float64) string {
	if mb >= 1024*1024 { // >= 1TB
		return fmt.Sprintf("%.1fTB", mb/1024/1024)
	} else if mb >= 1024 { // >= 1GB
		return fmt.Sprintf("%.1fGB", mb/1024)
	} else if mb >= 1 {
		return fmt.Sprintf("%.0fMB", mb)
	} else {
		return fmt.Sprintf("%.0fKB", mb*1024)
	}
}

// filterStats filters statistics based on options
func filterStats(stats []core.ResourceStats, opts StatsOptions) []core.ResourceStats {
	var filtered []core.ResourceStats

	for _, stat := range stats {
		// Filter by name
		if opts.Filter != "" && !strings.Contains(strings.ToLower(stat.Name), strings.ToLower(opts.Filter)) {
			continue
		}

		// Filter by category
		if opts.Category != "" && !strings.EqualFold(stat.Category, opts.Category) {
			continue
		}

		filtered = append(filtered, stat)
	}

	return filtered
}

// sortStats sorts statistics based on the sort option
func sortStats(stats []core.ResourceStats, sortBy string) []core.ResourceStats {
	if sortBy == "" {
		return stats
	}

	// Create a copy to sort
	sorted := make([]core.ResourceStats, len(stats))
	copy(sorted, stats)

	switch strings.ToLower(sortBy) {
	case "cpu":
		// Sort by CPU average (descending)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i].CPUAvg < sorted[j].CPUAvg {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	case "memory", "mem":
		// Sort by memory average (descending)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i].MemoryAvg < sorted[j].MemoryAvg {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	case "time":
		// Sort by active time (descending)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i].ActiveTime < sorted[j].ActiveTime {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	case "disk":
		// Sort by disk I/O (descending)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				totalI := sorted[i].DiskReadAvg + sorted[i].DiskWriteAvg
				totalJ := sorted[j].DiskReadAvg + sorted[j].DiskWriteAvg
				if totalI < totalJ {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	case "network", "net":
		// Sort by network usage (descending)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				totalI := sorted[i].NetSentAvg + sorted[i].NetRecvAvg
				totalJ := sorted[j].NetSentAvg + sorted[j].NetRecvAvg
				if totalI < totalJ {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	}

	return sorted
}

// displaySummary displays summary statistics
func displaySummary(title string, stats []core.ResourceStats, totalCount int) {
	if len(stats) == 0 {
		return
	}

	fmt.Printf("📋 %s - 汇总信息\n", title)
	fmt.Println(strings.Repeat("─", 60))

	// Calculate totals
	var totalCPU, totalMem, totalDisk, totalNet float64
	var totalActiveTime time.Duration
	activeProcesses := 0

	for _, stat := range stats {
		totalCPU += stat.CPUAvg
		totalMem += stat.MemoryAvg
		totalDisk += stat.DiskReadAvg + stat.DiskWriteAvg
		totalNet += stat.NetSentAvg + stat.NetRecvAvg
		totalActiveTime += stat.ActiveTime
		if stat.ActiveSamples > 0 {
			activeProcesses++
		}
	}

	avgCPU := totalCPU / float64(len(stats))
	avgMem := totalMem / float64(len(stats))

	fmt.Printf("  进程总数: %d 个\n", totalCount)
	fmt.Printf("  活跃进程: %d 个\n", activeProcesses)
	fmt.Printf("  总活跃时间: %s\n", formatDuration(totalActiveTime))
	fmt.Printf("  平均CPU使用: %.1f%%\n", avgCPU)
	fmt.Printf("  总内存使用: %s\n", formatBytes(totalMem))
	fmt.Printf("  平均内存使用: %s\n", formatBytes(avgMem))
	fmt.Printf("  总磁盘I/O: %s\n", formatBytes(totalDisk))
}

// calculateTotalMemory calculates the total memory usage
func calculateTotalMemory(stats []core.ResourceStats) float64 {
	var total float64
	for _, stat := range stats {
		total += stat.MemoryAvg
	}
	if total == 0 {
		return 1 // Avoid division by zero
	}
	return total
}

// ExportDataJSON exports data to JSON format
func ExportDataJSON(data []core.ResourceStats, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ 数据已导出到: %s (JSON格式)\n", filename)
	return nil
}

// ExportDataCSV exports data to CSV format
func ExportDataCSV(data []core.ResourceStats, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write CSV header
	header := "进程名称,分类,活跃时间(秒),平均CPU(%),最大CPU(%),平均内存(MB),最大内存(MB)," +
		"平均磁盘读(MB),平均磁盘写(MB),平均网络发送(KB),平均网络接收(KB),样本数,活跃样本数\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write data rows
	for _, stat := range data {
		row := fmt.Sprintf("%s,%s,%.0f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%d,%d\n",
			stat.Name,
			stat.Category,
			stat.ActiveTime.Seconds(),
			stat.CPUAvg,
			stat.CPUMax,
			stat.MemoryAvg,
			stat.MemoryMax,
			stat.DiskReadAvg,
			stat.DiskWriteAvg,
			stat.NetSentAvg,
			stat.NetRecvAvg,
			stat.Samples,
			stat.ActiveSamples,
		)
		if _, err := file.WriteString(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	fmt.Printf("✅ 数据已导出到: %s (CSV格式)\n", filename)
	return nil
}
