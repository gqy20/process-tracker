package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// TaskManager manages task scheduling, execution, and monitoring
type TaskManager struct {
	app           *App
	config        TaskManagerConfig
	tasks         map[string]*Task
	taskHistory   []*TaskResult
	runningTasks  map[string]*TaskExecution
	pendingQueue  []*Task
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	events        chan TaskEvent
	stats         TaskStats
}

// TaskExecution represents a running task
type TaskExecution struct {
	Task        *Task
	Cmd         *exec.Cmd
	CancelFunc  context.CancelFunc
	StartTime   time.Time
	OutputLog   *os.File
	ErrorLog    *os.File
	ProcessPID  int32  // Store PID instead of full ManagedProcess
}

// TaskEvent represents a task lifecycle event
type TaskEvent struct {
	Type      TaskEventType
	TaskID    string
	Timestamp time.Time
	Data      interface{}
}

// TaskEventType represents the type of task event
type TaskEventType string

const (
	TaskEventCreated    TaskEventType = "created"
	TaskEventStarted    TaskEventType = "started"
	TaskEventCompleted  TaskEventType = "completed"
	TaskEventFailed     TaskEventType = "failed"
	TaskEventCancelled  TaskEventType = "cancelled"
	TaskEventRetry      TaskEventType = "retry"
	TaskEventTimeout    TaskEventType = "timeout"
)

// TaskStats represents task manager statistics
type TaskStats struct {
	TotalTasks      int64
	CompletedTasks  int64
	FailedTasks     int64
	RunningTasks    int64
	PendingTasks    int64
	AvgDuration     time.Duration
	TotalMemoryUsed int64
	TotalCPUUsed    float64
	LastUpdated     time.Time
}

// NewTaskManager creates a new task manager instance
func NewTaskManager(config TaskManagerConfig, app *App) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &TaskManager{
		app:          app,
		config:       config,
		tasks:        make(map[string]*Task),
		taskHistory:  make([]*TaskResult, 0),
		runningTasks: make(map[string]*TaskExecution),
		pendingQueue: make([]*Task, 0),
		ctx:          ctx,
		cancel:       cancel,
		events:       make(chan TaskEvent, 100),
		stats: TaskStats{
			LastUpdated: time.Now(),
		},
	}
}

// Start starts the task manager
func (tm *TaskManager) Start() error {
	if !tm.config.Enabled {
		return nil
	}

	log.Printf("🚀 TaskManager starting with %d max tasks, %d max concurrent", tm.config.MaxTasks, tm.config.MaxConcurrent)

	// Start task scheduler
	go tm.taskScheduler()

	// Start task monitor
	go tm.taskMonitor()

	// Start stats collector
	go tm.statsCollector()

	return nil
}

// Stop stops the task manager gracefully
func (tm *TaskManager) Stop() {
	log.Println("🛑 TaskManager stopping...")

	// Cancel all running tasks
	tm.mu.Lock()
	for _, execution := range tm.runningTasks {
		tm.cancelTask(execution.Task.ID)
	}
	tm.mu.Unlock()

	// Cancel context
	tm.cancel()

	log.Println("✅ TaskManager stopped")
}

// CreateTask creates a new task
func (tm *TaskManager) CreateTask(task *Task) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if len(tm.tasks) >= tm.config.MaxTasks {
		return fmt.Errorf("maximum number of tasks (%d) reached", tm.config.MaxTasks)
	}

	// Validate task
	if err := tm.validateTask(task); err != nil {
		return err
	}

	// Initialize task fields
	if task.ID == "" {
		task.ID = tm.generateTaskID()
	}
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if task.Priority == 0 {
		task.Priority = TaskPriorityNormal
	}
	if task.Timeout == 0 {
		task.Timeout = tm.config.DefaultTimeout
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.MaxRetries == 0 {
		task.MaxRetries = tm.config.RetryAttempts
	}

	// Add task
	tm.tasks[task.ID] = task
	tm.pendingQueue = append(tm.pendingQueue, task)

	// Emit event
	tm.emitEvent(TaskEventCreated, task.ID, task)

	// Log
	log.Printf("✅ Task created: %s (%s)", task.ID, task.Name)

	return nil
}

// StartTask starts a specific task
func (tm *TaskManager) StartTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != TaskStatusPending {
		return fmt.Errorf("task %s is not in pending state", taskID)
	}

	// Check dependencies
	if err := tm.checkDependencies(task); err != nil {
		return err
	}

	// Start task
	return tm.startTask(task)
}

