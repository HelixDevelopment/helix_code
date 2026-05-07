package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dev.helix.code/internal/agent"
	"dev.helix.code/internal/agent/subagent"
	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/approvalwire"
	"dev.helix.code/internal/autocommit"
	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/commands/builtin"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/hooks"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/continua"
	"dev.helix.code/internal/mcp"
	"dev.helix.code/internal/kilocode"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/roocode"
	"dev.helix.code/internal/projectmemory"
	"dev.helix.code/internal/render"
	"dev.helix.code/internal/server"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/telemetry"
	"dev.helix.code/internal/theme"
	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/tools/askuser"
	"dev.helix.code/internal/tools/browser"
	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
	"dev.helix.code/internal/plantree"
	taskplanner "dev.helix.code/internal/planner"
	"dev.helix.code/internal/tools/persistence"
	"dev.helix.code/internal/tools/sandbox"
	"dev.helix.code/internal/tools/smartedit"
	"dev.helix.code/internal/tools/task"
	"dev.helix.code/internal/tools/worktree"
	"dev.helix.code/internal/verifier"
	"dev.helix.code/internal/voice"
	"dev.helix.code/internal/worker"
	"dev.helix.code/internal/workflow"
	"dev.helix.code/internal/workflow/planmode"
	"dev.helix.code/internal/workspace"
	"go.uber.org/zap"
)

// defaultConfigPathFromEnv resolves the on-disk wizard config path using the
// supplied env-lookup. Mirrors internal/llm.defaultWizardConfigPath but lives
// here so cmd/cli does not need to import an unexported helper. Honours
// XDG_CONFIG_HOME, falls back to $HOME/.config/helixcode/llm.yaml.
func defaultConfigPathFromEnv(env func(string) string) string {
	if xdg := strings.TrimSpace(env("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "helixcode", "llm.yaml")
	}
	home := strings.TrimSpace(env("HOME"))
	if home == "" {
		return filepath.Join(".", ".config", "helixcode", "llm.yaml")
	}
	return filepath.Join(home, ".config", "helixcode", "llm.yaml")
}

// loadProviderConfigFromDisk reads the wizard-written YAML at
// $XDG_CONFIG_HOME/helixcode/llm.yaml (or $HOME/.config fallback) and returns
// the persisted ProviderType + ProviderConfigEntry. Missing-file is reported
// as an os.ErrNotExist-wrapped error so callers can fall through to other
// selection sources (env / wizard prompt). Unmarshal failures are surfaced as
// non-sentinel errors.
//
// Anti-bluff anchor: this is a real-disk reader. There is no stubbed config
// path or fake "no config means everything is fine" mode — callers handle
// the missing file with errors.Is(err, os.ErrNotExist).
func loadProviderConfigFromDisk(envLookup func(string) string) (string, llm.ProviderConfigEntry, error) {
	path := defaultConfigPathFromEnv(envLookup)
	res, err := llm.LoadWizardConfig(path)
	if err != nil {
		return "", llm.ProviderConfigEntry{}, err
	}
	if res == nil {
		return "", llm.ProviderConfigEntry{}, fmt.Errorf("loadProviderConfigFromDisk: nil result from %s", path)
	}
	return string(res.ProviderType), res.ConfigEntry, nil
}

// buildSubagentLLMProvider constructs the LLM provider used by a subagent
// helper child (P1-F15-T08). Wired into main() via subagent.RunAsSubagent.
//
// The child does NOT replay the parent's full F12 bootstrap (no flag parsing,
// no friendly Stderr hints): it reads the same config-file + HELIX_LLM_PROVIDER
// env that the parent uses, resolves the type via llm.Select, and constructs
// the cloud provider. On any failure (no config, env-only with missing creds,
// construction error) it falls back to the local Ollama default — matching the
// parent's "keep working for non-LLM paths" stance.
//
// Pragmatic v1: we read env+config-file only; the child does not see the
// parent's --provider flag because the spawner re-exec's the helper without
// argv. This is documented in the spec § 4.2; if a future task plumbs
// --provider through the env protocol, this function should consult it.
//
// Anti-bluff: this function MUST construct a real provider — never a stub or
// the FakeLLMProvider (which lives in the subagent package as a test-only
// type with the "fake-test-only" sentinel ProviderType).
func buildSubagentLLMProvider(ctx context.Context) (llm.Provider, error) {
	configProviderName, configEntry, configErr := loadProviderConfigFromDisk(os.Getenv)
	if configErr != nil && !errors.Is(configErr, os.ErrNotExist) {
		// Config read failed for a real reason; surface it but keep going so
		// we still try env / default.
		log.Printf("subagent: config load failed (continuing): %v", configErr)
	}
	selectorInput := llm.SelectorInput{
		Flag:   "",
		Env:    os.Getenv("HELIX_LLM_PROVIDER"),
		Config: configProviderName,
	}
	ptype, selErr := llm.Select(selectorInput)
	switch {
	case errors.Is(selErr, llm.ErrNoProviderConfigured):
		// Fall through to default Ollama.
	case selErr != nil:
		// Unknown provider name — surface and fall back to default rather
		// than aborting the subagent run.
		log.Printf("subagent: provider selector error: %v (falling back to default)", selErr)
	default:
		entry := configEntry
		entry.Type = ptype
		cloud, cErr := llm.NewCloudProvider(ptype, entry)
		if cErr == nil && cloud != nil {
			return cloud, nil
		}
		log.Printf("subagent: failed to construct cloud provider %q (%v); falling back to local default", ptype, cErr)
	}

	// Default: local Ollama on the standard port. Mirrors NewCLI()'s default.
	provider, err := llm.NewOllamaProvider(llm.OllamaConfig{
		DefaultModel: "llama3.2",
		BaseURL:      "http://localhost:11434",
	})
	if err != nil {
		return nil, fmt.Errorf("subagent: default Ollama provider construction failed: %w", err)
	}
	return provider, nil
}

// CLI represents the command-line interface
type CLI struct {
	workerPool         *worker.SSHWorkerPool
	llmProvider        llm.Provider
	notificationEngine *notification.NotificationEngine
	verifierAdapter    *verifier.Adapter
	permissionMode     string
	permissionsEngine  *permissions.Engine
	persistenceManager *persistence.Manager
	worktreeManager    *worktree.Manager
	sessionMgr         *session.Manager
	toolRegistry       *tools.ToolRegistry
	commandRegistry    *commands.Registry
	mcpManager         *mcp.Manager
	browserManager     *browser.BrowserManager        // F23: cline-style single-session browser façade
	memoryRegistry     *projectmemory.MemoryRegistry  // F24: codex-style project memory + hot-reload
	hooksLoaded        int                            // count of hooks loaded at startup (for diagnostics)
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	// Initialize LLM provider from config - use Ollama on port 11434
	llmProvider, _ := llm.NewOllamaProvider(llm.OllamaConfig{
		DefaultModel: "llama3.2",         // Use Ollama's model name
		BaseURL:      "http://localhost:11434", // Ollama default port
	})

	// Initialize LLMsVerifier subsystem if config is available
	var verifierAdapter *verifier.Adapter
	cfg, err := config.Load()
	if err == nil && cfg.Verifier != nil && cfg.Verifier.Enabled {
		vResult, vErr := verifier.Bootstrap(cfg.Verifier)
		if vErr == nil && vResult != nil {
			verifierAdapter = vResult.Adapter
		}
	}

	return &CLI{
		workerPool:         worker.NewSSHWorkerPool(true),
		llmProvider:        llmProvider,
		notificationEngine: notification.NewNotificationEngine(),
		verifierAdapter:    verifierAdapter,
	}
}

// initPermissions bootstraps the permissions.Engine with rules loaded from
// ~/.helixcode/permissions.yaml (user scope) and <cwd>/.helixcode/permissions.yaml
// (project scope), then registers a default confirmation.Policy with pe.
//
// CONCERN (T10 / Phase 3): pe is a locally constructed PolicyEngine that is not
// yet wired into the tool execution path. The --permission-mode flag is parsed
// and validated here, and the Engine is ready, but actual tool calls are not yet
// gated by it. T10 will introduce the `permissions` subcommand; Phase 3 will
// thread pe through the session/tool dispatcher so denies block execution.
func (c *CLI) initPermissions(ctx context.Context, pe *confirmation.PolicyEngine) error {
	if c.permissionMode != "" && !permissions.IsValidMode(c.permissionMode) {
		return fmt.Errorf("invalid --permission-mode %q (valid: %v)", c.permissionMode, permissions.ValidModes)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving user home dir: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving cwd: %w", err)
	}
	loader := &permissions.FileLoader{
		UserPath:    filepath.Join(home, ".helixcode", "permissions.yaml"),
		ProjectPath: filepath.Join(cwd, ".helixcode", "permissions.yaml"),
		Mode:        c.permissionMode,
	}
	eng, err := permissions.NewEngine(ctx, loader, pe)
	if err != nil {
		return fmt.Errorf("initialising permissions engine: %w", err)
	}
	c.permissionsEngine = eng
	return nil
}

// initPersistence bootstraps the persistence.Manager rooted at the current
// working directory. Large tool outputs (>50 KB) will be written under
// <cwd>/.helix/tool-results/ so the context window is not flooded.
// A background goroutine prunes files older than DefaultMaxAge (7 days).
func (c *CLI) initPersistence() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving cwd for persistence: %w", err)
	}
	c.persistenceManager = persistence.NewManager(cwd)
	go func() {
		if err := c.persistenceManager.CleanupOld(persistence.DefaultMaxAge); err != nil {
			log.Printf("WARN persistence cleanup: %v", err)
		}
	}()
	return nil
}

// initWorktree bootstraps the worktree.Manager with repoRoot resolved via
// `git rev-parse --show-toplevel`, falling back to os.Getwd() if the cwd
// is not a git repo.
func (c *CLI) initWorktree(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving cwd for worktree: %w", err)
	}
	repoRoot := cwd
	if root, err := worktreeRevParseToplevel(ctx, cwd); err == nil {
		repoRoot = root
	}
	c.worktreeManager = worktree.NewManager(repoRoot)
	return nil
}

// initHooks loads ~/.helixcode/hooks.yaml + <cwd>/.helixcode/hooks.yaml,
// wraps each enabled entry in a shell-runner HookFunc, and registers it
// with the session.Manager.hooksManager. Errors fail-fast.
func (c *CLI) initHooks(ctx context.Context, sessionMgr *session.Manager) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving home dir for hooks: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving cwd for hooks: %w", err)
	}
	loader := &hooks.FileLoader{
		UserPath:    filepath.Join(home, ".helixcode", "hooks.yaml"),
		ProjectPath: filepath.Join(cwd, ".helixcode", "hooks.yaml"),
	}
	hs, sources, err := loader.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading hooks: %w", err)
	}
	hm := sessionMgr.GetHooksManager()
	for _, h := range hs {
		scriptPath := h.Metadata["script"]
		h.Handler = hooks.NewShellRunner(scriptPath, h.Timeout)
		if err := hm.Register(h); err != nil {
			return fmt.Errorf("registering hook %q: %w", h.ID, err)
		}
	}
	c.hooksLoaded = len(hs)
	if len(sources) > 0 {
		log.Printf("hooks: loaded %d hook(s) from %v", len(hs), sources)
	}
	return nil
}

