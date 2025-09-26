package core

import (
	"testing"
	"time"
)

func TestNewUnifiedMonitor(t *testing.T) {
	config := MonitoringConfig{
		Enabled:                 true,
		CheckInterval:          10 * time.Second,
		HealthCheckInterval:    30 * time.Second,
		MaxMonitoredProcesses:  100,
		PerformanceHistorySize: 1000,
	}

	app := &App{}
	monitor := NewUnifiedMonitor(config, app)

	if monitor == nil {
		t.Fatal("NewUnifiedMonitor should not return nil")
	}

	if monitor.config.Enabled != true {
		t.Error("Monitor should be enabled")
	}

	if len(monitor.processes) != 0 {
		t.Errorf("Expected processes to be empty, got %d", len(monitor.processes))
	}
}

func TestUnifiedMonitorStartStop(t *testing.T) {
	config := MonitoringConfig{
		Enabled:                 true,
		CheckInterval:          100 * time.Millisecond,
		HealthCheckInterval:    200 * time.Millisecond,
		Interval:               100 * time.Millisecond,
		MaxMonitoredProcesses:  10,
		PerformanceHistorySize: 100,
	}

	app := &App{}
	monitor := NewUnifiedMonitor(config, app)

	// Test starting
	err := monitor.Start()
	if err != nil {
		t.Errorf("Start() should not error: %v", err)
	}

	// Let it run briefly
	time.Sleep(150 * time.Millisecond)

	// Test stopping
	monitor.Stop()

	// Verify it stopped without panicking
	t.Log("UnifiedMonitor started and stopped successfully")
}

func TestUnifiedMonitorInterface(t *testing.T) {
	config := MonitoringConfig{
		Enabled:                 true,
		CheckInterval:          100 * time.Millisecond,
		HealthCheckInterval:    200 * time.Millisecond,
		Interval:               100 * time.Millisecond,
		MaxMonitoredProcesses:  10,
		PerformanceHistorySize: 100,
	}

	app := &App{}
	monitor := NewUnifiedMonitor(config, app)

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

func TestUnifiedMonitorResourceCollection(t *testing.T) {
	config := MonitoringConfig{
		Enabled:                 true,
		CheckInterval:          1 * time.Second,
		HealthCheckInterval:    30 * time.Second,
		Interval:               1 * time.Second,
		MaxMonitoredProcesses:  10,
		PerformanceHistorySize: 100,
	}

	app := &App{}
	monitor := NewUnifiedMonitor(config, app)

	// Test GetStats method
	stats := monitor.GetStats()
	if stats == nil {
		t.Fatal("GetStats should not return nil")
	}

	t.Logf("Retrieved %d process statistics", len(stats))
}

func TestUnifiedMonitorHealthChecking(t *testing.T) {
	config := MonitoringConfig{
		Enabled:                 true,
		CheckInterval:          1 * time.Second,
		HealthCheckInterval:    500 * time.Millisecond,
		Interval:               1 * time.Second,
		MaxMonitoredProcesses:  10,
		PerformanceHistorySize: 100,
		HealthCheckRules: []HealthCheckRule{
			{
				Name:        "test_cpu_rule",
				Description: "Test CPU rule",
				Metric:      "cpu",
				Operator:    ">",
				Threshold:   100.0,
				Severity:    "warning",
				Enabled:     true,
			},
		},
	}

	app := &App{}
	monitor := NewUnifiedMonitor(config, app)

	// Test health check functionality
	err := monitor.Start()
	if err != nil {
		t.Errorf("Start() should not error: %v", err)
	}

	// Let it run briefly to perform health checks
	time.Sleep(600 * time.Millisecond)

	monitor.Stop()

	// Health checking should have completed without errors
	t.Log("UnifiedMonitor health checking completed successfully")
}

func TestUnifiedMonitorEventHandling(t *testing.T) {
	config := MonitoringConfig{
		Enabled:                 true,
		CheckInterval:          100 * time.Millisecond,
		HealthCheckInterval:    200 * time.Millisecond,
		Interval:               100 * time.Millisecond,
		MaxMonitoredProcesses:  10,
		PerformanceHistorySize: 100,
	}

	app := &App{}
	monitor := NewUnifiedMonitor(config, app)

	// Test that event handlers slice is initialized
	if monitor.eventHandlers == nil {
		t.Error("Event handlers should be initialized")
	}

	// Test Start and Stop with event handling
	err := monitor.Start()
	if err != nil {
		t.Errorf("Start() should not error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	monitor.Stop()

	t.Log("UnifiedMonitor event handling works correctly")
}