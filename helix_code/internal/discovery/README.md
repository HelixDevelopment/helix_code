# Discovery Package

The `discovery` package provides comprehensive service registration, discovery, and health monitoring for the HelixCode platform. It implements a complete service discovery system with multiple discovery strategies, TTL-based expiration, UDP multicast broadcast discovery, intelligent port allocation, and protocol-specific health checking.

## Overview

This package implements a robust service discovery system that handles:

- **Service Registry**: Central registry for service registration and lookup with TTL-based expiration
- **Multi-strategy Discovery**: Default port, registry, broadcast, and DNS discovery strategies
- **UDP Broadcast Discovery**: Zero-configuration LAN-based discovery using multicast
- **Dynamic Port Allocation**: Intelligent port allocation with fallback mechanisms
- **Protocol-specific Health Checking**: TCP, HTTP/HTTPS, and gRPC health checks
- **Configuration Management**: Centralized configuration with validation and callbacks

## Key Components

### ServiceRegistry

The central registry for managing service registrations with automatic expiration and health checking:

```go
type ServiceRegistry struct {
    config    RegistryConfig
    services  map[string]*ServiceInfo
    mu        sync.RWMutex
    stopChan  chan struct{}
    cleanupWg sync.WaitGroup
}
```

### DiscoveryClient

Client for discovering services using multiple strategies:

```go
type DiscoveryClient struct {
    config DiscoveryClientConfig
}
```

### BroadcastService

UDP multicast service for LAN-based discovery:

```go
type BroadcastService struct {
    config       BroadcastConfig
    conn         *net.UDPConn
    running      bool
    discovered   map[string]*ServiceInfo
    localService *ServiceInfo
}
```

### PortAllocator

Intelligent port allocation with range management:

```go
type PortAllocator struct {
    config      PortAllocatorConfig
    allocations map[int]*PortAllocation
    serviceMap  map[string]int
    mu          sync.RWMutex
}
```

### HealthMonitor

Protocol-specific health monitoring:

```go
type HealthMonitor struct {
    config          HealthMonitorConfig
    registry        *ServiceRegistry
    failureCounts   map[string]int
    successCounts   map[string]int
    lastResults     map[string]*HealthCheckResult
    customChecks    map[string]HealthCheckFunc
    serviceStrategy map[string]HealthCheckStrategy
}
```

## Key Types

### ServiceInfo

Comprehensive service metadata:

```go
type ServiceInfo struct {
    Name          string            `json:"name"`
    Host          string            `json:"host"`
    Port          int               `json:"port"`
    Protocol      string            `json:"protocol"`  // tcp, udp, http, https, grpc
    Version       string            `json:"version"`
    Metadata      map[string]string `json:"metadata"`
    RegisteredAt  time.Time         `json:"registered_at"`
    LastHeartbeat time.Time         `json:"last_heartbeat"`
    TTL           time.Duration     `json:"ttl"`
    Healthy       bool              `json:"healthy"`
}

// Address returns the full address of the service
func (s *ServiceInfo) Address() string {
    return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// IsExpired checks if the service registration has expired
func (s *ServiceInfo) IsExpired() bool {
    if s.TTL == 0 {
        return false // No TTL means never expires
    }
    return time.Since(s.LastHeartbeat) > s.TTL
}
```

### DiscoveryStrategy

Available discovery strategies:

```go
const (
    StrategyDefaultPort DiscoveryStrategy = "default_port"  // Well-known ports
    StrategyRegistry    DiscoveryStrategy = "registry"      // Service registry
    StrategyBroadcast   DiscoveryStrategy = "broadcast"     // UDP multicast
    StrategyDNS         DiscoveryStrategy = "dns"           // DNS resolution
)
```

### HealthCheckStrategy

Health check strategies:

```go
const (
    HealthCheckTCP    HealthCheckStrategy = "tcp"     // TCP connection check
    HealthCheckHTTP   HealthCheckStrategy = "http"    // HTTP GET /health
    HealthCheckCustom HealthCheckStrategy = "custom"  // Custom function
)
```

## Usage Examples

### Service Registry

```go
import "dev.helix.code/internal/discovery"

// Create registry with default configuration
registry := discovery.NewServiceRegistry(discovery.DefaultRegistryConfig())
registry.Start()
defer registry.Stop()

// Register a service
err := registry.Register(discovery.ServiceInfo{
    Name:     "api-server",
    Host:     "localhost",
    Port:     8080,
    Protocol: "http",
    Version:  "1.0.0",
    TTL:      30 * time.Second,
    Metadata: map[string]string{
        "health_endpoint": "/healthz",
    },
})
if err != nil {
    log.Fatal(err)
}

// Query services
service, err := registry.Get("api-server")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Service at: %s\n", service.Address())

// List all healthy services
healthyServices := registry.ListHealthy()

// Send heartbeat
registry.Heartbeat("api-server")

// Deregister when done
registry.Deregister("api-server")
```

