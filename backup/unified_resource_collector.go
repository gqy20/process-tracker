package core

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// UnifiedResourceCollectorImpl 统一资源收集器实现
type UnifiedResourceCollectorImpl struct {
	config        ResourceCollectionConfig
	cache         map[int32]*cachedResourceUsage
	history       *ResourceHistoryManager
	stats         CollectionStats
	mutex         sync.RWMutex
	startTime     time.Time
}

// cachedResourceUsage 缓存的资源使用情况
type cachedResourceUsage struct {
	usage      *UnifiedResourceUsage
	expiration time.Time
}

// ResourceHistoryManager 资源历史管理器
type ResourceHistoryManager struct {
	enabled    bool
	retention  time.Duration
	records    map[int32][]*UnifiedResourceUsage
	mutex      sync.RWMutex
}

// NewUnifiedResourceCollector 创建统一资源收集器
func NewUnifiedResourceCollector(config ResourceCollectionConfig) *UnifiedResourceCollectorImpl {
	collector := &UnifiedResourceCollectorImpl{
		config:    config,
		cache:     make(map[int32]*cachedResourceUsage),
		startTime: time.Now(),
		stats: CollectionStats{
			LastCollection: time.Now(),
		},
	}

	// 初始化历史管理器
	if config.EnableHistory {
		collector.history = &ResourceHistoryManager{
			enabled:   true,
			retention: config.HistoryRetention,
			records:   make(map[int32][]*UnifiedResourceUsage),
		}
	}

	return collector
}

// CollectProcessResources 收集单个进程资源使用情况
func (urc *UnifiedResourceCollectorImpl) CollectProcessResources(pid int32) (*UnifiedResourceUsage, error) {
	start := time.Now()
	
	// 检查缓存
	if cached := urc.getFromCache(pid); cached != nil {
		urc.stats.Hits++
		return cached, nil
	}
	
	urc.stats.Misses++

	// 创建进程对象
	p, err := process.NewProcess(pid)
	if err != nil {
		urc.recordError(fmt.Sprintf("failed to create process %d: %v", pid, err))
		return nil, fmt.Errorf("process not found: %w", err)
	}

	// 收集资源使用情况
	usage, err := urc.collectFromProcess(p)
	if err != nil {
		urc.recordError(fmt.Sprintf("failed to collect resources for process %d: %v", pid, err))
		return nil, err
	}

	// 更新缓存
	urc.addToCache(pid, usage)

	// 添加到历史记录
	if urc.history != nil && urc.history.enabled {
		if err := urc.history.AddRecord(pid, usage); err != nil {
			log.Printf("warning: failed to add history record for process %d: %v", pid, err)
		}
	}

	// 更新统计
	urc.stats.SuccessfulCollections++
	urc.stats.TotalCollections++
	urc.updateCollectionStats(start)

	return usage, nil
}

// CollectSystemResources 收集系统资源使用情况
func (urc *UnifiedResourceCollectorImpl) CollectSystemResources() (*SystemResourceUsage, error) {
	start := time.Now()
	
	systemUsage := &SystemResourceUsage{
		Timestamp: time.Now(),
	}

	// 收集系统CPU信息
	if urc.config.EnableCPUMonitoring {
		if err := urc.collectSystemCPU(systemUsage); err != nil {
			urc.recordError(fmt.Sprintf("failed to collect system CPU: %v", err))
		}
	}

	// 收集系统内存信息
	if urc.config.EnableMemoryMonitoring {
		if err := urc.collectSystemMemory(systemUsage); err != nil {
			urc.recordError(fmt.Sprintf("failed to collect system memory: %v", err))
		}
	}

	// 收集系统磁盘信息
	if urc.config.EnableIOMonitoring {
		if err := urc.collectSystemDisk(systemUsage); err != nil {
			urc.recordError(fmt.Sprintf("failed to collect system disk: %v", err))
		}
	}

	// 收集系统网络信息
	if urc.config.EnableNetworkMonitoring {
		if err := urc.collectSystemNetwork(systemUsage); err != nil {
			urc.recordError(fmt.Sprintf("failed to collect system network: %v", err))
		}
	}

	// 获取进程数量
	if processes, err := process.Pids(); err == nil {
		systemUsage.Processes = int32(len(processes))
	}

	// 获取系统运行时间 - 简化实现
	if bootTime, err := host.BootTime(); err == nil {
		systemUsage.Uptime = time.Since(time.Unix(int64(bootTime), 0))
	}

	urc.stats.SuccessfulCollections++
	urc.stats.TotalCollections++
	urc.updateCollectionStats(start)

	return systemUsage, nil
}

