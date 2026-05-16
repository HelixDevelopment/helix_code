// Package session provides development session management with lifecycle control and focus tracking.
//
// The session package manages development sessions within HelixCode, tracking
// session state, duration, context, and integrating with focus chains for
// file tracking. Sessions support different modes (planning, building, testing,
// etc.) and provide lifecycle callbacks for event-driven programming.
//
// # Key Components
//
// Manager coordinates all session operations:
//
//	manager := session.NewManager()
//
//	// Create a new session
//	sess, err := manager.Create("project-123", "Feature Implementation", "Implementing auth", session.ModeBuilding)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Start the session
//	err = manager.Start(sess.ID)
//
//	// Work on the session...
//
//	// Complete the session
//	err = manager.Complete(sess.ID)
//
// # Session Lifecycle
//
// Sessions follow a defined lifecycle:
//
//	StatusPaused    -> StatusActive (Start/Resume)
//	StatusActive    -> StatusPaused (Pause)
//	StatusActive    -> StatusCompleted (Complete)
//	StatusActive    -> StatusFailed (Fail)
//	StatusPaused    -> StatusCompleted (Complete)
//
// # Session Modes
//
// Sessions operate in different modes based on the development activity:
//
//	session.ModePlanning    // Architecture and design planning
//	session.ModeBuilding    // Active code development
//	session.ModeTesting     // Testing and validation
//	session.ModeRefactoring // Code improvement
//	session.ModeDebugging   // Bug investigation
//	session.ModeDeployment  // Deployment operations
//
// # Session Operations
//
// The Manager provides comprehensive session control:
//
//	// Start a session (makes it active)
//	err := manager.Start(sessionID)
//
//	// Pause an active session
//	err = manager.Pause(sessionID)
//
//	// Resume a paused session
//	err = manager.Resume(sessionID)
//
//	// Complete a session
//	err = manager.Complete(sessionID)
//
//	// Fail a session with reason
//	err = manager.Fail(sessionID, "build failed")
//
//	// Delete a session (only non-active)
//	err = manager.Delete(sessionID)
//
// # Querying Sessions
//
// Sessions can be retrieved in various ways:
//
//	// Get single session
//	sess, err := manager.Get(sessionID)
//
//	// Get currently active session
//	active := manager.GetActive()
//
//	// Get all sessions
//	all := manager.GetAll()
//
//	// Filter by project
//	projectSessions := manager.GetByProject("project-123")
//
//	// Filter by mode
//	buildingSessions := manager.GetByMode(session.ModeBuilding)
//
//	// Filter by status
//	activeSessions := manager.GetByStatus(session.StatusActive)
//
//	// Filter by tag
//	taggedSessions := manager.GetByTag("urgent")
//
//	// Get recent sessions
//	recent := manager.GetRecent(10)
//
//	// Search by name
//	found := manager.FindByName("auth")
//
// # Lifecycle Callbacks
//
// Register callbacks for session events:
//
//	// On session creation
//	manager.OnCreate(func(s *session.Session) {
//	    log.Printf("Session created: %s", s.Name)
//	})
//
//	// On session start
//	manager.OnStart(func(s *session.Session) {
//	    notifyTeam(s)
//	})
//
//	// On session completion
//	manager.OnComplete(func(s *session.Session) {
//	    generateReport(s)
//	})
//
//	// On session switch
//	manager.OnSwitch(func(from, to *session.Session) {
//	    saveContext(from)
//	    loadContext(to)
//	})
//
// # Session Context and Metadata
//
// Sessions maintain context and metadata:
//
//	// Set context data
//	sess.SetContext("current_file", "main.go")
//	sess.SetContext("cursor_position", 42)
//
//	// Get context data
//	file, ok := sess.GetContext("current_file")
//
//	// Set metadata
//	sess.SetMetadata("branch", "feature/auth")
//	sess.SetMetadata("commit", "abc123")
//
//	// Get metadata
//	branch, ok := sess.GetMetadata("branch")
//
// # Tags
//
// Sessions support tags for organization:
//
//	sess.AddTag("urgent")
//	sess.AddTag("security")
//
//	if sess.HasTag("urgent") {
//	    prioritize(sess)
//	}
//
//	sess.RemoveTag("urgent")
//
// # Focus Chain Integration
//
// Sessions integrate with focus chains for file tracking:
//
//	// Get the focus manager
//	focusMgr := manager.GetFocusManager()
//
//	// Each session has a dedicated focus chain
//	chainID := sess.FocusChainID
//
//	// Create manager with existing integrations
//	manager := session.NewManagerWithIntegrations(focusMgr, hooksMgr)
//
// # Statistics
//
// Get statistics about sessions:
//
//	stats := manager.GetStatistics()
//
//	fmt.Printf("Total: %d\n", stats.Total)
//	fmt.Printf("Active: %d\n", stats.ByStatus[session.StatusActive])
//	fmt.Printf("Completed: %d\n", stats.ByStatus[session.StatusCompleted])
//	fmt.Printf("Average Duration: %v\n", stats.AverageDuration)
//
// # Session Export/Import
//
// Sessions can be exported and imported:
//
//	// Export session with focus chain
//	snapshot, err := manager.Export(sessionID)
//
//	// Import session
//	err = manager.Import(snapshot)
//
// # History Management
//
// Manage session history:
//
//	// Set maximum history
//	manager.SetMaxHistory(100)
//
//	// Trim old completed sessions
//	removed := manager.TrimHistory()
//	fmt.Printf("Removed %d old sessions\n", removed)
//
// # Thread Safety
//
// All Manager operations are thread-safe through internal mutex protection,
// allowing concurrent access from multiple goroutines.
//
// # Duration Tracking
//
// Session duration is automatically tracked:
//
//	// Duration accumulates across start/pause cycles
//	fmt.Printf("Total duration: %v\n", sess.Duration)
//
//	// StartedAt tracks current active period
//	if !sess.StartedAt.IsZero() {
//	    currentDuration := time.Since(sess.StartedAt)
//	}
package session
