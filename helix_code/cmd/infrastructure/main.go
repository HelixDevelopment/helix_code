package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"dev.helix.code/cmd/infrastructure/i18n"
	containersadapter "dev.helix.code/internal/adapters/containers"
	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this CLI. Defaults to i18n.NoopTranslator{} (loud
// message-ID echo) so unit tests + ad-hoc invocations remain obvious.
// helix_code wires a real *i18nadapter.Translator at boot via
// SetTranslator (round-455 §11.4 anti-bluff sweep, 2026-05-20).
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — main()'s linear call graph does
// not warrant a constructor-injected struct.
var translator i18n.Translator = i18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr i18n.Translator) {
	if tr == nil {
		translator = i18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver used by every user-facing
// string emission in this file. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
func tr(ctx context.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = i18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}

const (
	banner = `
██╗  ██╗███████╗██╗   ██╗    ███╗   ███╗██╗ ██████╗ ██████╗ ██████╗ ███████╗
██║  ██║██╔════╝╚██╗ ██╔╝    ████╗ ████║██║██╔════╝██╔═══██╗██╔══██╗██╔════╝
███████║█████╗   ╚████╔╝     ██╔████╔██║██║██║     ██║   ██║██║  ██║█████╗  
██╔══██║██╔══╝    ╚██╔╝      ██║╚██╔╝██║██║██║     ██║   ██║██║  ██║██╔══╝  
██║  ██║███████╗   ██║       ██║ ╚═╝ ██║██║╚██████╗╚██████╔╝██████╔╝███████╗
╚═╝  ╚═╝╚══════╝   ╚═╝       ╚═╝     ╚═╝╚═╝ ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝
                                                                            
   Infrastructure Orchestration — Powered by containers Module
   
`
)

type InfraMode string

const (
	ModeProduction InfraMode = "production"
	ModeTesting    InfraMode = "testing"
	ModeFull       InfraMode = "full"
)

type InfrastructureManager struct {
	mode       InfraMode
	composeDir string
	manager    *boot.BootManager
	runtime    runtime.ContainerRuntime
	runtimeErr error
	logger     logging.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewInfrastructureManager(mode InfraMode) (*InfrastructureManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cwd, err := os.Getwd()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	composeDir := filepath.Join(cwd, "..")

	logger := logging.NewSlogAdapter(nil)

	// Detect a LOCAL container runtime. In remote-distribution mode the
	// local host only orchestrates over SSH (the heavy podman work runs
	// on the remote host), so a missing local runtime is NOT fatal —
	// it is recorded and Start() routes to the remote path. A nil
	// im.runtime is only an error if a local boot is actually attempted.
	rt, rtErr := runtime.AutoDetect(ctx)

	return &InfrastructureManager{
		mode:       mode,
		composeDir: composeDir,
		runtime:    rt,
		runtimeErr: rtErr,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

func (im *InfrastructureManager) defineEndpoints() map[string]endpoint.ServiceEndpoint {
	switch im.mode {
	case ModeProduction:
		return im.productionEndpoints()
	case ModeTesting:
		return im.testingEndpoints()
	case ModeFull:
		return im.fullEndpoints()
	default:
		return im.productionEndpoints()
	}
}

func (im *InfrastructureManager) productionEndpoints() map[string]endpoint.ServiceEndpoint {
	return map[string]endpoint.ServiceEndpoint{
		"postgres": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("5432").
			WithHealthType("tcp").
			WithRequired(true).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.yml")).
			WithServiceName("postgres").
			Build(),
		"redis": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("6379").
			WithHealthType("tcp").
			WithRequired(true).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.yml")).
			WithServiceName("redis").
			Build(),
		"helixcode-server": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("8080").
			WithHealthType("http").
			WithHealthPath("/health").
			WithRequired(true).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.yml")).
			WithServiceName("helixcode-server").
			Build(),
		"prometheus": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("9090").
			WithHealthType("http").
			WithHealthPath("/-/healthy").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.yml")).
			WithServiceName("prometheus").
			Build(),
		"grafana": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("3000").
			WithHealthType("http").
			WithHealthPath("/api/health").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.yml")).
			WithServiceName("grafana").
			Build(),
	}
}

func (im *InfrastructureManager) testingEndpoints() map[string]endpoint.ServiceEndpoint {
	return map[string]endpoint.ServiceEndpoint{
		"postgres-test": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("5433").
			WithHealthType("tcp").
			WithRequired(true).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("postgres").
			Build(),
		"redis-test": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("6380").
			WithHealthType("tcp").
			WithRequired(true).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("redis").
			Build(),
		"ollama": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("11434").
			WithHealthType("http").
			WithHealthPath("/api/tags").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("ollama").
			Build(),
		"memcached": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("11211").
			WithHealthType("tcp").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("memcached").
			Build(),
		"cognee": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("8000").
			WithHealthType("http").
			WithHealthPath("/health").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("cognee").
			Build(),
		"chromadb": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("8001").
			WithHealthType("http").
			WithHealthPath("/api/v1/heartbeat").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("chromadb").
			Build(),
		"qdrant": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("6333").
			WithHealthType("http").
			WithHealthPath("/healthz").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("qdrant").
			Build(),
		"weaviate": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("8080").
			WithHealthType("http").
			WithHealthPath("/v1/.well-known/ready").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("weaviate").
			Build(),
	}
}

func (im *InfrastructureManager) fullEndpoints() map[string]endpoint.ServiceEndpoint {
	prod := im.productionEndpoints()
	test := im.testingEndpoints()

	full := make(map[string]endpoint.ServiceEndpoint)
	for k, v := range prod {
		full[k] = v
	}
	for k, v := range test {
		full[k] = v
	}

	full["mock-llm"] = endpoint.NewEndpoint().
		WithHost("localhost").
		WithPort("8081").
		WithHealthType("http").
		WithHealthPath("/health").
		WithRequired(false).
		WithComposeFile(filepath.Join(im.composeDir, "docker-compose.full-test.yml")).
		WithServiceName("mock-llm").
		Build()

	full["selenium"] = endpoint.NewEndpoint().
		WithHost("localhost").
		WithPort("4444").
		WithHealthType("http").
		WithHealthPath("/wd/hub/status").
		WithRequired(false).
		WithComposeFile(filepath.Join(im.composeDir, "docker-compose.full-test.yml")).
		WithServiceName("selenium").
		Build()

	return full
}

// remoteEnvPath returns the absolute path to the containers submodule's
// CONST-045 .env, resolved relative to the compose directory (the
// meta-repo root when the binary runs from helix_code/).
func (im *InfrastructureManager) remoteEnvPath() string {
	return filepath.Join(im.composeDir, "submodules", "containers", ".env")
}

// fullTestComposeFile returns the absolute path to the full-test compose
// file (lives inside the inner helix_code module dir).
func (im *InfrastructureManager) fullTestComposeFile() string {
	return filepath.Join(im.composeDir, "helix_code", "docker-compose.full-test.yml")
}

// fullTestProjectRoot returns the directory the full-test compose file's
// relative build contexts resolve against.
func (im *InfrastructureManager) fullTestProjectRoot() string {
	return filepath.Join(im.composeDir, "helix_code")
}

// Start boots the infrastructure. When the containers submodule's .env
// has CONTAINERS_REMOTE_ENABLED=true AND mode is "full", the whole
// full-test System is distributed to the configured remote host(s) via
// the containers submodule's remote orchestration (§11.4.76). Otherwise
// it boots locally via BootManager (requires a local container runtime).
func (im *InfrastructureManager) Start() error {
	ctx := im.ctx

	// §11.4.76 remote-distribution path: load the CONST-045 config and,
	// if remote-enabled, route the full System to the remote host.
	if im.mode == ModeFull {
		adapter := containersadapter.NewAdapter()
		enabled, err := adapter.LoadRemoteConfig(im.remoteEnvPath())
		if err != nil {
			im.logger.Warn("remote config load failed (%v); falling back to local boot", err)
		} else if enabled {
			return im.startRemote(ctx, adapter)
		}
	}

	if im.runtime == nil {
		return fmt.Errorf("no local container runtime detected (%v) and remote distribution not enabled", im.runtimeErr)
	}

	fmt.Print(banner)
	fmt.Println(tr(ctx, "infra_start_starting", map[string]any{"Mode": string(im.mode)}))
	fmt.Printf("%s\n\n", tr(ctx, "infra_start_runtime", map[string]any{"Runtime": im.runtime.Name()}))

	endpoints := im.defineEndpoints()

	var opts []boot.BootManagerOption
	opts = append(opts, boot.WithRuntime(im.runtime))
	opts = append(opts, boot.WithLogger(im.logger))

	im.manager = boot.NewBootManager(endpoints, opts...)

	summary, err := im.manager.BootAll(im.ctx)
	if err != nil {
		return fmt.Errorf("failed to boot infrastructure: %w", err)
	}

	fmt.Printf("\n%s\n", tr(ctx, "infra_start_success", nil))
	fmt.Println(tr(ctx, "infra_start_count_started", map[string]any{"Count": summary.Started}))
	fmt.Println(tr(ctx, "infra_start_count_failed", map[string]any{"Count": summary.Failed}))
	fmt.Println(tr(ctx, "infra_start_count_skipped", map[string]any{"Count": summary.Skipped}))

	if summary.Failed > 0 {
		fmt.Printf("\n%s\n", tr(ctx, "infra_start_some_failed", nil))
		for name, result := range summary.Results {
			if result == nil || result.Status != "failed" {
				continue
			}
			errStr := "unknown error"
			if result.Error != nil {
				errStr = result.Error.Error()
			}
			fmt.Printf("   - %s: %s\n", name, errStr)
		}
	}

	fmt.Printf("\n%s\n", tr(ctx, "infra_start_service_status_heading", nil))
	for name, ep := range endpoints {
		status := "✓"
		if !ep.Required {
			status = "~"
		}
		fmt.Printf("   %s %-20s %s (port %s)\n", status, name, ep.ServiceName, ep.Port)
	}

	return nil
}

// startRemote distributes the full-test compose stack to the configured
// remote host(s) via the containers submodule's RemoteComposeOrchestrator
// (SCP compose file + build contexts, then `podman compose up -d --build`
// remotely). §11.4.76 — reuse the submodule, never a hand-rolled
// `podman compose up`.
func (im *InfrastructureManager) startRemote(ctx context.Context, adapter *containersadapter.Adapter) error {
	fmt.Print(banner)
	hosts := adapter.RemoteHostNames()
	fmt.Printf("Remote distribution ENABLED — target host(s): %s\n", strings.Join(hosts, ", "))
	fmt.Println("Distributing full-test System via containers submodule (SCP + remote compose up)...")

	composeFile := im.fullTestComposeFile()
	projectRoot := im.fullTestProjectRoot()

	// Default-profile build-context services live under these dirs:
	//   tests/e2e/mocks       → mock-llm-server, mock-slack
	//   tests/infrastructure  → ssh-server, ssh-worker-1/2/3
	// The whole-repo context "." (helixcode-server) is profile-gated
	// (profile "server") and therefore OFF by default, so the 27 GB repo
	// root is never shipped (submodule CLAUDE.md gotcha #4).
	buildContextDirs := []string{
		"tests/e2e/mocks",
		"tests/infrastructure",
	}

	workDir, err := adapter.RemoteComposeUp(ctx, composeFile, projectRoot, nil, buildContextDirs)
	if err != nil {
		return fmt.Errorf("remote compose up: %w", err)
	}

	fmt.Printf("\nRemote full-test System distributed. Remote work dir: %s\n", workDir)
	fmt.Printf("Verify on host: ssh %s podman ps\n", strings.Join(hosts, " / "))
	return nil
}

func (im *InfrastructureManager) Stop() error {
	ctx := im.ctx
	fmt.Printf("\n%s\n", tr(ctx, "infra_stop_stopping", nil))

	if im.manager == nil {
		return fmt.Errorf("infrastructure not started")
	}

	if err := im.manager.Shutdown(im.ctx); err != nil {
		return fmt.Errorf("failed to shutdown infrastructure: %w", err)
	}

	fmt.Println(tr(ctx, "infra_stop_success", nil))
	return nil
}

func (im *InfrastructureManager) Status() error {
	ctx := im.ctx
	fmt.Printf("\n%s\n", tr(ctx, "infra_status_heading", nil))
	fmt.Printf("   Mode: %s\n", im.mode)
	runtimeName := "<remote/none>"
	if im.runtime != nil {
		runtimeName = im.runtime.Name()
	}
	fmt.Printf("   Runtime: %s\n", runtimeName)

	if im.manager == nil {
		fmt.Println(tr(ctx, "infra_status_not_started", nil))
		return nil
	}

	health := im.manager.HealthCheckAll(im.ctx)
	fmt.Printf("%s\n\n", tr(ctx, "infra_status_running", nil))

	for name, healthErr := range health {
		emoji := "✅"
		status := "healthy"
		if healthErr != nil {
			emoji = "❌"
			status = healthErr.Error()
		}
		fmt.Printf("   %s %-20s %s\n", emoji, name, status)
	}

	return nil
}

func (im *InfrastructureManager) Wait() error {
	// In remote-distribution mode the services run detached on the
	// remote host(s); there is no local BootManager to keep alive, so
	// the orchestrator returns immediately rather than blocking on a
	// signal it would never use.
	if im.manager == nil {
		return nil
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("\n%s\n", tr(im.ctx, "infra_wait_running", nil))

	<-sigChan
	fmt.Printf("\n\n%s\n", tr(im.ctx, "infra_wait_received_signal", nil))
	return im.Stop()
}

func (im *InfrastructureManager) Close() {
	if im.cancel != nil {
		im.cancel()
	}
}

func printUsage() {
	ctx := context.Background()
	fmt.Print(banner)
	fmt.Println(tr(ctx, "infra_usage_heading", nil))
	fmt.Println(tr(ctx, "infra_usage_synopsis", nil))
	fmt.Println()
	fmt.Println(tr(ctx, "infra_usage_commands_heading", nil))
	fmt.Println(tr(ctx, "infra_usage_command_start", nil))
	fmt.Println(tr(ctx, "infra_usage_command_stop", nil))
	fmt.Println(tr(ctx, "infra_usage_command_status", nil))
	fmt.Println()
	fmt.Println(tr(ctx, "infra_usage_modes_heading", nil))
	fmt.Println(tr(ctx, "infra_usage_mode_production", nil))
	fmt.Println(tr(ctx, "infra_usage_mode_testing", nil))
	fmt.Println(tr(ctx, "infra_usage_mode_full", nil))
	fmt.Println()
	fmt.Println(tr(ctx, "infra_usage_examples_heading", nil))
	fmt.Println("  helixcode-infra start production")
	fmt.Println("  helixcode-infra start testing")
	fmt.Println("  helixcode-infra start full")
	fmt.Println("  helixcode-infra status")
	fmt.Println("  helixcode-infra stop")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	var mode InfraMode = ModeProduction
	if len(os.Args) >= 3 {
		switch os.Args[2] {
		case "production":
			mode = ModeProduction
		case "testing":
			mode = ModeTesting
		case "full":
			mode = ModeFull
		default:
			fmt.Printf("%s\n\n", tr(context.Background(), "infra_error_unknown_mode", map[string]any{"Mode": os.Args[2]}))
			printUsage()
			os.Exit(1)
		}
	}

	if command == "help" || command == "--help" || command == "-h" {
		printUsage()
		os.Exit(0)
	}

	manager, err := NewInfrastructureManager(mode)
	if err != nil {
		log.Fatalf("Failed to create infrastructure manager: %v", err)
	}
	defer manager.Close()

	switch command {
	case "start":
		if err := manager.Start(); err != nil {
			log.Fatalf("Failed to start infrastructure: %v", err)
		}
		if err := manager.Wait(); err != nil {
			log.Fatalf("Failed to shutdown infrastructure: %v", err)
		}
	case "stop":
		if err := manager.Stop(); err != nil {
			log.Fatalf("Failed to stop infrastructure: %v", err)
		}
	case "status":
		if err := manager.Status(); err != nil {
			log.Fatalf("Failed to get status: %v", err)
		}
	default:
		fmt.Printf("%s\n\n", tr(context.Background(), "infra_error_unknown_command", map[string]any{"Command": command}))
		printUsage()
		os.Exit(1)
	}
}