// sessionStoreBaseDir resolves the on-disk root for F11 session transcripts.
// Resolution order (XDG Base Directory Specification):
//  1. $XDG_DATA_HOME/helixcode/sessions/   (when $XDG_DATA_HOME is set and absolute)
//  2. $HOME/.local/share/helixcode/sessions/  (XDG default)
//  3. ./.helixcode-sessions/  (last-resort fallback when $HOME is also unset)
//
// The directory is not created here; TranscriptStore creates per-session
// subdirectories on first append.
func sessionStoreBaseDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "helixcode", "sessions")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "share", "helixcode", "sessions")
	}
	return filepath.Join(".", ".helixcode-sessions")
}

// worktreeRevParseToplevel is a tiny shim to avoid leaking the worktree
// package's internal helpers; it shells out to git directly.
func worktreeRevParseToplevel(ctx context.Context, cwd string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Run executes the CLI
func (c *CLI) Run() error {
	// Parse command-line flags
	var (
		command        = flag.String("command", "", "Command to execute")
		workerHost     = flag.String("worker", "", "Worker host to add")
		workerUser     = flag.String("user", "", "Worker SSH username")
		workerKey      = flag.String("key", "", "Worker SSH key path")
		model          = flag.String("model", "llama-3-8b", "LLM model to use")
		prompt         = flag.String("prompt", "", "Prompt for LLM generation")
		maxTokens      = flag.Int("max-tokens", 1000, "Maximum tokens to generate")
		temperature    = flag.Float64("temperature", 0.7, "Generation temperature")
		stream         = flag.Bool("stream", false, "Stream the response")
		listWorkers    = flag.Bool("list-workers", false, "List all workers")
		listModels     = flag.Bool("list-models", false, "List available models")
		healthCheck    = flag.Bool("health", false, "Perform health check")
		notify         = flag.String("notify", "", "Send notification with message")
		notifyType     = flag.String("notify-type", "info", "Notification type")
		notifyPriority = flag.String("notify-priority", "medium", "Notification priority")
		nonInteractive = flag.Bool("non-interactive", false, "Run in non-interactive mode")
		permissionMode = flag.String("permission-mode", "", "permission preset: default|auto|acceptEdits|dontAsk|bypassPermissions")

		// QA flags
		qaRun        = flag.Bool("qa-run", false, "Start a QA session")
		qaList       = flag.Bool("qa-list", false, "List QA sessions")
		qaReport     = flag.String("qa-report", "", "Get QA report for session ID")
		qaScreenshot = flag.String("qa-screenshot", "", "Capture screenshot for session ID")
		qaCancel     = flag.String("qa-cancel", "", "Cancel QA session by ID")
		qaPlatforms  = flag.String("qa-platforms", "web", "Comma-separated platforms for QA")
		qaBanks      = flag.String("qa-banks", "", "Comma-separated bank paths for QA")
		qaFormat     = flag.String("qa-format", "markdown", "Report format: markdown|html|json")
		qaWait       = flag.Bool("qa-wait", false, "Wait for QA session to complete")
		qaServerURL  = flag.String("qa-server", "http://localhost:8080", "HelixCode server URL for QA")

		// F11: session transcript resume.
		// `--resume` resumes the most recently active session for the current project.
		// `--continue` resumes the most recently active session globally (any project).
		// `--resume-session <id>` resumes a specific session by ID (overrides the bool flags).
		// We use three separate flags rather than a NoOptDefVal sentinel because this
		// command-line surface is built on stdlib `flag`, which does not support
		// optional-value flags. Three flags also disambiguate "no id supplied" from
		// "id is the empty string".
		resumeFlag        = flag.Bool("resume", false, "Resume most recent session for current project (F11)")
		continueFlag      = flag.Bool("continue", false, "Resume most recent session globally across projects (F11)")
		resumeSessionFlag = flag.String("resume-session", "", "Resume a specific session by ID (F11)")

		// F12: cloud LLM provider override.
		// Precedence (handled by llm.Select): --provider > HELIX_LLM_PROVIDER > config-file > wizard.
		// On ErrNoProviderConfigured we print a friendly message and continue with
		// the existing default Ollama provider — we never auto-launch the TUI here
		// because that would hang non-TTY runs. Users start the wizard explicitly
		// via `helixcode wizard`.
		providerFlag = flag.String("provider", "", "F12 cloud LLM provider override (anthropic|bedrock|vertexai|azure)")

		// F21: approval mode override.
		// Precedence (handled by approval.Select): --approval > HELIXCODE_APPROVAL >
		// $XDG_CONFIG_HOME/helixcode/approval.yaml > built-in default (suggest).
		// Garbage values fall through to the next source; the selector aggregates
		// parse errors so we can warn the user without losing the runtime mode.
		approvalFlag = flag.String("approval", "", "F21 approval mode override (suggest|auto-edit|full-auto|dangerously-bypass)")
	)
	flag.Parse()

	// Debug: print flag values
	fmt.Fprintf(os.Stderr, "Flags parsed: listWorkers=%v, nonInteractive=%v\n", *listWorkers, *nonInteractive)

	ctx := context.Background()

	// P1-F16-T10: Telemetry bootstrap.
	//
	// Order matters: telemetry MUST be constructed BEFORE the F12 LLM provider
	// is wrapped, BEFORE the tool registry is instrumented, and BEFORE the
	// /telemetry slash command is registered. Failure to construct (bad env
	// vars, exporter init failure) is non-fatal — NewTelemetryProvider returns
	// a noop provider in that case so the rest of the CLI keeps working. The
	// returned error is informational; we surface it via log so operators can
	// debug misconfiguration without losing the binary.
	//
	// Anti-bluff anchor: when HELIXCODE_OTEL_EXPORTER=stdout (or any other
	// real exporter), the wrapped LLM provider below ACTUALLY emits spans/
	// metrics through the OTel SDK pipeline. The gated integration tests in
	// tests/integration/telemetry_test.go prove this end-to-end with the real
	// stdouttrace + stdoutmetric exporters.
	telemetryCfg, telemetryCfgErr := telemetry.LoadConfigFromEnv(os.Getenv)
	if telemetryCfgErr != nil {
		log.Printf("telemetry: config invalid (continuing with noop): %v", telemetryCfgErr)
		telemetryCfg = telemetry.TelemetryConfig{Enabled: false, Exporter: telemetry.ExporterNoop}
	}
	telemetryProv, telemetryProvErr := telemetry.NewTelemetryProvider(telemetryCfg, zap.NewNop())
	if telemetryProvErr != nil {
		log.Printf("telemetry: provider construction failed (continuing with noop): %v", telemetryProvErr)
	}
	if telemetryProv == nil {
		// Defence in depth — NewTelemetryProvider should never return nil, but
		// if it ever did we'd nil-panic in the decorators below. Be paranoid.
		telemetryProv, _ = telemetry.NewTelemetryProvider(
			telemetry.TelemetryConfig{Enabled: false, Exporter: telemetry.ExporterNoop},
			zap.NewNop(),
		)
	}
	defer func() {
		shutdownTimeout := telemetryCfg.ShutdownTimeout
		if shutdownTimeout <= 0 {
			shutdownTimeout = telemetry.DefaultShutdownTimeout
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := telemetryProv.Shutdown(shutdownCtx); err != nil {
			log.Printf("telemetry: shutdown error: %v", err)
		}
	}()
	log.Printf("telemetry: initialised (exporter=%s)", string(telemetryProv.Exporter()))

	// Bootstrap permissions engine. A locally constructed PolicyEngine is used
	// here; T10/Phase 3 will thread it into the tool dispatcher so deny rules
	// actually block execution. For now the flag is parsed, validated, and the
	// engine is initialised — ready for wiring.
	c.permissionMode = *permissionMode
	policyEngine := confirmation.NewPolicyEngine()
	if err := c.initPermissions(ctx, policyEngine); err != nil {
		return fmt.Errorf("permissions bootstrap: %w", err)
	}
	if err := c.initPersistence(); err != nil {
		return fmt.Errorf("persistence init: %w", err)
	}
	if err := c.initWorktree(ctx); err != nil {
		return fmt.Errorf("worktree init: %w", err)
	}

	// Construct the session manager (carries the hooks.Manager inside it).
	sessionMgr := session.NewManager()
	c.sessionMgr = sessionMgr

	// Load hooks from ~/.helixcode/hooks.yaml + <cwd>/.helixcode/hooks.yaml
	// and register them with the session's hooks manager.
	if err := c.initHooks(ctx, sessionMgr); err != nil {
		return fmt.Errorf("hooks init: %w", err)
	}

	// Construct the tool registry and wire the hooks manager so that
	// BeforeToolCall / AfterToolCall / BeforeBash / AfterBash actually fire.
	toolReg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	if err != nil {
		return fmt.Errorf("tool registry init: %w", err)
	}
	toolReg.SetHooksManager(sessionMgr.GetHooksManager())
	// Wire F02 permissions engine into the confirmation pipeline.
	// The policyEngine was populated by initPermissions with deny/allow
	// rules from ~/.helixcode/permissions.yaml and <cwd>/.helixcode/
	// permissions.yaml. Without this wiring, permission rules validate
	// but do not block tool execution.
	toolReg.GetConfirmation().SetPolicyEngine(policyEngine)
	c.toolRegistry = toolReg

	// P1-F19-T05: ask_user tool registration. The askuser package's
	// stdinPrompter reads os.Stdin / writes os.Stdout, auto-detects whether
	// the destination is a TTY, and renders the question through the F18
	// renderer. Defaults: 3 retries, 5 minute per-line timeout. When the
	// destination is not a TTY, the prompter returns the question's Default
	// (if set) with UsedDefault=true, otherwise ErrInteractiveTerminalRequired.
	//
	// We use Register (not RegisterTool — no such method) and there is no
	// error path to handle. The previous in-tree bluff stub was removed in
	// internal/tools/registry.go::registerAllTools so this registration is
	// the SOLE wire-in for the "ask_user" name in the CLI.
	askUserPrompter, askUserErr := askuser.NewStdinPrompter(askuser.StdinPrompterOptions{})
	if askUserErr != nil {
		log.Printf("ask_user: stdinPrompter construction failed; tool unavailable: %v", askUserErr)
	} else {
		toolReg.Register(askuser.NewAskUserTool(askUserPrompter))
		log.Printf("ask_user: wired (interactive=auto-detect, max-retries=%d, timeout=%s)",
			askuser.DefaultMaxRetries, askuser.DefaultTimeout)
	}

	// F13: LSP manager — curated 5-server allowlist filtered by exec.LookPath
	// at startup. The manager is wired into the tool registry so successful
	// Edit-class tool calls (fs_edit / fs_write / multiedit_commit) auto-trigger
	// a NotifyChange and refresh diagnostics. The /lsp slash command and the
	// `helixcode lsp` cobra subcommand both consume this same manager.
	curatedLSPSpecs := tools.CuratedServerSpecs()
	detectedLSPSpecs := tools.DetectAvailableServers(curatedLSPSpecs)
	lspWorkingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("lsp manager: resolving cwd: %w", err)
	}
	lspManager := tools.NewLSPManager(lspWorkingDir, detectedLSPSpecs, zap.NewNop())
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = lspManager.Shutdown(shutCtx)
	}()
	toolReg.SetLSPManager(lspManager)

	// Construct the MCP Manager, load merged config from user + project YAML,
	// and start alwaysLoad servers. Errors are soft (logged) so a missing or
	// malformed mcp.yml never prevents the CLI from running.
	mcpMgr := mcp.NewManager()
	{
		var userMCPPath string
		if configHome, err := os.UserConfigDir(); err == nil {
			userMCPPath = filepath.Join(configHome, "helixcode", "mcp.yml")
		}
		projMCPPath := ".helixcode/mcp.yml"
		cfg, cfgErr := mcp.LoadMerged(userMCPPath, projMCPPath)
		if cfgErr != nil {
			log.Printf("mcp: config load failed: %v (continuing without MCP)", cfgErr)
		} else {
			mcpMgr.SetConfig(cfg)
			if startErr := mcpMgr.Start(ctx); startErr != nil {
				log.Printf("mcp: start failed: %v", startErr)
			}
		}
	}
	c.mcpManager = mcpMgr
	defer mcpMgr.Close() //nolint:errcheck

	// Wire MCP-discovered tools into the tool registry as "<server>:<tool>".
	toolReg.RegisterMCPManager(mcpMgr)

	// Build the commands registry and register all builtin slash commands,
	// including the /mcp command that requires an mcp.Manager.
	cmdRegistry := commands.NewRegistry()
	if regErr := builtin.RegisterBuiltinCommandsWithMCP(cmdRegistry, mcpMgr); regErr != nil {
		log.Printf("mcp: register slash command failed: %v", regErr)
	}
	c.commandRegistry = cmdRegistry

	// F07: background task manager — runs tools asynchronously when invoked
	// with run_in_background:true and powers the /tasks slash command.
	bgMgr := workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{})
	defer bgMgr.Close()
	toolReg.SetBackgroundManager(bgMgr)
	toolReg.RegisterTaskTools(bgMgr)
	// Register /tasks directly (not via RegisterBuiltinCommandsWithTasks which would
	// re-register the base commands already wired above by RegisterBuiltinCommandsWithMCP).
	if regErr := cmdRegistry.Register(commands.NewTasksCommand(bgMgr)); regErr != nil {
		log.Printf("tasks: register slash command failed: %v", regErr)
	}

	// F08: plan-mode gate — intercepts destructive tool calls when the agent is
	// operating in plan mode and enforces approval before execution.
	modeCtrl := planmode.NewModeController()
	planner := planmode.NewDefaultPlanner()
	gate := planmode.NewToolGate(modeCtrl, planner)
	toolReg.SetPlanModeGate(gate)

	// F08: register Enter/Exit plan-mode agent tools.
	toolReg.Register(tools.NewEnterPlanModeTool(modeCtrl))
	toolReg.Register(tools.NewExitPlanModeTool(modeCtrl))

	// F08: register /plan slash command directly (avoid double-registering base
	// commands through RegisterBuiltinCommandsWithPlanMode, which would conflict
	// with the F06 RegisterBuiltinCommandsWithMCP call already in this function).
	if regErr := cmdRegistry.Register(commands.NewPlanCommand(planner, modeCtrl)); regErr != nil {
		log.Printf("plan: register slash command failed: %v", regErr)
	}

	// F13: register /lsp slash command. Shares the same LSPManager + curated
	// allowlist constructed earlier in this function so the slash command and
	// the `helixcode lsp` cobra subcommand observe identical state.
	if regErr := cmdRegistry.Register(commands.NewLSPCommand(lspManager, curatedLSPSpecs)); regErr != nil {
		log.Printf("lsp: register slash command failed: %v", regErr)
	}

	// F14: sandbox manager + shell_sandboxed tool + /sandbox slash command.
	//
	// Wire order: load on-disk config (missing-file → defaults; ignore
	// not-exist), construct a SandboxManager via NewSandboxManagerFromDetector
	// using the inner Go module's working directory as the project root, then
	// register the agent-callable tool and the slash command. The detector
	// runs once at startup; /sandbox status surfaces the resolved capabilities
	// without re-detecting.
	//
	// Anti-bluff anchor: this is the SOLE wire-in for sandbox in the CLI. The
	// helper-mode dispatch at the top of main() handles the re-exec child;
	// every host-side command path goes through the manager constructed here.
	sandboxConfigPath := sandbox.DefaultConfigPath(os.Getenv)
	sandboxConfig, sandboxCfgErr := sandbox.LoadSandboxConfig(sandboxConfigPath)
	if sandboxCfgErr != nil {
		// LoadSandboxConfig already returns DefaultSandboxConfig() for missing
		// file; a non-nil error here means a bad YAML / negative limit. Log
		// and continue with safe defaults so a malformed user file does not
		// block the rest of the CLI.
		log.Printf("sandbox: config load failed (using defaults): %v", sandboxCfgErr)
		sandboxConfig = sandbox.DefaultSandboxConfig()
	}
	sandboxWorkDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("sandbox manager: resolving cwd: %w", err)
	}
	sandboxMgr, sandboxCaps, sbErr := sandbox.NewSandboxManagerFromDetector(sandboxWorkDir, sandboxConfig, zap.NewNop())
	if sbErr != nil {
		log.Printf("sandbox: manager init failed: %v", sbErr)
	} else {
		log.Printf("sandbox: backend=%s reason=%q",
			sandboxCaps.SelectedBackend.String(), sandboxCaps.UnavailableReason)
	}

	// Register the agent-callable tool. Even when SelectedBackend == None the
	// tool is registered so its Execute path surfaces a friendly fail-closed
	// error (with install hints) to the agent — better than the agent being
	// unable to see the tool at all.
	if sandboxMgr != nil {
		toolReg.Register(sandbox.NewSandboxedShellTool(sandboxMgr))
		if regErr := cmdRegistry.Register(commands.NewSandboxCommand(sandboxMgr)); regErr != nil {
			log.Printf("sandbox: register slash command failed: %v", regErr)
		}
	}

	// F21: approval gate. Resolution order is flag > env > config > default
	// (handled by approval.Select). When the resolved mode is ModeFullAuto,
	// the manager refuses to construct without an active sandbox — we surface
	// the error and fall through to the safe-default ModeSuggest so the rest
	// of the CLI keeps working. The /approval slash command is registered so
	// users can flip the mode at runtime via SetMode.
	//
	// Sandbox availability: SandboxManager.SelectedBackend() != BackendNone is
	// the canonical "sandbox usable" predicate (the manager surfaces a
	// FailClosedError on Execute when None). nil sandboxMgr (init failure) is
	// treated as unavailable.
	sandboxAvailable := sandboxMgr != nil && sandboxMgr.SelectedBackend() != sandbox.BackendNone
	approvalSelectorInput := approval.SelectorInput{
		Flag:       *approvalFlag,
		Env:        os.Getenv(approval.EnvVarName),
		ConfigPath: approval.DefaultConfigPath(os.Getenv),
	}
	approvalMode, approvalSource, approvalSelErr := approval.Select(approvalSelectorInput)
	if approvalSelErr != nil {
		log.Printf("approval: selector reported parse errors (using mode=%s source=%s): %v",
			approvalMode, approvalSource, approvalSelErr)
	}
	approvalMgrOpts := approval.ApprovalManagerOptions{
		InitialMode:      approvalMode,
		Source:           approvalSource,
		SandboxAvailable: sandboxAvailable,
		PauseDangerous:   2 * time.Second,
	}
	if askUserPrompter != nil {
		approvalMgrOpts.Responder = &approvalwire.AskUserYesNoPrompter{Inner: askUserPrompter}
	}
	approvalMgr, approvalMgrErr := approval.NewApprovalManager(approvalMgrOpts)
	if approvalMgrErr != nil {
		log.Printf("approval: manager init failed for mode=%s (falling back to suggest): %v",
			approvalMode, approvalMgrErr)
		approvalMgrOpts.InitialMode = approval.ModeSuggest
		approvalMgrOpts.Source = approval.SourceDefault
		approvalMgr, approvalMgrErr = approval.NewApprovalManager(approvalMgrOpts)
		if approvalMgrErr != nil {
			return fmt.Errorf("approval manager init (fallback): %w", approvalMgrErr)
		}
	}
	toolReg.SetApprovalManager(approvalMgr)
	log.Printf("approval: mode=%s source=%s sandbox_available=%t",
		approvalMgr.Mode(), approvalMgr.Source(), sandboxAvailable)
	if regErr := cmdRegistry.Register(commands.NewApprovalCommand(approvalMgr)); regErr != nil {
		log.Printf("approval: register slash command failed: %v", regErr)
	}

	// F22: per-edit git auto-commit (Aider-style).
	// Default-on; opt-out via HELIXCODE_GIT_AUTO_COMMIT=off or
	// /git_auto_commit off. WorkingDir is the process cwd at startup;
	// in subagent worktrees that's the worktree path (F04 invariant).
	acEnabled := os.Getenv(autocommit.EnvVarName) != "off"
	cwd, _ := os.Getwd()
	autoCommitter := autocommit.NewAutoCommitter(autocommit.Options{
		Enabled:    acEnabled,
		Provider:   c.llmProvider,
		WorkingDir: cwd,
		Logger:     zap.NewNop(),
	})
	toolReg.SetAutoCommitter(autoCommitter)
	log.Printf("git_auto_commit: enabled=%t cwd=%s git_repo=%t",
		autoCommitter.Enabled(), cwd, autoCommitter.IsGitRepo())
	if regErr := cmdRegistry.Register(commands.NewGitAutoCommitCommand(autoCommitter)); regErr != nil {
		log.Printf("git_auto_commit: register slash command failed: %v", regErr)
	}

	// F23: cline-style browser tool suite (browser_navigate, browser_snapshot,
	// browser_click, browser_type, browser_screenshot, browser_close) plus
	// the /browser slash command (status / navigate <url> / close).
	// Headless default; HELIXCODE_BROWSER_HEADED=true opt-in. Lazy-create on
	// first navigate; idempotent close. Defensive close-on-exit at the end
	// of main() via defer.
	browserMgr := browser.NewBrowserManager(browser.NewDefaultChromeDiscovery(), zap.NewNop())
	if err := tools.RegisterBrowserToolsV2(toolReg, browserMgr); err != nil {
		log.Printf("browser: register tools failed: %v", err)
	}
	c.browserManager = browserMgr
	if regErr := cmdRegistry.Register(commands.NewBrowserCommand(browserMgr)); regErr != nil {
		log.Printf("browser: register slash command failed: %v", regErr)
	}
	// Defensive close-on-exit: tear down chromium subprocess if a session
	// was lazily created during the run. Idempotent (sync.Once-guarded).
	defer browserMgr.CloseSession() //nolint:errcheck

	// F24: codex-style project memory subsystem.
	// Discovers helixcode.md / codex.md / AGENTS.md by parent-walking from
	// cwd up to a git root or filesystem root. Loads $XDG_CONFIG_HOME/
	// helixcode/memory.md as a per-user overlay (rendered AFTER project
	// memory). Hot-reloaded mid-session via fsnotify (200 ms debounce).
	// /memory slash exposes status/show/edit/reload. BaseAgent prepends
	// Memory.Render() to the system prompt on every LLM call (lock-free
	// atomic load — no caching).
	memCwd, _ := os.Getwd()
	memLoader := projectmemory.NewMemoryLoader(zap.NewNop())
	memRegistry := projectmemory.NewMemoryRegistry(memLoader, memCwd)
	if _, err := memRegistry.Reload(ctx); err != nil {
		log.Printf("projectmemory: initial reload failed: %v", err)
	}
	memWatcher := projectmemory.NewMemoryWatcher(memRegistry, zap.NewNop())
	if err := memWatcher.Start(ctx); err != nil {
		log.Printf("projectmemory: watcher start failed (degrading to slash-only reload): %v", err)
	}
	defer memWatcher.Close() //nolint:errcheck
	if regErr := cmdRegistry.Register(commands.NewMemoryCommand(memRegistry)); regErr != nil {
		log.Printf("projectmemory: register slash command failed: %v", regErr)
	}
	c.memoryRegistry = memRegistry

	// F25: plandex-style plan tree system.
	// Six agent tools (plan_create, plan_branch, plan_merge, plan_list,
	// plan_show, plan_delete) backed by FileStore at .helixcode/plans/.
	// /plan slash (list/show/compact/verify) with context compaction via
	// F01's compression.Summariser. Plan trees are agent-driven — the agent
	// calls plan_create/plan_branch/plan_merge as part of its task loop.
	planStore := plantree.NewFileStore(cwd)
	planSummariser := plantree.DeterministicSummariser{}
	if err := plantree.RegisterPlanTools(toolReg, planStore); err != nil {
		log.Printf("plantree: register tools failed: %v", err)
	}
	if regErr := cmdRegistry.Register(commands.NewPlanTreeCommand(planStore, planSummariser)); regErr != nil {
		log.Printf("plantree: register slash command failed: %v", regErr)
	}

	// F26: Openhands-style workspace + planner system.
	// Container-based per-task workspaces (Docker/Podman), sequential
	// step executor with retry, plan-tree-aware task planner. Tools:
	// workspace_create/list/cleanup + task_plan/task_step. Slash:
	// /openhands (list/create/cleanup).
	wsMgr, wsErr := workspace.NewWorkspaceManager()
	if wsErr != nil {
		log.Printf("workspace: manager init failed (container runtime not available): %v", wsErr)
	} else {
		toolReg.Register(workspace.NewWorkspaceCreateTool(wsMgr))
		toolReg.Register(workspace.NewWorkspaceListTool(wsMgr))
		toolReg.Register(workspace.NewWorkspaceCleanupTool(wsMgr))
		if regErr := cmdRegistry.Register(commands.NewOpenhandsCommand(wsMgr)); regErr != nil {
			log.Printf("openhands: register slash command failed: %v", regErr)
		}
	}

	plannerExec := taskplanner.NewSequentialExecutor(nil)
	toolReg.Register(taskplanner.NewTaskPlanTool(plannerExec))
	toolReg.Register(taskplanner.NewTaskStepTool(plannerExec))

	// F27: aider-style voice input + repo-map.
	// Voice: arecord/sox capture → Whisper API (with whisper.cpp fallback).
	// Repo-map: tree-sitter AST parsing (extends existing repomap package).
	// Tools: voice_start, voice_stop, voice_transcribe, repomap.
	// Slash: /aider (voice/repomap subcommands).
	voiceRec := voice.NewVoiceRecorder()
	voiceTrans := voice.NewVoiceTranscriber(voice.VoiceConfig{
		WhisperAPIKey: os.Getenv("OPENAI_API_KEY"),
	})
	toolReg.Register(voice.NewVoiceStartTool(voiceRec))
	toolReg.Register(voice.NewVoiceStopTool(voiceRec))
	toolReg.Register(voice.NewVoiceTranscribeTool(voiceRec, voiceTrans))
	if regErr := cmdRegistry.Register(commands.NewAiderCommand(voiceRec, voiceTrans)); regErr != nil {
		log.Printf("aider: register slash failed: %v", regErr)
	}

	// F28: kilo-code AST-aware refactoring.
	// Cross-file rename via tree-sitter + atomic F17 edits, impact analysis
	// with call graph + blast radius, refactoring suite (extract method,
	// inline call). Tools: kilocode_rename, kilocode_impact, kilocode_multi_edit.
	// Slash: /kilocode (rename/impact/edit subcommands).
	kcEngine := kilocode.NewRenameEngine(cwd)
	toolReg.Register(kilocode.NewKiloRenameTool(kcEngine))
	kcAnalyzer, kcErr := kilocode.NewImpactAnalyzer(cwd)
	if kcErr == nil {
		toolReg.Register(kilocode.NewKiloImpactTool(kcAnalyzer))
	}
	kcRefactorer := kilocode.NewRefactorer(cwd)
	toolReg.Register(kilocode.NewKiloMultiEditTool(kcRefactorer))
	if regErr := cmdRegistry.Register(commands.NewKilocodeCommand(kcEngine, kcAnalyzer, kcRefactorer)); regErr != nil {
		log.Printf("kilocode: register slash failed: %v", regErr)
	}

	// F29: Roo-code full port.
	// Task delegation via F15 subagents, template-based code generation,
	// diff-based code review, conversation-aware memory.
	// Tools: roo_delegate, roo_generate, roo_bootstrap. Slash: /roocode.
	rooDelegator := roocode.NewTaskDelegator()
	toolReg.Register(roocode.NewRooDelegateTool(rooDelegator))
	rooGen := roocode.NewCodeGenerator(cwd)
	toolReg.Register(roocode.NewRooGenerateTool(rooGen))
	toolReg.Register(roocode.NewRooBootstrapTool(rooGen))
	rooReviewer := roocode.NewCodeReviewer()
	rooConvStore := roocode.NewConversationStore()
	if regErr := cmdRegistry.Register(commands.NewRooCodeCommand(rooDelegator, rooGen, rooReviewer, rooConvStore)); regErr != nil {
		log.Printf("roocode: register slash failed: %v", regErr)
	}

	// F30: Continue.dev IDE integration.
	// Inline completions, workspace editor, multi-turn chat (F11 reuse),
	// diff viewer, model selector. Tools: continue_edit, continue_complete.
	// Slash: /continue (edit/complete/chat/diff subcommands).
	contEditor := continua.NewWorkspaceEditor()
	toolReg.Register(continua.NewContinueEditTool(contEditor))
	contCompletion := continua.NewCompletionEngine()
	toolReg.Register(continua.NewContinueCompleteTool(contCompletion))
	contChat := continua.NewChatManager()
	if regErr := cmdRegistry.Register(commands.NewContinueCommand(contEditor, contCompletion, contChat)); regErr != nil {
		log.Printf("continue: register slash failed: %v", regErr)
	}

	// F09: user-defined Markdown slash commands.
	// Project dir: ./.helix/commands; user dir: ~/.config/helixcode/commands (XDG).
	projectCmds := filepath.Join(".", ".helix", "commands")
	var userCmds string
	if userCfg, err := os.UserConfigDir(); err == nil {
		userCmds = filepath.Join(userCfg, "helixcode", "commands")
	}
	mdLoader := commands.NewMarkdownLoader(cmdRegistry, projectCmds, userCmds)
	if loadErr := mdLoader.Load(); loadErr != nil {
		log.Printf("markdown commands: load failed: %v", loadErr)
	}
	mdWatcher, mdwErr := commands.NewMarkdownWatcher(mdLoader, []string{projectCmds, userCmds})
	if mdwErr != nil {
		log.Printf("markdown commands: watcher init failed: %v", mdwErr)
	} else {
		go mdWatcher.Run(ctx)
		defer mdWatcher.Close() //nolint:errcheck
	}
	if regErr := cmdRegistry.Register(commands.NewCommandsCommand(mdLoader, cmdRegistry)); regErr != nil {
		log.Printf("commands: register slash failed: %v", regErr)
	}

	// F10: agent-invoked Skills.
	// Project dir: ./.helix/skills; user dir: ~/.config/helixcode/skills (XDG).
	skillProjectDir := filepath.Join(".", ".helix", "skills")
	var skillUserDir string
	if userCfg, err := os.UserConfigDir(); err == nil {
		skillUserDir = filepath.Join(userCfg, "helixcode", "skills")
	}
	skillReg := commands.NewSkillRegistry()
	skillLoader := commands.NewSkillLoader(skillReg, skillProjectDir, skillUserDir)
	if loadErr := skillLoader.Load(); loadErr != nil {
		log.Printf("skills: load failed: %v", loadErr)
	}
	skillWatcher, swErr := commands.NewSkillsWatcher(skillLoader, []string{skillProjectDir, skillUserDir})
	if swErr != nil {
		log.Printf("skills: watcher init failed: %v", swErr)
	} else {
		go skillWatcher.Run(ctx)
		defer skillWatcher.Close() //nolint:errcheck
	}
	// SkillDispatcher: caller can pass nil for wtMgr; isolation routing is
	// the caller's responsibility. Constructed here so the agent loop can
	// call .Match before each LLM turn.
	_ = agent.NewSkillDispatcher(skillReg, nil) // wired into baseAgent in a follow-up
	if regErr := cmdRegistry.Register(commands.NewSkillsCommand(skillLoader, skillReg)); regErr != nil {
		log.Printf("skills: register slash failed: %v", regErr)
	}

	// F11: session transcript persistence + resume.
	// Construct a TranscriptStore rooted at $XDG_DATA_HOME/helixcode/sessions/
	// (falling back to ~/.local/share/helixcode/sessions/), the ResumeFinder
	// that wraps it, and a SessionManager wired to the same store. Both the
	// /sessions slash command and the `helixcode sessions` cobra subcommand
	// share this TranscriptStore so they observe the same on-disk state.
	transcriptStore := session.NewTranscriptStore(sessionStoreBaseDir())
	resumeFinder := session.NewResumeFinder(transcriptStore)
	resumeMgr := session.NewSessionManager()
	resumeMgr.SetStore(transcriptStore)

	currentProject, projErr := session.ComputeProjectIdentity()
	if projErr != nil {
		log.Printf("session: project identity unresolved: %v (continuing with empty scope)", projErr)
		currentProject = ""
	}

	// Process F11 resume flags BEFORE the main dispatcher. A successful resume
	// rehydrates the in-memory transcript and sets the session manager's
	// CurrentID; a "no sessions found" error is downgraded to a friendly
	// message so the CLI continues with a fresh session.
	if *resumeSessionFlag != "" {
		if err := resumeMgr.Resume(ctx, *resumeSessionFlag); err != nil {
			return fmt.Errorf("resume session %s: %w", *resumeSessionFlag, err)
		}
		fmt.Fprintf(os.Stderr, "Resumed session %s (%d messages).\n",
			resumeMgr.CurrentID(), resumeMgr.LoadedMessageCountForTestF11())
	} else if *resumeFlag || *continueFlag {
		mode := session.ResumeProject
		scope := currentProject
		if *continueFlag {
			mode = session.ResumeGlobal
			scope = ""
		}
		target, ferr := resumeFinder.FindResumeTarget(ctx, mode, scope)
		if ferr != nil {
			fmt.Fprintf(os.Stderr, "No resumable session found (%v); starting fresh.\n", ferr)
		} else {
			if err := resumeMgr.Resume(ctx, target.SessionID); err != nil {
				return fmt.Errorf("resume session %s: %w", target.SessionID, err)
			}
			fmt.Fprintf(os.Stderr, "Resumed session %s (%d messages, last active %s).\n",
				resumeMgr.CurrentID(),
				resumeMgr.LoadedMessageCountForTestF11(),
				target.LastActivity.Format("2006-01-02 15:04:05"))
		}
	}

	// Register /sessions slash command. It uses the same TranscriptStore so
	// list/show/resume/delete operate on the live on-disk state.
	if regErr := cmdRegistry.Register(commands.NewSessionsCommand(transcriptStore, currentProject)); regErr != nil {
		log.Printf("sessions: register slash failed: %v", regErr)
	}

	// F12: resolve cloud LLM provider via flag > env > config-file precedence.
	// Failure modes are friendly: if no source is configured we keep the
	// existing default (Ollama) and tell the user how to configure cloud.
	// If construction fails (missing creds for the chosen type), we warn and
	// keep the default so the rest of the CLI still works for non-LLM paths.
	configProviderName, configEntry, configErr := loadProviderConfigFromDisk(os.Getenv)
	if configErr != nil && !errors.Is(configErr, os.ErrNotExist) {
		log.Printf("F12 provider: config load failed (continuing without): %v", configErr)
	}
	selectorInput := llm.SelectorInput{
		Flag:   *providerFlag,
		Env:    os.Getenv("HELIX_LLM_PROVIDER"),
		Config: configProviderName,
	}
	ptype, selErr := llm.Select(selectorInput)
	switch {
	case errors.Is(selErr, llm.ErrNoProviderConfigured):
		// Friendly hint, then keep the default provider.
		fmt.Fprintln(os.Stderr,
			"F12 provider: no cloud provider configured. "+
				"Run `helixcode wizard` or set HELIX_LLM_PROVIDER. "+
				"Continuing with the default local provider.")
	case selErr != nil:
		// User typed an unknown value -> fail loudly. This is non-zero exit
		// because it's an explicit, fixable user error.
		return fmt.Errorf("F12 provider: %w", selErr)
	default:
		// We have a resolved cloud type. If --provider/--env supplied a value
		// that's different from the on-disk config, configEntry won't have
		// matching credentials — try construction anyway and surface failures
		// as warnings (don't crash).
		entry := configEntry
		entry.Type = ptype
		cloud, cErr := llm.NewCloudProvider(ptype, entry)
		if cErr != nil {
			fmt.Fprintf(os.Stderr,
				"F12 provider: failed to construct %q (%v); "+
					"falling back to default local provider.\n", ptype, cErr)
		} else if cloud != nil {
			c.llmProvider = cloud
			fmt.Fprintf(os.Stderr, "F12 provider: using %q\n", ptype)
		}
	}

	// P1-F16-T10: Wrap the resolved LLM provider with the telemetry decorator.
	//
	// Order anchor: this MUST run AFTER F12 has settled c.llmProvider AND
	// BEFORE the F15 SubagentManager is constructed below. Wrapping later (e.g.
	// after F15) would dispatch raw, untraced calls through the SubagentManager
	// — the manager captures the provider reference at construction time, so
	// any subsequent reassignment of c.llmProvider would not propagate.
	//
	// REPLACE-NOT-DUPLICATE invariant: c.llmProvider IS reassigned to the
	// traced wrapper. There is no shadow "raw" provider kept on the struct;
	// every downstream Generate / GenerateStream call now flows through
	// telemetry.TracedLLMProvider.Generate, which records a span + metrics.
	// When telemetry is in noop mode (the default when no OTEL_* env vars are
	// set), the decorator is effectively pass-through — the OTel noop tracer
	// and noop meter make every span/record operation a stub.
	//
	// Failure mode: if the decorator constructor fails (extremely rare; OTel
	// metric instrument names are static and known-good), we log and keep the
	// undecorated provider so the CLI still works.
	if c.llmProvider != nil {
		tracedLLM, traceErr := telemetry.NewTracedLLMProvider(c.llmProvider, telemetryProv)
		if traceErr != nil {
			log.Printf("telemetry: LLM decorator construction failed (using undecorated provider): %v", traceErr)
		} else {
			c.llmProvider = tracedLLM
		}
	}

	// P1-F16-T10: Wire telemetry into the tool registry. Each successful
	// ToolRegistry.Execute will now emit a "tool.<name>" span + tool-call
	// metrics labelled with outcome={success|failure}.
	if toolInstr, tiErr := telemetry.NewToolInstrumentation(telemetryProv); tiErr != nil {
		log.Printf("telemetry: tool instrumentation construction failed: %v", tiErr)
	} else {
		toolReg.SetTelemetryInstrumentation(toolInstr)
	}

	// P1-F16-T10: Register the /telemetry slash command. The command queries
	// the same telemetryProv constructed above so /telemetry status / show /
	// flush observe identical state.
	if regErr := cmdRegistry.Register(commands.NewTelemetryCommand(telemetryProv)); regErr != nil {
		log.Printf("telemetry: register slash command failed: %v", regErr)
	}

	// P1-F20-T07: Register the /theme slash command.
	//
	// The slash command is observation-only: it inspects the active theme
	// name, depth, and source so the operator can verify F20 detection at
	// runtime without grepping logs. Registry construction here is parallel
	// to (not shared with) the per-handleGenerate registry built later — both
	// pre-load the same built-ins + same optional theme.yaml so /theme list
	// and the styling code path observe identical state.
	//
	// Failure mode: every fallible step degrades gracefully. YAML missing /
	// malformed -> log + continue with built-ins. Get failure -> log +
	// fall back to dark. The slash command always registers; worst case is
	// /theme show <user-theme> errors and /theme list shows the three
	// built-ins only.
	{
		slashThemeRegistry := theme.NewThemeRegistry()
		if path := theme.DefaultThemePath(os.Getenv); path != "" {
			if err := slashThemeRegistry.LoadFromFile(path); err != nil {
				log.Printf("theme(slash): yaml load failed (continuing with built-ins): %v", err)
			}
		}
		slashThemeName := theme.DetectThemeName(os.Getenv)
		slashSelectedTheme, slashErr := slashThemeRegistry.Get(slashThemeName)
		if slashErr != nil {
			log.Printf("theme(slash): get %q failed (%v), falling back to dark", slashThemeName, slashErr)
			slashSelectedTheme, _ = slashThemeRegistry.Get(theme.ThemeDark)
			slashThemeName = theme.ThemeDark
		}
		slashColorDepth := theme.DetectColorDepth(os.Getenv)
		slashSource := commands.ResolveThemeSource(os.Getenv)
		slashStyler := theme.NewStyler(slashSelectedTheme, slashColorDepth)
		if regErr := cmdRegistry.Register(commands.NewThemeCommand(slashThemeRegistry, slashThemeName, slashColorDepth, slashSource, slashStyler)); regErr != nil {
			log.Printf("theme: register slash command failed: %v", regErr)
		}
	}

	// Note: BaseAgent (telemetry.AgentInstrumentation consumer) is NOT
	// constructed in this CLI entry point; agent-loop instrumentation is wired
	// at the call site in callers that own a BaseAgent (e.g. subagent helper
	// flows have their own dispatch). The AgentInstrumentation factory is
	// available via telemetry.NewAgentInstrumentation(telemetryProv) for those
	// callers and is exercised end-to-end by the gated integration tests.

	// P1-F15-T10: Subagent system wiring.
	//
	// Helper-mode dispatch (T08) already sits as the very first statement in
	// main(); that path never reaches here. This block wires the host-side
	// SubagentManager: spawners, task tool, /subagents slash command.
	//
	// Order:
	//   1. Construct in-process + subprocess spawners.
	//   2. Construct SubagentManager (requires non-nil LLMProvider).
	//      If c.llmProvider is nil (F12 default Ollama also failed), skip
	//      the entire block with a warning — the rest of the CLI keeps
	//      working for non-subagent paths.
	//   3. Register the `task` agent tool against the manager.
	//   4. Register the /subagents slash command.
	//   5. Defer Shutdown so running subagents are cancelled on CLI exit.
	//
	// Anti-bluff anchor: the manager rejects nil LLMProvider at construction
	// (subagent.NewSubagentManager). We mirror that gate here so a misconfigured
	// CLI never silently produces dispatchable-but-broken subagents — the
	// `task` tool is only registered when the manager truly works.
	{
		subagentLogger := zap.NewNop()
		subagentWorkDir, swdErr := os.Getwd()
		if swdErr != nil {
			log.Printf("subagent: resolving cwd failed (skipping wire-in): %v", swdErr)
		} else if c.llmProvider == nil {
			log.Printf("subagent: no LLM provider available (F12 default failed); skipping wire-in")
		} else {
			inProcessSpawner := subagent.NewInProcessSpawner()
			subprocessSpawner, spErr := subagent.NewSubprocessSpawner(subagentWorkDir)
			if spErr != nil {
				log.Printf("subagent: subprocess spawner unavailable (continuing in-process only): %v", spErr)
			}
			subagentMgr, smErr := subagent.NewSubagentManager(subagent.SubagentManagerOptions{
				InProcessSpawner:  inProcessSpawner,
				SubprocessSpawner: subprocessSpawner,
				LLMProvider:       c.llmProvider,
				Logger:            subagentLogger,
				WorkDir:           subagentWorkDir,
			})
			if smErr != nil {
				log.Printf("subagent: manager construction failed (skipping wire-in): %v", smErr)
			} else {
				defer func() {
					shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer shutCancel()
					_ = subagentMgr.Shutdown(shutCtx)
				}()
				toolReg.Register(task.NewTaskTool(subagentMgr))
				if regErr := cmdRegistry.Register(commands.NewSubagentsCommand(subagentMgr)); regErr != nil {
					log.Printf("subagent: register slash command failed: %v", regErr)
				}
				log.Printf("subagent: manager initialised (max_concurrency=%d)",
					subagent.DefaultMaxConcurrency)
			}
		}
	}

	// P1-F17-T08: Smart File Editing wiring.
	//
	// SmartEditTool wraps the existing F08 *multiedit.MultiFileEditor (already
	// constructed inside the tool registry's NewToolRegistry path and exposed
	// via toolReg.GetMultiEdit()) — there is NO parallel editor instance, the
	// transactional rollback semantics are inherited verbatim from multiedit.
	//
	// Wire order:
	//   1. Pull the existing multiedit MultiFileEditor out of the registry.
	//   2. Build a smartedit.MultiEditCommitter adapter around it.
	//   3. Resolve the smart-edit workdir to the same cwd everything else uses.
	//   4. Construct the SmartEditTool and register it under the "smart_edit"
	//      tool name so agents can call it.
	//   5. Register the /edit slash command pointing at the same SmartEditTool
	//      (it satisfies commands.SmartEditInspector via ParsePrompt/DryRun/Commit).
	//
	// Anti-bluff anchor: when toolReg.GetMultiEdit() returns nil (multiedit
	// failed to construct during registry init), we skip the entire block with
	// a warning rather than register a tool that would explode on first call.
	{
		mfe := toolReg.GetMultiEdit()
		if mfe == nil {
			log.Printf("smart-edit: multiedit unavailable; smart_edit tool + /edit slash skipped")
		} else {
			smartWorkDir, swdErr := os.Getwd()
			if swdErr != nil {
				log.Printf("smart-edit: resolving cwd failed (skipping wire-in): %v", swdErr)
			} else {
				committer := smartedit.NewMultieditCommitter(mfe)
				smartTool := smartedit.NewSmartEditTool(committer, smartWorkDir)
				toolReg.Register(smartTool)
				if regErr := cmdRegistry.Register(commands.NewEditCommand(smartTool)); regErr != nil {
					log.Printf("smart-edit: register slash command failed: %v", regErr)
				}
				log.Printf("smart-edit: wired (workdir=%s)", smartWorkDir)
			}
		}
	}

	// Handle different commands
	switch {
	case *listWorkers:
		return c.handleListWorkers(ctx)
	case *listModels:
		return c.handleListModels(ctx)
	case *healthCheck:
		return c.handleHealthCheck(ctx)
	case *workerHost != "":
		return c.handleAddWorker(ctx, *workerHost, *workerUser, *workerKey)
	case *prompt != "":
		return c.handleGenerate(ctx, *prompt, *model, *maxTokens, *temperature, *stream)
	case *notify != "":
		return c.handleNotification(ctx, *notify, *notifyType, *notifyPriority)
	case *qaRun:
		return c.handleQARun(ctx, *qaServerURL, *qaPlatforms, *qaBanks, *qaWait)
	case *qaList:
		return c.handleQAList(ctx, *qaServerURL)
	case *qaReport != "":
		return c.handleQAReport(ctx, *qaServerURL, *qaReport, *qaFormat)
	case *qaScreenshot != "":
		return c.handleQAScreenshot(ctx, *qaServerURL, *qaScreenshot)
	case *qaCancel != "":
		return c.handleQACancel(ctx, *qaServerURL, *qaCancel)
	case *command != "":
		return c.handleCommand(ctx, *command)
	case *nonInteractive:
		// In non-interactive mode, exit gracefully if no command specified
		return nil
	default:
		return c.handleInteractive(ctx)
	}
}

