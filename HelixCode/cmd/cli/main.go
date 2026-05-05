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

	"dev.helix.code/internal/agent"
	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/commands/builtin"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/hooks"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/mcp"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/server"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
	"dev.helix.code/internal/tools/persistence"
	"dev.helix.code/internal/tools/worktree"
	"dev.helix.code/internal/verifier"
	"dev.helix.code/internal/worker"
	"dev.helix.code/internal/workflow"
	"dev.helix.code/internal/workflow/planmode"
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
	hooksLoaded        int // count of hooks loaded at startup (for diagnostics)
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
	)
	flag.Parse()

	// Debug: print flag values
	fmt.Fprintf(os.Stderr, "Flags parsed: listWorkers=%v, nonInteractive=%v\n", *listWorkers, *nonInteractive)

	ctx := context.Background()

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
	c.toolRegistry = toolReg

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

	if stream {
		// Real streaming from provider
		chunkChan := make(chan llm.LLMResponse, 100)
		err := provider.GenerateStream(ctx, req, chunkChan)
		if err != nil {
			return fmt.Errorf("streaming generation failed: %w", err)
		}
		for chunk := range chunkChan {
			fmt.Printf("%s ", chunk.Content)
		}
		fmt.Println()
	} else {
		// Real non-streaming from provider
		resp, err := provider.Generate(ctx, req)
		if err != nil {
			return fmt.Errorf("generation failed: %w", err)
		}
		fmt.Println(resp.Content)
	}

	fmt.Printf("\n✅ Generation completed\n")
	return nil
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
