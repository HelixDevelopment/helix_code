# Phase-1 HelixQA CPU-Embeddings Bank — Run Results

**Run ID:** `phase1_helixqa_embeddings_20260708T071321Z`
**Track:** `(T1/feature/helixllm-full-extension)`
**Date:** 2026-07-08
**Scope:** `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` §1.1
(Embeddings — bge-small via TEI, cos margin 0.3578) + the phase-3 proof
`docs/qa/phase3_embeddings_20260706/RESULTS.md`, extending HelixQA with
autonomous test-bank coverage mirroring `banks/helixllm_vision.yaml`.

## 1. Boot (§11.4.76/§11.4.161/§11.4.119, NO GPU)

Booted the existing, already-proven harness
(`docs/qa/phase3_embeddings_20260706/harness/phase3embed.bin`) via its
`boot-up` subcommand (§11.4.74 reuse), `TEI_MODEL_ID=BAAI/bge-small-en-v1.5`
(the ship-now CPU lane — the design-default nomic primary is a known
TEI-parser incompatibility, §11.4.6 honest substitution per the phase-3
proof):

```
$ ./phase3embed.bin boot-up compose.phase3embed.yml phase1qacpu_embed
UP-OK: phase1qacpu_embed tei-embed via containers submodule orchestrator
$ curl http://localhost:18435/health -> 200 (health OK after 1 poll)
```

## 2. Ground-truth fixture authoring (§11.4.6 — grounded against the LIVE service, this session)

The sentence triple was run against the live `:18435` endpoint during
authoring (2026-07-08, this session) via a NEW analyzer
(`bin/helixqa-verify-embeddings`):

```
$ helixqa-verify-embeddings ...
PASS: PROBE-EMB dim=384 cos(A,A')=0.7509 cos(A,U)=0.3931 margin=0.3578 raw_pass=true
    model_served=BAAI/bge-small-en-v1.5
```

The real margin 0.3578 (related pair "The cat sat on the mat." /
"A feline rested on the rug." out-scores the unrelated
"Quarterly revenue rose four percent." pair) exactly matches the
independently-proven signature recorded in
`docs/qa/phase3_embeddings_20260706/RESULTS.md` (2026-07-06).

## 3. Bank authored: `submodules/helix_qa/banks/helixllm_embeddings.yaml`

Analyzer: `submodules/helix_qa/cmd/helixqa-verify-embeddings` (copy alongside
this file). 3 test cases: `EMB-MARGIN-001`, `EMB-SELF-VALIDATE-001-GOOD`,
`EMB-SELF-VALIDATE-001-BAD` (mandatory §11.4.107(10)). The semantic-order
cosine-margin signature is unfakeable — a zero-vector/shuffled/wrong-dim stub
cannot produce related > unrelated by the margin floor.

## 4. Bank run — through the REAL `pkg/testbank.Dispatcher` mechanism

A throwaway Go harness (NOT committed, removed after use — §11.4.84) loaded
`banks/helixllm_embeddings.yaml` via `testbank.LoadFile` and ran every case
through the actual `testbank.Dispatcher` (`os/exec` `DeviceExecFunc` +
`testbank.ContentAssertingResolver`).

### 4a. Run with the correct (unmutated) analyzer

```
[EMB-MARGIN-001] PASS
[EMB-SELF-VALIDATE-001-GOOD] PASS
[EMB-SELF-VALIDATE-001-BAD] PASS

TOTAL: pass=3 fail=0 skip=0
overall exit=0
```

### 4b. Paired §1.1 mutation proof — the analyzer's discriminator is load-bearing

The semantic-order margin assertion —

```go
v.Pass = dimOK && normOK && !math.IsNaN(v.Margin) && v.Margin >= v.MarginFloor
```

— was replaced with `_ = dimOK; _ = normOK; v.Pass = true // MUTATED for
paired §1.1 mutation test - always pass`, rebuilt, and swapped in. The SAME
bank was re-run through the SAME `Dispatcher`:

```
[EMB-MARGIN-001] PASS
[EMB-SELF-VALIDATE-001-GOOD] PASS
[EMB-SELF-VALIDATE-001-BAD] FAIL  reason=dispatch_exit_1

TOTAL: pass=2 fail=1 skip=0
overall exit=1
```

**The mutation was caught**: with the real discriminator removed, the
analyzer's raw `pass` field wrongly flips to `true` on the golden-bad fixture
(the SWAPPED triple where the related sentence is placed in the "unrelated"
slot → margin = −0.3578), and `EMB-SELF-VALIDATE-001-BAD`'s `--expect-fail`
inversion correctly turns that bluff-PASS into a bank-level FAIL. The mutated
verdict is preserved at
`mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json`:
`"pass": true, "case_result": false`.

The mutation was then **reverted**, rebuilt, and re-run — confirmed
`grep -c "MUTATED for paired" main.go` == 0 (zero residue) and the bank
re-ran clean (`pass=3 fail=0 skip=0`). No mutated code was ever committed.

## 5. Teardown (§11.4.119 single-owner cleanup) — coder confirmed untouched

The TEI service was shared with the RAG bank (both ran against it before
teardown). After both banks completed:

```
$ ./phase3embed.bin boot-down compose.phase3embed.yml phase1qacpu_embed
DOWN-OK: phase1qacpu_embed tei-embed (volumes removed) via containers submodule orchestrator
$ curl http://localhost:18434/v1/models -> http_status=200
$ podman ps -> ONLY helixllm-coder (Up 2 hours), tei-embed container fully removed
```

## 6. Summary

| Item | Result |
|---|---|
| Embeddings TEI service booted | YES — reused proven harness `boot-up`, `/health` 200 after 1 poll |
| Bank authored | `banks/helixllm_embeddings.yaml`, 3 cases, grounded fixtures (§11.4.6, live-confirmed this session) |
| Bank run (correct analyzer) | 3 PASS / 0 FAIL / 0 SKIP |
| Self-validation (§11.4.107(10)) | golden-good PASS + golden-bad (swapped/inverted margin) FAIL (raw) / PASS (case, via `--expect-fail`) |
| Paired §1.1 mutation | Confirmed caught (mutated analyzer → `EMB-SELF-VALIDATE-001-BAD` FAILs the bank) |
| Mutation residue | Zero — reverted + re-verified before commit |
| Embeddings TEI teardown | YES — single-owner, container fully removed (shared with RAG bank) |
| Coder (`helixllm-coder`) | Untouched throughout — genuine `/v1/models` HTTP 200 before AND after |

## Sources / evidence paths

- `verdicts/*.json` — per-fixture verdict artefacts from the correct-analyzer run.
- `conduit/conduit.events.jsonl` + `conduit.status.json` — §11.4.116 trace.
- `mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json` — mutated-analyzer verdict.
- `helixllm_embeddings.yaml` — the committed bank (copy).
- `helixqa-verify-embeddings_main.go.txt` — the committed analyzer source (copy).
