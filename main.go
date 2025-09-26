package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/yourusername/process-tracker/core"
	"gopkg.in/yaml.v3"
)

// Version is set during build
var Version = "0.3.3"

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

// closeFile wraps the core.App's closeFile method
func (a *App) closeFile() error {
	return a.App.CloseFile()
}

// loadConfig loads configuration from file or returns default
func loadConfig(configPath string) (core.Config, error) {
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

// validateConfig validates configuration values
func validateConfig(config core.Config) error {
	validGranularities := map[string]bool{
		"simple":   true,
		"detailed": true,
		"full":     true,
	}

	if !validGranularities[config.StatisticsGranularity] {
		return fmt.Errorf("invalid statistics_granularity: %s", config.StatisticsGranularity)
	}

	if config.MaxCommandLength < 10 {
		return fmt.Errorf("max_command_length too small: %d", config.MaxCommandLength)
	}

	if config.MaxDirLength < 10 {
		return fmt.Errorf("max_dir_length too small: %d", config.MaxDirLength)
	}

	return nil
}

func main() {
	// Configuration
	dataFile := os.ExpandEnv("$HOME/.process-tracker.log")
	configPath := os.ExpandEnv("$HOME/.process-tracker.yaml")
	const interval = 5 * time.Second

	// Load configuration
	config, err := loadConfig(configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
		config = core.GetDefaultConfig()
	}

	// Create app with dependency injection
	app := NewApp(dataFile, interval, config)

	if len(os.Args) < 2 {
		app.printUsage()
		return
	}

	command := os.Args[1]
	switch command {
	case "version":
		fmt.Printf("进程跟踪器版本 %s\n", Version)
	case "start":
		app.startMonitoring()
	case "today":
		app.showTodayStats()
	case "week":
		app.showWeekStats()
	case "month":
		app.showMonthStats()
	case "details":
		app.showDetailedStats()
	case "export":
		app.exportData()
	case "cleanup":
		app.cleanupOldData()
	case "start-process":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定进程名称")
			fmt.Println("用法: process-tracker start-process <名称> [命令...]")
			return
		}
		app.startProcess(os.Args[2:])
	case "stop-process":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定进程名称或PID")
			fmt.Println("用法: process-tracker stop-process <名称或PID>")
			return
		}
		app.stopProcess(os.Args[2])
	case "restart-process":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定进程名称或PID")
			fmt.Println("用法: process-tracker restart-process <名称或PID>")
			return
		}
		app.restartProcess(os.Args[2])
	case "list-processes":
		app.listManagedProcesses()
	case "add-quota":
		if len(os.Args) < 4 {
			fmt.Println("❌ 请指定配额名称和进程PID")
			fmt.Println("用法: process-tracker add-quota <配额名称> <PID>")
			return
		}
		app.addProcessToQuota(os.Args[2], os.Args[3])
	case "remove-quota":
		if len(os.Args) < 4 {
			fmt.Println("❌ 请指定配额名称和进程PID")
			fmt.Println("用法: process-tracker remove-quota <配额名称> <PID>")
			return
		}
		app.removeProcessFromQuota(os.Args[2], os.Args[3])
	case "list-quotas":
		app.listQuotas()
	case "list-discovered":
		app.listDiscoveredProcesses()
	case "list-groups":
		app.listProcessGroups()
	case "add-group":
		if len(os.Args) < 4 {
			fmt.Println("❌ 请指定组名称和匹配模式")
			fmt.Println("用法: process-tracker add-group <组名称> <匹配模式> [自动管理] [配额名称]")
			return
		}
		autoManage := false
		quotaName := ""
		if len(os.Args) > 4 {
			autoManage = os.Args[4] == "true"
		}
		if len(os.Args) > 5 {
			quotaName = os.Args[5]
		}
		app.addCustomGroup(os.Args[2], os.Args[3], autoManage, quotaName)
	case "remove-group":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定组名称")
			fmt.Println("用法: process-tracker remove-group <组名称>")
			return
		}
		app.removeCustomGroup(os.Args[2])
	case "discovery-stats":
		app.showDiscoveryStats()
	case "help":
		app.printUsage()
	default:
		app.printUsage()
	}
}