// CollectMultipleProcesses 批量收集多个进程资源使用情况
func (urc *UnifiedResourceCollectorImpl) CollectMultipleProcesses(pids []int32) (map[int32]*UnifiedResourceUsage, error) {
	results := make(map[int32]*UnifiedResourceUsage)
	var errors []error

	for _, pid := range pids {
		usage, err := urc.CollectProcessResources(pid)
		if err != nil {
			errors = append(errors, fmt.Errorf("process %d: %w", pid, err))
			continue
		}
		results[pid] = usage
	}

	if len(errors) > 0 && len(results) == 0 {
		return nil, fmt.Errorf("all collections failed: %v", errors)
	}

	if len(errors) > 0 {
		log.Printf("warning: some resource collections failed: %v", errors)
	}

	return results, nil
}

// collectFromProcess 从进程对象收集资源信息
func (urc *UnifiedResourceCollectorImpl) collectFromProcess(p *process.Process) (*UnifiedResourceUsage, error) {
	usage := &UnifiedResourceUsage{
		PID:       p.Pid,
		Timestamp: time.Now(),
	}

	// 收集CPU信息
	if urc.config.EnableCPUMonitoring {
		if err := urc.collectCPU(p, usage); err != nil {
			return nil, fmt.Errorf("CPU collection failed: %w", err)
		}
	}

	// 收集内存信息
	if urc.config.EnableMemoryMonitoring {
		if err := urc.collectMemory(p, usage); err != nil {
			return nil, fmt.Errorf("memory collection failed: %w", err)
		}
	}

	// 收集磁盘I/O信息
	if urc.config.EnableIOMonitoring {
		if err := urc.collectDisk(p, usage); err != nil {
			return nil, fmt.Errorf("disk collection failed: %w", err)
		}
	}

	// 收集网络信息
	if urc.config.EnableNetworkMonitoring {
		if err := urc.collectNetwork(p, usage); err != nil {
			return nil, fmt.Errorf("network collection failed: %w", err)
		}
	}

	// 收集线程信息
	if urc.config.EnableThreadMonitoring {
		if err := urc.collectThreads(p, usage); err != nil {
			return nil, fmt.Errorf("thread collection failed: %w", err)
		}
	}

	// 计算性能分数
	urc.calculatePerformanceScore(usage)

	return usage, nil
}

// collectCPU 收集CPU信息
func (urc *UnifiedResourceCollectorImpl) collectCPU(p *process.Process, usage *UnifiedResourceUsage) error {
	cpuPercent, err := p.CPUPercent()
	if err != nil {
		return err
	}

	cpuTime, err := p.Times()
	if err != nil {
		return err
	}

	usage.CPU = CPUUsage{
		UsedPercent: cpuPercent,
		TimeUsed:    uint64(cpuTime.Total()),
	}

	return nil
}

// collectMemory 收集内存信息
func (urc *UnifiedResourceCollectorImpl) collectMemory(p *process.Process, usage *UnifiedResourceUsage) error {
	memInfo, err := p.MemoryInfo()
	if err != nil {
		return err
	}

	memPercent, err := p.MemoryPercent()
	if err != nil {
		return err
	}

	usage.Memory = MemoryUsage{
		UsedMB:      int64(memInfo.RSS / 1024 / 1024),
		UsedPercent: float64(memPercent),
		RSS:         memInfo.RSS,
		VMS:         memInfo.VMS,
	}

	return nil
}

// collectDisk 收集磁盘I/O信息
func (urc *UnifiedResourceCollectorImpl) collectDisk(p *process.Process, usage *UnifiedResourceUsage) error {
	ioCounters, err := p.IOCounters()
	if err != nil {
		return err
	}

	usage.Disk = DiskUsage{
		ReadMB:     int64(ioCounters.ReadBytes / 1024 / 1024),
		WriteMB:    int64(ioCounters.WriteBytes / 1024 / 1024),
		ReadCount:  int64(ioCounters.ReadCount),
		WriteCount: int64(ioCounters.WriteCount),
		ReadBytes:  ioCounters.ReadBytes,
		WriteBytes: ioCounters.WriteBytes,
	}

	return nil
}

