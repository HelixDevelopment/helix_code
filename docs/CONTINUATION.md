# HelixCode CLI-Agent Fusion — Programme Continuation Guide

**Last updated:** 2026-05-06T23:30:00Z by meta-repo HEAD (this commit — F21 close-out)
**Maintenance mandate:** This file MUST be updated on every commit that changes
programme state. Out-of-sync continuation is a CRITICAL DEFECT — see
`CONSTITUTION.md` Article XIII §13.1 (CONST-044), `CLAUDE.md` §12, and
`AGENTS.md` "Continuation Maintenance" anchors.

---

## TL;DR — Resume in 30 seconds

If you are a fresh CLI agent picking this up:
1. `cd /run/media/milosvasic/DATA4TB/Projects/HelixCode`
2. Read this file end to end.
3. Read `docs/improvements/PROGRESS.md` ("Current focus" + active task list).
4. Read the most recent feature plan in `docs/superpowers/plans/` (currently
   `2026-05-06-p2-f21-codex-approval-modes.md`).
5. Continue from the next unticked task in that plan's task list.

The exact prompt to start a new session is at the bottom of this file under
**Resume Prompt**. Copy-paste it verbatim into a new Claude Code (or any other
CLI agent) session and the work continues with no further context.

---

## Programme overview

The CLI-Agent Fusion programme has 5 phases per the synthesis design at
`docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`:

| Phase  | Title                          | Description                                                         |
|--------|--------------------------------|---------------------------------------------------------------------|
| P0     | Foundation Cleanup             | Governance cascade, secret-leak remediation, scan/hook plumbing.    |
| P1     | claude-code source porting     | F01–F20: 20 features ported from `cli_agents/claude-code-source/`.  |
| P1.5   | Foundation Cleanup (post-F20)  | cli_agents restructure, dedup, api_keys.sh, docs unification, etc.  |
| P2     | CLI agent porting              | codex, aider, cline, plandex, openhands, ... per synthesis §4.2.    |
| P3     | Test infrastructure expansion  | Real-infra-only test runners, full integration matrix.              |
| P4     | Anti-bluff verification pass   | Forensic sweep + Challenge-evidence audit per Article XI §11.9.     |
| P5     | End-user materials uplift      | Docs / installers / website / packaging.                            |

---

## Phase status

| Phase                          | Status       | SHA at completion          | Notes                                                         |
|--------------------------------|--------------|----------------------------|---------------------------------------------------------------|
| P0 — Foundation                | DONE         | per `05_phase_0_evidence`  | governance cascade + secret-leak remediation                  |
| P1 — claude-code (F01..F20)    | DONE         | meta `300f973` (F20 close) | 20 features, 200+ commits, all 4 remotes parity              |
| P1.5 — Foundation Cleanup      | DONE         | meta `4131bf0`             | 12 WPs, ~48 commits, deepest-first push complete             |
| P2 — CLI agent porting         | IN PROGRESS  | F23 DONE @ this commit     | F23 closed (10/10 tasks); F24 next candidate (brainstorm)     |
| P3 — Test infra                | NOT STARTED  | —                          | depends on Phase 2 close                                      |
| P4 — Anti-bluff audit          | NOT STARTED  | —                          | depends on Phase 3 close                                      |
| P5 — End-user materials uplift | NOT STARTED  | —                          | final phase                                                   |

---

## Repository state (snapshot @ 2026-05-06T19:00Z)

| Repo                                              | Local HEAD   | Origin status         | Notes                                                       |
|---------------------------------------------------|--------------|------------------------|-------------------------------------------------------------|
| meta-repo (HelixCode)                             | `5ef13b8`    | 6 commits ahead (push pending) | 4 remotes: origin / github / gitlab / upstream     |
| HelixAgent                                        | `9a314ab`    | aligned with origin    | submodule; large (>500 MB)                                  |
| HelixQA                                           | `33613a7`    | aligned with origin    | submodule                                                   |
| Challenges                                        | `7e94f28`    | aligned with origin    | single `origin` remote (no mirrors yet)                     |
| Containers                                        | `7bed5c5`    | aligned with origin    | submodule                                                   |
| Security                                          | `7fc1e26`    | aligned with origin    | submodule                                                   |
| Dependencies/HelixDevelopment/LLMsVerifier        | `b4db2f9`    | aligned with origin    | canonical pin; HelixAgent has divergent transitive view     |
| Dependencies/HelixDevelopment/LLMOrchestrator     | `17378f9`    | aligned with origin    |                                                             |
| Dependencies/HelixDevelopment/LLMProvider         | `262e20f`    | aligned with origin    |                                                             |
| Dependencies/HelixDevelopment/VisionEngine        | `4f42ac5`    | aligned with origin    |                                                             |
| Dependencies/HelixDevelopment/DocProcessor        | `3a571e0`    | aligned with origin    |                                                             |
| MCP-Servers                                       | `4503e2d`    | aligned with origin    | third-party (modelcontextprotocol/servers)                  |

