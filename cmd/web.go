package cmd

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/yourusername/process-tracker/core"
)

//go:embed static/*
var staticFS embed.FS

// WebServer represents the web dashboard server
type WebServer struct {
	app    *core.App
	port   string
	host   string
	server *http.Server
	cache  *StatsCache
}

// StatsCache provides caching for expensive API calls
type StatsCache struct {
	data   map[string]interface{}
	expiry map[string]time.Time
	ttl    time.Duration
	mu     sync.RWMutex
}

// NewStatsCache creates a new stats cache
func NewStatsCache(ttl time.Duration) *StatsCache {
	return &StatsCache{
		data:   make(map[string]interface{}),
		expiry: make(map[string]time.Time),
		ttl:    ttl,
	}
}

// Get retrieves cached data if not expired
func (c *StatsCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	expiry, exists := c.expiry[key]
	if !exists || time.Now().After(expiry) {
		return nil, false
	}

	data, exists := c.data[key]
	return data, exists
}

// Set stores data in cache with expiry
func (c *StatsCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = value
	c.expiry[key] = time.Now().Add(c.ttl)
}

// NewWebServer creates a new web server instance
func NewWebServer(app *core.App, host, port string) *WebServer {
	return &WebServer{
		app:   app,
		host:  host,
		port:  port,
		cache: NewStatsCache(5 * time.Second),
	}
}

// Start starts the web server
func (ws *WebServer) Start() error {
	mux := http.NewServeMux()

	// Serve static files
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("failed to create static file system: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	// API endpoints
	mux.HandleFunc("/api/stats/today", ws.handleStatsToday)
	mux.HandleFunc("/api/stats/week", ws.handleStatsWeek)
	mux.HandleFunc("/api/stats/month", ws.handleStatsMonth)
	mux.HandleFunc("/api/live", ws.handleLive)
	mux.HandleFunc("/api/processes", ws.handleProcesses)
	mux.HandleFunc("/api/health", ws.handleHealth)

	addr := fmt.Sprintf("%s:%s", ws.host, ws.port)
	ws.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go ws.handleShutdown()

	// Print access URLs
	ws.printAccessURLs()

	if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("web服务器错误: %w", err)
	}

	return nil
}

// handleShutdown handles graceful shutdown
func (ws *WebServer) handleShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Println("收到关闭信号，正在停止Web服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := ws.server.Shutdown(ctx); err != nil {
		log.Printf("Web服务器关闭错误: %v", err)
	}
}

// API Response Types
type DashboardStats struct {
	ProcessCount         int              `json:"process_count"`
	ActiveCount          int              `json:"active_count"`
	AvgCPU               float64          `json:"avg_cpu"`                  // Normalized average CPU (0-100% of system)
	MaxCPU               float64          `json:"max_cpu"`                  // Normalized max CPU (0-100% of system)
	TotalMemory          float64          `json:"total_memory"`             // MB
	TotalMemoryPercent   float64          `json:"total_memory_percent"`     // Percentage of system memory
	MaxMemory            float64          `json:"max_memory"`               // MB
	MaxMemoryPercent     float64          `json:"max_memory_percent"`       // Percentage of system memory
	TotalCPUCores        int              `json:"total_cpu_cores"`          // System total CPU cores
	SystemTotalMemory    float64          `json:"system_total_memory"`      // System total memory in MB
	Timeline             []TimelinePoint  `json:"timeline"`
	TopProcesses         []ProcessSummary `json:"top_processes"`
}

type TimelinePoint struct {
	Time                 string  `json:"time"`
	CPU                  float64 `json:"cpu"`                    // Raw CPU percent (for backward compatibility)
	CPUPercentNormalized float64 `json:"cpu_percent_normalized"` // Normalized CPU as percentage of total system
	Memory               float64 `json:"memory"`                 // MB (for backward compatibility)
	MemoryPercent        float64 `json:"memory_percent"`         // Memory as percentage
}

