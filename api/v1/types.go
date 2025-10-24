package v1

import (
	"time"
)

// APIVersion defines the API version
const APIVersion = "v1"

// ResponseKind defines the kind of response
type ResponseKind string

const (
	KindTaskList    ResponseKind = "TaskList"
	KindTask        ResponseKind = "Task"
	KindProcessList ResponseKind = "ProcessList"
	KindProcess     ResponseKind = "Process"
	KindStats       ResponseKind = "Stats"
	KindSystemInfo  ResponseKind = "SystemInfo"
	KindError       ResponseKind = "Error"
)

// ResponseMetadata provides pagination and filtering metadata
type ResponseMetadata struct {
	Total       int                    `json:"total,omitempty"`
	Limit       int                    `json:"limit,omitempty"`
	Offset      int                    `json:"offset,omitempty"`
	Sort        string                 `json:"sort,omitempty"`
	Filter      []string               `json:"filter,omitempty"`
	View        string                 `json:"view,omitempty"`
	GeneratedAt time.Time              `json:"generated_at"`
	RequestID   string                 `json:"request_id,omitempty"`
	Links       map[string]string      `json:"links,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// APIError represents a standardized API error
type APIError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// APIResponse represents a standardized API response
type APIResponse struct {
	Kind       ResponseKind         `json:"kind"`
	APIVersion string                `json:"apiVersion"`
	Metadata   *ResponseMetadata     `json:"metadata,omitempty"`
	Data       interface{}           `json:"data,omitempty"`
	Errors     []APIError            `json:"errors,omitempty"`
}

// QueryParams represents standardized query parameters
type QueryParams struct {
	Filter []string `form:"filter" binding:"omitempty"`
	Sort   string    `form:"sort" binding:"omitempty"`
	Limit  int       `form:"limit" binding:"min=0,max=100"`
	Offset int       `form:"offset" binding:"min=0"`
	Format string    `form:"format" binding:"omitempty,oneof=json yaml"`
	View   string    `form:"view" binding:"omitempty,oneof=tree flat"`
}

// TaskRequest represents a task creation request
type TaskRequest struct {
	Name      string            `json:"name" binding:"required"`
	Command   string            `json:"command" binding:"required"`
	WorkDir   string            `json:"workDir,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Priority  int               `json:"priority" binding:"min=1,max=10"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timeout   int               `json:"timeout,omitempty"`  // timeout in seconds
	RestartPolicy string         `json:"restartPolicy,omitempty"` // "never", "on-failure", "always"
}

// TaskResponse represents a task response
type TaskResponse struct {
	ID            int                    `json:"id"`
	Name          string                 `json:"name"`
	Command       string                 `json:"command"`
	Status        string                 `json:"status"`
	Priority      int                    `json:"priority"`
	WorkDir       string                 `json:"workDir,omitempty"`
	RootPID       int32                  `json:"rootPid,omitempty"`
	ProcessCount  int                    `json:"processCount"`
	CreatedAt     time.Time              `json:"createdAt"`
	StartedAt     *time.Time             `json:"startedAt,omitempty"`
	CompletedAt   *time.Time             `json:"completedAt,omitempty"`
	ExitCode      *int                   `json:"exitCode,omitempty"`
	ErrorMessage  string                 `json:"errorMessage,omitempty"`
	Labels        map[string]string      `json:"labels,omitempty"`
	Env           map[string]string      `json:"env,omitempty"`
	ResourceUsage *TaskResourceUsage     `json:"resourceUsage,omitempty"`
}

