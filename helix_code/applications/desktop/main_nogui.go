//go:build nogui

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
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

// CLIApp represents the CLI application (nogui mode)
type CLIApp struct {
	config         *config.Config
	db             *database.Database
	taskManager    *CLITaskManager
	workerManager  *CLIWorkerManager
	projectManager *project.Manager
	sessionManager *session.Manager
	llmManager     *llm.ModelManager
}

// NewCLIApp creates a new CLI application
func NewCLIApp() *CLIApp {
	return &CLIApp{}
}

// Initialize sets up the CLI application
func (cliApp *CLIApp) Initialize() error {
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

	return nil
}

// Close cleans up resources
func (cliApp *CLIApp) Close() error {
	if cliApp.db != nil {
		cliApp.db.Close()
	}
	return nil
}

// Run executes the CLI command
func (cliApp *CLIApp) Run(args []string) error {
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
	case "interactive":
		return cliApp.cmdInteractive()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		cliApp.printHelp()
		return fmt.Errorf("unknown command: %s", command)
	}

	return nil
}

func (cliApp *CLIApp) printHelp() {
	fmt.Println(`HelixCode Desktop CLI (nogui mode)

Usage: helix-desktop [command] [arguments]

Commands:
  help          Show this help message
  status        Show system status
  projects      Manage projects (list, create, delete, set-active)
  sessions      Manage sessions (list, create, start, pause, complete)
  tasks         Manage tasks (list, create, cancel)
  workers       Manage workers (list, add, remove)
  llm           LLM operations (providers, models, chat)
  interactive   Start interactive mode

Examples:
  helix-desktop status
  helix-desktop projects list
  helix-desktop projects create --name "MyProject" --path "/path/to/project"
  helix-desktop sessions create --project "proj_123" --name "Dev Session"
  helix-desktop tasks create --type building --desc "Build the project"
  helix-desktop llm providers
  helix-desktop interactive

Build with GUI disabled using: go build -tags nogui`)
}

func (cliApp *CLIApp) cmdStatus() error {
	fmt.Println("=== HelixCode System Status ===")
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

func (cliApp *CLIApp) cmdProjects(args []string) error {
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
		fmt.Println("=== Projects ===")
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

		project, err := cliApp.projectManager.CreateProject(ctx, *name, *desc, *path, *ptype)
		if err != nil {
			return err
		}
		fmt.Printf("Created project: %s (ID: %s)\n", project.Name, project.ID)

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

func (cliApp *CLIApp) cmdSessions(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list":
		sessions := cliApp.sessionManager.GetAll()
		fmt.Println("=== Sessions ===")
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

func (cliApp *CLIApp) cmdTasks(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}

	ctx := context.Background()

	switch args[0] {
	case "list":
		tasks := cliApp.taskManager.GetAllTasks()
		fmt.Println("=== Tasks ===")
		if len(tasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}
		for _, t := range tasks {
			fmt.Printf("- [%s] %s: %s\n", t.Status, t.Type, t.Description)
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

func (cliApp *CLIApp) cmdWorkers(args []string) error {
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

func (cliApp *CLIApp) cmdLLM(args []string) error {
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

func (cliApp *CLIApp) cmdInteractive() error {
	fmt.Println("=== HelixCode Interactive Mode ===")
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
		fmt.Print("helix> ")
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
	app := NewCLIApp()

	if err := app.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer app.Close()

	args := os.Args[1:]
	if err := app.Run(args); err != nil {
		os.Exit(1)
	}
}
