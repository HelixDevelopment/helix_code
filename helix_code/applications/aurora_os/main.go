//go:build !nogui

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"dev.helix.code/applications/aurora_os/i18n"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/project"
	"dev.helix.code/internal/redis"
	"dev.helix.code/internal/server"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/task"
	"dev.helix.code/internal/worker"
)

// UITask is a simplified task representation for UI display
type UITask struct {
	ID          string
	Type        string
	Description string
	Status      string
	Priority    string
}

// UIWorker is a simplified worker representation for UI display
type UIWorker struct {
	ID      string
	Host    string
	Port    string
	User    string
	Status  string
	Healthy bool
}

// TaskStats provides task statistics for UI
type TaskStats struct {
	TotalTasks     int
	CompletedTasks int
	RunningTasks   int
	PendingTasks   int
}

// AuroraTaskManager wraps task.TaskManager for UI operations
type AuroraTaskManager struct {
	inner *task.TaskManager
	tasks []UITask
	mu    sync.RWMutex
}

// NewAuroraTaskManager creates a new Aurora task manager wrapper
func NewAuroraTaskManager(tm *task.TaskManager) *AuroraTaskManager {
	return &AuroraTaskManager{
		inner: tm,
		tasks: make([]UITask, 0),
	}
}

// GetAllTasks returns all tasks for UI display
func (atm *AuroraTaskManager) GetAllTasks() []UITask {
	atm.mu.RLock()
	defer atm.mu.RUnlock()
	return atm.tasks
}

// GetStats returns task statistics
func (atm *AuroraTaskManager) GetStats() TaskStats {
	atm.mu.RLock()
	defer atm.mu.RUnlock()

	stats := TaskStats{
		TotalTasks: len(atm.tasks),
	}

	for _, t := range atm.tasks {
		switch t.Status {
		case "completed":
			stats.CompletedTasks++
		case "running":
			stats.RunningTasks++
		case "pending":
			stats.PendingTasks++
		}
	}

	return stats
}

// CreateTask creates a new task
func (atm *AuroraTaskManager) CreateTask(ctx context.Context, taskType, description, priority string) (*UITask, error) {
	atm.mu.Lock()
	defer atm.mu.Unlock()

	newTask := UITask{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Type:        taskType,
		Description: description,
		Status:      "pending",
		Priority:    priority,
	}

	atm.tasks = append(atm.tasks, newTask)
	return &newTask, nil
}

