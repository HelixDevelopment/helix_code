// Package hooks provides lifecycle hooks and event handling for the HelixCode platform.
//
// The hooks package enables extensible event-driven architecture by allowing components
// to register callbacks that execute at specific points in the application lifecycle.
// Hooks can be synchronous or asynchronous, have configurable priorities, and support
// conditional execution based on event data.
//
// # Architecture
//
// The package follows a Manager-Executor-Hook pattern:
//   - Manager: Central registry for hook registration and triggering
//   - Executor: Handles hook execution with concurrency control
//   - Hook: Individual hook with handler function and metadata
//
// # Hook Types
//
// The package supports various hook types for different lifecycle events:
//
//   - HookTypeBeforeTask / HookTypeAfterTask: Task execution lifecycle
//   - HookTypeBeforeLLM / HookTypeAfterLLM: LLM API call lifecycle
//   - HookTypeBeforeEdit / HookTypeAfterEdit: File editing lifecycle
//   - HookTypeBeforeBuild / HookTypeAfterBuild: Build process lifecycle
//   - HookTypeBeforeTest / HookTypeAfterTest: Test execution lifecycle
//   - HookTypeOnError / HookTypeOnSuccess: Completion status events
//   - HookTypeCustom: User-defined hook types
//
// # Priority System
//
// Hooks execute in priority order (highest first):
//
//   - PriorityHighest (100): Execute first
//   - PriorityHigh (75): High priority
//   - PriorityNormal (50): Default priority
//   - PriorityLow (25): Low priority
//   - PriorityLowest (1): Execute last
//
// # Basic Usage
//
// Creating and registering hooks:
//
//	manager := hooks.NewManager()
//
//	// Register a synchronous hook
//	hook := hooks.NewHook("lint", hooks.HookTypeBeforeBuild, func(ctx context.Context, event *hooks.Event) error {
//	    return runLinter(ctx)
//	})
//	manager.Register(hook)
//
//	// Register an async hook with priority
//	asyncHook := hooks.NewHookWithPriority("notify", hooks.HookTypeAfterBuild, notifyHandler, hooks.PriorityLow)
//	asyncHook.Async = true
//	manager.Register(asyncHook)
//
// # Triggering Hooks
//
// Hooks can be triggered in several ways:
//
//	// Trigger asynchronously (returns immediately)
//	results := manager.Trigger(ctx, hooks.HookTypeBeforeBuild)
//
//	// Trigger and wait for all hooks (including async)
//	results := manager.TriggerAndWait(ctx, hooks.HookTypeBeforeBuild)
//
//	// Trigger synchronously (all hooks execute in sequence)
//	results := manager.TriggerSync(ctx, hooks.HookTypeBeforeBuild)
//
//	// Trigger with event data
//	event := hooks.NewEventWithContext(ctx, hooks.HookTypeBeforeBuild)
//	event.SetData("project", projectName)
//	results := manager.TriggerEvent(event)
//
// # Conditional Execution
//
// Hooks can have conditions that determine whether they execute:
//
//	hook := hooks.NewHook("debug-only", hooks.HookTypeAfterBuild, handler)
//	hook.Condition = func(event *hooks.Event) bool {
//	    debug, _ := event.GetData("debug")
//	    return debug == true
//	}
//
// # Execution Results
//
// Each hook execution returns an ExecutionResult with status and timing:
//
//	results := manager.TriggerSync(ctx, hooks.HookTypeBeforeBuild)
//	for _, result := range results {
//	    fmt.Printf("Hook: %s, Status: %s, Duration: %v\n",
//	        result.HookName, result.Status, result.Duration)
//	    if result.Error != nil {
//	        fmt.Printf("Error: %v\n", result.Error)
//	    }
//	}
//
// # Callbacks
//
// The manager supports lifecycle callbacks for monitoring:
//
//	manager.OnCreate(func(hook *hooks.Hook) {
//	    log.Printf("Hook registered: %s", hook.Name)
//	})
//
//	manager.OnExecute(func(event *hooks.Event, results []*hooks.ExecutionResult) {
//	    log.Printf("Executed %d hooks for %s", len(results), event.Type)
//	})
//
// # Executor Configuration
//
// The executor controls concurrency for async hooks:
//
//	executor := hooks.NewExecutorWithLimit(5) // Max 5 concurrent async hooks
//	manager := hooks.NewManagerWithExecutor(executor)
//
//	// Configure result retention
//	executor.SetMaxResults(1000)
//
//	// Get execution statistics
//	stats := executor.GetStatistics()
//	fmt.Printf("Total: %d, Success Rate: %.1f%%\n",
//	    stats.TotalExecutions, stats.SuccessRate*100)
//
// # Thread Safety
//
// All Manager and Executor operations are thread-safe and can be called
// concurrently from multiple goroutines.
package hooks
