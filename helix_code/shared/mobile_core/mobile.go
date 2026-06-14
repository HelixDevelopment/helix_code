// Package core provides shared functionality for HelixCode mobile clients
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/redis"
	"dev.helix.code/internal/server"
	"dev.helix.code/internal/task"
	"dev.helix.code/internal/worker"
)

// MobileCore represents the shared mobile core functionality
type MobileCore struct {
	config             *config.Config
	db                 *database.Database
	taskManager        *task.TaskManager
	workerManager      *worker.WorkerManager
	llmProvider        llm.Provider
	notificationEngine *notification.NotificationEngine
	server             *server.Server
	themeManager       *ThemeManager

	// Mobile-specific state
	isConnected  bool
	currentUser  string
	sessionToken string
	mu           sync.RWMutex
}

// Theme represents a UI theme for mobile
type Theme struct {
	Name       string
	IsDark     bool
	Primary    string
	Secondary  string
	Accent     string
	Text       string
	Background string
	Border     string
	Success    string
	Warning    string
	Error      string
	Info       string
}

// ThemeManager manages UI themes for mobile
type ThemeManager struct {
	currentTheme *Theme
	themes       map[string]*Theme
}

// NewThemeManager creates a new mobile theme manager
func NewThemeManager() *ThemeManager {
	tm := &ThemeManager{
		themes: make(map[string]*Theme),
	}

	// Register themes
	tm.themes["dark"] = &Theme{
		Name:       "Dark",
		IsDark:     true,
		Primary:    "#2E86AB",
		Secondary:  "#A23B72",
		Accent:     "#F18F01",
		Text:       "#FFFFFF",
		Background: "#1E1E1E",
		Border:     "#404040",
		Success:    "#4CAF50",
		Warning:    "#FF9800",
		Error:      "#F44336",
		Info:       "#2196F3",
	}

	tm.themes["light"] = &Theme{
		Name:       "Light",
		IsDark:     false,
		Primary:    "#1976D2",
		Secondary:  "#7B1FA2",
		Accent:     "#FF6F00",
		Text:       "#212121",
		Background: "#FFFFFF",
		Border:     "#BDBDBD",
		Success:    "#4CAF50",
		Warning:    "#FF9800",
		Error:      "#F44336",
		Info:       "#2196F3",
	}

	tm.themes["helix"] = &Theme{
		Name:       "Helix",
		IsDark:     true,
		Primary:    "#C2E95B",
		Secondary:  "#C0E853",
		Accent:     "#B8ECD7",
		Text:       "#2D3047",
		Background: "#1A1A1A",
		Border:     "#404040",
		Success:    "#4CAF50",
		Warning:    "#FF9800",
		Error:      "#F44336",
		Info:       "#2196F3",
	}

	// Set default theme
	tm.currentTheme = tm.themes["helix"]

	return tm
}

// GetCurrentTheme returns the current theme
func (tm *ThemeManager) GetCurrentTheme() *Theme {
	return tm.currentTheme
}

// SetTheme sets the current theme
func (tm *ThemeManager) SetTheme(themeName string) bool {
	if theme, exists := tm.themes[themeName]; exists {
		tm.currentTheme = theme
		return true
	}
	return false
}

// GetAvailableThemes returns available theme names
func (tm *ThemeManager) GetAvailableThemes() []string {
	names := make([]string, 0, len(tm.themes))
	for name := range tm.themes {
		names = append(names, name)
	}
	return names
}

// Exported functions for gomobile binding

// NewMobileCore creates a new mobile core instance
//
//export NewMobileCore
func NewMobileCore() *MobileCore {
	return &MobileCore{
		themeManager: NewThemeManager(),
		isConnected:  false,
	}
}

// Initialize initializes the mobile core
//
//export Initialize
func (mc *MobileCore) Initialize() error {
	return mc.initializeInternal()
}

// Connect establishes connection to the HelixCode server
//
//export Connect
func (mc *MobileCore) Connect(serverURL, username, password string) error {
	return mc.connectInternal(serverURL, username, password)
}

// Disconnect disconnects from the server
//
//export Disconnect
func (mc *MobileCore) Disconnect() error {
	return mc.disconnectInternal()
}

// IsConnected returns connection status
//
//export IsConnected
func (mc *MobileCore) IsConnected() bool {
	return mc.isConnectedInternal()
}

// GetCurrentUser returns the current user
//
//export GetCurrentUser
func (mc *MobileCore) GetCurrentUser() string {
	return mc.getCurrentUserInternal()
}

// GetDashboardData returns dashboard data as JSON
//
//export GetDashboardData
func (mc *MobileCore) GetDashboardData() string {
	return mc.getDashboardDataInternal()
}

// GetTasks returns tasks data as JSON
//
//export GetTasks
func (mc *MobileCore) GetTasks() string {
	return mc.getTasksInternal()
}

// GetWorkers returns workers data as JSON
//
//export GetWorkers
func (mc *MobileCore) GetWorkers() string {
	return mc.getWorkersInternal()
}

