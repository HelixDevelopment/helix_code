# Discovery Package

The `discovery` package provides service and resource discovery for the HelixCode platform.

## Overview

This package handles:
- Service discovery
- Resource detection
- Network scanning
- Configuration discovery
- Auto-configuration

## Key Types

### DiscoveryService

```go
type DiscoveryService struct {
    scanners map[string]Scanner
    registry *ServiceRegistry
    config   *Config
}
```

### Scanner

```go
type Scanner interface {
    Scan(ctx context.Context) ([]*Resource, error)
    Type() ScannerType
}
```

## Usage

### Discovering Services

```go
import "dev.helix.code/internal/discovery"

service := discovery.NewService(config)
resources, err := service.Discover(ctx)
```

### Scanning Specific Types

```go
// Scan for databases
dbs, err := service.ScanDatabases(ctx)

// Scan for LLM providers
providers, err := service.ScanLLMProviders(ctx)

// Scan for workers
workers, err := service.ScanWorkers(ctx)
```

## Supported Discovery

- PostgreSQL databases
- Redis instances
- LLM API endpoints
- SSH workers
- Docker containers
- Kubernetes services

## Configuration

```yaml
discovery:
  enabled: true
  auto_scan: true
  scan_interval: 5m
  scanners:
    - database
    - redis
    - workers
```

## Testing

```bash
go test -v ./internal/discovery/...
```
