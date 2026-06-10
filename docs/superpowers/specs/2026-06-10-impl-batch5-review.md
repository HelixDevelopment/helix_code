# Wave-5 Code-Review Report (§11.4.125 / §11.4.134)

**Reviewer:** Independent code-review subagent (Opus 4.8)
**Date:** 2026-06-10
**Scope:** UNCOMMITTED wave-5 batch — SP4-cont (`submodules/helix_agent/internal/clis`) + SP7 (`helix_code/tests/{ddos,scaling,ux,ui}` + `scripts/{gates,tests}/*`)
**Mandate:** read-only verification; no edits/commits/push. Mutations applied during review were reverted (restore confirmed byte-identical, working tree clean).

---

## VERDICT: **GO**

Zero BLOCKING findings. 2 NON-BLOCKING notes (advisory). All required builds/tests/gates/meta-tests run GREEN; every §1.1 mutation FAILed the guard as required; restores confirmed clean.

- BLOCKING (must-fix): **0**
- NON-BLOCKING (advisory): **2**

---

## 1. Builds + vet (PASS)

```
helix_agent: go build ./internal/clis/...   -> EXIT=0  (PASS)
helix_agent: go vet   ./internal/clis/...   -> EXIT=0  (PASS)
helix_code:  go vet ./tests/ddos/... ./tests/scaling/... ./tests/ux/... ./tests/ui/...  -> EXIT=0 (PASS)
helix_code:  go vet (re-run, full SP7)      -> VET_OK  (PASS)
```

## 2. SP4-cont RED→GREEN + §1.1 mutations (PASS)

```
helix_agent: go test ./internal/clis/... -count=1            -> ok (all packages PASS)
GREEN guards (default):
  TestD12_RealExec_ExecsRealBinary               -> PASS (qwencoder/github_copilot/gemini_assist exec real fake-binary, surface stdout marker)
  TestD12_RealExec_AbsentBinaryIsHonestError     -> PASS (absent binary => honest error)
  TestD12_HonestError_NoFakeSuccess (40 agents)  -> PASS (each returns honest error, never the literal)
  TestD11_IsAgentTypeAvailable_RealLookPath      -> PASS (real exec.LookPath; Kiro/Continue/HelixAgent/Cursor unavailable)
RED reproductions (default => SKIP; PIN_STUB_BLUFF=1 => RUN):
  TestD12_StubsAreBluffs        default -> SKIP (SKIP-OK: ATM-SP4-D12);  PIN_STUB_BLUFF=1 -> PASS (bluff gone on fixed artifact)
  TestD11_AllowAllTypesBluff    default -> SKIP (SKIP-OK: ATM-SP4-D11);  PIN_STUB_BLUFF=1 -> PASS
§1.1 mutation A (executeQwenCoder real-exec -> "Qwen Coder execution completed" literal):
  -> TestD12_RealExec_ExecsRealBinary/qwencoder FAIL ("did NOT exec the agent binary … BLUFF-003 reintroduced?")  ✓ guard fires
  -> restore -> ok  ✓
§1.1 mutation B (executeKiro honest-error -> "Kiro execution completed" fake success):
  -> TestD12_HonestError_NoFakeSuccess/kiro FAIL ("returned success with no headless CLI … BLUFF-003")  ✓ guard fires
  -> restore -> ok  ✓
Bluff-literal scan (non-test source):
  grep 'execution completed' internal/clis/*.go (non-test) -> only //-comment CITATIONS of the removed bluff (lines 916,1071)  ✓ no live literal
  grep 'simulated|for now|TODO implement|in production this would' (non-test) -> only //-comment lines ("Honest error for now.", quote of "For now, allow all types")  ✓ constitution smoke excludes //-comment citations
```

The D-11 split is correct and honest: `agentDefaultCommand` maps 8 agents to confirmed CLI binaries (aider/claude/codex/cline/openhands + qwen/copilot/gemini) and 40 to empty-command honest-error class; `IsAgentTypeAvailable` is a real `exec.LookPath` honoring `HELIX_AGENT_BIN_<TYPE>`. The 3 real-exec D-12 methods go through `runCLIAgent` → real `exec.CommandContext` with real exit-code + CombinedOutput (BLUFF-003 satisfied).

