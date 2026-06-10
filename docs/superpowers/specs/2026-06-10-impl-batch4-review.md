# Wave-4 Code Review — SP5 substrate + verifier-wiring (§11.4.125 independent review)

**Date:** 2026-06-10
**Reviewer:** Independent code-review subagent (§11.4.125 / §11.4.142)
**Scope:** UNCOMMITTED wave-4 batch — `git diff` (root: helix_code go.mod/go.sum + new internal/substrate) + `git -C submodules/helix_agent diff` (verifier-wiring).
**Method:** read-only build/test VERIFICATION run this session. No edits/commits/pushes (mutations applied then byte-restored from backup; trees verified clean).

---

## VERDICT: GO-WITH-FIXES

The batch is functionally sound, anti-bluff genuine, and the gin 1.11→1.12 bump is SAFE (every gin/HTTP/server/handler/middleware package PASSES; the only failing packages are pre-existing and gin-independent). The single must-fix is a process item, not a code defect: a pre-existing deterministic unit failure in `internal/secrets` exists in the tree regardless of this batch — it does not block this batch's correctness but should be acknowledged/tracked so it is not mistaken for batch fallout at the release gate.

---

## LOAD-BEARING CHECK — gin 1.11→1.12 bump full unit sweep

`cd helix_code && go build ./...` → **exit 0**, zero compile errors (only benign `ld: warning: ignoring duplicate libraries: '-lobjc'` on the three Fyne desktop apps — unrelated to gin).

`go test ./internal/... ./cmd/... -count=1` (full broad sweep):

- **ok packages: 141**
- **FAIL packages: 3 distinct** — `internal/cognee`, `internal/helixqa`, `internal/secrets`
- no-test-files (informational): 55

**gin-bump safety is PROVEN — none of the 3 failures are gin/HTTP surface:**

- Every gin-surface package PASSES: `internal/server` ok, `cmd/server` ok, `internal/auth` ok, `internal/llm` (+ subpkgs) ok, `internal/provider`/`internal/providers` ok, `internal/server/i18n` ok.
- `internal/cognee` and `internal/helixqa` **PASS on isolated re-run** (`go test ./internal/cognee/ ./internal/helixqa/` → both `ok`; cognee 21.7s vs 43.7s in the full sweep). Their full-sweep failures are real-infra contention/timeout flakes under parallel load (cognee hits real backends), NOT gin breakage. They are not in this batch's diff.
- `internal/secrets` is the only DETERMINISTIC failure: `TestLoadAPIKeys_GapFillPrecedence` — `gapfill_test.go:46: RED expected defect (FOO overridden to from_file), got FOO="from_shell"`. **Proven pre-existing and gin-independent:** reverting `go.mod`/`go.sum` to HEAD (gin **1.11.0**) and re-running yields the IDENTICAL failure. `secrets` is not in this batch's diff. (This is an env-precedence RED-polarity test, no HTTP surface.)

**Sweep result: 141 ok / 3 FAIL — 0 of the FAILs attributable to the gin bump.** `go mod verify` → all modules verified. `go.sum` consistent with the tidied require/replace set.

---

## SP5 substrate (helix_code/internal/substrate)

