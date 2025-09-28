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
		basePath:    basePath,
		baseName:    filepath.Base(basePath),
		config:      config,
		currentFile: nil,
		currentSize: 0,
		currentIndex: 0,
	}
}

// Initialize sets up the storage manager and checks existing files
func (sm *StorageManager) Initialize() error {
	// Check for existing files and determine current index
	files, err := sm.getLogFiles()
	if err != nil {
		return fmt.Errorf("failed to get log files: %v", err)
	}

	// Clean up old files if auto cleanup is enabled
	if sm.config.AutoCleanup {
		if err := sm.cleanupOldFiles(files); err != nil {
			return fmt.Errorf("failed to cleanup old files: %v", err)
		}
	}

	// Compress old files if needed
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
	if sm.config.MaxFileSizeMB <= 0 {
		return false
	}
	maxBytes := int64(sm.config.MaxFileSizeMB) * 1024 * 1024
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
	
	// Simple cleanup: remove oldest if we have too many files (including the one we're about to create)
	if sm.config.MaxFiles > 0 {
		files, err := sm.getLogFiles()
		if err != nil {
			return err
		}
		
		// If we have too many files, remove the oldest one
		if len(files) >= sm.config.MaxFiles {
			oldest := files[0]
			for _, f := range files {
				if f.ModTime.Before(oldest.ModTime) {
					oldest = f
				}
			}
			os.Remove(oldest.Path)
		}
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
func (sm *StorageManager) determineCurrentIndex(files []FileInfo) int {
	maxIndex := 0
	for _, file := range files {
		if file.Index > maxIndex && !file.IsCompressed {
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

// cleanupOldFiles removes files older than CleanupAfterDays
func (sm *StorageManager) cleanupOldFiles(files []FileInfo) error {
	if sm.config.CleanupAfterDays <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -sm.config.CleanupAfterDays)
	
	for _, file := range files {
		if file.ModTime.Before(cutoff) {
			os.Remove(file.Path)
		}
	}
	
	return nil
}

// compressOldFiles compresses files older than CompressAfterDays
func (sm *StorageManager) compressOldFiles(files []FileInfo) error {
	if sm.config.CompressAfterDays <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -sm.config.CompressAfterDays)
	
	for _, file := range files {
		if file.IsCompressed {
			continue
		}
		
		if file.ModTime.Before(cutoff) && file.Index > 0 { // Don't compress current file (index 0)
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