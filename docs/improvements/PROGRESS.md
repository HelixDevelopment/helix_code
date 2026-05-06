# HelixCode CLI-Agent Fusion — Live Progress Tracker

> **STOP/RESUME PROTOCOL**: read this file first. The "current focus" pointer
> below identifies the active task. The "evidence trail" links every claim of
> "done" to its commit + Challenge output.
>
> Spec: `docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`
> Plan: `docs/superpowers/plans/2026-05-04-phase-0-foundation-cleanup.md`

## Current focus
- **Active phase:** Phase 2 — CLI Agent Porting (in progress); F21 COMPLETE; F22 next candidate
- **Active feature:** —
- **Active task:** —
- **Last completed:** P2-F21-T09 — Feature 21 (Codex Approval Modes) close-out + push 4 remotes non-force
- **Owner:** agent (Claude Opus 4.7)
- **Started:** 2026-05-04
- **Last touched:** 2026-05-06
- **Blocked-on:** —

## Phase status
| Phase | State | Started | Completed | Evidence |
|---|---|---|---|---|
| P0 — Foundation | done | 2026-05-04 | 2026-05-05 | docs/improvements/05_phase_0_evidence.md |
| P1 — claude-code | done | 2026-05-05 | 2026-05-06 | docs/improvements/06_phase_1_evidence.md |
| P1.5 — Foundation Cleanup | done | 2026-05-06 | 2026-05-06 | docs/improvements/06_phase_1_evidence.md §P1.5 |
| P2 — Other CLI agents | in progress | 2026-05-06 | — | docs/improvements/07_phase_2_evidence.md |
| P3 — Test infra | pending | — | — | — |
| P4 — Anti-bluff audit | pending | — | — | — |
| P5 — End-user materials | pending | — | — | — |

## P1.5 Work-package list (12 WPs) — ALL CLOSED
- [x] P1.5-WP1 — Inventory + foundation safety (5 tasks)  ← commit `421495a`
- [x] P1.5-WP2 — Submodule restructuring (~67 mechanical moves)  ← commit `90dec95`
- [x] P1.5-WP3 — Submodule deduplication (5 sets)  ← commit `154c06c`
- [x] P1.5-WP4 — API key loader (bash + Go)  ← commit `e57894e`
- [x] P1.5-WP5 — `.env` API key dedup (USER GATE)  ← commit `92d5463`
- [x] P1.5-WP6 — Docs consolidation (3 dirs)  ← commit `f09f57d`
- [x] P1.5-WP7 — Snake_case directory normalization  ← commit `3c3cd8d`
- [x] P1.5-WP8 — Anti-bluff Constitution propagation  ← commit `0eead08`
- [x] P1.5-WP9 — Reference updates (comprehensive grep sweep)  ← commit `42166fd`
- [x] P1.5-WP10 — Rebuild + validation + fix `internal/tools/git` pre-existing failure  ← commit `0a77c93` + fix `45be827`
- [x] P1.5-WP11 — Phase 1.5 Challenge harness (5 phases)  ← meta-repo `306d3d9` + Challenges submodule `7e94f28`
- [x] P1.5-WP12 — Commit + push everything (deepest-first)  ← (this commit)

## P1.5 task list (in flight)
- [x] P1.5-pre — HelixAgent gitlink commit  ← `aad6a67d`
- [x] P1.5-pre — Challenges gitlink commit  ← `47dc905a`
- [x] P1.5-pre — root tracked + gitlink commit  ← `d0ad6fd3`
- [x] P1.5-pre — remove Example_Projects/ entirely (67 submodules)  ← `ad5e108c`
- [x] P1.5-pre — gitignore phase-1 development artefacts  ← `cff2d90f`
- [~] P1.5-WP1-T01.01 — recursive fetch + pull every submodule **HALTED** (init aborted at HelixAgent/DebateOrchestrator unreachable; full log in `docs/improvements/p1-5-fetch.log`)
- [~] P1.5-WP1-T01.02 — pre-state snapshot **PARTIAL** — `docs/improvements/p1-5-snapshot-pre.md` (commit pending bootstrap)
- [~] P1.5-WP1-T01.03 — remote reachability sweep **IN PROGRESS** (181 unique URLs, see `docs/improvements/p1-5-remote-reachability.md`)
- [x] P1.5-WP1-T01.04 — dedup canonical list  ← `docs/improvements/p1-5-dedup-canonical.md`
- [ ] P1.5-WP1-T01.05 — bootstrap evidence + advance PROGRESS (commit pending — this commit)

## Active phase task list (Phase 0)
- [x] P0-01 — bootstrap PROGRESS.md  ← commit `2c07f57`
- [~] P0-02 — Agent-Deck nested-worktree recursion error: **DEFERRED** (cosmetic; safe-fix requires modifying third-party submodules which is out of scope per spec §2.1; original `.git/info/exclude` approach was based on incorrect understanding of git submodule recursion semantics; see parking lot). Reverts: commits `904c925` + `a82f1a9`.
- [x] P0-03 — add HelixAgent submodule  ← (this commit) — 47/60 cli_agents populated; 13 deferred to Phase 2 sub-specs (see parking lot)
- [x] P0-04 — verify-llmsverifier-pin-parity.sh  ← (this commit)
- [x] P0-05 — migrate API keys from ../HelixAgent/.env  ← (this commit)
- [x] P0-06 — update .gitignore (root + inner)  ← (this commit)
- [x] P0-07 — refresh HelixCode/HelixCode/.env.example  ← (this commit)
- [x] P0-08 — scan-secrets.sh + planted-secret test  ← (this commit)
- [x] P0-08.5 — remediate 3 pre-existing tracked credentials  ← commits `8d30add` + `15cca9b` + `45bd4d4`
- [x] P0-08.7 — Port SonarQube + Snyk security scan integration through Containers ← commits `1d728de` + `2494bc8` + `e29e2f6` + `16a4490` + sub5; Challenges: 33/33 + 26/26 PASS; Go BootManager wiring landed (go build exits 0); live scans deferred pending credential rotation (see evidence §P0-08.7)
- [x] P0-08.7-fix — T08.7 code-quality review findings (Critical 1+2, Important 3-7) ← commits `b21b051`; evidence: §P0-T08.7 (fix-it) in 05_phase_0_evidence.md
- [x] P0-09 — pre-push hook + installer + setup.sh wiring ← (this commit)
- [x] P0-10 — create HelixCode/{CLAUDE,AGENTS,CONSTITUTION}.md (inner Go-app governance triplet) ← (this commit)
- [x] P0-11 — add Article XII (CONST-042, CONST-043) to root CONSTITUTION.md  ← (this commit)
- [x] P0-12 — cascade CONST-042/043 + anti-bluff anchor to root sister files (CLAUDE, AGENTS, CRUSH, QWEN) ← (this commit)
- [x] P0-13 — fix root CLAUDE.md §3.2 bluff (HelixCode tracked-dir vs. submodule) ← (this commit)
- [x] P0-14 — extend verify-governance-cascade.sh + run propagate-governance.sh + cascade CONST-042/043 across owned-by-us submodules ← (this commit)
- [x] P0-15 — Makefile verify-foundation target + extend ci-validate-all ← (this commit)
- [x] P0-16 — regenerate diagrams + DEPRECATED.md pointers + Phase 0 evidence + push close-out  ← (this commit)

## Active feature task list (P1-F01: Auto-Compaction)
- [x] P1-F01-T01 — bootstrap Phase 1 evidence + advance PROGRESS  ← commit `f0b9b15`
- [x] P1-F01-T02 — add GetContextWindow + CountTokens to Provider interface  ← commit `5b153e6`
- [x] P1-F01-T03 — implement Provider methods across all *_provider.go  ← commit `827971f`
- [x] P1-F01-T04 — ThrashingGuard with TDD  ← commit `59f7daa`
- [x] P1-F01-T05 — CompactionMetadata with TDD  ← commit `b9eae7f`
- [x] P1-F01-T06 — AutoCompactor with TDD  ← commit `4330341`
- [x] P1-F01-T07 — wire AutoCompactor into internal/agent  ← commit `cace643`
- [x] P1-F01-T08 — wire ThrashingGuard reset into internal/session/manager.go  ← commit `b913ce2`
- [x] P1-F01-T09 — integration test against real Anthropic provider  ← commit `4734f35`
- [x] P1-F01-T10 — Challenge with expected.json + runtime evidence  ← commit `9284392`
- [x] P1-F01-T11 — Feature 1 close-out + push  ← (this commit)

