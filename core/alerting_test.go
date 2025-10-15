package core

import (
	"testing"
	"time"
)

// Helper to create a basic alert manager for testing
func newTestAlertManager() *AlertManager {
	config := AlertConfig{
		Enabled:          true,
		Rules:            []AlertRule{},
		SuppressDuration: 30,
	}
	return NewAlertManager(config, NotifiersConfig{})
}

// TestAlertRule_ThresholdExceeded tests basic threshold triggering
func TestAlertRule_ThresholdExceeded(t *testing.T) {
	am := newTestAlertManager()
	
	rule := AlertRule{
		Name:      "High CPU",
		Metric:    "cpu_percent",
		Threshold: 80.0,
		Duration:  0, // Trigger immediately
		Enabled:   true,
	}

	records := []ResourceRecord{
		{Name: "test-process", CPUPercent: 85.0, Timestamp: time.Now()},
	}

	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	
	if value <= rule.Threshold {
		t.Errorf("Expected metric value %.2f to exceed threshold %.2f", value, rule.Threshold)
	}
}

// TestAlertRule_ThresholdNotExceeded tests threshold not triggered
func TestAlertRule_ThresholdNotExceeded(t *testing.T) {
	am := newTestAlertManager()
	
	rule := AlertRule{
		Name:      "High CPU",
		Metric:    "cpu_percent",
		Threshold: 80.0,
		Duration:  0,
		Enabled:   true,
	}

	records := []ResourceRecord{
		{Name: "test-process", CPUPercent: 50.0, Timestamp: time.Now()},
	}

	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	
	if value > rule.Threshold {
		t.Errorf("Expected metric value %.2f to be below threshold %.2f", value, rule.Threshold)
	}
}

// TestAggregation_Max tests max aggregation
func TestAggregation_Max(t *testing.T) {
	rule := AlertRule{
		Name:        "High CPU",
		Metric:      "cpu_percent",
		Threshold:   80.0,
		Aggregation: "max",
		Enabled:     true,
	}

	records := []ResourceRecord{
		{Name: "process1", CPUPercent: 50.0, Timestamp: time.Now()},
		{Name: "process2", CPUPercent: 95.0, Timestamp: time.Now()}, // Max
		{Name: "process3", CPUPercent: 30.0, Timestamp: time.Now()},
	}

	am := newTestAlertManager()
	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	expected := 95.0
	
	if value != expected {
		t.Errorf("Expected max CPU %.2f, got %.2f", expected, value)
	}
}

// TestAggregation_Avg tests average aggregation
func TestAggregation_Avg(t *testing.T) {
	rule := AlertRule{
		Name:        "High CPU",
		Metric:      "cpu_percent",
		Threshold:   80.0,
		Aggregation: "avg",
		Enabled:     true,
	}

	records := []ResourceRecord{
		{Name: "process1", CPUPercent: 60.0, Timestamp: time.Now()},
		{Name: "process2", CPUPercent: 80.0, Timestamp: time.Now()},
		{Name: "process3", CPUPercent: 70.0, Timestamp: time.Now()},
	}

	am := newTestAlertManager()
	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	expected := 70.0 // (60 + 80 + 70) / 3
	
	if value != expected {
		t.Errorf("Expected avg CPU %.2f, got %.2f", expected, value)
	}
}

// TestAggregation_Sum tests sum aggregation
func TestAggregation_Sum(t *testing.T) {
	rule := AlertRule{
		Name:        "High Memory",
		Metric:      "memory_mb",
		Threshold:   1000.0,
		Aggregation: "sum",
		Enabled:     true,
	}

	records := []ResourceRecord{
		{Name: "process1", MemoryMB: 300.0, Timestamp: time.Now()},
		{Name: "process2", MemoryMB: 400.0, Timestamp: time.Now()},
		{Name: "process3", MemoryMB: 500.0, Timestamp: time.Now()},
	}

	am := newTestAlertManager()
	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	expected := 1200.0 // 300 + 400 + 500
	
	if value != expected {
		t.Errorf("Expected sum memory %.2f MB, got %.2f MB", expected, value)
	}
}

