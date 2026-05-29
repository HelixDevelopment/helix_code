# HXC-029 §11.4.98 — Integration-Test Manual-Dependency Classification Ledger

| Metadata | Value |
|---|---|
| Revision | 1 |
| Created | 2026-05-29 |
| Last modified | 2026-05-29 |
| Status | active |
| Ticket | HXC-029 (§11.4.98(F) full-automation anti-bluff — integration-test half) |
| Scope | `helix_code/tests/integration/**` (31 `*_test.go` files) |
| Live infra | http://localhost:8080 (health=200, register reachable), PostgreSQL :5432, Redis :6379 — all confirmed listening |
| Go | go1.26.2 (inner module `dev.helix.code`) |

## Summary

| Classification | Count |
|---|---|
| **Self-driving** (no human action during execution) | **31 / 31 files** |
| Converted (manual → programmatic) | 0 (none needed) |
| Obsolete-candidate (genuinely undriveable headless) | 0 |

**Headline result:** ZERO of the 31 integration test files require any human action
during execution. Every file was previously flagged NEEDS-MANUAL-REVIEW but on
inspection none meets a §11.4.98(C) manual-dependency criterion (typing, clicking,
hand-triggered webhook, manual file attach, hard-coded session UUID, 60 s
human-response window). They are all already self-driving — gated only by build
tags / `testing.Short` / env-var infra presence.

## Test-execution evidence (real captured runs)

Real commands run from `helix_code/`:

```
go vet -tags=integration ./tests/integration/                          # EXIT 0 (compiles clean)
go test -tags=integration -count=1 -v ./tests/integration/             # 110 PASS, 20 SKIP, 3 FAIL
go test -tags='integration testing_export' -count=1 -run Telemetry ... # 6 PASS, 2 SKIP
```

Captured logs (in this directory):
- `full_integration_run.log` — full `-v` run (110 PASS / 20 SKIP / 3 FAIL)
- `telemetry_run.log` — double-tagged telemetry suite (6 PASS / 2 SKIP)
- `askuser_run.log`, `approval_run.log` — focused FAIL captures
- `i18n-regression-evidence.txt` — the 3 real FAILs (NOT manual-dependency; see below)

### The 3 FAILs are a real i18n-render regression, NOT a manual dependency

All 3 failing tests are fully self-driving (deterministic `bytes.Buffer` I/O, no
human action). They FAIL because the prompter render path now emits **raw i18n
translation keys** instead of resolved+interpolated user-facing text:

- `TestAskUser_PreviewVisibleInOutput` — output contains `askuser_prompt_choice_preview_label`
  but NOT the actual preview value `RGB(255,0,0) — bright crimson`.
- `TestAskUser_InvalidInputThenValid` — output contains `askuser_prompt_invalid_choice_hint`
  but NOT the valid-range hint `1-3`.
- `TestApproval_AutoEdit_PromptsRun_UserAllows` — output is
  `internal_approval_prompt_allow_toolinternal_approval_prompt_args_suffix` instead of
  containing the tool name `stub_run_for_approval`.

This is a genuine §11.9 usability defect (the user is shown raw i18n keys instead of
text) that the self-driving tests correctly catch. Per §11.4.6 (no-guessing) it is
reported honestly here, NOT papered over with a PASS. **Fix is OUT of this task's
scope** (production i18n render path in `internal/tools/askuser` + approval prompter,
not a manual-dependency conversion). Flagged for the main stream as a §11.4.15 Bug.

### The 20 SKIPs are honest infra/credential-unavailable skips (not manual deps)

All cite concrete infra reasons — Anthropic key absent, gopls not on PATH, cloud
creds absent, Ollama/VLLM/LocalAI/Cognee not running, test admin user not
configured, OTLP exporter not running, `RUN_CONVERSION_TESTS` unset. These are
legitimate §11.4.3 SKIP-with-reason, NOT human-in-the-loop dependencies.

## Per-file classification