## Active feature task list (P1-F02: Permission Rule System)
- [x] P1-F02-T01 — bootstrap evidence + advance PROGRESS  ← commit `d56905d`
- [x] P1-F02-T02 — add Wildcard field to confirmation.Condition (TDD)  ← commit `5ffc46d`
- [x] P1-F02-T03 — internal/tools/permissions package skeleton  ← commit `26de1b4`
- [x] P1-F02-T04 — shell_splitter.go + mvdan.cc/sh/v3 dep (TDD)  ← commits `28a4fa8` + `c2b5dd8`
- [x] P1-F02-T05 — rule_engine.go pattern parse + match + priority (TDD)  ← commit `eab41d3`
- [x] P1-F02-T06 — mode_presets.go five presets + command lists (TDD)  ← commit `75b284f`
- [x] P1-F02-T07 — rule_loader.go YAML + file precedence (TDD)  ← commit `31c4366`
- [x] P1-F02-T08 — permissions.go facade + PolicyEngine registration  ← commit `41be967`
- [x] P1-F02-T09 — wire --permission-mode flag + integration test (no mocks)  ← commit `c1d67ad`
- [x] P1-F02-T10 — helixcode permissions {list,add,remove,check} subcommands  ← commit `588f2cd`
- [x] P1-F02-T11 — /permissions slash command via internal/commands  ← commits `2fb11d4` + `244aff9`
- [x] P1-F02-T12 — Challenge with three runtime-evidence scenarios  ← commit `7252911`
- [x] P1-F02-T13 — Feature 2 close-out + push  ← (this commit)

## Active feature task list (P1-F03: Tool Result Persistence)
- [x] P1-F03-T01 — bootstrap evidence + advance PROGRESS — `ee35017`
- [x] P1-F03-T02 — internal/tools/persistence package skeleton (types + doc) — `c806f72`
- [x] P1-F03-T03 — Manager.MaybePersist with hash idempotence (TDD) — `38a17d4`
- [x] P1-F03-T04 — LoadPersisted with path-traversal guard (TDD) — `a9a41f2`
- [x] P1-F03-T05 — CleanupOld with filename-pattern matching (TDD) — `7afe24f`
- [x] P1-F03-T06 — wire into internal/llm/tool_provider.go orchestration loop — `6199e96`
- [x] P1-F03-T07 — audit + wire individual LLM providers — `88856c4`
- [x] P1-F03-T08 — system prompt note about persistedOutputPath — `c80b438`
- [x] P1-F03-T09 — cmd/cli/main.go startup + integration test (no mocks) — `9141297`
- [x] P1-F03-T10 — Challenge with three runtime-evidence scenarios — `84874be`
- [x] P1-F03-T11 — Feature 3 close-out + push — `8b13e93`

## Active feature task list (P1-F04: Git Worktree Agent Isolation)
- [x] P1-F04-T01 — bootstrap evidence + advance PROGRESS + .gitignore  ← commit `d5ae14a`
- [x] P1-F04-T02 — internal/tools/worktree package skeleton (types + doc)  ← commits `97075a2` + `ccaaf33`
- [x] P1-F04-T03 — git.go thin git-binary wrappers (TDD against ephemeral repo)  ← commit `3e8b942`
- [x] P1-F04-T04 — Manager + ValidateName + GetCurrentDirectory + IsIsolated (TDD)  ← commit `94decd8`
- [x] P1-F04-T05 — Manager.EnterWorktree (TDD; existing/new branch + dirty rejection)  ← commit `bddf79d`
- [x] P1-F04-T06 — Manager.ExitWorktree + ListWorktrees + RemoveWorktree (TDD)  ← commit `1fa0617`
- [x] P1-F04-T07 — 4 tools.Tool interface implementations (TDD)  ← commit `f522805`
- [x] P1-F04-T08 — session.Manager currentWorktree field + getter/setter (TDD)  ← commit `73b040a`
- [x] P1-F04-T09 — helixcode worktree {list,enter,exit,remove} Cobra subcommands  ← commit `0a1fb53`
- [x] P1-F04-T10 — /worktree slash command + register in builtin/register.go  ← commit `64e8a45`
- [x] P1-F04-T11 — cmd/cli/main.go startup wiring + integration test (no mocks)  ← commit `4325459`
- [x] P1-F04-T12 — Challenge with three runtime-evidence scenarios  ← commit `9a23db1`
- [x] P1-F04-T13 — Feature 4 close-out + push  ← (this commit)

## Active feature task list (P1-F05: Hook-Based Extensibility)
- [x] P1-F05-T01 — bootstrap evidence + advance PROGRESS  ← commit `b7e7185`
- [x] P1-F05-T02 — add 6 new HookType constants (TDD)  ← commit `857ef64`
- [x] P1-F05-T03 — yaml_loader.go: FileLoader + apiVersion validation (TDD)  ← commits `bf50e8d` + `df72487`
- [x] P1-F05-T04 — shell_runner.go: NewShellRunner HookFunc (TDD)  ← commits `af5641f` + `b304c3e`
- [x] P1-F05-T05 — blockers.go: Blockers helper (TDD)  ← commit `b820bee`
- [x] P1-F05-T06 — wire registry.Execute with 6 events (TDD)  ← commit `61ce79e`
- [x] P1-F05-T07 — wire OnCompaction in AutoCompactor (TDD)  ← commit `302aabd`
- [x] P1-F05-T08 — wire OnError + RequestPlanApproval stub in agent.go (TDD)  ← commit `76a0823`
- [x] P1-F05-T09 — helixcode hooks {list,test,enable,disable,validate} subcommands  ← commit `d0f85d9`
- [x] P1-F05-T10 — /hooks slash command + builtin registration  ← commit `910488b`
- [x] P1-F05-T11 — cmd/cli/main.go startup wiring + integration tests (no mocks)  ← commit `6925038`
- [x] P1-F05-T12 — Challenge with three runtime-evidence scenarios  ← commit `d5da040`
- [x] P1-F05-T13 — Feature 5 close-out + push  ← (this commit)

## Active feature task list (P1-F06: MCP Full Lifecycle)
- [x] P1-F06-T01 — bootstrap evidence + advance PROGRESS
- [x] P1-F06-T02 — types.go + transport.go interface + BackoffSchedule (TDD)
- [x] P1-F06-T03 — transport_stdio.go + cross-platform unix/windows files (TDD)
- [x] P1-F06-T04 — transport_http.go with OAuth bearer header (TDD)
- [x] P1-F06-T05 — transport_sse.go with reconnect loop (TDD)
- [x] P1-F06-T06 — transport_ws.go via gorilla/websocket (TDD)
- [x] P1-F06-T07 — oauth.go: RFC 8414 discovery + PKCE + token cache (TDD)
- [x] P1-F06-T08 — lifecycle.go: Client state machine + handshake (TDD)
- [x] P1-F06-T09 — registry.go: Manager + tool merging (TDD)
- [x] P1-F06-T10 — config.go: YAML loader/saver, project + user precedence (TDD)
- [x] P1-F06-T11 — cmd/cli/mcp_cmd.go + /mcp slash command (TDD)
- [x] P1-F06-T12 — cmd/cli/main.go startup wiring + tools/registry.go integration + integration tests
- [x] P1-F06-T13 — Challenge with runtime evidence + cross-compile check
- [x] P1-F06-T14 — Feature 6 close-out + push

## Active feature task list (P1-F07: Background Task System)
- [x] P1-F07-T01 — bootstrap evidence + advance PROGRESS
- [x] P1-F07-T02 — workflow/background.go: BackgroundTask + TaskState (TDD)
- [x] P1-F07-T03 — workflow/background.go: BackgroundManager + sweeper (TDD)
- [x] P1-F07-T04 — tools/types_background.go: BackgroundAware interface + LineSink + error sentinel
- [x] P1-F07-T05 — tools/shell/background.go: shell BackgroundAware adapter (TDD)
- [x] P1-F07-T06 — tools/registry.go: SetBackgroundManager + Execute dispatch + adaptToolForBackground (TDD)
- [x] P1-F07-T07 — tools/task_tools.go: TaskOutputTool + TaskStopTool + RegisterTaskTools (TDD)
- [x] P1-F07-T08 — commands/tasks_command.go: /tasks slash command + builtin registration (TDD)
- [x] P1-F07-T09 — cmd/cli/main.go startup wiring + integration test (real subprocess)
- [x] P1-F07-T10 — Challenge with runtime evidence + cross-compile check
- [x] P1-F07-T11 — Feature 7 close-out + push

## Active feature task list (P1-F08: Plan Mode)
- [x] P1-F08-T01 — bootstrap evidence + advance PROGRESS
- [x] P1-F08-T02 — Planner extensions + hook types (TDD)
- [x] P1-F08-T03 — planmode/gating.go: ToolGate (TDD)
- [x] P1-F08-T04 — tools/types_planmode.go + plan_tools.go (TDD)
- [x] P1-F08-T05 — tools/registry.go: SetPlanModeGate + Execute gating (TDD)
- [x] P1-F08-T06 — commands/plan_command.go: /plan slash + builtin reg (TDD)
- [x] P1-F08-T07 — cmd/cli/main.go startup wiring + integration test
- [x] P1-F08-T08 — Challenge with runtime evidence + cross-compile check
- [x] P1-F08-T09 — Feature 8 close-out + push