## 3. §11.4.120 reconciled tests — NOT weakened to tautologies (PASS)

`instance_manager_test.go` `TestInstanceManager_IsAgentTypeAvailable` was reconciled, not faked:
- old `assert.True(...TypeKiro/TypeContinue)` rewritten to `assert.False(...)` matching the corrected honest contract (those agents genuinely have no resolvable binary);
- real-CLI types proven available by injecting a real fake binary via `HELIX_AGENT_BIN_<TYPE>` (not a hardcoded true);
- `CreateInstance` + benchmark reconciled to inject a fake aider binary since `CreateInstance` now gates on the real availability check.
```
TestInstanceManager_IsAgentTypeAvailable -> PASS
TestInstanceManager_CreateInstance       -> PASS
```

## 4. SP7 harnesses + meta-tests (PASS)

```
helix_code: go test ./tests/ddos/... ./tests/scaling/... ./tests/ux/... ./tests/ui/... -count=1
  ok dev.helix.code/tests/ddos      7.619s
  ok dev.helix.code/tests/scaling   6.205s
  ok dev.helix.code/tests/ux       18.139s
  ok dev.helix.code/tests/ui        0.998s
Go meta-tests (each plants a defect, asserts harness DETECTS it):
  ddos:    Detects5xxStorm / DetectsLatencyBomb / DetectsNoServedResponses / LimiterModeDetectsNoRefusals / PositivePathWritesEvidence -> all PASS
  scaling: DetectsFlatThroughput / DetectsDegradation / RejectsBelowFloor / PositivePathWritesEvidence -> all PASS
  ux:      DetectsCannedString / DetectsLeakedID / DetectsEmptyAndTerse / DetectsDivergence / PositivePathWritesEvidence -> all PASS
  ui:      DetectsBlankRender / DetectsDeadKeyHandler / DetectsLeakedID / PositivePathWritesEvidence -> all PASS
Bash gates+meta-tests CONST-068 (bash -n on all 8): ALL OK (shebang #!/usr/bin/env bash)
Bash meta-tests (gate FAILs bluff, PASSes real evidence):
  CM-DDOS-HITS-HELIXCODE   META-TEST PASS (EXIT=0)
  CM-SCALING-HITS-HELIXCODE META-TEST PASS (EXIT=0)
  CM-UX-HITS-HELIXCODE     META-TEST PASS (EXIT=0)
  CM-UI-HITS-HELIXCODE     META-TEST PASS (EXIT=0)
```

Verified the specific anti-bluff claims:
- **ux really shells the cli binary** — `ux_journey_test.go:TestUX_CLIJourney_RealBinary` builds the real `./cmd/cli` then `exec.CommandContext`s it, asserting a per-run nonce echoes through real stdout (BLUFF-003 path); honest SKIP only when `go` toolchain absent or build fails.
- **scaling shows real N-scaling numbers (not hardcoded)** — `scaling_harness.go` drives the REAL `internal/worker.WorkerPool` across N={1,2,4,8} via real `RegisterWorker`/`AssignTask`/`ReleaseWorker`/`GetPoolStats`; `GainAtMaxN` is computed from MEASURED throughput-per-second, FAILing if gain < 1.5x (flat) or throughput degrades. Honest boundary documented (in-process concurrent-capacity scale-out; SSH horizontal scale-out is a separate integration-tagged path that SKIPs when no SSH workers configured).
- **ddos 429 assertion correctly gated OFF** — `ddos_harness.go:18-19,288-294`: `internal/server` wires no rate-limit middleware today, so the 429-refusal assertion ships behind `DDOS_EXPECT_RATELIMIT=1` (OFF by default). Default assertions are graceful-degradation (zero 5xx, real served body-marker hits > 0, bounded p99). It never asserts a 429 the code cannot produce. The integration driver (`//go:build integration`) boots the REAL `server.New` with real PG/Redis and honest SKIP when infra unreachable.
- **ui** — `ui_harness.go` renders REAL tview/tcell components into a `tcell.SimulationScreen` cell grid and reads back exact composited cells (genuine pixel-equivalent evidence); Fyne native-pixel layer honestly tracked operator-attended.

