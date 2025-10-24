package v1

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/process-tracker/core"
)

// StatsHandler handles statistics-related API endpoints
type StatsHandler struct {
	app    *core.App
	cache  *StatsCache
}

// StatsCache provides caching for expensive statistics calculations
type StatsCache struct {
	data   map[string]interface{}
	expiry map[string]time.Time
	ttl    time.Duration
}

// NewStatsCache creates a new stats cache
func NewStatsCache(ttl time.Duration) *StatsCache {
	cache := &StatsCache{
		data:   make(map[string]interface{}),
		expiry: make(map[string]time.Time),
		ttl:    ttl,
	}
	go cache.cleanup()
	return cache
}

// Get retrieves cached data if not expired
func (c *StatsCache) Get(key string) (interface{}, bool) {
	expiry, exists := c.expiry[key]
	if !exists || time.Now().After(expiry) {
		return nil, false
	}
	return c.data[key], true
}

// Set stores data in cache with expiry
func (c *StatsCache) Set(key string, value interface{}) {
	c.data[key] = value
	c.expiry[key] = time.Now().Add(c.ttl)
}

// cleanup removes expired entries
func (c *StatsCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		for key, expiry := range c.expiry {
			if now.After(expiry) {
				delete(c.data, key)
				delete(c.expiry, key)
			}
		}
	}
}

// NewStatsHandler creates a new stats handler
func NewStatsHandler(app *core.App) *StatsHandler {
	return &StatsHandler{
		app:   app,
		cache: NewStatsCache(30 * time.Second), // Cache for 30 seconds
	}
}

