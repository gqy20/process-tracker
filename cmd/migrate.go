package cmd

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/yourusername/process-tracker/core"
	_ "github.com/mattn/go-sqlite3"
)

// Migration 数据迁移管理器
type Migration struct {
	csvDataFile string
	sqlitePath  string
	backup      bool
	verbose     bool
}

// NewMigration 创建新的迁移实例
func NewMigration(csvDataFile, sqlitePath string, backup, verbose bool) *Migration {
	if sqlitePath == "" {
		dir := filepath.Dir(csvDataFile)
		base := filepath.Base(csvDataFile)
		name := base[:len(base)-len(filepath.Ext(base))]
		sqlitePath = filepath.Join(dir, name+".db")
	}

	return &Migration{
		csvDataFile: csvDataFile,
		sqlitePath:  sqlitePath,
		backup:      backup,
		verbose:     verbose,
	}
}

// Run 执行数据迁移
func (m *Migration) Run() error {
	log.Printf("开始数据迁移...")
	log.Printf("CSV文件: %s", m.csvDataFile)
	log.Printf("SQLite文件: %s", m.sqlitePath)

	// 检查CSV文件是否存在
	if _, err := os.Stat(m.csvDataFile); os.IsNotExist(err) {
		return fmt.Errorf("CSV数据文件不存在: %s", m.csvDataFile)
	}

	// 检查SQLite文件是否已存在
	if _, err := os.Stat(m.sqlitePath); err == nil {
		return fmt.Errorf("SQLite数据库已存在: %s。如需重新迁移，请删除现有文件", m.sqlitePath)
	}

	// 备份CSV文件
	if m.backup {
		backupPath := m.csvDataFile + ".backup." + time.Now().Format("20060102-150405")
		if err := m.copyFile(m.csvDataFile, backupPath); err != nil {
			log.Printf("警告: 备份CSV文件失败: %v", err)
		} else {
			log.Printf("CSV文件已备份到: %s", backupPath)
		}
	}

	// 读取CSV数据
	records, err := m.readCSVRecords()
	if err != nil {
		return fmt.Errorf("读取CSV数据失败: %w", err)
	}

	log.Printf("读取到 %d 条记录", len(records))
	if len(records) == 0 {
		log.Printf("CSV文件为空，迁移完成")
		return nil
	}

	// 创建SQLite数据库
	if err := m.createSQLiteDatabase(records); err != nil {
		return fmt.Errorf("创建SQLite数据库失败: %w", err)
	}

	log.Printf("数据迁移完成: %d 条记录已迁移到 %s", len(records), m.sqlitePath)
	return nil
}

// readCSVRecords 读取CSV记录
func (m *Migration) readCSVRecords() ([]core.ResourceRecord, error) {
	// 使用现有的CSV存储管理器读取数据
	csvStorage := core.NewManager(m.csvDataFile, 0, false, core.StorageConfig{})
	if err := csvStorage.Initialize(); err != nil {
		return nil, fmt.Errorf("初始化CSV存储失败: %w", err)
	}
	defer csvStorage.Close()

	records, err := csvStorage.ReadRecords(m.csvDataFile)
	if err != nil {
		return nil, fmt.Errorf("读取CSV记录失败: %w", err)
	}

	return records, nil
}

// createSQLiteDatabase 创建SQLite数据库并导入数据
func (m *Migration) createSQLiteDatabase(records []core.ResourceRecord) error {
	// 确保目录存在
	dir := filepath.Dir(m.sqlitePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 打开SQLite数据库
	db, err := sql.Open("sqlite3", m.sqlitePath)
	if err != nil {
		return fmt.Errorf("打开SQLite数据库失败: %w", err)
	}
	defer db.Close()

	// 配置SQLite
	if err := m.configureSQLite(db); err != nil {
		return fmt.Errorf("配置SQLite失败: %w", err)
	}

	// 创建表
	if err := m.createTables(db, records); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	// 批量插入数据
	if err := m.insertRecords(db, records); err != nil {
		return fmt.Errorf("插入记录失败: %w", err)
	}

	return nil
}

// configureSQLite 配置SQLite
func (m *Migration) configureSQLite(db *sql.DB) error {
	// 启用外键约束
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	// 启用WAL模式
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return err
	}

	// 配置同步模式
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return err
	}

	return nil
}


