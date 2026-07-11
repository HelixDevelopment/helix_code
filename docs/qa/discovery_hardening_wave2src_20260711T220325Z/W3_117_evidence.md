# HXC-117 Phase 1 — CONST-040 capability-flag struct fields — evidence

Date: 2026-07-12
Discipline: §11.4.169 (test-type coverage) + §11.4.115 (RED-baseline-on-broken-artifact TDD) + §11.4.6 (anti-bluff, evidence-backed claims only)

## Design doc read

`docs/research/const040_capability_model_20260712/DESIGN.md` §2 (HXC-117 — capability-flag design), read in full before any edit.

## Scope

Touched ONLY:
- `helix_code/internal/verifier/types.go` — added 6 bool fields to `VerificationResult`, mirrored on `VerifiedModel`.
- `helix_code/internal/verifier/embedded_server.go` — the one existing local populator of `VerificationResult` (`handleModelDetail`), extended to mirror the 6 new fields from the `VerifiedModel` it already holds.
- `helix_code/internal/verifier/capability_flags_test.go` — new unit test (TDD).

NOT touched: `internal/mcp`, `cmd/cli`, any other subsystem, `go.mod` (verified below).

## RED (fields don't exist — compile fail)

```
$ go test -tags=nogui ./internal/verifier/... -run TestVerificationResult_CapabilityFields -count=1
# dev.helix.code/internal/verifier [dev.helix.code/internal/verifier.test]
internal/verifier/capability_flags_test.go:21:20: r.SupportsMCP undefined (type VerificationResult has no field or method SupportsMCP)
internal/verifier/capability_flags_test.go:22:20: r.SupportsLSP undefined (type VerificationResult has no field or method SupportsLSP)
internal/verifier/capability_flags_test.go:23:20: r.SupportsACP undefined (type VerificationResult has no field or method SupportsACP)
internal/verifier/capability_flags_test.go:24:20: r.SupportsRAG undefined (type VerificationResult has no field or method SupportsRAG)
internal/verifier/capability_flags_test.go:25:20: r.SupportsSkills undefined (type VerificationResult has no field or method SupportsSkills)
internal/verifier/capability_flags_test.go:26:20: r.SupportsPlugins undefined (type VerificationResult has no field or method SupportsPlugins)
internal/verifier/capability_flags_test.go:37:20: m.SupportsMCP undefined (type VerifiedModel has no field or method SupportsMCP)
... (too many errors, truncated)
FAIL	dev.helix.code/internal/verifier [build failed]
```
Full log: `red_test.log` in this same scratchpad dir.

## Implementation

- `VerificationResult` (types.go): added `SupportsMCP`, `SupportsLSP`, `SupportsACP`, `SupportsRAG`, `SupportsSkills`, `SupportsPlugins` — same `bool` type, same `json:"supports_xxx"` snake_case tag convention as the pre-existing `SupportsEmbeddings` field, with a doc comment explaining the false-means-"not verified"-never-"disabled" contract and an honest note that no populator sets these from a real probe yet (matches the design doc §2.2 verbatim field block).
- `VerifiedModel` (types.go): mirrored the same 6 fields, per the design doc's explicit "mirror on VerifiedModel" instruction (same rationale/shape as the existing `SupportsEmbeddings` duplication between the two structs).
- `embedded_server.go` `handleModelDetail`: the ONLY place in the whole inner app that constructs a `VerificationResult` value (confirmed via `grep -rn "VerificationResult{" internal/ | grep -v _test.go` → single hit). Extended it to copy the 6 new fields from the `*VerifiedModel` (`found`) it already has in hand, with a comment stating honestly that `FallbackModels` doesn't populate them yet so this always propagates `false` today. `client.go`'s `VerifyModel` needs NO change — it does a bare `json.Unmarshal` of the external LLMsVerifier HTTP response directly into `VerificationResult`, so the new tagged fields populate automatically the moment (if ever) that external service starts returning them.
- Deliberately did NOT touch `SupportsEmbeddings` propagation in `embedded_server.go` (it was already unset/always-false there before this change) — flipping that would be an unrelated behavior change outside this task's scope, so it was reverted after a first draft included it (self-review catch).