// collectNetwork 收集网络信息
func (urc *UnifiedResourceCollectorImpl) collectNetwork(p *process.Process, usage *UnifiedResourceUsage) error {
	connections, err := p.Connections()
	if err != nil {
		return err
	}

	// 简化的网络使用估算 - 基于连接数
	totalSent := uint64(len(connections)) * 1024  // 估算每个连接1KB
	totalRecv := uint64(len(connections)) * 1024  // 估算每个连接1KB

	usage.Network = NetworkUsage{
		SentKB:      int64(totalSent / 1024),
		RecvKB:      int64(totalRecv / 1024),
		SentBytes:   totalSent,
		RecvBytes:   totalRecv,
		Connections: len(connections),
	}

	return nil
}

// collectThreads 收集线程信息
func (urc *UnifiedResourceCollectorImpl) collectThreads(p *process.Process, usage *UnifiedResourceUsage) error {
	threads, err := p.NumThreads()
	if err != nil {
		return err
	}

	usage.Threads = threads
	return nil
}

// collectSystemCPU 收集系统CPU信息
func (urc *UnifiedResourceCollectorImpl) collectSystemCPU(systemUsage *SystemResourceUsage) error {
	cpuTimes, err := cpu.Times(false)
	if err != nil {
		return err
	}

	if len(cpuTimes) > 0 {
		total := cpuTimes[0]
		idlePercent := (total.Idle / total.Total()) * 100
		systemPercent := (total.System / total.Total()) * 100
		userPercent := (total.User / total.Total()) * 100

		systemUsage.CPU = SystemCPUUsage{
			SystemPercent: systemPercent,
			UserPercent:   userPercent,
			IdlePercent:   idlePercent,
		}
	}

	// 获取负载平均值
	if loadAvg, err := load.Avg(); err == nil {
		systemUsage.CPU.LoadAvg1 = loadAvg.Load1
		systemUsage.CPU.LoadAvg5 = loadAvg.Load5
		systemUsage.CPU.LoadAvg15 = loadAvg.Load15
	}

	return nil
}

// collectSystemMemory 收集系统内存信息
func (urc *UnifiedResourceCollectorImpl) collectSystemMemory(systemUsage *SystemResourceUsage) error {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return err
	}

	systemUsage.Memory = SystemMemoryUsage{
		TotalMB:     int64(memStats.Total / 1024 / 1024),
		UsedMB:      int64(memStats.Used / 1024 / 1024),
		FreeMB:      int64(memStats.Free / 1024 / 1024),
		AvailableMB: int64(memStats.Available / 1024 / 1024),
		UsedPercent: memStats.UsedPercent,
		BuffersMB:   int64(memStats.Buffers / 1024 / 1024),
		CachedMB:    int64(memStats.Cached / 1024 / 1024),
	}

	return nil
}

// collectSystemDisk 收集系统磁盘信息
func (urc *UnifiedResourceCollectorImpl) collectSystemDisk(systemUsage *SystemResourceUsage) error {
	diskStats, err := disk.IOCounters()
	if err != nil {
		return err
	}

	var totalReadMB, totalWriteMB int64
	for _, stats := range diskStats {
		totalReadMB += int64(stats.ReadBytes / 1024 / 1024)
		totalWriteMB += int64(stats.WriteBytes / 1024 / 1024)
	}

	systemUsage.Disk = SystemDiskUsage{
		ReadMB:  totalReadMB,
		WriteMB: totalWriteMB,
	}

	return nil
}

// collectSystemNetwork 收集系统网络信息
func (urc *UnifiedResourceCollectorImpl) collectSystemNetwork(systemUsage *SystemResourceUsage) error {
	netStats, err := net.IOCounters(false)
	if err != nil {
		return err
	}

	if len(netStats) > 0 {
		stats := netStats[0]
		systemUsage.Network = SystemNetworkUsage{
			SentKB:     int64(stats.BytesSent / 1024),
			RecvKB:     int64(stats.BytesRecv / 1024),
			PacketsIn:  int64(stats.PacketsRecv),
			PacketsOut: int64(stats.PacketsSent),
			ErrorsIn:   int64(stats.Errin),
			ErrorsOut:  int64(stats.Errout),
		}
	}

	return nil
}