func (a *App) printUsage() {
	fmt.Println("进程跟踪器 - 智能进程监控工具 v0.3.3")
	fmt.Println()
	fmt.Println("使用方法:")
	fmt.Println("  process-tracker <命令>")
	fmt.Println()
	fmt.Println("监控命令:")
	fmt.Println("  start              开始监控进程")
	fmt.Println("  today              显示今日使用统计")
	fmt.Println("  week               显示本周使用统计")
	fmt.Println("  month              显示本月使用统计")
	fmt.Println("  details            显示详细资源使用统计")
	fmt.Println("  export             导出数据为JSON格式")
	fmt.Println("  cleanup            清理30天前的旧数据")
	fmt.Println()
	fmt.Println("进程控制命令:")
	fmt.Println("  start-process      启动指定进程")
	fmt.Println("  stop-process       停止指定进程")
	fmt.Println("  restart-process    重启指定进程")
	fmt.Println("  list-processes     列出所有托管进程")
	fmt.Println()
	fmt.Println("资源配额命令:")
	fmt.Println("  add-quota          将进程添加到配额管理")
	fmt.Println("  remove-quota       从配额管理中移除进程")
	fmt.Println("  list-quotas        列出所有资源配额")
	fmt.Println()
	fmt.Println("进程发现命令:")
	fmt.Println("  list-discovered    列出所有自动发现的进程")
	fmt.Println("  list-groups        列出所有进程组")
	fmt.Println("  add-group          添加自定义进程组")
	fmt.Println("  remove-group       移除自定义进程组")
	fmt.Println("  discovery-stats    显示进程发现统计")
	fmt.Println()
	fmt.Println("其他命令:")
	fmt.Println("  version            显示版本信息")
	fmt.Println("  help               显示此帮助信息")
	fmt.Println()
	fmt.Println("配置文件:")
	fmt.Println("  ~/.process-tracker.yaml - 控制统计粒度和显示选项")
	fmt.Println("    statistics_granularity: simple|detailed|full")
	fmt.Println("    show_commands: true|false")
	fmt.Println("    show_working_dirs: true|false")
	fmt.Println("    use_smart_categories: true|false")
	fmt.Println("    process_control:")
	fmt.Println("      enabled: true|false")
	fmt.Println("      enable_auto_restart: true|false")
	fmt.Println("      max_restarts: 3")
	fmt.Println("      restart_delay: 5s")
	fmt.Println("      check_interval: 10s")
	fmt.Println("    resource_quota:")
	fmt.Println("      enabled: true|false")
	fmt.Println("      check_interval: 30s")
	fmt.Println("      default_action: warn|throttle|stop|restart|notify")
	fmt.Println("      max_violations: 5")
	fmt.Println("      violation_window: 5m")
	fmt.Println("    process_discovery:")
	fmt.Println("      enabled: true|false")
	fmt.Println("      discovery_interval: 30s")
	fmt.Println("      auto_manage: true|false")
	fmt.Println("      bio_tools_only: true|false")
	fmt.Println("      process_patterns: [pattern1, pattern2]")
	fmt.Println("      exclude_patterns: [pattern1, pattern2]")
	fmt.Println("      max_processes: 100")
	fmt.Println("      cpu_threshold: 80.0")
	fmt.Println("      memory_threshold_mb: 1024")
	fmt.Println()
	fmt.Println("v0.3.3 新特性:")
	fmt.Println("  🔍 进程自动发现 - 自动识别和分类系统进程")
	fmt.Println("  🧬 生物信息学工具 - 内置常见生物信息学工具识别")
	fmt.Println("  🎯 智能分组 - 基于模式匹配的进程分组")
	fmt.Println("  🤖 自动管理 - 发现的进程自动加入配额管理")
	fmt.Println("  📊 发现统计 - 实时统计和分析发现的进程")
	fmt.Println()
	fmt.Println("v0.3.2 新特性:")
	fmt.Println("  📊 资源配额管理 - CPU、内存、线程等资源限制")
	fmt.Println("  🚨 配额违规检测 - 自动监控和告警机制")
	fmt.Println("  🎯 智能动作 - 违规时自动执行警告、限制或停止")
	fmt.Println("  📈 实时统计 - 配额使用情况和违规统计")
	fmt.Println()
	fmt.Println("v0.3.1 新特性:")
	fmt.Println("  🎛️  进程控制 - 启动、停止、重启进程")
	fmt.Println("  🔄 自动恢复 - 进程崩溃时自动重启")
	fmt.Println("  📋 进程管理 - 托管进程生命周期")
	fmt.Println("  ⚙️  配置扩展 - YAML配置进程控制选项")
	fmt.Println()
	fmt.Println("v0.2.2 新特性:")
	fmt.Println("  🚀 性能优化 - 批量文件写入，减少I/O操作")
	fmt.Println("  🌐 网络统计增强 - 基于连接的流量估算")
	fmt.Println("  🎨 用户体验改进 - 更友好的界面和错误处理")
	fmt.Println("  ⚙️  YAML配置文件支持 - 灵活的配置管理")
	fmt.Println("  📤 数据导出功能 - JSON格式导出和分析")
	fmt.Println("  🛑 优雅关闭 - 支持信号处理")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  process-tracker start")
	fmt.Println("  process-tracker start-process my-server /usr/bin/server")
	fmt.Println("  process-tracker stop-process my-server")
	fmt.Println("  process-tracker list-processes")
	fmt.Println("  process-tracker today")
	fmt.Println("  process-tracker details")
	fmt.Println("  process-tracker cleanup")
}

