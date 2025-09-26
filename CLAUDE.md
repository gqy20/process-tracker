# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Process Tracker is a Go-based system monitoring tool that tracks process usage statistics on Linux servers. It provides detailed insights into CPU, memory, disk I/O, and network usage for all running processes, with configurable reporting and storage management.

## Build and Development Commands

### Building the Application
```bash
# Build for current platform
go build -o process-tracker main.go

# Build with version (for releases)
go build -ldflags="-X main.Version=0.3.7" -o process-tracker main.go

# Cross-platform build script
./build.sh
```

### Running the Application
```bash
# Start monitoring with default settings
./process-tracker start

# Start with custom configuration
./process-tracker start --config /path/to/config.yaml --data-file /path/to/data.log

# View statistics
./process-tracker today    # Today's usage
./process-tracker week     # Weekly summary  
./process-tracker month    # Monthly trends
./process-tracker details  # Detailed statistics

# Export data
./process-tracker export

# Cleanup old data
./process-tracker cleanup
```

### Testing
```bash
# Run all tests
go test ./...

# Test specific package
go test ./core

# Test with coverage
go test -cover ./...
```

## Architecture Overview

### Core Components

**main.go** (728 lines) - CLI Interface and Application Orchestration
- Command line argument parsing and routing
- Application lifecycle management
- Signal handling and graceful shutdown
- Integration with core monitoring functionality

**core/app.go** (628 lines) - Monitoring Engine
- Process data collection and buffering
- Resource record management with write-back caching
- Integration with storage management system
- Network statistics estimation based on connection patterns

**core/types.go** (678 lines) - Data Structures and Configuration
- `ResourceRecord`: Primary data structure for process metrics
- `Config`: Application configuration with YAML support
- `StorageConfig`: File rotation and compression settings
- Data format detection and validation logic

**core/storage_manager.go** (337 lines) - Storage Management System
- File rotation when exceeding size limits
- Automatic compression of old files (.gz format)
- Configurable retention policies and cleanup
- Backward compatibility with existing file formats

### Key Design Patterns

**Dependency Injection**: The main application creates and injects dependencies into the core App structure, promoting testability and modularity.

**Buffered Writing**: Uses in-memory buffering (100 records) with periodic flushing to optimize I/O performance and reduce disk writes.

**Storage Abstraction**: Supports both traditional file writing and new storage management through a unified interface, maintaining backward compatibility.

**Configuration Layer**: YAML-based configuration with sensible defaults, supporting hot-reloadable settings for storage management.

## Data Storage Architecture

### File Format
- CSV-style format with comma-separated values
- Timestamp, process name, CPU%, memory MB, threads, disk read/write MB, network sent/received KB, active status, command, working directory, category
- Supports multiple data format versions for backward compatibility

### Storage Management (v0.3.0+)
The application implements intelligent storage management to prevent unlimited log growth:

1. **File Rotation**: Automatically creates new files when size exceeds `max_file_size_mb`
2. **Compression**: Files older than `compress_after_days` are automatically compressed to .gz format
3. **Cleanup**: Files older than `cleanup_after_days` are automatically removed
4. **Retention**: Maintains maximum of `max_files` to control storage usage

### Configuration Example
```yaml
storage:
  max_file_size_mb: 100     # Rotate files at 100MB
  max_files: 10            # Keep 10 files maximum
  compress_after_days: 3   # Compress after 3 days
  cleanup_after_days: 30   # Delete after 30 days
  auto_cleanup: true        # Enable automatic management
```

## Process Monitoring Strategy

### Data Collection
- Uses `gopsutil/v3` for cross-platform process information
- Collects CPU, memory, thread count, disk I/O, and network statistics
- Implements smart network usage estimation based on connection patterns
- Filters out system processes and focuses on user applications

### Categorization System
- Built-in rules for categorizing processes (development, browsers, system tools)
- Smart category detection based on command paths and process names
- Configurable category mappings through YAML configuration

### Performance Optimizations
- Buffered writing reduces I/O operations from every 5 seconds to batch operations
- Memory-efficient data structures minimize allocation overhead
- Configurable sampling intervals balance detail with performance impact

## Configuration Management

### Default Configuration Path
- `~/.process-tracker.yaml` - User-level configuration
- Supports environment variable expansion for paths

### Key Configuration Sections
- `statistics_granularity`: Controls detail level (simple/detailed/full)
- `show_commands/show_working_dirs`: Display options for output formatting
- `storage`: File management and retention policies
- `use_smart_categories`: Enable intelligent process categorization

## Development Philosophy

Following Dave Cheney's Go programming principles:
- **Simple over complex**: Choose the simplest solution that works
- **Readable is correct**: Code should be clear and self-documenting
- **Errors are values**: Handle every error explicitly
- **Less is more**: Minimize code to reduce bug surface area
- **Small interfaces**: Keep interfaces minimal and focused

The codebase emphasizes maintainability through clear separation of concerns, comprehensive error handling, and backward compatibility.