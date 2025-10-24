package v1

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/process-tracker/core"
)

// TaskHandler handles task-related API endpoints
type TaskHandler struct {
	app *core.App
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(app *core.App) *TaskHandler {
	return &TaskHandler{app: app}
}

// CreateTask creates a new task
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendValidationError(c, []string{err.Error()})
		return
	}

	// Create task using core app
	task, err := h.app.CreateTask(req.Name, req.Command, req.Priority)
	if err != nil {
		SendBadRequest(c, "Failed to create task: "+err.Error())
		return
	}

	// Set additional fields if provided
	if req.WorkDir != "" {
		task.WorkDir = req.WorkDir
	}
	// Note: Labels are not yet supported in core.Task

	// Convert to response format
	response := h.taskToResponse(task)
	SendCreated(c, KindTask, response)
}

// ListTasks returns a list of tasks with filtering and pagination
func (h *TaskHandler) ListTasks(c *gin.Context) {
	params := c.MustGet("query_params").(QueryParams)

	// Parse filter parameters
	limit := params.Limit
	offset := params.Offset

	// Get all tasks from core app
	tasks, err := h.app.ListTasks(core.StatusPending)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to list tasks: %w", err))
		return
	}

	// Convert to response format
	taskResponses := make([]TaskResponse, len(tasks))
	for i, task := range tasks {
		taskResponses[i] = h.taskToResponse(task)
	}

	// Apply pagination
	total := len(taskResponses)
	start := offset
	end := offset + limit
	if start > total {
		taskResponses = []TaskResponse{}
	} else if end > total {
		end = total
	}
	paginatedTasks := taskResponses[start:end]

	SendPaginated(c, KindTaskList, paginatedTasks, total, params)
}

// GetTask returns a specific task by ID
func (h *TaskHandler) GetTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendBadRequest(c, "Invalid task ID: "+idStr)
		return
	}

	task, err := h.app.GetTask(id)
	if err != nil {
		SendNotFoundError(c, "task", id)
		return
	}

	response := h.taskToResponse(task)
	SendSuccess(c, KindTask, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// UpdateTask updates an existing task
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendBadRequest(c, "Invalid task ID: "+idStr)
		return
	}

	var req TaskUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendValidationError(c, []string{err.Error()})
		return
	}

	// Get existing task
	task, err := h.app.GetTask(id)
	if err != nil {
		SendNotFoundError(c, "task", id)
		return
	}

	// Update fields if provided
	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	// Note: Labels are not yet supported in core.Task

	// Save updated task
	// Note: This would require updating the core app to support task updates
	// For now, we'll return the updated task structure

	response := h.taskToResponse(task)
	SendSuccess(c, KindTask, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// DeleteTask deletes a task
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendBadRequest(c, "Invalid task ID: "+idStr)
		return
	}

	err = h.app.DeleteTask(id)
	if err != nil {
		SendBadRequest(c, "Failed to delete task: "+err.Error())
		return
	}

	SendNoContent(c)
}

