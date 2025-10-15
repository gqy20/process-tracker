//go:build !nodocker

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerStats represents Docker container statistics
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

// DockerMonitor provides simple Docker container monitoring
type DockerMonitor struct {
	client    *client.Client
	config    Config
	isRunning bool
	stopChan  chan struct{}
	mu        sync.RWMutex
	lastStats map[string]DockerStats
}

// NewDockerMonitor creates a new Docker monitor instance
func NewDockerMonitor(config Config) (*DockerMonitor, error) {
	if !config.Docker.Enabled {
		return &DockerMonitor{
			config:    config,
			isRunning: false,
			lastStats: make(map[string]DockerStats),
		}, nil
	}

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Test connection
	_, err = cli.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to docker daemon: %w", err)
	}

	return &DockerMonitor{
		client:    cli,
		config:    config,
		stopChan:  make(chan struct{}),
		lastStats: make(map[string]DockerStats),
	}, nil
}

// Start begins monitoring Docker containers
func (dm *DockerMonitor) Start() error {
	if !dm.config.Docker.Enabled {
		return nil
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.isRunning {
		return fmt.Errorf("docker monitor is already running")
	}

	dm.isRunning = true

	// Start background monitoring
	go dm.monitorLoop()

	log.Println("Docker monitoring started (10s interval)")
	return nil
}

// Stop stops monitoring Docker containers
func (dm *DockerMonitor) Stop() error {
	if !dm.config.Docker.Enabled {
		return nil
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if !dm.isRunning {
		return nil
	}

	close(dm.stopChan)
	dm.isRunning = false
	dm.stopChan = make(chan struct{})

	if dm.client != nil {
		dm.client.Close()
	}

	log.Println("Docker monitoring stopped")
	return nil
}

// GetContainerStats collects statistics from all running containers
func (dm *DockerMonitor) GetContainerStats() ([]DockerStats, error) {
	if !dm.config.Docker.Enabled || dm.client == nil {
		return []DockerStats{}, nil
	}

	// Get list of containers
	containers, err := dm.client.ContainerList(context.Background(), container.ListOptions{
		All: false, // Only running containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var stats []DockerStats
	var wg sync.WaitGroup
	statsChan := make(chan DockerStats, len(containers))
	errorChan := make(chan error, len(containers))

	// Collect stats for each container concurrently
	for _, container := range containers {
		wg.Add(1)
		go func(c types.Container) {
			defer wg.Done()

			containerStats, err := dm.getSingleContainerStats(c)
			if err != nil {
				errorChan <- err
				return
			}

			statsChan <- containerStats
		}(container)
	}

	// Wait for all container stats collection to complete
	go func() {
		wg.Wait()
		close(statsChan)
		close(errorChan)
	}()

	// Collect results and errors
	for stat := range statsChan {
		stats = append(stats, stat)
	}

	// Log any errors (but don't fail completely)
	for err := range errorChan {
		log.Printf("Warning: %v", err)
	}

	// Update last stats cache
	dm.updateLastStats(stats)

	return stats, nil
}

// GetContainerInfo returns basic information about all containers
func (dm *DockerMonitor) GetContainerInfo() ([]types.Container, error) {
	if !dm.config.Docker.Enabled || dm.client == nil {
		return []types.Container{}, nil
	}

	containers, err := dm.client.ContainerList(context.Background(), container.ListOptions{
		All: true, // Include stopped containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return containers, nil
}

// IsRunning returns whether the monitor is currently running
func (dm *DockerMonitor) IsRunning() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.isRunning
}

// GetLastStats returns the last cached statistics
func (dm *DockerMonitor) GetLastStats() map[string]DockerStats {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make(map[string]DockerStats)
	for k, v := range dm.lastStats {
		result[k] = v
	}
	return result
}

// Private methods

func (dm *DockerMonitor) monitorLoop() {
	ticker := time.NewTicker(10 * time.Second) // Fixed 10s interval for simplicity
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats, err := dm.GetContainerStats()
			if err != nil {
				log.Printf("Error collecting docker stats: %v", err)
			} else {
				log.Printf("Collected stats for %d containers", len(stats))
			}
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DockerMonitor) getSingleContainerStats(container types.Container) (DockerStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get container inspect info for PID and creation time
	inspectData, err := dm.client.ContainerInspect(ctx, container.ID)
	if err != nil {
		log.Printf("Warning: Failed to inspect container %s: %v", container.ID[:12], err)
	}

	// Get container stats
	statsResp, err := dm.client.ContainerStats(ctx, container.ID, false)
	if err != nil {
		return DockerStats{}, fmt.Errorf("failed to get stats for container %s: %w", container.ID[:12], err)
	}
	defer statsResp.Body.Close()

	// Decode stats
	var dockerStats types.StatsJSON
	if err := json.NewDecoder(statsResp.Body).Decode(&dockerStats); err != nil {
		return DockerStats{}, fmt.Errorf("failed to decode stats for container %s: %w", container.ID[:12], err)
	}

	// Calculate CPU percentage
	cpuPercent := calculateCPUPercent(&dockerStats)

	// Calculate memory percentage
	memoryPercent := calculateDockerMemoryPercent(&dockerStats)

	// Extract container name
	containerName := ""
	if len(container.Names) > 0 {
		containerName = container.Names[0]
		if containerName[0] == '/' {
			containerName = containerName[1:]
		}
	}

	// Get PID from inspect data
	var pid int32
	if inspectData.State != nil {
		pid = int32(inspectData.State.Pid)
	}

	// Get container creation time (convert to Unix milliseconds)
	var createdTime int64
	if inspectData.Created != "" {
		// Parse Docker's RFC3339Nano time format
		if t, err := time.Parse(time.RFC3339Nano, inspectData.Created); err == nil {
			createdTime = t.UnixMilli()
		}
	}

	// Calculate cumulative CPU time from total usage (convert nanoseconds to seconds)
	cpuTime := float64(dockerStats.CPUStats.CPUUsage.TotalUsage) / 1e9

	// Get network stats (safe access)
	var rxBytes, txBytes uint64 = 0, 0
	if networks := dockerStats.Networks; networks != nil {
		if eth0, exists := networks["eth0"]; exists {
			rxBytes = eth0.RxBytes
			txBytes = eth0.TxBytes
		}
	}

	// Get block I/O stats (safe access)
	var readBytes, writeBytes uint64 = 0, 0
	blkio := dockerStats.BlkioStats
	if len(blkio.IoServiceBytesRecursive) > 0 {
		for _, entry := range blkio.IoServiceBytesRecursive {
			if entry.Op == "Read" {
				readBytes = entry.Value
			} else if entry.Op == "Write" {
				writeBytes = entry.Value
			}
		}
	}

	return DockerStats{
		ContainerID:     container.ID,
		ContainerName:   containerName,
		Image:           container.Image,
		Status:          container.Status,
		CPUPercent:      cpuPercent,
		MemoryUsage:     dockerStats.MemoryStats.Usage,
		MemoryLimit:     dockerStats.MemoryStats.Limit,
		MemoryPercent:   memoryPercent,
		NetworkRxBytes:  rxBytes,
		NetworkTxBytes:  txBytes,
		BlockReadBytes:  readBytes,
		BlockWriteBytes: writeBytes,
		Timestamp:       time.Now(),
		PID:             pid,
		CreatedTime:     createdTime,
		CPUTime:         cpuTime,
	}, nil
}

func (dm *DockerMonitor) updateLastStats(stats []DockerStats) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for _, stat := range stats {
		dm.lastStats[stat.ContainerID] = stat
	}
}

func calculateCPUPercent(stats *types.StatsJSON) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		return (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}

func calculateDockerMemoryPercent(stats *types.StatsJSON) float64 {
	if stats.MemoryStats.Limit > 0 {
		return float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100.0
	}
	return 0.0
}
