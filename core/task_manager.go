package core

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// TaskManager manages task lifecycle and process tracking
type TaskManager struct {
	// Task storage
	tasks        map[int]*Task         // Task ID -> Task
	nextTaskID  int                    // Next available task ID

	// Process tracking
	pidMap       map[int32]int         // PID -> Task ID (fast lookup)
	processTrees map[int32]*ProcessTreeNode // PID -> Process tree

	// Synchronization
	mu           sync.RWMutex

	// Configuration
	config       TaskConfig
	storage      TaskStorage

	// Channels for communication
	taskEvents   chan TaskEvent
	stopSignals  map[int]chan struct{} // Task ID -> Stop channel

	// Daemon management
	dataDir      string
}

// TaskEvent represents a task-related event
type TaskEvent struct {
	Type     EventType
	TaskID   int
	Data     interface{}
	Timestamp time.Time
}

// EventType represents different types of task events
type EventType string

const (
	EventTaskCreated   EventType = "task_created"
	EventTaskStarted   EventType = "task_started"
	EventTaskCompleted EventType = "task_completed"
	EventTaskFailed    EventType = "task_failed"
	EventTaskStopped   EventType = "task_stopped"
	EventProcessAdded  EventType = "process_added"
	EventProcessRemoved EventType = "process_removed"
)

// TaskStorage interface for task persistence
type TaskStorage interface {
	SaveTask(task *Task) error
	LoadTasks() ([]*Task, error)
	DeleteTask(id int) error
	CleanupOldTasks(olderThan time.Time) error
}

// TaskStorageFile implements TaskStorage using file-based storage
type TaskStorageFile struct {
	filePath string
}

// NewTaskManager creates a new task manager
func NewTaskManager(dataDir string, config TaskConfig) *TaskManager {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Warning: Failed to create task data directory: %v", err)
	}

	storage := &TaskStorageFile{
		filePath: filepath.Join(dataDir, "tasks.json"),
	}

	tm := &TaskManager{
		tasks:        make(map[int]*Task),
		pidMap:       make(map[int32]int),
		processTrees: make(map[int32]*ProcessTreeNode),
		nextTaskID:  1,
		config:       config,
		storage:      storage,
		taskEvents:   make(chan TaskEvent, 100),
		stopSignals:  make(map[int]chan struct{}),
		dataDir:      dataDir,
	}

	// Load existing tasks
	if err := tm.loadTasks(); err != nil {
		log.Printf("Warning: Failed to load existing tasks: %v", err)
	}

	// Start background monitor
	go tm.monitorTasks()

	return tm
}

// CreateTask creates a new task with the given command
func (tm *TaskManager) CreateTask(name, command string, priority int) (*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check concurrent task limit
	if tm.config.MaxConcurrentTasks > 0 {
		runningCount := 0
		for _, task := range tm.tasks {
			if task.Status == StatusRunning {
				runningCount++
			}
		}
		if runningCount >= tm.config.MaxConcurrentTasks {
			return nil, fmt.Errorf("maximum concurrent tasks (%d) reached", tm.config.MaxConcurrentTasks)
		}
	}

	// Create task
	task := &Task{
		ID:            tm.nextTaskID,
		Name:          name,
		Command:       command,
		Status:        StatusPending,
		Priority:      priority,
		ProcessCount:  0,
		CreatedAt:     time.Now(),
		Tags:          []string{},
		PIDMap:        make(map[int32]int),
		WorkDir:       "",
	}

	// Add to storage
	tm.tasks[task.ID] = task
	tm.nextTaskID++

	// Create stop channel
	tm.stopSignals[task.ID] = make(chan struct{})

	// Save to storage
	if err := tm.storage.SaveTask(task); err != nil {
		log.Printf("Warning: Failed to save task %d: %v", task.ID, err)
	}

	// Send event
	tm.sendEvent(EventTaskCreated, task.ID, task)

	log.Printf("Task created: %s (ID: %d)", name, task.ID)
	return task, nil
}

// StartTask starts a task by executing its command
func (tm *TaskManager) StartTask(taskID int) error {
	tm.mu.Lock()
	task, exists := tm.tasks[taskID]
	if !exists {
		tm.mu.Unlock()
		return fmt.Errorf("task %d not found", taskID)
	}

	if task.Status != StatusPending {
		tm.mu.Unlock()
		return fmt.Errorf("task %d is not in pending state (current: %s)", taskID, task.Status)
	}

	// Mark as starting
	task.Status = StatusRunning
	now := time.Now()
	task.StartedAt = &now

	tm.mu.Unlock()

	// Execute command in goroutine
	go tm.executeTask(taskID)

	return nil
}

