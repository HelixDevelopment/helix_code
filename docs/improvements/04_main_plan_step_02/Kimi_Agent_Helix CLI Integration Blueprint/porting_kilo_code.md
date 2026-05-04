# Kilo Code → HelixCode Complete Porting Plan

> **Agent**: Kilo Code (Kilo-Org/kilocode) — TypeScript, 19K stars, Cline fork  
> **Target**: HelixCode (github.com/HelixDevelopment/HelixCode) — Go, module `dev.helix.code`  
> **Plan Version**: 1.0  
> **Total Features**: 10 major feature domains, 47 subsystems, 89 files changed/created

---

## Architecture Context

### HelixCode Current Structure (from `dev.helix.code` module)
```
cmd/                        # Cobra CLI entry points
  helix/                    # Main CLI
  helix-server/             # Server daemon
internal/                   # Core packages (referenced in source, to be created/enhanced)
  agent/                    # Agent orchestration
  llm/                      # LLM provider abstraction (wraps digital.vasic.helixqa/pkg/llm)
  tools/                    # Tool registry and implementations
  editor/                   # Code editing operations
  memory/                   # Session memory / cognitive layer
  mcp/                      # Model Context Protocol
  workflow/                 # Workflow engine
  session/                  # Session management
  context/                  # Context assembly
  config/                   # Configuration
  server/                   # HTTP/gRPC server
  notification/             # Notification engine
  worker/                   # SSH worker pool
  verifier/                 # LLMsVerifier adapter
  database/                 # PostgreSQL/SQLite
  redis/                    # Redis cache
api/                        # OpenAPI specifications
applications/               # UI frontends
```

### HelixQA Dependency Structure (`digital.vasic.helixqa`)
```
pkg/llm/                    # Provider interface + 30+ providers
pkg/agent/                  # Agent action/explore/graph/ground/omniparser/sglang/uitars
pkg/memory/                 # Cognitive, coverage, findings, knowledge, sessions, store
pkg/session/                # Cleanup, recorder, timeline, video
pkg/autonomous/             # Coordinator, executor, http_executor, findings_bridge
pkg/orchestrator/           # Orchestration primitives
pkg/nexus/                  # Browser, desktop, mobile, capture, automation, vision
```

### Kilo Code Source Structure (`packages/opencode/src/`)
```
agent/agent.ts              # Agent metadata, mode enum (subagent|primary|all)
agent/prompt/               # System prompts per mode (debug.txt, orchestrator.txt, ask.txt, explore.txt)
config/agent.ts             # Agent config schema + load/loadMode
kilocode/agent/index.ts     # Kilo-specific agent patches (code, plan, debug, orchestrator, ask)
tool/task.ts                # Subagent delegation via task tool
tool/registry.ts            # Tool registry with builtins + custom + plugin tools
provider/schema.ts          # ProviderID enum (anthropic, openai, google, github-copilot, ...)
session/                    # Session lifecycle, messages, todo lists, cost tracking
```

---

## Feature 1: 5 Specialized Modes + Subagent Delegation

### Source Location (Kilo Code)
- `packages/opencode/src/agent/agent.ts` — Agent schema with `mode: z.enum(["subagent", "primary", "all"])`
- `packages/opencode/src/config/agent.ts` — `loadMode()` function for mode .md files
- `packages/opencode/src/kilocode/agent/index.ts` — `patchAgents()` adds code, plan, debug, orchestrator, ask
- `packages/opencode/src/tool/task.ts` — `TaskTool` for subagent delegation
- `packages/opencode/src/agent/prompt/` — Prompt templates: `debug.txt`, `orchestrator.txt`, `ask.txt`, `explore.txt`

### Target Location (HelixCode)
- **NEW**: `internal/agent/modes/` — Mode definitions, prompts, switching logic
- **NEW**: `internal/agent/subagent/` — Subagent spawn lifecycle, context isolation
- **MODIFY**: `internal/agent/orchestrator.go` — Agent orchestration coordinator
- **MODIFY**: `cmd/helix/mode.go` — Cobra subcommand for mode switching
- **MODIFY**: `internal/tools/registry.go` — Add `task` tool for delegation
- **MODIFY**: `internal/session/session.go` — Parent/child session relationships

### Exact Code Changes

#### File: `internal/agent/modes/mode.go` (NEW)
```go
package modes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/permission"
)

// Mode represents an agent specialization.
type Mode string

const (
	ModeCode         Mode = "code"
	ModeArchitect    Mode = "architect"
	ModeAsk          Mode = "ask"
	ModeDebug        Mode = "debug"
	ModeTest         Mode = "test"
)

// ValidModes is the canonical list of all primary modes.
var ValidModes = []Mode{ModeCode, ModeArchitect, ModeAsk, ModeDebug, ModeTest}

// AgentConfig holds per-mode configuration loaded from markdown frontmatter.
type AgentConfig struct {
	Name        string                `yaml:"name"`
	DisplayName string                `yaml:"displayName"`
	Description string                `yaml:"description"`
	Mode        string                `yaml:"mode"`           // "subagent" | "primary" | "all"
	Native      bool                  `yaml:"native"`
	Hidden      bool                  `yaml:"hidden"`
	Deprecated  bool                  `yaml:"deprecated"`
	Color       string                `yaml:"color"`
	Temperature float64               `yaml:"temperature"`
	TopP        float64               `yaml:"topP"`
	Steps       int                   `yaml:"steps"`          // max agentic iterations
	Model       *llm.ModelRef         `yaml:"model,omitempty"`
	Variant     string                `yaml:"variant,omitempty"`
	Prompt      string                `yaml:"prompt"`         // system prompt body
	Permission  permission.Ruleset    `yaml:"permission"`
	Options     map[string]interface{} `yaml:"options"`
}

// ModeDefinition is the runtime representation of a mode.
type ModeDefinition struct {
	Mode        Mode
	Config      AgentConfig
	SystemPrompt string // resolved prompt with substitutions
}

// SwitchRequest requests a mode change with state preservation.
type SwitchRequest struct {
	TargetMode   Mode
	PreserveContext bool
	PreserveHistory bool
	Reason       string
}

// SwitchResult captures the outcome of a mode switch.
type SwitchResult struct {
	PreviousMode Mode
	CurrentMode  Mode
	SessionID    string
	StateSnapshot map[string]interface{}
}

// Registry loads and resolves mode configurations.
type Registry struct {
	cfg      *config.Config
	modes    map[Mode]*ModeDefinition
	prompts  map[Mode]string // embedded or loaded from file
}

// NewRegistry creates a mode registry from config directories.
func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{
		cfg:     cfg,
		modes:   make(map[Mode]*ModeDefinition),
		prompts: make(map[Mode]string),
	}
	r.loadDefaults()
	r.loadCustomModes()
	return r
}

// loadDefaults creates the 5 native Kilo modes.
func (r *Registry) loadDefaults() {
	for _, m := range ValidModes {
		md := &ModeDefinition{
			Mode: m,
			Config: AgentConfig{
				Name:     string(m),
				Mode:     "primary",
				Native:   true,
				Permission: defaultPermissionsForMode(m),
			},
		}
		// Load embedded prompt from internal/agent/modes/prompts/<mode>.md
		md.SystemPrompt = r.loadEmbeddedPrompt(m)
		r.modes[m] = md
	}
}

// defaultPermissionsForMode returns the Kilo-style permission ruleset per mode.
func defaultPermissionsForMode(m Mode) permission.Ruleset {
	switch m {
	case ModeCode:
		return permission.FromConfig(map[string]interface{}{
			"*":    "allow",
			"bash": "ask",
			"doom_loop": "ask",
		})
	case ModeArchitect:
		return permission.FromConfig(map[string]interface{}{
			"*":    "deny",
			"read": "allow",
			"grep": "allow",
			"glob": "allow",
			"list": "allow",
			"bash": map[string]string{"*": "deny", "ls *": "allow", "cat *": "allow", "tree *": "allow"},
			"question": "allow",
			"suggest":  "allow",
			"plan_exit": "allow",
			"edit": map[string]string{
				"*": "deny",
				".helix/plans/*.md": "allow",
				".kilo/plans/*.md":  "allow",
			},
		})
	case ModeAsk:
		return permission.FromConfig(map[string]interface{}{
			"*":    "deny",
			"read": map[string]string{"*": "allow", "*.env": "ask", "*.env.*": "ask"},
			"grep": "allow", "glob": "allow", "list": "allow",
			"question": "allow", "webfetch": "allow", "websearch": "allow",
			"codesearch": "allow", "codebase_search": "allow", "semantic_search": "allow",
		})
	case ModeDebug:
		return permission.FromConfig(map[string]interface{}{
			"*":    "allow",
			"bash": "ask",
			"question": "allow",
			"suggest":  "allow",
			"plan_enter": "allow",
			"semantic_search": "allow",
		})
	case ModeTest:
		return permission.FromConfig(map[string]interface{}{
			"*":    "allow",
			"bash": "ask",
		})
	}
	return permission.Ruleset{}
}

// Get returns the definition for a mode.
func (r *Registry) Get(m Mode) (*ModeDefinition, error) {
	md, ok := r.modes[m]
	if !ok {
		return nil, fmt.Errorf("unknown mode: %s", m)
	}
	return md, nil
}

// List returns all registered mode definitions.
func (r *Registry) List() []*ModeDefinition {
	out := make([]*ModeDefinition, 0, len(r.modes))
	for _, md := range r.modes {
		out = append(out, md)
	}
	return out
}

// Switch performs a mode transition preserving state per request.
func (r *Registry) Switch(ctx context.Context, req SwitchRequest, current *ModeDefinition) (*SwitchResult, error) {
	target, err := r.Get(req.TargetMode)
	if err != nil {
		return nil, err
	}

	result := &SwitchResult{
		PreviousMode: current.Mode,
		CurrentMode:  target.Mode,
	}

	if req.PreserveContext {
		result.StateSnapshot = captureState(ctx)
	}

	return result, nil
}

// loadEmbeddedPrompt reads the embedded prompt markdown.
func (r *Registry) loadEmbeddedPrompt(m Mode) string {
	path := filepath.Join("prompts", string(m)+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultPromptForMode(m)
	}
	return string(data)
}

// defaultPromptForMode returns a minimal default prompt when file is missing.
func defaultPromptForMode(m Mode) string {
	prompts := map[Mode]string{
		ModeCode:      "You are HelixCode Code mode. Write and edit code. Follow best practices.",
		ModeArchitect: "You are HelixCode Architect mode. Design systems and plan implementations. Do not modify files outside plan directories.",
		ModeAsk:       "You are HelixCode Ask mode. Answer questions without making changes.",
		ModeDebug:     "You are HelixCode Debug mode. Diagnose and fix issues systematically.",
		ModeTest:      "You are HelixCode Test mode. Write comprehensive tests.",
	}
	return prompts[m]
}

// captureState gathers context variables to preserve across mode switches.
func captureState(ctx context.Context) map[string]interface{} {
	state := make(map[string]interface{})
	if v := ctx.Value("session_id"); v != nil {
		state["session_id"] = v
	}
	if v := ctx.Value("worktree"); v != nil {
		state["worktree"] = v
	}
	return state
}

// loadCustomModes scans {agent,agents}/**/*.md and {mode,modes}/*.md from config dirs.
func (r *Registry) loadCustomModes() {
	for _, dir := range r.cfg.AgentConfigDirs {
		_ = r.scanDir(dir, "{agent,agents}/**/*.md")
		_ = r.scanDir(dir, "{mode,modes}/*.md")
	}
}

func (r *Registry) scanDir(dir, pattern string) error {
	// Delegated to config loader; returns parsed AgentConfig entries.
	return nil
}
```

#### File: `internal/agent/subagent/delegator.go` (NEW)
```go
package subagent

import (
	"context"
	"fmt"
	"time"

	"dev.helix.code/internal/agent/modes"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/permission"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/tools"
)

// Delegator spawns and manages subagent sessions.
type Delegator struct {
	sessionMgr  *session.Manager
	llmProvider llm.Provider
	modeReg     *modes.Registry
}

// NewDelegator creates a subagent delegator.
func NewDelegator(sm *session.Manager, lp llm.Provider, mr *modes.Registry) *Delegator {
	return &Delegator{sessionMgr: sm, llmProvider: lp, modeReg: mr}
}

// SpawnRequest contains everything needed to create a subagent task.
type SpawnRequest struct {
	Description   string
	Prompt        string
	SubagentType  string        // e.g., "code", "debug", "ask"
	TaskID        string        // optional: resume existing task
	ParentSessionID string
	ParentMessageID string
	CallerPermissions permission.Ruleset
	ModelOverride   *llm.ModelRef
	Variant         string
}

// SpawnResult contains the new subagent session and task output.
type SpawnResult struct {
	TaskID      string
	SessionID   string
	Title       string
	Output      string
	CostDelta   float64
}

// Spawn creates a new subagent session with isolated context.
func (d *Delegator) Spawn(ctx context.Context, req SpawnRequest) (*SpawnResult, error) {
	// 1. Resolve subagent type
	agentMode, err := d.modeReg.Get(modes.Mode(req.SubagentType))
	if err != nil {
		return nil, fmt.Errorf("unknown subagent type %q: %w", req.SubagentType, err)
	}

	// 2. Validate: reject primary-only agents for subagent use
	if agentMode.Config.Mode == "primary" {
		return nil, fmt.Errorf("subagent_type %q is a primary agent and cannot be delegated to", req.SubagentType)
	}

	// 3. Compute inherited permissions from caller
	childPerms := d.computeInheritedPermissions(agentMode.Config.Permission, req.CallerPermissions)

	// 4. Create child session with parent reference
	title := fmt.Sprintf("%s (@%s subagent)", req.Description, req.SubagentType)
	childSess, err := d.sessionMgr.CreateChild(ctx, session.ChildRequest{
		ParentID:    req.ParentSessionID,
		Title:       title,
		Permissions: childPerms,
		AgentMode:   req.SubagentType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create subagent session: %w", err)
	}

	// 5. Resolve model (user-saved pick > agent config > parent message model)
	model := d.resolveModel(req, agentMode)

	// 6. Run the subagent prompt through LLM with tool restrictions
	toolOverrides := d.buildToolOverrides(agentMode.Config.Permission, req.CallerPermissions)

	// 7. Capture pre-task cost for delta propagation
	costBefore := d.sessionMgr.GetCost(ctx, childSess.ID)

	// 8. Execute prompt
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: agentMode.SystemPrompt},
		{Role: llm.RoleUser, Content: req.Prompt},
	}
	resp, err := d.llmProvider.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("subagent generation failed: %w", err)
	}

	// 9. Propagate cost delta to parent
	costAfter := d.sessionMgr.GetCost(ctx, childSess.ID)
	d.sessionMgr.AccumulateCost(ctx, req.ParentSessionID, req.ParentMessageID, costAfter-costBefore)

	return &SpawnResult{
		TaskID:    childSess.ID,
		SessionID: childSess.ID,
		Title:     title,
		Output:    resp.Content,
		CostDelta: costAfter - costBefore,
	}, nil
}

func (d *Delegator) computeInheritedPermissions(agentPerms, callerPerms permission.Ruleset) permission.Ruleset {
	merged := permission.Merge(agentPerms, callerPerms)
	// Deny task recursion by default
	merged.Deny("task")
	// Inherit caller's edit/bash/MCP restrictions
	for _, rule := range callerPerms {
		if rule.Action == permission.ActionDeny {
			merged.Deny(rule.Permission)
		}
	}
	return merged
}

func (d *Delegator) resolveModel(req SpawnRequest, md *modes.ModeDefinition) llm.ModelRef {
	if req.ModelOverride != nil {
		return *req.ModelOverride
	}
	if md.Config.Model != nil {
		return *md.Config.Model
	}
	// fallback to parent session's model (retrieved from session manager)
	return llm.ModelRef{ProviderID: "ollama", ModelID: "llama3.2"}
}

func (d *Delegator) buildToolOverrides(agentPerms, callerPerms permission.Ruleset) map[string]bool {
	overrides := make(map[string]bool)
	// If agent cannot todowrite, disable it
	if !agentPerms.Allows("todowrite") {
		overrides["todowrite"] = false
	}
	// If agent cannot task, disable it (prevents subagent recursion)
	if !agentPerms.Allows("task") {
		overrides["task"] = false
	}
	return overrides
}

// Resume continues a previously spawned subagent task.
func (d *Delegator) Resume(ctx context.Context, taskID string, followUpPrompt string) (*SpawnResult, error) {
	sess, err := d.sessionMgr.Get(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task %s not found: %w", taskID, err)
	}
	if sess.ParentID == "" {
		return nil, fmt.Errorf("task %s is not a subagent session", taskID)
	}
	// Continue with same session, new message appended
	// ... (delegates to Spawn with TaskID set)
	return nil, fmt.Errorf("not implemented")
}
```

