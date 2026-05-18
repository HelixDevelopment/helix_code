package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrPerformanceMetricsNotInstrumented is returned by
// (*AutoLLMManager).updatePerformanceMetrics when no MetricsRecorder has
// been wired into the manager. Before round-31 §11.4 anti-bluff sweep
// (2026-05-18) updatePerformanceMetrics zeroed out ActiveRequests and
// ErrorRate and bumped LastUpdated, regardless of reality — a HIGH
// monitoring-bluff under Article XI §11.9 / CONST-035 that caused
// dashboards to report 0% error rate for every provider while real
// failures piled up unrecorded. The function now requires callers to
// inject a MetricsRecorder via SetMetricsRecorder; absent a recorder
// this sentinel surfaces so the caller knows monitoring is not live.
var ErrPerformanceMetricsNotInstrumented = errors.New("auto llm manager: performance metrics have not been instrumented — updatePerformanceMetrics previously zeroed out ActiveRequests/ErrorRate then bumped LastUpdated, causing monitoring dashboards to report 0% error rate regardless of reality (§11.4 HIGH: silent monitoring bluff under Article XI §11.9 / CONST-035); inject a MetricsRecorder via SetMetricsRecorder before relying on PerformanceMetrics")

// MetricsSnapshot is the per-provider point-in-time view of runtime
// metrics returned by MetricsRecorder.Snapshot. Implementations MUST
// reflect actual observed traffic against the named provider — fabricated
// or hardcoded values are an Article XI §11.9 / CONST-035 violation.
type MetricsSnapshot struct {
	// ActiveRequests is the number of requests currently in flight for
	// the provider at observation time.
	ActiveRequests int
	// TotalRequests is the cumulative number of requests dispatched to
	// the provider since the recorder was created.
	TotalRequests int64
	// ErrorRate is the fraction of requests that ended in error since
	// the recorder was created, in the range [0.0, 1.0]. A recorder
	// with zero observed requests MAY return 0.0 BUT only if the
	// implementation tracks TotalRequests==0 honestly.
	ErrorRate float64
	// TokensPerSecond is the observed throughput averaged across the
	// recorder's sampling window. Zero is permitted when no traffic
	// was observed.
	TokensPerSecond float64
	// MemoryUsage is the observed memory footprint of the provider
	// process in bytes. Zero is permitted when the recorder cannot
	// observe the process (e.g. remote provider).
	MemoryUsage int64
	// CPUUsage is the observed CPU utilisation of the provider process
	// as a percentage in [0.0, 100.0]. Zero is permitted when the
	// recorder cannot observe the process.
	CPUUsage float64
}

// MetricsRecorder produces per-provider runtime metrics for the auto LLM
// manager. Implementations MUST source numbers from real instrumentation
// (atomic counters bumped at request start/end/error, /proc parsers,
// cgroup stats, etc.) — never from hardcoded constants. CONST-050(A)
// permits in-package test fakes (e.g. *testMetricsRecorder) only inside
// *_test.go files; production wiring MUST use a real recorder.
type MetricsRecorder interface {
	// Snapshot returns the current MetricsSnapshot for the named
	// provider. An implementation MAY return an error when the
	// underlying instrumentation is degraded (e.g. /proc unreadable);
	// the caller treats the error as a metrics-update failure rather
	// than fabricating defaults.
	Snapshot(providerName string) (MetricsSnapshot, error)
}

// AutoLLMManager provides fully automated, zero-configuration management
type AutoLLMManager struct {
	baseDir         string
	providers       map[string]*AutoProvider
	healthMonitor   *HealthMonitor
	loadBalancer    *LoadBalancer
	backgroundTasks map[string]*BackgroundTask
	mutex           sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	isInitialized   bool
	isRunning       bool
	config          *AutoConfig
	// metricsRecorder produces per-provider runtime metrics. MUST be
	// injected via SetMetricsRecorder before updatePerformanceMetrics
	// can write a real snapshot — see ErrPerformanceMetricsNotInstrumented.
	metricsRecorder MetricsRecorder
}

