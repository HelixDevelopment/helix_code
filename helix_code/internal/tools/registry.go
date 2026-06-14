package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/autocommit"
	"dev.helix.code/internal/hooks"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/mcp"
	"dev.helix.code/internal/telemetry"
	"dev.helix.code/internal/tools/browser"
	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/filesystem"
	"dev.helix.code/internal/tools/mapping"
	"dev.helix.code/internal/tools/multiedit"
	"dev.helix.code/internal/tools/shell"
	"dev.helix.code/internal/tools/web"
	"dev.helix.code/internal/workflow"
	"dev.helix.code/internal/workflow/planmode"
)

// Tool represents a unified interface for all tools
type Tool interface {
	// Name returns the tool name
	Name() string

	// Description returns a brief description of what the tool does
	Description() string

	// Execute executes the tool with given parameters
	Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)

	// Schema returns the JSON schema for the tool's parameters
	Schema() ToolSchema

	// Category returns the tool category
	Category() ToolCategory

	// Validate validates the parameters before execution
	Validate(params map[string]interface{}) error

	// RequiresApproval returns the approval level the tool requires (P2-F21).
	// LevelReadOnly bypasses the approval gate (pure reads); LevelEdit gates
	// behind ModeAutoEdit or higher; LevelRun gates behind ModeFullAuto;
	// LevelAll gates behind ModeDangerous (escape hatch only). Tools that do
	// not explicitly classify themselves should embed approval.DefaultLevelEdit
	// for the safe default (LevelEdit).
	//
	// Per spec 7128289 §3.6, the explicit-override table is:
	//   LevelReadOnly: fs_read, glob, grep, codebase_map, file_definitions,
	//                  lsp_*, web_fetch, web_search, notebook_read,
	//                  browser_screenshot, task_tracker, TaskOutput,
	//                  ListWorktrees, ask_user.
	//   LevelEdit:     fs_write, fs_edit, multiedit_*, notebook_edit,
	//                  smart_edit, EnterPlanMode, ExitPlanMode,
	//                  EnterWorktree, ExitWorktree, RemoveWorktree, mcp_*.
	//   LevelRun:      shell, shell_background, shell_output, shell_kill,
	//                  shell_sandboxed, browser_launch, browser_navigate,
	//                  browser_close, TaskStop.
	//   LevelAll:      task (subagent dispatch — recursive agent spawn).
	RequiresApproval() approval.ApprovalLevel
}

// ToolSchema defines the JSON schema for tool parameters
type ToolSchema struct {
	Type        string                 `json:"type"`
	Properties  map[string]interface{} `json:"properties"`
	Required    []string               `json:"required"`
	Description string                 `json:"description"`
}

// ToolCategory represents the category of a tool
type ToolCategory string

const (
	CategoryFileSystem   ToolCategory = "filesystem"
	CategoryShell        ToolCategory = "shell"
	CategoryWeb          ToolCategory = "web"
	CategoryBrowser      ToolCategory = "browser"
	CategoryMapping      ToolCategory = "mapping"
	CategoryMultiEdit    ToolCategory = "multiedit"
	CategoryConfirmation ToolCategory = "confirmation"
	CategoryNotebook     ToolCategory = "notebook"
	CategoryInteractive  ToolCategory = "interactive"
	CategoryLSP          ToolCategory = "lsp"
	CategorySandbox      ToolCategory = "sandbox"
	CategorySubagent     ToolCategory = "subagent"
	CategorySmartEdit    ToolCategory = "smart-edit"
	CategoryAskUser      ToolCategory = "ask-user"
)

// ToolRegistry manages all available tools
type ToolRegistry struct {
	tools   map[string]Tool
	aliases map[string]string // alias -> tool name
	mu      sync.RWMutex

	// Hook lifecycle manager (optional; nil = passthrough).
	hooksManager *hooks.Manager

	// bgManager routes run_in_background:true calls (F07). Optional; nil
	// disables background dispatch (Execute returns ErrNoBackgroundMgr).
	bgManager *workflow.BackgroundManager

	// planGate is the optional plan-mode gate (F08). When set, Execute
	// consults it before running any tool; blocked calls return ErrPlanModeGated.
	planGate *planmode.ToolGate

	// lspManager is the optional LSP manager (F13). When set, Execute fires
	// a post-success NotifyChange for Edit-class tools (fs_edit / fs_write /
	// multiedit_commit) so subsequent calls to LSPManager.GetDiagnostics see
	// fresh state. nil disables the auto-trigger.
	lspManager *LSPManager

	// toolTelemetry is the optional F16 telemetry instrumentation. When set,
	// Execute wraps each successful-Validate tool dispatch in a span + metric
	// pair via toolTelemetry.Begin. nil disables the wrap (Execute behaves as
	// before).
	toolTelemetry *telemetry.ToolInstrumentation

	// approvalMgr is the optional F21 approval gate. When set, Execute
	// consults it BEFORE invoking the inner tool: read-only tools bypass the
	// gate, edit/run/all-level tools are routed through CheckApproval and
	// (when matrix dictates) PromptForApproval. ModeFullAuto + Run/All also
	// causes Execute to inject the sandbox markers
	// "_helix_sandbox_required"=true and "_helix_sandbox_network_allowed"=
	// false into the args map so downstream sandbox-aware tools wrap the
	// invocation. nil disables the gate (Execute behaves exactly as before).
	approvalMgr *approval.ApprovalManager

	// autoCommitter is the optional F22 git auto-commit committer. When
	// set, Execute fires a post-success MaybeCommit call adjacent to the
	// F13 LSP auto-trigger for Edit-class (LevelEdit / LevelAll) tools.
	// Failures from the committer are logged at WARN and never propagate
	// to the calling tool. nil disables auto-commit.
	autoCommitter *autocommit.AutoCommitter

	// Component instances
	filesystem   *filesystem.FileSystemTools
	shell        *shell.ShellExecutor
	web          *web.WebTools
	browser      *browser.BrowserTools
	mapper       mapping.Mapper
	multiEdit    *multiedit.MultiFileEditor
	confirmation *confirmation.ConfirmationCoordinator
}

