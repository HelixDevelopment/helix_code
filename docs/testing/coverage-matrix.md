# Test-Type Coverage Matrix (Living Ledger)

| Field | Value |
|-------|-------|
| Revision | 1 |
| Last modified | 2026-06-10 |
| Status | LIVING LEDGER — current state verified against the tree on 2026-06-10; targets are PLANNED (SP-owned) |
| Scope | `helix_code/` (inner Go app `dev.helix.code`) + delegated `submodules/helix_qa` + `submodules/challenges` |
| Source plan | `docs/superpowers/specs/plans/2026-06-10-SP7-testing-qa-plan.md` (§1 ground truth, §2 matrix, §7 guard rows) |
| Authority | Cascades from `constitution/` + root `CLAUDE.md`/`CONSTITUTION.md`. Anti-bluff §11.4 governs every PASS. |

> **Anti-bluff note (§11.4 / §11.4.123).** This ledger separates **what exists today** (file:line or `ABSENT`,
> verified against the live tree on 2026-06-10) from **the target** (PLANNED, owned by a sub-program). A cell
> reading `ABSENT` or `THIN` is an honest coverage gap, not a failure to find. CONST-050(B): a *delegated*
> HelixQA/Challenge slot counts ONLY if it truly exercises HelixCode with captured evidence — otherwise it is a
> coverage-bluff risk, flagged below.

---

## 1. How to read this ledger

- **Rows** = the 15 supported test types (plus two cross-cutting meta rows: §1.1 paired-mutation, regression-guard).
- **Columns**:
  - **Today's status** — `file:line` / `dir/` if present; `ABSENT` / `THIN` / `DELEGATED` otherwise (verified 2026-06-10).
  - **Target** — the intended end state per the SP7 plan.
  - **Owning SP** — which sub-program's feature area the type must cover.
- **SP feature areas** (the 6 columns of "what must be covered"):
  | SP | Feature area |
  |----|--------------|
  | **SP1** | Model access (key recognition → working-model funnel) |
  | **SP2** | HelixAgent exposure (unified catalog, per-provider/per-model targeting) |
  | **SP4** | CLI bridge (spawn real CLI agents; prompt→edit→commit) |
  | **SP5** | Parallel / subagent execution (worker-pool, scheduler, consensus) |
  | **SP6** | Dynamic flows (flow engine / DAG / pipeline / agent-mesh) |
  | **SP7** | Cross-cutting QA (this ledger; autonomous sessions; guard registry) |

> **Verified ground truth (SP7 §1, re-confirmed 2026-06-10):** HelixCode-local coverage is **STRONG** for
> unit / integration / e2e / security / chaos / stress / performance / benchmarking, with a real §1.1 meta-test
> harness at `helix_code/tests/stresschaos/stresschaos_meta_test.go`. The gaps are concentrated in
> ddos / scaling / ux / ui (local), HelixQA staleness, and the per-defect regression-guard registry.

---

## 2. The 15 test types × current status × target × owning SP

