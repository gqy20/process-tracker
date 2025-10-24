package v1

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/process-tracker/core"
)

// Router sets up the API v1 routes
type Router struct {
	engine       *gin.Engine
	taskHandler  *TaskHandler
	procHandler  *ProcessHandler
	statsHandler *StatsHandler
}

// NewRouter creates a new API v1 router
func NewRouter(app *core.App) *Router {
	engine := gin.New()

	// Configure middleware
	engine.Use(LoggingMiddleware())
	engine.Use(ErrorHandlingMiddleware())
	engine.Use(gin.Recovery())
	engine.Use(CORSMiddleware())
	engine.Use(SecurityHeadersMiddleware())
	engine.Use(RequestIDMiddleware())
	engine.Use(APIVersionMiddleware())
	engine.Use(ValidationMiddleware())
	engine.Use(ContentTypeMiddleware())
	engine.Use(RateLimitMiddleware())

	// Create handlers
	taskHandler := NewTaskHandler(app)
	procHandler := NewProcessHandler(app)
	statsHandler := NewStatsHandler(app)

	// Create router
	router := &Router{
		engine:       engine,
		taskHandler:  taskHandler,
		procHandler:  procHandler,
		statsHandler: statsHandler,
	}

	// Setup routes
	router.setupRoutes()

	return router
}

// GetEngine returns the gin engine
func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}

// setupRoutes configures all API routes
func (r *Router) setupRoutes() {
	v1 := r.engine.Group("/v1")

	// Task routes
	tasks := v1.Group("/tasks")
	{
		tasks.GET("", r.taskHandler.ListTasks)
		tasks.POST("", r.taskHandler.CreateTask)
		tasks.GET("/:id", r.taskHandler.GetTask)
		tasks.PATCH("/:id", r.taskHandler.UpdateTask)
		tasks.DELETE("/:id", r.taskHandler.DeleteTask)

		// Task actions
		tasks.POST("/:id/start", r.taskHandler.StartTask)
		tasks.POST("/:id/stop", r.taskHandler.StopTask)
		tasks.POST("/:id/restart", r.taskHandler.RestartTask)
		tasks.GET("/:id/logs", r.taskHandler.GetTaskLogs)
		tasks.GET("/:id/stats", r.taskHandler.GetTaskStats)
	}

	// Process routes
	processes := v1.Group("/processes")
	{
		processes.GET("", r.procHandler.ListProcesses)
		processes.GET("/live", r.procHandler.GetLiveProcesses)
		processes.GET("/:pid", r.procHandler.GetProcess)
		processes.GET("/:pid/children", r.procHandler.GetProcessChildren)
		processes.GET("/:pid/tree", r.procHandler.GetProcessTree)
	}

	// Statistics routes
	stats := v1.Group("/stats")
	{
		stats.GET("", r.statsHandler.GetStats)
		stats.GET("/summary", r.statsHandler.GetStatsSummary)
		stats.GET("/timeline", r.statsHandler.GetStatsTimeline)
		stats.GET("/top", r.statsHandler.GetStatsTop)
		stats.GET("/resources", r.statsHandler.GetStatsResources)
		stats.GET("/history", r.statsHandler.GetStatsHistory)
	}

	// Legacy compatibility routes (mapped to new endpoints)
	// This allows existing web UI to continue working
	legacy := v1.Group("/legacy")
	{
		// Map old endpoints to new ones
		legacy.GET("/stats/today", func(c *gin.Context) {
			c.Redirect(302, "/v1/stats?period=24h")
		})
		legacy.GET("/stats/week", func(c *gin.Context) {
			c.Redirect(302, "/v1/stats?period=7d")
		})
		legacy.GET("/stats/month", func(c *gin.Context) {
			c.Redirect(302, "/v1/stats?period=30d")
		})
		legacy.GET("/live", r.procHandler.GetLiveProcesses)
		legacy.GET("/processes", r.procHandler.ListProcesses)
		legacy.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":    "ok",
				"timestamp": "generated",
			})
		})
	}

	// System information routes
	system := v1.Group("/system")
	{
		system.GET("", r.handleSystemInfo)
		system.GET("/health", r.handleHealth)
		system.GET("/version", r.handleVersion)
		system.GET("/metrics", r.handleMetrics)
	}

	// API documentation
	v1.GET("/docs", r.handleAPIDocs)
	v1.GET("/openapi.json", r.handleOpenAPISpec)
}

