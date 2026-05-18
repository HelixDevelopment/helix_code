package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/helixqa"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/project"
	"dev.helix.code/internal/redis"
	"dev.helix.code/internal/server"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/task"
	"dev.helix.code/internal/worker"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/rivo/tview"
)

// TerminalUI represents the main terminal user interface
type TerminalUI struct {
	app                *tview.Application
	config             *config.Config
	helixConfig        *config.HelixConfig
	db                 *database.Database
	taskManager        *task.TaskManager
	workerManager      *worker.WorkerManager
	workerRepo         *worker.InMemoryWorkerRepository
	llmProvider        llm.Provider
	llmManager         *llm.ModelManager
	notificationEngine *notification.NotificationEngine
	server             *server.Server
	themeManager       *ThemeManager
	projectManager     *project.Manager
	sessionManager     *session.Manager
	qaEngine           *helixqa.Engine

	// UI Components
	pages     *tview.Pages
	mainFlex  *tview.Flex
	sidebar   *tview.List
	content   *tview.Pages
	statusBar *tview.TextView

	// LLM Chat state
	chatHistory []llm.Message
	chatInput   *tview.InputField
	chatOutput  *tview.TextView

	// Current state
	currentUser    string
	currentSession string
}

// NewTerminalUI creates a new Terminal UI instance
func NewTerminalUI() *TerminalUI {
	return &TerminalUI{
		app: tview.NewApplication(),
	}
}

// Initialize sets up the Terminal UI with all dependencies
func (tui *TerminalUI) Initialize() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}
	tui.config = cfg

	// Load Helix configuration
	helixCfg, err := config.LoadHelixConfig()
	if err != nil {
		// If helix config doesn't exist, use default
		helixCfg = &config.Config{} // Use zero value as default
	}
	tui.helixConfig = helixCfg

	// Initialize database
	db, err := database.New(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	tui.db = db

	// Initialize Redis
	rds, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		return fmt.Errorf("failed to initialize Redis: %v", err)
	}

	// Initialize components
	tui.taskManager = task.NewTaskManager(db, rds)

	// Initialize worker manager with in-memory repository for standalone UI
	workerRepo := worker.NewInMemoryWorkerRepository()
	tui.workerRepo = workerRepo
	tui.workerManager = worker.NewWorkerManager(workerRepo, 30*time.Second)

	tui.notificationEngine = notification.NewNotificationEngine()

	// Initialize project manager
	tui.projectManager = project.NewManager()

	// Initialize session manager
	tui.sessionManager = session.NewManager()

	// Initialize QA engine
	qaEngine, err := helixqa.NewEngine(cfg)
	if err == nil {
		tui.qaEngine = qaEngine
	}

	// Initialize LLM manager
	tui.llmManager = llm.NewModelManager()
	tui.chatHistory = make([]llm.Message, 0)

	// Initialize server for API calls
	tui.server = server.New(cfg, db, rds)

	// Initialize theme manager
	tui.themeManager = NewThemeManager()

	// Setup UI components
	tui.setupUI()

	return nil
}

// setupUI initializes the user interface components
func (tui *TerminalUI) setupUI() {
	// Create main pages container
	tui.pages = tview.NewPages()

	// Create main layout
	tui.mainFlex = tview.NewFlex().SetDirection(tview.FlexColumn)

	// Create sidebar navigation
	tui.sidebar = tview.NewList().
		AddItem("Dashboard", "System overview and status", 'd', tui.showDashboard).
		AddItem("Tasks", "Manage distributed tasks", 't', tui.showTasks).
		AddItem("Workers", "Monitor worker nodes", 'w', tui.showWorkers).
		AddItem("Projects", "Project management", 'p', tui.showProjects).
		AddItem("Sessions", "Active development sessions", 's', tui.showSessions).
		AddItem("LLM", "AI model interaction", 'l', tui.showLLM).
		AddItem("QA", "Quality assurance dashboard", 'q', tui.showQA).
		AddItem("Settings", "Configuration and preferences", 'c', tui.showSettings).
		ShowSecondaryText(false)

	tui.sidebar.SetBorder(true).SetTitle("HelixCode v1.0.0")

	// Create content area
	tui.content = tview.NewPages()

	// Create status bar
	tui.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[green]Ready[white] | User: [yellow]Not logged in[white] | Session: [yellow]None").
		SetTextAlign(tview.AlignCenter)
	tui.statusBar.SetBorder(true).SetTitle("Status")

	// Setup main layout
	tui.mainFlex.
		AddItem(tui.sidebar, 25, 0, true).
		AddItem(tui.content, 0, 1, false)

	// Create main flex with status bar
	mainContainer := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tui.mainFlex, 0, 1, false).
		AddItem(tui.statusBar, 3, 0, false)

	// Add main page
	tui.pages.AddPage("main", mainContainer, true, true)

	// Show initial dashboard
	tui.showDashboard()
}

// loadASCIIArt loads the ASCII art logo from file
func (tui *TerminalUI) loadASCIIArt() string {
	// Try to load from assets
	asciiPath := "assets/images/logo-ascii.txt"
	if data, err := ioutil.ReadFile(asciiPath); err == nil {
		return string(data)
	}

	// Fallback to default ASCII art
	return `
                 :=+**##**+-:
              :+##%#######%%#+-
            :*############**+*#*-
          .+#############*++***##*.
         :#############*++****#####:
        =#############+++****#######:
       =#*##########*+++****#########.
      =#**#########*++*****#########%*
     -#****#######*+++**#############%-
    .#*******#####++******#############
    +#**********#*+*+:.    :+#########%=
   :#**********#**+:         :*%#######*
   **************+             *%######%:
  :#************+    :=++=-.    #######%=
  +*************.  :***####*-   :%#####%+
  ************#-  -#***+--+##-   *######*
 :#************  -#***.    :**   -%######
 =***********#=  ****.  :-. -#-  :%######
 +#**********#: -#*#-  +##*. #=  .#######
 =++++++++++++. +**#. :#**#- *+  .######*
                +***. -#*##. #-  :%####%+
                +**#. .#**. =#   =%####%-
                +#*#-  =##-+#:   #######.
                =#*#*   -+*=.   =%####%*
                :#**#=         -######%:
                 *####=       =######%+
                 -#*####=---+**+****#*
                  +###########**+++**.
                   *##############%*.
                    +###########%#=
                     -*#%%##%%%#+.
                       :-++*+=:
`
}

// showDashboard displays the main dashboard
func (tui *TerminalUI) showDashboard() {
	dashboard := tview.NewFlex().SetDirection(tview.FlexRow)

	// Header with ASCII logo
	asciiArt := tui.loadASCIIArt()
	header := tview.NewTextView().
		SetText(asciiArt).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	header.SetBorder(true).SetTitle("HelixCode")

	// Stats grid
	statsGrid := tview.NewGrid().SetRows(3, 3, 3).SetColumns(30, 30, 30).SetBorders(true)

	// Worker stats
	workerStats := tui.createWorkerStatsView()
	statsGrid.AddItem(workerStats, 0, 0, 1, 1, 0, 0, false)

	// Task stats
	taskStats := tui.createTaskStatsView()
	statsGrid.AddItem(taskStats, 0, 1, 1, 1, 0, 0, false)

	// System stats
	systemStats := tui.createSystemStatsView()
	statsGrid.AddItem(systemStats, 0, 2, 1, 1, 0, 0, false)

	// Recent activity
	activityView := tui.createActivityView()
	statsGrid.AddItem(activityView, 1, 0, 1, 3, 0, 0, false)

	// Quick actions
	quickActions := tui.createQuickActionsView()
	statsGrid.AddItem(quickActions, 2, 0, 1, 3, 0, 0, false)

	dashboard.
		AddItem(header, 15, 0, false).
		AddItem(statsGrid, 0, 1, false)

	tui.content.SwitchToPage("dashboard")
	tui.content.AddPage("dashboard", dashboard, true, true)
}

// createWorkerStatsView creates the worker statistics view
func (tui *TerminalUI) createWorkerStatsView() tview.Primitive {
	view := tview.NewTextView()
	view.SetDynamicColors(true)
	view.SetBorder(true)
	view.SetTitle("Workers")

	// Get real worker stats from the worker repository
	ctx := context.Background()
	workers, err := tui.workerRepo.ListWorkers(ctx, "")

	totalWorkers := 0
	activeWorkers := 0
	healthyWorkers := 0

	if err == nil {
		totalWorkers = len(workers)
		for _, w := range workers {
			if w.Status == worker.WorkerStatusActive {
				activeWorkers++
			}
			if w.HealthStatus == worker.WorkerHealthHealthy {
				healthyWorkers++
			}
		}
	}

	content := fmt.Sprintf("[green]Total: %d\n[white]Active: %d\n[yellow]Healthy: %d\n[red]Failed: %d",
		totalWorkers, activeWorkers, healthyWorkers, totalWorkers-healthyWorkers)

	view.SetText(content)
	return view
}