| # | Test type | Today's status (helix_code-local + delegated) | Target (PLANNED) | Owning SP feature(s) |
|---|-----------|------------------------------------------------|------------------|----------------------|
| 1 | **unit** | PRESENT — `helix_code/internal/**/*_test.go`, `helix_code/tests/unit/`; Makefile `test:107` | none (strong); add SP-specific unit layers per new pkg | SP1 (filter logic), SP4 (provider adapters), SP5 (scheduler/resolver), SP6 (flow units) |
| 2 | **integration** (`-tags=integration`) | PRESENT — `helix_code/tests/integration/` (`api_integration_test.go`, `approval_test.go`); `test-integration-full:222` | none; add SP1 real-verifier+real-provider case `sp1_model_access_test.go` | SP1 (real verifier CONST-037 + real provider HTTP), SP2 (catalog vs real registry), SP4 (spawn real CLI), SP5 (subagent spawn) |
| 3 | **e2e** | PRESENT — `helix_code/tests/e2e/` (`complete_workflow_test.go`, `challenges/executor.go`, `comprehensive_matrix_test.go`); `test-e2e-full:228` | none; add SP1 `sp1_working_models` one-key→real-generate | SP1 (`/api/v1/llm/generate` real output), SP2 (catalog→completion), SP4 (round-trip), SP5/SP6 (multi-agent completes) |
| 4 | **full-automation** | DELEGATED + THIN local — HelixQA `cmd/helixqa autonomous` + `helix_code/tests/automation/`; `test-complete:246` | autonomous session wired to a §11.4.116 sync-channel (SP7 Phase C); `-count=3` re-runnable | ALL SPs |
| 5 | **security** | PRESENT — `helix_code/tests/security/{owasp,authentication,authorization,tools_security}_test.go` + scanners `security-scan*:751-775`; `test-security-full:234` | add SP1 key-leak / CONST-042 cases | SP1 (key handling CONST-042), SP4 (sandboxed exec), SP2 (cross-module authz) |
| 6 | **ddos** | **ABSENT local** (`helix_code/tests/ddos` does not exist) — DELEGATED only: HelixQA `challenges/scripts/ddos_health_flood_challenge.sh` + `banks/ddos-ratelimit-comprehensive.yaml` | **GAP-1** → local `helix_code/tests/ddos/ddos_harness.go` (flood N≥100, capture p50/p95/p99) + `CM-DDOS-HITS-HELIXCODE` gate (SP7 Task A1/A5) | SP1 (rate-limit + `/generate` flood), SP2 (exposure-endpoint flood) |
| 7 | **scaling** | **ABSENT local** (`helix_code/tests/scaling` does not exist) — DELEGATED only: HelixQA `challenges/scripts/scaling_horizontal_challenge.sh` | **GAP-2** → local `helix_code/tests/scaling/scaling_harness.go` (throughput vs N=1,2,4,8 over `internal/worker`) + `CM-SCALING-HITS-HELIXCODE` gate (SP7 Task A2/A5) | SP5 (SSH worker-pool scale-out), SP1 (provider-fanout throughput vs N) |
| 8 | **chaos** | PRESENT — `helix_code/tests/stresschaos/chaos.go` + per-pkg `*_chaos_test.go` (~15 pkgs); `stress-chaos:114` | extend for new features only (SP5 kill-mid-flight, SP4 CLI crash, SP6 partial-DAG) | SP5, SP4, SP6 |
| 9 | **stress** | PRESENT — `helix_code/tests/stresschaos/stresschaos.go` + per-pkg `*_stress_test.go` (`internal/worker/worker_pool_stress_test.go`, `task/`, `agent/`, `workflow/`) | extend for new features (SP5 N≥10 parallel §11.4.85, SP4 N concurrent CLI, SP6 wide fan-out) | SP5, SP4, SP6 |
| 10 | **performance** | PRESENT — `helix_code/tests/performance/{benchmark_test.go,competitor_baseline_test.go,pprof_harness_test.go,scenarios/}`; `test-benchmark:157` | add SP1 tokens/latency (ties §11.4.141) + SP5 throughput-vs-parallelism scenarios | SP1 (tokens/latency), SP5 (throughput vs parallelism) |
| 11 | **benchmarking** | PRESENT — same as #10 + PGO `pgo-refresh:90` + HelixQA `banks/benchmarking-baselines.yaml` | add baseline rows for new features (ABSENT today) | SP1 (provider baselines), SP5 (parallelism), SP6 (flow-engine) |
| 12 | **ui** | **THIN local** — `helix_code/applications/{terminal-ui,desktop}` tests; DELEGATED HelixQA `ui_terminal_interaction_challenge.sh` | **GAP-3** → local `helix_code/tests/ui/tui_interaction_test.go` (tcell `SimulationScreen`) + §11.4.117 CV/OCR pixel-oracle for non-introspectable UI (self-validated golden-good/golden-bad) (SP7 Task A4) | SP4 (terminal interaction), SP6 (human-in-loop flow node UI) |
| 13 | **ux** | **ABSENT local** (`helix_code/tests/ux` does not exist) — DELEGATED only: HelixQA `challenges/scripts/ux_end_to_end_flow_challenge.sh` | **GAP-4** → local `helix_code/tests/ux/ux_journey_test.go` (init→list→generate→exec, captured transcript §11.4.83) (SP7 Task A3) | SP4 (end-user CLI journey), SP6 (human-in-loop UX flows) |
| 14 | **Challenges** | PRESENT — `submodules/challenges/{banks,challenges,cmd}` (`p1-f06..p2-f24`) + `helix_code/tests/e2e/challenges/` | add new banks per new feature/submodule (ABSENT today) — e.g. `challenges/banks/sp1_model_access/` | ALL SPs |
| 15 | **HelixQA banks / autonomous-QA-session** | DELEGATED + **STALE** — HelixQA `cmd/helixqa autonomous` + `banks/helixcode-*.yaml`; pin `v4.0.0-393-g4d2dcb2` (round 219 / 2026-05-19) | **GAP-5** → fetch/pull to latest + bump pointer (SP7 Phase B); wire §11.4.116 sync-channel onto `internal/helixqa/wrapper.go` (Phase C); banks for new features | ALL SPs |