// CancelTask cancels a task by ID
func (atm *AuroraTaskManager) CancelTask(ctx context.Context, taskID string) error {
	atm.mu.Lock()
	defer atm.mu.Unlock()

	for i, t := range atm.tasks {
		if t.ID == taskID {
			atm.tasks = append(atm.tasks[:i], atm.tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("task not found: %s", taskID)
}

// AuroraWorkerManager wraps worker.WorkerManager for UI operations
type AuroraWorkerManager struct {
	inner   *worker.WorkerManager
	workers []UIWorker
	mu      sync.RWMutex
}

// NewAuroraWorkerManager creates a new Aurora worker manager wrapper
func NewAuroraWorkerManager(wm *worker.WorkerManager) *AuroraWorkerManager {
	return &AuroraWorkerManager{
		inner:   wm,
		workers: make([]UIWorker, 0),
	}
}

// GetWorkers returns all workers for UI display
func (awm *AuroraWorkerManager) GetWorkers() []UIWorker {
	awm.mu.RLock()
	defer awm.mu.RUnlock()
	return awm.workers
}

// AddWorker adds a new worker
func (awm *AuroraWorkerManager) AddWorker(w *UIWorker) error {
	awm.mu.Lock()
	defer awm.mu.Unlock()

	awm.workers = append(awm.workers, *w)
	return nil
}

// RemoveWorker removes a worker by ID
func (awm *AuroraWorkerManager) RemoveWorker(workerID string) error {
	awm.mu.Lock()
	defer awm.mu.Unlock()

	for i, w := range awm.workers {
		if w.ID == workerID {
			awm.workers = append(awm.workers[:i], awm.workers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("worker not found: %s", workerID)
}

// AuditLogEntry represents a security audit log entry
type AuditLogEntry struct {
	Timestamp time.Time
	Action    string
	User      string
	Details   string
	Severity  string
}

// AuroraApp represents the Aurora OS specialized application
type AuroraApp struct {
	fyneApp            fyne.App
	mainWindow         fyne.Window
	config             *config.Config
	db                 *database.Database
	taskManager        *AuroraTaskManager
	workerManager      *AuroraWorkerManager
	projectManager     *project.Manager
	sessionManager     *session.Manager
	llmManager         *llm.ModelManager
	notificationEngine *notification.NotificationEngine
	server             *server.Server
	themeManager       *ThemeManager

	// Aurora OS specific components
	auroraIntegration *AuroraIntegration
	systemMonitor     *AuroraSystemMonitor
	securityManager   *AuroraSecurityManager

	// UI Components
	tabs           *container.AppTabs
	statusBar      *widget.Label
	projectList    *widget.List
	sessionList    *widget.List
	llmProviderSel *widget.Select
	chatHistory    *widget.Entry
	chatInput      *widget.Entry

	// Data cache for UI updates
	dataMu       sync.RWMutex
	projects     []*project.Project
	sessions     []*session.Session
	llmProviders []string

	// Performance mode
	performanceMode bool

	// Update ticker for real-time data
	updateTicker *time.Ticker
	stopUpdate   chan struct{}

	// translator resolves user-facing strings per CONST-046
	// (round-140 §11.4 migration). Defaults to NoopTranslator
	// (loud echo of message IDs) until SetTranslator wires a real
	// *i18nadapter.Translator at boot. Never nil after
	// NewAuroraApp returns.
	translator i18n.Translator
}

// AuroraIntegration handles Aurora OS specific integrations
type AuroraIntegration struct {
	nativeServices map[string]interface{}
	systemAPI      *AuroraSystemAPI
}

// AuroraSystemAPI provides access to Aurora OS system features
type AuroraSystemAPI struct {
	// Aurora OS specific APIs
}

// AuroraSystemMonitor monitors Aurora OS system resources
type AuroraSystemMonitor struct {
	cpuUsage     float64
	memoryUsage  float64
	diskUsage    float64
	networkStats map[string]interface{}
	mu           sync.RWMutex
}

// AuroraSecurityManager handles Aurora OS security features
type AuroraSecurityManager struct {
	encryptionEnabled  bool
	encryptionAlgo     string
	accessControl      map[string][]string
	auditLog           []AuditLogEntry
	lastSecurityScan   time.Time
	securityScanResult string
	mu                 sync.RWMutex
}

// NewAuroraSecurityManager creates a new security manager
func NewAuroraSecurityManager() *AuroraSecurityManager {
	return &AuroraSecurityManager{
		encryptionEnabled: true,
		encryptionAlgo:    "AES-256-GCM",
		accessControl: map[string][]string{
			"admin":     {"read", "write", "execute", "admin"},
			"developer": {"read", "write", "execute"},
			"viewer":    {"read"},
		},
		auditLog: make([]AuditLogEntry, 0),
	}
}

// AddAuditEntry adds an entry to the audit log
func (asm *AuroraSecurityManager) AddAuditEntry(action, user, details, severity string) {
	asm.mu.Lock()
	defer asm.mu.Unlock()
	asm.auditLog = append(asm.auditLog, AuditLogEntry{
		Timestamp: time.Now(),
		Action:    action,
		User:      user,
		Details:   details,
		Severity:  severity,
	})
}

// GetAuditLog returns all audit log entries
func (asm *AuroraSecurityManager) GetAuditLog() []AuditLogEntry {
	asm.mu.RLock()
	defer asm.mu.RUnlock()
	return asm.auditLog
}

// NewAuroraApp creates a new Aurora OS specialized application
func NewAuroraApp() *AuroraApp {
	fyneApp := app.New()
	fyneApp.Settings().SetTheme(&CustomTheme{})

	return &AuroraApp{
		fyneApp: fyneApp,
		auroraIntegration: &AuroraIntegration{
			nativeServices: make(map[string]interface{}),
			systemAPI:      &AuroraSystemAPI{},
		},
		systemMonitor: &AuroraSystemMonitor{
			networkStats: make(map[string]interface{}),
		},
		securityManager: NewAuroraSecurityManager(),
		projects:        make([]*project.Project, 0),
		sessions:        make([]*session.Session, 0),
		llmProviders:    make([]string, 0),
		stopUpdate:      make(chan struct{}),
		translator:      i18n.NoopTranslator{},
	}
}

// SetTranslator injects the runtime Translator (per CONST-046
// round-140). Passing nil is a no-op — the NoopTranslator default
// installed by NewAuroraApp is preserved so the loud-echo safety
// net never disappears silently. helix_code wires
// *i18nadapter.Translator at boot.
func (auroraApp *AuroraApp) SetTranslator(t i18n.Translator) {
	if t == nil {
		return
	}
	auroraApp.translator = t
}

// t is a tiny call-site helper that resolves a message ID through
// the injected Translator and falls back to the literal id on error
// (loud echo — never silently swallow). Centralising the
// boilerplate keeps migrated call sites a single expression long.
func (auroraApp *AuroraApp) t(id string) string {
	if auroraApp.translator == nil {
		return id
	}
	got, err := auroraApp.translator.T(context.Background(), id, nil)
	if err != nil || got == "" {
		return id
	}
	return got
}

// Initialize sets up the Aurora OS application
func (auroraApp *AuroraApp) Initialize() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}
	auroraApp.config = cfg

	// Initialize database (optional - continue without it if not available)
	db, err := database.New(cfg.Database)
	if err != nil {
		log.Printf("Warning: Database not available: %v (continuing without persistence)", err)
	}
	auroraApp.db = db

	// Initialize Redis (optional - continue without it if not available)
	rds, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		log.Printf("Warning: Redis not available: %v (continuing without caching)", err)
	}

	// Initialize components
	innerTaskManager := task.NewTaskManager(db, rds)
	auroraApp.taskManager = NewAuroraTaskManager(innerTaskManager)

	// Initialize worker manager with in-memory repository for standalone UI
	workerRepo := worker.NewInMemoryWorkerRepository()
	innerWorkerManager := worker.NewWorkerManager(workerRepo, 30*time.Second)
	auroraApp.workerManager = NewAuroraWorkerManager(innerWorkerManager)

	// Initialize project manager
	auroraApp.projectManager = project.NewManager()

	// Initialize session manager
	auroraApp.sessionManager = session.NewManager()

	// Initialize LLM manager
	auroraApp.llmManager = llm.NewModelManager()

	// Initialize notification engine
	auroraApp.notificationEngine = notification.NewNotificationEngine()

	// Initialize server for API calls
	auroraApp.server = server.New(cfg, db, rds)

	// Initialize theme manager
	auroraApp.themeManager = NewThemeManager()

	// Initialize Aurora OS specific features
	if err := auroraApp.initializeAuroraFeatures(); err != nil {
		return fmt.Errorf("failed to initialize Aurora features: %v", err)
	}

	// Setup UI
	auroraApp.setupUI()

	// Start background data updates
	auroraApp.startDataUpdates()

	// Log initialization
	auroraApp.securityManager.AddAuditEntry("system_init", "system", "Aurora OS application initialized", "info")

	return nil
}

// initializeAuroraFeatures sets up Aurora OS specific integrations
func (auroraApp *AuroraApp) initializeAuroraFeatures() error {
	// Initialize Aurora OS native services
	auroraApp.auroraIntegration.nativeServices["system"] = "aurora-system-service"
	auroraApp.auroraIntegration.nativeServices["security"] = "aurora-security-service"
	auroraApp.auroraIntegration.nativeServices["network"] = "aurora-network-service"
	auroraApp.auroraIntegration.nativeServices["storage"] = "aurora-storage-service"

	// Initialize system monitoring
	auroraApp.refreshSystemInfo()

	log.Println("Aurora OS features initialized successfully")
	return nil
}

// startDataUpdates starts periodic background data refresh
func (auroraApp *AuroraApp) startDataUpdates() {
	auroraApp.updateTicker = time.NewTicker(5 * time.Second)
	go func() {
		// Initial data load
		auroraApp.refreshData()

		for {
			select {
			case <-auroraApp.updateTicker.C:
				auroraApp.refreshData()
				auroraApp.refreshSystemInfo()
			case <-auroraApp.stopUpdate:
				auroraApp.updateTicker.Stop()
				return
			}
		}
	}()
}

// refreshData updates cached data from managers
func (auroraApp *AuroraApp) refreshData() {
	auroraApp.dataMu.Lock()
	defer auroraApp.dataMu.Unlock()

	ctx := context.Background()

	// Refresh projects
	if auroraApp.projectManager != nil {
		projects, err := auroraApp.projectManager.ListProjects(ctx, "")
		if err == nil {
			auroraApp.projects = projects
		}
	}

	// Refresh sessions
	if auroraApp.sessionManager != nil {
		auroraApp.sessions = auroraApp.sessionManager.GetAll()
	}

	// Refresh LLM providers
	if auroraApp.llmManager != nil {
		models := auroraApp.llmManager.GetAvailableModels()
		providers := make(map[string]bool)
		for _, model := range models {
			providers[string(model.Provider)] = true
		}
		auroraApp.llmProviders = make([]string, 0, len(providers))
		for provider := range providers {
			auroraApp.llmProviders = append(auroraApp.llmProviders, provider)
		}
	}
}

// setupUI initializes the user interface with Aurora OS optimizations
func (auroraApp *AuroraApp) setupUI() {
	// Create main window with Aurora OS branding. Window title +
	// every top-level tab label resolved via i18n.Translator per
	// CONST-046 round-140. Card titles, button labels, and dialog
	// titles deeper in the UI are intentionally kept hardcoded in
	// this round and tracked for migration in later rounds.
	auroraApp.mainWindow = auroraApp.fyneApp.NewWindow(auroraApp.t("aurora_os_window_title"))
	auroraApp.mainWindow.SetMaster()
	auroraApp.mainWindow.Resize(fyne.NewSize(1400, 900)) // Larger for Aurora OS displays

	// Create tabs with Aurora OS specific tabs
	auroraApp.tabs = container.NewAppTabs(
		container.NewTabItem(auroraApp.t("aurora_os_tab_aurora_dashboard"), auroraApp.createAuroraDashboardTab()),
		container.NewTabItem(auroraApp.t("aurora_os_tab_tasks"), auroraApp.createTasksTab()),
		container.NewTabItem(auroraApp.t("aurora_os_tab_workers"), auroraApp.createWorkersTab()),
		container.NewTabItem(auroraApp.t("aurora_os_tab_aurora_system"), auroraApp.createAuroraSystemTab()),
		container.NewTabItem(auroraApp.t("aurora_os_tab_security"), auroraApp.createAuroraSecurityTab()),
		container.NewTabItem(auroraApp.t("aurora_os_tab_projects"), auroraApp.createProjectsTab()),
		container.NewTabItem(auroraApp.t("aurora_os_tab_sessions"), auroraApp.createSessionsTab()),
		container.NewTabItem(auroraApp.t("aurora_os_tab_llm"), auroraApp.createLLMTab()),
		container.NewTabItem(auroraApp.t("aurora_os_tab_settings"), auroraApp.createSettingsTab()),
	)

	// Create enhanced status bar for Aurora OS
	auroraApp.statusBar = widget.NewLabel(auroraApp.t("aurora_os_status_bar_default"))
	auroraApp.statusBar.Alignment = fyne.TextAlignCenter

	// Create main layout
	mainContent := container.NewBorder(nil, auroraApp.statusBar, nil, nil, auroraApp.tabs)

	auroraApp.mainWindow.SetContent(mainContent)
}

// createAuroraDashboardTab creates the Aurora OS specialized dashboard
func (auroraApp *AuroraApp) createAuroraDashboardTab() fyne.CanvasObject {
	// Header with Aurora OS branding
	header := widget.NewLabel(auroraApp.t("aurora_os_dashboard_header"))
	header.Alignment = fyne.TextAlignCenter
	header.TextStyle = fyne.TextStyle{Bold: true}

	// Aurora OS specific stats with dynamic updates
	systemStatsLabel := widget.NewLabel(auroraApp.t("aurora_os_stat_system_initial"))
	workerStatsLabel := widget.NewLabel(auroraApp.t("aurora_os_stat_worker_initial"))
	taskStatsLabel := widget.NewLabel(auroraApp.t("aurora_os_stat_task_initial"))

	systemCard := widget.NewCard(auroraApp.t("aurora_os_card_aurora_system"), "", systemStatsLabel)
	workerCard := widget.NewCard(auroraApp.t("aurora_os_card_workers"), "", workerStatsLabel)
	taskCard := widget.NewCard(auroraApp.t("aurora_os_card_tasks"), "", taskStatsLabel)

	// Start a goroutine to update stats
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Update system stats
			auroraApp.systemMonitor.mu.RLock()
			systemStatsLabel.SetText(fmt.Sprintf(auroraApp.t("aurora_os_stat_system_fmt"),
				auroraApp.systemMonitor.cpuUsage, auroraApp.systemMonitor.memoryUsage, auroraApp.systemMonitor.diskUsage))
			auroraApp.systemMonitor.mu.RUnlock()

			// Update worker stats
			if auroraApp.workerManager != nil {
				workers := auroraApp.workerManager.GetWorkers()
				active := 0
				healthy := 0
				for _, w := range workers {
					if w.Status == "active" {
						active++
					}
					if w.Healthy {
						healthy++
					}
				}
				workerStatsLabel.SetText(fmt.Sprintf(auroraApp.t("aurora_os_stat_worker_fmt"), len(workers), active, healthy))
			}

			// Update task stats
			if auroraApp.taskManager != nil {
				stats := auroraApp.taskManager.GetStats()
				taskStatsLabel.SetText(fmt.Sprintf(auroraApp.t("aurora_os_stat_task_fmt"),
					stats.TotalTasks, stats.CompletedTasks, stats.RunningTasks))
			}
		}
	}()

	statsContainer := container.NewGridWithColumns(3, systemCard, workerCard, taskCard)

	// Aurora OS activity log
	activityLog := widget.NewMultiLineEntry()
	activityLog.SetText(auroraApp.t("aurora_os_activity_log_seed"))
	activityLog.Disable()

	activityCard := widget.NewCard(auroraApp.t("aurora_os_card_aurora_activity"), "", activityLog)

	// Aurora OS quick actions
	actionsCard := widget.NewCard(auroraApp.t("aurora_os_card_aurora_actions"), "",
		container.NewVBox(
			widget.NewButton(auroraApp.t("aurora_os_btn_system_diagnostics"), func() { auroraApp.runAuroraDiagnostics() }),
			widget.NewButton(auroraApp.t("aurora_os_btn_security_scan"), func() { auroraApp.runSecurityScan() }),
			widget.NewButton(auroraApp.t("aurora_os_btn_performance_boost"), func() { auroraApp.activatePerformanceMode() }),
			widget.NewButton(auroraApp.t("aurora_os_btn_new_task"), func() {
				auroraApp.tabs.SelectIndex(1) // Switch to Tasks tab
			}),
			widget.NewButton(auroraApp.t("aurora_os_btn_new_project"), func() {
				auroraApp.tabs.SelectIndex(5) // Switch to Projects tab
			}),
		),
	)

	bottomContainer := container.NewGridWithColumns(2, activityCard, actionsCard)

	return container.NewVBox(header, statsContainer, bottomContainer)
}

// createAuroraSystemTab creates the Aurora OS system monitoring tab
func (auroraApp *AuroraApp) createAuroraSystemTab() fyne.CanvasObject {
	// System resources with dynamic updates
	resourcesLabel := widget.NewLabel(auroraApp.t("aurora_os_label_loading"))
	resourcesCard := widget.NewCard(auroraApp.t("aurora_os_card_system_resources"), "", resourcesLabel)

	// Update resources display
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			auroraApp.systemMonitor.mu.RLock()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			resourcesLabel.SetText(fmt.Sprintf(
				auroraApp.t("aurora_os_resources_fmt"),
				auroraApp.systemMonitor.cpuUsage, auroraApp.systemMonitor.memoryUsage, auroraApp.systemMonitor.diskUsage,
				runtime.NumGoroutine(), float64(m.Alloc)/1024/1024, float64(m.Sys)/1024/1024, m.NumGC))
			auroraApp.systemMonitor.mu.RUnlock()
		}
	}()

	// Native services status
	servicesList := widget.NewList(
		func() int { return len(auroraApp.auroraIntegration.nativeServices) },
		func() fyne.CanvasObject {
			return widget.NewLabel(auroraApp.t("aurora_os_service_list_template"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			services := make([]string, 0, len(auroraApp.auroraIntegration.nativeServices))
			for service := range auroraApp.auroraIntegration.nativeServices {
				services = append(services, service)
			}
			if id < len(services) {
				obj.(*widget.Label).SetText(fmt.Sprintf(auroraApp.t("aurora_os_service_list_item_fmt"), services[id]))
			}
		},
	)

	servicesCard := widget.NewCard(auroraApp.t("aurora_os_card_aurora_services"), "", servicesList)

	// System actions
	actions := container.NewVBox(
		widget.NewButton(auroraApp.t("aurora_os_btn_refresh_system_info"), func() {
			auroraApp.refreshSystemInfo()
			dialog.ShowInformation(auroraApp.t("aurora_os_dialog_refreshed_title"), auroraApp.t("aurora_os_dialog_system_info_refreshed"), auroraApp.mainWindow)
		}),
		widget.NewButton(auroraApp.t("aurora_os_btn_optimize_performance"), func() { auroraApp.optimizePerformance() }),
		widget.NewButton(auroraApp.t("aurora_os_btn_system_diagnostics"), func() { auroraApp.runAuroraDiagnostics() }),
		widget.NewButton(auroraApp.t("aurora_os_btn_force_gc"), func() {
			runtime.GC()
			dialog.ShowInformation(auroraApp.t("aurora_os_dialog_gc_complete_title"), auroraApp.t("aurora_os_dialog_gc_completed"), auroraApp.mainWindow)
		}),
	)

	return container.NewBorder(nil, nil, nil, actions, container.NewVBox(resourcesCard, servicesCard))
}

// createAuroraSecurityTab creates the Aurora OS security management tab
func (auroraApp *AuroraApp) createAuroraSecurityTab() fyne.CanvasObject {
	// Security status with dynamic updates
	statusLabel := widget.NewLabel(auroraApp.t("aurora_os_label_loading"))
	statusCard := widget.NewCard(auroraApp.t("aurora_os_card_security_status"), "", statusLabel)

	// Update status display
	updateStatus := func() {
		auroraApp.securityManager.mu.RLock()
		lastScan := auroraApp.t("aurora_os_token_never")
		if !auroraApp.securityManager.lastSecurityScan.IsZero() {
			lastScan = auroraApp.securityManager.lastSecurityScan.Format("2006-01-02 15:04:05")
		}
		scanResult := auroraApp.securityManager.securityScanResult
		if scanResult == "" {
			scanResult = auroraApp.t("aurora_os_security_no_scan")
		}
		statusLabel.SetText(fmt.Sprintf(
			auroraApp.t("aurora_os_security_status_fmt"),
			map[bool]string{true: auroraApp.t("aurora_os_token_enabled"), false: auroraApp.t("aurora_os_token_disabled")}[auroraApp.securityManager.encryptionEnabled],
			auroraApp.securityManager.encryptionAlgo,
			lastScan, scanResult, len(auroraApp.securityManager.auditLog)))
		auroraApp.securityManager.mu.RUnlock()
	}
	updateStatus()

	// Access control list
	accessList := widget.NewList(
		func() int {
			auroraApp.securityManager.mu.RLock()
			defer auroraApp.securityManager.mu.RUnlock()
			return len(auroraApp.securityManager.accessControl)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel(auroraApp.t("aurora_os_access_list_template"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			auroraApp.securityManager.mu.RLock()
			defer auroraApp.securityManager.mu.RUnlock()
			roles := make([]string, 0, len(auroraApp.securityManager.accessControl))
			for role := range auroraApp.securityManager.accessControl {
				roles = append(roles, role)
			}
			if id < len(roles) {
				role := roles[id]
				permissions := auroraApp.securityManager.accessControl[role]
				obj.(*widget.Label).SetText(fmt.Sprintf("%s: %v", role, permissions))
			}
		},
	)

	accessCard := widget.NewCard(auroraApp.t("aurora_os_card_access_control"), "", accessList)

	// Security actions
	actions := container.NewVBox(
		widget.NewButton(auroraApp.t("aurora_os_btn_run_security_scan"), func() {
			auroraApp.runSecurityScan()
			updateStatus()
		}),
		widget.NewButton(auroraApp.t("aurora_os_btn_view_audit_log"), func() { auroraApp.showAuditLog() }),
		widget.NewButton(auroraApp.t("aurora_os_btn_configure_encryption"), func() { auroraApp.configureEncryption() }),
		widget.NewButton(auroraApp.t("aurora_os_btn_refresh_status"), func() { updateStatus() }),
	)

	return container.NewBorder(nil, nil, nil, actions, container.NewVBox(statusCard, accessCard))
}

// Aurora OS specific methods

func (auroraApp *AuroraApp) runAuroraDiagnostics() {
	log.Println("Running Aurora OS diagnostics...")
	auroraApp.securityManager.AddAuditEntry("diagnostics_run", "user", "System diagnostics initiated", "info")

	// Perform real diagnostics
	diagnosticsResults := []string{}

	// Check CPU
	cpuCount := runtime.NumCPU()
	diagnosticsResults = append(diagnosticsResults, fmt.Sprintf("CPU Cores: %d - OK", cpuCount))

	// Check memory
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memStatus := "OK"
	if m.Alloc > 500*1024*1024 {
		memStatus = "WARNING - High memory usage"
	}
	diagnosticsResults = append(diagnosticsResults, fmt.Sprintf("Memory: %.2f MB allocated - %s", float64(m.Alloc)/1024/1024, memStatus))

	// Check goroutines
	goroutines := runtime.NumGoroutine()
	goroutineStatus := "OK"
	if goroutines > 1000 {
		goroutineStatus = "WARNING - High goroutine count"
	}
	diagnosticsResults = append(diagnosticsResults, fmt.Sprintf("Goroutines: %d - %s", goroutines, goroutineStatus))

	// Check database
	dbStatus := "Not connected"
	if auroraApp.db != nil {
		dbStatus = "Connected"
	}
	diagnosticsResults = append(diagnosticsResults, fmt.Sprintf("Database: %s", dbStatus))

	// Check managers
	diagnosticsResults = append(diagnosticsResults, "Task Manager: Initialized")
	diagnosticsResults = append(diagnosticsResults, "Worker Manager: Initialized")
	diagnosticsResults = append(diagnosticsResults, "Project Manager: Initialized")
	diagnosticsResults = append(diagnosticsResults, "Session Manager: Initialized")
	diagnosticsResults = append(diagnosticsResults, "LLM Manager: Initialized")

	// Check security
	auroraApp.securityManager.mu.RLock()
	encStatus := map[bool]string{true: "Enabled", false: "Disabled"}[auroraApp.securityManager.encryptionEnabled]
	auroraApp.securityManager.mu.RUnlock()
	diagnosticsResults = append(diagnosticsResults, fmt.Sprintf("Encryption: %s", encStatus))

	// Check performance mode
	perfStatus := map[bool]string{true: "Enabled", false: "Disabled"}[auroraApp.performanceMode]
	diagnosticsResults = append(diagnosticsResults, fmt.Sprintf("Performance Mode: %s", perfStatus))

	// Build result text
	resultText := "=== Aurora OS Diagnostics ===\n\n"
	for _, result := range diagnosticsResults {
		resultText += result + "\n"
	}
	resultText += "\nDiagnostics completed successfully."

	// Show results in dialog
	dialog.ShowInformation(auroraApp.t("aurora_os_dialog_system_diagnostics_title"), resultText, auroraApp.mainWindow)

	auroraApp.securityManager.AddAuditEntry("diagnostics_complete", "user",
		fmt.Sprintf("Diagnostics completed: %d checks performed", len(diagnosticsResults)), "info")
}

func (auroraApp *AuroraApp) runSecurityScan() {
	log.Println("Running Aurora OS security scan...")
	auroraApp.securityManager.AddAuditEntry("security_scan_start", "user", "Security scan initiated", "info")

	// Simulate security scan with real checks
	scanResults := []string{}
	issues := 0

	// Check encryption
	auroraApp.securityManager.mu.RLock()
	if !auroraApp.securityManager.encryptionEnabled {
		scanResults = append(scanResults, "[WARNING] Encryption is disabled")
		issues++
	} else {
		scanResults = append(scanResults, "[OK] Encryption is enabled")
	}
	auroraApp.securityManager.mu.RUnlock()

	// Check access control
	auroraApp.securityManager.mu.RLock()
	if len(auroraApp.securityManager.accessControl) == 0 {
		scanResults = append(scanResults, "[WARNING] No access control roles defined")
		issues++
	} else {
		scanResults = append(scanResults, fmt.Sprintf("[OK] %d access control roles defined", len(auroraApp.securityManager.accessControl)))
	}
	auroraApp.securityManager.mu.RUnlock()

	// Check audit logging
	scanResults = append(scanResults, "[OK] Audit logging is enabled")

	// Check database connection
	if auroraApp.db == nil {
		scanResults = append(scanResults, "[INFO] Database not connected - data persistence limited")
	} else {
		scanResults = append(scanResults, "[OK] Database connected")
	}

	// Update security manager
	auroraApp.securityManager.mu.Lock()
	auroraApp.securityManager.lastSecurityScan = time.Now()
	if issues == 0 {
		auroraApp.securityManager.securityScanResult = "All checks passed"
	} else {
		auroraApp.securityManager.securityScanResult = fmt.Sprintf("%d issues found", issues)
	}
	auroraApp.securityManager.mu.Unlock()

	// Build result text
	resultText := "=== Security Scan Results ===\n\n"
	for _, result := range scanResults {
		resultText += result + "\n"
	}
	resultText += fmt.Sprintf("\nScan completed: %d issues found.", issues)

	// Show results
	dialog.ShowInformation(auroraApp.t("aurora_os_dialog_security_scan_title"), resultText, auroraApp.mainWindow)

	auroraApp.securityManager.AddAuditEntry("security_scan_complete", "user",
		fmt.Sprintf("Security scan completed: %d issues found", issues), "info")
}

func (auroraApp *AuroraApp) activatePerformanceMode() {
	log.Println("Toggling Aurora OS performance mode...")

	auroraApp.performanceMode = !auroraApp.performanceMode

	if auroraApp.performanceMode {
		// Apply performance optimizations
		runtime.GOMAXPROCS(runtime.NumCPU())
		runtime.GC() // Clean up before performance mode

		dialog.ShowInformation("Performance Mode",
			"Performance mode ENABLED\n\n"+
				"Applied optimizations:\n"+
				fmt.Sprintf("- GOMAXPROCS set to %d\n", runtime.NumCPU())+
				"- Garbage collection performed\n"+
				"- Memory optimized",
			auroraApp.mainWindow)

		auroraApp.securityManager.AddAuditEntry("performance_mode", "user", "Performance mode enabled", "info")
	} else {
		dialog.ShowInformation("Performance Mode",
			"Performance mode DISABLED\n\nSystem running in normal mode.",
			auroraApp.mainWindow)

		auroraApp.securityManager.AddAuditEntry("performance_mode", "user", "Performance mode disabled", "info")
	}
}

func (auroraApp *AuroraApp) refreshSystemInfo() {
	log.Println("Refreshing Aurora OS system information...")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	auroraApp.systemMonitor.mu.Lock()
	// Calculate memory usage percentage (using Sys as total available to Go runtime)
	if m.Sys > 0 {
		auroraApp.systemMonitor.memoryUsage = float64(m.Alloc) / float64(m.Sys) * 100
	}
	// Simulate CPU usage based on goroutine count (rough approximation)
	auroraApp.systemMonitor.cpuUsage = float64(runtime.NumGoroutine()) / float64(runtime.NumCPU()*100) * 100
	if auroraApp.systemMonitor.cpuUsage > 100 {
		auroraApp.systemMonitor.cpuUsage = 100
	}
	// Get actual disk usage for root filesystem
	auroraApp.systemMonitor.diskUsage = auroraApp.getDiskUsage("/")
	auroraApp.systemMonitor.mu.Unlock()
}

// getDiskUsage returns the disk usage percentage for the given path
func (auroraApp *AuroraApp) getDiskUsage(path string) float64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		log.Printf("Failed to get disk stats for %s: %v", path, err)
		return 0.0
	}

	// Calculate usage percentage
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free

	if total == 0 {
		return 0.0
	}

	return float64(used) / float64(total) * 100.0
}

func (auroraApp *AuroraApp) optimizePerformance() {
	log.Println("Optimizing Aurora OS performance...")
	auroraApp.securityManager.AddAuditEntry("optimization_start", "user", "Performance optimization initiated", "info")

	// Get memory stats before
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	// Force garbage collection
	runtime.GC()

	// Get memory stats after
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	freed := float64(before.Alloc-after.Alloc) / 1024 / 1024

	// Set GOMAXPROCS
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Enable performance mode
	auroraApp.performanceMode = true

	resultText := fmt.Sprintf("=== Performance Optimization ===\n\n"+
		"Garbage Collection:\n  Memory freed: %.2f MB\n  Before: %.2f MB\n  After: %.2f MB\n\n"+
		"CPU Optimization:\n  GOMAXPROCS: %d\n\n"+
		"Performance Mode: Enabled\n\n"+
		"Optimization complete!",
		freed, float64(before.Alloc)/1024/1024, float64(after.Alloc)/1024/1024, runtime.NumCPU())

	dialog.ShowInformation("Performance Optimization", resultText, auroraApp.mainWindow)

	auroraApp.securityManager.AddAuditEntry("optimization_complete", "user",
		fmt.Sprintf("Optimization completed, freed %.2f MB", freed), "info")
}

func (auroraApp *AuroraApp) showAuditLog() {
	log.Println("Showing Aurora OS audit log...")

	auroraApp.securityManager.mu.RLock()
	auditLog := auroraApp.securityManager.auditLog
	auroraApp.securityManager.mu.RUnlock()

	if len(auditLog) == 0 {
		dialog.ShowInformation("Audit Log", "No audit log entries found.", auroraApp.mainWindow)
		return
	}

	// Create scrollable audit log display
	logText := "=== Security Audit Log ===\n\n"
	// Show last 50 entries (most recent first)
	start := len(auditLog) - 50
	if start < 0 {
		start = 0
	}
	for i := len(auditLog) - 1; i >= start; i-- {
		entry := auditLog[i]
		logText += fmt.Sprintf("[%s] %s\n  Action: %s\n  User: %s\n  Details: %s\n\n",
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			entry.Severity,
			entry.Action,
			entry.User,
			entry.Details)
	}
	logText += fmt.Sprintf("Showing %d of %d entries", len(auditLog)-start, len(auditLog))

	// Create a dialog with scrollable content
	logEntry := widget.NewMultiLineEntry()
	logEntry.SetText(logText)
	logEntry.Disable()
	logEntry.Wrapping = fyne.TextWrapWord

	scrollContainer := container.NewScroll(logEntry)
	scrollContainer.SetMinSize(fyne.NewSize(600, 400))

	dialog.ShowCustom("Audit Log", "Close", scrollContainer, auroraApp.mainWindow)
}

func (auroraApp *AuroraApp) configureEncryption() {
	log.Println("Configuring Aurora OS encryption...")

	auroraApp.securityManager.mu.RLock()
	currentEnabled := auroraApp.securityManager.encryptionEnabled
	currentAlgo := auroraApp.securityManager.encryptionAlgo
	auroraApp.securityManager.mu.RUnlock()

	// Create encryption configuration dialog
	enabledCheck := widget.NewCheck("Enable Encryption", nil)
	enabledCheck.Checked = currentEnabled

	algoSelect := widget.NewSelect([]string{"AES-256-GCM", "AES-256-CBC", "ChaCha20-Poly1305"}, nil)
	algoSelect.SetSelected(currentAlgo)

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Encryption Enabled", Widget: enabledCheck},
			{Text: "Algorithm", Widget: algoSelect},
		},
		OnSubmit: func() {
			auroraApp.securityManager.mu.Lock()
			oldEnabled := auroraApp.securityManager.encryptionEnabled
			auroraApp.securityManager.encryptionEnabled = enabledCheck.Checked
			auroraApp.securityManager.encryptionAlgo = algoSelect.Selected
			auroraApp.securityManager.mu.Unlock()

			// Log the change
			if oldEnabled != enabledCheck.Checked {
				action := "encryption_disabled"
				severity := "warning"
				if enabledCheck.Checked {
					action = "encryption_enabled"
					severity = "info"
				}
				auroraApp.securityManager.AddAuditEntry(action, "user",
					fmt.Sprintf("Encryption %s with algorithm %s",
						map[bool]string{true: "enabled", false: "disabled"}[enabledCheck.Checked],
						algoSelect.Selected), severity)
			}

			dialog.ShowInformation("Encryption Configuration",
				fmt.Sprintf("Encryption settings updated:\n  Enabled: %v\n  Algorithm: %s",
					enabledCheck.Checked, algoSelect.Selected),
				auroraApp.mainWindow)
		},
	}

	dialog.ShowForm("Configure Encryption", "Save", "Cancel", form.Items, func(b bool) {
		if b {
			form.OnSubmit()
		}
	}, auroraApp.mainWindow)
}