// handleListWorkers lists all workers
func (c *CLI) handleListWorkers(ctx context.Context) error {
	stats := c.workerPool.GetWorkerStats(ctx)

	fmt.Println("\n=== Worker Statistics ===")
	fmt.Printf("Total Workers: %d\n", stats.TotalWorkers)
	fmt.Printf("Active Workers: %d\n", stats.ActiveWorkers)
	fmt.Printf("Healthy Workers: %d\n", stats.HealthyWorkers)
	fmt.Printf("Total CPU: %d\n", stats.TotalCPU)
	fmt.Printf("Total Memory: %.2f GB\n", float64(stats.TotalMemory)/(1024*1024*1024))
	fmt.Printf("Total GPU: %d\n", stats.TotalGPU)

	return nil
}

// handleListModels lists available models.
// BLUFF-002 FIX: Uses LLMsVerifier as the single source of truth when enabled.
// Falls back to provider discovery and then to the constitutional fallback list.
func (c *CLI) handleListModels(ctx context.Context) error {
	fmt.Println("\n=== Available Models ===")

	// Priority 1: LLMsVerifier adapter (CONST-036 single source of truth)
	if c.verifierAdapter != nil && c.verifierAdapter.IsEnabled() {
		models, err := c.verifierAdapter.GetVerifiedModels(ctx)
		if err == nil && len(models) > 0 {
			c.printVerifiedModels(models)
			return nil
		}
		// Log warning but continue to fallback
		fmt.Fprintf(os.Stderr, "⚠️  Verifier unavailable (%v), using fallback...\n", err)
	}

	// Priority 2: Provider's own model list (e.g., Ollama /api/tags)
	if c.llmProvider != nil {
		providerModels := c.llmProvider.GetModels()
		if len(providerModels) > 0 {
			for _, m := range providerModels {
				fmt.Printf("ID: %s\n  Name: %s\n  Provider: %s\n  Context Size: %d\n  Status: available\n\n",
					m.ID, m.Name, m.Provider, m.ContextSize)
			}
			return nil
		}
	}

	// Priority 3: Constitutional fallback list (CONST-035 compliance)
	c.printVerifiedModels(verifier.FallbackModels)
	fmt.Println("ℹ️  Using fallback model list. Start LLMsVerifier for live data.")
	return nil
}