> **Coverage gaps found: 6** — GAP-1 ddos-local, GAP-2 scaling-local, GAP-3 ui-thin(+pixel-oracle),
> GAP-4 ux-local, GAP-5 HelixQA-stale/no-sync-channel, GAP-6 regression-guard-registry (rows below).

---

## 3. Cross-cutting meta rows

| Row | Today's status | Target (PLANNED) | Owning SP |
|-----|----------------|------------------|-----------|
| **§1.1 paired-mutation meta** | PRESENT — `helix_code/tests/stresschaos/stresschaos_meta_test.go` (plants deadlock/leak/error-rate/below-floor/chaos-panic, asserts detection); `stress-chaos-meta:121` | every NEW harness/gate gets its own paired mutation (each Phase A harness, every `CM-*` gate) | every SP harness + gate |
| **regression-guard** | PRESENT-but-not-a-registry — `helix_code/tests/regression/{critical_paths_test.go,server_timeout_test.go}` (no `RED_MODE` polarity, no D-N rows) | **GAP-6** → standing `helix_code/tests/regression/guards/` registry with `RED_MODE` switch (§11.4.115), tag-blocking, risk-ordered first (§11.4.132) (SP7 Phase E) | every fixed defect across all SPs |

---

## 4. Per-defect regression-guard rows (GAP-6, SP7 Phase E §7)

Each guard = one source with a `RED_MODE` polarity switch: `RED_MODE=1` reproduces the historical defect on the
pre-fix artifact (the proof the guard is real); `RED_MODE=0` is the standing GREEN guard asserting absence
(§11.4.115). **All ABSENT today** (`helix_code/tests/regression/guards/` does not exist).

| Guard file (PLANNED) | Defect | RED (`=1`) reproduces | GREEN (`=0`) asserts | Owning SP |
|----------------------|--------|------------------------|-----------------------|-----------|
| `D1_completion_models_guard_test.go` | D-1 hardcoded 3-model list — `helix_agent/internal/handlers/completion.go:406` (verified present) | `/v1/completion/models` returns the static 3-item list | endpoint returns LLMsVerifier-sourced models; count ≠ hardcoded set | SP2 |
| `D2_cli_listmodels_guard_test.go` | D-2 CLI lists failed/pending as available — `cmd/cli/main.go:1361` (verified present) | a `failed`/`pending` model appears in `handleListModels` output | only `Verified ∧ score≥min` models listed | SP1 |
| `D3_loadapikeys_wired_guard_test.go` | D-3 dead `secrets.LoadAPIKeys` — `internal/secrets/loader.go:30` (verified: no prod call-site) | call-graph proof `LoadAPIKeys` unreferenced in prod path | prod startup invokes `LoadAPIKeys` (real key load) | SP1 |
| `D4_working_model_filter_guard_test.go` | D-4 filter loaded-never-applied — `adapter.go:175` (verified: `GetMinAcceptableScore` exists, `GetWorkingModels` ABSENT) | filter config loaded but output includes non-working models | filter APPLIED — non-working excluded | SP1 |
| `D5_provider_hardcode_guard_test.go` | D-5 hardcoded lists — `openai_provider.go`, `anthropic_provider.go` | provider `GetModels` returns a static literal slice | `GetModels` issues a real provider HTTP query (CONST-036) | SP1 |
| `D6_cli_agent_exec_guard_test.go` | D-6 stub returns hardcoded strings — `helix_agent/.../qwencode.go:101/115/137` | method returns canned string without exec | method `os/exec`s the real binary + surfaces real exit code | SP4 |
| `D7_replace_dev_helix_agent_guard_test.go` | D-7 `helix_code/go.mod` lacks `replace dev.helix.agent` | go.mod has no `replace`; real `clis`/`agentic` un-importable | `replace` present; a HelixCode test imports + runs the real helix_agent `clis` | SP4/SP5 |

