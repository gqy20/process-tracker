package core

import (
    "context"
    "fmt"
    "log"
    "os/exec"
    "runtime"
    "sync"
    "syscall"
    "time"
    "github.com/shirou/gopsutil/v3/process"
)

// ProcessController manages process lifecycle
type ProcessController struct {
    processes map[int32]*ManagedProcess
    mutex     sync.RWMutex
    config    ControllerConfig
    events    chan ProcessEvent
    ctx       context.Context
    cancel    context.CancelFunc
}

// ManagedProcess represents a process under management
type ManagedProcess struct {
    PID         int32
    Name        string
    Command     []string
    WorkingDir  string
    Status      ProcessStatus
    StartTime   time.Time
    Restarts    int
    MaxRestarts int
    ExitCode    int
    LastCheck   time.Time
}

// ProcessStatus represents the current state of a process
type ProcessStatus string

const (
    StatusRunning    ProcessStatus = "running"
    StatusStopped    ProcessStatus = "stopped"
    StatusFailed     ProcessStatus = "failed"
    StatusRestarting ProcessStatus = "restarting"
    StatusUnknown    ProcessStatus = "unknown"
)

// ProcessEvent represents a process lifecycle event
type ProcessEvent struct {
    Type      EventType
    PID       int32
    Timestamp time.Time
    Message   string
    Details   map[string]interface{}
}

// EventType represents different types of process events
type EventType string

const (
    EventProcessStarted  EventType = "started"
    EventProcessStopped  EventType = "stopped"
    EventProcessFailed   EventType = "failed"
    EventProcessRestart EventType = "restarted"
)

// ControllerConfig configures the process controller
type ControllerConfig struct {
    EnableAutoRestart bool          `yaml:"enable_auto_restart"`
    MaxRestarts       int           `yaml:"max_restarts"`
    RestartDelay      time.Duration `yaml:"restart_delay"`
    CheckInterval     time.Duration `yaml:"check_interval"`
}


// NewProcessController creates a new process controller
func NewProcessController(config ControllerConfig) *ProcessController {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &ProcessController{
        processes: make(map[int32]*ManagedProcess),
        config:    config,
        events:    make(chan ProcessEvent, 100),
        ctx:       ctx,
        cancel:    cancel,
    }
}

// StartProcess starts a new managed process
func (pc *ProcessController) StartProcess(name string, command []string, workingDir string) (*ManagedProcess, error) {
    pc.mutex.Lock()
    defer pc.mutex.Unlock()
    
    cmd := exec.CommandContext(pc.ctx, command[0], command[1:]...)
    if workingDir != "" {
        cmd.Dir = workingDir
    }
    
    // Set up process attributes (Unix-like systems only)
    // Note: Setsid is not available on Windows
    if runtime.GOOS != "windows" {
        cmd.SysProcAttr = &syscall.SysProcAttr{
            Setsid: true, // Create new session
        }
    }
    
    // Start the process
    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start process %s: %w", name, err)
    }
    
    // Create managed process record
    managedProc := &ManagedProcess{
        PID:        int32(cmd.Process.Pid),
        Name:       name,
        Command:    command,
        WorkingDir: workingDir,
        Status:     StatusRunning,
        StartTime: time.Now(),
        Restarts:   0,
        MaxRestarts: pc.config.MaxRestarts,
        ExitCode:   0,
        LastCheck:  time.Now(),
    }
    
    pc.processes[managedProc.PID] = managedProc
    
    // Emit start event
    pc.emitEvent(EventProcessStarted, managedProc.PID, fmt.Sprintf("Process %s started", name), map[string]interface{}{
        "command": command,
        "working_dir": workingDir,
    })
    
    // Start monitoring the process
    go pc.monitorProcess(managedProc)
    
    log.Printf("✅ Started process %s (PID: %d)", name, managedProc.PID)
    return managedProc, nil
}

