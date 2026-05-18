//go:build !nogui

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
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
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/hardware"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/monitoring"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/project"
	"dev.helix.code/internal/redis"
	"dev.helix.code/internal/server"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/task"
	"dev.helix.code/internal/worker"
)

// APIClient handles communication with the HelixCode backend API
type APIClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
	mu         sync.RWMutex
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetToken sets the authentication token
func (c *APIClient) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

// doRequest performs an HTTP request with authentication
func (c *APIClient) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return c.httpClient.Do(req)
}

// APITask represents a task from the API
type APITask struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// APIWorker represents a worker from the API
type APIWorker struct {
	ID           string    `json:"id"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	User         string    `json:"user"`
	Status       string    `json:"status"`
	Healthy      bool      `json:"healthy"`
	Capabilities []string  `json:"capabilities"`
	LastSeen     time.Time `json:"last_seen"`
}

// APIProject represents a project from the API
type APIProject struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Path        string    `json:"path"`
	Type        string    `json:"type"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

// APISession represents a session from the API
type APISession struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ProjectID   string    `json:"project_id"`
	Mode        string    `json:"mode"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// HarmonyApp represents the main Harmony OS application
type HarmonyApp struct {
	fyneApp            fyne.App
	mainWindow         fyne.Window
	config             *config.Config
	db                 *database.Database
	taskManager        *task.TaskManager
	workerManager      *worker.WorkerManager
	projectManager     *project.Manager
	sessionManager     *session.Manager
	llmManager         *llm.ModelManager
	notificationEngine *notification.NotificationEngine
	server             *server.Server
	themeManager       *ThemeManager
	apiClient          *APIClient
	monitor            *monitoring.Monitor
	hardwareDetector   *hardware.HardwareDetector

	// Harmony OS specific components
	harmonyIntegration *HarmonyIntegration
	systemMonitor      *HarmonySystemMonitor
	resourceManager    *HarmonyResourceManager
	serviceCoordinator *HarmonyServiceCoordinator

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
	tasks        []APITask
	workers      []APIWorker
	projects     []APIProject
	sessions     []APISession
	llmProviders []string

	// Update control
	updateTicker *time.Ticker
	stopUpdate   chan struct{}
}

// HarmonyIntegration handles Harmony OS-specific native features
type HarmonyIntegration struct {
	nativeServices    map[string]interface{}
	systemAPI         *HarmonySystemAPI
	distributedEngine *HarmonyDistributedEngine
	harmonyContext    context.Context
}

// HarmonySystemAPI provides access to Harmony OS system services
type HarmonySystemAPI struct {
	deviceInfo    map[string]string
	capabilities  []string
	systemVersion string
	kernelVersion string
}

// HarmonyDistributedEngine manages distributed task scheduling across Harmony OS devices
type HarmonyDistributedEngine struct {
	connectedDevices []HarmonyDevice
	taskScheduler    *HarmonyTaskScheduler
	dataSync         *HarmonyDataSync
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// HarmonyDevice represents a connected Harmony OS device
type HarmonyDevice struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	Status       string           `json:"status"`
	Capabilities []string         `json:"capabilities"`
	Resources    HarmonyResources `json:"resources"`
	LastSeen     time.Time        `json:"last_seen"`
}

// HarmonyResources represents device resources
type HarmonyResources struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	GPUUsage    float64 `json:"gpu_usage"`
	Available   bool    `json:"available"`
}

// HarmonyTaskScheduler schedules tasks across Harmony OS ecosystem
type HarmonyTaskScheduler struct {
	schedulingPolicy string
	taskQueue        []*ScheduledTask
	priorityLevels   map[string]int
	mu               sync.RWMutex
}

// ScheduledTask represents a task scheduled for distributed execution
type ScheduledTask struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	DeviceID    string    `json:"device_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// HarmonyDataSync synchronizes data across Harmony OS devices
type HarmonyDataSync struct {
	syncInterval  time.Duration
	syncEnabled   bool
	lastSync      time.Time
	syncedDevices map[string]time.Time
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc

	// lastSyncErr captures the most recent (*HarmonyDataSync).performSync()
	// outcome. nil means "the last sync attempt actually exchanged state with
	// the Harmony OS distributed data manager"; non-nil means "the last
	// attempt failed and the values reported by GetSyncStatus are stale".
	// Replaces the previous design where performSync stamped lastSync = now
	// unconditionally and reported PASS-bluff via GetSyncStatus.
	lastSyncErr error
}

// ErrHarmonyDistributedSyncNotImplemented is returned by
// (*HarmonyDataSync).performSync() and surfaced through GetSyncStatus
// when the Harmony OS distributed-data SDK has not been wired into
// this build.
//
// Forensic anchor (round-31 §11.4 audit, 2026-05-18): the previous
// implementation only did `ds.lastSync = time.Now()` + a log line and
// returned no error. helix-harmony's distributed-sync UI (the
// `createDistributedServicesTab` card) and the nogui `distributed sync`
// CLI command therefore reported "Sync Status: Enabled / Last Sync:
// Just now / Synced Devices: 0" forever regardless of actual cluster
// state — a §11.4 CRITICAL PASS-bluff because the surface promised
// cross-device synchronization while the body did nothing of the kind.
//
// Implement against the Harmony OS distributed data manager
// (https://developer.harmonyos.com/en/docs/documentation/doc-references-V3/js-apis-distributeddatamanager-0000001478341417-V3)
// — wire a real KVManager / SingleKVStore / DeviceKVStore session,
// enumerate connected devices through the device-manager API, push
// + pull deltas, and only THEN clear this sentinel. Until that
// happens the sentinel is the loud, programmatically-detectable
// signal that the surface is non-functional, replacing the earlier
// silent bluff.
var ErrHarmonyDistributedSyncNotImplemented = errors.New(
	"harmony_os: distributed sync has not been wired to the real Harmony OS " +
		"distributed-data SDK — performSync previously only stamped " +
		"lastSync=time.Now() and logged success, reporting 'Synced Devices: 0' " +
		"forever regardless of actual cluster state (§11.4 CRITICAL: " +
		"helix-harmony distributed sync is a no-op). Implement against the " +
		"Harmony OS distributed data manager API (KVManager / SingleKVStore / " +
		"DeviceKVStore) or remove the command and document non-support",
)

// ErrHarmonyDiscoveryNotImplemented is returned by
// (*HarmonyDistributedEngine).DiscoverDevices when invoked without
// any devices having been added through AddDevice (the synthetic
// path used by the workers tab) AND the Harmony OS device-manager
// SDK has not been wired in.
//
// Forensic anchor (round-31 §11.4 audit, 2026-05-18): the previous
// implementation returned `e.connectedDevices` unconditionally with
// no error and the inline comment "In a real implementation, this
// would use Harmony OS distributed device discovery / For now, we
// return the currently connected devices". On a fresh app launch
// (no AddDevice calls yet) the workers-tab "Discover Devices" button
// always reported "Found 0 Harmony devices" with no indication that
// discovery was a stub. That is a §11.4 HIGH PASS-bluff: the surface
// promised real distributed discovery, the body did nothing.
//
// The new contract:
//
//   - DiscoverDevices returns the live `connectedDevices` slice AND
//     a nil error when at least one device has been added through
//     AddDevice (worker enrolment path — the legitimate non-discovery
//     callers exercise this).
//   - DiscoverDevices returns the same slice AND this sentinel error
//     when the slice is empty, because in that branch the function
//     would otherwise be indistinguishable from a real discovery that
//     genuinely found no devices — exactly the bluff we are fixing.
//
// Implement against the Harmony OS device-manager / distributed
// device API (HiSysEvent + DeviceManager) to clear this sentinel.
var ErrHarmonyDiscoveryNotImplemented = errors.New(
	"harmony_os: distributed device discovery has not been wired to the " +
		"real Harmony OS device manager — DiscoverDevices previously logged " +
		"and returned an empty list, so helix-harmony 'distributed discover' " +
		"always reports 'No devices found' regardless of cluster state " +
		"(§11.4 HIGH: discovery feature is a no-op). Implement against the " +
		"Harmony OS device discovery API (DeviceManager) or remove the " +
		"command and document non-support",
)

// NewHarmonyDistributedEngine creates a new distributed engine
func NewHarmonyDistributedEngine() *HarmonyDistributedEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &HarmonyDistributedEngine{
		connectedDevices: make([]HarmonyDevice, 0),
		taskScheduler: &HarmonyTaskScheduler{
			schedulingPolicy: "balanced",
			taskQueue:        make([]*ScheduledTask, 0),
			priorityLevels:   map[string]int{"low": 1, "normal": 2, "high": 3, "critical": 4},
		},
		dataSync: NewHarmonyDataSync(),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// NewHarmonyDataSync creates a new data sync manager
func NewHarmonyDataSync() *HarmonyDataSync {
	ctx, cancel := context.WithCancel(context.Background())
	return &HarmonyDataSync{
		syncInterval:  30 * time.Second,
		syncEnabled:   true,
		lastSync:      time.Now(),
		syncedDevices: make(map[string]time.Time),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// DiscoverDevices discovers nearby Harmony OS devices. The error
// return is non-nil when no devices have been enrolled through
// AddDevice (worker-tab enrolment path) AND the Harmony OS
// device-manager SDK has not been wired in — in that branch the
// previous implementation returned (nil-or-empty slice, no error)
// which was indistinguishable from a real discovery that found
// zero devices: a §11.4 HIGH PASS-bluff because the surface
// promised real discovery and the body did nothing.
//
// New contract:
//
//   - If at least one HarmonyDevice has been added through AddDevice,
//     return that slice with a nil error (the AddDevice call-path is
//     the legitimate non-discovery enrolment surface and remains
//     functional).
//   - If the slice is empty, return it with
//     ErrHarmonyDiscoveryNotImplemented so callers can distinguish
//     "real discovery genuinely found zero devices" (still a TODO
//     once the SDK is wired) from "discovery never ran". The UI and
//     CLI MUST surface this error instead of printing the
//     previous "Found 0 Harmony devices" / "No devices found"
//     bluff message.
func (e *HarmonyDistributedEngine) DiscoverDevices() ([]HarmonyDevice, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.connectedDevices) == 0 {
		return e.connectedDevices, ErrHarmonyDiscoveryNotImplemented
	}
	return e.connectedDevices, nil
}

// AddDevice adds a device to the distributed network
func (e *HarmonyDistributedEngine) AddDevice(device HarmonyDevice) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if device already exists
	for i, d := range e.connectedDevices {
		if d.ID == device.ID {
			e.connectedDevices[i] = device
			return
		}
	}
	e.connectedDevices = append(e.connectedDevices, device)
}

// RemoveDevice removes a device from the distributed network
func (e *HarmonyDistributedEngine) RemoveDevice(deviceID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, d := range e.connectedDevices {
		if d.ID == deviceID {
			e.connectedDevices = append(e.connectedDevices[:i], e.connectedDevices[i+1:]...)
			return
		}
	}
}

// ScheduleTask schedules a task across available devices
func (e *HarmonyDistributedEngine) ScheduleTask(taskType, description string, priority int) (*ScheduledTask, error) {
	e.mu.RLock()
	devices := e.connectedDevices
	e.mu.RUnlock()

	// Find the best device based on scheduling policy
	var targetDevice *HarmonyDevice
	switch e.taskScheduler.schedulingPolicy {
	case "balanced":
		targetDevice = e.findBalancedDevice(devices)
	case "performance":
		targetDevice = e.findPerformanceDevice(devices)
	case "power_efficient":
		targetDevice = e.findPowerEfficientDevice(devices)
	default:
		targetDevice = e.findBalancedDevice(devices)
	}

	task := &ScheduledTask{
		ID:          fmt.Sprintf("htask-%d", time.Now().UnixNano()),
		Type:        taskType,
		Description: description,
		Priority:    priority,
		Status:      "scheduled",
		CreatedAt:   time.Now(),
	}

	if targetDevice != nil {
		task.DeviceID = targetDevice.ID
	} else {
		task.DeviceID = "local"
	}

	e.taskScheduler.mu.Lock()
	e.taskScheduler.taskQueue = append(e.taskScheduler.taskQueue, task)
	e.taskScheduler.mu.Unlock()

	return task, nil
}

// findBalancedDevice finds a device with balanced resource usage
func (e *HarmonyDistributedEngine) findBalancedDevice(devices []HarmonyDevice) *HarmonyDevice {
	var best *HarmonyDevice
	bestScore := float64(-1)

	for i := range devices {
		d := &devices[i]
		if d.Status != "active" || !d.Resources.Available {
			continue
		}

		// Score based on available resources (lower usage = higher score)
		score := (100 - d.Resources.CPUUsage) + (100 - d.Resources.MemoryUsage)
		if score > bestScore {
			bestScore = score
			best = d
		}
	}
	return best
}

// findPerformanceDevice finds the device with best performance characteristics
func (e *HarmonyDistributedEngine) findPerformanceDevice(devices []HarmonyDevice) *HarmonyDevice {
	var best *HarmonyDevice
	bestScore := float64(-1)

	for i := range devices {
		d := &devices[i]
		if d.Status != "active" || !d.Resources.Available {
			continue
		}

		// Prioritize devices with GPU and low CPU usage
		score := (100 - d.Resources.CPUUsage)
		if d.Resources.GPUUsage > 0 {
			score += 50 - d.Resources.GPUUsage/2
		}
		if score > bestScore {
			bestScore = score
			best = d
		}
	}
	return best
}

// ErrPowerMetricsNotAvailable signals that
// (*HarmonyDistributedEngine).findPowerEfficientDevice could not honour
// the "power_efficient" scheduling policy with real power telemetry
// because the HarmonyResources type does not carry a power-consumption
// field (no battery level, no watts-drawn, no thermal-design-power
// envelope). The caller (ScheduleTask) consumes the returned device
// nonetheless — using lowest-aggregate-resource-usage as a proxy
// (CPUUsage + MemoryUsage + GPUUsage minimisation, which positively
// correlates with active-state power draw) — but logs the sentinel so
// the surface does not claim it has performed real power-efficient
// scheduling.
//
// Forensic anchor (round-34 §11.4 audit, 2026-05-18): the previous
// implementation returned the FIRST active+available device with the
// comment "in a real implementation, would consider power metrics".
// The caller (ScheduleTask, "power_efficient" branch) routed real
// workloads to that device, certifying to operators that the platform
// was making power-conscious scheduling decisions when it was making
// arrival-order decisions. That is a §11.4 HIGH PASS-bluff: the
// scheduling-policy surface promised power awareness, the body
// performed arrival-order fallback.
//
// The new implementation:
//
//   - Uses lowest-aggregate-active-resource-usage as the honest proxy
//     (rationale: idle CPUs/GPUs draw less power than busy ones — this
//     is a strictly better signal than arrival-order even without a
//     dedicated power field, but it is still a proxy, not a real
//     measurement).
//   - Returns nil when no device is active+available (matches the prior
//     contract that ScheduleTask handles by routing to "local").
//   - Surfaces the sentinel via log.Printf so the gap is loudly
//     visible in runtime evidence captures, not silently swallowed.
//
// Wire real power telemetry (HarmonyOS PowerManager.getBatteryInfo /
// getChargingInfo / HiSysEvent power-domain probes) and add the
// corresponding fields to HarmonyResources to clear this sentinel.
var ErrPowerMetricsNotAvailable = errors.New(
	"harmony_os: power-efficient scheduling has no real power telemetry — " +
		"HarmonyResources does not carry battery level, watts drawn, or thermal " +
		"envelope fields. Falling back to lowest-aggregate-resource-usage as a " +
		"proxy. Wire HarmonyOS PowerManager API + extend HarmonyResources to " +
		"clear this sentinel",
)

// findPowerEfficientDevice picks the available device that — in the
// absence of real HarmonyOS power telemetry on the HarmonyDevice /
// HarmonyResources types — minimises aggregate active-resource usage
// (CPU+Memory+GPU). This is a documented proxy, not a real
// power-consumption measurement; see ErrPowerMetricsNotAvailable for
// the §11.4 forensic anchor explaining the gap.
//
// Returns nil if no device is active+available.
func (e *HarmonyDistributedEngine) findPowerEfficientDevice(devices []HarmonyDevice) *HarmonyDevice {
	// Loudly mark the gap in runtime evidence captures. Operators
	// inspecting logs of a scheduling decision routed through the
	// "power_efficient" policy SHOULD see this line before the chosen
	// device is committed to the task queue.
	log.Printf("harmony_os: %v", ErrPowerMetricsNotAvailable)

	var best *HarmonyDevice
	// Use +Inf as a "no candidate yet" sentinel so even the worst real
	// score (300 = 100+100+100) beats it.
	const noCandidate = float64(1 << 30)
	bestScore := noCandidate

	for i := range devices {
		d := &devices[i]
		if d.Status != "active" || !d.Resources.Available {
			continue
		}
		// Lower score = lower aggregate active-resource usage = better
		// proxy for lower active-state power draw. GPU usage is the
		// dominant term in absolute watts; CPU + memory are kept in
		// the sum because they still contribute (and because two
		// otherwise-equal-GPU candidates should prefer the lower-CPU
		// one).
		score := d.Resources.CPUUsage + d.Resources.MemoryUsage + d.Resources.GPUUsage
		if score < bestScore {
			bestScore = score
			best = d
		}
	}
	return best
}

// GetScheduledTasks returns all scheduled tasks
func (e *HarmonyDistributedEngine) GetScheduledTasks() []*ScheduledTask {
	e.taskScheduler.mu.RLock()
	defer e.taskScheduler.mu.RUnlock()

	tasks := make([]*ScheduledTask, len(e.taskScheduler.taskQueue))
	copy(tasks, e.taskScheduler.taskQueue)
	return tasks
}

// Stop stops the distributed engine
func (e *HarmonyDistributedEngine) Stop() {
	e.cancel()
	e.dataSync.Stop()
}

// StartSync starts the data synchronization process
func (ds *HarmonyDataSync) StartSync() {
	if !ds.syncEnabled {
		return
	}

	go func() {
		ticker := time.NewTicker(ds.syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ds.ctx.Done():
				return
			case <-ticker.C:
				ds.performSync()
			}
		}
	}()
}

// Stop stops the data sync process
func (ds *HarmonyDataSync) Stop() {
	ds.cancel()
}

// performSync performs the actual data synchronization. Returns
// ErrHarmonyDistributedSyncNotImplemented in this build because the
// Harmony OS distributed-data SDK has not been wired in. Crucially:
// in the not-implemented branch this function NO LONGER stamps
// lastSync = time.Now() — the previous implementation did, which
// produced a silent "Last Sync: Just now" PASS-bluff (§11.4
// CRITICAL) on every UI render. The error is recorded on the
// receiver so GetSyncStatus and any future caller (CLI, monitoring,
// telemetry) can surface the gap loudly instead of trusting the
// stale timestamp.
//
// When the real SDK lands, replace the body with the real KV-store
// push/pull, update lastSync ONLY on success, populate
// syncedDevices with the device IDs that actually round-tripped,
// and clear lastSyncErr.
func (ds *HarmonyDataSync) performSync() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.lastSyncErr = ErrHarmonyDistributedSyncNotImplemented
	log.Printf("Harmony data sync NOT performed: %v (lastSync timestamp NOT advanced)", ds.lastSyncErr)
	return ds.lastSyncErr
}