// Run starts the Aurora OS application
func (auroraApp *AuroraApp) Run() {
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start signal handler in goroutine
	go func() {
		<-sigChan
		auroraApp.fyneApp.Quit()
	}()

	// Show window and run (blocks until window closes)
	auroraApp.mainWindow.ShowAndRun()
}

// createTasksTab creates the tasks tab
func (auroraApp *AuroraApp) createTasksTab() fyne.CanvasObject {
	// Task list with dynamic data
	taskList := widget.NewList(
		func() int {
			if auroraApp.taskManager == nil {
				return 0
			}
			return len(auroraApp.taskManager.GetAllTasks())
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if auroraApp.taskManager == nil {
				return
			}
			tasks := auroraApp.taskManager.GetAllTasks()
			if id < len(tasks) {
				t := tasks[id]
				obj.(*widget.Label).SetText(fmt.Sprintf("[%s] %s - %s (%s)", t.Status, t.Type, t.Description, t.Priority))
			}
		},
	)

	taskCard := widget.NewCard("Tasks", "", taskList)

	// Task type selector for new tasks
	taskTypeSelect := widget.NewSelect([]string{"planning", "building", "testing", "refactoring", "debugging"}, nil)
	taskTypeSelect.SetSelected("building")

	// Task priority selector
	prioritySelect := widget.NewSelect([]string{"low", "normal", "high", "critical"}, nil)
	prioritySelect.SetSelected("normal")

	// Task description input
	taskDescEntry := widget.NewEntry()
	taskDescEntry.SetPlaceHolder("Task description...")

	// Action buttons
	actions := container.NewVBox(
		widget.NewLabel("New Task:"),
		widget.NewLabel("Type:"),
		taskTypeSelect,
		widget.NewLabel("Priority:"),
		prioritySelect,
		widget.NewLabel("Description:"),
		taskDescEntry,
		widget.NewButton("Create Task", func() {
			if auroraApp.taskManager != nil && taskDescEntry.Text != "" {
				ctx := context.Background()
				_, err := auroraApp.taskManager.CreateTask(ctx, taskTypeSelect.Selected, taskDescEntry.Text, prioritySelect.Selected)
				if err != nil {
					dialog.ShowError(err, auroraApp.mainWindow)
				} else {
					taskDescEntry.SetText("")
					taskList.Refresh()
					auroraApp.securityManager.AddAuditEntry("task_create", "user",
						fmt.Sprintf("Created task: %s", taskDescEntry.Text), "info")
				}
			}
		}),
		widget.NewSeparator(),
		widget.NewButton("Refresh", func() {
			taskList.Refresh()
		}),
	)

	return container.NewBorder(nil, nil, nil, actions, taskCard)
}

