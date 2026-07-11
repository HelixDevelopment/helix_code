# D5 — Challenges + helix_qa Test-Infrastructure Audit

Auditor: read-only. Anti-bluff (§11.4.6): FACT + cited evidence; `UNCONFIRMED:` where not directly proven.

Paths used (note: task-given `$REPO` was the meta-repo root
`/home/milos/Factory/projects/tools_and_research/helix_code`, NOT the inner Go
app `helix_code/`):
- CH = `$REPO/submodules/challenges` (digital.vasic.challenges, vasic-digital/Challenges-class submodule)
- QA = `$REPO/submodules/helix_qa` (digital.vasic.helixqa, git@github.com:HelixDevelopment/HelixQA.git — confirmed via `.gitmodules`)
- HC = `$REPO/helix_code` (inner Go app, module `dev.helix.code`)

---

## 1. Challenges submodule (`submodules/challenges`, module `digital.vasic.challenges`, go 1.25.0)

### Structure
- `README.md` / `ARCHITECTURE.md` / `CLAUDE.md` / `AGENTS.md` / `CONSTITUTION.md` (+ HTML/PDF siblings) — governance in place.
- `pkg/` (18 packages): `assertion`, `bank`, `challenge`, `container`, `env`, `httpclient`, `i18n`, `infra`, `logging`, `metrics`, `monitor`, `panoptic`, `plugin`, `registry`, `report`, `runner`, `userflow` — a real challenge-execution framework (registry + topological ordering, 16 built-in assertion evaluators, Markdown/JSON/HTML reporters, WebSocket live-monitor, Prometheus metrics, `userflow` sub-framework with 8 adapter interfaces / 21 implementations covering browser/mobile/desktop/API/gRPC/WebSocket/build/recorder).
- `banks/` — JSON/YAML challenge-bank definitions (`banks/examples/*.json`, `banks/yole/*.yaml` with `fixtures/`, `feature-coverage/`).
- `challenges/scripts/` — 24 real bash Challenge scripts, e.g. `bluff_scanner_challenge.sh`, `mutation_ratchet_challenge.sh`, `chaos_failure_injection_challenge.sh`, `ddos_health_flood_challenge.sh`, `scaling_horizontal_challenge.sh`, `helixd_control_plane_challenge.sh`, `etcd_quorum_challenge.sh`, `helixllm_coder_live_e2e_challenge.sh`, `ux_end_to_end_flow_challenge.sh`, `ui_terminal_interaction_challenge.sh`, `no_suspend_calls_challenge.sh`, `host_no_auto_suspend_challenge.sh`, `anchor_manifest_challenge.sh`, `agentic_subagents_challenge.sh`, `persistent_memory_challenge.sh`.
- `cmd/userflow-runner/main.go` — a Go runner entry point.
- `p1-f06-…` through `p2-f24-…` — per-feature challenge directories (cataloguing CLI-agent reference features: MCP lifecycle, background tasks, plan mode, slash commands, skills, session resume, multi-provider, LSP, sandboxed shell, subagent team, OpenTelemetry, smart file editing, no-flicker rendering, ask-user-question, theme system, Codex approval modes, Aider git auto-commit, Cline browser tool, Codex project memory).

### Build / compile
```
cd submodules/challenges && go build ./...   -> exit 0, no output
cd submodules/challenges && go vet ./...     -> exit 0, no output
```
Clean build + vet. `go.mod` requires `digital.vasic.containers` via a local `replace ../containers` (CONST-051(C) grouped layout, satisfied — sibling exists at `submodules/containers`).