// StartTask starts a task
func (h *TaskHandler) StartTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendBadRequest(c, "Invalid task ID: "+idStr)
		return
	}

	err = h.app.StartTask(id)
	if err != nil {
		if err.Error() == fmt.Sprintf("task %d is not in pending state (current: running)", id) {
			SendConflict(c, "Task is already running")
		} else {
			SendBadRequest(c, "Failed to start task: "+err.Error())
		}
		return
	}

	task, err := h.app.GetTask(id)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to retrieve task after start: %w", err))
		return
	}

	response := h.taskToResponse(task)
	SendSuccess(c, KindTask, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// StopTask stops a task
func (h *TaskHandler) StopTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendBadRequest(c, "Invalid task ID: "+idStr)
		return
	}

	err = h.app.StopTask(id)
	if err != nil {
		if err.Error() == fmt.Sprintf("task %d is not running (current: pending)", id) {
			SendConflict(c, "Task is not running")
		} else {
			SendBadRequest(c, "Failed to stop task: "+err.Error())
		}
		return
	}

	task, err := h.app.GetTask(id)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to retrieve task after stop: %w", err))
		return
	}

	response := h.taskToResponse(task)
	SendSuccess(c, KindTask, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// RestartTask restarts a task
func (h *TaskHandler) RestartTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendBadRequest(c, "Invalid task ID: "+idStr)
		return
	}

	// Stop the task first if it's running
	task, err := h.app.GetTask(id)
	if err != nil {
		SendNotFoundError(c, "task", id)
		return
	}

	if task.Status == core.StatusRunning {
		err = h.app.StopTask(id)
		if err != nil {
			SendBadRequest(c, "Failed to stop task for restart: "+err.Error())
			return
		}

		// Wait a moment for the task to fully stop
		time.Sleep(1 * time.Second)
	}

	// Start the task
	err = h.app.StartTask(id)
	if err != nil {
		SendBadRequest(c, "Failed to restart task: "+err.Error())
		return
	}

	task, err = h.app.GetTask(id)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to retrieve task after restart: %w", err))
		return
	}

	response := h.taskToResponse(task)
	SendSuccess(c, KindTask, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetTaskLogs returns task logs (placeholder implementation)
func (h *TaskHandler) GetTaskLogs(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendBadRequest(c, "Invalid task ID: "+idStr)
		return
	}

	_, err = h.app.GetTask(id)
	if err != nil {
		SendNotFoundError(c, "task", id)
		return
	}

	// This is a placeholder implementation
	// In a real implementation, you would retrieve actual task logs
	logs := []string{
		fmt.Sprintf("Task %d logs would appear here", id),
		"This is a placeholder implementation",
	}

	SendSuccess(c, KindTask, logs, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetTaskStats returns task statistics
func (h *TaskHandler) GetTaskStats(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendBadRequest(c, "Invalid task ID: "+idStr)
		return
	}

	task, err := h.app.GetTask(id)
	if err != nil {
		SendNotFoundError(c, "task", id)
		return
	}

	stats := map[string]interface{}{
		"id":            task.ID,
		"name":          task.Name,
		"status":        task.Status,
		"processCount":  task.ProcessCount,
		"createdAt":     task.CreatedAt,
		"startedAt":     task.StartedAt,
		"completedAt":   task.CompletedAt,
		"totalCPU":      task.TotalCPU,
		"totalMemory":   task.TotalMemory,
		"totalDiskIO":   task.TotalDiskIO,
		"totalNetIO":    task.TotalNetIO,
	}

	SendSuccess(c, KindStats, stats, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// Helper functions

// taskToResponse converts a core.Task to TaskResponse
func (h *TaskHandler) taskToResponse(task *core.Task) TaskResponse {
	response := TaskResponse{
		ID:           task.ID,
		Name:         task.Name,
		Command:      task.Command,
		Status:       string(task.Status),
		Priority:     task.Priority,
		WorkDir:      task.WorkDir,
		RootPID:      task.RootPID,
		ProcessCount: task.ProcessCount,
		CreatedAt:    task.CreatedAt,
		StartedAt:    task.StartedAt,
		CompletedAt:  task.CompletedAt,
		ExitCode:     task.ExitCode,
		ErrorMessage: task.ErrorMessage,
		Labels:       make(map[string]string), // Empty labels for now
	}

	// Add resource usage if available
	if task.TotalCPU > 0 || task.TotalMemory > 0 {
		response.ResourceUsage = &TaskResourceUsage{
			CPU:        task.TotalCPU,
			Memory:     task.TotalMemory,
			DiskRead:   task.TotalDiskIO,
			DiskWrite:  task.TotalDiskIO,
			NetSent:    task.TotalNetIO,
			NetRecv:    task.TotalNetIO,
			UpdatedAt:  time.Now(),
		}
	}

	return response
}