// CancelTask cancels a running task
func (tm *TaskManager) CancelTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	return tm.cancelTask(taskID)
}

// cancelTask internal implementation
func (tm *TaskManager) cancelTask(taskID string) error {
	execution, exists := tm.runningTasks[taskID]
	if !exists {
		return fmt.Errorf("task %s is not running", taskID)
	}

	// Cancel context
	if execution.CancelFunc != nil {
		execution.CancelFunc()
	}

	// Stop process if managed
	if execution.ProcessPID != 0 {
		tm.app.StopProcess(execution.ProcessPID)
	}

	// Update task status
	task := tm.tasks[taskID]
	task.Status = TaskStatusCancelled
	task.CompletedAt = time.Now()

	// Emit event
	tm.emitEvent(TaskEventCancelled, taskID, nil)

	// Remove from running tasks
	delete(tm.runningTasks, taskID)

	log.Printf("🚫 Task cancelled: %s", taskID)
	return nil
}

// PauseTask pauses a task
func (tm *TaskManager) PauseTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != TaskStatusRunning {
		return fmt.Errorf("task %s is not running", taskID)
	}

	// Cancel the running task
	if err := tm.cancelTask(taskID); err != nil {
		return err
	}

	// Set status to paused
	task.Status = TaskStatusPaused
	task.PID = 0

	log.Printf("⏸️ Task paused: %s", taskID)
	return nil
}

// ResumeTask resumes a paused task
func (tm *TaskManager) ResumeTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != TaskStatusPaused {
		return fmt.Errorf("task %s is not paused", taskID)
	}

	// Reset status to pending
	task.Status = TaskStatusPending
	tm.pendingQueue = append(tm.pendingQueue, task)

	log.Printf("▶️ Task resumed: %s", taskID)
	return nil
}

// GetTask retrieves a task by ID
func (tm *TaskManager) GetTask(taskID string) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

// ListTasks returns all tasks
func (tm *TaskManager) ListTasks() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*Task, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		tasks = append(tasks, task)
	}

	// Sort by creation time
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks
}

// GetTaskHistory returns task execution history
func (tm *TaskManager) GetTaskHistory() []*TaskResult {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	history := make([]*TaskResult, len(tm.taskHistory))
	copy(history, tm.taskHistory)

	// Sort by timestamp (newest first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.After(history[j].Timestamp)
	})

	// Limit history size
	if len(history) > tm.config.TaskHistorySize {
		history = history[:tm.config.TaskHistorySize]
	}

	return history
}

// GetStats returns task manager statistics
func (tm *TaskManager) GetStats() TaskStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.stats
}

// taskScheduler schedules tasks from the pending queue
func (tm *TaskManager) taskScheduler() {
	ticker := time.NewTicker(tm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.processPendingTasks()
		}
	}
}

// processPendingTasks processes tasks from the pending queue
func (tm *TaskManager) processPendingTasks() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if we can start more tasks
	if len(tm.runningTasks) >= tm.config.MaxConcurrent {
		return
	}

	// Process pending queue
	for i := 0; i < len(tm.pendingQueue); i++ {
		if len(tm.runningTasks) >= tm.config.MaxConcurrent {
			break
		}

		task := tm.pendingQueue[i]
		
		// Check if task is still pending
		if task.Status != TaskStatusPending {
			continue
		}

		// Check dependencies
		if err := tm.checkDependencies(task); err != nil {
			continue
		}

		// Start task
		if err := tm.startTask(task); err == nil {
			// Remove from pending queue
			tm.pendingQueue = append(tm.pendingQueue[:i], tm.pendingQueue[i+1:]...)
			i--
		}
	}
}

