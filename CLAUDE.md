# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Process Tracker is a Go-based system monitoring tool that tracks process usage statistics on Linux servers. It provides detailed insights into CPU, memory, disk I/O, and network usage for all running processes, with configurable reporting and storage management. The tool features a modern web interface, task management system, and support for both CSV and SQLite storage backends.

## Build and Development Commands

### Building the Application
```bash
# Simple build for current platform
go build -o process-tracker main.go

# Build with version information
go build -ldflags="-X main.Version=0.4.1" -o process-tracker main.go

# Cross-platform build script (static compilation)
./build.sh

# Run tests
go test ./...

# Test specific package
go test ./core

# Test with coverage
go test -cover ./...
```

### Core Commands
```bash
# Process management (5 core commands following simple design)
./process-tracker start [-i N] [-w] [-p PORT]  # Start monitoring with optional web
./process-tracker stop                         # Stop monitoring
./process-tracker status                       # Check running status
./process-tracker stats [-d|-w|-m]             # View statistics (day/week/month)
./process-tracker web [-p PORT] [-h HOST]      # Start web interface only

# Advanced features
./process-tracker migrate-to-sqlite [--sqlite-path PATH]  # Migrate CSV to SQLite
```

### Web Interface
- Default URL: http://localhost:8080
- Real-time process monitoring dashboard
- Historical data visualization
- Task management interface

## Architecture Overview

### Core Components

**main.go** - CLI Interface and Application Orchestration
- 5-command interface following simplicity principles (start/stop/status/stats/web)
- Global options handling (port, interval, format, filter, sort, limit, offset)
- Application lifecycle management with graceful shutdown
- Integration with core monitoring and web functionality

**core/app.go** - Central Monitoring Engine
- Process data collection with configurable intervals
- Task management integration through TaskManager
- Storage abstraction layer (CSV/SQLite support)
- Real-time monitoring with buffering optimization

**core/task_manager.go** - Task Management System
- Process tracking and task lifecycle management
- PID-to-Task mapping for fast lookups
- Process tree management with parent-child relationships
- Event-driven architecture with task events

**core/types.go** - Data Structures and Configuration
- `ProcessInfo`: Core process monitoring data structure
- `Config`: YAML-based configuration with sensible defaults
- `Task` and `TaskConfig`: Task management data structures
- Support for multiple storage backends and notifiers

