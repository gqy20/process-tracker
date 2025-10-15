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
	fmt.Printf("进程跟踪器 v%s\n\n", version)
	fmt.Println("用法: process-tracker <命令> [选项]")
	fmt.Println("")
	fmt.Println("命令:")
	fmt.Println("  start              开始监控")
	fmt.Println("  today/week/month   显示统计")
	fmt.Println("  details            详细统计")
	fmt.Println("  export [文件]       导出JSON")
	fmt.Println("  version            版本信息")
	fmt.Println("  help               帮助信息")
	fmt.Println("")
	fmt.Println("选项:")
	fmt.Println("  -g, --granularity  统计粒度 (simple/detailed/full)")
	fmt.Println("  -c, --config       配置文件路径 (默认: ~/.process-tracker/config.yaml)")
	fmt.Println("  -d, --data-file    数据文件路径 (默认: ~/.process-tracker/process-tracker.log)")
	fmt.Println("  -i, --interval     监控间隔(秒)")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  process-tracker start")
	fmt.Println("  process-tracker today -g simple")
	fmt.Println("  process-tracker export data.json")
	fmt.Println("")
	fmt.Println("默认目录结构:")
	fmt.Println("  ~/.process-tracker/")
	fmt.Println("  ├── config.yaml         # 配置文件")
	fmt.Println("  ├── process-tracker.log  # 主日志文件")
	fmt.Println("  ├── process-tracker.log.1 # 轮转日志文件")
	fmt.Println("  └── process-tracker.log.2 # 轮转日志文件")
}

// DisplayStats displays statistics with appropriate formatting
func DisplayStats(title string, stats []core.ResourceStats, granularity string) {
	switch granularity {
	case "simple":
		displaySimpleStats(stats)
	case "detailed":
		displayDetailedStats(stats)
	case "full":
		displayFullStats(stats)
	default:
		displaySimpleStats(stats)
	}
}

// displaySimpleStats displays simplified statistics
func displaySimpleStats(stats []core.ResourceStats) {
	if len(stats) == 0 {
		fmt.Println("📊 暂无数据")
		return
	}

	fmt.Println("📊 进程使用时间统计 (简化)")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("%-20s %10s %10s %10s\n", "进程名称", "活跃时间", "平均内存", "平均网络")
	fmt.Println(strings.Repeat("─", 60))

	for _, stat := range stats {
		activeTime := formatDuration(stat.ActiveTime)
		memMB := fmt.Sprintf("%.1fMB", stat.MemoryAvg)
		netKB := fmt.Sprintf("%.1fKB", stat.NetSentAvg+stat.NetRecvAvg)

		fmt.Printf("%-20s %10s %10s %10s\n",
			truncateString(stat.Name, 20), activeTime, memMB, netKB)
	}

	if len(stats) > 10 {
		fmt.Printf("\n... 和其他 %d 个进程\n", len(stats)-10)
	}
}

// displayDetailedStats displays detailed statistics
func displayDetailedStats(stats []core.ResourceStats) {
	if len(stats) == 0 {
		fmt.Println("📊 暂无数据")
		return
	}

	fmt.Println("📊 进程使用时间统计 (详细)")
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("%-25s %12s %12s %12s %12s %12s\n",
		"进程名称", "活跃时间", "平均内存", "平均磁盘", "平均网络", "样本数")
	fmt.Println(strings.Repeat("─", 80))

	for _, stat := range stats {
		activeTime := formatDuration(stat.ActiveTime)
		memMB := fmt.Sprintf("%.1fMB", stat.MemoryAvg)
		diskMB := fmt.Sprintf("%.1fMB", stat.DiskReadAvg+stat.DiskWriteAvg)
		netKB := fmt.Sprintf("%.1fKB", stat.NetSentAvg+stat.NetRecvAvg)
		samples := fmt.Sprintf("%d", stat.Samples)

		fmt.Printf("%-25s %12s %12s %12s %12s %12s\n",
			truncateString(stat.Name, 25), activeTime, memMB, diskMB, netKB, samples)
	}

	if len(stats) > 15 {
		fmt.Printf("\n... 和其他 %d 个进程\n", len(stats)-15)
	}
}

// displayFullStats displays comprehensive statistics
func displayFullStats(stats []core.ResourceStats) {
	if len(stats) == 0 {
		fmt.Println("📊 暂无数据")
		return
	}

	fmt.Println("📊 进程使用时间统计 (完整)")
	fmt.Println(strings.Repeat("═", 100))

	for i, stat := range stats {
		if i >= 20 {
			fmt.Printf("\n... 和其他 %d 个进程\n", len(stats)-20)
			break
		}

		fmt.Printf("进程: %s\n", stat.Name)
		if stat.Command != "" {
			fmt.Printf("  命令: %s\n", truncateString(stat.Command, 80))
		}
		if stat.WorkingDir != "" {
			fmt.Printf("  目录: %s\n", truncateString(stat.WorkingDir, 60))
		}
		fmt.Printf("  类型: %s\n", stat.Category)
		fmt.Printf("  活跃时间: %s\n", formatDuration(stat.ActiveTime))
		fmt.Printf("  平均内存: %.1fMB\n", stat.MemoryAvg)
		fmt.Printf("  最大内存: %.1fMB\n", stat.MemoryMax)
		fmt.Printf("  平均CPU: %.1f%%\n", stat.CPUAvg)
		fmt.Printf("  最大CPU: %.1f%%\n", stat.CPUMax)
		fmt.Printf("  平均磁盘: %.1fMB\n", stat.DiskReadAvg+stat.DiskWriteAvg)
		fmt.Printf("  平均网络: %.1fKB\n", stat.NetSentAvg+stat.NetRecvAvg)
		fmt.Printf("  样本数: %d\n", stat.Samples)
		fmt.Printf("  活跃样本: %d\n", stat.ActiveSamples)
		fmt.Println(strings.Repeat("─", 80))
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

// truncateString truncates string to specified length
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// ExportData exports data to JSON format
func ExportData(data []core.ResourceStats, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ 数据已导出到: %s\n", filename)
	return nil
}
