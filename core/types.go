package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config represents application configuration
type Config struct {
	StatisticsGranularity string  // "simple", "detailed", "full"
	ShowCommands         bool    // Whether to show full commands
	ShowWorkingDirs      bool    // Whether to show working directories
	UseSmartCategories   bool    // Whether to use smart application categorization
	MaxCommandLength     int     // Maximum command length to display
	MaxDirLength         int     // Maximum directory length to display
	Storage              StorageConfig // Storage management configuration
	
	// Process control configuration
	ProcessControl       ProcessControlConfig `yaml:"process_control"`
	
	// Resource quota configuration
	ResourceQuota        ResourceQuotaConfig `yaml:"resource_quota"`
	
	// Process discovery configuration
	ProcessDiscovery     ProcessDiscoveryConfig `yaml:"process_discovery"`
	
	// Task manager configuration
	TaskManager         TaskManagerConfig `yaml:"task_manager"`
	
	// Health check and alerting configuration
	HealthCheck         HealthCheckConfig `yaml:"health_check"`
}

// ProcessControlConfig configures process management features
type ProcessControlConfig struct {
	Enabled           bool          `yaml:"enabled"`
	EnableAutoRestart bool          `yaml:"enable_auto_restart"`
	MaxRestarts       int           `yaml:"max_restarts"`
	RestartDelay      time.Duration `yaml:"restart_delay"`
	CheckInterval     time.Duration `yaml:"check_interval"`
	ManagedProcesses  []ManagedProcessConfig `yaml:"managed_processes"`
}

// ManagedProcessConfig defines a process to be managed
type ManagedProcessConfig struct {
	Name        string   `yaml:"name"`
	Command     []string `yaml:"command"`
	WorkingDir  string   `yaml:"working_dir"`
	MaxRestarts int      `yaml:"max_restarts"`
}

// ResourceQuotaConfig configures resource quota management
type ResourceQuotaConfig struct {
	Enabled           bool             `yaml:"enabled"`
	CheckInterval     time.Duration    `yaml:"check_interval"`
	DefaultAction     QuotaAction      `yaml:"default_action"`
	MaxViolations     int              `yaml:"max_violations"`
	ViolationWindow  time.Duration    `yaml:"violation_window"`
	Quotas           []ResourceQuota   `yaml:"quotas"`
}


// ResourceQuota defines resource limits for a process or group
type ResourceQuota struct {
	Name           string        `yaml:"name"`
	CPULimit       float64       `yaml:"cpu_limit"`       // CPU percentage limit (0-100)
	MemoryLimitMB  int64         `yaml:"memory_limit_mb"`  // Memory limit in MB
	DiskReadLimitMB int64        `yaml:"disk_read_limit_mb"` // Disk read limit in MB/s
	DiskWriteLimitMB int64       `yaml:"disk_write_limit_mb"` // Disk write limit in MB/s
	NetworkLimitKB int64         `yaml:"network_limit_kb"` // Network limit in KB/s
	ThreadLimit    int32         `yaml:"thread_limit"`    // Maximum number of threads
	ProcessLimit   int32         `yaml:"process_limit"`   // Maximum number of processes
	TimeLimit      time.Duration `yaml:"time_limit"`      // Maximum runtime duration
	Action         QuotaAction   `yaml:"action"`         // Action when quota exceeded
	Processes      []int32       `yaml:"processes"`      // Associated process PIDs
	LastCheck      time.Time     `yaml:"last_check"`
	Violations     int           `yaml:"violations"`
	Active         bool          `yaml:"active"`
}

// QuotaAction defines what action to take when a quota is exceeded
type QuotaAction string

const (
	ActionWarn      QuotaAction = "warn"
	ActionThrottle  QuotaAction = "throttle"
	ActionStop      QuotaAction = "stop"
	ActionRestart   QuotaAction = "restart"
	ActionNotify    QuotaAction = "notify"
)