// TestSystemCPUPercent tests system-level CPU percentage calculation
func TestSystemCPUPercent(t *testing.T) {
	// Mock system with 4 cores
	originalCores := SystemCPUCores()
	t.Logf("System has %d CPU cores", originalCores)

	rule := AlertRule{
		Name:        "System CPU High",
		Metric:      "system_cpu_percent",
		Threshold:   50.0,
		Aggregation: "sum", // Required for system metrics
		Enabled:     true,
	}

	// Simulate 2 processes each using 100% of one core
	// Total: 200% / 4 cores = 50%
	records := []ResourceRecord{
		{Name: "process1", CPUPercent: 100.0, Timestamp: time.Now()},
		{Name: "process2", CPUPercent: 100.0, Timestamp: time.Now()},
	}

	am := newTestAlertManager()
	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	
	// On a 4-core system, 200% total = 50% system utilization
	// On other systems, the value will be different
	expectedRange := 200.0 / float64(originalCores)
	
	if value < 0 || value > 100 {
		t.Errorf("System CPU percent should be between 0-100%%, got %.2f%%", value)
	}
	
	t.Logf("System CPU utilization: %.2f%% (expected ~%.2f%% on %d cores)", 
		value, expectedRange, originalCores)
}

// TestSystemMemoryPercent tests system-level memory percentage calculation
func TestSystemMemoryPercent(t *testing.T) {
	totalMemMB := SystemMemoryMB()
	t.Logf("System has %.2f MB total memory", totalMemMB)

	rule := AlertRule{
		Name:        "System Memory High",
		Metric:      "system_memory_percent",
		Threshold:   50.0,
		Aggregation: "sum", // Required for system metrics
		Enabled:     true,
	}

	// Simulate processes using 10% of total memory
	usedMemoryMB := totalMemMB * 0.1
	records := []ResourceRecord{
		{Name: "process1", MemoryMB: usedMemoryMB / 2, Timestamp: time.Now()},
		{Name: "process2", MemoryMB: usedMemoryMB / 2, Timestamp: time.Now()},
	}

	am := newTestAlertManager()
	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	expected := 10.0 // Should be ~10%
	
	if value < 0 || value > 100 {
		t.Errorf("System memory percent should be between 0-100%%, got %.2f%%", value)
	}
	
	// Allow 1% tolerance due to floating point calculations
	if value < expected-1 || value > expected+1 {
		t.Errorf("Expected system memory ~%.2f%%, got %.2f%%", expected, value)
	}
	
	t.Logf("System memory utilization: %.2f%% (expected ~%.2f%%)", value, expected)
}

// TestProcessFilter tests process-specific alert rules
func TestProcessFilter(t *testing.T) {
	rule := AlertRule{
		Name:      "Chrome High CPU",
		Metric:    "cpu_percent",
		Threshold: 80.0,
		Process:   "chrome", // Filter by process name
		Enabled:   true,
	}

	records := []ResourceRecord{
		{Name: "chrome", CPUPercent: 90.0, Timestamp: time.Now()},
		{Name: "firefox", CPUPercent: 95.0, Timestamp: time.Now()}, // Should be filtered out
	}

	am := newTestAlertManager()
	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	
	// Should only consider chrome process
	if value != 90.0 {
		t.Errorf("Expected chrome CPU 90%%, got %.2f%%", value)
	}
}

// TestEmptyRecords tests behavior with no records
func TestEmptyRecords(t *testing.T) {
	rule := AlertRule{
		Name:      "Test Rule",
		Metric:    "cpu_percent",
		Threshold: 80.0,
		Enabled:   true,
	}

	records := []ResourceRecord{} // Empty

	am := newTestAlertManager()
	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	
	if value != 0 {
		t.Errorf("Expected 0 for empty records, got %.2f", value)
	}
}

