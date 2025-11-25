# Logging Package

The `logging` package provides structured logging for the HelixCode platform.

## Overview

This package handles:
- Structured logging with levels
- Multiple output formats (JSON, text)
- Log rotation and management
- Context-aware logging
- Performance-optimized logging

## Key Types

### Logger

```go
type Logger struct {
    level   Level
    format  Format
    output  io.Writer
    context map[string]interface{}
}
```

### Level

```go
type Level int

const (
    DEBUG Level = iota
    INFO
    WARN
    ERROR
    FATAL
)
```

### Format

```go
type Format string

const (
    FormatJSON Format = "json"
    FormatText Format = "text"
)
```

## Usage

### Creating a Logger

```go
import "dev.helix.code/internal/logging"

// Create with default settings
logger := logging.NewLogger(logging.INFO)

// Create with options
logger := logging.NewLoggerWithOptions(&logging.Options{
    Level:  logging.DEBUG,
    Format: logging.FormatJSON,
    Output: os.Stdout,
})
```

### Basic Logging

```go
logger.Debug("Debug message: %s", value)
logger.Info("Info message: %s", value)
logger.Warn("Warning message: %s", value)
logger.Error("Error message: %s", value)
logger.Fatal("Fatal message: %s", value)  // Exits program
```

### Structured Logging

```go
// With fields
logger.WithFields(map[string]interface{}{
    "user_id":    userID,
    "request_id": requestID,
}).Info("Processing request")

// With single field
logger.WithField("duration", duration).Info("Request completed")

// Chained fields
logger.
    WithField("method", "POST").
    WithField("path", "/api/v1/users").
    Info("Incoming request")
```

### Context-Aware Logging

```go
// Create logger with context
ctxLogger := logger.WithContext(ctx)

// Context is automatically included in logs
ctxLogger.Info("Processing...")  // Includes trace_id, request_id if in context

// Add request context
logger := logging.FromContext(ctx)
logger.Info("Handler started")
```

### JSON Output

```json
{
    "level": "info",
    "timestamp": "2024-01-15T10:30:00Z",
    "message": "Request completed",
    "user_id": "user-123",
    "request_id": "req-456",
    "duration_ms": 45
}
```

### Log Rotation

```go
// Configure file output with rotation
rotator := &logging.FileRotator{
    Filename:   "/var/log/helixcode/app.log",
    MaxSize:    100, // MB
    MaxBackups: 3,
    MaxAge:     28,  // days
    Compress:   true,
}

logger := logging.NewLoggerWithOptions(&logging.Options{
    Output: rotator,
})
```

## Configuration

```yaml
logging:
  level: "info"      # debug, info, warn, error
  format: "json"     # json, text
  output: "stdout"   # stdout, stderr, file
  file:
    path: "/var/log/helixcode/app.log"
    max_size: 100
    max_backups: 3
    max_age: 28
    compress: true
```

## Global Logger

```go
// Set global logger
logging.SetDefault(logger)

// Use global logger
logging.Info("Using global logger")
logging.Error("Error: %v", err)
```

## Performance

```go
// Check level before expensive operations
if logger.IsDebugEnabled() {
    logger.Debug("Expensive debug info: %s", expensiveComputation())
}

// Use deferred logging for timing
defer logger.WithField("duration", time.Since(start)).Info("Operation completed")
```

## Testing

```bash
go test -v ./internal/logging/...
```

## Notes

- Use structured logging for machine parsing
- Include context (request_id, user_id) for tracing
- Use appropriate log levels
- Configure rotation for production logs