// createTables 创建表结构
func (m *Migration) createTables(db *sql.DB, records []core.ResourceRecord) error {
	// 创建资源记录表
	createRecordsSQL := `
	CREATE TABLE resource_records (
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

	if _, err := db.Exec(createRecordsSQL); err != nil {
		return fmt.Errorf("创建resource_records表失败: %w", err)
	}

	// 创建索引
	indexes := []string{
		"CREATE INDEX idx_resource_records_timestamp ON resource_records(timestamp)",
		"CREATE INDEX idx_resource_records_name ON resource_records(name)",
		"CREATE INDEX idx_resource_records_pid ON resource_records(pid)",
		"CREATE INDEX idx_resource_records_created_at ON resource_records(created_at)",
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}

	// 创建元数据表
	createMetaSQL := `
	CREATE TABLE storage_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(createMetaSQL); err != nil {
		return fmt.Errorf("创建storage_meta表失败: %w", err)
	}

	// 设置元数据
	metaInserts := []struct {
		key, value string
	}{
		{"storage_type", "sqlite"},
		{"migration_date", time.Now().Format(time.RFC3339)},
		{"migrated_from", m.csvDataFile},
		{"record_count", fmt.Sprintf("%d", len(records))},
	}

	for _, meta := range metaInserts {
		if _, err := db.Exec("INSERT INTO storage_meta (key, value) VALUES (?, ?)", meta.key, meta.value); err != nil {
			log.Printf("警告: 插入元数据失败 (%s): %v", meta.key, err)
		}
	}

	return nil
}

// insertRecords 批量插入记录
func (m *Migration) insertRecords(db *sql.DB, records []core.ResourceRecord) error {
	if len(records) == 0 {
		return nil
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
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
		return fmt.Errorf("准备插入语句失败: %w", err)
	}
	defer stmt.Close()

	// 批量插入
	batchSize := 1000
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		for _, record := range batch {
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
				return fmt.Errorf("插入记录失败: %w", err)
			}
		}

		if m.verbose {
			log.Printf("已插入 %d/%d 条记录", end, len(records))
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// copyFile 复制文件
func (m *Migration) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	buf := make([]byte, 64*1024) // 64KB buffer
	for {
		n, err := sourceFile.Read(buf)
		if err != nil && err.Error() != "EOF" {
			return err
		}
		if n == 0 {
			break
		}

		if _, err := destFile.Write(buf[:n]); err != nil {
			return err
		}
	}

	return nil
}

// VerifyMigration 验证迁移结果
func (m *Migration) VerifyMigration() error {
	log.Printf("验证迁移结果...")

	// 打开SQLite数据库
	db, err := sql.Open("sqlite3", m.sqlitePath)
	if err != nil {
		return fmt.Errorf("打开SQLite数据库失败: %w", err)
	}
	defer db.Close()

	// 检查记录数
	var sqliteCount int
	err = db.QueryRow("SELECT COUNT(*) FROM resource_records").Scan(&sqliteCount)
	if err != nil {
		return fmt.Errorf("查询SQLite记录数失败: %w", err)
	}

	// 读取CSV记录数
	csvStorage := core.NewManager(m.csvDataFile, 0, false, core.StorageConfig{})
	if err := csvStorage.Initialize(); err != nil {
		return fmt.Errorf("初始化CSV存储失败: %w", err)
	}
	defer csvStorage.Close()

	csvRecords, err := csvStorage.ReadRecords(m.csvDataFile)
	if err != nil {
		return fmt.Errorf("读取CSV记录失败: %w", err)
	}

	csvCount := len(csvRecords)

	log.Printf("CSV记录数: %d", csvCount)
	log.Printf("SQLite记录数: %d", sqliteCount)

	if csvCount != sqliteCount {
		return fmt.Errorf("记录数不匹配: CSV=%d, SQLite=%d", csvCount, sqliteCount)
	}

	// 检查时间范围
	var minTime, maxTime time.Time
	db.QueryRow("SELECT MIN(timestamp) FROM resource_records").Scan(&minTime)
	db.QueryRow("SELECT MAX(timestamp) FROM resource_records").Scan(&maxTime)

	log.Printf("数据时间范围: %s 到 %s", minTime.Format("2006-01-02 15:04:05"), maxTime.Format("2006-01-02 15:04:05"))

	// 检查文件大小
	if stat, err := os.Stat(m.sqlitePath); err == nil {
		log.Printf("SQLite数据库大小: %.2f MB", float64(stat.Size())/1024/1024)
	}

	log.Printf("✅ 迁移验证通过")
	return nil
}