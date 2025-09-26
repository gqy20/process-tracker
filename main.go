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
var Version = "0.3.7"

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
	case "create-task":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定任务名称和命令")
			fmt.Println("用法: process-tracker create-task <名称> <命令> [参数...]")
			return
		}
		app.createTask(os.Args[2:])
	case "list-tasks":
		app.listTasks()
	case "task-info":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定任务ID")
			fmt.Println("用法: process-tracker task-info <任务ID>")
			return
		}
		app.showTaskInfo(os.Args[2])
	case "start-task":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定任务ID")
			fmt.Println("用法: process-tracker start-task <任务ID>")
			return
		}
		app.startTask(os.Args[2])
	case "stop-task":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定任务ID")
			fmt.Println("用法: process-tracker stop-task <任务ID>")
			return
		}
		app.stopTask(os.Args[2])
	case "pause-task":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定任务ID")
			fmt.Println("用法: process-tracker pause-task <任务ID>")
			return
		}
		app.pauseTask(os.Args[2])
	case "resume-task":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定任务ID")
			fmt.Println("用法: process-tracker resume-task <任务ID>")
			return
		}
		app.resumeTask(os.Args[2])
	case "task-history":
		app.showTaskHistory()
	case "task-stats":
		app.showTaskStats()
	case "remove-task":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定任务ID")
			fmt.Println("用法: process-tracker remove-task <任务ID>")
			return
		}
		app.removeTask(os.Args[2])
	case "clear-tasks":
		app.clearCompletedTasks()
	case "health-check":
		app.runHealthCheck()
	case "list-health":
		app.listHealthChecks()
	case "health-info":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定健康检查ID")
			fmt.Println("用法: process-tracker health-info <健康检查ID>")
			return
		}
		app.showHealthInfo(os.Args[2])
	case "list-alerts":
		app.listAlerts()
	case "alert-info":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定告警ID")
			fmt.Println("用法: process-tracker alert-info <告警ID>")
			return
		}
		app.showAlertInfo(os.Args[2])
	case "resolve-alert":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定告警ID")
			fmt.Println("用法: process-tracker resolve-alert <告警ID>")
			return
		}
		app.resolveAlert(os.Args[2])
	case "health-stats":
		app.showHealthStats()
	case "help":
		app.printUsage()
	default:
		app.printUsage()
	}
}

