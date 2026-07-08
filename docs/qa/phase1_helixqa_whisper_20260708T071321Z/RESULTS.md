# Phase-1 HelixQA CPU-Whisper-STT Bank — Run Results

**Run ID:** `phase1_helixqa_whisper_20260708T071321Z`
**Track:** `(T1/feature/helixllm-full-extension)`
**Date:** 2026-07-08
**Scope:** `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` §1.1
(Whisper STT — `:18437`) + the phase-3 proof
`docs/qa/phase3_whisper_stt_20260707/RESULTS.md`, extending HelixQA with
autonomous test-bank coverage mirroring `banks/helixllm_vision.yaml`.

## 1. Boot (§11.4.76/§11.4.161/§11.4.119, NO GPU)

Booted the existing, already-proven harness
(`docs/qa/phase3_whisper_stt_20260707/harness/phase3whisper.bin`) via its
`boot-up` subcommand (§11.4.74 reuse), env-injected per `run_proof.sh`'s
convention (image `localhost/helixllm-stt:cpu` already cached from the
2026-07-07 proof, so `--build` hit cache instantly):

```
$ ./phase3whisper.bin boot-up compose.phase3whisper.yml phase1qacpu_whisper
UP-OK: phase1qacpu_whisper helixllm-stt via containers submodule orchestrator

$ curl http://localhost:18437/health
{"status":"ok","faster_whisper_version":"1.1.1","model":"base","device":"cpu","compute_type":"int8","no_speech_threshold":0.6}
(health OK after 1 poll)
```

## 2. Ground-truth fixture authoring (§11.4.6 — grounded against the LIVE service, this session)

The two known-text WAV fixtures already committed at
`docs/qa/phase3_whisper_stt_20260707/harness/{fox,helloworld}.wav`
(espeak-ng synthesized from a literal known string) were copied into
`submodules/helix_qa/data/whisper_gt/` (decoupling — the submodule owns its
own ground-truth fixtures, §11.4.28) and POSTed to the live `:18437`
endpoint during authoring (2026-07-08, this session) via a NEW analyzer
(`bin/helixqa-verify-whisper`):

```
$ helixqa-verify-whisper --wav data/whisper_gt/fox.wav --expect "quick,brown,fox,lazy,dog" ...
PASS: PROBE-FOX matched=5/5 missing=[] expect_fail=false raw_pass=true transcript="The quick brown fox dumps over the lazy dog."

$ helixqa-verify-whisper --wav data/whisper_gt/helloworld.wav --expect "hello,world,one,two,three" ...
PASS: PROBE-HW matched=5/5 missing=[] expect_fail=false raw_pass=true transcript="Hello World 1, 2, 3!"
```

Both real transcripts genuinely recover the known text — matching the
independently-proven transcripts already recorded in
`docs/qa/phase3_whisper_stt_20260707/RESULTS.md` (2026-07-07). Honest
engine-behavior notes carried forward (§11.4.6/§11.4.120, not a weakening):
"jumps"->"dumps" real minor ASR mis-hearing (not asserted); "one two
three"->"1, 2, 3" Whisper's documented number-normalization, reconciled via
digit/number-word canonicalization in the analyzer.

## 3. Bank authored: `submodules/helix_qa/banks/helixllm_whisper.yaml`

Analyzer: `submodules/helix_qa/cmd/helixqa-verify-whisper` (copy alongside
this file). 4 test cases: `STT-FOX-001`, `STT-HELLOWORLD-001`,
`STT-SELF-VALIDATE-001-GOOD`, `STT-SELF-VALIDATE-001-BAD` (mandatory
§11.4.107(10)).

## 4. Bank run — through the REAL `pkg/testbank.Dispatcher` mechanism

A throwaway Go harness (NOT committed, removed after use — §11.4.84) loaded
`banks/helixllm_whisper.yaml` via `testbank.LoadFile` and ran every case
through the actual `testbank.Dispatcher` (`os/exec` `DeviceExecFunc` +
`testbank.ContentAssertingResolver`).