func (c *CLI) printVerifiedModels(models []*verifier.VerifiedModel) {
	for _, m := range models {
		status := "✓ verified"
		if !m.Verified {
			status = "○ pending"
		}
		if m.VerificationStatus == "failed" {
			status = "✗ failed"
		}
		if m.VerificationStatus == "rate_limited" {
			status = "⏳ rate-limited"
		}
		scoreStr := fmt.Sprintf("SC:%.1f", m.OverallScore)
		fmt.Printf("ID: %s\n  Name: %s\n  Provider: %s\n  Score: %s\n  Context Size: %d\n  Status: %s\n\n",
			m.ID, m.DisplayName, m.Provider, scoreStr, m.ContextSize, status)
	}
}

// handleHealthCheck performs system health check
func (c *CLI) handleHealthCheck(ctx context.Context) error {
	fmt.Println("\n=== System Health Check ===")

	// Check worker pool
	stats := c.workerPool.GetWorkerStats(ctx)
	if stats.HealthyWorkers > 0 {
		fmt.Printf("✅ Worker Pool: %d healthy workers\n", stats.HealthyWorkers)
	} else {
		fmt.Printf("⚠️ Worker Pool: No healthy workers\n")
	}

	// Check notification engine
	channelStats := c.notificationEngine.GetChannelStats()
	enabledChannels := 0
	for _, stats := range channelStats {
		if statsMap, ok := stats.(map[string]interface{}); ok {
			if enabled, ok := statsMap["enabled"].(bool); ok && enabled {
				enabledChannels++
			}
		}
	}

	if enabledChannels > 0 {
		fmt.Printf("✅ Notification System: %d enabled channels\n", enabledChannels)
	} else {
		fmt.Printf("⚠️ Notification System: No enabled channels\n")
	}

	fmt.Println("✅ System is operational")
	return nil
}

