// Package workflow provides DAG-based workflow execution for development automation.
//
// The workflow package implements a comprehensive workflow execution engine that
// supports planning, building, testing, and refactoring workflows. Workflows are
// composed of steps with dependencies forming a directed acyclic graph (DAG),
// and can leverage LLM providers for AI-powered analysis and code generation.
//
// # Key Components
//
// Executor is the main workflow engine:
//
//	projectMgr := project.NewDatabaseManager(db)
//	executor := workflow.NewExecutor(projectMgr)
//
//	// Or with LLM support
//	executor := workflow.NewExecutorWithLLM(projectMgr, llmProvider, workflow.DefaultExecutorConfig())
//
// # Workflow Types
//
// The executor supports predefined workflow types:
//
//	// Planning workflow - analyze requirements and generate architecture
//	wf, err := executor.ExecutePlanningWorkflow(ctx, projectID)
//
//	// Building workflow - compile and build the project
//	wf, err = executor.ExecuteBuildingWorkflow(ctx, projectID)
//
//	// Testing workflow - run test suites
//	wf, err = executor.ExecuteTestingWorkflow(ctx, projectID)
//
//	// Refactoring workflow - analyze and improve code
//	wf, err = executor.ExecuteRefactoringWorkflow(ctx, projectID)
//
// # Workflow Structure
//
// Workflows consist of steps with dependencies:
//
//	workflow := &workflow.Workflow{
//	    ID:          "build-123",
//	    Name:        "Project Build",
//	    Description: "Build and compile project",
//	    Mode:        "building",
//	    Steps: []workflow.Step{
//	        {
//	            ID:     "setup",
//	            Name:   "Setup Environment",
//	            Type:   workflow.StepTypeExecution,
//	            Action: workflow.StepActionExecuteCommand,
//	        },
//	        {
//	            ID:           "build",
//	            Name:         "Build Project",
//	            Type:         workflow.StepTypeExecution,
//	            Action:       workflow.StepActionBuildProject,
//	            Dependencies: []string{"setup"},
//	        },
//	    },
//	    Status: workflow.WorkflowStatusPending,
//	}
//
// # Step Types
//
// Steps are categorized by their purpose:
//
//	workflow.StepTypeAnalysis   // Code analysis and inspection
//	workflow.StepTypeGeneration // Code generation
//	workflow.StepTypeExecution  // Command execution
//	workflow.StepTypeValidation // Validation and verification
//
// # Step Actions
//
// Steps perform specific actions:
//
//	workflow.StepActionAnalyzeCode    // Analyze codebase with LLM or static analysis
//	workflow.StepActionGenerateCode   // Generate code with LLM or templates
//	workflow.StepActionExecuteCommand // Execute shell command
//	workflow.StepActionRunTests       // Run test suite
//	workflow.StepActionLintCode       // Run linters
//	workflow.StepActionBuildProject   // Build/compile project
//
// # Workflow Status
//
// Workflows progress through statuses:
//
//	workflow.WorkflowStatusPending   // Not yet started
//	workflow.WorkflowStatusRunning   // Currently executing
//	workflow.WorkflowStatusCompleted // All steps completed
//	workflow.WorkflowStatusFailed    // A step failed
//
// # Step Status
//
// Steps have their own status tracking:
//
//	workflow.StepStatusPending   // Waiting to execute
//	workflow.StepStatusRunning   // Currently executing
//	workflow.StepStatusCompleted // Successfully completed
//	workflow.StepStatusFailed    // Execution failed
//	workflow.StepStatusSkipped   // Skipped (dependencies not met)
//
// # LLM Integration
//
// The executor integrates with LLM providers for AI-powered operations:
//
//	// Set LLM provider
//	executor.SetLLMProvider(llmProvider)
//
//	// LLM is used for:
//	// - Code analysis (StepActionAnalyzeCode)
//	// - Code generation (StepActionGenerateCode)
//	// - Architecture planning
//	// - Refactoring suggestions
//
// Without LLM, the executor falls back to:
//   - Static analysis for code inspection
//   - Template-based code generation
//
// # Configuration
//
// ExecutorConfig controls execution behavior:
//
//	config := &workflow.ExecutorConfig{
//	    MaxConcurrentSteps: 4,             // Parallel step execution
//	    StepTimeout:        10 * time.Minute,
//	    EnableLLM:          true,          // Use LLM when available
//	    EnableMetrics:      true,          // Track execution metrics
//	}
//
//	executor := workflow.NewExecutorWithLLM(projectMgr, llmProvider, config)
//
// # Execution Metrics
//
// Track workflow execution metrics:
//
//	metrics := executor.GetMetrics()
//
//	fmt.Printf("Workflows Started: %d\n", metrics.WorkflowsStarted)
//	fmt.Printf("Workflows Success: %d\n", metrics.WorkflowsSuccess)
//	fmt.Printf("Workflows Failed: %d\n", metrics.WorkflowsFailed)
//	fmt.Printf("Steps Executed: %d\n", metrics.StepsExecuted)
//	fmt.Printf("LLM Calls: %d\n", metrics.LLMCalls)
//	fmt.Printf("LLM Tokens Used: %d\n", metrics.LLMTokensUsed)
//
// # Active Workflow Management
//
// Monitor and control active workflows:
//
//	// Get all active workflows
//	active := executor.GetActiveWorkflows()
//
//	// Get specific workflow
//	wf, ok := executor.GetWorkflow(workflowID)
//
//	// Cancel a workflow
//	err := executor.CancelWorkflow(workflowID)
//
// # Project Type Support
//
// The executor supports multiple project types:
//   - go: Go projects (go build, go test)
//   - node: Node.js projects (npm build, npm test)
//   - python: Python projects (pytest, setup.py)
//   - rust: Rust projects (cargo build, cargo test)
//
// Commands are automatically selected based on project type.
//
// # Security
//
// Command execution includes security measures:
//
//	// Commands are validated against dangerous patterns:
//	// - rm -rf /
//	// - System shutdown commands
//	// - Raw disk access
//	// - Fork bombs
//	// - Piped shell execution
//
// # Sub-packages
//
// The workflow package includes specialized sub-packages:
//
// autonomy: Autonomous execution with guardrails
//
//	// Configure autonomy levels
//	controller := autonomy.NewController(config)
//	controller.SetAutonomyLevel(autonomy.LevelSemiAutonomous)
//
// planmode: Plan mode execution
//
//	// Execute in plan-only mode
//	planner := planmode.NewPlanner(config)
//	plan := planner.GeneratePlan(ctx, request)
//
// snapshots: Workflow state snapshots
//
//	// Create snapshot for recovery
//	snapshot := snapshots.Create(workflow)
//	snapshots.Restore(snapshot)
//
// # Dependency Resolution
//
// Steps execute in dependency order:
//
//	// Steps with no dependencies run first
//	// Steps wait for all dependencies to complete
//	// Failed dependencies cause step skipping
//
// # Template-Based Generation
//
// Without LLM, code generation uses templates:
//
//	// Generated code includes:
//	// - Project boilerplate
//	// - Error handling patterns
//	// - Graceful shutdown
//	// - Language-specific idioms
//
// Templates are provided for Go, Node.js, Python, and Rust.
//
// # Thread Safety
//
// The Executor is thread-safe for concurrent workflow management,
// but individual workflows execute their steps sequentially by default.
package workflow
