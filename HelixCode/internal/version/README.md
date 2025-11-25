# Version Package

The `version` package provides version information for the HelixCode platform.

## Overview

This package handles:
- Semantic version information
- Git commit hash tracking
- Build date tracking
- Go version information
- Platform information

## Variables

These variables are set at build time using `-ldflags`:

```go
var (
    Version   = "0.1.0"      // Semantic version
    GitCommit = "dev"        // Git commit hash
    BuildDate = "unknown"    // Build timestamp
    GoVersion = runtime.Version()
)
```

## Usage

### Getting Version Information

```go
import "dev.helix.code/internal/version"

// Get short version
ver := version.GetVersion()
// Returns: "0.1.0" or "0.1.0-abc1234" (with commit)

// Get full version
full := version.GetFullVersion()
// Returns: "HelixCode 0.1.0 (commit: abc1234, built: 2024-01-15, go: go1.21)"

// Get structured build info
info := version.GetBuildInfo()
// Returns map with version, commit, build_date, go_version, os, arch
```

### Build Info Map

```go
info := version.GetBuildInfo()

fmt.Println(info["version"])    // "0.1.0"
fmt.Println(info["commit"])     // "abc123456"
fmt.Println(info["build_date"]) // "2024-01-15T10:30:00Z"
fmt.Println(info["go_version"]) // "go1.21.0"
fmt.Println(info["os"])         // "darwin"
fmt.Println(info["arch"])       // "amd64"
```

## Setting Version at Build Time

```bash
# Build with version information
go build -ldflags "\
    -X dev.helix.code/internal/version.Version=1.0.0 \
    -X dev.helix.code/internal/version.GitCommit=$(git rev-parse HEAD) \
    -X dev.helix.code/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    ./cmd/server
```

## Makefile Integration

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse HEAD)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X dev.helix.code/internal/version.Version=$(VERSION) \
           -X dev.helix.code/internal/version.GitCommit=$(COMMIT) \
           -X dev.helix.code/internal/version.BuildDate=$(BUILD_DATE)

build:
	go build -ldflags "$(LDFLAGS)" ./cmd/server
```

## API Endpoint

The server exposes version info at `/health`:

```json
{
    "status": "healthy",
    "version": "0.1.0-abc1234",
    "build_info": {
        "version": "0.1.0",
        "commit": "abc1234567890",
        "build_date": "2024-01-15T10:30:00Z",
        "go_version": "go1.21.0",
        "os": "linux",
        "arch": "amd64"
    }
}
```

## Testing

```bash
go test -v ./internal/version/...
```

## Notes

- Always set version at build time for releases
- Include commit hash for debugging
- Build date helps identify deployments
- Platform info useful for cross-compilation
