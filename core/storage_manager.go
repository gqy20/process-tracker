package core

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo represents information about a log file
type FileInfo struct {
	Path         string
	Size         int64
	ModTime      time.Time
	IsCompressed bool
	Index        int
}

// StorageManager handles file rotation, compression, and cleanup
// Uses simplified configuration with smart defaults
type StorageManager struct {
	basePath     string
	baseName     string
	config       StorageConfig
	currentFile  *os.File
	currentSize  int64
	currentIndex int
}

// NewStorageManager creates a new storage manager instance
func NewStorageManager(basePath string, config StorageConfig) *StorageManager {
	return &StorageManager{
		basePath:     basePath,
		baseName:     filepath.Base(basePath),
		config:       config,
		currentFile:  nil,
		currentSize:  0,
		currentIndex: 0,
	}
}

// getMaxFileSizeMB returns the maximum file size for rotation (derived from total size)
func (sm *StorageManager) getMaxFileSizeMB() int {
	// Use 1/5 of total size for each file, allowing for 5 rotation files
	return sm.config.MaxSizeMB / 5
}

// getMaxFiles returns the maximum number of files to keep
func (sm *StorageManager) getMaxFiles() int {
	return 5 // Fixed at 5 for simplicity
}

// shouldCompress returns whether a file should be compressed based on age
func (sm *StorageManager) shouldCompress(modTime time.Time) bool {
	// Compress files older than 1 day
	return time.Since(modTime) > 24*time.Hour
}

// shouldCleanup returns whether a file should be deleted based on age and config
func (sm *StorageManager) shouldCleanup(modTime time.Time) bool {
	if sm.config.KeepDays == 0 {
		return false // 0 means keep forever
	}
	return time.Since(modTime) > time.Duration(sm.config.KeepDays)*24*time.Hour
}

// Initialize sets up the storage manager and checks existing files
func (sm *StorageManager) Initialize() error {
	// Check for existing files and determine current index
	files, err := sm.getLogFiles()
	if err != nil {
		return fmt.Errorf("failed to get log files: %v", err)
	}

	// Clean up old files (always enabled with smart defaults)
	if err := sm.cleanupOldFiles(files); err != nil {
		return fmt.Errorf("failed to cleanup old files: %v", err)
	}

	// Compress old files (automatic based on age)
	if err := sm.compressOldFiles(files); err != nil {
		return fmt.Errorf("failed to compress old files: %v", err)
	}

	// Determine current file index
	sm.currentIndex = sm.determineCurrentIndex(files)

	// Open or create current file
	return sm.openCurrentFile()
}

// WriteRecord writes a record to the current file, handling rotation if needed
func (sm *StorageManager) WriteRecord(record string) error {
	// Check if we need to rotate
	if sm.shouldRotate() {
		if err := sm.rotate(); err != nil {
			return fmt.Errorf("failed to rotate log file: %v", err)
		}
	}

	// Write the record
	if sm.currentFile == nil {
		if err := sm.openCurrentFile(); err != nil {
			return err
		}
	}

	n, err := sm.currentFile.WriteString(record + "\n")
	if err != nil {
		return err
	}

	sm.currentSize += int64(n)
	return nil
}

// Close closes the current file
func (sm *StorageManager) Close() error {
	if sm.currentFile != nil {
		return sm.currentFile.Close()
	}
	return nil
}

// shouldRotate checks if the current file needs rotation
func (sm *StorageManager) shouldRotate() bool {
	maxFileSizeMB := sm.getMaxFileSizeMB()
	if maxFileSizeMB <= 0 {
		return false
	}
	maxBytes := int64(maxFileSizeMB) * 1024 * 1024
	return sm.currentSize >= maxBytes
}

// rotate performs file rotation
func (sm *StorageManager) rotate() error {
	// Close current file
	if sm.currentFile != nil {
		sm.currentFile.Close()
		sm.currentFile = nil
	}

	// Find the next available index
	sm.currentIndex++

	// Get current list of files
	files, err := sm.getLogFiles()
	if err != nil {
		return err
	}

	// Compress old files (automatic based on age)
	if err := sm.compressOldFiles(files); err != nil {
		// Log error but don't fail rotation
		fmt.Printf("Warning: failed to compress old files: %v\n", err)
	}

	// Cleanup old files (automatic based on retention policy)
	if err := sm.cleanupOldFiles(files); err != nil {
		// Log error but don't fail rotation
		fmt.Printf("Warning: failed to cleanup old files: %v\n", err)
	}

	// Simple cleanup: remove oldest if we have too many files
	maxFiles := sm.getMaxFiles()
	// Refresh file list after compression
	files, err = sm.getLogFiles()
	if err != nil {
		return err
	}

	// If we have too many files, remove the oldest one
	if len(files) >= maxFiles {
		oldest := files[0]
		for _, f := range files {
			if f.ModTime.Before(oldest.ModTime) {
				oldest = f
			}
		}
		os.Remove(oldest.Path)
	}

	// Open new current file
	return sm.openCurrentFile()
}