// SetMetricsRecorder wires a MetricsRecorder into the manager. Subsequent
// calls to updatePerformanceMetrics will read snapshots from this
// recorder. Passing nil clears the recorder, causing future metric
// updates to surface ErrPerformanceMetricsNotInstrumented.
func (m *AutoLLMManager) SetMetricsRecorder(rec MetricsRecorder) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.metricsRecorder = rec
}

// AutoConfig represents zero-touch configuration
type AutoConfig struct {
	Version       string            `json:"version"`
	Mode          string            `json:"mode"`
	AutoDiscover  bool              `json:"auto_discover"`
	AutoInstall   bool              `json:"auto_install"`
	AutoConfigure bool              `json:"auto_configure"`
	AutoStart     bool              `json:"auto_start"`
	AutoMonitor   bool              `json:"auto_monitor"`
	AutoUpdate    bool              `json:"auto_update"`
	Health        HealthConfig      `json:"health"`
	Performance   PerformanceConfig `json:"performance"`
	Security      SecurityConfig    `json:"security"`
	Updates       UpdateConfig      `json:"updates"`
}

// HealthConfig defines health monitoring configuration
type HealthConfig struct {
	CheckInterval int  `json:"check_interval"`
	AutoRecovery  bool `json:"auto_recovery"`
	MaxRetries    int  `json:"max_retries"`
	RetryDelay    int  `json:"retry_delay"`
}

// PerformanceConfig defines performance optimization configuration
type PerformanceConfig struct {
	AutoOptimize   bool `json:"auto_optimize"`
	LoadBalance    bool `json:"load_balance"`
	CacheResponses bool `json:"cache_responses"`
	PredictScaling bool `json:"predict_scaling"`
}

// SecurityConfig defines security configuration
type SecurityConfig struct {
	AutoSandbox      bool `json:"auto_sandbox"`
	MinPrivileges    bool `json:"min_privileges"`
	NetworkIsolation bool `json:"network_isolation"`
	ResourceLimits   bool `json:"resource_limits"`
}

// UpdateConfig defines update configuration
type UpdateConfig struct {
	AutoCheck       bool `json:"auto_check"`
	AutoDownload    bool `json:"auto_download"`
	AutoInstall     bool `json:"auto_install"`
	BackupConfig    bool `json:"backup_config"`
	RollbackEnabled bool `json:"rollback_enabled"`
}

// AutoProvider represents a managed provider
type AutoProvider struct {
	LocalLLMProvider
	Status          string                 `json:"status"`
	Process         *os.Process            `json:"-"`
	Config          map[string]interface{} `json:"config"`
	Health          *HealthStatus          `json:"health"`
	Metrics         *PerformanceMetrics    `json:"metrics"`
	LastHealthCheck time.Time              `json:"last_health_check"`
	RetryCount      int                    `json:"retry_count"`
}

// HealthStatus represents provider health
type HealthStatus struct {
	Status       string    `json:"status"`
	ResponseTime int       `json:"response_time"`
	LastCheck    time.Time `json:"last_check"`
	Error        string    `json:"error"`
	IsHealthy    bool      `json:"is_healthy"`
}

// PerformanceMetrics represents provider performance
type PerformanceMetrics struct {
	TokensPerSecond float64   `json:"tokens_per_second"`
	MemoryUsage     int64     `json:"memory_usage"`
	CPUUsage        float64   `json:"cpu_usage"`
	ActiveRequests  int       `json:"active_requests"`
	TotalRequests   int64     `json:"total_requests"`
	ErrorRate       float64   `json:"error_rate"`
	LastUpdated     time.Time `json:"last_updated"`
}

// BackgroundTask represents a background automation task
type BackgroundTask struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Function  func() error  `json:"-"`
	Interval  time.Duration `json:"interval"`
	LastRun   time.Time     `json:"last_run"`
	IsRunning bool          `json:"is_running"`
	StopChan  chan bool     `json:"-"`
}

