# Phase-1 HelixQA CPU-RAG Bank — Run Results

**Run ID:** `phase1_helixqa_rag_20260708T071321Z`
**Track:** `(T1/feature/helixllm-full-extension)`
**Date:** 2026-07-08
**Scope:** `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` §1.1
(RAG-TEI — landed at :18440) + the phase-3 proof
`docs/qa/phase3_rag_20260707/RESULTS.md`, extending HelixQA with autonomous
test-bank coverage mirroring `banks/helixllm_vision.yaml`.

## 1. Services (§11.4.76/§11.4.161/§11.4.119/§11.4.122, NO GPU boot)

The RAG pipeline composes TWO live services this bank only READS: (1) the
CPU embeddings TEI (`BAAI/bge-small-en-v1.5`, `:18435`) — REUSED from the
embeddings-bank boot (§11.4.74, the same running container the embeddings
bank exercised), and (2) the live coder LLM (`Qwen3-Coder-30B`, `:18434`,
READ-ONLY, never restarted/stopped — §11.4.122/§11.4.119). This bank booted
NO new service.

```
$ curl http://localhost:18435/health -> 200 (TEI, shared with the embeddings bank)
$ curl http://localhost:18434/v1/models -> Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf
```

## 2. Ground-truth fixture authoring (§11.4.6 — grounded against the LIVE services, this session)

Two facts are INVENTED (the coder cannot know them from training):
"Borealis-9" (2027 HelixCode release codename, in `doc_codename`),
"Quillfeather-7" (HelixLLM routing sidecar, in `doc_sidecar`). Both queries
+ the no-context RED baseline were run against the live endpoints during
authoring (2026-07-08, this session) via `bin/helixqa-verify-rag`:

```
PROBE-RAG-Q1     top1=doc_codename(ok=true) tokenFound=true answer="...Borealis-9."
PROBE-RAG-Q2     top1=doc_sidecar(ok=true)  tokenFound=true answer="...Quillfeather-7."
PROBE-RAG-Q1-RED no_context=true tokenFound=false answer="I don't have any information about a 2027 HelixCode release..."
```

RED (no grounding) genuinely cannot produce the invented token → GREEN
(grounded) does — the unfakeable §11.4.115 proof that generation is
genuinely grounded in retrieval, corroborating the independently-proven
signatures in `docs/qa/phase3_rag_20260707/RESULTS.md` (2026-07-07):
q1 top1=doc_codename (0.8852), q2 top1=doc_sidecar (0.8855).

## 3. Bank authored: `submodules/helix_qa/banks/helixllm_rag.yaml`

Analyzer: `submodules/helix_qa/cmd/helixqa-verify-rag` (copy alongside this
file) — runs the full embed->retrieve->ground->generate pipeline against the
live endpoints and asserts BOTH retrieval top-1 == expected doc AND the
invented token present. 4 test cases: `RAG-Q1-001`, `RAG-Q2-001`,
`RAG-SELF-VALIDATE-001-GOOD`, `RAG-SELF-VALIDATE-001-BAD` (mandatory
§11.4.107(10) + §11.4.115 RED polarity via `--no-context`).

## 4. Bank run — through the REAL `pkg/testbank.Dispatcher` mechanism

A throwaway Go harness (NOT committed, removed after use — §11.4.84) loaded
`banks/helixllm_rag.yaml` via `testbank.LoadFile` and ran every case through
the actual `testbank.Dispatcher` (`os/exec` `DeviceExecFunc` +
`testbank.ContentAssertingResolver`).

### 4a. Run with the correct (unmutated) analyzer

```
[RAG-Q1-001] PASS
[RAG-Q2-001] PASS
[RAG-SELF-VALIDATE-001-GOOD] PASS
[RAG-SELF-VALIDATE-001-BAD] PASS

TOTAL: pass=4 fail=0 skip=0
overall exit=0
```

