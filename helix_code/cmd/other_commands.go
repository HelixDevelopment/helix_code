package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/redis"
	"dev.helix.code/internal/server"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: trc("cmd_server_short", nil),
	Long:  trc("cmd_server_long", nil),
	Run: func(cmd *cobra.Command, args []string) {
		// Speed programme P2-T07: config.Get() caches the process config.
		cfg, err := config.Get()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
			return
		}

		var db *database.Database
		if cfg.Database.Host != "" {
			db, err = database.New(cfg.Database)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Database unavailable: %v\n", err)
			} else {
				defer db.Close()
			}
		}

		var rds *redis.Client
		if cfg.Redis.Enabled && cfg.Redis.Host != "" {
			rds, err = redis.NewClient(&cfg.Redis)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Redis unavailable: %v\n", err)
			} else {
				defer rds.Close()
			}
		}

		srv := server.New(cfg, db, rds)

		errChan := make(chan error, 1)
		go func() {
			if err := srv.Start(); err != nil {
				errChan <- err
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		select {
		case err := <-errChan:
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		case sig := <-quit:
			fmt.Printf("\nReceived %v, shutting down...\n", sig)
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
		}
		fmt.Println("Server stopped")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: trc("cmd_version_short", nil),
	Long:  trc("cmd_version_long", nil),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		fmt.Println(tr(ctx, "cmd_version_platform_name", nil))
		fmt.Println(tr(ctx, "cmd_version_version", map[string]any{"Version": "1.0.0"}))
		fmt.Println(tr(ctx, "cmd_version_build", map[string]any{"Build": "2025.01.20"}))
		fmt.Println(tr(ctx, "cmd_version_providers", map[string]any{"Total": 29, "Cloud": 18, "Local": 11}))
		fmt.Println(tr(ctx, "cmd_version_token_context", map[string]any{"Context": "2M"}))
		fmt.Println(tr(ctx, "cmd_version_license", map[string]any{"License": "MIT"}))
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate [prompt]",
	Short: "Generate code/text with AI",
	Long:  `Generate code or text using configured AI providers.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Please provide a prompt\n")
			return
		}
		prompt := args[0]

		// Speed programme P2-T07: config.Get() caches the process config.
		cfg, err := config.Get()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
			return
		}

		mgr := llm.NewModelManager()
		defaultProvider := cfg.LLM.DefaultProvider
		if defaultProvider == "" {
			fmt.Fprintf(os.Stderr, "No default LLM provider configured in config\n")
			fmt.Fprintf(os.Stderr, "Set llm.default_provider in config.yaml\n")
			return
		}

		modelInfo, err := mgr.SelectOptimalModel(llm.ModelSelectionCriteria{
			TaskType:          "text-generation",
			QualityPreference: "balanced",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "No models available: %v\n", err)
			return
		}

		entryKey := llm.ProviderType(defaultProvider)
		prov, err := mgr.GetProviderForModel(modelInfo.Name, entryKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Provider not available: %v\n", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		request := &llm.LLMRequest{
			Model: modelInfo.Name,
			Messages: []llm.Message{
				{Role: "user", Content: prompt},
			},
			MaxTokens:   4096,
			Temperature: 0.7,
		}
		response, err := prov.Generate(ctx, request)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Generation failed: %v\n", err)
			return
		}
		fmt.Println(response.Content)
	},
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run tests",
	Long:  `Run HelixCode tests including unit, integration, and E2E tests.`,
	Run: func(cmd *cobra.Command, args []string) {
		testArgs := []string{"test", "-v"}
		if len(args) > 0 {
			testArgs = append(testArgs, args...)
		} else {
			testArgs = append(testArgs, "./...")
		}
		c := exec.Command("go", testArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Tests failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Manage distributed workers",
	Long:  `Add, remove, and manage distributed computing workers.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Speed programme P2-T07: config.Get() caches the process config.
		cfg, err := config.Get()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
			return
		}

		if cfg.Database.Host == "" {
			fmt.Println("Worker management requires a configured database")
			fmt.Println("Set database.host in config.yaml")
			return
		}

		fmt.Printf("Workers config: health_ttl=%ds, max_concurrent=%d\n",
			cfg.Workers.HealthTTL, cfg.Workers.MaxConcurrentTasks)
		fmt.Println("Use worker subcommands: add, list, status, remove")
	},
}

var notifyCmd = &cobra.Command{
	Use:   "notify [message]",
	Short: "Send notifications",
	Long:  `Send notifications through configured channels (Slack, Discord, etc.).`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Please provide a message\n")
			return
		}
		message := args[0]

		engine := notification.NewNotificationEngine()

		slackWebhook := os.Getenv("HELIX_SLACK_WEBHOOK_URL")
		if slackWebhook != "" {
			engine.RegisterChannel(notification.NewSlackChannel(slackWebhook, "helixcode", "HelixCode"))
		}

		discordWebhook := os.Getenv("HELIX_DISCORD_WEBHOOK_URL")
		if discordWebhook != "" {
			engine.RegisterChannel(notification.NewDiscordChannel(discordWebhook))
		}

		tgBotToken := os.Getenv("HELIX_TELEGRAM_BOT_TOKEN")
		tgChatID := os.Getenv("HELIX_TELEGRAM_CHAT_ID")
		if tgBotToken != "" && tgChatID != "" {
			engine.RegisterChannel(notification.NewTelegramChannel(tgBotToken, tgChatID))
		}

		emailServer := os.Getenv("HELIX_EMAIL_SMTP_SERVER")
		emailUser := os.Getenv("HELIX_EMAIL_USERNAME")
		emailPass := os.Getenv("HELIX_EMAIL_PASSWORD")
		if emailServer != "" && emailUser != "" && emailPass != "" {
			engine.RegisterChannel(notification.NewEmailChannel(emailServer, 587, emailUser, emailPass, emailUser))
		}

		notif := &notification.Notification{
			Title:    "HelixCode CLI Notification",
			Message:  message,
			Type:     notification.NotificationTypeInfo,
			Priority: notification.NotificationPriorityMedium,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := engine.SendNotification(ctx, notif); err != nil {
			fmt.Fprintf(os.Stderr, "Notification failed: %v\n", err)
			return
		}
		fmt.Println("Notification dispatched")
	},
}