// executeTask executes a task command and monitors it
func (tm *TaskManager) executeTask(taskID int) {
	tm.mu.RLock()
	task, exists := tm.tasks[taskID]
	if !exists {
		tm.mu.RUnlock()
		return
	}
	stopChan := tm.stopSignals[taskID]
	tm.mu.RUnlock()

	// Prepare command
	cmd := exec.Command("sh", "-c", task.Command)

	// Set working directory if specified
	if task.WorkDir != "" {
		cmd.Dir = task.WorkDir
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		tm.handleTaskFailure(taskID, fmt.Sprintf("Failed to start command: %v", err))
		return
	}

	// Get root PID
	rootPID := int32(cmd.Process.Pid)

	// Update task with root PID
	tm.mu.Lock()
	task.RootPID = rootPID
	task.ProcessCount = 1

	// Add to PID map
	tm.pidMap[rootPID] = taskID
	tm.mu.Unlock()

	log.Printf("Task started: %s (PID: %d)", task.Name, rootPID)

	// Send started event
	tm.sendEvent(EventTaskStarted, taskID, rootPID)

	// Monitor the process
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for completion or stop signal
	select {
	case err := <-done:
		tm.handleTaskCompletion(taskID, err)
	case <-stopChan:
		// Try to stop gracefully first
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
		}

		// Wait a bit for graceful shutdown
		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(5 * time.Second):
			// Force kill if still running
			if cmd.Process != nil {
				cmd.Process.Kill()
				<-done // Wait for final exit
			}
		}

		tm.handleTaskStop(taskID)
	}
}

// StopTask stops a running task
func (tm *TaskManager) StopTask(taskID int) error {
	tm.mu.RLock()
	task, exists := tm.tasks[taskID]
	stopChan, stopExists := tm.stopSignals[taskID]
	tm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task %d not found", taskID)
	}

	if task.Status != StatusRunning {
		return fmt.Errorf("task %d is not running (current: %s)", taskID, task.Status)
	}

	if !stopExists {
		return fmt.Errorf("task %d has no stop channel", taskID)
	}

	// Send stop signal
	close(stopChan)

	log.Printf("Stopping task: %s (ID: %d)", task.Name, taskID)
	return nil
}

// handleTaskCompletion handles task completion (success or failure)
func (tm *TaskManager) handleTaskCompletion(taskID int, err error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return
	}

	now := time.Now()
	task.CompletedAt = &now

	if err != nil {
		task.Status = StatusFailed
		task.ErrorMessage = err.Error()
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode := exitError.ExitCode()
			task.ExitCode = &exitCode
		}

		tm.sendEvent(EventTaskFailed, taskID, err)
		log.Printf("Task failed: %s (ID: %d) - %v", task.Name, taskID, err)
	} else {
		task.Status = StatusCompleted
		exitCode := 0
		task.ExitCode = &exitCode // Success exit code

		tm.sendEvent(EventTaskCompleted, taskID, nil)
		log.Printf("Task completed: %s (ID: %d)", task.Name, taskID)
	}

	// Clean up stop channel
	delete(tm.stopSignals, taskID)

	// Save to storage
	if saveErr := tm.storage.SaveTask(task); saveErr != nil {
		log.Printf("Warning: Failed to save completed task %d: %v", taskID, saveErr)
	}
}

// handleTaskStop handles task stop event
func (tm *TaskManager) handleTaskStop(taskID int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return
	}

	now := time.Now()
	task.CompletedAt = &now
	task.Status = StatusStopped

	tm.sendEvent(EventTaskStopped, taskID, nil)
	log.Printf("Task stopped: %s (ID: %d)", task.Name, taskID)

	// Clean up stop channel
	delete(tm.stopSignals, taskID)

	// Save to storage
	if err := tm.storage.SaveTask(task); err != nil {
		log.Printf("Warning: Failed to save stopped task %d: %v", taskID, err)
	}
}