// StopProcess stops a managed process
func (pc *ProcessController) StopProcess(pid int32) error {
    pc.mutex.Lock()
    defer pc.mutex.Unlock()
    
    proc, exists := pc.processes[pid]
    if !exists {
        return fmt.Errorf("process with PID %d not found", pid)
    }
    
    if proc.Status != StatusRunning {
        return fmt.Errorf("process %s is not running", proc.Name)
    }
    
    // Find the process and send SIGTERM
    p, err := process.NewProcess(pid)
    if err != nil {
        return fmt.Errorf("failed to find process %d: %w", pid, err)
    }
    
    // Try graceful shutdown first
    if err := p.Terminate(); err != nil {
        log.Printf("⚠️  Graceful shutdown failed for %s, killing process: %v", proc.Name, err)
        if err := p.Kill(); err != nil {
            return fmt.Errorf("failed to kill process %d: %w", pid, err)
        }
    }
    
    // Wait for process to exit
    time.Sleep(1 * time.Second)
    
    // Update status
    proc.Status = StatusStopped
    pc.emitEvent(EventProcessStopped, pid, fmt.Sprintf("Process %s stopped", proc.Name), map[string]interface{}{
        "exit_code": proc.ExitCode,
        "uptime": time.Since(proc.StartTime),
    })
    
    log.Printf("🛑 Stopped process %s (PID: %d)", proc.Name, pid)
    return nil
}

// RestartProcess restarts a managed process
func (pc *ProcessController) RestartProcess(pid int32) error {
    pc.mutex.Lock()
    defer pc.mutex.Unlock()
    
    proc, exists := pc.processes[pid]
    if !exists {
        return fmt.Errorf("process with PID %d not found", pid)
    }
    
    if proc.Restarts >= proc.MaxRestarts {
        return fmt.Errorf("process %s has reached maximum restart limit (%d)", proc.Name, proc.MaxRestarts)
    }
    
    log.Printf("🔄 Restarting process %s (PID: %d)", proc.Name, pid)
    
    // Stop the current process
    if err := pc.StopProcess(pid); err != nil {
        return fmt.Errorf("failed to stop process %s: %w", proc.Name, err)
    }
    
    // Wait for restart delay
    time.Sleep(pc.config.RestartDelay)
    
    // Start new process
    newProc, err := pc.StartProcess(proc.Name, proc.Command, proc.WorkingDir)
    if err != nil {
        return fmt.Errorf("failed to restart process %s: %w", proc.Name, err)
    }
    
    // Update restart count
    newProc.Restarts = proc.Restarts + 1
    
    pc.emitEvent(EventProcessRestart, newProc.PID, fmt.Sprintf("Process %s restarted", proc.Name), map[string]interface{}{
        "restart_count": newProc.Restarts,
        "old_pid": pid,
        "new_pid": newProc.PID,
    })
    
    return nil
}

// GetManagedProcesses returns all managed processes
func (pc *ProcessController) GetManagedProcesses() []*ManagedProcess {
    pc.mutex.RLock()
    defer pc.mutex.RUnlock()
    
    processes := make([]*ManagedProcess, 0, len(pc.processes))
    for _, proc := range pc.processes {
        processes = append(processes, proc)
    }
    
    return processes
}

// GetProcessByName returns a managed process by name
func (pc *ProcessController) GetProcessByName(name string) (*ManagedProcess, error) {
    pc.mutex.RLock()
    defer pc.mutex.RUnlock()
    
    for _, proc := range pc.processes {
        if proc.Name == name {
            return proc, nil
        }
    }
    
    return nil, fmt.Errorf("process %s not found", name)
}

// Events returns the event channel for process lifecycle events
func (pc *ProcessController) Events() <-chan ProcessEvent {
    return pc.events
}

// Start begins the process controller monitoring
func (pc *ProcessController) Start() {
    go pc.monitorAllProcesses()
}

// Stop shuts down the process controller
func (pc *ProcessController) Stop() {
    pc.cancel()
    
    // Stop all managed processes
    pc.mutex.Lock()
    defer pc.mutex.Unlock()
    
    for _, proc := range pc.processes {
        if proc.Status == StatusRunning {
            pc.StopProcess(proc.PID)
        }
    }
    
    close(pc.events)
}

// monitorAllProcesses periodically checks all managed processes
func (pc *ProcessController) monitorAllProcesses() {
    ticker := time.NewTicker(pc.config.CheckInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-pc.ctx.Done():
            return
        case <-ticker.C:
            pc.checkAllProcesses()
        }
    }
}

// checkAllProcesses checks the status of all managed processes
func (pc *ProcessController) checkAllProcesses() {
    pc.mutex.Lock()
    defer pc.mutex.Unlock()
    
    for pid, proc := range pc.processes {
        if proc.Status != StatusRunning {
            continue
        }
        
        // Check if process is still running
        p, err := process.NewProcess(pid)
        if err != nil {
            // Process no longer exists
            proc.Status = StatusFailed
            proc.ExitCode = -1
            pc.handleProcessFailure(proc)
            continue
        }
        
        // Check if process is still running
        running, err := p.IsRunning()
        if err != nil || !running {
            proc.Status = StatusFailed
            // Note: gopsutil doesn't provide exit code, set to -1
            proc.ExitCode = -1
            pc.handleProcessFailure(proc)
        }
        
        proc.LastCheck = time.Now()
    }
}