## 5. Anti-bluff + scope (PASS)

- Honest errors genuinely returned (not swallowed): `noHeadlessCLIError` returns a real `error`; `runCLIAgent` surfaces non-zero exit / spawn failure as a real wrapped error with the binary's actual output. ✓
- No NEW `simulated|for now|TODO implement` in production source — only pre-existing-style //-comment citations describing the removed bluff (constitution smoke excludes these). ✓
- §11.4.120 reconciled tests not weakened to tautologies (see §3). ✓
- **No go.mod/go.sum edits** in either repo. ✓
- SP4-cont touches only `helix_agent/internal/clis` (3 files). ✓
- SP7 touches only `helix_code/tests/{ddos,scaling,ux,ui}/` + `scripts/{gates,tests}/`. ✓ `qa-results/` is gitignored. ✓
- No secrets logged — only test fixtures: `TEST_PG_PASSWORD` default "helix" (local stack), empty Redis pw, JWT literal explicitly `"ddos-flood-test-secret-not-a-real-credential"`. These are unit/integration test creds, permitted. ✓
- Working-tree quiescence (§11.4.84): after mutation experiments, restore is byte-identical; mutation-residue scan finds NO `MUTATION`/`always pass` markers. ✓

---

## NON-BLOCKING advisory notes

**N1 (advisory) — RED reproduction polarity is "skip-on-fixed / pass-on-pinned", not literal RED-FAIL-on-broken-artifact.**
`TestD12_StubsAreBluffs` / `TestD11_AllowAllTypesBluff` (PIN_STUB_BLUFF=1) PASS on the fixed artifact via early-return-on-honest-error (`if err != nil { return // bluff gone }`). This is a valid §11.4.115 polarity-switch pattern — they would FAIL only if a stub re-emitted the literal — and the standing GREEN guards (`TestD12_RealExec_*`, `TestD12_HonestError_*`, `TestD11_IsAgentTypeAvailable_RealLookPath`) plus the confirmed §1.1 mutation FAILs are the real bug-catchers. No action required; noted only because the prompt framed the RED tests as "SKIP on fixed artifact" — under PIN they RUN and pass rather than skip, which is the correct §11.4.115 semantics.

**N2 (advisory) — `TestD12_StubsAreBluffs/realexec/github_copilot` took ~25s under PIN_STUB_BLUFF=1** because the host has a real `copilot` binary on PATH that `runCLIAgent` actually exec'd (the test injects no fake binary in the RED path, so the real CLI ran to its own timeout). Harmless (the reproduction still passes) but slow; if the suite runs in CI on a host with a real `copilot`/`qwen`/`gemini`, consider bounding via the instance `Config.Timeout` or injecting a fast fake in the RED path. Not a correctness defect.

---

## Captured build/test PASS-FAIL summary

```
helix_agent  go build ./internal/clis/...                         PASS
helix_agent  go vet   ./internal/clis/...                         PASS
helix_agent  go test  ./internal/clis/... -count=1                PASS
helix_agent  D11/D12 GREEN guards (default)                       PASS
helix_agent  RED reproductions (PIN_STUB_BLUFF=1)                 PASS (run) / SKIP (default)
helix_agent  §1.1 mutation A (qwen real-exec->literal)            FAIL-as-required, restore PASS
helix_agent  §1.1 mutation B (kiro honest-error->fake)            FAIL-as-required, restore PASS
helix_agent  reconciled instance_manager_test.go                  PASS
helix_code   go vet ./tests/{ddos,scaling,ux,ui}/...              PASS
helix_code   go test ./tests/{ddos,scaling,ux,ui}/... -count=1    PASS
helix_code   go meta-tests (all 4 packages)                       PASS (each detects its planted defect)
scripts      bash -n  (8 gate+meta scripts, CONST-068)            PASS
scripts      4 bash meta-tests (CM-{DDOS,SCALING,UX,UI})          PASS (gate FAILs bluff, PASSes real)
scope        no go.mod/go.sum edits; paths confined; no secrets   PASS
```

**FINAL VERDICT: GO** — 0 blocking, 2 advisory. Batch is anti-bluff-sound and ready to proceed to pre-build sweep + main build.
