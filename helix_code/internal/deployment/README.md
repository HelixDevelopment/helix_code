# Deployment Package

The `deployment` package provides comprehensive production deployment orchestration for the HelixCode platform, enabling safe, reliable, and automated deployments with multiple strategies, security gates, performance validation, and automatic rollback capabilities.

## Overview

This package implements a complete deployment pipeline that handles:

- **Multi-strategy deployments**: Blue-green, canary, rolling, recreate, and direct production deployments
- **Security gates**: Zero-tolerance security policy enforcement with comprehensive scanning
- **Performance gates**: Performance target validation before production deployment
- **Health checking**: Post-deployment health verification of deployed servers
- **Automatic rollback**: Rollback triggered on deployment failures when enabled
- **Multi-channel notifications**: Slack, email, and webhook notifications for deployment events
- **Deployment monitoring**: Production monitoring setup for deployed servers

## Key Types

### ProductionDeployer

The main orchestrator that manages the entire deployment lifecycle:

```go
type ProductionDeployer struct {
    config          *DeploymentConfig
    securityManager *security.SecurityManager
    monitoring      *monitoring.Monitor
    status          *DeploymentStatus
    mutex           sync.RWMutex
    running         atomic.Bool
}
```

The `ProductionDeployer` ensures thread-safe operation using atomic operations and mutex protection. Only one deployment can run at a time per deployer instance.

### DeploymentConfig

Comprehensive configuration for production deployments:

```go
type DeploymentConfig struct {
    ProjectName            string                `json:"project_name"`
    Environment            string                `json:"environment"`
    DeploymentStrategy     DeployStrategy        `json:"deployment_strategy"`
    SecurityGateEnabled    bool                  `json:"security_gate_enabled"`
    PerformanceGateEnabled bool                  `json:"performance_gate_enabled"`
    PerformanceGateStatus  PerformanceGateStatus `json:"performance_gate_status"`
    AutoRollbackEnabled    bool                  `json:"auto_rollback_enabled"`
    HealthCheckEnabled     bool                  `json:"health_check_enabled"`
    MonitoringEnabled      bool                  `json:"monitoring_enabled"`
    CanaryDuration         time.Duration         `json:"canary_duration"`
    RollbackTimeout        time.Duration         `json:"rollback_timeout"`
    HealthCheckTimeout     time.Duration         `json:"health_check_timeout"`
    MaxRetries             int                   `json:"max_retries"`
    TargetServers          []string              `json:"target_servers"`
    Credentials            map[string]string     `json:"credentials"`
    Notifications          NotificationConfig    `json:"notifications"`
}
```

### DeploymentStatus

Tracks the comprehensive state of a deployment:

```go
type DeploymentStatus struct {
    DeploymentID       string                `json:"deployment_id"`
    Status             DeploymentPhase       `json:"status"`
    StartTime          time.Time             `json:"start_time"`
    EndTime            time.Time             `json:"end_time"`
    Duration           time.Duration         `json:"duration"`
    CurrentPhase       string                `json:"current_phase"`
    CompletedPhases    []string              `json:"completed_phases"`
    FailedPhases       []string              `json:"failed_phases"`
    ServersDeployed    []string              `json:"servers_deployed"`
    ServersRollback    []string              `json:"servers_rollback"`
    SecurityGateStatus SecurityGateStatus    `json:"security_gate_status"`
    PerformanceGate    PerformanceGateStatus `json:"performance_gate_status"`
    HealthStatus       HealthCheckStatus     `json:"health_status"`
    RollbackTriggered  bool                  `json:"rollback_triggered"`
    Metrics            *DeploymentMetrics    `json:"metrics"`
    Notifications      []NotificationEvent   `json:"notifications"`
}
```

### DeployStrategy

Supported deployment strategies:

```go
const (
    BlueGreenDeploy  DeployStrategy = "blue_green"   // Instant switchover
    CanaryDeploy     DeployStrategy = "canary"       // Gradual traffic shift
    RollingDeploy    DeployStrategy = "rolling"      // Incremental updates
    RecreateDeploy   DeployStrategy = "recreate"     // Stop all, then deploy
    ProductionDeploy DeployStrategy = "production"   // Direct deployment
)
```

### DeploymentPhase

Deployment lifecycle phases:

```go
const (
    PhasePreparation      DeploymentPhase = "preparation"
    PhaseSecurityCheck    DeploymentPhase = "security_check"
    PhasePerformanceCheck DeploymentPhase = "performance_check"
    PhaseDeployment       DeploymentPhase = "deployment"
    PhaseHealthCheck      DeploymentPhase = "health_check"
    PhaseValidation       DeploymentPhase = "validation"
    PhaseMonitoring       DeploymentPhase = "monitoring"
    PhaseCompletion       DeploymentPhase = "completion"
    PhaseRollback         DeploymentPhase = "rollback"
    PhaseFailed           DeploymentPhase = "failed"
    PhaseSuccess          DeploymentPhase = "success"
)
```

