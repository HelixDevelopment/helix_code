# Wave-3 Implementation Batch — Independent Code Review (§11.4.125 / §11.4.142)

**Date:** 2026-06-10
**Reviewer:** Independent code-review subagent (read-only; no edits/commits/push)
**Scope:** UNCOMMITTED wave-3 code — SP1-funnel (helix_code) + SP2 catalog (submodules/helix_agent)
**Verdict:** **GO**

---

## 1. Batch under review

**SP1-funnel (helix_code, all builds/tests from `helix_code/`):**
- `internal/llm/keyrecognition.go` — new `PresentProviderNames() map[string]bool` (string-keyed bridge over `PresentProviders()`).
- `cmd/server/main.go` — new `loadAPIKeysAtStartup()` wired into `main()` BEFORE `config.Get()`.
- `internal/server/handlers.go` — `listLLMModels` + `listLLMProviders` switched `GetVerifiedModels(ctx)` → `GetWorkingModels(ctx, llm.PresentProviderNames())`.
- `cmd/cli/main.go` — `handleListModels` production call site switched to `llm.PresentProviderNames()`.
- New tests: `cmd/server/loadapikeys_wiring_test.go`, `internal/server/llm_working_funnel_test.go`, `cmd/cli/listmodels_funnel_source_test.go`, `internal/llm/keyrecognition_test.go` edits.

**SP2 (submodules/helix_agent):**
- New `internal/catalog/{catalog,registry_adapter,handler,defaults,catalog_test,handler_test}.go`.
- `internal/router/router.go` — registered `protected.GET("/catalog", …)` at line 785 + import reorder.

---

## 2. VERIFY results (commands run)

### 2.1 Build + vet — PASS

```
# SP1 (helix_code)
go build ./internal/llm/... ./internal/server/... ./cmd/server/... ./cmd/cli/...   → BUILD_EXIT=0
go vet   ./internal/llm/... ./internal/server/... ./cmd/server/... ./cmd/cli/...   → VET_EXIT=0
# SP2 (submodules/helix_agent)
go build ./internal/catalog/... ./internal/router/...                              → BUILD_EXIT=0
go vet   ./internal/catalog/...                                                     → VET_EXIT=0
```

### 2.2 RED→GREEN real — PASS

SP1 funnel/wiring tests (GREEN, explicit `RED_MODE=0` — tests default unset/`1` to RED):
```
RED_MODE=0 TestServerLoadAPIKeys_WiredAtStartup (cmd/server)                     --- PASS
RED_MODE=0 TestCLI_ModelListing_UsesCommittedKeyRecognitionSource (cmd/cli)      --- PASS
RED_MODE=0 TestPresentProviderNames_StringKeyedFunnelInput (internal/llm)        --- PASS
RED_MODE=0 TestServerListLLMModels_WorkingFunnelEndToEnd (internal/server)       --- PASS
RED_MODE=0 TestServerListLLMProviders_WorkingFunnelEndToEnd (internal/server)    --- PASS
RED_MODE=1 (RED guards reproduce pre-fix invariant)                              ok (all 4 pkgs)
```
SP2 catalog tests — **5 PASS**:
```
RED_MODE=0 go test -v ./internal/catalog/...
  --- PASS: TestCatalog_UnifiedList
  --- PASS: TestCatalog_HonestEmptyWhenVerifierDisabled
  --- PASS: TestCatalog_NamingGrammar
  --- PASS: TestHandler_CatalogRoute_Polarity
  --- PASS: TestHandler_CatalogEndpoint
  ok  dev.helix.agent/internal/catalog
```

### 2.3 §1.1 paired mutations — both FAIL-on-mutation (guards are genuine)

**SP1 mutation** — revert `listLLMModels` to `GetVerifiedModels(ctx)` (drop key gate):
```
RED_MODE=0 TestServerListLLMModels_WorkingFunnelEndToEnd
  FAIL — "failed model must be hidden", "pending model must be hidden",
         "no-key provider model must be hidden", served map has 5 items (want 1)
```
Restored; `handlers.go` diff back to original (19+/4-). Guard is NOT a tautology.

**SP2 mutation** — drop the aggregate `ensemble` entry from `Build()`:
```
RED_MODE=0 TestCatalog_UnifiedList
  FAIL — "unified catalog missing required targets [ensemble]"
```
Restored; package tests GREEN again. Guard is NOT a tautology.

### 2.4 Regression — full touched-package suites PASS

```
go test ./internal/llm/ ./internal/server/ ./cmd/cli/ ./cmd/server/  (helix_code)  → all ok
go test ./internal/catalog/...                                        (helix_agent) → ok
go build ./internal/router/...                                        (helix_agent) → exit 0
```

---

## 3. Anti-bluff assessment