// createTaskStatsView creates the task statistics view
func (tui *TerminalUI) createTaskStatsView() tview.Primitive {
	view := tview.NewTextView()
	view.SetDynamicColors(true)
	view.SetBorder(true)
	view.SetTitle("Tasks")

	// Get real task stats from session manager (tasks are tracked through sessions)
	sessions := tui.sessionManager.GetAll()
	stats := tui.sessionManager.GetStatistics()

	totalTasks := stats.Total
	completedTasks := stats.ByStatus[session.StatusCompleted]
	runningTasks := stats.ByStatus[session.StatusActive]
	failedTasks := stats.ByStatus[session.StatusFailed]
	pausedTasks := stats.ByStatus[session.StatusPaused]

	// Also count by mode for more detailed stats
	planningCount := 0
	buildingCount := 0
	testingCount := 0
	for _, s := range sessions {
		switch s.Mode {
		case session.ModePlanning:
			planningCount++
		case session.ModeBuilding:
			buildingCount++
		case session.ModeTesting:
			testingCount++
		}
	}

	content := fmt.Sprintf("[blue]Total: %d\n[green]Completed: %d\n[yellow]Running: %d\n[gray]Paused: %d\n[red]Failed: %d",
		totalTasks, completedTasks, runningTasks, pausedTasks, failedTasks)
	view.SetText(content)
	return view
}

// createSystemStatsView creates the system statistics view
func (tui *TerminalUI) createSystemStatsView() tview.Primitive {
	view := tview.NewTextView()
	view.SetDynamicColors(true)
	view.SetBorder(true)
	view.SetTitle("System")

	// Get real system stats from various managers
	ctx := context.Background()

	// Check database status
	dbStatus := "[green]Connected"
	if tui.db == nil {
		dbStatus = "[red]Not Connected"
	}

	// Check LLM provider status
	llmStatus := "[yellow]Not Configured"
	if tui.llmProvider != nil {
		if tui.llmProvider.IsAvailable(ctx) {
			llmStatus = "[green]Available"
		} else {
			llmStatus = "[red]Unavailable"
		}
	}

	// Get project count
	projects, _ := tui.projectManager.ListProjects(ctx, "")
	projectCount := len(projects)

	// Get active project
	activeProjectName := "None"
	if activeProject, err := tui.projectManager.GetActiveProject(ctx); err == nil && activeProject != nil {
		activeProjectName = activeProject.Name
	}

	content := fmt.Sprintf("[green]Status: Operational\n[white]Database: %s\n[yellow]LLM: %s\n[blue]Projects: %d\n[cyan]Active: %s",
		dbStatus, llmStatus, projectCount, activeProjectName)
	view.SetText(content)
	return view
}

// createActivityView creates the recent activity view
func (tui *TerminalUI) createActivityView() tview.Primitive {
	view := tview.NewTextView()
	view.SetDynamicColors(true)
	view.SetBorder(true)
	view.SetTitle("Recent Activity")

	var activities []string

	// Add system initialization
	activities = append(activities, "[green]+ System initialized")

	// Add recent sessions
	recentSessions := tui.sessionManager.GetRecent(3)
	for _, s := range recentSessions {
		statusIcon := "+"
		color := "[white]"
		switch s.Status {
		case session.StatusActive:
			statusIcon = ">"
			color = "[green]"
		case session.StatusPaused:
			statusIcon = "||"
			color = "[yellow]"
		case session.StatusCompleted:
			statusIcon = "+"
			color = "[blue]"
		case session.StatusFailed:
			statusIcon = "x"
			color = "[red]"
		}
		activities = append(activities, fmt.Sprintf("%s%s Session '%s' (%s)", color, statusIcon, s.Name, s.Status))
	}

	// Add worker info
	ctx := context.Background()
	workers, _ := tui.workerRepo.ListWorkers(ctx, "")
	if len(workers) > 0 {
		activities = append(activities, fmt.Sprintf("[cyan]+ %d workers registered", len(workers)))
	}

	// Add project info
	projects, _ := tui.projectManager.ListProjects(ctx, "")
	if len(projects) > 0 {
		activities = append(activities, fmt.Sprintf("[blue]+ %d projects loaded", len(projects)))
	}

	// Add LLM status
	models := tui.llmManager.GetAvailableModels()
	if len(models) > 0 {
		activities = append(activities, fmt.Sprintf("[magenta]+ %d LLM models available", len(models)))
	} else {
		activities = append(activities, "[yellow]! No LLM providers configured")
	}

	if len(activities) == 1 {
		activities = append(activities, "[gray]No recent activity")
	}

	view.SetText(strings.Join(activities, "\n"))
	return view
}

// createQuickActionsView creates the quick actions view
func (tui *TerminalUI) createQuickActionsView() *tview.List {
	list := tview.NewList().
		AddItem("New Task", "Create a new distributed task", 'n', func() {
			tui.showNewTaskForm()
		}).
		AddItem("Add Worker", "Register a new worker node", 'a', func() {
			tui.showAddWorkerForm()
		}).
		AddItem("New Project", "Create a new project", 'p', func() {
			tui.showNewProjectForm()
		}).
		AddItem("LLM Chat", "Start AI conversation", 'c', func() {
			tui.showLLM()
		})

	list.SetBorder(true).SetTitle("Quick Actions")
	return list
}

// showTasks displays the task management interface
func (tui *TerminalUI) showTasks() {
	tasksView := tview.NewFlex().SetDirection(tview.FlexRow)

	header := tview.NewTextView()
	header.SetText("[::b]Task Management")
	header.SetTextAlign(tview.AlignCenter)
	header.SetDynamicColors(true)
	header.SetBorder(true)

	// Create task list using components
	components := NewUIComponents(tui)

	// Sample task data - will be replaced with real data
	taskItems := []ListItem{
		{MainText: "Code Generation Task", SecondaryText: "Generate REST API endpoints", Shortcut: '1'},
		{MainText: "Testing Task", SecondaryText: "Run unit tests", Shortcut: '2'},
		{MainText: "Build Task", SecondaryText: "Compile application", Shortcut: '3'},
	}

	taskList := components.CreateList("Tasks", taskItems)

	// Action buttons
	actions := tview.NewFlex().SetDirection(tview.FlexColumn)
	newTaskBtn := tview.NewButton("New Task").SetSelectedFunc(func() {
		tui.showNewTaskForm()
	})
	actions.AddItem(newTaskBtn, 0, 1, false)

	tasksView.
		AddItem(header, 3, 0, false).
		AddItem(taskList, 0, 1, false).
		AddItem(actions, 3, 0, false)

	tui.content.SwitchToPage("tasks")
	tui.content.AddPage("tasks", tasksView, true, true)
}