**api/v1/** - REST API Layer
- Router with comprehensive middleware (CORS, security, validation)
- TaskHandler for task management endpoints
- ProcessHandler for process monitoring endpoints
- StatsHandler for statistics and analytics
- JSON-based REST API with proper error handling

**core/storage_*.go** - Storage Abstraction
- `storage_interface.go`: Common storage interface
- `storage.go`: CSV file storage implementation
- `storage_sqlite.go`: SQLite database storage
- `storage_manager.go`: File rotation and lifecycle management

### Modern Architecture Features

**Task Management System**:
- Persistent task tracking with process tree relationships
- Event-driven communication between components
- Daemon management for background operations

**Multi-Backend Storage**:
- CSV storage for simplicity and compatibility
- SQLite storage for performance and complex queries
- Seamless migration between storage types

**Web Interface Integration**:
- Gin-based REST API with comprehensive middleware
- Real-time process monitoring dashboard
- Task management through web interface
- Statistics visualization and export capabilities

**Notification System**:
- Multiple notifier support (DingTalk, WeChat, Feishu, Webhook)
- Configurable alerting rules and thresholds
- Docker container monitoring integration

## Data Storage Architecture

### Dual Storage Backend Support

**CSV Storage (Default)**:
- File path: `~/.process-tracker/process-tracker.log`
- Simple, human-readable format with comma-separated values
- Fields: timestamp, process name, CPU%, memory MB, threads, disk I/O, network I/O, status, command, working directory, category
- Backward compatibility with existing data formats

**SQLite Storage (Recommended for production)**:
- Database path: `~/.process-tracker/process-tracker.db`
- High performance with complex query support
- WAL mode enabled for better concurrency
- Configurable cache size for performance tuning

### Storage Configuration
```yaml
storage:
  type: "sqlite"              # Storage type: csv/sqlite
  sqlite_path: "~/.process-tracker/process-tracker.db"
  sqlite_wal: true           # Enable WAL mode
  sqlite_cache_size: 2000    # Cache size in pages
  max_file_size_mb: 50       # Max file size (CSV only)
  keep_days: 7               # Data retention period
```

### Migration and Management
```bash
# Migrate from CSV to SQLite (preserves existing data)
./process-tracker migrate-to-sqlite

# Custom migration path
./process-tracker migrate-to-sqlite --sqlite-path /custom/path.db
```

### File Rotation (CSV backend)
- Automatic rotation when exceeding `max_file_size_mb`
- Configurable retention policies and cleanup
- Backward compatibility with historical data formats

## Process Monitoring Strategy

### Data Collection and Monitoring
- Uses `gopsutil/v3` for cross-platform process information gathering
- Comprehensive metrics: CPU%, memory MB, threads, disk I/O, network I/O, process uptime
- Configurable monitoring intervals (default: 5 seconds)
- Smart filtering focusing on user applications vs system processes
- Docker container monitoring with integration support

### Task Management Integration
- Process tree tracking with parent-child relationships
- Event-driven task lifecycle management
- Persistent task storage with state recovery
- Background daemon management for long-running tasks

### Performance Optimizations
- In-memory buffering to reduce I/O overhead
- Configurable sampling intervals for performance vs detail balance
- SQLite WAL mode for better concurrent access
- Efficient PID-to-Task mapping for fast lookups

## Configuration Management

### Default Configuration Structure
```yaml
# Primary configuration location
~/.process-tracker/config.yaml

# Data files (location varies by storage type)
~/.process-tracker/process-tracker.log     # CSV storage
~/.process-tracker/process-tracker.db     # SQLite storage
```

### Key Configuration Sections
```yaml
# Core monitoring settings
monitoring:
  interval: "5s"              # Monitoring frequency
  enable_smart_categories: true  # Intelligent categorization

# Storage backend configuration
storage:
  type: "sqlite"              # csv or sqlite
  sqlite_path: "~/.process-tracker/process-tracker.db"
  max_file_size_mb: 50        # CSV rotation size
  keep_days: 7                # Data retention

# Web interface
web:
  enabled: true               # Enable web dashboard
  host: "0.0.0.0"            # Bind address
  port: "8080"               # Port number

# Alert and notification system
alerts:
  enabled: false              # Alert system toggle
  cpu_threshold: 80.0        # CPU usage alert threshold
  memory_threshold: 80.0     # Memory usage alert threshold

notifiers:
  dingtalk:
    enabled: false
    webhook_url: ""
  wechat:
    enabled: false
    webhook_url: ""
```

## API Architecture

### REST API Endpoints
- **GET /api/v1/processes** - List current processes with filtering
- **GET /api/v1/stats** - Retrieve statistical summaries
- **GET /api/v1/tasks** - List managed tasks
- **POST /api/v1/tasks** - Create new tasks
- **GET /api/v1/tasks/:id** - Get task details
- **DELETE /api/v1/tasks/:id** - Stop and remove tasks

### Middleware Stack
- Request ID generation and tracing
- CORS handling for cross-origin requests
- Security headers (X-Frame-Options, CSP, etc.)
- Request/response logging
- Error handling and recovery
- API version validation

## Development Philosophy

Following Dave Cheney's Go programming principles with modern additions:

**Simplicity First**:
- 5-command CLI interface (start/stop/status/stats/web)
- Minimal configuration with intelligent defaults
- Clear separation of concerns

**Reliability and Performance**:
- Comprehensive error handling at all levels
- Buffered operations for I/O efficiency
- Static compilation for maximum portability
- Event-driven architecture for scalability

**Maintainability**:
- Interface-based design for testability
- Clear module boundaries (core/api/web)
- Comprehensive test coverage
- Backward compatibility preservation

**Modern Features**:
- REST API with comprehensive middleware
- Web interface for intuitive management
- Multi-backend storage support
- Docker and container integration
- Real-time monitoring capabilities