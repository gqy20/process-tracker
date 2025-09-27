package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	useStorageMgr   bool
	storageConfig  StorageConfig
}

// NewManager creates a new storage manager
func NewManager(dataFile string, bufferSize int, useStorageManager bool, storageConfig StorageConfig) *Manager {
	return &Manager{
		dataFile:       dataFile,
		bufferSize:     bufferSize,
		buffer:         make([]ResourceRecord, 0, bufferSize),
		useStorageMgr:   useStorageManager,
		storageConfig:  storageConfig,
	}
}

// Initialize initializes the storage manager
func (m *Manager) Initialize() error {
	if err := m.initializeFile(); err != nil {
		return fmt.Errorf("failed to initialize file: %w", err)
	}

	if m.useStorageMgr {
		sm := NewStorageManager(m.dataFile, m.storageConfig)
		m.storageManager = sm
	}

	return nil
}

// Close closes all file handles and flushes remaining data
func (m *Manager) Close() error {
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
	m.buffer = append(m.buffer, record)
	
	if len(m.buffer) >= m.bufferSize {
		return m.flushBuffer()
	}
	return nil
}

// DetectDataFormat detects the format version of a data file
func (m *Manager) DetectDataFormat(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, fmt.Errorf("empty file")
	}

	line := scanner.Text()
	fields := strings.Split(line, ",")
	
	switch len(fields) {
	case 7: // Original format: timestamp,name,cpu,memory,threads,disk_io,network_io
		return 1, nil
	case 11: // Format v2: added activity status
		return 2, nil
	case 13: // Format v3: added command and working directory
		return 3, nil
	case 14: // Format v4: added category
		return 4, nil
	default:
		return 0, fmt.Errorf("unknown format with %d fields", len(fields))
	}
}

// ReadRecords reads resource records from file
func (m *Manager) ReadRecords(filePath string) ([]ResourceRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	format, err := m.DetectDataFormat(filePath)
	if err != nil {
		return nil, err
	}

	var records []ResourceRecord
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		record, err := m.parseRecord(scanner.Text(), format)
		if err != nil {
			continue // Skip malformed records
		}
		records = append(records, record)
	}

	return records, nil
}

// parseRecord parses a single line into a ResourceRecord
func (m *Manager) parseRecord(line string, format int) (ResourceRecord, error) {
	fields := strings.Split(line, ",")
	if len(fields) < 7 {
		return ResourceRecord{}, fmt.Errorf("invalid record format")
	}

	record := ResourceRecord{}
	
	// Parse timestamp (common to all formats)
	timestamp, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	record.Timestamp = time.Unix(timestamp, 0)

	// Parse basic fields (common to all formats)
	record.Name = fields[1]
	if cpu, err := strconv.ParseFloat(fields[2], 64); err == nil {
		record.CPUPercent = cpu
	}
	if mem, err := strconv.ParseFloat(fields[3], 64); err == nil {
		record.MemoryMB = mem
	}
	if threads, err := strconv.ParseInt(fields[4], 10, 32); err == nil {
		record.Threads = int32(threads)
	}
	if diskRead, err := strconv.ParseFloat(fields[5], 64); err == nil {
		record.DiskReadMB = diskRead
	}
	if diskWrite, err := strconv.ParseFloat(fields[6], 64); err == nil {
		record.DiskWriteMB = diskWrite
	}

	// Parse format-specific fields
	if format >= 1 && len(fields) > 7 {
		if netSent, err := strconv.ParseFloat(fields[7], 64); err == nil {
			record.NetSentKB = netSent
		}
		if netRecv, err := strconv.ParseFloat(fields[8], 64); err == nil {
			record.NetRecvKB = netRecv
		}
	}

	if format >= 2 && len(fields) > 10 {
		if active, err := strconv.ParseBool(fields[10]); err == nil {
			record.IsActive = active
		}
	}

	if format >= 3 && len(fields) > 12 {
		record.Command = fields[11]
		record.WorkingDir = fields[12]
	}

	if format >= 4 && len(fields) > 13 {
		record.Category = fields[13]
	}

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
			Name:      name,
			Category:  m.getMostCommonCategory(processRecords),
			Command:   m.getMostCommonCommand(processRecords),
			WorkingDir: m.getMostCommonWorkingDir(processRecords),
			Samples:   len(processRecords),
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
		strconv.FormatFloat(record.MemoryMB, 'f', 2, 64),
		strconv.FormatInt(int64(record.Threads), 10),
		strconv.FormatFloat(record.DiskReadMB, 'f', 2, 64),
		strconv.FormatFloat(record.DiskWriteMB, 'f', 2, 64),
		strconv.FormatFloat(record.NetSentKB, 'f', 2, 64),
		strconv.FormatFloat(record.NetRecvKB, 'f', 2, 64),
		strconv.FormatBool(record.IsActive),
		record.Command,
		record.WorkingDir,
		record.Category,
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