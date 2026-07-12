# HXC-133 — factory_test.go Azure_provider subtest hermeticity fix

Status: FIXED (not committed — per task instructions)

## Scope
Touched ONLY: `helix_code/internal/llm/factory_test.go` (no product code changed).

## Root cause (FACT)
`TestNewProvider_AllProviderTypes/Azure_provider` builds a `ProviderConfigEntry`
with `Endpoint: "https://test.openai.azure.com"` (the top-level `Endpoint`
field), but `NewAzureProvider` (internal/llm/azure_provider.go:194-203) only
ever reads `config.Parameters["endpoint"]`, never `config.Endpoint`. Since the
subtest's `Parameters` map only carries `"timeout"`, the function falls
through to `os.Getenv("AZURE_OPENAI_ENDPOINT")`. `.env.full-test:56` sets
`AZURE_OPENAI_ENDPOINT=http://localhost:8090` for the full-infra test stack —
when that var is ambient, `NewAzureProvider` succeeds (endpoint resolved from
env) and returns `err == nil`, breaking the subtest's
`wantErr: true // Requires AZURE_OPENAI_ENDPOINT env var` assumption. Same
class of issue as the already-fixed `TestAzureProvider_NewWithoutEndpoint`
(azure_provider_test.go:77-97), which documents the exact mechanism and was
previously hardened with `t.Setenv("AZURE_OPENAI_ENDPOINT", "")`. This fails
cleanly (assertion failure), it does not panic/crash.

## Fix
Added, at the top of the `t.Run` closure in
`TestNewProvider_AllProviderTypes`, a guard scoped to the Azure case only:

```go
if tt.providerType == ProviderTypeAzure {
    t.Setenv("AZURE_OPENAI_ENDPOINT", "")
}
```

`t.Setenv` clears the var for that subtest only and Go's testing package
auto-restores the previous value after the subtest returns, so every other
subtest (and any parallel/sequential sibling) is unaffected. No product code
touched.

## RED (reproduced before the fix)

Command:
```
AZURE_OPENAI_ENDPOINT=http://localhost:8090 go test -tags=nogui ./internal/llm/ -run 'TestNewProvider_AllProviderTypes' -v -count=1
```

Result (captured, see HXC133_red.log):
```
    --- FAIL: TestNewProvider_AllProviderTypes/Azure_provider (0.00s)
        factory_test.go:193: NewProvider() error = <nil>, wantErr true
--- FAIL: TestNewProvider_AllProviderTypes (1.15s)
FAIL
FAIL	dev.helix.code/internal/llm	1.163s
```
Fails cleanly (assertion, no panic) — confirms the FACT and that this is a
distinct bug from the earlier SIGSEGV-class Azure bug (which is why the task
explicitly notes "same class... fails cleanly, no crash").

## GREEN — env polluted (AZURE_OPENAI_ENDPOINT set)

Command:
```
AZURE_OPENAI_ENDPOINT=http://localhost:8090 go test -tags=nogui ./internal/llm/ -run 'TestNewProvider_AllProviderTypes' -v -count=1
```

Result (HXC133_green_envset.log):
```
    --- PASS: TestNewProvider_AllProviderTypes/Azure_provider (0.00s)
--- PASS: TestNewProvider_AllProviderTypes (0.84s)
PASS
ok  	dev.helix.code/internal/llm	0.854s
```

## GREEN — env unset (ambient AZURE_OPENAI_ENDPOINT absent)

Command:
```
env -u AZURE_OPENAI_ENDPOINT go test -tags=nogui ./internal/llm/ -run 'TestNewProvider_AllProviderTypes' -v -count=1
```

Result (HXC133_green_envunset.log):
```
    --- PASS: TestNewProvider_AllProviderTypes/Azure_provider (0.00s)
--- PASS: TestNewProvider_AllProviderTypes (1.05s)
PASS
ok  	dev.helix.code/internal/llm	1.057s
```

## Full package retest (no -run filter), both env states

```
-- unset --
ok  	dev.helix.code/internal/llm	69.268s
-- set --
ok  	dev.helix.code/internal/llm	75.514s
```
(all subtests in the package pass, including the previously-existing Azure
hermeticity tests in azure_provider_test.go / azure_provider_audit_test.go —
no regression introduced by the new t.Setenv guard.)

## Build check

```
cd helix_code && go build -tags=nogui ./internal/llm/...
```
Exit code: 0 (HXC133_build.log is empty — no compiler output/errors).

## Anti-bluff note (§11.4.6 / §11.4.115)
This evidence file records real captured command output from actual `go test`
/ `go build` runs in this session (see the sibling `.log` files
HXC133_red.log / HXC133_green_envset.log / HXC133_green_envunset.log /
HXC133_build.log in this same directory), not inferred/predicted text.

## Not done (per task instructions)
No `git commit` was executed. `git status` will show
`helix_code/internal/llm/factory_test.go` as modified only.