// calculatePerformanceScore 计算性能分数
func (urc *UnifiedResourceCollectorImpl) calculatePerformanceScore(usage *UnifiedResourceUsage) {
	score := 100.0

	// CPU评分
	if usage.CPU.UsedPercent > 80 {
		score -= 30
	} else if usage.CPU.UsedPercent > 60 {
		score -= 15
	} else if usage.CPU.UsedPercent > 40 {
		score -= 5
	}

	// 内存评分
	if usage.Memory.UsedPercent > 80 {
		score -= 30
	} else if usage.Memory.UsedPercent > 60 {
		score -= 15
	} else if usage.Memory.UsedPercent > 40 {
		score -= 5
	}

	// 线程评分
	if usage.Threads > 100 {
		score -= 10
	} else if usage.Threads > 50 {
		score -= 5
	}

	// 确保分数在合理范围内
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	usage.Performance.Score = score
	usage.Performance.HealthStatus = urc.getHealthStatus(score)
}

// getHealthStatus 根据分数获取健康状态
func (urc *UnifiedResourceCollectorImpl) getHealthStatus(score float64) string {
	if score >= 80 {
		return "healthy"
	} else if score >= 60 {
		return "warning"
	} else if score >= 40 {
		return "critical"
	}
	return "failing"
}

// 缓存管理方法

func (urc *UnifiedResourceCollectorImpl) getFromCache(pid int32) *UnifiedResourceUsage {
	urc.mutex.RLock()
	defer urc.mutex.RUnlock()

	cached, exists := urc.cache[pid]
	if !exists || time.Now().After(cached.expiration) {
		return nil
	}

	return cached.usage
}

func (urc *UnifiedResourceCollectorImpl) addToCache(pid int32, usage *UnifiedResourceUsage) {
	urc.mutex.Lock()
	defer urc.mutex.Unlock()

	// 如果缓存已满，清理最旧的条目
	if len(urc.cache) >= urc.config.MaxCacheSize {
		urc.evictOldestCacheEntry()
	}

	urc.cache[pid] = &cachedResourceUsage{
		usage:      usage,
		expiration: time.Now().Add(urc.config.CacheTTL),
	}
}

func (urc *UnifiedResourceCollectorImpl) evictOldestCacheEntry() {
	var oldestPID int32
	var oldestTime time.Time

	for pid, cached := range urc.cache {
		if oldestTime.IsZero() || cached.expiration.Before(oldestTime) {
			oldestPID = pid
			oldestTime = cached.expiration
		}
	}

	if oldestPID != 0 {
		delete(urc.cache, oldestPID)
		urc.stats.Evictions++
	}
}

// InvalidateCache 使特定进程的缓存失效
func (urc *UnifiedResourceCollectorImpl) InvalidateCache(pid int32) {
	urc.mutex.Lock()
	defer urc.mutex.Unlock()

	delete(urc.cache, pid)
}

// InvalidateAllCache 使所有缓存失效
func (urc *UnifiedResourceCollectorImpl) InvalidateAllCache() {
	urc.mutex.Lock()
	defer urc.mutex.Unlock()

	urc.cache = make(map[int32]*cachedResourceUsage)
}

// GetCacheStats 获取缓存统计
func (urc *UnifiedResourceCollectorImpl) GetCacheStats() CacheStats {
	urc.mutex.RLock()
	defer urc.mutex.RUnlock()

	stats := CacheStats{
		Size:      len(urc.cache),
		MaxSize:   urc.config.MaxCacheSize,
		Hits:      urc.stats.Hits,
		Misses:    urc.stats.Misses,
		Evictions: urc.stats.Evictions,
	}

	if stats.Hits+stats.Misses > 0 {
		stats.HitRate = float64(stats.Hits) / float64(stats.Hits+stats.Misses)
	}

	// 计算缓存条目年龄
	var oldestTime, newestTime time.Time
	for _, cached := range urc.cache {
		if oldestTime.IsZero() || cached.expiration.Before(oldestTime) {
			oldestTime = cached.expiration
		}
		if newestTime.IsZero() || cached.expiration.After(newestTime) {
			newestTime = cached.expiration
		}
	}

	stats.AgeOldest = time.Since(oldestTime.Add(-urc.config.CacheTTL))
	stats.AgeNewest = time.Since(newestTime.Add(-urc.config.CacheTTL))

	return stats
}

