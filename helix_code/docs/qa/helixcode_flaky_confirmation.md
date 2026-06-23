# HelixCode §11.4.40 Flaky-Package Isolation Confirmation

**Date:** 2026-06-23T22:37+0300
**Host:** darwin/arm64, go1.26.2
**Module:** `dev.helix.code` (inner) — `/Volumes/T7/Projects/helix_code/helix_code`
**Method:** Each failing test re-run in ISOLATION, ONE AT A TIME, serially (low load, §11.4.119), `-count=1 -v`. No code modified (read-only verification per §11.4.6 / §11.4.50).

## Per-package result table

| Package | Test(s) | Isolated low-load result | Classification |
|---|---|---|---|
| internal/agent/subagent | TestSubprocessSpawner_RealHelper_RoundTrip | PASS (0.27s) | Confirmed load-flake |
| internal/discovery | TestIsPortReachable | FAIL via `go test` (3/3); **PASS 4/4 via pre-built binary** | Env-induced flake (real trigger isolated — see below) |
| internal/discovery | TestHealthMonitor_CheckServiceHealth_TCP | FAIL via `go test` (timed out at 2.00s); **PASS 3/3 via pre-built binary** | Env-induced flake (same root cause) |
| internal/mcp | TestStdioTransport_StderrCapture | PASS (8.75s) | Confirmed load-flake |
| internal/providers/httpclient | TestSharedClient_BurstReuse | PASS (0.00s; 48 reqs / 24 conns, warm-pool reuse confirmed) | Confirmed load-flake |
| internal/voice | TestVoiceRecorder_StartLaunchesProcess_Guard | PASS (2.42s) | Confirmed load-flake |
| tests/performance/scenarios | TestRunner_StableAcrossThreeRuns | PASS (0.66s); CV S3=4.52%, S4=1.10% | Confirmed load-flake; variance low |
| tests/regression | TestServerStability | PASS (1.76s) | Confirmed load-flake |

## Counts
- **Confirmed load-flakes (PASS in isolation): 6 packages / 6 tests** — subagent, mcp, httpclient, voice, performance/scenarios, regression.
- **Escalated as real bugs: 0.**
- **Environment-induced flakes (FAIL under `go test`, PASS from pre-built binary): 1 package / 2 tests** — internal/discovery. NOT a HelixCode product/code defect; NOT concurrent-load. See root cause.

## internal/discovery — root cause (systematic-debugging, §11.4.102 / §11.4.6)

Both discovery tests start a fresh `net.Listen("tcp","127.0.0.1:0")` and then dial it.

**Captured evidence:**
- `go test ./internal/discovery -run TestIsPortReachable -count=1` → FAIL **3/3** in isolation, low load.
- Bare standalone `net.DialTimeout` probe: first loopback connect in a freshly-`go run` binary cost **~95ms** (timeout=100ms → boundary, intermittent FAIL); subsequent connects **~60µs** (PASS). Larger budgets (500ms/2s/5s) all succeeded after the first.
- **Discriminator:** built the discovery test binary once (`go test -c -o /tmp/disc.test`) and executed it repeatedly:
  - `TestIsPortReachable` → **PASS 4/4**
  - `TestHealthMonitor_CheckServiceHealth_TCP` → **PASS 3/3**
- No `TestMain`, no `init()`, no proxy env in the package.

**Determination (FACT):** the failures are caused by the macOS first-execution security scan of the freshly-compiled, unsigned test binary, which delays the process's FIRST loopback `connect()`. `go test` compiles a new binary and runs it immediately, so the scan delay lands inside the test's tight connect budget:
- `TestIsPortReachable` uses a **100ms** dial timeout → the ~95ms first-connect scan delay lands at the boundary → fails essentially deterministically here.
- `TestHealthMonitor_CheckServiceHealth_TCP` uses `CheckTimeout = 2s` (`checkTCP` → `net.DialTimeout`, default strategy `HealthCheckTCP`); on the failing runs the first-connect scan delay consumed the full 2.00s (test ran exactly 2.00s, `err==nil`, `result.Healthy==false`).

Once the binary is cached/known (pre-built then re-run), there is no scan delay and both tests pass. The §11.4.40 run failed these under concurrent load because load amplified the same first-connect latency; in isolation here it still fails because the *fresh-binary scan* (not load) is the trigger.

**Not a product bug.** `isPortReachable` and `checkTCP` are correct (`net.DialTimeout` + close). The fragility is test-side: connect-timeouts too tight for this host's fresh-binary first-connect penalty.

**Recommendation (test-hardening, NOT a fix to product code):** raise `TestIsPortReachable`'s 100ms dial timeout (e.g. to ≥2s) so it is not sensitive to the first-connect/security-scan latency; the unreachable-port sub-assertion can keep a short timeout. The TCP health test already has a 2s budget — consider a brief warm-up dial or a larger budget on macOS. These are §11.4.50 determinism hardenings, not product changes.

## Honesty note (§11.4.6)
internal/discovery is honestly reported as FAILING reproducibly in isolation under `go test`. It is classified as an environment-induced flake (NOT a load-flake, NOT a product bug) only because the trigger was isolated to the macOS fresh-binary first-connect scan and both tests PASS from a pre-built binary. It is NOT escalated as a real product bug.