// handleTaskFailure handles task failure
func (tm *TaskManager) handleTaskFailure(taskID int, errMsg string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return
	}

	now := time.Now()
	task.CompletedAt = &now
	task.Status = StatusFailed
	task.ErrorMessage = errMsg

	tm.sendEvent(EventTaskFailed, taskID, errMsg)
	log.Printf("Task failed: %s (ID: %d) - %s", task.Name, taskID, errMsg)

	// Clean up stop channel
	delete(tm.stopSignals, taskID)

	// Save to storage
	if err := tm.storage.SaveTask(task); err != nil {
		log.Printf("Warning: Failed to save failed task %d: %v", taskID, err)
	}
}

// GetTask retrieves a task by ID
func (tm *TaskManager) GetTask(taskID int) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %d not found", taskID)
	}

	return task, nil
}

// ListTasks returns all tasks, optionally filtered by status
func (tm *TaskManager) ListTasks(statusFilter TaskStatus) ([]*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tasks []*Task
	for _, task := range tm.tasks {
		if statusFilter == "" || task.Status == statusFilter {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

// DeleteTask removes a task from storage
func (tm *TaskManager) DeleteTask(taskID int) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %d not found", taskID)
	}

	// Stop if running
	if task.Status == StatusRunning {
		tm.StopTask(taskID)
		time.Sleep(1 * time.Second) // Give it time to stop
	}

	// Remove from maps
	delete(tm.tasks, taskID)

	// Remove PID mappings
	for pid, id := range tm.pidMap {
		if id == taskID {
			delete(tm.pidMap, pid)
		}
	}

	// Remove from storage
	if err := tm.storage.DeleteTask(taskID); err != nil {
		return fmt.Errorf("failed to delete task from storage: %w", err)
	}

	log.Printf("Task deleted: %s (ID: %d)", task.Name, taskID)
	return nil
}

// UpdateTaskFromProcessTree updates task resource usage from process tree
func (tm *TaskManager) UpdateTaskFromProcessTree(processes []ResourceRecord) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Build new process trees
	newTrees := BuildProcessTree(processes)

	// Clear old process trees
	tm.processTrees = make(map[int32]*ProcessTreeNode)

	// Update tasks with new process data
	for _, tree := range newTrees {
		pid := tree.Process.PID

		// Check if this PID belongs to a task
		if taskID, exists := tm.pidMap[pid]; exists {
			task := tm.tasks[taskID]
			if task != nil && task.Status == StatusRunning {
				// Update process tree
				task.ProcessTree = tree
				task.ProcessCount = tree.ChildCount + 1

				// Update resource usage
				task.TotalCPU = tree.TotalCPU
				task.TotalMemory = tree.TotalMemory

				// Calculate additional metrics
				task.TotalDiskIO = 0
				task.TotalNetIO = 0
				tm.calculateTaskIO(task, tree)

				// Update PID mappings for all processes in tree
				tm.updatePIDMappings(tree, taskID)
			}
		}

		// Store process tree for reference
		tm.processTrees[pid] = tree
	}
}

// updatePIDMappings recursively updates PID mappings for all processes in a tree
func (tm *TaskManager) updatePIDMappings(tree *ProcessTreeNode, taskID int) {
	// Update mapping for this process
	tm.pidMap[tree.Process.PID] = taskID

	// Recursively update children
	for _, child := range tree.Children {
		tm.updatePIDMappings(child, taskID)
	}
}

// calculateTaskIO calculates total I/O for a task from its process tree
func (tm *TaskManager) calculateTaskIO(task *Task, tree *ProcessTreeNode) {
	// Add I/O from root process
	task.TotalDiskIO += tree.Process.DiskReadMB + tree.Process.DiskWriteMB
	task.TotalNetIO += tree.Process.NetSentKB + tree.Process.NetRecvKB

	// Recursively add I/O from children
	for _, child := range tree.Children {
		task.TotalDiskIO += child.Process.DiskReadMB + child.Process.DiskWriteMB
		task.TotalNetIO += child.Process.NetSentKB + child.Process.NetRecvKB
		tm.calculateTaskIO(task, child)
	}
}

// monitorTasks runs background monitoring and cleanup
func (tm *TaskManager) monitorTasks() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.cleanupCompletedTasks()
		}
	}
}