// RegistryConfig contains configuration for the tool registry
type RegistryConfig struct {
	FileSystemConfig   *filesystem.Config
	ShellConfig        *shell.Config
	WebConfig          *web.Config
	BrowserConfig      *browser.Config
	MappingWorkspace   string
	MultiEditConfig    *multiedit.Config
	ConfirmationConfig *confirmation.Config
}

// DefaultRegistryConfig returns default registry configuration
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		FileSystemConfig:   filesystem.DefaultConfig(),
		ShellConfig:        shell.DefaultConfig(),
		WebConfig:          web.DefaultConfig(),
		BrowserConfig:      browser.DefaultConfig(),
		MappingWorkspace:   "",
		MultiEditConfig:    multiedit.DefaultConfig(),
		ConfirmationConfig: confirmation.DefaultConfig(),
	}
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(config *RegistryConfig) (*ToolRegistry, error) {
	if config == nil {
		config = DefaultRegistryConfig()
	}

	registry := &ToolRegistry{
		tools:   make(map[string]Tool),
		aliases: make(map[string]string),
	}

	// Initialize components
	if err := registry.initializeComponents(config); err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	// Register all tools
	if err := registry.registerAllTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return registry, nil
}

// initializeComponents initializes all tool components
func (r *ToolRegistry) initializeComponents(config *RegistryConfig) error {
	var err error

	// Initialize filesystem
	r.filesystem, err = filesystem.NewFileSystemTools(config.FileSystemConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize filesystem: %w", err)
	}

	// Initialize shell
	r.shell = shell.NewShellExecutor(config.ShellConfig)

	// Initialize web
	r.web, err = web.NewWebTools(config.WebConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize web: %w", err)
	}

	// Initialize browser
	r.browser = browser.NewBrowserTools(config.BrowserConfig)

	// Initialize mapper
	workspace := config.MappingWorkspace
	if workspace == "" {
		workspace = config.FileSystemConfig.WorkspaceRoot
	}
	r.mapper = mapping.NewMapper(workspace)

	// Initialize multi-edit
	r.multiEdit, err = multiedit.NewMultiFileEditor(
		multiedit.WithConfig(config.MultiEditConfig),
		multiedit.WithFileSystem(r.filesystem),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize multi-edit: %w", err)
	}

	// Initialize confirmation
	r.confirmation = confirmation.NewConfirmationCoordinator()

	return nil
}

// registerAllTools registers all available tools
func (r *ToolRegistry) registerAllTools() error {
	// File System Tools
	r.Register(&FSReadTool{registry: r})
	r.Register(&FSWriteTool{registry: r})
	r.Register(&FSEditTool{registry: r})
	r.Register(&GlobTool{registry: r})
	r.Register(&GrepTool{registry: r})

	// Shell Tools
	r.Register(&ShellTool{registry: r})
	r.Register(&ShellBackgroundTool{registry: r})
	r.Register(&ShellOutputTool{registry: r})
	r.Register(&ShellKillTool{registry: r})

	// Web Tools
	r.Register(&WebFetchTool{registry: r})
	r.Register(&WebSearchTool{registry: r})

	// Browser Tools
	r.Register(&BrowserLaunchTool{registry: r})
	r.Register(&BrowserNavigateTool{registry: r})
	r.Register(&BrowserScreenshotTool{registry: r})
	r.Register(&BrowserCloseTool{registry: r})

	// Mapping Tools
	r.Register(&CodebaseMapTool{registry: r})
	r.Register(&FileDefinitionsTool{registry: r})

	// Multi-Edit Tools
	r.Register(&MultiEditBeginTool{registry: r})
	r.Register(&MultiEditAddTool{registry: r})
	r.Register(&MultiEditPreviewTool{registry: r})
	r.Register(&MultiEditCommitTool{registry: r})

	// Interactive Tools
	// NOTE: ask_user is intentionally NOT auto-registered here. The real
	// askuser.AskUserTool is wired in cmd/cli/main.go via NewAskUserTool +
	// NewStdinPrompter so it has access to the process stdin/stdout. The
	// previous in-tree bluff stub (which returned the question struct
	// without prompting) has been removed (P1-F19-T05).
	r.Register(&TaskTrackerTool{registry: r})

	// Notebook Tools
	r.Register(&NotebookReadTool{registry: r})
	r.Register(&NotebookEditTool{registry: r})

	return nil
}

