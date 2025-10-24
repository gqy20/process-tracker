package core

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage SQLite存储实现
type SQLiteStorage struct {
	db         *sql.DB
	dataFile   string
	bufferSize int
	config     StorageConfig
	lastFlush  time.Time
	sqlitePath string
}

// NewSQLiteStorage 创建新的SQLite存储实例
func NewSQLiteStorage(dataFile string, bufferSize int, config StorageConfig) *SQLiteStorage {
	// 确定SQLite数据库路径
	sqlitePath := config.SQLitePath
	if sqlitePath == "" {
		// 默认使用与数据文件同目录的SQLite数据库
		dir := filepath.Dir(dataFile)
		base := filepath.Base(dataFile)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		sqlitePath = filepath.Join(dir, name+".db")
	}

	return &SQLiteStorage{
		dataFile:   dataFile,
		bufferSize: bufferSize,
		config:     config,
		sqlitePath: sqlitePath,
	}
}

// Initialize 初始化SQLite存储
func (s *SQLiteStorage) Initialize() error {
	// 展开环境变量路径和~符号
	expandedPath := os.ExpandEnv(s.sqlitePath)
	if strings.HasPrefix(expandedPath, "~/") {
		home := os.Getenv("HOME")
		if home != "" {
			expandedPath = filepath.Join(home, expandedPath[2:])
		}
	}
	s.sqlitePath = expandedPath

	// 确保目录存在
	dir := filepath.Dir(s.sqlitePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 打开SQLite数据库
	db, err := sql.Open("sqlite3", s.sqlitePath)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database: %w", err)
	}
	s.db = db

	// 配置SQLite连接
	if err := s.configureSQLite(); err != nil {
		return fmt.Errorf("failed to configure sqlite: %w", err)
	}

	// 创建表结构
	if err := s.createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// 创建索引
	if err := s.createIndexes(); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

// configureSQLite 配置SQLite连接
func (s *SQLiteStorage) configureSQLite() error {
	// 启用外键约束
	if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	// 配置WAL模式（提高并发性能）
	if s.config.SQLiteWAL {
		if _, err := s.db.Exec("PRAGMA journal_mode = WAL"); err != nil {
			return err
		}
	}

	// 配置缓存大小
	if s.config.SQLiteCacheSize > 0 {
		if _, err := s.db.Exec(fmt.Sprintf("PRAGMA cache_size = %d", s.config.SQLiteCacheSize)); err != nil {
			return err
		}
	}

	// 配置同步模式（平衡性能和安全性）
	if _, err := s.db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return err
	}

	// 配置批量提交
	if _, err := s.db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return err
	}

	return nil
}

// createTables 创建数据库表
func (s *SQLiteStorage) createTables() error {
	// 创建资源记录表
	createRecordsSQL := `
	CREATE TABLE IF NOT EXISTS resource_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		name TEXT NOT NULL,
		cpu_percent REAL NOT NULL,
		cpu_percent_normalized REAL NOT NULL,
		memory_mb REAL NOT NULL,
		memory_percent REAL NOT NULL,
		threads INTEGER NOT NULL,
		disk_read_mb REAL NOT NULL,
		disk_write_mb REAL NOT NULL,
		net_sent_kb REAL NOT NULL,
		net_recv_kb REAL NOT NULL,
		is_active BOOLEAN NOT NULL,
		command TEXT,
		working_dir TEXT,
		category TEXT,
		pid INTEGER NOT NULL,
		ppid INTEGER,
		create_time INTEGER,
		cpu_time REAL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(createRecordsSQL); err != nil {
		return fmt.Errorf("failed to create resource_records table: %w", err)
	}

	// 创建元数据表（用于存储存储信息）
	createMetaSQL := `
	CREATE TABLE IF NOT EXISTS storage_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(createMetaSQL); err != nil {
		return fmt.Errorf("failed to create storage_meta table: %w", err)
	}

	// 初始化元数据
	s.initMeta()

	return nil
}

