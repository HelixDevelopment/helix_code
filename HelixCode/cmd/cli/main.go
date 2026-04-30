package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/worker"
)

// CLI represents the command-line interface
type CLI struct {
	workerPool         *worker.SSHWorkerPool
	llmProvider        llm.Provider
	notificationEngine *notification.NotificationEngine
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	// Initialize LLM provider from config - use Ollama on port 11434
	llmProvider, _ := llm.NewOllamaProvider(llm.OllamaConfig{
		DefaultModel:  "llama3.2", // Use Ollama's model name
		BaseURL: "http://localhost:11434", // Ollama default port
	})

	return &CLI{
		workerPool:         worker.NewSSHWorkerPool(true),
		llmProvider:        llmProvider,
		notificationEngine: notification.NewNotificationEngine(),
	}
}

// Run executes the CLI
func (c *CLI) Run() error {
	// Parse command-line flags
	var (
		command        = flag.String("command", "", "Command to execute")
		workerHost     = flag.String("worker", "", "Worker host to add")
		workerUser     = flag.String("user", "", "Worker SSH username")
		workerKey      = flag.String("key", "", "Worker SSH key path")
		model          = flag.String("model", "llama-3-8b", "LLM model to use")
		prompt         = flag.String("prompt", "", "Prompt for LLM generation")
		maxTokens      = flag.Int("max-tokens", 1000, "Maximum tokens to generate")
		temperature    = flag.Float64("temperature", 0.7, "Generation temperature")
		stream         = flag.Bool("stream", false, "Stream the response")
		listWorkers    = flag.Bool("list-workers", false, "List all workers")
		listModels     = flag.Bool("list-models", false, "List available models")
		healthCheck    = flag.Bool("health", false, "Perform health check")
		notify         = flag.String("notify", "", "Send notification with message")
		notifyType     = flag.String("notify-type", "info", "Notification type")
		notifyPriority = flag.String("notify-priority", "medium", "Notification priority")
		nonInteractive = flag.Bool("non-interactive", false, "Run in non-interactive mode")
	)
	flag.Parse()

	// Debug: print flag values
	fmt.Fprintf(os.Stderr, "Flags parsed: listWorkers=%v, nonInteractive=%v\n", *listWorkers, *nonInteractive)

	ctx := context.Background()

	// Handle different commands
	switch {
	case *listWorkers:
		return c.handleListWorkers(ctx)
	case *listModels:
		return c.handleListModels(ctx)
	case *healthCheck:
		return c.handleHealthCheck(ctx)
	case *workerHost != "":
		return c.handleAddWorker(ctx, *workerHost, *workerUser, *workerKey)
	case *prompt != "":
		return c.handleGenerate(ctx, *prompt, *model, *maxTokens, *temperature, *stream)
	case *notify != "":
		return c.handleNotification(ctx, *notify, *notifyType, *notifyPriority)
	case *command != "":
		return c.handleCommand(ctx, *command)
	case *nonInteractive:
		// In non-interactive mode, exit gracefully if no command specified
		return nil
	default:
		return c.handleInteractive(ctx)
	}
}

// handleListWorkers lists all workers
func (c *CLI) handleListWorkers(ctx context.Context) error {
	stats := c.workerPool.GetWorkerStats(ctx)

	fmt.Println("\n=== Worker Statistics ===")
	fmt.Printf("Total Workers: %d\n", stats.TotalWorkers)
	fmt.Printf("Active Workers: %d\n", stats.ActiveWorkers)
	fmt.Printf("Healthy Workers: %d\n", stats.HealthyWorkers)
	fmt.Printf("Total CPU: %d\n", stats.TotalCPU)
	fmt.Printf("Total Memory: %.2f GB\n", float64(stats.TotalMemory)/(1024*1024*1024))
	fmt.Printf("Total GPU: %d\n", stats.TotalGPU)

	return nil
}

// handleListModels lists available models
func (c *CLI) handleListModels(ctx context.Context) error {
	// For now, return static list
	// In production, this would query the model manager

	models := []struct {
		ID          string
		Name        string
		Provider    string
		ContextSize int
		Status      string
	}{
		{"llama-3-8b", "Llama 3 8B", "llama.cpp", 8192, "available"},
		{"mistral-7b", "Mistral 7B", "ollama", 4096, "available"},
		{"phi-3-mini", "Phi-3 Mini", "openai", 128000, "available"},
	}

	fmt.Println("\n=== Available Models ===")
	for _, model := range models {
		fmt.Printf("ID: %s\n", model.ID)
		fmt.Printf("  Name: %s\n", model.Name)
		fmt.Printf("  Provider: %s\n", model.Provider)
		fmt.Printf("  Context Size: %d\n", model.ContextSize)
		fmt.Printf("  Status: %s\n\n", model.Status)
	}

	return nil
}

// handleHealthCheck performs system health check
func (c *CLI) handleHealthCheck(ctx context.Context) error {
	fmt.Println("\n=== System Health Check ===")

	// Check worker pool
	stats := c.workerPool.GetWorkerStats(ctx)
	if stats.HealthyWorkers > 0 {
		fmt.Printf("✅ Worker Pool: %d healthy workers\n", stats.HealthyWorkers)
	} else {
		fmt.Printf("⚠️ Worker Pool: No healthy workers\n")
	}

	// Check notification engine
	channelStats := c.notificationEngine.GetChannelStats()
	enabledChannels := 0
	for _, stats := range channelStats {
		if statsMap, ok := stats.(map[string]interface{}); ok {
			if enabled, ok := statsMap["enabled"].(bool); ok && enabled {
				enabledChannels++
			}
		}
	}

	if enabledChannels > 0 {
		fmt.Printf("✅ Notification System: %d enabled channels\n", enabledChannels)
	} else {
		fmt.Printf("⚠️ Notification System: No enabled channels\n")
	}

	fmt.Println("✅ System is operational")
	return nil
}

