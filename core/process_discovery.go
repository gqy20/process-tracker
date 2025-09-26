package core

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"github.com/shirou/gopsutil/v3/process"
)

// ProcessDiscovery 自动发现和管理系统中的进程
type ProcessDiscovery struct {
	app            *App
	config         ProcessDiscoveryConfig
	managedGroups  map[string]*ProcessGroup
	discoveredProcs map[int32]*DiscoveredProcess
	mutex          sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	events         chan ProcessDiscoveryEvent
}

// ProcessDiscoveryConfig 配置进程发现
type ProcessDiscoveryConfig struct {
	Enabled           bool          `yaml:"enabled"`
	DiscoveryInterval time.Duration `yaml:"discovery_interval"`
	AutoManage        bool          `yaml:"auto_manage"`
	BioToolsOnly      bool          `yaml:"bio_tools_only"`
	ProcessPatterns   []string      `yaml:"process_patterns"`
	ExcludePatterns   []string      `yaml:"exclude_patterns"`
	MaxProcesses      int           `yaml:"max_processes"`
	CPUThreshold      float64       `yaml:"cpu_threshold"`
	MemoryThresholdMB int64         `yaml:"memory_threshold_mb"`
}

// ProcessGroup 进程组定义
type ProcessGroup struct {
	Name        string   `yaml:"name"`
	Pattern     string   `yaml:"pattern"`
	Description string   `yaml:"description"`
	Processes   []int32  `yaml:"processes"`
	QuotaName   string   `yaml:"quota_name"`
	AutoManage  bool     `yaml:"auto_manage"`
	Tags        []string `yaml:"tags"`
}

// DiscoveredProcess 发现的进程信息
type DiscoveredProcess struct {
	PID         int32             `json:"pid"`
	Name        string            `json:"name"`
	Cmdline     string            `json:"cmdline"`
	GroupName   string            `json:"group_name"`
	Discovered  time.Time         `json:"discovered"`
	LastSeen    time.Time         `json:"last_seen"`
	CPUUsed     float64           `json:"cpu_used"`
	MemoryUsed  int64             `json:"memory_used_mb"`
	Status      ProcessStatus     `json:"status"`
	Tags        map[string]string `json:"tags"`
}

// ProcessDiscoveryEvent 进程发现事件
type ProcessDiscoveryEvent struct {
	Type      DiscoveryEventType `json:"type"`
	PID       int32             `json:"pid"`
	GroupName string            `json:"group_name"`
	Timestamp time.Time         `json:"timestamp"`
	Message   string            `json:"message"`
	Details   *DiscoveredProcess `json:"details,omitempty"`
}

// DiscoveryEventType 事件类型
type DiscoveryEventType string

const (
	EventProcessDiscovered  DiscoveryEventType = "process_discovered"
	EventProcessLost       DiscoveryEventType = "process_lost"
	EventProcessGroupAdded DiscoveryEventType = "process_group_added"
	EventProcessAutoManaged DiscoveryEventType = "process_auto_managed"
)