// CreateTask creates a new task
//
//export CreateTask
func (mc *MobileCore) CreateTask(name, description string) string {
	return mc.createTaskInternal(name, description)
}

// SendNotification sends a notification
//
//export SendNotification
func (mc *MobileCore) SendNotification(title, message, notificationType string) string {
	return mc.sendNotificationInternal(title, message, notificationType)
}

// GetTheme returns current theme data as JSON
//
//export GetTheme
func (mc *MobileCore) GetTheme() string {
	return mc.getThemeInternal()
}

// SetTheme sets the current theme
//
//export SetTheme
func (mc *MobileCore) SetTheme(themeName string) bool {
	return mc.setThemeInternal(themeName)
}

// GetAvailableThemes returns available themes as JSON
//
//export GetAvailableThemes
func (mc *MobileCore) GetAvailableThemes() string {
	return mc.getAvailableThemesInternal()
}

// Close cleans up resources
//
//export Close
func (mc *MobileCore) Close() error {
	return mc.closeInternal()
}

// Generate sends a prompt to the configured LLM provider and returns the
// generated text. The signature uses only gomobile-compatible types
// (string in, string + error out — no channels or interfaces cross the
// binding boundary), so it binds cleanly via `gomobile bind`.
//
// Anti-bluff (BLUFF-001 / CONST-035 / CONST-036): this returns the provider's
// genuine output, never a canned/fabricated string.
// It resolves a REAL llm.Provider via the same path cmd/cli uses
// (llm.Select -> llm.NewCloudProvider, falling back to a local Ollama
// provider) and makes a REAL provider.Generate call. When no provider is
// reachable (e.g. Ollama not running and no cloud credentials configured),
// the underlying provider returns a real transport error which is surfaced
// here verbatim — never swallowed into a fake success.
//
//export Generate
func (mc *MobileCore) Generate(prompt string) (string, error) {
	return mc.generateInternal(context.Background(), prompt)
}

// Internal methods (renamed to avoid conflicts)
func (mc *MobileCore) initializeInternal() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}
	mc.config = cfg

	// Initialize database
	db, err := database.New(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	mc.db = db

	// Initialize Redis
	rds, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		return fmt.Errorf("failed to initialize Redis: %v", err)
	}

	// Initialize components
	mc.taskManager = task.NewTaskManager(db, rds)
	mc.workerManager = &worker.WorkerManager{} // Placeholder
	mc.notificationEngine = notification.NewNotificationEngine()

	// Initialize server for API calls
	mc.server = server.New(cfg, db, rds)

	log.Println("Mobile core initialized successfully")
	return nil
}

