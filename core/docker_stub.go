//go:build nodocker

package core

import (
	"fmt"
	"time"
)

// DockerStats represents Docker container statistics (stub implementation)
type DockerStats struct {
	ContainerID     string    `json:"container_id"`
	ContainerName   string    `json:"container_name"`
	Image           string    `json:"image"`
	Status          string    `json:"status"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryUsage     uint64    `json:"memory_usage"`
	MemoryLimit     uint64    `json:"memory_limit"`
	MemoryPercent   float64   `json:"memory_percent"`
	NetworkRxBytes  uint64    `json:"network_rx_bytes"`
	NetworkTxBytes  uint64    `json:"network_tx_bytes"`
	BlockReadBytes  uint64    `json:"block_read_bytes"`
	BlockWriteBytes uint64    `json:"block_write_bytes"`
	Timestamp       time.Time `json:"timestamp"`
	PID             int32     `json:"pid"`             // Container main process PID on host
	CreatedTime     int64     `json:"created_time"`    // Container creation time (Unix ms)
	CPUTime         float64   `json:"cpu_time"`        // Cumulative CPU time in seconds
}

// DockerMonitor stub implementation when Docker support is disabled
type DockerMonitor struct {
	config Config
}

// NewDockerMonitor creates a stub Docker monitor instance
func NewDockerMonitor(config Config) (*DockerMonitor, error) {
	if config.Docker.Enabled {
		return nil, fmt.Errorf("Docker monitoring is not available (built with -tags=nodocker)")
	}
	return &DockerMonitor{config: config}, nil
}

// Start is a no-op in stub implementation
func (dm *DockerMonitor) Start() error {
	return nil
}

// Stop is a no-op in stub implementation
func (dm *DockerMonitor) Stop() error {
	return nil
}

// GetContainerStats returns empty stats in stub implementation
func (dm *DockerMonitor) GetContainerStats() ([]DockerStats, error) {
	return []DockerStats{}, nil
}

// GetContainerInfo returns empty container list in stub implementation
func (dm *DockerMonitor) GetContainerInfo() ([]interface{}, error) {
	return []interface{}{}, nil
}

// IsRunning always returns false in stub implementation
func (dm *DockerMonitor) IsRunning() bool {
	return false
}

// GetLastStats returns empty stats map in stub implementation
func (dm *DockerMonitor) GetLastStats() map[string]DockerStats {
	return make(map[string]DockerStats)
}
