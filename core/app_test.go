package core

import (
	"testing"
	"time"
)

func TestNewApp(t *testing.T) {
	config := Config{
		ProcessControl: ProcessControlConfig{
			Enabled: true,
		},
		ProcessDiscovery: ProcessDiscoveryConfig{
			Enabled:           true,
			DiscoveryInterval: 15 * time.Second,
		},
		HealthCheck: HealthCheckConfig{
			Enabled:       true,
			CheckInterval: 30 * time.Second,
		},
	}

	app := NewApp("test_data.json", 10*time.Second, config)
	if app == nil {
		t.Fatal("NewApp should not return nil")
	}

	if app.DataFile != "test_data.json" {
		t.Errorf("Expected DataFile to be 'test_data.json', got '%s'", app.DataFile)
	}

	if app.Interval != 10*time.Second {
		t.Errorf("Expected Interval to be 10s, got %v", app.Interval)
	}
}

func TestAppInitialization(t *testing.T) {
	config := Config{
		ProcessControl: ProcessControlConfig{
			Enabled:       true,
			CheckInterval: 10 * time.Second,
		},
		ProcessDiscovery: ProcessDiscoveryConfig{
			Enabled:           true,
			DiscoveryInterval: 15 * time.Second,
		},
		HealthCheck: HealthCheckConfig{
			Enabled:       true,
			CheckInterval: 30 * time.Second,
		},
		Monitoring: MonitoringConfig{
			Enabled:              true,
			Interval:             10 * time.Second,
			HealthCheckInterval:  30 * time.Second,
			MaxMonitoredProcesses: 100,
			PerformanceHistorySize: 1000,
		},
	}

	app := NewApp("test_data.json", 10*time.Second, config)
	if err := app.Initialize(); err != nil {
		t.Fatalf("App.Initialize should not error: %v", err)
	}

	// Verify unified modules are initialized
	if app.UnifiedMonitor == nil {
		t.Error("UnifiedMonitor should be initialized")
	}

	if app.SimplifiedProcessManager == nil {
		t.Error("SimplifiedProcessManager should be initialized")
	}

	if app.UnifiedHealthChecker == nil {
		t.Error("UnifiedHealthChecker should be initialized")
	}
}

func TestAppUnifiedMonitorInterface(t *testing.T) {
	config := Config{
		Monitoring: MonitoringConfig{
			Enabled:              true,
			Interval:             10 * time.Second,
			HealthCheckInterval:  30 * time.Second,
			MaxMonitoredProcesses: 100,
			PerformanceHistorySize: 1000,
		},
	}

	app := NewApp("test_data.json", 10*time.Second, config)
	if err := app.Initialize(); err != nil {
		t.Fatalf("Failed to initialize app: %v", err)
	}

	monitor := app.UnifiedMonitor
	if monitor == nil {
		t.Fatal("UnifiedMonitor should not be nil")
	}

	// Test interface compliance
	var pm ProcessMonitor = monitor
	if pm == nil {
		t.Error("UnifiedMonitor should implement ProcessMonitor interface")
	}

	// Test ProcessMonitor methods
	if err := pm.Start(); err != nil {
		t.Errorf("ProcessMonitor.Start() should not error: %v", err)
	}
	pm.Stop()
	if stats := pm.GetStats(); len(stats) == 0 {
		t.Log("ProcessMonitor.GetStats() returned empty stats (expected for new monitor)")
	}
}

func TestAppSimplifiedProcessManagerInterface(t *testing.T) {
	config := Config{
		ProcessControl: ProcessControlConfig{
			Enabled:       true,
			CheckInterval: 10 * time.Second,
		},
	}

	app := NewApp("test_data.json", 10*time.Second, config)
	if err := app.Initialize(); err != nil {
		t.Fatalf("Failed to initialize app: %v", err)
	}

	spm := app.SimplifiedProcessManager
	if spm == nil {
		t.Fatal("SimplifiedProcessManager should not be nil")
	}

	// Test interface compliance
	var pm ProcessMonitor = spm
	if pm == nil {
		t.Error("SimplifiedProcessManager should implement ProcessMonitor interface")
	}

	// Test ProcessMonitor methods
	if err := pm.Start(); err != nil {
		t.Errorf("ProcessMonitor.Start() should not error: %v", err)
	}
	pm.Stop()
	if stats := pm.GetStats(); len(stats) == 0 {
		t.Log("ProcessMonitor.GetStats() returned empty stats (expected for new manager)")
	}
}

func TestAppUnifiedHealthCheckerInterface(t *testing.T) {
	config := Config{
		HealthCheck: HealthCheckConfig{
			Enabled:       true,
			CheckInterval: 30 * time.Second,
		},
	}

	app := NewApp("test_data.json", 10*time.Second, config)
	if err := app.Initialize(); err != nil {
		t.Fatalf("Failed to initialize app: %v", err)
	}

	uhc := app.UnifiedHealthChecker
	if uhc == nil {
		t.Fatal("UnifiedHealthChecker should not be nil")
	}

	// Test interface compliance
	var hc HealthChecker = uhc
	if hc == nil {
		t.Error("UnifiedHealthChecker should implement HealthChecker interface")
	}

	// Test HealthChecker methods
	if health := hc.CheckHealth(); !health.IsHealthy {
		t.Log("HealthChecker.CheckHealth() returned unhealthy status")
	}
}

func TestResourceUsageStruct(t *testing.T) {
	usage := ResourceUsage{
		CPUUsed:        75.5,
		MemoryUsedMB:   1024,
		DiskReadMB:     512,
		DiskWriteMB:    256,
		NetworkInKB:    128,
		NetworkOutKB:   64,
		PerformanceScore: 85.0,
	}

	if usage.CPUUsed != 75.5 {
		t.Errorf("Expected CPUUsed to be 75.5, got %f", usage.CPUUsed)
	}

	if usage.MemoryUsedMB != 1024 {
		t.Errorf("Expected MemoryUsedMB to be 1024, got %d", usage.MemoryUsedMB)
	}

	if usage.PerformanceScore != 85.0 {
		t.Errorf("Expected PerformanceScore to be 85.0, got %f", usage.PerformanceScore)
	}
}

func TestHealthStatusStruct(t *testing.T) {
	status := HealthStatus{
		IsHealthy: true,
		Status:    "healthy",
		Score:     95.0,
		Issues:    []string{},
	}

	if !status.IsHealthy {
		t.Error("Expected IsHealthy to be true")
	}

	if status.Status != "healthy" {
		t.Errorf("Expected Status to be 'healthy', got '%s'", status.Status)
	}

	if status.Score != 95.0 {
		t.Errorf("Expected Score to be 95.0, got %f", status.Score)
	}

	if len(status.Issues) != 0 {
		t.Errorf("Expected Issues to be empty, got %d issues", len(status.Issues))
	}
}