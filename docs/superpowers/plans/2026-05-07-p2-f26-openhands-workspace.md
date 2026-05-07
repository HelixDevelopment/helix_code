# P2-F26 — Openhands Workspace + Task Planner + Step Executor

> **For agentic workers:** Use superpowers:subagent-driven-development.

> **Programme position:** F26 is the **sixth** Phase 2 feature (after F21-F25).

**Goal:** Ship container-based workspace management, a plan-tree-aware task planner, and a sequential step executor. Three new packages: `internal/workspace/`, `internal/planner/`. Reuses F25 plan trees and the Containers submodule.

**Spec:** `docs/superpowers/specs/2026-05-07-p2-f26-openhands-workspace-design.md`
**Q1-Q5:** Openhands / Core / Container-based / F25 plan tree + executor / Tools+slash+cobra

**Zero new external deps** — `digital.vasic.containers` already a submodule, `google/uuid` already direct.

---

## Task list

- [ ] P2-F26-T01 — bootstrap F26 evidence + advance PROGRESS + CONTINUATION
- [ ] P2-F26-T02 — `internal/workspace/types.go` + `manager.go`: Workspace, WorkspaceStatus, Manager (TDD)
- [ ] P2-F26-T03 — `internal/workspace/workspace_tools.go`: workspace_create/list/cleanup (TDD)
- [ ] P2-F26-T04 — `internal/planner/types.go` + `executor.go`: TaskStep, TaskPlan, StepExecutor (TDD)
- [ ] P2-F26-T05 — `internal/planner/planner_tools.go`: task_plan, task_step (TDD)
- [ ] P2-F26-T06 — `/openhands` slash + cobra subcommands (TDD)
- [ ] P2-F26-T07 — main.go wiring + integration tests
- [ ] P2-F26-T08 — Challenge harness 5 phases + close-out + push 4 remotes

---

## Task 1: Bootstrap F26

Append F26 header to `07_phase_2_evidence.md`. Update PROGRESS.md: "F25 COMPLETE; F26 in flight". Update CONTINUATION.md: F26 mid-flight section.

---

## Task 2: workspace/types.go + manager.go

`internal/workspace/types.go`:
- `Workspace` struct (ID, Name, ContainerID, Image, ProjectDir, Status, CreatedAt, TTL)
- `WorkspaceStatus` enum (Creating, Running, Stopped, Error)
- Sentinels: `ErrContainerRuntimeNotFound`, `ErrWorkspaceNotFound`, `ErrImagePullFailed`
- Constants: `DefaultImage = "alpine:latest"`, `DefaultTTL = 30 * time.Minute`

`internal/workspace/manager.go`:
- `WorkspaceManager` with `ContainersOrchestrator` interface for testability
- Methods: `CreateWorkspace(ctx, name, image, projectDir) (Workspace, error)`, `ListWorkspaces() ([]Workspace, error)`, `GetWorkspace(id string) (Workspace, error)`, `CleanupWorkspace(ctx, id string) error`
- Container lifecycle: pull image → create container → start container
- In-memory state tracking with `sync.RWMutex`
- Container runtime detection: docker → podman → fallback error

Tests: real Docker/Podman gated (SKIP-OK if neither available); mock orchestrator for unit tests.

---

## Task 3: workspace_tools.go

Three tools: `workspace_create`, `workspace_list`, `workspace_cleanup`.
All implement `tools.Tool`. Category: `workspace`.
RequiresApproval: create/cleanup → `LevelEdit`; list → `LevelReadOnly`.

---

## Task 4: planner/types.go + executor.go

`internal/planner/types.go`:
- `TaskStep` with StepType, Command, Prompt, Timeout, MaxRetries, Status, Output, Error
- `TaskPlan` with ID, Name, WorkspaceID, Steps, Status
- `StepExecutor` interface: `ExecuteStep(ctx, step, workspace) error`

`internal/planner/executor.go`:
- `SequentialExecutor` implementing StepExecutor
- ShellStep dispatch via `exec.CommandContext` in workspace container
- LLMStep dispatch via `llm.Provider.Generate`
- Retry logic: max retries with exponential backoff
- Status tracking: Pending → Running → Completed/Failed

Tests with mock executor and real shell executor.

---

## Task 5: planner_tools.go

Two tools: `task_plan` (creates a TaskPlan from an F25 PlanTree), `task_step` (executes a single step).
Category: `planner`. RequiresApproval: both `LevelEdit`.

---

## Task 6: /openhands slash + cobra

`/openhands` slash: status (list workspaces), create <name>, cleanup <id>
`helixcode workspace {create,list,cleanup}` cobra subcommands

---

## Task 7: main.go wiring

Construct WorkspaceManager → Planner → Register tools → Register slash → Register cobra subcommands.

---

## Task 8: Challenge harness

5 phases:
- A: Create workspace → verify container running → cleanup → verify stopped
- B: Create plan with F25 tree → execute shell step → verify output
- C: Step retry on failure → verify retry count + eventual success
- D: LLM step execution → verify response non-empty
- E: Workspace list → verify count after create/cleanup

---

*Plan written. Execute via TDD starting with T01.*
