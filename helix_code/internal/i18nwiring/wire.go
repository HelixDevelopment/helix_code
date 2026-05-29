// Package i18nwiring is the central boot-time CONST-046 translator wiring
// for helix_code (HXC-036, Option A — boot-time wiring via a central
// WireTranslators helper, 2026-05-29).
//
// Background / why this package exists
// ------------------------------------
// The CONST-046 i18n migration (rounds 93..440) gave 74 internal packages
// each their OWN per-package DI seam: a local Translator interface, a
// NoopTranslator{} loud-echo default, a package-level SetTranslator(tr),
// and a bundles/active.en.yaml catalogue. That migration deliberately kept
// the packages project-NOT-aware (CONST-051(B) decoupling): a package
// declares the contract and a default, but is injected with a real
// translator from OUTSIDE.
//
// The boot-time injection was never written. `grep -rn '\.SetTranslator('`
// (non-test) across helix_code returned 0 call sites, so every package ran
// with NoopTranslator{} and users saw raw message-ID keys
// (e.g. "askuser_prompt_invalid_choice_hint") instead of resolved +
// interpolated text. This package is the missing wiring: it constructs a
// real *i18nadapter.Translator per package (from that package's embedded
// active.en.yaml) and injects it via the package's SetTranslator.
//
// WireAll() is called once at every real entry point that exercises a
// CONST-046-migrated prompter/flow, BEFORE any user-facing string is
// emitted (see cmd/cli buildSubsystems). It is safe to call more than once
// (each SetTranslator simply re-injects an equivalent translator) and is
// safe to call from tests that need the production render path.
//
// Scope (Phase 2 complete, 2026-05-29)
// ------------------------------------
// WireAll wires every CONST-046-migrated package that is IMPORTABLE from
// here: askuser + approval (Phase 1) plus the 61 importable Phase-2
// packages (every internal/* package with a SetTranslator seam + bundle,
// the importable `cmd` library package, and the self-seamed examples/i18n
// package). The 9 `cmd/*` entry-point packages are `package main` and
// therefore cannot be imported by this package (Go forbids importing
// main); each such binary wires its own translator in its own main() by
// calling its own i18n subpackage's NewTranslator + the binary's
// SetTranslator (see e.g. cmd/cli/main.go). Their bundle.go constructors
// were added in Phase 2 so that per-binary wiring is a one-liner.
//
// Per-package onboarding recipe (for any future migrated package)
// ---------------------------------------------------------------
// Each migrated package already ships its own Translator interface,
// NoopTranslator default, SetTranslator(tr), and i18n/bundles/active.<lang>.yaml.
// To wire one in:
//
//  1. Add a constructor to the package's i18n subpackage (copy
//     internal/tools/askuser/i18n/bundle.go verbatim, changing only the
//     package doc + the error-prefix string):
//
//     //go:embed bundles/active.en.yaml
//     var activeBundleFS embed.FS
//     const activeBundlePath = "bundles/active.en.yaml"
//     func NewTranslator(langs ...string) (Translator, error) { ... }
//
//     The constructor body is identical for every package because the
//     pkg/i18n + pkg/i18nadapter APIs are package-agnostic.
//
//  2. Add the package + its i18n subpackage to this file's import block
//     (aliased uniquely, e.g. `fooi18n "dev.helix.code/internal/foo/i18n"`
//     + `foopkg "dev.helix.code/internal/foo"`), then add one block to
//     WireAll below, copied from the askuser block verbatim:
//
//     if tr, err := fooi18n.NewTranslator(langs...); err != nil {
//     errs = append(errs, fmt.Errorf("foo: %w", err))
//     } else {
//     foopkg.SetTranslator(tr)
//     }
//
//     The explicit per-package block keeps the concrete Translator types
//     statically checked (no any-boxing / runtime type assertions) — each
//     SetTranslator's parameter type is verified at compile time. When a
//     package's directory name differs from its Go package name (e.g.
//     dir cmd → package cmd, or two `config` dirs at different paths), the
//     import alias is derived from the path so leaf-name collisions cannot
//     occur, and the SetTranslator call targets the correct Go identifier.
//
// A failed NewTranslator (corrupt/missing embedded bundle) is collected and
// returned as a joined error — WireAll never silently leaves a package on
// NoopTranslator{}, which would be a §11.4 PASS-bluff at the i18n layer.
package i18nwiring

