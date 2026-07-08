# Phase-1 HelixQA CPU-Tesseract-OCR Bank — Run Results

**Run ID:** `phase1_helixqa_tesseract_20260708T071321Z`
**Track:** `(T1/feature/helixllm-full-extension)`
**Date:** 2026-07-08
**Scope:** `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` §1.1
(Tesseract OCR — `:18438`) + the phase-3 proof
`docs/qa/phase3_tesseract_ocr_20260707/RESULTS.md`, extending HelixQA with
autonomous test-bank coverage mirroring `banks/helixllm_vision.yaml`.

## 1. Boot (§11.4.76/§11.4.161/§11.4.119, NO GPU)

Booted the existing, already-proven harness
(`docs/qa/phase3_tesseract_ocr_20260707/harness/phase3ocr.bin`) via its
`boot-up` subcommand (§11.4.74 reuse), image `localhost/phase3ocr_ocr:latest`
already cached:

```
$ ulimit -u "$(ulimit -Hu)"   # same self-scoped headroom bump the harness applies
$ ./phase3ocr.bin boot-up compose.phase3ocr.yml phase1qacpu_tesseract
UP-OK: phase1qacpu_tesseract ocr via containers submodule orchestrator

$ curl http://localhost:18438/health
{"config":"--oem 1 --psm 6","engine":"tesseract","status":"ok","tesseract_version":"tesseract 5.3.0"}
(health OK after 1 poll)
```

## 2. Ground-truth fixture authoring (§11.4.6 — grounded against the LIVE service, this session)

Both known-text strings were run through a real render->OCR round trip
against the live `:18438` endpoint during authoring (2026-07-08, this
session) via a NEW analyzer (`bin/helixqa-verify-tesseract`):

```
$ helixqa-verify-tesseract --known-text "HELIX OCR 2026 quick brown fox" --expect "helix,ocr,2026,quick,brown,fox" --conf-floor 60 ...
PASS: PROBE-OCR-1 matched=6/6 mean_conf=95.99 expect_fail=false raw_pass=true full_text="HELIX OCR 2026 quick brown fox"

$ helixqa-verify-tesseract --known-text "PHASE THREE TESSERACT PROOF SEVEN" --expect "phase,three,tesseract,proof,seven" --conf-floor 60 ...
PASS: PROBE-OCR-2 matched=5/5 mean_conf=94.77 expect_fail=false raw_pass=true full_text="PHASE THREE TESSERACT PROOF SEVEN"
```

Both real recognized texts genuinely match the rendered known text at high
confidence, matching the independently-proven fixtures already recorded in
`docs/qa/phase3_tesseract_ocr_20260707/RESULTS.md` (2026-07-07): mean_conf
95.99 and 94.77 respectively — unfakeable, since the shim itself controls
the rendered pixels and a SEPARATE `/v1/ocr` call recovers text from them.
CONF_FLOOR=60 is calibrated from this capability's own observed data
(§11.4.107(13)) — good cluster ~95-96, phase-3 proof's worst bad-fixture
~12.

## 3. Bank authored: `submodules/helix_qa/banks/helixllm_tesseract.yaml`

Analyzer: `submodules/helix_qa/cmd/helixqa-verify-tesseract` (copy alongside
this file). 4 test cases: `OCR-GOOD-001`, `OCR-GOOD-002`,
`OCR-SELF-VALIDATE-001-GOOD`, `OCR-SELF-VALIDATE-001-BAD` (mandatory
§11.4.107(10)).

## 4. Bank run — through the REAL `pkg/testbank.Dispatcher` mechanism

A throwaway Go harness (NOT committed, removed after use — §11.4.84) loaded
`banks/helixllm_tesseract.yaml` via `testbank.LoadFile` and ran every case
through the actual `testbank.Dispatcher`.

### 4a. Run with the correct (unmutated) analyzer

