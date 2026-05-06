# HelixCode CLI-Agent Fusion — Programme Continuation Guide

**Last updated:** 2026-05-06T19:00:00Z by meta-repo HEAD `5ef13b8` (will roll forward on this commit)
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
| P2 — CLI agent porting         | IN PROGRESS  | —                          | F21 mid-flight (6 of 9 tasks done)                            |
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

**P2-F21 — Codex Approval Modes**

- Spec: `7128289` (`docs/superpowers/specs/2026-05-06-p2-f21-codex-approval-modes-design.md`)
- Plan: `bbb61de` (`docs/superpowers/plans/2026-05-06-p2-f21-codex-approval-modes.md`)
- Goal: Codex-compatible 4-mode approval system (suggest / auto-edit /
  full-auto / dangerously-bypass) with CLI flag > env > config > default
  precedence; per-tool `RequiresApproval()` gate; F14 sandbox coupling for
  full-auto; F02 final-deny authority retained; `/approval` slash + atomic-
  pointer runtime mode swap.

**Tasks completed (6 of 9):**

| Task | Commit       | Subject                                                                        |
|------|--------------|--------------------------------------------------------------------------------|
| T01  | `a7a349f`    | bootstrap Phase 2 evidence + advance PROGRESS to F21                           |
| T02  | `9c2664d`    | approval/types.go (ApprovalMode + ApprovalLevel + Decision + sentinels + ModeDescriptors) |
| T03  | `0d655d8`    | approval/selector.go (flag > env > config > default precedence)                |
| T04  | `5ef13b8`    | approval/manager.go (4×4 matrix gate + F02/F14/F19 integration)                |
| T05  | `19bffce`    | tools.Tool gains `RequiresApproval()`; spec §3.6 explicit-override applied to all ~38 tool impls + DefaultLevelEdit safe-default |
| T06  | (this commit)| `/approval` slash command (status/set/show) — ApprovalInspector seam + ParseMode + ModeDescriptors render |

**Tasks remaining (3 of 9):**

- **T07** — `cmd/cli/main.go` wiring + `--approval` pflag + registry hook + integration test (TDD).
- **T08** — Challenge harness 5 phases:
  1. `suggest-deny` (suggest mode rejects fs_edit without prompt)
  2. `auto-edit-prompt` (auto-edit shows F19 prompt; user denies)
  3. `full-auto-sandbox` (full-auto routes fs_edit through F14 sandbox)
  4. `runtime-change` (atomic-pointer swap mid-session via /approval set)
  5. `F02-final-deny` (F02 PolicyEngine has final say even in full-auto)
- **T09** — Feature 21 close-out + push to all 4 meta-repo remotes (non-force).

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
1. Verify state: `git log --oneline -5` should show the T06 close-out at HEAD
   (the `/approval` slash command commit on `main`).
2. Read `docs/superpowers/plans/2026-05-06-p2-f21-codex-approval-modes.md` end
   to end.
3. Continue at **T07** (`cmd/cli/main.go` wiring + `--approval` pflag +
   registry hook + integration test).

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