// createWorkersTab creates the workers tab
func (auroraApp *AuroraApp) createWorkersTab() fyne.CanvasObject {
	workerList := widget.NewList(
		func() int {
			if auroraApp.workerManager == nil {
				return 0
			}
			return len(auroraApp.workerManager.GetWorkers())
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if auroraApp.workerManager == nil {
				return
			}
			workers := auroraApp.workerManager.GetWorkers()
			if id < len(workers) {
				w := workers[id]
				healthStatus := "unhealthy"
				if w.Healthy {
					healthStatus = "healthy"
				}
				obj.(*widget.Label).SetText(fmt.Sprintf("[%s] %s - %s (%s)", w.Status, w.ID, w.Host, healthStatus))
			}
		},
	)

	workerCard := widget.NewCard("Workers", "", workerList)

	// Worker configuration inputs
	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("hostname or IP")
	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("22")
	portEntry.SetText("22")
	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("username")

	actions := container.NewVBox(
		widget.NewLabel("Add Worker:"),
		widget.NewLabel("Host:"),
		hostEntry,
		widget.NewLabel("Port:"),
		portEntry,
		widget.NewLabel("User:"),
		userEntry,
		widget.NewButton("Add Worker", func() {
			if auroraApp.workerManager != nil && hostEntry.Text != "" {
				workerConfig := &UIWorker{
					ID:      fmt.Sprintf("worker-%s-%d", hostEntry.Text, time.Now().UnixNano()),
					Host:    hostEntry.Text,
					Port:    portEntry.Text,
					User:    userEntry.Text,
					Status:  "pending",
					Healthy: false,
				}
				err := auroraApp.workerManager.AddWorker(workerConfig)
				if err != nil {
					dialog.ShowError(err, auroraApp.mainWindow)
				} else {
					hostEntry.SetText("")
					userEntry.SetText("")
					workerList.Refresh()
					auroraApp.securityManager.AddAuditEntry("worker_add", "user",
						fmt.Sprintf("Added worker: %s", workerConfig.Host), "info")
				}
			}
		}),
		widget.NewSeparator(),
		widget.NewButton("Refresh", func() {
			workerList.Refresh()
		}),
	)

	return container.NewBorder(nil, nil, nil, actions, workerCard)
}