// createIndexes 创建索引
func (s *SQLiteStorage) createIndexes() error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_resource_records_timestamp ON resource_records(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_resource_records_name ON resource_records(name)",
		"CREATE INDEX IF NOT EXISTS idx_resource_records_pid ON resource_records(pid)",
		"CREATE INDEX IF NOT EXISTS idx_resource_records_created_at ON resource_records(created_at)",
	}

	for _, indexSQL := range indexes {
		if _, err := s.db.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// initMeta 初始化元数据
func (s *SQLiteStorage) initMeta() {
	// 设置存储类型
	s.db.Exec("INSERT OR REPLACE INTO storage_meta (key, value) VALUES ('storage_type', 'sqlite')")

	// 设置创建时间
	if _, err := s.db.Exec("INSERT OR IGNORE INTO storage_meta (key, value) VALUES ('created_at', ?)", time.Now().Format(time.RFC3339)); err != nil {
		log.Printf("Warning: failed to set created_at meta: %v", err)
	}
}

// SaveRecords 保存多条资源记录
func (s *SQLiteStorage) SaveRecords(records []ResourceRecord) error {
	if len(records) == 0 {
		return nil
	}

	// 开始事务
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 准备插入语句
	stmt, err := tx.Prepare(`
		INSERT INTO resource_records (
			timestamp, name, cpu_percent, cpu_percent_normalized,
			memory_mb, memory_percent, threads, disk_read_mb, disk_write_mb,
			net_sent_kb, net_recv_kb, is_active, command, working_dir,
			category, pid, ppid, create_time, cpu_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	// 批量插入记录
	for _, record := range records {
		_, err := stmt.Exec(
			record.Timestamp,
			record.Name,
			record.CPUPercent,
			record.CPUPercentNormalized,
			record.MemoryMB,
			record.MemoryPercent,
			record.Threads,
			record.DiskReadMB,
			record.DiskWriteMB,
			record.NetSentKB,
			record.NetRecvKB,
			record.IsActive,
			record.Command,
			record.WorkingDir,
			record.Category,
			record.PID,
			record.PPID,
			record.CreateTime,
			record.CPUTime,
		)
		if err != nil {
			return fmt.Errorf("failed to insert record: %w", err)
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.lastFlush = time.Now()
	return nil
}

// SaveRecord 保存单条资源记录
func (s *SQLiteStorage) SaveRecord(record ResourceRecord) error {
	return s.SaveRecords([]ResourceRecord{record})
}

// ReadRecords 读取资源记录
func (s *SQLiteStorage) ReadRecords(filePath string) ([]ResourceRecord, error) {
	// 检查数据库连接
	if s.db == nil {
		return nil, fmt.Errorf("SQLite database not initialized")
	}

	// SQLite存储不依赖filePath参数，而是读取所有记录
	query := `
		SELECT timestamp, name, cpu_percent, cpu_percent_normalized,
			   memory_mb, memory_percent, threads, disk_read_mb, disk_write_mb,
			   net_sent_kb, net_recv_kb, is_active, command, working_dir,
			   category, pid, ppid, create_time, cpu_time
		FROM resource_records
		ORDER BY timestamp DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query records: %w", err)
	}
	defer rows.Close()

	var records []ResourceRecord
	for rows.Next() {
		var record ResourceRecord
		err := rows.Scan(
			&record.Timestamp,
			&record.Name,
			&record.CPUPercent,
			&record.CPUPercentNormalized,
			&record.MemoryMB,
			&record.MemoryPercent,
			&record.Threads,
			&record.DiskReadMB,
			&record.DiskWriteMB,
			&record.NetSentKB,
			&record.NetRecvKB,
			&record.IsActive,
			&record.Command,
			&record.WorkingDir,
			&record.Category,
			&record.PID,
			&record.PPID,
			&record.CreateTime,
			&record.CPUTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading rows: %w", err)
	}

	return records, nil
}

// ReadRecordsByTimeRange 按时间范围读取记录
func (s *SQLiteStorage) ReadRecordsByTimeRange(start, end time.Time) ([]ResourceRecord, error) {
	query := `
		SELECT timestamp, name, cpu_percent, cpu_percent_normalized,
			   memory_mb, memory_percent, threads, disk_read_mb, disk_write_mb,
			   net_sent_kb, net_recv_kb, is_active, command, working_dir,
			   category, pid, ppid, create_time, cpu_time
		FROM resource_records
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC
	`

	rows, err := s.db.Query(query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query records by time range: %w", err)
	}
	defer rows.Close()

	var records []ResourceRecord
	for rows.Next() {
		var record ResourceRecord
		err := rows.Scan(
			&record.Timestamp,
			&record.Name,
			&record.CPUPercent,
			&record.CPUPercentNormalized,
			&record.MemoryMB,
			&record.MemoryPercent,
			&record.Threads,
			&record.DiskReadMB,
			&record.DiskWriteMB,
			&record.NetSentKB,
			&record.NetRecvKB,
			&record.IsActive,
			&record.Command,
			&record.WorkingDir,
			&record.Category,
			&record.PID,
			&record.PPID,
			&record.CreateTime,
			&record.CPUTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// GetRecordCount 获取记录总数
func (s *SQLiteStorage) GetRecordCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM resource_records").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get record count: %w", err)
	}
	return count, nil
}

// CalculateStats 计算资源统计信息
func (s *SQLiteStorage) CalculateStats(records []ResourceRecord) []ResourceStats {
	// 复用现有的统计计算逻辑
	return s.calculateResourceStats(records)
}

// calculateResourceStats 计算资源统计信息的内部方法
func (s *SQLiteStorage) calculateResourceStats(records []ResourceRecord) []ResourceStats {
	// 简单的统计计算实现，可以后续优化
	if len(records) == 0 {
		return []ResourceStats{}
	}

	// 按进程名称分组
	processMap := make(map[string][]ResourceRecord)
	for _, record := range records {
		processMap[record.Name] = append(processMap[record.Name], record)
	}

	var stats []ResourceStats
	for name, processRecords := range processMap {
		if len(processRecords) == 0 {
			continue
		}

		stat := ResourceStats{
			Name:    name,
			Samples: len(processRecords),
		}

		// 计算统计数据
		var totalCPU, totalMem, totalDiskRead, totalDiskWrite, totalNetSent, totalNetRecv float64
		var maxCPU, maxMem, maxDiskRead, maxDiskWrite, maxNetSent, maxNetRecv float64
		var activeTime time.Duration
		var activeSamples int

		firstSeen := processRecords[0].Timestamp
		lastSeen := processRecords[0].Timestamp

		for _, record := range processRecords {
			totalCPU += record.CPUPercent
			totalMem += record.MemoryMB
			totalDiskRead += record.DiskReadMB
			totalDiskWrite += record.DiskWriteMB
			totalNetSent += record.NetSentKB
			totalNetRecv += record.NetRecvKB

			if record.CPUPercent > maxCPU {
				maxCPU = record.CPUPercent
			}
			if record.MemoryMB > maxMem {
				maxMem = record.MemoryMB
			}
			if record.DiskReadMB > maxDiskRead {
				maxDiskRead = record.DiskReadMB
			}
			if record.DiskWriteMB > maxDiskWrite {
				maxDiskWrite = record.DiskWriteMB
			}
			if record.NetSentKB > maxNetSent {
				maxNetSent = record.NetSentKB
			}
			if record.NetRecvKB > maxNetRecv {
				maxNetRecv = record.NetRecvKB
			}

			if record.IsActive {
				activeTime += 5 * time.Second // 假设5秒间隔
				activeSamples++
			}

			if record.Timestamp.Before(firstSeen) {
				firstSeen = record.Timestamp
			}
			if record.Timestamp.After(lastSeen) {
				lastSeen = record.Timestamp
			}
		}

		sampleCount := float64(len(processRecords))
		stat.CPUAvg = totalCPU / sampleCount
		stat.CPUMax = maxCPU
		stat.MemoryAvg = totalMem / sampleCount
		stat.MemoryMax = maxMem
		stat.DiskReadAvg = totalDiskRead / sampleCount
		stat.DiskWriteAvg = totalDiskWrite / sampleCount
		stat.NetSentAvg = totalNetSent / sampleCount
		stat.NetRecvAvg = totalNetRecv / sampleCount
		stat.ActiveTime = activeTime
		stat.ActiveSamples = activeSamples
		stat.FirstSeen = firstSeen
		stat.LastSeen = lastSeen
		stat.TotalUptime = lastSeen.Sub(firstSeen)

		// 设置其他字段
		if len(processRecords) > 0 {
			firstRecord := processRecords[0]
			stat.Category = firstRecord.Category
			stat.Command = firstRecord.Command
			stat.WorkingDir = firstRecord.WorkingDir
			stat.PIDs = []int32{firstRecord.PID}
			stat.ProcessStartTime = time.UnixMilli(firstRecord.CreateTime)
			stat.TotalCPUTime = time.Duration(firstRecord.CPUTime * float64(time.Second))
			stat.AvgCPUTime = firstRecord.CPUTime
		}

		stats = append(stats, stat)
	}

	return stats
}

// CleanOldData 清理旧数据
func (s *SQLiteStorage) CleanOldData(keepDays int) error {
	cutoff := time.Now().AddDate(0, 0, -keepDays)

	// 删除旧记录
	result, err := s.db.Exec("DELETE FROM resource_records WHERE timestamp < ?", cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete old records: %w", err)
	}

	// 获取删除的记录数
	deletedRows, _ := result.RowsAffected()
	log.Printf("Cleaned up %d old records (older than %d days)", deletedRows, keepDays)

	// 清理数据库
	if _, err := s.db.Exec("VACUUM"); err != nil {
		log.Printf("Warning: failed to vacuum database: %v", err)
	}

	return nil
}

// GetStorageInfo 获取存储信息
func (s *SQLiteStorage) GetStorageInfo() StorageInfo {
	info := StorageInfo{
		Type:     "sqlite",
		FilePath: s.sqlitePath,
	}

	// 获取记录总数
	if count, err := s.GetRecordCount(); err == nil {
		info.TotalRecords = count
	}

	// 获取文件大小
	if stat, err := os.Stat(s.sqlitePath); err == nil {
		info.TotalSize = stat.Size()
		info.LastModified = stat.ModTime()
	}

	// 获取最早和最新记录时间
	s.db.QueryRow("SELECT MIN(timestamp) FROM resource_records").Scan(&info.OldestRecord)
	s.db.QueryRow("SELECT MAX(timestamp) FROM resource_records").Scan(&info.NewestRecord)

	return info
}

// Close 关闭存储
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}