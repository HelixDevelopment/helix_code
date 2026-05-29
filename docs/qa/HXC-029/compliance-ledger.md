# HXC-029 — §11.4.98 Full-Automation Anti-Bluff Compliance Ledger

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-05-29 |
| Last modified | 2026-05-29 |
| Status | active |
| Tracked item | HXC-029 (§11.4.98 full-automation anti-bluff forward sweep) |
| Pass type | **Static first-pass audit** (source reading + grep). NO tests executed. NO tests modified. |

## Method

§11.4.98 mandate: every live/integration/e2e/Challenge/stress/chaos test MUST be
**fully self-driving end-to-end** — NO human action during execution (no "operator
must type", no manual UI click, no hand-triggered webhook, no hard-coded session
UUIDs colliding with the active dev session, no human-response time windows). The
single permissible exception is one-time credential bootstrap performed OUTSIDE
test execution.

This audit grepped the in-scope test/challenge surface for human-in-the-loop
signals: `os.Stdin` reads, `fmt.Scan`, `bufio.NewReader(os.Stdin)`, `read -p` /
`read -r` (stdin), prompt strings ("press enter", "type a message", "operator
must", "manually", "by hand", "run manually"), `t.Skip` with manual-step reasons,
human-response `time.Sleep` windows (≥30 s), hard-coded session UUIDs, interactive
flags. Findings classified **COMPLIANT** / **NON-COMPLIANT** / **NEEDS-MANUAL-REVIEW**.

Scope audited (canonical tree only — `.claude/worktrees/` agent-copy duplicates
were EXCLUDED as non-canonical):
- `helix_code/tests/{integration,e2e,security,performance}/**`
- `helix_qa/**` (banks, challenge scripts, executor `pkg/`, runner `cmd/`)
- `challenges/**` challenge scripts (`challenges/challenges/scripts/`, `challenges/p1-*`, `challenges/p2-*`, `challenges/lib`)
- `*.sh` challenge / live-test scripts under the repo

Greps run (representative, all from repo root):
```
grep -rn "os.Stdin|bufio.NewReader(os.Stdin)|fmt.Scan|ReadString('\n')|StdinPipe" helix_code/tests/
grep -rni "press enter|type a message|operator must|manually|by hand|run manually|please type|user must" helix_code/tests/
grep -rn "t.Skip" helix_code/tests/ | grep -iv testing.Short
grep -rni "press enter|read -p|read -r |type a message|operator must|manually|stdin" challenges/... helix_qa/challenges/scripts ...
grep -rniE "[0-9a-f]{8}-...-...-...-[0-9a-f]{12}|claude --resume|--session-id|--resume" helix_code/tests challenges helix_qa/banks
grep -rniE "manual|operator must|press enter|interactive: *true" helix_qa/banks
grep -rnE "time.Sleep\((30|[4-9][0-9]|[0-9]{3,})\s*\*\s*time.Second|...time.Minute" helix_code/tests
```

## Classification table

| Test/Suite (path) | Layer | Classification | Signal found (file:line + snippet) |
|---|---|---|---|
| `helix_code/tests/integration/askuser_test.go` | integration | COMPLIANT | Uses deterministic `bytes.Buffer` I/O, NOT `os.Stdin`. L180 comment: "Provide an empty buffer so the prompter doesn't try to probe os.Stdin during construction; we never call Prompt in this test." All `os.Stdin` matches are comments only. |
| `helix_code/tests/regression/server_timeout_test.go` | regression/integration | **NON-COMPLIANT** | L121 `t.Skip("Long-running stability test - run manually for verification")  // SKIP-OK: #legacy-untriaged` — `TestServerStability` is gated on manual human invocation; body never starts the server (L131 comment: "In a real test, we would start the server…"). Manual-dependency + non-self-driving. |
| `helix_code/tests/e2e/scripts/clean.sh` | e2e (helper script) | **NON-COMPLIANT (helper, not a test)** | L59 `read -p "Remove test results? (y/N) " -n 1 -r` — interactive stdin prompt. This is a cleanup helper, not a governed test, but it blocks any automated pipeline that invokes it. Flagged for awareness; lower priority than test-path findings. |
| `helix_qa/banks/full-qa-web.json` (572 entries) | full-automation / e2e bank | NEEDS-MANUAL-REVIEW | 572× `"_conversion_note": "manual-review-required"`; steps carry `"_original_action": "Open … in the browser"` converted to `playwright: navigate …`. Per `helix_qa/pkg/testbank/schema.go:189-191` `ActionTypePlaywright` steps currently **SKIP** with `PLAYWRIGHT-RUNTIME-PENDING` (runtime not wired). SKIPPED ≠ human-driven, so NOT a live §11.4.98 manual-intervention violation, but the prose origin + un-executed state needs human review to confirm no step expects a human. |
| `helix_qa/banks/atmosphere.json` (423 entries) | full-automation bank | NEEDS-MANUAL-REVIEW | 423× `manual-review-required`. Same converted-from-manual prose pattern; executor classifies unrecognised prose as `ActionTypeDescription` (schema.go:101-102 "text-only action (legacy, non-executable)") → placeholder/skipped (structured_executor.go:241-247). Confirm none expects human action. |
| `helix_qa/banks/full-qa-android.json` (318 entries) | full-automation bank | NEEDS-MANUAL-REVIEW | 318× `manual-review-required`. Same pattern (adb_shell/tap/swipe converted from manual steps). |
| `helix_qa/banks/performance-validation.json` (116) | performance bank | NEEDS-MANUAL-REVIEW | 116× `manual-review-required`. |
| `helix_qa/banks/storage-configuration.json` (88) | full-automation bank | NEEDS-MANUAL-REVIEW | 88× `manual-review-required`. |
| `helix_qa/banks/cli-agents-comprehensive.json` (79) | full-automation bank | NEEDS-MANUAL-REVIEW | 79× `manual-review-required`. |
| `helix_qa/banks/edge-cases-stress.json` (78) | stress bank | NEEDS-MANUAL-REVIEW | 78× `manual-review-required`. |
| `helix_qa/banks/aichat-bash-tools-comprehensive.json` (72) | full-automation bank | NEEDS-MANUAL-REVIEW | 72× `manual-review-required`. |
| `helix_qa/banks/cli-agents-test-helixagent.json` (71) | full-automation bank | NEEDS-MANUAL-REVIEW | 71× `manual-review-required`. |
| `helix_qa/banks/editor-operations.json` (63) | full-automation bank | NEEDS-MANUAL-REVIEW | 63× `manual-review-required`. |
| `helix_qa/banks/cloud-storage-operations.json` (63) | full-automation bank | NEEDS-MANUAL-REVIEW | 63× `manual-review-required`. |
| `helix_qa/banks/file-browser.json` (61) | full-automation bank | NEEDS-MANUAL-REVIEW | 61× `manual-review-required`. |
| `helix_qa/banks/app-navigation.json` (54) | full-automation bank | NEEDS-MANUAL-REVIEW | 54× `manual-review-required`. |
| `helix_qa/banks/security-validation.json` (41) | security bank | NEEDS-MANUAL-REVIEW | 41× `manual-review-required`. |
| `helix_qa/banks/all-formats.json` (40) | full-automation bank | NEEDS-MANUAL-REVIEW | 40× `manual-review-required`. |
| `helix_qa/banks/entity-management.json` (38) | full-automation bank | NEEDS-MANUAL-REVIEW | 38× `manual-review-required`. |
| `helix_qa/banks/full-qa-cross-platform.json` (37) | full-automation bank | NEEDS-MANUAL-REVIEW | 37× `manual-review-required`. |
| `helix_qa/banks/admin-operations.json` (29) | full-automation bank | NEEDS-MANUAL-REVIEW | 29× `manual-review-required`; e.g. ADM-012 step "Wait for completion" / "Poll scan status until complete" (L462-466) is prose → skipped. |
| `helix_qa/banks/full-qa-api.json` (27) | full-automation bank | NEEDS-MANUAL-REVIEW | 27× `manual-review-required`. |
| `helix_qa/challenges/scripts/ui_terminal_interaction_challenge.sh` | UI challenge | COMPLIANT | Despite the "interaction" name, drives `helixqa --help`/`-h` non-interactively, asserts on stdout schema; SKIP-OK when binary absent. No stdin read. |
| `helix_qa/challenges/scripts/ux_end_to_end_flow_challenge.sh` | UX challenge | COMPLIANT | Same non-interactive `--help`/`-h` driving + panic-census assertions. |
| `helix_qa/challenges/scripts/{chaos_failure_injection,ddos_health_flood,scaling_horizontal,stress_sustained_load,...}_challenge.sh` | chaos/ddos/scaling/stress | NEEDS-MANUAL-REVIEW | No human-in-loop grep hit (only `IFS=',' read -r -a` array-split, not stdin). Static grep cannot confirm the binary they drive runs unattended; runtime review needed but no manual-intervention signal found. |
| `challenges/p1-f06..p2-f24/run.sh` (20 scripts) | feature challenges | NEEDS-MANUAL-REVIEW | No human-in-loop grep hit. `challenges/lib/anti_bluff.sh` helper present. Static grep clean; runtime self-driving not verifiable statically. |
| `helix_code/tests/integration/*.go` (most: simple, workflow_tools, integration, cognee, api, multi_provider, provider, memory, mcp_stdio, background_shell, planmode_gating, markdown_commands, skills, sessions_resume, lsp, sandbox, subagent, telemetry, smartedit, approval, autocommit, browser, theme, auto_compaction) | integration | NEEDS-MANUAL-REVIEW | No stdin/prompt/human-wait signal found. `t.Skip` calls are all `testing.Short()` / infra-unavailable / server-not-available (NOT manual-step). Static-clean for §11.4.98 but real-infra dependence + true self-driving not verifiable by grep alone. |
| `helix_code/tests/e2e/**` (complete_workflow_test, e2e_test_framework, challenges/*, core/*, phase2/*, phase3/*, orchestrator/*) | e2e | NEEDS-MANUAL-REVIEW | No interactive-flag / stdin / human-wait signal in framework files. `t.Skip` are short-mode / CI-env / server-not-available / API-key-not-configured (config bootstrap = permitted §11.4.98(B) exception). Static-clean; runtime self-driving not verifiable by grep. |
| `helix_code/tests/security/*.go` (authentication, authorization, tools_security, owasp, simple) | security | NEEDS-MANUAL-REVIEW | No human-in-loop signal. UUID hits (`authorization_test.go:566`, `owasp_test.go:122` = `00000000-0000-0000-0000-000000000001`) are fixed test-data IDs, NOT session UUIDs colliding with a dev session — NOT the §11.4.98(C) hazard. Static-clean. |
| `helix_code/tests/performance/*.go` (benchmark, competitor_baseline, pprof_harness, pgo, scenarios/*) | performance | NEEDS-MANUAL-REVIEW | No human-in-loop signal. Static-clean. |

## Counts by classification

| Classification | Count | Notes |
|---|---|---|
| COMPLIANT | 4 | askuser_test.go; ui_terminal_interaction_challenge.sh; ux_end_to_end_flow_challenge.sh; (askuser explicitly anti-bluff via bytes.Buffer) |
| **NON-COMPLIANT** | 2 | server_timeout_test.go (manual-only `TestServerStability`); clean.sh (interactive `read -p` — helper, lower severity) |
| NEEDS-MANUAL-REVIEW | 19 banks + ~24 Go integration files + ~all e2e/security/performance suites + chaos/scaling/feature challenge scripts | Predominantly: (a) HelixQA banks with `manual-review-required` converted-prose steps that the executor SKIPs (not human-driven, but origin needs review); (b) integration/e2e suites where grep finds no manual signal but true unattended self-driving is not statically provable. |

**Honest summary:** Only **2 genuinely NON-COMPLIANT** §11.4.98 findings were located
by static analysis with cited file:line. The 19 HelixQA banks carrying
`manual-review-required` are NOT confirmed live manual-intervention violations —
the executor SKIPs unrecognised prose steps (`schema.go:101-102`,
`structured_executor.go:241-247`) and SKIPs `ActionTypePlaywright` steps with
`PLAYWRIGHT-RUNTIME-PENDING` (`schema.go:189-191`) — but their converted-from-manual
origin and un-executed state warrant human review per §11.4.98's obsolescence-audit
clause (manual-dependency tests not rewritten within 30 days → §11.4.90 Obsolete).

## Top NON-COMPLIANT to rewrite first (prioritized)

1. **`helix_code/tests/regression/server_timeout_test.go:121` — `TestServerStability`.**
   The ONLY confirmed code-level §11.4.98 violation: a test gated entirely on manual
   human invocation (`t.Skip("…run manually for verification")`) whose body never
   actually starts the server (stub comment "In a real test, we would start the server").
   Rewrite to programmatically boot the server, run for `2× IdleTimeout`, assert no
   premature shutdown — fully unattended. (Also a §11.4 PASS-bluff: skipped + non-functional.)

2. **HelixQA `full-qa-web.json` (572 `manual-review-required`, `ActionTypePlaywright`
   PLAYWRIGHT-RUNTIME-PENDING).** Largest converted-from-manual bank. Wire the
   Playwright runtime (`PlaywrightCLIAdapter` per `schema.go:186`) so the 572 web
   steps actually execute unattended instead of SKIPping — currently the largest
   block of "looks covered but does not run" surface.

3. **HelixQA `atmosphere.json` (423) + `full-qa-android.json` (318).** Next-largest
   converted-prose banks; confirm every `manual-review-required` step maps to an
   executable `adb_shell`/`tap`/`http`/`assert` action with NO residual human step,
   or mark Obsolete per §11.4.98(F)/§11.4.90.

4. **`helix_code/tests/e2e/scripts/clean.sh:59` — interactive `read -p`.** Replace the
   `y/N` prompt with a `--force` / `CLEAN_TEST_RESULTS=1` env-driven non-interactive
   path so any pipeline invoking it never blocks on stdin. (Helper, not a governed
   test, but blocks automation.)

5. **Remaining `*-validation`/`*-operations`/`*-comprehensive` HelixQA banks
   (performance-validation 116, storage-configuration 88, cli-agents-comprehensive 79,
   edge-cases-stress 78, …).** Batch-audit the `manual-review-required` entries for
   any genuine human dependency, then either make executable or Obsolete.

## Limitations of this static pass

A static grep + source read CANNOT determine the following — these require runtime
execution (out of scope for this pass) or human review:

1. **Runtime human dependence not visible in source.** A test with no prompt string
   may still block on an external system that only a human can advance (e.g. an OAuth
   consent screen, a device-pairing tap) without any literal "press enter" in the code.
2. **Whether SKIPPED prose/Playwright steps would demand human action IF wired.** The
   executor currently SKIPs them; the original manual intent (`_original_action`) is
   prose like "Open … in the browser" — only review of the converted action confirms
   no human is expected once the runtime lands.
3. **Re-runnability (`-count=3` consecutive PASS with self-cleaning state)** per
   §11.4.98(C) — provable only by running the tests, which this pass did not do.
4. **Hidden human-response time windows** implemented via channels, context deadlines,
   or polling loops that wait on externally-injected human input without an obvious
   `time.Sleep` literal.
5. **Hard-coded session-UUID collisions** with a live dev session — the UUIDs found
   (`00000000-…-0001`, slack mock `550e8400-…`) are fixed test fixtures, but a
   collision hazard depends on the runtime session ID, unknowable statically.
6. **Banks not loaded by any active runner.** Whether a given bank is actually executed
   in a QA session (vs orphaned) was not traced end-to-end; an unexecuted bank cannot
   violate §11.4.98 at runtime regardless of its contents.
7. **`.claude/worktrees/` duplicates were excluded** as non-canonical agent copies;
   if any are independently executed, they were not audited here.

## Sources verified

Internal source-of-truth files read during this audit (no external web sources —
this is a static code audit, not a documentation-authoring task under §11.4.99):
- `helix_qa/pkg/testbank/schema.go` (ActionType taxonomy, ParseAction, SKIP semantics)
- `helix_qa/pkg/autonomous/structured_executor.go` (step dispatch + placeholder skip)
- `helix_code/tests/integration/askuser_test.go`
- `helix_code/tests/regression/server_timeout_test.go`
- `helix_code/tests/e2e/scripts/clean.sh`
- `helix_qa/challenges/scripts/{ui_terminal_interaction,ux_end_to_end_flow}_challenge.sh`