### 4a. Run with the correct (unmutated) analyzer

```
[STT-FOX-001] PASS
[STT-HELLOWORLD-001] PASS
[STT-SELF-VALIDATE-001-GOOD] PASS
[STT-SELF-VALIDATE-001-BAD] PASS

TOTAL: pass=4 fail=0 skip=0
overall exit=0
```

Per-fixture PASS verdict JSONs are in `verdicts/` alongside this file
(copied from `submodules/helix_qa/qa-results/helixllm_whisper/*.json`,
git-ignored raw evidence per §11.4.128).

### 4b. Paired §1.1 mutation proof — the analyzer's discriminator is load-bearing

The word-match assertion —

```go
v.Pass = v.Normalized != "" && v.ExpectedCount > 0 && v.MatchedWords == v.ExpectedCount
```

— was replaced with an unconditional `v.Pass = true // MUTATED for paired
§1.1 mutation test - always pass`, rebuilt, and swapped into
`bin/helixqa-verify-whisper`. The SAME bank was re-run through the SAME
`Dispatcher`:

```
[STT-FOX-001] PASS
[STT-HELLOWORLD-001] PASS
[STT-SELF-VALIDATE-001-GOOD] PASS
[STT-SELF-VALIDATE-001-BAD] FAIL  reason=dispatch_exit_1

TOTAL: pass=3 fail=1 skip=0
overall exit=1
```

**The mutation was caught**: with the real discriminator removed, the
analyzer's raw `pass` field wrongly flips to `true` on the golden-bad
fixture (the real fox transcript genuinely does not contain "hello" or
"world"), and `STT-SELF-VALIDATE-001-BAD`'s `--expect-fail` inversion
correctly turns that bluff-PASS into a bank-level FAIL. The mutated verdict
is preserved at
`mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json`:
`"pass": true, "expect_fail": true, "case_result": false`.

The mutation was then **reverted**, rebuilt, and re-run — confirmed
`grep -c "MUTATED for paired" main.go` == 0 (zero residue) and the bank
re-ran clean (`pass=4 fail=0 skip=0`). No mutated code was ever committed.

## 5. Teardown (§11.4.119 single-owner cleanup) — coder confirmed untouched

```
$ ./phase3whisper.bin boot-down compose.phase3whisper.yml phase1qacpu_whisper
DOWN-OK: phase1qacpu_whisper helixllm-stt (volumes removed) via containers submodule orchestrator

$ curl http://localhost:18434/v1/models -> http_status=200
$ podman ps -> ONLY helixllm-coder (unchanged), helixllm-stt container fully removed
```

## 6. Summary

| Item | Result |
|---|---|
| Whisper STT service booted | YES — reused proven harness `boot-up`, `/health` 200 after 1 poll (cached image) |
| Bank authored | `banks/helixllm_whisper.yaml`, 4 cases, grounded fixtures (§11.4.6, live-confirmed this session) |
| Bank run (correct analyzer) | 4 PASS / 0 FAIL / 0 SKIP |
| Self-validation (§11.4.107(10)) | golden-good PASS + golden-bad FAIL (raw) / PASS (case, via `--expect-fail`) |
| Paired §1.1 mutation | Confirmed caught (mutated analyzer → `STT-SELF-VALIDATE-001-BAD` FAILs the bank) |
| Mutation residue | Zero — reverted + re-verified before commit |
| Whisper STT teardown | YES — single-owner, container fully removed |
| Coder (`helixllm-coder`) | Untouched throughout — genuine `/v1/models` HTTP 200 before AND after |

## Sources / evidence paths

- `verdicts/*.json` — per-fixture verdict artefacts from the correct-analyzer run.
- `conduit/conduit.events.jsonl` + `conduit.status.json` — §11.4.116 trace.
- `mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json` — mutated-analyzer verdict.
- `helixllm_whisper.yaml` — the committed bank (copy).
- `helixqa-verify-whisper_main.go.txt` — the committed analyzer source (copy).