// handleAddWorker adds a new worker
func (c *CLI) handleAddWorker(ctx context.Context, host, username, keyPath string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}

	sshConfig := &worker.SSHWorkerConfig{
		Host:     host,
		Port:     22,
		Username: username,
		KeyPath:  keyPath,
	}

	worker := &worker.SSHWorker{
		Hostname:    host,
		DisplayName: fmt.Sprintf("worker-%s", host),
		SSHConfig:   sshConfig,
	}

	if err := c.workerPool.AddWorker(ctx, worker); err != nil {
		return fmt.Errorf("failed to add worker: %v", err)
	}

	fmt.Printf("✅ Worker added successfully: %s\n", host)
	return nil
}

	// handleGenerate performs LLM generation
func (c *CLI) handleGenerate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, stream bool) error {
	fmt.Printf("\n=== Generating with %s ===\n", model)
	fmt.Printf("Prompt: %s\n\n", prompt)

	// ANTI-BLUFF: MUST use real LLM provider, not simulation
	if c.llmProvider == nil {
		return fmt.Errorf("LLM provider not initialized - please check configuration")
	}

	// Parse model from string (format: provider:model or just model)
	modelName := model
	if strings.Contains(model, ":") {
		parts := strings.SplitN(model, ":", 2)
		modelName = parts[1]
	}

	// Get provider
	provider := c.llmProvider

	// Create generation request with the prompt as a user message
	req := &llm.LLMRequest{
		Model:       modelName,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      stream,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	}

	// P1-F20-T06: Theme + Styler construction.
	//
	// Building the registry / theme / depth here (rather than once at process
	// start) keeps the wire-in scoped to the only call site that consumes it
	// in F20 (the non-stream LLM body print). The cost is one map allocation
	// + an opportunistic YAML stat per generate; both are negligible relative
	// to the LLM call itself, and centralising the construction would require
	// threading a Styler through CLI's struct and Run() which are out of
	// scope for this task.
	//
	// Failure mode: every fallible step degrades gracefully. YAML missing /
	// malformed -> log + use built-ins. Unknown theme name -> log + dark
	// fallback. The user always gets a working binary; the worst case is
	// that styling falls back to the dark baseline.
	themeRegistry := theme.NewThemeRegistry()
	if path := theme.DefaultThemePath(os.Getenv); path != "" {
		if err := themeRegistry.LoadFromFile(path); err != nil {
			log.Printf("theme: yaml load failed (continuing with built-ins): %v", err)
		}
	}
	themeName := theme.DetectThemeName(os.Getenv)
	selectedTheme, themeErr := themeRegistry.Get(themeName)
	if themeErr != nil {
		// Should not happen for built-in names, but be defensive: fall back
		// to dark explicitly so an exotic HELIXCODE_THEME value can never
		// produce a zero-Theme that styles to no-op silently.
		log.Printf("theme: get %q failed (%v), falling back to dark", themeName, themeErr)
		selectedTheme, _ = themeRegistry.Get(theme.ThemeDark)
		themeName = theme.ThemeDark
	}
	colorDepth := theme.DetectColorDepth(os.Getenv)

	if stream {
		// Real streaming from provider, wired through the P1-F18 Renderer
		// so that token output respects HELIXCODE_RENDER + TTY detection
		// (fancy = in-place line update, plain = line-buffered transcript).
		r, rerr := render.NewRenderer(render.FactoryOptions{})
		if rerr != nil {
			// Constructing a renderer should be infallible for default
			// options; surface the error rather than silently dropping
			// to fmt.Printf — callers need to know if config is broken.
			return fmt.Errorf("renderer init failed: %w", rerr)
		}
		defer func() { _ = r.Close() }()
		// Plain-mode protection (F20 spec §11): even though we don't style
		// the streaming path in v1, log the active theme depth AFTER the
		// adjustment so operators see the same observable depth they'd see
		// for the non-stream branch.
		effectiveDepth := adjustDepthForRenderer(r, colorDepth)
		log.Printf("theme: name=%s depth=%s", themeName, effectiveDepth)

		chunkChan := make(chan llm.LLMResponse, 100)
		errCh := make(chan error, 1)
		go func() {
			errCh <- provider.GenerateStream(ctx, req, chunkChan)
		}()

		blockID := "llm-" + modelName
		if rerr := streamToRenderer(ctx, chunkChan, r, blockID); rerr != nil {
			// Drain provider error before returning to avoid goroutine leak.
			<-errCh
			return fmt.Errorf("renderer stream failed: %w", rerr)
		}
		if perr := <-errCh; perr != nil {
			return fmt.Errorf("streaming generation failed: %w", perr)
		}
	} else {
		// Real non-streaming from provider, wired through the P1-F18
		// Renderer (T08) so the full LLM response respects HELIXCODE_RENDER
		// + TTY detection. Plain mode -> zero-ANSI/zero-CR transcript;
		// fancy mode -> hide-cursor + per-line emit. The blockID is empty
		// (one-shot) because a non-stream completion is not re-rendered.
		//
		// P1-F20-T06: decorate the response body with theme.Styler before
		// it reaches RenderTextBlock. The depth is forced to DepthOff when
		// the renderer is in plain mode so the styler emits zero ANSI bytes
		// that the plain renderer would otherwise pass through verbatim.
		r, rerr := render.NewRenderer(render.FactoryOptions{})
		if rerr != nil {
			return fmt.Errorf("renderer init failed: %w", rerr)
		}
		defer func() { _ = r.Close() }()

		effectiveDepth := adjustDepthForRenderer(r, colorDepth)
		styler := theme.NewStyler(selectedTheme, effectiveDepth)
		log.Printf("theme: name=%s depth=%s", themeName, effectiveDepth)

		resp, err := provider.Generate(ctx, req)
		if err != nil {
			return fmt.Errorf("generation failed: %w", err)
		}
		if perr := printResponseThroughRendererStyled(r, styler, resp.Content); perr != nil {
			return fmt.Errorf("renderer print failed: %w", perr)
		}
	}

	fmt.Printf("\n✅ Generation completed\n")
	return nil
}

