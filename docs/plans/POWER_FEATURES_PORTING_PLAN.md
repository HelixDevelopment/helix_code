# POWER FEATURES PORTING PLAN — CLI-Agent Power Features → HelixCode

| Field         | Value                                                                                  |
|---------------|----------------------------------------------------------------------------------------|
| Revision      | 1                                                                                      |
| Created       | 2026-06-14                                                                             |
| Last modified | 2026-06-14                                                                             |
| Status        | DRAFT — research + plan only, NO code (operator decision pending)                      |
| Authority     | §11.4.70 subagent-driven, §11.4.74 catalogue-first, §11.4.6 no-guessing, §11.4.50/§11.4.98 anti-bluff |
| Method        | Read-only research of `cli_agents/<agent>/` READMEs+docs; cross-checked against `helix_code/internal/*` + `helix_code/applications/*` |
| Scope         | Map distinctive power features of every in-repo CLI agent for porting into HelixCode   |

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Method & Evidence Discipline](#2-method--evidence-discipline)
3. [What HelixCode Already Has (baseline)](#3-what-helixcode-already-has-baseline)
4. [Agents Surveyed](#4-agents-surveyed)
5. [Master Feature Matrix](#5-master-feature-matrix)
6. [Top-10 Highest-Value Missing Features](#6-top-10-highest-value-missing-features)
7. [Per-Feature Acceptance Bar (anti-bluff)](#7-per-feature-acceptance-bar-anti-bluff)
8. [Phases → Tasks → Sub-Tasks](#8-phases--tasks--sub-tasks)
   - [Phase 0 — Foundations & Audit](#phase-0--foundations--audit)
   - [Phase 1 — High-Value / Low-Effort (already-partial)](#phase-1--high-value--low-effort-already-partial)
   - [Phase 2 — Session & Context Power Tools](#phase-2--session--context-power-tools)
   - [Phase 3 — Autonomy & Safety Surface](#phase-3--autonomy--safety-surface)
   - [Phase 4 — Multi-Agent & Parallel Orchestration](#phase-4--multi-agent--parallel-orchestration)
   - [Phase 5 — Connectors, Server & Headless Surface](#phase-5--connectors-server--headless-surface)
   - [Phase 6 — Spec-Driven & Quality-Gate Workflow](#phase-6--spec-driven--quality-gate-workflow)
   - [Phase 7 — Long-Tail / Niche Features](#phase-7--long-tail--niche-features)
9. [Dependency Touch-Map](#9-dependency-touch-map)
10. [Honest Effort Caveats](#10-honest-effort-caveats)
11. [Sources Verified](#11-sources-verified)

---

## 1. Executive Summary

- **Agents enumerated under `cli_agents/`:** 51 directories. Of these, **~38 are coding/agent tools** worth mining; the remainder are MCP servers (`git-mcp`, `postgres-mcp`, `conduit`, `noi`, `snow-cli`, `x-cmd`, `cli-agent`, `aiagent`, `superset`, `ui-ux-pro-max`, `fauxpilot`, `deepseek-cli*`, `xela-cli`, `codex-skills`) or a skills/plugin marketplace (`bridle` = "Tons of Skills") rather than full agents.
- **Distinctive power features mapped:** **~140** across the 24 agents researched in depth (aider, cline, plandex, crush, codex, gemini-cli, qwen-code, copilot-cli, amazon-q, mistral-code, codename-goose, gptme, open-interpreter, forge, gpt-engineer, swe-agent, shai, nanocoder, claude-squad, claude-code, vtcode, warp, get-shit-done, spec-kit, junie, aichat, codai, taskweaver, octogen, zeroshot, multiagent-coding, agent-deck). After de-duplicating equivalent features across agents, they collapse into **~62 distinct capabilities**.
- **HelixCode is already mature.** It HAS: agentic tool loop (`internal/agent`), read-only tools + browser suite (`internal/tools`), MCP client+lifecycle (`internal/mcp`), plugins (`internal/plugins`), skills (via plugins + crush/vtcode-style `SKILL.md` precedent), LSP-equipped context, ensemble + verifier (`internal/verifier`), repomap (`internal/repomap`, tree-sitter), sessions + history-condense (`internal/session`), hooks (`internal/hooks`), sandbox + worktrees (`internal/worker`, `internal/session/manager_worktree`), memory + cognee (`internal/memory`), voice-to-code (`internal/voice`, the aider P2-F27 port), clarification (`internal/clarification`), task checkpoints (`internal/task/checkpoint.go`), roocode delegation (`internal/roocode`), kilocode callgraph/impact (`internal/kilocode`), planner/plantree (`internal/planner`, `internal/plantree`), approval/approvalwire, autocommit, git_status, multi-platform apps (desktop/terminal-ui/ios/android/aurora/harmony).
- **Net result:** Most "power features" are **HAVE** or **PARTIAL** — the real porting work is in **autonomy presets, cumulative-diff sandbox review, conversation branching, messaging connectors, daemon/server-shared-session mode, spec-driven workflow surface, and tangent/rewind UX**. Few features are genuinely **MISSING/large**.
- **Phasing:** 8 phases (0–7), ordered high-value-low-effort first, ending in niche features. Every feature ships with the operator's full anti-bluff bar (§7).

---

## 2. Method & Evidence Discipline

Per §11.4.6 (no-guessing) and §11.4.74 (catalogue-first), every feature row cites the source-agent file where the capability is textually evidenced. HelixCode status was cross-checked by reading `helix_code/internal/<pkg>/doc.go`, function signatures, and the existing port plan `docs/plans/HXC-031-codex-cline-port.md`. No source was modified. Where the in-repo doc only links out to a vendor website (codex `docs/*.md` stubs, mistral-code's inherited gemini docs), that is noted so we do not over-claim a feature exists in-tree.

**Status legend:** HAVE (shipped in HelixCode) · PARTIAL (substrate exists, surface/UX gap) · MISSING.
**Effort legend:** S (small, ≤~300-line unit) · M (medium, 1–3 units) · L (large, multi-component).

---

## 3. What HelixCode Already Has (baseline)

Verified by reading the inner module (`helix_code/internal`, `helix_code/applications`):

| Capability | HelixCode location | Evidence |
|---|---|---|
| Multi-agent orchestration + agent types | `internal/agent` | `agent/doc.go` (Coordinator, WorkflowExecutor, planning/coding/testing/debug/review agents) |
| Repo-map (tree-sitter, ranked, incremental) | `internal/repomap` | `repomap/file_ranker.go`, `cache.go`, `incremental_p2t06_test.go` |
| MCP client + lifecycle + chaos-tested | `internal/mcp` | `mcp/lifecycle.go`, `config.go`, `mcp_chaos_test.go` |
| Plugins (loader, activation, registry, exec) | `internal/plugins` | `plugins/loader.go`, `activation.go`, `registry.go` |
| Ensemble + verifier (LLMsVerifier single source) | `internal/verifier`, `internal/llm` | `verifier/client.go`, `adapter.go`; ensemble commits in git log |
| Sessions + history compaction/condense | `internal/session` | `session/condense.go` (`CompactIfNeeded`, `ShouldCompact`), `manager_worktree.go` |
| Hooks (lifecycle blockers/executor) | `internal/hooks` | `hooks/hook.go`, `blockers.go`, `executor.go` |
| Sandbox + worktree isolation | `internal/worker`, `internal/session` | `worker/isolation_test.go`, `session/manager_worktree.go` |
| Memory + cognee + project memory | `internal/memory`, `internal/projectmemory` | `memory/manager.go`, `cognee_integration.go`, `projectmemory/loader.go` |
| Voice-to-code (aider port P2-F27) | `internal/voice` | `voice/doc.go` (Whisper API + whisper.cpp fallback) |
| Clarification (LLM-generated questions) | `internal/clarification` | `clarification/engine.go` (`DetectAmbiguity`, `Resolve`) |
| Task checkpoints (DB-backed) | `internal/task` | `task/checkpoint.go` (`CreateCheckpoint`/`GetLatestCheckpoint`/restore) |
| Auto-commit + secret-filter + summariser | `internal/autocommit` | `autocommit/committer.go`, `secret_filter.go`, `summariser.go` |
| Roo-code delegation, Kilo-code callgraph/impact | `internal/roocode`, `internal/kilocode` | `roocode/delegator.go`, `kilocode/callgraph.go`, `impact.go` |
| Planner / plan-tree (compact, operations) | `internal/planner`, `internal/plantree` | `planner/executor.go`, `plantree/operations.go` |
| Approval engine + yes/no prompter | `internal/approval`, `internal/approvalwire` | `approval/manager.go`, `approvalwire/yesno_prompter.go` |
| Browser tools (chromedp, click/type/screenshot) | `internal/tools` | `tools/browser_*_v2.go` |
| Quality gate + scorer | `internal/quality` | `quality/gate.go`, `scorer.go` |
| Multi-platform apps + TUI | `applications/*` | `applications/{desktop,terminal_ui,ios,android,aurora_os,harmony_os}` |
| Multimodal (in-flight per HXC-031) | `internal/llm/vision`, HXC-031 plan | `docs/plans/HXC-031-codex-cline-port.md` |

This baseline means the plan is **mostly closing UX/surface gaps over existing substrate**, not building from zero.

---

## 4. Agents Surveyed

**Full coding agents mined (24):** aider, cline, plandex, crush, codex, gemini-cli, qwen-code, copilot-cli, amazon-q, mistral-code, codename-goose, gptme, open-interpreter, forge, gpt-engineer, swe-agent, shai, nanocoder, claude-code, vtcode, aichat, codai, taskweaver, octogen.

**Orchestration / multi-agent managers (6):** claude-squad, agent-deck, zeroshot, multiagent-coding, warp, get-shit-done.

**Workflow / spec toolkits (2):** spec-kit, junie.

**Not full agents (noted, low-priority mining):** `bridle` (skills/plugin marketplace), `git-mcp`/`postgres-mcp`/`conduit`/`noi`/`snow-cli`/`x-cmd`/`cli-agent`/`aiagent`/`superset`/`ui-ux-pro-max`/`fauxpilot`/`deepseek-cli*`/`xela-cli`/`codex-skills` (MCP servers / dashboards / stubs).

**Confirmed ABSENT from `cli_agents/`:** `openhands` (operator example) is NOT present — do not plan against it.

---

## 5. Master Feature Matrix

`feature` × `representative source-agent(s)` × `HelixCode status` × `effort` × `priority` (1=highest). Evidence column cites the canonical source file.

| # | Feature | Source agent(s) | Evidence (cli_agents/…) | HelixCode status | Effort | Prio |
|---|---|---|---|---|---|---|
| F01 | Repo-map (tree-sitter project map) | aider, plandex, codai | `aider/aider/website/docs/repomap.md`; `plandex/README.md`; `codai/README.md` | HAVE (`internal/repomap`) | — | — |
| F02 | Git auto-commit w/ generated message | aider, plandex, forge | `aider/README.md`; `plandex/docs/.../version-control.md`; `forge/README.md` (`:commit`) | HAVE (`internal/autocommit`) | — | — |
| F03 | Architect/editor dual-model mode | aider | `aider/aider/website/docs/usage/modes.md` | PARTIAL (ensemble + roles exist; no explicit architect→editor handoff) | M | 2 |
| F04 | Chat modes (code/ask/architect/plan) | aider, plandex, gemini-cli, forge | `aider/.../modes.md`; `plandex/.../repl.md`; `gemini-cli/docs/cli/plan-mode.md`; `forge` (sage/muse) | PARTIAL (planner + read-only tools; no first-class mode switch) | M | 1 |
| F05 | Voice-to-code | aider | `aider/.../voice.md` | HAVE (`internal/voice`) | — | — |
| F06 | Lint & test on edit (auto-fix loop) | aider, cline, swe-agent | `aider/.../lint-test.md`; `cline/README.md`; `swe-agent/docs/background/aci.md` | PARTIAL (quality gate exists; not wired as per-edit linter-block) | M | 1 |
| F07 | `/undo` + `/diff` (git-aware) | aider, forge | `aider/.../commands.md` | PARTIAL (autocommit + checkpoints; no `/undo`/`/diff` UX) | S | 1 |
| F08 | Images & web pages in chat (`/web`,`/paste`) | aider, gpt-engineer | `aider/.../images-urls.md`; `gpt-engineer/README.md` (`--image_directory`) | PARTIAL (HXC-031 multimodal in-flight; `/web` scrape via browser tools) | M | 2 |
| F09 | Watch-files / IDE-comment mode | aider | `aider/.../watch.md` | MISSING | M | 3 |
| F10 | Copy/paste-to-web-chat (`/copy-context`) | aider | `aider/.../copypaste.md` | MISSING | S | 5 |
| F11 | Plan/Act dual mode | cline, gemini-cli | `cline/README.md`; `gemini-cli/docs/cli/plan-mode.md` | PARTIAL (planner; no explicit Plan/Act toggle) | M | 1 |
| F12 | Checkpoints (workspace snapshot + restore/undo) | cline, gemini-cli, amazon-q, nanocoder | `cline/README.md`; `gemini-cli/docs/checkpointing.md`; `amazon-q/docs/experiments.md`; `nanocoder/docs/features/index.md` | PARTIAL (task DB checkpoints; not workspace-file snapshot/restore) | M | 1 |
| F13 | MCP marketplace + on-the-fly tool creation | cline, codename-goose | `cline/README.md`; goose extension system | PARTIAL (MCP client HAVE; no marketplace/auto-create) | M | 2 |
| F14 | `.clinerules`/`GEMINI.md`/`AGENTS.md` context files | cline, gemini-cli, codex, forge, shai, mistral-code | `cline/.clinerules/*`; `gemini-cli/GEMINI.md`; `codex/AGENTS.md`; `shai/README.md` (SHAI.md) | HAVE (`internal/projectmemory`, rules) | — | — |
| F15 | Plugin SDK (programmatic tools + hooks) | cline, gptme | `cline/README.md` (`@cline/sdk`); `gptme/README.md` | HAVE (`internal/plugins`) | — | — |
| F16 | Multi-agent teams (coordinator → specialists) | cline, codename-goose, multiagent-coding, zeroshot | `cline/README.md`; goose GooseTeam; `multiagent-coding/README.md` | PARTIAL (agent coordinator + roocode delegation; no persistent team state) | L | 2 |
| F17 | Scheduled/cron agents | cline, nanocoder | `cline/README.md`; `nanocoder` daemon (croner) | MISSING | M | 3 |
| F18 | Messaging connectors (Slack/Telegram/Discord/…) | cline | `cline/README.md` | MISSING (Herald submodule precedent exists org-wide) | L | 3 |
| F19 | Headless / non-interactive (JSON/stream-json out) | cline, gemini-cli, qwen-code, codex, shai, vtcode, gpt-engineer | `cline/README.md`; `gemini-cli/docs/cli/headless.md`; `codex/docs/exec.md`; `shai/README.md`; `vtcode/README.md` | PARTIAL (`cmd/cli`; verify structured JSON/stream-json output) | M | 1 |
| F20 | Background long-process monitoring | cline, plandex | `cline/README.md`; `plandex/.../execution-and-debugging.md` | PARTIAL (`internal/workflow/background.go`) | M | 2 |
| F21 | Kanban / parallel task board | cline, agent-deck | `cline/README.md`; `agent-deck/llms.txt` | MISSING | L | 4 |
| F22 | Cumulative diff-review sandbox (stage-before-apply) | plandex, claude-squad, forge | `plandex/.../reviewing-changes.md`; `claude-squad/README.md`; `forge/README.md` (`--sandbox`) | PARTIAL (worktree isolation; no cumulative-diff staging/apply-reject UX) | L | 1 |
| F23 | Configurable autonomy levels (preset tiers) | plandex, nanocoder, copilot-cli | `plandex/.../autonomy.md`; `nanocoder` modes; `copilot-cli` Autopilot | PARTIAL (approval engine; no named autonomy presets) | M | 1 |
| F24 | Automated command/build/test debugging | plandex, shai, gptme | `plandex/.../execution-and-debugging.md`; `shai/README.md` (shell hook); `gptme` | PARTIAL (verifier + quality; no auto-debug loop on failing cmd) | M | 2 |
| F25 | Conversation branches / fork | plandex, forge, claude-squad, agent-deck | `plandex/.../branches.md`; `forge/README.md`; `agent-deck/llms.txt` | MISSING (sessions exist; no branch/fork) | M | 2 |
| F26 | Big-context management (load files/dirs/urls; smart context) | plandex, gemini-cli, nanocoder | `plandex/.../context-management.md`; `gemini-cli/README.md`; `nanocoder` File Explorer | HAVE/PARTIAL (context pkg + repomap; `@`-mention & explorer UX partial) | M | 2 |
| F27 | Model packs / mid-session model switch | plandex, crush, copilot-cli, junie, qwen-code | `plandex` model packs; `crush/README.md`; `copilot-cli` `/model`; `qwen-code` `/model` | HAVE (LLM aliases, `internal/llm/aliases.go`) | — | — |
| F28 | LSP-enhanced context | crush, copilot-cli, nanocoder | `crush/README.md`; `copilot-cli/README.md`; `nanocoder` (`source/lsp/`) | HAVE (LSP in context/tools) | — | — |
| F29 | Permission allowlist + `--yolo`/auto-approve | crush, vtcode, open-interpreter, claude-squad, qwen-code | `crush/README.md`; `vtcode/README.md`; `open-interpreter/README.md` (`-y`) | HAVE (`internal/approval`) | — | — |
| F30 | Provider auto-update catalogue | crush (Catwalk) | `crush/README.md` | HAVE (LLMsVerifier = single source, CONST-036) | — | — |
| F31 | Shared workspace across clients (server backend) | crush, qwen-code (daemon), shai (serve) | `crush/README.md`; `qwen-code/README.md`; `shai/README.md` | MISSING (server pkg exists; no shared-session multi-client) | L | 3 |
| F32 | Self-config skill ("configure yourself") | crush | `crush/README.md` (crush-config skill) | PARTIAL (config tool exists; not LLM-driven skill) | S | 4 |
| F33 | OS sandbox for tool exec (Seatbelt/Landlock/bwrap) | codex, gemini-cli, octogen, taskweaver | `codex/AGENTS.md`; `codex/docs/sandbox.md`; `octogen/README.md`; `taskweaver` container | PARTIAL (worker isolation + infraboot; OS-level seatbelt/landlock not confirmed) | L | 2 |
| F34 | Exec-policy / command-approval engine | codex, amazon-q, vtcode | `codex/docs/execpolicy.md`; `amazon-q/docs/built-in-tools.md` | HAVE (`internal/approval` + selector) | — | — |
| F35 | ACP mode (Agent Client Protocol over stdio) | gemini-cli, gptme, qwen-code | `gemini-cli/docs/cli/acp-mode.md`; `gptme/README.md`; `qwen-code` daemon (ACP) | MISSING | M | 3 |
| F36 | `/rewind` (conversation + file-state rewind) | gemini-cli | `gemini-cli/docs/cli/rewind.md` | PARTIAL (checkpoints; no rewind UX) | M | 2 |
| F37 | Model routing / auto-fallback on quota | gemini-cli, crush | `gemini-cli/docs/cli/model-routing.md` | HAVE (`internal/llm/auto_llm_manager.go`, degraded-mode) | — | — |
| F38 | Model steering (mid-task course-correct) | gemini-cli | `gemini-cli/docs/cli/model-steering.md` | MISSING | M | 4 |
| F39 | Git worktrees per session | gemini-cli, claude-squad, forge, zeroshot, agent-deck | `gemini-cli/docs/cli/git-worktrees.md`; `claude-squad/README.md` | HAVE (`internal/session/manager_worktree.go`) | — | — |
| F40 | Agent Skills (`SKILL.md` open standard) | gemini-cli, crush, vtcode, forge, nanocoder, codex, qwen-code | `gemini-cli/docs/cli/skills.md`; `crush/README.md`; `vtcode/README.md`; `forge/README.md` | HAVE/PARTIAL (plugins+skills; confirm `SKILL.md` precedence) | S | 2 |
| F41 | `ask_user` / TODO / tracker tools | gemini-cli, amazon-q, nanocoder | `gemini-cli/docs/tools/{ask-user,todos,tracker}.md`; `amazon-q/docs/experiments.md` | HAVE/PARTIAL (clarification + plantree; TODO-tool UX partial) | S | 2 |
| F42 | Knowledge base (persistent semantic index) | amazon-q, aichat (RAG), gptme (RAG), forge (semantic search) | `amazon-q/docs/knowledge-management.md`; `aichat/README.md`; `forge/README.md` (`:sync`) | HAVE (`internal/memory` cognee + RAG) | — | — |
| F43 | Tangent mode (side-topic checkpoint) | amazon-q | `amazon-q/docs/tangent-mode.md` | MISSING | M | 3 |
| F44 | Introspect tool (self-aware about own features) | amazon-q | `amazon-q/docs/introspect-tool.md` | MISSING | S | 4 |
| F45 | Delegate (async background sub-sessions) | amazon-q, gptme (subagent), nanocoder, vtcode | `amazon-q/docs/experiments.md`; `gptme/README.md`; `vtcode/README.md` | PARTIAL (roocode delegation + workflow background) | M | 2 |
| F46 | Recipes (reusable parametrized workflows) | codename-goose, forge (skills) | goose recipes; `forge/README.md` | PARTIAL (plugins/skills; no parametrized-recipe primitive) | M | 3 |
| F47 | Lessons system (auto-injected contextual guidance) | gptme | `gptme/README.md` | MISSING | M | 4 |
| F48 | Computer use (GUI/desktop control) | gptme, open-interpreter, warp (Oz) | `gptme/README.md`; `open-interpreter/README.md` | PARTIAL (browser tools; no full desktop control) | L | 4 |
| F49 | Local code-exec REPL (NL → run code) | open-interpreter, gptme, octogen, taskweaver | `open-interpreter/README.md`; `octogen/README.md` | HAVE (command exec tools; BLUFF-003 resolved) | — | — |
| F50 | Profiles (YAML behavior presets) | open-interpreter, codename-goose, claude-squad | `open-interpreter/README.md`; goose profiles | PARTIAL (config profiles; no named behavior-profile UX) | S | 4 |
| F51 | Whole-project generation from a prompt | gpt-engineer | `gpt-engineer/README.md` | PARTIAL (agent loop can; no `gpte <dir>` one-shot scaffold mode) | M | 3 |
| F52 | Benchmark harness (SWE-bench/TerminalBench) | gpt-engineer, swe-agent, codename-goose, multiagent-coding | `gpt-engineer/README.md`; `swe-agent/README.md`; goose `goose-bench` | MISSING (helix_qa is QA, not agent-benchmark) | L | 5 |
| F53 | Agent-Computer Interface tools (windowed editor/viewer, succinct search) | swe-agent | `swe-agent/docs/background/aci.md` | PARTIAL (editor + repomap; windowed-viewer UX absent) | M | 3 |
| F54 | Shell assistant (failed-command auto-fix hook) | shai, aichat, plandex | `shai/README.md`; `aichat/README.md` | MISSING | M | 3 |
| F55 | Chainable `--trace` (pipe conversation between runs) | shai | `shai/README.md` | MISSING | S | 4 |
| F56 | HTTP server (OpenAI-compatible endpoints) | shai, aichat, gptme, open-interpreter, octogen | `shai/README.md`; `aichat/README.md`; `gptme/README.md` | PARTIAL (`internal/server`; OpenAI-compat surface unconfirmed) | M | 3 |
| F57 | Context compression `/compact` + `/usage` | nanocoder, qwen-code, forge | `nanocoder/docs/features/index.md`; `qwen-code/README.md` | HAVE (`internal/session/condense.go`) | — | — |
| F58 | Session auto-save + `/resume` | nanocoder, codex, vtcode | `nanocoder/docs/features/index.md`; `vtcode/README.md` | HAVE (`internal/session`) | — | — |
| F59 | Event triggers (file-watch + cron → skill) | nanocoder | `nanocoder/CLAUDE.md` (daemon) | MISSING | M | 3 |
| F60 | VS Code / IDE extension bridge | nanocoder, codex, qwen-code, junie | `nanocoder/docs/features/vscode-extension.md`; `codex/README.md` | MISSING (out of CLI scope; note for app team) | L | 5 |
| F61 | Spec-driven workflow (specify→plan→tasks→implement) | spec-kit, get-shit-done | `spec-kit/README.md`; `get-shit-done/docs/FEATURES.md` | PARTIAL (planner/plantree; no spec-command surface) | L | 2 |
| F62 | QA / regression gates in workflow | get-shit-done | `get-shit-done/docs/FEATURES.md` | HAVE (quality gate + verifier + helix_qa) | — | — |
| F63 | Cross-AI peer review (multi-runtime review) | get-shit-done, multiagent-coding | `get-shit-done/docs/FEATURES.md` | PARTIAL (ensemble + verifier; no cross-runtime reviewer) | M | 3 |
| F64 | Token/usage tracking per request + context-% | codai, amazon-q, nanocoder | `codai/README.md`; `amazon-q/docs/experiments.md` | HAVE/PARTIAL (telemetry pkg; in-prompt % indicator partial) | S | 3 |
| F65 | Stateful code-first execution (DataFrames persist) | taskweaver, octogen | `taskweaver/README.md`; `octogen/README.md` | PARTIAL (substrate; not data-analytics-stateful) | L | 5 |
| F66 | MCP socket pool (share MCP procs across sessions) | agent-deck | `agent-deck/llms.txt` | MISSING | M | 4 |
| F67 | Global conversation search (fuzzy + regex) | agent-deck | `agent-deck/llms.txt` | MISSING | S | 4 |
| F68 | Conversation export (`:dump` JSON/HTML) | forge, aichat | `forge/README.md` | MISSING | S | 4 |
| F69 | Restricted/secure shell mode (path/symlink guards) | forge, vtcode | `forge/README.md`; `vtcode/README.md` (layered security) | PARTIAL (approval + security pkg) | M | 3 |
| F70 | ChatGPT/account-plan auth (non-API-key sign-in) | codex, junie, copilot-cli | `codex/docs/authentication.md`; `copilot-cli/README.md` | OUT-OF-SCOPE (provider-specific; verifier owns auth) | — | — |

---

## 6. Top-10 Highest-Value Missing Features

Ranked by (user impact × low duplication-with-existing × evidence strength). "Missing" or "Partial-surface" only — excludes things HelixCode already HAS.

1. **F22 — Cumulative diff-review sandbox (stage-before-apply, apply/reject hotkeys)** — plandex/forge/claude-squad. The single biggest trust/UX lever; HelixCode has worktrees but no cumulative-diff staging+review surface. **L.**
2. **F23 — Configurable autonomy levels (named preset tiers None→Full)** — plandex/nanocoder/copilot. Wraps the existing approval engine into discoverable presets controlling auto-continue/build/apply/exec/commit. **M.**
3. **F12 — Workspace checkpoints (file snapshot + restore/undo)** — cline/gemini/amazon-q/nanocoder. HelixCode checkpoints are task-DB rows, not workspace-file snapshots; this is the "oops, revert everything" safety net. **M.**
4. **F11/F04 — Plan/Act + first-class chat modes (code/ask/architect/plan)** — cline/aider/gemini/forge. Mode switching is a defining UX of every top agent; planner substrate exists. **M.**
5. **F61 — Spec-driven workflow surface (specify→clarify→plan→tasks→implement + constitution)** — spec-kit/get-shit-done. Aligns directly with HelixCode's constitution discipline. **L.**
6. **F25 — Conversation branches / fork** — plandex/forge/agent-deck. Explore multiple approaches/models from a shared point. **M.**
7. **F36/F43 — `/rewind` + Tangent mode** — gemini-cli/amazon-q. Side-explore and time-travel without losing place. **M.**
8. **F19/F56 — Robust headless (JSON/stream-json) + OpenAI-compatible server** — cline/gemini/shai/aichat. Unlocks CI/CD + programmatic embedding. **M.**
9. **F33 — OS-level tool-exec sandbox (Seatbelt/Landlock/bwrap)** — codex/gemini. Hard isolation beyond worktrees; safety-critical for full-auto. **L.**
10. **F18 — Messaging connectors (Slack/Telegram/Discord/Linear)** — cline. Org already has the Herald precedent to extend (§11.4.74 catalogue-first), making this lower-risk than greenfield. **L.**

Runners-up (just outside top-10): F35 ACP mode, F45 async delegate, F59 event triggers, F54 shell-assistant auto-fix.

---

## 7. Per-Feature Acceptance Bar (anti-bluff)

Per the operator's bar and §11.4.27 / §11.4.69 / §11.4.83 / §11.4.98 / §11.4.107 / §11.4.135, **every** ported feature is DONE only when it ships ALL of:

1. **Real implementation** — no simulation/placeholder (§Rule 2, BLUFF-001/002/003); touches real files/processes/LLM calls.
2. **A triggering prompt** — the exact natural-language prompt that exercises the feature end-to-end (§11.4.105 intent-recognition compatible).
3. **A recorded video/transcript demo** under `docs/qa/<run-id>/` (§11.4.83) — full bidirectional thread + captured evidence.
4. **Tests** — unit + integration + (where applicable) stress + chaos (§11.4.85), each anti-bluff with a paired §1.1 mutation; integration/E2E hit real infra (§11.4.50(A)).
5. **A Challenge** in `challenges/` exercising the complete user workflow (§Rule 6/7).
6. **A HelixQA bank entry** executed in an autonomous session with captured wire evidence (§11.4.27 (B)).
7. **A standing regression guard** registered on close (§11.4.135) with RED-on-broken-artifact polarity switch (§11.4.115).
8. **Docs in sync** — feature manual/guide + tracker row with `Catalogue-Check:` field (§11.4.74).

---

## 8. Phases → Tasks → Sub-Tasks

Each task names concrete file targets (under `helix_code/internal/…` or `helix_code/cmd/cli/…`) and acceptance criteria. Tasks are sized for §11.4.70 subagent-driven, ≤~300-line units. Every sub-task inherits the §7 acceptance bar.

### Phase 0 — Foundations & Audit

> Goal: replace assumptions with FACTS before any port. No new features.

- **T0.1 — Capability-confirmation audit.** Verify the PARTIAL rows by reading code, not guessing (§11.4.6).
  - ST0.1.1 — Confirm headless JSON/stream-json output state in `cmd/cli/main.go` + `internal/render`. Acceptance: documented yes/no with file:line.
  - ST0.1.2 — Confirm `SKILL.md` precedence vs plugins in `internal/plugins` (F40). Acceptance: precedence table captured.
  - ST0.1.3 — Confirm OS-sandbox depth in `internal/worker/isolation` + `internal/infraboot` (F33). Acceptance: "worktree-only" vs "seatbelt/landlock" determination.
  - ST0.1.4 — Confirm OpenAI-compat surface in `internal/server` (F56). Acceptance: endpoint inventory.
- **T0.2 — Catalogue-check sweep (§11.4.74).** For each planned feature, record `reuse|extend|no-match <org/repo@sha>` (e.g. F18 → extend Herald; F42 → reuse cognee). Acceptance: every Phase 1–6 feature has a `Catalogue-Check:` line.
- **T0.3 — Tracker rows.** Open one Feature item per planned feature in the workable-items DB (§11.4.93) with `created_by`/`assigned_to` (§11.4.104). Acceptance: DB↔MD in sync (`workable-items validate`).

### Phase 1 — High-Value / Low-Effort (already-partial)

> Wrap existing substrate into discoverable surfaces. Mostly S/M.

- **T1.1 — `/undo` + `/diff` (F07, S).** Targets: `cmd/cli` command handlers, bridge to `internal/autocommit` + `internal/task/checkpoint.go`. AC: `/undo` reverts last agent commit; `/diff` shows changes since last message; Challenge reverts a real edit.
- **T1.2 — Autonomy presets (F23, M).** Targets: new `internal/autonomy` wrapping `internal/approval`; presets None/Basic/Plus/Semi/Full controlling auto-continue/build/apply/exec/commit. AC: each tier provably changes approval behavior; mutation flips a tier → test FAILs.
- **T1.3 — First-class chat/Plan-Act modes (F04, F11, M).** Targets: `cmd/cli` mode state + `internal/planner` (read-only ask, plan, architect, act). AC: mode switch changes tool availability (ask = read-only); Challenge proves no writes in ask mode.
- **T1.4 — TODO/tracker tool surface (F41, S).** Targets: bridge `internal/plantree` + `internal/clarification` to a user-visible `/tasks` + `ask_user` tool. AC: multi-step task auto-tracked; evidence transcript.
- **T1.5 — Token/usage + context-% indicator (F64, S).** Targets: `internal/telemetry` → prompt render in `internal/render`. AC: per-request token count + color-coded context % shown; numbers match `usage` object (§11.4.141 — no tiktoken estimate).
- **T1.6 — Skill `SKILL.md` precedence (F40, S, gated on ST0.1.2).** AC: repo/user/built-in precedence documented + tested.

### Phase 2 — Session & Context Power Tools

- **T2.1 — Workspace checkpoints (F12, M).** Targets: extend `internal/session` + worktree to snapshot/restore working-tree files (not just task DB). AC: create→edit→restore round-trips real files; chaos test mid-write SIGKILL recovers cleanly.
- **T2.2 — Conversation branches/fork (F25, M).** Targets: `internal/session` branch primitive + `cmd/cli` `/branch`. AC: fork from a point, diverge, compare; both branches persist.
- **T2.3 — `/rewind` (F36, M).** Targets: compose T2.1 checkpoints + history rewind in `internal/session/condense.go`. AC: rewind reverts conversation + optionally files.
- **T2.4 — Tangent mode (F43, M).** Targets: `internal/session` tangent stack. AC: enter tangent, explore, return to exact prior state; `tangent tail` keeps last Q+A.
- **T2.5 — Big-context `@`-mention + file explorer (F26, M).** Targets: `internal/context` + repomap; `@file`/`@dir`/`@url` loaders + token estimates. AC: `@`-load injects real content with token budget shown.
- **T2.6 — Conversation export + global search (F68, F67, S each).** Targets: `internal/session` `:dump` JSON/HTML; fuzzy+regex search across sessions. AC: exported HTML opens; search finds a known phrase.

### Phase 3 — Autonomy & Safety Surface

- **T3.1 — OS-level exec sandbox (F33, L, gated on ST0.1.3).** Targets: `internal/worker`/`internal/infraboot` — Seatbelt (macOS), Landlock/bwrap (Linux), per §11.4.81 cross-platform parity (per-OS branches + honest kernel-gap citation). AC: a write outside the sandbox is blocked on each OS branch; paired mutation strips a branch → gate FAILs.
- **T3.2 — Lint/test-on-edit auto-fix loop (F06, M).** Targets: `internal/editor` + `internal/quality` — block edit on syntax/lint failure, auto-retry. AC: a deliberately broken edit is blocked then fixed; SWE-agent-style linter-block evidenced.
- **T3.3 — Automated command/build/test debugging (F24, M).** Targets: `internal/verifier` + new debug loop on failing exec. AC: a failing build is auto-diagnosed + fixed with captured evidence.
- **T3.4 — Shell-assistant failed-command hook (F54, M).** Targets: `cmd/cli` shell hook + `internal/approval`. AC: a failed shell command yields an LLM-proposed fix (opt-in).
- **T3.5 — Restricted/secure shell mode (F69, M).** Targets: extend `internal/security` path/symlink/dangerous-command guards. AC: a path-escape command is blocked; audit log captured.

### Phase 4 — Multi-Agent & Parallel Orchestration

- **T4.1 — Cumulative diff-review sandbox (F22, L).** Targets: new `internal/changeset` staging layer over worktrees + `cmd/cli` review menu (apply/reject per-file/hunk). AC: changes accumulate unapplied; `diff --ui` apply/reject lands only accepted hunks; reject leaves tree clean.
- **T4.2 — Persistent multi-agent teams (F16, L).** Targets: extend `internal/agent` coordinator + `internal/roocode` with named-team persistence. AC: coordinator delegates to specialists with isolated context; team state survives restart (§11.4.119 single-resource-owner for shared files).
- **T4.3 — Async delegate / background sub-sessions (F45, M).** Targets: `internal/workflow/background.go` + `internal/roocode`. AC: background sub-session runs in parallel, summary injected into main context (§11.4.89 background execution).
- **T4.4 — Cross-AI peer review (F63, M).** Targets: `internal/verifier` ensemble → multi-runtime reviewer. AC: a second model/runtime reviews a diff; findings captured.
- **T4.5 — Kanban/parallel task board + MCP socket pool (F21, F66, L).** Targets: app/server task board; `internal/mcp` shared-process pool. AC: parallel cards each on own worktree; MCP procs shared with measured memory reduction.

### Phase 5 — Connectors, Server & Headless Surface

- **T5.1 — Robust headless output (F19, M, gated on ST0.1.1).** Targets: `cmd/cli` `--output-format json|stream-json`. AC: NDJSON event stream; `git diff | helix --json` works in a pipe; `-count=3` deterministic (§11.4.98).
- **T5.2 — OpenAI-compatible server (F56, M, gated on ST0.1.4).** Targets: `internal/server` chat-completions/responses/embeddings + SSE. AC: a real OpenAI client hits the endpoint and gets a completion.
- **T5.3 — Shared-session daemon (F31, L).** Targets: `internal/server` multi-client shared session keyed by cwd (crush/qwen/shai pattern). AC: two clients mirror the same session live; loopback no-auth, remote token-gated.
- **T5.4 — ACP mode (F35, M).** Targets: `cmd/cli --acp` JSON-RPC over stdio. AC: a JSON-RPC initialize/prompt round-trips (Zed/JetBrains-compatible shape).
- **T5.5 — Messaging connectors (F18, L, extend Herald per §11.4.74).** Targets: connector adapters → session mapping with access control. AC: a real Slack/Telegram thread drives a session with @-attribution (§11.4.104) + intent recognition (§11.4.105).
- **T5.6 — Chainable `--trace` (F55, S).** AC: `helix ... | helix "now run it"` pipes a real conversation trace.

### Phase 6 — Spec-Driven & Quality-Gate Workflow

- **T6.1 — Spec-driven command surface (F61, L).** Targets: `cmd/cli` + `internal/planner`/`plantree` — `/specify`,`/clarify`,`/plan`,`/tasks`,`/implement`,`/analyze` with a project constitution file. AC: a spec produces a plan → tasks → real implementation; constitution governs phases. (Strong synergy with HelixCode's existing constitution discipline.)
- **T6.2 — Recipes (parametrized reusable workflows) (F46, M).** Targets: `internal/plugins`/skills — parametrized recipe primitive. AC: a recipe runs with substituted parameters end-to-end.
- **T6.3 — Event triggers (file-watch + cron → skill/recipe) (F59, F17, M).** Targets: new `internal/triggers` daemon (file-watch + cron) firing skills. AC: a file change fires a subscribed skill; a cron schedule survives restart.
- **T6.4 — Whole-project generation one-shot (F51, M).** Targets: `cmd/cli` scaffold mode reading a prompt file → full project. AC: `helix gen <dir>` from a prompt file produces a buildable project.

### Phase 7 — Long-Tail / Niche Features

- **T7.1 — Watch-files / IDE-comment mode (F09, M).** AC: an `AI!` comment in a file triggers an edit.
- **T7.2 — Lessons system (auto-injected guidance) (F47, M).** AC: a keyword/tool match injects a lesson; opt-out works.
- **T7.3 — Introspect tool + self-config skill (F44, F32, S each).** AC: "what can you do?" answered from bundled docs; "configure yourself" edits config.
- **T7.4 — Model steering mid-task (F38, M).** AC: a steer message redirects without restarting the loop.
- **T7.5 — Profiles (YAML behavior presets) (F50, S).** AC: `--profile` switches a named behavior set.
- **T7.6 — Copy-context to web chat (F10, S).** AC: `/copy-context` yields paste-ready context.
- **T7.7 — Benchmark harness (F52, L) + ACI windowed editor (F53, M).** AC: agent runs against a public benchmark task; windowed viewer shows ~100 lines/turn with in-file search.
- **T7.8 — Computer use / desktop control (F48, L).** AC: a GUI action is driven + verified (compose §11.4.107 liveness).
- **T7.9 — Stateful code-first execution (DataFrames persist) (F65, L)** + **token-economy review** — only if a data-analytics use case is greenlit.

---

## 9. Dependency Touch-Map

Which org submodules / internal subsystems each feature cluster touches (per §11.4.51 equal-codebase, extend-don't-reimplement):

| Feature cluster | HelixAgent | LLMsVerifier | HelixMemory (cognee) | HelixLLM | HelixSpecifier | Herald | helix_qa | challenges |
|---|---|---|---|---|---|---|---|---|
| Modes/Autonomy (F04,F11,F23) | ✓ agent loop | ✓ model status | — | ✓ routing | — | — | ✓ | ✓ |
| Checkpoints/Rewind/Tangent/Branches (F12,F25,F36,F43) | ✓ session | — | partial (history) | — | — | — | ✓ | ✓ |
| Diff-sandbox (F22) | ✓ worktree | — | — | — | — | — | ✓ | ✓ |
| Multi-agent/Delegate (F16,F45,F63) | ✓ coordinator/roocode | ✓ ensemble | ✓ shared mem | ✓ | — | — | ✓ | ✓ |
| Server/Headless/ACP/Connectors (F19,F31,F35,F56,F18) | ✓ | ✓ | — | ✓ | — | ✓ extend | ✓ | ✓ |
| Spec-driven (F61) + Recipes/Triggers (F46,F59) | ✓ planner/plantree | — | — | — | ✓ (spec engine) | — | ✓ | ✓ |
| Sandbox/Lint-fix/Auto-debug (F33,F06,F24) | ✓ worker/editor | ✓ verifier | — | — | — | — | ✓ | ✓ |
| Context/`@`-mention/RAG (F26,F42) | ✓ context | — | ✓ cognee | — | — | — | ✓ | ✓ |

`HelixSpecifier` is the natural owner for F61's spec engine (catalogue-check before greenfield). `Herald` is the catalogue-first base for F18 messaging connectors.

---

## 10. Honest Effort Caveats

- **No over-claiming "missing".** Many marquee features (repo-map F01, auto-commit F02, voice F05, model routing F37, worktrees F39, RAG/knowledge F42, condense F57, sessions F58, approval F29/F34, LSP F28, provider catalogue F30) are **already HAVE** in HelixCode — porting work there is zero or surface-only.
- **PARTIAL ≠ trivial.** F12/F22/F33/F61 have substrate but the user-facing surface + cross-platform + anti-bluff coverage are the real cost (each is M–L).
- **Vendor-doc stubs.** codex `docs/*.md` and mistral-code's inherited gemini docs link out to websites; their feature claims were taken from README/AGENTS/crate-names, not in-tree prose — do not assume the in-repo copy is the spec.
- **Out-of-scope rows.** F60 (IDE extension), F70 (account-plan auth), F52 (agent benchmark) are flagged but are app-team / provider / research concerns, not core-CLI ports.
- **`openhands` absent.** The operator's example list named openhands; it is NOT in `cli_agents/` — excluded.
- This is a **planning document** (§11.4.73 spec-versioning applies once accepted). No code written. Effort estimates are S/M/L bands, not hour commitments.

---

## 11. Sources Verified

Researched 2026-06-14 by read-only subagents against the in-repo vendored copies under `cli_agents/`. Primary evidence files (non-exhaustive):

- aider: `aider/README.md`, `aider/aider/website/docs/{repomap,usage/modes,usage/commands,usage/voice,usage/watch,usage/lint-test,usage/copypaste,usage/images-urls}.md`
- cline: `cline/README.md`, `cline/.clinerules/*`
- plandex: `plandex/README.md`, `plandex/docs/docs/core-concepts/{plans,reviewing-changes,context-management,autonomy,execution-and-debugging,version-control,branches}.md`, `plandex/docs/docs/repl.md`
- crush: `crush/README.md`, `crush/docs/hooks/`
- codex: `codex/README.md`, `codex/AGENTS.md`, `codex/docs/{sandbox,exec,skills,slash_commands,config,agents_md,execpolicy,authentication}.md`, `codex/codex-rs/` crate layout
- gemini-cli: `gemini-cli/README.md`, `gemini-cli/docs/cli/{plan-mode,rewind,model-routing,model-steering,git-worktrees,acp-mode,skills,headless}.md`, `gemini-cli/docs/{checkpointing,hooks/index}.md`, `gemini-cli/docs/tools/*`
- qwen-code: `qwen-code/README.md`, `qwen-code/.qwen/skills/`
- copilot-cli: `copilot-cli/README.md`
- amazon-q: `amazon-q/docs/{built-in-tools,agent-format,knowledge-management,tangent-mode,introspect-tool,hooks,experiments,todo-lists}.md`
- mistral-code: `mistral-code/README.md`, `mistral-code/MISTRAL.md`, `mistral-code/docs/*` (mostly inherited)
- codename-goose: `codename-goose/README.md`, `ARCHITECTURE.md`, `crates/`, `documentation/blog/2025-02-21-gooseteam-mcp/index.md`
- gptme: `gptme/README.md`
- open-interpreter: `open-interpreter/README.md`
- forge: `forge/README.md`
- gpt-engineer: `gpt-engineer/README.md`
- swe-agent: `swe-agent/README.md`, `swe-agent/docs/background/aci.md`, `swe-agent/tools/`
- shai: `shai/README.md`
- nanocoder: `nanocoder/README.md`, `nanocoder/CLAUDE.md`, `nanocoder/docs/features/*`
- claude-squad: `claude-squad/README.md`
- claude-code: `claude-code/README.md`, `claude-code/plugins/`
- vtcode: `vtcode/README.md`
- warp: `warp/README.md`, `.warp/` skills
- get-shit-done: `get-shit-done/docs/FEATURES.md`, `commands/gsd/`, `AGENTS.md`
- spec-kit: `spec-kit/README.md`
- junie: `junie/README.md`
- aichat: `aichat/README.md`
- codai: `codai/README.md`
- taskweaver: `taskweaver/README.md`
- octogen: `octogen/README.md`
- zeroshot: `zeroshot/AGENTS.md`
- multiagent-coding: `multiagent-coding/README.md`
- agent-deck: `agent-deck/llms.txt`
- bridle: `bridle/CLAUDE.md`, `bridle/AGENTS.md`, `.claude-plugin/marketplace.json` (marketplace, not an agent)

HelixCode baseline verified against: `helix_code/internal/{agent,repomap,mcp,plugins,verifier,session,hooks,worker,memory,voice,clarification,task,autocommit,roocode,kilocode,planner,plantree,approval,approvalwire,quality,server,telemetry,context}` and `helix_code/applications/*`, plus `docs/plans/HXC-031-codex-cline-port.md`.
