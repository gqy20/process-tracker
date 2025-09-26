package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BioToolInfo 生物信息学工具基本信息
type BioToolInfo struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Path         string            `json:"path"`
	Description  string            `json:"description"`
	Dependencies []string          `json:"dependencies"`
	Tags         []string          `json:"tags"`
	Category     string            `json:"category"`
	Parameters   map[string]string `json:"parameters"`
}

// BioToolInstance 生物信息学工具实例
type BioToolInstance struct {
	ToolID      string    `json:"tool_id"`
	PID         int32     `json:"pid"`
	Cmdline     string    `json:"cmdline"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	ExitCode    int       `json:"exit_code"`
	WorkingDir  string    `json:"working_dir"`
	InputFiles  []string  `json:"input_files"`
	OutputFiles []string  `json:"output_files"`
}

// BioToolsManager 简化的生物信息学工具管理器
type BioToolsManager struct {
	availableTools map[string]*BioToolInfo
	activeInstances map[string]*BioToolInstance
	processManager *SimplifiedProcessManager
	config        *BioToolsConfig
	mu            sync.RWMutex
}


// NewBioToolsManager 创建新的生物信息学工具管理器
func NewBioToolsManager(config *BioToolsConfig, processManager *SimplifiedProcessManager) *BioToolsManager {
	btm := &BioToolsManager{
		availableTools:  make(map[string]*BioToolInfo),
		activeInstances: make(map[string]*BioToolInstance),
		processManager:  processManager,
		config:         config,
	}
	
	// 自动发现工具
	btm.DiscoverTools()
	
	return btm
}

// DiscoverTools 发现系统中可用的生物信息学工具
func (btm *BioToolsManager) DiscoverTools() {
	btm.mu.Lock()
	defer btm.mu.Unlock()

	// 常见生物信息学工具及其特征
	bioTools := map[string]struct {
		name         string
		binaries     []string
		description  string
		category     string
		dependencies []string
		tags         []string
	}{
		"blast": {
			name:         "BLAST",
			binaries:     []string{"blastn", "blastp", "blastx", "tblastn", "tblastx"},
			description:  "Basic Local Alignment Search Tool",
			category:     "Alignment",
			dependencies: []string{},
			tags:         []string{"alignment", "sequence", "search"},
		},
		"bwa": {
			name:         "BWA",
			binaries:     []string{"bwa"},
			description:  "Burrows-Wheeler Aligner for short-read alignment",
			category:     "Alignment",
			dependencies: []string{},
			tags:         []string{"alignment", "short-read", "mapping"},
		},
		"samtools": {
			name:         "SAMtools",
			binaries:     []string{"samtools"},
			description:  "Tools for manipulating alignments in SAM/BAM format",
			category:     "Utilities",
			dependencies: []string{},
			tags:         []string{"sam", "bam", "format", "utilities"},
		},
		"bcftools": {
			name:         "BCFtools",
			binaries:     []string{"bcftools"},
			description:  "Tools for calling and manipulating VCF/BCF files",
			category:     "Variant Calling",
			dependencies: []string{},
			tags:         []string{"vcf", "bcf", "variant", "calling"},
		},
		"gatk": {
			name:         "GATK",
			binaries:     []string{"gatk"},
			description:  "Genome Analysis Toolkit for variant discovery",
			category:     "Variant Calling",
			dependencies: []string{"java"},
			tags:         []string{"variant", "calling", "java"},
		},
		"fastqc": {
			name:         "FastQC",
			binaries:     []string{"fastqc"},
			description:  "Quality control tool for high throughput sequence data",
			category:     "Quality Control",
			dependencies: []string{"java"},
			tags:         []string{"quality", "control", "fastq"},
		},
		"trimmomatic": {
			name:         "Trimmomatic",
			binaries:     []string{"trimmomatic"},
			description:  "Flexible read trimming tool for Illumina NGS data",
			category:     "Preprocessing",
			dependencies: []string{"java"},
			tags:         []string{"trimming", "preprocessing", "adapter"},
		},
		"bowtie2": {
			name:         "Bowtie2",
			binaries:     []string{"bowtie2"},
			description:  "Ultrafast, memory-efficient short read aligner",
			category:     "Alignment",
			dependencies: []string{},
			tags:         []string{"alignment", "short-read", "fast"},
		},
		"hisat2": {
			name:         "HISAT2",
			binaries:     []string{"hisat2"},
			description:  "Hierarchical indexing for spliced alignment of transcripts",
			category:     "Alignment",
			dependencies: []string{},
			tags:         []string{"alignment", "rna-seq", "spliced"},
		},
		"star": {
			name:         "STAR",
			binaries:     []string{"STAR"},
			description:  "Spliced Transcripts Alignment to a Reference",
			category:     "Alignment",
			dependencies: []string{},
			tags:         []string{"alignment", "rna-seq", "spliced"},
		},
		"bedtools": {
			name:         "BEDTools",
			binaries:     []string{"bedtools"},
			description:  "Toolbox for genome arithmetic",
			category:     "Utilities",
			dependencies: []string{},
			tags:         []string{"bed", "interval", "arithmetic"},
		},
		"vcftools": {
			name:         "VCFtools",
			binaries:     []string{"vcftools"},
			description:  "Tools for working with VCF files",
			category:     "Variant Calling",
			dependencies: []string{},
			tags:         []string{"vcf", "variant", "format"},
		},
		"plink": {
			name:         "PLINK",
			binaries:     []string{"plink", "plink2"},
			description:  "Whole genome association analysis toolset",
			category:     "Analysis",
			dependencies: []string{},
			tags:         []string{"association", "gwas", "analysis"},
		},
		"minimap2": {
			name:         "minimap2",
			binaries:     []string{"minimap2"},
			description:  "Versatile pairwise aligner for genomic and spliced sequences",
			category:     "Alignment",
			dependencies: []string{},
			tags:         []string{"alignment", "long-read", "versatile"},
		},
		"seqtk": {
			name:         "seqtk",
			binaries:     []string{"seqtk"},
			description:  "Toolkit for processing sequences in FASTA/FASTQ format",
			category:     "Utilities",
			dependencies: []string{},
			tags:         []string{"fasta", "fastq", "processing"},
		},
		"multiqc": {
			name:         "MultiQC",
			binaries:     []string{"multiqc"},
			description:  "Aggregate results from bioinformatics analyses across many samples",
			category:     "Quality Control",
			dependencies: []string{"python"},
			tags:         []string{"aggregation", "qc", "reporting"},
		},
	}

	// 检查每个工具是否可用
	for toolID, toolInfo := range bioTools {
		foundPath := ""
		for _, binary := range toolInfo.binaries {
			path, err := exec.LookPath(binary)
			if err == nil {
				foundPath = path
				break
			}
		}
		
		if foundPath != "" {
			// 获取版本信息
			version := btm.getToolVersion(foundPath, toolID)
			
			btm.availableTools[toolID] = &BioToolInfo{
				Name:         toolInfo.name,
				Version:      version,
				Path:         foundPath,
				Description:  toolInfo.description,
				Dependencies: toolInfo.dependencies,
				Tags:         toolInfo.tags,
				Category:     toolInfo.category,
				Parameters:   make(map[string]string),
			}
		}
	}
}

// getToolVersion 获取工具版本
func (btm *BioToolsManager) getToolVersion(path string, toolID string) string {
	var cmd *exec.Cmd
	
	switch toolID {
	case "blast":
		cmd = exec.Command(path, "-version")
	case "bwa":
		cmd = exec.Command(path)
	case "samtools":
		cmd = exec.Command(path, "--version")
	case "bcftools":
		cmd = exec.Command(path, "--version")
	case "gatk":
		cmd = exec.Command(path, "--version")
	case "fastqc":
		cmd = exec.Command(path, "--version")
	case "bowtie2":
		cmd = exec.Command(path, "--version")
	case "hisat2":
		cmd = exec.Command(path, "--version")
	case "star":
		cmd = exec.Command(path, "--version")
	case "bedtools":
		cmd = exec.Command(path, "--version")
	case "vcftools":
		cmd = exec.Command(path, "--version")
	case "plink":
		cmd = exec.Command(path, "--version")
	case "minimap2":
		cmd = exec.Command(path, "--version")
	case "seqtk":
		cmd = exec.Command(path)
	case "multiqc":
		cmd = exec.Command(path, "--version")
	default:
		return "unknown"
	}
	
	if cmd != nil {
		output, err := cmd.CombinedOutput()
		if err == nil {
			// 简单提取版本号
			lines := strings.Split(string(output), "\n")
			if len(lines) > 0 {
				versionLine := strings.TrimSpace(lines[0])
				if strings.Contains(versionLine, "version") {
					return strings.TrimSpace(strings.Split(versionLine, "version")[1])
				}
				return versionLine
			}
		}
	}
	
	return "unknown"
}

// GetAvailableTools 获取所有可用工具
func (btm *BioToolsManager) GetAvailableTools() map[string]*BioToolInfo {
	btm.mu.RLock()
	defer btm.mu.RUnlock()
	
	result := make(map[string]*BioToolInfo)
	for k, v := range btm.availableTools {
		result[k] = v
	}
	return result
}

// GetToolInfo 获取特定工具信息
func (btm *BioToolsManager) GetToolInfo(toolID string) (*BioToolInfo, error) {
	btm.mu.RLock()
	defer btm.mu.RUnlock()
	
	tool, exists := btm.availableTools[toolID]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", toolID)
	}
	
	return tool, nil
}

// RunTool 运行生物信息学工具
func (btm *BioToolsManager) RunTool(toolID string, args []string, workingDir string, inputFiles []string) (*BioToolInstance, error) {
	btm.mu.Lock()
	defer btm.mu.Unlock()

	tool, exists := btm.availableTools[toolID]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", toolID)
	}

	// 检查实例限制
	if len(btm.activeInstances) >= btm.config.MaxInstances {
		return nil, fmt.Errorf("maximum number of instances (%d) reached", btm.config.MaxInstances)
	}

	// 创建命令
	cmd := exec.Command(tool.Path, args...)
	
	// 设置工作目录
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start tool %s: %v", toolID, err)
	}

	// 创建实例记录
	instance := &BioToolInstance{
		ToolID:     toolID,
		PID:        int32(cmd.Process.Pid),
		Cmdline:    strings.Join([]string{tool.Path}, " ") + " " + strings.Join(args, " "),
		Status:     "running",
		StartTime:  time.Now(),
		WorkingDir: workingDir,
		InputFiles: inputFiles,
	}

	btm.activeInstances[instance.ToolID+"_"+fmt.Sprintf("%d", instance.PID)] = instance

	// 异步等待进程完成
	go btm.waitForProcess(instance, cmd)

	return instance, nil
}

// waitForProcess 等待进程完成
func (btm *BioToolsManager) waitForProcess(instance *BioToolInstance, cmd *exec.Cmd) {
	err := cmd.Wait()
	
	btm.mu.Lock()
	defer btm.mu.Unlock()
	
	instance.EndTime = time.Now()
	instance.Status = "completed"
	
	if err != nil {
		instance.ExitCode = 1
		instance.Status = "failed"
	} else {
		instance.ExitCode = 0
	}
	
	// 收集输出文件
	btm.collectOutputFiles(instance)
}

// collectOutputFiles 收集输出文件
func (btm *BioToolsManager) collectOutputFiles(instance *BioToolInstance) {
	if instance.WorkingDir == "" {
		return
	}
	
	// 简单的输出文件收集逻辑
	// 实际应用中可以根据工具类型和输入文件推断输出文件
	files, err := os.ReadDir(instance.WorkingDir)
	if err != nil {
		return
	}
	
	for _, file := range files {
		if !file.IsDir() {
			instance.OutputFiles = append(instance.OutputFiles, 
				filepath.Join(instance.WorkingDir, file.Name()))
		}
	}
}

// GetActiveInstances 获取活动实例
func (btm *BioToolsManager) GetActiveInstances() map[string]*BioToolInstance {
	btm.mu.RLock()
	defer btm.mu.RUnlock()
	
	result := make(map[string]*BioToolInstance)
	for k, v := range btm.activeInstances {
		result[k] = v
	}
	return result
}

// GetInstance 获取特定实例
func (btm *BioToolsManager) GetInstance(instanceID string) (*BioToolInstance, error) {
	btm.mu.RLock()
	defer btm.mu.RUnlock()
	
	instance, exists := btm.activeInstances[instanceID]
	if !exists {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}
	
	return instance, nil
}

// StopTool 停止工具实例
func (btm *BioToolsManager) StopTool(instanceID string) error {
	btm.mu.Lock()
	defer btm.mu.Unlock()
	
	instance, exists := btm.activeInstances[instanceID]
	if !exists {
		return fmt.Errorf("instance %s not found", instanceID)
	}
	
	if instance.Status != "running" {
		return fmt.Errorf("instance %s is not running", instanceID)
	}
	
	// 通过进程管理器停止进程
	err := btm.processManager.StopProcess(instance.PID)
	if err != nil {
		return fmt.Errorf("failed to stop process %d: %v", instance.PID, err)
	}
	
	instance.Status = "stopped"
	instance.EndTime = time.Now()
	
	return nil
}

// GetToolStatus 获取工具状态（使用统一监控）
func (btm *BioToolsManager) GetToolStatus(instanceID string) (map[string]interface{}, error) {
	btm.mu.RLock()
	defer btm.mu.RUnlock()
	
	instance, exists := btm.activeInstances[instanceID]
	if !exists {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}
	
	status := map[string]interface{}{
		"instance_id":   instanceID,
		"tool_id":       instance.ToolID,
		"status":        instance.Status,
		"pid":           instance.PID,
		"start_time":    instance.StartTime,
		"end_time":      instance.EndTime,
		"exit_code":     instance.ExitCode,
		"working_dir":   instance.WorkingDir,
		"input_files":   instance.InputFiles,
		"output_files":  instance.OutputFiles,
	}
	
	// 如果进程正在运行，获取详细的资源使用情况
	if instance.Status == "running" && btm.config.EnableMonitoring {
		stats := btm.processManager.GetProcessStats()
		if processStats, ok := stats[fmt.Sprintf("%d", instance.PID)]; ok {
			status["resource_usage"] = processStats
		}
	}
	
	return status, nil
}

// CleanupCompletedInstances 清理完成的实例
func (btm *BioToolsManager) CleanupCompletedInstances(maxAge time.Duration) {
	btm.mu.Lock()
	defer btm.mu.Unlock()
	
	now := time.Now()
	for id, instance := range btm.activeInstances {
		if instance.Status != "running" && now.Sub(instance.EndTime) > maxAge {
			delete(btm.activeInstances, id)
		}
	}
}

// GetToolsByCategory 按类别获取工具
func (btm *BioToolsManager) GetToolsByCategory(category string) map[string]*BioToolInfo {
	btm.mu.RLock()
	defer btm.mu.RUnlock()
	
	result := make(map[string]*BioToolInfo)
	for id, tool := range btm.availableTools {
		if tool.Category == category {
			result[id] = tool
		}
	}
	return result
}

// SearchTools 搜索工具
func (btm *BioToolsManager) SearchTools(query string) map[string]*BioToolInfo {
	btm.mu.RLock()
	defer btm.mu.RUnlock()
	
	query = strings.ToLower(query)
	result := make(map[string]*BioToolInfo)
	
	for id, tool := range btm.availableTools {
		if strings.Contains(strings.ToLower(tool.Name), query) ||
		   strings.Contains(strings.ToLower(tool.Description), query) ||
		   strings.Contains(strings.ToLower(tool.Category), query) {
			result[id] = tool
		}
	}
	
	return result
}