// showWorkers displays the worker management interface
func (tui *TerminalUI) showWorkers() {
	workersView := tview.NewFlex().SetDirection(tview.FlexRow)

	header := tview.NewTextView().
		SetText("[::b]Worker Management").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	header.SetBorder(true)

	// Create worker table
	workerTable := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)
	workerTable.SetBorder(true).SetTitle("Workers")

	// Set headers
	headers := []string{"ID", "Hostname", "Status", "Health", "CPU %", "Memory %", "Tasks", "Last Heartbeat"}
	for col, h := range headers {
		workerTable.SetCell(0, col, tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
	}

	// Get workers from repository
	ctx := context.Background()
	workers, err := tui.workerRepo.ListWorkers(ctx, "")
	if err != nil {
		log.Printf("Failed to list workers: %v", err)
	}

	if len(workers) == 0 {
		workerTable.SetCell(1, 0, tview.NewTableCell("No workers registered").
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
	} else {
		for row, w := range workers {
			statusColor := "[green]"
			if w.Status == worker.WorkerStatusOffline {
				statusColor = "[red]"
			} else if w.Status == worker.WorkerStatusMaintenance {
				statusColor = "[yellow]"
			}

			healthColor := "[green]"
			if w.HealthStatus == worker.WorkerHealthUnhealthy {
				healthColor = "[red]"
			} else if w.HealthStatus == worker.WorkerHealthDegraded {
				healthColor = "[yellow]"
			}

			workerTable.SetCell(row+1, 0, tview.NewTableCell(w.ID.String()[:8]).SetAlign(tview.AlignLeft))
			workerTable.SetCell(row+1, 1, tview.NewTableCell(w.Hostname).SetAlign(tview.AlignLeft))
			workerTable.SetCell(row+1, 2, tview.NewTableCell(statusColor+string(w.Status)).SetAlign(tview.AlignCenter))
			workerTable.SetCell(row+1, 3, tview.NewTableCell(healthColor+string(w.HealthStatus)).SetAlign(tview.AlignCenter))
			workerTable.SetCell(row+1, 4, tview.NewTableCell(fmt.Sprintf("%.1f%%", w.CPUUsagePercent)).SetAlign(tview.AlignRight))
			workerTable.SetCell(row+1, 5, tview.NewTableCell(fmt.Sprintf("%.1f%%", w.MemoryUsagePercent)).SetAlign(tview.AlignRight))
			workerTable.SetCell(row+1, 6, tview.NewTableCell(fmt.Sprintf("%d/%d", w.CurrentTasksCount, w.MaxConcurrentTasks)).SetAlign(tview.AlignCenter))
			workerTable.SetCell(row+1, 7, tview.NewTableCell(w.LastHeartbeat.Format("15:04:05")).SetAlign(tview.AlignCenter))
		}
	}

	// Worker stats panel
	statsPanel := tui.createWorkerStatsPanel(workers)

	// Action buttons
	actions := tview.NewFlex().SetDirection(tview.FlexColumn)
	addWorkerBtn := tview.NewButton("Add Worker").SetSelectedFunc(func() {
		tui.showAddWorkerForm()
	})
	refreshBtn := tview.NewButton("Refresh").SetSelectedFunc(func() {
		tui.showWorkers()
	})
	actions.AddItem(addWorkerBtn, 0, 1, false)
	actions.AddItem(refreshBtn, 0, 1, false)

	// Main content area
	contentFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	contentFlex.AddItem(workerTable, 0, 3, true)
	contentFlex.AddItem(statsPanel, 30, 0, false)

	workersView.
		AddItem(header, 3, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(actions, 3, 0, false)

	tui.content.SwitchToPage("workers")
	tui.content.AddPage("workers", workersView, true, true)
}

// createWorkerStatsPanel creates a statistics panel for workers
func (tui *TerminalUI) createWorkerStatsPanel(workers []*worker.Worker) *tview.TextView {
	panel := tview.NewTextView().SetDynamicColors(true)
	panel.SetBorder(true).SetTitle("Statistics")

	totalWorkers := len(workers)
	activeWorkers := 0
	healthyWorkers := 0
	totalTasks := 0
	var totalCPU, totalMem float64

	for _, w := range workers {
		if w.Status == worker.WorkerStatusActive {
			activeWorkers++
		}
		if w.HealthStatus == worker.WorkerHealthHealthy {
			healthyWorkers++
		}
		totalTasks += w.CurrentTasksCount
		totalCPU += w.CPUUsagePercent
		totalMem += w.MemoryUsagePercent
	}

	avgCPU := 0.0
	avgMem := 0.0
	if totalWorkers > 0 {
		avgCPU = totalCPU / float64(totalWorkers)
		avgMem = totalMem / float64(totalWorkers)
	}

	content := fmt.Sprintf(`[::b]Worker Summary[white]

[green]Total:[white] %d
[green]Active:[white] %d
[green]Healthy:[white] %d
[red]Offline:[white] %d

[::b]Resource Usage[white]

[yellow]Avg CPU:[white] %.1f%%
[yellow]Avg Memory:[white] %.1f%%
[blue]Active Tasks:[white] %d`,
		totalWorkers, activeWorkers, healthyWorkers, totalWorkers-activeWorkers,
		avgCPU, avgMem, totalTasks)

	panel.SetText(content)
	return panel
}

// showAddWorkerForm displays a form for adding a new worker
func (tui *TerminalUI) showAddWorkerForm() {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle("Add New Worker").SetTitleAlign(tview.AlignLeft)

	var hostname, displayName, sshHost, sshUser string
	var sshPort int = 22
	var maxTasks int = 4

	form.AddInputField("Hostname", "", 30, nil, func(text string) {
		hostname = text
	})
	form.AddInputField("Display Name", "", 30, nil, func(text string) {
		displayName = text
	})
	form.AddInputField("SSH Host", "", 30, nil, func(text string) {
		sshHost = text
	})
	form.AddInputField("SSH Port", "22", 10, tview.InputFieldInteger, func(text string) {
		fmt.Sscanf(text, "%d", &sshPort)
	})
	form.AddInputField("SSH User", "", 20, nil, func(text string) {
		sshUser = text
	})
	form.AddInputField("Max Concurrent Tasks", "4", 10, tview.InputFieldInteger, func(text string) {
		fmt.Sscanf(text, "%d", &maxTasks)
	})

	form.AddButton("Add", func() {
		ctx := context.Background()
		newWorker := &worker.Worker{
			ID:          uuid.New(),
			Hostname:    hostname,
			DisplayName: displayName,
			SSHConfig: map[string]interface{}{
				"host":     sshHost,
				"port":     sshPort,
				"username": sshUser,
			},
			Status:             worker.WorkerStatusActive,
			HealthStatus:       worker.WorkerHealthHealthy,
			MaxConcurrentTasks: maxTasks,
			LastHeartbeat:      time.Now(),
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		if err := tui.workerRepo.CreateWorker(ctx, newWorker); err != nil {
			tui.statusBar.SetText(fmt.Sprintf("[red]Failed to add worker: %v", err))
		} else {
			tui.statusBar.SetText(fmt.Sprintf("[green]Worker added: %s", hostname))
			tui.showWorkers()
		}

		tui.pages.RemovePage("addWorkerForm")
		tui.app.SetFocus(tui.content)
	})

	form.AddButton("Cancel", func() {
		tui.pages.RemovePage("addWorkerForm")
		tui.app.SetFocus(tui.content)
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 22, 1, true).
			AddItem(nil, 0, 1, false), 60, 1, true).
		AddItem(nil, 0, 1, false)

	tui.pages.AddPage("addWorkerForm", modal, true, true)
	tui.app.SetFocus(form)
}

// showProjects displays the project management interface
func (tui *TerminalUI) showProjects() {
	projectsView := tview.NewFlex().SetDirection(tview.FlexRow)

	header := tview.NewTextView().
		SetText("[::b]Project Management").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	header.SetBorder(true)

	// Create project table
	projectTable := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)
	projectTable.SetBorder(true).SetTitle("Projects")

	// Set headers
	headers := []string{"Name", "Type", "Path", "Status", "Created", "Updated"}
	for col, h := range headers {
		projectTable.SetCell(0, col, tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
	}

	// Get projects from manager
	ctx := context.Background()
	projects, err := tui.projectManager.ListProjects(ctx, "")
	if err != nil {
		log.Printf("Failed to list projects: %v", err)
	}

	if len(projects) == 0 {
		projectTable.SetCell(1, 0, tview.NewTableCell("No projects found. Click 'New Project' to create one.").
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
	} else {
		for row, p := range projects {
			statusColor := "[white]"
			statusText := "Inactive"
			if p.Active {
				statusColor = "[green]"
				statusText = "Active"
			}

			projectTable.SetCell(row+1, 0, tview.NewTableCell(p.Name).SetAlign(tview.AlignLeft))
			projectTable.SetCell(row+1, 1, tview.NewTableCell(p.Type).SetAlign(tview.AlignCenter))
			projectTable.SetCell(row+1, 2, tview.NewTableCell(truncatePath(p.Path, 30)).SetAlign(tview.AlignLeft))
			projectTable.SetCell(row+1, 3, tview.NewTableCell(statusColor+statusText).SetAlign(tview.AlignCenter))
			projectTable.SetCell(row+1, 4, tview.NewTableCell(p.CreatedAt.Format("2006-01-02")).SetAlign(tview.AlignCenter))
			projectTable.SetCell(row+1, 5, tview.NewTableCell(p.UpdatedAt.Format("2006-01-02")).SetAlign(tview.AlignCenter))
		}
	}

	// Handle project selection
	projectTable.SetSelectedFunc(func(row, col int) {
		if row > 0 && row <= len(projects) {
			selectedProject := projects[row-1]
			tui.showProjectDetails(selectedProject)
		}
	})

	// Project details panel
	detailsPanel := tview.NewTextView().SetDynamicColors(true)
	detailsPanel.SetBorder(true).SetTitle("Project Details")

	activeProject, _ := tui.projectManager.GetActiveProject(ctx)
	if activeProject != nil {
		detailsPanel.SetText(tui.formatProjectDetails(activeProject))
	} else {
		detailsPanel.SetText("[gray]Select a project to view details")
	}

	// Action buttons
	actions := tview.NewFlex().SetDirection(tview.FlexColumn)
	newProjectBtn := tview.NewButton("New Project").SetSelectedFunc(func() {
		tui.showNewProjectForm()
	})
	setActiveBtn := tview.NewButton("Set Active").SetSelectedFunc(func() {
		row, _ := projectTable.GetSelection()
		if row > 0 && row <= len(projects) {
			selectedProject := projects[row-1]
			if err := tui.projectManager.SetActiveProject(ctx, selectedProject.ID); err != nil {
				tui.statusBar.SetText(fmt.Sprintf("[red]Failed to set active: %v", err))
			} else {
				tui.statusBar.SetText(fmt.Sprintf("[green]Active project: %s", selectedProject.Name))
				tui.showProjects()
			}
		}
	})
	refreshBtn := tview.NewButton("Refresh").SetSelectedFunc(func() {
		tui.showProjects()
	})
	actions.AddItem(newProjectBtn, 0, 1, false)
	actions.AddItem(setActiveBtn, 0, 1, false)
	actions.AddItem(refreshBtn, 0, 1, false)

	// Main content area
	contentFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	contentFlex.AddItem(projectTable, 0, 2, true)
	contentFlex.AddItem(detailsPanel, 40, 0, false)

	projectsView.
		AddItem(header, 3, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(actions, 3, 0, false)

	tui.content.SwitchToPage("projects")
	tui.content.AddPage("projects", projectsView, true, true)
}

// formatProjectDetails formats project details for display
func (tui *TerminalUI) formatProjectDetails(p *project.Project) string {
	return fmt.Sprintf(`[::b]%s[white]

[yellow]Type:[white] %s
[yellow]Path:[white] %s
[yellow]Description:[white] %s

[::b]Build Commands[white]
[green]Build:[white] %s
[green]Test:[white] %s
[green]Lint:[white] %s

[::b]Metadata[white]
[blue]Framework:[white] %s
[blue]Language:[white] %s
[blue]Created:[white] %s`,
		p.Name, p.Type, p.Path, p.Description,
		p.Metadata.BuildCommand, p.Metadata.TestCommand, p.Metadata.LintCommand,
		p.Metadata.Framework, p.Metadata.LanguageVersion,
		p.CreatedAt.Format("2006-01-02 15:04:05"))
}

// showProjectDetails shows detailed information about a project
func (tui *TerminalUI) showProjectDetails(p *project.Project) {
	modal := tview.NewModal()
	modal.SetText(fmt.Sprintf("Project: %s\nType: %s\nPath: %s\n\nBuild: %s\nTest: %s",
		p.Name, p.Type, p.Path, p.Metadata.BuildCommand, p.Metadata.TestCommand))
	modal.AddButtons([]string{"Close", "Set Active", "Delete"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		ctx := context.Background()
		switch buttonLabel {
		case "Set Active":
			if err := tui.projectManager.SetActiveProject(ctx, p.ID); err != nil {
				tui.statusBar.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else {
				tui.statusBar.SetText(fmt.Sprintf("[green]Active: %s", p.Name))
			}
		case "Delete":
			if err := tui.projectManager.DeleteProject(ctx, p.ID); err != nil {
				tui.statusBar.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else {
				tui.statusBar.SetText(fmt.Sprintf("[yellow]Deleted: %s", p.Name))
			}
		}
		tui.pages.RemovePage("projectDetails")
		tui.showProjects()
	})

	tui.pages.AddPage("projectDetails", modal, true, true)
}

// showNewProjectForm displays a form for creating a new project
func (tui *TerminalUI) showNewProjectForm() {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle("Create New Project").SetTitleAlign(tview.AlignLeft)

	var name, description, path, projectType string

	form.AddInputField("Name", "", 30, nil, func(text string) {
		name = text
	})
	form.AddInputField("Description", "", 50, nil, func(text string) {
		description = text
	})
	form.AddInputField("Path", "", 50, nil, func(text string) {
		path = text
	})
	form.AddDropDown("Type", []string{"go", "node", "python", "rust", "generic"}, 0, func(option string, index int) {
		projectType = option
	})

	form.AddButton("Create", func() {
		ctx := context.Background()
		if name == "" || path == "" {
			tui.statusBar.SetText("[red]Name and path are required")
			return
		}

		newProject, err := tui.projectManager.CreateProject(ctx, name, description, path, projectType)
		if err != nil {
			tui.statusBar.SetText(fmt.Sprintf("[red]Failed to create project: %v", err))
		} else {
			tui.statusBar.SetText(fmt.Sprintf("[green]Project created: %s", newProject.Name))
			tui.showProjects()
		}

		tui.pages.RemovePage("newProjectForm")
		tui.app.SetFocus(tui.content)
	})

	form.AddButton("Cancel", func() {
		tui.pages.RemovePage("newProjectForm")
		tui.app.SetFocus(tui.content)
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 18, 1, true).
			AddItem(nil, 0, 1, false), 70, 1, true).
		AddItem(nil, 0, 1, false)

	tui.pages.AddPage("newProjectForm", modal, true, true)
	tui.app.SetFocus(form)
}

// truncatePath truncates a path for display
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// showSessions displays the session management interface
func (tui *TerminalUI) showSessions() {
	sessionsView := tview.NewFlex().SetDirection(tview.FlexRow)

	header := tview.NewTextView().
		SetText("[::b]Session Management").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	header.SetBorder(true)

	// Create session table
	sessionTable := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)
	sessionTable.SetBorder(true).SetTitle("Development Sessions")

	// Set headers
	headers := []string{"Name", "Project", "Mode", "Status", "Duration", "Created"}
	for col, h := range headers {
		sessionTable.SetCell(0, col, tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
	}

	// Get sessions from manager
	sessions := tui.sessionManager.GetAll()

	if len(sessions) == 0 {
		sessionTable.SetCell(1, 0, tview.NewTableCell("No sessions found. Click 'New Session' to create one.").
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
	} else {
		for row, s := range sessions {
			statusColor := "[white]"
			switch s.Status {
			case session.StatusActive:
				statusColor = "[green]"
			case session.StatusPaused:
				statusColor = "[yellow]"
			case session.StatusCompleted:
				statusColor = "[blue]"
			case session.StatusFailed:
				statusColor = "[red]"
			}

			modeColor := "[white]"
			switch s.Mode {
			case session.ModePlanning:
				modeColor = "[cyan]"
			case session.ModeBuilding:
				modeColor = "[green]"
			case session.ModeTesting:
				modeColor = "[yellow]"
			case session.ModeDebugging:
				modeColor = "[red]"
			case session.ModeRefactoring:
				modeColor = "[magenta]"
			}

			duration := s.Duration
			if s.Status == session.StatusActive && !s.StartedAt.IsZero() {
				duration += time.Since(s.StartedAt)
			}

			sessionTable.SetCell(row+1, 0, tview.NewTableCell(s.Name).SetAlign(tview.AlignLeft))
			sessionTable.SetCell(row+1, 1, tview.NewTableCell(s.ProjectID[:8]+"...").SetAlign(tview.AlignLeft))
			sessionTable.SetCell(row+1, 2, tview.NewTableCell(modeColor+string(s.Mode)).SetAlign(tview.AlignCenter))
			sessionTable.SetCell(row+1, 3, tview.NewTableCell(statusColor+string(s.Status)).SetAlign(tview.AlignCenter))
			sessionTable.SetCell(row+1, 4, tview.NewTableCell(formatDuration(duration)).SetAlign(tview.AlignRight))
			sessionTable.SetCell(row+1, 5, tview.NewTableCell(s.CreatedAt.Format("01-02 15:04")).SetAlign(tview.AlignCenter))
		}
	}

	// Session statistics panel
	statsPanel := tui.createSessionStatsPanel()

	// Handle session selection
	sessionTable.SetSelectedFunc(func(row, col int) {
		if row > 0 && row <= len(sessions) {
			selectedSession := sessions[row-1]
			tui.showSessionActions(selectedSession)
		}
	})

	// Action buttons
	actions := tview.NewFlex().SetDirection(tview.FlexColumn)
	newSessionBtn := tview.NewButton("New Session").SetSelectedFunc(func() {
		tui.showNewSessionForm()
	})
	startBtn := tview.NewButton("Start").SetSelectedFunc(func() {
		row, _ := sessionTable.GetSelection()
		if row > 0 && row <= len(sessions) {
			s := sessions[row-1]
			if err := tui.sessionManager.Start(s.ID); err != nil {
				tui.statusBar.SetText(fmt.Sprintf("[red]Failed to start: %v", err))
			} else {
				tui.statusBar.SetText(fmt.Sprintf("[green]Started: %s", s.Name))
				tui.showSessions()
			}
		}
	})
	pauseBtn := tview.NewButton("Pause").SetSelectedFunc(func() {
		row, _ := sessionTable.GetSelection()
		if row > 0 && row <= len(sessions) {
			s := sessions[row-1]
			if err := tui.sessionManager.Pause(s.ID); err != nil {
				tui.statusBar.SetText(fmt.Sprintf("[red]Failed to pause: %v", err))
			} else {
				tui.statusBar.SetText(fmt.Sprintf("[yellow]Paused: %s", s.Name))
				tui.showSessions()
			}
		}
	})
	completeBtn := tview.NewButton("Complete").SetSelectedFunc(func() {
		row, _ := sessionTable.GetSelection()
		if row > 0 && row <= len(sessions) {
			s := sessions[row-1]
			if err := tui.sessionManager.Complete(s.ID); err != nil {
				tui.statusBar.SetText(fmt.Sprintf("[red]Failed to complete: %v", err))
			} else {
				tui.statusBar.SetText(fmt.Sprintf("[blue]Completed: %s", s.Name))
				tui.showSessions()
			}
		}
	})
	refreshBtn := tview.NewButton("Refresh").SetSelectedFunc(func() {
		tui.showSessions()
	})

	actions.AddItem(newSessionBtn, 0, 1, false)
	actions.AddItem(startBtn, 0, 1, false)
	actions.AddItem(pauseBtn, 0, 1, false)
	actions.AddItem(completeBtn, 0, 1, false)
	actions.AddItem(refreshBtn, 0, 1, false)

	// Main content area
	contentFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	contentFlex.AddItem(sessionTable, 0, 2, true)
	contentFlex.AddItem(statsPanel, 35, 0, false)

	sessionsView.
		AddItem(header, 3, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(actions, 3, 0, false)

	tui.content.SwitchToPage("sessions")
	tui.content.AddPage("sessions", sessionsView, true, true)
}

// createSessionStatsPanel creates session statistics panel
func (tui *TerminalUI) createSessionStatsPanel() *tview.TextView {
	panel := tview.NewTextView().SetDynamicColors(true)
	panel.SetBorder(true).SetTitle("Statistics")

	stats := tui.sessionManager.GetStatistics()
	activeSession := tui.sessionManager.GetActive()

	activeInfo := "[gray]None"
	if activeSession != nil {
		activeInfo = fmt.Sprintf("[green]%s[white] (%s)", activeSession.Name, activeSession.Mode)
	}

	content := fmt.Sprintf(`[::b]Session Summary[white]

[green]Total:[white] %d
[green]Active:[white] %d
[yellow]Paused:[white] %d
[blue]Completed:[white] %d
[red]Failed:[white] %d

[::b]Current Session[white]
%s

[::b]Average Duration[white]
%s`,
		stats.Total,
		stats.ByStatus[session.StatusActive],
		stats.ByStatus[session.StatusPaused],
		stats.ByStatus[session.StatusCompleted],
		stats.ByStatus[session.StatusFailed],
		activeInfo,
		formatDuration(stats.AverageDuration))

	panel.SetText(content)
	return panel
}

// showSessionActions shows actions for a specific session
func (tui *TerminalUI) showSessionActions(s *session.Session) {
	modal := tview.NewModal()
	modal.SetText(fmt.Sprintf("Session: %s\nMode: %s\nStatus: %s\nDuration: %s",
		s.Name, s.Mode, s.Status, formatDuration(s.Duration)))

	buttons := []string{"Close"}
	switch s.Status {
	case session.StatusPaused:
		buttons = append([]string{"Start", "Resume"}, buttons...)
	case session.StatusActive:
		buttons = append([]string{"Pause", "Complete"}, buttons...)
	}
	if s.Status != session.StatusActive {
		buttons = append(buttons, "Delete")
	}

	modal.AddButtons(buttons)
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		switch buttonLabel {
		case "Start":
			tui.sessionManager.Start(s.ID)
		case "Resume":
			tui.sessionManager.Resume(s.ID)
		case "Pause":
			tui.sessionManager.Pause(s.ID)
		case "Complete":
			tui.sessionManager.Complete(s.ID)
		case "Delete":
			tui.sessionManager.Delete(s.ID)
		}
		tui.pages.RemovePage("sessionActions")
		tui.showSessions()
	})

	tui.pages.AddPage("sessionActions", modal, true, true)
}

// showNewSessionForm displays a form for creating a new session
func (tui *TerminalUI) showNewSessionForm() {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle("Create New Session").SetTitleAlign(tview.AlignLeft)

	var name, description string
	var mode session.Mode = session.ModePlanning

	// Get active project for default project ID
	ctx := context.Background()
	activeProject, _ := tui.projectManager.GetActiveProject(ctx)
	projectID := ""
	if activeProject != nil {
		projectID = activeProject.ID
	}

	form.AddInputField("Name", "", 30, nil, func(text string) {
		name = text
	})
	form.AddInputField("Description", "", 50, nil, func(text string) {
		description = text
	})
	form.AddInputField("Project ID", projectID, 40, nil, func(text string) {
		projectID = text
	})
	form.AddDropDown("Mode", []string{
		string(session.ModePlanning),
		string(session.ModeBuilding),
		string(session.ModeTesting),
		string(session.ModeRefactoring),
		string(session.ModeDebugging),
		string(session.ModeDeployment),
	}, 0, func(option string, index int) {
		mode = session.Mode(option)
	})

	form.AddButton("Create", func() {
		if name == "" {
			tui.statusBar.SetText("[red]Session name is required")
			return
		}
		if projectID == "" {
			tui.statusBar.SetText("[red]Project ID is required (create or select a project first)")
			return
		}

		newSession, err := tui.sessionManager.Create(projectID, name, description, mode)
		if err != nil {
			tui.statusBar.SetText(fmt.Sprintf("[red]Failed to create session: %v", err))
		} else {
			tui.statusBar.SetText(fmt.Sprintf("[green]Session created: %s", newSession.Name))
			tui.showSessions()
		}

		tui.pages.RemovePage("newSessionForm")
		tui.app.SetFocus(tui.content)
	})

	form.AddButton("Cancel", func() {
		tui.pages.RemovePage("newSessionForm")
		tui.app.SetFocus(tui.content)
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 18, 1, true).
			AddItem(nil, 0, 1, false), 70, 1, true).
		AddItem(nil, 0, 1, false)

	tui.pages.AddPage("newSessionForm", modal, true, true)
	tui.app.SetFocus(form)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// showLLM displays the LLM interaction interface
func (tui *TerminalUI) showLLM() {
	llmView := tview.NewFlex().SetDirection(tview.FlexRow)

	header := tview.NewTextView().
		SetText("[::b]AI Model Interaction").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	header.SetBorder(true)

	// Main content area - split between chat and info panels
	mainContent := tview.NewFlex().SetDirection(tview.FlexColumn)

	// Chat area
	chatFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Chat output/history
	tui.chatOutput = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	tui.chatOutput.SetBorder(true).SetTitle("Chat")
	tui.chatOutput.SetText(tui.formatChatHistory())

	// Chat input
	tui.chatInput = tview.NewInputField().
		SetLabel("You: ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorDarkSlateGray)
	tui.chatInput.SetBorder(true).SetTitle("Message")

	// Handle input submission
	tui.chatInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			message := tui.chatInput.GetText()
			if message != "" {
				tui.sendChatMessage(message)
				tui.chatInput.SetText("")
			}
		}
	})

	chatFlex.
		AddItem(tui.chatOutput, 0, 1, false).
		AddItem(tui.chatInput, 3, 0, true)

	// Info panel
	infoPanel := tui.createLLMInfoPanel()

	mainContent.
		AddItem(chatFlex, 0, 2, true).
		AddItem(infoPanel, 40, 0, false)

	// Action buttons
	actions := tview.NewFlex().SetDirection(tview.FlexColumn)
	selectModelBtn := tview.NewButton("Select Model").SetSelectedFunc(func() {
		tui.showModelSelector()
	})
	clearChatBtn := tview.NewButton("Clear Chat").SetSelectedFunc(func() {
		tui.chatHistory = make([]llm.Message, 0)
		tui.chatOutput.SetText(tui.formatChatHistory())
		tui.statusBar.SetText("[green]Chat cleared")
	})
	settingsBtn := tview.NewButton("Settings").SetSelectedFunc(func() {
		tui.showLLMSettings()
	})
	actions.AddItem(selectModelBtn, 0, 1, false)
	actions.AddItem(clearChatBtn, 0, 1, false)
	actions.AddItem(settingsBtn, 0, 1, false)

	llmView.
		AddItem(header, 3, 0, false).
		AddItem(mainContent, 0, 1, true).
		AddItem(actions, 3, 0, false)

	tui.content.SwitchToPage("llm")
	tui.content.AddPage("llm", llmView, true, true)
}

// createLLMInfoPanel creates the LLM information panel
func (tui *TerminalUI) createLLMInfoPanel() *tview.TextView {
	panel := tview.NewTextView().SetDynamicColors(true)
	panel.SetBorder(true).SetTitle("LLM Info")

	// Get available models
	models := tui.llmManager.GetAvailableModels()
	modelCount := len(models)

	// Get provider status
	ctx := context.Background()
	health := tui.llmManager.HealthCheck(ctx)
	healthyProviders := 0
	for _, h := range health {
		if h.Status == "healthy" {
			healthyProviders++
		}
	}

	currentModel := "Not selected"
	if tui.llmProvider != nil {
		currentModel = tui.llmProvider.GetName()
	}

	content := fmt.Sprintf(`[::b]Current Model[white]
%s

[::b]Provider Status[white]
[green]Healthy:[white] %d
[red]Unhealthy:[white] %d

[::b]Available Models[white]
Total: %d

[::b]Chat Statistics[white]
Messages: %d
Tokens Used: N/A

[::b]Quick Commands[white]
/help - Show help
/clear - Clear chat
/model - Change model
/system - Set system prompt`,
		currentModel,
		healthyProviders,
		len(health)-healthyProviders,
		modelCount,
		len(tui.chatHistory))

	panel.SetText(content)
	return panel
}

// formatChatHistory formats the chat history for display
func (tui *TerminalUI) formatChatHistory() string {
	if len(tui.chatHistory) == 0 {
		return `[::b]Welcome to HelixCode AI Chat[white]

Start a conversation by typing a message below.
Use the buttons to select a model or configure settings.

[::b]Tips:[white]
- Type your message and press Enter to send
- Use /help for available commands
- Select a model before chatting for best results

[gray]No messages yet...`
	}

	var sb strings.Builder
	for _, msg := range tui.chatHistory {
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("[green]You:[white] %s\n\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("[cyan]AI:[white] %s\n\n", msg.Content))
		case "system":
			sb.WriteString(fmt.Sprintf("[yellow]System:[white] %s\n\n", msg.Content))
		}
	}
	return sb.String()
}

// sendChatMessage sends a message to the LLM
func (tui *TerminalUI) sendChatMessage(message string) {
	// Handle commands
	if strings.HasPrefix(message, "/") {
		tui.handleChatCommand(message)
		return
	}

	// Add user message to history
	tui.chatHistory = append(tui.chatHistory, llm.Message{
		Role:    "user",
		Content: message,
	})

	// Update display
	tui.chatOutput.SetText(tui.formatChatHistory())
	tui.chatOutput.ScrollToEnd()

	// Check if provider is available
	if tui.llmProvider == nil {
		tui.chatHistory = append(tui.chatHistory, llm.Message{
			Role:    "assistant",
			Content: "Error: No LLM provider configured. Please configure a provider in Settings > System to enable AI responses, then try again.",
		})
		tui.chatOutput.SetText(tui.formatChatHistory())
		tui.chatOutput.ScrollToEnd()
		tui.statusBar.SetText("[red]No LLM provider configured")
		return
	}

	// Send to LLM provider
	ctx := context.Background()
	request := &llm.LLMRequest{
		ID:          uuid.New(),
		Messages:    tui.chatHistory,
		MaxTokens:   2048,
		Temperature: 0.7,
	}

	tui.statusBar.SetText("[yellow]Generating response...")
	tui.app.Draw()

	response, err := tui.llmProvider.Generate(ctx, request)
	if err != nil {
		tui.chatHistory = append(tui.chatHistory, llm.Message{
			Role:    "assistant",
			Content: fmt.Sprintf("[Error: %v]", err),
		})
		tui.statusBar.SetText(fmt.Sprintf("[red]Error: %v", err))
	} else {
		tui.chatHistory = append(tui.chatHistory, llm.Message{
			Role:    "assistant",
			Content: response.Content,
		})
		tui.statusBar.SetText(fmt.Sprintf("[green]Response received (tokens: %d)", response.Usage.TotalTokens))
	}

	tui.chatOutput.SetText(tui.formatChatHistory())
	tui.chatOutput.ScrollToEnd()
	tui.app.Draw()
}

// handleChatCommand handles chat commands
func (tui *TerminalUI) handleChatCommand(cmd string) {
	switch {
	case cmd == "/help":
		tui.chatHistory = append(tui.chatHistory, llm.Message{
			Role: "system",
			Content: `Available Commands:
/help - Show this help message
/clear - Clear chat history
/model - Show model selector
/system <prompt> - Set system prompt
/info - Show current model info`,
		})
	case cmd == "/clear":
		tui.chatHistory = make([]llm.Message, 0)
		tui.statusBar.SetText("[green]Chat cleared")
	case cmd == "/model":
		tui.showModelSelector()
		return
	case cmd == "/info":
		info := "No model selected"
		if tui.llmProvider != nil {
			info = fmt.Sprintf("Provider: %s\nModels: %d available", tui.llmProvider.GetName(), len(tui.llmProvider.GetModels()))
		}
		tui.chatHistory = append(tui.chatHistory, llm.Message{
			Role:    "system",
			Content: info,
		})
	case strings.HasPrefix(cmd, "/system "):
		systemPrompt := strings.TrimPrefix(cmd, "/system ")
		// Prepend system message to chat history
		tui.chatHistory = append([]llm.Message{{Role: "system", Content: systemPrompt}}, tui.chatHistory...)
		tui.statusBar.SetText("[green]System prompt set")
	default:
		tui.chatHistory = append(tui.chatHistory, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd),
		})
	}

	tui.chatOutput.SetText(tui.formatChatHistory())
	tui.chatOutput.ScrollToEnd()
}

// showModelSelector displays a modal for selecting LLM models
func (tui *TerminalUI) showModelSelector() {
	list := tview.NewList()
	list.SetBorder(true).SetTitle("Select Model")

	// Get available models
	models := tui.llmManager.GetAvailableModels()

	if len(models) == 0 {
		list.AddItem("No models available", "Configure providers in Settings", 0, nil)
		list.AddItem("Configure Ollama", "Local LLM provider", 'o', func() {
			tui.statusBar.SetText("[yellow]Configure Ollama in Settings > System")
			tui.pages.RemovePage("modelSelector")
		})
	} else {
		for i, model := range models {
			shortcut := rune('1' + i)
			if i >= 9 {
				shortcut = 0
			}
			modelInfo := model
			list.AddItem(model.Name, fmt.Sprintf("Provider: %s, Context: %d", model.Provider, model.ContextSize), shortcut, func() {
				tui.selectModel(modelInfo)
				tui.pages.RemovePage("modelSelector")
			})
		}
	}

	list.AddItem("Cancel", "Return to chat", 'c', func() {
		tui.pages.RemovePage("modelSelector")
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, 20, 1, true).
			AddItem(nil, 0, 1, false), 60, 1, true).
		AddItem(nil, 0, 1, false)

	tui.pages.AddPage("modelSelector", modal, true, true)
	tui.app.SetFocus(list)
}

// selectModel selects an LLM model
func (tui *TerminalUI) selectModel(model *llm.ModelInfo) {
	ctx := context.Background()
	provider, err := tui.llmManager.GetProviderForModel(model.Name, model.Provider)
	if err != nil {
		tui.statusBar.SetText(fmt.Sprintf("[red]Failed to get provider: %v", err))
		return
	}

	tui.llmProvider = provider
	tui.statusBar.SetText(fmt.Sprintf("[green]Selected model: %s (%s)", model.Name, model.Provider))

	// Add system message about model selection
	tui.chatHistory = append(tui.chatHistory, llm.Message{
		Role:    "system",
		Content: fmt.Sprintf("Model changed to: %s (Provider: %s)", model.Name, model.Provider),
	})
	tui.chatOutput.SetText(tui.formatChatHistory())

	// Refresh LLM view if currently showing
	if provider.IsAvailable(ctx) {
		tui.showLLM()
	}
}

// showLLMSettings displays LLM configuration settings
func (tui *TerminalUI) showLLMSettings() {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle("LLM Settings").SetTitleAlign(tview.AlignLeft)

	var temperature float64 = 0.7
	var maxTokens int = 2048
	var systemPrompt string

	form.AddInputField("Temperature", "0.7", 10, nil, func(text string) {
		fmt.Sscanf(text, "%f", &temperature)
	})
	form.AddInputField("Max Tokens", "2048", 10, tview.InputFieldInteger, func(text string) {
		fmt.Sscanf(text, "%d", &maxTokens)
	})
	form.AddInputField("System Prompt", "", 50, nil, func(text string) {
		systemPrompt = text
	})

	form.AddButton("Apply", func() {
		if systemPrompt != "" {
			// Add system prompt to beginning of chat
			tui.chatHistory = append([]llm.Message{{Role: "system", Content: systemPrompt}}, tui.chatHistory...)
		}
		tui.statusBar.SetText(fmt.Sprintf("[green]Settings applied: temp=%.2f, max_tokens=%d", temperature, maxTokens))
		tui.pages.RemovePage("llmSettings")
		tui.showLLM()
	})

	form.AddButton("Cancel", func() {
		tui.pages.RemovePage("llmSettings")
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 16, 1, true).
			AddItem(nil, 0, 1, false), 60, 1, true).
		AddItem(nil, 0, 1, false)

	tui.pages.AddPage("llmSettings", modal, true, true)
	tui.app.SetFocus(form)
}

// showSettings displays the settings interface
func (tui *TerminalUI) showSettings() {
	settingsView := tview.NewFlex().SetDirection(tview.FlexRow)

	header := tview.NewTextView()
	header.SetText("[::b]Settings & Configuration")
	header.SetTextAlign(tview.AlignCenter)
	header.SetDynamicColors(true)
	header.SetBorder(true)

	// Create tabs for different settings categories
	tabs := tview.NewTextView()
	tabs.SetText("[::b]1. Theme  2. Cognee  3. System")
	tabs.SetTextAlign(tview.AlignCenter)
	tabs.SetDynamicColors(true)
	tabs.SetBorder(true)
	tabs.SetTitle("Settings Categories")

	// Theme settings
	themeView := tui.createThemeSettingsView()

	// Cognee settings
	cogneeView := tui.createCogneeSettingsView()

	// System settings
	systemView := tui.createSystemSettingsView()

	// Start with theme view
	contentArea := tview.NewPages()
	contentArea.AddPage("theme", themeView, true, true)
	contentArea.AddPage("cognee", cogneeView, true, true)
	contentArea.AddPage("system", systemView, true, true)
	contentArea.SwitchToPage("theme")

	// Handle tab navigation
	tabs.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF1, tcell.KeyCtrlT:
			contentArea.SwitchToPage("theme")
			return nil
		case tcell.KeyF2, tcell.KeyCtrlC:
			contentArea.SwitchToPage("cognee")
			return nil
		case tcell.KeyF3, tcell.KeyCtrlS:
			contentArea.SwitchToPage("system")
			return nil
		}
		return event
	})

	settingsView.
		AddItem(header, 3, 0, false).
		AddItem(tabs, 3, 0, false).
		AddItem(contentArea, 0, 1, false)

	tui.content.SwitchToPage("settings")
	tui.content.AddPage("settings", settingsView, true, true)
}

// createThemeSettingsView creates the theme settings view
func (tui *TerminalUI) createThemeSettingsView() tview.Primitive {
	view := tview.NewFlex().SetDirection(tview.FlexRow)

	// Theme selection
	themeList := tview.NewList()
	themeList.SetBorder(true)
	themeList.SetTitle("Theme Selection")
	themeList.SetTitleAlign(tview.AlignLeft)

	themes := tui.themeManager.GetAvailableThemes()
	for _, themeName := range themes {
		// Capture themeName by value — Go's loop variable was the
		// previous gotcha that hid the bluff (every callback would
		// have closed over the same final value). Round-33 §11.4
		// anti-bluff fix: previous closure body was empty with the
		// comment "// For now, just show a message" — the
		// "Theme Selection" UI element rendered every theme but
		// selecting one was a no-op, fabricating UX completion.
		// CONST-035 / Article XI §11.9 / CONST-050(A).
		name := themeName
		themeList.AddItem(name, "", 0, func() {
			tui.themeManager.SetTheme(name)
		})
	}

	// Current theme info
	currentTheme := tui.themeManager.GetCurrentTheme()
	themeInfo := tview.NewTextView()
	themeInfo.SetBorder(true)
	themeInfo.SetTitle("Current Theme")
	themeInfo.SetTitleAlign(tview.AlignLeft)
	themeInfo.SetText(fmt.Sprintf("Name: %s\nDark: %t\nPrimary: %s\nSecondary: %s\nAccent: %s",
		currentTheme.Name, currentTheme.IsDark,
		currentTheme.Primary, currentTheme.Secondary, currentTheme.Accent))

	view.
		AddItem(themeList, 0, 1, false).
		AddItem(themeInfo, 0, 1, false)

	return view
}

// createCogneeSettingsView creates the Cognee settings view
func (tui *TerminalUI) createCogneeSettingsView() tview.Primitive {
	view := tview.NewFlex().SetDirection(tview.FlexRow)

	// Cognee status and controls
	statusView := tview.NewTextView()
	statusView.SetBorder(true)
	statusView.SetTitle("Cognee Status")
	statusView.SetTitleAlign(tview.AlignLeft)
	statusView.SetDynamicColors(true)

	// Get Cognee config from helix config
	cogneeEnabled := "Disabled"
	cogneeMode := "N/A"
	if tui.helixConfig != nil && tui.helixConfig.Cognee.Enabled {
		cogneeEnabled = "[green]Enabled"
		cogneeMode = tui.helixConfig.Cognee.Mode
	}

	statusView.SetText(fmt.Sprintf("Status: %s\nMode: %s\nHost: %s\nPort: %d",
		cogneeEnabled, cogneeMode,
		tui.helixConfig.Cognee.Host, tui.helixConfig.Cognee.Port))

	// Control buttons
	controls := tview.NewFlex().SetDirection(tview.FlexColumn)

	enableBtn := tview.NewButton("Enable Cognee").SetSelectedFunc(func() {
		// Enable Cognee in configuration
		tui.helixConfig.Cognee.Enabled = true
		if tui.helixConfig.Cognee.Host == "" {
			tui.helixConfig.Cognee.Host = "localhost"
		}
		if tui.helixConfig.Cognee.Port == 0 {
			tui.helixConfig.Cognee.Port = 8000
		}

		// Save configuration
		if err := config.SaveHelixConfig(tui.helixConfig); err != nil {
			log.Printf("Failed to save Cognee config: %v", err)
		}

		// Update status display
		statusView.SetText(fmt.Sprintf("Status: [green]Enabled\nMode: %s\nHost: %s\nPort: %d",
			tui.helixConfig.Cognee.Mode,
			tui.helixConfig.Cognee.Host,
			tui.helixConfig.Cognee.Port))

		tui.statusBar.SetText(" Status: Cognee enabled successfully")
	})
	controls.AddItem(enableBtn, 0, 1, false)

	disableBtn := tview.NewButton("Disable Cognee").SetSelectedFunc(func() {
		// Disable Cognee in configuration
		tui.helixConfig.Cognee.Enabled = false

		// Save configuration
		if err := config.SaveHelixConfig(tui.helixConfig); err != nil {
			log.Printf("Failed to save Cognee config: %v", err)
		}

		// Update status display
		statusView.SetText("Status: [red]Disabled\nMode: N/A\nHost: N/A\nPort: N/A")

		tui.statusBar.SetText(" Status: Cognee disabled successfully")
	})
	controls.AddItem(disableBtn, 0, 1, false)

	// Configuration options
	configView := tview.NewTextView()
	configView.SetBorder(true)
	configView.SetTitle("Configuration Options")
	configView.SetTitleAlign(tview.AlignLeft)
	configView.SetText(`[::b]Basic Settings:
• Auto Start: Enabled
• Mode: Local/Remote
• Host: localhost
• Port: 8000

[::b]Features:
• Knowledge Graph: Enabled
• Semantic Search: Enabled
• Real-time Processing: Enabled
• Multi-modal Support: Enabled

[::b]Performance:
• Workers: 4
• Cache: Redis
• Optimization: High`)

	view.
		AddItem(statusView, 6, 0, false).
		AddItem(controls, 3, 0, false).
		AddItem(configView, 0, 1, false)

	return view
}

// showNewTaskForm displays a modal form for creating a new task
func (tui *TerminalUI) showNewTaskForm() {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle("Create New Task").SetTitleAlign(tview.AlignLeft)

	// Form fields
	var taskType, taskData, taskPriority, taskCriticality string

	form.AddDropDown("Task Type", []string{
		"Planning",
		"Building",
		"Testing",
		"Refactoring",
		"Debugging",
		"Deployment",
	}, 0, func(option string, index int) {
		taskType = option
	})

	form.AddInputField("Task Data (JSON)", `{"description": "Task description"}`, 50, nil, func(text string) {
		taskData = text
	})

	form.AddDropDown("Priority", []string{"Low", "Normal", "High", "Critical"}, 1, func(option string, index int) {
		taskPriority = option
	})

	form.AddDropDown("Criticality", []string{"Low", "Normal", "High", "Critical"}, 1, func(option string, index int) {
		taskCriticality = option
	})

	// Buttons
	form.AddButton("Create", func() {
		// Parse task data
		var data map[string]interface{}
		if taskData != "" {
			// In production, would properly parse JSON
			data = map[string]interface{}{
				"description": taskData,
			}
		}

		// Map string values to task constants
		typeMap := map[string]task.TaskType{
			"Planning":    task.TaskTypePlanning,
			"Building":    task.TaskTypeBuilding,
			"Testing":     task.TaskTypeTesting,
			"Refactoring": task.TaskTypeRefactoring,
			"Debugging":   task.TaskTypeDebugging,
			"Deployment":  task.TaskTypeDeployment,
		}

		priorityMap := map[string]task.TaskPriority{
			"Low":      task.PriorityLow,
			"Normal":   task.PriorityNormal,
			"High":     task.PriorityHigh,
			"Critical": task.PriorityCritical,
		}

		criticalityMap := map[string]task.TaskCriticality{
			"Low":      task.CriticalityLow,
			"Normal":   task.CriticalityNormal,
			"High":     task.CriticalityHigh,
			"Critical": task.CriticalityCritical,
		}

		// Create task
		newTask, err := tui.taskManager.CreateTask(
			typeMap[taskType],
			data,
			priorityMap[taskPriority],
			criticalityMap[taskCriticality],
			[]uuid.UUID{}, // No dependencies for now
		)

		if err != nil {
			tui.statusBar.SetText(fmt.Sprintf(" Status: Failed to create task: %v", err))
		} else {
			tui.statusBar.SetText(fmt.Sprintf(" Status: Task created successfully: %s", newTask.ID))
		}

		// Close the modal
		tui.pages.RemovePage("newTaskForm")
		tui.app.SetFocus(tui.content)
	})

	form.AddButton("Cancel", func() {
		tui.pages.RemovePage("newTaskForm")
		tui.app.SetFocus(tui.content)
	})

	// Create a modal
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 20, 1, true).
			AddItem(nil, 0, 1, false), 60, 1, true).
		AddItem(nil, 0, 1, false)

	tui.pages.AddPage("newTaskForm", modal, true, true)
	tui.app.SetFocus(form)
}