// BioinformaticsTools 预定义的生物信息学工具
var BioinformaticsTools = []ProcessGroup{
	{
		Name:        "blast",
		Pattern:     "blast(n|p|x)?",
		Description: "BLAST sequence alignment tools",
		Tags:        []string{"bioinformatics", "alignment", "sequence"},
		AutoManage:  true,
	},
	{
		Name:        "bwa",
		Pattern:     "bwa.*(mem|aln|sampe|samse)",
		Description: "Burrows-Wheeler Aligner",
		Tags:        []string{"bioinformatics", "alignment", "NGS"},
		AutoManage:  true,
	},
	{
		Name:        "samtools",
		Pattern:     "samtools.*(view|sort|index|merge)",
		Description: "SAM/BAM file manipulation tools",
		Tags:        []string{"bioinformatics", "NGS", "sam"},
		AutoManage:  true,
	},
	{
		Name:        "gatk",
		Pattern:     "gatk.*",
		Description: "Genome Analysis Toolkit",
		Tags:        []string{"bioinformatics", "variant", "NGS"},
		AutoManage:  true,
	},
	{
		Name:        "fastqc",
		Pattern:     "fastqc",
		Description: "FastQC quality control tool",
		Tags:        []string{"bioinformatics", "quality", "NGS"},
		AutoManage:  true,
	},
	{
		Name:        "trimmomatic",
		Pattern:     "trimmomatic",
		Description: "Trimmomatic read trimming tool",
		Tags:        []string{"bioinformatics", "trimming", "NGS"},
		AutoManage:  true,
	},
	{
		Name:        "hisat2",
		Pattern:     "hisat2.*",
		Description: "Hierarchical Indexing for Spliced Alignment",
		Tags:        []string{"bioinformatics", "alignment", "RNA-seq"},
		AutoManage:  true,
	},
	{
		Name:        "cufflinks",
		Pattern:     "cuff(links|compare|merge)",
		Description: "Cufflinks transcript assembly",
		Tags:        []string{"bioinformatics", "RNA-seq", "assembly"},
		AutoManage:  true,
	},
	{
		Name:        "rscript",
		Pattern:     "Rscript",
		Description: "R statistical computing",
		Tags:        []string{"bioinformatics", "statistics", "analysis"},
		AutoManage:  true,
	},
	{
		Name:        "python_bio",
		Pattern:     "python.*",
		Description: "Python bioinformatics scripts",
		Tags:        []string{"bioinformatics", "python", "analysis"},
		AutoManage:  true,
	},
	{
		Name:        "perl_bio",
		Pattern:     "perl.*",
		Description: "Perl bioinformatics scripts",
		Tags:        []string{"bioinformatics", "perl", "analysis"},
		AutoManage:  true,
	},
}