Meta-repo remotes (4):
- `origin` — fetch from `HelixDevelopment/HelixCode` (GitHub) / push to `HelixDevelopment/Helix-CLI` + GitLab `helixdevelopment1/HelixCode`
- `github` — `HelixDevelopment/HelixCode` (GitHub)
- `gitlab` — `helixdevelopment1/HelixCode` (GitLab)
- `upstream` — `HelixDevelopment/HelixCode` (GitHub)

---

## Active feature in flight

**P2-F25 — Plandex Plan Trees + Context Compaction.** Fifth Phase 2 feature; spec + plan landed. T01 in progress.

### Most recent feature (closed): P2-F24 — Codex Project Memory

- Spec: `7128289` (`docs/superpowers/specs/2026-05-06-p2-f21-codex-approval-modes-design.md`)
- Plan: `bbb61de` (`docs/superpowers/plans/2026-05-06-p2-f21-codex-approval-modes.md`)
- Closed: 2026-05-06 (T09 close-out commit)
- Status: **DONE** — first Phase 2 feature shipped
- Goal: Codex-compatible 4-mode approval system (suggest / auto-edit /
  full-auto / dangerously-bypass) with CLI flag > env > config > default
  precedence; per-tool `RequiresApproval()` gate; F14 sandbox coupling for
  full-auto; F02 final-deny authority retained; `/approval` slash + atomic-
  pointer runtime mode swap.

**Tasks completed (9 of 9):**

| Task | Commit       | Subject                                                                        |
|------|--------------|--------------------------------------------------------------------------------|
| T01  | `a7a349f`    | bootstrap Phase 2 evidence + advance PROGRESS to F21                           |
| T02  | `9c2664d`    | approval/types.go (ApprovalMode + ApprovalLevel + Decision + sentinels + ModeDescriptors) |
| T03  | `0d655d8`    | approval/selector.go (flag > env > config > default precedence)                |
| T04  | `5ef13b8`    | approval/manager.go (4×4 matrix gate + F02/F14/F19 integration)                |
| T05  | `19bffce` (+ `1195ef9`) | tools.Tool gains `RequiresApproval()`; spec §3.6 explicit-override applied to all ~38 tool impls + DefaultLevelEdit safe-default |
| T06  | `ad8843b` (+ `9b72c26`) | `/approval` slash command (status/set/show) — ApprovalInspector seam + ParseMode + ModeDescriptors render |
| T07  | `c022968` (+ `bd67324`) | main.go wiring + `--approval` flag + registry hook + 8 integration tests (approvalwire.AskUserYesNoPrompter + ToolRegistry.SetApprovalManager + applyApprovalGate; sandbox markers injected for full-auto Run/All) |
| T08  | meta `2781c1a` / sub `f2ea964` (+ `ee413c3`) | Challenge harness 5 phases (suggest-deny, auto-edit-prompt, full-auto-sandbox, runtime-change, F02-final-deny); harness PASS, cross-compile linux/amd64 PASS, anti-bluff smoke clean; F02 contract pinned via in-stub deny-rule (registry-level F02 seam not yet wired — see CHALLENGE.md §11) |
| T09  | (this commit) | Feature 21 close-out + push 4 remotes non-force (PROGRESS + plan ticks + evidence T09 section + this CONTINUATION update) |

---

## Known issues / bugs / failures (out of scope but tracked)

### Pre-existing (from before P1.5)

- **HelixAgent build FAIL:** missing `Agentic/go.mod` (replace target empty
  submodule). Tests: 79 PASS / 302 FAIL (cascading `[setup failed]`).
- **HelixQA build FAIL:** missing replace-dir targets (`../VisionEngine`,
  `../LLMOrchestrator`, `../LLMsVerifier/llm-verifier`); missing go.sum
  entries. Tests: 100 PASS / 35 FAIL.