// createSystemSettingsView creates the system settings view
func (tui *TerminalUI) createSystemSettingsView() tview.Primitive {
	view := tview.NewTextView()
	view.SetBorder(true)
	view.SetTitle("System Settings")
	view.SetTitleAlign(tview.AlignLeft)
	view.SetText(`[::b]System Configuration:

Database: PostgreSQL
Redis: Enabled
Workers: 4 active
Tasks: 0 running

[::b]Performance:
CPU Usage: 15%
Memory: 2.1GB / 8GB
Disk: 45GB free

[::b]Network:
Port: 8080
SSL: Disabled
CORS: Enabled`)

	return view
}

// showQA displays the QA dashboard with session list and controls.
func (tui *TerminalUI) showQA() {
	qaView := tview.NewFlex().SetDirection(tview.FlexRow)

	header := tview.NewTextView().
		SetText("[::b]QA Dashboard").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	header.SetBorder(true)

	// Engine status line
	statusText := "[red]QA Engine: DISABLED"
	if tui.qaEngine != nil && tui.qaEngine.Enabled() {
		statusText = "[green]QA Engine: ENABLED"
	}
	statusView := tview.NewTextView().
		SetText(statusText).
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)
	statusView.SetBorder(true).SetTitle("Status")

	// Session table
	sessionTable := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)
	sessionTable.SetBorder(true).SetTitle("Sessions")

	headers := []string{"ID", "Status", "Phase", "Progress", "Platforms", "Banks", "Duration"}
	for col, h := range headers {
		sessionTable.SetCell(0, col, tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
	}

	if tui.qaEngine == nil || !tui.qaEngine.Enabled() {
		sessionTable.SetCell(1, 0, tview.NewTableCell("QA engine is disabled. Enable in config (qa.enabled = true).").
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
	} else {
		sessions := tui.qaEngine.ListSessions()
		if len(sessions) == 0 {
			sessionTable.SetCell(1, 0, tview.NewTableCell("No sessions. Start a QA session to see results.").
				SetAlign(tview.AlignCenter).
				SetSelectable(false))
		} else {
			for row, s := range sessions {
				statusColor := "[yellow]"
				switch s.Status {
				case "completed":
					statusColor = "[green]"
				case "failed", "cancelled":
					statusColor = "[red]"
				case "running":
					statusColor = "[blue]"
				}

				duration := "-"
				if !s.StartTime.IsZero() {
					if s.EndTime != nil {
						duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
					} else {
						duration = time.Since(s.StartTime).Round(time.Second).String()
					}
				}

				progressStr := fmt.Sprintf("%.0f%%", s.PhaseProgress*100)

				sessionTable.SetCell(row+1, 0, tview.NewTableCell(s.ID[:min(8, len(s.ID))]).SetAlign(tview.AlignLeft))
				sessionTable.SetCell(row+1, 1, tview.NewTableCell(statusColor+s.Status).SetAlign(tview.AlignCenter))
				sessionTable.SetCell(row+1, 2, tview.NewTableCell(s.Phase).SetAlign(tview.AlignLeft))
				sessionTable.SetCell(row+1, 3, tview.NewTableCell(progressStr).SetAlign(tview.AlignCenter))
				sessionTable.SetCell(row+1, 4, tview.NewTableCell(strings.Join(s.Platforms, ", ")).SetAlign(tview.AlignLeft))
				sessionTable.SetCell(row+1, 5, tview.NewTableCell(strings.Join(s.Banks, ", ")).SetAlign(tview.AlignLeft))
				sessionTable.SetCell(row+1, 6, tview.NewTableCell(duration).SetAlign(tview.AlignCenter))
			}
		}
	}

	// Stats panel
	statsPanel := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	statsPanel.SetBorder(true).SetTitle("QA Stats")

	var statsBuilder strings.Builder
	if tui.qaEngine != nil && tui.qaEngine.Enabled() {
		sessions := tui.qaEngine.ListSessions()
		var running, completed, failed int
		for _, s := range sessions {
			switch s.Status {
			case "running":
				running++
			case "completed":
				completed++
			case "failed":
				failed++
			}
		}
		statsBuilder.WriteString(fmt.Sprintf("[white]Total Sessions: [yellow]%d\n", len(sessions)))
		statsBuilder.WriteString(fmt.Sprintf("[white]Running: [blue]%d\n", running))
		statsBuilder.WriteString(fmt.Sprintf("[white]Completed: [green]%d\n", completed))
		statsBuilder.WriteString(fmt.Sprintf("[white]Failed: [red]%d\n", failed))
		statsBuilder.WriteString(fmt.Sprintf("[white]Coverage Target: [yellow]%.0f%%", tui.config.QA.CoverageTarget*100))
	} else {
		statsBuilder.WriteString("[gray]QA not configured.\n")
		statsBuilder.WriteString("[gray]Set qa.enabled = true in config.")
	}
	statsPanel.SetText(statsBuilder.String())

	// Action buttons
	actions := tview.NewFlex().SetDirection(tview.FlexColumn)
	refreshBtn := tview.NewButton("Refresh").SetSelectedFunc(func() {
		tui.showQA()
	})
	actions.AddItem(refreshBtn, 0, 1, false)
	actions.AddItem(tview.NewBox(), 1, 0, false)

	if tui.qaEngine != nil && tui.qaEngine.Enabled() {
		startBtn := tview.NewButton("Start Session").SetSelectedFunc(func() {
			tui.showStartQAForm()
		})
		actions.AddItem(startBtn, 0, 1, false)

		cancelBtn := tview.NewButton("Cancel Selected").SetSelectedFunc(func() {
			row, _ := sessionTable.GetSelection()
			if row > 0 {
				cell := sessionTable.GetCell(row, 0)
				if cell != nil {
					sessionID := cell.Text
					if err := tui.qaEngine.CancelSession(sessionID); err != nil {
						tui.statusBar.SetText(fmt.Sprintf("[red]Cancel failed: %v", err))
					} else {
						tui.statusBar.SetText(fmt.Sprintf("[green]Session %s cancelled", sessionID))
						tui.showQA()
					}
				}
			}
		})
		actions.AddItem(cancelBtn, 0, 1, false)
	}

	// Main content area
	contentFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	contentFlex.AddItem(sessionTable, 0, 3, true)
	contentFlex.AddItem(statsPanel, 30, 0, false)

	qaView.
		AddItem(header, 3, 0, false).
		AddItem(statusView, 3, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(actions, 3, 0, false)

	tui.content.SwitchToPage("qa")
	tui.content.AddPage("qa", qaView, true, true)
}

// showStartQAForm displays a form for starting a new QA session.
func (tui *TerminalUI) showStartQAForm() {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle("Start QA Session").SetTitleAlign(tview.AlignLeft)

	var platformsStr, banksStr string
	var autonomous bool

	form.AddInputField("Platforms", "web", 30, nil, func(text string) {
		platformsStr = text
	})
	form.AddInputField("Banks", "default", 30, nil, func(text string) {
		banksStr = text
	})
	form.AddCheckbox("Autonomous", false, func(checked bool) {
		autonomous = checked
	})

	form.AddButton("Start", func() {
		if platformsStr == "" || banksStr == "" {
			tui.statusBar.SetText("[red]Platforms and banks are required")
			tui.pages.RemovePage("startQAForm")
			return
		}
		platforms := strings.Split(platformsStr, ",")
		for i := range platforms {
			platforms[i] = strings.TrimSpace(platforms[i])
		}
		banks := strings.Split(banksStr, ",")
		for i := range banks {
			banks[i] = strings.TrimSpace(banks[i])
		}

		sessionID := uuid.New().String()
		_, err := tui.qaEngine.StartSession(context.Background(), sessionID, platforms, banks, autonomous)
		if err != nil {
			tui.statusBar.SetText(fmt.Sprintf("[red]Start session failed: %v", err))
		} else {
			tui.statusBar.SetText(fmt.Sprintf("[green]Session %s started", sessionID[:8]))
		}
		tui.pages.RemovePage("startQAForm")
		tui.showQA()
	})

	form.AddButton("Cancel", func() {
		tui.pages.RemovePage("startQAForm")
	})

	modal := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(form, 15, 0, true).
		AddItem(tview.NewBox(), 0, 1, false)

	tui.pages.AddPage("startQAForm", modal, true, true)
}

// Run starts the Terminal UI application
func (tui *TerminalUI) Run() error {
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the application in a goroutine
	go func() {
		if err := tui.app.SetRoot(tui.pages, true).Run(); err != nil {
			log.Printf("TUI application error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	tui.app.Stop()

	return nil
}

// Close cleans up resources
func (tui *TerminalUI) Close() error {
	if tui.db != nil {
		tui.db.Close()
	}
	return nil
}

func main() {
	tui := NewTerminalUI()

	if err := tui.Initialize(); err != nil {
		log.Fatalf("Failed to initialize Terminal UI: %v", err)
	}
	defer tui.Close()

	if err := tui.Run(); err != nil {
		log.Fatalf("Terminal UI error: %v", err)
	}
}