// TaskResourceUsage represents task resource usage
type TaskResourceUsage struct {
	CPU        float64 `json:"cpu"`         // CPU percentage
	Memory     float64 `json:"memory"`      // Memory in MB
	DiskRead   float64 `json:"diskRead"`    // Disk read in MB
	DiskWrite  float64 `json:"diskWrite"`   // Disk write in MB
	NetSent    float64 `json:"netSent"`     // Network sent in KB
	NetRecv    float64 `json:"netRecv"`     // Network received in KB
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TaskUpdateRequest represents a task update request
type TaskUpdateRequest struct {
	Name     *string            `json:"name,omitempty"`
	Priority *int               `json:"priority,omitempty"`
	Labels   map[string]string  `json:"labels,omitempty"`
}

// ProcessResponse represents a process response
type ProcessResponse struct {
	PID           int32               `json:"pid"`
	PPID          int32               `json:"ppid"`
	Name          string              `json:"name"`
	Command       string              `json:"command"`
	Args          []string            `json:"args,omitempty"`
	Status        string              `json:"status"`
	CPUPercent    float64             `json:"cpuPercent"`
	MemoryMB      float64             `json:"memoryMb"`
	MemoryPercent float64             `json:"memoryPercent"`
	Category      string              `json:"category"`
	WorkDir       string              `json:"workDir"`
	CreatedAt     time.Time           `json:"createdAt"`
	IsActive      bool                `json:"isActive"`
	Uptime        string              `json:"uptime,omitempty"`
	Children      []ProcessResponse   `json:"children,omitempty"`
}

// StatsResponse represents statistics response
type StatsResponse struct {
	System        SystemStats          `json:"system"`
	Processes     ProcessStats         `json:"processes"`
	Timeline      []TimelinePoint      `json:"timeline,omitempty"`
	GeneratedAt   time.Time            `json:"generatedAt"`
	Period        string               `json:"period"`
}

// SystemStats represents system statistics
type SystemStats struct {
	TotalMemoryMB    float64 `json:"totalMemoryMb"`
	UsedMemoryMB     float64 `json:"usedMemoryMb"`
	AvailableMemoryMB float64 `json:"availableMemoryMb"`
	MemoryPercent    float64 `json:"memoryPercent"`
	CPUUsage         float64 `json:"cpuUsage"`
	CPUCores         int     `json:"cpuCores"`
	LoadAverage      []float64 `json:"loadAverage"`
	Uptime           string  `json:"uptime"`
	ProcessCount     int     `json:"processCount"`
	ActiveCount      int     `json:"activeCount"`
}

// ProcessStats represents process statistics
type ProcessStats struct {
	TotalCount    int              `json:"totalCount"`
	ActiveCount   int              `json:"activeCount"`
	TopProcesses  []ProcessSummary `json:"topProcesses"`
	CategoryStats map[string]int   `json:"categoryStats"`
}

// ProcessSummary represents process summary for statistics
type ProcessSummary struct {
	PID           int32   `json:"pid"`
	Name          string  `json:"name"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryMB      float64 `json:"memoryMb"`
	MemoryPercent float64 `json:"memoryPercent"`
	Status        string  `json:"status"`
	Category      string  `json:"category"`
	Command       string  `json:"command"`
	Uptime        string  `json:"uptime"`
}

// TimelinePoint represents a timeline data point
type TimelinePoint struct {
	Timestamp    time.Time `json:"timestamp"`
	CPU          float64   `json:"cpu"`
	Memory       float64   `json:"memory"`
	ProcessCount int       `json:"processCount"`
}

// SystemInfoResponse represents system information
type SystemInfoResponse struct {
	Hostname     string            `json:"hostname"`
	OS           string            `json:"os"`
	Architecture string            `json:"architecture"`
	CPUInfo      CPUInfo           `json:"cpuInfo"`
	MemoryInfo   MemoryInfo        `json:"memoryInfo"`
	DiskInfo     []DiskInfo        `json:"diskInfo"`
	NetworkInfo  []NetworkInfo     `json:"networkInfo"`
	Uptime       string            `json:"uptime"`
	LoadAverage  []float64         `json:"loadAverage"`
	GeneratedAt  time.Time         `json:"generatedAt"`
}

// CPUInfo represents CPU information
type CPUInfo struct {
	Model     string  `json:"model"`
	Cores     int     `json:"cores"`
	Threads   int     `json:"threads"`
	Frequency float64 `json:"frequency"` // GHz
	Cache     string  `json:"cache"`
}

// MemoryInfo represents memory information
type MemoryInfo struct {
	Total     uint64  `json:"total"`
	Available uint64  `json:"available"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Buffers   uint64  `json:"buffers"`
	Cached    uint64  `json:"cached"`
	SwapTotal uint64  `json:"swapTotal"`
	SwapUsed  uint64  `json:"swapUsed"`
}

// DiskInfo represents disk information
type DiskInfo struct {
	Device     string  `json:"device"`
	Mountpoint string  `json:"mountpoint"`
	Fstype     string  `json:"fstype"`
	Total      uint64  `json:"total"`
	Free       uint64  `json:"free"`
	Used       uint64  `json:"used"`
	Percent    float64 `json:"percent"`
}

// NetworkInfo represents network interface information
type NetworkInfo struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	MTU       int      `json:"mtu"`
	Flags     []string `json:"flags"`
	Stats     NetworkStats `json:"stats"`
}

// NetworkStats represents network statistics
type NetworkStats struct {
	BytesReceived uint64 `json:"bytesReceived"`
	BytesSent     uint64 `json:"bytesSent"`
	PacketsReceived uint64 `json:"packetsReceived"`
	PacketsSent   uint64 `json:"packetsSent"`
	ErrorsIn      uint64 `json:"errorsIn"`
	ErrorsOut     uint64 `json:"errorsOut"`
	DroppedIn     uint64 `json:"droppedIn"`
	DroppedOut    uint64 `json:"droppedOut"`
}