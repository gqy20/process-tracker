package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewManager tests Manager creation
func TestNewManager(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-manager.log")
	defer os.Remove(tmpFile)

	config := StorageConfig{
		MaxSizeMB: 100,
		KeepDays:  7,
	}

	manager := NewManager(tmpFile, 100, true, config)

	if manager == nil {
		t.Fatal("Expected Manager to be created")
	}

	if manager.dataFile != tmpFile {
		t.Errorf("Expected dataFile %s, got %s", tmpFile, manager.dataFile)
	}

	if manager.bufferSize != 100 {
		t.Errorf("Expected bufferSize 100, got %d", manager.bufferSize)
	}

	if !manager.useStorageMgr {
		t.Error("Expected useStorageMgr to be true")
	}
}

// TestManager_InitializeAndClose tests initialization and cleanup
func TestManager_InitializeAndClose(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-init.log")
	defer os.Remove(tmpFile)

	config := GetDefaultStorageConfig()
	manager := NewManager(tmpFile, 10, false, config) // Not using StorageManager

	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// File should exist after initialization (when not using StorageManager)
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("Expected data file to be created")
	}

	err = manager.Close()
	if err != nil {
		t.Errorf("Failed to close: %v", err)
	}
}

// TestSaveAndRead tests the write-read cycle
func TestSaveAndRead(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-save-read.log")
	defer os.Remove(tmpFile)

	config := GetDefaultStorageConfig()
	manager := NewManager(tmpFile, 100, false, config)

	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	defer manager.Close()

	// Create test records
	now := time.Now()
	records := []ResourceRecord{
		{
			Name:         "test-process-1",
			Timestamp:    now,
			CPUPercent:   75.5,
			MemoryMB:     1024.0,
			Threads:      5,
			DiskReadMB:   10.0,
			DiskWriteMB:  5.0,
			NetSentKB:    100.0,
			NetRecvKB:    200.0,
			IsActive:     true,
			Command:      "/usr/bin/test",
			WorkingDir:   "/home/user",
			Category:     "development",
			PID:          12345,
			CreateTime:   now.Unix() * 1000,
			CPUTime:      123.45,
		},
		{
			Name:         "test-process-2",
			Timestamp:    now.Add(time.Second),
			CPUPercent:   50.0,
			MemoryMB:     512.0,
			IsActive:     false,
			Category:     "system",
		},
	}

	// Save records
	err = manager.SaveRecords(records)
	if err != nil {
		t.Fatalf("Failed to save records: %v", err)
	}

	// Force flush
	err = manager.Close()
	if err != nil {
		t.Fatalf("Failed to close manager: %v", err)
	}

	// Read back records
	readManager := NewManager(tmpFile, 100, false, config)
	readRecords, err := readManager.ReadRecords(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read records: %v", err)
	}

	if len(readRecords) != len(records) {
		t.Fatalf("Expected %d records, got %d", len(records), len(readRecords))
	}

	// Verify first record
	r := readRecords[0]
	if r.Name != "test-process-1" {
		t.Errorf("Expected name 'test-process-1', got '%s'", r.Name)
	}
	if r.CPUPercent != 75.5 {
		t.Errorf("Expected CPU 75.5%%, got %.2f%%", r.CPUPercent)
	}
	if r.MemoryMB != 1024.0 {
		t.Errorf("Expected memory 1024MB, got %.2fMB", r.MemoryMB)
	}
	if r.Category != "development" {
		t.Errorf("Expected category 'development', got '%s'", r.Category)
	}
}

// TestBuffering tests buffer flush mechanism
func TestBuffering(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-buffer.log")
	defer os.Remove(tmpFile)

	config := GetDefaultStorageConfig()
	bufferSize := 5
	manager := NewManager(tmpFile, bufferSize, false, config)

	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	defer manager.Close()

	// Add records one at a time
	for i := 0; i < bufferSize-1; i++ {
		record := ResourceRecord{
			Name:       "test-process",
			Timestamp:  time.Now(),
			CPUPercent: float64(i * 10),
		}
		err = manager.SaveRecord(record)
		if err != nil {
			t.Fatalf("Failed to save record %d: %v", i, err)
		}
	}

	// Buffer should not have flushed yet
	// Check current buffer length
	manager.mu.RLock()
	bufferLen := len(manager.buffer)
	manager.mu.RUnlock()

	if bufferLen != bufferSize-1 {
		t.Errorf("Expected buffer length %d, got %d", bufferSize-1, bufferLen)
	}

	// Add one more record to trigger flush
	record := ResourceRecord{
		Name:       "test-process",
		Timestamp:  time.Now(),
		CPUPercent: 99.0,
	}
	err = manager.SaveRecord(record)
	if err != nil {
		t.Fatalf("Failed to save final record: %v", err)
	}

	// Buffer should be cleared after flush
	manager.mu.RLock()
	bufferLen = len(manager.buffer)
	manager.mu.RUnlock()

	if bufferLen != 0 {
		t.Errorf("Expected buffer to be cleared, got length %d", bufferLen)
	}
}

// TestDataFormatV7 tests v7 format (18 fields) parsing
func TestDataFormatV7(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-v7.log")
	defer os.Remove(tmpFile)

	// Create v7 format data manually
	v7Data := "1729000000,test-process,75.50,1.049,1024.00,0.32,5,10.00,5.00,100.00,200.00,true,/usr/bin/test,/home/user,development,12345,1729000000000,123.45\n"
	
	err := os.WriteFile(tmpFile, []byte(v7Data), 0644)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	config := GetDefaultStorageConfig()
	manager := NewManager(tmpFile, 100, false, config)

	records, err := manager.ReadRecords(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read v7 format: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.Name != "test-process" {
		t.Errorf("Expected name 'test-process', got '%s'", r.Name)
	}
	if r.CPUPercent != 75.50 {
		t.Errorf("Expected CPU 75.50%%, got %.2f%%", r.CPUPercent)
	}
	if r.CPUPercentNormalized != 1.049 {
		t.Errorf("Expected normalized CPU 1.049%%, got %.3f%%", r.CPUPercentNormalized)
	}
	if r.MemoryMB != 1024.00 {
		t.Errorf("Expected memory 1024MB, got %.2fMB", r.MemoryMB)
	}
	if r.MemoryPercent != 0.32 {
		t.Errorf("Expected memory percent 0.32%%, got %.2f%%", r.MemoryPercent)
	}
}

// TestEmptyFile tests reading from non-existent file
func TestEmptyFile(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "non-existent.log")

	config := GetDefaultStorageConfig()
	manager := NewManager(tmpFile, 100, false, config)

	_, err := manager.ReadRecords(tmpFile)
	if err == nil {
		t.Error("Expected error when reading non-existent file")
	}
}

// TestMalformedData tests handling of invalid data
func TestMalformedData(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-malformed.log")
	defer os.Remove(tmpFile)

	// Create malformed data
	malformedData := "invalid,data,format\n1729000000,valid-process,75.50,1.049,1024.00,0.32,5,10.00,5.00,100.00,200.00,true,/usr/bin/test,/home/user,development,12345,1729000000000,123.45\n"
	
	err := os.WriteFile(tmpFile, []byte(malformedData), 0644)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	config := GetDefaultStorageConfig()
	manager := NewManager(tmpFile, 100, false, config)

	records, err := manager.ReadRecords(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Should skip malformed line and read valid one
	if len(records) != 1 {
		t.Errorf("Expected 1 valid record, got %d", len(records))
	}

	if records[0].Name != "valid-process" {
		t.Errorf("Expected 'valid-process', got '%s'", records[0].Name)
	}
}
