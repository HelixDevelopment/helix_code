# Phase 2 / Feature 26 — Openhands Workspace + Task Planner + Step Executor

**Date:** 2026-05-07
**Status:** Approved (auto-approved per programme cadence)
**Programme:** CLI-Agent Fusion — Phase 2 port (openhands — workspace, planner, executor)

> **Programme position:** F26 is the **sixth** Phase 2 feature (F21-F25 shipped before it). T01 appends an F26 evidence section to `docs/improvements/07_phase_2_evidence.md`.

---

## 1. Goal

Ship a real, end-to-end **workspace + task planner + step executor** subsystem for the HelixCode CLI agent, modelled on Openhands' agent orchestration architecture (`cli_agents/openhands/`). Three components: (1) container-based per-task workspaces managed through the Containers submodule (`digital.vasic.containers`), (2) a sequential step executor that walks F25 plan trees and dispatches shell/LLM steps, and (3) a task planner that bridges F25's structured plan trees with Openhands-style execution pipelines. Tools surface: `workspace_create`, `workspace_list`, `workspace_cleanup`, `task_plan`, `task_step`. Slash: `/openhands`. Cobra: `helixcode workspace {create,list,cleanup}`.

---

## 2. Architecture

```
                 Agent (LLM loop)
                      │
         ┌────────────┼──────────────┐
         ▼            ▼              ▼
    Workspace     Task Planner    Step Executor
    Manager       (F25 plan       (sequential
    (container    tree adapter)   shell/LLM
     lifecycle)                    dispatch)
         │            │              │
         ▼            ▼              ▼
    Containers    plantree        llm.Provider
    (Docker/      PlanTree/       workflow.
     Podman)      PlanNode        Executor
```

### 2.1 Workspace Manager

- **Package:** `internal/workspace/` (NEW — avoids collision with `internal/tools/worktree/` from F04)
- Container-based per-task workspaces using `digital.vasic.containers` orchestrator
- Lifecycle: CreateWorkspace → RunTask → CleanupWorkspace
- Each workspace is a Docker/Podman container with:
  - Mounted project directory (read-write)
  - Isolated network (default deny; opt-in for web tasks)
  - Resource limits (CPU/memory caps)
  - Auto-cleanup TTL (idle timeout)

### 2.2 Task Planner

- **Package:** `internal/planner/` (NEW)
- Bridges F25 plan trees with execution: converts a PlanNode to a TaskStep chain
- Validates dependencies between steps
- Tracks execution state (Pending → Running → Completed/Failed)
- Reports progress back to the plan tree (updates PlanNode.Status)

### 2.3 Step Executor

- **Package:** `internal/planner/executor.go` (within planner package)
- Sequential step execution with retry
- Two executor types: ShellStep (real shell execution via F06 executor) and LLMStep (LLM provider call)
- Each step has: timeout, retry count, action type, parameters
- Status propagation to plan tree nodes

---

## 3. Data Model

### 3.1 Workspace

```go
type Workspace struct {
    ID          string    // UUID
    Name        string    // human-readable
    ContainerID string    // Docker/Podman container ID
    Image       string    // container image (default: alpine:latest)
    ProjectDir  string    // host path mounted into container
    Status      WorkspaceStatus  // Creating/Running/Stopped/Error
    CreatedAt   time.Time
    TTL         time.Duration    // idle timeout before auto-cleanup
}
```

### 3.2 TaskStep

```go
type StepType int
const (
    StepShell StepType = iota
    StepLLM
)

type TaskStep struct {
    ID          string
    PlanNodeID  string            // backlink to F25 PlanNode
    Type        StepType
    Command     string            // for StepShell
    Prompt      string            // for StepLLM
    Timeout     time.Duration
    MaxRetries  int
    Status      StepStatus        // Pending/Running/Completed/Failed
    Output      string
    Error       string
    StartedAt   time.Time
    CompletedAt time.Time
}
```

### 3.3 TaskPlan

```go
type TaskPlan struct {
    ID          string
    Name        string
    WorkspaceID string
    Steps       []TaskStep
    Status      PlanStatus
    PlanTree    *plantree.PlanTree  // backlink to F25
}
```

---

## 4. Components

### 4.1 New files

| File | Purpose |
|------|---------|
| `internal/workspace/types.go` | Workspace, WorkspaceStatus, sentinels |
| `internal/workspace/manager.go` | Container lifecycle via Containers orchestrator |
| `internal/workspace/workspace_tools.go` | workspace_create/list/cleanup tools.Tool impls |
| `internal/planner/types.go` | TaskStep, TaskPlan, StepType, StepStatus |
| `internal/planner/executor.go` | Sequential step executor with retry |
| `internal/planner/planner_tools.go` | task_plan, task_step tools.Tool impls |
| `internal/commands/openhands_command.go` | /openhands slash |
| `cmd/cli/workspace_cmd.go` | Cobra subcommands |

### 4.2 Modified files

| File | Change |
|------|--------|
| `cmd/cli/main.go` | Wire workspace manager + planner + tools + slash |
| `internal/tools/registry.go` | Register CategoryWorkspace |
| `internal/commands/builtin/register.go` | Register /openhands |

---

## 5. Anti-Bluff Hot Zone

Five critical degenerate patterns:

1. **Workspace created but container not running** — `docker ps` after CreateWorkspace MUST show container
2. **Step executor claims completed but command never ran** — step.Output MUST contain real command output
3. **Task plan references workspace that doesn't exist** — Validate workspace ID before starting plan
4. **Cleanup claims success but container still running** — `docker ps` after CleanupWorkspace MUST NOT show container
5. **Executor returns success on failed step** — StepStatus MUST match exit code / LLM error

---

## 6. Task Breakdown

| # | Task | Description |
|---|------|------------|
| T01 | Bootstrap | F26 evidence + advance PROGRESS + CONTINUATION |
| T02 | workspace/types.go + manager.go | Workspace + Manager (container lifecycle) (TDD) |
| T03 | workspace/workspace_tools.go | workspace_create/list/cleanup tools (TDD) |
| T04 | planner/types.go + executor.go | TaskStep + TaskPlan + StepExecutor (TDD) |
| T05 | planner/planner_tools.go | task_plan, task_step tools (TDD) |
| T06 | /openhands slash + Cobra subcommands | (TDD) |
| T07 | main.go wiring | Register tools + slash + cobra |
| T08 | Challenge harness + close-out | 5 phases + push 4 remotes |

---

*F26 spec — Openhands workspace + task planner + step executor.*