## GREEN

```
$ go build -tags=nogui ./internal/verifier/...
(exit 0, no output)

$ go test -tags=nogui ./internal/verifier/... -count=1
ok  	dev.helix.code/internal/verifier	4.285s
?   	dev.helix.code/internal/verifier/i18n	[no test files]

$ go test -tags=nogui ./internal/verifier/... -run 'TestVerificationResult_CapabilityFields|TestVerifiedModel_CapabilityFields' -v -count=1
=== RUN   TestVerificationResult_CapabilityFields_DefaultFalse
--- PASS: TestVerificationResult_CapabilityFields_DefaultFalse (0.00s)
=== RUN   TestVerifiedModel_CapabilityFields_DefaultFalse
--- PASS: TestVerifiedModel_CapabilityFields_DefaultFalse (0.00s)
=== RUN   TestVerificationResult_CapabilityFields_JSONRoundTrip
--- PASS: TestVerificationResult_CapabilityFields_JSONRoundTrip (0.00s)
=== RUN   TestVerifiedModel_CapabilityFields_JSONRoundTrip
--- PASS: TestVerifiedModel_CapabilityFields_JSONRoundTrip (0.00s)
PASS
ok  	dev.helix.code/internal/verifier	0.008s
?   	dev.helix.code/internal/verifier/i18n	[no test files]
```

Whole-package suite (all pre-existing verifier tests, unrelated to this change) still green — no regression:
```
$ go test -tags=nogui ./internal/verifier/... -count=1
ok  	dev.helix.code/internal/verifier	4.285s
```

Whole inner-app build unaffected (proves the additive struct change didn't break any other package that imports `verifier`):
```
$ go build -tags=nogui ./...
(exit 0, no output)
```

`gofmt` clean after formatting pass (manual struct-field alignment initially broke gofmt; fixed and re-verified GREEN):
```
$ gofmt -l internal/verifier/types.go internal/verifier/embedded_server.go internal/verifier/capability_flags_test.go
(no output = clean)
```

`go vet` clean:
```
$ go vet ./internal/verifier/...
(exit 0, no output)
```

## Additive / non-breaking confirmation

- `git diff --stat -- helix_code/internal/verifier` → only `embedded_server.go` (+13/-0 net addition) and `types.go` (field additions + gofmt realignment of existing lines, no field removed/renamed/retyped) changed; `capability_flags_test.go` is a new untracked file.
- No existing field on `VerificationResult` or `VerifiedModel` was removed, renamed, or retyped — confirmed by re-reading both structs post-edit; all pre-existing fields (`SupportsEmbeddings`, `SupportsToolUse`, etc.) are untouched.
- No other package was touched: `git status --porcelain -- helix_code/` shows only `internal/verifier/{types.go,embedded_server.go,capability_flags_test.go}` from this task. (Other pre-existing dirty files in the shared checkout — `helix_code/Makefile`, `helix_code/go.sum`, `helix_code/go.mod` [+1 line for `github.com/coder/acp-go-sdk`], `helix_code/internal/acp/` — are NOT from this task; they were already present before this session started, evidently a concurrent/parallel HXC-119-track's in-progress work in this shared checkout. Confirmed not caused by this task's `go build`/`go test` invocations: `grep -rln "coder/acp-go-sdk" --include="*.go" .` returns zero hits, i.e. nothing in the tree imports that module yet, so `go build`/`go test` could not have auto-added it to `go.mod`/`go.sum`.)
- `go.mod` untouched by this task (per instructions) — confirmed the pre-existing diff on `go.mod` predates this session (see above).

## Self-review (§11.4.92-lite, per task instructions)

- Additive only: yes — 6 new bool fields on 2 structs, 1 populator extended to fill them from data it already has (still always `false` today, honestly documented as such).
- No other package touched: yes.
- `go.mod` untouched by this task: yes.
- No `git add`/`git commit` performed (per instructions).
- No `--force` used.

## Status: DONE
