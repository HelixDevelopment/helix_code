package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"dev.helix.code/internal/hooks"
	"dev.helix.code/internal/mcp"
	"dev.helix.code/internal/tools/browser"
	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/filesystem"
	"dev.helix.code/internal/tools/mapping"
	"dev.helix.code/internal/tools/multiedit"
	"dev.helix.code/internal/tools/shell"
	"dev.helix.code/internal/tools/web"
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
)

// ToolRegistry manages all available tools
type ToolRegistry struct {
	tools   map[string]Tool
	aliases map[string]string // alias -> tool name
	mu      sync.RWMutex

	// Hook lifecycle manager (optional; nil = passthrough).
	hooksManager *hooks.Manager

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
	r.Register(&AskUserTool{registry: r})
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

// Execute executes a tool by name with given parameters.
// Fires hook lifecycle events around the inner tool.Execute when a hooks
// manager is configured via SetHooksManager. A blocking before-hook prevents
// the tool from running and returns an error wrapping the blockers.
// After-hooks fire even when the tool returned an error so observability
// hooks see the full picture; a blocking after-hook is logged at WARN but
// does not retroactively undo the operation.
func (r *ToolRegistry) Execute(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	tool, err := r.Get(name)
	if err != nil {
		return nil, err
	}
	if err := tool.Validate(params); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// BeforeToolCall + specialised before-events (block aborts).
	if r.hooksManager != nil {
		if err := r.fireBefore(ctx, name, params); err != nil {
			return nil, err
		}
	}

	result, execErr := tool.Execute(ctx, params)

	// AfterToolCall + specialised after-events (block logged, not propagated).
	if r.hooksManager != nil {
		r.fireAfter(ctx, name, params, result, execErr)
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
		r.Register(&mcpTool{
			registry: r,
			mcpMgr:   m,
			server:   server,
			toolName: toolName,
			name:     name,
			desc:     desc,
		})
	}
}

// mcpTool is an internal Tool adapter that routes Execute to an mcp.Manager.
type mcpTool struct {
	registry *ToolRegistry
	mcpMgr   *mcp.Manager
	server   string
	toolName string
	name     string
	desc     string
}

func (t *mcpTool) Name() string           { return t.name }
func (t *mcpTool) Description() string    { return t.desc }
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