// GetSyncStatus returns the current sync status. The final error
// return is non-nil if the most recent performSync call failed —
// callers MUST inspect it instead of trusting the (enabled, lastSync,
// syncedDevices) tuple in isolation, otherwise they recreate the
// PASS-bluff fixed in round-31 §11.4.
func (ds *HarmonyDataSync) GetSyncStatus() (bool, time.Time, int, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	return ds.syncEnabled, ds.lastSync, len(ds.syncedDevices), ds.lastSyncErr
}

// HarmonySystemMonitor monitors Harmony OS system resources and performance
type HarmonySystemMonitor struct {
	cpuUsage       float64
	memoryUsage    float64
	gpuUsage       float64
	networkTraffic int64
	diskIO         int64
	temperature    float64
	powerUsage     float64
	updateInterval time.Duration
	monitoring     bool
}

// HarmonyResourceManager manages system resources and optimization
type HarmonyResourceManager struct {
	resourcePolicies map[string]string
	optimization     bool
	autoTuning       bool
}

// HarmonyServiceCoordinator coordinates distributed services
type HarmonyServiceCoordinator struct {
	services        map[string]bool
	serviceRegistry map[string]interface{}
	coordinator     *ServiceCoordinator
}

// ServiceCoordinator manages service lifecycle
type ServiceCoordinator struct {
	activeServices  []string
	failoverEnabled bool
}