// streamToRenderer pumps LLM streaming chunks from ch through the supplied
// Renderer using the Begin -> WriteToken... -> Commit token-streaming flow
// (P1-F18 spec §4.1, §11.6).
//
// Invariants enforced:
//   - Begin is called exactly once with blockID before the first WriteToken.
//   - Commit runs unconditionally on return so the trailing newline is always
//     emitted, even if the producer goroutine returns an error mid-stream.
//   - ctx cancellation aborts the loop without surfacing an error from the
//     channel-drain phase; the caller is responsible for joining the producer
//     goroutine separately.
//
// The function reads until ch is closed by the producer; it does NOT close the
// channel itself. Empty content chunks are forwarded so the renderer's
// auto-Begin / no-op fast paths get exercised in the same way the production
// path will see them under real provider implementations.
func streamToRenderer(ctx context.Context, ch <-chan llm.LLMResponse, r render.Renderer, blockID string) (retErr error) {
	if err := r.Begin(blockID); err != nil {
		return fmt.Errorf("renderer begin: %w", err)
	}
	defer func() {
		if cerr := r.Commit(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("renderer commit: %w", cerr)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-ch:
			if !ok {
				return nil
			}
			if err := r.WriteToken(chunk.Content); err != nil {
				return fmt.Errorf("renderer write: %w", err)
			}
		}
	}
}

