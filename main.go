package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/yourusername/process-tracker/cmd"
	"github.com/yourusername/process-tracker/core"
)

// Version is set during build
var Version = "0.4.1"

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
	// 简化的命令行参数处理
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	// 加载配置
	config, err := cmd.LoadConfig(os.ExpandEnv("$HOME/.process-tracker/config.yaml"))
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
		config = core.GetDefaultConfig()
	}

	// 初始化daemon管理器
	dataDir := os.ExpandEnv("$HOME/.process-tracker")
	daemon := core.NewDaemonManager(dataDir)

	// 处理5个核心命令
	switch command {
	case "start":
		handleStart(daemon, config)
	case "stop":
		handleStop(daemon)
	case "status":
		handleStatus(daemon)
	case "stats":
		handleStats(config)
	case "web":
		handleWeb(config)
	case "run":
		handleRun(config)
	case "task":
		handleTask(config)
	case "help", "-h":
		printUsage()
	case "version", "-v":
		fmt.Printf("Process Tracker v%s\n", Version)
	default:
		fmt.Printf("❌ 未知命令: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

// printUsage prints simplified usage information
func printUsage() {
	fmt.Printf("Process Tracker v%s\n\n", Version)
	fmt.Printf("用法:\n")
	fmt.Printf("  process-tracker <command> [options]\n\n")
	fmt.Printf("核心命令:\n")
	fmt.Printf("  start   启动监控\n")
	fmt.Printf("  stop    停止监控\n")
	fmt.Printf("  status  状态\n")
	fmt.Printf("  stats   统计\n")
	fmt.Printf("  web     Web界面\n\n")
	fmt.Printf("任务管理:\n")
	fmt.Printf("  run     运行任务\n")
	fmt.Printf("  task    任务管理\n\n")
	fmt.Printf("帮助:\n")
	fmt.Printf("  help    帮助\n")
	fmt.Printf("  version 版本\n\n")
	fmt.Printf("选项:\n")
	fmt.Printf("  -i N    间隔(秒)\n")
	fmt.Printf("  -w      启动Web\n")
	fmt.Printf("  -p N    端口\n")
	fmt.Printf("  -d      今日统计\n")
	fmt.Printf("  -w      本周统计\n")
	fmt.Printf("  -m      本月统计\n\n")
	fmt.Printf("示例:\n")
	fmt.Printf("  process-tracker start\n")
	fmt.Printf("  process-tracker run 'sleep 60'\n")
	fmt.Printf("  process-tracker task list\n")
	fmt.Printf("  process-tracker task stop 1\n")
	fmt.Printf("  process-tracker stats -w\n")
	fmt.Printf("  process-tracker web -p 9090\n")
}

// handleStart handles the start command
func handleStart(daemon *core.DaemonManager, config core.Config) {
	// Check if already running
	if running, pid, _ := daemon.IsRunning(); running {
		fmt.Printf("❌ 进程已在运行 (PID: %d)\n", pid)
		fmt.Println("💡 使用 'process-tracker stop' 停止")
		os.Exit(1)
	}

	// Write PID file
	if err := daemon.WritePID(); err != nil {
		log.Printf("Warning: Failed to write PID file: %v", err)
	}

	// Parse flags for start command
	interval := 5 * time.Second
	webEnabled := false
	webPort := config.Web.Port

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-i":
			if i+1 < len(os.Args) {
				if sec, err := strconv.Atoi(os.Args[i+1]); err == nil && sec > 0 {
					interval = time.Duration(sec) * time.Second
					i++
				}
			}
		case "-w":
			webEnabled = true
		case "-p":
			if i+1 < len(os.Args) {
				webPort = os.Args[i+1]
				i++
			}
		}
	}

	// Create app
	dataFile := os.ExpandEnv("$HOME/.process-tracker/process-tracker.log")
	app := NewApp(dataFile, interval, config)
	monitorCmd := cmd.NewMonitoringCommands(app.App)

	// Start web server if enabled
	if webEnabled || config.Web.Enabled {
		port := webPort
		webServer := cmd.NewWebServer(app.App, config.Web.Host, port)
		go func() {
			if err := webServer.Start(); err != nil {
				log.Printf("Web服务器错误: %v", err)
			}
		}()
		fmt.Printf("🚀 启动进程监控 (间隔: %v, Web: http://%s:%s)\n", interval, config.Web.Host, webPort)
	} else {
		fmt.Printf("🚀 启动进程监控 (间隔: %v)\n", interval)
	}

	if err := monitorCmd.StartMonitoring(); err != nil {
		daemon.RemovePID()
		log.Fatalf("启动监控失败: %v", err)
	}
}