// startTask starts a task execution
func (tm *TaskManager) startTask(task *Task) error {
	// Create execution context
	ctx, cancel := context.WithCancel(tm.ctx)
	if task.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, task.Timeout)
	}

	// Prepare command
	cmd := exec.CommandContext(ctx, task.Command, task.Args...)
	if task.WorkingDir != "" {
		cmd.Dir = task.WorkingDir
	}

	// Set environment variables
	if task.EnvVars != nil {
		env := os.Environ()
		for key, value := range task.EnvVars {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		cmd.Env = env
	}

	// Create log files
	logDir := filepath.Join("logs", "tasks")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	outputLog, err := os.Create(filepath.Join(logDir, fmt.Sprintf("%s.out.log", task.ID)))
	if err != nil {
		return fmt.Errorf("failed to create output log: %v", err)
	}

	errorLog, err := os.Create(filepath.Join(logDir, fmt.Sprintf("%s.err.log", task.ID)))
	if err != nil {
		return fmt.Errorf("failed to create error log: %v", err)
	}

	// Set up pipes for stdout and stderr
	cmd.Stdout = outputLog
	cmd.Stderr = errorLog

	// Start command
	if err := cmd.Start(); err != nil {
		outputLog.Close()
		errorLog.Close()
		return fmt.Errorf("failed to start command: %v", err)
	}

	// Create task execution
	execution := &TaskExecution{
		Task:       task,
		Cmd:        cmd,
		CancelFunc: cancel,
		StartTime:  time.Now(),
		OutputLog:  outputLog,
		ErrorLog:   errorLog,
	}

	// Add to running tasks
	tm.runningTasks[task.ID] = execution

	// Update task status
	task.Status = TaskStatusRunning
	task.StartedAt = time.Now()
	task.PID = int32(cmd.Process.Pid)
	task.LogPath = filepath.Join(logDir, fmt.Sprintf("%s.log", task.ID))

	// Emit event
	tm.emitEvent(TaskEventStarted, task.ID, execution)

	// Start task monitor goroutine
	go tm.monitorTaskExecution(task.ID, execution)

	log.Printf("▶️ Task started: %s (PID: %d)", task.ID, task.PID)
	return nil
}

// monitorTaskExecution monitors a running task
func (tm *TaskManager) monitorTaskExecution(taskID string, execution *TaskExecution) {
	defer execution.OutputLog.Close()
	defer execution.ErrorLog.Close()

	task := execution.Task
	startTime := time.Now()

	// Wait for command to complete
	err := execution.Cmd.Wait()
	endTime := time.Now()

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Remove from running tasks
	delete(tm.runningTasks, taskID)

	// Create result
	result := &TaskResult{
		TaskID:    taskID,
		ExitCode:  0,
		Duration:  endTime.Sub(startTime),
		LogPath:   task.LogPath,
		Timestamp: endTime,
	}

	if err != nil {
		// Handle task failure
		result.ExitCode = 1
		result.Error = err.Error()

		// Check if it was a timeout
		if err == context.DeadlineExceeded {
			task.Status = TaskStatusFailed
			tm.emitEvent(TaskEventTimeout, taskID, result)
			log.Printf("⏰ Task timeout: %s", taskID)
		} else if err == context.Canceled {
			// Task was cancelled, don't retry
			log.Printf("🚫 Task cancelled: %s", taskID)
		} else {
			// Check if we should retry
			if task.RetryCount < task.MaxRetries {
				task.RetryCount++
				task.Status = TaskStatusRetry
				task.PID = 0
				
				// Add back to pending queue
				tm.pendingQueue = append(tm.pendingQueue, task)
				
				tm.emitEvent(TaskEventRetry, taskID, result)
				log.Printf("🔄 Task retry %d/%d: %s", task.RetryCount, task.MaxRetries, taskID)
				return
			} else {
				task.Status = TaskStatusFailed
				tm.emitEvent(TaskEventFailed, taskID, result)
				log.Printf("❌ Task failed: %s (exit code: %d)", taskID, result.ExitCode)
			}
		}
	} else {
		// Task completed successfully
		if execution.Cmd.ProcessState != nil {
			result.ExitCode = execution.Cmd.ProcessState.ExitCode()
		}
		
		task.Status = TaskStatusCompleted
		task.CompletedAt = endTime
		task.ExitCode = result.ExitCode
		
		// Read output
		output, err := tm.readTaskOutput(task.LogPath)
		if err == nil {
			result.Output = output
		}
		
		tm.emitEvent(TaskEventCompleted, taskID, result)
		log.Printf("✅ Task completed: %s (exit code: %d, duration: %v)", taskID, result.ExitCode, result.Duration)
	}

	// Add to history
	tm.taskHistory = append(tm.taskHistory, result)
	
	// Trim history if needed
	if len(tm.taskHistory) > tm.config.TaskHistorySize {
		tm.taskHistory = tm.taskHistory[1:]
	}
}

// taskMonitor monitors task execution and handles timeouts
func (tm *TaskManager) taskMonitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.checkTaskTimeouts()
		}
	}
}

