package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/process-tracker/core"
)

// WebHandler handles web interface requests
type WebHandler struct {
	app *core.App
}

// NewWebHandler creates a new web handler
func NewWebHandler(app *core.App) *WebHandler {
	return &WebHandler{app: app}
}

// DashboardData represents dashboard data
type DashboardData struct {
	Title       string
	SystemStats SystemStats
	Tasks       []TaskInfo
	Processes   []ProcessInfo
	GeneratedAt time.Time
}

// SystemStats represents system statistics
type SystemStats struct {
	CPUUsage    float64
	MemoryUsage float64
	ProcessCount int
	ActiveCount  int
	LoadAverage  []float64
	Uptime      string
}

// TaskInfo represents task information for web display
type TaskInfo struct {
	ID          int
	Name        string
	Command     string
	Status      string
	Priority    int
	ProcessCount int
	CreatedAt   time.Time
	StartedAt   *time.Time
}

// ProcessInfo represents process information for web display
type ProcessInfo struct {
	PID           int32
	Name          string
	Command       string
	Status        string
	CPUPercent    float64
	MemoryMB      float64
	MemoryPercent float64
	Category      string
	Uptime        string
}

// SetupWebRoutes configures web interface routes
func SetupWebRoutes(router *gin.Engine, app *core.App) {
	webHandler := NewWebHandler(app)

	// Load HTML templates
	templateFiles := []string{
		"web/templates/dashboard.html",
		"web/templates/tasks.html",
		"web/templates/processes.html",
	}

	templates := template.Must(template.ParseFiles(templateFiles...))
	router.SetHTMLTemplate(templates)

	// Web interface routes
	router.GET("/", webHandler.Dashboard)
	router.GET("/dashboard", webHandler.Dashboard)
	router.GET("/tasks", webHandler.Tasks)
	router.GET("/processes", webHandler.Processes)

	// API endpoints for web interface
	router.GET("/api/dashboard", webHandler.GetDashboardData)
	router.GET("/api/tasks", webHandler.GetTaskData)
	router.GET("/api/processes", webHandler.GetProcessData)
	router.POST("/api/tasks/:id/start", webHandler.StartTask)
	router.POST("/api/tasks/:id/stop", webHandler.StopTask)
	router.POST("/api/tasks", webHandler.CreateTask)

	// Static files
	router.Static("/static", "./web/static")
}

// Dashboard renders the dashboard page
func (h *WebHandler) Dashboard(c *gin.Context) {
	data := h.prepareDashboardData()
	c.HTML(http.StatusOK, "dashboard.html", data)
}

// Tasks renders the tasks page
func (h *WebHandler) Tasks(c *gin.Context) {
	tasks, _ := h.app.ListTasks(core.StatusPending)

	var taskInfos []TaskInfo
	for _, task := range tasks {
		taskInfo := TaskInfo{
			ID:           task.ID,
			Name:         task.Name,
			Command:      task.Command,
			Status:       string(task.Status),
			Priority:     task.Priority,
			ProcessCount: task.ProcessCount,
			CreatedAt:    task.CreatedAt,
			StartedAt:    task.StartedAt,
		}
		taskInfos = append(taskInfos, taskInfo)
	}

	data := gin.H{
		"Title":     "任务管理 - Process Tracker",
		"Tasks":     taskInfos,
		"TaskCount": len(taskInfos),
	}
	c.HTML(http.StatusOK, "tasks.html", data)
}

// Processes renders the processes page
func (h *WebHandler) Processes(c *gin.Context) {
	// Get recent processes
	storageManager := core.NewManager(h.app.DataFile, 0, false, core.StorageConfig{})
	allRecords, _ := storageManager.ReadRecords(h.app.DataFile)

	// Filter recent records (last 5 minutes)
	cutoffTime := time.Now().Add(-5 * time.Minute)
	latest := make(map[int32]core.ResourceRecord)

	for _, record := range allRecords {
		if record.Timestamp.After(cutoffTime) {
			if existing, ok := latest[record.PID]; !ok || record.Timestamp.After(existing.Timestamp) {
				latest[record.PID] = record
			}
		}
	}

	var processInfos []ProcessInfo
	totalMemoryMB := core.SystemMemoryMB()

	for _, record := range latest {
		memoryPercent := 0.0
		if totalMemoryMB > 0 {
			memoryPercent = (record.MemoryMB / totalMemoryMB) * 100
		}

		uptime := ""
		if record.CreateTime > 0 {
			startTime := time.UnixMilli(record.CreateTime)
			uptime = formatUptime(time.Since(startTime))
		}

		status := "idle"
		if record.IsActive {
			status = "active"
		}

		processInfo := ProcessInfo{
			PID:           record.PID,
			Name:          record.Name,
			Command:       record.Command,
			Status:        status,
			CPUPercent:    record.CPUPercentNormalized,
			MemoryMB:      record.MemoryMB,
			MemoryPercent: memoryPercent,
			Category:      record.Category,
			Uptime:        uptime,
		}
		processInfos = append(processInfos, processInfo)
	}

	data := gin.H{
		"Title":       "进程监控 - Process Tracker",
		"Processes":   processInfos,
		"ProcessCount": len(processInfos),
	}
	c.HTML(http.StatusOK, "processes.html", data)
}

// GetDashboardData returns dashboard data as JSON
func (h *WebHandler) GetDashboardData(c *gin.Context) {
	data := h.prepareDashboardData()
	c.JSON(http.StatusOK, data)
}