- **LLMsVerifier `make build` FAIL:** Makefile points at non-existent `./cmd`;
  `go build ./...` FAIL — missing go.sum for kafka-go, rabbitmq, etc.
- **Containers `make build` FAIL:** missing go.sum for `golang.org/x/{sys,
  crypto,term}`, prometheus/procfs.
- **`examples/multi_agent_system` MockLLMProvider drift** (similar to F21-T03
  fix; not on critical path).
- **`applications/desktop` link FAIL on host:** missing X11/Xcursor.h
  (environment issue, not code).

### Phase 1.5 deferred items (would have been WP scope but pragmatically deferred)

- **WP2 network-failed cli_agents (6):** `continue`, `kilo-code`, `mobile-agent`,
  `opencode-cli`, `openhands`, `roo-code` — retriable.
- **WP7 deferred snake_case renames (23 dirs):** 10 umbrella/top-level dirs
  (e.g. `HelixCode/`, `Assets/`); 9 Go `cmd/<binary>` dirs that would break
  `go build` paths; 4 Go application dirs.
- **WP4 api_keys.sh loader propagation deferred to:** Challenges, Security,
  Assets, Dependencies/HelixDevelopment/{LLama_CPP, Ollama, HuggingFace_Hub,
  …}, Github-Pages-Website, MCP-Servers, plus all submodules nested under
  HelixAgent/HelixLLM/.
- **HelixLLM/.gitmodules has stale `submodules/HelixQA` declaration**
  (directory absent on disk; only declaration remains). Stale; would surface
  in a future submodule recurse.

### Recursive submodule dedup pass (2026-05-06) — partial

Audit: `docs/improvements/recursive-dedup-audit.md`. Of 27 URLs flagged as
appearing at >1 path:

- **Removed (3):** orphan `[submodule "Toolkit/SiliconFlow"]`,
  `[submodule "Toolkit/Chutes"]`, `[submodule "Toolkit/Toolkit/Chutes"]`
  entries in `HelixAgent/.gitmodules`. None corresponded to a tracked
  gitlink (`HelixAgent/Toolkit/` is checked in as plain files), so removal
  is pure config cleanup.
