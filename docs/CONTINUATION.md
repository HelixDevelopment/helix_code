# HelixCode CLI-Agent Fusion — Programme Continuation Guide

**Last updated: 2026-05-08T14:30:00Z (Phase 4 closed, Phase 5 pending)
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
4. Read `docs/improvements/PROGRESS.md` §Phase 3 — Issue remediation section.
5. Continue from the next pending task in the Phase 3 remediation list.

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
| P2     | CLI agent porting              | F21–F30: 10 features ported from codex, aider, cline, plandex, etc. |
| P3     | Test infrastructure expansion  | Real-infra-only test runners, full integration matrix, remediation. |
| P4     | Anti-bluff verification pass   | Forensic sweep + Challenge-evidence audit per Article XI §11.9.     |
| P5     | End-user materials uplift      | Docs / installers / website / packaging.                            |

---

## Phase status

| Phase                          | Status       | SHA at completion          | Notes                                                         |
|--------------------------------|--------------|----------------------------|---------------------------------------------------------------|
| P0 — Foundation                | DONE         | per `05_phase_0_evidence`  | governance cascade + secret-leak remediation                  |
| P1 — claude-code (F01..F20)    | DONE         | meta `300f973` (F20 close) | 20 features, 200+ commits, all 4 remotes parity              |
| P1.5 — Foundation Cleanup      | DONE         | meta `4131bf0`             | 12 WPs, ~48 commits, deepest-first push complete             |
| P2 — CLI agent porting         | DONE         | `f821d65` (Phase 3 entry)  | 10 features (F21-F30), all tests + challenges PASS           |
| P3 — Test infra                | DONE         | `f821d65` (Phase 3 entry)  | remediation + test runner + anti-bluff verification sweep    |
| P4 — Anti-bluff audit          | DONE         | (this commit)             | forensic anti-bluff sweep per Article XI §11.9 — clean        |
| P5 — End-user materials uplift | NOT STARTED  | —                          | final phase                                                   |

---

## Repository state (snapshot @ 2026-05-08T02:00Z)

| Repo                                              | Local HEAD   | Origin status         | Notes                                                       |
|---------------------------------------------------|--------------|------------------------|-------------------------------------------------------------|
| meta-repo (HelixCode)                             | `fa4499b`    | in sync with origin    | 4 remotes: origin / github / gitlab / upstream              |
| HelixAgent                                        | `7625fbb`    | aligned with origin    | submodule; large (>500 MB)                                  |
| HelixQA                                           | `04bd45b`    | aligned with origin    | submodule                                                   |
| Challenges                                        | `79b947b`    | aligned with origin    | now has 3 remotes (origin + gitlab + upstream)              |
| Containers                                        | `a04ce66`    | aligned with origin    | submodule; governance cascaded                              |
| Security                                          | `1ea5383`    | aligned with origin    | submodule; governance cascaded                              |
| Dependencies/HelixDevelopment/LLMsVerifier        | `a3f2c4b`    | aligned with origin    | canonical pin; HelixAgent has divergent transitive view     |
| Dependencies/HelixDevelopment/LLMOrchestrator     | `9bd899a`    | aligned with origin    |                                                             |
| Dependencies/HelixDevelopment/LLMProvider         | `efad22b`    | aligned with origin    |                                                             |
| Dependencies/HelixDevelopment/VisionEngine        | `ac96ddb`    | aligned with origin    |                                                             |
| Dependencies/HelixDevelopment/DocProcessor        | `1d3a624`    | aligned with origin    |                                                             |
| MCP-Servers                                       | `4503e2d`    | aligned with origin    | third-party (modelcontextprotocol/servers)                  |

Meta-repo remotes (4):
- `origin` — fetch from `HelixDevelopment/HelixCode` (GitHub) / push to `HelixDevelopment/Helix-CLI` + GitLab `helixdevelopment1/HelixCode`
- `github` — `HelixDevelopment/HelixCode` (GitHub)
- `gitlab` — `helixdevelopment1/HelixCode` (GitLab)
- `upstream` — `HelixDevelopment/HelixCode` (GitHub)

---

## Active phase in flight

**Phase 4 — Anti-bluff audit.** Forensic sweep + Challenge-evidence audit per Article XI §11.9.

### Phase 4 objectives:

- Run forensic anti-bluff sweep across all Go source files (HelixCode `internal/` and `cmd/`)
- Verify every Challenge has positive runtime evidence (not absence-of-error PASS)
- Audit CONST-035 compliance (Quality + Completion + Usability)
- Close out any remaining Phase 3 deferred items that are code-actionable

### Phase 3 close-out summary:

Phase 3 closed on 2026-05-08. Code-actionable remediation items resolved:
- LLMsVerifier build fixed (go.mod replace path)
- HelixQA build fixed (4 replace directives)
- 6 internal/server tests now PASS (0 FAIL)
- Full test suite 99/99 packages ok, 0 FAIL
- HelixAgent submodules populated, individual builds/tests PASS
- Containers build PASS (go build ./... exits 0)
- All previous resolved items (F23 test, F02 wiring, Challenges mirrors, governance cascade, anti-bluff verification, duplicate submodule)

Items still pending (pre-existing, operator-blocked or out of scope):
- HelixAgent DebateOrchestrator gap (repo not on GitHub)
- Historical credential rotation (operator)
- Stale cli_agents pins (13, out of scope per spec §1.3 N2)
- 23 snake_case renames (deferred, build-path-breaking)
- Codex Multimodal & Cline Computer Use (feature work not ported)

### Remaining pre-existing issues (Phase 3 backlog):

- **HelixAgent build** — IMPROVED: submodules populated, `./cmd/helixagent/...` builds and tests pass. Wildcard `./...` blocked by single stale DebateOrchestrator replace (repo not on GitHub).
- **Containers build** — RESOLVED: `go build ./...` exits 0.
- **Historical credential leaks** — operator rotation required
- **Stale cli_agents pins (13)** — HelixAgent submodule SHAs expired upstream
- **23 snake_case renames** — build-path-breaking, deferred
- **Codex Multimodal, Cline Computer Use** — not yet ported

---

## Phase 2 completed features (F21-F30)

### P2-F21 — Codex Approval Modes (CLOSED)

- Spec: `docs/superpowers/specs/2026-05-06-p2-f21-codex-approval-modes-design.md`
- Plan: `docs/superpowers/plans/2026-05-06-p2-f21-codex-approval-modes.md`
- Commits: T01 `a7a349f` → T09 close-out `2781c1a` (sub `f2ea964`)
- 9 tasks: approval/types.go + selector + manager + tool interface + slash + wiring + Challenge + close-out
- First Phase 2 feature shipped. All 4 remotes pushed non-force.

### P2-F22 — Aider Git Auto-Commit Per Change (CLOSED)

- Spec: `docs/superpowers/specs/2026-05-06-p2-f22-aider-git-auto-commit-design.md` (`8be7fba`)
- Plan: `docs/superpowers/plans/2026-05-06-p2-f22-aider-git-auto-commit.md` (`b4f217d`)
- Commits: T01 `550be34` → T09 close-out `bab7ebc`
- 9 tasks: types + git wrapper + summariser + committer + registry hook + slash + wiring + Challenge + close-out
- One commit per accepted edit; LLM-summarised; Co-Authored-By trailer; default ON.

### P2-F23 — Cline Browser Tool (CLOSED)

- Spec: `docs/superpowers/specs/2026-05-07-p2-f23-cline-browser-tool-design.md` (`83d401d`)
- Plan: `docs/superpowers/plans/2026-05-07-p2-f23-cline-browser-tool.md` (`bc5fd3e`)
- Commits: T01 `64e499b` → T10 close-out `f39f686`
- 10 tasks: chromedp-based 6-tool suite (navigate/snapshot/click/type/screenshot/close) + /browser slash
- 7/7 integration tests PASS against real chromium. Legacy tools renamed to browser_legacy_*.

### P2-F24 — Codex Project Memory (CLOSED)

- Spec: `docs/superpowers/specs/2026-05-07-p2-f24-codex-project-memory-design.md` (`c31b9ac`)
- Plan: `docs/superpowers/plans/2026-05-07-p2-f24-codex-project-memory.md` (`19094b8`)
- Commits: T01 `f55b3e3` → T08 close-out `40927fc`
- 8 tasks: Memory + loader + registry + watcher + /memory slash + BaseAgent + Challenge + close-out
- 17/17 checks PASS against real tempdirs + real fsnotify.

### P2-F25 — Plandex Plan Trees + Context Compaction (CLOSED)

- Spec: `docs/superpowers/specs/2026-05-07-p2-f25-plandex-plan-trees-design.md` (`a978371`)
- Plan: `docs/superpowers/plans/2026-05-07-p2-f25-plandex-plan-trees.md` (`a978371`)
- Commits: T01 `c744a27` → T10 close-out `ff9097d`
- 10 tasks: PlanNode/PlanTree types + FileStore + operations + verify + compact + 6 tools + /plantree slash + wiring + Challenge + close-out
- 35/35 checks PASS. Context compaction via F01 AutoCompactor reuse (128 KB threshold).

### P2-F26 — Openhands Workspace + Task Planner (CLOSED)