### Discovery Client

```go
// Create port allocator and registry
allocator := discovery.NewDefaultPortAllocator()
registry := discovery.NewDefaultServiceRegistry()
registry.Start()

// Create discovery client
client := discovery.NewDiscoveryClient(
    discovery.DefaultDiscoveryClientConfig(registry, allocator),
)

// Discover a service
result, err := client.Discover("database")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Found database at %s via %s\n",
    result.ServiceInfo.Address(),
    result.Strategy)

// Discover with timeout
result, err = client.DiscoverWithTimeout("cache", 5*time.Second)

// Wait for a service to become available
result, err = client.WaitForService("api", 30*time.Second)

// Get service address directly
address, err := client.GetServiceAddress("grpc-server")

// List all registered services
services := client.ListServices()

// List only healthy services
healthy := client.ListHealthyServices()
```

### Broadcast Discovery

```go
// Create broadcast service
broadcast := discovery.NewBroadcastService(discovery.DefaultBroadcastConfig())

// Set local service to announce
broadcast.SetLocalService(discovery.ServiceInfo{
    Name:     "my-service",
    Host:     "192.168.1.100",
    Port:     8080,
    Protocol: "http",
})

// Start broadcast service
if err := broadcast.Start(); err != nil {
    log.Fatal(err)
}
defer broadcast.Stop()

// Discover services via broadcast
service, err := broadcast.Discover("other-service")
if err != nil {
    log.Printf("Service not found: %v", err)
}

// List all discovered services
discovered := broadcast.List()
for _, svc := range discovered {
    fmt.Printf("Found: %s at %s\n", svc.Name, svc.Address())
}

// Clean expired services
broadcast.CleanExpired()
```

### Port Allocation

```go
// Create port allocator
allocator := discovery.NewPortAllocator(discovery.PortAllocatorConfig{
    AllowEphemeral: false,
    PortRanges: map[string]discovery.PortRange{
        "database":  {Start: 5433, End: 5442},
        "cache":     {Start: 6380, End: 6389},
        "api":       {Start: 8081, End: 8099},
        "grpc":      {Start: 9091, End: 9109},
    },
    ReservedPorts: []int{5432, 6379, 8080, 9090},
})

// Allocate a port (tries preferred, then finds available)
port, err := allocator.AllocatePort("my-database", 5432)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Allocated port: %d\n", port)

// Allocate within specific range
port, err = allocator.AllocatePortInRange("my-api", 8081, 8099)

// Check if port is available
available := allocator.IsPortAvailable(8080)

// Get port for a service
port, exists := allocator.GetPortForService("my-database")

// List all allocations
allocations := allocator.ListAllocations()

// Release port
allocator.ReleaseServicePort("my-database")
```

### Health Monitoring

```go
// Create health monitor
monitor := discovery.NewHealthMonitor(
    discovery.DefaultHealthMonitorConfig(),
    registry,
)

// Start monitoring
if err := monitor.Start(); err != nil {
    log.Fatal(err)
}
defer monitor.Stop()

// Register custom health check
monitor.RegisterCustomCheck("my-service", func(info *discovery.ServiceInfo) error {
    // Custom health check logic
    conn, err := connectToService(info.Address())
    if err != nil {
        return err
    }
    defer conn.Close()
    return conn.Ping()
})

// Set health check strategy for a service
monitor.SetServiceStrategy("api-server", discovery.HealthCheckHTTP)

// Check service health immediately
result, err := monitor.CheckServiceHealth("api-server")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Healthy: %t, Latency: %v\n", result.Healthy, result.Latency)

// Get last health check result
result, exists := monitor.GetLastResult("api-server")

// Get all health check results
results := monitor.GetAllResults()

// Get healthy/unhealthy services
healthy := monitor.GetHealthyServices()
unhealthy := monitor.GetUnhealthyServices()

// Get failure/success counts
failures := monitor.GetFailureCount("api-server")
successes := monitor.GetSuccessCount("api-server")

// Reset counts
monitor.ResetCounts("api-server")
```

### Configuration Management