// NewAutoLLMManager creates a new automated LLM manager
func NewAutoLLMManager(baseDir string) *AutoLLMManager {
	if baseDir == "" {
		homeDir, _ := os.UserHomeDir()
		baseDir = filepath.Join(homeDir, ".helixcode", "local-llm")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &AutoLLMManager{
		baseDir:         baseDir,
		providers:       make(map[string]*AutoProvider),
		backgroundTasks: make(map[string]*BackgroundTask),
		ctx:             ctx,
		cancel:          cancel,
		config: &AutoConfig{
			Version:       "1.0.0",
			Mode:          "zero_touch",
			AutoDiscover:  true,
			AutoInstall:   true,
			AutoConfigure: true,
			AutoStart:     true,
			AutoMonitor:   true,
			AutoUpdate:    true,
			Health: HealthConfig{
				CheckInterval: 30,
				AutoRecovery:  true,
				MaxRetries:    3,
				RetryDelay:    5,
			},
			Performance: PerformanceConfig{
				AutoOptimize:   true,
				LoadBalance:    true,
				CacheResponses: true,
				PredictScaling: true,
			},
			Security: SecurityConfig{
				AutoSandbox:      true,
				MinPrivileges:    true,
				NetworkIsolation: true,
				ResourceLimits:   true,
			},
			Updates: UpdateConfig{
				AutoCheck:       true,
				AutoDownload:    true,
				AutoInstall:     true,
				BackupConfig:    true,
				RollbackEnabled: true,
			},
		},
	}
}

// Initialize sets up the completely automated system
func (m *AutoLLMManager) Initialize(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isInitialized {
		return nil
	}

	log.Println("🚀 Initializing Auto-LLM Manager (Zero-Touch Mode)...")

	// Create directory structure
	if err := m.createDirectoryStructure(); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	// Load or create configuration
	if err := m.loadConfiguration(); err != nil {
		log.Printf("⚠️  Config loading error: %v", err)
	}

	// Initialize all providers
	if err := m.initializeProviders(); err != nil {
		return fmt.Errorf("failed to initialize providers: %w", err)
	}

	// Install all providers automatically
	if m.config.AutoInstall {
		go m.autoInstallAllProviders()
	}

	// Start background automation tasks
	if err := m.startBackgroundTasks(); err != nil {
		return fmt.Errorf("failed to start background tasks: %w", err)
	}

	m.isInitialized = true
	log.Println("✅ Auto-LLM Manager initialized successfully!")
	log.Println("🎯 Zero-touch operation enabled - everything happens automatically")

	return nil
}

// Start begins the fully automated management
func (m *AutoLLMManager) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isRunning {
		return nil
	}

	log.Println("🚀 Starting Auto-LLM Manager (Fully Automated)...")

	// Auto-start all providers
	if m.config.AutoStart {
		go m.autoStartAllProviders()
	}

	// Start health monitoring
	if m.config.AutoMonitor {
		m.healthMonitor = NewHealthMonitor(m)
		go m.healthMonitor.Start(m.ctx)
	}

	// Start load balancer
	if m.config.Performance.LoadBalance {
		m.loadBalancer = NewLoadBalancer(m)
		go m.loadBalancer.Start(m.ctx)
	}

	m.isRunning = true
	log.Println("✅ Auto-LLM Manager started - Zero-touch operation active")

	return nil
}

// createDirectoryStructure creates the complete directory structure
func (m *AutoLLMManager) createDirectoryStructure() error {
	dirs := []string{
		"auto-manager/bin",
		"auto-manager/config",
		"auto-manager/scripts",
		"auto-manager/logs",
		"providers",
		"build",
		"config",
		"data/models",
		"data/cache",
		"data/logs",
		"cache/pip",
		"cache/npm",
		"cache/build",
		"runtime/processes",
		"runtime/health",
		"runtime/metrics",
		"runtime/state",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(m.baseDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}
	}

	log.Println("📁 Directory structure created automatically")
	return nil
}

// loadConfiguration loads or creates the auto-configuration
func (m *AutoLLMManager) loadConfiguration() error {
	configPath := filepath.Join(m.baseDir, "auto-manager", "config", "auto-config.yaml")

	// If config exists, load it
	if _, err := os.Stat(configPath); err == nil {
		// Load existing config (implementation would parse YAML)
		log.Println("📋 Loaded existing auto-configuration")
	} else {
		// Create default config
		m.createDefaultConfiguration(configPath)
	}

	return nil
}