// GetStats returns comprehensive statistics
func (h *StatsHandler) GetStats(c *gin.Context) {
	// Check cache first
	if cached, ok := h.cache.Get("comprehensive"); ok {
		SendSuccess(c, KindStats, cached, &ResponseMetadata{
			GeneratedAt: time.Now(),

		})
		return
	}

	// Calculate comprehensive statistics
	stats, err := h.calculateComprehensiveStats()
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to calculate statistics: %w", err))
		return
	}

	// Cache result
	h.cache.Set("comprehensive", stats)

	SendSuccess(c, KindStats, stats, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetStatsSummary returns summary statistics
func (h *StatsHandler) GetStatsSummary(c *gin.Context) {
	// Check cache first
	if cached, ok := h.cache.Get("summary"); ok {
		SendSuccess(c, KindStats, cached, &ResponseMetadata{
			GeneratedAt: time.Now(),

		})
		return
	}

	// Calculate summary statistics
	summary, err := h.calculateSummaryStats()
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to calculate summary statistics: %w", err))
		return
	}

	// Cache result
	h.cache.Set("summary", summary)

	SendSuccess(c, KindStats, summary, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetStatsTimeline returns timeline statistics
func (h *StatsHandler) GetStatsTimeline(c *gin.Context) {
	// Get query parameters
	periodStr := c.DefaultQuery("period", "1h")
	duration, err := time.ParseDuration(periodStr)
	if err != nil {
		SendBadRequest(c, "Invalid period format. Use format like '1h', '24h', '7d'")
		return
	}

	// Check cache first
	cacheKey := "timeline_" + periodStr
	if cached, ok := h.cache.Get(cacheKey); ok {
		SendSuccess(c, KindStats, cached, &ResponseMetadata{
			GeneratedAt: time.Now(),

		})
		return
	}

	// Calculate timeline statistics
	timeline, err := h.calculateTimelineStats(duration)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to calculate timeline statistics: %w", err))
		return
	}

	// Cache result
	h.cache.Set(cacheKey, timeline)

	response := map[string]interface{}{
		"period":     periodStr,
		"timeline":   timeline,
		"generatedAt": time.Now(),
	}

	SendSuccess(c, KindStats, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetStatsTop returns top processes by various metrics
func (h *StatsHandler) GetStatsTop(c *gin.Context) {
	// Get query parameters
	metric := c.DefaultQuery("metric", "cpu")
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 10
	}

	// Check cache first
	cacheKey := fmt.Sprintf("top_%s_%d", metric, limit)
	if cached, ok := h.cache.Get(cacheKey); ok {
		SendSuccess(c, KindStats, cached, &ResponseMetadata{
			GeneratedAt: time.Now(),

		})
		return
	}

	// Get recent processes
	records, err := h.readRecentRecords(5 * time.Minute)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to read process records: %w", err))
		return
	}

	// Calculate top processes
	topProcesses, err := h.calculateTopProcesses(records, metric, limit)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to calculate top processes: %w", err))
		return
	}

	response := map[string]interface{}{
		"metric":     metric,
		"limit":      limit,
		"processes":  topProcesses,
		"generatedAt": time.Now(),
	}

	// Cache result
	h.cache.Set(cacheKey, response)

	SendSuccess(c, KindStats, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetStatsResources returns resource usage statistics
func (h *StatsHandler) GetStatsResources(c *gin.Context) {
	// Check cache first
	if cached, ok := h.cache.Get("resources"); ok {
		SendSuccess(c, KindStats, cached, &ResponseMetadata{
			GeneratedAt: time.Now(),

		})
		return
	}

	// Calculate resource statistics
	resources, err := h.calculateResourceStats()
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to calculate resource statistics: %w", err))
		return
	}

	// Cache result
	h.cache.Set("resources", resources)

	SendSuccess(c, KindStats, resources, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetStatsHistory returns historical statistics
func (h *StatsHandler) GetStatsHistory(c *gin.Context) {
	// Get query parameters
	periodStr := c.DefaultQuery("period", "24h")
	duration, err := time.ParseDuration(periodStr)
	if err != nil {
		SendBadRequest(c, "Invalid period format. Use format like '1h', '24h', '7d'")
		return
	}

	granularity := c.DefaultQuery("granularity", "1h")

	// Check cache first
	cacheKey := fmt.Sprintf("history_%s_%s", periodStr, granularity)
	if cached, ok := h.cache.Get(cacheKey); ok {
		SendSuccess(c, KindStats, cached, &ResponseMetadata{
			GeneratedAt: time.Now(),

		})
		return
	}

	// Calculate historical statistics
	history, err := h.calculateHistoryStats(duration, granularity)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to calculate historical statistics: %w", err))
		return
	}

	response := map[string]interface{}{
		"period":      periodStr,
		"granularity": granularity,
		"history":     history,
		"generatedAt": time.Now(),
	}

	// Cache result
	h.cache.Set(cacheKey, response)

	SendSuccess(c, KindStats, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// Helper functions

// readRecentRecords reads recent records from storage
func (h *StatsHandler) readRecentRecords(duration time.Duration) ([]core.ResourceRecord, error) {
	// Create a temporary storage manager to read data
	storageManager := core.NewManager(h.app.DataFile, 0, false, core.StorageConfig{})

	// Read all records from the data file
	allRecords, err := storageManager.ReadRecords(h.app.DataFile)
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

// calculateComprehensiveStats calculates comprehensive statistics
func (h *StatsHandler) calculateComprehensiveStats() (*StatsResponse, error) {
	// Get recent records (last hour)
	records, err := h.readRecentRecords(time.Hour)
	if err != nil {
		return nil, err
	}

	// Get system information
	systemStats := h.getSystemStats()
	processStats := h.getProcessStats(records)
	timeline := h.generateTimeline(records, time.Hour)

	return &StatsResponse{
		System:      systemStats,
		Processes:   processStats,
		Timeline:    timeline,
		GeneratedAt: time.Now(),
		Period:      "1h",
	}, nil
}

// calculateSummaryStats calculates summary statistics
func (h *StatsHandler) calculateSummaryStats() (map[string]interface{}, error) {
	// Get system information
	totalMemoryMB := core.SystemMemoryMB()
	cpuCores := core.SystemCPUCores()

	// Get recent records
	records, err := h.readRecentRecords(5 * time.Minute)
	if err != nil {
		return nil, err
	}

	// Calculate process statistics
	processMap := make(map[string]*processSummary)
	for _, r := range records {
		if _, exists := processMap[r.Name]; !exists {
			processMap[r.Name] = &processSummary{
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
	categoryStats := make(map[string]int)

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

		// Category statistics
		category := ps.lastRecord.Category
		if category == "" {
			category = "unknown"
		}
		categoryStats[category]++
	}

	memoryPercent := 0.0
	if totalMemoryMB > 0 {
		memoryPercent = (totalMem / totalMemoryMB) * 100
	}

	return map[string]interface{}{
		"processCount":  len(processMap),
		"activeCount":   activeCount,
		"totalCPU":      totalCPU,
		"maxCPU":        maxCPU,
		"totalMemory":   totalMem,
		"memoryPercent": memoryPercent,
		"maxMemory":     maxMem,
		"cpuCores":      cpuCores,
		"totalMemoryMB": totalMemoryMB,
		"categoryStats": categoryStats,
		"generatedAt":   time.Now(),
	}, nil
}

// calculateTimelineStats calculates timeline statistics
func (h *StatsHandler) calculateTimelineStats(duration time.Duration) ([]TimelinePoint, error) {
	records, err := h.readRecentRecords(duration)
	if err != nil {
		return nil, err
	}

	return h.generateTimeline(records, duration), nil
}

// calculateTopProcesses calculates top processes by metric
func (h *StatsHandler) calculateTopProcesses(records []core.ResourceRecord, metric string, limit int) ([]ProcessSummary, error) {
	if len(records) == 0 {
		return []ProcessSummary{}, nil
	}

	totalMemoryMB := core.SystemMemoryMB()

	// Group by process name and get latest
	processMap := make(map[string]*processSummary)
	for _, r := range records {
		if _, exists := processMap[r.Name]; !exists {
			processMap[r.Name] = &processSummary{
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

	// Convert to slice
	var processes []ProcessSummary
	for _, ps := range processMap {
		avgCPU := ps.totalCPU / float64(ps.count)
		avgMem := ps.totalMem / float64(ps.count)

		memoryPercent := 0.0
		if totalMemoryMB > 0 {
			memoryPercent = (avgMem / totalMemoryMB) * 100
		}

		uptime := ""
		if ps.lastRecord.CreateTime > 0 {
			startTime := time.UnixMilli(ps.lastRecord.CreateTime)
			uptime = formatUptime(time.Since(startTime))
		}

		processes = append(processes, ProcessSummary{
			PID:           ps.lastRecord.PID,
			Name:          ps.name,
			CPUPercent:    avgCPU,
			MemoryMB:      avgMem,
			MemoryPercent: memoryPercent,
			Status:        (func(isActive bool) string { if isActive { return "active" } else { return "idle" } })(ps.lastRecord.IsActive),
			Category:      ps.lastRecord.Category,
			Command:       ps.lastRecord.Command,
			Uptime:        uptime,
		})
	}

	// Sort by metric
	switch metric {
	case "cpu", "cpuPercent":
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].CPUPercent > processes[j].CPUPercent
		})
	case "memory", "memoryMb":
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].MemoryMB > processes[j].MemoryMB
		})
	case "name":
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].Name < processes[j].Name
		})
	default:
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].CPUPercent > processes[j].CPUPercent
		})
	}

	// Apply limit
	if len(processes) > limit {
		processes = processes[:limit]
	}

	return processes, nil
}