- Spec: `docs/superpowers/specs/2026-05-07-p2-f26-openhands-workspace-design.md` (`fbfea77`)
- Plan: `docs/superpowers/plans/2026-05-07-p2-f26-openhands-workspace.md` (`fbfea77`)
- Commits: T01 `613b204` → T06+T07 `5cdc6e7` → T08 `b7572c0` + close-out `5eee71c`
- 8 tasks: workspace types/manager + workspace tools + planner types/executor + planner tools + /openhands slash + wiring + Challenge + close-out
- 10/10 checks PASS. Container-based workspaces via Containers submodule. CONST-045 introduced.

### P2-F27 — Aider Voice Input + Repo-Map (CLOSED)

- Spec: `docs/superpowers/specs/2026-05-07-p2-f27-aider-voice-input-design.md` (`e702a85`)
- Plan: `docs/superpowers/plans/2026-05-07-p2-f27-aider-voice-input.md` (`e702a85`)
- Commits: T01 `0dc01b3` → T02-T07 `29218cc` → T09 `8e89c48` + close-out `2ecefde`
- Voice input via speech-to-text + repo-map integration with F24 project memory.
- 12/12 checks PASS in Challenge harness.

### P2-F28 — Kilo-code AST-Aware Refactoring (CLOSED)

- Spec: `docs/superpowers/specs/2026-05-07-p2-f28-kilocode-refactoring-design.md` (`13ece51`)
- Plan: `docs/superpowers/plans/2026-05-07-p2-f28-kilocode-refactoring.md` (`13ece51`)
- Commits: T01 bootstrap `13ece51` → CLOSED `95efa82`
- Tree-sitter-based callgraph + rename + impact analysis + refactoring tools.

### P2-F29 — Roo-code Full Port (CLOSED)

- Spec: `docs/superpowers/specs/2026-05-07-p2-f29-roocode-port-design.md` (`beeebe4`)
- Plan: `docs/superpowers/plans/2026-05-07-p2-f29-roocode-port.md` (`beeebe4`)
- Commits: T01 bootstrap `beeebe4` → CLOSED `acf158f`
- Full Roo-code feature parity port.

### P2-F30 — Continue IDE Integration (CLOSED) — FINAL Phase 2 feature

- Spec: `docs/superpowers/specs/2026-05-07-p2-f30-continue-ide-design.md` (`2aa3901`)
- Plan: `docs/superpowers/plans/2026-05-07-p2-f30-continue-ide.md` (`2aa3901`)
- Commits: T01 bootstrap `2aa3901` → CLOSED `78aaace`
- **PHASE 2 COMPLETE.** All 10 features (F21-F30) shipped.

---

## Known issues / bugs / failures (out of scope but tracked)

### Pre-existing (from before P1.5)

- **HelixAgent build FAIL:** IMPROVED — 100+ submodules now populated.
  `go build ./cmd/helixagent/...` PASS. `go test ./cmd/helixagent/...` and
  `go test ./internal/...` both PASS. Only `DebateOrchestrator` (repo not
  on GitHub) blocks wildcard `./...`.
- **HelixQA build FAIL:** RESOLVED — 4 replace directives fixed to point to
  `Dependencies/HelixDevelopment/`. `go build ./...` and `go mod tidy` both
  pass clean. Tests: all PASS.
- **LLMsVerifier `make build` FAIL:** Makefile points at non-existent `./cmd`;
  `go build ./...` FAIL — missing go.sum for kafka-go, rabbitmq, etc.
  RESOLVED — fixed go.mod replace path `../../Challenges` → `../../../Challenges`.
  `go build ./...` pass clean.
- **Containers `make build`:** RESOLVED — `go build ./...` exits 0. Build
  passes clean.
- **`examples/multi_agent_system` MockLLMProvider drift** (similar to F21-T03
  fix; not on critical path).
- **`applications/desktop` link FAIL on host:** missing X11/Xcursor.h
  (environment issue, not code).
- **6 internal/server tests:** RESOLVED — now 0 FAIL (fresh `-count=1` run).

### Phase 1.5 deferred items

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
  (directory absent on disk; only declaration remains).

### Constitutional debt (open since P0)

- **LLMsVerifier dual-pin divergence** (P0-04): canonical pin in
  `Dependencies/HelixDevelopment/LLMsVerifier` is one commit ahead of the
  transitive HelixAgent view. `make verify-foundation` exits 2 until
  resolved or explicitly waived via `VERIFY_FOUNDATION_WARN_ONLY=1`.
- **Historical SSH key + helix.security.json leaks** (P0-T08.5): material is
  immortal in git history. Mitigated; rotation required by operator.
- **SonarQube + Snyk live-scan deferral** (P0-T08.7): infrastructure wired,
  awaiting credential rotation by operator.

### Phase 3 remaining items (carried forward, non-blocking for Phase 4)