// NewHarmonyApp creates a new Harmony OS application
func NewHarmonyApp() *HarmonyApp {
	return &HarmonyApp{
		fyneApp:      app.NewWithID("dev.helix.code.harmonyos"),
		themeManager: NewThemeManager(),
		tasks:        make([]APITask, 0),
		workers:      make([]APIWorker, 0),
		projects:     make([]APIProject, 0),
		sessions:     make([]APISession, 0),
		llmProviders: make([]string, 0),
		stopUpdate:   make(chan struct{}),
	}
}

// Initialize initializes the Harmony OS application
func (app *HarmonyApp) Initialize() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}
	app.config = cfg

	// Initialize API client with default URL (can be changed in settings)
	serverURL := "http://localhost:8080"
	if cfg.Server.Address != "" && cfg.Server.Port > 0 {
		serverURL = fmt.Sprintf("http://%s:%d", cfg.Server.Address, cfg.Server.Port)
	}
	app.apiClient = NewAPIClient(serverURL)

	// Initialize database (optional - continue without it if not available)
	db, err := database.New(cfg.Database)
	if err != nil {
		log.Printf("Warning: Database not available: %v (continuing without persistence)", err)
	}
	app.db = db

	// Initialize Redis (optional - continue without it if not available)
	rds, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		log.Printf("Warning: Redis not available: %v (continuing without caching)", err)
	}

	// Initialize components
	app.taskManager = task.NewTaskManager(db, rds)

	// Initialize worker manager with in-memory repository for standalone UI
	workerRepo := worker.NewInMemoryWorkerRepository()
	app.workerManager = worker.NewWorkerManager(workerRepo, 30*time.Second)

	// Initialize project manager
	app.projectManager = project.NewManager()

	// Initialize session manager
	app.sessionManager = session.NewManager()

	// Initialize LLM manager
	app.llmManager = llm.NewModelManager()

	// Initialize notification engine
	app.notificationEngine = notification.NewNotificationEngine()

	// Initialize server for API calls
	app.server = server.New(cfg, db, rds)

	// Initialize monitoring
	app.monitor = monitoring.NewMonitor()

	// Initialize hardware detector
	app.hardwareDetector = hardware.NewHardwareDetector()

	// Initialize theme manager
	app.themeManager = NewThemeManager()

	// Initialize Harmony OS specific features
	if err := app.initializeHarmonyComponents(); err != nil {
		return fmt.Errorf("failed to initialize Harmony features: %v", err)
	}

	// Setup UI
	app.SetupUI()

	// Start background data updates
	app.startDataUpdates()

	return nil
}

