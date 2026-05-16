# Agent Package

The `agent` package provides multi-agent orchestration and coordination for the HelixCode platform.

## Overview

This package handles:
- Agent creation and lifecycle
- Multi-agent coordination
- Task delegation between agents
- Agent communication
- Specialized agent types

## Key Types

### Agent

```go
type Agent struct {
    ID           string
    Type         AgentType
    Name         string
    Description  string
    Capabilities []string
    Status       Status
    LLMProvider  llm.Provider
}
```

### AgentType

```go
type AgentType string

const (
    TypePlanner    AgentType = "planner"
    TypeBuilder    AgentType = "builder"
    TypeTester     AgentType = "tester"
    TypeReviewer   AgentType = "reviewer"
    TypeDebugger   AgentType = "debugger"
    TypeDocumentor AgentType = "documentor"
)
```

### Orchestrator

```go
type Orchestrator struct {
    agents      map[string]*Agent
    coordinator *Coordinator
    config      *Config
}
```

## Usage

### Creating the Orchestrator

```go
import "dev.helix.code/internal/agent"

orchestrator := agent.NewOrchestrator(config, llmProvider)
```

### Creating Agents

```go
// Create planner agent
planner := &agent.Agent{
    Type:        agent.TypePlanner,
    Name:        "PlannerAgent",
    Description: "Analyzes requirements and creates implementation plans",
    Capabilities: []string{"planning", "analysis", "decomposition"},
}

err := orchestrator.RegisterAgent(ctx, planner)
```

### Task Delegation

```go
// Create task for agent
task := &agent.Task{
    Type:        agent.TaskTypePlanning,
    Description: "Analyze project requirements",
    Input:       requirements,
}

// Assign to best-suited agent
result, err := orchestrator.Delegate(ctx, task)

// Assign to specific agent
result, err := orchestrator.DelegateToAgent(ctx, planner.ID, task)
```

### Multi-Agent Workflows

```go
// Create workflow with multiple agents
workflow := &agent.Workflow{
    Steps: []*agent.WorkflowStep{
        {AgentType: agent.TypePlanner, Action: "analyze"},
        {AgentType: agent.TypeBuilder, Action: "implement"},
        {AgentType: agent.TypeTester, Action: "test"},
        {AgentType: agent.TypeReviewer, Action: "review"},
    },
}

result, err := orchestrator.ExecuteWorkflow(ctx, workflow, input)
```

### Agent Communication

```go
// Send message between agents
msg := &agent.Message{
    From:    plannerID,
    To:      builderID,
    Type:    agent.MessageTypeTask,
    Content: taskDetails,
}

err := orchestrator.SendMessage(ctx, msg)

// Broadcast to all agents
err := orchestrator.Broadcast(ctx, &agent.Message{
    Type:    agent.MessageTypeUpdate,
    Content: statusUpdate,
})
```

## Agent Types

### Planner Agent

Analyzes requirements and creates implementation plans:

```go
planner := agent.NewPlannerAgent(llmProvider, &agent.PlannerConfig{
    MaxSteps:     10,
    DetailLevel:  "high",
})
```

### Builder Agent

Implements code based on plans:

```go
builder := agent.NewBuilderAgent(llmProvider, &agent.BuilderConfig{
    Language:    "go",
    Style:       "idiomatic",
    TestsEnabled: true,
})
```

### Tester Agent

Writes and runs tests:

```go
tester := agent.NewTesterAgent(llmProvider, &agent.TesterConfig{
    Framework:     "testing",
    Coverage:      80.0,
    TestTypes:     []string{"unit", "integration"},
})
```

### Reviewer Agent

Reviews code for quality and issues:

```go
reviewer := agent.NewReviewerAgent(llmProvider, &agent.ReviewerConfig{
    CheckSecurity: true,
    CheckPerformance: true,
    StyleGuide:   "uber-go",
})
```

## Configuration

```yaml
agent:
  max_agents: 10
  default_llm: "gpt-4"
  coordination:
    strategy: "round-robin"
    timeout: 5m
  agents:
    planner:
      enabled: true
      max_concurrent: 2
    builder:
      enabled: true
      max_concurrent: 4
    tester:
      enabled: true
      max_concurrent: 2
```

## Coordination Strategies

- **round-robin**: Distribute tasks evenly
- **capability-based**: Match tasks to agent capabilities
- **load-based**: Assign to least busy agent
- **priority-based**: Honor task priority

## Testing

```bash
go test -v ./internal/agent/...
```

## Notes

- Agents can be specialized for specific tasks
- Use coordination strategies for optimal task distribution
- Monitor agent load for scaling decisions
- Implement fallback for agent failures
