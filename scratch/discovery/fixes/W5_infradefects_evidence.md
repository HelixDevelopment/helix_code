# W5 infra-defects sweep — evidence

Date: 2026-07-12
Repo (inner app): `helix_code/`
Scope touched: `internal/llm/azure_provider.go`, `internal/llm/azure_provider_test.go`,
`test/integration/integration_test.go`,
`tests/e2e/challenges/cmd/runner/main.go`. No edits to `internal/rag`, `internal/acp`,
`internal/verifier`, `cmd/cli`, or `go.mod`. Nothing committed (per instructions).

Discipline followed: §11.4.115 (reproduce-first RED before any fix), §11.4.102
(root-cause via investigation, not guessing), §11.4.120 (stale-test reconciliation,
never fake-pass / never revert-to-satisfy-a-stale-assertion).

---

## Defect A (HIGHEST priority — real product bug) — `internal/llm` SIGSEGV

**Status: FIXED.**

### Root cause (investigated, not assumed)

`TestAzureProvider_NewWithoutEndpoint` in isolation passed. Full-package
`go test ./internal/llm/...` also passed cleanly. The crash is
environment-conditional: `.env.full-test:56` sets
`AZURE_OPENAI_ENDPOINT=http://localhost:8090` for the `test-infra-up` /
`test-full` pipeline. When that var is present in the process environment,
`NewAzureProvider` legitimately (and correctly — see
`TestAzureProvider_EndpointPrecedence_EnvWinsOverError` in
`azure_provider_audit_test.go`, which pins this as intended behavior) picks
up the env fallback and returns a non-nil provider + nil error. The test at
`azure_provider_test.go:85-86` used `assert.Error(t, err)` (non-fatal) then
unconditionally called `err.Error()`, so when `err` was nil the call
panicked and took down the whole `internal/llm` test binary.

### RED (reproduces the exact original crash trigger)

```
$ cd helix_code && AZURE_OPENAI_ENDPOINT=http://localhost:8090 \
    go test -tags=nogui -run TestAzureProvider_NewWithoutEndpoint -v ./internal/llm/
=== RUN   TestAzureProvider_NewWithoutEndpoint
    azure_provider_test.go:85:
        Error Trace: .../azure_provider_test.go:85
        Error:       An error is expected but got nil.
        Test:        TestAzureProvider_NewWithoutEndpoint
--- FAIL: TestAzureProvider_NewWithoutEndpoint (0.00s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
[signal SIGSEGV: segmentation violation code=0x1 addr=0x18 pc=0x1303139]
...
dev.helix.code/internal/llm.TestAzureProvider_NewWithoutEndpoint(0x259d0f8a2908)
	.../azure_provider_test.go:86 +0xf9
...
FAIL	dev.helix.code/internal/llm	0.011s
FAIL
```

### Fix applied (both product and test, as required)

- **Product** (`internal/llm/azure_provider.go`, `NewAzureProvider`): added a
  `strings.TrimSpace` on the resolved endpoint (config parameter or env
  fallback) before the empty check, so a whitespace-only endpoint from
  either source is now treated as genuinely missing/empty and produces the
  clear `"azure endpoint is required..."` error, rather than being silently
  accepted as a malformed resource URL. This is a real, targeted hardening
  of the "must return an error when the required field is missing/empty"
  contract; it does not change behavior for any correctly-configured
  endpoint (config or env).
- **Test** (`internal/llm/azure_provider_test.go`,
  `TestAzureProvider_NewWithoutEndpoint`): added `t.Setenv("AZURE_OPENAI_ENDPOINT", "")`
  so the test is hermetic against ambient environment (mirrors the existing
  pattern already used by the T06 audit tests in
  `azure_provider_audit_test.go`'s `withoutAzureEnv` helper for the
  equivalent scenario), and replaced `assert.Error` with `require.Error` so
  a nil error fails the test cleanly instead of reaching the unconditional
  `err.Error()` dereference.

### GREEN

```
$ cd helix_code && go test -tags=nogui -run TestAzureProvider -v ./internal/llm/
... (all 32 TestAzureProvider_* subtests) ...
PASS
ok  	dev.helix.code/internal/llm	0.013s

$ AZURE_OPENAI_ENDPOINT=http://localhost:8090 \
    go test -tags=nogui -run TestAzureProvider_NewWithoutEndpoint -v ./internal/llm/
=== RUN   TestAzureProvider_NewWithoutEndpoint
--- PASS: TestAzureProvider_NewWithoutEndpoint (0.00s)
PASS
ok  	dev.helix.code/internal/llm	0.009s
```

