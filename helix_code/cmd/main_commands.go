package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/spf13/cobra"
)

// mainCmd represents the main command that starts everything
var mainCmd = &cobra.Command{
	Use:   "start",
	Short: trc("cmd_start_short", nil),
	Long:  trc("cmd_start_long", nil),
	Run:   runMainStart,
}

// autoCmd represents the auto command
var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: trc("cmd_auto_short", nil),
	Long:  trc("cmd_auto_long", nil),
	Run:   runAutoMode,
}

func init() {
	rootCmd.AddCommand(mainCmd)
	rootCmd.AddCommand(autoCmd)

	// Add flags for main command
	mainCmd.Flags().Bool("auto", true, trc("cmd_start_flag_auto", nil))
	mainCmd.Flags().Bool("monitor", true, trc("cmd_start_flag_monitor", nil))
	mainCmd.Flags().Bool("optimize", true, trc("cmd_start_flag_optimize", nil))
	mainCmd.Flags().Duration("check-interval", 30*time.Second, trc("cmd_start_flag_check_interval", nil))
}

func runMainStart(cmd *cobra.Command, args []string) {
	ctx0 := context.Background()
	fmt.Println(tr(ctx0, "cmd_start_banner", nil))
	fmt.Println(tr(ctx0, "cmd_start_zerotouch", nil))
	fmt.Println()

	// Create context with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create auto-LLM manager
	fmt.Println(tr(ctx, "cmd_start_init_manager", nil))
	manager := llm.NewAutoLLMManager("")

	// Initialize system
	if err := manager.Initialize(ctx); err != nil {
		fmt.Println(tr(ctx, "cmd_init_failed", map[string]any{"Error": err.Error()}))
		os.Exit(1)
	}

	fmt.Println(tr(ctx, "cmd_start_manager_ready", nil))

	// Start automated system
	if err := manager.Start(ctx); err != nil {
		fmt.Println(tr(ctx, "cmd_start_failed", map[string]any{"Error": err.Error()}))
		os.Exit(1)
	}

	fmt.Println(tr(ctx, "cmd_start_llm_started", nil))
	fmt.Println()

	// Show status
	showProviderStatus(manager)

	// Start main server in background
	go startMainServer(manager)

	// Start monitoring dashboard
	go startMonitoringDashboard(manager)

	fmt.Println(tr(ctx, "cmd_start_running", nil))
	fmt.Println()
	fmt.Println(tr(ctx, "cmd_start_endpoints_header", nil))
	showRunningEndpoints(manager)
	fmt.Println()
	fmt.Println(tr(ctx, "cmd_start_mgmt_header", nil))
	fmt.Println(tr(ctx, "cmd_start_mgmt_status", nil))
	fmt.Println(tr(ctx, "cmd_start_mgmt_logs", nil))
	fmt.Println(tr(ctx, "cmd_start_mgmt_monitor", nil))
	fmt.Println()
	fmt.Println(tr(ctx, "cmd_press_ctrlc_graceful", nil))

	// Wait for signals
	select {
	case <-sigChan:
		fmt.Println("\n" + tr(ctx, "cmd_shutdown_signal", nil))
	case <-ctx.Done():
		fmt.Println("\n" + tr(ctx, "cmd_shutdown_ctx_cancelled", nil))
	}

	// Graceful shutdown
	if err := manager.Stop(); err != nil {
		fmt.Println(tr(ctx, "cmd_shutdown_error", map[string]any{"Error": err.Error()}))
	}

	fmt.Println(tr(ctx, "cmd_start_stopped", nil))
}