// NewProcessDiscovery 创建新的进程发现管理器
func NewProcessDiscovery(config ProcessDiscoveryConfig, app *App) *ProcessDiscovery {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ProcessDiscovery{
		app:            app,
		config:         config,
		managedGroups:  make(map[string]*ProcessGroup),
		discoveredProcs: make(map[int32]*DiscoveredProcess),
		events:         make(chan ProcessDiscoveryEvent, 100),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 开始进程发现
func (pd *ProcessDiscovery) Start() {
	// Initialize bioinformatics tools groups
	if pd.config.BioToolsOnly {
		for _, tool := range BioinformaticsTools {
			group := tool
			pd.managedGroups[group.Name] = &group
			
			// Emit event for bio tool group
			event := ProcessDiscoveryEvent{
				Type:      EventProcessGroupAdded,
				GroupName: group.Name,
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Added bioinformatics tool group: %s", group.Name),
			}
			pd.emitEvent(event)
			
			log.Printf("🧬 添加生物信息学工具组: %s - %s", group.Name, group.Description)
		}
	}
	
	// Start discovery loop
	go pd.discoverProcesses()
}

// Stop 停止进程发现
func (pd *ProcessDiscovery) Stop() {
	pd.cancel()
	close(pd.events)
}

// discoverProcesses 发现进程的主循环
func (pd *ProcessDiscovery) discoverProcesses() {
	ticker := time.NewTicker(pd.config.DiscoveryInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-pd.ctx.Done():
			return
		case <-ticker.C:
			pd.scanSystemProcesses()
		}
	}
}

// scanSystemProcesses 扫描系统进程
func (pd *ProcessDiscovery) scanSystemProcesses() {
	// Get all system processes
	processes, err := process.Processes()
	if err != nil {
		log.Printf("❌ 获取系统进程列表失败: %v", err)
		return
	}
	
	// Track current processes for cleanup
	currentPIDs := make(map[int32]bool)
	
	for _, p := range processes {
		pid := p.Pid
		
		// Skip system processes and self
		if pid <= 1 || pid == int32(pd.app.GetPID()) {
			continue
		}
		
		currentPIDs[pid] = true
		
		// Skip if already discovered and recently seen
		if existing, exists := pd.discoveredProcs[pid]; exists {
			if time.Since(existing.LastSeen) < pd.config.DiscoveryInterval*2 {
				existing.LastSeen = time.Now()
				continue
			}
		}
		
		// Check if process matches any pattern
		if group, matched := pd.matchProcess(p); matched {
			pd.addDiscoveredProcess(p, group)
		}
	}
	
	// Clean up lost processes
	pd.cleanupLostProcesses(currentPIDs)
}

// matchProcess 检查进程是否匹配任何模式
func (pd *ProcessDiscovery) matchProcess(p *process.Process) (*ProcessGroup, bool) {
	// Get process name
	name, err := p.Name()
	if err != nil {
		return nil, false
	}
	
	// Get command line for better matching
	cmdline, err := p.Cmdline()
	if err != nil {
		cmdline = name
	}
	
	// Check exclude patterns first
	for _, pattern := range pd.config.ExcludePatterns {
		if matched, _ := regexp.MatchString(pattern, name); matched {
			return nil, false
		}
		if matched, _ := regexp.MatchString(pattern, cmdline); matched {
			return nil, false
		}
	}
	
	// Check bio tools first if enabled
	if pd.config.BioToolsOnly {
		for _, group := range pd.managedGroups {
			if matched, _ := regexp.MatchString(group.Pattern, name); matched {
				return group, true
			}
			if matched, _ := regexp.MatchString(group.Pattern, cmdline); matched {
				return group, true
			}
		}
		return nil, false
	}
	
	// Check custom patterns
	for _, pattern := range pd.config.ProcessPatterns {
		if matched, _ := regexp.MatchString(pattern, name); matched {
			// Create ad-hoc group for custom pattern
			group := &ProcessGroup{
				Name:       fmt.Sprintf("custom_%s", pattern),
				Pattern:    pattern,
				AutoManage: pd.config.AutoManage,
				Tags:       []string{"custom"},
			}
			return group, true
		}
		if matched, _ := regexp.MatchString(pattern, cmdline); matched {
			group := &ProcessGroup{
				Name:       fmt.Sprintf("custom_%s", pattern),
				Pattern:    pattern,
				AutoManage: pd.config.AutoManage,
				Tags:       []string{"custom"},
			}
			return group, true
		}
	}
	
	// Check CPU/Memory thresholds for general processes
	if pd.config.CPUThreshold > 0 || pd.config.MemoryThresholdMB > 0 {
		if pd.checkResourceThresholds(p) {
			group := &ProcessGroup{
				Name:       "high_resource",
				Pattern:    ".*",
				AutoManage: pd.config.AutoManage,
				Tags:       []string{"high_resource"},
			}
			return group, true
		}
	}
	
	return nil, false
}

// checkResourceThresholds 检查进程是否超过资源阈值
func (pd *ProcessDiscovery) checkResourceThresholds(p *process.Process) bool {
	// Check CPU threshold
	if pd.config.CPUThreshold > 0 {
		if cpuPercent, err := p.CPUPercent(); err == nil {
			if cpuPercent > pd.config.CPUThreshold {
				return true
			}
		}
	}
	
	// Check Memory threshold
	if pd.config.MemoryThresholdMB > 0 {
		if memInfo, err := p.MemoryInfo(); err == nil {
			memoryMB := memInfo.RSS / 1024 / 1024
			if int64(memoryMB) > pd.config.MemoryThresholdMB {
				return true
			}
		}
	}
	
	return false
}

// addDiscoveredProcess 添加发现的进程
func (pd *ProcessDiscovery) addDiscoveredProcess(p *process.Process, group *ProcessGroup) {
	pid := p.Pid
	
	// Get process details
	name, _ := p.Name()
	cmdline, _ := p.Cmdline()
	
	// Create discovered process record
	discovered := &DiscoveredProcess{
		PID:        pid,
		Name:       name,
		Cmdline:    cmdline,
		GroupName:  group.Name,
		Discovered: time.Now(),
		LastSeen:   time.Now(),
		Status:     StatusRunning,
		Tags:       make(map[string]string),
	}
	
	// Add tags from group
	for _, tag := range group.Tags {
		discovered.Tags[tag] = "true"
	}
	
	pd.mutex.Lock()
	pd.discoveredProcs[pid] = discovered
	pd.mutex.Unlock()
	
	// Auto-add to quota if configured
	if group.AutoManage && group.QuotaName != "" && pd.app.QuotaManager != nil {
		if err := pd.app.AddProcessToQuota(group.QuotaName, pid); err == nil {
			log.Printf("✅ 自动将进程 %s (PID: %d) 添加到配额 %s", name, pid, group.QuotaName)
			
			// Emit auto-manage event
			event := ProcessDiscoveryEvent{
				Type:      EventProcessAutoManaged,
				PID:       pid,
				GroupName: group.Name,
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Auto-managed process %s added to quota %s", name, group.QuotaName),
				Details:   discovered,
			}
			pd.emitEvent(event)
		}
	}
	
	// Emit discovery event
	event := ProcessDiscoveryEvent{
		Type:      EventProcessDiscovered,
		PID:       pid,
		GroupName: group.Name,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Discovered process %s (PID: %d) in group %s", name, pid, group.Name),
		Details:   discovered,
	}
	pd.emitEvent(event)
	
	log.Printf("🔍 发现进程: %s (PID: %d) 组: %s", name, pid, group.Name)
}

// cleanupLostProcesses 清理丢失的进程
func (pd *ProcessDiscovery) cleanupLostProcesses(currentPIDs map[int32]bool) {
	pd.mutex.Lock()
	defer pd.mutex.Unlock()
	
	for pid, discovered := range pd.discoveredProcs {
		if !currentPIDs[pid] {
			// Remove from quota management
			if pd.app.QuotaManager != nil {
				group := pd.managedGroups[discovered.GroupName]
				if group != nil && group.QuotaName != "" {
					_ = pd.app.RemoveProcessFromQuota(group.QuotaName, pid)
				}
			}
			
			// Emit loss event
			event := ProcessDiscoveryEvent{
				Type:      EventProcessLost,
				PID:       pid,
				GroupName: discovered.GroupName,
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Lost process %s (PID: %d)", discovered.Name, pid),
				Details:   discovered,
			}
			pd.emitEvent(event)
			
			delete(pd.discoveredProcs, pid)
			log.Printf("👋 进程丢失: %s (PID: %d)", discovered.Name, pid)
		}
	}
}

// emitEvent 发送事件
func (pd *ProcessDiscovery) emitEvent(event ProcessDiscoveryEvent) {
	select {
	case pd.events <- event:
	default:
		log.Printf("⚠️  Process discovery event channel full, dropping event: %s", event.Message)
	}
}

// Events 返回事件通道
func (pd *ProcessDiscovery) Events() <-chan ProcessDiscoveryEvent {
	return pd.events
}

// GetDiscoveredProcesses 返回所有发现的进程
func (pd *ProcessDiscovery) GetDiscoveredProcesses() []*DiscoveredProcess {
	pd.mutex.RLock()
	defer pd.mutex.RUnlock()
	
	processes := make([]*DiscoveredProcess, 0, len(pd.discoveredProcs))
	for _, proc := range pd.discoveredProcs {
		processes = append(processes, proc)
	}
	
	// Sort by discovery time
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].Discovered.Before(processes[j].Discovered)
	})
	
	return processes
}