- **Preserved by design (23):** the `HelixAgent/HelixLLM/submodules/*`
  tree (22) and `HelixAgent/Challenges` (1) are wired into their parent
  module's `go.mod` via `replace ./submodules/<Name>` and `replace
  ./Challenges` directives. They are vendored dependency trees, NOT
  duplicates. Removing them would break HelixLLM and HelixAgent
  compilation. Promoting them to root canonicals (`replace
  ../../../Containers` style parent-traversal) requires a Phase 2
  architectural ticket and full `go mod tidy` + build verification per
  module.
- **Skipped (1):** `cli_agents/bridle` ↔ `cli_agents/claude-plugins`
  (third-party, excluded from removal per task constraints).

Net effect: governance hygiene only. No functional submodule was removed.

### Constitutional debt (open since P0)

- **LLMsVerifier dual-pin divergence** (P0-04): canonical pin in
  `Dependencies/HelixDevelopment/LLMsVerifier` is one commit ahead of the
  transitive HelixAgent view. `make verify-foundation` exits 2 until
  resolved or explicitly waived via `VERIFY_FOUNDATION_WARN_ONLY=1`.
- **Historical SSH key + helix.security.json leaks** (P0-T08.5): material is
  immortal in git history. Mitigated; rotation required by operator.
- **SonarQube + Snyk live-scan deferral** (P0-T08.7): infrastructure wired,
  awaiting credential rotation by operator.

### Phase 2 backlog (not yet specced)

F22+ porting targets per synthesis design §4.2:

- **Codex follow-on:** image input / multimodal, project memory (codex.md).
- **Aider:** voice input, repo-map enhancements, git auto-commit per change.
- **Cline:** browser tool (chromium), computer use / screenshot.
- **Plandex:** branching plan trees, context compaction.
- **Openhands / Kiro / Kilo-code / Roo-code / Continue:** TBD per per-port
  brainstorming Q1-Q5.

The exact F22-FNN list is decided per-port via brainstorming Q1-Q5 (no
batch-spec). Choose next CLI-agent target after F21 closes.

---

## How to resume

### From a new CLI agent / LLM session

Type the **Resume Prompt** at the end of this file verbatim. It triggers
continuation without further user context.

### Programme conventions to apply (verbatim list)

1. **Subagent-driven-development always.** Never inline-implement multi-task
   features. Skip approval gates per the user's auto-approve memory
   (`memory/auto_approve_designs.md`).
2. **Commit on `main`.** All work flows through `main`. No feature branches.
3. **Push to 4 remotes (non-force only):** `origin`, `github`, `gitlab`,
   `upstream` for the meta-repo. Submodules push to their `origin` only
   (Challenges has no mirrors yet — known infra gap).
4. **Deepest-first push order.** Submodules → meta-repo. If meta-repo's
   gitlinks reference unpushed submodule SHAs, the meta-repo push will
   succeed but cloners will fail to resolve submodule pointers.
5. **Each feature has:** spec → plan → per-task TDD commits → Challenge
   harness commit → close-out commit. No exceptions.
6. **Anti-bluff smoke must always be `clean`.** Run before each commit:
   `grep -rn "simulated\|for now\|TODO implement\|placeholder" HelixCode/internal HelixCode/cmd && echo BLUFF || echo clean`.
7. **Runtime evidence required for every PASS** per CONST-035 / Article XI
   §11.9. No metadata-only / configuration-only / absence-of-error PASS.
8. **api_keys.sh > .env precedence.** Any tool that needs API keys sources
   them via `scripts/lib/api_keys.sh` first; falls back to `.env`.
9. **Non-FF push = STOP.** Never force, never `--force-with-lease`. If a
   push is rejected, investigate before retrying.
10. **No CI/CD pipelines.** All gates run via Makefile / scripts. Per CLAUDE.md
    Rule 1.
11. **No HTTPS for git.** SSH only.
12. **Every claim of "done" carries pasted terminal output** from a real run
    against real artefacts. Per CLAUDE.md Rule 8.

### Picking up F21 specifically

If F21 is the active feature when you resume:
1. Verify state: `git log --oneline -5` should show the T08 commit (`2781c1a`) at HEAD
   (the main.go wiring + registry hook commit on `main`, SHA `c022968`).
2. Read `docs/superpowers/plans/2026-05-06-p2-f21-codex-approval-modes.md` end
   to end.
3. Continue at **T09** (Feature 21 close-out + push to all 4 meta-repo
   remotes non-force; also push the Challenges submodule's own 4 remotes
   for the T08 commit `f2ea964` that was deferred). T08 completed at
   meta `2781c1a` / submodule `f2ea964` — challenge harness PASS,
   cross-compile linux/amd64 PASS, anti-bluff smoke clean.

### Picking up new feature after F21

If F21 is closed and Phase 2 next port not yet brainstormed:
1. Choose next CLI-agent target (codex follow-on / aider / cline / plandex
   per synthesis §4.2).
2. Brainstorm Q1-Q5 with user (or read existing brainstorm if one exists).
3. Spec + plan dispatch per the established pattern (see F12-F20 plans for
   reference shape).
4. Execute tasks per pattern (T01 bootstrap → TDD tasks → Challenge → close-out).

---

## Maintenance mandate

This document MUST be updated when:

- Any task is completed (update T-status table + add commit SHA).
- Any feature is closed out (update Phase status table + repository SHAs).
- Any known issue is discovered (add to "Known issues" section).
- Any phase boundary is crossed.
- Any deferred item is fixed or further deferred.
- Any new remote/submodule is added or removed.
- Any constitutional clause is added or amended.

If this document is out-of-sync with the actual state of the work, the
inconsistency is a **CRITICAL DEFECT** — same severity as a false-success
test result (CONST-035). See:

- `CONSTITUTION.md` Article XIII §13.1 (CONST-044) — Continuation Document Maintenance Mandate
- `CLAUDE.md` §12 — Continuation Maintenance
- `AGENTS.md` — "Continuation Maintenance" anchor

**Verification (TBD):** `scripts/verify_continuation_sync.sh` will compare:
- `Last updated` SHA in this file vs `git rev-parse HEAD` on `main`.
- `Active feature` here vs `Current focus` in `docs/improvements/PROGRESS.md`.
- Tasks-done count here vs ticked-tasks count in `PROGRESS.md`.
- Known-issue list here covers all documented failures in evidence files.

Non-zero exit = sync violation → blocking pre-push.

---

## Resume Prompt

Copy-paste this verbatim into a new CLI-agent session to continue:

```
Read /run/media/milosvasic/DATA4TB/Projects/HelixCode/docs/CONTINUATION.md and continue all work. Use subagent-driven-development. Skip approval gates per the project's auto-approve memory. Push all submodules + meta-repo to all configured remotes (non-force only) when each work package or feature is closed out.
```

---

## Document version log

| Date           | Updater       | What changed                                                       |
|----------------|---------------|--------------------------------------------------------------------|
| 2026-05-06     | Initial create| Captures state through P2-F21-T04 (`5ef13b8`); Phase 2 in flight.  |
| 2026-05-06     | T06 update    | T06 (`/approval` slash command) closed; 6 of 9 F21 tasks done.     |
| 2026-05-06     | T07 update    | T07 (main.go wiring + registry hook + integration test, `c022968`) closed; 7 of 9 F21 tasks done. |
| 2026-05-06     | T08 update    | T08 (Challenge harness 5 phases, meta `2781c1a` + sub `f2ea964`) closed; 8 of 9 F21 tasks done; T09 (close-out + push 4 remotes) is next. |
| 2026-05-06     | T09 close-out | F21 (Codex Approval Modes) CLOSED — first Phase 2 feature shipped. All 9 tasks ticked in plan + PROGRESS. F22 next candidate (brainstorming required). Decision log entry added in PROGRESS.md. Test summary, anti-bluff `clean`, cross-compile linux/amd64 (94 MB), and harness final-2-lines (`ALL CHECKS PASSED` / `P2-F21 challenge harness PASS`) recorded in `07_phase_2_evidence.md` §P2-F21-T09. |
| 2026-05-06     | F22 docs      | F22 (Aider Git Auto-Commit Per Change) spec + plan landed on `main`. Spec `8be7fba` (`docs/superpowers/specs/2026-05-06-p2-f22-aider-git-auto-commit-design.md`), plan `b4f217d` (`docs/superpowers/plans/2026-05-06-p2-f22-aider-git-auto-commit.md`). 9 tasks (T01 bootstrap → T09 Challenge + close-out). Q1-Q5=A,A,A,A,A. Zero new external deps. F22 in flight; T01 next. |
| 2026-05-07     | F23 docs      | F23 (Cline Browser Tool) spec + plan landed on `main`. Spec `83d401d` (`docs/superpowers/specs/2026-05-07-p2-f23-cline-browser-tool-design.md`), plan `bc5fd3e` (`docs/superpowers/plans/2026-05-07-p2-f23-cline-browser-tool.md`). 10 tasks (T01 bootstrap → T10 Challenge 7 phases + close-out). Q1-Q5=A,A,A,A,A. Zero new external deps (chromedp v0.15.1 + cdproto already direct from earlier work). F23 in flight; T01 next. |
| 2026-05-07     | F24 docs      | F24 (Codex Project Memory) spec + plan landed on `main`. Spec `c31b9ac` (`docs/superpowers/specs/2026-05-07-p2-f24-codex-project-memory-design.md`), plan `19094b8` (`docs/superpowers/plans/2026-05-07-p2-f24-codex-project-memory.md`). 8 tasks (T01 bootstrap → T08 Challenge 5 phases + close-out). Q1-Q5=A,A,A,A,A. Zero new external deps. F24 in flight; T01 next. |
| 2026-05-07     | F25 docs      | F25 (Plandex Plan Trees + Context Compaction) spec + plan landed on `main`. Spec `docs/superpowers/specs/2026-05-07-p2-f25-plandex-plan-trees-design.md`, plan `docs/superpowers/plans/2026-05-07-p2-f25-plandex-plan-trees.md`. 10 tasks (T01 bootstrap → T10 Challenge 7 phases + close-out). Q1-Q5=A,A,A,A,A (full plandex port; .helixcode/plans/ storage; 6 tools + /plan slash; F01 AutoCompactor reuse). Zero new external deps (google/uuid already direct). F25 in flight; T01 next. |

## F22 mid-flight section (active feature)

**Active feature:** P2-F22 — Aider Git Auto-Commit Per Change (second Phase 2 feature).

**Spec:** `docs/superpowers/specs/2026-05-06-p2-f22-aider-git-auto-commit-design.md` (commit `8be7fba`).

**Plan:** `docs/superpowers/plans/2026-05-06-p2-f22-aider-git-auto-commit.md` (commit `b4f217d`).

**User-confirmed design (Q1-Q5 = A,A,A,A,A):**
- Q1=A: ONE commit per accepted edit (aider default).
- Q2=A: LLM-summarised commit message; deterministic fallback on LLM error.
- Q3=A: `Co-Authored-By: HelixCode <noreply@helixcode.dev>` trailer on every auto-commit.
- Q4=A: Default ON; opt-out via env `HELIXCODE_GIT_AUTO_COMMIT=off`, runtime `/git_auto_commit off`, per-edit `_helix_skip_git_commit:true` param.
- Q5=A: `/git_auto_commit` slash command (status/on/off/show); NO cobra subcommand.

**Task progress:** 9 of 9 complete — feature CLOSED.

| Task | Status         | Subject                                                                                                                                  |
|------|----------------|------------------------------------------------------------------------------------------------------------------------------------------|
| T01  | DONE `550be34` | bootstrap F22 evidence section + advance PROGRESS to F22                                                                                 |
| T02  | DONE `0468beb` | autocommit/types.go: CommitContext + CommitResult + Options + sentinels + EnvVarName/CoAuthorTrailer/SkipParamKey constants (TDD)        |
| T03  | DONE `cb4fc30` | autocommit/git.go: thin git wrapper (IsRepo/StatusPorcelain/DiffStaged/DiffUnstaged/Add/Commit/HeadSHA) (real-git TDD)                 |
| T04  | DONE `4b2ab67` | autocommit/summariser.go + secret_filter.go: LLM summariser + deterministic fallback + 4 secret patterns (TDD)                        |
| T05  | DONE `3a28ca6` | autocommit/committer.go: AutoCommitter.MaybeCommit pipeline + atomic.Bool enabled + co-author trailer (real-git TDD)                  |
| T06  | DONE `db55e72` | registry.go: SetAutoCommitter + post-Execute fireAutoCommit hook + per-tool mutated-paths derivation (TDD)                            |
| T07  | DONE `a999b3a` | /git_auto_commit slash command (status/on/off/show) (TDD)                                                                             |
| T08  | DONE `bab7ebc` | main.go wiring (env + autocommit construct + registry hook + /git_auto_commit) + integration test                                     |
| T09  | DONE (this commit) | Challenge harness 6+1 phases (default-on + LLM-summary + non-edit-no-op + env-off + runtime-toggle + per-edit-skip + secret) + close-out + push 4 remotes |

**Anti-bluff hot zone:** §5.2 of spec — five critical patterns (commit-success-but-tree-dirty / message-doesn't-reflect-diff / fires-on-non-edit-tools / runtime-toggle-not-honoured / secret-leak); each pinned by unit + integration + Challenge phase. Challenge MUST exit non-zero on byte-evidence mismatch.

**Picking up F22 specifically:** F22 is CLOSED. F23 (Cline Browser Tool) docs landed; T01 next.

---

## F23 mid-flight section (active feature)

**Active feature:** P2-F23 — Cline Browser Tool (third Phase 2 feature).

**Spec:** `docs/superpowers/specs/2026-05-07-p2-f23-cline-browser-tool-design.md` (commit `83d401d`).

**Plan:** `docs/superpowers/plans/2026-05-07-p2-f23-cline-browser-tool.md` (commit `bc5fd3e`).

**User-confirmed design (Q1-Q5 = A,A,A,A,A):**
- Q1=A: Browser engine `chromedp` (already direct in `HelixCode/go.mod` v0.15.1 + cdproto v0.0.0-20260405000525-47a8ff65b46a). Chromium subprocess managed by chromedp via `NewExecAllocator` + `NewContext`.
- Q2=A: Six tools — `browser_navigate`, `browser_snapshot`, `browser_click`, `browser_type`, `browser_screenshot`, `browser_close`. Per-tool `RequiresApproval` per spec §3.6: `navigate`/`click`/`type`/`close` → `LevelEdit`; `snapshot`/`screenshot` → `LevelReadOnly`. NO `LevelRun`/`LevelAll`.
- Q3=A: Headless default; `HELIXCODE_BROWSER_HEADED=true` (case-insensitive literal `true`) opt-in for headed mode. Read once at session-create via `OptionsFromEnv()`.
- Q4=A: Screenshots written to `$XDG_DATA_HOME/helixcode/browser/screenshots/<session-id>/<n>.png` (mode `0600`); tool result returns absolute file path (NOT base64).
- Q5=A: `/browser` slash (`status` / `navigate <url>` / `close`) PLUS the six tools. NO cobra subcommand.

**Task progress:** 10 of 10 complete — F23 CLOSED.

| Task | Status | Subject                                                                                                                                          |
|------|--------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| T01  | DONE `64e499b` | bootstrap F23 evidence section + advance PROGRESS to F23                                                                                         |
| T02  | DONE `cdb323e` | browser/types.go + options.go: Snapshot + ScreenshotResult + ManagerStatus + sentinels + EnvVarHeadedMode + MaxSnapshotBytes + MaxScreenshotBytes (TDD) |
| T03  | DONE `e0ff1bf` | browser/manager.go + session.go: BrowserManager atomic-pointer lifecycle + BrowserSession with chromedp Context + per-session tempdir + sync.Once close + sessionFactory seam (TDD) |
| T04  | DONE `2bcb281` | browser_navigate_v2.go: browser_navigate Tool impl with WaitReady + lazy session-create + 30 s timeout (TDD); legacy tools renamed to browser_legacy_* |
| T05  | DONE `ec0b3cc` | browser_snapshot_v2.go: browser_snapshot Tool impl with html/text mode + 64 KB cap + Truncated flag (TDD)                                     |
| T06  | DONE `659307d` | browser_click_type_v2.go: browser_click + browser_type Tool impls with NodeVisible + 5 s selector timeout + ClickWaitDuration settle (TDD)   |
| T07  | DONE `3ca50a3` | browser_screenshot_v2.go: browser_screenshot Tool impl with PNG-magic + DecodeConfig + size>1024 + tempdir 0600 (TDD)                         |
| T08  | DONE `ad0c0df` | browser_close_v2.go: browser_close Tool impl with idempotent CloseSession + post-close RequireSession fails (TDD)                             |
| T09  | DONE `f39f686` | /browser slash + main.go wiring + RegisterBrowserToolsV2 + integration test (7/7 PASS against real chromium) |
| T10  | DONE (this commit) | Challenge harness 7 phases A-G + close-out + push 4 remotes — all 7 phases PASS against real chromium |

**Anti-bluff hot zone:** §5.2 of spec — five critical patterns (navigate-but-not-loaded / snapshot-empty-HTML / click-but-DOM-unchanged / screenshot-0-bytes-or-non-PNG / close-but-chromium-still-running); each pinned by unit + integration + Challenge phase. Challenge MUST exit non-zero on byte-evidence mismatch. Skip permitted ONLY if chromium absent (must emit `SKIP-OK: #P2-F23 chromium not available`).