// System info handlers
func (r *Router) handleSystemInfo(c *gin.Context) {
	systemInfo := SystemInfoResponse{
		Hostname:     "localhost", // Would need actual hostname
		OS:           "linux",     // Would need actual OS info
		Architecture: "x86_64",   // Would need actual architecture
		CPUInfo: CPUInfo{
			Model:     "Unknown CPU",
			Cores:     core.SystemCPUCores(),
			Threads:   core.SystemCPUCores(), // Assuming 1:1 threads:cores
			Frequency: 0.0,
			Cache:     "Unknown",
		},
		MemoryInfo: MemoryInfo{
			Total:     uint64(core.SystemMemoryMB()) * 1024 * 1024,
			Available: 0, // Would need calculation
			Used:      0, // Would need calculation
			Free:      0, // Would need calculation
			Buffers:   0,
			Cached:    0,
			SwapTotal: 0,
			SwapUsed:  0,
		},
		NetworkInfo: []NetworkInfo{}, // Would need actual network info
		DiskInfo:    []DiskInfo{},    // Would need actual disk info
		Uptime:      "unknown",
		LoadAverage: []float64{0, 0, 0},
		GeneratedAt:  time.Now(),
	}

	SendSuccess(c, KindSystemInfo, systemInfo, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

func (r *Router) handleHealth(c *gin.Context) {
	SendSuccess(c, KindSystemInfo, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "v1.0.0",
	}, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

func (r *Router) handleVersion(c *gin.Context) {
	SendSuccess(c, KindSystemInfo, map[string]interface{}{
		"version":     "1.0.0",
		"apiVersion":  APIVersion,
		"buildTime":   "unknown",
		"gitCommit":   "unknown",
		"goVersion":   "unknown",
	}, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

func (r *Router) handleMetrics(c *gin.Context) {
	// This would return Prometheus-style metrics
	// For now, return a placeholder
	c.Header("Content-Type", "text/plain")
	c.String(200, "# HELP process_tracker_metrics\n# Process Tracker metrics\n\n"+
		"# Total number of tasks\nprocess_tracker_tasks_total 0\n\n"+
		"# Number of running tasks\nprocess_tracker_tasks_running 0\n\n"+
		"# Total number of processes\nprocess_tracker_processes_total 0\n\n"+
		"# CPU usage percentage\nprocess_tracker_cpu_usage 0.0\n\n"+
		"# Memory usage in MB\nprocess_tracker_memory_usage 0.0")
}

func (r *Router) handleAPIDocs(c *gin.Context) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Process Tracker API v1 Documentation</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .method { color: #fff; padding: 3px 8px; border-radius: 3px; font-weight: bold; }
        .get { background: #61affe; }
        .post { background: #49cc90; }
        .patch { background: #f0ad4e; }
        .delete { background: #d9534f; }
        code { background: #f8f8f8; padding: 2px 4px; border-radius: 3px; }
        pre { background: #f8f8f8; padding: 10px; border-radius: 5px; overflow-x: auto; }
    </style>
</head>
<body>
    <h1>Process Tracker API v1 Documentation</h1>

    <h2>Overview</h2>
    <p>The Process Tracker API provides RESTful endpoints for managing processes and tasks. All responses follow a consistent format with versioning.</p>

    <h2>Base URL</h2>
    <code>http://localhost:8080/v1</code>

    <h2>Authentication</h2>
    <p>Currently no authentication is required. This may change in future versions.</p>

    <h2>Response Format</h2>
    <pre>
{
  "kind": "TaskList",
  "apiVersion": "v1",
  "metadata": {
    "total": 10,
    "limit": 20,
    "offset": 0,
    "generated_at": "2023-12-01T10:00:00Z"
  },
  "data": [...],
  "errors": null
}
    </pre>

    <h2>Endpoints</h2>

    <div class="endpoint">
        <h3><span class="method get">GET</span> /v1/tasks</h3>
        <p>List all tasks with filtering and pagination support.</p>
        <p><strong>Query Parameters:</strong></p>
        <ul>
            <li><code>filter</code> - Filter tasks (e.g., filter=status=running)</li>
            <li><code>sort</code> - Sort results (e.g., sort=-created_at)</li>
            <li><code>limit</code> - Number of results to return (max 100)</li>
            <li><code>offset</code> - Number of results to skip</li>
        </ul>
    </div>

    <div class="endpoint">
        <h3><span class="method post">POST</span> /v1/tasks</h3>
        <p>Create a new task.</p>
        <p><strong>Request Body:</strong></p>
        <pre>
{
  "name": "Example Task",
  "command": "sleep 60",
  "priority": 1,
  "labels": {
    "environment": "test"
  }
}
        </pre>
    </div>

    <div class="endpoint">
        <h3><span class="method get">GET</span> /v1/tasks/{id}</h3>
        <p>Get a specific task by ID.</p>
    </div>

    <div class="endpoint">
        <h3><span class="method post">POST</span> /v1/tasks/{id}/start</h3>
        <p>Start a task.</p>
    </div>

    <div class="endpoint">
        <h3><span class="method post">POST</span> /v1/tasks/{id}/stop</h3>
        <p>Stop a task.</p>
    </div>

    <div class="endpoint">
        <h3><span class="method get">GET</span> /v1/processes</h3>
        <p>List all processes with filtering support.</p>
        <p><strong>Query Parameters:</strong></p>
        <ul>
            <li><code>view</code> - View mode: flat or tree</li>
            <li><code>sort</code> - Sort results</li>
        </ul>
    </div>

    <div class="endpoint">
        <h3><span class="method get">GET</span> /v1/stats</h3>
        <p>Get comprehensive statistics.</p>
    </div>

    <div class="endpoint">
        <h3><span class="method get">GET</span> /v1/stats/timeline</h3>
        <p>Get timeline statistics.</p>
        <p><strong>Query Parameters:</strong></p>
        <ul>
            <li><code>period</code> - Time period (e.g., 1h, 24h, 7d)</li>
        </ul>
    </div>

    <h2>Error Handling</h2>
    <p>Errors are returned with appropriate HTTP status codes and consistent error format:</p>
    <pre>
{
  "kind": "Error",
  "apiVersion": "v1",
  "metadata": {
    "generated_at": "2023-12-01T10:00:00Z"
  },
  "errors": [
    {
      "code": "NOT_FOUND",
      "message": "Task with id 123 not found",
      "details": {}
    }
  ]
}
    </pre>

    <h2>Rate Limiting</h2>
    <p>API requests are rate-limited to 100 requests per minute per IP address.</p>

    <h2>CORS</h2>
    <p>The API supports Cross-Origin Resource Sharing (CORS) for web applications.</p>
</body>
</html>`

	c.Header("Content-Type", "text/html")
	c.String(200, html)
}

func (r *Router) handleOpenAPISpec(c *gin.Context) {
	// This would return the OpenAPI specification
	// For now, return a placeholder
	openAPI := `{
  "openapi": "3.0.3",
  "info": {
    "title": "Process Tracker API",
    "version": "1.0.0",
    "description": "Modern process monitoring and task management API"
  },
  "servers": [
    {
      "url": "http://localhost:8080/v1",
      "description": "Development server"
    }
  ],
  "paths": {
    "/tasks": {
      "get": {
        "summary": "List tasks",
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/TaskList"
                }
              }
            }
          }
        }
      }
    }
  }
}`

	c.Header("Content-Type", "application/json")
	c.String(200, openAPI)
}