```
[OCR-GOOD-001] PASS
[OCR-GOOD-002] PASS
[OCR-SELF-VALIDATE-001-GOOD] PASS
[OCR-SELF-VALIDATE-001-BAD] PASS

TOTAL: pass=4 fail=0 skip=0
overall exit=0
```

Per-fixture PASS verdict JSONs are in `verdicts/` alongside this file
(copied from `submodules/helix_qa/qa-results/helixllm_tesseract/*.json`).

### 4b. Paired §1.1 mutation proof — the analyzer's discriminator is load-bearing

The token-match + confidence-floor assertion —

```go
v.Pass = v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount && v.MeanConf >= v.ConfFloor
```

— was replaced with an unconditional `v.Pass = true // MUTATED for paired
§1.1 mutation test - always pass`, rebuilt, and swapped into
`bin/helixqa-verify-tesseract`. The SAME bank was re-run through the SAME
`Dispatcher`:

```
[OCR-GOOD-001] PASS
[OCR-GOOD-002] PASS
[OCR-SELF-VALIDATE-001-GOOD] PASS
[OCR-SELF-VALIDATE-001-BAD] FAIL  reason=dispatch_exit_1

TOTAL: pass=3 fail=1 skip=0
overall exit=1
```

**The mutation was caught**: with the real discriminator removed, the
analyzer's raw `pass` field wrongly flips to `true` on the golden-bad
fixture (the real recognized text "HELIX OCR 2026 quick brown fox"
genuinely contains none of "phase"/"three"/"tesseract"/"proof"/"seven"),
and `OCR-SELF-VALIDATE-001-BAD`'s `--expect-fail` inversion correctly turns
that bluff-PASS into a bank-level FAIL. The mutated verdict is preserved at
`mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json`:
`"pass": true, "expect_fail": true, "case_result": false`.

The mutation was then **reverted**, rebuilt, and re-run — confirmed
`grep -c "MUTATED for paired" main.go` == 0 (zero residue) and the bank
re-ran clean (`pass=4 fail=0 skip=0`). No mutated code was ever committed.

## 5. Teardown (§11.4.119 single-owner cleanup) — coder confirmed untouched

```
$ ./phase3ocr.bin boot-down compose.phase3ocr.yml phase1qacpu_tesseract
DOWN-OK: phase1qacpu_tesseract ocr via containers submodule orchestrator

$ curl http://localhost:18434/v1/models -> http_status=200
$ podman ps -> ONLY helixllm-coder (unchanged), ocr container fully removed
```

## 6. Summary

| Item | Result |
|---|---|
| Tesseract OCR service booted | YES — reused proven harness `boot-up`, `/health` 200 after 1 poll (cached image) |
| Bank authored | `banks/helixllm_tesseract.yaml`, 4 cases, grounded fixtures (§11.4.6, live-confirmed this session) |
| Bank run (correct analyzer) | 4 PASS / 0 FAIL / 0 SKIP |
| Self-validation (§11.4.107(10)) | golden-good PASS + golden-bad FAIL (raw) / PASS (case, via `--expect-fail`) |
| Paired §1.1 mutation | Confirmed caught (mutated analyzer → `OCR-SELF-VALIDATE-001-BAD` FAILs the bank) |
| Mutation residue | Zero — reverted + re-verified before commit |
| Tesseract OCR teardown | YES — single-owner, container fully removed |
| Coder (`helixllm-coder`) | Untouched throughout — genuine `/v1/models` HTTP 200 before AND after |

## Sources / evidence paths

- `verdicts/*.json` — per-fixture verdict artefacts from the correct-analyzer run.
- `conduit/conduit.events.jsonl` + `conduit.status.json` — §11.4.116 trace.
- `mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json` — mutated-analyzer verdict.
- `helixllm_tesseract.yaml` — the committed bank (copy).
- `helixqa-verify-tesseract_main.go.txt` — the committed analyzer source (copy).