func (a *App) startMonitoring() {
	log.Printf("🚀 开始监控进程...")
	log.Printf("📁 数据文件: %s", a.DataFile)
	log.Printf("⏱️  监控间隔: %v", a.Interval)
	log.Printf("⚙️  配置: 统计粒度=%s, 显示命令=%v, 显示目录=%v, 智能分类=%v", 
		a.Config.StatisticsGranularity, a.Config.ShowCommands, a.Config.ShowWorkingDirs, a.Config.UseSmartCategories)
	
	// Log storage configuration
	if a.Config.Storage.MaxFileSizeMB > 0 {
		log.Printf("💾 存储管理: 最大文件=%dMB, 保留文件=%d, 压缩天数=%d, 清理天数=%d", 
			a.Config.Storage.MaxFileSizeMB, a.Config.Storage.MaxFiles, 
			a.Config.Storage.CompressAfterDays, a.Config.Storage.CleanupAfterDays)
	}
	
	// Initialize storage manager if enabled
	if err := a.Initialize(); err != nil {
		log.Fatalf("❌ 初始化失败: %v", err)
	}
	
	// Check data file accessibility
	if _, err := os.Stat(a.DataFile); os.IsNotExist(err) {
		// Create data file directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(a.DataFile), 0755); err != nil {
			log.Fatalf("❌ 无法创建数据目录: %v", err)
		}
		log.Printf("📝 将创建新数据文件")
	}

	// Simple monitoring loop - explicit and clear
	ticker := time.NewTicker(a.Interval)
	defer ticker.Stop()
	
	// Ensure file is closed when monitoring stops
	defer a.closeFile()
	
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Progress tracking
	cycleCount := 0
	startTime := time.Now()

	log.Printf("✅ 监控已启动，按 Ctrl+C 停止")

	for {
		select {
		case <-ticker.C:
			cycleCount++
			if err := a.monitorAndSave(); err != nil {
				log.Printf("❌ 监控错误: %v", err)
			} else if cycleCount%12 == 0 { // Every minute (assuming 5-second intervals)
				elapsed := time.Since(startTime)
				log.Printf("📊 运行状态: %d 次采样，运行时间 %v", cycleCount, elapsed.Round(time.Minute))
			}
		case <-sigChan:
			log.Printf("🛑 收到停止信号，正在清理...")
			return
		}
	}
}

func (a *App) monitorAndSave() error {
	// Get current processes with resource usage
	resources, err := a.getCurrentResources()
	if err != nil {
		return fmt.Errorf("failed to get resources: %w", err)
	}

	// Save to file using buffered writing
	if err := a.SaveResourceRecords(resources); err != nil {
		return fmt.Errorf("failed to save resources: %w", err)
	}

	return nil
}

func (a *App) getCurrentResources() ([]core.ResourceRecord, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var records []core.ResourceRecord
	for _, p := range processes {
		info, err := a.GetProcessInfo(p)
		if err != nil {
			continue // Skip processes we can't read
		}

		name := info.Name
		name = strings.TrimSpace(name)
		if name == "" || info.Pid <= 0 {
			continue // Skip invalid processes
		}

		// Skip obvious system processes
		if a.isSystemProcess(name) {
			continue
		}

		// Normalize process name
		name = a.normalizeProcessName(name)

		// Create resource record
		resource := core.ResourceRecord{
			Name:        name,
			Timestamp:   time.Now(),
			CPUPercent:  info.CPUPercent,
			MemoryMB:    info.MemoryMB,
			Threads:     info.Threads,
			DiskReadMB:  info.DiskReadMB,
			DiskWriteMB: info.DiskWriteMB,
			NetSentKB:   info.NetSentKB,
			NetRecvKB:   info.NetRecvKB,
			IsActive:    false, // Will be set below
			Command:     info.Cmdline,
			WorkingDir:  info.Cwd,
			Category:    "", // Will be set below
		}

		// Determine if process is active
		activityConfig := core.GetDefaultActivityConfig()
		resource.IsActive = core.IsActive(resource, activityConfig)
		
		// Set application category
		resource.Category = core.IdentifyApplication(name, info.Cmdline, a.Config.UseSmartCategories)

		records = append(records, resource)
	}

	return records, nil
}