**Picking up F23 specifically:** Start with T01 (bootstrap F23 evidence + advance PROGRESS); proceed sequentially through T02..T10. T03 introduces the `sessionFactory` test seam to keep unit tests off real chromium; integration test (T09) and Challenge (T10) gate on real chromium availability.

---

## F24 mid-flight section (active feature)

**Active feature:** P2-F24 — Codex Project Memory (fourth Phase 2 feature).

**Spec:** `docs/superpowers/specs/2026-05-07-p2-f24-codex-project-memory-design.md` (commit `c31b9ac`).

**Plan:** `docs/superpowers/plans/2026-05-07-p2-f24-codex-project-memory.md` (commit `19094b8`).

**User-confirmed design (Q1-Q5 = A,A,A,A,A):**
- Q1=A: Memory file discovery walks parent dirs from cwd; matches first of `helixcode.md` / `codex.md` / `AGENTS.md` (case-insensitive); stops at git root or filesystem root. PROJECT scope.
- Q2=A: Format is plain Markdown. Prepended to system prompt at every LLM call.
- Q3=A: Per-user overlay at `$XDG_CONFIG_HOME/helixcode/memory.md` (or `$HOME/.config/helixcode/memory.md` fallback) loaded AFTER project memory in the system prompt with delimiter.
- Q4=A: `/memory` slash with subcommands `status` (default) / `show` / `edit` (open in `$EDITOR`, default `vi`) / `reload`.
- Q5=A: Hot reload via existing `fsnotify` dep. 200 ms debounce. fsnotify failure degrades to slash-only reload; registry continues to work.