// TaskManagerConfig configures task management and scheduling
type TaskManagerConfig struct {
	Enabled          bool          `yaml:"enabled"`
	MaxTasks         int           `yaml:"max_tasks"`
	CheckInterval    time.Duration `yaml:"check_interval"`
	DefaultTimeout   time.Duration `yaml:"default_timeout"`
	MaxConcurrent    int           `yaml:"max_concurrent"`
	RetryAttempts    int           `yaml:"retry_attempts"`
	RetryDelay       time.Duration `yaml:"retry_delay"`
	LogLevel         string        `yaml:"log_level"`
	EnableStats      bool          `yaml:"enable_stats"`
	TaskHistorySize  int           `yaml:"task_history_size"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusPaused     TaskStatus = "paused"
	TaskStatusRetry      TaskStatus = "retry"
)

// TaskPriority represents task priority levels
type TaskPriority int

const (
	TaskPriorityLow    TaskPriority = 1
	TaskPriorityNormal TaskPriority = 2
	TaskPriorityHigh   TaskPriority = 3
	TaskPriorityCritical TaskPriority = 4
)

// Task represents a task to be executed
type Task struct {
	ID            string                 `yaml:"id"`
	Name          string                 `yaml:"name"`
	Description   string                 `yaml:"description"`
	Command       string                 `yaml:"command"`
	Args          []string               `yaml:"args"`
	WorkingDir    string                 `yaml:"working_dir"`
	Status        TaskStatus             `yaml:"status"`
	Priority      TaskPriority           `yaml:"priority"`
	Timeout       time.Duration          `yaml:"timeout"`
	MaxRetries    int                    `yaml:"max_retries"`
	RetryCount    int                    `yaml:"retry_count"`
	Dependencies  []string               `yaml:"dependencies"`
	CreatedAt     time.Time              `yaml:"created_at"`
	StartedAt     time.Time              `yaml:"started_at"`
	CompletedAt   time.Time              `yaml:"completed_at"`
	ExitCode      int                    `yaml:"exit_code"`
	PID           int32                  `yaml:"pid"`
	LogPath       string                 `yaml:"log_path"`
	EnvVars       map[string]string      `yaml:"env_vars"`
	ResourceQuota string                 `yaml:"resource_quota"`
	Tags          []string               `yaml:"tags"`
	Metadata      map[string]interface{} `yaml:"metadata"`
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID      string        `yaml:"task_id"`
	ExitCode    int           `yaml:"exit_code"`
	Output      string        `yaml:"output"`
	Error       string        `yaml:"error"`
	Duration    time.Duration `yaml:"duration"`
	MemoryUsed  int64         `yaml:"memory_used"`
	CPUUsed     float64       `yaml:"cpu_used"`
	LogPath     string        `yaml:"log_path"`
	Timestamp   time.Time     `yaml:"timestamp"`
}

// StorageConfig represents storage management configuration
type StorageConfig struct {
	MaxFileSizeMB      int     `yaml:"max_file_size_mb"`      // Maximum file size in MB before rotation
	MaxFiles           int     `yaml:"max_files"`              // Maximum number of files to keep
	CompressAfterDays  int     `yaml:"compress_after_days"`   // Compress files after N days
	CleanupAfterDays   int     `yaml:"cleanup_after_days"`    // Delete files after N days
	AutoCleanup        bool    `yaml:"auto_cleanup"`          // Enable automatic cleanup
}

// getDefaultConfig returns the default configuration
func GetDefaultConfig() Config {
	return Config{
		StatisticsGranularity: "detailed",  // simple, detailed, full
		ShowCommands:         true,
		ShowWorkingDirs:      true,
		UseSmartCategories:   true,
		MaxCommandLength:     100,
		MaxDirLength:         50,
		Storage: StorageConfig{
			MaxFileSizeMB:     100,
			MaxFiles:         10,
			CompressAfterDays: 3,
			CleanupAfterDays:  30,
			AutoCleanup:      true,
		},
		ProcessControl: ProcessControlConfig{
			Enabled:           true,
			EnableAutoRestart: true,
			MaxRestarts:       3,
			RestartDelay:      5 * time.Second,
			CheckInterval:     10 * time.Second,
			ManagedProcesses:  []ManagedProcessConfig{},
		},
		ResourceQuota: ResourceQuotaConfig{
			Enabled:          true,
			CheckInterval:    30 * time.Second,
			DefaultAction:    ActionWarn,
			MaxViolations:    5,
			ViolationWindow: 5 * time.Minute,
			Quotas:          []ResourceQuota{},
		},
		ProcessDiscovery: ProcessDiscoveryConfig{
			Enabled:           true,
			DiscoveryInterval: 15 * time.Second,
			AutoManage:        true,
			BioToolsOnly:      true,
			ProcessPatterns:   []string{},
			ExcludePatterns:   []string{"^$", "^systemd$", "^kworker", "^dbus.*"},
			MaxProcesses:      1000,
			CPUThreshold:      50.0,
			MemoryThresholdMB: 1024,
		},
		TaskManager: TaskManagerConfig{
			Enabled:          true,
			MaxTasks:         100,
			CheckInterval:    10 * time.Second,
			DefaultTimeout:    30 * time.Minute,
			MaxConcurrent:    5,
			RetryAttempts:    3,
			RetryDelay:       5 * time.Second,
			LogLevel:         "info",
			EnableStats:      true,
			TaskHistorySize:  1000,
		},
		HealthCheck: HealthCheckConfig{
			Enabled:              true,
			CheckInterval:        30 * time.Second,
			EnableProcessChecks:  true,
			EnableResourceChecks: true,
			EnableTaskChecks:     true,
			EnableSystemChecks:   true,
			AlertManager: AlertManagerConfig{
				Enabled:         true,
				CheckInterval:   10 * time.Second,
				RetryInterval:   30 * time.Second,
				MaxRetries:     3,
				NotificationChannels: []NotificationChannelConfig{
					{
						Name:    "console",
						Type:    NotificationTypeConsole,
						Enabled: true,
						Config:  map[string]interface{}{},
						Rules:   []string{"*"},
					},
					{
						Name:    "log",
						Type:    NotificationTypeLog,
						Enabled: true,
						Config: map[string]interface{}{
							"log_file": "logs/alerts.log",
						},
						Rules: []string{"*"},
					},
				},
				AlertHistorySize: 1000,
			},
			HealthRules: []HealthRule{
				{
					ID:          "high_cpu_usage",
					Name:        "High CPU Usage",
					Description: "Alert when CPU usage exceeds threshold",
					Type:        HealthCheckTypeProcess,
					Enabled:     true,
					Severity:    AlertSeverityWarning,
					Conditions: []HealthCondition{
						{
							Metric:    "cpu_percent",
							Operator:  OpGreaterThan,
							Threshold: 80.0,
							Duration:  5 * time.Minute,
							Count:     3,
						},
					},
					Actions: []AlertAction{
						{
							Type:    ActionTypeNotify,
							Channel: "console",
							Enabled: true,
						},
						{
							Type:    ActionTypeLog,
							Channel: "log",
							Enabled: true,
						},
					},
				},
				{
					ID:          "high_memory_usage",
					Name:        "High Memory Usage",
					Description: "Alert when memory usage exceeds threshold",
					Type:        HealthCheckTypeProcess,
					Enabled:     true,
					Severity:    AlertSeverityWarning,
					Conditions: []HealthCondition{
						{
							Metric:    "memory_mb",
							Operator:  OpGreaterThan,
							Threshold: 2048.0,
							Duration:  5 * time.Minute,
							Count:     3,
						},
					},
					Actions: []AlertAction{
						{
							Type:    ActionTypeNotify,
							Channel: "console",
							Enabled: true,
						},
					},
				},
				{
					ID:          "task_failure",
					Name:        "Task Failure",
					Description: "Alert when tasks fail repeatedly",
					Type:        HealthCheckTypeTask,
					Enabled:     true,
					Severity:    AlertSeverityError,
					Conditions: []HealthCondition{
						{
							Metric:    "failure_count",
							Operator:  OpGreaterEqual,
							Threshold: 3.0,
							Duration:  10 * time.Minute,
							Count:     1,
						},
					},
					Actions: []AlertAction{
						{
							Type:    ActionTypeNotify,
							Channel: "console",
							Enabled: true,
						},
					},
				},
				{
					ID:          "system_load_high",
					Name:        "High System Load",
					Description: "Alert when system load average is high",
					Type:        HealthCheckTypeSystem,
					Enabled:     true,
					Severity:    AlertSeverityWarning,
					Conditions: []HealthCondition{
						{
							Metric:    "load_average",
							Operator:  OpGreaterThan,
							Threshold: 2.0,
							Duration:  5 * time.Minute,
							Count:     3,
						},
					},
					Actions: []AlertAction{
						{
							Type:    ActionTypeNotify,
							Channel: "console",
							Enabled: true,
						},
					},
				},
			},
		},
	}
}

// ResourceRecord represents detailed process resource usage
type ResourceRecord struct {
	Name        string
	Timestamp   time.Time
	CPUPercent  float64   // CPU usage percentage
	MemoryMB    float64   // Memory usage in MB
	Threads     int32     // Number of threads
	DiskReadMB  float64   // Disk read in MB
	DiskWriteMB float64   // Disk write in MB
	NetSentKB   float64   // Network sent in KB
	NetRecvKB   float64   // Network received in KB
	IsActive    bool      // Process is actively being used
	Command     string    // Full command line
	WorkingDir  string    // Working directory
	Category    string    // Application category for better classification
}

// ResourceStats represents accumulated resource statistics
type ResourceStats struct {
	Name         string
	Samples      int     // Number of samples
	ActiveSamples int     // Number of active samples
	CPUAvg       float64 // Average CPU percentage
	CPUMax       float64 // Maximum CPU percentage
	MemoryAvg    float64 // Average memory in MB
	MemoryMax    float64 // Maximum memory in MB
	ThreadsAvg   float64 // Average thread count
	DiskReadAvg  float64 // Average disk read in MB
	DiskWriteAvg float64 // Average disk write in MB
	NetSentAvg   float64 // Average network sent in KB
	NetRecvAvg   float64 // Average network received in KB
	ActiveTime   time.Duration // Total active time based on interval
	Category     string  // Application category
	Command      string  // Command line (first sample)
	WorkingDir   string  // Working directory (first sample)
}

// ActivityConfig defines thresholds for active detection
type ActivityConfig struct {
	CPUThreshold    float64 // Minimum CPU percentage to be considered active
	MemoryThreshold float64 // Minimum memory change in MB
	DiskThreshold   float64 // Minimum disk I/O in MB
	NetThreshold    float64 // Minimum network activity in KB
}

// Data format versions
const (
	DataFormatV1 = 1 // v0.1.2 format: 9 fields (timestamp,name,cpu,memory,threads,diskRead,diskWrite,netSent,netRecv)
	DataFormatV2 = 2 // v0.1.3+ format: 10 fields (adds isActive)
	DataFormatV3 = 3 // v0.2.1+ format: 13 fields (adds command, workingDir, category)
)

// GetDefaultActivityConfig returns default activity thresholds
func GetDefaultActivityConfig() ActivityConfig {
	return ActivityConfig{
		CPUThreshold:    1.0,    // 1% CPU usage
		MemoryThreshold: 0.5,    // 0.5MB memory change
		DiskThreshold:   0.1,    // 0.1MB disk I/O
		NetThreshold:    1.0,    // 1KB network activity
	}
}

// IsActive determines if a process is actively being used based on resource usage
func IsActive(resource ResourceRecord, config ActivityConfig) bool {
	if resource.CPUPercent >= config.CPUThreshold {
		return true
	}
	if resource.DiskReadMB >= config.DiskThreshold || resource.DiskWriteMB >= config.DiskThreshold {
		return true
	}
	if resource.NetSentKB >= config.NetThreshold || resource.NetRecvKB >= config.NetThreshold {
		return true
	}
	if resource.MemoryMB >= 50.0 {
		return true
	}
	return false
}

// IdentifyApplication identifies applications based on process name and command line
func IdentifyApplication(name, cmd string, useSmartCategories bool) string {
	if !useSmartCategories {
		return name
	}

	cmdLower := strings.ToLower(cmd)
	nameLower := strings.ToLower(name)

	// Java applications
	if nameLower == "java" {
		switch {
		case strings.Contains(cmdLower, "tika-server"):
			return "Tika Server"
		case strings.Contains(cmdLower, "spring-boot"):
			return "Spring Boot Application"
		case strings.Contains(cmdLower, "elasticsearch"):
			return "Elasticsearch"
		case strings.Contains(cmdLower, "kafka"):
			return "Kafka"
		case strings.Contains(cmdLower, "tomcat"):
			return "Tomcat"
		case strings.Contains(cmdLower, "jetty"):
			return "Jetty"
		case strings.Contains(cmdLower, "wildfly") || strings.Contains(cmdLower, "jboss"):
			return "WildFly/JBoss"
		case strings.Contains(cmdLower, "gradle"):
			return "Gradle Build"
		case strings.Contains(cmdLower, "maven"):
			return "Maven Build"
		default:
			// Extract main class or JAR name
			if strings.Contains(cmdLower, ".jar") {
				jarMatch := regexp.MustCompile(`([a-zA-Z0-9_-]+)\.jar`).FindStringSubmatch(cmd)
				if len(jarMatch) > 1 {
					return "Java: " + jarMatch[1]
				}
			}
			return "Java Application"
		}
	}

	// Node.js applications
	if nameLower == "node" || strings.Contains(cmdLower, "node") {
		if strings.Contains(cmdLower, "npm") {
			return ExtractNpmPackageName(cmd)
		}
		if strings.Contains(cmdLower, "yarn") {
			return "Yarn: " + extractYarnPackageName(cmd)
		}
		if strings.Contains(cmdLower, "next") {
			return "Next.js"
		}
		if strings.Contains(cmdLower, "react") {
			return "React Application"
		}
		if strings.Contains(cmdLower, "vue") {
			return "Vue.js Application"
		}
		if strings.Contains(cmdLower, "express") {
			return "Express.js"
		}
		if strings.Contains(cmdLower, "nest") {
			return "NestJS"
		}
		return "Node.js Application"
	}

	// Python applications
	if nameLower == "python" || nameLower == "python3" {
		if strings.Contains(cmdLower, "uv run") {
			return ExtractPythonProjectName(cmd)
		}
		if strings.Contains(cmdLower, "pip") {
			return "Pip Package Manager"
		}
		if strings.Contains(cmdLower, "conda") {
			return "Conda"
		}
		if strings.Contains(cmdLower, "jupyter") {
			return "Jupyter"
		}
		if strings.Contains(cmdLower, "django") {
			return "Django"
		}
		if strings.Contains(cmdLower, "flask") {
			return "Flask"
		}
		if strings.Contains(cmdLower, "fastapi") {
			return "FastAPI"
		}
		if strings.Contains(cmdLower, "streamlit") {
			return "Streamlit"
		}
		if strings.Contains(cmdLower, "pytest") {
			return "Pytest"
		}
		if strings.Contains(cmdLower, "gunicorn") {
			return "Gunicorn"
		}
		if strings.Contains(cmdLower, "uvicorn") {
			return "Uvicorn"
		}
		return "Python Application"
	}

	// Go applications
	if nameLower == "go" {
		if strings.Contains(cmdLower, "test") {
			return "Go Test"
		}
		if strings.Contains(cmdLower, "build") {
			return "Go Build"
		}
		if strings.Contains(cmdLower, "run") {
			return "Go Run"
		}
		return "Go Compiler"
	}

	// Rust applications
	if nameLower == "cargo" {
		if strings.Contains(cmdLower, "run") {
			return "Cargo Run"
		}
		if strings.Contains(cmdLower, "build") {
			return "Cargo Build"
		}
		if strings.Contains(cmdLower, "test") {
			return "Cargo Test"
		}
		return "Cargo"
	}

	// Databases
	if strings.Contains(nameLower, "mysql") || strings.Contains(nameLower, "mysqld") {
		return "MySQL"
	}
	if strings.Contains(nameLower, "postgres") || strings.Contains(nameLower, "postgresql") {
		return "PostgreSQL"
	}
	if strings.Contains(nameLower, "mongodb") || strings.Contains(nameLower, "mongod") {
		return "MongoDB"
	}
	if strings.Contains(nameLower, "redis") {
		return "Redis"
	}
	if strings.Contains(nameLower, "sqlite") {
		return "SQLite"
	}

	// Web servers
	if strings.Contains(nameLower, "nginx") {
		return "Nginx"
	}
	if strings.Contains(nameLower, "apache") || strings.Contains(nameLower, "httpd") {
		return "Apache"
	}

	// Browsers
	if strings.Contains(nameLower, "chrome") || strings.Contains(nameLower, "chromium") {
		return "Chrome/Chromium"
	}
	if strings.Contains(nameLower, "firefox") {
		return "Firefox"
	}
	if strings.Contains(nameLower, "safari") {
		return "Safari"
	}

	// Development tools
	if strings.Contains(nameLower, "git") {
		return "Git"
	}
	if strings.Contains(nameLower, "docker") {
		return "Docker"
	}
	if strings.Contains(nameLower, "kubectl") {
		return "Kubernetes (kubectl)"
	}
	if strings.Contains(nameLower, "vim") || strings.Contains(nameLower, "nvim") {
		return "Vim/Neovim"
	}
	if strings.Contains(nameLower, "code") {
		return "VS Code"
	}
	if strings.Contains(nameLower, "idea") {
		return "IntelliJ IDEA"
	}

	// System tools
	if strings.Contains(nameLower, "bash") || strings.Contains(nameLower, "zsh") {
		return "Shell"
	}
	if strings.Contains(nameLower, "ssh") {
		return "SSH"
	}
	if strings.Contains(nameLower, "scp") {
		return "SCP"
	}
	if strings.Contains(nameLower, "rsync") {
		return "Rsync"
	}
	if strings.Contains(nameLower, "wget") || strings.Contains(nameLower, "curl") {
		return "HTTP Client"
	}

	return name
}

// ExtractNpmPackageName extracts package name from npm command
func ExtractNpmPackageName(cmd string) string {
	cmdLower := strings.ToLower(cmd)
	
	// Try to extract script name from package.json
	if strings.Contains(cmdLower, "run") {
		parts := strings.Fields(cmd)
		for i, part := range parts {
			if part == "run" && i+1 < len(parts) {
				return "NPM: " + parts[i+1]
			}
		}
	}
	
	// Extract from command
	if strings.Contains(cmdLower, "start") {
		return "NPM: start"
	}
	if strings.Contains(cmdLower, "dev") {
		return "NPM: dev"
	}
	if strings.Contains(cmdLower, "test") {
		return "NPM: test"
	}
	if strings.Contains(cmdLower, "build") {
		return "NPM: build"
	}
	
	return "NPM Script"
}

// extractYarnPackageName extracts package name from yarn command
func extractYarnPackageName(cmd string) string {
	cmdLower := strings.ToLower(cmd)
	
	if strings.Contains(cmdLower, "run") {
		parts := strings.Fields(cmd)
		for i, part := range parts {
			if part == "run" && i+1 < len(parts) {
				return "Yarn: " + parts[i+1]
			}
		}
	}
	
	if strings.Contains(cmdLower, "start") {
		return "Yarn: start"
	}
	if strings.Contains(cmdLower, "dev") {
		return "Yarn: dev"
	}
	if strings.Contains(cmdLower, "test") {
		return "Yarn: test"
	}
	if strings.Contains(cmdLower, "build") {
		return "Yarn: build"
	}
	
	return "Yarn Script"
}

// ExtractPythonProjectName extracts project name from Python command
func ExtractPythonProjectName(cmd string) string {
	cmdLower := strings.ToLower(cmd)
	
	// Extract script or module name
	if strings.Contains(cmdLower, "uv run") {
		parts := strings.Fields(cmd)
		// Find the argument after "run"
		for i, part := range parts {
			if part == "run" && i+1 < len(parts) {
				scriptName := parts[i+1]
				// Extract just the filename without path and extension
				base := filepath.Base(scriptName)
				if strings.Contains(base, ".py") {
					return "UV: " + strings.TrimSuffix(base, ".py")
				}
				return "UV: " + base
			}
		}
	}
	
	// Try to extract module name
	if strings.Contains(cmdLower, "-m") {
		parts := strings.Fields(cmd)
		for i, part := range parts {
			if part == "-m" && i+1 < len(parts) {
				return "Python: " + parts[i+1]
			}
		}
	}
	
	return "Python: uv run"
}

// ExtractProjectName extracts project name from working directory
func ExtractProjectName(cwd string) string {
	if cwd == "" {
		return ""
	}
	
	// Extract project directory name
	parts := strings.Split(cwd, "/")
	if len(parts) > 0 {
		projectName := parts[len(parts)-1]
		// Clean up common project suffixes
		projectName = strings.TrimSuffix(projectName, "-main")
		projectName = strings.TrimSuffix(projectName, "-master")
		projectName = strings.TrimSuffix(projectName, ".git")
		return projectName
	}
	
	return ""
}

// TruncateString truncates a string to maximum length
func TruncateString(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ParseResourceLineV1 parses V1 format line (9 fields)
func ParseResourceLineV1(line string) (ResourceRecord, error) {
	fields := strings.Split(line, ",")
	if len(fields) != 9 {
		return ResourceRecord{}, fmt.Errorf("invalid V1 line: expected 9 fields, got %d", len(fields))
	}

	timestamp, err := time.Parse("2006-01-02 15:04:05", fields[0])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid timestamp: %v", err)
	}

	cpuPercent, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid CPU percent: %v", err)
	}

	memoryMB, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid memory: %v", err)
	}

	threads, err := strconv.ParseInt(fields[4], 10, 32)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid threads: %v", err)
	}

	diskReadMB, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk read: %v", err)
	}

	diskWriteMB, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk write: %v", err)
	}

	netSentKB, err := strconv.ParseFloat(fields[7], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net sent: %v", err)
	}

	netRecvKB, err := strconv.ParseFloat(fields[8], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net recv: %v", err)
	}

	return ResourceRecord{
		Name:        fields[1],
		Timestamp:   timestamp,
		CPUPercent:  cpuPercent,
		MemoryMB:    memoryMB,
		Threads:     int32(threads),
		DiskReadMB:  diskReadMB,
		DiskWriteMB: diskWriteMB,
		NetSentKB:   netSentKB,
		NetRecvKB:   netRecvKB,
		IsActive:    false, // V1 doesn't have this field
		Command:     "",
		WorkingDir:  "",
		Category:    "",
	}, nil
}

// ParseResourceLineV2 parses V2 format line (10 fields)
func ParseResourceLineV2(line string) (ResourceRecord, error) {
	fields := strings.Split(line, ",")
	if len(fields) != 10 {
		return ResourceRecord{}, fmt.Errorf("invalid V2 line: expected 10 fields, got %d", len(fields))
	}

	timestamp, err := time.Parse("2006-01-02 15:04:05", fields[0])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid timestamp: %v", err)
	}

	cpuPercent, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid CPU percent: %v", err)
	}

	memoryMB, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid memory: %v", err)
	}

	threads, err := strconv.ParseInt(fields[4], 10, 32)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid threads: %v", err)
	}

	diskReadMB, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk read: %v", err)
	}

	diskWriteMB, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk write: %v", err)
	}

	netSentKB, err := strconv.ParseFloat(fields[7], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net sent: %v", err)
	}

	netRecvKB, err := strconv.ParseFloat(fields[8], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net recv: %v", err)
	}

	isActive, err := strconv.ParseBool(fields[9])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid isActive: %v", err)
	}

	return ResourceRecord{
		Name:        fields[1],
		Timestamp:   timestamp,
		CPUPercent:  cpuPercent,
		MemoryMB:    memoryMB,
		Threads:     int32(threads),
		DiskReadMB:  diskReadMB,
		DiskWriteMB: diskWriteMB,
		NetSentKB:   netSentKB,
		NetRecvKB:   netRecvKB,
		IsActive:    isActive,
		Command:     "",
		WorkingDir:  "",
		Category:    "",
	}, nil
}

// ParseResourceLineV3 parses V3 format line (13 fields)
func ParseResourceLineV3(line string) (ResourceRecord, error) {
	// Handle quoted fields (command and working directory might contain commas)
	var fields []string
	var inQuotes bool
	var currentField strings.Builder

	for _, char := range line {
		if char == '"' {
			inQuotes = !inQuotes
		} else if char == ',' && !inQuotes {
			fields = append(fields, currentField.String())
			currentField.Reset()
		} else {
			currentField.WriteRune(char)
		}
	}
	// Add the last field
	fields = append(fields, currentField.String())

	if len(fields) != 13 {
		return ResourceRecord{}, fmt.Errorf("invalid V3 line: expected 13 fields, got %d", len(fields))
	}

	timestamp, err := time.Parse("2006-01-02 15:04:05", fields[0])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid timestamp: %v", err)
	}

	cpuPercent, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid CPU percent: %v", err)
	}

	memoryMB, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid memory: %v", err)
	}

	threads, err := strconv.ParseInt(fields[4], 10, 32)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid threads: %v", err)
	}

	diskReadMB, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk read: %v", err)
	}

	diskWriteMB, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid disk write: %v", err)
	}

	netSentKB, err := strconv.ParseFloat(fields[7], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net sent: %v", err)
	}

	netRecvKB, err := strconv.ParseFloat(fields[8], 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid net recv: %v", err)
	}

	isActive, err := strconv.ParseBool(fields[9])
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid isActive: %v", err)
	}

	return ResourceRecord{
		Name:        fields[1],
		Timestamp:   timestamp,
		CPUPercent:  cpuPercent,
		MemoryMB:    memoryMB,
		Threads:     int32(threads),
		DiskReadMB:  diskReadMB,
		DiskWriteMB: diskWriteMB,
		NetSentKB:   netSentKB,
		NetRecvKB:   netRecvKB,
		IsActive:    isActive,
		Command:     fields[10],
		WorkingDir:  fields[11],
		Category:    fields[12],
	}, nil
}

// HealthCheckConfig configures health check and alerting system
type HealthCheckConfig struct {
	Enabled              bool             `yaml:"enabled"`
	CheckInterval        time.Duration    `yaml:"check_interval"`
	EnableProcessChecks  bool             `yaml:"enable_process_checks"`
	EnableResourceChecks bool             `yaml:"enable_resource_checks"`
	EnableTaskChecks     bool             `yaml:"enable_task_checks"`
	EnableSystemChecks   bool             `yaml:"enable_system_checks"`
	AlertManager         AlertManagerConfig `yaml:"alert_manager"`
	HealthRules          []HealthRule     `yaml:"health_rules"`
}

// AlertManagerConfig configures alert management and notification
type AlertManagerConfig struct {
	Enabled         bool          `yaml:"enabled"`
	CheckInterval   time.Duration `yaml:"check_interval"`
	RetryInterval   time.Duration `yaml:"retry_interval"`
	MaxRetries     int           `yaml:"max_retries"`
	NotificationChannels []NotificationChannelConfig `yaml:"notification_channels"`
	AlertHistorySize int         `yaml:"alert_history_size"`
}

// NotificationChannelConfig defines a notification channel
type NotificationChannelConfig struct {
	Name     string                 `yaml:"name"`
	Type     NotificationType       `yaml:"type"`
	Enabled  bool                   `yaml:"enabled"`
	Config   map[string]interface{} `yaml:"config"`
	Rules    []string               `yaml:"rules"` // Health rule names that use this channel
}

// NotificationType represents the type of notification channel
type NotificationType string

const (
	NotificationTypeEmail    NotificationType = "email"
	NotificationTypeWebhook  NotificationType = "webhook"
	NotificationTypeSlack    NotificationType = "slack"
	NotificationTypeTelegram NotificationType = "telegram"
	NotificationTypeDiscord  NotificationType = "discord"
	NotificationTypeConsole  NotificationType = "console"
	NotificationTypeLog      NotificationType = "log"
)

// HealthRule defines a health check rule
type HealthRule struct {
	ID          string                 `yaml:"id"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Type        HealthCheckType        `yaml:"type"`
	Enabled     bool                   `yaml:"enabled"`
	Severity    AlertSeverity          `yaml:"severity"`
	Conditions  []HealthCondition      `yaml:"conditions"`
	Actions     []AlertAction          `yaml:"actions"`
	Metadata    map[string]interface{} `yaml:"metadata"`
}

