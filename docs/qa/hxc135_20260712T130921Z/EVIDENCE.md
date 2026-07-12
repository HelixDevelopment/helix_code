# HXC-135 — QA evidence (§11.4.83)

**Item:** HXC-135 (Feature / Medium) — HelixCode reads six advanced capability indicators from the central verifier, but the verifier's live responses never included those fields, so the flags always read as unsupported
**Submodule:** `submodules/llms_verifier` (inner Go module `llm-verifier`, module `digital.vasic.llmsverifier`)
**Fix commit:** llms_verifier `1096057f` (pushed ff-only to both mirrors: github + gitlab vasic-digital/LLMsVerifier — verified `1096057f70b5` on both via `git ls-remote`)
**helix_code pointer bump:** `submodules/llms_verifier` → `1096057f`
**Date (UTC):** 2026-07-12T13:09:21Z
**Closure vocab:** Implemented (§11.4.33, Feature)
**Discipline:** §11.4.102 systematic-debugging, §11.4.115 RED-reproduce-first, §11.4.142/§11.4.125 independent review, §11.4.145 impact-research (contract-integrity angle), §11.4.28 decoupling, CONST-036/CONST-040 (verifier single source of truth, no hardcoded capability flags).

## Root cause (Phase 1 — FACT)

The verifier's C4 probes in `verification.Verifier.Verify` (`verification/verification.go:118-127`)
COMPUTE and PERSIST six CONST-040 capability flags on `database.VerificationResult`
(`SupportsMCPs / SupportsLSPs / SupportsACPs / SupportsRAG / SupportsSkills / SupportsPlugins`),
but the REST API layer (`api/handlers.go`) never surfaced them in the `ListModelsHandler`,
`GetModelHandler`, or `VerifyModelHandler` responses. The consumer (helix_code
`internal/verifier/client.go`) already ships a `capabilityAliasFields` decoder keyed on the
plural wire names `supports_mcps / supports_lsps / supports_acps` (+ `supports_rag / supports_skills /
supports_plugins`) — so with the fields absent, every capability decoded as its zero value
(`false`), i.e. "unsupported", regardless of the verifier's real computed result. The computation
was real; only the API surfacing was missing.

## Fix

`api/handlers.go` — the 3 REST handlers now emit the 6 fields, populated from the computed
`*database.VerificationResult` via a new `addCapabilityFields()` helper (honest `false` when the
result is `nil` — never a fabricated `true`, per CONST-036/040 no-hardcoding). `ListModelsHandler`
uses a new `latestVerificationResultsByModelID()` batch wrapper around the pre-existing
`database.Database.GetLatestVerificationResults` ("latest completed per model") to avoid an N+1.
Plural JSON tags preserved to match the verifier's own convention (`supports_rag/skills/plugins`)
AND the consumer's existing decoder (feature genuinely reachable end-to-end).

## Guard test (RED→GREEN, §11.4.115) — `api/capability_fields_test.go`

4 tests using a MIXED 3-true / 3-false capability profile (so a hardcoded constant can't pass):
`TestListModelsHandler_CapabilityFields_SourcedFromComputedVerificationResult`,
`TestGetModelHandler_CapabilityFields_SourcedFromComputedVerificationResult`,
`TestGetModelHandler_CapabilityFields_HonestFalse_WhenNoVerificationResult`,
`TestVerifyModelHandler_CapabilityFields_SourcedFromComputedVerificationResult`.

## Captured verification

```
go build ./...  -> exit 0
go vet   ./...  -> exit 0
go test -count=1 ./api/...  -> all PASS (4 guard tests + full api suite, 0.170s)
full go test ./... -> zero NEW failures. Pre-existing digital.vasic.llmsverifier/tests
  failures (TestCommandFlagValidation, TestOutputFormats — TLS-to-localhost self-signed
  cert mismatch) reproduce IDENTICALLY on unmodified parent 376b74f1 (throwaway git worktree)
  -> unrelated infra, not caused by HXC-135.
```

## Independent review (§11.4.142/§11.4.145) — VERDICT: GO, zero blocking findings

Structurally-separate reviewer re-ran build/vet/guard (all green), performed the §1.1 mutation
(hardcode `supports_rag` to the opposite polarity → 3 of 4 guard tests FAIL; the `vr==nil`
honest-false test correctly stays green as it exercises the untouched branch; restored
byte-identical → all GREEN — proves the guard depends on real computed data), confirmed no
N+1 / no wrong-model cross-wire in the batch wrapper, and INDEPENDENTLY confirmed the wire
names emitted here are exactly the plural names helix_code's `capabilityAliasFields` decoder
consumes (feature reachable end-to-end, not dead-on-arrival). Pre-existing TLS failures
reproduced on parent `376b74f1` → not caused by this change. Decoupling clean.

Non-blocking nit (folded into the helix_code closure commit): the consumer-side
`internal/verifier/client.go` doc comment said the capability reconciliation is "a documented
no-op on the live wire" — now stale, since HXC-135 makes it live. Corrected to reflect the
wire-live state.

## Decoupling (§11.4.28)

Pure internal API-surfacing of already-computed data in `digital.vasic.llmsverifier` — no
consuming-project context injected; the plural wire schema is the verifier's own convention,
not a helix_code-specific shape.
