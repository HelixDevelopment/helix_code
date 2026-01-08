// Package version provides build version information and metadata for the HelixCode application.
//
// The version package exposes version information that is typically set at build time
// using ldflags. It provides functions to retrieve version strings, build details,
// and structured build information for use in CLI output, health checks, and logging.
//
// # Build Variables
//
// The following variables can be set at build time:
//
//	go build -ldflags "-X dev.helix.code/internal/version.Version=1.0.0 \
//	                   -X dev.helix.code/internal/version.GitCommit=abc1234 \
//	                   -X dev.helix.code/internal/version.BuildDate=2025-01-08"
//
// Default values are used when not overridden:
//   - Version: "0.1.0"
//   - GitCommit: "dev"
//   - BuildDate: "unknown"
//   - GoVersion: runtime.Version()
//
// # Basic Version
//
// Get the basic version string:
//
//	v := version.GetVersion()
//	// Returns: "0.1.0" or "1.0.0-abc1234" (version with commit hash)
//
// The version string includes the short commit hash when available and not "dev".
//
// # Full Version
//
// Get detailed version information:
//
//	full := version.GetFullVersion()
//	// Returns: "HelixCode 1.0.0 (commit: abc1234def5678, built: 2025-01-08, go: go1.24)"
//
// This format is suitable for CLI help output and logging.
//
// # Build Information
//
// Get structured build information as a map:
//
//	info := version.GetBuildInfo()
//
//	fmt.Printf("Version: %s\n", info["version"])
//	fmt.Printf("Commit: %s\n", info["commit"])
//	fmt.Printf("Build Date: %s\n", info["build_date"])
//	fmt.Printf("Go Version: %s\n", info["go_version"])
//	fmt.Printf("OS: %s\n", info["os"])
//	fmt.Printf("Architecture: %s\n", info["arch"])
//
// The returned map contains:
//   - version: Semantic version string
//   - commit: Full git commit hash
//   - build_date: Build timestamp
//   - go_version: Go compiler version
//   - os: Operating system (runtime.GOOS)
//   - arch: Architecture (runtime.GOARCH)
//
// # CLI Usage Example
//
//	func versionCmd() *cobra.Command {
//	    return &cobra.Command{
//	        Use:   "version",
//	        Short: "Print version information",
//	        Run: func(cmd *cobra.Command, args []string) {
//	            fmt.Println(version.GetFullVersion())
//	        },
//	    }
//	}
//
// # Health Check Example
//
//	func healthHandler(c *gin.Context) {
//	    c.JSON(200, gin.H{
//	        "status":  "healthy",
//	        "version": version.GetVersion(),
//	        "build":   version.GetBuildInfo(),
//	    })
//	}
//
// # Makefile Integration
//
// Typical Makefile setup for version injection:
//
//	VERSION := $(shell git describe --tags --always --dirty)
//	COMMIT := $(shell git rev-parse HEAD)
//	BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
//
//	LDFLAGS := -X dev.helix.code/internal/version.Version=$(VERSION) \
//	           -X dev.helix.code/internal/version.GitCommit=$(COMMIT) \
//	           -X dev.helix.code/internal/version.BuildDate=$(BUILD_DATE)
//
//	build:
//	    go build -ldflags "$(LDFLAGS)" -o bin/helixcode ./cmd/server
//
// # Development Mode
//
// When running in development without ldflags:
//
//	// GitCommit will be "dev"
//	// GetVersion() returns just "0.1.0"
//	// GetFullVersion() includes "commit: dev"
//
// This makes it easy to identify development builds vs production releases.
package version