// checkTaskTimeouts checks for tasks that have exceeded their timeout
func (tm *TaskManager) checkTaskTimeouts() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	for taskID, execution := range tm.runningTasks {
		task := execution.Task
		if task.Timeout > 0 && now.Sub(execution.StartTime) > task.Timeout {
			log.Printf("⏰ Task timeout detected: %s", taskID)
			tm.cancelTask(taskID)
		}
	}
}

// statsCollector collects and updates task statistics
func (tm *TaskManager) statsCollector() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.updateStats()
		}
	}
}

// updateStats updates task manager statistics
func (tm *TaskManager) updateStats() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.stats.TotalTasks = int64(len(tm.tasks))
	tm.stats.RunningTasks = int64(len(tm.runningTasks))
	tm.stats.PendingTasks = int64(len(tm.pendingQueue))

	// Count completed and failed tasks
	completed := int64(0)
	failed := int64(0)
	var totalDuration time.Duration
	var count int

	for _, task := range tm.tasks {
		switch task.Status {
		case TaskStatusCompleted:
			completed++
			if !task.CompletedAt.IsZero() && !task.StartedAt.IsZero() {
				totalDuration += task.CompletedAt.Sub(task.StartedAt)
				count++
			}
		case TaskStatusFailed:
			failed++
		}
	}

	tm.stats.CompletedTasks = completed
	tm.stats.FailedTasks = failed

	if count > 0 {
		tm.stats.AvgDuration = totalDuration / time.Duration(count)
	}

	tm.stats.LastUpdated = time.Now()
}

// validateTask validates a task configuration
func (tm *TaskManager) validateTask(task *Task) error {
	if task.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if task.Command == "" {
		return fmt.Errorf("task command is required")
	}

	// Check for duplicate task ID
	if _, exists := tm.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	return nil
}

// checkDependencies checks if task dependencies are satisfied
func (tm *TaskManager) checkDependencies(task *Task) error {
	for _, depID := range task.Dependencies {
		depTask, exists := tm.tasks[depID]
		if !exists {
			return fmt.Errorf("dependency task %s not found", depID)
		}
		if depTask.Status != TaskStatusCompleted {
			return fmt.Errorf("dependency task %s is not completed", depID)
		}
	}
	return nil
}

// generateTaskID generates a unique task ID
func (tm *TaskManager) generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

// emitEvent emits a task event
func (tm *TaskManager) emitEvent(eventType TaskEventType, taskID string, data interface{}) {
	select {
	case tm.events <- TaskEvent{
		Type:      eventType,
		TaskID:    taskID,
		Timestamp: time.Now(),
		Data:      data,
	}:
	default:
		log.Printf("⚠️ Task event channel full, dropping event: %s for task %s", eventType, taskID)
	}
}

// readTaskOutput reads task output from log files
func (tm *TaskManager) readTaskOutput(logPath string) (string, error) {
	outputLogPath := logPath + ".out.log"
	errorLogPath := logPath + ".err.log"

	var output strings.Builder

	// Read stdout
	if stdout, err := os.ReadFile(outputLogPath); err == nil {
		output.WriteString("STDOUT:\n")
		output.WriteString(string(stdout))
	}

	// Read stderr
	if stderr, err := os.ReadFile(errorLogPath); err == nil {
		output.WriteString("\nSTDERR:\n")
		output.WriteString(string(stderr))
	}

	return output.String(), nil
}

// RemoveTask removes a task
func (tm *TaskManager) RemoveTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Cancel if running
	if task.Status == TaskStatusRunning {
		if err := tm.cancelTask(taskID); err != nil {
			return err
		}
	}

	// Remove from tasks
	delete(tm.tasks, taskID)

	// Remove from pending queue
	for i, pendingTask := range tm.pendingQueue {
		if pendingTask.ID == taskID {
			tm.pendingQueue = append(tm.pendingQueue[:i], tm.pendingQueue[i+1:]...)
			break
		}
	}

	log.Printf("🗑️ Task removed: %s", taskID)
	return nil
}

// ClearCompletedTasks removes completed tasks from memory
func (tm *TaskManager) ClearCompletedTasks() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	removed := 0
	for taskID, task := range tm.tasks {
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
			delete(tm.tasks, taskID)
			removed++
		}
	}

	log.Printf("🧹 Cleared %d completed/failed tasks", removed)
	return removed
}