// GetProcessesByGroup 按组获取进程
func (pd *ProcessDiscovery) GetProcessesByGroup(groupName string) []*DiscoveredProcess {
	pd.mutex.RLock()
	defer pd.mutex.RUnlock()
	
	var processes []*DiscoveredProcess
	for _, proc := range pd.discoveredProcs {
		if proc.GroupName == groupName {
			processes = append(processes, proc)
		}
	}
	
	return processes
}

// GetProcessGroups 获取所有进程组
func (pd *ProcessDiscovery) GetProcessGroups() map[string]*ProcessGroup {
	pd.mutex.RLock()
	defer pd.mutex.RUnlock()
	
	groups := make(map[string]*ProcessGroup)
	for name, group := range pd.managedGroups {
		// Copy group with current processes
		groupCopy := *group
		groupCopy.Processes = []int32{}
		
		for _, proc := range pd.discoveredProcs {
			if proc.GroupName == name {
				groupCopy.Processes = append(groupCopy.Processes, proc.PID)
			}
		}
		
		groups[name] = &groupCopy
	}
	
	return groups
}

// AddCustomGroup 添加自定义进程组
func (pd *ProcessDiscovery) AddCustomGroup(name, pattern string, autoManage bool, quotaName string) error {
	pd.mutex.Lock()
	defer pd.mutex.Unlock()
	
	if _, exists := pd.managedGroups[name]; exists {
		return fmt.Errorf("group %s already exists", name)
	}
	
	group := &ProcessGroup{
		Name:       name,
		Pattern:    pattern,
		AutoManage: autoManage,
		QuotaName:  quotaName,
		Tags:       []string{"custom"},
	}
	
	pd.managedGroups[name] = group
	
	// Emit group added event
	event := ProcessDiscoveryEvent{
		Type:      EventProcessGroupAdded,
		GroupName: name,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Added custom process group: %s", name),
	}
	pd.emitEvent(event)
	
	log.Printf("➕ 添加自定义进程组: %s (模式: %s)", name, pattern)
	return nil
}