// HealthCheckType represents the type of health check
type HealthCheckType string

const (
	HealthCheckTypeProcess  HealthCheckType = "process"
	HealthCheckTypeResource HealthCheckType = "resource"
	HealthCheckTypeTask     HealthCheckType = "task"
	HealthCheckTypeSystem   HealthCheckType = "system"
)

// HealthCondition defines a condition for health checks
type HealthCondition struct {
	Metric     string      `yaml:"metric"`
	Operator   ComparisonOp `yaml:"operator"`
	Threshold  float64     `yaml:"threshold"`
	Duration   time.Duration `yaml:"duration"`
	Count      int         `yaml:"count"` // Number of consecutive violations
}

// ComparisonOp represents comparison operators
type ComparisonOp string

const (
	OpGreaterThan    ComparisonOp = ">"
	OpGreaterEqual    ComparisonOp = ">="
	OpLessThan        ComparisonOp = "<"
	OpLessEqual       ComparisonOp = "<="
	OpEqual           ComparisonOp = "=="
	OpNotEqual        ComparisonOp = "!="
)

// AlertSeverity represents alert severity levels
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityError    AlertSeverity = "error"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertAction defines actions to take when an alert is triggered
type AlertAction struct {
	Type        ActionType             `yaml:"type"`
	Channel     string                 `yaml:"channel"`     // Channel name for notifications
	Parameters  map[string]interface{} `yaml:"parameters"`
	Timeout     time.Duration          `yaml:"timeout"`
	Enabled     bool                   `yaml:"enabled"`
}