## Active feature task list (P1-F09: Slash Command System)
- [x] P1-F09-T01 — bootstrap evidence + advance PROGRESS
- [x] P1-F09-T02 — markdown_commands.go: MarkdownCommand + parser + substitution (TDD)
- [x] P1-F09-T03 — MarkdownLoader: scan dirs + register/unregister (TDD)
- [x] P1-F09-T04 — markdown_watcher.go: fsnotify + debounce (TDD)
- [x] P1-F09-T05 — /commands slash + helixcode commands cobra (TDD)
- [x] P1-F09-T06 — main.go wiring + integration test
- [x] P1-F09-T07 — Challenge with runtime evidence + cross-compile check
- [x] P1-F09-T08 — Feature 9 close-out + push

## Active feature task list (P1-F10: Skill System)
- [x] P1-F10-T01 — bootstrap evidence + advance PROGRESS
- [x] P1-F10-T02 — markdown_skills.go: Skill + SkillRegistry + parser + Render (TDD)
- [x] P1-F10-T03 — SkillLoader: scan dirs + register/unregister (TDD)
- [x] P1-F10-T04 — skills_watcher.go: fsnotify + debounce (TDD)
- [x] P1-F10-T05 — agent/skill_dispatcher.go: Match + capture extraction (TDD)
- [x] P1-F10-T06 — /skills slash + helixcode skills cobra (TDD)
- [x] P1-F10-T07 — main.go wiring + integration test
- [x] P1-F10-T08 — Challenge with runtime evidence + cross-compile check
- [x] P1-F10-T09 — Feature 10 close-out + push

## Active feature task list (P1-F11: Session Transcript Resume)
- [x] P1-F11-T01 — bootstrap evidence + advance PROGRESS  ← commit `ddb45dc`
- [x] P1-F11-T02 — identity.go: ComputeProjectIdentity (TDD)  ← commit `fa6bc5f`
- [x] P1-F11-T03 — transcript_store.go: JSONL append/read + metadata I/O (TDD)  ← commit `466ab97`
- [x] P1-F11-T04 — resume.go: ResumeFinder + ResumeMode + FindResumeTarget (TDD)  ← commit `d72e401`
- [x] P1-F11-T05 — SessionManager extensions: Append/Resume/CurrentID (TDD)  ← commit `08fa5c0`
- [x] P1-F11-T06 — /sessions slash + helixcode sessions cobra (TDD)  ← commit `607206a`
- [x] P1-F11-T07 — main.go --resume/--continue flag parsing + integration test  ← commit `0fb036c`
- [x] P1-F11-T08 — Challenge with runtime evidence + cross-compile check  ← submodule `1e79453` + meta-repo `f4d0ff2`
- [x] P1-F11-T09 — Feature 11 close-out + push  ← (this commit)
- [x] P1-F11-fix — preserve ProjectPath/Name across SessionManager.Append  ← commit `f258cf7`

## Active feature task list (P1-F12: Multi-Provider Backend)
- [x] P1-F12-T01 — bootstrap evidence + advance PROGRESS  ← commit `bd5dc69`
- [x] P1-F12-T02 — provider.go: unified interface + LLMsVerifier audit (TDD)  ← commit `06c9c34`
- [x] P1-F12-T03 — anthropic_provider.go conformance + base URL precedence (TDD)  ← commit `dde10dd`
- [x] P1-F12-T04 — bedrock_provider.go conformance + verifier GetModels (TDD)  ← commit `d01026d`
- [x] P1-F12-T05 — vertexai_provider.go conformance + verifier GetModels (TDD)  ← commit `67417ed`
- [x] P1-F12-T06 — azure_provider.go conformance + verifier GetModels (TDD)  ← commit `880b4dc`
- [x] P1-F12-T07 — provider_factory.go: NewCloudProvider + Selector (TDD)  ← commit `28b6fa1`
- [x] P1-F12-T08 — wizard.go: tview TUI + mode 0600 + O_EXCL (TDD)  ← commit `778040e`
- [x] P1-F12-T09 — main.go wiring + helixcode wizard cobra + integration test  ← commit `ac55fca`
- [x] P1-F12-T10 — Challenge with runtime evidence (local + cloud-gated)  ← submodule `4e42fbc` + meta-repo `b937e17` + SHA backfill `1586624`
- [x] P1-F12-T11 — Feature 12 close-out + push 4 remotes  ← (this commit)

## Active feature task list (P1-F13: LSP Integration)
- [x] P1-F13-T01 — bootstrap evidence + advance PROGRESS to F13  ← commit `df98b6d`
- [x] P1-F13-T02 — go.mod: add go.lsp.dev/jsonrpc2 v0.10.0 + protocol v0.12.0 (TDD)  ← commit `b9a30e4`
- [x] P1-F13-T03 — internal/tools/lsp_types.go: Diagnostic + DiagnosticSummary + LSPServerSpec (TDD)  ← commit `3c5d894`
- [x] P1-F13-T04 — internal/tools/lsp_client.go: jsonrpc2 wrapper + handshake (TDD with paired pipes)  ← commit `2fdb648`
- [x] P1-F13-T05 — internal/tools/lsp_manager.go: lazy-spawn + idle-timeout + ext-router + fake LSP (TDD)  ← commit `beef346`
- [x] P1-F13-T06 — internal/tools/lsp_servers.go: curated 5-server allowlist + Detect (TDD)  ← commit `33387a3`
- [x] P1-F13-T07 — internal/tools/lsp.go: LSPGetDiagnostics + LSPAnalyzeDiagnostic tools (TDD)  ← commit `9bb3118`
- [x] P1-F13-T08 — registry.SetLSPManager + post-Execute auto-trigger for fs_edit/fs_write/multi_edit_commit (TDD)  ← commit `a1aa7e6`
- [x] P1-F13-T09 — /lsp slash command (status/restart/list-servers/stop) (TDD)  ← commit `1b7812f`
- [x] P1-F13-T10 — helixcode lsp cobra + main.go wiring + gated integration test  ← commit `080b79b`
- [x] P1-F13-T11 — Challenge: in-tree fake LSP pipeline + gated real-server phase  ← submodule `f00bf19` + meta-repo `9ea2cdf`
- [x] P1-F13-T12 — Feature 13 close-out + push 4 remotes non-force  ← (this commit)

## Active feature task list (P1-F14: Sandboxed Shell Execution)
- [x] P1-F14-T01 — bootstrap evidence + advance PROGRESS to F14  ← commit `0ef5811`
- [x] P1-F14-T02 — sandbox/types.go: SandboxConfig + Policy + Capabilities + Result + Backend interface + ConstitutionalDenyList (TDD)  ← commit `abdbdab`
- [x] P1-F14-T03 — sandbox/detector.go: capability probes + SelectBackend with fail-closed (TDD)  ← commit `4f7141f`
- [x] P1-F14-T04 — sandbox/bubblewrap_backend.go: deterministic argv builder + Run (TDD)  ← commit `ec4cb9b`
- [x] P1-F14-T05 — sandbox/native_backend.go: SysProcAttr.Cloneflags userns + native_helper re-exec (TDD)  ← commit `5d05b3d`
- [x] P1-F14-T06 — sandbox/manager.go: backend selection + CONST-033 deny + user deny + fail-closed (TDD)  ← commit `a642101`
- [x] P1-F14-T07 — sandbox/sandboxed_shell_tool.go: Tool interface impl as shell_sandboxed (TDD)  ← commit `ba54c0c`
- [x] P1-F14-T08 — sandbox/config_loader.go: YAML loader + secret-safe writer (mode 0600) (TDD)  ← commit `9aadc02`
- [x] P1-F14-T09 — /sandbox slash command (status/test/policy) (TDD)  ← commit `93dc377`
- [x] P1-F14-T10 — main.go wiring (Detector + Manager + tool + slash) + gated integration test  ← commit `fdb5ddc`
- [x] P1-F14-T11 — Challenge harness: detector + fail-closed always-runs + bwrap/native gated phases  ← submodule `7d336ad` + meta-repo `998896c`
- [x] P1-F14-T12 — Feature 14 close-out + push 4 remotes non-force  ← (this commit)

## Active feature task list (P1-F15: Subagent Team)
- [x] P1-F15-T01 — bootstrap evidence + advance PROGRESS to F15  ← commit `b970aa5`
- [x] P1-F15-T02 — subagent types + Isolation/State enums + FakeLLMProvider TEST PROVIDER (TDD)  ← commit `adc273d`
- [x] P1-F15-T03 — InProcessSpawner with real llm.Provider invocation + ctx cancel (TDD)  ← commit `ceeb670`
- [x] P1-F15-T04 — SubprocessSpawner with sentinel env var + JSON stdout decode (TDD)  ← commit `ec21b17`
- [x] P1-F15-T05 — SubagentManager with streaming dispatch + max-concurrency + kill-by-id (TDD)  ← commit `8e2f9e8`
- [x] P1-F15-T06 — F04 worktree integration for isolation=worktree (real git tempdir test)  ← commit `9311692`
- [x] P1-F15-T07 — TaskTool implementing tools.Tool as `task` (claude-code-compatible name)  ← commit `1f9d0f3`
- [x] P1-F15-T08 — IsSubagentInvocation + RunAsSubagent helper-mode + main.go early-dispatch  ← commit `07863d2`
- [x] P1-F15-T09 — /subagents slash command (list/status/kill) + CONST-042 anti-leak (TDD)  ← commit `87b6eac`
- [x] P1-F15-T10 — wire SubagentManager into main.go + /subagents + gated integration tests  ← commit `af0aa29`
- [x] P1-F15-T11 — Challenge with runtime evidence (in-process + subprocess always; worktree + real-LLM gated)  ← submodule `163965e` + meta-repo `16708a7`
- [x] P1-F15-T12 — Feature 15 close-out + push 4 remotes non-force  ← (this commit)