// Register registers a tool
func (r *ToolRegistry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// RegisterAlias registers an alias for a tool
func (r *ToolRegistry) RegisterAlias(alias, toolName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[toolName]; !exists {
		return fmt.Errorf("tool %s not found", toolName)
	}

	r.aliases[alias] = toolName
	return nil
}

// Get retrieves a tool by name or alias
func (r *ToolRegistry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check aliases first
	if actualName, ok := r.aliases[name]; ok {
		name = actualName
	}

	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return tool, nil
}

// SetHooksManager wires a hooks.Manager so Execute can fire lifecycle events
// (BeforeToolCall / AfterToolCall plus specialised BeforeBash/AfterBash for
// Bash and BeforeEdit/AfterEdit for Edit/Write/MultiEdit). A nil manager
// disables hook firing (Execute behaves as before).
func (r *ToolRegistry) SetHooksManager(m *hooks.Manager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooksManager = m
}

// SetBackgroundManager wires a BackgroundManager. Calls to Execute with
// run_in_background:true require this to be set. Optional; nil disables
// background dispatch (Execute returns ErrNoBackgroundMgr in that case).
func (r *ToolRegistry) SetBackgroundManager(m *workflow.BackgroundManager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bgManager = m
}

// SetPlanModeGate wires a plan-mode gate. When set, Execute consults it before
// running any tool; blocked calls return ErrPlanModeGated.
func (r *ToolRegistry) SetPlanModeGate(g *planmode.ToolGate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.planGate = g
}

// SetLSPManager wires an LSPManager onto the registry. Once set, every
// successful Execute call for an Edit-class tool (fs_edit, fs_write,
// multiedit_commit) triggers a NotifyChange on the manager so subsequent
// calls to LSPManager.GetDiagnostics see fresh state for the edited files.
//
// Pass nil to disable the auto-trigger.
//
// The trigger is best-effort and synchronous, capped at autoTriggerTimeout
// (2s). It never propagates LSP-side errors back to the Execute caller —
// diagnostics are a side-channel surfaced via separate tooling
// (lsp_get_diagnostics).
func (r *ToolRegistry) SetLSPManager(m *LSPManager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lspManager = m
}

// SetTelemetryInstrumentation wires F16 telemetry onto the registry. Once set,
// Execute wraps each successful-Validate tool dispatch in an OTel span +
// helixcode_tool_calls_total / helixcode_tool_latency_seconds metrics labelled
// with outcome={success|failure}.
//
// Pass nil to disable instrumentation (Execute behaves exactly as before).
//
// Telemetry is a side-channel: errors are NEVER propagated back to the
// Execute caller (the OTel SDK doesn't return errors from span.End/Record/Add
// in normal use; the noop tracer/meter make every operation a no-op when the
// provider is in noop mode).
func (r *ToolRegistry) SetTelemetryInstrumentation(t *telemetry.ToolInstrumentation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.toolTelemetry = t
}

// SetApprovalManager wires the F21 approval gate. Once set, Execute consults
// the manager BEFORE invoking the inner tool. ApprovalLevel.LevelReadOnly
// short-circuits the gate (matrix permits it in every mode); other levels
// route through CheckApproval/PromptForApproval.
//
// Sandbox markers: when the active mode is ModeFullAuto AND the tool's
// RequiresApproval level is LevelRun or LevelAll, Execute injects the
// sentinels "_helix_sandbox_required"=true and "_helix_sandbox_network_allowed"=
// false into the args map so downstream sandbox-aware tools (F14
// shell_sandboxed) wrap the invocation. Tools that ignore the markers are
// unaffected; nil-args inputs are upgraded to a fresh map. The markers are
// documented in spec 7128289 §11 (Non-obvious calls).
//
// Pass nil to disable the gate (Execute behaves exactly as before).
func (r *ToolRegistry) SetApprovalManager(a *approval.ApprovalManager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.approvalMgr = a
}

// SetAutoCommitter wires the F22 per-edit git auto-commit. Once set,
// Execute fires a post-success MaybeCommit hook for Edit-class tools
// (LevelEdit / LevelAll). The hook is a no-op when:
//   - the committer's enabled flag is false (env or /git_auto_commit off);
//   - the working dir is not a git repo;
//   - the working tree is clean;
//   - params[autocommit.SkipParamKey] is true.
//
// Failures inside MaybeCommit are logged at WARN and NEVER propagated to
// the calling tool — auto-commit is best-effort.
//
// Pass nil to disable the auto-commit hook.
func (r *ToolRegistry) SetAutoCommitter(c *autocommit.AutoCommitter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.autoCommitter = c
}

// fireAutoCommit fires the F22 auto-commit pipeline after a successful
// edit-class tool execution. Best-effort: any error from MaybeCommit is
// logged at WARN and discarded.
//
// Filters:
//   - nil committer → no-op
//   - tool.RequiresApproval() not in {LevelEdit, LevelAll} → no-op
//   - params[autocommit.SkipParamKey] == true → no-op
//
// MutatedPaths is derived via derivePaths() — see that function for the
// per-tool table.
func (r *ToolRegistry) fireAutoCommit(ctx context.Context, name string,
	params map[string]interface{}, tool Tool, _ interface{}) {
	r.mu.RLock()
	c := r.autoCommitter
	r.mu.RUnlock()
	if c == nil {
		return
	}
	level := tool.RequiresApproval()
	if level != approval.LevelEdit && level != approval.LevelAll {
		return
	}
	skip := false
	if v, ok := params[autocommit.SkipParamKey].(bool); ok && v {
		skip = true
	}
	if skip {
		return
	}
	paths := derivePaths(name, params)
	cctx := autocommit.CommitContext{
		ToolName:      name,
		Args:          params,
		MutatedPaths:  paths,
		SkipRequested: false,
	}
	if _, err := c.MaybeCommit(ctx, cctx); err != nil {
		log.Printf("auto-commit: %v", err)
	}
}

// derivePaths is the per-tool mutated-paths table. It is intentionally
// explicit rather than generic introspection: a generic "find every
// `path`-shaped string" would over-trigger on unrelated args (e.g. the
// `path` field of a `glob` query). Future tools that mutate files with
// novel param shapes need an explicit entry here; the fallthrough
// returns nil and the committer's porcelain-based discovery handles it
// safely.
//
// Per spec §3.5 of the F22 design.
func derivePaths(toolName string, params map[string]interface{}) []string {
	switch toolName {
	case "fs_write", "fs_edit", "smart_edit", "notebook_edit", "write_file":
		if p, ok := params["path"].(string); ok && p != "" {
			return []string{p}
		}
	case "multiedit_commit":
		if edits, ok := params["edits"].([]interface{}); ok {
			seen := map[string]struct{}{}
			var out []string
			for _, e := range edits {
				m, ok := e.(map[string]interface{})
				if !ok {
					continue
				}
				p, ok := m["path"].(string)
				if !ok || p == "" {
					continue
				}
				if _, dup := seen[p]; dup {
					continue
				}
				seen[p] = struct{}{}
				out = append(out, p)
			}
			return out
		}
	case "mapping_edit":
		if p, ok := params["target_file"].(string); ok && p != "" {
			return []string{p}
		}
	}
	return nil
}

// checkPlanModeGate consults the plan-mode gate (if wired). Returns
// ErrPlanModeGated wrapped with tool name + reason when blocked.
func (r *ToolRegistry) checkPlanModeGate(name string, params map[string]interface{}) error {
	r.mu.RLock()
	g := r.planGate
	r.mu.RUnlock()
	if g == nil {
		return nil
	}
	blocked, reason := g.IsBlocked(name, params)
	if !blocked {
		return nil
	}
	return fmt.Errorf("%w: %s (%s)", ErrPlanModeGated, name, reason)
}

// applyApprovalGate consults the F21 approval manager (when wired) for the
// given tool. Read-only tools bypass the gate. Edit/Run/All-level tools are
// routed through CheckApproval; ActionDenyWithReason returns the wrapped
// ErrApprovalDenied error verbatim, ActionPromptUser invokes
// PromptForApproval to obtain a yes/no answer.
//
// When the active mode is ModeFullAuto AND the tool's level is LevelRun or
// LevelAll, the gate also injects sandbox markers into a copy of params so
// the original caller's map is not mutated. The returned params (cloned or
// not) replace the original map for the rest of Execute. Returns the
// (possibly rewritten) params + nil on allow, original params + error on
// deny.
//
// nil approval manager → no-op (returns input params unchanged).
func (r *ToolRegistry) applyApprovalGate(ctx context.Context, tool Tool, name string, params map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	a := r.approvalMgr
	r.mu.RUnlock()
	if a == nil {
		return params, nil
	}

	level := tool.RequiresApproval()
	// Read-only short-circuit — every mode allows pure reads, and the
	// matrix never asks the user. Skipping the gate avoids the cost of a
	// CheckApproval call on the hottest path (file reads, greps).
	if level == approval.LevelReadOnly {
		return params, nil
	}

	req := approval.ApprovalRequest{
		ToolName: name,
		Level:    level,
		Args:     params,
	}
	action, err := a.CheckApproval(req)
	if err != nil {
		// CheckApproval returns the wrapped ErrApprovalDenied or
		// ErrInvalidLevel for matrix denials and bad inputs respectively.
		// Surface the error verbatim so callers can errors.Is() classify.
		return params, err
	}
	switch action {
	case approval.ActionAllow:
		// proceed (sandbox marker injection below)
	case approval.ActionPromptUser:
		allowed, perr := a.PromptForApproval(ctx, req)
		if perr != nil {
			return params, perr
		}
		if !allowed {
			return params, fmt.Errorf("%w: user denied tool %q (level=%s)",
				approval.ErrApprovalDenied, name, level)
		}
	case approval.ActionDenyWithReason:
		// Defensive: CheckApproval returns an error alongside the deny;
		// the err branch above already handled it. If a future
		// implementation returns Action without an error, surface a
		// generic deny so the gate cannot be silently bypassed.
		return params, fmt.Errorf("%w: tool %q (level=%s) denied by mode %s",
			approval.ErrApprovalDenied, name, level, a.Mode())
	}

	// F14/F21 integration: when ModeFullAuto + (LevelRun|LevelAll), inject
	// sandbox markers so downstream sandbox-aware tools wrap the
	// invocation. Markers are documented in spec 7128289 §11.
	if a.SandboxRequired(level) {
		// Clone the map so the caller's original is untouched.
		cloned := make(map[string]interface{}, len(params)+2)
		for k, v := range params {
			cloned[k] = v
		}
		cloned["_helix_sandbox_required"] = true
		cloned["_helix_sandbox_network_allowed"] = a.NetworkAllowed()
		params = cloned
	}
	return params, nil
}

// markPlanActionExecuted consumes the matched plan action after a successful
// Execute. No-op when no gate is wired or no action matches.
func (r *ToolRegistry) markPlanActionExecuted(name string, params map[string]interface{}) {
	r.mu.RLock()
	g := r.planGate
	r.mu.RUnlock()
	if g == nil {
		return
	}
	if planID, action, ok := g.MatchApprovedAction(name, params); ok && action != nil {
		g.MarkExecuted(planID, action.ID)
	}
}

// Execute executes a tool by name with given parameters.
// Fires hook lifecycle events around the inner tool.Execute when a hooks
// manager is configured via SetHooksManager. A blocking before-hook prevents
// the tool from running and returns an error wrapping the blockers.
// After-hooks fire even when the tool returned an error so observability
// hooks see the full picture; a blocking after-hook is logged at WARN but
// does not retroactively undo the operation.
func (r *ToolRegistry) Execute(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	// F07: dispatch to BackgroundManager when run_in_background:true.
	if bg, ok := params["run_in_background"].(bool); ok && bg {
		return r.executeInBackground(ctx, name, params)
	}

	// F08: plan-mode gate — block destructive tools when in plan mode without
	// an approved plan action authorising the call.
	if err := r.checkPlanModeGate(name, params); err != nil {
		return nil, err
	}

	tool, err := r.Get(name)
	if err != nil {
		return nil, err
	}
	if err := tool.Validate(params); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// F21: approval gate. Runs AFTER plan-mode + Validate so policy errors
	// surface in their canonical form, but BEFORE before-hooks fire so a
	// denied call never reaches BeforeToolCall observers (matches spec §4
	// data-flow: gate sits between Validate and Execute). May rewrite params
	// to inject sandbox markers when the active mode is ModeFullAuto.
	params, err = r.applyApprovalGate(ctx, tool, name, params)
	if err != nil {
		return nil, err
	}

	// BeforeToolCall + specialised before-events (block aborts).
	if r.hooksManager != nil {
		if err := r.fireBefore(ctx, name, params); err != nil {
			return nil, err
		}
	}

	// F16: wrap the inner tool.Execute call in a telemetry span + metric pair
	// when instrumentation is wired. The wrap fires ONLY after Validate +
	// before-hooks succeed so the metrics reflect actual execution latency,
	// not validation/policy overhead.
	r.mu.RLock()
	toolTelemetry := r.toolTelemetry
	r.mu.RUnlock()
	var finishTelemetry func(error)
	if toolTelemetry != nil {
		ctx, finishTelemetry = toolTelemetry.Begin(ctx, name, string(tool.Category()))
	}

	result, execErr := tool.Execute(ctx, params)

	if finishTelemetry != nil {
		finishTelemetry(execErr)
	}

	// AfterToolCall + specialised after-events (block logged, not propagated).
	if r.hooksManager != nil {
		r.fireAfter(ctx, name, params, result, execErr)
	}

	// F08: on success, consume the matched plan action so it cannot be re-used.
	if execErr == nil {
		r.markPlanActionExecuted(name, params)
	}

	// F13: on success, fire the LSP auto-trigger for Edit-class tools so
	// subsequent GetDiagnostics calls see fresh state. Best-effort: errors
	// are swallowed inside triggerLSPAfterEdit.
	if execErr == nil {
		r.triggerLSPAfterEdit(ctx, name, params)
	}

	// F22: on success, fire the auto-commit hook for Edit-class tools.
	// Runs AFTER F13 LSP auto-trigger so diagnostics settle before the
	// working tree gets committed (per spec §11 #1). Best-effort:
	// errors from MaybeCommit are logged + discarded.
	if execErr == nil {
		r.fireAutoCommit(ctx, name, params, tool, result)
	}

	return result, execErr
}

// fireBefore dispatches BeforeToolCall + the specialised event for the tool.
// Returns the first non-nil blocker as a wrapped error; nil if everything OK.
func (r *ToolRegistry) fireBefore(ctx context.Context, name string, params map[string]interface{}) error {
	if err := r.dispatchAndCheck(ctx, hooks.HookTypeBeforeToolCall, "tool_registry", map[string]interface{}{
		"toolName": name,
		"params":   params,
	}); err != nil {
		return err
	}
	if specialised, ok := specialisedBeforeEvent(name); ok {
		if err := r.dispatchAndCheck(ctx, specialised, "tool_registry", map[string]interface{}{
			"toolName": name,
			"params":   params,
		}); err != nil {
			return err
		}
	}
	return nil
}

// fireAfter dispatches AfterToolCall + the specialised event for the tool.
// Blockers from after-events are logged at WARN; this function never returns
// them as errors (the operation already happened).
func (r *ToolRegistry) fireAfter(ctx context.Context, name string, params map[string]interface{}, result interface{}, execErr error) {
	data := map[string]interface{}{
		"toolName": name,
		"params":   params,
		"result":   result,
		"error":    errString(execErr),
	}
	r.dispatchAndLog(ctx, hooks.HookTypeAfterToolCall, "tool_registry", data)
	if specialised, ok := specialisedAfterEvent(name); ok {
		r.dispatchAndLog(ctx, specialised, "tool_registry", data)
	}
}

// dispatchAndCheck fires an event synchronously and returns the first blocker
// as a wrapped error.
func (r *ToolRegistry) dispatchAndCheck(ctx context.Context, evtType hooks.HookType, source string, data map[string]interface{}) error {
	event := hooks.NewEventWithContext(ctx, evtType)
	event.Source = source
	for k, v := range data {
		event.SetData(k, v)
	}
	results := r.hooksManager.TriggerEventAndWait(event)
	if blockers := hooks.Blockers(results); len(blockers) > 0 {
		return fmt.Errorf("operation blocked by hook(s) on %s: %v", evtType, blockers[0])
	}
	return nil
}

// dispatchAndLog fires an event synchronously, logging any blockers at WARN.
func (r *ToolRegistry) dispatchAndLog(ctx context.Context, evtType hooks.HookType, source string, data map[string]interface{}) {
	event := hooks.NewEventWithContext(ctx, evtType)
	event.Source = source
	for k, v := range data {
		event.SetData(k, v)
	}
	results := r.hooksManager.TriggerEventAndWait(event)
	if blockers := hooks.Blockers(results); len(blockers) > 0 {
		log.Printf("WARN registry: %d hook blocker(s) on %s ignored: %v", len(blockers), evtType, blockers[0])
	}
}

// specialisedBeforeEvent maps tool names to the specialised before-event
// (BeforeBash for Bash; BeforeEdit for Edit/Write/MultiEdit). Returns false
// for tools without a specialisation.
func specialisedBeforeEvent(toolName string) (hooks.HookType, bool) {
	switch toolName {
	case "Bash":
		return hooks.HookTypeBeforeBash, true
	case "Edit", "Write", "MultiEdit":
		return hooks.HookTypeBeforeEdit, true
	}
	return "", false
}

// specialisedAfterEvent mirrors specialisedBeforeEvent for the after side.
func specialisedAfterEvent(toolName string) (hooks.HookType, bool) {
	switch toolName {
	case "Bash":
		return hooks.HookTypeAfterBash, true
	case "Edit", "Write", "MultiEdit":
		return hooks.HookTypeAfterEdit, true
	}
	return "", false
}

// errString safely renders an error for inclusion in event payloads.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// List returns all registered tools
func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	return tools
}