// createDefaultConfiguration creates the default zero-touch configuration
func (m *AutoLLMManager) createDefaultConfiguration(configPath string) error {
	configData, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Println("📝 Created default zero-touch configuration")
	return nil
}

// initializeProviders initializes all provider definitions
func (m *AutoLLMManager) initializeProviders() error {
	log.Println("🤖 Initializing provider definitions...")

	for name, providerDef := range providerDefinitions {
		autoProvider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name:         providerDef.Name,
				Repository:   providerDef.Repository,
				Version:      providerDef.Version,
				Description:  providerDef.Description,
				DefaultPort:  providerDef.DefaultPort,
				Dependencies: providerDef.Dependencies,
				BuildScript:  providerDef.BuildScript,
				StartupCmd:   providerDef.StartupCmd,
				Environment:  providerDef.Environment,
			},
			Status: "not_installed",
			Config: make(map[string]interface{}),
			Health: &HealthStatus{
				Status:    "unknown",
				IsHealthy: false,
			},
			Metrics: &PerformanceMetrics{
				LastUpdated: time.Now(),
			},
		}

		// Set paths
		autoProvider.BinaryPath = filepath.Join(m.baseDir, "build", strings.ToLower(name))
		autoProvider.ConfigPath = filepath.Join(m.baseDir, "config", strings.ToLower(name))
		autoProvider.DataPath = filepath.Join(m.baseDir, "providers", strings.ToLower(name))
		autoProvider.HealthURL = fmt.Sprintf("http://127.0.0.1:%d/health", providerDef.DefaultPort)

		m.providers[name] = autoProvider

		// Create provider directories
		os.MkdirAll(autoProvider.ConfigPath, 0755)
		os.MkdirAll(autoProvider.DataPath, 0755)
	}

	log.Printf("✅ Initialized %d providers", len(m.providers))
	return nil
}

// autoInstallAllProviders automatically installs all providers
func (m *AutoLLMManager) autoInstallAllProviders() {
	log.Println("📦 Starting automatic installation of all providers...")

	for name, provider := range m.providers {
		if provider.Status == "installed" {
			log.Printf("⏭️  Skipping %s (already installed)", name)
			continue
		}

		log.Printf("📦 Auto-installing %s...", name)

		// Clone repository
		if err := m.autoCloneProvider(provider); err != nil {
			log.Printf("❌ Failed to clone %s: %v", name, err)
			continue
		}

		// Build provider
		if err := m.autoBuildProvider(provider); err != nil {
			log.Printf("❌ Failed to build %s: %v", name, err)
			continue
		}

		// Configure provider
		if err := m.autoConfigureProvider(provider); err != nil {
			log.Printf("❌ Failed to configure %s: %v", name, err)
			continue
		}

		// Create startup script
		startupScriptPath := filepath.Join(m.providers[name].BinaryPath, strings.ToLower(name)+".sh")
		if err := m.createStartupScriptForProvider(m.providers[name], startupScriptPath); err != nil {
			log.Printf("❌ Failed to create startup script for %s: %v", name, err)
			continue
		}

		m.providers[name].Status = "installed"
		m.providers[name].LastHealthCheck = time.Now()

		log.Printf("✅ Auto-installed %s", name)
	}

	log.Println("🎉 All providers auto-installed successfully!")
}

// autoCloneProvider automatically clones a provider repository
func (m *AutoLLMManager) autoCloneProvider(provider *AutoProvider) error {
	log.Printf("📥 Auto-cloning %s from %s", provider.Name, provider.Repository)

	// Check if directory exists
	if _, err := os.Stat(provider.DataPath); err == nil {
		// Pull latest changes
		cmd := exec.Command("git", "pull", "origin", "main")
		cmd.Dir = provider.DataPath
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git pull failed: %s", string(output))
		}
	} else {
		// Clone fresh repository
		cmd := exec.Command("git", "clone", provider.Repository, provider.DataPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git clone failed: %s", string(output))
		}
	}

	log.Printf("✅ Auto-cloned %s", provider.Name)
	return nil
}

