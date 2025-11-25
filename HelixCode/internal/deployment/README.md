# Deployment Package

The `deployment` package provides deployment automation for the HelixCode platform.

## Overview

This package handles:
- Multi-target deployment (cloud, containers, servers)
- Deployment strategies (rolling, blue-green, canary)
- Configuration management
- Rollback support
- Deployment monitoring

## Key Types

### Deployer

```go
type Deployer struct {
    targets   map[string]Target
    strategy  Strategy
    config    *Config
}
```

### Target

```go
type Target interface {
    Deploy(ctx context.Context, artifact *Artifact) error
    Rollback(ctx context.Context, version string) error
    Status(ctx context.Context) (*Status, error)
}
```

## Usage

### Creating a Deployer

```go
import "dev.helix.code/internal/deployment"

deployer := deployment.NewDeployer(config)
```

### Deploying

```go
artifact := &deployment.Artifact{
    Name:    "myapp",
    Version: "1.0.0",
    Path:    "/path/to/artifact",
}

err := deployer.Deploy(ctx, artifact, &deployment.Options{
    Target:   "production",
    Strategy: deployment.StrategyRolling,
})
```

### Rollback

```go
err := deployer.Rollback(ctx, "production", "0.9.0")
```

## Supported Targets

- Docker/Kubernetes
- AWS (EC2, ECS, Lambda)
- Google Cloud (GCE, Cloud Run)
- Azure (VMs, Container Apps)
- SSH servers

## Configuration

```yaml
deployment:
  default_target: "production"
  targets:
    production:
      type: "kubernetes"
      config:
        namespace: "production"
        replicas: 3

    staging:
      type: "docker"
      config:
        host: "staging.example.com"
```

## Testing

```bash
go test -v ./internal/deployment/...
```
