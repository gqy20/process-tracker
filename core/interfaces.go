package core

import (
	"time"
)

// ProcessStats 统一的进程状态统计
type ProcessStats struct {
	PID             int32     `json:"pid"`
	Name            string    `json:"name"`
	Cmdline         string    `json:"cmdline"`
	Status          string    `json:"status"`
	CPUUsed         float64   `json:"cpu_used"`
	MemoryUsedMB    int64     `json:"memory_used_mb"`
	IOReadBytes     uint64    `json:"io_read_bytes"`
	IOWriteBytes    uint64    `json:"io_write_bytes"`
	IOReadCount     uint64    `json:"io_read_count"`
	IOWriteCount    uint64    `json:"io_write_count"`
	FileDescriptors uint32    `json:"file_descriptors"`
	Threads         uint32    `json:"threads"`
	LastUpdated     time.Time `json:"last_updated"`
}

// ResourceUsage 统一的资源使用情况
type ResourceUsage struct {
	CPUUsed        float64   `json:"cpu_used"`
	CPUExpected    float64   `json:"cpu_expected"`
	MemoryUsedMB   int64     `json:"memory_used_mb"`
	MemoryExpected int64     `json:"memory_expected"`
	DiskReadMB     int64     `json:"disk_read_mb"`
	DiskWriteMB    int64     `json:"disk_write_mb"`
	NetworkInKB    int64     `json:"network_in_kb"`
	NetworkOutKB   int64     `json:"network_out_kb"`
	PerformanceScore float64 `json:"performance_score"`
	AnomalyType    []string  `json:"anomaly_type"`
	LastAnomaly    time.Time `json:"last_anomaly"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	IsHealthy    bool      `json:"is_healthy"`
	Status       string    `json:"status"`
	Score        float64   `json:"score"`
	Issues       []string  `json:"issues"`
	LastCheck    time.Time `json:"last_check"`
	NextCheck    time.Time `json:"next_check"`
}

// PerformanceRecord 性能记录
type PerformanceRecord struct {
	Timestamp       time.Time  `json:"timestamp"`
	PID             int32      `json:"pid"`
	CPUUsed         float64    `json:"cpu_used"`
	MemoryUsedMB    int64      `json:"memory_used_mb"`
	IOReadBytes     uint64     `json:"io_read_bytes"`
	IOWriteBytes    uint64     `json:"io_write_bytes"`
	PerformanceScore float64   `json:"performance_score"`
	Status          string     `json:"status"`
	Tags            map[string]string `json:"tags"`
}

// ProcessMonitor 统一进程监控接口
type ProcessMonitor interface {
	// 基础监控和生命周期
	Start() error
	Stop()
	GetStats() map[string]interface{}
}

// ResourceCollector 统一资源收集器接口
type ResourceCollector interface {
	// CPU相关
	CollectCPU(pid int32) (float64, error)
	CollectCPUHistory(pid int32, duration time.Duration) ([]float64, error)
	
	// 内存相关
	CollectMemory(pid int32) (int64, error)
	CollectMemoryHistory(pid int32, duration time.Duration) ([]int64, error)
	
	// I/O相关
	CollectIO(pid int32) (*IOStats, error)
	CollectIOHistory(pid int32, duration time.Duration) ([]*IOStats, error)
	
	// 系统资源
	CollectSystemResources() (*SystemResources, error)
	
	// 清理
	CleanupOldRecords(maxAge time.Duration)
}

// IOStats I/O统计信息
type IOStats struct {
	ReadBytes     uint64    `json:"read_bytes"`
	WriteBytes    uint64    `json:"write_bytes"`
	ReadCount     uint64    `json:"read_count"`
	WriteCount    uint64    `json:"write_count"`
	Timestamp     time.Time `json:"timestamp"`
}

// SystemResources 系统资源信息
type SystemResources struct {
	CPUCount        int     `json:"cpu_count"`
	CPUUsage        float64 `json:"cpu_usage"`
	TotalMemory     uint64  `json:"total_memory"`
	AvailableMemory uint64  `json:"available_memory"`
	UsedMemory      uint64  `json:"used_memory"`
	MemoryUsage     float64 `json:"memory_usage"`
	LoadAvg1        float64 `json:"load_avg_1"`
	LoadAvg5        float64 `json:"load_avg_5"`
	LoadAvg15       float64 `json:"load_avg_15"`
	LastUpdated     time.Time `json:"last_updated"`
}

// HealthChecker 统一健康检查接口
type HealthChecker interface {
	// 基础健康检查
	CheckHealth() *HealthStatus
}

// HealthCheckRule 健康检查规则
type HealthCheckRule struct {
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Metric      string            `json:"metric"`
	Operator    string            `json:"operator"`    // ">", "<", "==", "!=", ">=", "<="
	Threshold   float64           `json:"threshold"`
	Severity    string            `json:"severity"`    // "info", "warning", "error", "critical"
	Action      string            `json:"action"`      // "alert", "restart", "stop", "log"
	Description string            `json:"description"`
	Tags        map[string]string `json:"tags"`
	Enabled     bool              `json:"enabled"`
}

// Event 统一事件结构
type Event struct {
	ID          string                 `json:"id"`
	Type        EventType              `json:"type"`
	Source      string                 `json:"source"`
	Level       EventLevel             `json:"level"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	PID         int32                  `json:"pid,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Tags        []string               `json:"tags"`
}

// EventType 事件类型
type EventType string

const (
	EventProcessStarted   EventType = "process_started"
	EventProcessStopped   EventType = "process_stopped"
	EventProcessCrashed   EventType = "process_crashed"
	EventHealthCheckFailed EventType = "health_check_failed"
	EventResourceAlert    EventType = "resource_alert"
	EventPerformanceIssue EventType = "performance_issue"
	EventCustom          EventType = "custom"
)

// EventLevel 事件级别
type EventLevel string

const (
	LevelDebug   EventLevel = "debug"
	LevelInfo    EventLevel = "info"
	LevelWarning EventLevel = "warning"
	LevelError   EventLevel = "error"
	LevelCritical EventLevel = "critical"
)

// EventHandler 事件处理器接口
type EventHandler interface {
	HandleEvent(event Event) error
	GetEventHistory(filter EventFilter, limit int) ([]Event, error)
	ClearOldEvents(maxAge time.Duration) error
}

// EventFilter 事件过滤器
type EventFilter struct {
	Types      []EventType   `json:"types,omitempty"`
	Levels     []EventLevel  `json:"levels,omitempty"`
	Sources    []string      `json:"sources,omitempty"`
	PIDs       []int32       `json:"pids,omitempty"`
	StartTime  time.Time     `json:"start_time,omitempty"`
	EndTime    time.Time     `json:"end_time,omitempty"`
	Tags       []string      `json:"tags,omitempty"`
}

// ConfigManager 统一配置管理接口
type ConfigManager interface {
	// 基础配置管理
	GetConfig(key string) (interface{}, bool)
	SetConfig(key string, value interface{}) error
	
	// 配置分组
	GetGroupConfig(group string) (map[string]interface{}, error)
	SetGroupConfig(group string, config map[string]interface{}) error
	
	// 动态配置
	WatchConfig(key string, callback func(interface{})) error
	UnwatchConfig(key string)
	
	// 配置验证
	ValidateConfig(config map[string]interface{}) error
	
	// 配置持久化
	LoadConfig() error
	SaveConfig() error
}

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	// 指标记录
	RecordCounter(name string, value int64, tags map[string]string)
	RecordGauge(name string, value float64, tags map[string]string)
	RecordHistogram(name string, value float64, tags map[string]string)
	
	// 指标查询
	GetCounter(name string, tags map[string]string) (int64, error)
	GetGauge(name string, tags map[string]string) (float64, error)
	GetHistogramStats(name string, tags map[string]string) (map[string]float64, error)
	
	// 指标聚合
	AggregateMetrics(name string, interval time.Duration) ([]MetricPoint, error)
}

// MetricPoint 指标数据点
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Tags      map[string]string `json:"tags"`
}