// printResponseThroughRenderer prints a single non-stream LLM response
// through the P1-F18 Renderer (T08) so the bytes flow through plain/fancy
// resolution + zero-ANSI/zero-CR enforcement instead of bare fmt.Println.
//
// Construction is local: the non-stream branch in handleGenerate doesn't
// need a longer-lived renderer (the response is one-shot, never re-rendered),
// so we build, render, and Close on each call. blockID is empty so
// RenderTextBlock generates a synthetic one-shot ID under the hood.
//
// Empty content is a no-op (mirrors RenderTextBlock contract); since
// fmt.Println("") would have emitted a stray newline we keep the empty case
// silent here to preserve transcript hygiene.
//
// Retained as a thin wrapper for the F18-T08 smoke tests; new call sites
// should prefer printResponseThroughRendererStyled which composes the
// theme.Styler decorator (P1-F20-T06) on top of the same RenderTextBlock
// path.
func printResponseThroughRenderer(content string) error {
	r, err := render.NewRenderer(render.FactoryOptions{})
	if err != nil {
		return fmt.Errorf("renderer init failed: %w", err)
	}
	defer func() { _ = r.Close() }()
	return printResponseThroughRendererStyled(r, nil, content)
}

// adjustDepthForRenderer collapses requested to theme.DepthOff when r is in
// plain mode; otherwise returns requested unchanged.
//
// Load-bearing per F20 spec §11: "plain mode forces zero color emission
// regardless of theme setting". The plain renderer's pass-through clause
// (plain_renderer.go: tool-output passthrough lets caller-supplied ANSI
// bytes reach the writer verbatim) means we MUST prevent the Styler from
// emitting ANSI in the first place when targeting a plain renderer —
// otherwise pre-styled bytes from Stylize() would leak straight through into
// log files and pipes. Forcing the depth here rather than down-converting
// the bytes keeps the chokepoint at a single, testable function.
//
// The fancy renderer applies its own escape sequences for cursor moves and
// dirty-line redraws, but it does NOT strip caller-supplied ANSI either; for
// fancy mode we WANT the styler to colour the text, so we pass the requested
// depth through untouched.
func adjustDepthForRenderer(r render.Renderer, requested theme.ColorDepth) theme.ColorDepth {
	if r != nil && r.Mode() == render.ModePlain {
		return theme.DepthOff
	}
	return requested
}

// printResponseThroughRendererStyled prints a single non-stream LLM response
// through the supplied Renderer, optionally decorating the text with the
// supplied Styler before passing it to RenderTextBlock.
//
// Why styling lives here and NOT in streamToRenderer: the streaming hot path
// emits one token at a time and relies on a stable per-line dirty-diff in
// fancy mode; injecting a per-token open/close sequence would either
// fragment the role-styled region across ANSI clear-line boundaries or
// require buffering the entire stream before colouring it. Both options
// regress streaming UX. We therefore restrict styling to the non-stream
// branch in v1, which is the plan's documented compromise (cf. plan T06).
//
// Role choice: theme.RoleHighlight. Justification: the LLM's final answer is
// the most important content in the transcript and benefits from a visual
// pop relative to the surrounding `=== Generating … ===` headers and the
// trailing `✅ Generation completed`, both of which are emitted via bare
// fmt.Printf and remain unstyled. RoleInfo would render the LLM body the
// same colour as routine status messages and lose the contrast.
//
// nil styler short-circuits to RenderTextBlock with the raw content — used
// by callers that opt out of theming (e.g., the F18 backward-compat shim
// printResponseThroughRenderer above) and by tests.
//
// Empty content is a no-op (mirrors RenderTextBlock contract).
func printResponseThroughRendererStyled(r render.Renderer, styler *theme.Styler, content string) error {
	if styler != nil {
		content = styler.Stylize(theme.RoleHighlight, content)
	}
	return render.RenderTextBlock(r, "", content)
}

// handleNotification sends a notification
func (c *CLI) handleNotification(ctx context.Context, message, notifyType, priority string) error {
	notificationType := notification.NotificationType(notifyType)
	notificationPriority := notification.NotificationPriority(priority)

	notif := &notification.Notification{
		Title:    "CLI Notification",
		Message:  message,
		Type:     notificationType,
		Priority: notificationPriority,
		Channels: []string{"cli"}, // Default to CLI output
	}

	if err := c.notificationEngine.SendDirect(ctx, notif, []string{"cli"}); err != nil {
		return fmt.Errorf("failed to send notification: %v", err)
	}

	fmt.Printf("✅ Notification sent: %s\n", message)
	return nil
}

// handleCommand executes a command locally via os/exec.
// ANTI-BLUFF (BLUFF-003 FIX): This executes REAL commands, not simulations.
func (c *CLI) handleCommand(ctx context.Context, command string) error {
	fmt.Printf("\n=== Executing Command ===\n")
	fmt.Printf("Command: %s\n\n", command)

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	fmt.Printf("\n✅ Command completed (exit code: %d)\n", cmd.ProcessState.ExitCode())
	return nil
}

// handleInteractive starts interactive mode
func (c *CLI) handleInteractive(ctx context.Context) error {
	fmt.Println("=== Helix CLI Interactive Mode ===")
	fmt.Println("Type 'help' for available commands, 'exit' to quit")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			fmt.Println("\n\nShutting down...")
			return nil
		default:
			// Continue with interactive loop
		}

		fmt.Print("\nhelix> ")

		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			if err.Error() == "unexpected newline" {
				continue
			}
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return nil
		}

		if input == "help" {
			c.showHelp()
			continue
		}

		// Handle interactive commands
		switch input {
		case "workers":
			c.handleListWorkers(ctx)
		case "models":
			c.handleListModels(ctx)
		case "health":
			c.handleHealthCheck(ctx)
		default:
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", input)
		}
	}
}

