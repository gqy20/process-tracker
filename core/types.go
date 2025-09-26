package core

import (
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	StatisticsGranularity string  // "simple", "detailed", "full"
	ShowCommands         bool    // Whether to show full commands
	ShowWorkingDirs      bool    // Whether to show working directories
	UseSmartCategories   bool    // Whether to use smart application categorization
	MaxCommandLength     int     // Maximum command length to display
	MaxDirLength         int     // Maximum directory length to display
	Storage              StorageConfig // Storage management configuration
}

// StorageConfig represents storage management configuration
type StorageConfig struct {
	MaxFileSizeMB      int     `yaml:"max_file_size_mb"`      // Maximum file size in MB before rotation
	MaxFiles           int     `yaml:"max_files"`              // Maximum number of files to keep
	CompressAfterDays  int     `yaml:"compress_after_days"`   // Compress files after N days
	CleanupAfterDays   int     `yaml:"cleanup_after_days"`    // Delete files after N days
	AutoCleanup        bool    `yaml:"auto_cleanup"`          // Enable automatic cleanup
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
}

// ResourceStats represents calculated resource statistics
type ResourceStats struct {
	Name         string        `json:"name"`
	Category     string        `json:"category"`
	Command      string        `json:"command"`
	WorkingDir   string        `json:"working_dir"`
	ActiveTime   time.Duration `json:"active_time"`
	CPUAvg       float64       `json:"cpu_avg"`
	CPUMax       float64       `json:"cpu_max"`
	MemoryAvg    float64       `json:"memory_avg"`
	MemoryMax    float64       `json:"memory_max"`
	DiskReadAvg  float64       `json:"disk_read_avg"`
	DiskWriteAvg float64       `json:"disk_write_avg"`
	NetSentAvg   float64       `json:"net_sent_avg"`
	NetRecvAvg   float64       `json:"net_recv_avg"`
	Samples      int           `json:"samples"`
	ActiveSamples int          `json:"active_samples"`
}

// ActivityConfig represents activity detection configuration
type ActivityConfig struct {
	CPUThreshold      float64 `yaml:"cpu_threshold"`       // CPU usage threshold for active status
	MemoryThresholdMB float64 `yaml:"memory_threshold_mb"` // Memory usage threshold in MB
	MinActiveTime     int     `yaml:"min_active_time"`     // Minimum time in seconds to be considered active
}

// GetDefaultConfig returns default configuration
func GetDefaultConfig() Config {
	return Config{
		StatisticsGranularity: "simple",
		ShowCommands:         false,
		ShowWorkingDirs:      false,
		UseSmartCategories:   true,
		MaxCommandLength:     100,
		MaxDirLength:         50,
		Storage: StorageConfig{
			MaxFileSizeMB:     100,
			MaxFiles:          10,
			CompressAfterDays: 3,
			CleanupAfterDays:  30,
			AutoCleanup:       true,
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
		MaxFileSizeMB:     100,
		MaxFiles:          10,
		CompressAfterDays: 3,
		CleanupAfterDays:  30,
		AutoCleanup:       true,
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