// 配置管理方法

// UpdateConfig 更新配置
func (urc *UnifiedResourceCollectorImpl) UpdateConfig(config ResourceCollectionConfig) error {
	urc.mutex.Lock()
	defer urc.mutex.Unlock()

	urc.config = config

	// 如果历史功能状态改变，更新历史管理器
	if config.EnableHistory && urc.history == nil {
		urc.history = &ResourceHistoryManager{
			enabled:   true,
			retention: config.HistoryRetention,
			records:   make(map[int32][]*UnifiedResourceUsage),
		}
	} else if !config.EnableHistory && urc.history != nil {
		urc.history = nil
	}

	return nil
}

// GetConfig 获取当前配置
func (urc *UnifiedResourceCollectorImpl) GetConfig() ResourceCollectionConfig {
	urc.mutex.RLock()
	defer urc.mutex.RUnlock()

	return urc.config
}

// GetCollectionStats 获取收集统计
func (urc *UnifiedResourceCollectorImpl) GetCollectionStats() CollectionStats {
	urc.mutex.RLock()
	defer urc.mutex.RUnlock()

	return urc.stats
}

// 内部辅助方法

func (urc *UnifiedResourceCollectorImpl) recordError(errMsg string) {
	urc.mutex.Lock()
	defer urc.mutex.Unlock()

	urc.stats.Errors = append(urc.stats.Errors, errMsg)
	urc.stats.FailedCollections++

	// 保持错误列表在合理大小
	if len(urc.stats.Errors) > 100 {
		urc.stats.Errors = urc.stats.Errors[len(urc.stats.Errors)-100:]
	}
}

func (urc *UnifiedResourceCollectorImpl) updateCollectionStats(start time.Time) {
	urc.mutex.Lock()
	defer urc.mutex.Unlock()

	duration := time.Since(start)
	totalCollections := urc.stats.TotalCollections
	
	if totalCollections > 0 {
		urc.stats.AvgCollectionTime = time.Duration(
			(int64(urc.stats.AvgCollectionTime)*(totalCollections-1) + duration.Nanoseconds()) /
			totalCollections,
		)
	} else {
		urc.stats.AvgCollectionTime = duration
	}

	urc.stats.LastCollection = time.Now()
}

// ResourceHistoryManager 方法实现

// AddRecord 添加历史记录
func (rhm *ResourceHistoryManager) AddRecord(pid int32, usage *UnifiedResourceUsage) error {
	if !rhm.enabled {
		return nil
	}

	rhm.mutex.Lock()
	defer rhm.mutex.Unlock()

	rhm.records[pid] = append(rhm.records[pid], usage)

	// 清理过期记录
	rhm.cleanupOldRecords(pid)

	return nil
}

// GetHistory 获取历史记录
func (rhm *ResourceHistoryManager) GetHistory(pid int32, duration time.Duration) ([]*UnifiedResourceUsage, error) {
	if !rhm.enabled {
		return []*UnifiedResourceUsage{}, nil
	}

	rhm.mutex.RLock()
	defer rhm.mutex.RUnlock()

	records := rhm.records[pid]
	cutoff := time.Now().Add(-duration)

	var result []*UnifiedResourceUsage
	for _, record := range records {
		if record.Timestamp.After(cutoff) {
			result = append(result, record)
		}
	}

	return result, nil
}

// GetLatest 获取最新记录
func (rhm *ResourceHistoryManager) GetLatest(pid int32) (*UnifiedResourceUsage, error) {
	if !rhm.enabled {
		return nil, nil
	}

	rhm.mutex.RLock()
	defer rhm.mutex.RUnlock()

	records := rhm.records[pid]
	if len(records) == 0 {
		return nil, nil
	}

	return records[len(records)-1], nil
}

