# HXC-140 — QA evidence (§11.4.83)

**Item:** HXC-140 (Bug / Medium) — helix_qa copies a lock by value and one test-bank test fails
**Submodule:** `submodules/helix_qa` (HelixQA)
**Fix commit:** helix_qa `a0bd20f8` (pushed ff-only to all mirrors)
**helix_code pointer bump:** `submodules/helix_qa` → `a0bd20f8`
**Date (UTC):** 2026-07-12T12:47:24Z
**Discipline:** §11.4.102 systematic-debugging, §11.4.115 RED-reproduce-first, §11.4.146 extend-to-all-cases, §11.4.142 independent review, §11.4.28 decoupling.

## Root cause (Phase 1 — FACT, reproduced)

Two independent defects:

1. **Lock copy-by-value** (`go vet` reported 8 findings). `pkg/challengegen/generator.go` passed/stored
   `challenge.Result` **by value**, but `digital.vasic.challenges/pkg/challenge.Result`
   (`submodules/challenges/pkg/challenge/result.go`) embeds an unexported `mu sync.Mutex` — copying it
   copies the lock (unsafe; the whole challenge package's own API is pointer-only: `Execute` returns
   `*Result`, `RecordAction`/`AllPassed`/`IsFinal` take `*Result`).
2. **Failing test-bank test** `TestLoadDir_W6A_RealBanksDirLoadsCleanly` (`pkg/testbank/loader_test.go`) —
   two pre-existing bank-YAML bugs, the second masked by the first:
   - `banks/helixcode-generate-e2e.yaml` case HXC-GEN-006 declared `expect_json_path` three times under
     one step — invalid YAML duplicate mapping keys (`TestStep.ExpectJSONPath` is a single string field,
     `pkg/testbank/schema.go:316`).
   - `banks/helixcode_coder_race.yaml` (2026-07-08) and `banks/helixllm_coder_race.yaml` (2026-07-11, a
     distinct subsystem) both reused the bare `CODER-RACE-*` id namespace — an id collision.

## Fix

1. `pkg/challengegen/generator.go`: `aggregate.failExample` `challenge.Result` → `*challenge.Result`;
   `GenerateFromOutcomes` signature `[]challenge.Result` → `[]*challenge.Result` (aligning with the
   challenge package's established pointer-only convention). `generator_test.go` `res()` helper + all 10
   `[]challenge.Result{...}` literals updated. `go vet ./...` 8 → 0.
2. `banks/helixcode-generate-e2e.yaml`: HXC-GEN-006's one invalid step split into 3 independent steps,
   each checking one JSON path — no assertion dropped (`expect_body_contains: "helixllm"` strengthened
   to all 3). `banks/helixllm_coder_race.yaml`: 7 ids renamed `CODER-RACE-*` → `CODER-RACE-LLM-*`
   (id + paired `--challenge-id` args + prose, 27 occurrences in sync); `helixcode_coder_race.yaml`
   untouched → disjoint namespaces, zero collision.

## Captured verification

```
go vet ./...  -> 0 findings (was 8)
go build ./... -> exit 0
TestLoadDir_W6A_RealBanksDirLoadsCleanly -> PASS
go test -race ./pkg/challengegen/... -> PASS
full go test -count=1 ./... -> 141 ok packages, 0 FAIL
```

## Independent review (§11.4.142) — VERDICT: GO, zero findings

Structurally-separate reviewer re-ran everything: reverting generator.go/test → `go vet` reproduced
**exactly 8** lock-copy diagnostics at the described sites; reverting the 2 bank YAMLs → the test FAILed
with the exact duplicate-mapping-key parse error; restored → clean/PASS. Blast radius verified — zero
callers of `GenerateFromOutcomes` outside the package (all updated), `go build ./...` = 0. Rename
completeness verified (27 `CODER-RACE-LLM-*`, zero dangling, disjoint from `helixcode_coder_race.yaml`).
`ExpectJSONPath` single-string confirmed (root-cause validated); the shared `*Result` is read-only in
the synchronous call — no semantic-copy violation. Decoupling clean (only the `challenges` submodule's
own type referenced).

## Decoupling (§11.4.28)

Pure internal fix in HelixQA — no consuming-project context injected; aligns HelixQA's generator with
the upstream `challenges` type's pointer convention.