**Task progress:** 8 of 8 complete — F24 CLOSED.

| Task | Status | Subject                                                                                                                                          |
|------|--------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| T01  | DONE `f55b3e3` | bootstrap F24 evidence section + advance PROGRESS to F24                                                                                  |
| T02  | DONE `fd90eed` | projectmemory/types.go: Memory + Render + sentinels + MaxMemoryBytes + DiscoveryFilenames (TDD)                                          |
| T03  | DONE `99d3971` | projectmemory/loader.go: parent-walk + git-root-stop + user overlay + truncation (TDD with real tempdirs)                                |
| T04  | DONE `0760562` | projectmemory/registry.go: atomic-pointer Snapshot/Set/Reload + MemorySnapshotter interface (TDD -race)                                  |
| T05  | DONE `d740964` | projectmemory/watcher.go: fsnotify + 200 ms debounce + graceful degrade (TDD with real fsnotify)                                         |
| T06  | DONE `7af2859` | /memory slash command (status/show/edit/reload) with editor seam (TDD)                                                                    |
| T07  | DONE `40927fc` | BaseAgent SetMemoryRegistry + getSystemPrompt prepend + main.go wiring + integration test (TDD)                                          |
| T08  | DONE (this commit) | Challenge harness 5 phases A-E + close-out + push 4 remotes non-force — all 17/17 checks PASS against real tempdirs + real fsnotify |

