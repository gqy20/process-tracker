package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Manager handles data storage and retrieval operations
type Manager struct {
	dataFile       string
	buffer         []ResourceRecord
	bufferSize     int
	file           *os.File
	writer         *bufio.Writer
	storageManager *StorageManager
	useStorageMgr  bool
	storageConfig  StorageConfig
	mu             sync.RWMutex // Protects buffer from concurrent access
}

// NewManager creates a new storage manager
func NewManager(dataFile string, bufferSize int, useStorageManager bool, storageConfig StorageConfig) *Manager {
	return &Manager{
		dataFile:      dataFile,
		bufferSize:    bufferSize,
		buffer:        make([]ResourceRecord, 0, bufferSize),
		useStorageMgr: useStorageManager,
		storageConfig: storageConfig,
	}
}

// Initialize initializes the storage manager
func (m *Manager) Initialize() error {
	if m.useStorageMgr {
		// Use StorageManager for file management
		sm := NewStorageManager(m.dataFile, m.storageConfig)
		m.storageManager = sm
	} else {
		// Only initialize file directly when not using StorageManager
		if err := m.initializeFile(); err != nil {
			return fmt.Errorf("failed to initialize file: %w", err)
		}
	}

	return nil
}

// Close closes all file handles and flushes remaining data
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.writer != nil {
		if err := m.flushBuffer(); err != nil {
			return err
		}
		if err := m.writer.Flush(); err != nil {
			return err
		}
	}
	if m.file != nil {
		if err := m.file.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SaveRecords saves resource records with buffering
func (m *Manager) SaveRecords(records []ResourceRecord) error {
	for _, record := range records {
		if err := m.SaveRecord(record); err != nil {
			return err
		}
	}
	return nil
}

// SaveRecord saves a single resource record
func (m *Manager) SaveRecord(record ResourceRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.buffer = append(m.buffer, record)

	if len(m.buffer) >= m.bufferSize {
		return m.flushBuffer()
	}
	return nil
}

// ReadRecords reads resource records from file
func (m *Manager) ReadRecords(filePath string) ([]ResourceRecord, error) {
	// Try main file first, then newest rotated file
	filesToTry := []string{filePath}

	// Look for rotated files if main file doesn't exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if matches, _ := filepath.Glob(filePath + ".*"); len(matches) > 0 {
			filesToTry = append(filesToTry, matches...)
		}
	}

	// Try each file until we find one that works
	var lastErr error
	for _, file := range filesToTry {
		if records, err := m.readSingleFile(file); err == nil {
			return records, nil
		} else {
			lastErr = err
		}
	}

	return nil, fmt.Errorf("no readable log files found: %v", lastErr)
}

// readSingleFile reads records from a single file
func (m *Manager) readSingleFile(filePath string) ([]ResourceRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []ResourceRecord
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		record, err := m.parseRecord(scanner.Text())
		if err != nil {
			continue // Skip malformed records
		}
		records = append(records, record)
	}

	return records, nil
}

