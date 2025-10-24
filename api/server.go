package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/process-tracker/api/v1"
	"github.com/yourusername/process-tracker/core"
	"github.com/yourusername/process-tracker/web"
	"github.com/gin-gonic/gin"
)

// Server represents the API server
type Server struct {
	app      *core.App
	v1Router *v1.Router
	engine   *gin.Engine
	httpPort int
}

// NewServer creates a new API server
func NewServer(app *core.App, port int) *Server {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create Gin engine
	engine := gin.New()

	// Create server
	server := &Server{
		app:      app,
		httpPort: port,
		engine:   engine,
	}

	// Initialize handlers
	server.initializeHandlers()

	return server
}

// initializeHandlers sets up all API handlers
func (s *Server) initializeHandlers() {
	// Create v1 API router
	s.v1Router = v1.NewRouter(s.app)

	// Setup routes
	s.setupRoutes()
}

// setupRoutes configures all routes
func (s *Server) setupRoutes() {
	// Add global middleware
	s.engine.Use(gin.Logger())
	s.engine.Use(gin.Recovery())

	// Setup web interface routes
	web.SetupWebRoutes(s.engine, s.app)

	// Health check endpoint
	s.engine.GET("/health", s.healthCheck)

	// API version 1 routes
	s.engine.Any("/v1/*path", s.v1Router.GetEngine().HandleContext)

	// Root redirect to dashboard
	s.engine.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/dashboard")
	})
}

// healthCheck provides a simple health check endpoint
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"apis": []string{
			"/v1/docs",
			"/v1/tasks",
			"/v1/processes",
			"/v1/stats",
		},
	})
}

// Start starts the API server
func (s *Server) Start() error {
	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.httpPort),
		Handler:      s.engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("API server starting on port %d", s.httpPort)
		log.Printf("API documentation available at http://localhost:%d/v1/docs", s.httpPort)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
		return err
	}

	log.Println("Server shutdown complete")
	return nil
}

// GetEngine returns the Gin engine (for testing purposes)
func (s *Server) GetEngine() *gin.Engine {
	return s.engine
}

// GetV1Router returns the v1 router (for testing purposes)
func (s *Server) GetV1Router() *v1.Router {
	return s.v1Router
}

// GetAccessURL returns the access URL for the API
func (s *Server) GetAccessURL() string {
	return fmt.Sprintf("http://localhost:%d", s.httpPort)
}

// GetDocsURL returns the documentation URL
func (s *Server) GetDocsURL() string {
	return fmt.Sprintf("http://localhost:%d/v1/docs", s.httpPort)
}

// PrintStartupInfo prints startup information
func (s *Server) PrintStartupInfo() {
	fmt.Printf("🚀 Process Tracker API Server\n")
	fmt.Printf("📍 API Server: %s\n", s.GetAccessURL())
	fmt.Printf("📖 API Docs: %s\n", s.GetDocsURL())
	fmt.Printf("❤️  Health Check: %s/health\n", s.GetAccessURL())
	fmt.Printf("\nAvailable Endpoints:\n")
	fmt.Printf("  GET  /v1/tasks          - List tasks\n")
	fmt.Printf("  POST /v1/tasks          - Create task\n")
	fmt.Printf("  GET  /v1/processes      - List processes\n")
	fmt.Printf("  GET  /v1/stats          - Get statistics\n")
	fmt.Printf("  GET  /v1/system/info    - System information\n")
	fmt.Printf("  GET  /v1/docs           - API documentation\n")
	fmt.Printf("\nPress Ctrl+C to stop the server\n")
}

// Run starts the server and prints startup information
func (s *Server) Run() error {
	s.PrintStartupInfo()
	return s.Start()
}