### Realness
Read `bluff_scanner_challenge.sh` and `helixllm_coder_live_e2e_challenge.sh` in full/partial:
- `bluff_scanner_challenge.sh`: two-phase — (1) scanner self-test against hand-crafted fixtures with known verdicts (`scripts/anti-bluff/tests/run-fixtures.sh`), (2) tree-wide scan (`scripts/anti-bluff/bluff-scanner.sh --mode all`). Real self-verifying gate, not a stub.
- `helixllm_coder_live_e2e_challenge.sh` (394 lines): sends a REAL HTTP request to a live llama.cpp/OpenAI-compatible coder endpoint (`HELIXLLM_CODER_URL`, default `localhost:18434`), extracts the returned code, statically scans it for bluff markers (TODO/simulate/NotImplementedError/"for now"), then ACTUALLY EXECUTES it via `python3` against 3 fixed + 1 freshly-randomised assertion vector (defeats hardcoded lookup-table cheats). Captures full bidirectional transcript (`request.json`, `response.json`, `solution.py`, `harness_stdout.txt`, `SUMMARY.txt`). Implements a paired-mutation (`--anti-bluff-mutate`) that substitutes a deliberately-wrong stub and must be caught (exit 99) — self-validating per §11.4.107(10). Honest `SKIP-OK` when toolchain/coder/model unreachable (exit 0). This is genuine, sophisticated anti-bluff engineering, not a placeholder.

### Wired to run against real infra?
Confirmed via **captured evidence in `docs/qa/`** at the meta-repo root (not inside the Challenges submodule itself — the submodule stays project-agnostic per CONST-051(B), the parent project supplies the run + evidence capture):
- `docs/qa/helixllm_coder_bench_live_20260709_*/bench_all_verdict.json` — real concurrency-level benchmark data (p50/p95/p99 latency, throughput req/s and tok/s, real token counts, e.g. concurrency=100 → 20/20 ok, p50=1535ms, throughput=271 tok/s) — this is REAL measured data, not fabricated placeholders.
- `docs/qa/phase1_fullhttp_e2e_*/0{1..5}_*.txt` (41 run directories, spanning 2026-07-08 through 2026-07-11) — real captured HTTP transcripts (401 no-auth, 200 OpenAI chat-completions, 200 Anthropic messages, 200 OpenAI tool_calls, 200 Anthropic tool_use).
- The actual **producer** of `bench_all_verdict.json` / `golden_bad_verdict.json` is `submodules/helix_qa/cmd/helixqa-verify-coder-bench/main.go` (found by content grep, not the Challenges submodule) — i.e., the Challenges submodule provides the generic framework/scripts, and HelixQA is the concrete HelixCode-specific driver that actually executes and produces the evidence found under `docs/qa/`.

**Verdict: REAL.** The Challenges submodule is a substantial, working, decoupled framework; its own challenge scripts (bluff scanner, HelixLLM coder E2E, etc.) build/run and are honestly SKIP-OK-gated. It is consumed (not merely present) — HelixQA's verify-coder-* binaries and the meta-repo's `docs/qa/` runs are the proof of actual execution against live HelixCode/HelixLLM endpoints.

---

## 2. helix_qa submodule (`submodules/helix_qa`, module `digital.vasic.helixqa`, go 1.26)

### Remote / ownership
`.gitmodules` at meta-repo root: `url = git@github.com:HelixDevelopment/HelixQA.git`, `branch = main` — confirmed owned by HelixDevelopment org (CONST-047/§11.4.35 in scope).