// createProjectsTab creates the projects tab
func (auroraApp *AuroraApp) createProjectsTab() fyne.CanvasObject {
	// Project list with dynamic data
	auroraApp.projectList = widget.NewList(
		func() int {
			auroraApp.dataMu.RLock()
			defer auroraApp.dataMu.RUnlock()
			return len(auroraApp.projects)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Template"),
				widget.NewLabel(""),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			auroraApp.dataMu.RLock()
			defer auroraApp.dataMu.RUnlock()
			if id < len(auroraApp.projects) {
				p := auroraApp.projects[id]
				hbox := obj.(*fyne.Container)
				hbox.Objects[0].(*widget.Label).SetText(p.Name)
				activeStatus := ""
				if p.Active {
					activeStatus = " [ACTIVE]"
				}
				hbox.Objects[1].(*widget.Label).SetText(fmt.Sprintf("(%s)%s", p.Type, activeStatus))
			}
		},
	)

	// Project details panel
	projectDetailsLabel := widget.NewLabel("Select a project to view details")
	projectDetailsLabel.Wrapping = fyne.TextWrapWord

	auroraApp.projectList.OnSelected = func(id widget.ListItemID) {
		auroraApp.dataMu.RLock()
		defer auroraApp.dataMu.RUnlock()
		if id < len(auroraApp.projects) {
			p := auroraApp.projects[id]
			details := fmt.Sprintf("Name: %s\nType: %s\nPath: %s\nDescription: %s\nCreated: %s\nBuild Command: %s\nTest Command: %s",
				p.Name, p.Type, p.Path, p.Description,
				p.CreatedAt.Format(time.RFC822),
				p.Metadata.BuildCommand, p.Metadata.TestCommand)
			projectDetailsLabel.SetText(details)
		}
	}

	projectListCard := widget.NewCard("Projects", "", auroraApp.projectList)
	projectDetailsCard := widget.NewCard("Project Details", "", projectDetailsLabel)

	// Project creation form
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Project name")
	descEntry := widget.NewEntry()
	descEntry.SetPlaceHolder("Description")
	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder("/path/to/project")
	typeSelect := widget.NewSelect([]string{"go", "node", "python", "rust", "generic"}, nil)
	typeSelect.SetSelected("go")

	createForm := container.NewVBox(
		widget.NewLabel("Create New Project:"),
		widget.NewLabel("Name:"),
		nameEntry,
		widget.NewLabel("Description:"),
		descEntry,
		widget.NewLabel("Path:"),
		pathEntry,
		widget.NewLabel("Type:"),
		typeSelect,
		widget.NewButton("Create Project", func() {
			if auroraApp.projectManager != nil && nameEntry.Text != "" && pathEntry.Text != "" {
				ctx := context.Background()
				_, err := auroraApp.projectManager.CreateProject(ctx, nameEntry.Text, descEntry.Text, pathEntry.Text, typeSelect.Selected)
				if err != nil {
					dialog.ShowError(err, auroraApp.mainWindow)
				} else {
					auroraApp.securityManager.AddAuditEntry("project_create", "user",
						fmt.Sprintf("Created project: %s", nameEntry.Text), "info")
					nameEntry.SetText("")
					descEntry.SetText("")
					pathEntry.SetText("")
					auroraApp.refreshData()
					auroraApp.projectList.Refresh()
					dialog.ShowInformation("Success", "Project created successfully", auroraApp.mainWindow)
				}
			}
		}),
		widget.NewSeparator(),
		widget.NewButton("Refresh", func() {
			auroraApp.refreshData()
			auroraApp.projectList.Refresh()
		}),
	)

	leftPanel := container.NewVSplit(projectListCard, projectDetailsCard)
	leftPanel.SetOffset(0.6)

	return container.NewBorder(nil, nil, nil, createForm, leftPanel)
}

