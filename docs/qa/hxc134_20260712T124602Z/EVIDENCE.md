# HXC-134 — QA evidence (§11.4.83)

**Item:** HXC-134 (Bug / Medium) — Model verifier reports the model id as a number but the platform expects text
**Submodule:** `submodules/llms_verifier` (inner Go module `llm-verifier`, module `digital.vasic.llmsverifier`)
**Fix commit:** llms_verifier `376b74f1` (pushed ff-only to both mirrors: github + gitlab vasic-digital/LLMsVerifier)
**helix_code pointer bump:** `submodules/llms_verifier` → `376b74f1`
**Date (UTC):** 2026-07-12T12:46:02Z
**Discipline:** §11.4.102 systematic-debugging, §11.4.115 RED-reproduce-first, §11.4.120 test reconciliation, §11.4.142 independent review, §11.4.146 extend-to-all-cases (scope adjudicated), §11.4.28 decoupling.

## Root cause (Phase 1 — FACT, reproduced)

`database.Model.ID` is `int64` (internal DB primary key). The REST API emitted it as a **bare JSON
number** at 3 handler sites in `llm-verifier/api/handlers.go`:
- `ListModelsHandler` `"id": model.ID`
- `GetModelHandler` `"id": model.ID`
- `VerifyModelHandler` `"model_id": modelID` (int64 parsed from the URL path)

The consumer contract (helix_code `internal/verifier/types.go`) and the submodule's own canonical
`pkg/api/types.go` both declare the id field a **string**. helix_code's client carried a numeric-tolerant
workaround for the list endpoint only; `GetModelByID`/`VerifyModel` had none and would fail decode with
`json: cannot unmarshal number into Go struct field ... of type string` — the exact HXC-134 symptom.

## Fix

At all 3 REST sites the id is now emitted as a **string** via `strconv.FormatInt(<int64>, 10)`. The
internal DB storage type stays `int64`; only the JSON wire boundary is text. (Bonus correctness:
FormatInt is lossless for the full int64 range, whereas a bare JSON number silently loses precision
for ids beyond 2^53 in JS/float64 consumers.)

## Guard tests (RED→GREEN, §11.4.115) + reconciliation (§11.4.120)

New in `api/handlers_integration_test.go`: `TestListModelsHandler_ModelIDIsString`,
`TestGetModelHandler_ModelIDIsString`, `TestVerifyModelHandler_ModelIDIsString` — each reproduced RED
against the pre-fix handler (the literal unmarshal-number error + raw-body `"id":1,` present /
`"id":"1"` absent), then GREEN after the fix. Reconciled `TestVerifyModelHandler_Success`'s stale
`float64(1)` assertion → `"1"` (not fake-passed).

## Captured verification

```
go build ./...  -> exit 0
go vet   ./...  -> exit 0
api package suite -> all PASS
full go test ./... -> zero NEW failures. One pre-existing tests/automation_test.go TLS-to-localhost
  failure (self-signed cert vs localhost) reproduces IDENTICALLY on the parent commit -> unrelated infra.
```

## Independent review (§11.4.142) — VERDICT: GO, zero findings

Structurally-separate reviewer re-ran all of the above itself: RED reproduced (3 guard tests fail with
the exact unmarshal error on pre-fix handlers.go), GREEN restored, build/vet 0; §1.1 confirmed
(reverting one `strconv.FormatInt` → the corresponding guard test FAILs, restored byte-identical); the
pre-existing TLS-localhost `tests/` failure independently confirmed pre-existing (reproduces at
`376b74f1^`). Decoupling clean (no helix_code context injected).

## §11.4.146 scope adjudication (the extend-to-all-cases check)

The author flagged `cmd/ultimate-challenge/main.go:349` (same `"id": model.ID` pattern) as untouched.
The reviewer independently adjudicated it **genuinely out-of-scope, not a gap**: that site's `model` is
`providers.Model` whose `ID` field is ALREADY `string` — a different struct from `database.Model.ID
int64` — and that CLI writes a static OpenCode config file to disk, never touching the REST API.
helix_code's consumer (`internal/verifier/client.go`) only calls the 3 fixed REST endpoints; nothing
references `ultimate-challenge`/`opencode_ultimate.json`. "Consistently text end to end" holds for the
REST service HelixCode consumes.

## Decoupling (§11.4.28)

Pure internal wire-boundary type conversion in `digital.vasic.llmsverifier` — no consuming-project
context injected (comments cite helix_code's contract as rationale only).