// GetTaskData returns task data as JSON
func (h *WebHandler) GetTaskData(c *gin.Context) {
	tasks, _ := h.app.ListTasks(core.StatusPending)

	var taskInfos []TaskInfo
	for _, task := range tasks {
		taskInfo := TaskInfo{
			ID:           task.ID,
			Name:         task.Name,
			Command:      task.Command,
			Status:       string(task.Status),
			Priority:     task.Priority,
			ProcessCount: task.ProcessCount,
			CreatedAt:    task.CreatedAt,
			StartedAt:    task.StartedAt,
		}
		taskInfos = append(taskInfos, taskInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": taskInfos,
		"count": len(taskInfos),
	})
}

// GetProcessData returns process data as JSON
func (h *WebHandler) GetProcessData(c *gin.Context) {
	storageManager := core.NewManager(h.app.DataFile, 0, false, core.StorageConfig{})
	allRecords, _ := storageManager.ReadRecords(h.app.DataFile)

	cutoffTime := time.Now().Add(-5 * time.Minute)
	latest := make(map[int32]core.ResourceRecord)

	for _, record := range allRecords {
		if record.Timestamp.After(cutoffTime) {
			if existing, ok := latest[record.PID]; !ok || record.Timestamp.After(existing.Timestamp) {
				latest[record.PID] = record
			}
		}
	}

	var processInfos []ProcessInfo
	totalMemoryMB := core.SystemMemoryMB()

	for _, record := range latest {
		memoryPercent := 0.0
		if totalMemoryMB > 0 {
			memoryPercent = (record.MemoryMB / totalMemoryMB) * 100
		}

		status := "idle"
		if record.IsActive {
			status = "active"
		}

		processInfo := ProcessInfo{
			PID:           record.PID,
			Name:          record.Name,
			Command:       record.Command,
			Status:        status,
			CPUPercent:    record.CPUPercentNormalized,
			MemoryMB:      record.MemoryMB,
			MemoryPercent: memoryPercent,
			Category:      record.Category,
		}
		processInfos = append(processInfos, processInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"processes": processInfos,
		"count":     len(processInfos),
	})
}

// StartTask starts a task
func (h *WebHandler) StartTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	err = h.app.StartTask(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task started successfully"})
}

// StopTask stops a task
func (h *WebHandler) StopTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	err = h.app.StopTask(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task stopped successfully"})
}

// CreateTask creates a new task
func (h *WebHandler) CreateTask(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Command string `json:"command" binding:"required"`
		Priority int   `json:"priority"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Priority == 0 {
		req.Priority = 1
	}

	task, err := h.app.CreateTask(req.Name, req.Command, req.Priority)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Task created successfully",
		"task":    task,
	})
}

// Helper functions

// prepareDashboardData prepares dashboard data
func (h *WebHandler) prepareDashboardData() DashboardData {
	// Get tasks
	tasks, _ := h.app.ListTasks(core.StatusPending)
	var taskInfos []TaskInfo
	for _, task := range tasks {
		taskInfo := TaskInfo{
			ID:           task.ID,
			Name:         task.Name,
			Command:      task.Command,
			Status:       string(task.Status),
			Priority:     task.Priority,
			ProcessCount: task.ProcessCount,
			CreatedAt:    task.CreatedAt,
			StartedAt:    task.StartedAt,
		}
		taskInfos = append(taskInfos, taskInfo)
	}

	// Get processes
	storageManager := core.NewManager(h.app.DataFile, 0, false, core.StorageConfig{})
	allRecords, _ := storageManager.ReadRecords(h.app.DataFile)

	cutoffTime := time.Now().Add(-5 * time.Minute)
	latest := make(map[int32]core.ResourceRecord)

	for _, record := range allRecords {
		if record.Timestamp.After(cutoffTime) {
			if existing, ok := latest[record.PID]; !ok || record.Timestamp.After(existing.Timestamp) {
				latest[record.PID] = record
			}
		}
	}

	var processInfos []ProcessInfo
	totalMemoryMB := core.SystemMemoryMB()
	activeCount := 0

	for _, record := range latest {
		if record.IsActive {
			activeCount++
		}

		memoryPercent := 0.0
		if totalMemoryMB > 0 {
			memoryPercent = (record.MemoryMB / totalMemoryMB) * 100
		}

		uptime := ""
		if record.CreateTime > 0 {
			startTime := time.UnixMilli(record.CreateTime)
			uptime = formatUptime(time.Since(startTime))
		}

		status := "idle"
		if record.IsActive {
			status = "active"
		}

		processInfo := ProcessInfo{
			PID:           record.PID,
			Name:          record.Name,
			Command:       record.Command,
			Status:        status,
			CPUPercent:    record.CPUPercentNormalized,
			MemoryMB:      record.MemoryMB,
			MemoryPercent: memoryPercent,
			Category:      record.Category,
			Uptime:        uptime,
		}
		processInfos = append(processInfos, processInfo)
	}

	// Calculate system stats (simplified)
	systemStats := SystemStats{
		CPUUsage:    25.0, // Mock data
		MemoryUsage: 45.0, // Mock data
		ProcessCount: len(processInfos),
		ActiveCount:  activeCount,
		LoadAverage:  []float64{0.5, 0.8, 1.2}, // Mock data
		Uptime:      "2d 15h 30m", // Mock data
	}

	return DashboardData{
		Title:       "系统概览 - Process Tracker",
		SystemStats: systemStats,
		Tasks:       taskInfos,
		Processes:   processInfos[:10], // Top 10 processes
		GeneratedAt: time.Now(),
	}
}

// formatUptime formats duration into human readable string
func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	} else {
		days := int(d.Hours() / 24)
		hours := d.Hours() - float64(days*24)
		if hours > 0.1 {
			return fmt.Sprintf("%dd%.1fh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
}