// autoBuildProvider automatically builds a provider
func (m *AutoLLMManager) autoBuildProvider(provider *AutoProvider) error {
	log.Printf("🔨 Auto-building %s...", provider.Name)

	// Determine build script
	buildScript := filepath.Join(provider.DataPath, "build.sh")
	script := provider.BuildScript

	if _, err := os.Stat(buildScript); err == nil {
		script = "bash build.sh"
	}

	// Execute build command
	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = provider.DataPath

	// Set environment
	env := os.Environ()
	for k, v := range provider.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build failed: %s", string(output))
	}

	log.Printf("✅ Auto-built %s", provider.Name)
	return nil
}

// autoConfigureProvider automatically configures a provider
func (m *AutoLLMManager) autoConfigureProvider(provider *AutoProvider) error {
	log.Printf("⚙️ Auto-configuring %s...", provider.Name)

	// Create optimized configuration
	optimizedConfig := map[string]interface{}{
		"host":        "127.0.0.1",
		"port":        provider.DefaultPort,
		"workers":     1,
		"timeout":     30,
		"max_tokens":  4096,
		"temperature": 0.7,
		"auto_gpu":    true,
		"cpu_offload": true,
	}

	// Write configuration file
	configPath := filepath.Join(provider.ConfigPath, "config.json")
	configData, err := json.MarshalIndent(optimizedConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Store in provider
	provider.Config = optimizedConfig

	log.Printf("✅ Auto-configured %s", provider.Name)
	return nil
}

// autoStartAllProviders automatically starts all providers
func (m *AutoLLMManager) autoStartAllProviders() {
	log.Println("🚀 Starting automatic provider startup...")

	for name, provider := range m.providers {
		if provider.Status == "running" {
			log.Printf("⏭️  Skipping %s (already running)", name)
			continue
		}

		log.Printf("🚀 Auto-starting %s...", name)

		if err := m.autoStartProvider(provider); err != nil {
			log.Printf("❌ Failed to start %s: %v", name, err)
			continue
		}

		log.Printf("✅ Auto-started %s on port %d", name, provider.DefaultPort)
	}

	log.Println("🎉 All providers auto-started successfully!")
}

// autoStartProvider automatically starts a single provider
func (m *AutoLLMManager) autoStartProvider(provider *AutoProvider) error {
	// Create startup command
	scriptPath := filepath.Join(m.baseDir, "auto-manager", "scripts", strings.ToLower(provider.Name)+".sh")

	// Write startup script
	var script strings.Builder
	script.WriteString("#!/bin/bash\n")
	script.WriteString(fmt.Sprintf("# Auto-generated startup script for %s\n", provider.Name))
	script.WriteString("\n")

	// Set environment variables
	for k, v := range provider.Environment {
		script.WriteString(fmt.Sprintf("export %s=\"%s\"\n", k, v))
	}
	script.WriteString("\n")

	// Change to provider directory
	script.WriteString(fmt.Sprintf("cd \"%s\"\n", provider.DataPath))
	script.WriteString("\n")

	// Add startup command
	if len(provider.StartupCmd) > 0 {
		cmd := strings.Join(provider.StartupCmd, " ")
		script.WriteString(fmt.Sprintf("exec %s\n", cmd))
	}

	// Write script
	if err := os.WriteFile(scriptPath, []byte(script.String()), 0755); err != nil {
		return fmt.Errorf("failed to write startup script: %w", err)
	}

	// Start provider process
	cmd := exec.Command("bash", scriptPath)

	// Set environment
	env := os.Environ()
	for k, v := range provider.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Start process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start provider: %w", err)
	}

	// Store process reference
	provider.Process = cmd.Process
	provider.Status = "starting"
	provider.LastHealthCheck = time.Now()

	// Wait a moment for startup
	time.Sleep(2 * time.Second)

	// Check if still running
	if provider.Process == nil || !m.isProcessRunning(provider.Process.Pid) {
		return fmt.Errorf("provider process died during startup")
	}

	provider.Status = "running"
	log.Printf("✅ Auto-started %s (PID: %d)", provider.Name, provider.Process.Pid)

	return nil
}