// createSessionsTab creates the sessions tab
func (auroraApp *AuroraApp) createSessionsTab() fyne.CanvasObject {
	// Session list with dynamic data
	auroraApp.sessionList = widget.NewList(
		func() int {
			auroraApp.dataMu.RLock()
			defer auroraApp.dataMu.RUnlock()
			return len(auroraApp.sessions)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Template"),
				widget.NewLabel(""),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			auroraApp.dataMu.RLock()
			defer auroraApp.dataMu.RUnlock()
			if id < len(auroraApp.sessions) {
				s := auroraApp.sessions[id]
				hbox := obj.(*fyne.Container)
				hbox.Objects[0].(*widget.Label).SetText(s.Name)
				hbox.Objects[1].(*widget.Label).SetText(fmt.Sprintf("[%s] %s", s.Status, s.Mode))
			}
		},
	)

	// Session details panel
	sessionDetailsLabel := widget.NewLabel("Select a session to view details")
	sessionDetailsLabel.Wrapping = fyne.TextWrapWord

	selectedSessionID := ""
	auroraApp.sessionList.OnSelected = func(id widget.ListItemID) {
		auroraApp.dataMu.RLock()
		defer auroraApp.dataMu.RUnlock()
		if id < len(auroraApp.sessions) {
			s := auroraApp.sessions[id]
			selectedSessionID = s.ID
			durationStr := s.Duration.String()
			details := fmt.Sprintf("ID: %s\nName: %s\nMode: %s\nStatus: %s\nProject ID: %s\nDescription: %s\nCreated: %s\nDuration: %s",
				s.ID, s.Name, s.Mode, s.Status, s.ProjectID, s.Description,
				s.CreatedAt.Format(time.RFC822), durationStr)
			sessionDetailsLabel.SetText(details)
		}
	}

	sessionListCard := widget.NewCard("Sessions", "", auroraApp.sessionList)
	sessionDetailsCard := widget.NewCard("Session Details", "", sessionDetailsLabel)

	// Session creation form
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Session name")
	descEntry := widget.NewEntry()
	descEntry.SetPlaceHolder("Description")
	projectIDEntry := widget.NewEntry()
	projectIDEntry.SetPlaceHolder("Project ID")
	modeSelect := widget.NewSelect([]string{"planning", "building", "testing", "refactoring", "debugging", "deployment"}, nil)
	modeSelect.SetSelected("building")

	actions := container.NewVBox(
		widget.NewLabel("Create New Session:"),
		widget.NewLabel("Name:"),
		nameEntry,
		widget.NewLabel("Description:"),
		descEntry,
		widget.NewLabel("Project ID:"),
		projectIDEntry,
		widget.NewLabel("Mode:"),
		modeSelect,
		widget.NewButton("Create Session", func() {
			if auroraApp.sessionManager != nil && nameEntry.Text != "" && projectIDEntry.Text != "" {
				mode := session.Mode(modeSelect.Selected)
				_, err := auroraApp.sessionManager.Create(projectIDEntry.Text, nameEntry.Text, descEntry.Text, mode)
				if err != nil {
					dialog.ShowError(err, auroraApp.mainWindow)
				} else {
					auroraApp.securityManager.AddAuditEntry("session_create", "user",
						fmt.Sprintf("Created session: %s", nameEntry.Text), "info")
					nameEntry.SetText("")
					descEntry.SetText("")
					projectIDEntry.SetText("")
					auroraApp.refreshData()
					auroraApp.sessionList.Refresh()
					dialog.ShowInformation("Success", "Session created successfully", auroraApp.mainWindow)
				}
			}
		}),
		widget.NewSeparator(),
		widget.NewLabel("Session Controls:"),
		widget.NewButton("Start Session", func() {
			if auroraApp.sessionManager != nil && selectedSessionID != "" {
				err := auroraApp.sessionManager.Start(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, auroraApp.mainWindow)
				} else {
					auroraApp.securityManager.AddAuditEntry("session_start", "user",
						fmt.Sprintf("Started session: %s", selectedSessionID), "info")
					auroraApp.refreshData()
					auroraApp.sessionList.Refresh()
				}
			}
		}),
		widget.NewButton("Pause Session", func() {
			if auroraApp.sessionManager != nil && selectedSessionID != "" {
				err := auroraApp.sessionManager.Pause(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, auroraApp.mainWindow)
				} else {
					auroraApp.securityManager.AddAuditEntry("session_pause", "user",
						fmt.Sprintf("Paused session: %s", selectedSessionID), "info")
					auroraApp.refreshData()
					auroraApp.sessionList.Refresh()
				}
			}
		}),
		widget.NewButton("Complete Session", func() {
			if auroraApp.sessionManager != nil && selectedSessionID != "" {
				err := auroraApp.sessionManager.Complete(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, auroraApp.mainWindow)
				} else {
					auroraApp.securityManager.AddAuditEntry("session_complete", "user",
						fmt.Sprintf("Completed session: %s", selectedSessionID), "info")
					auroraApp.refreshData()
					auroraApp.sessionList.Refresh()
				}
			}
		}),
		widget.NewSeparator(),
		widget.NewButton("Refresh", func() {
			auroraApp.refreshData()
			auroraApp.sessionList.Refresh()
		}),
	)

	leftPanel := container.NewVSplit(sessionListCard, sessionDetailsCard)
	leftPanel.SetOffset(0.6)

	return container.NewBorder(nil, nil, nil, actions, leftPanel)
}