---

## 5. Anti-delegation-bluff gates (SP7 Task A5 — PLANNED)

To stop a delegated HelixQA slot from being a config-only PASS (CONST-050(B)), each delegated row earns a
pre-build gate that asserts the HelixQA script's last run emitted captured evidence referencing a **live**
HelixCode endpoint/binary (parsing `qa-results/`), each with a paired §1.1 mutation:

| Gate (PLANNED) | Converts row | Asserts |
|----------------|--------------|---------|
| `CM-DDOS-HITS-HELIXCODE` | #6 ddos | the ddos script hit a real HelixCode endpoint with captured p50/p95/p99 |
| `CM-SCALING-HITS-HELIXCODE` | #7 scaling | scale-out exercised real `internal/worker` with a captured throughput table |
| `CM-UX-HITS-HELIXCODE` | #13 ux | the journey drove the real CLI with a captured bidirectional transcript |
| `CM-UI-HITS-HELIXCODE` | #12 ui | the TUI interaction asserted on real rendered cells / OCR self-validation |

---

## 6. N/A or delegated-by-design (honest coverage notes, §11.4.3)

- **SP1 (server/CLI-only surface):** UI/UX/DDoS/scaling are N/A-or-delegated at the SP1 layer; SP7 owns the
  cross-cutting local harnesses (Phase A). Recorded as an honest coverage note, never faked (SP1 plan §7).
- **DDoS/scaling/ux** currently have NO HelixCode-local harness — the delegated HelixQA scripts are the only
  assets and are unproven against HelixCode endpoints until the Task A5 gates land. Treat their PASS as
  provisional until evidence-backed.

---

## 7. Summary

1. The 15 types split into **strong-local** (unit, integration, e2e, security, chaos, stress, performance,
   benchmarking) and **gap** (ddos, scaling, ux, ui-thin, HelixQA-stale) plus the regression-guard registry gap.
2. **8 types are working locally today** with a real §1.1 meta-test harness backing the stress/chaos floor.
3. **4 types have local gaps** (ddos/scaling/ux ABSENT, ui THIN) — covered today only by delegated HelixQA
   scripts, which are a CONST-050(B) coverage-bluff risk until the `CM-*-HITS-HELIXCODE` gates prove they hit
   real HelixCode.
4. **HelixQA is stale** (round 219 / 2026-05-19, pin `v4.0.0-393-g4d2dcb2`) and not wired to a §11.4.116
   sync-channel — Phase B/C target.
5. **The per-defect regression-guard registry (GAP-6) is ABSENT** — D-1..D-7 guards are all PLANNED; the existing
   `tests/regression/` files are not a `RED_MODE` registry.
6. Every PLANNED cell traces to a specific SP7 task (Phase A harnesses, Phase B HelixQA bump, Phase C sync-channel,
   Phase D code-review gate, Phase E guards); nothing here is claimed done that the tree does not show.

## Sources verified 2026-06-10
- Plan: `docs/superpowers/specs/plans/2026-06-10-SP7-testing-qa-plan.md` (§1, §2, §7).
- Live tree: `helix_code/tests/` (present: automation, e2e, integration, performance, regression, security,
  stresschaos, unit; **ABSENT**: ddos, scaling, ux, ui), `helix_code/tests/stresschaos/stresschaos_meta_test.go`
  (present), `helix_code/tests/regression/` (`critical_paths_test.go`, `server_timeout_test.go` only),
  `helix_code/internal/helixqa/wrapper.go` (present), `submodules/helix_qa` pin `v4.0.0-393-g4d2dcb2`,
  `submodules/helix_agent/internal/handlers/completion.go:406` (hardcoded list present),
  `helix_code/internal/verifier/adapter.go:175` (`GetMinAcceptableScore` present, `GetWorkingModels` ABSENT).
