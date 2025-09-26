package core

import (
	"testing"
	"time"
)

func TestBioToolsManagerBasic(t *testing.T) {
	// 创建简化的生物信息学工具管理器配置
	config := &BioToolsConfig{
		Enabled:           true,
		ToolPaths:         []string{},
		DefaultTimeout:    300,
		MaxInstances:      10,
		LogLevel:          "info",
		EnableMonitoring:  true,
	}

	// 创建简化的进程管理器配置
	processConfig := ProcessManagerConfig{
		Enabled:           true,
		DiscoveryInterval: 5 * time.Second,
		AutoDiscovery:     true,
		ProcessPatterns:   []string{},
		ExcludePatterns:   []string{},
		MaxProcesses:      100,
		EnableControl:     true,
	}

	// 创建进程管理器
	processManager := NewSimplifiedProcessManager(processConfig, nil)

	// 创建生物信息学工具管理器
	bioToolsManager := NewBioToolsManager(config, processManager)

	// 测试1: 获取可用工具
	t.Run("GetAvailableTools", func(t *testing.T) {
		tools := bioToolsManager.GetAvailableTools()
		
		// 验证返回的不是nil
		if tools == nil {
			t.Error("GetAvailableTools should not return nil")
		}
		
		t.Logf("Found %d bioinformatics tools", len(tools))
		
		// 验证每个工具都有基本信息
		for toolID, tool := range tools {
			if tool.Name == "" {
				t.Errorf("Tool %s should have a name", toolID)
			}
			if tool.Path == "" {
				t.Errorf("Tool %s should have a path", toolID)
			}
		}
	})

	// 测试2: 获取特定工具信息
	t.Run("GetToolInfo", func(t *testing.T) {
		// 先获取所有工具
		tools := bioToolsManager.GetAvailableTools()
		
		// 如果有工具，测试获取工具信息
		for toolID := range tools {
			info, err := bioToolsManager.GetToolInfo(toolID)
			if err != nil {
				t.Errorf("GetToolInfo failed for tool %s: %v", toolID, err)
				continue
			}
			
			if info == nil {
				t.Errorf("GetToolInfo returned nil for tool %s", toolID)
				continue
			}
			
			if info.Name == "" {
				t.Errorf("Tool info should have a name for tool %s", toolID)
			}
			
			t.Logf("Tool %s: %s v%s", toolID, info.Name, info.Version)
			break // 只测试第一个工具
		}
	})

	// 测试3: 获取不存在的工具
	t.Run("GetNonExistentTool", func(t *testing.T) {
		_, err := bioToolsManager.GetToolInfo("nonexistent_tool")
		if err == nil {
			t.Error("GetToolInfo should return error for non-existent tool")
		}
	})

	// 测试4: 获取活动实例
	t.Run("GetActiveInstances", func(t *testing.T) {
		instances := bioToolsManager.GetActiveInstances()
		
		if instances == nil {
			t.Error("GetActiveInstances should not return nil")
		}
		
		t.Logf("Active instances: %d", len(instances))
	})

	// 测试5: 按类别获取工具
	t.Run("GetToolsByCategory", func(t *testing.T) {
		categoryTools := bioToolsManager.GetToolsByCategory("Alignment")
		
		if categoryTools == nil {
			t.Error("GetToolsByCategory should not return nil")
		}
		
		t.Logf("Alignment tools: %d", len(categoryTools))
	})

	// 测试6: 搜索工具
	t.Run("SearchTools", func(t *testing.T) {
		searchResults := bioToolsManager.SearchTools("blast")
		
		if searchResults == nil {
			t.Error("SearchTools should not return nil")
		}
		
		t.Logf("Search results for 'blast': %d", len(searchResults))
	})

	// 测试7: 获取不存在的实例状态
	t.Run("GetNonExistentInstanceStatus", func(t *testing.T) {
		_, err := bioToolsManager.GetToolStatus("nonexistent_instance")
		if err == nil {
			t.Error("GetToolStatus should return error for non-existent instance")
		}
	})

	// 测试8: 停止不存在的实例
	t.Run("StopNonExistentInstance", func(t *testing.T) {
		err := bioToolsManager.StopTool("nonexistent_instance")
		if err == nil {
			t.Error("StopTool should return error for non-existent instance")
		}
	})

	// 测试9: 清理完成的实例
	t.Run("CleanupCompletedInstances", func(t *testing.T) {
		// 这应该不会产生错误
		bioToolsManager.CleanupCompletedInstances(1 * time.Hour)
	})

	t.Log("✅ BioToolsManager basic functionality tests completed")
}

func TestBioToolsManagerWithApp(t *testing.T) {
	// 测试简化的生物信息学工具管理器与App的集成
	config := GetDefaultConfig()
	
	// 确保生物信息学工具功能启用
	config.BioTools.Enabled = true
	
	app := NewApp("test_data.json", 1*time.Second, config)
	
	// 测试生物信息学工具管理器是否正确初始化
	if app.BioToolsManager == nil {
		t.Error("BioToolsManager should be initialized when enabled in config")
	}
	
	// 测试基本功能
	tools := app.BioToolsManager.GetAvailableTools()
	if tools == nil {
		t.Error("GetAvailableTools should not return nil")
	}
	
	t.Logf("Integration test: Found %d bioinformatics tools", len(tools))
	
	t.Log("✅ BioToolsManager integration test completed")
}

func TestBioToolsManagerPerformance(t *testing.T) {
	// 性能测试：验证简化的生物信息学工具管理器性能
	config := &BioToolsConfig{
		Enabled:           true,
		ToolPaths:         []string{},
		DefaultTimeout:    300,
		MaxInstances:      10,
		LogLevel:          "info",
		EnableMonitoring:  true,
	}

	processConfig := ProcessManagerConfig{
		Enabled:           true,
		DiscoveryInterval: 5 * time.Second,
		AutoDiscovery:     true,
		ProcessPatterns:   []string{},
		ExcludePatterns:   []string{},
		MaxProcesses:      100,
		EnableControl:     true,
	}

	processManager := NewSimplifiedProcessManager(processConfig, nil)
	bioToolsManager := NewBioToolsManager(config, processManager)

	// 测试多次调用的性能
	start := time.Now()
	for i := 0; i < 100; i++ {
		tools := bioToolsManager.GetAvailableTools()
		if tools == nil {
			t.Error("GetAvailableTools should not return nil")
		}
	}
	duration := time.Since(start)
	
	t.Logf("100 GetAvailableTools calls took: %v", duration)
	
	// 测试搜索性能
	start = time.Now()
	for i := 0; i < 50; i++ {
		results := bioToolsManager.SearchTools("alignment")
		if results == nil {
			t.Error("SearchTools should not return nil")
		}
	}
	duration = time.Since(start)
	
	t.Logf("50 SearchTools calls took: %v", duration)
	
	t.Log("✅ BioToolsManager performance test completed")
}