// ListByCategory returns all tools in a category
func (r *ToolRegistry) ListByCategory(category ToolCategory) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []Tool
	for _, tool := range r.tools {
		if tool.Category() == category {
			tools = append(tools, tool)
		}
	}

	return tools
}

// GetSchema returns the schema for a tool
func (r *ToolRegistry) GetSchema(name string) (*ToolSchema, error) {
	tool, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	schema := tool.Schema()
	return &schema, nil
}

// GetAllSchemas returns schemas for all tools
func (r *ToolRegistry) GetAllSchemas() map[string]ToolSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make(map[string]ToolSchema)
	for name, tool := range r.tools {
		schemas[name] = tool.Schema()
	}

	return schemas
}

// ExportSchemas exports all tool schemas as JSON
func (r *ToolRegistry) ExportSchemas() ([]byte, error) {
	schemas := r.GetAllSchemas()
	return json.MarshalIndent(schemas, "", "  ")
}

// RegisterMCPManager exposes external MCP server tools to the agent.
// Tool names are prefixed "<server>:<tool>" so they are unambiguous.
// Only tools from servers currently in StateReady are registered; call
// this after Manager.Start has had time to connect alwaysLoad servers.
//
// LIMITATION: This is called once at startup. After mcp.Manager.Reload, the
// tool registry is NOT automatically reconciled — new tools are invisible
// and removed tools error on call. To pick up Reload changes, the caller
// must invoke RegisterMCPManager again. (Reconciliation will be addressed
// in a follow-up.)
func (r *ToolRegistry) RegisterMCPManager(m *mcp.Manager) {
	// Build a per-server read-only allowlist from the loaded config so a
	// server marked `readOnly: true` has all its tools registered at
	// approval.LevelReadOnly (otherwise the ReadOnlyOnly agent tool loop
	// blocks every MCP tool, since the default level is LevelEdit).
	readOnlyServers := map[string]bool{}
	if cfg := m.Config(); cfg != nil {
		for _, s := range cfg.Servers {
			if s.ReadOnly {
				readOnlyServers[s.Name] = true
			}
		}
	}

	for _, t := range m.Tools() {
		name := t.Server + ":" + t.Name
		if _, err := r.Get(name); err == nil {
			log.Printf("WARN tools: MCP tool %q replaces existing registration", name)
		}
		// Capture loop variables so the closure references the correct values.
		server, toolName := t.Server, t.Name
		desc := t.Desc
		if desc == "" {
			desc = t.Title
		}
		// Decide the approval level for this tool. A server explicitly
		// marked read-only in config has every tool registered as
		// LevelReadOnly. Otherwise, individual tools whose names match a
		// well-known read-only pattern (read_file, list_directory,
		// search, …) are also LevelReadOnly; everything else keeps the
		// conservative LevelEdit default.
		level := approval.LevelEdit
		if readOnlyServers[server] || isReadOnlyMCPToolName(toolName) {
			level = approval.LevelReadOnly
		}
		r.Register(&mcpTool{
			registry:      r,
			mcpMgr:        m,
			server:        server,
			toolName:      toolName,
			name:          name,
			desc:          desc,
			approvalLevel: level,
		})
	}
}