// openCurrentFile opens the current log file
func (sm *StorageManager) openCurrentFile() error {
	if sm.currentIndex == 0 {
		// Always try to create the main file first
		file, err := os.OpenFile(sm.basePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			sm.currentFile = file
			// Get current size
			if info, err := file.Stat(); err == nil {
				sm.currentSize = info.Size()
			}
			return nil
		}
		// If main file fails, fall back to indexed file
		sm.currentIndex = 1
	}

	filename := sm.getIndexedFilename(sm.currentIndex, false)
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	sm.currentFile = file
	sm.currentSize = 0 // Start fresh for new indexed files
	return nil
}

// GetLogFiles gets all log files (including compressed ones) - public method for testing
func (sm *StorageManager) GetLogFiles() ([]FileInfo, error) {
	return sm.getLogFiles()
}

// getLogFiles gets all log files (including compressed ones)
func (sm *StorageManager) getLogFiles() ([]FileInfo, error) {
	var files []FileInfo

	dir := filepath.Dir(sm.basePath)
	base := sm.baseName

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check if this is a log file for our base
		if name == base {
			files = append(files, FileInfo{
				Path:         filepath.Join(dir, name),
				Size:         info.Size(),
				ModTime:      info.ModTime(),
				IsCompressed: false,
				Index:        0,
			})
		} else if strings.HasPrefix(name, base+".") {
			// Check for indexed files (base.1, base.2.gz, etc.)
			suffix := strings.TrimPrefix(name, base+".")
			if suffix == "" {
				continue
			}

			isCompressed := strings.HasSuffix(suffix, ".gz")
			if isCompressed {
				suffix = strings.TrimSuffix(suffix, ".gz")
			}

			var index int
			if _, err := fmt.Sscanf(suffix, "%d", &index); err == nil && index > 0 {
				files = append(files, FileInfo{
					Path:         filepath.Join(dir, name),
					Size:         info.Size(),
					ModTime:      info.ModTime(),
					IsCompressed: isCompressed,
					Index:        index,
				})
			}
		}
	}

	return files, nil
}

// determineCurrentIndex determines the current file index
// Returns the highest index among all files (compressed or not) to avoid conflicts
func (sm *StorageManager) determineCurrentIndex(files []FileInfo) int {
	maxIndex := 0
	for _, file := range files {
		if file.Index > maxIndex {
			maxIndex = file.Index
		}
	}
	return maxIndex
}

// getIndexedFilename generates a filename for a given index
func (sm *StorageManager) getIndexedFilename(index int, compressed bool) string {
	dir := filepath.Dir(sm.basePath)
	base := sm.baseName

	if index == 0 {
		return filepath.Join(dir, base)
	}

	if compressed {
		return filepath.Join(dir, fmt.Sprintf("%s.%d.gz", base, index))
	}
	return filepath.Join(dir, fmt.Sprintf("%s.%d", base, index))
}

// cleanupOldFiles removes files based on retention policy
func (sm *StorageManager) cleanupOldFiles(files []FileInfo) error {
	for _, file := range files {
		if sm.shouldCleanup(file.ModTime) {
			os.Remove(file.Path)
		}
	}
	return nil
}

// compressOldFiles compresses files based on age
func (sm *StorageManager) compressOldFiles(files []FileInfo) error {
	for _, file := range files {
		if file.IsCompressed {
			continue
		}

		// Don't compress current file (index 0), only rotated files
		if file.Index > 0 && sm.shouldCompress(file.ModTime) {
			if err := sm.compressFile(file.Path); err != nil {
				// Log error but continue with other files
				fmt.Printf("Warning: failed to compress %s: %v\n", file.Path, err)
			}
		}
	}
	return nil
}

// compressFile compresses a single file
func (sm *StorageManager) compressFile(filepath string) error {
	// Open source file
	src, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer src.Close()

	// Create destination file
	dstPath := filepath + ".gz"
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(dst)
	defer gzWriter.Close()

	// Copy data
	if _, err := io.Copy(gzWriter, src); err != nil {
		return err
	}

	// Close gzip writer to flush all data
	gzWriter.Close()

	// Remove original file
	return os.Remove(filepath)
}