// RemoveCustomGroup 移除自定义进程组
func (pd *ProcessDiscovery) RemoveCustomGroup(name string) error {
	pd.mutex.Lock()
	defer pd.mutex.Unlock()
	
	group, exists := pd.managedGroups[name]
	if !exists {
		return fmt.Errorf("group %s not found", name)
	}
	
	// Don't remove bioinformatics tool groups
	if strings.HasPrefix(name, "blast") || strings.HasPrefix(name, "bwa") ||
		strings.HasPrefix(name, "samtools") || strings.HasPrefix(name, "gatk") {
		return fmt.Errorf("cannot remove built-in bioinformatics tool group: %s", name)
	}
	
	// Remove processes from quota if needed
	for _, proc := range pd.discoveredProcs {
		if proc.GroupName == name && group.QuotaName != "" && pd.app.QuotaManager != nil {
			_ = pd.app.RemoveProcessFromQuota(group.QuotaName, proc.PID)
		}
	}
	
	delete(pd.managedGroups, name)
	log.Printf("➖ 移除自定义进程组: %s", name)
	return nil
}

// GetDiscoveryStats 获取发现统计信息
func (pd *ProcessDiscovery) GetDiscoveryStats() DiscoveryStats {
	pd.mutex.RLock()
	defer pd.mutex.RUnlock()
	
	stats := DiscoveryStats{
		TotalDiscovered: len(pd.discoveredProcs),
		TotalGroups:     len(pd.managedGroups),
		GroupCounts:     make(map[string]int),
		AutoManaged:     0,
	}
	
	for _, proc := range pd.discoveredProcs {
		stats.GroupCounts[proc.GroupName]++
		group := pd.managedGroups[proc.GroupName]
		if group != nil && group.AutoManage && group.QuotaName != "" {
			stats.AutoManaged++
		}
	}
	
	return stats
}

// DiscoveryStats 发现统计信息
type DiscoveryStats struct {
	TotalDiscovered int            `json:"total_discovered"`
	TotalGroups     int            `json:"total_groups"`
	GroupCounts     map[string]int `json:"group_counts"`
	AutoManaged     int            `json:"auto_managed"`
}