- **HelixAgent build** — IMPROVED: submodules populated, `./cmd/helixagent/...` builds and tests pass. Wildcard `./...` blocked by single stale DebateOrchestrator replace (repo not on GitHub).
- **Containers build** — RESOLVED: `go build ./...` exits 0.
- **Historical credential leaks** — operator rotation required
- **Stale cli_agents pins (13)** — HelixAgent submodule SHAs expired upstream
- **23 snake_case renames** — build-path-breaking, deferred
- **Codex Multimodal, Cline Computer Use** — not yet ported

### CONST-045 — No Hardcoded Distribution Hosts
ALL container distribution targets SHALL be configured exclusively through `CONTAINERS_REMOTE_HOST_N_*` env vars in `Containers/.env` (N=1..100; iteration stops at first absent `_NAME`; the Containers module `pkg/envconfig/parser.go` is the authoritative loader). The .env file is the sole source of truth for host enrolment — no host is hardcoded in HelixCode source, tests, challenges, or governance documents. Every non-unit test run and every production deployment MUST use whichever hosts are currently configured when `CONTAINERS_REMOTE_ENABLED=true`. Adding, removing, or modifying a host means editing `Containers/.env`; no code change is required. The CURRENT configured set can be audited with `grep '^CONTAINERS_REMOTE_HOST_' Containers/.env`; at the time of this rule's introduction (2026-05-07) the configured hosts were `thinker.local`, but the rule applies to whatever set is in `.env` at any future point (N>=1). Direct `docker`/`podman` commands, manual container start/stop, and ad-hoc remote hosts outside the `.env` mechanism are strictly prohibited.

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
   (Challenges now has 3 remotes: origin + gitlab + upstream).
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

### Picking up Phase 3 work

If Phase 3 is the active phase when you resume:
1. Verify state: `git log --oneline -3` should show the latest Phase 3 commits.
2. Read `docs/improvements/PROGRESS.md` §Phase 3 — Issue remediation section.
3. Continue with the next pending remediation item from the Phase 3 remaining list.
4. Run `make test` to verify current state before making changes.

### Picking up Phase 4 work

If Phase 4 is the active phase when you resume:
1. Verify state: `git log --oneline -3` should show the latest Phase 4 commits.
2. Read `docs/improvements/PROGRESS.md` §Phase 4 section.
3. Continue the forensic anti-bluff sweep per Article XI §11.9 across:
   - All Go source files in `internal/` and `cmd/`
   - All Challenge harness files in `tests/`
   - Any file containing `TODO`, `placeholder`, `simulated`, or `for now` comments
4. Verify every PASS has positive runtime evidence (not absence-of-error).
5. Run `make test` to verify current state before making changes.
6. Once Phase 4 is complete, advance to Phase 5 (end-user materials).

### Picking up Phase 5 work

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
- `Last updated: 2026-05-08T02:00:00Z (Phase 3 — remediation + test infra)
- `Active phase` here vs `Current focus` in `docs/improvements/PROGRESS.md`.
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
| 2026-05-06     | T09 close-out | F21 (Codex Approval Modes) CLOSED — first Phase 2 feature shipped. |
| 2026-05-06     | F22 docs      | F22 (Aider Git Auto-Commit Per Change) spec + plan landed. |
| 2026-05-07     | F23 docs      | F23 (Cline Browser Tool) spec + plan landed. |
| 2026-05-07     | F24 docs      | F24 (Codex Project Memory) spec + plan landed. |
| 2026-05-07     | F25 docs      | F25 (Plandex Plan Trees + Context Compaction) spec + plan landed. |
| 2026-05-07     | F26 docs      | F26 (Openhands Workspace + Task Planner) spec + plan landed. |
| 2026-05-07     | F27 docs      | F27 (Aider Voice Input + Repo-Map) spec + plan landed. |
| 2026-05-07     | F28 docs      | F28 (Kilo-code AST-Aware Refactoring) spec + plan landed. |
| 2026-05-07     | F29 docs      | F29 (Roo-code Full Port) spec + plan landed. |
| 2026-05-07     | F30 docs      | F30 (Continue IDE Integration) spec + plan landed. |
| 2026-05-07     | Phase 3 entry | Phase 2 CLOSED (F21-F30 complete). Phase 3 started — remediation + test infra expansion. |
| 2026-05-08     | Full sync     | F26-F30 close-out sections added; Phase 3 active section added; repo SHAs updated; known issues synced with PROGRESS.md. |
| 2026-05-08     | Phase 3 sync  | LLMsVerifier build, HelixQA build, 6 internal/server tests, full test suite — all RESOLVED. Remaining list pruned. CONTINUATION.md synced with PROGRESS.md. |
| 2026-05-08     | Phase 3 close | Phase 3 CLOSED. All code-actionable remediation resolved. HelixAgent submodules populated, Containers build fixed. Phase 4 started (anti-bluff audit). |