// monitorProcess monitors a specific process
func (pc *ProcessController) monitorProcess(proc *ManagedProcess) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-pc.ctx.Done():
            return
        case <-ticker.C:
            pc.checkProcessStatus(proc)
        }
    }
}

// checkProcessStatus checks the status of a specific process
func (pc *ProcessController) checkProcessStatus(proc *ManagedProcess) {
    if proc.Status != StatusRunning {
        return
    }
    
    p, err := process.NewProcess(proc.PID)
    if err != nil {
        proc.Status = StatusFailed
        proc.ExitCode = -1
        pc.handleProcessFailure(proc)
        return
    }
    
    running, err := p.IsRunning()
    if err != nil {
        log.Printf("⚠️  Failed to get status for process %s (PID: %d): %v", proc.Name, proc.PID, err)
        return
    }
    
    if !running {
        proc.Status = StatusFailed
        // Note: gopsutil doesn't provide exit code, set to -1
        proc.ExitCode = -1
        pc.handleProcessFailure(proc)
    }
}

// handleProcessFailure handles process failure and potentially restarts it
func (pc *ProcessController) handleProcessFailure(proc *ManagedProcess) {
    pc.mutex.Lock()
    defer pc.mutex.Unlock()
    
    log.Printf("❌ Process %s (PID: %d) failed with exit code %d", proc.Name, proc.PID, proc.ExitCode)
    
    // Emit failure event
    pc.emitEvent(EventProcessFailed, proc.PID, fmt.Sprintf("Process %s failed", proc.Name), map[string]interface{}{
        "exit_code": proc.ExitCode,
        "restarts": proc.Restarts,
        "uptime": time.Since(proc.StartTime),
    })
    
    // Remove from managed processes
    delete(pc.processes, proc.PID)
    
    // Attempt restart if auto-restart is enabled and we haven't exceeded restart limit
    if pc.config.EnableAutoRestart && proc.Restarts < proc.MaxRestarts {
        log.Printf("🔄 Attempting to restart process %s (attempt %d/%d)", 
            proc.Name, proc.Restarts+1, proc.MaxRestarts)
        
        go func() {
            time.Sleep(pc.config.RestartDelay)
            if err := pc.RestartProcess(proc.PID); err != nil {
                log.Printf("❌ Failed to restart process %s: %v", proc.Name, err)
            }
        }()
    } else if proc.Restarts >= proc.MaxRestarts {
        log.Printf("⚠️  Process %s has reached maximum restart limit (%d)", proc.Name, proc.MaxRestarts)
    }
}

// emitEvent sends a process event
func (pc *ProcessController) emitEvent(eventType EventType, pid int32, message string, details map[string]interface{}) {
    event := ProcessEvent{
        Type:      eventType,
        PID:       pid,
        Timestamp: time.Now(),
        Message:   message,
        Details:   details,
    }
    
    select {
    case pc.events <- event:
    default:
        log.Printf("⚠️  Event channel full, dropping event: %s", message)
    }
}

// GetProcessStats returns statistics about managed processes
func (pc *ProcessController) GetProcessStats() ProcessStats {
    pc.mutex.RLock()
    defer pc.mutex.RUnlock()
    
    stats := ProcessStats{
        TotalProcesses: len(pc.processes),
        Running:        0,
        Stopped:        0,
        Failed:         0,
        Restarting:     0,
    }
    
    for _, proc := range pc.processes {
        switch proc.Status {
        case StatusRunning:
            stats.Running++
        case StatusStopped:
            stats.Stopped++
        case StatusFailed:
            stats.Failed++
        case StatusRestarting:
            stats.Restarting++
        }
    }
    
    return stats
}

// ProcessStats provides statistics about managed processes
type ProcessStats struct {
    TotalProcesses int           `json:"total_processes"`
    Running        int           `json:"running"`
    Stopped        int           `json:"stopped"`
    Failed         int           `json:"failed"`
    Restarting     int           `json:"restarting"`
    Uptime         time.Duration `json:"uptime"`
}