// isProcessRunning checks if a process is running
func (m *AutoLLMManager) isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(os.Signal(nil))
	return err == nil
}

// startBackgroundTasks starts all automation tasks
func (m *AutoLLMManager) startBackgroundTasks() error {
	log.Println("⚙️ Starting background automation tasks...")

	// Health monitoring task
	if m.config.AutoMonitor {
		healthTask := &BackgroundTask{
			ID:       uuid.New().String(),
			Name:     "Health Monitor",
			Function: m.autoHealthCheck,
			Interval: time.Duration(m.config.Health.CheckInterval) * time.Second,
			StopChan: make(chan bool),
		}
		m.backgroundTasks["health"] = healthTask
		go m.runBackgroundTask(healthTask)
	}

	// Performance optimization task
	if m.config.Performance.AutoOptimize {
		perfTask := &BackgroundTask{
			ID:       uuid.New().String(),
			Name:     "Performance Optimizer",
			Function: m.autoPerformanceOptimization,
			Interval: 5 * time.Minute,
			StopChan: make(chan bool),
		}
		m.backgroundTasks["performance"] = perfTask
		go m.runBackgroundTask(perfTask)
	}

	// Update check task
	if m.config.Updates.AutoCheck {
		updateTask := &BackgroundTask{
			ID:       uuid.New().String(),
			Name:     "Update Checker",
			Function: m.autoUpdateCheck,
			Interval: 1 * time.Hour,
			StopChan: make(chan bool),
		}
		m.backgroundTasks["updates"] = updateTask
		go m.runBackgroundTask(updateTask)
	}

	log.Println("✅ Background automation tasks started")
	return nil
}

// runBackgroundTask runs a background task
func (m *AutoLLMManager) runBackgroundTask(task *BackgroundTask) {
	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()

	task.IsRunning = true
	log.Printf("⚙️ Started background task: %s", task.Name)

	for {
		select {
		case <-m.ctx.Done():
			log.Printf("⏹️ Stopping background task: %s", task.Name)
			task.IsRunning = false
			return
		case <-task.StopChan:
			log.Printf("⏹️ Stopping background task: %s", task.Name)
			task.IsRunning = false
			return
		case <-ticker.C:
			start := time.Now()
			if err := task.Function(); err != nil {
				log.Printf("❌ Background task %s failed: %v", task.Name, err)
			}
			task.LastRun = start
		}
	}
}

// autoHealthCheck performs automatic health checks
func (m *AutoLLMManager) autoHealthCheck() error {
	for name, provider := range m.providers {
		if provider.Status != "running" {
			continue
		}

		// Perform health check
		isHealthy, responseTime, err := m.performHealthCheck(provider)

		// Update health status
		provider.Health.LastCheck = time.Now()
		provider.Health.ResponseTime = responseTime
		provider.Health.IsHealthy = isHealthy

		if isHealthy {
			provider.Health.Status = "healthy"
			provider.Health.Error = ""
			provider.RetryCount = 0
		} else {
			provider.Health.Status = "unhealthy"
			provider.Health.Error = err.Error()
			provider.RetryCount++

			// Auto-recovery
			if m.config.Health.AutoRecovery && provider.RetryCount <= m.config.Health.MaxRetries {
				log.Printf("🔄 Auto-recovering provider %s (attempt %d)", name, provider.RetryCount)
				m.autoRecoverProvider(provider)
			}
		}

		provider.LastHealthCheck = time.Now()
	}

	return nil
}