type ProcessSummary struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemoryMB   float64 `json:"memory_mb"`
	Status     string  `json:"status"`
	Uptime     string  `json:"uptime"`
}

// handleStatsToday handles today's statistics
func (ws *WebServer) handleStatsToday(w http.ResponseWriter, r *http.Request) {
	ws.handleStatsPeriod(w, r, "today", 24*time.Hour)
}

// handleStatsWeek handles weekly statistics
func (ws *WebServer) handleStatsWeek(w http.ResponseWriter, r *http.Request) {
	ws.handleStatsPeriod(w, r, "week", 7*24*time.Hour)
}

// handleStatsMonth handles monthly statistics
func (ws *WebServer) handleStatsMonth(w http.ResponseWriter, r *http.Request) {
	ws.handleStatsPeriod(w, r, "month", 30*24*time.Hour)
}

// handleStatsPeriod handles statistics for a given period
func (ws *WebServer) handleStatsPeriod(w http.ResponseWriter, r *http.Request, cacheKey string, duration time.Duration) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check cache
	if cached, ok := ws.cache.Get(cacheKey); ok {
		json.NewEncoder(w).Encode(cached)
		return
	}

	// Read records from data file
	records, err := ws.readRecentRecords(duration)
	if err != nil {
		http.Error(w, fmt.Sprintf("读取数据失败: %v", err), http.StatusInternalServerError)
		return
	}

	// Calculate statistics
	stats := ws.calculateStats(records, duration)

	// Cache result
	ws.cache.Set(cacheKey, stats)

	json.NewEncoder(w).Encode(stats)
}

// handleLive handles real-time live data
func (ws *WebServer) handleLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get recent records (last minute)
	records, err := ws.readRecentRecords(1 * time.Minute)
	if err != nil {
		http.Error(w, fmt.Sprintf("读取数据失败: %v", err), http.StatusInternalServerError)
		return
	}

	// Group by process name and get latest
	latest := ws.getLatestProcesses(records)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"processes": latest,
	})
}

// handleProcesses handles process list
func (ws *WebServer) handleProcesses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get query parameters
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "cpu"
	}

	// Get recent records
	records, err := ws.readRecentRecords(1 * time.Minute)
	if err != nil {
		http.Error(w, fmt.Sprintf("读取数据失败: %v", err), http.StatusInternalServerError)
		return
	}

	// Get latest processes
	processes := ws.getLatestProcesses(records)

	// Sort processes
	switch sortBy {
	case "cpu":
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].CPUPercent > processes[j].CPUPercent
		})
	case "memory":
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].MemoryMB > processes[j].MemoryMB
		})
	case "name":
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].Name < processes[j].Name
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"processes": processes,
		"sort_by":   sortBy,
	})
}

// handleHealth handles health check
func (ws *WebServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	})
}

// readRecentRecords reads records from the last N duration
func (ws *WebServer) readRecentRecords(duration time.Duration) ([]core.ResourceRecord, error) {
	// Create a temporary storage manager to read data
	storageManager := core.NewManager(ws.app.DataFile, 0, false, core.StorageConfig{})
	
	// Read all records from the data file
	allRecords, err := storageManager.ReadRecords(ws.app.DataFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read records: %w", err)
	}
	
	// Filter records within the time window
	cutoffTime := time.Now().Add(-duration)
	var recentRecords []core.ResourceRecord
	
	for _, record := range allRecords {
		if record.Timestamp.After(cutoffTime) {
			recentRecords = append(recentRecords, record)
		}
	}
	
	return recentRecords, nil
}