**Anti-bluff hot zone:** §5.2 of spec — five critical patterns (case-sensitive filename mismatch / hot-reload event-but-no-update / agent-cached-old-blob / silent-truncation-no-flag / missing-file-error-instead-of-empty); each pinned by unit + integration + Challenge phase. Challenge MUST exit non-zero on byte-evidence mismatch.

**Picking up F24 specifically:** F24 is CLOSED. F25 (Plandex Plan Trees) docs landed; T01 next.

---

## F25 mid-flight section (active feature)

**Active feature:** P2-F25 — Plandex Plan Trees + Context Compaction (fifth Phase 2 feature).

**Spec:** `docs/superpowers/specs/2026-05-07-p2-f25-plandex-plan-trees-design.md` (commit `a978371`).

**Plan:** `docs/superpowers/plans/2026-05-07-p2-f25-plandex-plan-trees.md` (commit `a978371`).

**User-confirmed design (Q1-Q5 = A,A,A,A,A):**
- Q1=A: Plandex — branching plan trees + context compaction (full plandex port).
- Q2=A: Full scope — plan tree data structure, 6 tools, /plan slash, context compaction, plan verification.
- Q3=A: `.helixcode/plans/<name>.json` (project dir, per-project storage, git-trackable).
- Q4=A: 6 tools (plan_create/branch/merge/list/show/delete) + /plan slash (list/show/compact/verify). NO cobra subcommands.
- Q5=A: Context compaction via F01 AutoCompactor reuse. Threshold at 128 KB serialized JSON.