// calculateResourceStats calculates resource usage statistics
func (h *StatsHandler) calculateResourceStats() (map[string]interface{}, error) {
	// Get system information
	totalMemoryMB := core.SystemMemoryMB()
	cpuCores := core.SystemCPUCores()

	// Get recent records
	records, err := h.readRecentRecords(time.Hour)
	if err != nil {
		return nil, err
	}

	// Calculate resource usage
	cpuUsage := 0.0
	memoryUsage := 0.0
	if len(records) > 0 {
		// Calculate average CPU usage
		for _, record := range records {
			cpuUsage += record.CPUPercentNormalized
			memoryUsage += record.MemoryMB
		}
		cpuUsage /= float64(len(records))
		memoryUsage /= float64(len(records))
	}

	memoryPercent := 0.0
	if totalMemoryMB > 0 {
		memoryPercent = (memoryUsage / totalMemoryMB) * 100
	}

	return map[string]interface{}{
		"cpu": map[string]interface{}{
			"cores":    cpuCores,
			"usage":    cpuUsage,
			"percent":  (cpuUsage / float64(cpuCores)) * 100,
		},
		"memory": map[string]interface{}{
			"total":     totalMemoryMB,
			"used":      memoryUsage,
			"percent":   memoryPercent,
			"available": totalMemoryMB - memoryUsage,
		},
		"generatedAt": time.Now(),
	}, nil
}