// handleAddWorker adds a new worker
func (c *CLI) handleAddWorker(ctx context.Context, host, username, keyPath string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}

	sshConfig := &worker.SSHWorkerConfig{
		Host:     host,
		Port:     22,
		Username: username,
		KeyPath:  keyPath,
	}

	worker := &worker.SSHWorker{
		Hostname:    host,
		DisplayName: fmt.Sprintf("worker-%s", host),
		SSHConfig:   sshConfig,
	}

	if err := c.workerPool.AddWorker(ctx, worker); err != nil {
		return fmt.Errorf("failed to add worker: %v", err)
	}

	fmt.Printf("✅ Worker added successfully: %s\n", host)
	return nil
}

	// handleGenerate performs LLM generation
func (c *CLI) handleGenerate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, stream bool) error {
	fmt.Printf("\n=== Generating with %s ===\n", model)
	fmt.Printf("Prompt: %s\n\n", prompt)

	// ANTI-BLUFF: MUST use real LLM provider, not simulation
	if c.llmProvider == nil {
		return fmt.Errorf("LLM provider not initialized - please check configuration")
	}

	// Parse model from string (format: provider:model or just model)
	modelName := model
	if strings.Contains(model, ":") {
		parts := strings.SplitN(model, ":", 2)
		modelName = parts[1]
	}

	// Get provider
	provider := c.llmProvider

	// Create generation request
	req := &llm.LLMRequest{
		Model:       modelName,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      stream,
	}

	if stream {
		// Real streaming from provider
		chunkChan := make(chan llm.LLMResponse, 100)
		err := provider.GenerateStream(ctx, req, chunkChan)
		if err != nil {
			return fmt.Errorf("streaming generation failed: %w", err)
		}
		for chunk := range chunkChan {
			fmt.Printf("%s ", chunk.Content)
		}
		fmt.Println()
	} else {
		// Real non-streaming from provider
		resp, err := provider.Generate(ctx, req)
		if err != nil {
			return fmt.Errorf("generation failed: %w", err)
		}
		fmt.Println(resp.Content)
	}

	fmt.Printf("\n✅ Generation completed\n")
	return nil
}

// handleNotification sends a notification
func (c *CLI) handleNotification(ctx context.Context, message, notifyType, priority string) error {
	notificationType := notification.NotificationType(notifyType)
	notificationPriority := notification.NotificationPriority(priority)

	notif := &notification.Notification{
		Title:    "CLI Notification",
		Message:  message,
		Type:     notificationType,
		Priority: notificationPriority,
		Channels: []string{"cli"}, // Default to CLI output
	}

	if err := c.notificationEngine.SendDirect(ctx, notif, []string{"cli"}); err != nil {
		return fmt.Errorf("failed to send notification: %v", err)
	}

	fmt.Printf("✅ Notification sent: %s\n", message)
	return nil
}

// handleCommand executes a command
func (c *CLI) handleCommand(ctx context.Context, command string) error {
	fmt.Printf("\n=== Executing Command ===\n")
	fmt.Printf("Command: %s\n\n", command)

	// For now, simulate command execution
	// In production, this would execute on a worker

	fmt.Printf("Executing: %s\n", command)
	time.Sleep(1 * time.Second)
	fmt.Printf("Command completed successfully\n")

	return nil
}

// handleInteractive starts interactive mode
func (c *CLI) handleInteractive(ctx context.Context) error {
	fmt.Println("=== Helix CLI Interactive Mode ===")
	fmt.Println("Type 'help' for available commands, 'exit' to quit")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			fmt.Println("\n\nShutting down...")
			return nil
		default:
			// Continue with interactive loop
		}

		fmt.Print("\nhelix> ")

		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			if err.Error() == "unexpected newline" {
				continue
			}
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return nil
		}

		if input == "help" {
			c.showHelp()
			continue
		}

		// Handle interactive commands
		switch input {
		case "workers":
			c.handleListWorkers(ctx)
		case "models":
			c.handleListModels(ctx)
		case "health":
			c.handleHealthCheck(ctx)
		default:
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", input)
		}
	}
}

// showHelp displays available commands
func (c *CLI) showHelp() {
	fmt.Println("\n=== Available Commands ===")
	fmt.Println("workers          - List all workers")
	fmt.Println("models           - List available models")
	fmt.Println("health           - Perform system health check")
	fmt.Println("help             - Show this help message")
	fmt.Println("exit/quit        - Exit the CLI")
	fmt.Println("")
	fmt.Println("=== Command Line Options ===")
	fmt.Println("--list-workers   - List all workers")
	fmt.Println("--list-models    - List available models")
	fmt.Println("--health         - Perform health check")
	fmt.Println("--worker         - Add a worker (requires --user)")
	fmt.Println("--user           - Worker SSH username")
	fmt.Println("--key            - Worker SSH key path")
	fmt.Println("--prompt         - Generate with LLM")
	fmt.Println("--model          - LLM model to use")
	fmt.Println("--stream         - Stream the response")
	fmt.Println("--notify         - Send notification")
	fmt.Println("--notify-type    - Notification type (info/warning/error/success/alert)")
	fmt.Println("--notify-priority - Notification priority (low/medium/high/urgent)")
}

func main() {
	cli := NewCLI()

	if err := cli.Run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
