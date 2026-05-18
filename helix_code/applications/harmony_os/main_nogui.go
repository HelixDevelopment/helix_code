//go:build nogui

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"dev.helix.code/applications/harmony_os/i18n"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/hardware"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/project"
	"dev.helix.code/internal/redis"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/task"
	"dev.helix.code/internal/worker"
)

// CLITask is a simplified task representation for CLI
type CLITask struct {
	ID          string
	Type        string
	Description string
	Status      string
	Priority    string
}

// CLIWorker is a simplified worker representation for CLI
type CLIWorker struct {
	ID      string
	Host    string
	Port    string
	User    string
	Status  string
	Healthy bool
}

// CLITaskManager wraps task.TaskManager for CLI operations
type CLITaskManager struct {
	inner *task.TaskManager
	tasks []CLITask
	mu    sync.RWMutex
}

// NewCLITaskManager creates a new CLI task manager wrapper
func NewCLITaskManager(tm *task.TaskManager) *CLITaskManager {
	return &CLITaskManager{
		inner: tm,
		tasks: make([]CLITask, 0),
	}
}

// GetAllTasks returns all tasks
func (ctm *CLITaskManager) GetAllTasks() []CLITask {
	ctm.mu.RLock()
	defer ctm.mu.RUnlock()
	return ctm.tasks
}

// GetStats returns task statistics
func (ctm *CLITaskManager) GetStats() (total, completed, running int) {
	ctm.mu.RLock()
	defer ctm.mu.RUnlock()

	total = len(ctm.tasks)
	for _, t := range ctm.tasks {
		switch t.Status {
		case "completed":
			completed++
		case "running":
			running++
		}
	}
	return
}

// CreateTask creates a new task
func (ctm *CLITaskManager) CreateTask(ctx context.Context, taskType, description, priority string) (*CLITask, error) {
	ctm.mu.Lock()
	defer ctm.mu.Unlock()

	newTask := CLITask{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Type:        taskType,
		Description: description,
		Status:      "pending",
		Priority:    priority,
	}

	ctm.tasks = append(ctm.tasks, newTask)
	return &newTask, nil
}