// startDataUpdates starts periodic background data refresh
func (app *HarmonyApp) startDataUpdates() {
	app.updateTicker = time.NewTicker(5 * time.Second)
	go func() {
		// Initial data load
		app.refreshData()

		for {
			select {
			case <-app.updateTicker.C:
				app.refreshData()
			case <-app.stopUpdate:
				app.updateTicker.Stop()
				return
			}
		}
	}()
}

// refreshData updates cached data from API and local managers
func (app *HarmonyApp) refreshData() {
	app.dataMu.Lock()
	defer app.dataMu.Unlock()

	ctx := context.Background()

	// Try to fetch tasks from API first, fallback to local
	tasks, err := app.fetchTasksFromAPI()
	if err != nil {
		// Fallback to local task manager
		log.Printf("API tasks unavailable, using local: %v", err)
	} else {
		app.tasks = tasks
	}

	// Try to fetch workers from API first, fallback to local
	workers, err := app.fetchWorkersFromAPI()
	if err != nil {
		// Fallback to local worker manager
		log.Printf("API workers unavailable, using local: %v", err)
	} else {
		app.workers = workers
	}

	// Refresh projects from local manager
	if app.projectManager != nil {
		projects, err := app.projectManager.ListProjects(ctx, "")
		if err == nil {
			app.projects = make([]APIProject, len(projects))
			for i, p := range projects {
				app.projects[i] = APIProject{
					ID:          p.ID,
					Name:        p.Name,
					Description: p.Description,
					Path:        p.Path,
					Type:        p.Type,
					Active:      p.Active,
					CreatedAt:   p.CreatedAt,
				}
			}
		}
	}

	// Refresh sessions from local manager
	if app.sessionManager != nil {
		sessions := app.sessionManager.GetAll()
		app.sessions = make([]APISession, len(sessions))
		for i, s := range sessions {
			app.sessions[i] = APISession{
				ID:          s.ID,
				Name:        s.Name,
				Description: s.Description,
				ProjectID:   s.ProjectID,
				Mode:        string(s.Mode),
				Status:      string(s.Status),
				CreatedAt:   s.CreatedAt,
			}
		}
	}

	// Refresh LLM providers
	if app.llmManager != nil {
		models := app.llmManager.GetAvailableModels()
		providers := make(map[string]bool)
		for _, model := range models {
			providers[string(model.Provider)] = true
		}
		app.llmProviders = make([]string, 0, len(providers))
		for provider := range providers {
			app.llmProviders = append(app.llmProviders, provider)
		}
	}
}