// TestAlertManager_Initialization tests alert manager creation
func TestAlertManager_Initialization(t *testing.T) {
	config := AlertConfig{
		Enabled: true,
		Rules: []AlertRule{
			{Name: "Test Rule 1", Metric: "cpu_percent", Threshold: 80, Enabled: true},
			{Name: "Test Rule 2", Metric: "memory_mb", Threshold: 1000, Enabled: true},
		},
		SuppressDuration: 30,
	}

	notifiers := NotifiersConfig{}

	am := NewAlertManager(config, notifiers)

	if am == nil {
		t.Fatal("AlertManager should not be nil")
	}

	if len(am.rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(am.rules))
	}

	if am.suppressDuration != 30*time.Minute {
		t.Errorf("Expected suppress duration 30m, got %v", am.suppressDuration)
	}
}

// TestAlertManager_Evaluate tests alert evaluation with mock records
func TestAlertManager_Evaluate(t *testing.T) {
	config := AlertConfig{
		Enabled: true,
		Rules: []AlertRule{
			{
				Name:      "High CPU Alert",
				Metric:    "cpu_percent",
				Threshold: 80.0,
				Duration:  0,
				Enabled:   true,
			},
		},
		SuppressDuration: 30,
	}

	am := NewAlertManager(config, NotifiersConfig{})

	// First evaluation - threshold exceeded
	records1 := []ResourceRecord{
		{Name: "test-process", CPUPercent: 90.0, Timestamp: time.Now()},
	}

	am.Evaluate(records1)

	// Check that alert state was created
	am.mu.RLock()
	state1, exists1 := am.states["High CPU Alert"]
	am.mu.RUnlock()

	if !exists1 {
		t.Error("Expected alert state to be created after threshold exceeded")
	}
	
	if state1 != nil && state1.Count == 0 {
		t.Error("Expected alert count to be > 0")
	}

	// Second evaluation - threshold not exceeded (should clear alert)
	records2 := []ResourceRecord{
		{Name: "test-process", CPUPercent: 50.0, Timestamp: time.Now()},
	}

	am.Evaluate(records2)

	// State should be cleared when threshold not exceeded
	am.mu.RLock()
	_, exists2 := am.states["High CPU Alert"]
	am.mu.RUnlock()

	if exists2 {
		t.Error("Expected alert state to be cleared when threshold not exceeded")
	}
}

// TestMemoryMetric tests memory-based alerting
func TestMemoryMetric(t *testing.T) {
	rule := AlertRule{
		Name:      "High Memory",
		Metric:    "memory_mb",
		Threshold: 1000.0,
		Enabled:   true,
	}

	records := []ResourceRecord{
		{Name: "memory-hog", MemoryMB: 1500.0, Timestamp: time.Now()},
	}

	am := newTestAlertManager()
	value := am.getMetricValue(records, rule.Metric, rule.Process, rule.Aggregation)
	
	if value <= rule.Threshold {
		t.Errorf("Expected memory value %.2f to exceed threshold %.2f", value, rule.Threshold)
	}
}

// TestDisabledRule tests that disabled rules are skipped
func TestDisabledRule(t *testing.T) {
	config := AlertConfig{
		Enabled: true,
		Rules: []AlertRule{
			{
				Name:      "Disabled Rule",
				Metric:    "cpu_percent",
				Threshold: 1.0, // Very low threshold
				Enabled:   false, // Disabled
			},
		},
	}

	am := NewAlertManager(config, NotifiersConfig{})

	records := []ResourceRecord{
		{Name: "test", CPUPercent: 100.0, Timestamp: time.Now()},
	}

	am.Evaluate(records)

	// Should not create any alert state for disabled rule
	am.mu.RLock()
	stateCount := len(am.states)
	am.mu.RUnlock()

	if stateCount > 0 {
		t.Errorf("Expected no alert states for disabled rule, got %d", stateCount)
	}
}