// calculateHistoryStats calculates historical statistics
func (h *StatsHandler) calculateHistoryStats(duration time.Duration, granularity string) ([]map[string]interface{}, error) {
	records, err := h.readRecentRecords(duration)
	if err != nil {
		return nil, err
	}

	// Parse granularity
	var bucketSize time.Duration
	switch granularity {
	case "1m":
		bucketSize = time.Minute
	case "5m":
		bucketSize = 5 * time.Minute
	case "15m":
		bucketSize = 15 * time.Minute
	case "1h":
		bucketSize = time.Hour
	case "6h":
		bucketSize = 6 * time.Hour
	case "1d":
		bucketSize = 24 * time.Hour
	default:
		bucketSize = time.Hour
	}

	// Group records into buckets
	buckets := make(map[string][]core.ResourceRecord)
	for _, r := range records {
		key := r.Timestamp.Truncate(bucketSize).Format("2006-01-02T15:04:05Z")
		buckets[key] = append(buckets[key], r)
	}

	// Convert to timeline
	var history []map[string]interface{}
	for _, bucket := range buckets {
		if len(bucket) == 0 {
			continue
		}

		// Calculate bucket statistics
		avgCPU := 0.0
		avgMem := 0.0
		processCount := len(bucket)

		for _, r := range bucket {
			avgCPU += r.CPUPercentNormalized
			avgMem += r.MemoryMB
		}
		avgCPU /= float64(len(bucket))
		avgMem /= float64(len(bucket))

		// Use first record's timestamp as bucket time
		bucketTime := bucket[0].Timestamp

		history = append(history, map[string]interface{}{
			"timestamp":    bucketTime,
			"cpu":          avgCPU,
			"memory":       avgMem,
			"processCount": processCount,
			"count":        len(bucket),
		})
	}

	// Sort by timestamp
	sort.Slice(history, func(i, j int) bool {
		timeI := history[i]["timestamp"].(time.Time)
		timeJ := history[j]["timestamp"].(time.Time)
		return timeI.Before(timeJ)
	})

	return history, nil
}

// Helper data structures
type processSummary struct {
	name       string
	totalCPU   float64
	totalMem   float64
	maxCPU     float64
	maxMem     float64
	count      int
	lastRecord core.ResourceRecord
}

// getSystemStats returns system statistics
func (h *StatsHandler) getSystemStats() SystemStats {
	totalMemoryMB := core.SystemMemoryMB()
	cpuCores := core.SystemCPUCores()

	return SystemStats{
		TotalMemoryMB:    totalMemoryMB,
		CPUCores:         cpuCores,
		Uptime:           "unknown", // Would need system uptime calculation
		LoadAverage:      []float64{0, 0, 0}, // Would need load average calculation
	}
}