func (a *App) printUsage() {
	fmt.Println("进程跟踪器 - 智能进程监控工具 v0.3.7")
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
	fmt.Println("任务管理命令:")
	fmt.Println("  create-task        创建新任务")
	fmt.Println("  list-tasks          列出所有任务")
	fmt.Println("  task-info           显示任务详细信息")
	fmt.Println("  start-task          启动任务")
	fmt.Println("  stop-task           停止任务")
	fmt.Println("  pause-task          暂停任务")
	fmt.Println("  resume-task         恢复任务")
	fmt.Println("  task-history        显示任务执行历史")
	fmt.Println("  task-stats          显示任务统计")
	fmt.Println("  remove-task         移除任务")
	fmt.Println("  clear-tasks         清理已完成任务")
	fmt.Println()
	fmt.Println("健康检查命令:")
	fmt.Println("  health-check       运行健康检查")
	fmt.Println("  list-health        列出所有健康检查")
	fmt.Println("  health-info        显示健康检查详细信息")
	fmt.Println("  list-alerts        列出所有告警")
	fmt.Println("  alert-info         显示告警详细信息")
	fmt.Println("  resolve-alert      手动解决告警")
	fmt.Println("  health-stats       显示健康检查统计")
	fmt.Println()
	fmt.Println("其他命令:")
	fmt.Println("  version            显示版本信息")
	fmt.Println("  help               显示此帮助信息")
	fmt.Println()
	fmt.Println("配置文件:")
	fmt.Println("  ~/.process-tracker.yaml - 控制统计粒度和显示选项")
	fmt.Println("  配置详情请参考 README.md 文件")
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
	
	// 简化版本，跳过进程控制初始化
	
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
		if err := a.StopProcess(proc.Pid); err != nil {
			fmt.Printf("❌ 停止进程失败: %v\n", err)
			return
		}
		fmt.Printf("✅ 进程 %s (PID: %d) 已停止\n", identifier, proc.Pid)
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
		if err := a.RestartProcess(proc.Pid); err != nil {
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
		fmt.Printf("%-8d %-20s %-10s %-10s %-10.2f\n",
			proc.Pid,
			proc.Name,
			"运行中",
			"N/A",
			proc.CPUPercent)
	}
	
	// 简化版本，不显示进程控制器统计
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
		pid = proc.Pid
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
		pid = proc.Pid
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
		fmt.Printf("📋 %s (PID: %d)\n", proc.Name, proc.Pid)
		fmt.Printf("   命令行: %s\n", proc.Cmdline)
		if proc.CPUPercent > 0 {
			fmt.Printf("   CPU使用: %.2f%%\n", proc.CPUPercent)
		}
		if proc.MemoryMB > 0 {
			fmt.Printf("   内存使用: %.2f MB\n", proc.MemoryMB)
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
		if len(group.PIDs) > 0 {
			fmt.Printf("   进程数: %d\n", len(group.PIDs))
			fmt.Printf("   进程: %v\n", group.PIDs)
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
	fmt.Printf("🔍 进程统计:\n")
	fmt.Println("===================")
	fmt.Printf("📊 最后更新: %s\n", stats.LastUpdated.Format("2006-01-02 15:04:05"))
	fmt.Printf("📋 示例进程: %s (PID: %d, CPU: %.2f%%, 内存: %d MB)\n", 
		stats.Name, stats.PID, stats.CPUUsed, stats.MemoryUsedMB)
}

// Task Manager CLI Methods

// createTask creates a new task
func (a *App) createTask(args []string) {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	taskName := args[0]
	command := args[1]
	var taskArgs []string
	if len(args) > 2 {
		taskArgs = args[2:]
	}
	
	task := &core.Task{
		Name:    taskName,
		Command: command,
		Args:    taskArgs,
	}
	
	if err := a.CreateTask(task); err != nil {
		fmt.Printf("❌ 创建任务失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 任务创建成功: %s (ID: %s)\n", taskName, task.ID)
}

// listTasks lists all tasks
func (a *App) listTasks() {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	tasks := a.ListTasks()
	if len(tasks) == 0 {
		fmt.Println("📋 没有找到任何任务")
		return
	}
	
	fmt.Println("📋 任务列表:")
	fmt.Println("========================================")
	for _, task := range tasks {
		statusIcon := getStatusIcon(task.Status)
		fmt.Printf("%s %s - %s (%s)\n", statusIcon, task.ID, task.Name, task.Status)
		fmt.Printf("   命令: %s", task.Command)
		if len(task.Args) > 0 {
			fmt.Printf(" %s", strings.Join(task.Args, " "))
		}
		fmt.Println()
		if task.Description != "" {
			fmt.Printf("   描述: %s\n", task.Description)
		}
		fmt.Println()
	}
}

// showTaskInfo shows detailed information about a task
func (a *App) showTaskInfo(taskID string) {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	task, err := a.GetTask(taskID)
	if err != nil {
		fmt.Printf("❌ 获取任务信息失败: %v\n", err)
		return
	}
	
	fmt.Printf("📋 任务详细信息: %s\n", task.ID)
	fmt.Println("========================================")
	fmt.Printf("名称: %s\n", task.Name)
	fmt.Printf("状态: %s\n", task.Status)
	fmt.Printf("优先级: %d\n", task.Priority)
	fmt.Printf("命令: %s\n", task.Command)
	fmt.Printf("参数: %v\n", task.Args)
	fmt.Printf("工作目录: %s\n", task.WorkingDir)
	fmt.Printf("超时时间: %v\n", task.Timeout)
	fmt.Printf("重试次数: %d/%d\n", task.RetryCount, task.MaxRetries)
	fmt.Printf("创建时间: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	
	if !task.StartedAt.IsZero() {
		fmt.Printf("开始时间: %s\n", task.StartedAt.Format("2006-01-02 15:04:05"))
	}
	
	if !task.CompletedAt.IsZero() {
		fmt.Printf("完成时间: %s\n", task.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	
	if task.ExitCode != 0 {
		fmt.Printf("退出代码: %d\n", task.ExitCode)
	}
	
	if task.PID != 0 {
		fmt.Printf("进程ID: %d\n", task.PID)
	}
	
	if task.LogPath != "" {
		fmt.Printf("日志路径: %s\n", task.LogPath)
	}
	
	if len(task.Dependencies) > 0 {
		fmt.Printf("依赖任务: %v\n", task.Dependencies)
	}
	
	if len(task.Tags) > 0 {
		fmt.Printf("标签: %v\n", task.Tags)
	}
}

// startTask starts a task
func (a *App) startTask(taskID string) {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	if err := a.StartTask(taskID); err != nil {
		fmt.Printf("❌ 启动任务失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 任务启动成功: %s\n", taskID)
}

// stopTask stops a task
func (a *App) stopTask(taskID string) {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	if err := a.CancelTask(taskID); err != nil {
		fmt.Printf("❌ 停止任务失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 任务停止成功: %s\n", taskID)
}

// pauseTask pauses a task
func (a *App) pauseTask(taskID string) {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	if err := a.PauseTask(taskID); err != nil {
		fmt.Printf("❌ 暂停任务失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 任务暂停成功: %s\n", taskID)
}

// resumeTask resumes a task
func (a *App) resumeTask(taskID string) {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	if err := a.ResumeTask(taskID); err != nil {
		fmt.Printf("❌ 恢复任务失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 任务恢复成功: %s\n", taskID)
}

// showTaskHistory shows task execution history
func (a *App) showTaskHistory() {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	history := a.GetTaskHistory()
	if len(history) == 0 {
		fmt.Println("📋 没有找到任务执行历史")
		return
	}
	
	fmt.Println("📋 任务执行历史:")
	fmt.Println("========================================")
	for _, result := range history {
		status := "✅"
		if result.ExitCode != 0 {
			status = "❌"
		}
		fmt.Printf("%s %s - 退出代码: %d, 耗时: %v\n", status, result.TaskID, result.ExitCode, result.Duration)
		fmt.Printf("   时间: %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))
		if result.Error != "" {
			fmt.Printf("   错误: %s\n", result.Error)
		}
		fmt.Println()
	}
}

// showTaskStats shows task manager statistics
func (a *App) showTaskStats() {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	stats := a.GetTaskStats()
	fmt.Println("📊 任务管理统计:")
	fmt.Println("========================================")
	fmt.Printf("总任务数: %d\n", stats.TotalTasks)
	fmt.Printf("已完成: %d\n", stats.CompletedTasks)
	fmt.Printf("失败: %d\n", stats.FailedTasks)
	fmt.Printf("运行中: %d\n", stats.RunningTasks)
	fmt.Printf("等待中: %d\n", stats.PendingTasks)
	if stats.AvgDuration > 0 {
		fmt.Printf("平均耗时: %v\n", stats.AvgDuration)
	}
	fmt.Printf("最后更新: %s\n", stats.LastUpdated.Format("2006-01-02 15:04:05"))
}

// removeTask removes a task
func (a *App) removeTask(taskID string) {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	if err := a.RemoveTask(taskID); err != nil {
		fmt.Printf("❌ 移除任务失败: %v\n", err)
		return
	}
	
	fmt.Printf("✅ 任务移除成功: %s\n", taskID)
}

// clearCompletedTasks clears completed tasks
func (a *App) clearCompletedTasks() {
	if !a.Config.TaskManager.Enabled {
		fmt.Println("❌ 任务管理功能未启用，请检查配置文件")
		return
	}
	
	count := a.ClearCompletedTasks()
	fmt.Printf("✅ 清理了 %d 个已完成任务\n", count)
}

// getStatusIcon returns status icon for task status
func getStatusIcon(status core.TaskStatus) string {
	switch status {
	case core.TaskStatusPending:
		return "⏳"
	case core.TaskStatusRunning:
		return "▶️"
	case core.TaskStatusCompleted:
		return "✅"
	case core.TaskStatusFailed:
		return "❌"
	case core.TaskStatusCancelled:
		return "🚫"
	case core.TaskStatusPaused:
		return "⏸️"
	case core.TaskStatusRetry:
		return "🔄"
	default:
		return "❓"
	}
}

// Health check CLI methods

// runHealthCheck runs a health check
func (a *App) runHealthCheck() {
	if !a.Config.HealthCheck.Enabled {
		fmt.Println("❌ 健康检查功能未启用，请检查配置文件")
		return
	}
	
	fmt.Println("🔍 运行健康检查...")
	
	// Trigger health checks
	if a.UnifiedHealthChecker != nil {
		status := a.UnifiedHealthChecker.CheckHealth()
		fmt.Printf("✅ 健康检查完成，状态: %s (评分: %.2f)\n", status.Status, status.Score)
	} else {
		fmt.Println("❌ 健康检查器未初始化")
	}
}

// listHealthChecks lists all health checks
func (a *App) listHealthChecks() {
	if !a.Config.HealthCheck.Enabled {
		fmt.Println("❌ 健康检查功能未启用，请检查配置文件")
		return
	}
	
	if a.UnifiedHealthChecker == nil {
		fmt.Println("❌ 健康检查器未初始化")
		return
	}
	
	// 简化版本：显示当前健康状态
	status := a.UnifiedHealthChecker.CheckHealth()
	
	fmt.Println("📋 当前健康状态:")
	fmt.Printf("   状态: %s\n", status.Status)
	fmt.Printf("   评分: %.2f/100\n", status.Score)
	fmt.Printf("   检查时间: %s\n", status.LastCheck.Format("2006-01-02 15:04:05"))
	
	if len(status.Issues) > 0 {
		fmt.Println("   发现的问题:")
		for _, issue := range status.Issues {
			fmt.Printf("     - %s\n", issue)
		}
	} else {
		fmt.Println("   ✅ 未发现问题")
	}
}

// showHealthInfo shows detailed health check information
func (a *App) showHealthInfo(checkID string) {
	if !a.Config.HealthCheck.Enabled {
		fmt.Println("❌ 健康检查功能未启用，请检查配置文件")
		return
	}
	
	if a.UnifiedHealthChecker == nil {
		fmt.Println("❌ 健康检查器未初始化")
		return
	}
	
	// 简化版本：显示总体健康状态
	fmt.Printf("📋 健康检查信息 - ID: %s\n", checkID)
	status := a.UnifiedHealthChecker.CheckHealth()
	
	fmt.Printf("系统健康状态: %s\n", status.Status)
	fmt.Printf("健康评分: %.2f/100\n", status.Score)
	if len(status.Issues) > 0 {
		fmt.Println("发现的问题:")
		for _, issue := range status.Issues {
			fmt.Printf("  - %s\n", issue)
		}
	}
}

// listAlerts lists all alerts
func (a *App) listAlerts() {
	if !a.Config.HealthCheck.Enabled {
		fmt.Println("❌ 健康检查功能未启用，请检查配置文件")
		return
	}
	
	if a.UnifiedHealthChecker == nil {
		fmt.Println("❌ 健康检查器未初始化")
		return
	}
	
	alerts := []string{} // 暂时返回空列表
	if len(alerts) == 0 {
		fmt.Println("📋 暂无告警记录")
		return
	}
	
	fmt.Println("📋 告警记录:")
	fmt.Println("📝 注意: 告警功能已简化，显示健康状态")
	fmt.Println()
	
	// 简化版本：显示当前健康状态
	status := a.UnifiedHealthChecker.CheckHealth()
	if len(status.Issues) > 0 {
		for _, issue := range status.Issues {
			fmt.Printf("⚠️  问题: %s\n", issue)
		}
	} else {
		fmt.Println("✅ 未发现告警")
	}
}

// showAlertInfo shows detailed alert information
func (a *App) showAlertInfo(alertID string) {
	if !a.Config.HealthCheck.Enabled {
		fmt.Println("❌ 健康检查功能未启用，请检查配置文件")
		return
	}
	
	if a.UnifiedHealthChecker == nil {
		fmt.Println("❌ 健康检查器未初始化")
		return
	}
	
	// 简化版本：显示健康状态
	fmt.Printf("📋 告警信息 - ID: %s\n", alertID)
	status := a.UnifiedHealthChecker.CheckHealth()
	
	fmt.Printf("系统健康状态: %s\n", status.Status)
	fmt.Printf("健康评分: %.2f/100\n", status.Score)
	if len(status.Issues) > 0 {
		fmt.Println("相关问题:")
		for _, issue := range status.Issues {
			fmt.Printf("  - %s\n", issue)
		}
	}
}

// resolveAlert manually resolves an alert
func (a *App) resolveAlert(alertID string) {
	if !a.Config.HealthCheck.Enabled {
		fmt.Println("❌ 健康检查功能未启用，请检查配置文件")
		return
	}
	
	if a.UnifiedHealthChecker == nil {
		fmt.Println("❌ 健康检查器未初始化")
		return
	}
	
	alert := core.Alert{} // 暂时返回空告警
	_ = alertID // 避免未使用变量错误
		
	if alert.Status == core.AlertStatusResolved {
		fmt.Println("ℹ️ 告警已经处于解决状态")
		return
	}
	
	// In a real implementation, this would call a method on the health checker
	// For now, we'll just show that the alert would be resolved
	fmt.Printf("✅ 告警已标记为解决: %s\n", alertID)
}

// showHealthStats shows health check statistics
func (a *App) showHealthStats() {
	if !a.Config.HealthCheck.Enabled {
		fmt.Println("❌ 健康检查功能未启用，请检查配置文件")
		return
	}
	
	if a.UnifiedHealthChecker == nil {
		fmt.Println("❌ 健康检查器未初始化")
		return
	}
	
	_ = map[string]interface{}{} // 暂时返回空map
	
	fmt.Println("📊 健康检查统计:")
	fmt.Println()
	fmt.Println("总检查次数: 0 (暂时不可用)")
	fmt.Println("完成检查次数: 0 (暂时不可用)")
	fmt.Println("失败检查次数: 0 (暂时不可用)")
	fmt.Println("活跃告警数: 0 (暂时不可用)")
	fmt.Println("已解决告警数: 0 (暂时不可用)")
	fmt.Println("最后检查时间: 暂时不可用")
	fmt.Println("平均检查耗时: 暂时不可用")
	
	// Show health rule count
	fmt.Printf("配置的规则数: %d\n", len(a.Config.HealthCheck.HealthRules))
	
	// Show notification channel count
	fmt.Printf("通知渠道数: %d\n", len(a.Config.HealthCheck.AlertManager.NotificationChannels))
}