// fetchTasksFromAPI fetches tasks from the backend API
func (app *HarmonyApp) fetchTasksFromAPI() ([]APITask, error) {
	resp, err := app.apiClient.doRequest("GET", "/api/v1/tasks", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response struct {
		Tasks []APITask `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Tasks, nil
}

// fetchWorkersFromAPI fetches workers from the backend API
func (app *HarmonyApp) fetchWorkersFromAPI() ([]APIWorker, error) {
	resp, err := app.apiClient.doRequest("GET", "/api/v1/workers", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response struct {
		Workers []APIWorker `json:"workers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Workers, nil
}

// initializeHarmonyComponents initializes Harmony OS-specific features
func (app *HarmonyApp) initializeHarmonyComponents() error {
	// Initialize distributed engine
	distributedEngine := NewHarmonyDistributedEngine()

	// Initialize Harmony integration
	app.harmonyIntegration = &HarmonyIntegration{
		nativeServices: make(map[string]interface{}),
		systemAPI: &HarmonySystemAPI{
			deviceInfo: map[string]string{
				"platform":  "HarmonyOS",
				"version":   "4.0",
				"device":    "HelixCode Device",
				"ecosystem": "Harmony",
			},
			capabilities: []string{
				"distributed_computing",
				"cross_device_sync",
				"ai_acceleration",
				"multi_screen_collaboration",
				"super_device",
			},
			systemVersion: "HarmonyOS 4.0",
			kernelVersion: "Linux 5.10-Harmony",
		},
		distributedEngine: distributedEngine,
		harmonyContext:    context.Background(),
	}

	// Start data sync
	distributedEngine.dataSync.StartSync()

	// Initialize system monitor
	app.systemMonitor = &HarmonySystemMonitor{
		updateInterval: 5 * time.Second,
		monitoring:     true,
	}

	// Initialize resource manager
	app.resourceManager = &HarmonyResourceManager{
		resourcePolicies: map[string]string{
			"cpu":    "balanced",
			"memory": "optimized",
			"power":  "efficient",
		},
		optimization: true,
		autoTuning:   true,
	}

	// Initialize service coordinator
	app.serviceCoordinator = &HarmonyServiceCoordinator{
		services:        make(map[string]bool),
		serviceRegistry: make(map[string]interface{}),
		coordinator: &ServiceCoordinator{
			activeServices:  []string{},
			failoverEnabled: true,
		},
	}

	// Start system monitoring
	go app.monitorSystem()

	log.Println("Harmony OS features initialized successfully")
	return nil
}

// monitorSystem continuously monitors system resources
func (app *HarmonyApp) monitorSystem() {
	ticker := time.NewTicker(app.systemMonitor.updateInterval)
	defer ticker.Stop()

	for app.systemMonitor.monitoring {
		select {
		case <-ticker.C:
			// Update system metrics
			app.updateSystemMetrics()
		}
	}
}

// updateSystemMetrics updates current system metrics from real system data
func (app *HarmonyApp) updateSystemMetrics() {
	// Get memory statistics from runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get hardware profile
	profile := app.hardwareDetector.GetProfile()

	// Calculate CPU usage based on goroutines and CPU count
	numGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()
	// Estimate CPU usage based on goroutine count (simplified)
	estimatedCPUUsage := float64(numGoroutines) / float64(numCPU*10) * 100
	if estimatedCPUUsage > 100 {
		estimatedCPUUsage = 100
	}
	app.systemMonitor.cpuUsage = estimatedCPUUsage

	// Memory usage from runtime (convert to MB)
	app.systemMonitor.memoryUsage = float64(memStats.Alloc) / (1024 * 1024)

	// GPU usage - would need platform-specific implementation
	// For Harmony OS, this would use HarmonyOS NPU/GPU APIs
	app.systemMonitor.gpuUsage = 0 // Set to 0 when not available

	// Network traffic - would need platform-specific implementation
	// Track approximate based on time since last check
	app.systemMonitor.networkTraffic = 0

	// Disk I/O - would need platform-specific implementation
	app.systemMonitor.diskIO = 0

	// Temperature - would need platform-specific implementation
	// For Harmony OS, this would use thermal APIs
	app.systemMonitor.temperature = 0

	// Power usage - would need platform-specific implementation
	// For Harmony OS, this would use power management APIs
	app.systemMonitor.powerUsage = 0

	// Log metrics for debugging (optional)
	log.Printf("System metrics updated - CPU: %.1f%%, Memory: %.1fMB, Goroutines: %d, CPUs: %d, Arch: %s",
		app.systemMonitor.cpuUsage,
		app.systemMonitor.memoryUsage,
		numGoroutines,
		profile.CPU.Cores,
		profile.OS.Arch)
}

// GetSystemStats returns formatted system statistics for display
func (app *HarmonyApp) GetSystemStats() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	profile := app.hardwareDetector.GetProfile()

	return map[string]interface{}{
		"cpu_cores":       runtime.NumCPU(),
		"cpu_arch":        runtime.GOARCH,
		"os":              runtime.GOOS,
		"goroutines":      runtime.NumGoroutine(),
		"memory_alloc":    memStats.Alloc,
		"memory_total":    memStats.TotalAlloc,
		"memory_sys":      memStats.Sys,
		"gc_cycles":       memStats.NumGC,
		"go_version":      runtime.Version(),
		"hardware_cpu":    profile.CPU.Cores,
		"hardware_memory": profile.Memory.Total,
	}
}

// SetupUI creates and configures the user interface
func (app *HarmonyApp) SetupUI() {
	// Apply Harmony theme
	app.fyneApp.Settings().SetTheme(app.themeManager.GetCustomTheme())

	// Create main window
	app.mainWindow = app.fyneApp.NewWindow("HelixCode - Harmony OS Edition")
	app.mainWindow.Resize(fyne.NewSize(1200, 800))
	app.mainWindow.CenterOnScreen()

	// Create status bar
	app.statusBar = widget.NewLabel("Ready - Harmony OS Initialized")
	app.statusBar.Alignment = fyne.TextAlignCenter

	// Create tabs
	app.tabs = container.NewAppTabs(
		container.NewTabItem("Dashboard", app.createDashboardTab()),
		container.NewTabItem("Tasks", app.createTasksTab()),
		container.NewTabItem("Workers", app.createWorkersTab()),
		container.NewTabItem("Projects", app.createProjectsTab()),
		container.NewTabItem("Sessions", app.createSessionsTab()),
		container.NewTabItem("LLM", app.createLLMTab()),
		container.NewTabItem("Harmony System", app.createHarmonySystemTab()),
		container.NewTabItem("Distributed Services", app.createDistributedServicesTab()),
		container.NewTabItem("Resource Management", app.createResourceManagementTab()),
		container.NewTabItem("Settings", app.createSettingsTab()),
	)

	// Create main layout
	mainContent := container.NewBorder(
		nil,           // top
		app.statusBar, // bottom
		nil,           // left
		nil,           // right
		app.tabs,      // center
	)

	app.mainWindow.SetContent(mainContent)
}

// createDashboardTab creates the dashboard tab
func (app *HarmonyApp) createDashboardTab() fyne.CanvasObject {
	// Welcome label
	welcomeLabel := widget.NewLabelWithStyle(
		"HelixCode - Harmony OS Edition",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// System info
	systemInfo := widget.NewCard(
		"System Information",
		"Harmony OS Platform Details",
		widget.NewLabel(fmt.Sprintf(
			"Platform: %s\nVersion: %s\nKernel: %s\nEcosystem: Harmony",
			app.harmonyIntegration.systemAPI.systemVersion,
			app.harmonyIntegration.systemAPI.deviceInfo["version"],
			app.harmonyIntegration.systemAPI.kernelVersion,
		)),
	)

	// Quick stats
	statsCard := widget.NewCard(
		"Quick Stats",
		"Current System Status",
		widget.NewLabel("Loading stats..."),
	)

	// Harmony features
	featuresCard := widget.NewCard(
		"Harmony OS Features",
		"Advanced Capabilities",
		widget.NewLabel(fmt.Sprintf(
			"• Distributed Computing\n• Cross-Device Sync\n• AI Acceleration\n• Multi-Screen Collaboration\n• Super Device Integration",
		)),
	)

	return container.NewVBox(
		welcomeLabel,
		layout.NewSpacer(),
		systemInfo,
		statsCard,
		featuresCard,
		layout.NewSpacer(),
	)
}

// createTasksTab creates the tasks management tab
func (app *HarmonyApp) createTasksTab() fyne.CanvasObject {
	// Task list with dynamic data
	taskList := widget.NewList(
		func() int {
			app.dataMu.RLock()
			defer app.dataMu.RUnlock()
			return len(app.tasks)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Status"),
				widget.NewLabel("Type"),
				widget.NewLabel("Description"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			app.dataMu.RLock()
			defer app.dataMu.RUnlock()
			if id < len(app.tasks) {
				t := app.tasks[id]
				hbox := obj.(*fyne.Container)
				hbox.Objects[0].(*widget.Label).SetText(fmt.Sprintf("[%s]", t.Status))
				hbox.Objects[1].(*widget.Label).SetText(t.Type)
				hbox.Objects[2].(*widget.Label).SetText(t.Description)
			}
		},
	)

	taskCard := widget.NewCard("Tasks", "", taskList)

	// Task type selector for new tasks
	taskTypeSelect := widget.NewSelect([]string{"planning", "building", "testing", "refactoring", "debugging"}, nil)
	taskTypeSelect.SetSelected("building")

	// Priority selector
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
			if taskDescEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("description is required"), app.mainWindow)
				return
			}

			// Create task via distributed engine for Harmony OS
			priority := app.harmonyIntegration.distributedEngine.taskScheduler.priorityLevels[prioritySelect.Selected]
			task, err := app.harmonyIntegration.distributedEngine.ScheduleTask(
				taskTypeSelect.Selected,
				taskDescEntry.Text,
				priority,
			)
			if err != nil {
				dialog.ShowError(err, app.mainWindow)
			} else {
				taskDescEntry.SetText("")
				taskList.Refresh()
				app.statusBar.SetText(fmt.Sprintf("Task created: %s on device %s", task.ID, task.DeviceID))
				dialog.ShowInformation("Success", fmt.Sprintf("Task %s created and scheduled", task.ID), app.mainWindow)
			}
		}),
		widget.NewSeparator(),
		widget.NewButton("Refresh", func() {
			app.refreshData()
			taskList.Refresh()
			app.statusBar.SetText("Tasks refreshed")
		}),
	)

	return container.NewBorder(nil, nil, nil, actions, taskCard)
}

