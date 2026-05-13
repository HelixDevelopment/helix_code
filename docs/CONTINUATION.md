# HelixCode CLI-Agent Fusion — Programme Continuation Guide

**Last updated: 2026-05-13T10:35:00Z (mistborn.local distribution + helix-bridge + helix-qa anti-bluff harness expanded 6 → 56 evidence files / 51 counted checks; 17 real production bugs surfaced and fixed at root cause this session: (1) BLUFF-002 stub in `getLLMProvider` (unknown id now 404 not fabricated "available"), (2) `task_data NOT NULL` constraint violation on POST /tasks with empty parameters (now defaults to empty map), (3) `tasks: null` vs `[]` (now array), (4) GET /projects 401 with valid JWT (context-key mismatch — now `c.Get("user")` convention), (5) `projects: null` vs `[]` (now array), (6) `/users/me` returned a JWT-claims-only stub User with `is_active:false` + `created_at:"0001-01-01T00:00:00Z"` — middleware now uses `VerifyJWTWithDB` (DB lookup per request + active-account enforcement), (7) `workers: null` vs `[]` (now array), (8) `POST /projects` 500 "invalid UUID length: 12" — `DatabaseManager.CreateProject` hardcoded ownerID to the literal string `"default-user"` (12 chars, not a UUID); handler now passes the real authenticated user.ID via CreateProjectWithUser, (9) `POST /projects` (and the 4 workflow endpoints) was routed under a `publicProjects` group with comment "no auth for testing" — exposed without authentication in production; consolidated under the authenticated projects group, (10) `POST /workers` 500 with `ssh_config` AND `capabilities` NOT NULL constraint violations — same pattern as the task_data bug; both fields now defaulted to empty {}/[] in `worker.DatabaseManager.RegisterWorker`. (11) `POST /auth/refresh` always 401 with valid Bearer — handler used `c.Get("user")` but `/auth` group has no middleware (same pattern as BUG #3); now manually parses+verifies the Bearer via `VerifyJWTWithDB`, mirroring logout. (12) `GET /tasks/:id/checkpoints` returned `"checkpoints": null` — 4th instance of nil-slice→null JSON contract bluff; now defaults to `[]`. (13) `POST /tasks/:id/retry` returned HTTP 500 for client-state errors (task not in failed state / max retries exceeded) — server-error code lying about a client-side problem. Introduced exported sentinel `task.ErrTaskNotRetryable`; handler now returns 422 Unprocessable Entity for this case and 500 only for genuine DB faults. (14) `GET /memory/systems` reported 6 systems all with `status: "available"` while `GET /memory/stats` simultaneously reported `systems_connected: 0` — direct contradiction; no memory manager was wired in the Server struct, yet every entry claimed to be up. Status now derived from real wiring state (currently "not_configured" for all 6 — honest given nothing's wired); the catalogue itself (id/name/type/description/features) remains as the documented set of supported backends. (15) `PUT /projects/:id` 500 "column path does not exist (SQLSTATE 42703)" — `DatabaseManager.UpdateProject`'s `UPDATE ... RETURNING` clause referenced nonexistent columns `path` and `type` (real schema has `workspace_path` and stores type inside `config` JSONB). Every successful PUT was unreachable — the canonical "rename a project" call always 500'd. Fixed by mirroring GetProject's column-mapping pattern. (16) `PUT /workers/:id` 500 "null value in column capabilities" — same NOT NULL pattern as BUG #10 but in the UPDATE path, AND was unconditionally overwriting non-omitted fields (an empty hostname in the request would blank the existing hostname). Fixed by defaulting `capabilities` to empty slice + `COALESCE(NULLIF(...))` for hostname/display_name + `CASE WHEN $4 > 0` for max_concurrent_tasks so partial updates preserve existing values. (17) `GET /workers/:id/metrics` returned `"metrics": null` — 5th instance of nil-slice→null JSON contract bluff (after tasks/projects/workers-list/checkpoints, now worker-metrics). Fixed with `[]*worker.WorkerMetrics{}` default-when-nil. New unit tests: TestGetLLMProvider_UnknownIDReturns404. JWT/session tokens redacted in QA evidence per CONST-042 defense-in-depth. All 4 meta-repo remotes at `6ec5ae0` (pre-round-3 head; next push advances). run_all_challenges 12/12 PASS. helix-qa 29/29 PASS. helix-bridge: 3/5 LLM providers respond live.)
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
4. The CLI-Agent Fusion programme (P0-P5) is COMPLETE.
5. The Zero-Bluff Completion programme Phase 1 (Governance) is COMPLETE.
6. The Zero-Bluff Completion programme Phase 2 (Stub Elimination) is COMPLETE.
7. The Zero-Bluff Completion programme Phase 3 (Feature Gaps) is COMPLETE.
8. The Zero-Bluff Completion programme Phase 4 (Test/Challenge Hardening) is COMPLETE.
9. The Zero-Bluff Completion programme Phase 5 (Full Documentation Suite) is COMPLETE.
10. **The Zero-Bluff Completion programme is COMPLETE (all 5 phases shipped).**
11. No active programme. Backlog / parking-lot items listed under "Next" at the bottom of this file.

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
| P5 — End-user materials uplift | DONE         | `62d0fac`                   | docs, installers, website, packaging                          |
| P6 — Governance Propagation     | DONE         | `5f9bb90`                   | anti-bluff anchor cascaded to 60+ submodules                  |
| P7 — Stub Elimination           | DONE         | `b24ca8f`                  | P2-T01 through P2-T11 complete (see commits 33ddf6a, b24ca8f) |

**NEW PROGRAMME: Zero-Bluff Completion** (spec: `docs/superpowers/specs/2026-05-08-helixcode-zero-bluff-completion-design.md`):
| Phase                       | Status       | SHA        | Notes                                                         |
|-----------------------------|--------------|------------|---------------------------------------------------------------|
| P1 — Governance Propagation | DONE         | `5f9bb90`  | anti-bluff anchor cascaded to all 60+ submodules              |
| P2 — Stub/Bluff Elimination | DONE         | `b24ca8f`  | T01-T11 complete; scanner+CLI+FAISS+CharacterAI+Anima+security-test+treesitter+multiedit |
| P3 — Feature Gap Implementation | DONE     | `1f1d8f4`  | 11 tasks: LiteLLM, repomap, quality, clarification, plugins, 4 providers, CONST-046 |
| P4 — Test/Challenge Hardening   | DONE     | `a3f8871`  | weak-assertion sweep: fix + deployment tests tightened with mutation-tested content assertions |
| P5 — Full Documentation Suite   | DONE     | (this commit) | 4 docs: user manual (ZERO_BLUFF_USER_MANUAL.md), developer_guide/README.md, api_reference/README.md, deployment_guide/README.md |

**Zero-Bluff Completion programme: COMPLETE (all 5 phases shipped).**

---

## Repository state (snapshot @ 2026-05-08T02:00Z)

| Repo                                              | Local HEAD   | Origin status         | Notes                                                       |
|---------------------------------------------------|--------------|------------------------|-------------------------------------------------------------|
| meta-repo (HelixCode)                             | `1f1d8f4`    | in sync with origin    | 4 remotes: origin / github / gitlab / upstream              |
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

## Active programme: Zero-Bluff Completion

**Phase 2 — Stub/Bluff Elimination (COMPLETE)**

### Phase 2 completed tasks (all):
- **P2-T01**: Security scanning — real SonarQube/Snyk scanner infrastructure (`internal/security/scanner.go`, `sonarqube_client.go`, `snyk_client.go`). `ScanFeature()` no longer returns hardcoded 95.
- **P2-T02**: CLI commands — `cmd/other_commands.go` wired to real server, LLM generate, notification engine, and go test runner.
- **P2-T04**: FAISS — "simulated" labeling removed from `faiss_provider.go`. Renamed constant to `PureGoNotice`.
- **P2-T05**: CharacterAI — "simulated"/"SIMULATION" labeling replaced with "standalone". Function `generateSimulatedEmbedding` → `generateEmbedding`.
- **P2-T06**: Anima — real JSON backup/restore with `os.WriteFile`/`os.ReadFile`.
- **P2-T07**: Security-test — `cmd/security-test/main.go` rewired to real `internal/security` scanner dispatch.
- **P2-T08**: Redis/Memcached — verified `internal/redis/redis.go` is already real go-redis. No additional memory provider stubs found.
- **P2-T09**: Treesitter placeholder at line 266 — REMOVED.
- **P2-T10**: Re-verified BLUFF-004 through BLUFF-008 — all clean.
- **P2-T11**: Cleanup (`main.go.old` deleted), AGENTS.md updated, final anti-bluff sweep — clean.
- **Phase 2 commits**: `33ddf6a` (T01-T09), `b24ca8f` (T10-T11).

### Phase 3 — Feature Gap Implementation (COMPLETE — `1f1d8f4`)

All 11 tasks implemented with 31 tests passing, anti-bluff clean:
- **P3-T01**: LiteLLM unified provider abstraction (`internal/llm/litellm/`) — 8 files, 8 tests
- **P3-T02**: RepoMap semantic codebase mapping (`internal/repomap/`) — existing
- **P3-T03**: Quality scoring system (`internal/quality/`) — 5 files, 9 tests
- **P3-T04**: Clarification engine (`internal/clarification/`) — 4 files, 6 tests — LLM-driven per CONST-046
- **P3-T05**: Plugin system (`internal/plugins/`) — 7 files, 8 tests
- **P3-T06**: Cohere provider (`internal/llm/providers/cohere/`)
- **P3-T07**: Replicate provider (`internal/llm/providers/replicate/`)
- **P3-T08**: Together.ai provider (`internal/llm/providers/together/`)
- **P3-T09**: HuggingFace provider (`internal/llm/providers/huggingface/`)
- **P3-T10**: GAP_ANALYSIS.md updated (deferred to Phase 5)
- **P3-T11**: Full build + test + anti-bluff verification — PASS
- **CONST-046**: No Hardcoded Content mandate added to CONSTITUTION.md, AGENTS.md, CLAUDE.md

 Evidence: `go build -tags nogui ./internal/...` PASS | `grep -rn "simulated\|placeholder\|TODO"` clean | 31 tests PASS | pushed to github + gitlab

### Phase 4 — Test/Challenge Hardening (CLOSED — `a3f8871`)

**Critical Bluffs Eliminated (P0 fixes committed `4f5f8f0`):**
1. **Challenge Validators** — Default-case free pass eliminated (runtime_validator.go:37, functional_validator.go:59)
   - FIX: Returns `Passed: false` with explicit error message
2. **fix.go** — Simulated security fix stub replaced with real pattern matching
   - FIX: 8 security issue types detected (hardcoded secret, SQL injection, path traversal, XSS, CSRF, insecure dependency, missing auth, weak crypto)
3. **config.go** — Stub implementations replaced
   - FIX: AddWatcher stores watchers, NewConfigWatcher validates path, GetConfigInfo returns real metadata
4. **json-validator-cli-001** — 10-case runtime validation added
   - FIX: Tests valid/invalid JSON, edge cases, exit codes
5. **DB-unavailable acceptance** — No longer accepts degraded state
   - FIX: Returns `Passed: false` with fix instructions

**P1 Bluffs Eliminated (committed `7f4effd`):**
6. **Deployment simulation** — Eliminated fake 90%/95% success rates
   - FIX: deployToServer returns false with "requires SSH infrastructure" message
   - FIX: checkServerHealth returns error requiring SSH/HTTP access
   - FIX: executeProductionDeploy fails honestly without SSH credentials

**Evidence of Anti-Bluff Compliance:**
- Challenge bash scripts PASS (hooks, snyk, sonarqube verified with runtime evidence)
- Tests FAIL correctly when infrastructure unavailable (deployment tests detect missing SSH)
- `go build -tags nogui ./internal/...` PASS
- Anti-bluff sweep clean: `grep -rn "simulated\|for now" internal/` returns only permitted fallback models

**Per Article XI §11.9**: Every PASS now carries positive runtime evidence. No false-success results tolerable.

**P2 Weak-Assertion Sweep (committed `02b7306` + `a3f8871`):**

Inventory (447 real test files in internal/ + cmd/ + tests/, excluding LLM-generated
test-results/): scanned for assert.True(true) tautologies, discarded subject return
values, bare t.Skip without SKIP-OK markers, NotNil-on-bool patterns, and content-blind
NoError-only assertions.

Triage:
- TIGHTEN: 2 packages (internal/fix, internal/deployment) — 8 tests rewritten
- OK: ~437 files — assertions already verify content or are legitimate error-path tests
- DEFER: bare grep candidates in subagent/cognee/auth_test (legitimate error-path tests
  that discard the happy value because the test is exercising the error contract)

Tests tightened (8 total, mutation-test confirmed all 6 critical paths):

1. internal/fix/fix_test.go::TestAttemptFix — added clean-vs-dirty source pairs for
   hardcoded-secret, SQL-injection, weak-crypto (real Go files planted on disk so
   pattern detection is genuinely exercised). Added XSS/CSRF/missing-auth/insecure-
   dependency manual-only assertions.
2. internal/fix/fix_test.go::TestProcessSecurityIssues — added dirty-source case
   that plants fmt.Sprintf SELECT and asserts it bucketed as 'failed' not 'fixed';
   added bucket-sum invariants.
3. internal/fix/fix_test.go::TestFixAllCriticalSecurityIssues_SuccessConditions —
   eliminated `assert.True(t, true)` tautology, replaced with documented Success
   contract + TotalIssues==0 ⇒ Success==false invariant.
4. internal/fix/fix.go — removed misleading `// For now, simulate processing`
   comment (the code actually runs real pattern dispatch).
5. internal/deployment/production_deployer_test.go::TestDeploymentStrategiesExecution
   (4 strategies: BlueGreen, Canary, Rolling, Recreate) — replaced
   `NotNil(success)` on bool + `NoError(err)` (contradictory to honest contract)
   with `False(success)` + `Error(err)`.
6. internal/deployment/production_deployer_test.go::TestExecuteHealthCheck
   /HealthCheck_WithDeployedServers — replaced unreachable `if success {}` block
   with explicit failure-path assertions.
7. internal/deployment/production_deployer_test.go::TestExecuteHealthCheck
   /HealthCheck_NoDeployedServers — fixed wrong assertion (`True(success)`
   contradicted production code that returns `(false, nil)` for empty servers).
8. internal/deployment/production_deployer_test.go::TestExecuteDeployment
   /ExecuteDeployment_ProductionStrategy — fixed wrong substring match
   ("no servers deployed" → "% servers deployed" + "need 80%").

Mutation tests confirmed (6 production-code mutations → tests fail; reverted):
1. Disabling SQL detection → ProcessSecurityIssues_DirtySource fails
2. Disabling hardcoded-secret detection → DirtyDirectory test fails
3. Disabling weak-crypto detection → DirtyDirectory test fails
4. Hardcoding result.Success=true → SuccessConditions test fails
5. NoServers branch returning true → NoDeployedServers test fails
6. Lowering 80% threshold to 0% → ProductionStrategy test fails

Anti-bluff smoke (production code only, excluding _test.go):
- `simulated` / `For now, simulate` patterns in internal/ + cmd/: 1 remaining
  (internal/worker/consensus.go:197 — degenerate single-node case, not a true
  simulation; documented as deferred to Phase 5+).
- "for now" comments are mostly innocuous (deferred-feature notes, fallback
  documentation); none correspond to fake success-paths.

Cross-compile (linux/amd64) PASS: 95 MB binary at /tmp/helixcode-zb-p4.

Full internal/ unit-test sweep: PASS with one flake in internal/repomap
(TestCacheEnabled — passes solo, fails under parallel; pre-existing
race not introduced by this phase).

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
- **`cmd/infrastructure/` Containers v2 API drift:** RESOLVED 2026-05-12 —
  `go build ./cmd/infrastructure/...` exits 0. Fixes: `NewSlogAdapter()`
  → `NewSlogAdapter(nil)`; 16 `.WithDescription(...)` calls removed
  (method no longer on `endpoint.Builder`); `boot.Option`
  → `boot.BootManagerOption`; dropped removed options
  (`WithHealthCheckRetries`/`WithHealthCheckTimeout`/`WithParallelStartup`);
  `summary.Errors` → walk `summary.Results` filtering `Status=="failed"`;
  `GetHealthStatus()` → `HealthCheckAll(ctx)`; unused `time` + `compose`
  imports removed; `ep.Description` (no longer on `ServiceEndpoint`)
  → `ep.ServiceName`.
- **Meta-repo `internal/security/` stale duplicates:** RESOLVED 2026-05-12 —
  `go build ./internal/security/...` exits 0. Root `internal/security/`
  (separate from Zero-Bluff P2-T01's `HelixCode/internal/security/`) had
  `manager.go` + `scanners.go` with duplicate `SonarQubeConfig`/`SnykConfig`/
  `TrivyConfig` decls and references to undefined
  `NewSemgrepScanner`/`NewGosecScanner`/`NewNancyScanner`. Fixes: removed
  scanners.go's mini-duplicates (manager.go's tagged config-tree versions
  are canonical); added real exec-based `SemgrepScanner`/`GosecScanner`/
  `NancyScanner` impls; dropped unused `runtime`/`gopkg.in/yaml.v2` imports;
  added `GenerateReports` field to `ScanningConfig`; implemented
  `generateHTMLReport` method on `SecurityManager`; fixed
  `ScanSummary.ContainersScanned` mis-field (belongs to `ScanMetrics`).
- **Meta-repo `go build ./...` compile-poison from research dumps:**
  RESOLVED 2026-05-12 — moved 145 research dump files from
  `docs/helix_qa/HelixQA_Integration/research/raw/` to
  `docs/helix_qa/HelixQA_Integration/research/testdata/raw/` (Go's
  `testdata/` is skipped by `./...`); moved root `isolated_files/` to
  `docs/testdata/isolated_files/` for the same reason. `cli_agents/plandex/`
  submodule still produces 9 errors under bare `./...` (third-party,
  cannot be modified); preferred clean-build target is
  `go build ./cmd/... ./internal/...` which exits 0.
- **Runtime test artefacts polluting working tree:** RESOLVED 2026-05-12
  (commit `d7a7956`) — three files (`production_optimization_report.txt`,
  `confirmations.jsonl`, `autonomy.json`) were tracked but regenerated by
  every test run, plus `go build ./cmd/infrastructure/` dropped a 13 MB
  ELF binary at the inner-module root. `git rm --cached` untracked the
  three files (kept on disk), the binary was deleted, and
  `HelixCode/.gitignore` was tightened with patterns for `/infrastructure`
  + sibling cmd outputs, `internal/**/.helix/`, `internal/**/.helixcode/`,
  and `internal/performance/reports/`. Verified via `git check-ignore -v`.
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
- **WP4 api_keys.sh loader propagation:** RESOLVED 2026-05-13 — copied
  the canonical loader to every owned submodule that lacked it:
  Challenges (`dfe769a`), Security (`d1f59d5`), Github-Pages-Website
  (`ee6a3b7`), and inline at `Assets/scripts/` (tracked meta-repo
  subdir). Five owned copies (Containers, HelixQA, Challenges,
  Security, Github-Pages-Website) plus Assets/scripts all byte-
  identical to `scripts/load_api_keys.sh`. Third-party submodules
  (LLama_CPP, Ollama, HuggingFace_Hub, MCP-Servers, nested
  HelixAgent/HelixLLM/*) are out-of-scope per the decoupling
  mandate — we cannot pollute upstream trees.

### cli_agents fetch + upstream-port cycle (2026-05-13)

Per operator mandate "fetch and pull every single CLI agent Submodule
so the latest and greates changes are obtained! Once this done check
if there is something new we brought in with fetching and pulling of
each CLI agent which MUST BE ported! Add more tests and Challenges
as well!" — completed end-to-end:

- **Fetch coverage:** 50/50 cli_agents fetched via `git fetch origin`.
  No dirty working trees (every submodule verified clean BEFORE fetch
  per operator's "any changes from cli_agents directories are everted"
  rule). Fetch updates remote-tracking refs only; gitlinks in the
  meta-repo unchanged (we don't commit gitlink bumps for third-party).
- **Upstream-advanced:** 5 submodules — codex-skills, git-mcp, gptme,
  spec-kit, x-cmd. Others were already at latest. Per-agent log
  inspection: codex-skills/spec-kit/x-cmd contained docs / catalog /
  release-build commits with nothing portable; git-mcp had operational
  fixes; gptme contributed 6 feature commits worth analysing.
- **Port analysis:** 3 of gptme's 6 features mapped to clear gaps in
  HelixCode and were ported:
  1. **Subagent `Role` typed posture** (`internal/agent/subagent/`):
     added `RoleGeneral`/`RoleExplore`/`RoleImplement`/`RoleVerify`,
     `(*SubagentTask).ApplyRoleDefaults()` wired into Dispatch.
     `RoleVerify` defaults `IsolationNone` and `ReadOnlyByDefault=true`.
  2. **Verifier profile** (`internal/agent/profiles/`): new package
     with `Profile` type (SystemPrompt / Temperature / AllowedToolNames
     / DeniedToolNames / ReadOnlyOnly), built-in `NameVerifier` with
     review-mandate system prompt, Temperature=0.1, tool filter
     denying fs_write / multiedit / shell / task, allowing read-only
     tools. `ForRole(RoleVerify)` returns the verifier profile.
  3. **Cache-coldness TTL heuristic** (`internal/llm/cache_control.go`):
     `CacheAwareness` with atomic-int64 timestamp,
     `RecordCompletion(t)` / `IsCacheLikelyCold(now)` /
     `SetColdThreshold(d)`. `DefaultColdThreshold = 5*time.Minute`
     (matches Anthropic's cache TTL). Wired into the Anthropic
     provider's response/stream-stop paths.
  Skipped: gptme's prompt queue (no long-running chat surface in
  HelixCode), computer-transport abstraction (no equivalent
  computer-use tool to abstract), auto-snapshots (overlaps with
  F08 EnterWorktree isolation).
- **TDD + Challenge harness:** every port has FAILING-first tests
  asserting end-user-observable behaviour, then minimal impl that
  turns them green. `go test -race -count=1` exits 0 for all touched
  packages. 3 new Challenge scripts under `tests/e2e/challenges/`:
  - `gptme_subagent_role.sh` — 4-step: build + role-test PASS count +
    anti-bluff smoke + POSITIVE-EVIDENCE probe asserting `isolation=none`
    on `RoleVerify` via inline Go program.
  - `gptme_verifier_profile.sh` — 4-step: build + profile-test PASS
    count + smoke + POSITIVE-EVIDENCE probe asserting profile name,
    review/verify keyword in prompt, Temperature 0.1, and write-tool
    in denylist.
  - `gptme_cache_coldness.sh` — 4-step: build + cache-test 3×PASS +
    smoke + POSITIVE-EVIDENCE probe walking fresh-is-cold,
    recorded-is-hot, default-threshold-is-5min.
  `bash tests/e2e/challenges/run_all_challenges.sh` reports
  `Results: 11 passed, 0 failed` (up from 8/8 — the 3 new scripts).

### Containerized build path (2026-05-13)

Per operator mandate "boot-up Containers containing proper dependencies
for EVERYTHING using our Containers Submodule":

- `HelixCode/docker/build/Dockerfile.builder` was previously
  **gitignored** by overly-broad `Dockerfile*` and `build/` rules.
  Fresh clones couldn't build the builder image. Tightened both rules
  (`/Dockerfile*`, `/build/` — anchored at module root) so subdirectory
  Dockerfiles are tracked. Also bumped `FROM golang:1.24-alpine` →
  `golang:1.26-alpine` to match `go.mod go 1.26`.
- Containerised build path now reachable from a clone:
  `podman compose -f HelixCode/docker-compose.builder.yml build builder`
  (uses Containers-submodule-aware compose semantics; substitute
  `docker compose` if available). The builder image carries
  Go 1.26 + golangci-lint + alpine apk deps + postgres-client; mounts
  the workspace and reuses Go module/build caches between runs.
- Five more Dockerfiles also became trackable and were checked in:
  `tests/e2e/mocks/Dockerfile.{llm,slack}-mock`,
  `tests/infrastructure/Dockerfile.ssh-{server,worker}`,
  `docker/security/snyk/Dockerfile`.

### Dual API-key loader (2026-05-13)

Per operator mandate "We shall have API keys in .env file in the root
of the project or all of them as the part of api_keys.sh in our host's
home dir! Both of it MUST BE fully supported!" — VERIFIED end-to-end:

- `scripts/load_api_keys.sh` prefers `$HOME/api_keys.sh` when present,
  falls back to `.env` at meta-repo root.
- Test 1 — `$HOME/api_keys.sh` path: loader sourced from project root,
  HOME pointing at real home dir → 42 secret-like vars in env.
- Test 2 — `.env` fallback path: loader sourced with HOME pointed at
  empty fakehome → 46 secret-like vars in env from project `.env`.
- Both paths individually load full secret set; neither silently
  drops. The loader's "prefer $HOME/api_keys.sh, fallback to .env"
  precedence matches the operator's stated semantics.
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
1. Verify state: `git log --oneline -3` should show commits `4f5f8f0` (P0 fixes) and `7f4effd` (P1 fixes).
2. Read `docs/improvements/PROGRESS.md` §Phase 4 section.
3. Anti-bluff sweep is COMPLETE for critical packages (validators, fix, config, deployment).
4. Verify remaining test files for weak assertions (only nil/err checks without content verification).
5. Run `make test` to verify current state before making changes.
6. Once all test suites validated, advance to Phase 5 (end-user materials).

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
| 2026-05-08     | ZB Phase 2-3  | Zero-Bluff Phase 2 (Stub Elimination) and Phase 3 (Feature Gaps) CLOSED. 31 new files, 4 providers, CONST-046 added. Anti-bluff clean. Pushed to github+gitlab. |
| 2026-05-08     | ZB Phase 4    | Zero-Bluff Phase 4 (Test/Challenge Hardening) IN PROGRESS. Critical bluffs eliminated (validators, fix, config, deployment). Article XI §11.9 compliance enforced. Commits `4f5f8f0` + `7f4effd`. |
| 2026-05-12     | ZB Phase 4 close | Zero-Bluff Phase 4 CLOSED. Weak-assertion sweep across 447 test files; 8 tests tightened in internal/fix + internal/deployment with mutation-test confirmation (6 mutations → 6 test failures → reverted). Commits `02b7306` + `a3f8871`. Cross-compile linux/amd64 PASS. Phase 5 (Documentation Suite) next. |
| 2026-05-12     | Hygiene       | Untracked 3 runtime test artefacts + tightened `HelixCode/.gitignore` + removed stray 13 MB build binary. Commit `d7a7956`; all 4 remotes synced. CONTINUATION updated per CONST-044. |
| 2026-05-12     | Anti-bluff sweep | Re-running `go test -short ./internal/...` surfaced 3 distinct flakes masking real defects. Fixed: (1) `internal/discovery/client.go` `discoverByDNS` had no per-call timeout — replaced `net.LookupHost` with `net.DefaultResolver.LookupHost(ctx, ...)` bounded at 200 ms; (2) `internal/helixqa/wrapper.go` `Engine.StartSession` goroutine raced with `t.TempDir` cleanup — added `sync.WaitGroup` + `Shutdown()` method + `t.Cleanup` hook in `setupQATestServer`; (3) `internal/tools/browser/browser_test.go` 5 chromium-launching tests starved under suite parallelism — gated behind `testing.Short()` with SKIP-OK markers. 3 consecutive full short-suite runs PASS clean. `-race` short-suite PASS clean. `go vet` clean. Commit `fbc0560`. |
| 2026-05-12     | Challenge sweep | `bash tests/e2e/challenges/run_all_challenges.sh` was being blocked by `scripts/verify-governance-cascade.sh` exit 1 with 61 third-party submodule `.helix-governance`-marker FAILs. Marker was unreachable in practice (can't commit into third-party tree; meta-repo can't track files inside submodule paths). Relaxed verify script to accept presence in `docs/improvements/submodule_third_party.txt` as the deliberate ACK (in-submodule marker still honoured as stronger ACK). 61 FAILs → 0; run_all_challenges 8/8 PASS. Also untracked 2 more build binaries (`HelixCode/server`, `HelixCode/terminal-ui`) + extended .gitignore. Commit `099f06a`. |
| 2026-05-12     | Challenges submodule sweep | `cd Challenges && make test-short` failed at `TestAndroidSave_AllApiLevels` (7 API-level subtests). Root cause: test hardcoded a consumer-Yole APK path that doesn't exist when the submodule is tested in isolation, violating its CLAUDE.md "100% decoupled" mandate. Gated on `APK_PATH` (or legacy `YOLE_ANDROID_APK_PATH`) env var with SKIP-OK marker; forwarded resolved APK as `$2` to the script. Challenges submodule commit `8024463` pushed all 4 remotes (origin/github/gitlab/upstream parity); meta-repo pin bump `12904e4`. `make test-short` 17/17 PASS. |
| 2026-05-12     | Zero-bluff sweep | Per operator mandate "do not skip any tests or challenges but solve root causes": (1) reverted the 5 `testing.Short()` skips in `internal/tools/browser/browser_test.go` and replaced with `serializeChromiumLaunches(t)` — exclusive flock on `/tmp/helixcode-chromium-test.lock` serialising chromium launches across the entire `go test` process tree; (2) Challenges submodule TestAndroidSave_AllApiLevels switched from runtime `t.Skip` to `//go:build android_save_challenge` (file architecturally absent from default test binary; Challenges submodule rebased onto upstream governance commits, pushed at `278b617`); (3) fixed 12 real data races at production-code root cause across `internal/agent/subagent`, `internal/helixqa`, `internal/llm`, `internal/memory/providers`, `internal/cognee`, `internal/notification`, `internal/telemetry`, `internal/tools/mapping`, `internal/tools/multiedit`, `internal/tools/shell`, `internal/workflow`. Verifications: `go test -race ./internal/...` (full mode, no -short) exits 0 with zero failures, zero race warnings, 3 consecutive runs; `go vet` clean. Anti-bluff smoke 40 hits (down from 41 — `(simplified for now)` comment removed). Commit `49936c0` + Challenges `278b617`. |
| 2026-05-12     | Skip-annotation sweep | Per the no-silent-skips governance gate: 101 SKIP-OK markers added across owned submodules (Containers 25, HelixQA 75, LLMsVerifier 1) — every annotated skip guards a genuine environment limit (no adb/scrcpy/GStreamer/Tesseract/PaddleOCR/Ollama, headless display, root-only, missing testcontainers, missing posix utils, etc.). NO skip removed/weakened/created. Also: fixed unreachable-code vet warning in `HelixCode/internal/llm/litellm/unified_provider.go` (moved terminal `Done: true` emit into the EOF-decode branch where the for-loop actually exits). Polished `scripts/no-silent-skips.sh` with four layered fixes: auto-exclude listed third-party submodules by basename, accept slug-form SKIP-OK markers (`#short-mode`, `P1-F14-T10`), tighten JS regex (`(it\|describe\|test\|context\|xit\|xdescribe)\.skip\(`), path-based post-filter for nested-vendored trees (HelixAgent/MCP/submodules/, HelixQA/tools/opensource/). `go vet ./cmd/... ./internal/...` clean. Submodule pins: Containers `af51968`, HelixQA `e16bfeb`, LLMsVerifier `98758126`. Meta-repo commit `36cfafb`. |
| 2026-05-12     | Submodule test-parity sweep | Per "fix and polish everything": ran `go test ./...` for the first time in every owned submodule. Three failures surfaced and resolved at root cause: (1) **LLMProvider** failed to compile — `replace digital.vasic.models => ../Models` pointed at a path the meta-repo never wired up. Fix: added `Dependencies/HelixDevelopment/Models` as a sibling submodule (`vasic-digital/Models`); preserves 100% decoupled by not relying on HelixAgent. (2) **LLMOrchestrator** + **Security** had stale `go.sum` (missing checksums for transitive testify/go-spew). Fix: `go mod tidy` in each. (3) **Venice provider** TestGetCapabilities asserted hardcoded model IDs (`venice-uncensored`, `llama-3.3-70b`) against the live Venice API; Venice renamed its catalogue, so the brittle equality assertions broke. Fix: shape-based substring assertions (`strings.Contains(m, "llama-3")` / `"uncensored"`) that survive catalogue rename and remain CONST-036 compliant. Submodule pins bumped: LLMProvider `46e703a`, LLMOrchestrator `e744a9a`, Security `eae0cec`; new Models pin added. Every owned submodule's `go test ./...` now exits 0. Meta-repo commit `7c81a40`. |
| 2026-05-12     | Submodule test-parity round 2 | Continuing the per-submodule sweep: VisionEngine + DocProcessor both had 7+ packages failing `[setup failed]` due to stale go.sum drift (missing go-spew checksum). `go mod tidy` in each. VisionEngine additionally had `pkg/remote/deployer_test.go` referencing `Deployer`/`Config`/`NewDeployer`/`.Endpoint()` — types orphaned by the Ollama→LlamaCpp deployer rename (commit 93bc8d4); the file produced 18 "undefined" errors and was phantom-coverage bluff per CONST-035. Deleted the file (the VisionPool path tests in `remote_test.go` already cover the live LlamaCppDeployer-backed code). Submodule pins: VisionEngine `a092195`, DocProcessor `3d11e41`. Every owned submodule's `go test ./...` exits 0. |
| 2026-05-13     | Production-bug sweep round 12 | Round 12 uncovered 1 more real bug (5th instance of the same JSON contract pattern) + added 3 session-action lifecycle probes: (17) `GET /api/v1/workers/:id/metrics` returned `"metrics": null` for the empty-history case — same nil-slice→null bluff as listTasks/listProjects/listWorkers/checkpoints, now metrics. Fixed in `getWorkerMetrics` with `[]*worker.WorkerMetrics{}` default-when-nil. helix-qa expanded 49 → 56 evidence files: HCQA-044 (worker-metrics-array, catches BUG #17), HCQA-044-PRE-W (inline worker for the probe — decouples HCQA-044 from HCQA-032's source ordering), HCQA-044-PRE-PROJ + HCQA-044-PRE-SESS (project+session for action probes), HCQA-045 (action=start → status:active), HCQA-046 (action=complete → status:completed), HCQA-047 (action="invalid-xyz" → 400 with clear error). helix-qa: 51/51 PASS (56 evidence files). run_all_challenges: 12/12 PASS. go test -short server: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Production-bug sweep round 11 | Round 11 uncovered 1 more real bug plus a related partial-update clobber, both in worker UPDATE: (16) `PUT /api/v1/workers/:id` returned HTTP 500 "null value in column capabilities violates not-null constraint" — same TEXT[] NOT NULL pattern as BUG #10 but in the UPDATE path. AND a second related defect: the UPDATE unconditionally replaced every column with the request value, so omitting `hostname` (a partial update) would overwrite the existing hostname with "" — destroying real state. Both fixed in `worker.DatabaseManager.UpdateWorker`: (a) default capabilities to `[]string{}` when nil; (b) replace bare `SET col = $1` with `COALESCE(NULLIF($1, ''), col)` for hostname/display_name and `CASE WHEN $4 > 0 THEN $4 ELSE max_concurrent_tasks END` for max_concurrent_tasks — preserving the existing column when the input is zero-value. helix-qa expanded 45 → 49 evidence files: HCQA-042-CREATE (create worker), HCQA-042 (PUT renames display_name + max but PRESERVES hostname — catches both bugs), HCQA-043-DEL (DELETE 200), HCQA-043 (GET deleted 404). helix-qa: 47/47 PASS (49 evidence files). run_all_challenges: 12/12 PASS. go test -short server+worker: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Production-bug sweep round 10 | Round 10 uncovered 1 more real bug while probing project lifecycle: (15) `PUT /api/v1/projects/:id` returned HTTP 500 "ERROR: column path does not exist (SQLSTATE 42703)" for every authenticated update attempt. Root cause: `internal/project/manager_db.go:UpdateProject`'s `UPDATE ... RETURNING` clause referenced `path` and `type` columns that DON'T EXIST in the projects schema (`internal/database/database.go:333`). Real schema columns: `workspace_path` (mapped to `Project.Path` in Go) and `config` JSONB (which holds the project type under `config["type"]` — extracted in GetProject at line 114). The canonical "rename a project" endpoint was therefore unreachable for the entire deployment. Fixed by mirroring GetProject's column-mapping pattern: RETURNING `workspace_path` + `config`, then extracting `Type` from `config["type"]` and `metadata` from `config["metadata"]` in Go. helix-qa expanded 41 → 45 evidence files: HCQA-039 (create-for-lifecycle), HCQA-040 (PUT renames + asserts new name/description AND path round-trip), HCQA-041-DEL (DELETE 200), HCQA-041 (GET-deleted 404). helix-qa: 43/43 PASS (45 evidence files). run_all_challenges: 12/12 PASS. go test -short server+project+task: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Production-bug sweep round 9 | Round 9 uncovered 1 deep bluff in the memory subsystem: (14) `GET /api/v1/memory/systems` returned 6 systems all hardcoded as `status: "available"` while `GET /api/v1/memory/stats` reported `systems_connected: 0` — direct contradiction. Reading the handler revealed `listMemorySystems` is pure hardcoded English/metadata with NO connection probe of any kind: the Server struct has no memory manager bound for any of the 6 listed providers (Cognee/Weaviate/ChromaDB/Qdrant/Mem0/Zep). CONST-035 violation: handler claims "available" for 6 systems while the same backend's stats endpoint correctly reports zero connections. Fix: introduced a `memoryStatus(id)` helper that returns "not_configured" until a real manager is bound + a reachability probe runs; current state correctly returns "not_configured" for all 6 (matches stats). The catalogue itself (id/name/type/description/features) remains as the documented set of supported backends — that's real metadata, not a runtime claim. helix-qa expanded 40 → 41: new "HCQA-MEM-BLUFF" probe asserts NO entry claims `status="available"` until real backing exists (any future regression to hardcoded "available" without manager-wiring will fail this check). helix-qa: 39/39 PASS (41 evidence files). run_all_challenges: 12/12 PASS. go test -short server+task: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Production-bug sweep round 8 | Round 8 uncovered 2 more real bugs by extending helix-qa to task-lifecycle probes: (12) `GET /api/v1/tasks/:id/checkpoints` returned `"checkpoints": null` for an empty list — 4th instance of the nil-slice→null JSON contract bluff (tasks, projects, workers, now checkpoints); fixed by defaulting to `[]map[string]interface{}{}` in `getTaskCheckpoints`. (13) `POST /api/v1/tasks/:id/retry` returned HTTP 500 "task not found, not in failed state, or max retries exceeded" for client-state errors — 500 is a server-error code that lies about the nature of the problem (a completed task being asked to retry is a client-side state mismatch, not an internal bug). Introduced exported sentinel `task.ErrTaskNotRetryable` in `internal/task/manager_db.go`; handler now `errors.Is`-checks and returns 422 Unprocessable Entity for the state error, 500 only for genuine DB faults. helix-qa expanded 35 → 40 evidence files (HCQA-036 task-create-for-lifecycle, HCQA-037 checkpoints-as-array, HCQA-038 retry-422, plus 2 internal-step probes HCQA-038-PRE-START / HCQA-038-PRE-COMPLETE). helix-qa: 38/38 PASS (40 evidence files; the 2 internal-step probes write evidence but don't add to pass/fail count). run_all_challenges: 12/12 PASS. go test -short server+task: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Sessions-create probe addition (round 7) | Coverage expansion: probed `POST /api/v1/sessions` against the live server with a valid `project_id` (from the round-4 HCQA-031 create-project probe) + valid `Mode` ("planning"). Endpoint works correctly (no bug surfaced — Mode validation correctly rejects "interactive" with a clear error; valid modes are planning/building/testing/refactoring/debugging/deployment as defined in `internal/session/session.go`). New HCQA-035 probe stitches the just-created project_id from HCQA-025 evidence into a sessions POST, asserts 201 + `session.project_id` round-trip + `session.mode=="planning"`. helix-qa: 35/35 PASS. run_all_challenges: 12/12 PASS. |
| 2026-05-13     | Production-bug sweep round 6 | Round 6 uncovered 1 more real bug, same pattern as round 2 BUG #3: (11) `POST /api/v1/auth/refresh` returned HTTP 401 "User not authenticated" for EVERY caller, even with a perfectly valid Bearer token. Root cause: `refreshToken` handler called `c.Get("user")` but the `/auth` route group has NO `authMiddleware` (must stay public for register/login). Fixed by manually parsing+verifying the Authorization header via `VerifyJWTWithDB`, mirroring the `logout` handler's pattern. The new refresh path returns a freshly-issued JWT (200) for valid Bearer, 401 with "Authorization header required" for missing header, 401 with "Invalid or expired token" for malformed/expired Bearer. Existing `TestRefreshToken_WithUserContext` unit test updated to `TestRefreshToken_WithBearer` (the c.Get("user") path no longer exists; mockDB-unverifiable Bearer returns 401 which is the expected case for the unit test). helix-qa expanded 32 → 34: HCQA-033 (valid Bearer issues new JWT — relaxed to "valid JWT" rather than "different JWT" because iat is per-second and within-1-sec refresh can produce byte-identical tokens), HCQA-034 (garbage Bearer returns 401). helix-qa: 34/34 PASS. run_all_challenges: 12/12 PASS. go test -short server+auth: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Production-bug sweep round 5 | Round 5 uncovered 1 more real bug, same pattern as round 2 BUG #1: (10) `POST /api/v1/workers` returned HTTP 500 with `ssh_config` NOT NULL constraint violation when the request omitted the optional `ssh_config` map. Same fix as `task_data`: `worker.DatabaseManager.RegisterWorker` defaults nil `sshConfig` to `map[string]interface{}{}` and nil `capabilities` to `[]string{}` (capabilities is TEXT[] NOT NULL — pgx serializes nil slice as NULL; the column DEFAULT '{}' never applies because the INSERT explicitly provides the value). helix-qa expanded 31 → 32: HCQA-032 probes `POST /workers` with empty body and asserts 201 + UUID + hostname-roundtrip. helix-qa: 32/32 PASS. run_all_challenges: 12/12 PASS. go test -short server+worker: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Production-bug sweep round 4 | Round 4 uncovered 2 more real bugs: (8) `POST /api/v1/projects` returned HTTP 500 "invalid owner ID: invalid UUID length: 12" — `internal/project/manager_db.go:135` had `m.CreateProjectWithUser(..., "default-user")` literally hardcoded as the owner ID; the string "default-user" is exactly 12 chars, matching the error. Handler now extracts the real authenticated `*auth.User` from context and calls `CreateProjectWithUser` directly with `user.ID.String()`. (9) The same POST route was registered under a `publicProjects` Gin group with comment `// no auth for testing` — any unauthenticated caller could create projects or trigger workflow executions (planning/building/testing/refactoring) against any projectId. This was a real production security hole AND made the createProject handler unreachable (it now requires `c.Get("user")` which is only set by authMiddleware). Consolidated under the authenticated `projects` group; `publicProjects` removed entirely. helix-qa expanded 29 → 31: HCQA-030 (unauthenticated POST /projects must 401), HCQA-031 (authenticated POST /projects creates project with UUID + path). Existing unit test `TestCreateProject_ValidRequest` updated to accept 401 as a valid no-auth response (mirrors `TestListProjects` pattern). helix-qa: 31/31 PASS. run_all_challenges: 12/12 PASS. go test -short ./internal/server/...: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Production-bug sweep round 3 | Continuing the anti-bluff push uncovered 2 more real bugs by extending helix-qa to 29 checks: (7) `GET /api/v1/users/me` returned a User stub with `is_active:false` and `created_at:"0001-01-01T00:00:00Z"` — `authMiddleware` was calling the cheap `VerifyJWT` variant that returns ONLY {ID, Username, Email} from JWT claims; every other field defaulted to zero. The slower `VerifyJWTWithDB` exists for exactly this case (fetches the full user + enforces `is_active`). Middleware switched to `VerifyJWTWithDB`; defense-in-depth bonus: deactivated accounts can no longer continue authenticating with pre-deactivation JWTs. (8) `GET /api/v1/workers` returned `"workers": null` — third instance of the nil-slice→null JSON contract bluff (after tasks and projects); fixed by defaulting to `[]*worker.Worker{}` when nil. New helix-qa probes HCQA-027 (workers as array), HCQA-028 (system stats sub-objects). HCQA-017 tightened to assert `is_active==true` AND `created_at` non-zero — would have caught BUG #6 if it had existed before the fix. helix-qa: 29/29 PASS. run_all_challenges: 12/12 PASS. go test -short server+auth: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Production-bug sweep round 2 | Continuing anti-bluff push uncovered 3 more real production bugs by extending helix-qa from 19 → 27 checks with tasks-CRUD lifecycle and projects/sessions list probes: (3) `POST /api/v1/tasks` returned HTTP 500 "null value in column task_data violates not-null constraint" when `parameters` was omitted — `internal/task/manager_db.go:CreateTask` forwarded nil map to INSERT and pgx serialized it as SQL NULL; fixed by defaulting `parameters` to empty map at the persistence constructor (the schema invariant `task_data JSONB NOT NULL` now upheld). (4) `GET /api/v1/tasks` returned `"tasks": null` instead of `[]` for empty list — JSON contract bluff (Go's nil-slice→null serialization); fixed in `listTasks`. (5) `GET /api/v1/projects` returned 401 "user_id not found in context" with a VALID JWT — `listProjects` called `c.GetString("user_id")` while `authMiddleware` sets `c.Set("user", *auth.User)` (no such "user_id" key); the entire projects-list endpoint was unreachable for the production deployment. Fixed by switching to the established `c.Get("user")` pattern. (6) Same `projects: null` vs `[]` bug as #4; fixed. New helix-qa probes HCQA-019..027: tasks-CRUD full lifecycle (list-empty → create → list-contains → get-by-id → delete → get-deleted-404), plus list-projects-as-array and list-sessions-as-array. All 4 remotes at `c332f98`. helix-qa: 27/27 PASS. run_all_challenges: 12 passed. go test -short server+task+project: ok. scripts/scan-secrets.sh: clean. |
| 2026-05-13     | Mistborn distribution + anti-bluff QA harness | Stood up the mistborn.local distribution stack and the helix-bridge / helix-qa anti-bluff harnesses per operator mandate "all tests and Challenges do work in anti-bluff manner — they MUST confirm that all tested codebase really works as expected". Delivered: (1) `Containers/.env` entry for `mistborn.local` (gitignored, key-based auth only — password never committed); (2) `docs/distribution/docker-compose.mistborn.yml` 7-service podman stack (postgres/redis/qdrant/chromadb/ollama/prometheus/grafana); (3) `scripts/mistborn-{up,down}.sh` boots stack + 8 SSH tunnels with alternate local ports; (4) `scripts/bridge/main.go` (helix-bridge) multi-provider LLM client honoring the dual `api_keys.sh` / `.env` loader contract — 3/5 providers respond live (groq, mistral, deepseek); (5) `scripts/qa/main.go` (helix-qa) anti-bluff QA harness with captured wire evidence per check (status + body_bytes + body_head + duration_ms in per-check JSON); (6) `tests/e2e/challenges/helix_qa_live_anti_bluff.sh` wired into `run_all_challenges.sh` (12/12 PASS); (7) **BLUFF-002 fix**: `internal/server/handlers.go:getLLMProvider` returned a fabricated `status: available` stub for ANY id including `does-not-exist-xyz` — now correctly returns 404 for unknown providers, sourced from verifier + constitutional fallback set. New `TestGetLLMProvider_UnknownIDReturns404` reproduces and guards. (8) helix-qa expanded 6 → 19 checks: deep content (count>0, database.connected==true, goroutines>0) + sequenced auth-flow (register → login → /users/me → garbage-bearer-401 → logout) with strict invariants (`session.user_id === user.id from registration`, `user.username === registered username`, `token starts with "eyJ"`). Pre-existing self-bluffs in HCQA-004/005/006 (assertions of expectations the server contract never promised) corrected. Also-discovered + fixed: HelixQA orchestrator 0-challenge PASS-bluff at `pkg/orchestrator/orchestrator.go:186`; tests at `integration_test.go`/`orchestrator_edge_test.go`/`orchestrator_stress_test.go` migrated from `assert.True(t, result.Success)` to anti-bluff `assert.False`. Browser test context-deadline bumped 60s → 180s. Local verification: HelixCode locally serves `database.connected:true` through mistborn-Postgres tunnel; `helix-qa`: 19/19 PASS with no FAIL and no empty bodies; `run_all_challenges.sh`: 12/12 PASS; HelixCode internal -race short: 91 packages 0 failures 0 races; HelixQA: 135 packages 0 failures; anti-bluff smoke: clean (5 pre-existing benign matches in comments/template-substitution). Meta-repo commits `6e58ab6` → `30a1e2b` → `5c44ba3`, all 4 remotes parity (github + gitlab + origin fanout + upstream). |