func runAutoMode(cmd *cobra.Command, args []string) {
	ctx0 := context.Background()
	fmt.Println(tr(ctx0, "cmd_auto_banner", nil))
	fmt.Println(tr(ctx0, "cmd_auto_zerotouch", nil))
	fmt.Println()

	// Create context for auto mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create auto-LLM manager
	manager := llm.NewAutoLLMManager("")

	// Initialize and start in background
	fmt.Println(tr(ctx, "cmd_auto_init_system", nil))
	if err := manager.Initialize(ctx); err != nil {
		fmt.Println(tr(ctx, "cmd_init_failed", map[string]any{"Error": err.Error()}))
		os.Exit(1)
	}

	fmt.Println(tr(ctx, "cmd_auto_start_operations", nil))
	if err := manager.Start(ctx); err != nil {
		fmt.Println(tr(ctx, "cmd_start_failed", map[string]any{"Error": err.Error()}))
		os.Exit(1)
	}

	fmt.Println(tr(ctx, "cmd_auto_mode_active", nil))
	fmt.Println()

	// Start all background processes
	go runBackgroundProcesses(manager)

	// Show initial status
	showAutomationStatus(manager)

	fmt.Println(tr(ctx, "cmd_auto_mode_running", nil))
	fmt.Println(tr(ctx, "cmd_auto_web_dashboard", nil))
	fmt.Println(tr(ctx, "cmd_auto_api_docs", nil))
	fmt.Println()
	fmt.Println(tr(ctx, "cmd_auto_bg_header", nil))
	fmt.Println(tr(ctx, "cmd_auto_bg_install", nil))
	fmt.Println(tr(ctx, "cmd_auto_bg_health", nil))
	fmt.Println(tr(ctx, "cmd_auto_bg_optimize", nil))
	fmt.Println(tr(ctx, "cmd_auto_bg_updates", nil))
	fmt.Println(tr(ctx, "cmd_auto_bg_recovery", nil))
	fmt.Println()
	fmt.Println(tr(ctx, "cmd_auto_no_action", nil))
	fmt.Println(tr(ctx, "cmd_press_ctrlc_stop", nil))

	// Run forever with automation
	select {
	case <-sigChan:
		fmt.Println("\n" + tr(ctx, "cmd_auto_shutdown", nil))
	case <-ctx.Done():
		fmt.Println("\n" + tr(ctx, "cmd_auto_stopped_signal", nil))
	}

	// Stop automation
	if err := manager.Stop(); err != nil {
		fmt.Println(tr(ctx, "cmd_auto_stop_error", map[string]any{"Error": err.Error()}))
	}

	fmt.Println(tr(ctx, "cmd_auto_system_stopped", nil))
}

func showProviderStatus(manager *llm.AutoLLMManager) {
	ctx := context.Background()
	fmt.Println(tr(ctx, "cmd_provider_status_header", nil))
	fmt.Println(strings.Repeat("─", 50))

	status := manager.GetStatus()
	for name, provider := range status {
		statusIcon := getStatusIcon(provider.Status)
		healthIcon := getHealthIcon(provider.Health.IsHealthy)
		port := provider.DefaultPort

		fmt.Println(tr(ctx, "cmd_provider_status_line", map[string]any{
			"StatusIcon": statusIcon,
			"Name":       fmt.Sprintf("%-12s", name),
			"HealthIcon": healthIcon,
			"Port":       port,
		}))
	}
	fmt.Println()
}

func showRunningEndpoints(manager *llm.AutoLLMManager) {
	endpoints := manager.GetRunningEndpoints()
	for i, endpoint := range endpoints {
		fmt.Printf("  %d. %s\n", i+1, endpoint)
	}
}

func showAutomationStatus(manager *llm.AutoLLMManager) {
	ctx := context.Background()
	status := manager.GetStatus()

	fmt.Println(tr(ctx, "cmd_automation_status_header", nil))
	fmt.Println(strings.Repeat("─", 50))

	installedCount := 0
	runningCount := 0
	healthyCount := 0

	for _, provider := range status {
		if provider.Status == "installed" || provider.Status == "running" {
			installedCount++
		}
		if provider.Status == "running" {
			runningCount++
		}
		if provider.Health.IsHealthy {
			healthyCount++
		}
	}

	fmt.Println(tr(ctx, "cmd_automation_installed", map[string]any{"Count": installedCount}))
	fmt.Println(tr(ctx, "cmd_automation_running", map[string]any{"Count": runningCount}))
	fmt.Println(tr(ctx, "cmd_automation_healthy", map[string]any{"Count": healthyCount}))
	fmt.Println(tr(ctx, "cmd_automation_auto_optimize", nil))
	fmt.Println(tr(ctx, "cmd_automation_auto_updates", nil))
	fmt.Println(tr(ctx, "cmd_automation_auto_recovery", nil))
	fmt.Println()
}

func runBackgroundProcesses(manager *llm.AutoLLMManager) {
	// This would run various background processes
	// For demo purposes, just show periodic status updates

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Show brief status update
			status := manager.GetStatus()
			running := 0
			for _, provider := range status {
				if provider.Status == "running" {
					running++
				}
			}

			fmt.Print("\r" + tr(context.Background(), "cmd_automation_bg_status", map[string]any{"Count": running}))
		}
	}
}

func startMainServer(manager *llm.AutoLLMManager) {
	// This would start the main HelixCode server
	// For demo purposes, just log
	fmt.Println(tr(context.Background(), "cmd_main_server_started", nil))
}

func startMonitoringDashboard(manager *llm.AutoLLMManager) {
	// This would start the monitoring dashboard
	// For demo purposes, just log
	fmt.Println(tr(context.Background(), "cmd_monitoring_dashboard_started", nil))
}

func getHealthIcon(isHealthy bool) string {
	if isHealthy {
		return "✅"
	}
	return "❌"
}