// createWorkersTab creates the workers management tab
func (app *HarmonyApp) createWorkersTab() fyne.CanvasObject {
	// Worker list with dynamic data
	workerList := widget.NewList(
		func() int {
			app.dataMu.RLock()
			defer app.dataMu.RUnlock()
			return len(app.workers)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Status"),
				widget.NewLabel("ID"),
				widget.NewLabel("Host"),
				widget.NewLabel("Health"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			app.dataMu.RLock()
			defer app.dataMu.RUnlock()
			if id < len(app.workers) {
				w := app.workers[id]
				hbox := obj.(*fyne.Container)
				hbox.Objects[0].(*widget.Label).SetText(fmt.Sprintf("[%s]", w.Status))
				hbox.Objects[1].(*widget.Label).SetText(w.ID)
				hbox.Objects[2].(*widget.Label).SetText(fmt.Sprintf("%s:%d", w.Host, w.Port))
				healthStatus := "unhealthy"
				if w.Healthy {
					healthStatus = "healthy"
				}
				hbox.Objects[3].(*widget.Label).SetText(healthStatus)
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
			if hostEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("host is required"), app.mainWindow)
				return
			}

			// Add worker to distributed engine as a Harmony device
			device := HarmonyDevice{
				ID:     fmt.Sprintf("worker-%s-%d", hostEntry.Text, time.Now().UnixNano()),
				Name:   fmt.Sprintf("Worker@%s", hostEntry.Text),
				Type:   "remote_worker",
				Status: "pending",
				Capabilities: []string{
					"task_execution",
					"code_analysis",
					"build",
					"test",
				},
				Resources: HarmonyResources{
					CPUUsage:    0,
					MemoryUsage: 0,
					GPUUsage:    0,
					Available:   true,
				},
				LastSeen: time.Now(),
			}

			app.harmonyIntegration.distributedEngine.AddDevice(device)

			// Also add as API worker for UI display
			app.dataMu.Lock()
			app.workers = append(app.workers, APIWorker{
				ID:           device.ID,
				Host:         hostEntry.Text,
				Port:         22,
				User:         userEntry.Text,
				Status:       "pending",
				Healthy:      false,
				Capabilities: device.Capabilities,
				LastSeen:     time.Now(),
			})
			app.dataMu.Unlock()

			hostEntry.SetText("")
			userEntry.SetText("")
			workerList.Refresh()
			app.statusBar.SetText(fmt.Sprintf("Worker %s added", device.ID))
		}),
		widget.NewSeparator(),
		widget.NewButton("Refresh", func() {
			app.refreshData()
			workerList.Refresh()
			app.statusBar.SetText("Workers refreshed")
		}),
		widget.NewButton("Discover Devices", func() {
			devices, err := app.harmonyIntegration.distributedEngine.DiscoverDevices()
			if err != nil {
				// Surface the sentinel loudly instead of printing the
				// previous "Found 0 Harmony devices" PASS-bluff (round-31
				// §11.4). The dialog text mirrors the sentinel message so
				// the user sees WHY 0 devices were returned.
				app.statusBar.SetText(fmt.Sprintf("Discover Devices: %v", err))
				dialog.ShowError(err, app.mainWindow)
				return
			}
			app.statusBar.SetText(fmt.Sprintf("Found %d Harmony devices (enrolled via AddDevice)", len(devices)))
		}),
	)

	return container.NewBorder(nil, nil, nil, actions, workerCard)
}

// createProjectsTab creates the projects tab
func (app *HarmonyApp) createProjectsTab() fyne.CanvasObject {
	// Project list with dynamic data
	app.projectList = widget.NewList(
		func() int {
			app.dataMu.RLock()
			defer app.dataMu.RUnlock()
			return len(app.projects)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Name"),
				widget.NewLabel("Type"),
				widget.NewLabel("Status"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			app.dataMu.RLock()
			defer app.dataMu.RUnlock()
			if id < len(app.projects) {
				p := app.projects[id]
				hbox := obj.(*fyne.Container)
				hbox.Objects[0].(*widget.Label).SetText(p.Name)
				hbox.Objects[1].(*widget.Label).SetText(fmt.Sprintf("(%s)", p.Type))
				activeStatus := ""
				if p.Active {
					activeStatus = " [ACTIVE]"
				}
				hbox.Objects[2].(*widget.Label).SetText(activeStatus)
			}
		},
	)

	// Project details panel
	projectDetailsLabel := widget.NewLabel("Select a project to view details")
	projectDetailsLabel.Wrapping = fyne.TextWrapWord

	app.projectList.OnSelected = func(id widget.ListItemID) {
		app.dataMu.RLock()
		defer app.dataMu.RUnlock()
		if id < len(app.projects) {
			p := app.projects[id]
			details := fmt.Sprintf("Name: %s\nType: %s\nPath: %s\nDescription: %s\nCreated: %s",
				p.Name, p.Type, p.Path, p.Description,
				p.CreatedAt.Format(time.RFC822))
			projectDetailsLabel.SetText(details)
		}
	}

	projectListCard := widget.NewCard("Projects", "", app.projectList)
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
			if app.projectManager != nil && nameEntry.Text != "" && pathEntry.Text != "" {
				ctx := context.Background()
				proj, err := app.projectManager.CreateProject(ctx, nameEntry.Text, descEntry.Text, pathEntry.Text, typeSelect.Selected)
				if err != nil {
					dialog.ShowError(err, app.mainWindow)
				} else {
					nameEntry.SetText("")
					descEntry.SetText("")
					pathEntry.SetText("")
					app.refreshData()
					app.projectList.Refresh()
					app.statusBar.SetText(fmt.Sprintf("Project %s created", proj.Name))
					dialog.ShowInformation("Success", "Project created successfully", app.mainWindow)
				}
			} else {
				dialog.ShowError(fmt.Errorf("name and path are required"), app.mainWindow)
			}
		}),
		widget.NewSeparator(),
		widget.NewButton("Set as Active", func() {
			if app.projectList.Length() > 0 {
				// Get selected project
				app.dataMu.RLock()
				selectedIndex := -1
				// Note: In Fyne, we need to track selection separately
				app.dataMu.RUnlock()

				if selectedIndex >= 0 {
					ctx := context.Background()
					p := app.projects[selectedIndex]
					err := app.projectManager.SetActiveProject(ctx, p.ID)
					if err != nil {
						dialog.ShowError(err, app.mainWindow)
					} else {
						app.refreshData()
						app.projectList.Refresh()
						app.statusBar.SetText(fmt.Sprintf("Project %s set as active", p.Name))
					}
				}
			}
		}),
		widget.NewButton("Refresh", func() {
			app.refreshData()
			app.projectList.Refresh()
			app.statusBar.SetText("Projects refreshed")
		}),
	)

	leftPanel := container.NewVSplit(projectListCard, projectDetailsCard)
	leftPanel.SetOffset(0.6)

	return container.NewBorder(nil, nil, nil, createForm, leftPanel)
}