- **Guards genuine** — both paired mutations flip GREEN→FAIL (§2.3). Not config-only / not tautologies.
- **No new bluff strings** — `simulated | for now | TODO implement | in production this would` scan of the SP1 + SP2 **production** files is clean. The one hit, `submodules/helix_agent/internal/router/router.go:458` (`// For now, log that messaging…`), is **pre-existing** (confirmed not in this diff) and out of scope; the messaging adapter is unrelated to the catalog block at :767–785.
- **Honest-empty correct** — `catalog.go` Build() emits a `KindModel` entry only when `verified != nil` AND `vm.Verified==true`; `NewDiscoveryVerifiedSource(nil)` returns a nil source ⇒ no fabricated model list. `TestCatalog_HonestEmptyWhenVerifierDisabled` proves it. The server funnel hides failed/pending/sub-threshold/no-key models (verified end-to-end over real httptest HTTP, not metadata-only).
- **CONST-042** — no API-key value is logged/printed in any changed production file; `loadAPIKeysAtStartup()` returns only a bool, comment explicitly states values are never logged.

---

## 4. No-regression / dead-code / scope findings

- **No sibling break** — full suites of all five touched packages PASS (§2.4). `listLLMProviders` + `listLLMModels` both migrated symmetrically; `buildProvidersFromVerifiedModels` reused unchanged.
- **No route collision (SP2)** — `/catalog` is registered exactly once (router.go:785). No pre-existing `/catalog` route. `os` already imported (router.go:47), so `os.Getenv("USE_HELIX_LLM")` compiles. Import reorder is cosmetic (gofmt-grouped), no functional change.
- **No go.mod / go.sum edits** in either lane. Lanes are disjoint (no shared file).
- **CLI-local `presentProviders` + `providerEnvAliases` are now production-dead** (finding F1, minor) — the production call site (`cmd/cli/main.go:1370`) switched to `llm.PresentProviderNames()`; the only remaining live references to `presentProviders`/`providerEnvAliases` are tests (`listmodels_d2_test.go`, `listmodels_funnel_source_test.go`). Per §11.4.124 the wave correctly did NOT delete on sight (extra-caution default), and the funnel-source test even uses the now-orphaned table as a *negative discriminator* (proving qwen recognition can only come from the committed table). Acceptable as-is; flagged for a future tracked §11.4.124 cleanup (investigate git-history + remove-or-rewire in a removal-only commit). Not a blocker.

---

## 5. Informational note (not a finding)

- **SP2 `/catalog` model section is latent at the wired endpoint** — router.go passes `Verified: NewDiscoveryVerifiedSource(nil)`, so the live endpoint will never surface `<provider>/<model>` entries until a `ModelDiscoveryService` is wired in `SetupRouterWithContext`. This is **consistent** with the surrounding code (the existing discovery path at router.go:1314 is also explicitly `nil` — the service genuinely isn't available in this scope) and is the **anti-bluff-correct** choice (honest-empty over fabrication). The model-join logic is fully implemented + unit-tested via the registry/discovery adapters; it activates the moment discovery is wired project-wide. No action required for this wave.

---

## 6. Verdict

**GO** — both lanes build + vet clean, every new test passes at GREEN, both §1.1 paired mutations genuinely FAIL, no regression in full touched-package suites, anti-bluff posture sound (genuine guards, honest-empty, no new bluff strings, CONST-042 respected), lanes disjoint, no go.mod edits, no route collision.

**Must-fix (ordered): none (zero blocking).**

**Recommended (non-blocking):**
1. F1 — track a §11.4.124 follow-up to retire the now-dead CLI-local `presentProviders` + `providerEnvAliases` (`helix_code/cmd/cli/main.go:1430,1461`) in a removal-only commit after git-history investigation, and migrate/retire `cmd/cli/listmodels_d2_test.go`'s `presentProviders` assertions.

---

## Captured build/test PASS-FAIL lines

```
SP1 build  : go build ./internal/llm/... ./internal/server/... ./cmd/server/... ./cmd/cli/...   PASS (exit 0)
SP1 vet    : go vet   (same set)                                                                 PASS (exit 0)
SP2 build  : go build ./internal/catalog/... ./internal/router/...                               PASS (exit 0)
SP2 vet    : go vet   ./internal/catalog/...                                                      PASS (exit 0)
SP1 GREEN  : 5/5 funnel+wiring tests RED_MODE=0                                                   PASS
SP2 GREEN  : 5/5 catalog tests                                                                    PASS
SP1 mut    : revert handler→GetVerifiedModels ⇒ TestServerListLLMModels_WorkingFunnelEndToEnd     FAIL (expected)
SP2 mut    : drop ensemble entry ⇒ TestCatalog_UnifiedList                                        FAIL (expected)
Regression : full suites internal/llm internal/server cmd/cli cmd/server (helix_code)            PASS
Regression : full suite internal/catalog (helix_agent)                                           PASS
```

**Finding counts:** blocking = 0 · non-blocking (recommended) = 1 (F1) · informational = 1.
**VERDICT: GO.**
