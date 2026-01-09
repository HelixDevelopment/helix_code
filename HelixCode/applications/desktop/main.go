//go:build !nogui

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

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

// DesktopTaskManager wraps task.TaskManager for UI operations
type DesktopTaskManager struct {
	inner *task.TaskManager
	tasks []UITask // In-memory task list for UI
	mu    sync.RWMutex
}

// NewDesktopTaskManager creates a new desktop task manager wrapper
func NewDesktopTaskManager(tm *task.TaskManager) *DesktopTaskManager {
	return &DesktopTaskManager{
		inner: tm,
		tasks: make([]UITask, 0),
	}
}

// GetAllTasks returns all tasks for UI display
func (dtm *DesktopTaskManager) GetAllTasks() []UITask {
	dtm.mu.RLock()
	defer dtm.mu.RUnlock()
	return dtm.tasks
}

// GetStats returns task statistics
func (dtm *DesktopTaskManager) GetStats() TaskStats {
	dtm.mu.RLock()
	defer dtm.mu.RUnlock()

	stats := TaskStats{
		TotalTasks: len(dtm.tasks),
	}

	for _, t := range dtm.tasks {
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

// CreateTask creates a new task (simplified for UI)
func (dtm *DesktopTaskManager) CreateTask(ctx context.Context, taskType, description, priority string) (*UITask, error) {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()

	newTask := UITask{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Type:        taskType,
		Description: description,
		Status:      "pending",
		Priority:    priority,
	}

	dtm.tasks = append(dtm.tasks, newTask)
	return &newTask, nil
}

// CancelTask cancels a task by ID
func (dtm *DesktopTaskManager) CancelTask(ctx context.Context, taskID string) error {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()

	for i, t := range dtm.tasks {
		if t.ID == taskID {
			dtm.tasks = append(dtm.tasks[:i], dtm.tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("task not found: %s", taskID)
}

// DesktopWorkerManager wraps worker.WorkerManager for UI operations
type DesktopWorkerManager struct {
	inner   *worker.WorkerManager
	workers []UIWorker // In-memory worker list for UI
	mu      sync.RWMutex
}

// NewDesktopWorkerManager creates a new desktop worker manager wrapper
func NewDesktopWorkerManager(wm *worker.WorkerManager) *DesktopWorkerManager {
	return &DesktopWorkerManager{
		inner:   wm,
		workers: make([]UIWorker, 0),
	}
}

// GetWorkers returns all workers for UI display
func (dwm *DesktopWorkerManager) GetWorkers() []UIWorker {
	dwm.mu.RLock()
	defer dwm.mu.RUnlock()
	return dwm.workers
}

// AddWorker adds a new worker (simplified for UI)
func (dwm *DesktopWorkerManager) AddWorker(w *UIWorker) error {
	dwm.mu.Lock()
	defer dwm.mu.Unlock()

	dwm.workers = append(dwm.workers, *w)
	return nil
}

// RemoveWorker removes a worker by ID
func (dwm *DesktopWorkerManager) RemoveWorker(workerID string) error {
	dwm.mu.Lock()
	defer dwm.mu.Unlock()

	for i, w := range dwm.workers {
		if w.ID == workerID {
			dwm.workers = append(dwm.workers[:i], dwm.workers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("worker not found: %s", workerID)
}

// DesktopApp represents the desktop application
type DesktopApp struct {
	fyneApp            fyne.App
	mainWindow         fyne.Window
	config             *config.Config
	db                 *database.Database
	taskManager        *DesktopTaskManager
	workerManager      *DesktopWorkerManager
	projectManager     *project.Manager
	sessionManager     *session.Manager
	llmManager         *llm.ModelManager
	notificationEngine *notification.NotificationEngine
	server             *server.Server
	themeManager       *ThemeManager

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

	// Update ticker for real-time data
	updateTicker *time.Ticker
	stopUpdate   chan struct{}
}

// NewDesktopApp creates a new desktop application
func NewDesktopApp() *DesktopApp {
	fyneApp := app.New()
	fyneApp.Settings().SetTheme(&CustomTheme{})

	return &DesktopApp{
		fyneApp:      fyneApp,
		projects:     make([]*project.Project, 0),
		sessions:     make([]*session.Session, 0),
		llmProviders: make([]string, 0),
		stopUpdate:   make(chan struct{}),
	}
}

// Initialize sets up the desktop application
func (da *DesktopApp) Initialize() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}
	da.config = cfg

	// Initialize database (optional - continue without it if not available)
	db, err := database.New(cfg.Database)
	if err != nil {
		log.Printf("Warning: Database not available: %v (continuing without persistence)", err)
	}
	da.db = db

	// Initialize Redis (optional - continue without it if not available)
	rds, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		log.Printf("Warning: Redis not available: %v (continuing without caching)", err)
	}

	// Initialize components
	innerTaskManager := task.NewTaskManager(db, rds)
	da.taskManager = NewDesktopTaskManager(innerTaskManager)

	// Initialize worker manager with in-memory repository for standalone UI
	workerRepo := worker.NewInMemoryWorkerRepository()
	innerWorkerManager := worker.NewWorkerManager(workerRepo, 30*time.Second)
	da.workerManager = NewDesktopWorkerManager(innerWorkerManager)

	// Initialize project manager
	da.projectManager = project.NewManager()

	// Initialize session manager
	da.sessionManager = session.NewManager()

	// Initialize LLM manager
	da.llmManager = llm.NewModelManager()

	// Initialize notification engine
	da.notificationEngine = notification.NewNotificationEngine()

	// Initialize server for API calls
	da.server = server.New(cfg, db, rds)

	// Initialize theme manager
	da.themeManager = NewThemeManager()

	// Setup UI
	da.setupUI()

	// Start background data updates
	da.startDataUpdates()

	return nil
}

// startDataUpdates starts periodic background data refresh
func (da *DesktopApp) startDataUpdates() {
	da.updateTicker = time.NewTicker(5 * time.Second)
	go func() {
		// Initial data load
		da.refreshData()

		for {
			select {
			case <-da.updateTicker.C:
				da.refreshData()
			case <-da.stopUpdate:
				da.updateTicker.Stop()
				return
			}
		}
	}()
}

// refreshData updates cached data from managers
func (da *DesktopApp) refreshData() {
	da.dataMu.Lock()
	defer da.dataMu.Unlock()

	ctx := context.Background()

	// Refresh projects
	if da.projectManager != nil {
		projects, err := da.projectManager.ListProjects(ctx, "")
		if err == nil {
			da.projects = projects
		}
	}

	// Refresh sessions
	if da.sessionManager != nil {
		da.sessions = da.sessionManager.GetAll()
	}

	// Refresh LLM providers
	if da.llmManager != nil {
		models := da.llmManager.GetAvailableModels()
		providers := make(map[string]bool)
		for _, model := range models {
			providers[string(model.Provider)] = true
		}
		da.llmProviders = make([]string, 0, len(providers))
		for provider := range providers {
			da.llmProviders = append(da.llmProviders, provider)
		}
	}
}

// setupUI initializes the user interface
func (da *DesktopApp) setupUI() {
	// Create main window
	da.mainWindow = da.fyneApp.NewWindow("HelixCode - Distributed AI Development Platform")
	da.mainWindow.SetMaster()
	da.mainWindow.Resize(fyne.NewSize(1200, 800))

	// Create tabs
	da.tabs = container.NewAppTabs(
		container.NewTabItem("Dashboard", da.createDashboardTab()),
		container.NewTabItem("Tasks", da.createTasksTab()),
		container.NewTabItem("Workers", da.createWorkersTab()),
		container.NewTabItem("Projects", da.createProjectsTab()),
		container.NewTabItem("Sessions", da.createSessionsTab()),
		container.NewTabItem("LLM", da.createLLMTab()),
		container.NewTabItem("Settings", da.createSettingsTab()),
	)

	// Create status bar
	da.statusBar = widget.NewLabel("Ready | User: Not logged in | Session: None")
	da.statusBar.Alignment = fyne.TextAlignCenter

	// Create main layout
	mainContent := container.NewBorder(nil, da.statusBar, nil, nil, da.tabs)

	da.mainWindow.SetContent(mainContent)
}

// createDashboardTab creates the dashboard tab
func (da *DesktopApp) createDashboardTab() fyne.CanvasObject {
	// Header with integrated logo
	header := widget.NewLabel("HelixCode - Distributed AI Development Platform")
	header.Alignment = fyne.TextAlignCenter
	header.TextStyle = fyne.TextStyle{Bold: true}

	// Stats cards with dynamic data
	workerStatsLabel := widget.NewLabel("Total: 0\nActive: 0\nHealthy: 0")
	taskStatsLabel := widget.NewLabel("Total: 0\nCompleted: 0\nRunning: 0")
	systemStatsLabel := widget.NewLabel("Status: Operational\nUptime: 00:00:00")

	workerCard := widget.NewCard("Workers", "", workerStatsLabel)
	taskCard := widget.NewCard("Tasks", "", taskStatsLabel)
	systemCard := widget.NewCard("System", "", systemStatsLabel)

	// Start a goroutine to update stats
	go func() {
		startTime := time.Now()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Update worker stats
			if da.workerManager != nil {
				workers := da.workerManager.GetWorkers()
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
				workerStatsLabel.SetText(fmt.Sprintf("Total: %d\nActive: %d\nHealthy: %d", len(workers), active, healthy))
			}

			// Update task stats
			if da.taskManager != nil {
				stats := da.taskManager.GetStats()
				taskStatsLabel.SetText(fmt.Sprintf("Total: %d\nCompleted: %d\nRunning: %d",
					stats.TotalTasks, stats.CompletedTasks, stats.RunningTasks))
			}

			// Update system stats
			uptime := time.Since(startTime)
			hours := int(uptime.Hours())
			minutes := int(uptime.Minutes()) % 60
			seconds := int(uptime.Seconds()) % 60
			systemStatsLabel.SetText(fmt.Sprintf("Status: Operational\nUptime: %02d:%02d:%02d", hours, minutes, seconds))
		}
	}()

	statsContainer := container.NewGridWithColumns(3, workerCard, taskCard, systemCard)

	// Activity log
	activityLog := widget.NewMultiLineEntry()
	activityLog.SetText("System initialized\nWorker pool started\nTask manager ready\nLLM providers loaded")
	activityLog.Disable()

	activityCard := widget.NewCard("Recent Activity", "", activityLog)

	// Quick actions
	actionsCard := widget.NewCard("Quick Actions", "",
		container.NewVBox(
			widget.NewButton("New Task", func() {
				da.tabs.SelectIndex(1) // Switch to Tasks tab
			}),
			widget.NewButton("Add Worker", func() {
				da.tabs.SelectIndex(2) // Switch to Workers tab
			}),
			widget.NewButton("LLM Chat", func() {
				da.tabs.SelectIndex(5) // Switch to LLM tab
			}),
			widget.NewButton("New Project", func() {
				da.tabs.SelectIndex(3) // Switch to Projects tab
			}),
		),
	)

	bottomContainer := container.NewGridWithColumns(2, activityCard, actionsCard)

	return container.NewVBox(header, statsContainer, bottomContainer)
}

// createTasksTab creates the tasks tab
func (da *DesktopApp) createTasksTab() fyne.CanvasObject {
	// Task list with dynamic data
	taskList := widget.NewList(
		func() int {
			if da.taskManager == nil {
				return 0
			}
			return len(da.taskManager.GetAllTasks())
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if da.taskManager == nil {
				return
			}
			tasks := da.taskManager.GetAllTasks()
			if id < len(tasks) {
				t := tasks[id]
				obj.(*widget.Label).SetText(fmt.Sprintf("[%s] %s - %s", t.Status, t.Type, t.Description))
			}
		},
	)

	taskCard := widget.NewCard("Tasks", "", taskList)

	// Task type selector for new tasks
	taskTypeSelect := widget.NewSelect([]string{"planning", "building", "testing", "refactoring", "debugging"}, nil)
	taskTypeSelect.SetSelected("building")

	// Task description input
	taskDescEntry := widget.NewEntry()
	taskDescEntry.SetPlaceHolder("Task description...")

	// Action buttons
	actions := container.NewVBox(
		widget.NewLabel("New Task:"),
		taskTypeSelect,
		taskDescEntry,
		widget.NewButton("Create Task", func() {
			if da.taskManager != nil && taskDescEntry.Text != "" {
				ctx := context.Background()
				_, err := da.taskManager.CreateTask(ctx, taskTypeSelect.Selected, taskDescEntry.Text, "normal")
				if err != nil {
					dialog.ShowError(err, da.mainWindow)
				} else {
					taskDescEntry.SetText("")
					taskList.Refresh()
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
func (da *DesktopApp) createWorkersTab() fyne.CanvasObject {
	workerList := widget.NewList(
		func() int {
			if da.workerManager == nil {
				return 0
			}
			return len(da.workerManager.GetWorkers())
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if da.workerManager == nil {
				return
			}
			workers := da.workerManager.GetWorkers()
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
			if da.workerManager != nil && hostEntry.Text != "" {
				// Create worker configuration
				workerConfig := &UIWorker{
					ID:      fmt.Sprintf("worker-%s-%d", hostEntry.Text, time.Now().UnixNano()),
					Host:    hostEntry.Text,
					Port:    portEntry.Text,
					User:    userEntry.Text,
					Status:  "pending",
					Healthy: false,
				}
				err := da.workerManager.AddWorker(workerConfig)
				if err != nil {
					dialog.ShowError(err, da.mainWindow)
				} else {
					hostEntry.SetText("")
					userEntry.SetText("")
					workerList.Refresh()
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
func (da *DesktopApp) createProjectsTab() fyne.CanvasObject {
	// Project list with dynamic data
	da.projectList = widget.NewList(
		func() int {
			da.dataMu.RLock()
			defer da.dataMu.RUnlock()
			return len(da.projects)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Template"),
				widget.NewLabel(""),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			da.dataMu.RLock()
			defer da.dataMu.RUnlock()
			if id < len(da.projects) {
				p := da.projects[id]
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

	da.projectList.OnSelected = func(id widget.ListItemID) {
		da.dataMu.RLock()
		defer da.dataMu.RUnlock()
		if id < len(da.projects) {
			p := da.projects[id]
			details := fmt.Sprintf("Name: %s\nType: %s\nPath: %s\nDescription: %s\nCreated: %s\nBuild Command: %s\nTest Command: %s",
				p.Name, p.Type, p.Path, p.Description,
				p.CreatedAt.Format(time.RFC822),
				p.Metadata.BuildCommand, p.Metadata.TestCommand)
			projectDetailsLabel.SetText(details)
		}
	}

	projectListCard := widget.NewCard("Projects", "", da.projectList)
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
			if da.projectManager != nil && nameEntry.Text != "" && pathEntry.Text != "" {
				ctx := context.Background()
				_, err := da.projectManager.CreateProject(ctx, nameEntry.Text, descEntry.Text, pathEntry.Text, typeSelect.Selected)
				if err != nil {
					dialog.ShowError(err, da.mainWindow)
				} else {
					nameEntry.SetText("")
					descEntry.SetText("")
					pathEntry.SetText("")
					da.refreshData()
					da.projectList.Refresh()
					dialog.ShowInformation("Success", "Project created successfully", da.mainWindow)
				}
			}
		}),
		widget.NewSeparator(),
		widget.NewButton("Set as Active", func() {
			if da.projectList.Length() > 0 {
				da.dataMu.RLock()
				selectedID := -1
				// Get currently selected
				da.dataMu.RUnlock()

				if selectedID >= 0 && selectedID < len(da.projects) {
					ctx := context.Background()
					p := da.projects[selectedID]
					err := da.projectManager.SetActiveProject(ctx, p.ID)
					if err != nil {
						dialog.ShowError(err, da.mainWindow)
					} else {
						da.refreshData()
						da.projectList.Refresh()
					}
				}
			}
		}),
		widget.NewButton("Delete Project", func() {
			dialog.ShowConfirm("Confirm Delete", "Are you sure you want to delete this project?", func(confirmed bool) {
				if confirmed {
					// Delete selected project
					da.dataMu.RLock()
					// Implementation would need to track selected index
					da.dataMu.RUnlock()
				}
			}, da.mainWindow)
		}),
		widget.NewButton("Refresh", func() {
			da.refreshData()
			da.projectList.Refresh()
		}),
	)

	leftPanel := container.NewVSplit(projectListCard, projectDetailsCard)
	leftPanel.SetOffset(0.6)

	return container.NewBorder(nil, nil, nil, createForm, leftPanel)
}

// createSessionsTab creates the sessions tab
func (da *DesktopApp) createSessionsTab() fyne.CanvasObject {
	// Session list with dynamic data
	da.sessionList = widget.NewList(
		func() int {
			da.dataMu.RLock()
			defer da.dataMu.RUnlock()
			return len(da.sessions)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Template"),
				widget.NewLabel(""),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			da.dataMu.RLock()
			defer da.dataMu.RUnlock()
			if id < len(da.sessions) {
				s := da.sessions[id]
				hbox := obj.(*fyne.Container)
				hbox.Objects[0].(*widget.Label).SetText(s.Name)
				hbox.Objects[1].(*widget.Label).SetText(fmt.Sprintf("[%s] %s", s.Status, s.Mode))
			}
		},
	)

	// Session details panel
	sessionDetailsLabel := widget.NewLabel("Select a session to view details")
	sessionDetailsLabel.Wrapping = fyne.TextWrapWord

	da.sessionList.OnSelected = func(id widget.ListItemID) {
		da.dataMu.RLock()
		defer da.dataMu.RUnlock()
		if id < len(da.sessions) {
			s := da.sessions[id]
			durationStr := s.Duration.String()
			details := fmt.Sprintf("Name: %s\nMode: %s\nStatus: %s\nProject ID: %s\nDescription: %s\nCreated: %s\nDuration: %s",
				s.Name, s.Mode, s.Status, s.ProjectID, s.Description,
				s.CreatedAt.Format(time.RFC822), durationStr)
			sessionDetailsLabel.SetText(details)
		}
	}

	sessionListCard := widget.NewCard("Sessions", "", da.sessionList)
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

	// Session control buttons (for selected session)
	selectedSessionID := ""
	da.sessionList.OnSelected = func(id widget.ListItemID) {
		da.dataMu.RLock()
		defer da.dataMu.RUnlock()
		if id < len(da.sessions) {
			s := da.sessions[id]
			selectedSessionID = s.ID
			durationStr := s.Duration.String()
			details := fmt.Sprintf("Name: %s\nMode: %s\nStatus: %s\nProject ID: %s\nDescription: %s\nCreated: %s\nDuration: %s",
				s.Name, s.Mode, s.Status, s.ProjectID, s.Description,
				s.CreatedAt.Format(time.RFC822), durationStr)
			sessionDetailsLabel.SetText(details)
		}
	}

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
			if da.sessionManager != nil && nameEntry.Text != "" && projectIDEntry.Text != "" {
				mode := session.Mode(modeSelect.Selected)
				_, err := da.sessionManager.Create(projectIDEntry.Text, nameEntry.Text, descEntry.Text, mode)
				if err != nil {
					dialog.ShowError(err, da.mainWindow)
				} else {
					nameEntry.SetText("")
					descEntry.SetText("")
					projectIDEntry.SetText("")
					da.refreshData()
					da.sessionList.Refresh()
					dialog.ShowInformation("Success", "Session created successfully", da.mainWindow)
				}
			}
		}),
		widget.NewSeparator(),
		widget.NewLabel("Session Controls:"),
		widget.NewButton("Start Session", func() {
			if da.sessionManager != nil && selectedSessionID != "" {
				err := da.sessionManager.Start(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, da.mainWindow)
				} else {
					da.refreshData()
					da.sessionList.Refresh()
				}
			}
		}),
		widget.NewButton("Pause Session", func() {
			if da.sessionManager != nil && selectedSessionID != "" {
				err := da.sessionManager.Pause(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, da.mainWindow)
				} else {
					da.refreshData()
					da.sessionList.Refresh()
				}
			}
		}),
		widget.NewButton("Resume Session", func() {
			if da.sessionManager != nil && selectedSessionID != "" {
				err := da.sessionManager.Resume(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, da.mainWindow)
				} else {
					da.refreshData()
					da.sessionList.Refresh()
				}
			}
		}),
		widget.NewButton("Complete Session", func() {
			if da.sessionManager != nil && selectedSessionID != "" {
				err := da.sessionManager.Complete(selectedSessionID)
				if err != nil {
					dialog.ShowError(err, da.mainWindow)
				} else {
					da.refreshData()
					da.sessionList.Refresh()
				}
			}
		}),
		widget.NewSeparator(),
		widget.NewButton("Refresh", func() {
			da.refreshData()
			da.sessionList.Refresh()
		}),
	)

	leftPanel := container.NewVSplit(sessionListCard, sessionDetailsCard)
	leftPanel.SetOffset(0.6)

	return container.NewBorder(nil, nil, nil, actions, leftPanel)
}

// createLLMTab creates the LLM tab
func (da *DesktopApp) createLLMTab() fyne.CanvasObject {
	// Available models list
	modelList := widget.NewList(
		func() int {
			if da.llmManager == nil {
				return 0
			}
			return len(da.llmManager.GetAvailableModels())
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Model"),
				widget.NewLabel("Provider"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			models := da.llmManager.GetAvailableModels()
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
		models := da.llmManager.GetAvailableModels()
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
	da.chatHistory = widget.NewMultiLineEntry()
	da.chatHistory.SetPlaceHolder("Chat history will appear here...")
	da.chatHistory.Disable()
	da.chatHistory.Wrapping = fyne.TextWrapWord

	da.chatInput = widget.NewMultiLineEntry()
	da.chatInput.SetPlaceHolder("Type your message here...")
	da.chatInput.SetMinRowsVisible(3)

	// Provider/model selection for chat
	da.llmProviderSel = widget.NewSelect([]string{"ollama", "openai", "anthropic", "gemini", "local"}, nil)
	da.llmProviderSel.SetSelected("ollama")

	modelNameEntry := widget.NewEntry()
	modelNameEntry.SetPlaceHolder("Model name (e.g., llama2)")
	modelNameEntry.SetText("llama2")

	sendButton := widget.NewButton("Send Message", func() {
		if da.chatInput.Text == "" {
			return
		}

		// Add user message to history
		currentHistory := da.chatHistory.Text
		userMessage := da.chatInput.Text
		userMsg := fmt.Sprintf("\n[User]: %s\n", userMessage)
		da.chatHistory.SetText(currentHistory + userMsg)

		// Clear input immediately
		da.chatInput.SetText("")

		// Make LLM call in goroutine to not block UI
		go func(msg string) {
			var responseMsg string
			providerName := da.llmProviderSel.Selected
			modelName := modelNameEntry.Text

			if da.llmManager != nil {
				// Get provider from manager
				provider := da.llmManager.GetProvider(providerName)
				if provider != nil {
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
					responseMsg = fmt.Sprintf("[AI (%s/%s)]: Provider '%s' not available. Please configure it in Settings.\n",
						providerName, modelName, providerName)
				}
			} else {
				// No LLM manager configured - show informative message
				responseMsg = fmt.Sprintf("[AI (%s/%s)]: LLM service not initialized. Please restart the application or check configuration.\n",
					providerName, modelName)
			}

			// Update UI on main thread
			da.chatHistory.SetText(da.chatHistory.Text + responseMsg)
		}(userMessage)
	})

	clearButton := widget.NewButton("Clear Chat", func() {
		da.chatHistory.SetText("")
	})

	chatControls := container.NewVBox(
		widget.NewLabel("Chat Settings:"),
		widget.NewLabel("Provider:"),
		da.llmProviderSel,
		widget.NewLabel("Model:"),
		modelNameEntry,
		widget.NewSeparator(),
		sendButton,
		clearButton,
	)

	chatPanel := container.NewBorder(
		widget.NewLabel("Chat with AI"),
		container.NewBorder(nil, nil, nil, chatControls, da.chatInput),
		nil, nil,
		da.chatHistory,
	)

	chatCard := widget.NewCard("LLM Chat", "", chatPanel)

	// Provider health status
	healthLabel := widget.NewLabel("Provider Health:\nChecking...")

	// Start health check goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		checkHealth := func() {
			if da.llmManager == nil {
				healthLabel.SetText("Provider Health:\nNo LLM manager available")
				return
			}
			ctx := context.Background()
			health := da.llmManager.HealthCheck(ctx)
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
func (da *DesktopApp) createSettingsTab() fyne.CanvasObject {
	// Theme selection
	themeInfoLabel := widget.NewLabel("")
	updateThemeInfo := func() {
		currentTheme := da.themeManager.GetCurrentTheme()
		themeInfo := fmt.Sprintf("Name: %s\nDark: %t\nPrimary: %s\nSecondary: %s\nAccent: %s",
			currentTheme.Name, currentTheme.IsDark,
			currentTheme.Primary, currentTheme.Secondary, currentTheme.Accent)
		themeInfoLabel.SetText(themeInfo)
	}

	themeSelect := widget.NewSelect(da.themeManager.GetAvailableThemes(), func(selected string) {
		da.themeManager.SetTheme(selected)
		updateThemeInfo()
	})
	themeSelect.SetSelected(da.themeManager.GetCurrentTheme().Name)

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
				dialog.ShowInformation("Connection Test", "Server connection test would be performed here.", da.mainWindow)
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

	// About section
	aboutLabel := widget.NewLabel("HelixCode Desktop Application\nVersion: 1.0.0\nDistributed AI Development Platform")
	aboutLabel.Alignment = fyne.TextAlignCenter
	aboutCard := widget.NewCard("About", "", aboutLabel)

	// Layout in scrollable container
	settingsContent := container.NewVBox(
		themeCard,
		infoCard,
		serverCard,
		dbCard,
		llmCard,
		aboutCard,
	)

	return container.NewScroll(settingsContent)
}

// Run starts the desktop application
func (da *DesktopApp) Run() {
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start signal handler in goroutine
	go func() {
		<-sigChan
		da.fyneApp.Quit()
	}()

	// Show window and run (blocks until window closes)
	da.mainWindow.ShowAndRun()
}

// Close cleans up resources
func (da *DesktopApp) Close() error {
	// Stop background updates
	if da.stopUpdate != nil {
		close(da.stopUpdate)
	}

	// Close database connection
	if da.db != nil {
		da.db.Close()
	}
	return nil
}

func main() {
	desktopApp := NewDesktopApp()

	if err := desktopApp.Initialize(); err != nil {
		log.Fatalf("Failed to initialize desktop app: %v", err)
	}
	defer desktopApp.Close()

	desktopApp.Run()
}