import (
	"errors"
	"fmt"

	cmdpkg "dev.helix.code/cmd"
	cmdi18n "dev.helix.code/cmd/i18n"
	examplesi18n "dev.helix.code/examples/i18n"
	adapterspkg "dev.helix.code/internal/adapters"
	adaptersi18n "dev.helix.code/internal/adapters/i18n"
	agentpkg "dev.helix.code/internal/agent"
	agenti18n "dev.helix.code/internal/agent/i18n"
	approvalpkg "dev.helix.code/internal/approval"
	approvali18n "dev.helix.code/internal/approval/i18n"
	approvalwirepkg "dev.helix.code/internal/approvalwire"
	approvalwirei18n "dev.helix.code/internal/approvalwire/i18n"
	authpkg "dev.helix.code/internal/auth"
	authi18n "dev.helix.code/internal/auth/i18n"
	autocommitpkg "dev.helix.code/internal/autocommit"
	autocommiti18n "dev.helix.code/internal/autocommit/i18n"
	clarificationpkg "dev.helix.code/internal/clarification"
	clarificationi18n "dev.helix.code/internal/clarification/i18n"
	cogneepkg "dev.helix.code/internal/cognee"
	cogneei18n "dev.helix.code/internal/cognee/i18n"
	commandspkg "dev.helix.code/internal/commands"
	commands_builtinpkg "dev.helix.code/internal/commands/builtin"
	commands_builtini18n "dev.helix.code/internal/commands/builtin/i18n"
	commandsi18n "dev.helix.code/internal/commands/i18n"
	configpkg "dev.helix.code/internal/config"
	configi18n "dev.helix.code/internal/config/i18n"
	contextpkg "dev.helix.code/internal/context"
	contexti18n "dev.helix.code/internal/context/i18n"
	continuapkg "dev.helix.code/internal/continua"
	continuai18n "dev.helix.code/internal/continua/i18n"
	databasepkg "dev.helix.code/internal/database"
	databasei18n "dev.helix.code/internal/database/i18n"
	deploymentpkg "dev.helix.code/internal/deployment"
	deploymenti18n "dev.helix.code/internal/deployment/i18n"
	discoverypkg "dev.helix.code/internal/discovery"
	discoveryi18n "dev.helix.code/internal/discovery/i18n"
	editorpkg "dev.helix.code/internal/editor"
	editori18n "dev.helix.code/internal/editor/i18n"
	eventpkg "dev.helix.code/internal/event"
	eventi18n "dev.helix.code/internal/event/i18n"
	fixpkg "dev.helix.code/internal/fix"
	fixi18n "dev.helix.code/internal/fix/i18n"
	focuspkg "dev.helix.code/internal/focus"
	focusi18n "dev.helix.code/internal/focus/i18n"
	hardwarepkg "dev.helix.code/internal/hardware"
	hardwarei18n "dev.helix.code/internal/hardware/i18n"
	helixqapkg "dev.helix.code/internal/helixqa"
	helixqai18n "dev.helix.code/internal/helixqa/i18n"
	hookspkg "dev.helix.code/internal/hooks"
	hooksi18n "dev.helix.code/internal/hooks/i18n"
	kilocodepkg "dev.helix.code/internal/kilocode"
	kilocodei18n "dev.helix.code/internal/kilocode/i18n"
	llmpkg "dev.helix.code/internal/llm"
	llmi18n "dev.helix.code/internal/llm/i18n"
	loggingpkg "dev.helix.code/internal/logging"
	loggingi18n "dev.helix.code/internal/logging/i18n"
	logopkg "dev.helix.code/internal/logo"
	logoi18n "dev.helix.code/internal/logo/i18n"
	mcppkg "dev.helix.code/internal/mcp"
	mcpi18n "dev.helix.code/internal/mcp/i18n"
	memorypkg "dev.helix.code/internal/memory"
	memoryi18n "dev.helix.code/internal/memory/i18n"
	monitoringpkg "dev.helix.code/internal/monitoring"
	monitoringi18n "dev.helix.code/internal/monitoring/i18n"
	notificationpkg "dev.helix.code/internal/notification"
	notificationi18n "dev.helix.code/internal/notification/i18n"
	performancepkg "dev.helix.code/internal/performance"
	performancei18n "dev.helix.code/internal/performance/i18n"
	persistencepkg "dev.helix.code/internal/persistence"
	persistencei18n "dev.helix.code/internal/persistence/i18n"
	plannerpkg "dev.helix.code/internal/planner"
	planneri18n "dev.helix.code/internal/planner/i18n"
	plantreepkg "dev.helix.code/internal/plantree"
	plantreei18n "dev.helix.code/internal/plantree/i18n"
	pluginspkg "dev.helix.code/internal/plugins"
	pluginsi18n "dev.helix.code/internal/plugins/i18n"
	projectpkg "dev.helix.code/internal/project"
	projecti18n "dev.helix.code/internal/project/i18n"
	projectmemorypkg "dev.helix.code/internal/projectmemory"
	projectmemoryi18n "dev.helix.code/internal/projectmemory/i18n"
	providerpkg "dev.helix.code/internal/provider"
	provideri18n "dev.helix.code/internal/provider/i18n"
	providerspkg "dev.helix.code/internal/providers"
	providersi18n "dev.helix.code/internal/providers/i18n"
	qualitypkg "dev.helix.code/internal/quality"
	qualityi18n "dev.helix.code/internal/quality/i18n"
	redispkg "dev.helix.code/internal/redis"
	redisi18n "dev.helix.code/internal/redis/i18n"
	renderpkg "dev.helix.code/internal/render"
	renderi18n "dev.helix.code/internal/render/i18n"
	repomappkg "dev.helix.code/internal/repomap"
	repomapi18n "dev.helix.code/internal/repomap/i18n"
	roocodepkg "dev.helix.code/internal/roocode"
	roocodei18n "dev.helix.code/internal/roocode/i18n"
	rulespkg "dev.helix.code/internal/rules"
	rulesi18n "dev.helix.code/internal/rules/i18n"
	secretspkg "dev.helix.code/internal/secrets"
	secretsi18n "dev.helix.code/internal/secrets/i18n"
	securitypkg "dev.helix.code/internal/security"
	securityi18n "dev.helix.code/internal/security/i18n"
	serverpkg "dev.helix.code/internal/server"
	serveri18n "dev.helix.code/internal/server/i18n"
	sessionpkg "dev.helix.code/internal/session"
	sessioni18n "dev.helix.code/internal/session/i18n"
	taskpkg "dev.helix.code/internal/task"
	taski18n "dev.helix.code/internal/task/i18n"
	templatepkg "dev.helix.code/internal/template"
	templatei18n "dev.helix.code/internal/template/i18n"
	toolspkg "dev.helix.code/internal/tools"
	askuserpkg "dev.helix.code/internal/tools/askuser"
	askuseri18n "dev.helix.code/internal/tools/askuser/i18n"
	tools_confirmationpkg "dev.helix.code/internal/tools/confirmation"
	tools_confirmationi18n "dev.helix.code/internal/tools/confirmation/i18n"
	toolsi18n "dev.helix.code/internal/tools/i18n"
	tools_multieditpkg "dev.helix.code/internal/tools/multiedit"
	tools_multiediti18n "dev.helix.code/internal/tools/multiedit/i18n"
	verifierpkg "dev.helix.code/internal/verifier"
	verifieri18n "dev.helix.code/internal/verifier/i18n"
	versionpkg "dev.helix.code/internal/version"
	versioni18n "dev.helix.code/internal/version/i18n"
	voicepkg "dev.helix.code/internal/voice"
	voicei18n "dev.helix.code/internal/voice/i18n"
	workerpkg "dev.helix.code/internal/worker"
	workeri18n "dev.helix.code/internal/worker/i18n"
	workspacepkg "dev.helix.code/internal/workspace"
	workspacei18n "dev.helix.code/internal/workspace/i18n"
	autonomypkg "dev.helix.code/internal/workflow/autonomy"
	planmodepkg "dev.helix.code/internal/workflow/planmode"
	workflowi18n "dev.helix.code/internal/workflow/i18n"
)