// ActionType represents alert action types
type ActionType string

const (
	ActionTypeNotify   ActionType = "notify"
	ActionTypeRestart  ActionType = "restart"
	ActionTypeStop     ActionType = "stop"
	ActionTypeThrottle ActionType = "throttle"
	ActionTypeExecute  ActionType = "execute"
	ActionTypeLog      ActionType = "log"
)

// HealthStatus represents health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusWarning   HealthStatus = "warning"
	HealthStatusCritical  HealthStatus = "critical"
	HealthStatusUnknown  HealthStatus = "unknown"
)

// HealthCheck represents a health check result
type HealthCheck struct {
	ID          string                 `yaml:"id"`
	Name        string                 `yaml:"name"`
	Type        HealthCheckType        `yaml:"type"`
	Status      HealthStatus           `yaml:"status"`
	Score       float64                `yaml:"score"` // 0-100 health score
	Message     string                 `yaml:"message"`
	Details     map[string]interface{} `yaml:"details"`
	Timestamp   time.Time              `yaml:"timestamp"`
	Duration    time.Duration          `yaml:"duration"`
	Tags        []string               `yaml:"tags"`
}

// Alert represents an alert notification
type Alert struct {
	ID          string                 `yaml:"id"`
	RuleID      string                 `yaml:"rule_id"`
	RuleName    string                 `yaml:"rule_name"`
	Type        HealthCheckType        `yaml:"type"`
	Severity    AlertSeverity          `yaml:"severity"`
	Status      AlertStatus            `yaml:"status"`
	Title       string                 `yaml:"title"`
	Message     string                 `yaml:"message"`
	Details     map[string]interface{} `yaml:"details"`
	TriggeredAt time.Time              `yaml:"triggered_at"`
	UpdatedAt   time.Time              `yaml:"updated_at"`
	ResolvedAt  time.Time              `yaml:"resolved_at"`
	RetryCount  int                    `yaml:"retry_count"`
	Tags        []string               `yaml:"tags"`
	Actions     []AlertAction          `yaml:"actions"`
}

// AlertStatus represents alert status
type AlertStatus string

const (
	AlertStatusActive    AlertStatus = "active"
	AlertStatusResolved  AlertStatus = "resolved"
	AlertStatusSuppressed AlertStatus = "suppressed"
	AlertStatusExpired   AlertStatus = "expired"
)

// SystemMetrics represents system-wide metrics
type SystemMetrics struct {
	CPUUsage     float64 `yaml:"cpu_usage"`     // Overall CPU usage
	MemoryUsage  float64 `yaml:"memory_usage"`  // Overall memory usage
	DiskUsage    float64 `yaml:"disk_usage"`    // Overall disk usage
	LoadAverage  float64 `yaml:"load_average"`  // System load average
	ProcessCount int64   `yaml:"process_count"` // Total process count
	NetworkIn    float64 `yaml:"network_in"`    // Network inbound KB/s
	NetworkOut   float64 `yaml:"network_out"`   // Network outbound KB/s
	Timestamp    time.Time `yaml:"timestamp"`
}