// createSessionsTab creates the sessions tab
func (app *HarmonyApp) createSessionsTab() fyne.CanvasObject {
	// Session list with dynamic data
	app.sessionList = widget.NewList(
		func() int {
			app.dataMu.RLock()
			defer app.dataMu.RUnlock()
			return len(app.sessions)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Name"),
				widget.NewLabel("Status"),
				widget.NewLabel("Mode"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			app.dataMu.RLock()
			defer app.dataMu.RUnlock()
			if id < len(app.sessions) {
				s := app.sessions[id]
				hbox := obj.(*fyne.Container)
				hbox.Objects[0].(*widget.Label).SetText(s.Name)
				hbox.Objects[1].(*widget.Label).SetText(fmt.Sprintf("[%s]", s.Status))
				hbox.Objects[2].(*widget.Label).SetText(s.Mode)
			}
		},
	)

	// Session details panel
	sessionDetailsLabel := widget.NewLabel("Select a session to view details")
	sessionDetailsLabel.Wrapping = fyne.TextWrapWord

	selectedSessionID := ""
	app.sessionList.OnSelected = func(id widget.ListItemID) {
		app.dataMu.RLock()
		defer app.dataMu.RUnlock()
		if id < len(app.sessions) {
			s := app.sessions[id]
			selectedSessionID = s.ID
			details := fmt.Sprintf("Name: %s\nMode: %s\nStatus: %s\nProject ID: %s\nDescription: %s\nCreated: %s",
				s.Name, s.Mode, s.Status, s.ProjectID, s.Description,
				s.CreatedAt.Format(time.RFC822))
			sessionDetailsLabel.SetText(details)
		}
	}

	sessionListCard := widget.NewCard("Sessions", "", app.sessionList)
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
			if app.sessionManager != nil && nameEntry.Text != "" && projectIDEntry.Text != "" {
				mode := session.Mode(modeSelect.Selected)
				sess, err := app.sessionManager.Create(projectIDEntry.Text, nameEntry.Text, descEntry.Text, mode)
				if err != nil {
					dialog.ShowError(err, app.mainWindow)
				} else {
					nameEntry.SetText("")
					descEntry.SetText("")
					projectIDEntry.SetText("")
					app.refreshData()
					app.sessionList.Refresh()
					app.statusBar.SetText(fmt.Sprintf("Session %s created", sess.Name))
					dialog.ShowInformation("Success", "Session created successfully", app.mainWindow)
				}
			} else {
				dialog.ShowError(fmt.Errorf("name and project ID are required"), app.mainWindow)
			}
		}),
		widget.NewSeparator(),
		widget.NewLabel("Session Controls:"),
		widget.NewButton("Start Session", func() {
			if app.sessionManager != nil && selectedSessionID != "" {
				err := app.sessionManager.Start(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, app.mainWindow)
				} else {
					app.refreshData()
					app.sessionList.Refresh()
					app.statusBar.SetText("Session started")
				}
			}
		}),
		widget.NewButton("Pause Session", func() {
			if app.sessionManager != nil && selectedSessionID != "" {
				err := app.sessionManager.Pause(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, app.mainWindow)
				} else {
					app.refreshData()
					app.sessionList.Refresh()
					app.statusBar.SetText("Session paused")
				}
			}
		}),
		widget.NewButton("Resume Session", func() {
			if app.sessionManager != nil && selectedSessionID != "" {
				err := app.sessionManager.Resume(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, app.mainWindow)
				} else {
					app.refreshData()
					app.sessionList.Refresh()
					app.statusBar.SetText("Session resumed")
				}
			}
		}),
		widget.NewButton("Complete Session", func() {
			if app.sessionManager != nil && selectedSessionID != "" {
				err := app.sessionManager.Complete(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, app.mainWindow)
				} else {
					app.refreshData()
					app.sessionList.Refresh()
					app.statusBar.SetText("Session completed")
				}
			}
		}),
		widget.NewSeparator(),
		widget.NewButton("Refresh", func() {
			app.refreshData()
			app.sessionList.Refresh()
			app.statusBar.SetText("Sessions refreshed")
		}),
	)

	leftPanel := container.NewVSplit(sessionListCard, sessionDetailsCard)
	leftPanel.SetOffset(0.6)

	return container.NewBorder(nil, nil, nil, actions, leftPanel)
}

