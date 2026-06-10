# SP5-D7-part-2 — Independent Code Review (§11.4.142 / §11.4.125)

| | |
|---|---|
| Revision | 1 |
| Created | 2026-06-10 |
| Last modified | 2026-06-10 |
| Status | active |
| Reviewer | Independent code-review subagent |
| Scope | Uncommitted SP5-D7-part-2 change (helix_code go.mod/go.sum + internal/agentbridge) |

## Table of contents

- [Verdict](#verdict)
- [Change under review](#change-under-review)
- [Verification results](#verification-results)
- [Findings](#findings)
- [Honesty assessment (D-7 genuinely closed?)](#honesty-assessment-d-7-genuinely-closed)
- [Ordered must-fix](#ordered-must-fix)
- [Evidence summary](#evidence-summary)

## Verdict

**GO-WITH-FIXES** — the change is real, correct, builds clean, the agentbridge proves a genuine cross-module HTTP round-trip, and no dep-bump regression was introduced. The one must-fix is a scope/quiescence hygiene item (§11.4.84): an UNRELATED uncommitted change exists inside `submodules/helix_agent` that must NOT be swept into the same commit as the helix_code go.mod/agentbridge change. The technical change itself is clean; the verdict is GO-WITH-FIXES only to guard the commit boundary.

## Change under review

- `helix_code/go.mod`: `+ require dev.helix.agent v0.0.0-…` + `replace dev.helix.agent => ../submodules/helix_agent`. MVS-driven transitive bumps: pgx 5.7.6→5.9.2, lib/pq 1.10.9→1.12.3, go-redis 9.17.2→9.18.0, otel otlptracehttp/stdouttrace 1.30→1.40, x-crypto 0.49→0.52 / x-net 0.52→0.54 / x-text 0.36→0.37 / x-sys 0.43→0.45 / x-term 0.41→0.43, fatih/color 1.18→1.19, golang-jwt/v5 5.2.1→5.3.1, plus indirects (go-colorable, segmentio/asm, go.uber.org/atomic added, multierr 1.10→1.11).
- `helix_code/go.sum`: matching checksum updates.
- New `helix_code/internal/agentbridge/{bridge.go,bridge_test.go}`: `VerifierBridge` wraps the real `dev.helix.agent/pkg/sdk/go/verifier.Client`; the test round-trips `VerifyModel` against an `httptest` server.

## Verification results

All commands run read-only against the working tree (no edit/commit/push).

1. **Build stability (load-bearing) — PASS.** `cd helix_code && go build ./...` → `BUILD_EXIT=0`. Only benign macOS `ld: warning: ignoring duplicate libraries: '-lobjc'` on the three Fyne/cgo `applications/*` targets. **gin stayed `v1.12.0`** (`go list -m github.com/gin-gonic/gin` → `v1.12.0`). `go mod verify` → `all modules verified`. **No dependency conflict**: `go mod tidy` is a NO-OP — `go.mod` and `go.sum` are byte-identical before/after (`GOMOD_CLEAN` / `GOSUM_CLEAN`), proving the `replace`-driven MVS resolution is already minimal and consistent.

2. **agentbridge real round-trip — PASS (deterministic).** `go test ./internal/agentbridge/... -v -count=1` → `--- PASS: TestVerifierBridge_RealRoundTrip`; `-count=3` → `ok` (stable, no flake). Cross-module import PROVEN: `go list -deps ./internal/agentbridge | grep dev.helix.agent` → `dev.helix.agent/pkg/sdk/go/verifier`. The round-trip is GENUINELY HTTP, not a mock-only assertion: the real SDK's `VerifyModel` (`submodules/helix_agent/pkg/sdk/go/verifier/client.go:74`) calls `doRequest(... "POST", "/api/v1/verifier/verify" ...)` which at `client.go:363` builds `http.NewRequestWithContext` and at line 373 executes `c.httpClient.Do(req)` against the test's `httptest.NewServer`. The test asserts on the wire: method `POST`, path `/api/v1/verifier/verify`, `Authorization: Bearer test-key-123`, the JSON-decoded request body, AND the decoded `agentverifier.VerificationResult` type — real cross-module type interchange. The `httptest` fixture is allowed for a unit test (the SDK client itself is not mocked).

3. **No regression — PASS (vs known-good).** Broad `go test ./internal/... ./cmd/... -count=1`: **144 ok, 1 FAIL package** (`internal/cognee`). The 3 failing subtests — `TestProbeAMDGPU_ParsesRocmSmiJSON`, `TestProbeAMDGPU_HandlesMultiCard`, `TestProbeAppleGPU_HandlesWhitespaceVariants` — fail with the probe subprocess returning `signal: killed` → `-1` honest sentinel under heavy parallel load. **Proven to be the documented cognee GPU-probe load-flake, NOT a dep-bump regression**: rerun in isolation `go test ./internal/cognee/ -run '…' -count=1 -p 1` → `ok 0.753s`. This is the exact caveat behind recent commit `43521764 fix(cognee): HXC-064 — AMD-GPU parser tests load-robust`. The failure surface touches `os/exec` of `rocm-smi`/`ioreg` under load — zero overlap with the bumped deps (pgx/redis/otel/crypto/net/text/jwt). Baseline was 141 ok; this run is 144 ok (agentbridge among the 3 net-new green), with only the pre-existing flake.

4. **Scope — PASS for helix_code; FINDING for submodule.** `git diff --stat` shows ONLY `helix_code/go.mod` + `helix_code/go.sum` changed; new untracked tree is `helix_code/internal/agentbridge/` only. **`submodules/helix_agent` is NOT modified by this change** at the parent-pointer level: `git submodule status` shows clean ` ` prefix (recorded SHA `7c89410c` == HEAD). No secret is logged anywhere in the new code (the test's `test-key-123` is a fixture token, asserted but never a real credential). HOWEVER — see Finding F1 — the helix_agent working tree DOES carry an unrelated uncommitted change (`M internal/clis/agents/claude_code/claude_code.go` + new `claude_code_pin_test.go`, mtimes 16:32–16:33, a separate D-11/D-12 stream), which must not be co-committed.

5. **Honesty — PASS.** D-7 is genuinely closed; the agentbridge is NOT a token import (see below).

## Findings

**F1 (must-fix, §11.4.84 quiescence / CONST-051) — unrelated in-flight change inside the submodule.** `submodules/helix_agent` has uncommitted work in a DIFFERENT area (`internal/clis/agents/claude_code/`), part of a separate CLI-dispatch stream, not part of SP5-D7-part-2. Because the parent shows `submodules/helix_agent` as a dir-level untracked marker, a careless `git add -A` at the parent OR an `git add -A` inside the submodule could sweep this unrelated change into the D-7 commit. The D-7 commit MUST stage ONLY `helix_code/go.mod`, `helix_code/go.sum`, and `helix_code/internal/agentbridge/`. The submodule change is its own separate commit in its own repo. (No technical defect in the D-7 code — this is a commit-boundary guard.)

**F2 (nit, non-blocking) — `baseURL` field is stored but unused.** `VerifierBridge.baseURL` (bridge.go:27,46) is set in the constructor but never read (the SDK client owns the effective base URL). Harmless; either drop it or add an accessor. Not a blocker.

**F3 (informational, NOT introduced here) — `clis` is module-internal, blocks the SP4 seam as currently shaped.** The HelixAgent CLI-dispatch code lives at `submodules/helix_agent/internal/clis/...`, i.e. under `internal/`, so it is NOT importable cross-module from `dev.helix.code`. The agentbridge correctly imports only the public `pkg/sdk/go/verifier`. Confirmed no illegal cross-module internal import from helix_code (`grep dev.helix.agent/internal helix_code/` → empty — would not compile anyway). **This means the SP4 `CLIAgentProvider` cannot build on a `clis` import as-is**; it needs either (a) a public SDK facade exported from helix_agent under `pkg/` (the verifier-bridge pattern, extended to CLI dispatch), or (b) the `clis` surface promoted out of `internal/`. The subagent flagged this correctly. The SP2 plan doc references `internal/clis/aider/repo_map.go` for the tree-sitter migration but does NOT explicitly document the "clis-is-internal blocks SP4 provider" constraint — recommend recording it in the SP4 plan / CONTINUATION so it is not re-discovered. This is a forward-looking design note, not a defect in D-7.

## Honesty assessment (D-7 genuinely closed?)

**Genuinely closed — real import proven, not a token import.** The bridge constructs the real `agentverifier.NewClient` and the test drives a real `httptest` HTTP server through the real SDK's `http.Client.Do`, asserting the wire request AND the decoded `VerificationResult` type. `go list -deps` proves the cross-module edge is in the real build graph, not a blank `_` import. The `replace => ../submodules/helix_agent` directive resolves and compiles the whole module. This satisfies the SP5-D7 goal: prove `dev.helix.code` can consume a real `dev.helix.agent` type end-to-end. The CONST-036/037 rationale (route verifier access through the single-source-of-truth SDK rather than duplicate a client) is sound and documented in the package comment.

The one honesty caveat the subagent correctly surfaced (F3) is NOT hidden: it is a forward limitation on the SP4 provider, not a D-7 bluff. D-7's scope was the bridge seam, which is real and working.

## Ordered must-fix

1. **(blocking) Commit boundary:** stage and commit ONLY `helix_code/go.mod`, `helix_code/go.sum`, `helix_code/internal/agentbridge/{bridge.go,bridge_test.go}`. Do NOT co-commit the unrelated `submodules/helix_agent/internal/clis/agents/claude_code/` change (F1). Verify with `git diff --staged --stat` before commit per §11.4.84.
2. **(recommended, non-blocking) Document the F3 constraint** in the SP4 plan / `docs/CONTINUATION.md`: `clis` is module-internal; SP4 `CLIAgentProvider` needs a public `pkg/` facade from helix_agent (or promotion of `clis` out of `internal/`) — do not assume a direct `clis` import.
3. **(optional nit) F2:** drop or expose the unused `VerifierBridge.baseURL` field.

## Evidence summary

- `go build ./...` → **BUILD_EXIT=0** (only benign `-lobjc` warnings).
- gin → **`github.com/gin-gonic/gin v1.12.0`** (unchanged).
- `go mod tidy` → **no-op** (go.mod/go.sum byte-identical; no dep conflict). `go mod verify` → all modules verified.
- `go test ./internal/agentbridge/... -count=1 -v` → **PASS**; `-count=3` → **PASS** (deterministic). Cross-module dep: `dev.helix.agent/pkg/sdk/go/verifier` present in `go list -deps`. Real HTTP via `httpClient.Do` at `client.go:373`.
- Broad sweep `go test ./internal/... ./cmd/... -count=1` → **144 ok / 1 FAIL** (`internal/cognee`). The 3 cognee subtest failures are the documented GPU-probe load-flake (`signal: killed` → `-1` sentinel), confirmed by isolation rerun `-p 1` → **ok 0.753s**. **No NEW failure attributable to the dep bumps.**
- Scope: only `helix_code/{go.mod,go.sum,internal/agentbridge}` touched; `submodules/helix_agent` parent pointer clean (recorded == HEAD `7c89410c`); no secret logged.

**VERDICT: GO-WITH-FIXES.** Sweep: **144 ok / 1 FAIL** (cognee GPU-probe load-flake only, pre-existing, passes in isolation; no dep-bump regression). gin confirmed **v1.12.0**. agentbridge proves a REAL cross-module HTTP round-trip (`dev.helix.agent/pkg/sdk/go/verifier`, real `httpClient.Do`); D-7 genuinely closed. Single blocking must-fix is the commit-boundary guard (F1) to keep the unrelated helix_agent `clis` change out of this commit.