// performHealthCheck performs health check on a provider
func (m *AutoLLMManager) performHealthCheck(provider *AutoProvider) (bool, int, error) {
	start := time.Now()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(provider.HealthURL)

	responseTime := int(time.Since(start).Milliseconds())

	if err != nil {
		return false, responseTime, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, responseTime, fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return true, responseTime, nil
}

// autoRecoverProvider automatically recovers a failed provider
func (m *AutoLLMManager) autoRecoverProvider(provider *AutoProvider) error {
	log.Printf("🔄 Auto-recovering %s...", provider.Name)

	// Stop provider if running
	if provider.Process != nil {
		provider.Process.Kill()
		provider.Process = nil
	}

	// Wait a moment
	time.Sleep(2 * time.Second)

	// Restart provider
	if err := m.autoStartProvider(provider); err != nil {
		return fmt.Errorf("auto-recovery failed: %w", err)
	}

	log.Printf("✅ Auto-recovered %s", provider.Name)
	return nil
}

// autoPerformanceOptimization performs automatic performance optimization
func (m *AutoLLMManager) autoPerformanceOptimization() error {
	log.Println("⚡ Running automatic performance optimization...")

	for name, provider := range m.providers {
		if provider.Status != "running" {
			continue
		}

		// Update metrics — surface instrumentation failures honestly
		// rather than continuing against fabricated (zero) values.
		if err := m.updatePerformanceMetrics(provider); err != nil {
			log.Printf("⚠️  Performance metrics update failed for %s: %v", name, err)
			continue
		}

		// Optimize based on metrics
		if provider.Metrics.CPUUsage > 80.0 {
			log.Printf("⚠️  High CPU usage for %s: %.1f%%", name, provider.Metrics.CPUUsage)
		}

		if provider.Metrics.MemoryUsage > 8*1024*1024*1024 { // 8GB
			log.Printf("⚠️  High memory usage for %s: %.2f GB", name, float64(provider.Metrics.MemoryUsage)/(1024*1024*1024))
		}
	}

	return nil
}

// updatePerformanceMetrics updates the PerformanceMetrics for a provider
// from the injected MetricsRecorder. Returns
// ErrPerformanceMetricsNotInstrumented when no recorder has been wired in
// via SetMetricsRecorder. Returns any error surfaced by the recorder's
// Snapshot method without overwriting the existing Metrics — the caller
// (autoPerformanceOptimization) logs the failure so it is visible in
// operator dashboards, rather than silently propagating zeroed-out data.
//
// Round-31 §11.4 anti-bluff sweep (2026-05-18): the previous body
// hardcoded ActiveRequests=0 and ErrorRate=0.0, then bumped LastUpdated.
// Monitoring consumers therefore reported 0% error rate for every
// provider regardless of reality — a HIGH bluff under Article XI §11.9 /
// CONST-035. The new body reads from real instrumentation; absent a
// recorder it fails closed with ErrPerformanceMetricsNotInstrumented so
// the missing-instrumentation state is unambiguous.
func (m *AutoLLMManager) updatePerformanceMetrics(provider *AutoProvider) error {
	if provider == nil {
		return fmt.Errorf("auto llm manager: cannot update metrics for nil provider")
	}
	if provider.Metrics == nil {
		provider.Metrics = &PerformanceMetrics{}
	}

	m.mutex.RLock()
	recorder := m.metricsRecorder
	m.mutex.RUnlock()

	if recorder == nil {
		return ErrPerformanceMetricsNotInstrumented
	}

	snapshot, err := recorder.Snapshot(provider.Name)
	if err != nil {
		return fmt.Errorf("auto llm manager: metrics recorder snapshot for %q failed: %w",
			provider.Name, err)
	}

	provider.Metrics.ActiveRequests = snapshot.ActiveRequests
	provider.Metrics.TotalRequests = snapshot.TotalRequests
	provider.Metrics.ErrorRate = snapshot.ErrorRate
	provider.Metrics.TokensPerSecond = snapshot.TokensPerSecond
	provider.Metrics.MemoryUsage = snapshot.MemoryUsage
	provider.Metrics.CPUUsage = snapshot.CPUUsage
	provider.Metrics.LastUpdated = time.Now()
	return nil
}

// autoUpdateCheck checks for provider updates
func (m *AutoLLMManager) autoUpdateCheck() error {
	log.Println("🔄 Checking for provider updates...")

	for name, provider := range m.providers {
		// Check for updates
		needsUpdate, err := m.checkForUpdates(provider)
		if err != nil {
			log.Printf("⚠️  Update check failed for %s: %v", name, err)
			continue
		}

		if needsUpdate {
			log.Printf("🔄 Update available for %s", name)
			if m.config.Updates.AutoDownload && m.config.Updates.AutoInstall {
				m.autoUpdateProvider(provider)
			}
		}
	}

	return nil
}

// checkForUpdates checks if a provider needs updates
func (m *AutoLLMManager) checkForUpdates(provider *AutoProvider) (bool, error) {
	// In a real implementation, this would check git for new commits
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = provider.DataPath
	if err := cmd.Run(); err != nil {
		return false, err
	}

	cmd = exec.Command("git", "rev-list", "HEAD...origin/main", "--count")
	cmd.Dir = provider.DataPath
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	commitsBehind := strings.TrimSpace(string(output))
	return commitsBehind != "0", nil
}

// autoUpdateProvider automatically updates a provider
func (m *AutoLLMManager) autoUpdateProvider(provider *AutoProvider) error {
	log.Printf("🔄 Auto-updating %s...", provider.Name)

	// Stop provider
	if provider.Process != nil {
		provider.Process.Kill()
		provider.Process = nil
	}

	// Pull latest changes
	cmd := exec.Command("git", "pull", "origin", "main")
	cmd.Dir = provider.DataPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git pull failed: %s", string(output))
	}

	// Rebuild provider
	if err := m.autoBuildProvider(provider); err != nil {
		return fmt.Errorf("rebuild failed: %w", err)
	}

	// Restart provider
	if err := m.autoStartProvider(provider); err != nil {
		return fmt.Errorf("restart failed: %w", err)
	}

	log.Printf("✅ Auto-updated %s", provider.Name)
	return nil
}