// readOnlyMCPToolNames is the set of MCP tool names that are pure reads
// (no side effects) for well-known MCP servers — notably the official
// @modelcontextprotocol/server-filesystem. A tool matched here is
// registered at approval.LevelReadOnly even when its server is not
// explicitly flagged `readOnly: true`, so the ReadOnlyOnly agent tool
// loop can offer + execute it. The match is conservative: anything not
// listed keeps the LevelEdit default.
var readOnlyMCPToolNames = map[string]bool{
	"read_file":                 true,
	"read_text_file":            true,
	"read_media_file":           true,
	"read_multiple_files":       true,
	"list_directory":            true,
	"list_directory_with_sizes": true,
	"directory_tree":            true,
	"search_files":              true,
	"search":                    true,
	"get_file_info":             true,
	"list_allowed_directories":  true,
}

// isReadOnlyMCPToolName reports whether an MCP tool name is a known
// pure-read operation. Case-insensitive on the bare tool name (NOT the
// server-prefixed name).
func isReadOnlyMCPToolName(name string) bool {
	return readOnlyMCPToolNames[strings.ToLower(name)]
}

// mcpTool is an internal Tool adapter that routes Execute to an mcp.Manager.
type mcpTool struct {
	registry *ToolRegistry
	mcpMgr   *mcp.Manager
	server   string
	toolName string
	name     string
	desc     string
	// approvalLevel is the level this tool reports from RequiresApproval.
	// Set by RegisterMCPManager: LevelReadOnly for read-only servers /
	// known read-only tool names, LevelEdit otherwise (the conservative
	// default for an unclassified, possibly-mutating tool).
	approvalLevel approval.ApprovalLevel
}