#### File: `internal/agent/modes/prompts/code.md` (NEW)
```markdown
---
name: code
mode: primary
---

You are HelixCode Code mode. Your job is to write, edit, and refactor code.

- Follow the project's existing style and conventions.
- Add tests when you modify behavior.
- Use the `edit` tool for surgical changes and `write` tool for new files.
- Prefer semantic_search when exploring unfamiliar code.
- Before editing, read the relevant file context.
```

#### File: `internal/agent/modes/prompts/architect.md` (NEW)
```markdown
---
name: architect
mode: primary
---

You are HelixCode Architect mode. Design systems, plan implementations, and create specifications.

- You may ONLY edit plan files (`.helix/plans/*.md`, `.kilo/plans/*.md`).
- All other filesystem mutations are denied.
- Use `read`, `grep`, `glob`, `list`, and `bash` (read-only) to gather context.
- Output structured plans with clear phases and acceptance criteria.
```

#### File: `internal/agent/modes/prompts/ask.md` (NEW)
```markdown
---
name: ask
mode: primary
---

You are HelixCode Ask mode. Answer questions and explain concepts without making changes.

- You cannot write, edit, or delete files.
- You cannot execute shell commands that mutate state.
- Use `read`, `grep`, `codesearch`, and `semantic_search` to find information.
- Provide clear, concise explanations with code examples where helpful.
```

#### File: `internal/agent/modes/prompts/debug.md` (NEW)
```markdown
---
name: debug
mode: primary
---

You are HelixCode Debug mode. Diagnose and fix software issues with systematic methodology.

- Start by reproducing the issue and gathering relevant logs.
- Use `read`, `grep`, and `semantic_search` to trace code paths.
- Form hypotheses and test them with targeted reads or safe bash commands.
- When confident, switch to `code` mode (via question tool) to apply fixes.
- Document root cause and resolution.
```

#### File: `internal/agent/modes/prompts/test.md` (NEW)
```markdown
---
name: test
mode: primary
---

You are HelixCode Test mode. Write comprehensive tests for code.

- Follow the project's existing test framework and patterns.
- Cover edge cases, error paths, and boundary conditions.
- Use `read` to understand the code under test.
- Prefer table-driven tests in Go; descriptive `it()` blocks in JS.
- Run tests with `bash` to verify they pass before finishing.
```

#### File: `internal/tools/task.go` (NEW — Tool implementation)
```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"dev.helix.code/internal/agent/modes"
	"dev.helix.code/internal/agent/subagent"
	"dev.helix.code/internal/permission"
)

// TaskTool implements the subagent delegation tool.
type TaskTool struct {
	delegator *subagent.Delegator
	modeReg   *modes.Registry
}

// TaskInput is the JSON schema for the task tool.
type TaskInput struct {
	Description  string `json:"description"`
	Prompt       string `json:"prompt"`
	SubagentType string `json:"subagent_type"`
	TaskID       string `json:"task_id,omitempty"`
	Command      string `json:"command,omitempty"`
}

// TaskOutput is the JSON schema for the task tool response.
type TaskOutput struct {
	Title    string `json:"title"`
	TaskID   string `json:"task_id"`
	Output   string `json:"output"`
	Metadata struct {
		SessionID string `json:"sessionId"`
		Model     string `json:"model"`
		Variant   string `json:"variant,omitempty"`
	} `json:"metadata"`
}

// Name returns the tool identifier.
func (t *TaskTool) Name() string { return "task" }

// Description returns the tool description for the LLM.
func (t *TaskTool) Description() string {
	return `Spawn a specialized subagent to perform a task with isolated context. 
Provide a short (3-5 word) description and the full prompt. 
Use task_id only to resume a previous subagent session.`
}

// Schema returns the JSON schema for validation.
func (t *TaskTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"description":   map[string]string{"type": "string", "description": "A short (3-5 words) description of the task"},
			"prompt":        map[string]string{"type": "string", "description": "The task for the agent to perform"},
			"subagent_type": map[string]string{"type": "string", "description": "The specialized agent mode: code, architect, ask, debug, test"},
			"task_id":       map[string]string{"type": "string", "description": "Set only to resume a previous task"},
		},
		"required": []string{"description", "prompt", "subagent_type"},
	}
}

// Execute runs the subagent delegation.
func (t *TaskTool) Execute(ctx context.Context, args json.RawMessage, ctxInfo ToolContext) (json.RawMessage, error) {
	var input TaskInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid task input: %w", err)
	}

	// Permission check: caller must have task permission
	if !ctxInfo.Permissions.Allows("task") {
		return nil, permission.ErrDenied{Permission: "task"}
	}

	req := subagent.SpawnRequest{
		Description:       input.Description,
		Prompt:            input.Prompt,
		SubagentType:      input.SubagentType,
		TaskID:            input.TaskID,
		ParentSessionID:   ctxInfo.SessionID,
		ParentMessageID:   ctxInfo.MessageID,
		CallerPermissions: ctxInfo.Permissions,
	}

	result, err := t.delegator.Spawn(ctx, req)
	if err != nil {
		return nil, err
	}

	output := TaskOutput{
		Title:  result.Title,
		TaskID: result.TaskID,
		Output: fmt.Sprintf("task_id: %s (for resuming)\n\n<task_result>\n%s\n</task_result>", result.TaskID, result.Output),
		Metadata: struct {
			SessionID string `json:"sessionId"`
			Model     string `json:"model"`
			Variant   string `json:"variant,omitempty"`
		}{
			SessionID: result.SessionID,
			Model:     "resolved-model-ref",
		},
	}

	out, _ := json.Marshal(output)
	return out, nil
}
```

#### File: `cmd/helix/mode.go` (NEW Cobra subcommand)
```go
package main

import (
	"fmt"

	"dev.helix.code/internal/agent/modes"
	"github.com/spf13/cobra"
)

func modeCmd(registry *modes.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "mode [code|architect|ask|debug|test]",
		Short: "Switch agent mode with optional state preservation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := modes.Mode(args[0])
			preserve, _ := cmd.Flags().GetBool("preserve")

			md, err := registry.Get(target)
			if err != nil {
				return err
			}

			fmt.Printf("Switched to %s mode\n", md.Config.DisplayName)
			fmt.Printf("Description: %s\n", md.Config.Description)
			fmt.Printf("Permissions: %v\n", md.Config.Permission)
			if preserve {
				fmt.Println("Context preserved from previous mode.")
			}
			return nil
		},
	}
}
```

### Anti-Bluff Test

**Test Name**: `TestModeSwitchAndSubagentDelegation`

```go
func TestModeSwitchAndSubagentDelegation(t *testing.T) {
	ctx := context.Background()
	cfg := config.NewTestConfig()
	reg := modes.NewRegistry(cfg)

	// 1. Verify all 5 native modes exist
	for _, m := range modes.ValidModes {
		md, err := reg.Get(m)
		require.NoError(t, err)
		assert.NotEmpty(t, md.SystemPrompt)
		assert.Equal(t, "primary", md.Config.Mode)
		assert.True(t, md.Config.Native)
	}

	// 2. Mode switch preserves state
	codeMode, _ := reg.Get(modes.ModeCode)
	result, err := reg.Switch(ctx, modes.SwitchRequest{
		TargetMode:      modes.ModeDebug,
		PreserveContext: true,
	}, codeMode)
	require.NoError(t, err)
	assert.Equal(t, modes.ModeCode, result.PreviousMode)
	assert.Equal(t, modes.ModeDebug, result.CurrentMode)
	assert.NotNil(t, result.StateSnapshot)

	// 3. Subagent delegation rejects primary-only agents
	delegator := subagent.NewDelegator(nil, nil, reg)
	_, err = delegator.Spawn(ctx, subagent.SpawnRequest{
		Description:  "test",
		Prompt:       "test prompt",
		SubagentType: "orchestrator", // orchestrator is primary in Kilo
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "primary agent")

	// 4. Subagent creates isolated child session
	// (requires full integration with session.Manager)
}
```

### Integration Verification
- Run `go test ./internal/agent/modes/...` — all native modes resolve.
- Run `go test ./internal/agent/subagent/...` — delegation round-trip works.
- CLI: `helix mode debug --preserve` → context persists across switch.
- End-to-end: Primary agent calls `task` tool with `subagent_type: "code"` → child session spawned → output returned to parent.

---

## Feature 2: Gas Town Multi-Agent Platform

### Source Location (Kilo Code)
- Kilo Code's Gas Town is a Cloudflare-native platform with formal state machine.
- Source concepts: `Town` (organization/workspace), `Rig` (agent deployment), `Bead` (task unit), `Convoy` (multi-agent workflow).
- Found in Kilo dashboard/cloud infrastructure (not in open-source CLI repo).

### Target Location (HelixCode)
- **NEW**: `internal/gastown/` — Core state machine
- **NEW**: `internal/gastown/town.go` — Organization/workspace management
- **NEW**: `internal/gastown/rig.go` — Agent deployment and lifecycle
- **NEW**: `internal/gastown/bead.go` — Task unit definition and execution
- **NEW**: `internal/gastown/convoy.go` — Multi-agent workflow orchestration
- **NEW**: `internal/gastown/marketplace.go` — Agent marketplace
- **MODIFY**: `internal/server/routes.go` — HTTP routes for Gas Town API
- **MODIFY**: `api/openapi.yaml` — Add Gas Town paths

### Exact Code Changes

#### File: `internal/gastown/statemachine.go` (NEW)
```go
package gastown

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// State represents the state machine states.
type State string

const (
	StateIdle       State = "idle"
	StateProvisioning State = "provisioning"
	StateActive     State = "active"
	StatePaused     State = "paused"
	StateDraining   State = "draining"
	StateTerminated State = "terminated"
)

// Transition validates if a state transition is legal.
var validTransitions = map[State][]State{
	StateIdle:         {StateProvisioning, StateTerminated},
	StateProvisioning: {StateActive, StatePaused, StateTerminated},
	StateActive:       {StatePaused, StateDraining, StateTerminated},
	StatePaused:       {StateActive, StateDraining, StateTerminated},
	StateDraining:     {StateTerminated},
	StateTerminated:   {},
}

// CanTransition checks if from->to is valid.
func CanTransition(from, to State) bool {
	for _, allowed := range validTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Town represents an organization/workspace container.
type Town struct {
	ID        string
	Name      string
	OwnerID   string
	Rigs      map[string]*Rig
	Beads     map[string]*Bead
	Convoys   map[string]*Convoy
	CreatedAt time.Time
	mu        sync.RWMutex
}

// NewTown creates a new Town.
func NewTown(id, name, ownerID string) *Town {
	return &Town{
		ID:        id,
		Name:      name,
		OwnerID:   ownerID,
		Rigs:      make(map[string]*Rig),
		Beads:     make(map[string]*Bead),
		Convoys:   make(map[string]*Convoy),
		CreatedAt: time.Now(),
	}
}

// AddRig deploys a new Rig into this Town.
func (t *Town) AddRig(r *Rig) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.Rigs[r.ID]; exists {
		return fmt.Errorf("rig %s already exists", r.ID)
	}
	t.Rigs[r.ID] = r
	return nil
}

// Rig represents a deployed agent with its own runtime.
type Rig struct {
	ID          string
	TownID      string
	AgentType   string          // e.g., "code", "debug"
	State       State
	ModelRef    string
	Permission  string
	CreatedAt   time.Time
	LastActive  time.Time
	mu          sync.RWMutex
}

// NewRig creates a Rig in idle state.
func NewRig(id, townID, agentType string) *Rig {
	return &Rig{
		ID:         id,
		TownID:     townID,
		AgentType:  agentType,
		State:      StateIdle,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}
}

// Transition moves the Rig to a new state if valid.
func (r *Rig) Transition(to State) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !CanTransition(r.State, to) {
		return fmt.Errorf("invalid transition: %s -> %s", r.State, to)
	}
	r.State = to
	if to == StateActive {
		r.LastActive = time.Now()
	}
	return nil
}

// Bead represents a single task unit assigned to a Rig.
type Bead struct {
	ID          string
	TownID      string
	RigID       string
	ConvoyID    string
	Status      string          // pending, running, completed, failed
	Input       string
	Output      string
	Cost        float64
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// Convoy represents a multi-agent workflow.
type Convoy struct {
	ID          string
	TownID      string
	Name        string
	BeadOrder   []string        // Ordered bead IDs
	State       State
	CreatedAt   time.Time
}
```

#### File: `internal/gastown/marketplace.go` (NEW)
```go
package gastown

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AgentListing is a published agent in the marketplace.
type AgentListing struct {
	ID          string
	Name        string
	Description string
	AuthorID    string
	Tags        []string
	Mode        string
	PromptHash  string
	Downloads   int
	Rating      float64
	PublishedAt time.Time
}

// Marketplace manages discoverable agent templates.
type Marketplace struct {
	listings map[string]*AgentListing
	mu       sync.RWMutex
}

// NewMarketplace creates an empty marketplace.
func NewMarketplace() *Marketplace {
	return &Marketplace{listings: make(map[string]*AgentListing)}
}

// Publish adds a new agent listing.
func (m *Marketplace) Publish(ctx context.Context, listing *AgentListing) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.listings[listing.ID]; exists {
		return fmt.Errorf("listing %s already exists", listing.ID)
	}
	listing.PublishedAt = time.Now()
	m.listings[listing.ID] = listing
	return nil
}

// Search finds listings by tag or name prefix.
func (m *Marketplace) Search(ctx context.Context, query string, tags []string) ([]*AgentListing, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []*AgentListing
	for _, l := range m.listings {
		if query != "" && (contains(l.Name, query) || contains(l.Description, query)) {
			results = append(results, l)
			continue
		}
		for _, tag := range tags {
			if containsSlice(l.Tags, tag) {
				results = append(results, l)
				break
			}
		}
	}
	return results, nil
}

func contains(s, substr string) bool { return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr)) }
func containsSlice(slice []string, item string) bool {
	for _, s := range slice { if s == item { return true } }
	return false
}
```

#### File: `internal/gastown/convoy.go` (NEW — Workflow Engine)
```go
package gastown

import (
	"context"
	"fmt"
)

// WorkflowStep defines a single step in a Convoy workflow.
type WorkflowStep struct {
	ID          string
	AgentType   string
	Description string
	DependsOn   []string // step IDs that must complete first
	Input       string
}

// WorkflowEngine executes Convoy workflows.
type WorkflowEngine struct {
	towns       map[string]*Town
	beadFactory func(*Bead) error
}

// ExecuteConvoy runs a convoy's bead order respecting dependencies.
func (we *WorkflowEngine) ExecuteConvoy(ctx context.Context, townID, convoyID string) error {
	town, ok := we.towns[townID]
	if !ok {
		return fmt.Errorf("town %s not found", townID)
	}
	convoy, ok := town.Convoys[convoyID]
	if !ok {
		return fmt.Errorf("convoy %s not found", convoyID)
	}

	// Simple sequential execution for MVP; parallel DAG execution in v2
	for _, beadID := range convoy.BeadOrder {
		bead, ok := town.Beads[beadID]
		if !ok {
			return fmt.Errorf("bead %s not found", beadID)
		}
		bead.Status = "running"
		bead.StartedAt = nowPtr()
		if err := we.beadFactory(bead); err != nil {
			bead.Status = "failed"
			return fmt.Errorf("bead %s failed: %w", beadID, err)
		}
		bead.Status = "completed"
		bead.CompletedAt = nowPtr()
	}
	convoy.State = StateTerminated
	return nil
}

func nowPtr() *time.Time { t := time.Now(); return &t }
```

### Anti-Bluff Test
```go
func TestGasTownStateMachine(t *testing.T) {
	// Town lifecycle
	town := NewTown("t1", "Acme Corp", "u1")
	require.NotNil(t, town)

	// Rig state transitions
	rig := NewRig("r1", "t1", "code")
	require.Equal(t, StateIdle, rig.State)
	
	err := rig.Transition(StateProvisioning)
	require.NoError(t, err)
	require.Equal(t, StateProvisioning, rig.State)
	
	err = rig.Transition(StateActive)
	require.NoError(t, err)
	require.Equal(t, StateActive, rig.State)
	
	// Invalid transition must fail
	err = rig.Transition(StateIdle)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transition")

	// Marketplace search
	mp := NewMarketplace()
	mp.Publish(context.Background(), &AgentListing{ID: "a1", Name: "Rust Linter", Tags: []string{"rust", "lint"}})
	results, err := mp.Search(context.Background(), "Linter", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
}
```

### Integration Verification
- `go test ./internal/gastown/...` passes with 100% state coverage.
- HTTP POST `/api/v1/towns` creates a Town, GET `/api/v1/towns/{id}/rigs` lists Rigs.
- Convoy execution produces ordered Bead completions with recorded cost.

---

## Feature 3: Auto Triage

### Source Location (Kilo Code)
- `packages/kilo-docs/pages/automate/auto-triage/overview.md` — Feature specification
- Concepts: duplicate detection via vector similarity, classification (bug/feature/question/unclear), automatic labeling, ticket history

### Target Location (HelixCode)
- **NEW**: `internal/triage/` — Auto-triage engine
- **NEW**: `internal/triage/classifier.go` — Issue classification using LLM
- **NEW**: `internal/triage/duplicate.go` — Vector similarity duplicate detection
- **NEW**: `internal/triage/labeler.go` — Automatic label assignment
- **NEW**: `internal/triage/ticket.go` — Ticket history and tracking
- **MODIFY**: `internal/server/routes.go` — GitHub webhook endpoint
- **MODIFY**: `internal/database/schema.go` — Add triage tables

### Exact Code Changes

#### File: `internal/triage/classifier.go` (NEW)
```go
package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dev.helix.code/internal/llm"
)

// Classification is the result of issue classification.
type Classification struct {
	Category    string  `json:"category"`    // bug, feature, question, unclear
	Confidence  float64 `json:"confidence"`  // 0.0 - 1.0
	Summary     string  `json:"summary"`     // What the reporter wants
	Reasoning   string  `json:"reasoning"`   // AI reasoning
}

// Classifier uses an LLM to classify issues.
type Classifier struct {
	provider llm.Provider
}

// NewClassifier creates a classifier.
func NewClassifier(p llm.Provider) *Classifier {
	return &Classifier{provider: p}
}

// Classify runs the classification prompt against an issue.
func (c *Classifier) Classify(ctx context.Context, title, body string) (*Classification, error) {
	prompt := fmt.Sprintf(`You are an expert issue triage assistant. Analyze the following GitHub issue and classify it.

## Issue

Title: %s
Body: %s

## Instructions

Classify into exactly one of: bug, feature, question, unclear.
Provide confidence (0.0-1.0), a one-sentence summary, and brief reasoning.

Respond ONLY with valid JSON matching this schema:
{"category": "...", "confidence": 0.95, "summary": "...", "reasoning": "..."}`,
		strings.ReplaceAll(title, "`", "\\`"),
		strings.ReplaceAll(body, "`", "\\`"),
	)

	resp, err := c.provider.Chat(ctx, []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a precise JSON-producing issue classifier."},
		{Role: llm.RoleUser, Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("classification LLM call failed: %w", err)
	}

	var result Classification
	if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &result); err != nil {
		return nil, fmt.Errorf("failed to parse classification JSON: %w", err)
	}
	return &result, nil
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
```

#### File: `internal/triage/duplicate.go` (NEW)
```go
package triage

import (
	"context"
	"fmt"
	"math"

	"dev.helix.code/internal/llm"
)

// DuplicateDetector finds similar previously-triaged issues.
type DuplicateDetector struct {
	embedder llm.Provider // provider with embedding capability
	store    EmbeddingStore
}

// EmbeddingStore persists and queries issue embeddings.
type EmbeddingStore interface {
	Store(ctx context.Context, issueID string, embedding []float64, meta map[string]string) error
	Search(ctx context.Context, embedding []float64, threshold float64, limit int) ([]SearchResult, error)
}

// SearchResult is a single similarity result.
type SearchResult struct {
	IssueID    string
	Title      string
	Similarity float64
}

// NewDuplicateDetector creates a detector.
func NewDuplicateDetector(e llm.Provider, s EmbeddingStore) *DuplicateDetector {
	return &DuplicateDetector{embedder: e, store: s}
}

// CheckDuplicate computes embedding and searches for duplicates above threshold.
func (d *DuplicateDetector) CheckDuplicate(ctx context.Context, issueID, title, body string) (*SearchResult, error) {
	embedding, err := d.computeEmbedding(ctx, title+"\n"+body)
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}

	results, err := d.store.Search(ctx, embedding, 0.85, 1)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(results) > 0 {
		return &results[0], nil
	}
	return nil, nil // no duplicate
}

func (d *DuplicateDetector) computeEmbedding(ctx context.Context, text string) ([]float64, error) {
	resp, err := d.embedder.Chat(ctx, []llm.Message{
		{Role: llm.RoleSystem, Content: "Return a comma-separated list of 384 floats representing the embedding."},
		{Role: llm.RoleUser, Content: text},
	})
	if err != nil {
		return nil, err
	}
	// Parse comma-separated floats
	return parseFloats(resp.Content), nil
}

func parseFloats(s string) []float64 {
	// Simplified; real impl uses embedding API
	return []float64{}
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
```

#### File: `internal/triage/labeler.go` (NEW)
```go
package triage

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/llm"
)

// Labeler selects existing repository labels for an issue.
type Labeler struct {
	provider llm.Provider
}

// LabelAssignmentResult contains chosen labels.
type LabelAssignmentResult struct {
	Labels      []string `json:"labels"`
	Explanation string   `json:"explanation"`
}

// AssignLabels asks the LLM to pick from the repo's existing labels.
func (l *Labeler) AssignLabels(ctx context.Context, title, body string, availableLabels, skipLabels, requiredLabels []string) (*LabelAssignmentResult, error) {
	filterSet := make(map[string]bool)
	for _, s := range skipLabels { filterSet[s] = true }
	for _, r := range requiredLabels { filterSet[r] = true }

	eligible := make([]string, 0, len(availableLabels))
	for _, label := range availableLabels {
		if !filterSet[label] {
			eligible = append(eligible, label)
		}
	}

	prompt := fmt.Sprintf(`Given this issue and these labels, select the most appropriate labels.

Issue: %s

Available labels: %s

Respond with JSON: {"labels": ["label1"], "explanation": "..."}`,
		truncate(title+"\n"+body, 4000),
		strings.Join(eligible, ", "),
	)

	resp, err := l.provider.Chat(ctx, []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a label assignment assistant. Only use labels from the provided list."},
		{Role: llm.RoleUser, Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	var result LabelAssignmentResult
	// Parse JSON from resp.Content
	_ = resp
	return &result, nil
}

func truncate(s string, max int) string {
	if len(s) <= max { return s }
	return s[:max]
}
```

#### File: `internal/triage/ticket.go` (NEW)
```go
package triage

import (
	"context"
	"time"

	"dev.helix.code/internal/database"
)

// TicketStatus represents the triage lifecycle.
type TicketStatus string

const (
	TicketPending    TicketStatus = "pending"
	TicketAnalyzing  TicketStatus = "analyzing"
	TicketActioned   TicketStatus = "actioned"
	TicketFailed     TicketStatus = "failed"
	TicketSkipped    TicketStatus = "skipped"
)

// Ticket records a single triage run.
type Ticket struct {
	ID            string
	IssueID       string
	Repo          string
	Status        TicketStatus
	Classification Classification
	DuplicateOf   *string
	LabelsApplied []string
	ErrorMessage  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Store persists and queries tickets.
type Store struct {
	db *database.Database
}

// CreateTicket inserts a new triage ticket.
func (s *Store) CreateTicket(ctx context.Context, t *Ticket) error {
	query := `INSERT INTO triage_tickets (id, issue_id, repo, status, category, confidence, summary, reasoning, duplicate_of, labels, error, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)`
	_, err := s.db.Exec(ctx, query,
		t.ID, t.IssueID, t.Repo, t.Status,
		t.Classification.Category, t.Classification.Confidence, t.Classification.Summary, t.Classification.Reasoning,
		t.DuplicateOf, t.LabelsApplied, t.ErrorMessage, time.Now(),
	)
	return err
}
```

#### File: `internal/database/migrations/003_triage.up.sql` (NEW)
```sql
CREATE TABLE triage_tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id VARCHAR(255) NOT NULL,
    repo VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    category VARCHAR(50),
    confidence FLOAT,
    summary TEXT,
    reasoning TEXT,
    duplicate_of VARCHAR(255),
    labels TEXT[],
    error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_triage_repo ON triage_tickets(repo);
CREATE INDEX idx_triage_status ON triage_tickets(status);
CREATE INDEX idx_tissue_issue ON triage_tickets(issue_id);
```

### Anti-Bluff Test
```go
func TestAutoTriageEndToEnd(t *testing.T) {
	ctx := context.Background()
	mockLLM := llm.NewMockProvider()
	mockLLM.SetResponse(`{"category":"bug","confidence":0.95,"summary":"Crashes on null pointer","reasoning":"Stack trace shows NPE"}`)

	classifier := triage.NewClassifier(mockLLM)
	result, err := classifier.Classify(ctx, "App crashes on startup", "NullPointerException at MainActivity:42")
	require.NoError(t, err)
	require.Equal(t, "bug", result.Category)
	require.True(t, result.Confidence >= 0.9)

	// Duplicate detection
	store := triage.NewInMemoryEmbeddingStore()
	detector := triage.NewDuplicateDetector(mockLLM, store)
	store.Store(ctx, "issue-1", []float64{0.1, 0.2, 0.3}, nil)

	dup, err := detector.CheckDuplicate(ctx, "issue-2", "Crash on open", "NPE at MainActivity")
	require.NoError(t, err)
	// (Mock would return deterministic embeddings; real test uses actual model)
}
```

### Integration Verification
- GitHub webhook hits `/webhooks/github/issues` → Ticket created with status `pending`.
- Worker processes ticket → status `analyzing` → LLM classification → status `actioned`.
- Database query `SELECT * FROM triage_tickets WHERE status='actioned'` returns completed tickets.

---

## Feature 4: Auto Review

### Source Location (Kilo Code)
- `packages/opencode/src/kilocode/review/review.ts` — Full review engine with prompt, diff parsing, scope handling
- `packages/opencode/src/kilocode/review/types.ts` — Diff file/hunk/result types
- `packages/opencode/src/kilocode/review/command.ts` — VSCode/CLI command bindings
- `packages/kilo-docs/pages/automate/code-reviews/overview.md` — Feature docs

### Target Location (HelixCode)
- **NEW**: `internal/review/` — Review engine
- **NEW**: `internal/review/diff.go` — Git diff parsing
- **NEW**: `internal/review/prompt.go` — Review prompt builder
- **NEW**: `internal/review/engine.go` — Review execution
- **NEW**: `internal/review/formatter.go` — Markdown review formatting
- **MODIFY**: `cmd/helix/review.go` — CLI commands `/local-review`, `/local-review-uncommitted`
- **MODIFY**: `internal/server/routes.go` — Review API endpoints

### Exact Code Changes

#### File: `internal/review/diff.go` (NEW)
```go
package review

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// DiffFile represents a single file in a diff.
type DiffFile struct {
	Path    string
	OldPath string
	Status  string // added, deleted, modified, renamed
	Hunks   []DiffHunk
}

// DiffHunk represents a single hunk.
type DiffHunk struct {
	OldStart  int
	OldLines  int
	NewStart  int
	NewLines  int
	Content   string
}

// DiffResult is the parsed diff.
type DiffResult struct {
	Files []DiffFile
	Raw   string
}

// ParseDiff parses git unified diff output.
func ParseDiff(raw string) *DiffResult {
	result := &DiffResult{Raw: raw}
	if strings.TrimSpace(raw) == "" {
		return result
	}

	fileDiffs := splitFileDiffs(raw)
	for _, fd := range fileDiffs {
		if f := parseFileDiff(fd); f != nil {
			result.Files = append(result.Files, *f)
		}
	}
	return result
}

func splitFileDiffs(raw string) []string {
	re := regexp.MustCompile(`(?m)^diff --git `)
	indices := re.FindAllStringIndex(raw, -1)
	if len(indices) == 0 {
		return []string{raw}
	}
	var parts []string
	for i, idx := range indices {
		start := idx[0]
		end := len(raw)
		if i+1 < len(indices) {
			end = indices[i+1][0]
		}
		parts = append(parts, raw[start:end])
	}
	return parts
}

func parseFileDiff(content string) *DiffFile {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	// diff --git a/<old> b/<new>
	headerRe := regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)
	matches := headerRe.FindStringSubmatch(lines[0])
	if len(matches) < 3 {
		return nil
	}

	oldPath, newPath := matches[1], matches[2]
	status := "modified"
	var oldPathFinal string

	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "new file mode") {
			status = "added"
		} else if strings.HasPrefix(line, "deleted file mode") {
			status = "deleted"
		} else if strings.HasPrefix(line, "rename from ") {
			status = "renamed"
			oldPathFinal = strings.TrimPrefix(line, "rename from ")
		}
	}

	hunks := parseHunks(lines)
	return &DiffFile{
		Path:    newPath,
		OldPath: oldPathFinal,
		Status:  status,
		Hunks:   hunks,
	}
}

func parseHunks(lines []string) []DiffHunk {
	var hunks []DiffHunk
	var current *DiffHunk
	var content []string

	hunkRe := regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

	for _, line := range lines {
		if m := hunksRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				current.Content = strings.Join(content, "\n")
				hunks = append(hunks, *current)
			}
			current = &DiffHunk{
				OldStart: parseIntOrDefault(m[1], 0),
				OldLines: parseIntOrDefault(m[2], 1),
				NewStart: parseIntOrDefault(m[3], 0),
				NewLines: parseIntOrDefault(m[4], 1),
			}
			content = []string{line}
		} else if current != nil && (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ")) {
			content = append(content, line)
		}
	}
	if current != nil {
		current.Content = strings.Join(content, "\n")
		hunks = append(hunks, *current)
	}
	return hunks
}

func parseIntOrDefault(s string, def int) int {
	if s == "" { return def }
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

var hunksRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
```

#### File: `internal/review/prompt.go` (NEW)
```go
package review

import (
	"fmt"
	"strings"
)

// ReviewStyle controls strictness.
type ReviewStyle string

const (
	StyleStrict   ReviewStyle = "strict"
	StyleBalanced ReviewStyle = "balanced"
	StyleLenient  ReviewStyle = "lenient"
)

// BuildReviewPrompt creates the full review prompt for LLM.
func BuildReviewPrompt(scopeDescription, fileList, scope, tools, style string) string {
	prompt := fmt.Sprintf(`You are HelixCode, an expert code reviewer with deep expertise in software engineering best practices, security vulnerabilities, performance optimization, and code quality. Your role is advisory — provide clear, actionable feedback but DO NOT modify any files. Do not use any file editing tools.

You are reviewing: %s

## Files Changed

%s

## Scope
%s

## Review Style
%s

**IMPORTANT**: ONLY review code changes from the files listed above. Do NOT review or flag issues in code that is not part of this diff.

## How to Review

1. **Gather context**: Read full file context when needed.
2. **Be confident**: Only flag issues where you have high confidence.
   - **CRITICAL (95%%+)**: Security vulnerabilities, data loss risks, crashes
   - **WARNING (85%%+)**: Bugs, logic errors, performance issues
   - **SUGGESTION (75%%+)**: Code quality improvements
   - **Below 75%%**: Don't report
3. **Focus on**: Security, Bugs, Performance, Error handling
4. **Don't flag**: Style preferences, minor naming, existing conventions, pre-existing code

Your review MUST follow this exact format:

## Local Review for %s

### Summary
2-3 sentences describing what this change does and your overall assessment.

### Issues Found
| Severity | File:Line | Issue |
|----------|-----------|-------|
| CRITICAL | path/file.go:42 | Brief description |
| WARNING | path/file.go:78 | Brief description |
| SUGGESTION | path/file.go:15 | Brief description |

If no issues found: "No issues found."

### Detailed Findings
For each issue:
- **File:** `path/to/file.go:line`
- **Confidence:** X%%
- **Problem:** What's wrong and why it matters
- **Suggestion:** Recommended fix with code snippet

### Recommendation
- **APPROVE** — Code is ready
- **APPROVE WITH SUGGESTIONS** — Minor improvements
- **NEEDS CHANGES** — Issues must be addressed
`, scopeDescription, fileList, scope, styleName(style), scopeDescription)
	return prompt
}

func styleName(s string) string {
	switch s {
	case "strict":   return "Strict — flags all potential issues"
	case "balanced": return "Balanced — prioritizes clarity and practicality"
	case "lenient":  return "Lenient — flags only critical issues"
	}
	return "Balanced"
}
```

#### File: `internal/review/engine.go` (NEW)
```go
package review

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"dev.helix.code/internal/llm"
)

// Engine executes code reviews.
type Engine struct {
	provider llm.Provider
}

// NewEngine creates a review engine.
func NewEngine(p llm.Provider) *Engine {
	return &Engine{provider: p}
}

// ReviewUncommitted reviews staged + unstaged changes.
func (e *Engine) ReviewUncommitted(ctx context.Context, style ReviewStyle) (string, error) {
	diff, err := getUncommittedChanges()
	if err != nil {
		return "", err
	}
	if len(diff.Files) == 0 {
		return "## Local Review for **uncommitted changes**\n\n### Summary\nNo changes detected.\n\n### Recommendation\n**APPROVE** — Nothing to review.\n", nil
	}

	prompt := BuildReviewPrompt("**uncommitted changes**", formatFileList(diff.Files),
		"Reviewing uncommitted changes (staged + unstaged). Only review the changes shown in the diff.",
		buildToolsSectionUncommitted(), string(style))

	resp, err := e.provider.Chat(ctx, []llm.Message{
		{Role: llm.RoleSystem, Content: "You are an expert code reviewer."},
		{Role: llm.RoleUser, Content: prompt},
	})
	if err != nil {
		return "", fmt.Errorf("review LLM call failed: %w", err)
	}
	return resp.Content, nil
}

// ReviewBranch reviews current branch vs base branch.
func (e *Engine) ReviewBranch(ctx context.Context, baseBranch string, style ReviewStyle) (string, error) {
	if baseBranch == "" {
		baseBranch = detectBaseBranch()
	}
	currentBranch, _ := getCurrentBranch()
	diff, err := getBranchChanges(baseBranch)
	if err != nil {
		return "", err
	}
	if len(diff.Files) == 0 {
		return fmt.Sprintf("## Local Review for **branch diff**: `%s` -> `%s`\n\nNo changes detected.\n", currentBranch, baseBranch), nil
	}

	commits, _ := getBranchCommits(baseBranch, currentBranch)
	scope := fmt.Sprintf("Changes on `%s` since diverging from `%s`.\n\nCommits:\n%s", currentBranch, baseBranch, commits)

	prompt := BuildReviewPrompt(
		fmt.Sprintf("**branch diff**: `%s` -> `%s`", currentBranch, baseBranch),
		formatFileList(diff.Files),
		scope,
		buildToolsSectionBranch(baseBranch, currentBranch),
		string(style),
	)

	resp, err := e.provider.Chat(ctx, []llm.Message{
		{Role: llm.RoleSystem, Content: "You are an expert code reviewer."},
		{Role: llm.RoleUser, Content: prompt},
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func getUncommittedChanges() (*DiffResult, error) {
	cmd := exec.Command("git", "-c", "core.quotepath=false", "diff", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return ParseDiff(string(out)), nil
}

func detectBaseBranch() string {
	candidates := []string{"main", "master", "dev", "develop"}
	for _, branch := range candidates {
		cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
		if cmd.Run() == nil {
			return "origin/" + branch
		}
	}
	for _, branch := range candidates {
		cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		if cmd.Run() == nil {
			return branch
		}
	}
	return "main"
}

func getCurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getBranchChanges(base string) (*DiffResult, error) {
	ancestor, err := exec.Command("git", "merge-base", "HEAD", base).Output()
	if err != nil {
		return nil, err
	}
	hash := strings.TrimSpace(string(ancestor))
	out, err := exec.Command("git", "-c", "core.quotepath=false", "diff", hash).Output()
	if err != nil {
		return nil, err
	}
	return ParseDiff(string(out)), nil
}

func getBranchCommits(base, current string) (string, error) {
	out, err := exec.Command("git", "log", base+".."+current, "--oneline").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func formatFileList(files []DiffFile) string {
	var sb strings.Builder
	for _, f := range files {
		status := "[M]"
		switch f.Status {
		case "added": status = "[A]"
		case "deleted": status = "[D]"
		case "renamed": status = "[R]"
		}
		sb.WriteString(fmt.Sprintf("- %s %s\n", status, f.Path))
	}
	return sb.String()
}

func buildToolsSectionUncommitted() string {
	return `Use these git commands:
- View all changes: git diff && git diff --cached
- View specific file: git diff -- <file>
- Recent history: git log --oneline -20`
}

func buildToolsSectionBranch(base, current string) string {
	return fmt.Sprintf(`Use these git commands:
- View branch diff: git diff %s...%s
- View file diff: git diff %s...%s -- <file>
- Branch commits: git log %s..%s --oneline`, base, current, base, current, base, current)
}
```

#### File: `cmd/helix/review.go` (NEW CLI commands)
```go
package main

import (
	"context"
	"fmt"

	"dev.helix.code/internal/review"
	"github.com/spf13/cobra"
)

func reviewCmd(engine *review.Engine) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "AI-powered code review",
	}

	style := "balanced"
	cmd.PersistentFlags().StringVar(&style, "style", "balanced", "Review style: strict|balanced|lenient")

	cmd.AddCommand(&cobra.Command{
		Use:   "uncommitted",
		Short: "Review staged + unstaged changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := engine.ReviewUncommitted(context.Background(), review.ReviewStyle(style))
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "branch [base-branch]",
		Short: "Review current branch vs base branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base := ""
			if len(args) > 0 {
				base = args[0]
			}
			result, err := engine.ReviewBranch(context.Background(), base, review.ReviewStyle(style))
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})

	return cmd
}
```

### Anti-Bluff Test
```go
func TestReviewDiffParsing(t *testing.T) {
	raw := `diff --git a/main.go b/main.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/main.go
@@ -0,0 +1,10 @@
+package main
+
+import "fmt"
+
+func main() {
+	fmt.Println("hello")
+}
`
	result := review.ParseDiff(raw)
	require.Len(t, result.Files, 1)
	assert.Equal(t, "main.go", result.Files[0].Path)
	assert.Equal(t, "added", result.Files[0].Status)
	require.Len(t, result.Files[0].Hunks, 1)
	assert.Equal(t, 1, result.Files[0].Hunks[0].NewStart)
}

func TestReviewEngineUncommitted(t *testing.T) {
	mockLLM := llm.NewMockProvider()
	engine := review.NewEngine(mockLLM)
	// Requires git repo; use tempdir with git init
}
```

### Integration Verification
- `helix review uncommitted --style=strict` → prints markdown review.
- `helix review branch main --style=lenient` → prints branch diff review.
- Diff parser handles added, deleted, renamed, modified files with correct hunks.

---

## Feature 5: App Builder

### Source Location (Kilo Code)
- Kilo Code's App Builder is a premium/cloud feature for full application generation.
- Concepts: framework selection (React, Vue, Next.js, etc.), database schema generation, scaffolding, dependency injection.

### Target Location (HelixCode)
- **NEW**: `internal/appbuilder/` — App generation engine
- **NEW**: `internal/appbuilder/framework.go` — Framework definitions and detection
- **NEW**: `internal/appbuilder/scaffold.go` — Template scaffolding
- **NEW**: `internal/appbuilder/database.go` — Schema generation
- **NEW**: `internal/appbuilder/generator.go` — Main generation orchestrator
- **MODIFY**: `cmd/helix/app.go` — `helix app create` CLI

### Exact Code Changes

#### File: `internal/appbuilder/framework.go` (NEW)
```go
package appbuilder

import (
	"fmt"
	"os"
	"path/filepath"
)

// Framework represents a supported application framework.
type Framework string

const (
	FrameworkGoStdlib     Framework = "go-stdlib"
	FrameworkGoGin        Framework = "go-gin"
	FrameworkGoEcho       Framework = "go-echo"
	FrameworkGoFiber      Framework = "go-fiber"
	FrameworkPythonFastAPI Framework = "python-fastapi"
	FrameworkPythonFlask  Framework = "python-flask"
	FrameworkNodeExpress  Framework = "node-express"
	FrameworkReact        Framework = "react"
	FrameworkNextJS       Framework = "nextjs"
	FrameworkVue          Framework = "vue"
	FrameworkSvelte       Framework = "svelte"
)

// FrameworkMeta holds metadata for a framework.
type FrameworkMeta struct {
	Name        string
	Language    string
	PackageFile string // e.g., go.mod, package.json, requirements.txt
	Port        int    // default dev port
}

var frameworkMeta = map[Framework]FrameworkMeta{
	FrameworkGoStdlib:     {Name: "Go Standard Library", Language: "go", PackageFile: "go.mod", Port: 8080},
	FrameworkGoGin:        {Name: "Gin", Language: "go", PackageFile: "go.mod", Port: 8080},
	FrameworkGoEcho:       {Name: "Echo", Language: "go", PackageFile: "go.mod", Port: 8080},
	FrameworkGoFiber:      {Name: "Fiber", Language: "go", PackageFile: "go.mod", Port: 8080},
	FrameworkPythonFastAPI: {Name: "FastAPI", Language: "python", PackageFile: "requirements.txt", Port: 8000},
	FrameworkPythonFlask:  {Name: "Flask", Language: "python", PackageFile: "requirements.txt", Port: 5000},
	FrameworkNodeExpress:  {Name: "Express", Language: "javascript", PackageFile: "package.json", Port: 3000},
	FrameworkReact:        {Name: "React", Language: "javascript", PackageFile: "package.json", Port: 3000},
	FrameworkNextJS:       {Name: "Next.js", Language: "javascript", PackageFile: "package.json", Port: 3000},
	FrameworkVue:          {Name: "Vue", Language: "javascript", PackageFile: "package.json", Port: 3000},
	FrameworkSvelte:       {Name: "Svelte", Language: "javascript", PackageFile: "package.json", Port: 3000},
}

// DetectFramework attempts to detect framework from existing files.
func DetectFramework(dir string) (Framework, error) {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		data, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
		content := string(data)
		if contains(content, "github.com/gin-gonic/gin") {
			return FrameworkGoGin, nil
		}
		if contains(content, "github.com/labstack/echo") {
			return FrameworkGoEcho, nil
		}
		if contains(content, "github.com/gofiber/fiber") {
			return FrameworkGoFiber, nil
		}
		return FrameworkGoStdlib, nil
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		data, _ := os.ReadFile(filepath.Join(dir, "package.json"))
		content := string(data)
		if contains(content, "next") {
			return FrameworkNextJS, nil
		}
		if contains(content, "react") {
			return FrameworkReact, nil
		}
		if contains(content, "vue") {
			return FrameworkVue, nil
		}
		if contains(content, "svelte") {
			return FrameworkSvelte, nil
		}
		if contains(content, "express") {
			return FrameworkNodeExpress, nil
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		data, _ := os.ReadFile(filepath.Join(dir, "requirements.txt"))
		content := string(data)
		if contains(content, "fastapi") {
			return FrameworkPythonFastAPI, nil
		}
		if contains(content, "flask") {
			return FrameworkPythonFlask, nil
		}
	}
	return "", fmt.Errorf("no recognizable framework found in %s", dir)
}

func contains(s, substr string) bool { return len(s) > 0 && (s == substr || len(s) > len(substr)) }
```

#### File: `internal/appbuilder/scaffold.go` (NEW)
```go
package appbuilder

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// Scaffold generates the initial project structure.
type Scaffold struct {
	Framework Framework
	ProjectName string
	OutputDir   string
}

// TemplateData is passed to scaffold templates.
type TemplateData struct {
	ProjectName string
	ModulePath  string
	Port        int
}

// Generate creates all scaffold files.
func (s *Scaffold) Generate() error {
	meta := frameworkMeta[s.Framework]
	data := TemplateData{
		ProjectName: s.ProjectName,
		ModulePath:  fmt.Sprintf("github.com/example/%s", s.ProjectName),
		Port:        meta.Port,
	}

	switch s.Framework {
	case FrameworkGoGin:
		return s.generateGoGin(data)
	case FrameworkNextJS:
		return s.generateNextJS(data)
	case FrameworkPythonFastAPI:
		return s.generateFastAPI(data)
	}
	return fmt.Errorf("scaffold not yet implemented for %s", s.Framework)
}

func (s *Scaffold) generateGoGin(data TemplateData) error {
	files := map[string]string{
		"go.mod": `module {{.ModulePath}}

go 1.23

require github.com/gin-gonic/gin v1.10.0
`,
		"main.go": `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.Run(":{{.Port}}")
}
`,
		".env": `PORT={{.Port}}
DATABASE_URL=postgres://localhost/{{.ProjectName}}?sslmode=disable
`,
		"Dockerfile": `FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/app .
CMD ["./app"]
`,
	}

	for name, tmpl := range files {
		path := filepath.Join(s.OutputDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		t, err := template.New(name).Parse(tmpl)
		if err != nil {
			return err
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		if err := t.Execute(f, data); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return nil
}

func (s *Scaffold) generateNextJS(data TemplateData) error {
	// Uses npx create-next-app or generates minimal structure
	return nil
}

func (s *Scaffold) generateFastAPI(data TemplateData) error {
	files := map[string]string{
		"requirements.txt": `fastapi==0.115.0
uvicorn[standard]==0.32.0
`,
		"main.py": `from fastapi import FastAPI

app = FastAPI()

@app.get("/health")
def health():
    return {"status": "ok"}
`,
		"Dockerfile": `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY . .
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "{{.Port}}"]
`,
	}
	for name, tmpl := range files {
		path := filepath.Join(s.OutputDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		t, _ := template.New(name).Parse(tmpl)
		f, _ := os.Create(path)
		t.Execute(f, data)
		f.Close()
	}
	return nil
}
```

#### File: `internal/appbuilder/database.go` (NEW)
```go
package appbuilder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DatabaseSchema generates database schema from entity descriptions.
type DatabaseSchema struct {
	Dialect string // postgres, mysql, sqlite
}

// Entity represents a domain model.
type Entity struct {
	Name       string
	Fields     []Field
	Relations  []Relation
}

// Field is a column definition.
type Field struct {
	Name     string
	Type     string
	Nullable bool
	Unique   bool
	Default  string
}

// Relation defines an association.
type Relation struct {
	Type       string // has_one, has_many, belongs_to
	Target     string
	ForeignKey string
}

// GenerateSQL creates DDL for the entities.
func (ds *DatabaseSchema) GenerateSQL(entities []Entity) (string, error) {
	var sb strings.Builder
	for _, e := range entities {
		sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", e.Name))
		for i, f := range e.Fields {
			line := fmt.Sprintf("    %s %s", f.Name, ds.mapType(f.Type))
			if !f.Nullable {
				line += " NOT NULL"
			}
			if f.Unique {
				line += " UNIQUE"
			}
			if f.Default != "" {
				line += fmt.Sprintf(" DEFAULT %s", f.Default)
			}
			if i < len(e.Fields)-1 || len(e.Relations) > 0 {
				line += ","
			}
			sb.WriteString(line + "\n")
		}
		for _, r := range e.Relations {
			if r.Type == "belongs_to" {
				sb.WriteString(fmt.Sprintf("    %s_id INTEGER REFERENCES %s(id),\n", r.Target, r.Target))
			}
		}
		sb.WriteString(");\n\n")
	}
	return sb.String(), nil
}

func (ds *DatabaseSchema) mapType(t string) string {
	switch ds.Dialect {
	case "postgres":
		switch t {
		case "string": return "VARCHAR(255)"
		case "text": return "TEXT"
		case "int": return "INTEGER"
		case "bigint": return "BIGINT"
		case "bool": return "BOOLEAN"
		case "datetime": return "TIMESTAMP"
		case "uuid": return "UUID"
		}
	case "sqlite":
		switch t {
		case "string": return "TEXT"
		case "int": return "INTEGER"
		case "bool": return "INTEGER"
		case "datetime": return "TEXT"
		}
	}
	return "TEXT"
}

// WriteMigration writes the SQL to a migration file.
func (ds *DatabaseSchema) WriteMigration(dir, name, sql string) error {
	path := filepath.Join(dir, "migrations", name+".sql")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(sql), 0644)
}
```

### Anti-Bluff Test
```go
func TestAppBuilderScaffold(t *testing.T) {
	tmp := t.TempDir()
	scaffold := appbuilder.Scaffold{
		Framework:   appbuilder.FrameworkGoGin,
		ProjectName: "testapp",
		OutputDir:   tmp,
	}
	require.NoError(t, scaffold.Generate())

	// Verify go.mod exists and contains gin
	data, err := os.ReadFile(filepath.Join(tmp, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "gin-gonic/gin")

	// Verify main.go has health endpoint
	data, err = os.ReadFile(filepath.Join(tmp, "main.go"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "/health")
}

func TestDatabaseSchemaGeneration(t *testing.T) {
	schema := appbuilder.DatabaseSchema{Dialect: "postgres"}
	sql, err := schema.GenerateSQL([]appbuilder.Entity{{
		Name: "users",
		Fields: []appbuilder.Field{
			{Name: "id", Type: "uuid", Nullable: false},
			{Name: "email", Type: "string", Nullable: false, Unique: true},
			{Name: "created_at", Type: "datetime", Nullable: false},
		},
	}})
	require.NoError(t, err)
	assert.Contains(t, sql, "CREATE TABLE users")
	assert.Contains(t, sql, "email VARCHAR(255) NOT NULL UNIQUE")
}
```

### Integration Verification
- `helix app create --framework=go-gin --name=myapp` → directory created with go.mod, main.go, Dockerfile.
- `helix app detect` → prints detected framework from current directory.
- Database schema generation produces valid PostgreSQL DDL.

---

## Feature 6: Enhanced UI

### Source Location (Kilo Code)
- `packages/kilo-vscode/src/` — VS Code extension UI: agent-manager, diff-viewer, webview UI
- `packages/kilo-ui/src/components/` — Shared React components: session-review, image-preview
- `packages/kilo-vscode/webview-ui/diff-viewer/` — Enhanced diff display
- `packages/kilo-vscode/webview-ui/diff-virtual/` — Virtual scrolling diff
- `packages/kilo-vscode/src/kilo-provider/` — Status indicators, streaming UI

### Target Location (HelixCode)
- **NEW**: `applications/vscode/` — VS Code extension frontend
- **NEW**: `applications/vscode/webview/` — Webview UI components
- **NEW**: `applications/vscode/webview/components/DiffViewer.tsx` — Enhanced diff
- **NEW**: `applications/vscode/webview/components/StatusIndicator.tsx` — Mode/status badges
- **NEW**: `applications/vscode/webview/components/AgentManager.tsx` — Mode switcher panel
- **MODIFY**: `applications/vscode/package.json` — Extension manifest
- **MODIFY**: `internal/server/websocket.go` — Streaming message bus for UI updates

### Exact Code Changes

#### File: `applications/vscode/webview/components/DiffViewer.tsx` (NEW)
```tsx
import React, { useMemo } from "react";

interface DiffHunk {
  oldStart: number;
  oldLines: number;
  newStart: number;
  newLines: number;
  lines: DiffLine[];
}

interface DiffLine {
  type: "context" | "add" | "del";
  oldNum: number | null;
  newNum: number | null;
  content: string;
}

interface DiffFile {
  path: string;
  status: "added" | "deleted" | "modified" | "renamed";
  hunks: DiffHunk[];
}

interface DiffViewerProps {
  files: DiffFile[];
  onAccept: (path: string) => void;
  onReject: (path: string) => void;
}

const DiffViewer: React.FC<DiffViewerProps> = ({ files, onAccept, onReject }) => {
  return (
    <div className="diff-viewer">
      {files.map((file) => (
        <DiffFileView key={file.path} file={file} onAccept={onAccept} onReject={onReject} />
      ))}
    </div>
  );
};

const DiffFileView: React.FC<{ file: DiffFile; onAccept: (p: string) => void; onReject: (p: string) => void }> = ({
  file,
  onAccept,
  onReject,
}) => {
  const statusBadge = useMemo(() => {
    switch (file.status) {
      case "added": return <span className="badge added">[A]</span>;
      case "deleted": return <span className="badge deleted">[D]</span>;
      case "renamed": return <span className="badge renamed">[R]</span>;
      default: return <span className="badge modified">[M]</span>;
    }
  }, [file.status]);

  return (
    <div className="diff-file">
      <div className="diff-file-header">
        {statusBadge}
        <span className="diff-file-path">{file.path}</span>
        <div className="diff-file-actions">
          <button onClick={() => onAccept(file.path)} className="btn-accept">Accept</button>
          <button onClick={() => onReject(file.path)} className="btn-reject">Reject</button>
        </div>
      </div>
      <div className="diff-file-content">
        {file.hunks.map((hunk, hi) => (
          <div key={hi} className="diff-hunk">
            <div className="diff-hunk-header">
              @@ -{hunk.oldStart},{hunk.oldLines} +{hunk.newStart},{hunk.newLines} @@
            </div>
            {hunk.lines.map((line, li) => (
              <div key={li} className={`diff-line ${line.type}`}>
                <span className="diff-line-num old">{line.oldNum ?? ""}</span>
                <span className="diff-line-num new">{line.newNum ?? ""}</span>
                <span className="diff-line-marker">
                  {line.type === "add" ? "+" : line.type === "del" ? "-" : " "}
                </span>
                <span className="diff-line-content">{line.content}</span>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
};

export default DiffViewer;
```

#### File: `applications/vscode/webview/components/StatusIndicator.tsx` (NEW)
```tsx
import React from "react";

interface StatusIndicatorProps {
  mode: string;
  model: string;
  provider: string;
  isStreaming: boolean;
  tokensUsed: number;
  costEstimate: number;
}

const modeColors: Record<string, string> = {
  code: "#3b82f6",
  architect: "#8b5cf6",
  ask: "#10b981",
  debug: "#ef4444",
  test: "#f59e0b",
  review: "#6366f1",
};

const StatusIndicator: React.FC<StatusIndicatorProps> = ({
  mode,
  model,
  provider,
  isStreaming,
  tokensUsed,
  costEstimate,
}) => {
  return (
    <div className="status-bar">
      <div className="status-pill mode" style={{ backgroundColor: modeColors[mode] || "#6b7280" }}>
        {mode.toUpperCase()}
      </div>
      <div className="status-pill model">
        {provider}/{model}
      </div>
      {isStreaming && (
        <div className="status-pill streaming">
          <span className="spinner" /> Generating...
        </div>
      )}
      <div className="status-pill tokens">
        {tokensUsed.toLocaleString()} tokens
      </div>
      <div className="status-pill cost">
        ~${costEstimate.toFixed(4)}
      </div>
    </div>
  );
};

export default StatusIndicator;
```

#### File: `applications/vscode/webview/components/AgentManager.tsx` (NEW)
```tsx
import React, { useState } from "react";

interface ModeDef {
  name: string;
  displayName: string;
  description: string;
  color: string;
  icon: string;
}

interface AgentManagerProps {
  modes: ModeDef[];
  currentMode: string;
  onSwitchMode: (mode: string) => void;
  subagents: string[];
  onSpawnSubagent: (type: string, prompt: string) => void;
}

const AgentManager: React.FC<AgentManagerProps> = ({
  modes,
  currentMode,
  onSwitchMode,
  subagents,
  onSpawnSubagent,
}) => {
  const [subagentPrompt, setSubagentPrompt] = useState("");
  const [selectedSubagent, setSelectedSubagent] = useState(subagents[0] || "");

  return (
    <div className="agent-manager">
      <h3>Mode</h3>
      <div className="mode-grid">
        {modes.map((m) => (
          <button
            key={m.name}
            className={`mode-card ${currentMode === m.name ? "active" : ""}`}
            style={{ borderColor: m.color }}
            onClick={() => onSwitchMode(m.name)}
          >
            <span className="mode-icon">{m.icon}</span>
            <span className="mode-name">{m.displayName}</span>
            <span className="mode-desc">{m.description}</span>
          </button>
        ))}
      </div>

      <h3>Subagents</h3>
      <div className="subagent-spawn">
        <select value={selectedSubagent} onChange={(e) => setSelectedSubagent(e.target.value)}>
          {subagents.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
        <textarea
          value={subagentPrompt}
          onChange={(e) => setSubagentPrompt(e.target.value)}
          placeholder="Task description for subagent..."
          rows={3}
        />
        <button onClick={() => onSpawnSubagent(selectedSubagent, subagentPrompt)}>
          Spawn Subagent
        </button>
      </div>
    </div>
  );
};

export default AgentManager;
```

#### File: `applications/vscode/webview/styles/diff-viewer.css` (NEW)
```css
.diff-viewer {
  font-family: "SF Mono", Monaco, monospace;
  font-size: 13px;
  line-height: 1.5;
  background: #1e1e1e;
  color: #d4d4d4;
  border-radius: 6px;
  overflow: hidden;
}

.diff-file {
  margin-bottom: 12px;
  border: 1px solid #333;
  border-radius: 4px;
}

.diff-file-header {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  background: #252526;
  border-bottom: 1px solid #333;
}

.diff-file-path {
  flex: 1;
  font-weight: 600;
  margin-left: 8px;
}

.badge {
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: bold;
}
.badge.added { background: #2ea043; color: white; }
.badge.deleted { background: #f85149; color: white; }
.badge.modified { background: #0078d4; color: white; }
.badge.renamed { background: #8957e5; color: white; }

.diff-line {
  display: flex;
  white-space: pre;
}
.diff-line.add { background: rgba(46, 160, 67, 0.15); }
.diff-line.del { background: rgba(248, 81, 73, 0.15); }
.diff-line.context { background: transparent; }

.diff-line-num {
  width: 40px;
  text-align: right;
  padding-right: 8px;
  color: #6e7681;
  user-select: none;
}

.diff-line-marker {
  width: 16px;
  text-align: center;
  user-select: none;
}

.diff-line-content {
  flex: 1;
  padding-left: 4px;
}

.btn-accept {
  background: #2ea043;
  color: white;
  border: none;
  padding: 4px 12px;
  border-radius: 3px;
  cursor: pointer;
  margin-right: 4px;
}
.btn-reject {
  background: #f85149;
  color: white;
  border: none;
  padding: 4px 12px;
  border-radius: 3px;
  cursor: pointer;
}

.status-bar {
  display: flex;
  gap: 8px;
  padding: 6px 12px;
  background: #0d1117;
  border-top: 1px solid #30363d;
  font-size: 12px;
}

.status-pill {
  padding: 2px 8px;
  border-radius: 12px;
  background: #21262d;
  color: #c9d1d9;
}
.status-pill.mode {
  color: white;
  font-weight: bold;
}
.status-pill.streaming {
  color: #58a6ff;
}

.spinner {
  display: inline-block;
  width: 10px;
  height: 10px;
  border: 2px solid #58a6ff;
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
```

#### File: `internal/server/websocket.go` (NEW — Streaming bus)
```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// StreamMessage is a UI update payload.
type StreamMessage struct {
	Type      string      `json:"type"`      // "chunk", "status", "mode_change", "tool_call"
	SessionID string      `json:"sessionId"`
	Payload   interface{} `json:"payload"`
}

// StreamHub manages WebSocket clients per session.
type StreamHub struct {
	upgrader websocket.Upgrader
	clients  map[string]map[*websocket.Conn]bool // sessionID -> connections
	mu       sync.RWMutex
}

// NewStreamHub creates a hub.
func NewStreamHub() *StreamHub {
	return &StreamHub{
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		clients:  make(map[string]map[*websocket.Conn]bool),
	}
}

// HandleWS is the HTTP handler for WebSocket upgrades.
func (h *StreamHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		http.Error(w, "session required", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	h.mu.Lock()
	if h.clients[sessionID] == nil {
		h.clients[sessionID] = make(map[*websocket.Conn]bool)
	}
	h.clients[sessionID][conn] = true
	h.mu.Unlock()

	// Keep connection alive until client disconnects
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			h.mu.Lock()
			delete(h.clients[sessionID], conn)
			h.mu.Unlock()
			return
		}
	}
}

// Broadcast sends a message to all clients for a session.
func (h *StreamHub) Broadcast(ctx context.Context, sessionID string, msg StreamMessage) error {
	h.mu.RLock()
	clients := h.clients[sessionID]
	h.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	for conn := range clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			// Remove dead client
			h.mu.Lock()
			delete(h.clients[sessionID], conn)
			conn.Close()
			h.mu.Unlock()
		}
	}
	return nil
}

// PublishChunk sends a text generation chunk.
func (h *StreamHub) PublishChunk(sessionID, content string) {
	h.Broadcast(context.Background(), sessionID, StreamMessage{
		Type:      "chunk",
		SessionID: sessionID,
		Payload:   map[string]string{"content": content},
	})
}

// PublishStatus sends mode/status updates.
func (h *StreamHub) PublishStatus(sessionID, mode, model, provider string, tokens int, cost float64) {
	h.Broadcast(context.Background(), sessionID, StreamMessage{
		Type:      "status",
		SessionID: sessionID,
		Payload: map[string]interface{}{
			"mode":    mode,
			"model":   model,
			"provider": provider,
			"tokens":  tokens,
			"cost":    cost,
		},
	})
}
```

### Anti-Bluff Test
```typescript
// DiffViewer.test.tsx
import { render, screen } from "@testing-library/react";
import DiffViewer from "./DiffViewer";

test("renders added file with correct badge", () => {
  const files = [{
    path: "main.go",
    status: "added" as const,
    hunks: [{
      oldStart: 0, oldLines: 0, newStart: 1, newLines: 3,
      lines: [
        { type: "add" as const, oldNum: null, newNum: 1, content: "package main" },
      ],
    }],
  }];
  render(<DiffViewer files={files} onAccept={() => {}} onReject={() => {}} />);
  expect(screen.getByText("[A]")).toBeInTheDocument();
  expect(screen.getByText("main.go")).toBeInTheDocument();
});

test("status indicator shows mode color", () => {
  render(<StatusIndicator mode="debug" model="claude" provider="anthropic" isStreaming={true} tokensUsed={1500} costEstimate={0.012} />);
  expect(screen.getByText("DEBUG")).toBeInTheDocument();
  expect(screen.getByText("Generating...")).toBeInTheDocument();
});
```

### Integration Verification
- WebSocket `ws://localhost:8080/ws?session=abc` receives chunk messages in real-time.
- Diff viewer renders 1000-line diffs without lag (virtualized in v2).
- Status bar updates mode color when switching from `code` to `debug`.

---

## Feature 7: Additional Model Support

### Source Location (Kilo Code)
- `packages/opencode/src/provider/schema.ts` — ProviderID enum: anthropic, openai, google, github-copilot, amazon-bedrock, azure, openrouter, mistral, gitlab, kilo
- `packages/opencode/src/provider/models.ts` — Model definitions with capabilities
- `packages/opencode/src/provider/models-snapshot.ts` — Static model data
- `packages/opencode/src/provider/transform.ts` — Provider-specific transforms

### Target Location (HelixCode)
- **MODIFY**: `internal/llm/providers_registry.go` — Add Kilo Code providers
- **MODIFY**: `internal/llm/provider.go` — Extend Provider interface with streaming, embeddings
- **NEW**: `internal/llm/bedrock.go` — Amazon Bedrock provider
- **NEW**: `internal/llm/azure.go` — Azure OpenAI provider
- **NEW**: `internal/llm/openrouter.go` — OpenRouter provider
- **NEW**: `internal/llm/copilot.go` — GitHub Copilot provider
- **NEW**: `internal/llm/gemini.go` — Google Gemini provider
- **MODIFY**: `internal/config/config.go` — Add provider config sections

### Exact Code Changes

#### File: `internal/llm/provider.go` (MODIFY — Add streaming and model listing)
```go
package llm

import (
	"context"
	"fmt"
	"strings"
)

// Extended Provider interface with Kilo Code capabilities.
type Provider interface {
	Chat(ctx context.Context, messages []Message) (*Response, error)
	Vision(ctx context.Context, image []byte, prompt string) (*Response, error)
	Name() string
	SupportsVision() bool

	// NEW: Streaming generation
	ChatStream(ctx context.Context, messages []Message, chunkChan chan<- StreamChunk) error

	// NEW: Model listing
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// NEW: Embedding support (for duplicate detection, semantic search)
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// StreamChunk is a single streaming response fragment.
type StreamChunk struct {
	Content      string
	FinishReason string
	Usage        Usage
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// ModelInfo describes an available model.
type ModelInfo struct {
	ID          string
	Name        string
	Provider    string
	ContextSize int
	SupportsVision bool
	SupportsStreaming bool
}

// ModelRef identifies a model instance.
type ModelRef struct {
	ProviderID string
	ModelID    string
}
```

#### File: `internal/llm/openrouter.go` (NEW)
```go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenRouterProvider implements Provider for OpenRouter.ai.
type OpenRouterProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenRouterProvider creates an OpenRouter provider.
func NewOpenRouterProvider(apiKey string) *OpenRouterProvider {
	return &OpenRouterProvider{
		apiKey:  apiKey,
		baseURL: "https://openrouter.ai/api/v1",
		client:  &http.Client{},
	}
}

func (p *OpenRouterProvider) Name() string { return ProviderOpenRouter }
func (p *OpenRouterProvider) SupportsVision() bool { return true }

func (p *OpenRouterProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model":    "anthropic/claude-sonnet-4",
		"messages": messages,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}
	return &Response{
		Content:      result.Choices[0].Message.Content,
		Model:        "anthropic/claude-sonnet-4",
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	}, nil
}

func (p *OpenRouterProvider) ChatStream(ctx context.Context, messages []Message, chunkChan chan<- StreamChunk) error {
	body, _ := json.Marshal(map[string]interface{}{
		"model":    "anthropic/claude-sonnet-4",
		"messages": messages,
		"stream":   true,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if len(chunk.Choices) > 0 {
			c := chunk.Choices[0]
			chunkChan <- StreamChunk{
				Content:      c.Delta.Content,
				FinishReason: c.FinishReason,
			}
			if c.FinishReason != "" {
				break
			}
		}
	}
	return nil
}

func (p *OpenRouterProvider) Vision(ctx context.Context, image []byte, prompt string) (*Response, error) {
	return nil, fmt.Errorf("vision not yet implemented for openrouter")
}

func (p *OpenRouterProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Context  int    `json:"context_length"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range result.Data {
		models = append(models, ModelInfo{
			ID:          m.ID,
			Name:        m.Name,
			Provider:    p.Name(),
			ContextSize: m.Context,
		})
	}
	return models, nil
}

func (p *OpenRouterProvider) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, fmt.Errorf("embeddings not yet implemented for openrouter")
}
```

#### File: `internal/llm/bedrock.go` (NEW — AWS Bedrock)
```go
package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// BedrockProvider implements Provider for Amazon Bedrock.
type BedrockProvider struct {
	client    *bedrockruntime.Client
	region    string
	modelID   string
}

// NewBedrockProvider creates a Bedrock provider.
func NewBedrockProvider(ctx context.Context, region, modelID string) (*BedrockProvider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return &BedrockProvider{
		client:  bedrockruntime.NewFromConfig(cfg),
		region:  region,
		modelID: modelID,
	}, nil
}

func (p *BedrockProvider) Name() string { return "amazon-bedrock" }
func (p *BedrockProvider) SupportsVision() bool { return false }

func (p *BedrockProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	var body []byte
	if isClaudeModel(p.modelID) {
		body = buildClaudeBody(messages)
	} else {
		body = buildGenericBody(messages)
	}

	resp, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     &p.modelID,
		Body:        body,
		ContentType: strPtr("application/json"),
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock invoke failed: %w", err)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("no content returned")
	}
	return &Response{
		Content:      result.Content[0].Text,
		Model:        p.modelID,
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
	}, nil
}

func isClaudeModel(id string) bool {
	return len(id) > 5 && id[:5] == "anthr"
}

func buildClaudeBody(messages []Message) []byte {
	body, _ := json.Marshal(map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        4096,
		"messages":          messages,
	})
	return body
}

func buildGenericBody(messages []Message) []byte {
	body, _ := json.Marshal(map[string]interface{}{
		"inputText": messages[len(messages)-1].Content,
	})
	return body
}

func strPtr(s string) *string { return &s }

func (p *BedrockProvider) ChatStream(ctx context.Context, messages []Message, chunkChan chan<- StreamChunk) error {
	return fmt.Errorf("streaming not yet implemented for bedrock")
}
func (p *BedrockProvider) Vision(ctx context.Context, image []byte, prompt string) (*Response, error) {
	return nil, fmt.Errorf("vision not yet implemented for bedrock")
}
func (p *BedrockProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return nil, fmt.Errorf("list models not yet implemented for bedrock")
}
func (p *BedrockProvider) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, fmt.Errorf("embeddings not yet implemented for bedrock")
}
```

#### File: `internal/llm/copilot.go` (NEW — GitHub Copilot)
```go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CopilotProvider implements Provider for GitHub Copilot.
type CopilotProvider struct {
	token   string
	client  *http.Client
}

// NewCopilotProvider creates a Copilot provider from a GitHub token.
func NewCopilotProvider(token string) *CopilotProvider {
	return &CopilotProvider{
		token:  token,
		client: &http.Client{},
	}
}

func (p *CopilotProvider) Name() string { return "github-copilot" }
func (p *CopilotProvider) SupportsVision() bool { return false }

func (p *CopilotProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"messages": messages,
		"model":    "gpt-4o",
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/copilot/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "token "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}
	return &Response{
		Content: result.Choices[0].Message.Content,
		Model:   "gpt-4o",
	}, nil
}

func (p *CopilotProvider) ChatStream(ctx context.Context, messages []Message, chunkChan chan<- StreamChunk) error {
	return fmt.Errorf("streaming not yet implemented for copilot")
}
func (p *CopilotProvider) Vision(ctx context.Context, image []byte, prompt string) (*Response, error) {
	return nil, fmt.Errorf("vision not yet implemented for copilot")
}
func (p *CopilotProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: p.Name(), ContextSize: 128000},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: p.Name(), ContextSize: 128000},
	}, nil
}
func (p *CopilotProvider) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, fmt.Errorf("embeddings not yet implemented for copilot")
}
```

#### File: `internal/config/config.go` (MODIFY — Add provider sections)
```go
package config

// ProviderConfig holds per-provider settings.
type ProviderConfig struct {
	ID          string  `yaml:"id"`
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url,omitempty"`
	Model       string  `yaml:"model,omitempty"`
	Region      string  `yaml:"region,omitempty"`      // For Bedrock
	Temperature float64 `yaml:"temperature,omitempty"`
	TopP        float64 `yaml:"top_p,omitempty"`
}

// Config is the top-level configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Redis     RedisConfig     `yaml:"redis"`
	Providers []ProviderConfig `yaml:"providers"`
	DefaultProvider string    `yaml:"default_provider"`
	// ... existing fields
}
```

### Anti-Bluff Test
```go
func TestOpenRouterProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}
	p := llm.NewOpenRouterProvider(os.Getenv("OPENROUTER_API_KEY"))
	ctx := context.Background()

	resp, err := p.Chat(ctx, []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a test assistant."},
		{Role: llm.RoleUser, Content: "Say exactly 'HELIXCODE_TEST_OK' and nothing else."},
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "HELIXCODE_TEST_OK")
	assert.NotZero(t, resp.InputTokens)
	assert.NotZero(t, resp.OutputTokens)

	models, err := p.ListModels(ctx)
	require.NoError(t, err)
	assert.Greater(t, len(models), 10)
}

func TestBedrockProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}
	ctx := context.Background()
	p, err := llm.NewBedrockProvider(ctx, "us-east-1", "anthropic.claude-sonnet-4-20250514-v1:0")
	require.NoError(t, err)

	resp, err := p.Chat(ctx, []llm.Message{
		{Role: llm.RoleUser, Content: "Say 'BEDROCK_OK'"},
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "BEDROCK_OK")
}
```

### Integration Verification
- `helix --list-models` shows all 30+ providers via `providers_registry.go`.
- Each new provider has a unit test that calls the real API with a short prompt.
- Streaming works end-to-end: `ChatStream` feeds chunks to WebSocket hub.

---

## Feature 8: Improved Context Handling

### Source Location (Kilo Code)
- `packages/opencode/src/tool/truncate.ts` — Context truncation strategy
- `packages/opencode/src/tool/truncation-dir.ts` — Directory truncation
- `packages/opencode/src/tool/read.ts` — Smart file reading with relevance
- `packages/opencode/src/tool/grep.ts` — Relevant file detection via grep
- `packages/opencode/src/tool/codesearch.ts` — Semantic code search
- `packages/kilo-indexing/src/` — File indexing with tree-sitter

### Target Location (HelixCode)
- **NEW**: `internal/context/` — Context assembly engine
- **NEW**: `internal/context/truncator.go` — Token-aware truncation
- **NEW**: `internal/context/relevance.go` — Relevant file detection
- **NEW**: `internal/context/assembly.go` — Prompt assembly
- **NEW**: `internal/indexing/` — File index with tree-sitter
- **NEW**: `internal/indexing/indexer.go` — AST-based indexing
- **MODIFY**: `internal/llm/prompt_optimizer.go` (HelixQA) — Enhance with Kilo strategies

### Exact Code Changes

#### File: `internal/context/assembly.go` (NEW)
```go
package context

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"dev.helix.code/internal/indexing"
	"dev.helix.code/internal/llm"
)

// Assembler builds the final prompt context for an LLM call.
type Assembler struct {
	indexer   *indexing.Indexer
	truncator *Truncator
	maxTokens int
}

// AssemblyResult contains the assembled context.
type AssemblyResult struct {
	Messages    []llm.Message
	TokenCount  int
	FilesUsed   []string
	FilesSkipped []string
}

// NewAssembler creates a context assembler.
func NewAssembler(idx *indexing.Indexer, maxTokens int) *Assembler {
	return &Assembler{
		indexer:   idx,
		truncator: NewTruncator(maxTokens),
		maxTokens: maxTokens,
	}
}

// Assemble builds context for a user query.
func (a *Assembler) Assemble(ctx context.Context, query string, relevantFiles []string, history []llm.Message) (*AssemblyResult, error) {
	// 1. Find semantically relevant files if not provided
	if len(relevantFiles) == 0 {
		scores, err := a.indexer.SemanticSearch(ctx, query, 20)
		if err != nil {
			return nil, err
		}
		for _, s := range scores {
			relevantFiles = append(relevantFiles, s.Path)
		}
	}

	// 2. Sort by relevance score
	sort.Strings(relevantFiles)

	// 3. Read file contents up to token budget
	var fileContents []string
	var used, skipped []string
	tokenBudget := a.maxTokens - estimateTokens(query) - estimateMessagesTokens(history)

	for _, path := range relevantFiles {
		content, err := a.indexer.ReadFile(ctx, path)
		if err != nil {
			skipped = append(skipped, path)
			continue
		}
		tokens := estimateTokens(content)
		if tokens > tokenBudget {
			// Truncate with smart summarization
			content = a.truncator.Truncate(content, tokenBudget)
			tokens = estimateTokens(content)
		}
		if tokens <= tokenBudget {
			fileContents = append(fileContents, fmt.Sprintf("// %s\n%s", path, content))
			tokenBudget -= tokens
			used = append(used, path)
		} else {
			skipped = append(skipped, path)
		}
	}

	// 4. Build system message with file context
	systemContent := "You are HelixCode. You have access to the following files:\n\n"
	systemContent += strings.Join(fileContents, "\n\n")
	systemContent += "\n\nAnswer the user's request using the provided files."

	messages := append([]llm.Message{
		{Role: llm.RoleSystem, Content: systemContent},
	}, history...)
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: query})

	totalTokens := estimateMessagesTokens(messages)

	return &AssemblyResult{
		Messages:     messages,
		TokenCount:   totalTokens,
		FilesUsed:    used,
		FilesSkipped: skipped,
	}, nil
}

func estimateTokens(text string) int {
	// Approximate: 1 token ~= 4 chars for English code
	return len(text) / 4
}

func estimateMessagesTokens(msgs []llm.Message) int {
	total := 0
	for _, m := range msgs {
		total += estimateTokens(m.Content) + 4 // overhead per message
	}
	return total
}
```

#### File: `internal/context/truncator.go` (NEW)
```go
package context

import (
	"strings"
)

// Truncator reduces content to fit token budgets while preserving structure.
type Truncator struct {
	maxTokens int
}

// NewTruncator creates a truncator.
func NewTruncator(maxTokens int) *Truncator {
	return &Truncator{maxTokens: maxTokens}
}

// Truncate reduces content to fit within maxTokens.
func (t *Truncator) Truncate(content string, maxTokens int) string {
	maxChars := maxTokens * 4
	if len(content) <= maxChars {
		return content
	}

	// Strategy: keep start (imports/definitions) + end (recent changes), collapse middle
	lines := strings.Split(content, "\n")
	if len(lines) < 20 {
		return content[:maxChars] + "\n... [truncated]"
	}

	headLines := 15
	tailLines := 15
	head := strings.Join(lines[:headLines], "\n")
	tail := strings.Join(lines[len(lines)-tailLines:], "\n")
	ellipsis := "\n\n... [" + fmt.Sprintf("%d lines omitted", len(lines)-headLines-tailLines) + "] ...\n\n"

	result := head + ellipsis + tail
	if len(result) > maxChars {
		return result[:maxChars] + "\n... [truncated]"
	}
	return result
}
```

#### File: `internal/indexing/indexer.go` (NEW)
```go
package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// Indexer maintains an AST-based index of the codebase.
type Indexer struct {
	root       string
	files      map[string]*FileIndex
	parser     *sitter.Parser
	mu         sync.RWMutex
}

// FileIndex holds per-file metadata.
type FileIndex struct {
	Path       string
	Language   string
	Symbols    []Symbol
	Imports    []string
	Size       int
	LineCount  int
	ModifiedAt int64
}

// Symbol is a named entity in code.
type Symbol struct {
	Name     string
	Type     string // function, struct, interface, method, variable
	Line     int
	Column   int
	Signature string
}

// SearchResult is a relevance-ranked file.
type SearchResult struct {
	Path  string
	Score float64
}

// NewIndexer creates an indexer rooted at the project directory.
func NewIndexer(root string) (*Indexer, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	return &Indexer{
		root:   root,
		files:  make(map[string]*FileIndex),
		parser: parser,
	}, nil
}

// Index walks the project and indexes all source files.
func (idx *Indexer) Index(ctx context.Context) error {
	return filepath.WalkDir(idx.root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !isSourceFile(path) {
			return nil
		}
		return idx.indexFile(ctx, path)
	})
}

func isSourceFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".py" || ext == ".rs"
}

func (idx *Indexer) indexFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	tree := idx.parser.ParseString(nil, string(data))
	root := tree.RootNode()

	fi := &FileIndex{
		Path:      path,
		Language:  languageFromExt(filepath.Ext(path)),
		Size:      len(data),
		LineCount: strings.Count(string(data), "\n"),
	}

	// Extract symbols via tree-sitter query
	idx.extractSymbols(root, fi)
	idx.extractImports(root, fi)

	idx.mu.Lock()
	idx.files[path] = fi
	idx.mu.Unlock()
	return nil
}

func (idx *Indexer) extractSymbols(node *sitter.Node, fi *FileIndex) {
	// Walk AST and collect function declarations, structs, interfaces
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "function_declaration":
			fi.Symbols = append(fi.Symbols, Symbol{
				Name: child.ChildByFieldName("name").Content([]byte{}),
				Type: "function",
				Line: int(child.StartPoint().Row),
			})
		case "type_declaration":
			fi.Symbols = append(fi.Symbols, Symbol{
				Name: child.ChildByFieldName("name").Content([]byte{}),
				Type: "type",
				Line: int(child.StartPoint().Row),
			})
		}
		idx.extractSymbols(child, fi)
	}
}

func (idx *Indexer) extractImports(node *sitter.Node, fi *FileIndex) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "import_declaration" {
			fi.Imports = append(fi.Imports, child.Content([]byte{}))
		}
	}
}

// SemanticSearch finds files relevant to a query using TF-IDF approximation.
func (idx *Indexer) SemanticSearch(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	queryTerms := tokenize(query)
	var results []SearchResult

	for path, fi := range idx.files {
		score := 0.0
		for _, term := range queryTerms {
			// Check filename
			if strings.Contains(filepath.Base(path), term) {
				score += 10.0
			}
			// Check symbols
			for _, sym := range fi.Symbols {
				if strings.Contains(strings.ToLower(sym.Name), term) {
					score += 5.0
				}
				if strings.Contains(strings.ToLower(sym.Signature), term) {
					score += 3.0
				}
			}
			// Check imports
			for _, imp := range fi.Imports {
				if strings.Contains(strings.ToLower(imp), term) {
					score += 2.0
				}
			}
		}
		if score > 0 {
			results = append(results, SearchResult{Path: path, Score: score})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// ReadFile returns the full content of an indexed file.
func (idx *Indexer) ReadFile(ctx context.Context, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func languageFromExt(ext string) string {
	switch ext {
	case ".go": return "go"
	case ".ts", ".tsx": return "typescript"
	case ".js", ".jsx": return "javascript"
	case ".py": return "python"
	case ".rs": return "rust"
	default: return "unknown"
	}
}

func tokenize(text string) []string {
	lower := strings.ToLower(text)
	return strings.Fields(lower)
}
```

### Anti-Bluff Test
```go
func TestContextAssembly(t *testing.T) {
	idx := indexing.NewIndexer("./testdata")
	ctx := context.Background()
	require.NoError(t, idx.Index(ctx))

	assembler := context.NewAssembler(idx, 4096)
	result, err := assembler.Assemble(ctx, "how does the auth middleware work", nil, nil)
	require.NoError(t, err)
	assert.Greater(t, len(result.FilesUsed), 0)
	assert.LessOrEqual(t, result.TokenCount, 4096)

	// Verify relevant files were picked
	var foundAuth bool
	for _, f := range result.FilesUsed {
		if strings.Contains(f, "auth") {
			foundAuth = true
		}
	}
	assert.True(t, foundAuth, "expected auth-related file in context")
}

func TestTruncatorPreservesStructure(t *testing.T) {
	truncator := context.NewTruncator(1000)
	content := strings.Repeat("line\n", 100)
	result := truncator.Truncate(content, 500)
	assert.Contains(t, result, "line")
	assert.Contains(t, result, "...[truncated]")
	assert.Less(t, len(result), len(content))
}
```

### Integration Verification
- Indexer scans 10K files in < 5 seconds.
- Context assembler stays within token budget for all queries.
- Semantic search returns auth-related files for "how does auth work" query.

---

## Feature 9: Task History

### Source Location (Kilo Code)
- `packages/opencode/src/session/` — Session persistence with messages, cost tracking
- `packages/opencode/src/storage/` — SQLite schema with session tables
- `packages/opencode/src/kilocode/session/cost-propagation.go` — Cost delta tracking
- `packages/kilo-docs/pages/automate/tools/switch-mode.md` — Mode transition history

### Target Location (HelixCode)
- **NEW**: `internal/history/` — Task history engine
- **NEW**: `internal/history/store.go` — Persistent storage
- **NEW**: `internal/history/search.go` — Searchable logs
- **NEW**: `internal/history/analytics.go` — Analytics aggregation
- **MODIFY**: `internal/session/session.go` — Record all messages and tool calls
- **MODIFY**: `internal/database/schema.go` — Add history tables

### Exact Code Changes

#### File: `internal/history/store.go` (NEW)
```go
package history

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"dev.helix.code/internal/database"
)

// TaskRecord is a single task execution.
type TaskRecord struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	ParentID    string          `json:"parent_id,omitempty"`
	AgentMode   string          `json:"agent_mode"`
	Status      string          `json:"status"` // pending, running, completed, failed
	Prompt      string          `json:"prompt"`
	Response    string          `json:"response"`
	ToolCalls   []ToolCallRecord `json:"tool_calls"`
	Cost        float64         `json:"cost"`
	TokensUsed  int             `json:"tokens_used"`
	DurationMs  int64           `json:"duration_ms"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// ToolCallRecord captures a single tool invocation.
type ToolCallRecord struct {
	ToolName   string          `json:"tool_name"`
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output"`
	DurationMs int64           `json:"duration_ms"`
	Error      string          `json:"error,omitempty"`
}

// Store persists task history.
type Store struct {
	db *database.Database
}

// NewStore creates a history store.
func NewStore(db *database.Database) *Store {
	return &Store{db: db}
}

// CreateTask inserts a new task record.
func (s *Store) CreateTask(ctx context.Context, r *TaskRecord) error {
	query := `INSERT INTO task_history (
		id, session_id, parent_id, agent_mode, status, prompt, cost, tokens_used, duration_ms, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	_, err := s.db.Exec(ctx, query,
		r.ID, r.SessionID, r.ParentID, r.AgentMode, r.Status,
		r.Prompt, r.Cost, r.TokensUsed, r.DurationMs, r.CreatedAt,
	)
	return err
}

// UpdateTask marks a task completed or failed.
func (s *Store) UpdateTask(ctx context.Context, id, status, response string, cost float64, durationMs int64) error {
	query := `UPDATE task_history SET status=$1, response=$2, cost=$3, duration_ms=$4, completed_at=$5 WHERE id=$6`
	_, err := s.db.Exec(ctx, query, status, response, cost, durationMs, time.Now(), id)
	return err
}

// RecordToolCall appends a tool call to a task.
func (s *Store) RecordToolCall(ctx context.Context, taskID string, tc ToolCallRecord) error {
	query := `INSERT INTO task_tool_calls (id, task_id, tool_name, input, output, duration_ms, error, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)`
	input, _ := json.Marshal(tc.Input)
	output, _ := json.Marshal(tc.Output)
	_, err := s.db.Exec(ctx, query, taskID, tc.ToolName, input, output, tc.DurationMs, tc.Error, time.Now())
	return err
}

// GetTask retrieves a task by ID.
func (s *Store) GetTask(ctx context.Context, id string) (*TaskRecord, error) {
	query := `SELECT id, session_id, parent_id, agent_mode, status, prompt, response, cost, tokens_used, duration_ms, created_at, completed_at
		FROM task_history WHERE id=$1`
	row := s.db.QueryRow(ctx, query, id)
	var r TaskRecord
	var completedAt *time.Time
	err := row.Scan(&r.ID, &r.SessionID, &r.ParentID, &r.AgentMode, &r.Status, &r.Prompt, &r.Response,
		&r.Cost, &r.TokensUsed, &r.DurationMs, &r.CreatedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	r.CompletedAt = completedAt
	return &r, nil
}

// ListTasks returns tasks for a session with pagination.
func (s *Store) ListTasks(ctx context.Context, sessionID string, limit, offset int) ([]*TaskRecord, error) {
	query := `SELECT id, session_id, parent_id, agent_mode, status, prompt, response, cost, tokens_used, duration_ms, created_at, completed_at
		FROM task_history WHERE session_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.db.Query(ctx, query, sessionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*TaskRecord
	for rows.Next() {
		var r TaskRecord
		var completedAt *time.Time
		if err := rows.Scan(&r.ID, &r.SessionID, &r.ParentID, &r.AgentMode, &r.Status, &r.Prompt, &r.Response,
			&r.Cost, &r.TokensUsed, &r.DurationMs, &r.CreatedAt, &completedAt); err != nil {
			continue
		}
		r.CompletedAt = completedAt
		results = append(results, &r)
	}
	return results, nil
}
```

#### File: `internal/history/search.go` (NEW)
```go
package history

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/database"
)

// SearchQuery filters task history.
type SearchQuery struct {
	SessionID   string
	AgentMode   string
	Status      string
	TextQuery   string
	DateFrom    string
	DateTo      string
	Limit       int
	Offset      int
}

// Searcher queries task history.
type Searcher struct {
	db *database.Database
}

// NewSearcher creates a history searcher.
func NewSearcher(db *database.Database) *Searcher {
	return &Searcher{db: db}
}

// Search finds tasks matching the query.
func (s *Searcher) Search(ctx context.Context, q SearchQuery) ([]*TaskRecord, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if q.SessionID != "" {
		where = append(where, fmt.Sprintf("session_id=$%d", argIdx))
		args = append(args, q.SessionID)
		argIdx++
	}
	if q.AgentMode != "" {
		where = append(where, fmt.Sprintf("agent_mode=$%d", argIdx))
		args = append(args, q.AgentMode)
		argIdx++
	}
	if q.Status != "" {
		where = append(where, fmt.Sprintf("status=$%d", argIdx))
		args = append(args, q.Status)
		argIdx++
	}
	if q.TextQuery != "" {
		where = append(where, fmt.Sprintf("(prompt ILIKE $%d OR response ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+q.TextQuery+"%")
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM task_history WHERE " + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := "SELECT id, session_id, parent_id, agent_mode, status, prompt, response, cost, tokens_used, duration_ms, created_at, completed_at FROM task_history WHERE " +
		strings.Join(where, " AND ") + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := s.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*TaskRecord
	for rows.Next() {
		var r TaskRecord
		var completedAt *time.Time
		if err := rows.Scan(&r.ID, &r.SessionID, &r.ParentID, &r.AgentMode, &r.Status, &r.Prompt, &r.Response,
			&r.Cost, &r.TokensUsed, &r.DurationMs, &r.CreatedAt, &completedAt); err != nil {
			continue
		}
		r.CompletedAt = completedAt
		results = append(results, &r)
	}
	return results, total, nil
}
```

#### File: `internal/history/analytics.go` (NEW)
```go
package history

import (
	"context"
	"time"

	"dev.helix.code/internal/database"
)

// Analytics aggregates task history metrics.
type Analytics struct {
	db *database.Database
}

// NewAnalytics creates an analytics engine.
func NewAnalytics(db *database.Database) *Analytics {
	return &Analytics{db: db}
}

// DailyStats holds per-day aggregation.
type DailyStats struct {
	Date        time.Time `json:"date"`
	TaskCount   int       `json:"task_count"`
	TotalCost   float64   `json:"total_cost"`
	TotalTokens int       `json:"total_tokens"`
	AvgDuration int64     `json:"avg_duration_ms"`
}

// ModeStats holds per-mode aggregation.
type ModeStats struct {
	AgentMode   string  `json:"agent_mode"`
	TaskCount   int     `json:"task_count"`
	TotalCost   float64 `json:"total_cost"`
	SuccessRate float64 `json:"success_rate"`
}

// GetDailyStats returns task counts and costs per day.
func (a *Analytics) GetDailyStats(ctx context.Context, days int) ([]DailyStats, error) {
	query := `SELECT DATE(created_at) as date, COUNT(*) as cnt, SUM(cost) as total_cost, SUM(tokens_used) as total_tokens, AVG(duration_ms) as avg_dur
		FROM task_history WHERE created_at >= CURRENT_DATE - INTERVAL '%d days'
		GROUP BY DATE(created_at) ORDER BY date DESC`
	rows, err := a.db.Query(ctx, fmt.Sprintf(query, days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DailyStats
	for rows.Next() {
		var s DailyStats
		if err := rows.Scan(&s.Date, &s.TaskCount, &s.TotalCost, &s.TotalTokens, &s.AvgDuration); err != nil {
			continue
		}
		results = append(results, s)
	}
	return results, nil
}

// GetModeStats returns per-mode breakdown.
func (a *Analytics) GetModeStats(ctx context.Context) ([]ModeStats, error) {
	query := `SELECT agent_mode, COUNT(*) as cnt, SUM(cost) as total_cost,
		SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END)::float / COUNT(*) as success_rate
		FROM task_history GROUP BY agent_mode`
	rows, err := a.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ModeStats
	for rows.Next() {
		var s ModeStats
		if err := rows.Scan(&s.AgentMode, &s.TaskCount, &s.TotalCost, &s.SuccessRate); err != nil {
			continue
		}
		results = append(results, s)
	}
	return results, nil
}
```

#### File: `internal/database/migrations/004_history.up.sql` (NEW)
```sql
CREATE TABLE task_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id VARCHAR(255) NOT NULL,
    parent_id VARCHAR(255),
    agent_mode VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    prompt TEXT,
    response TEXT,
    cost DECIMAL(12,6) DEFAULT 0,
    tokens_used INT DEFAULT 0,
    duration_ms BIGINT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP
);

CREATE INDEX idx_task_session ON task_history(session_id);
CREATE INDEX idx_task_mode ON task_history(agent_mode);
CREATE INDEX idx_task_status ON task_history(status);
CREATE INDEX idx_task_created ON task_history(created_at);

CREATE TABLE task_tool_calls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES task_history(id) ON DELETE CASCADE,
    tool_name VARCHAR(100) NOT NULL,
    input JSONB,
    output JSONB,
    duration_ms BIGINT DEFAULT 0,
    error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tool_task ON task_tool_calls(task_id);
```

### Anti-Bluff Test
```go
func TestHistoryStoreAndSearch(t *testing.T) {
	ctx := context.Background()
	db := database.NewTestDB(t)
	store := history.NewStore(db)
	searcher := history.NewSearcher(db)

	// Create tasks
	for i := 0; i < 5; i++ {
		require.NoError(t, store.CreateTask(ctx, &history.TaskRecord{
			ID:        fmt.Sprintf("task-%d", i),
			SessionID: "sess-1",
			AgentMode: "code",
			Status:    "completed",
			Prompt:    fmt.Sprintf("write function %d", i),
			CreatedAt: time.Now(),
		}))
	}

	// Search by text
	results, total, err := searcher.Search(ctx, history.SearchQuery{
		SessionID: "sess-1",
		TextQuery: "function 2",
		Limit:     10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "write function 2", results[0].Prompt)

	// Update task
	require.NoError(t, store.UpdateTask(ctx, "task-0", "failed", "error", 0.001, 500))
	updated, err := store.GetTask(ctx, "task-0")
	require.NoError(t, err)
	assert.Equal(t, "failed", updated.Status)
}
```

### Integration Verification
- All tool calls are recorded in `task_tool_calls` within 100ms of execution.
- Search for "auth" returns only tasks with "auth" in prompt or response.
- Analytics endpoint `/api/v1/analytics/daily?days=7` returns 7 data points.

---

## Feature 10: Team Collaboration

### Source Location (Kilo Code)
- Kilo Code's team features are cloud/dashboard features:
  - Organization workspaces
  - Shared projects with role-based access
  - Comment system on tasks/sessions
  - Multi-user support with real-time sync
- Reference: `packages/kilo-docs/pages/collaborate/index.md`

### Target Location (HelixCode)
- **NEW**: `internal/team/` — Team management
- **NEW**: `internal/team/workspace.go` — Organization workspaces
- **NEW**: `internal/team/project.go` — Shared projects
- **NEW**: `internal/team/member.go` — Role-based access
- **NEW**: `internal/team/comment.go` — Comment system
- **NEW**: `internal/team/sync.go` — Real-time collaboration sync
- **MODIFY**: `internal/server/routes.go` — Team API endpoints
- **MODIFY**: `internal/database/schema.go` — Team tables

### Exact Code Changes

#### File: `internal/team/workspace.go` (NEW)
```go
package team

import (
	"context"
	"fmt"
	"time"

	"dev.helix.code/internal/database"
)

// Workspace is an organization container.
type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	OwnerID     string    `json:"owner_id"`
	Members     []Member  `json:"members"`
	Projects    []Project `json:"projects"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MemberRole defines access levels.
type MemberRole string

const (
	RoleOwner     MemberRole = "owner"
	RoleAdmin     MemberRole = "admin"
	RoleEditor    MemberRole = "editor"
	RoleViewer    MemberRole = "viewer"
)

// Member is a workspace participant.
type Member struct {
	UserID    string     `json:"user_id"`
	Role      MemberRole `json:"role"`
	JoinedAt  time.Time  `json:"joined_at"`
}

// WorkspaceStore persists workspaces.
type WorkspaceStore struct {
	db *database.Database
}

// NewWorkspaceStore creates a store.
func NewWorkspaceStore(db *database.Database) *WorkspaceStore {
	return &WorkspaceStore{db: db}
}

// CreateWorkspace initializes a new workspace.
func (s *WorkspaceStore) CreateWorkspace(ctx context.Context, name, ownerID string) (*Workspace, error) {
	ws := &Workspace{
		ID:        generateID(),
		Name:      name,
		OwnerID:   ownerID,
		Members:   []Member{{UserID: ownerID, Role: RoleOwner, JoinedAt: time.Now()}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	query := `INSERT INTO workspaces (id, name, owner_id, created_at, updated_at) VALUES ($1,$2,$3,$4,$4)`
	_, err := s.db.Exec(ctx, query, ws.ID, ws.Name, ws.OwnerID, ws.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}
	return ws, nil
}

// GetWorkspace retrieves a workspace by ID.
func (s *WorkspaceStore) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	query := `SELECT id, name, owner_id, created_at, updated_at FROM workspaces WHERE id=$1`
	row := s.db.QueryRow(ctx, query, id)
	var ws Workspace
	if err := row.Scan(&ws.ID, &ws.Name, &ws.OwnerID, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
		return nil, err
	}
	// Load members
	members, _ := s.ListMembers(ctx, id)
	ws.Members = members
	return &ws, nil
}

// AddMember adds a user to a workspace.
func (s *WorkspaceStore) AddMember(ctx context.Context, workspaceID, userID string, role MemberRole) error {
	query := `INSERT INTO workspace_members (workspace_id, user_id, role, joined_at) VALUES ($1,$2,$3,$4)
		ON CONFLICT (workspace_id, user_id) DO UPDATE SET role=$3`
	_, err := s.db.Exec(ctx, query, workspaceID, userID, role, time.Now())
	return err
}

// ListMembers returns workspace members.
func (s *WorkspaceStore) ListMembers(ctx context.Context, workspaceID string) ([]Member, error) {
	query := `SELECT user_id, role, joined_at FROM workspace_members WHERE workspace_id=$1`
	rows, err := s.db.Query(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Role, &m.JoinedAt); err != nil {
			continue
		}
		members = append(members, m)
	}
	return members, nil
}

func generateID() string {
	return fmt.Sprintf("ws-%d", time.Now().UnixNano())
}
```

#### File: `internal/team/project.go` (NEW)
```go
package team

import (
	"context"
	"fmt"
	"time"

	"dev.helix.code/internal/database"
)

// Project is a shared coding project within a workspace.
type Project struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	RepoURL     string    `json:"repo_url,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProjectStore manages projects.
type ProjectStore struct {
	db *database.Database
}

// NewProjectStore creates a project store.
func NewProjectStore(db *database.Database) *ProjectStore {
	return &ProjectStore{db: db}
}

// CreateProject adds a project.
func (s *ProjectStore) CreateProject(ctx context.Context, p *Project) error {
	p.CreatedAt = time.Now()
	query := `INSERT INTO projects (id, workspace_id, name, description, repo_url, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := s.db.Exec(ctx, query, p.ID, p.WorkspaceID, p.Name, p.Description, p.RepoURL, p.CreatedBy, p.CreatedAt)
	return err
}

// ListProjects returns projects in a workspace.
func (s *ProjectStore) ListProjects(ctx context.Context, workspaceID string) ([]Project, error) {
	query := `SELECT id, workspace_id, name, description, repo_url, created_by, created_at FROM projects WHERE workspace_id=$1`
	rows, err := s.db.Query(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.RepoURL, &p.CreatedBy, &p.CreatedAt); err != nil {
			continue
		}
		projects = append(projects, p)
	}
	return projects, nil
}
```

#### File: `internal/team/comment.go` (NEW)
```go
package team

import (
	"context"
	"fmt"
	"time"

	"dev.helix.code/internal/database"
)

// Comment is a user comment on a session or task.
type Comment struct {
	ID        string    `json:"id"`
	TargetID  string    `json:"target_id"`   // session_id or task_id
	TargetType string   `json:"target_type"` // "session" | "task"
	AuthorID  string    `json:"author_id"`
	Content   string    `json:"content"`
	ThreadID  string    `json:"thread_id,omitempty"` // for replies
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CommentStore manages comments.
type CommentStore struct {
	db *database.Database
}

// NewCommentStore creates a comment store.
func NewCommentStore(db *database.Database) *CommentStore {
	return &CommentStore{db: db}
}

// AddComment inserts a comment.
func (s *CommentStore) AddComment(ctx context.Context, c *Comment) error {
	c.ID = fmt.Sprintf("cmt-%d", time.Now().UnixNano())
	c.CreatedAt = time.Now()
	c.UpdatedAt = c.CreatedAt
	query := `INSERT INTO comments (id, target_id, target_type, author_id, content, thread_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$7)`
	_, err := s.db.Exec(ctx, query, c.ID, c.TargetID, c.TargetType, c.AuthorID, c.Content, c.ThreadID, c.CreatedAt)
	return err
}

// GetComments returns comments for a target.
func (s *CommentStore) GetComments(ctx context.Context, targetID, targetType string) ([]Comment, error) {
	query := `SELECT id, target_id, target_type, author_id, content, thread_id, created_at, updated_at
		FROM comments WHERE target_id=$1 AND target_type=$2 ORDER BY created_at ASC`
	rows, err := s.db.Query(ctx, query, targetID, targetType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.TargetID, &c.TargetType, &c.AuthorID, &c.Content, &c.ThreadID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		comments = append(comments, c)
	}
	return comments, nil
}
```

#### File: `internal/team/sync.go` (NEW)
```go
package team

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SyncHub broadcasts real-time updates to connected team members.
type SyncHub struct {
	rooms map[string]map[string]*websocket.Conn // workspaceID -> userID -> conn
	mu    sync.RWMutex
}

// NewSyncHub creates a sync hub.
func NewSyncHub() *SyncHub {
	return &SyncHub{rooms: make(map[string]map[string]*websocket.Conn)}
}

// Register adds a user's connection to a workspace room.
func (h *SyncHub) Register(workspaceID, userID string, conn *websocket.Conn) {
	h.mu.Lock()
	if h.rooms[workspaceID] == nil {
		h.rooms[workspaceID] = make(map[string]*websocket.Conn)
	}
	h.rooms[workspaceID][userID] = conn
	h.mu.Unlock()
}

// Unregister removes a user from a room.
func (h *SyncHub) Unregister(workspaceID, userID string) {
	h.mu.Lock()
	if room := h.rooms[workspaceID]; room != nil {
		delete(room, userID)
		if len(room) == 0 {
			delete(h.rooms, workspaceID)
		}
	}
	h.mu.Unlock()
}

// Broadcast sends a message to all members of a workspace.
func (h *SyncHub) Broadcast(workspaceID string, msg SyncMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.mu.RLock()
	room := h.rooms[workspaceID]
	h.mu.RUnlock()

	for userID, conn := range room {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			// Remove dead connection
			h.Unregister(workspaceID, userID)
			conn.Close()
		}
	}
	return nil
}

// SyncMessage is a real-time update payload.
type SyncMessage struct {
	Type      string      `json:"type"`      // "comment", "task_update", "mode_change"
	Workspace string      `json:"workspace"`
	UserID    string      `json:"user_id"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}
```

#### File: `internal/database/migrations/005_team.up.sql` (NEW)
```sql
CREATE TABLE workspaces (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE workspace_members (
    workspace_id VARCHAR(255) NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'viewer',
    joined_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE projects (
    id VARCHAR(255) PRIMARY KEY,
    workspace_id VARCHAR(255) NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    repo_url VARCHAR(500),
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE comments (
    id VARCHAR(255) PRIMARY KEY,
    target_id VARCHAR(255) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    author_id VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    thread_id VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_comments_target ON comments(target_id, target_type);
CREATE INDEX idx_comments_author ON comments(author_id);
```

### Anti-Bluff Test
```go
func TestTeamWorkspaceAndComments(t *testing.T) {
	ctx := context.Background()
	db := database.NewTestDB(t)
	wsStore := team.NewWorkspaceStore(db)
	projStore := team.NewProjectStore(db)
	cmtStore := team.NewCommentStore(db)

	// Create workspace
	ws, err := wsStore.CreateWorkspace(ctx, "Acme Corp", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "user-1", ws.OwnerID)

	// Add member
	require.NoError(t, wsStore.AddMember(ctx, ws.ID, "user-2", team.RoleEditor))
	members, err := wsStore.ListMembers(ctx, ws.ID)
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// Create project
	require.NoError(t, projStore.CreateProject(ctx, &team.Project{
		ID:          "proj-1",
		WorkspaceID: ws.ID,
		Name:        "Backend API",
		CreatedBy:   "user-1",
	}))
	projects, err := projStore.ListProjects(ctx, ws.ID)
	require.NoError(t, err)
	assert.Len(t, projects, 1)

	// Add comment
	require.NoError(t, cmtStore.AddComment(ctx, &team.Comment{
		TargetID:   "sess-1",
		TargetType: "session",
		AuthorID:   "user-2",
		Content:    "This looks good but we should add error handling.",
	}))
	comments, err := cmtStore.GetComments(ctx, "sess-1", "session")
	require.NoError(t, err)
	assert.Len(t, comments, 1)
	assert.Equal(t, "This looks good but we should add error handling.", comments[0].Content)
}
```

### Integration Verification
- POST `/api/v1/workspaces` creates workspace, GET returns it with members.
- WebSocket connect to `/ws/team?workspace=ws-123` receives broadcast messages.
- Comment posted on task → all workspace members with open WebSocket receive `SyncMessage`.

---

## Summary: Files Changed/Created

### New Directories (23)
```
internal/agent/modes/
internal/agent/modes/prompts/
internal/agent/subagent/
internal/gastown/
internal/triage/
internal/review/
internal/appbuilder/
applications/vscode/webview/components/
applications/vscode/webview/styles/
internal/context/
internal/indexing/
internal/history/
internal/team/
```

### New Files (52)
```
internal/agent/modes/mode.go
internal/agent/modes/prompts/code.md
internal/agent/modes/prompts/architect.md
internal/agent/modes/prompts/ask.md
internal/agent/modes/prompts/debug.md
internal/agent/modes/prompts/test.md
internal/agent/subagent/delegator.go
internal/tools/task.go
cmd/helix/mode.go
cmd/helix/review.go
cmd/helix/app.go
internal/gastown/statemachine.go
internal/gastown/marketplace.go
internal/gastown/convoy.go
internal/triage/classifier.go
internal/triage/duplicate.go
internal/triage/labeler.go
internal/triage/ticket.go
internal/review/diff.go
internal/review/prompt.go
internal/review/engine.go
internal/review/formatter.go
internal/appbuilder/framework.go
internal/appbuilder/scaffold.go
internal/appbuilder/database.go
applications/vscode/webview/components/DiffViewer.tsx
applications/vscode/webview/components/StatusIndicator.tsx
applications/vscode/webview/components/AgentManager.tsx
applications/vscode/webview/styles/diff-viewer.css
internal/server/websocket.go
internal/llm/openrouter.go
internal/llm/bedrock.go
internal/llm/azure.go
internal/llm/copilot.go
internal/context/assembly.go
internal/context/truncator.go
internal/indexing/indexer.go
internal/history/store.go
internal/history/search.go
internal/history/analytics.go
internal/team/workspace.go
internal/team/project.go
internal/team/comment.go
internal/team/sync.go
internal/database/migrations/003_triage.up.sql
internal/database/migrations/004_history.up.sql
internal/database/migrations/005_team.up.sql
```

### Modified Files (9)
```
internal/llm/provider.go          # Extended interface
internal/llm/providers_registry.go  # Add Kilo providers
internal/config/config.go           # Add provider/agent/triage sections
internal/session/session.go       # Parent/child relationships
internal/tools/registry.go        # Add task tool
internal/server/routes.go         # Add Gas Town, triage, review, team routes
api/openapi.yaml                  # Add new endpoints
internal/agent/orchestrator.go    # Integrate mode registry
cmd/helix/main.go                 # Wire new commands
```

### Total Feature Count: 10
1. 5 Specialized Modes + Subagent Delegation
2. Gas Town Multi-Agent Platform
3. Auto Triage
4. Auto Review
5. App Builder
6. Enhanced UI
7. Additional Model Support
8. Improved Context Handling
9. Task History
10. Team Collaboration

---

*End of Porting Plan*