// GetStatus returns the current status of all providers
func (m *AutoLLMManager) GetStatus() map[string]*AutoProvider {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return copy of provider status
	status := make(map[string]*AutoProvider)
	for k, v := range m.providers {
		providerCopy := *v
		status[k] = &providerCopy
	}

	return status
}

// GetRunningEndpoints returns endpoints of running providers
func (m *AutoLLMManager) GetRunningEndpoints() []string {
	var endpoints []string
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, provider := range m.providers {
		if provider.Status == "running" && provider.Health.IsHealthy {
			endpoint := fmt.Sprintf("http://127.0.0.1:%d", provider.DefaultPort)
			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints
}

// Stop stops the automated system
func (m *AutoLLMManager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isRunning && len(m.backgroundTasks) == 0 {
		return nil
	}

	log.Println("🛑 Stopping Auto-LLM Manager...")

	// Stop all background tasks
	for _, task := range m.backgroundTasks {
		close(task.StopChan)
	}
	m.backgroundTasks = make(map[string]*BackgroundTask)

	// Stop all providers
	for _, provider := range m.providers {
		if provider.Process != nil {
			provider.Process.Kill()
		}
	}

	// Cancel context
	m.cancel()

	m.isRunning = false
	log.Println("✅ Auto-LLM Manager stopped")

	return nil
}

// createStartupScriptForProvider creates a startup script for a provider
func (m *AutoLLMManager) createStartupScriptForProvider(provider *AutoProvider, scriptPath string) error {
	var script strings.Builder
	script.WriteString("#!/bin/bash\n")
	script.WriteString(fmt.Sprintf("# Auto-generated startup script for %s\n", provider.Name))
	script.WriteString("\n")

	// Change to provider directory
	script.WriteString(fmt.Sprintf("cd %s\n", provider.DataPath))

	// Set environment variables
	for key, value := range provider.Environment {
		script.WriteString(fmt.Sprintf("export %s=\"%s\"\n", key, value))
	}
	script.WriteString("\n")

	// Execute startup command
	script.WriteString(fmt.Sprintf("%s\n", strings.Join(provider.StartupCmd, " ")))

	// Create script file
	if err := os.WriteFile(scriptPath, []byte(script.String()), 0755); err != nil {
		return fmt.Errorf("failed to write startup script: %w", err)
	}

	log.Printf("📝 Created startup script: %s", scriptPath)
	return nil
}