| File | Build gate | Classification | Driving mechanism | Run result |
|---|---|---|---|---|
| api_integration_test.go | `testing.Short` | self-driving | real HTTP to :8080 + httptest | PASS (TestHealthEndpoint, TestAuthenticationFlow); SKIP task/worker (no admin user) |
| approval_test.go | `//go:build integration` | self-driving | in-memory approval engine, deterministic | 7 PASS / 1 FAIL (i18n regression) |
| askuser_test.go | `//go:build integration` | self-driving | `bytes.Buffer` Reader/Writer (NOT os.Stdin), `IsTTY` closure | 4 PASS / 2 FAIL (i18n regression) |
| autocommit_test.go | `//go:build integration` | self-driving | real `git` in `t.TempDir()` | 5 PASS |
| auto_compaction_integration_test.go | `//go:build integration` | self-driving | real Anthropic (SKIP w/o key) | SKIP-OK (no key) |
| background_shell_test.go | `//go:build integration` | self-driving | real `os/exec` shell, POSIX-gated | 3 PASS |
| browser_test.go | `//go:build integration` | self-driving | **chromedp programmatic** click/type/screenshot vs `httptest.Server` fixture (the "click" is a chromedp DOM action, NOT human) | 7 PASS |
| cognee_integration_test.go | none (default-skip) | self-driving | real Cognee HTTP (env URL, SKIP if down) | 6 SKIP (Cognee not running) |
| cognee_real_llm_test.go | none (suite, SKIP guard) | self-driving | real LLM (SKIP w/o config) | SKIP (config invalid) |
| integration_test.go | `testing.Short` | self-driving | real HTTP to :8080 | 2 PASS / 4 SKIP (no admin user) |
| lsp_test.go | `//go:build integration` | self-driving | real LSP subprocess + fake-server fixture; gopls-gated | 3 PASS / 1 SKIP (no gopls) |
| markdown_commands_test.go | `//go:build integration` | self-driving | real FS + fsnotify watcher | 3 PASS |
| mcp_stdio_test.go | `//go:build integration` | self-driving | real MCP stdio subprocess handshake | 2 PASS |
| memory_providers_integration_test.go | none | self-driving | real provider constructors | 4 PASS |
| memory_test.go | `//go:build integration` | self-driving | real FS + hot-reload | 3 PASS |
| multi_provider_test.go | `//go:build integration` | self-driving | config/env/flag resolution; cloud-call SKIP w/o creds | 7 PASS / 1 SKIP |
| planmode_gating_test.go | `//go:build integration` | self-driving | in-memory plan-mode gate, POSIX-gated | 3 PASS |
| provider_integration_test.go | `//go:build integration` | self-driving | real provider HTTP (SKIP if backend down); `testTimeout=60s` is an HTTP-client deadline, NOT a human-response window | 2 PASS / several SKIP (no Ollama/VLLM/LocalAI) |
| sandbox_test.go | `//go:build integration` | self-driving | real bubblewrap (gated), CONST-033 reject | 6 PASS |
| sessions_resume_test.go | `//go:build integration` | self-driving | real session persistence across restart | 2 PASS |
| simple_test.go | `testing.Short` | self-driving | real `os/exec` + FS + net dial | 6 PASS |
| skills_test.go | `//go:build integration` | self-driving | real FS + watcher | 3 PASS |
| smartedit_test.go | `//go:build integration` | self-driving | real FS edits + git commit | PASS (all) |
| subagent_test.go | `//go:build integration` | self-driving | real subagent subprocess via testhelper | PASS |
| telemetry_test.go | `//go:build integration && testing_export` | self-driving | OTel stdout/in-memory exporter; OTLP SKIP if no collector | 6 PASS / 2 SKIP |
| theme_test.go | `//go:build integration` | self-driving | real theme load/render | PASS |
| workflow_tools_integration_test.go | `testing.Short` | self-driving | real workflow tools | PASS |
| (cmd/, hooks/, permissions/, persistence/, worktree/ subdirs) | challenge runners / sub-suites | self-driving | httptest fixtures, generated `uuid.NewString()` (NOT hard-coded), real FS/git | covered by suite run |

## §11.4.98(C) criterion-by-criterion negative findings

- **Typing during execution**: NONE — `askuser_test.go` feeds input via `bytes.NewBufferString("1\n")`, never `os.Stdin`.
- **Clicking**: the only "click" is `browser_test.go` / `cmd/p2f23` chromedp programmatic `click.Execute({selector:"#b"})` against a fixture — machine-driven, asserts `UNCLICKED→CLICKED_42`.
- **Hand-triggered webhook**: NONE.
- **Manual file attach**: NONE — all file ops use `t.TempDir()` + `os.WriteFile`.
- **Hard-coded session UUID** (Herald lesson): NONE — `cmd/p1f11_challenge` uses `uuid.NewString()` (freshly generated); no `claude --resume` collision risk.
- **60 s human-response window**: NONE — the only `60 * time.Second` literals are HTTP-client timeouts (`provider_integration_test.go:25`, `cognee_integration_test.go:31`), not human-input deadlines.

## Anti-bluff statement (§11.4 / §11.4.98 / §11.9)

Every "self-driving + PASS" claim above is backed by a real captured `go test`
invocation in `full_integration_run.log` / `telemetry_run.log` — not "looks
self-driving". The 3 FAILs and 20 SKIPs are reported honestly (no fabricated PASS).
No test file was modified. No file changes were staged or committed by this task.