func (a *App) isSystemProcess(name string) bool {
	name = strings.ToLower(name)
	systemPrefixes := []string{
		"kworker", "ksoftirqd", "migration", "rcu_", "watchdog",
		"khugepaged", "kthreadd", "kswapd", "pool", "cpuhp",
		"irq", "migration", "md", "jbd2", "ext4", "xfs",
		"loop", "sr_", "ata_", "scsi_", "usb", "pci",
		"idle_inject", "systemd", "dbus-daemon", "containerd-shim",
		"s6-supervise", "docker-proxy", "pipewire", "pulseaudio",
		"gvfsd", "gnome-keyring", "xdg-desktop-portal",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	// Additional system processes to filter by exact match
	systemProcesses := map[string]bool{
		"system": true,
		"init": true,
		"bash": true,  // Usually shell, not user application
		"ssh": true,   // System service
	}

	if systemProcesses[name] {
		return true
	}

	return false
}

func (a *App) normalizeProcessName(name string) string {
	// Remove common suffixes and normalize
	name = strings.TrimSuffix(name, ".exe")
	name = strings.TrimSuffix(name, ".so")
	name = strings.TrimSpace(name)
	return name
}

func (a *App) showTodayStats() {
	fmt.Printf("📊 正在计算今日统计...\n")
	stats, err := a.CalculateResourceStats(24 * time.Hour)
	if err != nil {
		log.Printf("❌ 计算今日统计时出错: %v", err)
		return
	}

	if len(stats) == 0 {
		fmt.Printf("📭 今日暂无进程使用数据\n")
		return
	}

	fmt.Printf("✅ 找到 %d 个进程的今日数据\n\n", len(stats))
	a.displayStats("今日进程使用统计", stats)
}

func (a *App) showWeekStats() {
	fmt.Printf("📊 正在计算本周统计...\n")
	stats, err := a.CalculateResourceStats(7 * 24 * time.Hour)
	if err != nil {
		log.Printf("❌ 计算本周统计时出错: %v", err)
		return
	}

	if len(stats) == 0 {
		fmt.Printf("📭 本周暂无进程使用数据\n")
		return
	}

	fmt.Printf("✅ 找到 %d 个进程的本周数据\n\n", len(stats))
	a.displayStats("本周进程使用统计", stats)
}

func (a *App) showMonthStats() {
	fmt.Printf("📊 正在计算本月统计...\n")
	stats, err := a.CalculateResourceStats(30 * 24 * time.Hour)
	if err != nil {
		log.Printf("❌ 计算本月统计时出错: %v", err)
		return
	}

	if len(stats) == 0 {
		fmt.Printf("📭 本月暂无进程使用数据\n")
		return
	}

	fmt.Printf("✅ 找到 %d 个进程的本月数据\n\n", len(stats))
	a.displayStats("本月进程使用统计", stats)
}

func (a *App) showDetailedStats() {
	fmt.Printf("📊 正在计算详细统计...\n")
	stats, err := a.CalculateResourceStats(24 * time.Hour)
	if err != nil {
		log.Printf("❌ 计算详细统计时出错: %v", err)
		return
	}

	if len(stats) == 0 {
		fmt.Printf("📭 今日暂无进程使用数据\n")
		return
	}

	fmt.Printf("✅ 找到 %d 个进程的详细数据\n\n", len(stats))
	fmt.Println("📈 详细资源使用统计 (今日)")
	fmt.Println("================================")
	fmt.Printf("%-30s %8s %8s %8s %8s %8s %12s %12s %8s %8s\n", 
		"进程名称", "样本数", "活跃数", "CPU平均", "CPU最大", "内存平均", "磁盘读取", "磁盘写入", "网络发送", "网络接收")
	fmt.Printf("%-30s %8s %8s %8s %8s %8s %12s %12s %8s %8s\n", 
		"--------", "------", "------", "------", "------", "------", "------", "------", "------", "------")

	for _, stat := range stats {
		processName := a.GetProcessNameFromStats(stat)
		if len(processName) > 30 {
			processName = processName[:27] + "..."
		}
		
		fmt.Printf("%-30s %8d %8d %8.1f %8.1f %8.1f %12.2f %12.2f %8.1f %8.1f\n",
			processName,
			stat.Samples,
			stat.ActiveSamples,
			stat.CPUAvg,
			stat.CPUMax,
			stat.MemoryAvg,
			stat.DiskReadAvg,
			stat.DiskWriteAvg,
			stat.NetSentAvg,
			stat.NetRecvAvg)
	}

	// Show additional details for top processes
	if len(stats) > 0 {
		fmt.Println("\n🔍 详细信息 (前5个进程):")
		fmt.Println("================================")
		for i := 0; i < 5 && i < len(stats); i++ {
			stat := stats[i]
			processName := a.GetProcessNameFromStats(stat)
			
			fmt.Printf("\n📍 %d. %s\n", i+1, processName)
			fmt.Printf("   ⏱️  活跃时间: %v\n", stat.ActiveTime.Round(time.Minute))
			fmt.Printf("   💻 命令行: %s\n", stat.Command)
			fmt.Printf("   📁 工作目录: %s\n", stat.WorkingDir)
			fmt.Printf("   🏷️  类别: %s\n", stat.Category)
		}
	}
}

func (a *App) displayStats(title string, stats []core.ResourceStats) {
	fmt.Println(title)
	fmt.Println("================================")
	
	if len(stats) == 0 {
		fmt.Println("没有找到进程使用数据")
		return
	}

	// Show summary based on configuration
	switch a.Config.StatisticsGranularity {
	case "simple":
		a.displaySimpleStats(stats)
	case "detailed":
		a.displayDetailedStats(stats)
	case "full":
		a.displayFullStats(stats)
	default:
		a.displayDetailedStats(stats)
	}
}

func (a *App) displaySimpleStats(stats []core.ResourceStats) {
	fmt.Printf("%-30s %10s %12s %12s\n", "进程名称", "样本数", "活跃时间", "CPU平均")
	fmt.Printf("%-30s %10s %12s %12s\n", "--------", "------", "------", "------")

	for _, stat := range stats[:10] { // Top 10
		processName := a.GetProcessNameFromStats(stat)
		if len(processName) > 30 {
			processName = processName[:27] + "..."
		}
		
		fmt.Printf("%-30s %10d %12v %10.1f%%\n",
			processName,
			stat.Samples,
			stat.ActiveTime.Round(time.Minute),
			stat.CPUAvg)
	}
}

func (a *App) displayDetailedStats(stats []core.ResourceStats) {
	fmt.Printf("%-30s %8s %8s %8s %8s %8s %12s\n", 
		"进程名称", "样本数", "活跃数", "CPU平均", "CPU最大", "内存平均", "活跃时间")
	fmt.Printf("%-30s %8s %8s %8s %8s %8s %12s\n", 
		"--------", "------", "------", "------", "------", "------", "------")

	for _, stat := range stats[:15] { // Top 15
		processName := a.GetProcessNameFromStats(stat)
		if len(processName) > 30 {
			processName = processName[:27] + "..."
		}
		
		fmt.Printf("%-30s %8d %8d %8.1f %8.1f %8.1f %12v\n",
			processName,
			stat.Samples,
			stat.ActiveSamples,
			stat.CPUAvg,
			stat.CPUMax,
			stat.MemoryAvg,
			stat.ActiveTime.Round(time.Minute))
	}
}

func (a *App) displayFullStats(stats []core.ResourceStats) {
	// Show all stats with full details
	fmt.Printf("%-30s %8s %8s %8s %8s %8s %8s %12s %12s %12s %12s\n", 
		"进程名称", "样本数", "活跃数", "CPU平均", "CPU最大", "内存平均", "内存最大", "磁盘读取", "磁盘写入", "网络发送", "网络接收")
	fmt.Printf("%-30s %8s %8s %8s %8s %8s %8s %12s %12s %12s %12s\n", 
		"--------", "------", "------", "------", "------", "------", "------", "------", "------", "------", "------")

	for _, stat := range stats {
		processName := a.GetProcessNameFromStats(stat)
		if len(processName) > 30 {
			processName = processName[:27] + "..."
		}
		
		fmt.Printf("%-30s %8d %8d %8.1f %8.1f %8.1f %8.1f %12.2f %12.2f %12.1f %12.1f\n",
			processName,
			stat.Samples,
			stat.ActiveSamples,
			stat.CPUAvg,
			stat.CPUMax,
			stat.MemoryAvg,
			stat.MemoryMax,
			stat.DiskReadAvg,
			stat.DiskWriteAvg,
			stat.NetSentAvg,
			stat.NetRecvAvg)
	}
}

func (a *App) GetProcessNameFromStats(stat core.ResourceStats) string {
	name := stat.Name

	// Apply smart categorization if enabled
	if a.Config.UseSmartCategories {
		name = stat.Category
		if name == "" {
			name = stat.Name
		}
	}

	// Add command context if enabled
	if a.Config.ShowCommands && stat.Command != "" {
		truncatedCmd := core.TruncateString(stat.Command, a.Config.MaxCommandLength)
		name = fmt.Sprintf("%s [%s]", name, truncatedCmd)
	}

	// Add working directory context if enabled
	if a.Config.ShowWorkingDirs && stat.WorkingDir != "" {
		projectName := core.ExtractProjectName(stat.WorkingDir)
		if projectName != "" {
			truncatedDir := core.TruncateString(projectName, a.Config.MaxDirLength)
			name = fmt.Sprintf("%s (%s)", name, truncatedDir)
		}
	}

	return name
}

func (a *App) cleanupOldData() {
	fmt.Printf("🧹 正在清理30天前的旧数据...\n")
	
	// Clean old data from main file
	if err := a.CleanOldData(30); err != nil {
		log.Printf("❌ 清理旧数据时出错: %v", err)
		fmt.Println("❌ 清理失败")
		return
	}
	
	// Get total records count
	totalRecords, err := a.GetTotalRecords()
	if err != nil {
		log.Printf("⚠️  获取记录数量时出错: %v", err)
		fmt.Println("✅ 清理完成")
	} else {
		fmt.Printf("✅ 清理完成！当前数据文件包含 %d 条记录\n", totalRecords)
	}
}

func (a *App) exportData() {
	fmt.Printf("📤 正在导出数据...\n")
	
	// Get all records from data file
	records, err := a.ReadResourceRecords(a.DataFile)
	if err != nil {
		log.Printf("❌ 读取数据文件时出错: %v", err)
		fmt.Println("❌ 导出失败")
		return
	}
	
	if len(records) == 0 {
		fmt.Printf("📭 暂无数据可导出\n")
		return
	}
	
	// Calculate statistics for different time periods
	todayStats, _ := a.CalculateResourceStats(24 * time.Hour)
	weekStats, _ := a.CalculateResourceStats(7 * 24 * time.Hour)
	monthStats, _ := a.CalculateResourceStats(30 * 24 * time.Hour)
	
	// Create export structure
	exportData := struct {
		Metadata struct {
			Version     string    `json:"version"`
			ExportTime  time.Time `json:"export_time"`
			DataFile    string    `json:"data_file"`
			TotalRecords int      `json:"total_records"`
		} `json:"metadata"`
		Summary struct {
			TodayProcessCount  int `json:"today_process_count"`
			WeekProcessCount  int `json:"week_process_count"`
			MonthProcessCount int `json:"month_process_count"`
		} `json:"summary"`
		Records []core.ResourceRecord `json:"records"`
		TodayStats  []core.ResourceStats `json:"today_stats"`
		WeekStats   []core.ResourceStats `json:"week_stats"`
		MonthStats  []core.ResourceStats `json:"month_stats"`
	}{
		Metadata: struct {
			Version     string    `json:"version"`
			ExportTime  time.Time `json:"export_time"`
			DataFile    string    `json:"data_file"`
			TotalRecords int      `json:"total_records"`
		}{
			Version:      Version,
			ExportTime:  time.Now(),
			DataFile:     a.DataFile,
			TotalRecords: len(records),
		},
		Summary: struct {
			TodayProcessCount  int `json:"today_process_count"`
			WeekProcessCount  int `json:"week_process_count"`
			MonthProcessCount int `json:"month_process_count"`
		}{
			TodayProcessCount:  len(todayStats),
			WeekProcessCount:  len(weekStats),
			MonthProcessCount: len(monthStats),
		},
		Records:    records,
		TodayStats:  todayStats,
		WeekStats:   weekStats,
		MonthStats:  monthStats,
	}
	
	// Generate output filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	outputFile := fmt.Sprintf("process-tracker-export-%s.json", timestamp)
	
	// Write to JSON file
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		log.Printf("❌ 生成JSON时出错: %v", err)
		fmt.Println("❌ 导出失败")
		return
	}
	
	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		log.Printf("❌ 写入文件时出错: %v", err)
		fmt.Println("❌ 导出失败")
		return
	}
	
	fmt.Printf("✅ 导出完成！\n")
	fmt.Printf("📁 输出文件: %s\n", outputFile)
	fmt.Printf("📊 导出统计:\n")
	fmt.Printf("   - 总记录数: %d\n", len(records))
	fmt.Printf("   - 今日进程: %d\n", len(todayStats))
	fmt.Printf("   - 本周进程: %d\n", len(weekStats))
	fmt.Printf("   - 本月进程: %d\n", len(monthStats))
	fmt.Printf("📏 文件大小: %.2f KB\n", float64(len(jsonData))/1024.0)
}