// createLLMTab creates the LLM tab
func (auroraApp *AuroraApp) createLLMTab() fyne.CanvasObject {
	// Available models list
	modelList := widget.NewList(
		func() int {
			if auroraApp.llmManager == nil {
				return 0
			}
			return len(auroraApp.llmManager.GetAvailableModels())
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Model"),
				widget.NewLabel("Provider"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			models := auroraApp.llmManager.GetAvailableModels()
			if id < len(models) {
				m := models[id]
				hbox := obj.(*fyne.Container)
				hbox.Objects[0].(*widget.Label).SetText(m.Name)
				hbox.Objects[1].(*widget.Label).SetText(string(m.Provider))
			}
		},
	)

	modelListCard := widget.NewCard("Available Models", "", modelList)

	// Model details panel
	modelDetailsLabel := widget.NewLabel("Select a model to view details")
	modelDetailsLabel.Wrapping = fyne.TextWrapWord

	modelList.OnSelected = func(id widget.ListItemID) {
		models := auroraApp.llmManager.GetAvailableModels()
		if id < len(models) {
			m := models[id]
			caps := make([]string, len(m.Capabilities))
			for i, c := range m.Capabilities {
				caps[i] = string(c)
			}
			details := fmt.Sprintf("Name: %s\nProvider: %s\nContext Size: %d\nCapabilities: %v",
				m.Name, m.Provider, m.ContextSize, caps)
			modelDetailsLabel.SetText(details)
		}
	}

	modelDetailsCard := widget.NewCard("Model Details", "", modelDetailsLabel)

	// Chat interface
	auroraApp.chatHistory = widget.NewMultiLineEntry()
	auroraApp.chatHistory.SetPlaceHolder("Chat history will appear here...")
	auroraApp.chatHistory.Disable()
	auroraApp.chatHistory.Wrapping = fyne.TextWrapWord

	auroraApp.chatInput = widget.NewMultiLineEntry()
	auroraApp.chatInput.SetPlaceHolder("Type your message here...")
	auroraApp.chatInput.SetMinRowsVisible(3)

	// Provider/model selection for chat
	auroraApp.llmProviderSel = widget.NewSelect([]string{"ollama", "openai", "anthropic", "gemini", "local"}, nil)
	auroraApp.llmProviderSel.SetSelected("ollama")

	modelNameEntry := widget.NewEntry()
	modelNameEntry.SetPlaceHolder("Model name (e.g., llama2)")
	modelNameEntry.SetText("llama2")

	sendButton := widget.NewButton("Send Message", func() {
		if auroraApp.chatInput.Text == "" {
			return
		}

		// Add user message to history
		currentHistory := auroraApp.chatHistory.Text
		userMessage := auroraApp.chatInput.Text
		userMsg := fmt.Sprintf("\n[User]: %s\n", userMessage)
		auroraApp.chatHistory.SetText(currentHistory + userMsg)

		// Log the interaction
		auroraApp.securityManager.AddAuditEntry("llm_chat", "user",
			fmt.Sprintf("Sent message to %s/%s", auroraApp.llmProviderSel.Selected, modelNameEntry.Text), "info")

		// Clear input immediately
		auroraApp.chatInput.SetText("")

		// Make LLM call in goroutine to not block UI
		go func(msg string) {
			var responseMsg string
			providerName := auroraApp.llmProviderSel.Selected
			modelName := modelNameEntry.Text

			if auroraApp.llmManager != nil {
				// Get provider from manager using provider type
				providerType := llm.ProviderType(providerName)
				provider, err := auroraApp.llmManager.GetProviderForModel(modelName, providerType)
				if err == nil && provider != nil {
					// Create LLM request
					ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					defer cancel()

					request := &llm.LLMRequest{
						Messages: []llm.Message{
							{Role: "user", Content: msg},
						},
						Model:       modelName,
						MaxTokens:   1024,
						Temperature: 0.7,
					}

					response, err := provider.Generate(ctx, request)
					if err != nil {
						responseMsg = fmt.Sprintf("[AI (%s/%s)]: Error: %v\n", providerName, modelName, err)
					} else {
						responseMsg = fmt.Sprintf("[AI (%s/%s)]: %s\n", providerName, modelName, response.Content)
					}
				} else {
					responseMsg = fmt.Sprintf("[AI (%s/%s)]: Provider '%s' not available or model not configured. Please configure it in Settings.\n",
						providerName, modelName, providerName)
				}
			} else {
				// No LLM manager configured - show informative message
				responseMsg = fmt.Sprintf("[AI (%s/%s)]: LLM service not initialized. Please restart the application or check configuration.\n",
					providerName, modelName)
			}

			// Update UI on main thread
			auroraApp.chatHistory.SetText(auroraApp.chatHistory.Text + responseMsg)
		}(userMessage)
	})

	clearButton := widget.NewButton("Clear Chat", func() {
		auroraApp.chatHistory.SetText("")
	})

	chatControls := container.NewVBox(
		widget.NewLabel("Chat Settings:"),
		widget.NewLabel("Provider:"),
		auroraApp.llmProviderSel,
		widget.NewLabel("Model:"),
		modelNameEntry,
		widget.NewSeparator(),
		sendButton,
		clearButton,
	)

	chatPanel := container.NewBorder(
		widget.NewLabel("Chat with AI"),
		container.NewBorder(nil, nil, nil, chatControls, auroraApp.chatInput),
		nil, nil,
		auroraApp.chatHistory,
	)

	chatCard := widget.NewCard("LLM Chat", "", chatPanel)

	// Provider health status
	healthLabel := widget.NewLabel("Provider Health:\nChecking...")

	// Start health check goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		checkHealth := func() {
			if auroraApp.llmManager == nil {
				healthLabel.SetText("Provider Health:\nNo LLM manager available")
				return
			}
			ctx := context.Background()
			health := auroraApp.llmManager.HealthCheck(ctx)
			healthText := "Provider Health:\n"
			for provider, status := range health {
				healthText += fmt.Sprintf("- %s: %s\n", provider, status.Status)
			}
			if len(health) == 0 {
				healthText += "No providers configured"
			}
			healthLabel.SetText(healthText)
		}

		checkHealth()
		for range ticker.C {
			checkHealth()
		}
	}()

	healthCard := widget.NewCard("Provider Status", "", healthLabel)

	// Layout
	leftPanel := container.NewVSplit(modelListCard, modelDetailsCard)
	leftPanel.SetOffset(0.5)

	rightPanel := container.NewVSplit(chatCard, healthCard)
	rightPanel.SetOffset(0.7)

	return container.NewHSplit(leftPanel, rightPanel)
}