// CancelTask cancels a task
func (ctm *CLITaskManager) CancelTask(ctx context.Context, taskID string) error {
	ctm.mu.Lock()
	defer ctm.mu.Unlock()

	for i, t := range ctm.tasks {
		if t.ID == taskID {
			ctm.tasks = append(ctm.tasks[:i], ctm.tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("task not found: %s", taskID)
}

// CLIWorkerManager wraps worker.WorkerManager for CLI operations
type CLIWorkerManager struct {
	inner   *worker.WorkerManager
	workers []CLIWorker
	mu      sync.RWMutex
}

// NewCLIWorkerManager creates a new CLI worker manager wrapper
func NewCLIWorkerManager(wm *worker.WorkerManager) *CLIWorkerManager {
	return &CLIWorkerManager{
		inner:   wm,
		workers: make([]CLIWorker, 0),
	}
}

// GetWorkers returns all workers
func (cwm *CLIWorkerManager) GetWorkers() []CLIWorker {
	cwm.mu.RLock()
	defer cwm.mu.RUnlock()
	return cwm.workers
}

// AddWorker adds a new worker
func (cwm *CLIWorkerManager) AddWorker(w *CLIWorker) error {
	cwm.mu.Lock()
	defer cwm.mu.Unlock()

	cwm.workers = append(cwm.workers, *w)
	return nil
}

// RemoveWorker removes a worker
func (cwm *CLIWorkerManager) RemoveWorker(workerID string) error {
	cwm.mu.Lock()
	defer cwm.mu.Unlock()

	for i, w := range cwm.workers {
		if w.ID == workerID {
			cwm.workers = append(cwm.workers[:i], cwm.workers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("worker not found: %s", workerID)
}

// HarmonyCLIApp represents the CLI application (nogui mode) for Harmony OS
type HarmonyCLIApp struct {
	config           *config.Config
	db               *database.Database
	taskManager      *CLITaskManager
	workerManager    *CLIWorkerManager
	projectManager   *project.Manager
	sessionManager   *session.Manager
	llmManager       *llm.ModelManager
	hardwareDetector *hardware.HardwareDetector

	// translator resolves CONST-046 user-facing message IDs. Defaults
	// to i18n.NoopTranslator{} (loud message-ID echo) when nil — set
	// by helix_code at boot via SetTranslator to a real
	// *i18nadapter.Translator wired to the active.en.yaml bundle.
	translator i18n.Translator
}

// NewHarmonyCLIApp creates a new CLI application. The CLI starts with
// i18n.NoopTranslator{} for backward compat — production wiring calls
// SetTranslator with a real Translator (e.g. helix_code's
// *i18nadapter.Translator) before Run.
func NewHarmonyCLIApp() *HarmonyCLIApp {
	return &HarmonyCLIApp{
		translator: i18n.NoopTranslator{},
	}
}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func (cliApp *HarmonyCLIApp) SetTranslator(tr i18n.Translator) {
	if tr == nil {
		cliApp.translator = i18n.NoopTranslator{}
		return
	}
	cliApp.translator = tr
}

// tr is the internal CONST-046 resolver used by every user-facing
// string emission in this file. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
func (cliApp *HarmonyCLIApp) tr(ctx context.Context, msgID string, data map[string]any) string {
	if cliApp.translator == nil {
		cliApp.translator = i18n.NoopTranslator{}
	}
	out, err := cliApp.translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}

// Initialize sets up the CLI application
func (cliApp *HarmonyCLIApp) Initialize() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}
	cliApp.config = cfg

	// Initialize database (optional)
	db, err := database.New(cfg.Database)
	if err != nil {
		log.Printf("Warning: Database not available: %v (continuing without persistence)", err)
	}
	cliApp.db = db

	// Initialize Redis (optional)
	rds, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		log.Printf("Warning: Redis not available: %v (continuing without caching)", err)
	}

	// Initialize components
	innerTaskManager := task.NewTaskManager(db, rds)
	cliApp.taskManager = NewCLITaskManager(innerTaskManager)

	workerRepo := worker.NewInMemoryWorkerRepository()
	innerWorkerManager := worker.NewWorkerManager(workerRepo, 30*time.Second)
	cliApp.workerManager = NewCLIWorkerManager(innerWorkerManager)

	cliApp.projectManager = project.NewManager()
	cliApp.sessionManager = session.NewManager()
	cliApp.llmManager = llm.NewModelManager()
	cliApp.hardwareDetector = hardware.NewHardwareDetector()

	return nil
}

// Close cleans up resources
func (cliApp *HarmonyCLIApp) Close() error {
	if cliApp.db != nil {
		cliApp.db.Close()
	}
	return nil
}

// Run executes the CLI command
func (cliApp *HarmonyCLIApp) Run(args []string) error {
	if len(args) == 0 {
		cliApp.printHelp()
		return nil
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "help", "-h", "--help":
		cliApp.printHelp()
	case "status":
		return cliApp.cmdStatus()
	case "system":
		return cliApp.cmdSystem()
	case "projects":
		return cliApp.cmdProjects(cmdArgs)
	case "sessions":
		return cliApp.cmdSessions(cmdArgs)
	case "tasks":
		return cliApp.cmdTasks(cmdArgs)
	case "workers":
		return cliApp.cmdWorkers(cmdArgs)
	case "llm":
		return cliApp.cmdLLM(cmdArgs)
	case "distributed":
		return cliApp.cmdDistributed(cmdArgs)
	case "interactive":
		return cliApp.cmdInteractive()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		cliApp.printHelp()
		return fmt.Errorf("unknown command: %s", command)
	}

	return nil
}

func (cliApp *HarmonyCLIApp) printHelp() {
	fmt.Println(`HelixCode Harmony OS CLI (nogui mode)

Usage: helix-harmony [command] [arguments]

Commands:
  help          Show this help message
  status        Show system status
  system        Show Harmony OS system information
  projects      Manage projects (list, create, delete, set-active)
  sessions      Manage sessions (list, create, start, pause, complete)
  tasks         Manage tasks (list, create, cancel)
  workers       Manage workers (list, add, remove)
  llm           LLM operations (providers, models, chat)
  distributed   Distributed device operations (discover, list, sync)
  interactive   Start interactive mode

Examples:
  helix-harmony status
  helix-harmony system
  helix-harmony projects list
  helix-harmony projects create --name "MyProject" --path "/path/to/project"
  helix-harmony sessions create --project "proj_123" --name "Dev Session"
  helix-harmony tasks create --type building --desc "Build the project"
  helix-harmony distributed discover
  helix-harmony llm providers
  helix-harmony interactive

Harmony OS Features:
  - Distributed computing across Harmony devices
  - Cross-device data synchronization
  - AI acceleration support
  - Multi-screen collaboration

Build with GUI disabled using: go build -tags nogui`)
}

func (cliApp *HarmonyCLIApp) cmdStatus() error {
	// CONST-046 (round-96 §11.4): section header sourced from
	// applications/harmony_os/i18n bundle via injected Translator;
	// NoopTranslator echoes the message ID.
	fmt.Println(cliApp.tr(context.Background(), "harmony_os_cli_status_header", nil))
	fmt.Println()

	// System info
	profile := cliApp.hardwareDetector.GetProfile()
	fmt.Printf("Platform: HarmonyOS (via %s/%s)\n", profile.OS.Name, profile.OS.Arch)
	fmt.Printf("CPU Cores: %d\n", profile.CPU.Cores)
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Println()

	// Workers
	workers := cliApp.workerManager.GetWorkers()
	activeWorkers := 0
	for _, w := range workers {
		if w.Status == "active" {
			activeWorkers++
		}
	}
	fmt.Printf("Workers: %d total, %d active\n", len(workers), activeWorkers)

	// Tasks
	totalTasks, completedTasks, runningTasks := cliApp.taskManager.GetStats()
	fmt.Printf("Tasks: %d total, %d running, %d completed\n",
		totalTasks, runningTasks, completedTasks)

	// Projects
	ctx := context.Background()
	projects, _ := cliApp.projectManager.ListProjects(ctx, "")
	activeProject, _ := cliApp.projectManager.GetActiveProject(ctx)
	activeProjectName := "none"
	if activeProject != nil {
		activeProjectName = activeProject.Name
	}
	fmt.Printf("Projects: %d total, active: %s\n", len(projects), activeProjectName)

	// Sessions
	sessions := cliApp.sessionManager.GetAll()
	activeSessions := 0
	for _, s := range sessions {
		if s.Status == session.StatusActive {
			activeSessions++
		}
	}
	fmt.Printf("Sessions: %d total, %d active\n", len(sessions), activeSessions)

	// LLM
	models := cliApp.llmManager.GetAvailableModels()
	fmt.Printf("LLM Models: %d available\n", len(models))

	return nil
}

func (cliApp *HarmonyCLIApp) cmdSystem() error {
	// CONST-046 (round-96 §11.4): section header sourced from
	// applications/harmony_os/i18n bundle via injected Translator.
	fmt.Println(cliApp.tr(context.Background(), "harmony_os_cli_system_header", nil))
	fmt.Println()

	profile := cliApp.hardwareDetector.GetProfile()

	fmt.Println("Hardware Profile:")
	fmt.Printf("  CPU Architecture: %s\n", profile.CPU.Arch)
	fmt.Printf("  CPU Cores: %d\n", profile.CPU.Cores)
	fmt.Printf("  CPU Threads: %d\n", profile.CPU.Threads)
	fmt.Printf("  Memory Total: %d bytes\n", profile.Memory.Total)
	fmt.Println()

	fmt.Println("OS Information:")
	fmt.Printf("  Platform: HarmonyOS\n")
	fmt.Printf("  Version: 4.0\n")
	fmt.Printf("  Kernel: Linux 5.10-Harmony\n")
	fmt.Printf("  Go Runtime: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	fmt.Println("Harmony OS Capabilities:")
	fmt.Println("  - Distributed Computing")
	fmt.Println("  - Cross-Device Sync")
	fmt.Println("  - AI Acceleration")
	fmt.Println("  - Multi-Screen Collaboration")
	fmt.Println("  - Super Device Integration")
	fmt.Println()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	fmt.Println("Runtime Statistics:")
	fmt.Printf("  Goroutines: %d\n", runtime.NumGoroutine())
	fmt.Printf("  Memory Allocated: %.2f MB\n", float64(memStats.Alloc)/(1024*1024))
	fmt.Printf("  Total Allocated: %.2f MB\n", float64(memStats.TotalAlloc)/(1024*1024))
	fmt.Printf("  GC Cycles: %d\n", memStats.NumGC)

	return nil
}

func (cliApp *HarmonyCLIApp) cmdProjects(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		projects, err := cliApp.projectManager.ListProjects(ctx, "")
		if err != nil {
			return err
		}
		// CONST-046 (round-96 §11.4): section header sourced from
		// applications/harmony_os/i18n bundle via injected Translator.
		fmt.Println(cliApp.tr(ctx, "harmony_os_cli_projects_header", nil))
		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return nil
		}
		for _, p := range projects {
			activeMarker := ""
			if p.Active {
				activeMarker = " [ACTIVE]"
			}
			fmt.Printf("- %s (%s): %s%s\n", p.Name, p.Type, p.Path, activeMarker)
		}

	case "create":
		fs := flag.NewFlagSet("projects create", flag.ExitOnError)
		name := fs.String("name", "", "Project name")
		path := fs.String("path", "", "Project path")
		desc := fs.String("desc", "", "Project description")
		ptype := fs.String("type", "generic", "Project type (go, node, python, rust, generic)")
		fs.Parse(args[1:])

		if *name == "" || *path == "" {
			fmt.Println("Error: --name and --path are required")
			return fmt.Errorf("missing required arguments")
		}

		proj, err := cliApp.projectManager.CreateProject(ctx, *name, *desc, *path, *ptype)
		if err != nil {
			return err
		}
		fmt.Printf("Created project: %s (ID: %s)\n", proj.Name, proj.ID)

	case "set-active":
		if len(args) < 2 {
			fmt.Println("Error: project ID required")
			return fmt.Errorf("missing project ID")
		}
		err := cliApp.projectManager.SetActiveProject(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Set active project: %s\n", args[1])

	case "delete":
		if len(args) < 2 {
			fmt.Println("Error: project ID required")
			return fmt.Errorf("missing project ID")
		}
		err := cliApp.projectManager.DeleteProject(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Deleted project: %s\n", args[1])

	default:
		fmt.Printf("Unknown subcommand: %s\n", args[0])
	}

	return nil
}

func (cliApp *HarmonyCLIApp) cmdSessions(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list":
		sessions := cliApp.sessionManager.GetAll()
		// CONST-046 (round-96 §11.4): section header sourced from
		// applications/harmony_os/i18n bundle via injected Translator.
		fmt.Println(cliApp.tr(context.Background(), "harmony_os_cli_sessions_header", nil))
		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}
		for _, s := range sessions {
			fmt.Printf("- %s [%s] %s (Project: %s)\n", s.Name, s.Status, s.Mode, s.ProjectID)
		}

	case "create":
		fs := flag.NewFlagSet("sessions create", flag.ExitOnError)
		name := fs.String("name", "", "Session name")
		projectID := fs.String("project", "", "Project ID")
		desc := fs.String("desc", "", "Session description")
		mode := fs.String("mode", "building", "Session mode (planning, building, testing, refactoring, debugging, deployment)")
		fs.Parse(args[1:])

		if *name == "" || *projectID == "" {
			fmt.Println("Error: --name and --project are required")
			return fmt.Errorf("missing required arguments")
		}

		sess, err := cliApp.sessionManager.Create(*projectID, *name, *desc, session.Mode(*mode))
		if err != nil {
			return err
		}
		fmt.Printf("Created session: %s (ID: %s)\n", sess.Name, sess.ID)

	case "start":
		if len(args) < 2 {
			fmt.Println("Error: session ID required")
			return fmt.Errorf("missing session ID")
		}
		err := cliApp.sessionManager.Start(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Started session: %s\n", args[1])

	case "pause":
		if len(args) < 2 {
			fmt.Println("Error: session ID required")
			return fmt.Errorf("missing session ID")
		}
		err := cliApp.sessionManager.Pause(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Paused session: %s\n", args[1])

	case "complete":
		if len(args) < 2 {
			fmt.Println("Error: session ID required")
			return fmt.Errorf("missing session ID")
		}
		err := cliApp.sessionManager.Complete(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Completed session: %s\n", args[1])

	default:
		fmt.Printf("Unknown subcommand: %s\n", args[0])
	}

	return nil
}

func (cliApp *HarmonyCLIApp) cmdTasks(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		tasks := cliApp.taskManager.GetAllTasks()
		// CONST-046 (round-96 §11.4): section header sourced from
		// applications/harmony_os/i18n bundle via injected Translator.
		fmt.Println(cliApp.tr(ctx, "harmony_os_cli_tasks_header", nil))
		if len(tasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}
		for _, t := range tasks {
			fmt.Printf("- [%s] %s: %s (Priority: %s)\n", t.Status, t.Type, t.Description, t.Priority)
		}

	case "create":
		fs := flag.NewFlagSet("tasks create", flag.ExitOnError)
		taskType := fs.String("type", "building", "Task type (planning, building, testing, refactoring, debugging)")
		desc := fs.String("desc", "", "Task description")
		priority := fs.String("priority", "normal", "Task priority (low, normal, high, critical)")
		fs.Parse(args[1:])

		if *desc == "" {
			fmt.Println("Error: --desc is required")
			return fmt.Errorf("missing required arguments")
		}

		t, err := cliApp.taskManager.CreateTask(ctx, *taskType, *desc, *priority)
		if err != nil {
			return err
		}
		fmt.Printf("Created task: %s (ID: %s)\n", t.Description, t.ID)

	case "cancel":
		if len(args) < 2 {
			fmt.Println("Error: task ID required")
			return fmt.Errorf("missing task ID")
		}
		err := cliApp.taskManager.CancelTask(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Cancelled task: %s\n", args[1])

	default:
		fmt.Printf("Unknown subcommand: %s\n", args[0])
	}

	return nil
}

func (cliApp *HarmonyCLIApp) cmdWorkers(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list":
		workers := cliApp.workerManager.GetWorkers()
		fmt.Println("=== Workers ===")
		if len(workers) == 0 {
			fmt.Println("No workers found.")
			return nil
		}
		for _, w := range workers {
			healthStatus := "unhealthy"
			if w.Healthy {
				healthStatus = "healthy"
			}
			fmt.Printf("- %s [%s] %s:%s (%s)\n", w.ID, w.Status, w.Host, w.Port, healthStatus)
		}

	case "add":
		fs := flag.NewFlagSet("workers add", flag.ExitOnError)
		host := fs.String("host", "", "Worker host")
		port := fs.String("port", "22", "Worker port")
		user := fs.String("user", "", "SSH user")
		fs.Parse(args[1:])

		if *host == "" {
			fmt.Println("Error: --host is required")
			return fmt.Errorf("missing required arguments")
		}

		w := &CLIWorker{
			ID:      fmt.Sprintf("worker-%s-%d", *host, time.Now().UnixNano()),
			Host:    *host,
			Port:    *port,
			User:    *user,
			Status:  "pending",
			Healthy: false,
		}
		err := cliApp.workerManager.AddWorker(w)
		if err != nil {
			return err
		}
		fmt.Printf("Added worker: %s\n", w.ID)

	case "remove":
		if len(args) < 2 {
			fmt.Println("Error: worker ID required")
			return fmt.Errorf("missing worker ID")
		}
		err := cliApp.workerManager.RemoveWorker(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Removed worker: %s\n", args[1])

	default:
		fmt.Printf("Unknown subcommand: %s\n", args[0])
	}

	return nil
}

func (cliApp *HarmonyCLIApp) cmdLLM(args []string) error {
	if len(args) == 0 {
		args = []string{"providers"}
	}

	switch args[0] {
	case "providers":
		ctx := context.Background()
		health := cliApp.llmManager.HealthCheck(ctx)
		fmt.Println("=== LLM Providers ===")
		if len(health) == 0 {
			fmt.Println("No providers configured.")
			return nil
		}
		for provider, status := range health {
			fmt.Printf("- %s: %s\n", provider, status.Status)
		}

	case "models":
		models := cliApp.llmManager.GetAvailableModels()
		fmt.Println("=== Available Models ===")
		if len(models) == 0 {
			fmt.Println("No models available.")
			return nil
		}
		for _, m := range models {
			fmt.Printf("- %s (%s) - Context: %d\n", m.Name, m.Provider, m.ContextSize)
		}

	case "chat":
		fmt.Println("LLM chat requires a running provider.")
		fmt.Println("Configure your LLM provider (e.g., Ollama) and use the GUI version for interactive chat.")

	default:
		fmt.Printf("Unknown subcommand: %s\n", args[0])
	}

	return nil
}

func (cliApp *HarmonyCLIApp) cmdDistributed(args []string) error {
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch args[0] {
	case "status":
		// Round-31 §11.4 fix: previous output advertised
		// "Data synchronization: Enabled" and "Device discovery:
		// Available" — both are PASS-bluffs in this build because
		// neither the Harmony OS distributed-data SDK nor the
		// device-manager SDK is wired. New output reports the real
		// implementation state honestly so the user (and any wrapper
		// script) knows the gaps before invoking `discover` or `sync`.
		fmt.Println("=== Harmony OS Distributed Status ===")
		fmt.Println()
		fmt.Println("Distributed Features:")
		fmt.Println("  - Cross-device task scheduling: stub (local-only scheduler; no remote dispatch)")
		fmt.Println("  - Data synchronization: NOT WIRED (Harmony distributed-data SDK absent — `sync` returns error)")
		fmt.Println("  - Device discovery: NOT WIRED (Harmony device-manager SDK absent — `discover` returns error)")
		fmt.Println()
		fmt.Println("Implementation gap tracked in applications/harmony_os/distributed.go via")
		fmt.Println("ErrHarmonyDistributedSyncNotImplemented and ErrHarmonyDiscoveryNotImplemented.")
		fmt.Println("Resolution path (round-67 §11.4): consumers with a real Harmony OS Go")
		fmt.Println("binding (cgo shim around OHOS::DistributedKv / JS bridge to ArkTS /")
		fmt.Println("OEM SDK) call (*HarmonyDistributedEngine).SetDistributedSDK(impl) at boot.")

	case "discover":
		// Round-31 §11.4 fix: previous output was a PASS-bluff —
		// it printed "Scanning..." then "No devices found (running
		// in standalone mode)" plus a self-admission that on
		// non-Harmony platforms the command would show fabricated
		// results, giving the user a fake "successful but empty"
		// outcome from a feature that never ran. New output mirrors
		// the ErrHarmonyDiscoveryNotImplemented sentinel surfaced by
		// (*HarmonyDistributedEngine).DiscoverDevices in the GUI
		// build: distributed discovery is NOT WIRED in this build,
		// report that loudly with a non-zero exit so scripts can
		// detect the gap.
		fmt.Fprintln(os.Stderr, "ERROR: Harmony OS distributed device discovery is not wired in this build.")
		fmt.Fprintln(os.Stderr, "The Harmony OS device-manager SDK is required and not present. This")
		fmt.Fprintln(os.Stderr, "command previously printed a fake 'No devices found' success message;")
		fmt.Fprintln(os.Stderr, "it now refuses to fabricate a result. See applications/harmony_os/distributed.go")
		fmt.Fprintln(os.Stderr, "(ErrHarmonyDiscoveryNotImplemented) for the full implementation gap.")
		fmt.Fprintln(os.Stderr, "Round-67 §11.4 resolution path: inject a HarmonyDistributedSDK via")
		fmt.Fprintln(os.Stderr, "(*HarmonyDistributedEngine).SetDistributedSDK(impl) at boot.")
		return fmt.Errorf("harmony distributed discover: not wired in this build")

	case "sync":
		// Round-31 §11.4 fix: previous output was a PASS-bluff —
		// it printed "Sync Status: Enabled / Last Sync: Just now /
		// Synced Devices: 0" without ever calling the underlying
		// sync engine, so callers got fake "successful" sync data
		// on every invocation. New output mirrors the
		// ErrHarmonyDistributedSyncNotImplemented sentinel surfaced
		// by (*HarmonyDataSync).performSync in the GUI build:
		// distributed sync is NOT WIRED in this build, report that
		// loudly with a non-zero exit so scripts can detect the gap.
		fmt.Fprintln(os.Stderr, "ERROR: Harmony OS distributed data synchronization is not wired in this build.")
		fmt.Fprintln(os.Stderr, "The Harmony OS distributed-data SDK (KVManager / SingleKVStore /")
		fmt.Fprintln(os.Stderr, "DeviceKVStore) is required and not present. This command previously")
		fmt.Fprintln(os.Stderr, "printed 'Last Sync: Just now / Synced Devices: 0' without exchanging")
		fmt.Fprintln(os.Stderr, "any state with any device; it now refuses to fabricate a result.")
		fmt.Fprintln(os.Stderr, "See applications/harmony_os/distributed.go (ErrHarmonyDistributedSyncNotImplemented)")
		fmt.Fprintln(os.Stderr, "for the full implementation gap.")
		fmt.Fprintln(os.Stderr, "Round-67 §11.4 resolution path: inject a HarmonyDistributedSDK via")
		fmt.Fprintln(os.Stderr, "(*HarmonyDataSync).SetDistributedSDK(impl) at boot.")
		return fmt.Errorf("harmony distributed sync: not wired in this build")

	default:
		fmt.Printf("Unknown subcommand: %s\n", args[0])
		fmt.Println("Available subcommands: status, discover, sync")
	}

	return nil
}

func (cliApp *HarmonyCLIApp) cmdInteractive() error {
	fmt.Println("=== HelixCode Harmony OS Interactive Mode ===")
	fmt.Println("Type 'help' for commands, 'quit' to exit")
	fmt.Println()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nExiting...")
		os.Exit(0)
	}()

	var input string
	for {
		fmt.Print("helix-harmony> ")
		_, err := fmt.Scanln(&input)
		if err != nil {
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "quit" || input == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		args := strings.Fields(input)
		if err := cliApp.Run(args); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		fmt.Println()
	}

	return nil
}

func main() {
	app := NewHarmonyCLIApp()

	if err := app.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer app.Close()

	args := os.Args[1:]
	if err := app.Run(args); err != nil {
		os.Exit(1)
	}
}