// parseRecord parses a single line into ResourceRecord
// Supports v7 (18 fields with CPUPercentNormalized), v6 (17 fields with MemoryPercent), and v5 (16 fields) formats
func (m *Manager) parseRecord(line string) (ResourceRecord, error) {
	fields := strings.Split(line, ",")
	
	// Support v5 (16), v6 (17), and v7 (18) formats
	if len(fields) != 16 && len(fields) != 17 && len(fields) != 18 {
		return ResourceRecord{}, fmt.Errorf("invalid format: expected 16, 17, or 18 fields, got %d", len(fields))
	}

	record := ResourceRecord{}

	// Parse common fields (timestamp, name, cpu_percent)
	timestamp, _ := strconv.ParseInt(fields[0], 10, 64)
	record.Timestamp = time.Unix(timestamp, 0)
	record.Name = fields[1]
	record.CPUPercent, _ = strconv.ParseFloat(fields[2], 64)
	
	// Handle different format versions
	var fieldOffset int
	if len(fields) == 18 {
		// v7 format: includes CPUPercentNormalized at position 3, MemoryPercent at position 5
		record.CPUPercentNormalized, _ = strconv.ParseFloat(fields[3], 64)
		record.MemoryMB, _ = strconv.ParseFloat(fields[4], 64)
		record.MemoryPercent, _ = strconv.ParseFloat(fields[5], 64)
		fieldOffset = 2 // Skip both new fields in old format
	} else if len(fields) == 17 {
		// v6 format: includes MemoryPercent at position 4, no CPUPercentNormalized
		record.CPUPercentNormalized = 0 // Will be calculated if needed
		record.MemoryMB, _ = strconv.ParseFloat(fields[3], 64)
		record.MemoryPercent, _ = strconv.ParseFloat(fields[4], 64)
		fieldOffset = 1
	} else {
		// v5 format: no MemoryPercent or CPUPercentNormalized
		record.CPUPercentNormalized = 0
		record.MemoryMB, _ = strconv.ParseFloat(fields[3], 64)
		record.MemoryPercent = 0
		fieldOffset = 0
	}
	
	// Parse remaining fields with offset (starting from threads)
	threads, _ := strconv.ParseInt(fields[4+fieldOffset], 10, 32)
	record.Threads = int32(threads)
	record.DiskReadMB, _ = strconv.ParseFloat(fields[5+fieldOffset], 64)
	record.DiskWriteMB, _ = strconv.ParseFloat(fields[6+fieldOffset], 64)
	record.NetSentKB, _ = strconv.ParseFloat(fields[7+fieldOffset], 64)
	record.NetRecvKB, _ = strconv.ParseFloat(fields[8+fieldOffset], 64)
	record.IsActive, _ = strconv.ParseBool(fields[9+fieldOffset])
	record.Command = fields[10+fieldOffset]
	record.WorkingDir = fields[11+fieldOffset]
	record.Category = fields[12+fieldOffset]
	pid, _ := strconv.ParseInt(fields[13+fieldOffset], 10, 32)
	record.PID = int32(pid)
	record.CreateTime, _ = strconv.ParseInt(fields[14+fieldOffset], 10, 64)
	record.CPUTime, _ = strconv.ParseFloat(fields[15+fieldOffset], 64)

	return record, nil
}

// CalculateStats calculates resource statistics from records
func (m *Manager) CalculateStats(records []ResourceRecord) []ResourceStats {
	// Group records by process name
	processMap := make(map[string][]ResourceRecord)
	for _, record := range records {
		processMap[record.Name] = append(processMap[record.Name], record)
	}

	// Calculate statistics for each process
	var stats []ResourceStats
	for name, processRecords := range processMap {
		if len(processRecords) == 0 {
			continue
		}

		stat := ResourceStats{
			Name:       name,
			Category:   m.getMostCommonCategory(processRecords),
			Command:    m.getMostCommonCommand(processRecords),
			WorkingDir: m.getMostCommonWorkingDir(processRecords),
			Samples:    len(processRecords),
		}

		// Calculate averages and maximums
		var totalCPU, totalMemory, totalDiskRead, totalDiskWrite, totalNetSent, totalNetRecv float64
		var activeSamples int
		var maxCPU, maxMemory float64

		for _, record := range processRecords {
			totalCPU += record.CPUPercent
			totalMemory += record.MemoryMB
			totalDiskRead += record.DiskReadMB
			totalDiskWrite += record.DiskWriteMB
			totalNetSent += record.NetSentKB
			totalNetRecv += record.NetRecvKB

			if record.CPUPercent > maxCPU {
				maxCPU = record.CPUPercent
			}
			if record.MemoryMB > maxMemory {
				maxMemory = record.MemoryMB
			}
			if record.IsActive {
				activeSamples++
			}
		}

		stat.CPUAvg = totalCPU / float64(len(processRecords))
		stat.MemoryAvg = totalMemory / float64(len(processRecords))
		stat.DiskReadAvg = totalDiskRead / float64(len(processRecords))
		stat.DiskWriteAvg = totalDiskWrite / float64(len(processRecords))
		stat.NetSentAvg = totalNetSent / float64(len(processRecords))
		stat.NetRecvAvg = totalNetRecv / float64(len(processRecords))
		stat.CPUMax = maxCPU
		stat.MemoryMax = maxMemory
		stat.ActiveSamples = activeSamples

		// Calculate active time
		stat.ActiveTime = m.calculateActiveTime(processRecords)

		// Calculate new statistics
		pidSet := make(map[int32]bool)
		var firstSeen, lastSeen time.Time
		var earliestCreateTime int64
		var totalCPUTime float64

		for i, record := range processRecords {
			// Collect PIDs
			if record.PID > 0 {
				pidSet[record.PID] = true
			}

			// Track observation times
			if i == 0 || record.Timestamp.Before(firstSeen) {
				firstSeen = record.Timestamp
			}
			if i == 0 || record.Timestamp.After(lastSeen) {
				lastSeen = record.Timestamp
			}

			// Find earliest process start time (CreateTime is Unix milliseconds)
			if record.CreateTime > 0 {
				if earliestCreateTime == 0 || record.CreateTime < earliestCreateTime {
					earliestCreateTime = record.CreateTime
				}
			}

			// Sum CPU time
			if record.CPUTime > 0 {
				totalCPUTime += record.CPUTime
			}
		}

		// Convert PID set to slice
		pids := make([]int32, 0, len(pidSet))
		for pid := range pidSet {
			pids = append(pids, pid)
		}
		stat.PIDs = pids

		// Set observation times
		stat.FirstSeen = firstSeen
		stat.LastSeen = lastSeen

		// Calculate observation duration (monitoring time span)
		if !firstSeen.IsZero() && !lastSeen.IsZero() {
			stat.TotalUptime = lastSeen.Sub(firstSeen)
		}

		// Set process actual start time
		if earliestCreateTime > 0 {
			stat.ProcessStartTime = time.Unix(earliestCreateTime/1000, (earliestCreateTime%1000)*1000000)
		}

		// Calculate CPU time statistics
		stat.TotalCPUTime = time.Duration(totalCPUTime * float64(time.Second))
		if len(processRecords) > 0 {
			stat.AvgCPUTime = totalCPUTime / float64(len(processRecords))
		}

		stats = append(stats, stat)
	}

	// Sort by active time (descending)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].ActiveTime > stats[j].ActiveTime
	})

	return stats
}