### 4b. Paired §1.1 mutation proof — the analyzer's discriminator is load-bearing

The golden-bad case exercises the `--no-context` (RED) branch, so its
load-bearing discriminator is the no-context pass rule —

```go
v.Pass = v.TokenFound // will be false for a genuine invented fact
```

— replaced with `v.Pass = true // MUTATED for paired §1.1 mutation test -
always pass (no-context discriminator stripped)`, rebuilt, swapped in. The
SAME bank re-run through the SAME `Dispatcher`:

```
[RAG-Q1-001] PASS
[RAG-Q2-001] PASS
[RAG-SELF-VALIDATE-001-GOOD] PASS
[RAG-SELF-VALIDATE-001-BAD] FAIL  reason=dispatch_exit_1

TOTAL: pass=3 fail=1 skip=0
overall exit=1
```

**The mutation was caught**: with the no-context discriminator removed, the
analyzer's raw `pass` wrongly flips to `true` on the golden-bad no-context
fixture (the bare question genuinely cannot yield "Borealis-9"), and
`RAG-SELF-VALIDATE-001-BAD`'s `--expect-fail` inversion correctly turns that
bluff-PASS into a bank-level FAIL. The mutated verdict is preserved at
`mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json`:
`"pass": true, "token_found": false, "case_result": false`.

(Honest note §11.4.6: a first mutation attempt touched only the with-context
branch — which the golden-bad case does not exercise — so it was NOT caught;
that was recognised immediately and the mutation was retargeted to the actual
no-context discriminator the golden-bad case runs, which IS caught. This
confirms the correct discriminator is load-bearing.)

The mutation was then **reverted**, rebuilt, and re-run — confirmed
`grep -c "MUTATED for paired" main.go` == 0 (zero residue) and the bank
re-ran clean (`pass=4 fail=0 skip=0`). No mutated code was ever committed.

## 5. Teardown (§11.4.119 single-owner cleanup) — coder confirmed untouched

The shared TEI (`:18435`) was torn down after both the embeddings and RAG
banks completed; the coder was never touched (read-only throughout):

```
$ ./phase3embed.bin boot-down compose.phase3embed.yml phase1qacpu_embed
DOWN-OK: phase1qacpu_embed tei-embed (volumes removed)
$ curl http://localhost:18434/v1/models -> http_status=200
$ podman ps -> ONLY helixllm-coder (Up 2 hours)
```

## 6. Summary

| Item | Result |
|---|---|
| Services | TEI :18435 (reused, no new boot) + coder :18434 read-only — NO GPU service booted by this bank |
| Bank authored | `banks/helixllm_rag.yaml`, 4 cases, grounded invented-fact fixtures (§11.4.6, live-confirmed this session) |
| Bank run (correct analyzer) | 4 PASS / 0 FAIL / 0 SKIP |
| Self-validation (§11.4.107(10) + §11.4.115) | golden-good grounded PASS + golden-bad no-context RED FAIL (raw) / PASS (case, via `--expect-fail`) |
| Paired §1.1 mutation | Confirmed caught (mutated no-context discriminator → `RAG-SELF-VALIDATE-001-BAD` FAILs the bank) |
| Mutation residue | Zero — reverted + re-verified before commit |
| Teardown | YES — shared TEI single-owner removed; coder untouched |
| Coder (`helixllm-coder`) | Untouched throughout — genuine `/v1/models` HTTP 200 before AND after |

## Sources / evidence paths

- `verdicts/*.json` — per-fixture verdict artefacts (with real retrieval ranking + grounded answer) from the correct-analyzer run.
- `conduit/conduit.events.jsonl` + `conduit.status.json` — §11.4.116 trace.
- `mutation_proof/self_validate_001_golden_bad_MUTATED_verdict.json` — mutated-analyzer verdict.
- `helixllm_rag.yaml` — the committed bank (copy).
- `helixqa-verify-rag_main.go.txt` — the committed analyzer source (copy).