```go
// Create configuration manager
config := discovery.DefaultDiscoveryConfig()
configManager, err := discovery.NewConfigManager(config)
if err != nil {
    log.Fatal(err)
}

// Register components for automatic updates
configManager.RegisterComponents(registry, allocator, broadcast, client)

// Register callback for config changes
configManager.RegisterCallback(func(old, new discovery.DiscoveryConfig) error {
    log.Printf("Config updated: TTL changed from %v to %v",
        old.DefaultTTL, new.DefaultTTL)
    return nil
})

// Update configuration
err = configManager.UpdateConfig(discovery.DiscoveryConfig{
    DefaultTTL:          60 * time.Second,
    HealthCheckInterval: 10 * time.Second,
    EnableRegistry:      true,
    EnableBroadcast:     true,
})

// Update partial configuration
err = configManager.UpdatePartial(func(cfg *discovery.DiscoveryConfig) {
    cfg.HealthCheckInterval = 15 * time.Second
    cfg.EnableBroadcast = true
})

// Lock configuration to prevent changes
configManager.Lock()

// Get current configuration
currentConfig := configManager.GetConfig()

// Set port range for service type
err = configManager.SetPortRange("websocket", discovery.PortRange{
    Start: 8001,
    End:   8020,
})

// Add/remove reserved ports
configManager.AddReservedPort(3000)
configManager.RemoveReservedPort(3000)

// Export configuration
exported := configManager.ExportConfig()
```

## Configuration Options

### Registry Configuration

```go
type RegistryConfig struct {
    DefaultTTL          time.Duration  // Default service TTL (30s)
    CleanupInterval     time.Duration  // Cleanup interval (10s)
    EnableHealthChecks  bool           // Enable automatic health checks
    HealthCheckInterval time.Duration  // Health check interval (15s)
}
```

### Broadcast Configuration

```go
type BroadcastConfig struct {
    MulticastAddress     string         // Default: "239.255.0.1:7001"
    AnnouncementInterval time.Duration  // Default: 5s
    DiscoveryTimeout     time.Duration  // Default: 3s
    Interface            string         // Network interface (empty for default)
    TTL                  int            // Multicast TTL (default: 2)
}
```

### Port Allocator Configuration

```go
type PortAllocatorConfig struct {
    AllowEphemeral bool                    // Allow ephemeral ports
    PortRanges     map[string]PortRange    // Port ranges per service type
    ReservedPorts  []int                   // Ports to never allocate
}
```

### Health Monitor Configuration

```go
type HealthMonitorConfig struct {
    CheckInterval      time.Duration        // Check interval (5s)
    CheckTimeout       time.Duration        // Check timeout (2s)
    UnhealthyThreshold int                  // Failures before unhealthy (3)
    HealthyThreshold   int                  // Successes before healthy (2)
    DefaultStrategy    HealthCheckStrategy  // Default check strategy
    EnableAutoRemoval  bool                 // Remove after threshold
    RemovalThreshold   int                  // Failures before removal (5)
}
```

### YAML Configuration

```yaml
discovery:
  enabled: true
  auto_scan: true
  scan_interval: 5m

  registry:
    default_ttl: 30s
    cleanup_interval: 10s
    enable_health_checks: true
    health_check_interval: 15s

  broadcast:
    enabled: true
    multicast_address: "239.255.0.1:7001"
    announcement_interval: 5s
    discovery_timeout: 3s
    ttl: 2

  port_allocation:
    allow_ephemeral: false
    port_ranges:
      database: {start: 5433, end: 5442}
      cache: {start: 6380, end: 6389}
      api: {start: 8081, end: 8099}
      grpc: {start: 9091, end: 9109}
    reserved_ports:
      - 5432
      - 6379
      - 8080
      - 9090

  default_ports:
    database: 5432
    cache: 6379
    api: 8080
    grpc: 9090
    metrics: 9100

  strategies:
    - default_port
    - registry
    - dns
```

## Default Port Mappings

The discovery client automatically maps service names to well-known ports:

| Service Type | Default Port |
|-------------|--------------|
| database/postgres | 5432 |
| cache/redis | 6379 |
| api/http | 8080 |
| grpc | 9090 |
| metrics/prometheus | 9100 |

## Best Practices

### Service Registration

```go
// Always set TTL for automatic expiration
info := discovery.ServiceInfo{
    Name:     "my-service",
    Host:     "localhost",
    Port:     8080,
    Protocol: "http",
    TTL:      30 * time.Second,  // Always set TTL
    Metadata: map[string]string{
        "version":         "1.0.0",
        "health_endpoint": "/health",
    },
}

// Send regular heartbeats to prevent expiration
go func() {
    ticker := time.NewTicker(info.TTL / 2)
    defer ticker.Stop()
    for range ticker.C {
        registry.Heartbeat(info.Name)
    }
}()
```

### Discovery Strategy Order