func (t *mcpTool) Name() string        { return t.name }
func (t *mcpTool) Description() string { return t.desc }

// RequiresApproval reports the per-server / per-tool approval level chosen
// at registration time. A read-only-configured server's tools report
// LevelReadOnly so the ReadOnlyOnly agent loop can run them; everything
// else reports LevelEdit (the conservative default).
func (t *mcpTool) RequiresApproval() approval.ApprovalLevel { return t.approvalLevel }

func (t *mcpTool) Category() ToolCategory { return ToolCategory("mcp") }

func (t *mcpTool) Schema() ToolSchema {
	return ToolSchema{
		Type:        "object",
		Properties:  map[string]interface{}{"args": map[string]interface{}{"type": "object"}},
		Required:    []string{},
		Description: t.desc,
	}
}

func (t *mcpTool) Validate(params map[string]interface{}) error { return nil }

func (t *mcpTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	args, _ := params["args"].(map[string]any)
	if args == nil {
		// Treat the whole params map as the tool arguments when no "args" key.
		args = make(map[string]any, len(params))
		for k, v := range params {
			args[k] = v
		}
	}
	res, err := t.mcpMgr.CallTool(ctx, t.server, t.toolName, args)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// executeInBackground routes to BackgroundManager.StartTask. Returns immediately
// with task_id, state, and a message. Foreground logic is unchanged.
//
// Both hooksManager and bgManager are captured under a single RLock to avoid
// a second lock acquisition after the first release.
//
// Before dispatching the task, this function:
//  1. Fires fireBefore synchronously (spec §4.7 — hooks fire on the dispatch
//     event for run_in_background:true). A blocking hook rejects the dispatch,
//     preserving F05 policy enforcement (e.g. user-confirmation for Bash).
//  2. Calls tool.Validate on the cleaned params so bad parameters fail fast at
//     dispatch time instead of inside the background goroutine with an opaque error.
func (r *ToolRegistry) executeInBackground(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	r.mu.RLock()
	bm := r.bgManager
	hm := r.hooksManager
	r.mu.RUnlock()

	if bm == nil {
		return nil, ErrNoBackgroundMgr
	}

	// F08: plan-mode gate — background dispatch is also subject to plan-mode
	// policy. Blocked calls are rejected synchronously before any task is queued.
	if err := r.checkPlanModeGate(name, params); err != nil {
		return nil, err
	}

	// Fire before-hooks synchronously at dispatch time. A blocking hook can
	// reject the dispatch, preventing the task from ever being queued.
	if hm != nil {
		if err := r.fireBefore(ctx, name, params); err != nil {
			return nil, err
		}
	}

	tool, err := r.Get(name)
	if err != nil {
		return nil, err
	}
	cleanArgs := stripBackgroundFlag(params)

	// Validate synchronously so bad params fail at dispatch time rather than
	// inside the background goroutine.
	if err := tool.Validate(cleanArgs); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// F21: approval gate also applies to background dispatch. Denied calls
	// are rejected synchronously so the user sees the deny immediately
	// instead of having a phantom task ID returned.
	cleanArgs, err = r.applyApprovalGate(ctx, tool, name, cleanArgs)
	if err != nil {
		return nil, err
	}

	bgExec := r.adaptToolForBackground(tool)
	task, err := bm.StartTask(name, cleanArgs, bgExec)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"task_id": task.ID,
		"state":   string(task.State()),
		"message": fmt.Sprintf("Task started in background. ID: %s — use TaskOutput to check progress.", task.ID),
	}, nil
}

// adaptToolForBackground returns a workflow.BackgroundExecutor that bridges
// the tool's Execute / ExecuteWithProgress methods. Streaming-aware tools
// get the sink directly; plain tools get a final-result-only fallback that
// writes the formatted result as a single sink line at completion.
func (r *ToolRegistry) adaptToolForBackground(tool Tool) workflow.BackgroundExecutor {
	if ba, ok := tool.(BackgroundAware); ok {
		return func(ctx context.Context, args map[string]interface{}, sink workflow.LineSink) (interface{}, error) {
			return ba.ExecuteWithProgress(ctx, args, LineSink(sink))
		}
	}
	return func(ctx context.Context, args map[string]interface{}, sink workflow.LineSink) (interface{}, error) {
		res, err := tool.Execute(ctx, args)
		if err == nil && res != nil {
			sink(fmt.Sprintf("%v", res))
		}
		return res, err
	}
}