// startProcess starts a managed process
func (a *App) startProcess(args []string) {
	if !a.Config.ProcessControl.Enabled {
		fmt.Println("❌ 进程控制功能未启用，请检查配置文件")
		return
	}
	
	name := args[0]
	var command []string
	var workingDir string
	
	if len(args) > 1 {
		command = args[1:]
	} else {
		// If no command provided, try to find it in config
		found := false
		for _, proc := range a.Config.ProcessControl.ManagedProcesses {
			if proc.Name == name {
				command = proc.Command
				workingDir = proc.WorkingDir
				found = true
				break
			}
		}
		if !found {
			fmt.Println("❌ 未找到进程配置，请提供命令")
			return
		}
	}
	
	if err := a.InitializeProcessControl(); err != nil {
		fmt.Printf("❌ 初始化进程控制失败: %v\n", err)
		return
	}
	
	if err := a.StartProcess(name, command, workingDir); err != nil {
		fmt.Printf("❌ 启动进程失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 进程 %s 已启动\n", name)
}

// stopProcess stops a managed process
func (a *App) stopProcess(identifier string) {
	if !a.Config.ProcessControl.Enabled {
		fmt.Println("❌ 进程控制功能未启用，请检查配置文件")
		return
	}
	
	// Try to find process by name first
	proc, err := a.GetProcessByName(identifier)
	if err == nil {
		if err := a.StopProcess(proc.PID); err != nil {
			fmt.Printf("❌ 停止进程失败: %v\n", err)
			return
		}
		fmt.Printf("✅ 进程 %s (PID: %d) 已停止\n", identifier, proc.PID)
		return
	}
	
	// If not found by name, try to parse as PID
	pid, err := strconv.ParseInt(identifier, 10, 32)
	if err == nil {
		if err := a.StopProcess(int32(pid)); err != nil {
			fmt.Printf("❌ 停止进程失败: %v\n", err)
			return
		}
		fmt.Printf("✅ 进程 PID %d 已停止\n", pid)
		return
	}
	
	fmt.Printf("❌ 未找到进程: %s\n", identifier)
}

// restartProcess restarts a managed process
func (a *App) restartProcess(identifier string) {
	if !a.Config.ProcessControl.Enabled {
		fmt.Println("❌ 进程控制功能未启用，请检查配置文件")
		return
	}
	
	// Try to find process by name first
	proc, err := a.GetProcessByName(identifier)
	if err == nil {
		if err := a.RestartProcess(proc.PID); err != nil {
			fmt.Printf("❌ 重启进程失败: %v\n", err)
			return
		}
		fmt.Printf("✅ 进程 %s 已重启\n", identifier)
		return
	}
	
	// If not found by name, try to parse as PID
	pid, err := strconv.ParseInt(identifier, 10, 32)
	if err == nil {
		if err := a.RestartProcess(int32(pid)); err != nil {
			fmt.Printf("❌ 重启进程失败: %v\n", err)
			return
		}
		fmt.Printf("✅ 进程 PID %d 已重启\n", pid)
		return
	}
	
	fmt.Printf("❌ 未找到进程: %s\n", identifier)
}

// listManagedProcesses lists all managed processes
func (a *App) listManagedProcesses() {
	if !a.Config.ProcessControl.Enabled {
		fmt.Println("❌ 进程控制功能未启用，请检查配置文件")
		return
	}
	
	processes := a.GetManagedProcesses()
	if len(processes) == 0 {
		fmt.Println("📭 当前没有托管进程")
		return
	}
	
	fmt.Println("📋 托管进程列表")
	fmt.Println("================================")
	fmt.Printf("%-8s %-20s %-10s %-10s %-10s %-10s\n", "PID", "名称", "状态", "重启次数", "运行时间", "退出码")
	fmt.Printf("%-8s %-20s %-10s %-10s %-10s %-10s\n", "---", "----", "----", "----", "----", "----")
	
	for _, proc := range processes {
		uptime := time.Since(proc.StartTime).Round(time.Second)
		status := string(proc.Status)
		if len(status) > 10 {
			status = status[:7] + "..."
		}
		
		fmt.Printf("%-8d %-20s %-10s %-10d %-10v %-10d\n",
			proc.PID,
			proc.Name,
			status,
			proc.Restarts,
			uptime,
			proc.ExitCode)
	}
	
	// Show process controller statistics
	if a.ProcessController != nil {
		stats := a.ProcessController.GetProcessStats()
		fmt.Printf("\n📊 进程控制器统计:\n")
		fmt.Printf("   总进程数: %d\n", stats.TotalProcesses)
		fmt.Printf("   运行中: %d\n", stats.Running)
		fmt.Printf("   已停止: %d\n", stats.Stopped)
		fmt.Printf("   失败: %d\n", stats.Failed)
		fmt.Printf("   重启中: %d\n", stats.Restarting)
	}
}

// addProcessToQuota adds a process to a resource quota
func (a *App) addProcessToQuota(quotaName, processIdentifier string) {
	if !a.Config.ResourceQuota.Enabled {
		fmt.Println("❌ 资源配额功能未启用，请检查配置文件")
		return
	}
	
	// Try to find process by name first
	var pid int32
	proc, err := a.GetProcessByName(processIdentifier)
	if err == nil {
		pid = proc.PID
	} else {
		// If not found by name, try to parse as PID
		parsedPid, err := strconv.ParseInt(processIdentifier, 10, 32)
		if err != nil {
			fmt.Printf("❌ 未找到进程: %s\n", processIdentifier)
			return
		}
		pid = int32(parsedPid)
	}
	
	// Add process to quota
	if err := a.AddProcessToQuota(quotaName, pid); err != nil {
		fmt.Printf("❌ 添加进程到配额失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 进程 %s (PID: %d) 已添加到配额 %s\n", processIdentifier, pid, quotaName)
}

// removeProcessFromQuota removes a process from a resource quota
func (a *App) removeProcessFromQuota(quotaName, processIdentifier string) {
	if !a.Config.ResourceQuota.Enabled {
		fmt.Println("❌ 资源配额功能未启用，请检查配置文件")
		return
	}
	
	// Try to find process by name first
	var pid int32
	proc, err := a.GetProcessByName(processIdentifier)
	if err == nil {
		pid = proc.PID
	} else {
		// If not found by name, try to parse as PID
		parsedPid, err := strconv.ParseInt(processIdentifier, 10, 32)
		if err != nil {
			fmt.Printf("❌ 未找到进程: %s\n", processIdentifier)
			return
		}
		pid = int32(parsedPid)
	}
	
	// Remove process from quota
	if err := a.RemoveProcessFromQuota(quotaName, pid); err != nil {
		fmt.Printf("❌ 从配额移除进程失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 进程 %s (PID: %d) 已从配额 %s 移除\n", processIdentifier, pid, quotaName)
}

// listQuotas lists all resource quotas and their processes
func (a *App) listQuotas() {
	if !a.Config.ResourceQuota.Enabled {
		fmt.Println("❌ 资源配额功能未启用，请检查配置文件")
		return
	}
	
	quotas := a.GetAllQuotas()
	if len(quotas) == 0 {
		fmt.Println("📭 当前没有配置资源配额")
		return
	}
	
	fmt.Println("📋 资源配额列表")
	fmt.Println("================================")
	
	for _, quota := range quotas {
		fmt.Printf("配额名称: %s\n", quota.Name)
		fmt.Printf("状态: %s\n", func() string {
			if quota.Active {
				return "🟢 活跃"
			}
			return "🔴 非活跃"
		}())
		fmt.Printf("CPU限制: %.1f%%\n", quota.CPULimit)
		fmt.Printf("内存限制: %d MB\n", quota.MemoryLimitMB)
		fmt.Printf("线程限制: %d\n", quota.ThreadLimit)
		fmt.Printf("时间限制: %v\n", quota.TimeLimit)
		fmt.Printf("违规次数: %d\n", quota.Violations)
		fmt.Printf("操作: %s\n", quota.Action)
		
		if len(quota.Processes) > 0 {
			fmt.Printf("关联进程 (%d):\n", len(quota.Processes))
			for _, pid := range quota.Processes {
				// Get process name
				if p, err := process.NewProcess(pid); err == nil {
					if name, err := p.Name(); err == nil {
						fmt.Printf("  - %s (PID: %d)\n", name, pid)
					} else {
						fmt.Printf("  - PID: %d\n", pid)
					}
				} else {
					fmt.Printf("  - PID: %d (进程不存在)\n", pid)
				}
			}
		} else {
			fmt.Println("关联进程: 无")
		}
		fmt.Println("================================")
	}
	
	// Show quota statistics
	stats := a.GetQuotaStats()
	fmt.Printf("📊 配额统计: 总计 %d 个配额，%d 个活跃，%d 个进程，%d 次违规\n",
		stats.TotalQuotas, stats.ActiveQuotas, stats.TotalProcesses, stats.TotalViolations)
}

// listDiscoveredProcesses lists all automatically discovered processes
func (a *App) listDiscoveredProcesses() {
	if !a.Config.ProcessDiscovery.Enabled {
		fmt.Println("❌ 进程发现功能未启用，请检查配置文件")
		return
	}
	
	processes := a.GetDiscoveredProcesses()
	if len(processes) == 0 {
		fmt.Println("🔍 未发现任何进程")
		return
	}
	
	fmt.Printf("🔍 发现的进程 (%d 个):\n", len(processes))
	fmt.Println("==========================================")
	
	for _, proc := range processes {
		fmt.Printf("📋 %s (PID: %d)\n", proc.Name, proc.PID)
		fmt.Printf("   组: %s\n", proc.GroupName)
		fmt.Printf("   命令行: %s\n", proc.Cmdline)
		fmt.Printf("   状态: %s\n", proc.Status)
		fmt.Printf("   发现时间: %s\n", proc.Discovered.Format("2006-01-02 15:04:05"))
		fmt.Printf("   最后见: %s\n", proc.LastSeen.Format("2006-01-02 15:04:05"))
		if proc.CPUUsed > 0 {
			fmt.Printf("   CPU使用: %.2f%%\n", proc.CPUUsed)
		}
		if proc.MemoryUsed > 0 {
			fmt.Printf("   内存使用: %d MB\n", proc.MemoryUsed)
		}
		if len(proc.Tags) > 0 {
			fmt.Printf("   标签: %v\n", proc.Tags)
		}
		fmt.Println("---")
	}
}

// listProcessGroups lists all process groups
func (a *App) listProcessGroups() {
	if !a.Config.ProcessDiscovery.Enabled {
		fmt.Println("❌ 进程发现功能未启用，请检查配置文件")
		return
	}
	
	groups := a.GetProcessGroups()
	if len(groups) == 0 {
		fmt.Println("📋 未定义任何进程组")
		return
	}
	
	fmt.Printf("📋 进程组 (%d 个):\n", len(groups))
	fmt.Println("=========================")
	
	for name, group := range groups {
		fmt.Printf("🏷️  %s\n", name)
		fmt.Printf("   描述: %s\n", group.Description)
		fmt.Printf("   模式: %s\n", group.Pattern)
		fmt.Printf("   自动管理: %t\n", group.AutoManage)
		if group.QuotaName != "" {
			fmt.Printf("   配额名称: %s\n", group.QuotaName)
		}
		if len(group.Tags) > 0 {
			fmt.Printf("   标签: %v\n", group.Tags)
		}
		if len(group.Processes) > 0 {
			fmt.Printf("   进程数: %d\n", len(group.Processes))
			fmt.Printf("   进程: %v\n", group.Processes)
		}
		fmt.Println("---")
	}
}

// addCustomGroup adds a custom process group
func (a *App) addCustomGroup(name, pattern string, autoManage bool, quotaName string) {
	if !a.Config.ProcessDiscovery.Enabled {
		fmt.Println("❌ 进程发现功能未启用，请检查配置文件")
		return
	}
	
	if err := a.AddCustomGroup(name, pattern, autoManage, quotaName); err != nil {
		fmt.Printf("❌ 添加自定义组失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 成功添加自定义进程组: %s\n", name)
	fmt.Printf("   模式: %s\n", pattern)
	fmt.Printf("   自动管理: %t\n", autoManage)
	if quotaName != "" {
		fmt.Printf("   配额名称: %s\n", quotaName)
	}
}

// removeCustomGroup removes a custom process group
func (a *App) removeCustomGroup(name string) {
	if !a.Config.ProcessDiscovery.Enabled {
		fmt.Println("❌ 进程发现功能未启用，请检查配置文件")
		return
	}
	
	if err := a.RemoveCustomGroup(name); err != nil {
		fmt.Printf("❌ 移除自定义组失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 成功移除自定义进程组: %s\n", name)
}

// showDiscoveryStats shows process discovery statistics
func (a *App) showDiscoveryStats() {
	if !a.Config.ProcessDiscovery.Enabled {
		fmt.Println("❌ 进程发现功能未启用，请检查配置文件")
		return
	}
	
	stats := a.GetDiscoveryStats()
	fmt.Printf("🔍 进程发现统计:\n")
	fmt.Println("===================")
	fmt.Printf("📊 总发现进程: %d\n", stats.TotalDiscovered)
	fmt.Printf("📋 进程组数: %d\n", stats.TotalGroups)
	fmt.Printf("🤖 自动管理: %d\n", stats.AutoManaged)
	
	if len(stats.GroupCounts) > 0 {
		fmt.Println("\n📈 各组统计:")
		for group, count := range stats.GroupCounts {
			fmt.Printf("   %s: %d\n", group, count)
		}
	}
}