// GetAggregatedStats 获取聚合统计
func (rhm *ResourceHistoryManager) GetAggregatedStats(pid int32, duration time.Duration) (*AggregatedResourceStats, error) {
	records, err := rhm.GetHistory(pid, duration)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return &AggregatedResourceStats{
			PID:         pid,
			Duration:    duration,
			SampleCount: 0,
		}, nil
	}

	stats := &AggregatedResourceStats{
		PID:          pid,
		Duration:     duration,
		SampleCount:  int64(len(records)),
		StartTime:    records[0].Timestamp,
		EndTime:      records[len(records)-1].Timestamp,
	}

	// 计算CPU统计
	var cpuValues []float64
	var memoryValues []float64
	var diskReadValues []float64
	var diskWriteValues []float64

	for _, record := range records {
		cpuValues = append(cpuValues, record.CPU.UsedPercent)
		memoryValues = append(memoryValues, float64(record.Memory.UsedMB))
		diskReadValues = append(diskReadValues, float64(record.Disk.ReadMB))
		diskWriteValues = append(diskWriteValues, float64(record.Disk.WriteMB))
	}

	stats.CPUStats = rhm.calculateResourceStat(cpuValues)
	stats.MemoryStats = rhm.calculateResourceStat(memoryValues)
	stats.DiskStats = rhm.calculateResourceStat(diskReadValues)
	stats.NetworkStats = rhm.calculateResourceStat(diskWriteValues)

	return stats, nil
}

// calculateResourceStat 计算资源统计
func (rhm *ResourceHistoryManager) calculateResourceStat(values []float64) ResourceStat {
	if len(values) == 0 {
		return ResourceStat{}
	}

	sort.Float64s(values)

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	avg := sum / float64(len(values))
	min := values[0]
	max := values[len(values)-1]

	// 计算标准差
	var variance float64
	for _, v := range values {
		variance += (v - avg) * (v - avg)
	}
	variance /= float64(len(values))
	stdDev := variance

	// 计算趋势
	trend := "stable"
	if len(values) > 1 {
		firstHalf := values[:len(values)/2]
		secondHalf := values[len(values)/2:]
		
		firstAvg := sum / float64(len(firstHalf))
		secondAvg := sum / float64(len(secondHalf))
		
		if secondAvg > firstAvg*1.1 {
			trend = "increasing"
		} else if secondAvg < firstAvg*0.9 {
			trend = "decreasing"
		}
	}

	// 计算95百分位
	percentile95 := values[int(float64(len(values))*0.95)]
	if len(values) > 0 {
		percentile95 = values[len(values)-1]
	}

	return ResourceStat{
		Average:      avg,
		Min:          min,
		Max:          max,
		Percentile95: percentile95,
		StdDev:       stdDev,
		Trend:        trend,
	}
}

// CleanupOldRecords 清理过期记录
func (rhm *ResourceHistoryManager) CleanupOldRecords(olderThan time.Duration) error {
	if !rhm.enabled {
		return nil
	}

	rhm.mutex.Lock()
	defer rhm.mutex.Unlock()

	cutoff := time.Now().Add(-olderThan)
	for pid := range rhm.records {
		rhm.cleanupOldRecordsByTime(pid, cutoff)
	}

	return nil
}

// cleanupOldRecords 清理特定进程的过期记录
func (rhm *ResourceHistoryManager) cleanupOldRecords(pid int32) {
	cutoff := time.Now().Add(-rhm.retention)
	rhm.cleanupOldRecordsByTime(pid, cutoff)
}

func (rhm *ResourceHistoryManager) cleanupOldRecordsByTime(pid int32, cutoff time.Time) {
	records := rhm.records[pid]
	var validRecords []*UnifiedResourceUsage

	for _, record := range records {
		if record.Timestamp.After(cutoff) {
			validRecords = append(validRecords, record)
		}
	}

	rhm.records[pid] = validRecords
}

// GetHistoryStats 获取历史统计
func (rhm *ResourceHistoryManager) GetHistoryStats() HistoryStats {
	if !rhm.enabled {
		return HistoryStats{}
	}

	rhm.mutex.RLock()
	defer rhm.mutex.RUnlock()

	stats := HistoryStats{
		RecordCountByPID: make(map[int32]int),
	}

	var oldestTime, newestTime time.Time
	totalRecords := 0

	for pid, records := range rhm.records {
		stats.RecordCountByPID[pid] = len(records)
		totalRecords += len(records)

		for _, record := range records {
			if oldestTime.IsZero() || record.Timestamp.Before(oldestTime) {
				oldestTime = record.Timestamp
			}
			if newestTime.IsZero() || record.Timestamp.After(newestTime) {
				newestTime = record.Timestamp
			}
		}
	}

	stats.TotalRecords = int64(totalRecords)
	stats.OldestRecord = oldestTime
	stats.NewestRecord = newestTime

	return stats
}