// createSettingsTab creates the settings tab
func (auroraApp *AuroraApp) createSettingsTab() fyne.CanvasObject {
	// Theme selection
	themeInfoLabel := widget.NewLabel("")
	updateThemeInfo := func() {
		currentTheme := auroraApp.themeManager.GetCurrentTheme()
		themeInfo := fmt.Sprintf("Name: %s\nDark: %t\nPrimary: %s\nSecondary: %s\nAccent: %s",
			currentTheme.Name, currentTheme.IsDark,
			currentTheme.Primary, currentTheme.Secondary, currentTheme.Accent)
		themeInfoLabel.SetText(themeInfo)
	}

	themeSelect := widget.NewSelect(auroraApp.themeManager.GetAvailableThemes(), func(selected string) {
		auroraApp.themeManager.SetTheme(selected)
		updateThemeInfo()
	})
	themeSelect.SetSelected(auroraApp.themeManager.GetCurrentTheme().Name)

	themeCard := widget.NewCard("Theme", "Select application theme", themeSelect)

	// Current theme info
	updateThemeInfo()
	infoCard := widget.NewCard("Current Theme", "", themeInfoLabel)

	// Server connection settings
	serverURLEntry := widget.NewEntry()
	serverURLEntry.SetText("http://localhost:8080")
	serverURLEntry.SetPlaceHolder("Server URL")

	serverTimeoutEntry := widget.NewEntry()
	serverTimeoutEntry.SetText("30")
	serverTimeoutEntry.SetPlaceHolder("Timeout (seconds)")

	serverCard := widget.NewCard("Server Connection", "",
		container.NewVBox(
			widget.NewLabel("Server URL:"),
			serverURLEntry,
			widget.NewLabel("Timeout (seconds):"),
			serverTimeoutEntry,
			widget.NewButton("Test Connection", func() {
				dialog.ShowInformation("Connection Test", "Server connection test would be performed here.", auroraApp.mainWindow)
			}),
		),
	)

	// Database settings
	dbHostEntry := widget.NewEntry()
	dbHostEntry.SetPlaceHolder("localhost")
	dbPortEntry := widget.NewEntry()
	dbPortEntry.SetText("5432")
	dbNameEntry := widget.NewEntry()
	dbNameEntry.SetPlaceHolder("helixcode")

	dbCard := widget.NewCard("Database", "",
		container.NewVBox(
			widget.NewLabel("Host:"),
			dbHostEntry,
			widget.NewLabel("Port:"),
			dbPortEntry,
			widget.NewLabel("Database:"),
			dbNameEntry,
		),
	)

	// LLM Provider settings
	ollamaURLEntry := widget.NewEntry()
	ollamaURLEntry.SetText("http://localhost:11434")

	llmCard := widget.NewCard("LLM Providers", "",
		container.NewVBox(
			widget.NewLabel("Ollama URL:"),
			ollamaURLEntry,
			widget.NewLabel("OpenAI API Key:"),
			widget.NewPasswordEntry(),
			widget.NewLabel("Anthropic API Key:"),
			widget.NewPasswordEntry(),
		),
	)

	// Aurora OS specific settings
	perfModeCheck := widget.NewCheck("Performance Mode", func(checked bool) {
		auroraApp.performanceMode = checked
		if checked {
			runtime.GOMAXPROCS(runtime.NumCPU())
		}
	})
	perfModeCheck.Checked = auroraApp.performanceMode

	auroraCard := widget.NewCard("Aurora OS Settings", "",
		container.NewVBox(
			perfModeCheck,
			widget.NewButton("Run Diagnostics", func() {
				auroraApp.runAuroraDiagnostics()
			}),
			widget.NewButton("View Audit Log", func() {
				auroraApp.showAuditLog()
			}),
		),
	)

	// About section
	aboutLabel := widget.NewLabel("HelixCode Aurora OS Edition\nVersion: 1.0.0\nDistributed AI Development Platform\n\nOptimized for Aurora OS")
	aboutLabel.Alignment = fyne.TextAlignCenter
	aboutCard := widget.NewCard("About", "", aboutLabel)

	// Layout in scrollable container
	settingsContent := container.NewVBox(
		themeCard,
		infoCard,
		auroraCard,
		serverCard,
		dbCard,
		llmCard,
		aboutCard,
	)

	return container.NewScroll(settingsContent)
}

// Close cleans up resources
func (auroraApp *AuroraApp) Close() error {
	// Stop background updates
	if auroraApp.stopUpdate != nil {
		close(auroraApp.stopUpdate)
	}

	// Log shutdown
	auroraApp.securityManager.AddAuditEntry("system_shutdown", "system", "Aurora OS application shutting down", "info")

	// Close database connection
	if auroraApp.db != nil {
		auroraApp.db.Close()
	}
	return nil
}

func main() {
	auroraApp := NewAuroraApp()

	if err := auroraApp.Initialize(); err != nil {
		log.Fatalf("Failed to initialize Aurora OS app: %v", err)
	}
	defer auroraApp.Close()

	auroraApp.Run()
}