// stripBackgroundFlag returns a copy of params without the "run_in_background" key.
func stripBackgroundFlag(params map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(params))
	for k, v := range params {
		if k == "run_in_background" {
			continue
		}
		out[k] = v
	}
	return out
}

// ----------------------------------------------------------------------------
// P3-T04 — Aggressive tool-call parallelism (speed programme Phase 3).
//
// When an LLM turn requests multiple tool calls, INDEPENDENT calls (read-only /
// side-effect-free, or calls whose mutation targets do not conflict) are run
// concurrently through a bounded worker pool. Calls that conflict (writes to the
// same file, run/shell calls, calls that may depend on a prior call's output)
// run serially, in their LLM-requested order. Results are always assembled back
// in the request order — the turn outcome is identical to fully serial
// execution. R2 #5 (Claude Code /batch), R4 O12.
// ----------------------------------------------------------------------------

// ToolCallRequest is one tool call inside an LLM turn. Order in the slice passed
// to ExecuteBatch is the LLM-requested order — results are assembled back in
// exactly this order regardless of completion order.
type ToolCallRequest struct {
	// ID is the LLM-assigned call ID (e.g. Anthropic tool_use id). Optional;
	// purely passed through to BatchResult so callers can correlate.
	ID string
	// Name is the tool (or alias) name to dispatch.
	Name string
	// Params is the tool argument map.
	Params map[string]interface{}
}

// BatchResult is the outcome of one ToolCallRequest. The slice returned by
// ExecuteBatch has one BatchResult per input ToolCallRequest, in the SAME order
// as the input slice (request order), never completion order.
type BatchResult struct {
	ID     string
	Name   string
	Result interface{}
	Err    error
	// RanParallel is true when this call was dispatched in the concurrent
	// wave rather than the serial wave. Diagnostic only — does not affect the
	// result value.
	RanParallel bool
}

// ParallelClassifier is an OPTIONAL interface a Tool may implement to declare
// whether it is safe to run concurrently with other tool calls in the same
// turn. Tools that do NOT implement it fall back to RequiresApproval()-based
// inference: LevelReadOnly tools are treated as parallel-safe (pure reads, no
// side effects), every other level is treated as must-serialise.
//
// A tool should return true ONLY when calling it concurrently with other
// parallel-safe tools cannot change the turn outcome — i.e. it has no
// observable side effects and does not depend on shared mutable state.
type ParallelClassifier interface {
	// ParallelSafe reports whether the tool is safe to run concurrently.
	ParallelSafe() bool
}

// isParallelSafe decides whether a single resolved tool is safe to run
// concurrently. Explicit ParallelClassifier wins; otherwise the approval
// level is the proxy — only LevelReadOnly (pure reads) is parallel-safe.
func isParallelSafe(tool Tool) bool {
	if pc, ok := tool.(ParallelClassifier); ok {
		return pc.ParallelSafe()
	}
	return tool.RequiresApproval() == approval.LevelReadOnly
}

// conflictKey extracts the file path a call mutates (if any) so two writes to
// the SAME path are never parallelised even when classified individually. It
// reuses derivePaths — the per-tool mutated-path table already maintained for
// auto-commit — plus a couple of read-target shapes. An empty string means
// "no identifiable single target" (treated as conflicting-with-everything to
// stay safe).
func conflictKeys(name string, params map[string]interface{}) []string {
	if paths := derivePaths(name, params); len(paths) > 0 {
		return paths
	}
	// Fall back to common single-target param shapes for tools not in the
	// mutated-path table (covers read tools whose target we still want to
	// key on for dependency detection).
	for _, k := range []string{"path", "file", "file_path", "target_file"} {
		if p, ok := params[k].(string); ok && p != "" {
			return []string{p}
		}
	}
	return nil
}

// classifyBatch partitions a turn's tool calls into a parallel set and a serial
// set. A call joins the parallel set only when ALL hold:
//   - the tool resolves and is itself parallel-safe (isParallelSafe);
//   - it carries no run_in_background flag (background dispatch is its own path);
//   - its mutation/target key (if any) is not shared with ANY other call in the
//     turn — a shared key means a potential write/read ordering dependency, so
//     both calls drop to the serial set to preserve serial-equivalent ordering.
//
// Everything else is serial. The returned slices hold INDICES into reqs so the
// caller can write results back into the request-ordered output slice.
func (r *ToolRegistry) classifyBatch(reqs []ToolCallRequest) (parallel, serial []int) {
	// Count how many calls touch each conflict key across the whole turn.
	keyCount := make(map[string]int)
	keysPerReq := make([][]string, len(reqs))
	for i, req := range reqs {
		keys := conflictKeys(req.Name, req.Params)
		keysPerReq[i] = keys
		for _, k := range keys {
			keyCount[k]++
		}
	}

	for i, req := range reqs {
		// run_in_background calls are never folded into the parallel wave —
		// they have their own async dispatch semantics via executeInBackground.
		if bg, ok := req.Params["run_in_background"].(bool); ok && bg {
			serial = append(serial, i)
			continue
		}
		tool, err := r.Get(req.Name)
		if err != nil {
			// Unresolvable tool — keep it serial so the error surfaces in
			// the deterministic position.
			serial = append(serial, i)
			continue
		}
		if !isParallelSafe(tool) {
			serial = append(serial, i)
			continue
		}
		// Shared-target check: if any conflict key for this call is also used
		// by a different call in the turn, this call may have an ordering
		// dependency — keep it serial.
		shared := false
		for _, k := range keysPerReq[i] {
			if keyCount[k] > 1 {
				shared = true
				break
			}
		}
		if shared {
			serial = append(serial, i)
			continue
		}
		parallel = append(parallel, i)
	}
	return parallel, serial
}

// defaultBatchConcurrency bounds the worker pool for parallel tool dispatch.
// Matches the R2 #5 "/batch up to 10×" observation — never spawn more than 10
// concurrent tool executions regardless of how many calls a turn requests.
const defaultBatchConcurrency = 10

