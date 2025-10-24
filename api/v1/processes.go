package v1

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/process-tracker/core"
)

// ProcessHandler handles process-related API endpoints
type ProcessHandler struct {
	app *core.App
}

// NewProcessHandler creates a new process handler
func NewProcessHandler(app *core.App) *ProcessHandler {
	return &ProcessHandler{app: app}
}

// ListProcesses returns a list of processes with filtering, sorting, and pagination
func (h *ProcessHandler) ListProcesses(c *gin.Context) {
	params := c.MustGet("query_params").(QueryParams)

	// Get view mode (tree or flat)
	view := params.View
	if view == "" {
		view = "flat"
	}

	// Get recent processes from storage
	records, err := h.readRecentRecords(5 * time.Minute)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to read process records: %w", err))
		return
	}

	var response interface{}
	if view == "tree" {
		response = h.getProcessTree(records)
	} else {
		response = h.getProcessList(records, params)
	}

	metadata := &ResponseMetadata{
		View:        view,
		Sort:        params.Sort,
		GeneratedAt: time.Now(),
	}

	SendSuccess(c, KindProcessList, response, metadata)
}

// GetProcess returns a specific process by PID
func (h *ProcessHandler) GetProcess(c *gin.Context) {
	pidStr := c.Param("pid")
	pid, err := strconv.ParseInt(pidStr, 10, 32)
	if err != nil {
		SendBadRequest(c, "Invalid PID: "+pidStr)
		return
	}

	// Get recent processes
	records, err := h.readRecentRecords(5 * time.Minute)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to read process records: %w", err))
		return
	}

	// Find the specific process
	var process *core.ResourceRecord
	for _, record := range records {
		if record.PID == int32(pid) {
			process = &record
			break
		}
	}

	if process == nil {
		SendNotFoundError(c, "process", pid)
		return
	}

	response := h.processToResponse(process)
	SendSuccess(c, KindProcess, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetProcessChildren returns child processes of a specific process
func (h *ProcessHandler) GetProcessChildren(c *gin.Context) {
	pidStr := c.Param("pid")
	pid, err := strconv.ParseInt(pidStr, 10, 32)
	if err != nil {
		SendBadRequest(c, "Invalid PID: "+pidStr)
		return
	}

	// Get recent processes
	records, err := h.readRecentRecords(5 * time.Minute)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to read process records: %w", err))
		return
	}

	// Build process tree
	trees := core.BuildProcessTree(records)

	// Find children of the specified PID
	var children []ProcessResponse
	for _, tree := range trees {
		if tree.Process.PID == int32(pid) {
			for _, child := range tree.Children {
				children = append(children, h.processTreeToResponse(child))
			}
			break
		}
	}

	SendSuccess(c, KindProcessList, children, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetProcessTree returns the process tree for a specific process
func (h *ProcessHandler) GetProcessTree(c *gin.Context) {
	pidStr := c.Param("pid")
	pid, err := strconv.ParseInt(pidStr, 10, 32)
	if err != nil {
		SendBadRequest(c, "Invalid PID: "+pidStr)
		return
	}

	// Get recent processes
	records, err := h.readRecentRecords(5 * time.Minute)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to read process records: %w", err))
		return
	}

	// Build process tree
	trees := core.BuildProcessTree(records)

	// Find the specific process tree
	var targetTree *core.ProcessTreeNode
	for _, tree := range trees {
		if tree.Process.PID == int32(pid) {
			targetTree = tree
			break
		}
		// Also search in children
		if targetTree == nil {
			if found := h.findProcessInTree(tree, int32(pid)); found != nil {
				targetTree = found
			}
		}
	}

	if targetTree == nil {
		SendNotFoundError(c, "process", pid)
		return
	}

	response := h.processTreeToResponse(targetTree)
	SendSuccess(c, KindProcess, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// GetLiveProcesses returns real-time process data
func (h *ProcessHandler) GetLiveProcesses(c *gin.Context) {
	// Get very recent records (last 30 seconds)
	records, err := h.readRecentRecords(30 * time.Second)
	if err != nil {
		SendInternalServerError(c, fmt.Errorf("failed to read live process records: %w", err))
		return
	}

	// Get latest processes
	processes := h.getLatestProcesses(records)

	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"processes": processes,
		"count":     len(processes),
	}

	SendSuccess(c, KindProcessList, response, &ResponseMetadata{
		GeneratedAt: time.Now(),
	})
}

// Helper functions

// readRecentRecords reads recent process records from storage
func (h *ProcessHandler) readRecentRecords(duration time.Duration) ([]core.ResourceRecord, error) {
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

// getProcessTree returns the process tree structure
func (h *ProcessHandler) getProcessTree(records []core.ResourceRecord) map[string]interface{} {
	// Get latest record for each process
	latest := make(map[int32]core.ResourceRecord)
	for _, r := range records {
		if existing, ok := latest[r.PID]; !ok || r.Timestamp.After(existing.Timestamp) {
			latest[r.PID] = r
		}
	}

	// Convert map to slice
	var latestRecords []core.ResourceRecord
	for _, r := range latest {
		latestRecords = append(latestRecords, r)
	}

	// Build process tree
	tree := core.BuildProcessTree(latestRecords)

	// Convert to response format
	var trees []map[string]interface{}
	for _, node := range tree {
		trees = append(trees, h.processTreeToMap(node))
	}

	return map[string]interface{}{
		"tree": trees,
		"view": "tree",
	}
}

// getProcessList returns the flat process list
func (h *ProcessHandler) getProcessList(records []core.ResourceRecord, params QueryParams) []ProcessResponse {
	processes := h.getLatestProcesses(records)

	// Apply sorting
	if params.Sort != "" {
		h.sortProcesses(processes, params.Sort)
	}

	// Apply pagination
	limit := params.Limit
	offset := params.Offset
	if limit == 0 {
		limit = 20
	}

	total := len(processes)
	start := offset
	end := offset + limit
	if start > total {
		return []ProcessResponse{}
	}
	if end > total {
		end = total
	}

	return processes[start:end]
}

// getLatestProcesses gets the latest record for each process
func (h *ProcessHandler) getLatestProcesses(records []core.ResourceRecord) []ProcessResponse {
	if len(records) == 0 {
		return []ProcessResponse{}
	}

	// Get latest record for each process
	latest := make(map[int32]core.ResourceRecord)
	for _, r := range records {
		if existing, ok := latest[r.PID]; !ok || r.Timestamp.After(existing.Timestamp) {
			latest[r.PID] = r
		}
	}

	// Convert to ProcessResponse
	var processes []ProcessResponse
	for _, r := range latest {
		process := h.processToResponse(&r)
		processes = append(processes, process)
	}

	return processes
}

// processToResponse converts a ResourceRecord to ProcessResponse
func (h *ProcessHandler) processToResponse(record *core.ResourceRecord) ProcessResponse {
	totalMemoryMB := core.SystemMemoryMB()

	memoryPercent := 0.0
	if totalMemoryMB > 0 {
		memoryPercent = (record.MemoryMB / totalMemoryMB) * 100
	}

	uptime := ""
	if record.CreateTime > 0 {
		startTime := time.UnixMilli(record.CreateTime)
		uptime = formatUptime(time.Since(startTime))
	}

	status := "idle"
	if record.IsActive {
		status = "active"
	}

	return ProcessResponse{
		PID:           record.PID,
		PPID:          record.PPID,
		Name:          record.Name,
		Command:       record.Command,
		Status:        status,
		CPUPercent:    record.CPUPercentNormalized,
		MemoryMB:      record.MemoryMB,
		MemoryPercent: memoryPercent,
		Category:      record.Category,
		WorkDir:       record.WorkingDir,
		CreatedAt:     record.Timestamp,
		IsActive:      record.IsActive,
		Uptime:        uptime,
	}
}

// processTreeToResponse converts a ProcessTreeNode to ProcessResponse
func (h *ProcessHandler) processTreeToResponse(node *core.ProcessTreeNode) ProcessResponse {
	process := h.processToResponse(&node.Process)

	// Add children recursively
	for _, child := range node.Children {
		process.Children = append(process.Children, h.processTreeToResponse(child))
	}

	return process
}

// processTreeToMap converts a ProcessTreeNode to a map for JSON response
func (h *ProcessHandler) processTreeToMap(node *core.ProcessTreeNode) map[string]interface{} {
	process := h.processToResponse(&node.Process)

	// Convert to map
	result := map[string]interface{}{
		"pid":           process.PID,
		"ppid":          process.PPID,
		"name":          process.Name,
		"command":       process.Command,
		"status":        process.Status,
		"cpuPercent":    process.CPUPercent,
		"memoryMb":      process.MemoryMB,
		"memoryPercent": process.MemoryPercent,
		"category":      process.Category,
		"workDir":       process.WorkDir,
		"createdAt":     process.CreatedAt,
		"isActive":      process.IsActive,
		"uptime":        process.Uptime,
	}

	// Add children recursively
	if len(node.Children) > 0 {
		var children []map[string]interface{}
		for _, child := range node.Children {
			children = append(children, h.processTreeToMap(child))
		}
		result["children"] = children
	}

	return result
}

// findProcessInTree searches for a process in a process tree
func (h *ProcessHandler) findProcessInTree(node *core.ProcessTreeNode, pid int32) *core.ProcessTreeNode {
	if node.Process.PID == pid {
		return node
	}

	for _, child := range node.Children {
		if found := h.findProcessInTree(child, pid); found != nil {
			return found
		}
	}

	return nil
}

// sortProcesses sorts processes by the specified field
func (h *ProcessHandler) sortProcesses(processes []ProcessResponse, sortField string) {
	// Parse sort direction (default is descending for processes)
	direction := "desc"
	if len(sortField) > 0 && sortField[0] != '-' {
		direction = "asc"
	}

	field := sortField
	if len(sortField) > 0 && sortField[0] == '-' {
		field = sortField[1:]
	}

	switch field {
	case "pid":
		h.sortByPID(processes, direction)
	case "name":
		h.sortByName(processes, direction)
	case "cpu", "cpuPercent":
		h.sortByCPU(processes, direction)
	case "memory", "memoryMb":
		h.sortByMemory(processes, direction)
	case "status":
		h.sortByStatus(processes, direction)
	case "category":
		h.sortByCategory(processes, direction)
	}
}

// Sorting functions
func (h *ProcessHandler) sortByPID(processes []ProcessResponse, direction string) {
	if direction == "asc" {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].PID > processes[j].PID {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	} else {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].PID < processes[j].PID {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	}
}

func (h *ProcessHandler) sortByName(processes []ProcessResponse, direction string) {
	if direction == "asc" {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].Name > processes[j].Name {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	} else {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].Name < processes[j].Name {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	}
}

func (h *ProcessHandler) sortByCPU(processes []ProcessResponse, direction string) {
	if direction == "asc" {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].CPUPercent > processes[j].CPUPercent {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	} else {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].CPUPercent < processes[j].CPUPercent {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	}
}

func (h *ProcessHandler) sortByMemory(processes []ProcessResponse, direction string) {
	if direction == "asc" {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].MemoryMB > processes[j].MemoryMB {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	} else {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].MemoryMB < processes[j].MemoryMB {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	}
}

func (h *ProcessHandler) sortByStatus(processes []ProcessResponse, direction string) {
	if direction == "asc" {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].Status > processes[j].Status {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	} else {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].Status < processes[j].Status {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	}
}

func (h *ProcessHandler) sortByCategory(processes []ProcessResponse, direction string) {
	if direction == "asc" {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].Category > processes[j].Category {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	} else {
		for i := 0; i < len(processes)-1; i++ {
			for j := i + 1; j < len(processes); j++ {
				if processes[i].Category < processes[j].Category {
					processes[i], processes[j] = processes[j], processes[i]
				}
			}
		}
	}
}

// formatUptime formats duration into human readable string
func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	} else {
		days := int(d.Hours() / 24)
		hours := d.Hours() - float64(days*24)
		if hours > 0.1 {
			return fmt.Sprintf("%dd%.1fh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
}