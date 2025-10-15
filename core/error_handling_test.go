package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestReadOnlyDirectory tests handling of read-only directory
func TestReadOnlyDirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	tmpDir := filepath.Join(os.TempDir(), "readonly-test")
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Make directory read-only
	err = os.Chmod(tmpDir, 0444)
	if err != nil {
		t.Fatalf("Failed to set read-only permissions: %v", err)
	}
	defer os.Chmod(tmpDir, 0755) // Restore permissions for cleanup

	tmpFile := filepath.Join(tmpDir, "test.log")
	config := GetDefaultStorageConfig()
	manager := NewManager(tmpFile, 10, false, config)

	// Should fail to initialize due to permission error
	err = manager.Initialize()
	if err == nil {
		t.Error("Expected error when initializing in read-only directory")
	}
}

// TestInvalidDataFile tests handling of invalid file path
func TestInvalidDataFile(t *testing.T) {
	// Use a path that definitely doesn't exist and can't be created
	invalidPath := "/nonexistent/path/that/does/not/exist/test.log"
	
	config := GetDefaultStorageConfig()
	manager := NewManager(invalidPath, 10, false, config)

	err := manager.Initialize()
	if err == nil {
		t.Error("Expected error when using invalid file path")
	}
}

// TestCorruptedDataRecovery tests recovery from corrupted data
func TestCorruptedDataRecovery(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-corrupted.log")
	defer os.Remove(tmpFile)

	// Create file with mixed valid and corrupted data
	corruptedData := `
invalid line
another,invalid,line,with,few,fields
1729000000,valid-process-1,75.50,1.049,1024.00,0.32,5,10.00,5.00,100.00,200.00,true,/usr/bin/test,/home/user,development,12345,1729000000000,123.45
corrupted,,,,,,,,,,,,
1729000001,valid-process-2,50.00,0.694,512.00,0.16,3,5.00,2.00,50.00,100.00,false,/usr/bin/app,/home/user,system,67890,1729000001000,67.89
`

	err := os.WriteFile(tmpFile, []byte(corruptedData), 0644)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	config := GetDefaultStorageConfig()
	manager := NewManager(tmpFile, 100, false, config)

	records, err := manager.ReadRecords(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file with corrupted data: %v", err)
	}

	// Should successfully read only the 2 valid records
	if len(records) != 2 {
		t.Errorf("Expected 2 valid records, got %d", len(records))
	}

	if records[0].Name != "valid-process-1" {
		t.Errorf("Expected first record name 'valid-process-1', got '%s'", records[0].Name)
	}

	if records[1].Name != "valid-process-2" {
		t.Errorf("Expected second record name 'valid-process-2', got '%s'", records[1].Name)
	}
}

// TestEmptyRecordsArray tests handling of empty array
func TestEmptyRecordsArray(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-empty-array.log")
	defer os.Remove(tmpFile)

	config := GetDefaultStorageConfig()
	manager := NewManager(tmpFile, 10, false, config)

	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	defer manager.Close()

	// Save empty array - should not cause errors
	err = manager.SaveRecords([]ResourceRecord{})
	if err != nil {
		t.Errorf("Should handle empty array gracefully: %v", err)
	}
}

// TestConcurrentAccess tests thread-safe buffer access
func TestConcurrentAccess(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-concurrent.log")
	defer os.Remove(tmpFile)

	config := GetDefaultStorageConfig()
	manager := NewManager(tmpFile, 50, false, config)

	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	defer manager.Close()

	// Spawn multiple goroutines writing concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				record := ResourceRecord{
					Name:       "concurrent-process",
					Timestamp:  time.Now(),
					CPUPercent: float64(id*10 + j),
					MemoryMB:   float64(id * 100),
				}
				manager.SaveRecord(record)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// No assertion needed - test passes if no data races occur
	t.Log("Concurrent access test completed without data races")
}

// TestSystemMetrics_ZeroValues tests handling of zero system values
func TestSystemMetrics_ZeroValues(t *testing.T) {
	// This test verifies graceful handling when system metrics are unavailable
	am := newTestAlertManager()

	rule := AlertRule{
		Name:        "System Metrics Test",
		Metric:      "system_cpu_percent",
		Threshold:   100.0,
		Aggregation: "sum",
		Enabled:     true,
	}

	// Empty records should return 0, not crash
	value := am.getMetricValue([]ResourceRecord{}, rule.Metric, "", rule.Aggregation)
	if value != 0 {
		t.Errorf("Expected 0 for empty records, got %.2f", value)
	}
}

// TestAlertManager_NoNotifiers tests alert manager with no configured notifiers
func TestAlertManager_NoNotifiers(t *testing.T) {
	config := AlertConfig{
		Enabled: true,
		Rules: []AlertRule{
			{
				Name:      "Test Alert",
				Metric:    "cpu_percent",
				Threshold: 50.0,
				Channels:  []string{"nonexistent-channel"},
				Enabled:   true,
			},
		},
	}

	am := NewAlertManager(config, NotifiersConfig{})

	records := []ResourceRecord{
		{Name: "test", CPUPercent: 75.0, Timestamp: time.Now()},
	}

	// Should not panic when notifier doesn't exist
	am.Evaluate(records)

	// Test passed if no panic occurred
	t.Log("Alert evaluation completed without panic despite missing notifier")
}

// TestLargeBufferFlush tests flushing a large buffer
func TestLargeBufferFlush(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-large-buffer.log")
	defer os.Remove(tmpFile)

	config := GetDefaultStorageConfig()
	largeBufferSize := 1000
	manager := NewManager(tmpFile, largeBufferSize, false, config)

	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	defer manager.Close()

	// Create many records
	now := time.Now()
	for i := 0; i < largeBufferSize; i++ {
		record := ResourceRecord{
			Name:       "bulk-process",
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i % 100),
			MemoryMB:   float64(i),
		}
		err = manager.SaveRecord(record)
		if err != nil {
			t.Fatalf("Failed to save record %d: %v", i, err)
		}
	}

	// Force flush by closing
	err = manager.Close()
	if err != nil {
		t.Errorf("Failed to flush large buffer: %v", err)
	}

	// Verify all records were written
	readManager := NewManager(tmpFile, 100, false, config)
	records, err := readManager.ReadRecords(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read records: %v", err)
	}

	if len(records) != largeBufferSize {
		t.Errorf("Expected %d records, got %d", largeBufferSize, len(records))
	}
}