```go
// Order strategies from fastest to slowest
config.PreferredStrategies = []discovery.DiscoveryStrategy{
    discovery.StrategyDefaultPort,  // Check well-known ports first
    discovery.StrategyRegistry,     // Then check registry
    discovery.StrategyDNS,          // DNS as fallback
}
```

### Health Check Configuration

```go
// Set appropriate thresholds based on service criticality
config := discovery.HealthMonitorConfig{
    CheckInterval:      5 * time.Second,
    CheckTimeout:       2 * time.Second,
    UnhealthyThreshold: 3,   // Mark unhealthy after 3 failures
    HealthyThreshold:   2,   // Mark healthy after 2 successes
    EnableAutoRemoval:  true,
    RemovalThreshold:   10,  // Remove after 10 consecutive failures
}
```

### Port Allocation

```go
// Reserve critical ports
config := discovery.PortAllocatorConfig{
    ReservedPorts: []int{
        22,    // SSH
        80,    // HTTP
        443,   // HTTPS
        5432,  // PostgreSQL
        6379,  // Redis
    },
}
```

## Integration Patterns

### With Worker Pool

```go
// Register worker with discovery
func registerWorker(registry *discovery.ServiceRegistry, worker *Worker) error {
    return registry.Register(discovery.ServiceInfo{
        Name:     fmt.Sprintf("worker-%s", worker.ID),
        Host:     worker.Host,
        Port:     worker.Port,
        Protocol: "grpc",
        TTL:      30 * time.Second,
        Metadata: map[string]string{
            "capabilities": strings.Join(worker.Capabilities, ","),
            "cpu_cores":    strconv.Itoa(worker.CPUCores),
            "memory_gb":    strconv.Itoa(worker.MemoryGB),
        },
    })
}

// Discover available workers
func findWorkers(client *discovery.DiscoveryClient) []*discovery.ServiceInfo {
    workers := []*discovery.ServiceInfo{}
    for _, service := range client.ListHealthyServices() {
        if strings.HasPrefix(service.Name, "worker-") {
            workers = append(workers, service)
        }
    }
    return workers
}
```

### With LLM Providers

```go
// Register LLM provider endpoint
registry.Register(discovery.ServiceInfo{
    Name:     "ollama-server",
    Host:     "localhost",
    Port:     11434,
    Protocol: "http",
    TTL:      60 * time.Second,
    Metadata: map[string]string{
        "type":   "llm",
        "models": "llama2,codellama,mistral",
    },
})
```

## Thread Safety

All components are thread-safe and designed for concurrent access:

- **ServiceRegistry**: Uses `sync.RWMutex` for safe concurrent reads and writes
- **BroadcastService**: Uses mutex protection for running state and discovered services
- **PortAllocator**: Uses `sync.RWMutex` for allocation tracking
- **HealthMonitor**: Thread-safe health check state management
- **ConfigManager**: Protected configuration updates with callbacks

## Error Handling

The package defines specific error types for common scenarios:

```go
var (
    ErrServiceNotFound          = errors.New("service not found")
    ErrServiceAlreadyRegistered = errors.New("service already registered")
    ErrInvalidServiceInfo       = errors.New("invalid service information")
    ErrServiceUnavailable       = errors.New("service unavailable")
    ErrBroadcastNotRunning      = errors.New("broadcast service not running")
    ErrNoPortsAvailable         = errors.New("no ports available")
    ErrInvalidPortRange         = errors.New("invalid port range")
    ErrPortAlreadyAllocated     = errors.New("port already allocated")
    ErrHealthCheckFailed        = errors.New("health check failed")
)
```

## Testing

```bash
# Run all discovery tests
go test -v ./internal/discovery/...

# Run specific test
go test -v ./internal/discovery -run TestServiceRegistry

# Run integration tests
go test -v ./internal/discovery -run Integration

# Run with coverage
go test -cover ./internal/discovery/...

# Run benchmarks
go test -bench=. ./internal/discovery/...
```

## Protocol-Specific Health Checks

The registry performs protocol-specific health checks:

- **HTTP/HTTPS**: GET request to `/health` endpoint (or custom path from metadata)
- **gRPC**: Standard gRPC health checking protocol
- **TCP/UDP**: Connection establishment check

```go
// Configure custom health endpoint via metadata
registry.Register(discovery.ServiceInfo{
    Name:     "api-server",
    Protocol: "http",
    Metadata: map[string]string{
        "health_endpoint": "/healthz",  // Custom health path
    },
})

// Configure gRPC service name for health check
registry.Register(discovery.ServiceInfo{
    Name:     "grpc-server",
    Protocol: "grpc",
    Metadata: map[string]string{
        "grpc_service_name": "myapp.MyService",
    },
})
```