// ExecuteBatch dispatches all tool calls in an LLM turn, parallelising the
// independent (side-effect-free / non-conflicting) calls through a bounded
// worker pool while running conflicting / ordering-dependent calls serially in
// request order.
//
// GUARANTEE: the returned []BatchResult is in the SAME order as the input reqs
// slice (LLM-requested order), regardless of which call finished first. The
// turn outcome — every result value and the final filesystem/registry state —
// is identical to fully serial execution. Only genuinely independent calls run
// concurrently.
//
// maxConcurrency <= 0 selects defaultBatchConcurrency. A single-element batch
// (or a batch with no parallel-safe members) degrades to plain serial dispatch.
//
// Each individual call goes through the SAME Execute path (plan-mode gate,
// approval gate, hooks, telemetry, LSP/auto-commit triggers) — ExecuteBatch
// only changes WHEN calls run, never HOW.
func (r *ToolRegistry) ExecuteBatch(ctx context.Context, reqs []ToolCallRequest, maxConcurrency int) []BatchResult {
	results := make([]BatchResult, len(reqs))
	for i, req := range reqs {
		results[i] = BatchResult{ID: req.ID, Name: req.Name}
	}
	if len(reqs) == 0 {
		return results
	}
	if maxConcurrency <= 0 {
		maxConcurrency = defaultBatchConcurrency
	}

	parallelIdx, serialIdx := r.classifyBatch(reqs)

	// Run the parallel wave first through a bounded worker pool. Each result
	// is written to its request-ordered slot — slot writes are disjoint
	// (one goroutine per index) so no mutex on results is needed.
	if len(parallelIdx) > 0 {
		sem := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup
		for _, idx := range parallelIdx {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				res, err := r.Execute(ctx, reqs[i].Name, reqs[i].Params)
				results[i].Result = res
				results[i].Err = err
				results[i].RanParallel = true
			}(idx)
		}
		wg.Wait()
	}

	// Run the serial wave in request order. Serial calls run AFTER the
	// parallel wave so any ordering dependency (a serial call that reads
	// state a parallel read produced) is preserved — and so two writes to
	// the same file always apply in request order.
	for _, idx := range serialIdx {
		// Honour context cancellation between serial calls.
		if err := ctx.Err(); err != nil {
			results[idx].Err = err
			continue
		}
		res, err := r.Execute(ctx, reqs[idx].Name, reqs[idx].Params)
		results[idx].Result = res
		results[idx].Err = err
		results[idx].RanParallel = false
	}

	return results
}

// ExecuteToolBatch is the P3-T04 bridging method that makes *ToolRegistry
// satisfy the llm.BatchToolExecutor interface — the seam that wires the
// parallel tool-dispatch facility into the live internal/llm tool-execution
// path (ToolCallingProvider.executeToolCalls).
//
// It adapts the LLM-layer call shape ([]llm.ToolCall) to the lower-level
// ExecuteBatch primitive ([]ToolCallRequest), dispatches the turn — independent
// calls concurrently, conflicting / ordering-dependent calls serially in
// request order — and adapts the results back to []llm.ToolCallResult in the
// SAME order as the input slice.
//
// Cycle note: internal/llm CANNOT import internal/tools (internal/tools already
// transitively imports internal/llm). The dependency is therefore inverted:
// internal/llm DEFINES the BatchToolExecutor interface, and internal/tools — a
// downstream package that may import internal/llm — provides this satisfying
// method. internal/llm discovers the capability through a runtime type
// assertion on the ToolExecutor it was handed.
//
// maxConcurrency <= 0 selects defaultBatchConcurrency (10).
//
// GUARANTEE (inherited from ExecuteBatch): the returned slice has one entry per
// input call, in input (LLM-requested) order, regardless of completion order;
// the turn outcome is identical to fully serial execution.
func (r *ToolRegistry) ExecuteToolBatch(ctx context.Context, calls []llm.ToolCall, maxConcurrency int) []llm.ToolCallResult {
	out := make([]llm.ToolCallResult, len(calls))
	if len(calls) == 0 {
		return out
	}

	reqs := make([]ToolCallRequest, len(calls))
	for i, c := range calls {
		reqs[i] = ToolCallRequest{
			ID:     c.ID,
			Name:   c.Function.Name,
			Params: c.Function.Arguments,
		}
	}

	batch := r.ExecuteBatch(ctx, reqs, maxConcurrency)
	for i, b := range batch {
		entry := llm.ToolCallResult{
			CallID:      b.ID,
			ToolName:    b.Name,
			RanParallel: b.RanParallel,
		}
		// Normalise an execution error into the same informative-string shape
		// the serial path in ToolCallingProvider.executeToolCalls produces, so
		// the final prompt renders identically regardless of dispatch path.
		if b.Err != nil {
			entry.Result = fmt.Sprintf("Tool error: %v", b.Err)
		} else {
			entry.Result = b.Result
		}
		out[i] = entry
	}
	return out
}

// Close closes the registry and releases all resources
func (r *ToolRegistry) Close() error {
	if r.web != nil {
		if err := r.web.Close(); err != nil {
			return fmt.Errorf("failed to close web tools: %w", err)
		}
	}

	if r.browser != nil {
		if err := r.browser.CloseAllBrowsers(); err != nil {
			return fmt.Errorf("failed to close browsers: %w", err)
		}
	}

	return nil
}

// GetFileSystem returns the filesystem tools instance
func (r *ToolRegistry) GetFileSystem() *filesystem.FileSystemTools {
	return r.filesystem
}

// GetShell returns the shell executor instance
func (r *ToolRegistry) GetShell() *shell.ShellExecutor {
	return r.shell
}

// GetWeb returns the web tools instance
func (r *ToolRegistry) GetWeb() *web.WebTools {
	return r.web
}

// GetBrowser returns the browser tools instance
func (r *ToolRegistry) GetBrowser() *browser.BrowserTools {
	return r.browser
}

// GetMapper returns the mapper instance
func (r *ToolRegistry) GetMapper() mapping.Mapper {
	return r.mapper
}

// GetMultiEdit returns the multi-edit instance
func (r *ToolRegistry) GetMultiEdit() *multiedit.MultiFileEditor {
	return r.multiEdit
}

// GetConfirmation returns the confirmation instance
func (r *ToolRegistry) GetConfirmation() *confirmation.ConfirmationCoordinator {
	return r.confirmation
}
