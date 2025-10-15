package unit

import (
	"github.com/yourusername/process-tracker/core"
	"testing"
	"time"
)

func TestNewApp(t *testing.T) {
	config := core.GetDefaultConfig()
	app := core.NewApp("test.db", time.Second, config)
	if app == nil {
		t.Error("Failed to create app")
	}
}

func TestAppInitialization(t *testing.T) {
	config := core.GetDefaultConfig()
	app := core.NewApp("test.db", time.Second, config)
	if app.DataFile != "test.db" {
		t.Errorf("Expected data file 'test.db', got '%s'", app.DataFile)
	}
	if app.Interval != time.Second {
		t.Errorf("Expected interval 1s, got %v", app.Interval)
	}
	// Verify Docker monitoring is enabled by default
	if !config.Docker.Enabled {
		t.Error("Docker monitoring should be enabled by default")
	}
}

func TestAppInterface(t *testing.T) {
	config := core.GetDefaultConfig()
	var app interface{} = core.NewApp("test.db", time.Second, config)
	if app == nil {
		t.Error("App should not be nil")
	}
	// Test that it's the correct type
	if _, ok := app.(*core.App); !ok {
		t.Error("App should be of type *core.App")
	}
}
