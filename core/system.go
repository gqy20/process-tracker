package core

import (
	"log"
	"runtime"
	"sync"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	cachedTotalMemoryMB float64
	cachedTotalCPUCores int
	memoryMu            sync.RWMutex
	cpuMu               sync.RWMutex
)

// SystemMemoryMB returns the total system memory in MB (cached)
func SystemMemoryMB() float64 {
	memoryMu.RLock()
	if cachedTotalMemoryMB > 0 {
		memoryMu.RUnlock()
		return cachedTotalMemoryMB
	}
	memoryMu.RUnlock()

	memoryMu.Lock()
	defer memoryMu.Unlock()

	// Double-check after acquiring write lock
	if cachedTotalMemoryMB > 0 {
		return cachedTotalMemoryMB
	}

	// Get system memory info
	v, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("Warning: Failed to get system memory: %v", err)
		return 0
	}

	cachedTotalMemoryMB = float64(v.Total) / 1024 / 1024
	log.Printf("System total memory: %.2f MB", cachedTotalMemoryMB)
	
	return cachedTotalMemoryMB
}

// SystemCPUCores returns the total number of CPU cores (cached)
func SystemCPUCores() int {
	cpuMu.RLock()
	if cachedTotalCPUCores > 0 {
		cpuMu.RUnlock()
		return cachedTotalCPUCores
	}
	cpuMu.RUnlock()

	cpuMu.Lock()
	defer cpuMu.Unlock()

	// Double-check after acquiring write lock
	if cachedTotalCPUCores > 0 {
		return cachedTotalCPUCores
	}

	// Try gopsutil first (logical cores)
	if counts, err := cpu.Counts(true); err == nil && counts > 0 {
		cachedTotalCPUCores = counts
		log.Printf("System total CPU cores: %d", cachedTotalCPUCores)
		return cachedTotalCPUCores
	}

	// Fallback to runtime.NumCPU
	cachedTotalCPUCores = runtime.NumCPU()
	log.Printf("System total CPU cores (from runtime): %d", cachedTotalCPUCores)
	return cachedTotalCPUCores
}

// CalculateMemoryPercent calculates memory usage as percentage of system total
func CalculateMemoryPercent(memoryMB float64) float64 {
	totalMB := SystemMemoryMB()
	if totalMB == 0 {
		return 0
	}
	return (memoryMB / totalMB) * 100
}

// CalculateCPUPercentNormalized calculates CPU usage as percentage of total system CPU
// For example: 100% on single core = 100/72 = 1.39% on 72-core system
func CalculateCPUPercentNormalized(cpuPercent float64) float64 {
	totalCores := SystemCPUCores()
	if totalCores == 0 {
		return 0
	}
	return cpuPercent / float64(totalCores)
}