// handleStats handles the stats command
func handleStats(config core.Config) {
	// Default to today's stats
	period := 24 * time.Hour
	title := "今日统计"

	// Parse stats flags
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-d":
			period = 24 * time.Hour
			title = "今日统计"
		case "-w":
			period = 7 * 24 * time.Hour
			title = "本周统计"
		case "-m":
			period = 30 * 24 * time.Hour
			title = "本月统计"
		}
	}

	// Create app and show stats
	dataFile := os.ExpandEnv("$HOME/.process-tracker/process-tracker.log")
	app := NewApp(dataFile, 5*time.Second, config)
	monitorCmd := cmd.NewMonitoringCommands(app.App)

	opts := cmd.StatsOptions{
		Granularity: "simple",
		ShowSummary: true,
	}

	if err := monitorCmd.ShowStats(title, period, opts); err != nil {
		log.Fatalf("显示统计失败: %v", err)
	}
}

// handleWeb handles the web command
func handleWeb(config core.Config) {
	// Parse web flags
	host := config.Web.Host
	port := config.Web.Port

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-p":
			if i+1 < len(os.Args) {
				port = os.Args[i+1]
				i++
			}
		case "-h":
			if i+1 < len(os.Args) {
				host = os.Args[i+1]
				i++
			}
		}
	}

	// Create app
	dataFile := os.ExpandEnv("$HOME/.process-tracker/process-tracker.log")
	app := NewApp(dataFile, 5*time.Second, config)

	if err := app.Initialize(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// Start monitoring in background
	monitorCmd := cmd.NewMonitoringCommands(app.App)
	go func() {
		if err := monitorCmd.StartMonitoring(); err != nil {
			log.Printf("监控启动失败: %v", err)
		}
	}()

	// Start web server
	webServer := cmd.NewWebServer(app.App, host, port)
	fmt.Printf("🌐 启动Web界面: http://%s:%s\n", host, port)
	if err := webServer.Start(); err != nil {
		log.Fatalf("Web服务器启动失败: %v", err)
	}
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

// handleRun handles the run command - create and start a task
func handleRun(config core.Config) {
	if len(os.Args) < 3 {
		fmt.Println("❌ 用法: process-tracker run '<command>' [name]")
		fmt.Println("示例: process-tracker run 'sleep 60'")
		fmt.Println("      process-tracker run 'make build' '编译项目'")
		os.Exit(1)
	}

	command := os.Args[2]
	taskName := command
	if len(os.Args) > 3 {
		taskName = os.Args[3]
	}

	// Create app
	dataFile := os.ExpandEnv("$HOME/.process-tracker/process-tracker.log")
	app := NewApp(dataFile, 5*time.Second, config)

	if err := app.Initialize(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// Create and start task
	task, err := app.CreateTask(taskName, command, 1)
	if err != nil {
		log.Fatalf("创建任务失败: %v", err)
	}

	fmt.Printf("✅ 任务已创建: %s (ID: %d)\n", task.Name, task.ID)

	if err := app.StartTask(task.ID); err != nil {
		log.Fatalf("启动任务失败: %v", err)
	}

	fmt.Printf("🚀 任务已启动: %s (PID: %d)\n", task.Name, task.RootPID)
	fmt.Printf("💡 使用 'process-tracker task list' 查看任务状态\n")
	fmt.Printf("💡 使用 'process-tracker task stop %d' 停止任务\n", task.ID)
}

// handleTask handles the task command - task management
func handleTask(config core.Config) {
	if len(os.Args) < 3 {
		fmt.Println("❌ 用法: process-tracker task <action> [args]")
		fmt.Println("")
		fmt.Println("操作:")
		fmt.Println("  list           列出所有任务")
		fmt.Println("  running        列出运行中的任务")
		fmt.Println("  stop <id>      停止指定任务")
		fmt.Println("  delete <id>    删除指定任务")
		fmt.Println("  show <id>      显示任务详情")
		fmt.Println("")
		fmt.Println("示例:")
		fmt.Println("  process-tracker task list")
		fmt.Println("  process-tracker task stop 1")
		fmt.Println("  process-tracker task delete 1")
		os.Exit(1)
	}

	action := os.Args[2]

	// Create app
	dataFile := os.ExpandEnv("$HOME/.process-tracker/process-tracker.log")
	app := NewApp(dataFile, 5*time.Second, config)

	if err := app.Initialize(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	switch action {
	case "list":
		handleTaskList(app)
	case "running":
		handleTaskList(app, core.StatusRunning)
	case "stop":
		if len(os.Args) < 4 {
			fmt.Println("❌ 用法: process-tracker task stop <id>")
			os.Exit(1)
		}
		taskID, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Println("❌ 任务ID必须是数字")
			os.Exit(1)
		}
		handleTaskStop(app, taskID)
	case "delete":
		if len(os.Args) < 4 {
			fmt.Println("❌ 用法: process-tracker task delete <id>")
			os.Exit(1)
		}
		taskID, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Println("❌ 任务ID必须是数字")
			os.Exit(1)
		}
		handleTaskDelete(app, taskID)
	case "show":
		if len(os.Args) < 4 {
			fmt.Println("❌ 用法: process-tracker task show <id>")
			os.Exit(1)
		}
		taskID, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Println("❌ 任务ID必须是数字")
			os.Exit(1)
		}
		handleTaskShow(app, taskID)
	default:
		fmt.Printf("❌ 未知操作: %s\n", action)
		os.Exit(1)
	}
}