// cleanupCompletedTasks removes old completed tasks
func (tm *TaskManager) cleanupCompletedTasks() {
	if !tm.config.AutoCleanup {
		return
	}

	cutoff := time.Now().Add(-24 * time.Hour) // Keep tasks for 24 hours

	tm.mu.Lock()
	var tasksToDelete []int
	for id, task := range tm.tasks {
		if (task.Status == StatusCompleted || task.Status == StatusFailed || task.Status == StatusStopped) &&
			task.CompletedAt != nil && task.CompletedAt.Before(cutoff) {
			tasksToDelete = append(tasksToDelete, id)
		}
	}
	tm.mu.Unlock()

	// Delete old tasks
	for _, id := range tasksToDelete {
		if err := tm.DeleteTask(id); err != nil {
			log.Printf("Warning: Failed to cleanup old task %d: %v", id, err)
		}
	}
}

// sendEvent sends a task event
func (tm *TaskManager) sendEvent(eventType EventType, taskID int, data interface{}) {
	select {
	case tm.taskEvents <- TaskEvent{
			Type:      eventType,
			TaskID:    taskID,
			Data:      data,
			Timestamp: time.Now(),
		}:
	default:
		// Channel full, drop event
	}
}

// loadTasks loads tasks from storage
func (tm *TaskManager) loadTasks() error {
	tasks, err := tm.storage.LoadTasks()
	if err != nil {
		return err
	}

	for _, task := range tasks {
		tm.tasks[task.ID] = task

		// Rebuild PID mappings
		if task.RootPID > 0 {
			tm.pidMap[task.RootPID] = task.ID
		}

		// Update next task ID
		if task.ID >= tm.nextTaskID {
			tm.nextTaskID = task.ID + 1
		}

		// Recreate stop channels for running tasks
		if task.Status == StatusRunning {
			tm.stopSignals[task.ID] = make(chan struct{})
		}
	}

	log.Printf("Loaded %d tasks from storage", len(tasks))
	return nil
}

// GetTaskEvents returns the task events channel
func (tm *TaskManager) GetTaskEvents() <-chan TaskEvent {
	return tm.taskEvents
}

// ============== TaskStorageFile Implementation ==============

// SaveTask saves a task to file
func (ts *TaskStorageFile) SaveTask(task *Task) error {
	// Load existing tasks
	tasks, err := ts.LoadTasks()
	if err != nil {
		return err
	}

	// Find and update existing task or add new one
	found := false
	for i, t := range tasks {
		if t.ID == task.ID {
			tasks[i] = task
			found = true
			break
		}
	}
	if !found {
		tasks = append(tasks, task)
	}

	// Write back to file
	return ts.saveTasksToFile(tasks)
}

// LoadTasks loads tasks from file
func (ts *TaskStorageFile) LoadTasks() ([]*Task, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(ts.filePath), 0755); err != nil {
		return nil, err
	}

	// Check if file exists
	if _, err := os.Stat(ts.filePath); os.IsNotExist(err) {
		return []*Task{}, nil
	}

	// Read file
	data, err := os.ReadFile(ts.filePath)
	if err != nil {
		return nil, err
	}

	// If file is empty, return empty list
	if len(data) == 0 {
		return []*Task{}, nil
	}

	// Parse JSON
	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

// DeleteTask removes a task from file
func (ts *TaskStorageFile) DeleteTask(id int) error {
	// Load existing tasks
	tasks, err := ts.LoadTasks()
	if err != nil {
		return err
	}

	// Remove task with matching ID
	var filtered []*Task
	for _, task := range tasks {
		if task.ID != id {
			filtered = append(filtered, task)
		}
	}

	// Write back to file
	return ts.saveTasksToFile(filtered)
}

// CleanupOldTasks removes old tasks from file
func (ts *TaskStorageFile) CleanupOldTasks(olderThan time.Time) error {
	// Load existing tasks
	tasks, err := ts.LoadTasks()
	if err != nil {
		return err
	}

	// Filter out old tasks
	var filtered []*Task
	for _, task := range tasks {
		if task.CompletedAt == nil || task.CompletedAt.After(olderThan) {
			filtered = append(filtered, task)
		}
	}

	// Write back to file
	return ts.saveTasksToFile(filtered)
}

// saveTasksToFile is a helper method to save tasks to JSON file
func (ts *TaskStorageFile) saveTasksToFile(tasks []*Task) error {
	// Marshal to JSON
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(ts.filePath), 0755); err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(ts.filePath, data, 0644)
}