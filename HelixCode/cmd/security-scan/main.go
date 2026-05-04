// cmd/security-scan — HelixCode security scanner bootstrap via Containers BootManager.
//
// This binary wires the SonarQube and Snyk container lifecycle through the
// digital.vasic.containers BootManager (pkg/boot, pkg/endpoint, pkg/health, pkg/runtime).
// It replaces bare docker-compose calls in scripts/security-scan.sh (P0-T08.7/4).
//
// Usage:
//
//	go run ./cmd/security-scan -scanner=sonarqube [-action=start|stop|status]
//	go run ./cmd/security-scan -scanner=snyk [-action=start|stop|status]
//
// Credentials are read from the environment (loaded by the calling script from .env).
// No credentials are baked into this binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/runtime"
)

const (
	sonarqubePort    = "9000"
	sonarqubeHealth  = "/api/system/status"
	defaultTimeout   = 5 * time.Minute
	defaultRetries   = 30
	retryInterval    = 10 * time.Second
)

func main() {
	scanner := flag.String("scanner", "", "Scanner to boot: sonarqube|snyk")
	action := flag.String("action", "start", "Action: start|status (stop is not yet implemented)")
	flag.Parse()

	if *scanner == "" {
		fmt.Fprintln(os.Stderr, "Usage: security-scan -scanner=sonarqube|snyk [-action=start|stop|status]")
		os.Exit(1)
	}

	// Resolve project directory (two levels up from binary location or working dir)
	projectDir, err := resolveProjectDir()
	if err != nil {
		log.Fatalf("security-scan: failed to resolve project dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Auto-detect container runtime (Docker or Podman)
	rt, err := runtime.AutoDetect(ctx)
	if err != nil {
		log.Fatalf("security-scan: no container runtime available: %v", err)
	}
	log.Printf("security-scan: detected runtime: %s", rt.Name())

	switch *scanner {
	case "sonarqube", "sonar":
		if err := handleSonarQube(ctx, projectDir, rt, *action); err != nil {
			log.Fatalf("security-scan: sonarqube %s failed: %v", *action, err)
		}
	case "snyk":
		if err := handleSnyk(ctx, projectDir, rt, *action); err != nil {
			log.Fatalf("security-scan: snyk %s failed: %v", *action, err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown scanner %q. Use: sonarqube|snyk\n", *scanner)
		os.Exit(1)
	}
}

func handleSonarQube(ctx context.Context, projectDir string, rt runtime.ContainerRuntime, action string) error {
	composeFile := filepath.Join(projectDir, "docker", "security", "sonarqube", "docker-compose.yml")

	sonarEp := endpoint.NewEndpoint().
		WithHost("localhost").
		WithPort(sonarqubePort).
		WithHealthPath(sonarqubeHealth).
		WithHealthType("http").
		WithRequired(true).
		WithEnabled(true).
		WithComposeFile(composeFile).
		WithServiceName("sonarqube").
		WithTimeout(30 * time.Second).
		WithRetryCount(defaultRetries).
		Build()

	postgresEp := endpoint.NewEndpoint().
		WithHost("localhost").
		WithPort("5432").
		WithHealthType("tcp").
		WithEnabled(true).
		WithRequired(false).
		WithComposeFile(composeFile).
		WithServiceName("postgres").
		WithTimeout(15 * time.Second).
		WithRetryCount(10).
		Build()

	endpoints := map[string]endpoint.ServiceEndpoint{
		"sonarqube": sonarEp,
		"postgres":  postgresEp,
	}

	checker := health.NewDefaultChecker()
	mgr := boot.NewBootManager(
		endpoints,
		boot.WithRuntime(rt),
		boot.WithHealthChecker(checker),
		boot.WithProjectDir(projectDir),
	)

	switch action {
	case "start":
		log.Println("security-scan: booting SonarQube via Containers BootManager...")
		summary, err := mgr.BootAll(ctx)
		if err != nil {
			return fmt.Errorf("BootAll failed: %w", err)
		}
		log.Printf("security-scan: SonarQube boot complete — started=%d skipped=%d failed=%d",
			summary.Started, summary.Skipped, summary.Failed)
		if summary.Failed > 0 {
			return fmt.Errorf("one or more required services failed to start")
		}
		log.Printf("security-scan: SonarQube ready at http://localhost:%s", sonarqubePort)
	case "status":
		target := health.HealthTarget{
			Name:    "sonarqube",
			Host:    "localhost",
			Port:    sonarqubePort,
			Path:    sonarqubeHealth,
			Type:    health.HealthHTTP,
			Timeout: 10 * time.Second,
		}
		result := checker.Check(ctx, target)
		if result.Healthy {
			fmt.Printf("SonarQube: healthy (checked in %v)\n", result.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("SonarQube: unhealthy — %s\n", result.Error)
			os.Exit(1)
		}
	case "stop":
		return fmt.Errorf("stop action not yet implemented; use 'make scan-stop' or 'docker-compose -f <file> down' (TODO: wire ComposeOrchestrator.Down())")
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	return nil
}

func handleSnyk(ctx context.Context, projectDir string, rt runtime.ContainerRuntime, action string) error {
	composeFile := filepath.Join(projectDir, "docker", "security", "snyk", "docker-compose.yml")

	snykEp := endpoint.NewEndpoint().
		WithEnabled(true).
		WithRequired(false).
		WithComposeFile(composeFile).
		WithServiceName("snyk-full").
		WithProfile("full").
		WithTimeout(30 * time.Second).
		WithRetryCount(5).
		Build()

	endpoints := map[string]endpoint.ServiceEndpoint{
		"snyk": snykEp,
	}

	checker := health.NewDefaultChecker()
	mgr := boot.NewBootManager(
		endpoints,
		boot.WithRuntime(rt),
		boot.WithHealthChecker(checker),
		boot.WithProjectDir(projectDir),
	)

	switch action {
	case "start":
		log.Println("security-scan: starting Snyk container via Containers BootManager...")
		summary, err := mgr.BootAll(ctx)
		if err != nil {
			return fmt.Errorf("BootAll failed: %w", err)
		}
		log.Printf("security-scan: Snyk boot complete — started=%d skipped=%d failed=%d",
			summary.Started, summary.Skipped, summary.Failed)
	case "status":
		fmt.Println("Snyk: container-based (no persistent health endpoint — check docker ps)")
	case "stop":
		return fmt.Errorf("stop action not yet implemented; use 'make scan-stop' or 'docker-compose -f <file> down' (TODO: wire ComposeOrchestrator.Down())")
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	return nil
}

// resolveProjectDir returns the HelixCode project directory.
// When running via `go run ./cmd/security-scan` the working directory is the
// module root; this function validates and returns it.
func resolveProjectDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Verify it looks like the HelixCode project root (sanity check)
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return "", fmt.Errorf("go.mod not found in %s — run from HelixCode module root", dir)
	}
	return dir, nil
}
