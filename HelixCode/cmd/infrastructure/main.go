package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
)

const (
	banner = `
██╗  ██╗███████╗██╗   ██╗    ███╗   ███╗██╗ ██████╗ ██████╗ ██████╗ ███████╗
██║  ██║██╔════╝╚██╗ ██╔╝    ████╗ ████║██║██╔════╝██╔═══██╗██╔══██╗██╔════╝
███████║█████╗   ╚████╔╝     ██╔████╔██║██║██║     ██║   ██║██║  ██║█████╗  
██╔══██║██╔══╝    ╚██╔╝      ██║╚██╔╝██║██║██║     ██║   ██║██║  ██║██╔══╝  
██║  ██║███████╗   ██║       ██║ ╚═╝ ██║██║╚██████╗╚██████╔╝██████╔╝███████╗
╚═╝  ╚═╝╚══════╝   ╚═╝       ╚═╝     ╚═╝╚═╝ ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝
                                                                            
   Infrastructure Orchestration — Powered by Containers Module
   
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
	
	rt, err := runtime.AutoDetect(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to detect container runtime: %w", err)
	}
	
	logger := logging.NewSlogAdapter()
	
	return &InfrastructureManager{
		mode:       mode,
		composeDir: composeDir,
		runtime:    rt,
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
			WithDescription("PostgreSQL 15 database").
			Build(),
		"redis": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("6379").
			WithHealthType("tcp").
			WithRequired(true).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.yml")).
			WithServiceName("redis").
			WithDescription("Redis 7 cache").
			Build(),
		"helixcode-server": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("8080").
			WithHealthType("http").
			WithHealthPath("/health").
			WithRequired(true).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.yml")).
			WithServiceName("helixcode-server").
			WithDescription("HelixCode main server").
			Build(),
		"prometheus": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("9090").
			WithHealthType("http").
			WithHealthPath("/-/healthy").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.yml")).
			WithServiceName("prometheus").
			WithDescription("Prometheus monitoring").
			Build(),
		"grafana": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("3000").
			WithHealthType("http").
			WithHealthPath("/api/health").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.yml")).
			WithServiceName("grafana").
			WithDescription("Grafana dashboards").
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
			WithDescription("PostgreSQL test database").
			Build(),
		"redis-test": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("6380").
			WithHealthType("tcp").
			WithRequired(true).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("redis").
			WithDescription("Redis test cache").
			Build(),
		"ollama": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("11434").
			WithHealthType("http").
			WithHealthPath("/api/tags").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("ollama").
			WithDescription("Ollama local LLM").
			Build(),
		"memcached": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("11211").
			WithHealthType("tcp").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("memcached").
			WithDescription("Memcached cache").
			Build(),
		"cognee": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("8000").
			WithHealthType("http").
			WithHealthPath("/health").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("cognee").
			WithDescription("Cognee memory system").
			Build(),
		"chromadb": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("8001").
			WithHealthType("http").
			WithHealthPath("/api/v1/heartbeat").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("chromadb").
			WithDescription("ChromaDB vector store").
			Build(),
		"qdrant": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("6333").
			WithHealthType("http").
			WithHealthPath("/healthz").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("qdrant").
			WithDescription("Qdrant vector store").
			Build(),
		"weaviate": endpoint.NewEndpoint().
			WithHost("localhost").
			WithPort("8080").
			WithHealthType("http").
			WithHealthPath("/v1/.well-known/ready").
			WithRequired(false).
			WithComposeFile(filepath.Join(im.composeDir, "docker-compose.test.yml")).
			WithServiceName("weaviate").
			WithDescription("Weaviate vector store").
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
		WithDescription("Mock LLM server").
		Build()
	
	full["selenium"] = endpoint.NewEndpoint().
		WithHost("localhost").
		WithPort("4444").
		WithHealthType("http").
		WithHealthPath("/wd/hub/status").
		WithRequired(false).
		WithComposeFile(filepath.Join(im.composeDir, "docker-compose.full-test.yml")).
		WithServiceName("selenium").
		WithDescription("Selenium browser automation").
		Build()
	
	return full
}

func (im *InfrastructureManager) Start() error {
	fmt.Print(banner)
	fmt.Printf("Starting HelixCode infrastructure in %s mode...\n", im.mode)
	fmt.Printf("Using container runtime: %s\n\n", im.runtime.Name())
	
	endpoints := im.defineEndpoints()
	
	var opts []boot.Option
	opts = append(opts, boot.WithRuntime(im.runtime))
	opts = append(opts, boot.WithLogger(im.logger))
	opts = append(opts, boot.WithHealthCheckRetries(3))
	opts = append(opts, boot.WithHealthCheckTimeout(30*time.Second))
	opts = append(opts, boot.WithParallelStartup(true))
	
	im.manager = boot.NewBootManager(endpoints, opts...)
	
	summary, err := im.manager.BootAll(im.ctx)
	if err != nil {
		return fmt.Errorf("failed to boot infrastructure: %w", err)
	}
	
	fmt.Printf("\n✅ Infrastructure started successfully!\n")
	fmt.Printf("   Started: %d services\n", summary.Started)
	fmt.Printf("   Failed:  %d services\n", summary.Failed)
	fmt.Printf("   Skipped: %d services\n", summary.Skipped)
	
	if summary.Failed > 0 {
		fmt.Printf("\n⚠️  Some services failed to start:\n")
		for name, errStr := range summary.Errors {
			fmt.Printf("   - %s: %s\n", name, errStr)
		}
	}
	
	fmt.Printf("\n📊 Service Status:\n")
	for name, ep := range endpoints {
		status := "✓"
		if !ep.Required {
			status = "~"
		}
		fmt.Printf("   %s %-20s %s (port %s)\n", status, name, ep.Description, ep.Port)
	}
	
	return nil
}

func (im *InfrastructureManager) Stop() error {
	fmt.Println("\n🛑 Stopping HelixCode infrastructure...")
	
	if im.manager == nil {
		return fmt.Errorf("infrastructure not started")
	}
	
	if err := im.manager.Shutdown(im.ctx); err != nil {
		return fmt.Errorf("failed to shutdown infrastructure: %w", err)
	}
	
	fmt.Println("✅ Infrastructure stopped successfully!")
	return nil
}

func (im *InfrastructureManager) Status() error {
	fmt.Println("\n📊 HelixCode Infrastructure Status")
	fmt.Printf("   Mode: %s\n", im.mode)
	fmt.Printf("   Runtime: %s\n", im.runtime.Name())
	
	if im.manager == nil {
		fmt.Println("   Status: NOT STARTED")
		return nil
	}
	
	health := im.manager.GetHealthStatus()
	fmt.Printf("   Status: RUNNING\n\n")
	
	for name, status := range health {
		emoji := "✅"
		if status != "healthy" {
			emoji = "❌"
		}
		fmt.Printf("   %s %-20s %s\n", emoji, name, status)
	}
	
	return nil
}

func (im *InfrastructureManager) Wait() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	fmt.Println("\n⏳ Infrastructure is running. Press Ctrl+C to stop...")
	
	<-sigChan
	fmt.Println("\n\nReceived shutdown signal...")
	return im.Stop()
}

func (im *InfrastructureManager) Close() {
	if im.cancel != nil {
		im.cancel()
	}
}

func printUsage() {
	fmt.Print(banner)
	fmt.Println("Usage:")
	fmt.Println("  helixcode-infra <command> [mode]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  start    Start infrastructure")
	fmt.Println("  stop     Stop infrastructure")
	fmt.Println("  status   Show infrastructure status")
	fmt.Println()
	fmt.Println("Modes:")
	fmt.Println("  production  Production services (PostgreSQL, Redis, Server)")
	fmt.Println("  testing     Testing services (Test DB, Ollama, Vector DBs)")
	fmt.Println("  full        All services (production + testing + extras)")
	fmt.Println()
	fmt.Println("Examples:")
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
			fmt.Printf("Unknown mode: %s\n\n", os.Args[2])
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
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}