// createLLMTab creates the LLM tab
func (app *HarmonyApp) createLLMTab() fyne.CanvasObject {
	// Available models list
	modelList := widget.NewList(
		func() int {
			if app.llmManager == nil {
				return 0
			}
			return len(app.llmManager.GetAvailableModels())
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Model"),
				widget.NewLabel("Provider"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			models := app.llmManager.GetAvailableModels()
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
		models := app.llmManager.GetAvailableModels()
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
	app.chatHistory = widget.NewMultiLineEntry()
	app.chatHistory.SetPlaceHolder("Chat history will appear here...")
	app.chatHistory.Disable()
	app.chatHistory.Wrapping = fyne.TextWrapWord

	app.chatInput = widget.NewMultiLineEntry()
	app.chatInput.SetPlaceHolder("Type your message here...")
	app.chatInput.SetMinRowsVisible(3)

	// Provider/model selection for chat
	app.llmProviderSel = widget.NewSelect([]string{"ollama", "openai", "anthropic", "gemini", "local"}, nil)
	app.llmProviderSel.SetSelected("ollama")

	modelNameEntry := widget.NewEntry()
	modelNameEntry.SetPlaceHolder("Model name (e.g., llama2)")
	modelNameEntry.SetText("llama2")

	sendButton := widget.NewButton("Send Message", func() {
		if app.chatInput.Text == "" {
			return
		}

		// Add user message to history
		currentHistory := app.chatHistory.Text
		userMessage := app.chatInput.Text
		userMsg := fmt.Sprintf("\n[User]: %s\n", userMessage)
		app.chatHistory.SetText(currentHistory + userMsg)

		// Clear input immediately
		app.chatInput.SetText("")

		// Make LLM call in goroutine to not block UI
		// In Harmony OS, this could leverage distributed AI capabilities across devices
		go func(msg string) {
			var responseMsg string
			providerName := app.llmProviderSel.Selected
			modelName := modelNameEntry.Text

			if app.llmManager != nil {
				// Get provider from manager using provider type
				providerType := llm.ProviderType(providerName)
				provider, err := app.llmManager.GetProviderForModel(modelName, providerType)
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
			app.chatHistory.SetText(app.chatHistory.Text + responseMsg)
		}(userMessage)
	})

	clearButton := widget.NewButton("Clear Chat", func() {
		app.chatHistory.SetText("")
	})

	chatControls := container.NewVBox(
		widget.NewLabel("Chat Settings:"),
		widget.NewLabel("Provider:"),
		app.llmProviderSel,
		widget.NewLabel("Model:"),
		modelNameEntry,
		widget.NewSeparator(),
		sendButton,
		clearButton,
	)

	chatPanel := container.NewBorder(
		widget.NewLabel("Chat with AI"),
		container.NewBorder(nil, nil, nil, chatControls, app.chatInput),
		nil, nil,
		app.chatHistory,
	)

	chatCard := widget.NewCard("LLM Chat", "", chatPanel)

	// Provider health status
	healthLabel := widget.NewLabel("Provider Health:\nChecking...")

	// Start health check goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		checkHealth := func() {
			if app.llmManager == nil {
				healthLabel.SetText("Provider Health:\nNo LLM manager available")
				return
			}
			ctx := context.Background()
			health := app.llmManager.HealthCheck(ctx)
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

// createHarmonySystemTab creates the Harmony OS system monitoring tab
func (app *HarmonyApp) createHarmonySystemTab() fyne.CanvasObject {
	// System metrics
	metricsLabel := widget.NewLabel("System Metrics")

	cpuLabel := widget.NewLabel(fmt.Sprintf("CPU Usage: %.1f%%", app.systemMonitor.cpuUsage))
	memLabel := widget.NewLabel(fmt.Sprintf("Memory Usage: %.0f MB", app.systemMonitor.memoryUsage))
	gpuLabel := widget.NewLabel(fmt.Sprintf("GPU Usage: %.1f%%", app.systemMonitor.gpuUsage))
	tempLabel := widget.NewLabel(fmt.Sprintf("Temperature: %.1f°C", app.systemMonitor.temperature))
	powerLabel := widget.NewLabel(fmt.Sprintf("Power Usage: %.1fW", app.systemMonitor.powerUsage))

	metricsCard := widget.NewCard(
		"System Monitoring",
		"Real-time Performance Metrics",
		container.NewVBox(
			cpuLabel,
			memLabel,
			gpuLabel,
			tempLabel,
			powerLabel,
		),
	)

	// Harmony capabilities
	capabilitiesCard := widget.NewCard(
		"Harmony OS Capabilities",
		"Available Features",
		widget.NewLabel(fmt.Sprintf("• %s",
			fmt.Sprintf("%v", app.harmonyIntegration.systemAPI.capabilities)),
		),
	)

	return container.NewVBox(
		metricsLabel,
		metricsCard,
		capabilitiesCard,
	)
}

// createDistributedServicesTab creates the distributed services tab
func (app *HarmonyApp) createDistributedServicesTab() fyne.CanvasObject {
	// Connected devices
	devicesLabel := widget.NewLabel(fmt.Sprintf("Connected Devices: %d",
		len(app.harmonyIntegration.distributedEngine.connectedDevices)))

	// Task scheduler info
	schedulerCard := widget.NewCard(
		"Task Scheduler",
		"Distributed Task Scheduling",
		widget.NewLabel(fmt.Sprintf(
			"Policy: %s\nQueue Size: %d",
			app.harmonyIntegration.distributedEngine.taskScheduler.schedulingPolicy,
			len(app.harmonyIntegration.distributedEngine.taskScheduler.taskQueue),
		)),
	)

	// Data sync info. Reads sync status through GetSyncStatus so the
	// lastSyncErr sentinel surface from round-31 §11.4 is shown to the
	// user instead of the previous "Last Sync: Just now" PASS-bluff.
	enabled, lastSync, syncedCount, lastSyncErr := app.harmonyIntegration.distributedEngine.dataSync.GetSyncStatus()
	syncStatusText := fmt.Sprintf(
		"Sync Enabled: %v\nInterval: %v\nLast Successful Sync: %s\nSynced Devices: %d",
		enabled,
		app.harmonyIntegration.distributedEngine.dataSync.syncInterval,
		lastSync.Format(time.RFC3339),
		syncedCount,
	)
	if lastSyncErr != nil {
		syncStatusText += fmt.Sprintf("\n\nLast Sync Result: FAILED\nError: %v", lastSyncErr)
	}
	syncCard := widget.NewCard(
		"Data Synchronization",
		"Cross-Device Data Sync",
		widget.NewLabel(syncStatusText),
	)

	return container.NewVBox(
		devicesLabel,
		schedulerCard,
		syncCard,
	)
}

// createResourceManagementTab creates the resource management tab
func (app *HarmonyApp) createResourceManagementTab() fyne.CanvasObject {
	// Resource policies
	policiesLabel := widget.NewLabel("Resource Optimization Policies")

	policiesCard := widget.NewCard(
		"Active Policies",
		"Resource Management Configuration",
		widget.NewLabel(fmt.Sprintf(
			"CPU Policy: %s\nMemory Policy: %s\nPower Policy: %s\nOptimization: %v\nAuto-Tuning: %v",
			app.resourceManager.resourcePolicies["cpu"],
			app.resourceManager.resourcePolicies["memory"],
			app.resourceManager.resourcePolicies["power"],
			app.resourceManager.optimization,
			app.resourceManager.autoTuning,
		)),
	)

	// Service coordinator
	servicesCard := widget.NewCard(
		"Service Coordinator",
		"Distributed Service Management",
		widget.NewLabel(fmt.Sprintf(
			"Active Services: %d\nFailover: %v",
			len(app.serviceCoordinator.coordinator.activeServices),
			app.serviceCoordinator.coordinator.failoverEnabled,
		)),
	)

	return container.NewVBox(
		policiesLabel,
		policiesCard,
		servicesCard,
	)
}

// createSettingsTab creates the settings tab
func (app *HarmonyApp) createSettingsTab() fyne.CanvasObject {
	// Theme selector
	themeLabel := widget.NewLabel("Theme Selection")
	themeSelect := widget.NewSelect(
		[]string{"Dark", "Light", "Helix", "Harmony"},
		func(selected string) {
			app.themeManager.SetTheme(selected)
			app.fyneApp.Settings().SetTheme(app.themeManager.GetCustomTheme())
			app.statusBar.SetText(fmt.Sprintf("Theme changed to: %s", selected))
		},
	)
	themeSelect.SetSelected("Harmony")

	// Server controls
	serverLabel := widget.NewLabel("Server Controls")
	startServerBtn := widget.NewButton("Start Server", func() {
		go func() {
			if err := app.server.Start(); err != nil {
				log.Printf("Server error: %v", err)
				app.statusBar.SetText(fmt.Sprintf("Server error: %v", err))
			}
		}()
		app.statusBar.SetText("Server started")
	})

	stopServerBtn := widget.NewButton("Stop Server", func() {
		if err := app.server.Shutdown(context.Background()); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		app.statusBar.SetText("Server stopped")
	})

	serverControls := container.NewHBox(startServerBtn, stopServerBtn)

	return container.NewVBox(
		themeLabel,
		themeSelect,
		layout.NewSpacer(),
		serverLabel,
		serverControls,
		layout.NewSpacer(),
	)
}

// Run starts the Harmony OS application
func (app *HarmonyApp) Run() {
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start signal handler in goroutine
	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		app.fyneApp.Quit()
	}()

	// Show window and run (blocks until window closes)
	app.mainWindow.ShowAndRun()
}

// Cleanup performs cleanup on application shutdown
func (app *HarmonyApp) Cleanup() {
	// Stop background updates
	if app.stopUpdate != nil {
		close(app.stopUpdate)
	}

	// Stop system monitoring
	app.systemMonitor.monitoring = false

	// Stop distributed engine
	if app.harmonyIntegration != nil && app.harmonyIntegration.distributedEngine != nil {
		app.harmonyIntegration.distributedEngine.Stop()
	}

	// Shutdown server
	if app.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}

	// Close database
	if app.db != nil {
		app.db.Close()
	}

	log.Println("Harmony OS application cleaned up successfully")
}

func main() {
	// Create application
	harmonyApp := NewHarmonyApp()

	// Initialize
	if err := harmonyApp.Initialize(); err != nil {
		log.Fatalf("Failed to initialize Harmony OS application: %v", err)
		os.Exit(1)
	}

	// Run application (SetupUI is already called in Initialize)
	log.Println("Starting HelixCode Harmony OS Edition...")
	harmonyApp.Run()

	// Cleanup on exit
	harmonyApp.Cleanup()
}