### Structure
- Governance files present (README/CLAUDE/AGENTS/CONSTITUTION/CHANGELOG + HTML/PDF).
- `docs/test-coverage.md` — a genuine CONST-050(B) 15-row test-type coverage ledger (round 219, dated 2026-05-19), every row marked `FILLED` with a concrete asset path and evidence shape (unit/integration/e2e/full-automation/security/ddos/scaling/chaos/stress/performance/benchmarking/ui/ux/Challenges/**autonomous-QA-session**).
- `pkg/` — 40+ packages incl. `autonomous`, `orchestrator`, `session`, `issuedetector`, `evidence`, `screenshot`, `recordingqa`, `replay`, `regression`, `vision`, `visionnav`, `navigator`, `discovery`, `testbank`, `ticket`, `reporter`, `challengegen`, `agent`, `learning`, `training`, `maestro`, `conduit`, `bridge(s)`, `gpu`, `gst`, `audio`, `video`, `capture`.
- `cmd/` — 40+ binaries: `helixqa` (main CLI with `run|list|report|autonomous|version` subcommands — confirmed via `grep` in `main.go`), `helixqa-verify-coder-{bench,chaos,concurrency,ddos,memory,race,race-llm}`, `helixqa-verify-{a2a,embeddings,mcp-gateway,netprov,rag,tesseract,translate-nllb,vision,whisper}`, `helixqa-concrete-runner`, `helixqa-bridge`, `helixqa-conduit-{demo,monitor}`, capture/vision tooling (`helixqa-omniparser`, `helixqa-uitars`, `helixqa-dreamsim`, `helixqa-lpips`, `helixqa-frida-bridge`, `helixqa-kmsgrab`, `helixqa-x11grab`), `recording-analyzer`, `ocu-probe`/`ocu-dispatch-test`, `qa-audio-probe`.
- `banks/` — 60+ YAML/JSON test banks, including **HelixCode-specific** banks: `helixcode-task-workflow.yaml`, `helixcode-admin-operations.json`, `helixcode-lsp.yaml`, `helixcode-generate-e2e.yaml`; and **HelixLLM-specific** banks: `helixllm_coder.yaml` (+ `_concurrency`/`_memory` variants), `helixllm_embeddings.yaml`, `helixllm_network_provider.yaml`, `helixllm_translate_nllb.yaml`. Plus generic cross-project banks (`nexus-*`, `ocu-*`, `openclawing2-*`, `catalog_*`, `fixes-validation-*`).
- `challenges/` — its own Challenge scripts (built on the Challenges framework), incl. `helixqa_orchestrator_challenge.sh` (8-phase orchestrator-surface validator with built-in §1.1 paired mutation, per README banner round 219).

### Build / compile
```
cd submodules/helix_qa && go build ./...   -> exit 0, no output (clean)
cd submodules/helix_qa && go vet ./...     -> 8 findings, all in pkg/challengegen/generator*.go —
    real (mundane) "passes lock by value" / "copies lock value" warnings on a
    struct embedding challenge.Result (which itself embeds sync.Mutex). Genuine
    code, genuine minor smell — NOT a build failure, NOT evidence of bluffing;
    if anything it is evidence the code is real (a stub package would not trip
    a copylocks vet finding).
```

### Realness — bank content check
Read `banks/helixcode-task-workflow.yaml` in full header + first test cases: it targets real HelixCode server routes (`helix_code/internal/server/server.go` `tasks.*`/`projects.*`/`workflows/*`), asserts the real 401-without-auth boundary against real route paths, and HONESTLY documents a coverage gap ("authenticated CRUD + workflow-execution flow is an honest _skip because a JWT cannot be minted in this offline bank — register is broken on a fresh DB per HXC-029"). This is genuine, specific, HelixCode-aware QA engineering with an honest gap disclosure — not a stub/placeholder bank.

### Wired to run against real HelixCode — direct evidence
- `qa-results/helixcode_coder_race/self_validate_001_golden_good_verdict.json` (inside the HelixQA submodule itself, mtime 2026-07-11 21:53 — the most recent artefact in the whole tree): real concurrent-read HTTP results against `http://localhost:8082/api/v1/llm/providers` and `.../llm/models` — 5/5 ok, status_code 200, real body sizes (1006 bytes), `valid_json: true`, `struct_consistent: true`. Sibling `self_validate_001_golden_bad_verdict.json` present alongside — confirms the self-validating golden-good/golden-bad analyzer pattern (§11.4.107(10)) is actually exercised, not merely documented.
- `qa-results/` also holds 15+ other real recent run directories (`helixllm_coder_bench`, `_race`, `_memory`, `_ddos`, `_chaos`, `_concurrency`, `_mcp_gateway`, `_a2a`, `_network_provider`, `_rag`, `_embeddings`, `_tesseract`, `_whisper`, `_translate_nllb`, `_vision`), timestamped 2026-07-08 through 2026-07-11 — i.e., these Challenges/banks are run repeatedly and recently, not a one-off historical artefact.
- The meta-repo's `docs/qa/helixllm_coder_bench_live_*` / `phase1_fullhttp_e2e_*` directories (41 of the latter) are further downstream copies/exports of these live runs.

### Integration into HelixCode itself
`helix_code/internal/helixqa/wrapper.go` — the inner HelixCode Go application directly imports and embeds HelixQA as a first-class server feature:
```go
hqaConfig "digital.vasic.helixqa/pkg/config"
hqaEvidence "digital.vasic.helixqa/pkg/evidence"
hqaOrchestrator "digital.vasic.helixqa/pkg/orchestrator"
"digital.vasic.helixqa/pkg/reporter"
hqaScreenshot "digital.vasic.helixqa/pkg/screenshot"
```
`internal/server/qa_handlers.go` and `internal/server/screenshot_handlers.go` also import it — meaning HelixCode's HTTP server exposes QA-session orchestration as a live product feature (session state, phase/progress tracking, autonomous-result reporting), not just an external test tool. `helix_code/tests/e2e/challenges/cmd/runner/main.go` exists (a separate, home-grown E2E harness distinct from `digital.vasic.challenges`) and the whole `tests/e2e/challenges/...` package tree compiles cleanly (`go build` exit 0).

**Verdict: REAL and ACTIVELY RUN.** helix_qa is a large, mature, HelixCode-aware anti-bluff QA framework: it builds cleanly, its coverage ledger claims (and evidences) all 15 test types FILLED, its HelixCode/HelixLLM-specific banks make real assertions against real server routes with honest gap disclosure, and its `qa-results/` directory contains dozens of dated real-HTTP-evidence runs as recent as the most recent mtime in the entire audited tree.

---

## 3. Overall assessment

Both submodules are genuine anti-bluff test infrastructure, not scaffolding-only:
1. Both build and vet cleanly (Challenges: 0/0; HelixQA: 0 build errors, only benign copylocks vet notes).
2. Both ship real, non-trivial runner/orchestrator entry points (`cmd/userflow-runner`, `cmd/helixqa` with `run|list|report|autonomous|version`, 30+ `helixqa-verify-*` binaries).
3. Challenge scripts are self-validating (golden-good/golden-bad, paired `--anti-bluff-mutate`, honest `SKIP-OK` on unreachable targets) rather than pattern-matching stubs.
4. **Wired, not just present**: `qa-results/` (inside helix_qa) and `docs/qa/` (meta-repo root) contain dozens of dated, real-HTTP-evidence run directories from 2026-07-08 through 2026-07-11 (most recent mtime in the whole audited tree), proving these suites are actually executed against live HelixCode (`localhost:8082`) and HelixLLM (`localhost:18434`) endpoints — this satisfies §11.4.50(B) (deterministic, re-runnable, evidence-producing) rather than being present-but-unrun.
5. HelixCode's own server (`internal/helixqa/wrapper.go`, `internal/server/qa_handlers.go`) embeds HelixQA's orchestrator directly, so QA-session execution is a first-class product capability, not merely an external CI-adjacent tool (consistent with Rule 1 — no CI/CD pipelines; these are manually/agent-invoked).

No stub or unrunnable component was found in either submodule during this audit. One caveat: this audit did NOT execute the shell Challenge scripts or `go test` (task scope was build/compile + read-only inspection of existing evidence), so per-script current-session pass/fail was not independently re-verified — the REAL/PARTIAL/STUB verdicts above rest on (a) clean compilation, (b) direct reading of script logic, and (c) pre-existing captured evidence artefacts already in the tree, dated as recently as 2026-07-11.
