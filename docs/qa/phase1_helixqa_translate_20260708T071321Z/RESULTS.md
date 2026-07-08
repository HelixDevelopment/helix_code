# Phase-1 HelixQA CPU-Translation Bank (NLLB-200-CTranslate2) — Run Results

**Run ID:** `phase1_helixqa_translate_20260708T071321Z`
**Track:** `(T1/feature/helixllm-full-extension)`
**Date:** 2026-07-08
**Scope:** `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` §1.1
(Translation (NLLB) — landed at `:18436`) + the phase-3 proof
`docs/qa/phase3_translation_nllb_20260707/RESULTS.md`, extending HelixQA with
autonomous test-bank coverage mirroring `banks/helixllm_vision.yaml` (Bank B).

## 1. Boot (§11.4.76/§11.4.161/§11.4.119, NO GPU)

Booted the existing, already-proven harness
(`docs/qa/phase3_translation_nllb_20260707/harness/phase3translatenllb.bin`)
via its `boot-up` subcommand (§11.4.74 reuse, not reinvent), env-injected
per `run_proof.sh`'s convention:

```
$ ./phase3translatenllb.bin boot-up compose.phase3translatenllb.yml phase1qacpu_translate
UP-OK: phase1qacpu_translate nllb-shim via containers submodule orchestrator

$ curl http://localhost:18436/health
{"status": "ok", "model": "entai2965/nllb-200-distilled-600M-ctranslate2"}
(health OK after 1 poll — model already cached from the 2026-07-07 proof run, §11.4.77)
```

Pre-boot port check confirmed 18435-18440 all free; coder (`:18434`) verified
serving real GGUF metadata before boot.

## 2. Ground-truth fixture authoring (§11.4.6 — grounded against the LIVE service, this session)

Both candidate fixtures were run against the live `:18436` endpoint during
authoring (2026-07-08, this session) via a NEW analyzer
(`bin/helixqa-verify-translate-nllb`) BEFORE being committed to the bank:

```
$ helixqa-verify-translate-nllb --source "The house is blue." --source-lang eng_Latn --target-lang deu_Latn --expect "haus,blau" ...
PASS: PROBE-EN-DE notIdentity=true matched=2/2 hallucinated=false expect_fail=false raw_pass=true forward="Das Haus ist blau."

$ helixqa-verify-translate-nllb --source "The cat sleeps." --source-lang eng_Latn --target-lang fra_Latn --expect "chat,dort" ...
PASS: PROBE-EN-FR notIdentity=true matched=2/2 hallucinated=false expect_fail=false raw_pass=true forward="Le chat dort."
```

Both real forward translations genuinely differ from the source and contain
every asserted fact — deterministic (CTranslate2 `beam_size=1` greedy
decoding), corroborating the independently-proven fixtures already recorded
in `docs/qa/phase3_translation_nllb_20260707/RESULTS.md` (2026-07-07).

## 3. Bank authored: `submodules/helix_qa/banks/helixllm_translate_nllb.yaml`

Analyzer: `submodules/helix_qa/cmd/helixqa-verify-translate-nllb` (copy
alongside this file as `helixqa-verify-translate-nllb_main.go.txt`) — a thin,
project-agnostic `dispatches_to` Go binary (CONST-051(B), no HelixQA
core-engine change) that POSTs `{q,source,target}` to the shim's `/translate`
endpoint, requires every `--expect` fact token present (case-insensitive
substring) AND the forward translation NOT identical to the source (kills the
echo/passthrough bluff), and writes an auditable verdict JSON.

4 test cases: `TRA-UNDERSTAND-001`, `TRA-UNDERSTAND-002`,
`TRA-SELF-VALIDATE-001-GOOD`, `TRA-SELF-VALIDATE-001-BAD` (mandatory
§11.4.107(10)).

## 4. Bank run — through the REAL `pkg/testbank.Dispatcher` mechanism

A throwaway Go harness (NOT committed, removed after use — §11.4.84) loaded
`banks/helixllm_translate_nllb.yaml` via `testbank.LoadFile` and ran every
case through the actual `testbank.Dispatcher` (`os/exec` `DeviceExecFunc` +
`testbank.ContentAssertingResolver` — the same content-asserting evidence
resolver the vision bank proof used), proving the BANK ITSELF is wired
correctly end-to-end, not just manual CLI calls.

### 4a. Run with the correct (unmutated) analyzer