// WireAll constructs a real translator for every CONST-046-migrated
// IMPORTABLE package and injects it via that package's SetTranslator.
// langs is the ordered accept-language preference chain forwarded to every
// package's NewTranslator (empty → en). It returns a joined error
// enumerating every package whose translator failed to construct; on
// success it returns nil and every wired package renders real interpolated
// text instead of raw message-ID keys.
//
// The 9 `cmd/*` main packages are not wired here (Go cannot import main);
// see the package doc — each main wires its own i18n subpackage's
// NewTranslator in its own boot path.
func WireAll(langs ...string) error {
	var errs []error

	// internal/tools/askuser — numbered-choice CLI prompt narrative.
	if tr, err := askuseri18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("askuser: %w", err))
	} else {
		askuserpkg.SetTranslator(tr)
	}

	// internal/approval — approval-gate prompt + mode-description narrative.
	if tr, err := approvali18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("approval: %w", err))
	} else {
		approvalpkg.SetTranslator(tr)
	}

	// cmd — CONST-046 user-facing narrative.
	if tr, err := cmdi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("cmd: %w", err))
	} else {
		cmdpkg.SetTranslator(tr)
	}

	// examples — CONST-046 user-facing narrative (seam in i18n subpkg).
	if tr, err := examplesi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("examples: %w", err))
	} else {
		examplesi18n.SetTranslator(tr)
	}

	// internal/adapters — CONST-046 user-facing narrative.
	if tr, err := adaptersi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/adapters: %w", err))
	} else {
		adapterspkg.SetTranslator(tr)
	}

	// internal/agent — CONST-046 user-facing narrative.
	if tr, err := agenti18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/agent: %w", err))
	} else {
		agentpkg.SetTranslator(tr)
	}

	// internal/approvalwire — CONST-046 user-facing narrative.
	if tr, err := approvalwirei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/approvalwire: %w", err))
	} else {
		approvalwirepkg.SetTranslator(tr)
	}

	// internal/auth — CONST-046 user-facing narrative.
	if tr, err := authi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/auth: %w", err))
	} else {
		authpkg.SetTranslator(tr)
	}

	// internal/autocommit — CONST-046 user-facing narrative.
	if tr, err := autocommiti18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/autocommit: %w", err))
	} else {
		autocommitpkg.SetTranslator(tr)
	}

	// internal/clarification — CONST-046 user-facing narrative.
	if tr, err := clarificationi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/clarification: %w", err))
	} else {
		clarificationpkg.SetTranslator(tr)
	}

	// internal/cognee — CONST-046 user-facing narrative.
	if tr, err := cogneei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/cognee: %w", err))
	} else {
		cogneepkg.SetTranslator(tr)
	}

	// internal/commands/builtin — CONST-046 user-facing narrative.
	if tr, err := commands_builtini18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/commands/builtin: %w", err))
	} else {
		commands_builtinpkg.SetTranslator(tr)
	}

	// internal/commands — CONST-046 user-facing narrative.
	if tr, err := commandsi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/commands: %w", err))
	} else {
		commandspkg.SetTranslator(tr)
	}

	// internal/config — CONST-046 user-facing narrative.
	if tr, err := configi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/config: %w", err))
	} else {
		configpkg.SetTranslator(tr)
	}

	// internal/context — CONST-046 user-facing narrative.
	if tr, err := contexti18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/context: %w", err))
	} else {
		contextpkg.SetTranslator(tr)
	}

	// internal/continua — CONST-046 user-facing narrative.
	if tr, err := continuai18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/continua: %w", err))
	} else {
		continuapkg.SetTranslator(tr)
	}

	// internal/database — CONST-046 user-facing narrative.
	if tr, err := databasei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/database: %w", err))
	} else {
		databasepkg.SetTranslator(tr)
	}

	// internal/deployment — CONST-046 user-facing narrative.
	if tr, err := deploymenti18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/deployment: %w", err))
	} else {
		deploymentpkg.SetTranslator(tr)
	}

	// internal/discovery — CONST-046 user-facing narrative.
	if tr, err := discoveryi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/discovery: %w", err))
	} else {
		discoverypkg.SetTranslator(tr)
	}

	// internal/editor — CONST-046 user-facing narrative.
	if tr, err := editori18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/editor: %w", err))
	} else {
		editorpkg.SetTranslator(tr)
	}

	// internal/event — CONST-046 user-facing narrative.
	if tr, err := eventi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/event: %w", err))
	} else {
		eventpkg.SetTranslator(tr)
	}

	// internal/fix — CONST-046 user-facing narrative.
	if tr, err := fixi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/fix: %w", err))
	} else {
		fixpkg.SetTranslator(tr)
	}

	// internal/focus — CONST-046 user-facing narrative.
	if tr, err := focusi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/focus: %w", err))
	} else {
		focuspkg.SetTranslator(tr)
	}

	// internal/hardware — CONST-046 user-facing narrative.
	if tr, err := hardwarei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/hardware: %w", err))
	} else {
		hardwarepkg.SetTranslator(tr)
	}

	// internal/helixqa — CONST-046 user-facing narrative.
	if tr, err := helixqai18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/helixqa: %w", err))
	} else {
		helixqapkg.SetTranslator(tr)
	}

	// internal/hooks — CONST-046 user-facing narrative.
	if tr, err := hooksi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/hooks: %w", err))
	} else {
		hookspkg.SetTranslator(tr)
	}

	// internal/kilocode — CONST-046 user-facing narrative.
	if tr, err := kilocodei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/kilocode: %w", err))
	} else {
		kilocodepkg.SetTranslator(tr)
	}

	// internal/llm — CONST-046 user-facing narrative.
	if tr, err := llmi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/llm: %w", err))
	} else {
		llmpkg.SetTranslator(tr)
	}

	// internal/logging — CONST-046 user-facing narrative.
	if tr, err := loggingi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/logging: %w", err))
	} else {
		loggingpkg.SetTranslator(tr)
	}

	// internal/logo — CONST-046 user-facing narrative.
	if tr, err := logoi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/logo: %w", err))
	} else {
		logopkg.SetTranslator(tr)
	}

	// internal/mcp — CONST-046 user-facing narrative.
	if tr, err := mcpi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/mcp: %w", err))
	} else {
		mcppkg.SetTranslator(tr)
	}

	// internal/memory — CONST-046 user-facing narrative.
	if tr, err := memoryi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/memory: %w", err))
	} else {
		memorypkg.SetTranslator(tr)
	}

	// internal/monitoring — CONST-046 user-facing narrative.
	if tr, err := monitoringi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/monitoring: %w", err))
	} else {
		monitoringpkg.SetTranslator(tr)
	}

	// internal/notification — CONST-046 user-facing narrative.
	if tr, err := notificationi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/notification: %w", err))
	} else {
		notificationpkg.SetTranslator(tr)
	}

	// internal/performance — CONST-046 user-facing narrative.
	if tr, err := performancei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/performance: %w", err))
	} else {
		performancepkg.SetTranslator(tr)
	}

	// internal/persistence — CONST-046 user-facing narrative.
	if tr, err := persistencei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/persistence: %w", err))
	} else {
		persistencepkg.SetTranslator(tr)
	}

	// internal/planner — CONST-046 user-facing narrative.
	if tr, err := planneri18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/planner: %w", err))
	} else {
		plannerpkg.SetTranslator(tr)
	}

	// internal/plantree — CONST-046 user-facing narrative.
	if tr, err := plantreei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/plantree: %w", err))
	} else {
		plantreepkg.SetTranslator(tr)
	}

	// internal/plugins — CONST-046 user-facing narrative.
	if tr, err := pluginsi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/plugins: %w", err))
	} else {
		pluginspkg.SetTranslator(tr)
	}

	// internal/project — CONST-046 user-facing narrative.
	if tr, err := projecti18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/project: %w", err))
	} else {
		projectpkg.SetTranslator(tr)
	}

	// internal/projectmemory — CONST-046 user-facing narrative.
	if tr, err := projectmemoryi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/projectmemory: %w", err))
	} else {
		projectmemorypkg.SetTranslator(tr)
	}

	// internal/provider — CONST-046 user-facing narrative.
	if tr, err := provideri18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/provider: %w", err))
	} else {
		providerpkg.SetTranslator(tr)
	}

	// internal/providers — CONST-046 user-facing narrative.
	if tr, err := providersi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/providers: %w", err))
	} else {
		providerspkg.SetTranslator(tr)
	}

	// internal/quality — CONST-046 user-facing narrative.
	if tr, err := qualityi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/quality: %w", err))
	} else {
		qualitypkg.SetTranslator(tr)
	}

	// internal/redis — CONST-046 user-facing narrative.
	if tr, err := redisi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/redis: %w", err))
	} else {
		redispkg.SetTranslator(tr)
	}

	// internal/render — CONST-046 user-facing narrative.
	if tr, err := renderi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/render: %w", err))
	} else {
		renderpkg.SetTranslator(tr)
	}

	// internal/repomap — CONST-046 user-facing narrative.
	if tr, err := repomapi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/repomap: %w", err))
	} else {
		repomappkg.SetTranslator(tr)
	}

	// internal/roocode — CONST-046 user-facing narrative.
	if tr, err := roocodei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/roocode: %w", err))
	} else {
		roocodepkg.SetTranslator(tr)
	}

	// internal/rules — CONST-046 user-facing narrative.
	if tr, err := rulesi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/rules: %w", err))
	} else {
		rulespkg.SetTranslator(tr)
	}

	// internal/secrets — CONST-046 user-facing narrative.
	if tr, err := secretsi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/secrets: %w", err))
	} else {
		secretspkg.SetTranslator(tr)
	}

	// internal/security — CONST-046 user-facing narrative.
	if tr, err := securityi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/security: %w", err))
	} else {
		securitypkg.SetTranslator(tr)
	}

	// internal/server — CONST-046 user-facing narrative.
	if tr, err := serveri18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/server: %w", err))
	} else {
		serverpkg.SetTranslator(tr)
	}

	// internal/session — CONST-046 user-facing narrative.
	if tr, err := sessioni18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/session: %w", err))
	} else {
		sessionpkg.SetTranslator(tr)
	}

	// internal/task — CONST-046 user-facing narrative.
	if tr, err := taski18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/task: %w", err))
	} else {
		taskpkg.SetTranslator(tr)
	}

	// internal/template — CONST-046 user-facing narrative.
	if tr, err := templatei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/template: %w", err))
	} else {
		templatepkg.SetTranslator(tr)
	}

	// internal/tools/confirmation — CONST-046 user-facing narrative.
	if tr, err := tools_confirmationi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/tools/confirmation: %w", err))
	} else {
		tools_confirmationpkg.SetTranslator(tr)
	}

	// internal/tools — CONST-046 user-facing narrative.
	if tr, err := toolsi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/tools: %w", err))
	} else {
		toolspkg.SetTranslator(tr)
	}

	// internal/tools/multiedit — CONST-046 user-facing narrative.
	if tr, err := tools_multiediti18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/tools/multiedit: %w", err))
	} else {
		tools_multieditpkg.SetTranslator(tr)
	}

	// internal/verifier — CONST-046 user-facing narrative.
	if tr, err := verifieri18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/verifier: %w", err))
	} else {
		verifierpkg.SetTranslator(tr)
	}

	// internal/version — CONST-046 user-facing narrative.
	if tr, err := versioni18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/version: %w", err))
	} else {
		versionpkg.SetTranslator(tr)
	}

	// internal/voice — CONST-046 user-facing narrative.
	if tr, err := voicei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/voice: %w", err))
	} else {
		voicepkg.SetTranslator(tr)
	}

	// internal/worker — CONST-046 user-facing narrative.
	if tr, err := workeri18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/worker: %w", err))
	} else {
		workerpkg.SetTranslator(tr)
	}

	// internal/workspace — CONST-046 user-facing narrative.
	if tr, err := workspacei18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/workspace: %w", err))
	} else {
		workspacepkg.SetTranslator(tr)
	}

	// internal/workflow/{autonomy,planmode} — autonomy-mode display
	// labels + plan-mode execution-progress / validation-report /
	// option-presenter narrative. Both packages share the single
	// internal/workflow/i18n Translator type and bundle, so one
	// translator is constructed and injected into both seams.
	if tr, err := workflowi18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("internal/workflow: %w", err))
	} else {
		autonomypkg.SetTranslator(tr)
		planmodepkg.SetTranslator(tr)
	}

	if len(errs) > 0 {
		return fmt.Errorf("i18nwiring.WireAll: %w", errors.Join(errs...))
	}
	return nil
}

// MustWireAll is the panic-on-error variant for entry points where a failed
// translator build is unrecoverable (a binary that cannot localize its
// prompts must not silently boot into raw-key-echo mode). Boot code that
// prefers to log-and-degrade should call WireAll and handle the error.
func MustWireAll(langs ...string) {
	if err := WireAll(langs...); err != nil {
		panic(err)
	}
}