// CleanOldData removes old data files
func (m *Manager) CleanOldData(keepDays int) error {
	if m.storageManager != nil {
		// Storage manager handles its own cleanup
		return nil
	}

	// Simple file cleanup for backward compatibility
	cutoff := time.Now().AddDate(0, 0, -keepDays)

	dir := filepath.Dir(m.dataFile)
	base := filepath.Base(m.dataFile)

	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if file matches our pattern
		if strings.HasPrefix(file.Name(), base) {
			info, err := file.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(dir, file.Name()))
			}
		}
	}

	return nil
}

// GetTotalRecords returns the total number of records in the data file
func (m *Manager) GetTotalRecords() (int, error) {
	file, err := os.Open(m.dataFile)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}

	return count, scanner.Err()
}

// GetRecordCount returns the total number of records (alias for GetTotalRecords for interface compatibility)
func (m *Manager) GetRecordCount() (int, error) {
	return m.GetTotalRecords()
}

// GetStorageInfo 获取存储信息
func (m *Manager) GetStorageInfo() StorageInfo {
	info := StorageInfo{
		Type:     "csv",
		FilePath: m.dataFile,
	}

	// 获取记录总数
	if count, err := m.GetRecordCount(); err == nil {
		info.TotalRecords = count
	}

	// 获取文件大小
	if stat, err := os.Stat(m.dataFile); err == nil {
		info.TotalSize = stat.Size()
		info.LastModified = stat.ModTime()
	}

	// 获取最早和最新记录时间
	// 对于CSV文件，这里简单处理，实际可以通过读取文件解析时间
	info.OldestRecord = time.Now() // 默认值
	info.NewestRecord = time.Now() // 默认值

	return info
}

// ReadRecordsByTimeRange 按时间范围读取记录 (CSV实现)
func (m *Manager) ReadRecordsByTimeRange(start, end time.Time) ([]ResourceRecord, error) {
	// 读取所有记录，然后过滤
	allRecords, err := m.ReadRecords(m.dataFile)
	if err != nil {
		return nil, err
	}

	var filteredRecords []ResourceRecord
	for _, record := range allRecords {
		if record.Timestamp.After(start) && record.Timestamp.Before(end) {
			filteredRecords = append(filteredRecords, record)
		}
	}

	return filteredRecords, nil
}

// Private helper methods