## Usage Examples

### Basic Production Deployment

```go
import "dev.helix.code/internal/deployment"

config := &deployment.DeploymentConfig{
    ProjectName:        "myapp",
    Environment:        "production",
    DeploymentStrategy: deployment.RollingDeploy,
    TargetServers:      []string{"server1.example.com", "server2.example.com"},
    Credentials:        map[string]string{"deploy_key": "secret"},
    HealthCheckEnabled: true,
    MonitoringEnabled:  true,
}

deployer, err := deployment.NewProductionDeployer(config)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
status, err := deployer.StartProductionDeployment(ctx)
if err != nil {
    log.Printf("Deployment failed: %v", err)
}

log.Printf("Deployment completed: %s, Duration: %v", status.Status, status.Duration)
```

### Deployment with Security and Performance Gates

```go
config := &deployment.DeploymentConfig{
    ProjectName:            "secure-app",
    Environment:            "production",
    DeploymentStrategy:     deployment.BlueGreenDeploy,
    SecurityGateEnabled:    true,
    PerformanceGateEnabled: true,
    PerformanceGateStatus: deployment.PerformanceGateStatus{
        ThroughputTarget: 2000,                    // ops/sec
        LatencyTarget:    "50ms",
        CPUTarget:        80.0,                    // percentage
        MemoryTarget:     4 * 1024 * 1024 * 1024,  // 4GB
    },
    AutoRollbackEnabled: true,
    TargetServers:       []string{"prod1", "prod2", "prod3"},
    Credentials:         map[string]string{"key": "value"},
}

deployer, err := deployment.NewProductionDeployer(config)
if err != nil {
    log.Fatal(err)
}

status, _ := deployer.StartProductionDeployment(ctx)

// Check gate statuses
if !status.SecurityGateStatus.Passed {
    log.Printf("Security issues: %d critical", status.SecurityGateStatus.CriticalIssues)
}
if !status.PerformanceGate.Passed {
    log.Printf("Performance not met: %s", status.PerformanceGate.Reason)
}
```

### Deployment with Notifications

```go
config := &deployment.DeploymentConfig{
    ProjectName:        "notified-app",
    Environment:        "production",
    DeploymentStrategy: deployment.CanaryDeploy,
    CanaryDuration:     10 * time.Minute,
    TargetServers:      []string{"canary", "prod1", "prod2"},
    Credentials:        map[string]string{"key": "value"},
    Notifications: deployment.NotificationConfig{
        SlackEnabled:    true,
        SlackWebhookURL: "https://hooks.slack.com/services/...",
        EmailEnabled:    true,
        EmailRecipients: []string{"ops@example.com", "dev@example.com"},
        WebhookEnabled:  true,
        WebhookURL:      "https://monitoring.example.com/webhooks/deploy",
    },
}
```

## Configuration Options

### YAML Configuration

```yaml
deployment:
  default_target: "production"
  default_strategy: "rolling"

  targets:
    production:
      type: "kubernetes"
      config:
        namespace: "production"
        replicas: 3
        strategy: "rolling_update"
        max_surge: "25%"
        max_unavailable: "25%"

    staging:
      type: "docker"
      config:
        host: "staging.example.com"
        port: 2375

  security_gate:
    enabled: true
    zero_tolerance: true
    scan_timeout: "5m"
    block_on_critical: true
    block_on_high: false

  performance_gate:
    enabled: true
    throughput_target: 2000
    latency_target: "100ms"
    cpu_target: 80
    memory_target: "4GB"

  health_check:
    enabled: true
    timeout: "30s"
    interval: "5s"
    healthy_threshold: 90  # percentage

  rollback:
    enabled: true
    timeout: "10m"
    automatic: true

  notifications:
    slack:
      enabled: true
      webhook_url: "https://hooks.slack.com/..."
      on_start: true
      on_success: true
      on_failure: true
    email:
      enabled: true
      recipients:
        - "ops@example.com"
```

### Environment Variables

```bash
HELIX_DEPLOYMENT_STRATEGY=rolling
HELIX_DEPLOYMENT_SECURITY_GATE=true
HELIX_DEPLOYMENT_PERFORMANCE_GATE=true
HELIX_DEPLOYMENT_AUTO_ROLLBACK=true
HELIX_DEPLOYMENT_HEALTH_CHECK=true
HELIX_SLACK_WEBHOOK_URL=https://hooks.slack.com/...
```