// calculateStats calculates dashboard statistics
func (ws *WebServer) calculateStats(records []core.ResourceRecord, duration time.Duration) DashboardStats {
	// Get system information
	cpuCores := core.SystemCPUCores()
	totalMemoryMB := core.SystemMemoryMB()
	
	if len(records) == 0 {
		return DashboardStats{
			TotalCPUCores:     cpuCores,
			SystemTotalMemory: totalMemoryMB,
		}
	}

	// Track unique processes
	processMap := make(map[string]*processStats)
	for _, r := range records {
		if _, exists := processMap[r.Name]; !exists {
			processMap[r.Name] = &processStats{
				name:       r.Name,
				totalCPU:   0,
				totalMem:   0,
				maxCPU:     0,
				maxMem:     0,
				count:      0,
				lastRecord: r,
			}
		}

		ps := processMap[r.Name]
		// Use normalized CPU for statistics
		ps.totalCPU += r.CPUPercentNormalized
		ps.totalMem += r.MemoryMB
		if r.CPUPercentNormalized > ps.maxCPU {
			ps.maxCPU = r.CPUPercentNormalized
		}
		if r.MemoryMB > ps.maxMem {
			ps.maxMem = r.MemoryMB
		}
		ps.count++
		ps.lastRecord = r
	}

	// Calculate aggregates
	var totalCPU, totalMem, maxCPU, maxMem float64
	activeCount := 0
	for _, ps := range processMap {
		avgCPU := ps.totalCPU / float64(ps.count)
		avgMem := ps.totalMem / float64(ps.count)
		totalCPU += avgCPU
		totalMem += avgMem
		if ps.maxCPU > maxCPU {
			maxCPU = ps.maxCPU
		}
		if ps.maxMem > maxMem {
			maxMem = ps.maxMem
		}
		if ps.lastRecord.IsActive {
			activeCount++
		}
	}

	// Calculate memory percentages
	totalMemPercent := 0.0
	maxMemPercent := 0.0
	if totalMemoryMB > 0 {
		totalMemPercent = (totalMem / totalMemoryMB) * 100
		maxMemPercent = (maxMem / totalMemoryMB) * 100
	}

	// Generate timeline
	timeline := ws.generateTimeline(records, duration)

	// Get top processes
	topProcesses := ws.getTopProcesses(processMap, 10)

	return DashboardStats{
		ProcessCount:       len(processMap),
		ActiveCount:        activeCount,
		AvgCPU:             totalCPU / float64(len(processMap)),
		MaxCPU:             maxCPU,
		TotalMemory:        totalMem,
		TotalMemoryPercent: totalMemPercent,
		MaxMemory:          maxMem,
		MaxMemoryPercent:   maxMemPercent,
		TotalCPUCores:      cpuCores,
		SystemTotalMemory:  totalMemoryMB,
		Timeline:           timeline,
		TopProcesses:       topProcesses,
	}
}

type processStats struct {
	name       string
	totalCPU   float64
	totalMem   float64
	maxCPU     float64
	maxMem     float64
	count      int
	lastRecord core.ResourceRecord
}

// generateTimeline generates timeline data points
func (ws *WebServer) generateTimeline(records []core.ResourceRecord, duration time.Duration) []TimelinePoint {
	if len(records) == 0 {
		return []TimelinePoint{}
	}

	// Determine bucket size based on duration
	var bucketSize time.Duration
	switch {
	case duration <= 24*time.Hour:
		bucketSize = 1 * time.Hour
	case duration <= 7*24*time.Hour:
		bucketSize = 6 * time.Hour
	default:
		bucketSize = 24 * time.Hour
	}

	// Group records into buckets
	buckets := make(map[string]*timelineBucket)
	for _, r := range records {
		key := r.Timestamp.Truncate(bucketSize).Format("2006-01-02 15:04")
		if _, exists := buckets[key]; !exists {
			buckets[key] = &timelineBucket{
				time:          key,
				cpu:           0,
				cpuNormalized: 0,
				memory:        0,
				memoryPercent: 0,
				count:         0,
			}
		}
		bucket := buckets[key]
		bucket.cpu += r.CPUPercent
		bucket.cpuNormalized += r.CPUPercentNormalized
		bucket.memory += r.MemoryMB
		bucket.memoryPercent += r.MemoryPercent
		bucket.count++
	}

	// Convert to sorted timeline
	var timeline []TimelinePoint
	for _, bucket := range buckets {
		timeline = append(timeline, TimelinePoint{
			Time:                 bucket.time,
			CPU:                  bucket.cpu / float64(bucket.count),
			CPUPercentNormalized: bucket.cpuNormalized / float64(bucket.count),
			Memory:               bucket.memory / float64(bucket.count),
			MemoryPercent:        bucket.memoryPercent / float64(bucket.count),
		})
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Time < timeline[j].Time
	})

	return timeline
}

