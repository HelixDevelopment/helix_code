// Package containers provides an adapter layer between HelixCode and the
// digital.vasic.containers module.
//
// This adapter centralizes all container operations (runtime detection,
// compose up/down, health checking) through the Containers module interfaces.
// All container workflows are containerized — no host dependencies required.
//
// Authority: CONST-035 — End-User Usability Mandate
package containers

import (
	"context"
	"fmt"
	gosync "sync"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/health"
	ctrRuntime "digital.vasic.containers/pkg/runtime"
	"golang.org/x/sync/semaphore"
)

// Adapter bridges HelixCode to the Containers orchestration module.
type Adapter struct {
	mu       gosync.RWMutex
	initOnce gosync.Once
	initErr  error

	// Runtime (Docker / Podman / Kubernetes)
	rt         ctrRuntime.ContainerRuntime
	rtName     string
	rtDetected bool

	// Compose orchestrator
	compose *compose.DefaultOrchestrator

	// Concurrency control for container operations
	sem *semaphore.Weighted
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// NewAdapter creates a new containers adapter with lazy initialization.
func NewAdapter() *Adapter {
	return &Adapter{
		sem: semaphore.NewWeighted(int64(max(2, min(10, 2*max(1, 4))))),
	}
}

// initRuntime performs one-time container runtime auto-detection.
//
// §11.4.85 concurrency fix: the field writes below (a.rt, a.rtName,
// a.rtDetected, a.compose) are read elsewhere under a.mu (RuntimeName,
// RuntimeAvailable, ListContainers). sync.Once provides a happens-before
// edge ONLY to callers that route through initRuntime — a getter that
// reads a.rtName under a.mu.RLock() WITHOUT first calling initRuntime
// shares no synchronisation with this Once writer, so the unguarded
// write is a data race (caught by `go test -race`: adapter.go:72 write
// vs adapter.go:96 read). The fix takes a.mu for the field mutation so
// the write side and the mutex-guarded read side share the same lock.
// AutoDetect / NewDefaultOrchestrator (slow, possibly blocking) stay
// OUTSIDE the lock so the mutex is held only for the cheap assignment.
func (a *Adapter) initRuntime(ctx context.Context) error {
	a.initOnce.Do(func() {
		rt, err := ctrRuntime.AutoDetect(ctx)
		if err != nil {
			a.initErr = fmt.Errorf("no container runtime detected (docker/podman required): %w", err)
			return
		}
		orch, err := compose.NewDefaultOrchestrator(".", nil)
		if err != nil {
			a.initErr = fmt.Errorf("compose orchestrator init failed: %w", err)
			return
		}
		a.mu.Lock()
		a.rt = rt
		a.rtName = rt.Name()
		a.rtDetected = true
		a.compose = orch
		a.mu.Unlock()
	})
	return a.initErr
}

// RuntimeAvailable returns true if a container runtime is available.
func (a *Adapter) RuntimeAvailable(ctx context.Context) bool {
	_ = a.initRuntime(ctx)
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rtDetected
}

// RuntimeName returns the detected runtime name (docker, podman, etc.).
func (a *Adapter) RuntimeName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rtName
}

// ComposeUp runs "compose up" for the given compose file with optional services.
func (a *Adapter) ComposeUp(ctx context.Context, composeFile string, services []string) error {
	if err := a.initRuntime(ctx); err != nil {
		return err
	}
	if err := a.sem.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("compose up cancelled: %w", err)
	}
	defer a.sem.Release(1)

	project := compose.ComposeProject{
		Name:     "helixcode",
		File:     composeFile,
		Services: services,
	}
	return a.compose.Up(ctx, project, compose.WithUpDetach(true))
}

// ComposeDown runs "compose down" for the given compose file.
func (a *Adapter) ComposeDown(ctx context.Context, composeFile string, removeVolumes bool) error {
	if err := a.initRuntime(ctx); err != nil {
		return err
	}
	if err := a.sem.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("compose down cancelled: %w", err)
	}
	defer a.sem.Release(1)

	project := compose.ComposeProject{
		Name: "helixcode",
		File: composeFile,
	}
	opts := []compose.DownOption{}
	if removeVolumes {
		opts = append(opts, compose.WithDownRemoveVolumes(true))
	}
	return a.compose.Down(ctx, project, opts...)
}

// ComposeStatus returns the status of services in a compose file.
func (a *Adapter) ComposeStatus(ctx context.Context, composeFile string) ([]compose.ServiceStatus, error) {
	if err := a.initRuntime(ctx); err != nil {
		return nil, err
	}
	project := compose.ComposeProject{
		Name: "helixcode",
		File: composeFile,
	}
	return a.compose.Status(ctx, project)
}

// HealthCheck performs a health check against a service endpoint.
func (a *Adapter) HealthCheck(ctx context.Context, target health.HealthTarget) error {
	checker := health.NewDefaultChecker()
	result := checker.Check(ctx, target)
	if !result.Healthy {
		return fmt.Errorf("health check failed for %s: %s", target.Name, result.Error)
	}
	return nil
}

// HealthCheckHTTP is a convenience helper for HTTP health checks.
func (a *Adapter) HealthCheckHTTP(url string) error {
	return a.HealthCheck(context.Background(), health.HealthTarget{
		Type: health.HealthHTTP,
		URL:  url,
	})
}

// HealthCheckTCP is a convenience helper for TCP health checks.
func (a *Adapter) HealthCheckTCP(host string, port string) error {
	return a.HealthCheck(context.Background(), health.HealthTarget{
		Type: health.HealthTCP,
		Host: host,
		Port: port,
	})
}

// BootAll boots all services defined in a compose file.
func (a *Adapter) BootAll(ctx context.Context, composeFile string) error {
	if err := a.initRuntime(ctx); err != nil {
		return err
	}
	if err := a.sem.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("boot cancelled: %w", err)
	}
	defer a.sem.Release(1)

	project := compose.ComposeProject{
		Name: "helixcode",
		File: composeFile,
	}
	return a.compose.Up(ctx, project, compose.WithUpDetach(true), compose.WithBuildFirst(true))
}

// ListContainers returns a list of running containers.
func (a *Adapter) ListContainers(ctx context.Context) ([]ctrRuntime.ContainerInfo, error) {
	if err := a.initRuntime(ctx); err != nil {
		return nil, err
	}
	return a.rt.List(ctx, ctrRuntime.ListFilter{})
}

// Shutdown gracefully shuts down the adapter.
func (a *Adapter) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	// ContainerRuntime has no Close() method; nothing to clean up at adapter level.
	return nil
}