// getProcessStats returns process statistics
func (h *StatsHandler) getProcessStats(records []core.ResourceRecord) ProcessStats {
	if len(records) == 0 {
		return ProcessStats{}
	}

	// Track unique processes
	processMap := make(map[string]*processSummary)
	for _, r := range records {
		if _, exists := processMap[r.Name]; !exists {
			processMap[r.Name] = &processSummary{
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
	var totalCPU, totalMem float64
	activeCount := 0
	categoryStats := make(map[string]int)

	for _, ps := range processMap {
		avgCPU := ps.totalCPU / float64(ps.count)
		avgMem := ps.totalMem / float64(ps.count)
		totalCPU += avgCPU
		totalMem += avgMem
		if ps.lastRecord.IsActive {
			activeCount++
		}

		// Category statistics
		category := ps.lastRecord.Category
		if category == "" {
			category = "unknown"
		}
		categoryStats[category]++
	}

	// Get top processes
	topProcesses := h.getTopProcessSummaries(processMap, 10)

	return ProcessStats{
		TotalCount:   len(processMap),
		ActiveCount:  activeCount,
		TopProcesses: topProcesses,
		CategoryStats: categoryStats,
	}
}

// getTopProcessSummaries converts process summaries to ProcessSummary slice
func (h *StatsHandler) getTopProcessSummaries(processMap map[string]*processSummary, n int) []ProcessSummary {
	totalMemoryMB := core.SystemMemoryMB()

	var processes []ProcessSummary
	for _, ps := range processMap {
		avgCPU := ps.totalCPU / float64(ps.count)
		avgMem := ps.totalMem / float64(ps.count)

		memoryPercent := 0.0
		if totalMemoryMB > 0 {
			memoryPercent = (avgMem / totalMemoryMB) * 100
		}

		uptime := ""
		if ps.lastRecord.CreateTime > 0 {
			startTime := time.UnixMilli(ps.lastRecord.CreateTime)
			uptime = formatUptime(time.Since(startTime))
		}

		processes = append(processes, ProcessSummary{
			PID:           ps.lastRecord.PID,
			Name:          ps.name,
			CPUPercent:    avgCPU,
			MemoryMB:      avgMem,
			MemoryPercent: memoryPercent,
			Status:        (func(isActive bool) string { if isActive { return "active" } else { return "idle" } })(ps.lastRecord.IsActive),
			Category:      ps.lastRecord.Category,
			Command:       ps.lastRecord.Command,
			Uptime:        uptime,
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

// generateTimeline generates timeline data points
func (h *StatsHandler) generateTimeline(records []core.ResourceRecord, duration time.Duration) []TimelinePoint {
	if len(records) == 0 {
		return []TimelinePoint{}
	}

	// Determine bucket size based on duration
	var bucketSize time.Duration
	switch {
	case duration <= time.Hour:
		bucketSize = 5 * time.Minute
	case duration <= 24*time.Hour:
		bucketSize = 30 * time.Minute
	default:
		bucketSize = 2 * time.Hour
	}

	// Group records into buckets
	buckets := make(map[string][]core.ResourceRecord)
	for _, r := range records {
		key := r.Timestamp.Truncate(bucketSize).Format("2006-01-02T15:04:05Z")
		buckets[key] = append(buckets[key], r)
	}

	// Convert to timeline
	var timeline []TimelinePoint
	for _, bucket := range buckets {
		if len(bucket) == 0 {
			continue
		}

		// Calculate averages for the bucket
		avgCPU := 0.0
		avgMem := 0.0
		for _, r := range bucket {
			avgCPU += r.CPUPercentNormalized
			avgMem += r.MemoryMB
		}
		avgCPU /= float64(len(bucket))
		avgMem /= float64(len(bucket))

		// Use first record's timestamp as bucket time
		bucketTime := bucket[0].Timestamp

		timeline = append(timeline, TimelinePoint{
			Timestamp:    bucketTime,
			CPU:          avgCPU,
			Memory:       avgMem,
			ProcessCount: len(bucket),
		})
	}

	// Sort by timestamp
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})

	return timeline
}