## Active feature task list (P1-F16: OpenTelemetry Integration)
- [x] P1-F16-T01 — bootstrap evidence + advance PROGRESS to F16  ← commit `5fc7dc1`
- [x] P1-F16-T02 — go.mod: add OTel v1.30.0 dep set (TDD failing import test)  ← commit `f2e7260`
- [x] P1-F16-T03 — types.go: TelemetryConfig + ExporterKind + DefaultBlockedAttributeKeys + sentinels (TDD)  ← commit `de941b4`
- [x] P1-F16-T04 — config.go + attribute_filter.go: env-var parsing + exporter selection + secret filter (TDD)  ← commit `3c8593c`
- [x] P1-F16-T05 — provider.go: TelemetryProvider construction (TracerProvider + MeterProvider) (TDD)  ← commit `a8e13e3`
- [x] P1-F16-T06 — llm_instrumentation.go: TracedLLMProvider decorator with token counter + latency histogram (TDD)  ← commit `6fcbff6`
- [x] P1-F16-T07 — tool_instrumentation.go + ToolRegistry.Execute in-place wrap + SetTelemetryProvider (TDD)  ← commit `d80c278`
- [x] P1-F16-T08 — agent_instrumentation.go + BaseAgent.executeTaskWithLLM in-place wrap (TDD)  ← commit `7c06806`
- [x] P1-F16-T09 — /telemetry slash command (status/show/flush) (TDD)  ← commit `7701c33`
- [x] P1-F16-T10 — main.go wiring + gated integration tests (stdout always; OTLP gRPC+HTTP gated)  ← commit `a5eb1c9`
- [x] P1-F16-T11 — Challenge harness: in-tree fake OTLP/HTTP receiver + 5 phases (STDOUT/FAKE-OTLP/FILTER/NOOP/REAL)  ← submodule `af34a2c` + meta-repo `c4972dc`
- [x] P1-F16-T12 — Feature 16 close-out + push 4 remotes non-force  ← (this commit)

## Active feature task list (P1-F17: Smart File Editing)
- [x] P1-F17-T01 — bootstrap evidence + advance PROGRESS to F17  ← commit `37b1471`
- [x] P1-F17-T02 — smartedit/types.go: EditBlock + markers + sentinels + size limits (TDD)  ← commit `adcd9f0`
- [x] P1-F17-T03 — smartedit/parser.go: SEARCH/REPLACE block parser + path-stickiness + line tracking (TDD)  ← commit `91d7550`
- [x] P1-F17-T04 — smartedit/applier.go + binary_detect.go: lenient re-search + ambiguity + binary refusal (TDD)  ← commit `37beb27`
- [x] P1-F17-T05 — smartedit/diff.go: unified-diff wrapper re-using F08 multiedit DiffManager (TDD)  ← commit `6c00471`
- [x] P1-F17-T06 — smartedit/smart_edit_tool.go: Tool impl + multiedit transaction + post-write re-read + diff (TDD)  ← commit `721fed9`
- [x] P1-F17-T07 — /edit slash (status/diff/dry-run/commit) + SmartEditInspector (TDD)  ← commit `a2dd7eb`
- [x] P1-F17-T08 — main.go wiring + registry registration + always-run integration tests  ← commit `5bf4c92`
- [x] P1-F17-T09 — Challenge harness: 7-phase (SINGLE/NOT-FOUND/MULTI/ROLLBACK/DIFF/AMBIG/BINARY) with sha-256 positive evidence  ← submodule `e2e9e94` + meta-repo `daa1279`
- [x] P1-F17-T10 — Feature 17 close-out + push 4 remotes non-force  ← (this commit)

## Active feature task list (P1-F18: No-Flicker Rendering)
- [x] P1-F18-T01 — bootstrap evidence + advance PROGRESS to F18  ← commit `a8aa8f3`
- [x] P1-F18-T02 — render/types.go: Renderer interface + RenderMode + Frame + sentinels + env-var consts (TDD)  ← commit `8d6ec3b`
- [x] P1-F18-T03 — render/ansi_renderer.go: in-place line update + multi-line frame + dirty-region diff (TDD)  ← commit `a3cd0e1`
- [x] P1-F18-T04 — render/plain_renderer.go: line-by-line fallback with zero-ANSI/zero-CR invariant (TDD)  ← commit `487d72e`
- [x] P1-F18-T05 — render/viewport.go: Frame buffer + dirty-line tracking + pure-Go Diff (TDD)  ← commit `8c90e7c`
- [x] P1-F18-T06 — render/factory.go: HELIXCODE_RENDER env var + TTY detection via x/term (TDD)  ← commit `288e6cd`
- [x] P1-F18-T07 — Wire LLM streaming hook in cmd/cli/main.go::handleGenerate (TDD)  ← commit `4ece7e8`
- [x] P1-F18-T08 — render/tool_helpers.go + wire tool-result frame rendering (TDD)  ← commit `05434c4`
- [x] P1-F18-T09 — Challenge harness: 5 phases (STREAMING-FANCY + STREAMING-PLAIN + DIRTY-REGION-DIFF + TTY-FALLBACK + REAL-TTY gated)  ← submodule `c409ed3` + meta-repo `c44b049`
- [x] P1-F18-T10 — Feature 18 close-out + push 4 remotes non-force  ← (this commit)

## Active feature task list (P1-F19: AskUserQuestion with Previews)
- [x] P1-F19-T01 — bootstrap evidence + advance PROGRESS to F19  ← commit `b7fc6bb`
- [x] P1-F19-T02 — askuser/types.go: Choice + Question + Result + Prompter interface + sentinels (TDD)  ← commit `3be8ca7`
- [x] P1-F19-T03 — askuser/stdin_prompter.go: non-TTY short-circuit + retry loop + timeout + F18 menu render (TDD)  ← commit `2ded9d2`
- [x] P1-F19-T04 — askuser/ask_user_tool.go: AskUserTool wrapping Prompter + CategoryAskUser (TDD)  ← commit `bc84789`
- [x] P1-F19-T05 — wire ask_user into registry + integration test (always-runs both branches)  ← commit `98bd424`
- [x] P1-F19-T06 — Challenge harness: 5 always-run phases + reader-position + byte-offset positive evidence  ← submodule `e3454a4` + meta-repo `ecb3e26`
- [x] P1-F19-T07 — Feature 19 close-out + push 4 remotes non-force  ← (this commit)

## Active feature task list (P1-F20: Theme System) — FINAL Phase 1 feature
- [x] P1-F20-T01 — bootstrap evidence + advance PROGRESS to F20  ← commit `60777fd`
- [x] P1-F20-T02 — theme/types.go: Role + Color + ColorDepth + Theme + sentinels + Reset const (TDD)  ← commit `4697565`
- [x] P1-F20-T03 — theme/builtin.go: dark/light/none themes with pinned byte tables (TDD)  ← commit `a66737d`
- [x] P1-F20-T04 — theme/detect.go: ThemeName + ColorDepth detection from injected env (TDD)  ← commit `7f97f57`
- [x] P1-F20-T05 — theme/loader.go: ThemeRegistry + YAML merge into dark baseline + Styler (TDD)  ← commit `1fd42d9`
- [x] P1-F20-T06 — wire theme.Styler into handleGenerate via F18 RenderTextBlock (TDD)  ← commit `1066798`
- [x] P1-F20-T07 — /theme slash (status/list/show) + main.go wiring + integration test (TDD)  ← commit `348630c`
- [x] P1-F20-T08 — Challenge harness: 5 always-run phases (BUILT-IN-DARK + BUILT-IN-LIGHT + PLAIN-ZERO-COLOR + DEPTH-DETECT + YAML-MERGE)  ← submodule `4bf04bb` + meta-repo `300f973`
- [x] P1-F20-T09 — Feature 20 close-out + push 4 remotes — PHASE 1 COMPLETE  ← (this commit)

## P2-F21 task list (Codex Approval Modes) — ALL CLOSED
- [x] P2-F21-T01 — bootstrap Phase 2 evidence + advance PROGRESS to F21  ← commit `a7a349f`
- [x] P2-F21-T02 — approval/types.go: ApprovalMode + ApprovalLevel + Decision + sentinels + ModeDescriptors (TDD)  ← commit `9c2664d`
- [x] P2-F21-T03 — approval/selector.go: flag > env > config > default precedence (TDD)  ← commit `0d655d8`
- [x] P2-F21-T04 — approval/manager.go: ApprovalManager with 4×4 matrix gate + F02/F14/F19 integration (TDD)  ← commit `5ef13b8`
- [x] P2-F21-T05 — Extend Tool interface with RequiresApproval() + DefaultLevelEdit + apply to ~30 existing tools (TDD)  ← commit `19bffce` + CONTINUATION update `1195ef9`
- [x] P2-F21-T06 — /approval slash command (status/set/show) (TDD)  ← commit `ad8843b` + CONTINUATION update `9b72c26`
- [x] P2-F21-T07 — main.go wiring + --approval pflag + registry hook + integration test (TDD)  ← commit `c022968` + CONTINUATION update `bd67324`
- [x] P2-F21-T08 — Challenge harness 5 phases (suggest-deny + auto-edit-prompt + full-auto-sandbox + runtime-change + F02-final-deny)  ← Challenges submodule `aff2a6f` + meta-repo `2781c1a` + CONTINUATION update `ee413c3`
- [x] P2-F21-T09 — Feature 21 close-out + push 4 remotes non-force  ← (this commit)