func (m *Manager) initializeFile() error {
	if m.file != nil {
		return nil // Already initialized
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(m.dataFile), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(m.dataFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	m.file = file
	m.writer = bufio.NewWriter(file)
	return nil
}

// flushBuffer writes buffered records to disk
// NOTE: This method must be called while holding m.mu lock
func (m *Manager) flushBuffer() error {
	if len(m.buffer) == 0 {
		return nil
	}

	// Use storage manager if enabled (handles rotation)
	if m.useStorageMgr && m.storageManager != nil {
		for _, record := range m.buffer {
			line := m.formatRecord(record)
			if err := m.storageManager.WriteRecord(line); err != nil {
				return err
			}
		}
		m.buffer = m.buffer[:0] // Clear buffer
		return nil
	}

	// Fall back to direct file writing for backward compatibility
	if m.writer == nil {
		if err := m.initializeFile(); err != nil {
			return err
		}
	}

	for _, record := range m.buffer {
		line := m.formatRecord(record)
		if _, err := m.writer.WriteString(line); err != nil {
			return err
		}
	}

	m.buffer = m.buffer[:0] // Clear buffer
	return m.writer.Flush()
}

func (m *Manager) formatRecord(record ResourceRecord) string {
	fields := []string{
		strconv.FormatInt(record.Timestamp.Unix(), 10),
		record.Name,
		strconv.FormatFloat(record.CPUPercent, 'f', 2, 64),
		strconv.FormatFloat(record.CPUPercentNormalized, 'f', 3, 64), // v7: normalized CPU percentage
		strconv.FormatFloat(record.MemoryMB, 'f', 2, 64),
		strconv.FormatFloat(record.MemoryPercent, 'f', 2, 64), // v6: memory percentage
		strconv.FormatInt(int64(record.Threads), 10),
		strconv.FormatFloat(record.DiskReadMB, 'f', 2, 64),
		strconv.FormatFloat(record.DiskWriteMB, 'f', 2, 64),
		strconv.FormatFloat(record.NetSentKB, 'f', 2, 64),
		strconv.FormatFloat(record.NetRecvKB, 'f', 2, 64),
		strconv.FormatBool(record.IsActive),
		record.Command,
		record.WorkingDir,
		record.Category,
		strconv.FormatInt(int64(record.PID), 10),
		strconv.FormatInt(record.CreateTime, 10),
		strconv.FormatFloat(record.CPUTime, 'f', 2, 64),
	}
	return strings.Join(fields, ",") + "\n"
}

func (m *Manager) calculateActiveTime(records []ResourceRecord) time.Duration {
	if len(records) == 0 {
		return 0
	}

	// Simple heuristic: if more than 50% of samples are active, consider the process active
	activeCount := 0
	for _, record := range records {
		if record.IsActive {
			activeCount++
		}
	}

	if float64(activeCount)/float64(len(records)) > 0.5 {
		// Estimate active time based on record timestamps
		if len(records) > 1 {
			timeSpan := records[len(records)-1].Timestamp.Sub(records[0].Timestamp)
			return time.Duration(float64(timeSpan) * float64(activeCount) / float64(len(records)))
		}
		return 5 * time.Minute // Default assumption
	}

	return 0
}

func (m *Manager) getMostCommonCategory(records []ResourceRecord) string {
	categoryCount := make(map[string]int)
	for _, record := range records {
		if record.Category != "" {
			categoryCount[record.Category]++
		}
	}

	return m.getMostCommon(categoryCount)
}

func (m *Manager) getMostCommonCommand(records []ResourceRecord) string {
	commandCount := make(map[string]int)
	for _, record := range records {
		if record.Command != "" {
			commandCount[record.Command]++
		}
	}

	return m.getMostCommon(commandCount)
}

func (m *Manager) getMostCommonWorkingDir(records []ResourceRecord) string {
	dirCount := make(map[string]int)
	for _, record := range records {
		if record.WorkingDir != "" {
			dirCount[record.WorkingDir]++
		}
	}

	return m.getMostCommon(dirCount)
}

func (m *Manager) getMostCommon(countMap map[string]int) string {
	if len(countMap) == 0 {
		return ""
	}

	var mostCommon string
	maxCount := 0
	for item, count := range countMap {
		if count > maxCount {
			maxCount = count
			mostCommon = item
		}
	}

	return mostCommon
}
