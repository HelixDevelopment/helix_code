// Package agent provides multi-agent orchestration and coordination for AI-powered
// development workflows in HelixCode.
//
// # Overview
//
// The agent package implements a flexible multi-agent system where specialized agents
// collaborate to complete development tasks. It provides agent lifecycle management,
// task execution, workflow orchestration, and resilience patterns.
//
// # Architecture
//
// The package is organized around several core components:
//
//   - Agent interface: Defines the contract for all agent implementations
//   - BaseAgent: Provides a reusable foundation for concrete agent types
//   - Coordinator: Orchestrates multiple agents and manages task distribution
//   - WorkflowExecutor: Executes multi-step workflows with dependency resolution
//   - Resilience patterns: Circuit breakers and retry policies for fault tolerance
//
// # Agent Types
//
// The package supports several specialized agent types:
//
//   - Planning agents: Break down high-level goals into actionable tasks
//   - Coding agents: Generate and modify code based on specifications
//   - Testing agents: Create and execute tests
//   - Debugging agents: Analyze and fix issues in code
//   - Review agents: Perform code review and quality analysis
//   - Refactoring agents: Improve code structure and maintainability
//   - Documentation agents: Generate and maintain documentation
//
// # Basic Usage
//
// Creating and registering an agent:
//
//	// Create a coordinator with default configuration
//	coord := agent.NewCoordinator(nil)
//
//	// Create an agent configuration
//	config := &agent.AgentConfig{
//	    ID:           "coding-1",
//	    Type:         agent.AgentTypeCoding,
//	    Name:         "Go Coder",
//	    Model:        "llama-3.2",
//	    Capabilities: []agent.Capability{agent.CapabilityCodeGeneration},
//	}
//
//	// Create and register the agent
//	coder := agent.NewBaseAgentFromConfig(config)
//	coord.RegisterAgent(coder)
//
// # Task Execution
//
// Submitting and executing tasks:
//
//	// Create a task
//	t := task.NewTask(
//	    task.TaskTypeBuilding,
//	    "implement-auth",
//	    "Implement authentication module",
//	    task.PriorityHigh,
//	)
//
//	// Submit and execute
//	coord.SubmitTask(ctx, t)
//	result, err := coord.ExecuteTask(ctx, t.ID)
//
// # Workflow Execution
//
// Creating and executing multi-step workflows:
//
//	// Create a workflow
//	workflow := agent.NewWorkflow("feature-impl", "Implement new feature")
//
//	// Add steps with dependencies
//	workflow.AddStep(&agent.WorkflowStep{
//	    ID:        "plan",
//	    Name:      "Planning",
//	    AgentType: agent.AgentTypePlanning,
//	})
//	workflow.AddStep(&agent.WorkflowStep{
//	    ID:        "code",
//	    Name:      "Implementation",
//	    AgentType: agent.AgentTypeCoding,
//	    DependsOn: []string{"plan"},
//	})
//
//	// Execute workflow
//	err := coord.ExecuteWorkflow(ctx, workflow)
//
// # Agent Collaboration
//
// Agents can collaborate on complex tasks:
//
//	result, err := agent.Collaborate(ctx, []Agent{planner, coder, reviewer}, task)
//	// result.Consensus contains the agreed-upon solution
//	// result.Conflicts lists any disagreements that occurred
//
// # Resilience Patterns
//
// The package provides built-in resilience through circuit breakers and retry policies:
//
//	// Coordinator with resilience enabled (default)
//	coord := agent.NewCoordinator(&agent.CoordinatorConfig{
//	    EnableResilience:      true,
//	    FailureThreshold:      5,
//	    SuccessThreshold:      2,
//	    CircuitBreakerTimeout: 60 * time.Second,
//	})
//
//	// Circuit breaker states: Closed (normal), Open (blocking), HalfOpen (testing)
//	state := coord.GetCircuitBreakerState(agentID)
//
// # Thread Safety
//
// All public types in this package are safe for concurrent use. The Coordinator,
// AgentRegistry, and BaseAgent implementations use appropriate synchronization.
//
// # Agent Capabilities
//
// Agents declare capabilities that determine which tasks they can handle:
//
//	const (
//	    CapabilityPlanning            // Strategic planning and task decomposition
//	    CapabilityCodeGeneration      // Writing new code
//	    CapabilityCodeAnalysis        // Understanding existing code
//	    CapabilityTestGeneration      // Creating tests
//	    CapabilityTestExecution       // Running tests
//	    CapabilityDebugging           // Finding and fixing bugs
//	    CapabilityRefactoring         // Improving code structure
//	    CapabilityDocumentation       // Writing documentation
//	    CapabilityCodeReview          // Reviewing code quality
//	    CapabilitySecurityAudit       // Security analysis
//	    CapabilityPerformanceAnalysis // Performance optimization
//	)
//
// # Health Monitoring
//
// Monitor agent health and performance:
//
//	health := agent.Health()
//	// health.Healthy - overall health status
//	// health.TaskCount - tasks processed
//	// health.ErrorRate - error rate
//	// health.Uptime - time since start
//
// # Subpackages
//
// The agent package contains the following subpackages:
//
//   - types: Specialized agent implementations (CodingAgent, PlanningAgent, etc.)
//   - task: Task definition, lifecycle management, and result types
package agent