// handleTaskList displays task list
func handleTaskList(app *App, statusFilter ...core.TaskStatus) {
	var filter core.TaskStatus
	if len(statusFilter) > 0 {
		filter = statusFilter[0]
	}

	tasks, err := app.ListTasks(filter)
	if err != nil {
		log.Fatalf("获取任务列表失败: %v", err)
	}

	if len(tasks) == 0 {
		if filter != "" {
			fmt.Printf("📝 没有%s状态的任务\n", filter)
		} else {
			fmt.Println("📝 没有任务")
		}
		return
	}

	fmt.Println("📋 任务列表")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%-6s %-20s %-12s %-10s %-10s %-15s %s\n", "ID", "名称", "状态", "PID", "进程数", "创建时间", "命令")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, task := range tasks {
		statusIcon := getStatusIcon(task.Status)
		pidStr := "-"
		if task.RootPID > 0 {
			pidStr = fmt.Sprintf("%d", task.RootPID)
		}

		fmt.Printf("%-6d %-20s %-12s %-10s %-10d %-15s %s\n",
			task.ID,
			truncateString(task.Name, 20),
			fmt.Sprintf("%s%s", statusIcon, task.Status),
			pidStr,
			task.ProcessCount,
			task.CreatedAt.Format("15:04:05"),
			truncateString(task.Command, 30))
	}

	fmt.Printf("\n共 %d 个任务\n", len(tasks))
}

// handleTaskStop stops a task
func handleTaskStop(app *App, taskID int) {
	task, err := app.GetTask(taskID)
	if err != nil {
		log.Fatalf("获取任务失败: %v", err)
	}

	if task.Status != core.StatusRunning {
		fmt.Printf("⚠️  任务 %d 状态为 %s，无需停止\n", taskID, task.Status)
		return
	}

	if err := app.StopTask(taskID); err != nil {
		log.Fatalf("停止任务失败: %v", err)
	}

	fmt.Printf("✅ 任务 %d (%s) 已停止\n", taskID, task.Name)
}

// handleTaskDelete deletes a task
func handleTaskDelete(app *App, taskID int) {
	task, err := app.GetTask(taskID)
	if err != nil {
		log.Fatalf("获取任务失败: %v", err)
	}

	if err := app.DeleteTask(taskID); err != nil {
		log.Fatalf("删除任务失败: %v", err)
	}

	fmt.Printf("✅ 任务 %d (%s) 已删除\n", taskID, task.Name)
}

// handleTaskShow shows task details
func handleTaskShow(app *App, taskID int) {
	task, err := app.GetTask(taskID)
	if err != nil {
		log.Fatalf("获取任务失败: %v", err)
	}

	fmt.Printf("📋 任务详情\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("ID:       %d\n", task.ID)
	fmt.Printf("名称:     %s\n", task.Name)
	fmt.Printf("状态:     %s%s\n", getStatusIcon(task.Status), task.Status)
	fmt.Printf("命令:     %s\n", task.Command)
	fmt.Printf("优先级:   %d\n", task.Priority)

	if task.RootPID > 0 {
		fmt.Printf("根PID:    %d\n", task.RootPID)
	}
	fmt.Printf("进程数:   %d\n", task.ProcessCount)

	fmt.Printf("创建时间: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	if task.StartedAt != nil {
		fmt.Printf("启动时间: %s\n", task.StartedAt.Format("2006-01-02 15:04:05"))
	}
	if task.CompletedAt != nil {
		fmt.Printf("完成时间: %s\n", task.CompletedAt.Format("2006-01-02 15:04:05"))
	}

	if task.TotalCPU > 0 || task.TotalMemory > 0 {
		fmt.Printf("资源使用:\n")
		if task.TotalCPU > 0 {
			fmt.Printf("  CPU:  %.1f%%\n", task.TotalCPU)
		}
		if task.TotalMemory > 0 {
			fmt.Printf("  内存: %s\n", formatBytes(task.TotalMemory))
		}
		if task.TotalDiskIO > 0 {
			fmt.Printf("  磁盘: %s\n", formatBytes(task.TotalDiskIO))
		}
		if task.TotalNetIO > 0 {
			fmt.Printf("  网络: %s\n", formatBytes(task.TotalNetIO))
		}
	}

	if task.ErrorMessage != "" {
		fmt.Printf("错误: %s\n", task.ErrorMessage)
	}
	if task.ExitCode != nil {
		fmt.Printf("退出码: %d\n", *task.ExitCode)
	}
}

// getStatusIcon returns status icon
func getStatusIcon(status core.TaskStatus) string {
	switch status {
	case core.StatusPending:
		return "⏳ "
	case core.StatusRunning:
		return "🟢 "
	case core.StatusCompleted:
		return "✅ "
	case core.StatusFailed:
		return "❌ "
	case core.StatusStopped:
		return "🛑 "
	default:
		return "❓ "
	}
}

// truncateString truncates string to specified length
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// formatBytes formats bytes/MB value with appropriate unit
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