```
Loaded bank "HelixLLM CPU Translation (NLLB-200-CTranslate2) Capability" (1.0) — 4 test cases
[TRA-UNDERSTAND-001] PASS
[TRA-UNDERSTAND-002] PASS
[TRA-SELF-VALIDATE-001-GOOD] PASS
[TRA-SELF-VALIDATE-001-BAD] PASS

TOTAL: pass=4 fail=0 skip=0
overall exit=0
```

Per-fixture PASS verdict JSONs are in `verdicts/` alongside this file (copied
from `submodules/helix_qa/qa-results/helixllm_translate_nllb/*.json`,
git-ignored raw evidence per §11.4.128 — this curated copy is the committed
record per §11.4.83).

### 4b. Paired §1.1 mutation proof — the analyzer's discriminator is load-bearing

The fact-matching+not-identity assertion —

```go
v.Pass = v.NotIdentity && v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount && !v.Hallucinated
```

— was replaced with an unconditional `v.Pass = true // MUTATED for paired
§1.1 mutation test - always pass`, rebuilt, and swapped into
`bin/helixqa-verify-translate-nllb`. The SAME bank was re-run through the SAME
`Dispatcher`:

```
[TRA-UNDERSTAND-001] PASS
[TRA-UNDERSTAND-002] PASS
[TRA-SELF-VALIDATE-001-GOOD] PASS
[TRA-SELF-VALIDATE-001-BAD] FAIL  reason=dispatch_exit_1

TOTAL: pass=3 fail=1 skip=0
overall exit=1
```

**The mutation was caught**: with the real discriminator removed, the
analyzer's raw `pass` field wrongly flips to `true` on the golden-bad fixture
(the live response "Das Haus ist blau." genuinely does not contain "chat" or
"dort"), and `TRA-SELF-VALIDATE-001-BAD`'s `--expect-fail` inversion
correctly turns that bluff-PASS into a bank-level FAIL (`case_result=false`
→ analyzer exit 1 → dispatch FAIL). This is the proof the constitution's
§11.4.107(10) self-validated-analyzer mandate requires.

The mutated verdict is preserved at
`mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json`:
`"pass": true, "expect_fail": true, "case_result": false`.

The mutation was then **reverted** (restored from a pre-mutation backup),
rebuilt, and re-run — confirmed `grep -c "MUTATED for paired" main.go` == 0
(zero residue, §11.4.84 quiescence) and the bank re-ran clean (§4a numbers,
`pass=4 fail=0 skip=0`). No mutated code was ever committed to the
repository at any point.

## 5. Teardown (§11.4.119 single-owner cleanup) — coder confirmed untouched

```
$ ./phase3translatenllb.bin boot-down compose.phase3translatenllb.yml phase1qacpu_translate
DOWN-OK: phase1qacpu_translate nllb-shim (volumes removed) via containers submodule orchestrator

$ curl http://localhost:18434/v1/models -> http_status=200
$ podman ps -> ONLY helixllm-coder (Up About an hour, unchanged), nllb-shim container fully removed
```

## 6. Summary

| Item | Result |
|---|---|
| Translation NLLB shim booted | YES — reused proven harness `boot-up`, `/health` 200 after 1 poll (cached model) |
| Bank authored | `banks/helixllm_translate_nllb.yaml`, 4 cases, grounded fixtures (§11.4.6, live-confirmed this session) |
| Bank run (correct analyzer) | 4 PASS / 0 FAIL / 0 SKIP |
| Self-validation (§11.4.107(10)) | golden-good PASS + golden-bad FAIL (raw) / PASS (case, via `--expect-fail`) |
| Paired §1.1 mutation | Confirmed caught (mutated analyzer → `TRA-SELF-VALIDATE-001-BAD` FAILs the bank) |
| Mutation residue | Zero — reverted + re-verified before commit |
| Translation shim teardown | YES — single-owner, container fully removed |
| Coder (`helixllm-coder`) | Untouched throughout — genuine `/v1/models` HTTP 200 before AND after |

## Sources / evidence paths

- `verdicts/*.json` — per-fixture verdict artefacts from the correct-analyzer run.
- `conduit/conduit.events.jsonl` + `conduit.status.json` — §11.4.116 real-time
  conductor channel trace.
- `mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json` —
  mutated-analyzer verdict, standalone-Dispatcher run.
- `helixllm_translate_nllb.yaml` — the committed bank (copy for this evidence bundle).
- `helixqa-verify-translate-nllb_main.go.txt` — the committed analyzer source (copy).