type timelineBucket struct {
	time           string
	cpu            float64
	cpuNormalized  float64
	memory         float64
	memoryPercent  float64
	count          int
}

// getTopProcesses returns top N processes by CPU usage
func (ws *WebServer) getTopProcesses(processMap map[string]*processStats, n int) []ProcessSummary {
	var processes []ProcessSummary
	for _, ps := range processMap {
		avgCPU := ps.totalCPU / float64(ps.count)
		avgMem := ps.totalMem / float64(ps.count)

		uptime := ""
		if ps.lastRecord.CreateTime > 0 {
			startTime := time.UnixMilli(ps.lastRecord.CreateTime)
			uptime = formatUptime(time.Since(startTime))
		}

		processes = append(processes, ProcessSummary{
			PID:        ps.lastRecord.PID,
			Name:       ps.name,
			CPUPercent: avgCPU,
			MemoryMB:   avgMem,
			Status:     getStatus(ps.lastRecord.IsActive),
			Uptime:     uptime,
		})
	}

	// Sort by CPU
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPUPercent > processes[j].CPUPercent
	})

	if len(processes) > n {
		processes = processes[:n]
	}

	return processes
}

// getLatestProcesses gets the latest record for each process
func (ws *WebServer) getLatestProcesses(records []core.ResourceRecord) []ProcessSummary {
	if len(records) == 0 {
		return []ProcessSummary{}
	}

	// Get latest record for each process
	latest := make(map[string]core.ResourceRecord)
	for _, r := range records {
		if existing, ok := latest[r.Name]; !ok || r.Timestamp.After(existing.Timestamp) {
			latest[r.Name] = r
		}
	}

	// Convert to ProcessSummary
	var processes []ProcessSummary
	for _, r := range latest {
		uptime := ""
		if r.CreateTime > 0 {
			startTime := time.UnixMilli(r.CreateTime)
			uptime = formatUptime(time.Since(startTime))
		}

		processes = append(processes, ProcessSummary{
			PID:        r.PID,
			Name:       r.Name,
			CPUPercent: r.CPUPercent,
			MemoryMB:   r.MemoryMB,
			Status:     getStatus(r.IsActive),
			Uptime:     uptime,
		})
	}

	// Sort by CPU
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPUPercent > processes[j].CPUPercent
	})

	return processes
}

// Helper functions

func getStatus(isActive bool) string {
	if isActive {
		return "active"
	}
	return "idle"
}

func formatUptime(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute

	if h > 24 {
		days := h / 24
		h = h % 24
		return fmt.Sprintf("%dd%dh", days, h)
	}
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// printAccessURLs prints all available access URLs for the web server
func (ws *WebServer) printAccessURLs() {
	ips := getLocalIPs()
	
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🌐 Web服务器已启动，可通过以下地址访问：")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	if len(ips) == 0 {
		log.Printf("  http://localhost:%s", ws.port)
	} else {
		for _, ip := range ips {
			log.Printf("  http://%s:%s", ip, ws.port)
		}
	}
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// getLocalIPs returns all non-loopback IPv4 addresses
func getLocalIPs() []string {
	var ips []string
	
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	
	for _, iface := range interfaces {
		// Skip down interfaces
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		
		// Skip loopback interface
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			
			// Skip loopback and non-IPv4
			if ip == nil || ip.IsLoopback() {
				continue
			}
			
			// Only include IPv4
			ip = ip.To4()
			if ip != nil {
				ips = append(ips, ip.String())
			}
		}
	}
	
	return ips
}