// showHelp displays available commands
func (c *CLI) showHelp() {
	fmt.Println("\n=== Available Commands ===")
	fmt.Println("workers          - List all workers")
	fmt.Println("models           - List available models")
	fmt.Println("health           - Perform system health check")
	fmt.Println("help             - Show this help message")
	fmt.Println("exit/quit        - Exit the CLI")
	fmt.Println("")
	fmt.Println("=== Command Line Options ===")
	fmt.Println("--list-workers   - List all workers")
	fmt.Println("--list-models    - List available models")
	fmt.Println("--health         - Perform health check")
	fmt.Println("--worker         - Add a worker (requires --user)")
	fmt.Println("--user           - Worker SSH username")
	fmt.Println("--key            - Worker SSH key path")
	fmt.Println("--prompt         - Generate with LLM")
	fmt.Println("--model          - LLM model to use")
	fmt.Println("--stream         - Stream the response")
	fmt.Println("--notify         - Send notification")
	fmt.Println("--notify-type    - Notification type (info/warning/error/success/alert)")
	fmt.Println("--notify-priority - Notification priority (low/medium/high/urgent)")
}

func main() {
	// P1-F15-T08: subagent helper-mode dispatch — MUST run BEFORE the F14
	// sandbox helper-mode dispatch (per spec §3 anchor). A subagent helper
	// child does NOT initialise the sandbox; the parent re-exec'd us with
	// HELIXCODE_SUBAGENT_HELPER=1 and a JSON-encoded SubagentTask in
	// HELIXCODE_SUBAGENT_HELPER_PAYLOAD. We must decode, run an
	// InProcessSpawner with the child's own LLM provider, write a
	// SubagentResult JSON to stdout, and exit — without touching the sandbox
	// or the rest of the CLI bootstrap.
	//
	// Anti-bluff anchor: this MUST be the very first statement in main().
	// Reordering it after the sandbox dispatch (or any flag parsing) would
	// break the protocol round-trip the parent depends on.
	if subagent.IsSubagentInvocation() {
		os.Exit(subagent.RunAsSubagent(buildSubagentLLMProvider))
	}

	// F14: native sandbox helper re-exec dispatch.
	//
	// The native backend launches the host binary with HELIX_SANDBOX_NATIVE_HELPER
	// set; that invocation must run the helper (mount /proc, apply rlimits,
	// chdir, exec /bin/sh -c <command>) and exit. It must NOT continue into
	// normal CLI logic — the helper child is inside fresh PID/MNT/USER/UTS/IPC
	// namespaces and re-entering the CLI bootstrap would hang or crash.
	//
	// Anti-bluff anchor: this dispatch MUST be the SECOND statement in
	// main() (the first is subagent.IsSubagentInvocation above) so no flag
	// parsing, cobra subcommand interception, or env-driven init runs before
	// the helper takes over. On non-Linux this is a no-op (IsHelperInvocation
	// returns false there — see native_backend_other.go).
	if sandbox.IsHelperInvocation() {
		os.Exit(sandbox.RunAsHelper())
	}

	// Minimal dispatcher: intercept the "permissions" subcommand group before
	// flag.Parse() so that Cobra handles its own flag parsing.
	if len(os.Args) > 1 && os.Args[1] == "permissions" {
		cmd := newPermissionsCommand()
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			log.Fatalf("Error: %v", err)
		}
		return
	}

	// Dispatcher: intercept the "worktree" subcommand group before flag.Parse()
	// so that Cobra handles its own flag parsing (same pattern as "permissions").
	if len(os.Args) >= 2 && os.Args[1] == "worktree" {
		cwd, _ := os.Getwd()
		m := worktree.NewManager(cwd)
		cmd := newWorktreeCommand(m)
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	// Dispatcher: intercept the "hooks" subcommand group before flag.Parse()
	// so that Cobra handles its own flag parsing (same pattern as "permissions" / "worktree").
	if len(os.Args) >= 2 && os.Args[1] == "hooks" {
		cmd := newHooksCommand()
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	// Dispatcher: intercept the "mcp" subcommand group before flag.Parse()
	// so that Cobra handles its own flag parsing (same pattern as "hooks").
	if len(os.Args) >= 2 && os.Args[1] == "mcp" {
		cmd := newMCPCommand(MCPCommandDeps{ConfigPath: ".helixcode/mcp.yml"})
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	// Dispatcher: intercept the "commands" subcommand group before flag.Parse()
	// so that Cobra handles its own flag parsing (same pattern as "mcp").
	// The loader and registry are constructed here with zero startup cost
	// (Load() is a no-op when dirs are absent); the full wiring runs later
	// inside cli.Run() for other code paths.
	if len(os.Args) >= 2 && os.Args[1] == "commands" {
		projectCmds := filepath.Join(".", ".helix", "commands")
		var userCmds string
		if userCfg, err := os.UserConfigDir(); err == nil {
			userCmds = filepath.Join(userCfg, "helixcode", "commands")
		}
		cmdReg := commands.NewRegistry()
		mdLdr := commands.NewMarkdownLoader(cmdReg, projectCmds, userCmds)
		if err := mdLdr.Load(); err != nil {
			log.Printf("commands dispatcher: load failed: %v", err)
		}
		cmd := newCommandsCmd(commandsCmdDeps{Loader: mdLdr, Registry: cmdReg})
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	// Dispatcher: intercept the "sessions" subcommand group before flag.Parse()
	// so that Cobra handles its own flag parsing (same pattern as "commands").
	// The TranscriptStore is constructed here directly off the F11 base dir so
	// `helixcode sessions list` works without a full CLI bootstrap.
	if len(os.Args) >= 2 && os.Args[1] == "sessions" {
		store := session.NewTranscriptStore(sessionStoreBaseDir())
		currentProject, _ := session.ComputeProjectIdentity()
		cmd := newSessionsCmd(sessionsCmdDeps{Store: store, CurrentProject: currentProject})
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	// Dispatcher: intercept the "wizard" subcommand group before flag.Parse()
	// so Cobra handles its own flag parsing (same pattern as "sessions").
	// The wizard runs RunWizard (interactive tview) or, when --provider is
	// supplied, builds a NonInteractiveResult and writes the YAML directly.
	if len(os.Args) >= 2 && os.Args[1] == "wizard" {
		cmd := newWizardCmd(wizardCmdDeps{})
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	// Dispatcher: intercept the "lsp" subcommand group before flag.Parse() so
	// Cobra handles its own flag parsing (same pattern as "sessions"/"wizard").
	// The LSPManager is constructed here directly off the curated allowlist
	// filtered by exec.LookPath, so `helixcode lsp list-servers` works without
	// the full CLI bootstrap. status/restart/stop also work but only over the
	// servers visible to this short-lived manager — for a long-running session
	// the in-process /lsp slash command is the right surface.
	if len(os.Args) >= 2 && os.Args[1] == "lsp" {
		curated := tools.CuratedServerSpecs()
		detected := tools.DetectAvailableServers(curated)
		cwd, _ := os.Getwd()
		mgr := tools.NewLSPManager(cwd, detected, zap.NewNop())
		defer func() {
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutCancel()
			_ = mgr.Shutdown(shutCtx)
		}()
		cmd := newLSPCmd(lspCmdDeps{Manager: mgr, CuratedSpecs: curated})
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	// Dispatcher: intercept the "skills" subcommand group before flag.Parse()
	// so that Cobra handles its own flag parsing (same pattern as "commands").
	if len(os.Args) >= 2 && os.Args[1] == "skills" {
		projDir := filepath.Join(".", ".helix", "skills")
		var userDir string
		if userCfg, err := os.UserConfigDir(); err == nil {
			userDir = filepath.Join(userCfg, "helixcode", "skills")
		}
		skillReg := commands.NewSkillRegistry()
		loader := commands.NewSkillLoader(skillReg, projDir, userDir)
		if err := loader.Load(); err != nil {
			log.Printf("skills dispatcher: load failed: %v", err)
		}
		cmd := newSkillsCmd(skillsCmdDeps{Loader: loader, Registry: skillReg})
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	cli := NewCLI()

	if err := cli.Run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}


// QA command handlers

func (c *CLI) handleQARun(ctx context.Context, serverURL, platforms, banks string, wait bool) error {
	if banks == "" {
		return fmt.Errorf("--qa-banks is required for --qa-run")
	}
	client := server.NewClient(serverURL)
	req := server.StartSessionRequest{
		Platforms: strings.Split(platforms, ","),
		Banks:     strings.Split(banks, ","),
	}
	state, err := client.StartQASession(req)
	if err != nil {
		return fmt.Errorf("failed to start QA session: %w", err)
	}
	fmt.Printf("QA Session started: %s\n", state.ID)
	fmt.Printf("Platforms: %s\n", strings.Join(state.Platforms, ", "))
	fmt.Printf("Status: %s\n", state.Status)

	if wait {
		fmt.Println("Waiting for session to complete...")
		if err := client.WaitForSession(state.ID, os.Stdout); err != nil {
			return err
		}
		fmt.Println("Session completed!")
	}
	return nil
}

func (c *CLI) handleQAList(ctx context.Context, serverURL string) error {
	client := server.NewClient(serverURL)
	sessions, err := client.ListQASessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}
	if len(sessions) == 0 {
		fmt.Println("No QA sessions found.")
		return nil
	}
	fmt.Println("\n=== QA Sessions ===")
	fmt.Printf("%-12s %-12s %-20s %-30s %-20s\n", "ID", "Status", "Phase", "Platforms", "Started")
	for _, s := range sessions {
		id := s.ID
		if len(id) > 10 {
			id = id[:10]
		}
		fmt.Printf("%-12s %-12s %-20s %-30s %-20s\n",
			id, s.Status, s.Phase, strings.Join(s.Platforms, ","), s.StartTime.Format("2006-01-02 15:04:05"))
	}
	return nil
}

func (c *CLI) handleQAReport(ctx context.Context, serverURL, sessionID, format string) error {
	client := server.NewClient(serverURL)
	data, err := client.GetReport(sessionID, format)
	if err != nil {
		return fmt.Errorf("failed to get report: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func (c *CLI) handleQAScreenshot(ctx context.Context, serverURL, sessionID string) error {
	client := server.NewClient(serverURL)
	data, meta, err := client.CaptureScreenshot(sessionID, "", false)
	if err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}
	filename := fmt.Sprintf("screenshot-%s.png", sessionID)
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}
	fmt.Printf("Screenshot saved: %s (%d bytes, platform=%s)\n", filename, len(data), meta["platform"])
	return nil
}

func (c *CLI) handleQACancel(ctx context.Context, serverURL, sessionID string) error {
	client := server.NewClient(serverURL)
	if err := client.CancelQASession(sessionID); err != nil {
		return fmt.Errorf("failed to cancel session: %w", err)
	}
	fmt.Printf("Session %s cancelled.\n", sessionID)
	return nil
}