**Task progress:** 1 of 10 complete — T01 in flight.

| Task | Status | Subject                                                                                                                                          |
|------|--------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| T01  | IN FLIGHT | bootstrap F25 evidence section + advance PROGRESS to F25                                                                                  |
| T02  | —        | plantree/types.go: PlanNode + PlanTree + PlanStatus + sentinels + constants + RenderTree (TDD)                                            |
| T03  | —        | plantree/store.go: FileStore Save/Load/List/Delete + atomic writes + Store interface (TDD real-tempdirs)                                  |
| T04  | —        | plantree/operations.go: CreateTree + BranchNode + MergeNode pure-tree transforms (TDD)                                                     |
| T05  | —        | plantree/verify.go: VerifyTree 6 checks (orphans, cycles, depth, uniqueness, self-parent, count) (TDD)                                    |
| T06  | —        | plantree/compact.go: CompactTree + Summariser adapter + F01 delegation + threshold guard (TDD)                                              |
| T07  | —        | plantree/plan_tools.go: Six tools.Tool implementations (plan_create/branch/merge/list/show/delete) (TDD)                                   |
| T08  | —        | /plan slash command (list/show/compact/verify) (TDD)                                                                                        |
| T09  | —        | main.go wiring + CategoryPlan + RegisterPlanTools + builtin registration + integration test                                                |
| T10  | —        | Challenge harness 7 phases A-G + close-out + push 4 remotes non-force                                                                       |

**Anti-bluff hot zone:** §5.2 of spec — five critical patterns (plan_create writes invalid JSON / plan_branch adds orphan / plan_merge leaves Metadata unchanged / compact claims success but no byte reduction / verify reports clean on corrupted tree) + two bonus (plan_show output mismatch / shallow compact no-op). Each pinned by unit + integration + Challenge phase. Challenge MUST exit non-zero on byte-evidence mismatch.

**Picking up F25 specifically:** F25 specs + plan landed; T01 (bootstrap) is in progress. Proceed with T02 (types.go) after T01 commits.
