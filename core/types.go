package core

import (
	"fmt"
	"strings"
	"time"
)

// Config represents the application configuration
// Simplified to follow "simple first" principle
type Config struct {
	// Core settings (rarely need to change)
	EnableSmartCategories bool          // Enable intelligent process categorization (default: true)
	Storage               StorageConfig // Storage management configuration
	Docker                DockerConfig  // Docker monitoring configuration
}

// StorageConfig represents storage management configuration
// Simplified: only essential parameters with smart defaults
type StorageConfig struct {
	MaxSizeMB int `yaml:"max_size_mb"` // Maximum total storage size in MB (default: 100)
	KeepDays  int `yaml:"keep_days"`   // Keep data for N days, 0=forever (default: 7)
}

// ResourceRecord represents a single resource usage record
type ResourceRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	Name        string    `json:"name"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryMB    float64   `json:"memory_mb"`
	Threads     int32     `json:"threads"`
	DiskReadMB  float64   `json:"disk_read_mb"`
	DiskWriteMB float64   `json:"disk_write_mb"`
	NetSentKB   float64   `json:"net_sent_kb"`
	NetRecvKB   float64   `json:"net_recv_kb"`
	IsActive    bool      `json:"is_active"`
	Command     string    `json:"command"`
	WorkingDir  string    `json:"working_dir"`
	Category    string    `json:"category"`
	PID         int32     `json:"pid"`         // Process ID
	CreateTime  int64     `json:"create_time"` // Process start time (Unix timestamp)
	CPUTime     float64   `json:"cpu_time"`    // Cumulative CPU time in seconds
}

// ResourceStats represents calculated resource statistics
type ResourceStats struct {
	Name          string        `json:"name"`
	Category      string        `json:"category"`
	Command       string        `json:"command"`
	WorkingDir    string        `json:"working_dir"`
	ActiveTime    time.Duration `json:"active_time"`
	CPUAvg        float64       `json:"cpu_avg"`
	CPUMax        float64       `json:"cpu_max"`
	MemoryAvg     float64       `json:"memory_avg"`
	MemoryMax     float64       `json:"memory_max"`
	DiskReadAvg   float64       `json:"disk_read_avg"`
	DiskWriteAvg  float64       `json:"disk_write_avg"`
	NetSentAvg    float64       `json:"net_sent_avg"`
	NetRecvAvg    float64       `json:"net_recv_avg"`
	Samples       int           `json:"samples"`
	ActiveSamples int           `json:"active_samples"`
	PIDs             []int32       `json:"pids"`               // All observed PIDs
	FirstSeen        time.Time     `json:"first_seen"`         // First observation time
	LastSeen         time.Time     `json:"last_seen"`          // Last observation time
	TotalUptime      time.Duration `json:"total_uptime"`       // Observation duration
	ProcessStartTime time.Time     `json:"process_start_time"` // Process actual start time
	TotalCPUTime     time.Duration `json:"total_cpu_time"`     // Total CPU time consumed
	AvgCPUTime       float64       `json:"avg_cpu_time"`       // Average CPU time per sample
}

// ActivityConfig represents activity detection configuration
type ActivityConfig struct {
	CPUThreshold      float64 `yaml:"cpu_threshold"`       // CPU usage threshold for active status
	MemoryThresholdMB float64 `yaml:"memory_threshold_mb"` // Memory usage threshold in MB
	MinActiveTime     int     `yaml:"min_active_time"`     // Minimum time in seconds to be considered active
}

// DockerConfig represents Docker monitoring configuration
// Simplified: auto-detect Docker availability
type DockerConfig struct {
	Enabled bool `yaml:"enabled"` // Enable Docker monitoring (default: auto-detect)
}

// GetDefaultConfig returns default configuration with smart defaults
// Following "simple first" principle - minimal configuration needed
func GetDefaultConfig() Config {
	return Config{
		EnableSmartCategories: true, // Enable intelligent categorization by default
		Storage: StorageConfig{
			MaxSizeMB: 100, // 100MB total storage (auto-rotates)
			KeepDays:  7,   // Keep 7 days of data
		},
		Docker: DockerConfig{
			Enabled: true, // Auto-detect and enable if available
		},
	}
}

// GetDefaultActivityConfig returns default activity configuration
func GetDefaultActivityConfig() ActivityConfig {
	return ActivityConfig{
		CPUThreshold:      1.0,
		MemoryThresholdMB: 50.0,
		MinActiveTime:     30,
	}
}

// GetDefaultStorageConfig returns default storage configuration
func GetDefaultStorageConfig() StorageConfig {
	return StorageConfig{
		MaxSizeMB: 100, // 100MB total storage
		KeepDays:  7,   // Keep 7 days
	}
}

// IsActive determines if a resource record represents an active process
func IsActive(record ResourceRecord, config ActivityConfig) bool {
	return record.CPUPercent >= config.CPUThreshold ||
		record.MemoryMB >= config.MemoryThresholdMB
}

// IdentifyApplication categorizes an application based on name and command
func IdentifyApplication(name, command string, useSmartCategories bool) string {
	if !useSmartCategories {
		return "other"
	}

	name = strings.ToLower(name)
	command = strings.ToLower(command)

	// Development tools
	if strings.Contains(name, "go") || strings.Contains(name, "gcc") || strings.Contains(name, "python") ||
		strings.Contains(name, "node") || strings.Contains(name, "java") || strings.Contains(name, "code") {
		return "development"
	}

	// Browsers
	if strings.Contains(name, "chrome") || strings.Contains(name, "firefox") || strings.Contains(name, "safari") ||
		strings.Contains(name, "edge") || strings.Contains(name, "opera") {
		return "browser"
	}

	// System tools
	if strings.Contains(name, "ssh") || strings.Contains(name, "bash") || strings.Contains(name, "zsh") ||
		strings.Contains(name, "systemd") || strings.Contains(name, "docker") {
		return "system"
	}

	// Media
	if strings.Contains(name, "vlc") || strings.Contains(name, "mpv") || strings.Contains(name, "spotify") ||
		strings.Contains(name, "rhythm") {
		return "media"
	}

	// Office
	if strings.Contains(name, "libre") || strings.Contains(name, "office") || strings.Contains(name, "word") ||
		strings.Contains(name, "excel") || strings.Contains(name, "powerpoint") {
		return "office"
	}

	return "other"
}

// ValidateStorageConfig validates storage configuration (simplified)
func ValidateStorageConfig(config StorageConfig) error {
	if config.MaxSizeMB < 10 {
		return fmt.Errorf("max_size_mb must be at least 10MB")
	}
	if config.MaxSizeMB > 10000 {
		return fmt.Errorf("max_size_mb too large (max 10GB)")
	}
	if config.KeepDays < 0 {
		return fmt.Errorf("keep_days must be non-negative (0 means forever)")
	}
	if config.KeepDays > 365 {
		return fmt.Errorf("keep_days too large (max 365 days)")
	}
	return nil
}

// ValidateConfig validates the entire configuration (simplified)
func ValidateConfig(config Config) error {
	return ValidateStorageConfig(config.Storage)
}