Exact original crash-trigger scenario now passes GREEN.

### Out-of-scope discovery (NOT fixed — flagged only)

Re-running the FULL `internal/llm` package under the same artificially
polluted `AZURE_OPENAI_ENDPOINT=http://localhost:8090` (for extra rigor,
beyond what the task required) surfaces one *pre-existing*, unrelated
failure: `TestNewProvider_AllProviderTypes/Azure_provider` in
`internal/llm/factory_test.go` (line ~83, `wantErr: true // Requires
AZURE_OPENAI_ENDPOINT env var`) also assumes a clean environment. This is
the SAME class of environmental assumption as Defect A, in a DIFFERENT
file (`factory_test.go`) that is explicitly outside this task's authorized
scope. It fails cleanly (no panic, no binary crash) — it does not block
`make verify-compile` or the required `go test -tags=nogui ./internal/llm/... -count=1`
invocation run WITHOUT artificial env pollution (see §Final verification
below, which is clean). Flagging for a future, separately-scoped fix; not
touched here per the explicit "Do NOT touch" instruction boundary and the
zero-risk mandate.

---

## Defect B (stale test, §11.4.120) — `test/integration/integration_test.go`

**Status: FIXED (reconciled to current API, not weakened).**

### Investigation

`llm.ProviderConfig` and `llm.NewProviderManager` no longer exist anywhere
in `internal/llm` (confirmed via package-wide symbol search — only
`internal/memory/providers/config.go` has an unrelated `ProviderConfig`
type for vector-store providers). The current, real API for "build a
manager from a set of provider configs and query availability + health" is:

```go
func InitializeModelManager(configs []ProviderConfigEntry) (*ModelManager, error)  // internal/llm/factory.go
func (m *ModelManager) GetAvailableModels() []*ModelInfo                          // internal/llm/model_manager.go
func (m *ModelManager) HealthCheck(ctx context.Context) map[ProviderType]*ProviderHealth
```

This is a genuine reconciliation (§11.4.120): the removed symbols were
replaced by a different concrete type during prior refactors, not left
half-implemented; the fix updates the test to the current API rather than
faking a pass or reverting anything.

### RED

```
$ cd helix_code && go vet -tags=nogui,integration ./test/integration/...
# dev.helix.code/test/integration
# [dev.helix.code/test/integration]
vet: test/integration/integration_test.go:75:16: undefined: llm.ProviderConfig
```

### Fix applied

