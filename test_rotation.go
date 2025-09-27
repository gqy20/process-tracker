package main

import (
	"fmt"
	"log"
	"time"

	"github.com/yourusername/process-tracker/core"
)

func main() {
	// Test storage rotation with large data
	config := core.GetDefaultConfig()
	config.Storage.MaxFileSizeMB = 1 // 1MB limit for quick testing
	config.Storage.MaxFiles = 3
	config.Storage.CompressAfterDays = 0
	config.Storage.CleanupAfterDays = 1
	config.Storage.AutoCleanup = true

	// Use storage manager directly
	storageMgr := core.NewManager("test-rotation.log", 10, true, config.Storage)
	if err := storageMgr.Initialize(); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storageMgr.Close()

	// Generate large amount of data to trigger rotation
	fmt.Println("Generating test data to trigger rotation...")
	
	for i := 0; i < 50000; i++ {
		record := core.ResourceRecord{
			Timestamp:   time.Now(),
			Name:        fmt.Sprintf("test-process-%d", i%100),
			CPUPercent:  float64(i % 100),
			MemoryMB:    float64(i % 1000),
			Threads:     int32(i % 50),
			DiskReadMB:  float64(i % 10),
			DiskWriteMB: float64(i % 5),
			NetSentKB:   float64(i % 100),
			NetRecvKB:   float64(i % 200),
			IsActive:    i%2 == 0,
			Command:     fmt.Sprintf("/usr/bin/test-process-%d", i%100),
			WorkingDir:  "/home/test",
			Category:    "test",
		}
		
		if err := storageMgr.SaveRecord(record); err != nil {
			log.Printf("Error saving record %d: %v", i, err)
		}
		
		if i%1000 == 0 {
			fmt.Printf("Processed %d records\n", i)
		}
	}
	
	fmt.Println("Test completed. Check for log rotation.")
}