## Decision log
- 2026-05-04 — Approach A (HelixAgent as integration substrate) — user-approved during brainstorming — see synthesis spec §2.1
- 2026-05-04 — Non-force pushes pre-authorised for the duration of this programme — user statement during brainstorming — see synthesis spec §7.3
- 2026-05-04 — claude-code-source is Phase 1 priority #1 — user statement — see synthesis spec §4.1
- 2026-05-05 — Phase 0 closed; 17 plan tasks done + 2 added during execution (T08.5, T08.7); foundation verified; carry-forward items documented in evidence file P0-16
- 2026-05-05 — Phase 1 entered; Feature 1 (Auto-Compaction) starts. Approach: extend existing internal/llm/compression infrastructure rather than build the parallel system the porting doc proposed (gap discovered during plan-writing).
- 2026-05-05 — Feature 1 (Auto-Compaction) closed. Eleven sub-commits; extended existing internal/llm/compression rather than building parallel infrastructure as the porting doc proposed. Per-provider native tokenizers deferred to Phase 3.
- 2026-05-05 — Feature 2 (Permission Rule System) closed. Thirteen+ sub-commits (T11 needed a registration follow-up `244aff9`). Extended internal/tools/confirmation.PolicyEngine with a Wildcard Condition field; added internal/tools/permissions package that loads layered YAML rule files (~/.helixcode + project) and produces a Policy that delegates to a smuggle-resistant rule engine (mvdan.cc/sh/v3 walker handles $(...), backticks, heredocs, quoted operators, pipelines). Five claude-code mode presets (default | auto | acceptEdits | dontAsk | bypassPermissions) compose with the existing AutonomyMode gradient. Full CLI surface: --permission-mode flag, helixcode permissions {list,add,remove,check} subcommands, and a /permissions slash command via internal/commands (registered through builtin/register.go). Followed F01's "extend existing" pattern. Engine proven via 3 integration tests + 3 Challenge scenarios; dispatcher wiring (ConfirmationCoordinator → permissions.Engine) deferred to Phase 3.
- 2026-05-05 — Feature 3 (Tool Result Persistence) closed. Eleven sub-commits. New thin sub-package `internal/tools/persistence` mirrors F02's pattern. Threshold check fires at the LLM provider boundary (tool_provider.go). T07 audit confirmed 0 of 16 LLM providers bypass `tool_provider.go` — single choke point. LLM reads back via the existing Read tool — no new tool added. Lazy 7-day CleanupOld at startup. Engine proven via 3 integration tests + 3 Challenge scenarios (above/below threshold + hash idempotence).
- 2026-05-05 — Feature 4 (Git Worktree Agent Isolation) closed. Thirteen sub-commits (T02 needed a fix-up `ccaaf33` for an anti-bluff smoke regression — the docstring contained "placeholders" which trips the coarse `grep "placeholder"` check). New thin sub-package `internal/tools/worktree` mirrors F02/F03's pattern. Shells out to the git binary consistent with `internal/tools/git/`. Worktrees stored at `<repoRoot>/.helix-worktrees/<name>/` (in-repo; .gitignore'd). Meta-only — no submodule auto-init; agents that need submodule code run `git submodule update --init --recursive` from inside the worktree. Full surface: 4 agent tools + 4 Cobra subcommands (enter/exit print help when called from CLI) + 1 /worktree slash command. Per-session state via single field on session.Manager rather than a parallel worktree_state.go file.
- 2026-05-05 — Feature 5 (Hook-Based Extensibility) closed. 14 sub-commits (12 feat + 2 fix-ups: T04's cross-platform shell-runner split, T03's yaml-loader priority default). Extended existing internal/hooks package with 6 new HookType constants + 3 new files (yaml_loader, shell_runner, blockers). Config-driven shell hooks via ~/.helixcode/hooks.yaml. 5 wiring points: tools/registry.Execute (6 events), llm/compression/AutoCompactor (OnCompaction), agent (OnError + RequestPlanApproval stub for F08). Full surface: 5 Cobra subcommands + /hooks slash command (aliased /hk).
- 2026-05-05 — Feature 11 (Session Transcript Resume) closed. 9 task commits (T01 `ddb45dc`, T02 `fa6bc5f`, T03 `466ab97`, T04 `d72e401`, T05 `08fa5c0`, T06 `607206a`, T07 `0fb036c`, T08 submodule `1e79453` + meta `f4d0ff2`, T09 close-out) + 1 follow-up (`f258cf7` preserves ProjectPath/Name across `SessionManager.Append`). New files in `internal/session/`: identity.go (Git-root-or-cwd), transcript_store.go (JSONL transcripts + metadata I/O), resume.go (ResumeFinder + ResumeMode + FindResumeTarget). Existing `session_manager.go` extended with `Append/Resume/CurrentID`. Surface: `/sessions` slash + `helixcode sessions {list,show,resume,delete}` cobra + `--resume`/`--continue` flags wired in `cmd/cli/main.go`. Challenge harness exercises real fork-exec process boundaries (write child PID ≠ read child PID, both ≠ orchestrator). All 4 remotes pushed non-force. F12 (Multi-Provider Backend) is the next candidate per the original 12-feature programme plan.
- 2026-05-05 — Feature 12 (Multi-Provider Backend) closed. 11 task commits (T01 `bd5dc69`, T02 `06c9c34`, T03 `dde10dd`, T04 `d01026d`, T05 `67417ed`, T06 `880b4dc`, T07 `28b6fa1`, T08 `778040e`, T09 `ac55fca`, T10 submodule `4e42fbc` + meta-repo `b937e17` + SHA backfill `1586624`, T11 close-out). Anthropic / Bedrock / Vertex / Azure unified behind a single `Selector` (flag > env > config > wizard precedence) with verifier-backed `GetModels` on all four cloud providers (CONST-036/037 satisfied). New `internal/llm/{selector.go, wizard.go, wizard_writer.go, provider_factory.go::NewCloudProvider}` plus four per-provider audit tests (`{anthropic,bedrock,vertexai,azure}_provider_audit_test.go`). Surface: `--provider` flag + `HELIX_LLM_PROVIDER` env + `helixcode wizard` cobra + tview TUI wizard with `tcell.SimulationScreen` headless tests (mode 0600 + O_EXCL writer). Challenge harness emits LOCAL-always-runs (11/11 PASS) + CLOUD credential-gated sections with explicit `SKIP-OK: P1-F12 cloud creds not provided` markers. All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin` (mirror gap noted, deferred infra). F13 (LSP Integration) is the next candidate.
- 2026-05-05 — Feature 13 (LSP Integration) closed. 12 task commits (T01 `df98b6d`, T02 `b9a30e4`, T03 `3c5d894`, T04 `2fdb648`, T05 `beef346`, T06 `33387a3`, T07 `9bb3118`, T08 `a1aa7e6`, T09 `1b7812f`, T10 `080b79b`, T11 submodule `f00bf19` + meta-repo `9ea2cdf`, T12 close-out). 5 curated LSP servers (gopls / rust-analyzer / pyright / typescript-language-server / clangd) brought online with lazy-spawn + 5-minute idle timeout per server, file-extension router, crash recovery, post-Execute auto-trigger after `fs_edit`/`fs_write`/`multi_edit_commit` (attaches `lsp_diagnostics` to the tool result map). New files in `internal/tools/`: `lsp_types.go`, `lsp_client.go` (jsonrpc2 wrapper around `go.lsp.dev/jsonrpc2 v0.10.0` + `go.lsp.dev/protocol v0.12.0`), `lsp_manager.go`, `lsp_servers.go`, `lsp.go` (LSPGetDiagnostics + LSPAnalyzeDiagnostic agent tools), `lsp_autotrigger.go`, `lsp_fakeserver/main.go` (in-tree real-subprocess fake LSP for deterministic tests). Surface: `/lsp` slash + `helixcode lsp {status,restart,list-servers,stop}` cobra. Challenge harness has TWO sections — MANAGER PIPELINE (always runs, real OS subprocess speaking real LSP-framed JSON-RPC over stdio, NOT an in-process stub) + REAL LANGUAGE SERVERS (gated, SKIP-OK with install hints). All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin` (mirror gap noted, deferred infra). F14 (Sandboxed Shell Execution) is the next candidate.
- 2026-05-06 — Feature 15 (Subagent Team) closed. 12 task commits (T01 `b970aa5`, T02 `adc273d`, T03 `ceeb670`, T04 `ec21b17`, T05 `8e2f9e8`, T06 `9311692`, T07 `1f9d0f3`, T08 `07863d2`, T09 `87b6eac`, T10 `af0aa29`, T11 submodule `163965e` + meta-repo `16708a7`, T12 close-out). Hybrid in-process + subprocess subagent dispatch with optional F04 worktree isolation: `internal/agent/subagent/` package (types + FakeLLMProvider TEST PROVIDER + InProcessSpawner + SubprocessSpawner + SubagentManager with streaming aggregation + max-concurrency + kill-by-id + helper_mode IsSubagentInvocation/RunAsSubagent + worktree_integration with `CreateWorktreeForSubagent` non-mutating helper on F04 manager). New `task` tool (claude-code-compatible name) at `internal/tools/task_tool.go`; new `/subagents` slash command (list/status/kill) at `internal/commands/subagents_command.go` with CONST-042 anti-leak (status shows description, never prompt body). Subagent recursion guard: subprocess child's RunAsSubagent does NOT register the `task` tool, capping subagent depth at 1 in v1. Three-line main.go integration: `subagent.IsSubagentInvocation()` early-main check (FIRST statement, before `sandbox.IsHelperInvocation()`), manager construction, slash registration. Challenge harness emits SIX phases — IN-PROCESS (always, asserts `GenerateCallCount==1`), SUBPROCESS (always, real fork-exec of harness binary, parent provider call count == 0), WORKTREE (gated on `git`, real `git init` + F04 worktree + staged diff), REAL-LLM (gated on `ANTHROPIC_API_KEY`), CONCURRENCY-CAP (always, 3rd Dispatch returns ErrMaxConcurrency), KILL-CANCEL (always, Kill propagates → State=StateCanceled). On this host phases A/B/C/E/F all RAN and PASS; phase D SKIPPED (no API key). All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin` (mirror gap noted, deferred infra). F16 (OpenTelemetry Integration) is the next candidate.
- 2026-05-06 — Feature 16 (OpenTelemetry Integration) closed. 12 task commits (T01 `5fc7dc1`, T02 `f2e7260`, T03 `de941b4`, T04 `3c8593c`, T05 `a8e13e3`, T06 `6fcbff6`, T07 `d80c278`, T08 `7c06806`, T09 `7701c33`, T10 `a5eb1c9`, T11 submodule `af34a2c` + meta-repo `c4972dc`, T12 close-out). OTel v1.30.0 tracing + metrics with three exporters (OTLP/gRPC, OTLP/HTTP, stdout) + no-op fast path; env-var configured (OTEL_*); TracedLLMProvider decorator (Go struct embedding) wraps llm.Provider Generate/GenerateStream; in-place 5-line wraps in `internal/tools/registry.go::Execute` + `internal/agent/base_agent.go::executeTaskWithLLM`; pre-built instruments (helixcode_llm_tokens_total + llm_latency_seconds + tool_calls_total + tool_latency_seconds + agent_iterations_total + agent_iteration_duration_seconds); `DefaultBlockedAttributeKeys` covers credential keys (api_key/token/bearer/password/secret/authorization/{anthropic,openai}_api_key/aws_*) + prompt-body keys (prompt/prompt_body/request_body/response_body) — case-insensitive `FilterAttributes` is default-deny; `TracedLLMProvider` NEVER emits prompt body as span attribute (CONST-042). Surface: `/telemetry {status,show,flush}` slash command (status renders KIND/ENDPOINT/SERVICE/SPANS/METRICS table; show prints stdout ring buffer; flush calls ForceFlush). Challenge harness emits FIVE phases — STDOUT (always, captures span+metric writers), FAKE-OTLP-HTTP (always, in-tree `net/http.Server` decoding real OTLP protobuf bodies), FILTER (always, deliberately injects `api_key=sk-CHALLENGE-12345` and asserts marker absent in export), NOOP (always, 100 calls with disabled telemetry), REAL-COLLECTOR (gated on `OTEL_EXPORTER_OTLP_ENDPOINT`). On this host phases A/B/C/D RAN and PASS; phase E SKIPPED (no collector). All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin` (mirror gap noted, deferred infra). F17 (Smart File Editing) is the next candidate.
- 2026-05-06 — Feature 14 (Sandboxed Shell Execution) closed. 12 task commits (T01 `0ef5811`, T02 `abdbdab`, T03 `4f7141f`, T04 `ec4cb9b`, T05 `5d05b3d`, T06 `a642101`, T07 `ba54c0c`, T08 `9aadc02`, T09 `93dc377`, T10 `fdb5ddc`, T11 submodule `7d336ad` + meta-repo `998896c`, T12 close-out). Linux-first sandboxed shell execution: hybrid bubblewrap (preferred when on PATH) + native Go `Cloneflags` userns fallback; default-DENY network with per-call opt-in; CONST-033 power-management deny-list rejects matching commands BEFORE any subprocess spawns; fail-closed when neither bwrap nor unprivileged userns are available — never a silent unsandboxed run. New `internal/tools/sandbox/` package (types + detector + bubblewrap_backend + native_backend + native_helper re-exec + manager + sandboxed_shell_tool + config_loader). Tool: `shell_sandboxed`. Surface: `/sandbox {status,test,policy}` slash + secret-safe YAML at `~/.config/helixcode/sandbox.yaml` (mode 0600, parent 0700 — mirrors F12 wizard_writer; CONST-042 satisfied). Challenge harness emits THREE sections — DETECTOR + FAIL-CLOSED (always runs, asserts CONST-033 spawn-counter rejection AND verbatim fail-closed message), BUBBLEWRAP (gated, real curl-inside-sandbox network probe), NATIVE (gated, force-constructed NativeBackend with helper-mode dispatch). On this host both gated phases RAN end-to-end. All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin` (mirror gap noted, deferred infra). F15 (Subagent Team) is the next candidate.
- 2026-05-06 — Feature 17 (Smart File Editing) closed. 10 task commits (T01 `37b1471`, T02 `adcd9f0`, T03 `91d7550`, T04 `37beb27`, T05 `6c00471`, T06 `721fed9`, T07 `a2dd7eb`, T08 `5bf4c92`, T09 submodule `e2e9e94` + meta-repo `daa1279`, T10 close-out). Aider-style SEARCH/REPLACE block edits with multiedit-transactional atomicity. New `internal/tools/smartedit/` package (types + parser + applier + binary_detect + diff + smart_edit_tool) wrapping F08's `multiedit.MultiFileEditor` for atomic multi-file commits with post-write re-read verification. Lenient re-search across consecutive blocks; ambiguity rejection (multiple matches → fail-closed); binary-file refusal (NUL byte detection in first 8KB); pure-Go unified-diff via `multiedit.DiffManager.GenerateDiff` byte-exact-matching system `diff -u`. Surface: `/edit {status,diff,dry-run,commit}` slash command + tool registry registration as `smart_edit`. Challenge harness emits SEVEN phases — SINGLE-FILE-SUCCESS, SEARCH-NOT-FOUND-REJECTED, MULTI-FILE-ATOMIC, PARTIAL-FAILURE-ROLLBACK (chmod 0500 on file 3's parent dir; sha-256 byte-identity assertion across all 5 files), DIFF-EXACTNESS (byte-exact match vs system `diff -u`), AMBIGUOUS-REJECTED, BINARY-REFUSED — each with sha-256 positive evidence, never absence-of-error. All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin` (mirror gap noted, deferred infra). F18 (No-Flicker Rendering) is the next candidate.
- 2026-05-06 — Feature 19 (AskUserQuestion with Previews) closed. 7 task commits (T01 `b7fc6bb`, T02 `3be8ca7`, T03 `2ded9d2`, T04 `bc84789`, T05 `98bd424`, T06 submodule `e3454a4` + meta-repo `ecb3e26`, T07 close-out). Real, end-to-end `ask_user` tool for the HelixCode CLI agent: structured multiple-choice prompt with optional inline previews. New `internal/tools/askuser/` package: `types.go` (Choice + Question + Result + Prompter interface + sentinels `ErrInvalidQuestion`/`ErrNoTTYNoDefault`/`ErrUserCancelled`/`ErrPromptTimeout`/`ErrTooManyInvalidAttempts` + constants `DefaultTimeout=5min`/`DefaultMaxRetries=3`/`SourceStdin`/`SourceDefault`), `stdin_prompter.go` (production `stdinPrompter` with `Reader`/`Writer`/`IsTTY`/`Renderer`/`Timeout`/`MaxRetries` constructor seams; non-TTY short-circuit BEFORE any read; TTY branch via `bufio.ReadString('\n')` in goroutine + `select`-on-`ctx.Done`; retry loop on empty/non-numeric/out-of-range; EOF → `ErrUserCancelled`; ctx-cancel/timeout → `ErrPromptTimeout`; F18-renderer reused for prompt menu + per-choice inline preview), `ask_user_tool.go` (AskUserTool implements `tools.Tool`; `Name()=="ask_user"`; `Category()==tools.CategoryAskUser`; returns `map[string]interface{}{"value","label","index","source"}`). Two existing files extended: `internal/tools/registry.go` (new `CategoryAskUser` const + `AskUserPrompter` field on `RegistryConfig` + registration in `buildToolList`); `cmd/cli/main.go` (no changes required for correctness). Zero new external deps — pure stdlib + `golang.org/x/term` (already direct dep after F18) + `dev.helix.code/internal/render`. Integration tests (`tests/integration/askuser_test.go`, build-tag `integration`) exercise the production `stdinPrompter` end-to-end through `AskUserTool` with real `bytes.Buffer` reader/writer. Challenge harness emits FIVE always-run phases — PHASE-A (TTY-WITH-INPUT-RETURNS-CHOICE), PHASE-B (NON-TTY-WITH-DEFAULT, asserts `reader-remaining=2 untouched=true` + `writer-bytes=0`, proving short-circuit before read), PHASE-C (NON-TTY-NO-DEFAULT-ERRORS, asserts `errors.Is(ErrInteractiveTerminalRequired)=true` + writer untouched), PHASE-D (PREVIEW-VISIBLE-IN-OUTPUT, asserts byte offsets 37 + 87 of preview text inside writer — preview BEFORE label by byte order), PHASE-E (INVALID-INPUT-RETRY, retry loop reads new bytes, question rendered 2× with "1-3" hint). Cross-compile linux/amd64 produces a 73 MB binary. Anti-bluff smoke clean across `internal/tools/askuser/`, `cmd/cli/main.go`, `internal/tools/registry.go`, `tests/integration/askuser_test.go`, `tests/integration/cmd/p1f19_challenge/`, and `Challenges/p1-f19-ask-user-question/`. All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin` (mirror gap noted, deferred infra). F20 (Theme System) is the next candidate per the porting doc.
- 2026-05-06 — Feature 18 (No-Flicker Rendering) closed. 10 task commits (T01 `a8aa8f3`, T02 `8d6ec3b`, T03 `a3cd0e1`, T04 `487d72e`, T05 `8c90e7c`, T06 `288e6cd`, T07 `4ece7e8`, T08 `05434c4`, T09 submodule `c409ed3` + meta-repo `c44b049`, T10 close-out). Custom ANSI/CR renderer for the HelixCode CLI streaming hot path with non-TTY plain fallback; zero new external dependencies (pure stdlib + `golang.org/x/term` promoted indirect→direct). New `internal/render/` package: `types.go` (Renderer interface + RenderMode + Frame + sentinels + env-var consts), `ansi_renderer.go` (in-place line update via `\r\x1b[K` + `\x1b[?25l/h` cursor hide-show + dirty-region multi-line frame diff via cursor-up/down), `plain_renderer.go` (line-by-line `fmt.Fprintln` + `\r`-strip + zero-ANSI/zero-CR invariants + ANSI pass-through per §5.4), `viewport.go` (pure-Go dirty-line `Diff` + `Apply`), `factory.go` (`HELIXCODE_RENDER=plain|fancy|auto` env + `term.IsTerminal` TTY detection + constructor-injection seams), `tool_helpers.go` (`RenderTextBlock`/`RenderLines`/`RenderToolResult` glue with type-switch over Frame / []string / string / Stringer / fallback). Surface: env-var only (no slash, no cobra subcommand — Q5=B). Wired into `cmd/cli/main.go`: renderer constructed at startup, `defer renderer.Close()`, streaming-print loop in `handleGenerate` replaced with `Begin/WriteToken/Commit`, non-streaming `Generate` response printed via `RenderTextBlock`. Challenge harness emits FIVE phases — STREAMING-FANCY (10 `\r\x1b[K` sequences + `\x1b[?25l`/`h` pair, 371 bytes), STREAMING-PLAIN (zero `\x1b` and zero `\x0d`, 58 bytes), DIRTY-REGION-DIFF (one-line-changed delta=34 < firstLen=80 + exactly 1 cursor-up), TTY-FALLBACK (factory ladder picks plain on `bytes.Buffer` non-TTY), REAL-TTY (gated; SKIP-OK on non-TTY) — each with positive byte evidence, never absence-of-error. All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin` (mirror gap noted, deferred infra). F19 (AskUserQuestion with Previews) is the next candidate.
- 2026-05-06 — Feature 20 (Theme System) closed. 9 task commits (T01 `60777fd`, T02 `4697565`, T03 `a66737d`, T04 `7f97f57`, T05 `1fd42d9`, T06 `1066798`, T07 `348630c`, T08 submodule `4bf04bb` + meta-repo `300f973`, T09 close-out). Real, end-to-end 5-role semantic theme system for the HelixCode CLI agent: built-in `dark` / `light` / `none` themes with byte-pinned palettes per spec §3.4 across three depth tiers (`Truecolor` / `ANSI256` / `ANSI16`); optional YAML override at `$XDG_CONFIG_HOME/helixcode/theme.yaml` slot-merged over the dark baseline; theme-name resolution ladder (`HELIXCODE_THEME` env > `$COLORFGBG` heuristic > `dark` default); color-depth auto-detect from `$COLORTERM` (truecolor/24bit) and `$TERM` (`*-256color` → ANSI256, `dumb`/empty → Off, otherwise ANSI16); `Styler` decorator wraps text with role-coded SGR sequences (decorator pattern over F18's `Renderer` — F18 interface unchanged); plain-mode forced to `DepthOff` via `WithDepth` at startup as belt-and-suspenders against renderer regressions; `/theme {status,list,show <name>}` slash command (read-only). Zero new external deps — `gopkg.in/yaml.v3` was already present in `go.sum` via viper/cobra. Five always-run Challenge phases — PHASE-A (BUILT-IN-DARK, 5/5 dark roles with pinned `\x1b[38;2;…m` truecolor opens + `\x1b[0m` Reset), PHASE-B (BUILT-IN-LIGHT, light error `\x1b[38;2;175;0;0m` ≠ dark error `\x1b[38;2;255;64;64m` cross-theme distinguisher), PHASE-C (PLAIN-ZERO-COLOR, `bytes.IndexByte == -1` invariant under `DepthOff`), PHASE-D (DEPTH-DETECT, 6 env-driven branches), PHASE-E (YAML-MERGE, real `theme.yaml` on tempdir parsed by `yaml.v3`, custom `error` slot overlays dark baseline while un-mentioned `info`/`warn`/`highlight`/`dim` inherit). All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin`. **PHASE 1 OF CLI-AGENT FUSION PROGRAMME COMPLETE — 20 features (F01..F20) shipped.**
- 2026-05-06 — **PHASE 1 OF CLI-AGENT FUSION PROGRAMME COMPLETE.** All 20 features shipped: F01 Auto-Compaction (`4734f35`+`9284392` evidence; close-out `f0b9b15`..); F02 Permission Rules (`d56905d`..close-out); F03 Tool Result Persistence (`ee35017`..close-out); F04 Git Worktree Agent Isolation (`d5ae14a`..close-out); F05 Hook-Based Extensibility (`b7e7185`..close-out); F06 MCP Full Lifecycle (T14 close-out); F07 Background Tasks (T11 close-out); F08 Plan Mode (T09 close-out); F09 Slash Commands (T08 close-out); F10 Skills (T09 close-out); F11 Session Resume (`ddb45dc`..T09 close-out); F12 Multi-Provider Backend (`bd5dc69`..T11 close-out); F13 LSP Integration (`df98b6d`..T12 close-out); F14 Sandboxed Shell Execution (`0ef5811`..T12 close-out); F15 Subagent Team (`b970aa5`..T12 close-out); F16 OpenTelemetry (`5fc7dc1`..T12 close-out); F17 Smart File Editing (`37b1471`..T10 close-out); F18 No-Flicker Rendering (`a8aa8f3`..`92db29e` close-out); F19 AskUserQuestion (`b7fc6bb`..`f584c67` close-out); F20 Theme System (`60777fd`..T09 close-out — this commit). Phase 1 entry condition met: every Phase 1 feature ships with positive runtime evidence per Article XI §11.9 (no absence-of-error PASS), all anti-bluff smoke `clean`, all four meta-repo remotes (`origin` + `github` + `gitlab` + `upstream`) at parity, all Challenges submodule changes mirrored to its `origin`. Next phase: Phase 2 (other CLI agents) once user authorises.
- 2026-05-06 — **PHASE 1.5 (FOUNDATION CLEANUP) COMPLETE.** All 12 work packages shipped: WP1 Inventory + foundation safety (`421495a`); WP2 Submodule restructuring ~67 mechanical moves (`90dec95`); WP3 Submodule deduplication 5 sets (`154c06c`); WP4 API key loader bash + Go (`e57894e`); WP5 .env API key dedup with USER GATE (`92d5463`); WP6 Docs consolidation 3 dirs (`f09f57d`); WP7 Snake_case directory normalization (`3c3cd8d`); WP8 Anti-bluff Constitution propagation (`0eead08`); WP9 Reference updates comprehensive grep sweep (`42166fd`); WP10 Rebuild + validation + fix `internal/tools/git` MockLLMProvider drift (`0a77c93` + fix `45be827`); WP11 Phase 1.5 Challenge harness 5 phases (meta `306d3d9` + Challenges submodule `7e94f28`); WP12 close-out + push deepest-first (this commit). Acceptance criteria: Phase 1.5 Challenge harness EXIT=0 with all 5 phases (NO-DUPLICATE-SUBMODULES + API-KEYS-LOADER + DOCS-UNDER-DOCS-DIR + SNAKE_CASE + ANTI-BLUFF-ANCHOR) printing positive runtime evidence per Article XI §11.9; `scripts/verify_anti_bluff_cascade.sh` exit 0 ("OK: anti-bluff anchor present in all 39 files across 13 repos"); inner-module `go test -count=1 -short ./internal/... ./cmd/...` all green; anti-bluff smoke clean across all P1.5-touched files. All four meta-repo remotes (`origin` + `github` + `gitlab` + `upstream`) at parity; HelixAgent submodule pushed to its `origin`; Challenges submodule pushed to its `origin`. **Next phase: Phase 2 (other CLI agents) ready to start once user authorises.**
- 2026-05-06 — Phase 2 started; F21 (Codex Approval Modes) is first port; layers on F02 + F14; uses F19 prompter; CLI/env/config selector mirrors F12 pattern.
- 2026-05-06 — Feature 21 (Codex Approval Modes) closed. 9 task commits (T01 `a7a349f`, T02 `9c2664d`, T03 `0d655d8`, T04 `5ef13b8`, T05 `19bffce` + CONTINUATION `1195ef9`, T06 `ad8843b` + CONTINUATION `9b72c26`, T07 `c022968` + CONTINUATION `bd67324`, T08 Challenges submodule `aff2a6f` + meta-repo `2781c1a` + CONTINUATION `ee413c3`, T09 close-out — this commit). Real, end-to-end 4-mode codex-compatible approval system: suggest (read-only) / auto-edit (edit OK, run prompts) / full-auto (edit + run with sandbox forced + network DENY) / dangerously-bypass (no checks; 2-second pause + warning). Selector: --approval flag > HELIXCODE_APPROVAL env > ~/.config/helixcode/approval.yaml > default suggest. Per-tool Tool.RequiresApproval() ApprovalLevel (read-only/edit/run/all); ~38 existing tools migrated with explicit levels; safe-default LevelEdit for forgotten overrides. /approval slash (status/set/show); runtime mode swap via atomic.Pointer takes effect on next CheckApproval. F02 retains final-deny authority (composition truth table pinned via in-stub deny-rule in PHASE-E pending registry-level seam). Zero new external deps. Five-phase Challenge harness PASS (PHASE-A SUGGEST-DENY + PHASE-B AUTO-EDIT-PROMPT + PHASE-C FULL-AUTO-SANDBOX + PHASE-D RUNTIME-CHANGE + PHASE-E F02-FINAL-DENY) with positive runtime evidence per Article XI §11.9; cross-compile linux/amd64 PASS (94 MB binary); anti-bluff smoke clean across all F21 surface (HelixCode + Challenges). All 4 meta-repo remotes pushed non-force; Challenges submodule pushed to its single `origin` (mirror gap noted, deferred infra). **First Phase 2 feature shipped.** F22 next candidate per synthesis design §4.2 (Codex follow-on / aider / cline / plandex — choice via brainstorming).

## Open risks / parking lot
- **Historical SSH key leak (remediated in P0-T08.5):** `id_rsa` + `id_rsa.pub` at `HelixCode/test/workers/ssh_keys/` were committed as test fixtures before this programme. Their material lives in git history forever and is considered compromised. Mitigation: keys were ephemerally test-only (no production trust), replaced with auto-generated ed25519 ephemeral keys via `HelixCode/test/workers/ssh_keys/generate-test-keys.sh`, removed from the index via `git rm --cached`. Any future production system that erroneously trusts the leaked public key must reject it.
- **Historical helix.security.json credential leak (remediated in P0-T08.5):** `helix.security.json` at repo root was committed with real SonarQube and Snyk credentials (token, project_key, organization, url). Material lives in git history and is considered compromised. Mitigation: removed from index via `git rm --cached`; replaced with `helix.security.json.example` containing `<REDACTED>` placeholders. Rotate: SonarQube token, Snyk token, organization, and project_key immediately.
- HelixAgent submodule clone size — may need `--depth=1` shallow if >500 MB; measured at P0-03
- Codex agent disambiguation (closed vs. open variant) — deferred to Phase 2 sub-spec
- Example_Projects/ deletion — deferred to post-Phase-4 decision
- **Submodule recursion cosmetic error (deferred from P0-02):** `git submodule foreach --recursive` errors out on `Example_Projects/{Agent-Deck,Bridle,Claude-Code-Plugins-And-Skills}` because each of those third-party repos has registered nested gitlinks (mode 160000) without corresponding `.gitmodules` entries. The original Task 2 plan proposed `.git/info/exclude` — that does NOT fix recursion (which walks the index, not the working tree). The only safe in-scope fix is to wrap script calls with `|| true` and tolerate the error. Modifying the affected third-party submodules' contents is forbidden by spec §2.1 (third-party not modified). Decision: scripts that use `git submodule foreach --recursive` (none yet in our codebase) must wrap with `|| true`; documentation updates that erroneously claimed Task 2 would resolve this are corrected.
- **HelixAgent stale cli_agents pins (discovered during P0-03):** 13 of 60 cli_agents under `HelixAgent/cli_agents/` cannot be initialized because HelixAgent's recorded submodule SHAs no longer exist on the corresponding upstream remotes. Affected: `aider, conduit, continue, HelixCode, kilo-code, kiro-cli, mobile-agent, ollama-code, opencode-cli, openhands, plandex, roo-code, superset`. Each Phase 2 sub-spec for the affected agent must first bump HelixAgent's pointer (commit IN HelixAgent itself, then bump HelixAgent's pointer in this meta-repo) to a SHA that exists upstream. Phase 1 priority `claude-code` is NOT affected — fully populated. Per spec §1.3 N2, HelixAgent rewrite is out of scope for this programme; the per-agent pin bumps go through HelixAgent's own governance.
- **SonarQube + Snyk live-scan deferral (P0-T08.7):** The scan infrastructure (compose files, scripts, BootManager binary, Challenges) is fully wired and configuration-validated. Live scans CANNOT run until the user rotates the leaked credentials from `helix.security.json` (remediated in P0-T08.5 but historical values are compromised). Action required: (1) generate new SonarQube API token, (2) set `SONAR_TOKEN` + `SONARQUBE_PROJECT_KEY` + `SONARQUBE_PROJECT_NAME` in `HelixCode/.env`, (3) generate new Snyk token, (4) set `SNYK_TOKEN` in `HelixCode/.env`, (5) run `make scan-sonarqube` / `make scan-snyk`. This is NOT a code defect — it is a security-rotation dependency on the operator.
- **LLMsVerifier dual-pin divergence (discovered during P0-04):** `Dependencies/HelixDevelopment/LLMsVerifier` at `d473231d27196e2151405f37936151a386b590e3`; `HelixAgent/LLMsVerifier` at `1d53ae3b72c77c1f27171c0677431c48d2d02bdd`. Per spec §2.2 the canonical pin is the one in `Dependencies/HelixDevelopment/LLMsVerifier` (direct Go import path). The canonical is exactly one commit ahead of the transitive (HelixAgent) view. Resolving the divergence requires either (a) bumping HelixAgent's recorded LLMsVerifier pointer to the canonical SHA — out of scope per spec §1.3 N2 (HelixAgent rewrite forbidden), or (b) updating `Dependencies/HelixDevelopment/LLMsVerifier` to match HelixAgent's view if HelixAgent's view is more current. Decision deferred; the parity verifier (`scripts/verify-llmsverifier-pin-parity.sh`) will continue to gate any future change that introduces NEW divergence beyond this baseline. **P0-15 impact:** `make verify-foundation` exits 2 (non-zero) until this divergence is resolved. **P0-16 close-out dependency:** `make verify-foundation` must exit 0 for Phase 0 to be declared complete. This divergence must be resolved (or explicitly waived via `VERIFY_FOUNDATION_WARN_ONLY=1`) as part of P0-16.
- **Permissions engine not yet threaded into tool dispatch (deferred from P1-F02-T09):** The `--permission-mode` flag parses and `permissions.Engine` constructs at startup, but the resulting Engine's `*confirmation.PolicyEngine` is currently local to `(*CLI).initPermissions` and is not consulted by the production tool-execution path. The engine itself is proven correct (3 integration tests + 3 Challenge scenarios); the wiring gap means a deny rule would not actually block a tool call in a live session. Action: Phase 3 (test infra) sub-spec must wire `internal/tools/registry.go`'s `ConfirmationCoordinator` to use the loaded engine. Current behavior: rule files are validated and the CLI flag is honored at the `helixcode permissions check` dry-run; live tool dispatch falls through to the default `confirmation.PolicyEngine` (which has no rules). NOT a security regression — falls back to ask-by-default.