Rewrote `TestLLMProviderIntegration` to build a `*llm.ModelManager` via
`llm.InitializeModelManager([]llm.ProviderConfigEntry{...})` (Ollama config,
matching `.env.full-test`'s `OLLAMA_HOST=http://localhost:11434`), then
assert on `GetAvailableModels()` (tolerates 0 discovered models when Ollama
is unreachable, same tolerance the original test documented) and
`HealthCheck(ctx)` (asserts the map is non-nil AND contains an entry keyed
by `llm.ProviderTypeLocal` — note: `OllamaProvider.GetType()` returns the
shared `ProviderTypeLocal`, not `ProviderTypeOllama`; verified by running
and reading the actual map key rather than guessing). Dropped the now-
unused `time` import (its only use was in the deleted `ProviderConfig{Timeout: ...}`
literal). Real component (`llm.NewProvider` → `NewOllamaProvider`),
no mocks — compliant with CONST-050(A).

### GREEN

```
$ cd helix_code && go vet -tags=nogui,integration ./test/integration/...
(exit 0, no output)

$ go test -tags=nogui,integration -run TestLLMProviderIntegration -v ./test/integration/...
=== RUN   TestLLMProviderIntegration
2026/07/12 12:48:16 Warning: Failed to discover Ollama models: failed to fetch models: ... connection refused
2026/07/12 12:48:16 ✅ Ollama provider discovered 0 models
2026/07/12 12:48:16 ✅ Provider registered: ollama with 0 models
    integration_test.go:129: ✅ LLM provider integration test completed
--- PASS: TestLLMProviderIntegration (0.00s)
PASS
ok  	dev.helix.code/test/integration	0.009s

$ go test -tags=nogui,integration -v ./test/integration/...
... (all TestDistributedWorkflow / TestLLMProviderIntegration /
     TestNotificationChannelIntegration / TestMCPProtocolIntegration /
     TestCrossComponentIntegration / TestErrorHandlingIntegration /
     TestSlackIntegration* / TestTelegramIntegration* subtests) ...
PASS
ok  	dev.helix.code/test/integration	0.838s
```

Full integration package passes — no other regressions introduced by the
import/rewrite.

---

## Defect C (broken target) — e2e challenge runner missing `-all`

**Status: FIXED.**

`make test-e2e-full` (Makefile line 262) invokes
`cd tests/e2e/challenges && go run cmd/runner/main.go -all`. No `-all` flag
was registered in `tests/e2e/challenges/cmd/runner/main.go`.

### RED

```
$ cd helix_code/tests/e2e/challenges && go build -o /tmp/e2e_runner ./cmd/runner/
$ /tmp/e2e_runner -all
flag provided but not defined: -all
Usage of ...
  -batch-desc string
  ...
$ echo $?
2
```

### Fix applied

Added `runAll := flag.Bool("all", false, "Run every loaded challenge ...")`
(mirrors the existing `-list` bool-flag declaration pattern) plus a
mutual-exclusion guard (`-all` + `-challenge` together → `log.Fatal`). The
existing "no `-challenge` specified" branch already enumerated and ran
every loaded challenge (was the implicit default) — `-all` now makes that
selection explicit and, critically, is registered so `flag.Parse()` no
longer rejects the flag the Makefile passes.

### GREEN

```
$ cd helix_code/tests/e2e/challenges && go build -o /tmp/e2e_runner_final ./cmd/runner/
(exit 0)

$ go vet ./cmd/runner/...
(exit 0, no output)

$ /tmp/e2e_runner -list          # smoke: existing flag still works
Available Challenges (6):
ID:          json-validator-cli-001
...
(exit 0)

$ timeout 6 /tmp/e2e_runner -all -max-concurrent=1
2026/07/12 12:49:40 Creating batch: challenge-test-run
2026/07/12 12:49:40   Challenges: [cli-task-manager-001 json-validator-cli-001 notes-project-001 tic-tac-toe-tui-001 url-shortener-001 ascii-art-generator-001]
...
2026/07/12 12:49:40 Batch created with ID: 4a982df2-67ee-40ef-9683-a8c03d88e0e6
2026/07/12 12:49:40 Starting batch execution...
2026/07/12 12:49:40 Batch execution completed successfully
================================================================================
BATCH EXECUTION SUMMARY
================================================================================
Total Executions: 6
  Completed:      0
  Failed:         6   <- expected: no local Ollama running in this sandbox;
                          this is the runner's real (non-bluffed) execution
                          path reporting genuine connection failures, not a
                          flag-parsing crash. Confirms -all is wired end to
                          end (batch created, all 6 challenge IDs resolved,
                          executed, summarized) without executing to
                          completion against a real LLM.
================================================================================
(exit 0)

$ /tmp/e2e_runner -all -challenge=foo
2026/07/12 12:49:46 cannot specify both -all and -challenge
$ echo $?
1
```

`-all` no longer exits 2 on flag parse; it resolves to the same "run every
loaded challenge" set as `-list` enumerates, and drives the runner all the
way through batch creation + execution + summary.

---

## Final verification (full build + full internal/llm suite, no cache, NO env pollution)

```
$ cd helix_code && make verify-compile
🔍 Verifying code compilation (nogui — no X11 system libs required)...
✅ All packages compile successfully

$ go test -tags=nogui ./internal/llm/... -count=1
ok  	dev.helix.code/internal/llm	72.694s
ok  	dev.helix.code/internal/llm/compression	0.014s
ok  	dev.helix.code/internal/llm/compressioniface	0.002s
ok  	dev.helix.code/internal/llm/i18n	0.002s
ok  	dev.helix.code/internal/llm/litellm	0.007s
ok  	dev.helix.code/internal/llm/promptcache	0.023s
ok  	dev.helix.code/internal/llm/providers/cerebras	0.008s
ok  	dev.helix.code/internal/llm/providers/cohere	0.008s
ok  	dev.helix.code/internal/llm/providers/helixagent	0.012s
ok  	dev.helix.code/internal/llm/providers/huggingface	0.010s
ok  	dev.helix.code/internal/llm/providers/replicate	0.026s
ok  	dev.helix.code/internal/llm/providers/together	0.009s
ok  	dev.helix.code/internal/llm/routing	0.002s
ok  	dev.helix.code/internal/llm/vision	0.208s

$ go vet -tags=nogui,integration ./test/integration/...
(exit 0, no output)
```

All three defects fixed, zero regressions, full build green, no panic. Nothing
committed (git status left untouched beyond the four edited files, per
instructions).