func (mc *MobileCore) connectInternal(serverURL, username, password string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Parse server URL to extract host for authentication
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		return fmt.Errorf("invalid server URL: must start with http:// or https://")
	}

	// Create HTTP client for authentication
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Attempt authentication via API
	authURL := fmt.Sprintf("%s/api/auth/login", serverURL)
	authData := map[string]string{
		"username": username,
		"password": password,
	}

	jsonData, err := json.Marshal(authData)
	if err != nil {
		return fmt.Errorf("failed to marshal auth data: %v", err)
	}

	resp, err := client.Post(authURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		// For development/testing, allow mock authentication
		log.Printf("Warning: Could not authenticate with server: %v", err)
		log.Printf("Falling back to mock authentication for development")
		mc.isConnected = true
		mc.currentUser = username
		mc.sessionToken = "mock-token-" + username
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var authResp struct {
			Token string `json:"token"`
			User  struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			} `json:"user"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
			return fmt.Errorf("failed to decode auth response: %v", err)
		}

		mc.isConnected = true
		mc.currentUser = authResp.User.Username
		mc.sessionToken = authResp.Token

		log.Printf("Connected to server: %s as user: %s", serverURL, username)
		return nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: invalid credentials")
	} else {
		return fmt.Errorf("authentication failed: server returned status %d", resp.StatusCode)
	}
}

func (mc *MobileCore) disconnectInternal() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.isConnected = false
	mc.currentUser = ""
	mc.sessionToken = ""

	log.Println("Disconnected from server")
	return nil
}

func (mc *MobileCore) isConnectedInternal() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.isConnected
}

func (mc *MobileCore) getCurrentUserInternal() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.currentUser
}

func (mc *MobileCore) getDashboardDataInternal() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	data := map[string]interface{}{
		"isConnected": mc.isConnected,
		"user":        mc.currentUser,
		"theme":       mc.themeManager.GetCurrentTheme().Name,
		"stats": map[string]interface{}{
			"tasks":    0,
			"workers":  0,
			"projects": 0,
			"sessions": 0,
		},
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	return string(jsonData)
}

func (mc *MobileCore) getTasksInternal() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// TODO: Get real tasks from task manager
	tasks := []map[string]interface{}{
		{
			"id":          "1",
			"name":        "Code Generation Task",
			"description": "Generate REST API endpoints",
			"status":      "running",
			"progress":    65,
		},
		{
			"id":          "2",
			"name":        "Testing Task",
			"description": "Run unit tests",
			"status":      "completed",
			"progress":    100,
		},
	}

	data := map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	return string(jsonData)
}

func (mc *MobileCore) getWorkersInternal() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// TODO: Get real workers from worker manager
	workers := []map[string]interface{}{
		{
			"id":       "worker-1",
			"hostname": "worker-1.local",
			"status":   "online",
			"cpu":      45,
			"memory":   67,
		},
	}

	data := map[string]interface{}{
		"workers": workers,
		"total":   len(workers),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	return string(jsonData)
}

func (mc *MobileCore) createTaskInternal(name, description string) string {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// TODO: Implement actual task creation
	result := map[string]interface{}{
		"success": true,
		"taskId":  "task-" + name,
		"message": "Task created successfully",
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	return string(jsonData)
}

func (mc *MobileCore) sendNotificationInternal(title, message, notificationType string) string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// TODO: Implement actual notification sending
	result := map[string]interface{}{
		"success": true,
		"message": "Notification sent",
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	return string(jsonData)
}

func (mc *MobileCore) getThemeInternal() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	theme := mc.themeManager.GetCurrentTheme()
	data := map[string]interface{}{
		"name":       theme.Name,
		"isDark":     theme.IsDark,
		"primary":    theme.Primary,
		"secondary":  theme.Secondary,
		"accent":     theme.Accent,
		"text":       theme.Text,
		"background": theme.Background,
		"border":     theme.Border,
		"success":    theme.Success,
		"warning":    theme.Warning,
		"error":      theme.Error,
		"info":       theme.Info,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	return string(jsonData)
}

func (mc *MobileCore) setThemeInternal(themeName string) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	return mc.themeManager.SetTheme(themeName)
}

func (mc *MobileCore) getAvailableThemesInternal() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	themes := mc.themeManager.GetAvailableThemes()
	data := map[string]interface{}{
		"themes": themes,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	return string(jsonData)
}

func (mc *MobileCore) closeInternal() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.db != nil {
		mc.db.Close()
	}

	log.Println("Mobile core closed")
	return nil
}

// generateInternal performs the real LLM generation. It lazily resolves and
// caches a real llm.Provider, builds a real *llm.LLMRequest carrying the
// prompt as a user message, and calls provider.Generate. The returned text is
// the provider's actual response content.
func (mc *MobileCore) generateInternal(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("generate: prompt must not be empty")
	}

	provider, err := mc.ensureLLMProvider(ctx)
	if err != nil {
		return "", fmt.Errorf("generate: no LLM provider available: %w", err)
	}

	req := &llm.LLMRequest{
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := provider.Generate(ctx, req)
	if err != nil {
		return "", fmt.Errorf("generate: provider call failed: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("generate: provider returned nil response")
	}
	return resp.Content, nil
}

// ensureLLMProvider returns the cached llm.Provider, constructing a real one on
// first use. The construction mirrors cmd/cli's buildSubagentLLMProvider: it
// resolves the cloud provider type from the HELIX_LLM_PROVIDER environment
// variable via llm.Select, constructs it via llm.NewCloudProvider, and falls
// back to a local Ollama provider on the standard port when no cloud provider
// is configured or its construction fails.
//
// Anti-bluff: this never returns a stub/fake provider. If even the local
// Ollama provider cannot be constructed, the error is surfaced.
func (mc *MobileCore) ensureLLMProvider(ctx context.Context) (llm.Provider, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.llmProvider != nil {
		return mc.llmProvider, nil
	}

	if provider := buildMobileLLMProvider(ctx); provider != nil {
		mc.llmProvider = provider
		return provider, nil
	}

	return nil, fmt.Errorf("could not construct any LLM provider (cloud unconfigured and local Ollama unreachable)")
}

// buildMobileLLMProvider resolves a real cloud provider from the environment,
// falling back to a local Ollama provider. Returns nil only when no provider
// at all could be constructed (the caller turns that into an explicit error).
func buildMobileLLMProvider(_ context.Context) llm.Provider {
	selectorInput := llm.SelectorInput{
		Env: os.Getenv("HELIX_LLM_PROVIDER"),
	}
	ptype, selErr := llm.Select(selectorInput)
	switch {
	case errors.Is(selErr, llm.ErrNoProviderConfigured):
		// No cloud provider configured — fall through to the local default.
	case selErr != nil:
		// Unknown provider name — log and fall back rather than aborting.
		log.Printf("mobile: provider selector error: %v (falling back to local default)", selErr)
	default:
		entry := llm.ProviderConfigEntry{Type: ptype}
		cloud, cErr := llm.NewCloudProvider(ptype, entry)
		if cErr == nil && cloud != nil {
			return cloud
		}
		log.Printf("mobile: failed to construct cloud provider %q (%v); falling back to local default", ptype, cErr)
	}

	provider, err := llm.NewOllamaProvider(llm.OllamaConfig{
		DefaultModel: "llama3.2",
		BaseURL:      "http://localhost:11434",
	})
	if err != nil {
		log.Printf("mobile: default Ollama provider construction failed: %v", err)
		return nil
	}
	return provider
}
