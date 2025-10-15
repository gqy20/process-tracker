package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DaemonManager handles process lifecycle management
type DaemonManager struct {
	pidFile string
}

// NewDaemonManager creates a new daemon manager
func NewDaemonManager(dataDir string) *DaemonManager {
	return &DaemonManager{
		pidFile: filepath.Join(dataDir, "process-tracker.pid"),
	}
}

// WritePID writes the current process PID to file
func (d *DaemonManager) WritePID() error {
	pid := os.Getpid()
	content := fmt.Sprintf("%d", pid)
	
	// Ensure directory exists
	dir := filepath.Dir(d.pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	return os.WriteFile(d.pidFile, []byte(content), 0644)
}

// ReadPID reads the PID from file
func (d *DaemonManager) ReadPID() (int, error) {
	content, err := os.ReadFile(d.pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("PID file not found: process not running")
		}
		return 0, err
	}
	
	pidStr := strings.TrimSpace(string(content))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}
	
	return pid, nil
}

// IsRunning checks if the process is still running
func (d *DaemonManager) IsRunning() (bool, int, error) {
	pid, err := d.ReadPID()
	if err != nil {
		return false, 0, err
	}
	
	// Check if process exists by sending signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, pid, nil
	}
	
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist
		return false, pid, nil
	}
	
	return true, pid, nil
}

// Stop sends SIGTERM to the running process
func (d *DaemonManager) Stop() error {
	pid, err := d.ReadPID()
	if err != nil {
		return err
	}
	
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process %d not found", pid)
	}
	
	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to stop process %d: %w", pid, err)
	}
	
	return nil
}

// RemovePID removes the PID file
func (d *DaemonManager) RemovePID() error {
	if err := os.Remove(d.pidFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetStatus returns the status of the process
func (d *DaemonManager) GetStatus() (string, int, error) {
	running, pid, err := d.IsRunning()
	if err != nil {
		return "stopped", 0, nil
	}
	
	if running {
		return "running", pid, nil
	}
	
	return "stopped", 0, nil
}