- `go test ./internal/substrate/... -v -count=1` → **5/5 PASS** (DispatchUnitThroughConcurrencyPool, SchedulerPriorityOrdering, DrainRunsAllUnitsOnPool, CapabilityGateBlocksUnsatisfiableUnit, UnitErrorPropagates).
- Real wiring: the substrate genuinely imports `digital.vasic.concurrency/pkg/{pool,queue}` and runs Units on the real `pool.WorkerPool` (the test observes `ran==1` + the Execute return value through `SubmitWait`). The replace target `../submodules/concurrency` EXISTS with real `pkg/pool/pool.go` + `pkg/queue/queue.go` — not a dangling directive. This closes the D-7 "own-org concurrency unused" gap by REUSE (§11.4.74), not a fourth reimplementation.
- **§1.1 mutation reproduced:** `Resolver.CanRun` → `return true` (always-true) makes `TestSubstrate_CapabilityGateBlocksUnsatisfiableUnit` FAIL (`An error is expected but got nil`); restored → ok. Genuine anti-bluff seam — the capability gate is a real error path, not a silent drop (`Dispatch` returns a real `fmt.Errorf` and the unit's Execute does NOT run).
- No `simulated|for now|TODO implement|in production this would` in substrate.go. No secret logging.

---

## verifier-wiring (submodules/helix_agent)

- `go build ./internal/catalog/... ./internal/router/...` → exit 0; `go vet` → clean (no output).
- `go test ./internal/catalog/ -count=1` → **8 PASS** (the 6 wiring tests: NilIsHonestEmpty, StalenessAndVerifiedGate, ZeroVerifiedAtIsStale + the adjacent UnifiedList/HonestEmpty/NamingGrammar/CatalogRoute_Polarity/CatalogEndpoint).
- Real wiring confirmed: `router.go:783` now feeds `catalog.NewStartupVerifierSource(providerRegistry.GetStartupVerifier())`. `GetStartupVerifier()` is a real method on `*services.ProviderRegistry` returning `*verifier.StartupVerifier` (nil-able). `verifiedModelsFromProviders` reads real `verifier.UnifiedProvider.Models` / `UnifiedModel.{Verified,VerifiedAt,Score,Provider}` fields (all exist in `provider_types.go`). The router uses the SAME StartupVerifier already wired into the debate team (router.go:911/931) — internally consistent, no second source.
- **§1.1 mutation #1 (CONST-037 staleness gate):** removing the `VerifiedAt.IsZero() || now.Sub > ttl` continue → `TestStartupVerifierSource_StalenessAndVerifiedGate` FAILs (`stale (>24h) verified model leaked`) AND `TestStartupVerifierSource_ZeroVerifiedAtIsStale` FAILs. Restored → ok. **Genuine.**
- **§1.1 mutation #2 (CONST-036 Verified filter):** removing the `if !m.Verified { continue }` → same test FAILs (`unverified model leaked past the CONST-036 Verified filter`). Restored → ok. **Genuine.**
- **RED_MODE polarity:** `RED_MODE=1` correctly reproduces the pre-fix honest-empty assertion (fresh verified models ABSENT) — proper §11.4.115 polarity switch.

---

## Anti-bluff assessment (§11.4 / CONST-036 / CONST-037)

- **Guards genuine?** YES — both constitutional gates are real, deterministic, pure-function-testable (`now`+`ttl` injected, §11.4.50), and each is individually mutation-proven to FAIL when removed.
- **Honest-empty correct?** YES — `NewStartupVerifierSource(nil)` returns a nil `VerifiedModelSource`; the catalog emits NO `KindModel` entry (test asserts no fabricated model). An un-run real `StartupVerifier` likewise yields zero models. No hardcoded/fabricated working list.
- **CONST-037 24h gate real?** YES — a model with zero `VerifiedAt` is treated as STALE/excluded (cannot prove recency, §11.4.6 no-guessing), and `>24h` is excluded; only `Verified==true` AND within-24h is surfaced.
- **No bluff strings introduced.** `simulated|for now|TODO implement|in production this would` scan on the changed files: the only `for now` hit is `router.go:458` — PROVEN pre-existing (git blame: commit 08ab556, 2026-03-01), NOT in this batch's diff, in an unrelated messaging block.

---

## Scope (SP5)

- **No `dev.helix.agent` in helix_code/go.mod** (grep count = 0). SP5 added ONLY the `digital.vasic.concurrency` require + `replace ... => ../submodules/concurrency`. Lanes disjoint — substrate (helix_code) and verifier-wiring (helix_agent) touch no shared files.
- go.mod tidy churn (gin 1.11→1.12 + transitive bumps: bytedance/sonic, validator, goccy, quic-go, ugorji, golang.org/x/arch, mongo-driver indirect, dropped go.uber.org/mock + x/mod + x/tools indirect) is consistent and `go mod verify`-clean.
- No secret logged in any new code.

---

## Ordered must-fix

1. **(process, non-blocking for THIS batch)** `helix_code/internal/secrets` `TestLoadAPIKeys_GapFillPrecedence` is a DETERMINISTIC pre-existing failure (identical on gin 1.11.0). It is unrelated to wave-4 and not in the diff, but it WILL turn the full `go test ./internal/...` sweep red at the release gate. Track it as a separate pre-existing-defect work item (it is an env-precedence RED-polarity test reporting `got FOO="from_shell"`) so it is not misattributed to this batch and does not silently block the §11.4.40 tag sweep. No edit to this batch required.
2. **(advisory)** `internal/cognee` / `internal/helixqa` are flaky under the full parallel sweep (real-infra contention) though green in isolation. Not introduced by this batch; consider serializing or `-p 1` for these real-infra packages in the release sweep to avoid false reds.

---

## Final summary

- **VERDICT: GO-WITH-FIXES** (must-fix is a pre-existing, gin-independent tracking item, not a wave-4 code defect).
- **gin-bump full-sweep: 141 ok / 3 FAIL — 0 FAILs attributable to the gin 1.11→1.12 bump.** All 3 failing packages (cognee, helixqa, secrets) are outside the batch diff; cognee+helixqa pass in isolation (contention flakes); secrets fails identically on pre-bump gin 1.11.0 (proven pre-existing).
- **Finding counts:** Blocking-for-batch = **0**; Must-fix (process/tracking) = **1**; Advisory = **1**.
- Substrate 5/5 PASS + 1 genuine mutation; verifier-wiring 8 PASS (6 wiring) + 2 genuine mutations + RED_MODE polarity correct; anti-bluff guards real; honest-empty correct; CONST-036/037 gates real; scope clean (no dev.helix.agent); `go mod verify` all-verified.
