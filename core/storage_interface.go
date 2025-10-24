package core

import (
	"time"
)

// Storage 定义了数据存储的接口
// 支持多种存储后端：CSV文件、SQLite数据库等
type Storage interface {
	// Initialize 初始化存储
	Initialize() error

	// Close 关闭存储并清理资源
	Close() error

	// SaveRecords 保存多条资源记录
	SaveRecords(records []ResourceRecord) error

	// SaveRecord 保存单条资源记录
	SaveRecord(record ResourceRecord) error

	// ReadRecords 从指定文件读取资源记录
	ReadRecords(filePath string) ([]ResourceRecord, error)

	// ReadRecordsByTimeRange 按时间范围读取记录
	ReadRecordsByTimeRange(start, end time.Time) ([]ResourceRecord, error)

	// GetRecordCount 获取记录总数
	GetRecordCount() (int, error)

	// CalculateStats 计算资源统计信息
	CalculateStats(records []ResourceRecord) []ResourceStats

	// CleanOldData 清理旧数据
	CleanOldData(keepDays int) error

	// GetStorageInfo 获取存储信息
	GetStorageInfo() StorageInfo
}

// StorageInfo 存储信息
type StorageInfo struct {
	Type           string    `json:"type"`            // 存储类型: "csv", "sqlite"
	TotalRecords   int       `json:"total_records"`   // 总记录数
	TotalSize      int64     `json:"total_size"`      // 总大小（字节）
	OldestRecord   time.Time `json:"oldest_record"`   // 最早记录时间
	NewestRecord   time.Time `json:"newest_record"`   // 最新记录时间
	FilePath       string    `json:"file_path"`       // 文件路径
	LastModified   time.Time `json:"last_modified"`   // 最后修改时间
}


// NewStorage 创建新的存储实例
func NewStorage(dataFile string, bufferSize int, useStorageMgr bool, config StorageConfig) Storage {
	if config.Type == "sqlite" || config.SQLitePath != "" {
		return NewSQLiteStorage(dataFile, bufferSize, config)
	}
	// 默认使用CSV存储（向后兼容）
	return NewManager(dataFile, bufferSize, useStorageMgr, config)
}