## Deployment Phases

A production deployment executes through the following phases:

1. **Preparation**: Prerequisites check, environment setup, server validation
2. **Security Check**: Zero-tolerance security gate validation (if enabled)
3. **Performance Check**: Performance target validation (if enabled)
4. **Deployment**: Actual deployment to target servers using selected strategy
5. **Health Check**: Post-deployment health verification (if enabled)
6. **Validation**: Final deployment validation
7. **Monitoring**: Production monitoring setup (if enabled)

Each phase can fail and trigger automatic rollback if `AutoRollbackEnabled` is true.

## Best Practices

### Deployment Strategy Selection

- **Blue-Green**: Best for zero-downtime deployments with instant rollback capability
- **Canary**: Ideal for gradually rolling out changes with risk mitigation
- **Rolling**: Good for stateless applications with multiple replicas
- **Recreate**: Use when complete restart is acceptable or required
- **Production**: Direct deployment for simple scenarios

### Security Gate Configuration

```go
// Always enable security gates for production
config.SecurityGateEnabled = true

// Zero-tolerance for critical issues
// The security gate will block deployment if any critical issues exist
```

### Performance Gate Thresholds

```go
// Set realistic performance targets based on baseline metrics
config.PerformanceGateStatus = deployment.PerformanceGateStatus{
    ThroughputTarget: baselineThroughput * 0.95,  // 95% of baseline
    LatencyTarget:    "100ms",                     // P99 latency target
    CPUTarget:        75.0,                        // Leave headroom
    MemoryTarget:     memoryLimit * 0.80,          // 80% of limit
}
```

### Health Check Configuration

```go
// Configure appropriate timeouts
config.HealthCheckTimeout = 30 * time.Second
config.HealthCheckEnabled = true

// Health checks require 90% of servers healthy by default
```

### Rollback Configuration

```go
// Enable automatic rollback for production
config.AutoRollbackEnabled = true
config.RollbackTimeout = 10 * time.Minute

// Rollback is triggered when:
// - Security gate fails
// - Performance gate fails
// - Health checks fail
// - Deployment phase fails
```

## Integration Patterns

### With CI/CD Pipeline

```go
// In your CI/CD script
func deploy(ctx context.Context, version string) error {
    config := loadDeploymentConfig()
    config.ProjectName = fmt.Sprintf("myapp-v%s", version)

    deployer, err := deployment.NewProductionDeployer(config)
    if err != nil {
        return fmt.Errorf("failed to create deployer: %w", err)
    }

    status, err := deployer.StartProductionDeployment(ctx)
    if err != nil {
        return fmt.Errorf("deployment failed: %w", err)
    }

    if status.Status != deployment.PhaseSuccess {
        return fmt.Errorf("deployment ended with status: %s", status.Status)
    }

    // Report metrics
    reportDeploymentMetrics(status.Metrics)

    return nil
}
```

### With Monitoring Systems

```go
// The deployer integrates with the monitoring package
config.MonitoringEnabled = true

// After deployment, monitoring is automatically set up for all deployed servers
// Metrics include:
// - Deployment time
// - Rollback time (if applicable)
// - Number of servers deployed/rolled back
// - Security scans performed
// - Performance tests run
// - Health checks executed
```

## Thread Safety

The `ProductionDeployer` uses atomic operations and mutex protection to ensure safe concurrent access:

- `atomic.Bool` for the running state prevents multiple simultaneous deployments
- `sync.RWMutex` protects status updates during deployment
- Each deployment gets a unique ID generated from timestamp

## Testing

```bash
# Run all deployment tests
go test -v ./internal/deployment/...

# Run specific test
go test -v ./internal/deployment -run TestStartProductionDeployment

# Run with coverage
go test -cover ./internal/deployment/...

# Run benchmarks
go test -bench=. ./internal/deployment/...
```

## Metrics and Observability

The package tracks comprehensive deployment metrics:

```go
type DeploymentMetrics struct {
    DeploymentTime   time.Duration  // Total deployment time
    RollbackTime     time.Duration  // Rollback time if triggered
    DeployedServers  int            // Number of successfully deployed servers
    RollbackServers  int            // Number of rolled back servers
    SecurityScans    int            // Number of security scans performed
    PerformanceTests int            // Number of performance tests run
    HealthChecks     int            // Number of health checks performed
    Retries          int            // Number of retries attempted
    Notifications    int            // Number of notifications sent
}
```

## Error Handling

Deployment errors are handled gracefully with automatic rollback support. Failed deployments update the status with:

- Failed phase name
- Error reason
- Rollback status (if enabled)
- Partial deployment information

The deployer always returns a `DeploymentStatus` even on failure, providing full visibility into what occurred.
