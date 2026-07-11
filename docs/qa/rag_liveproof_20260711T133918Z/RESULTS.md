# RAG Capability — LIVE Re-Validation (§11.4.5 / §11.4.107 / §11.4.108)

**Run ID:** `rag_liveproof_20260711T133918Z`
**Track:** T1 / `feature/helixllm-full-extension`
**Date (UTC):** 2026-07-11T13:40:05Z – 2026-07-11T13:40:11Z
**Harness:** `submodules/helix_llm/docs/qa/phase3_rag_20260707/harness/` (committed `875e53a`, **RUN READ-ONLY** — no source file in the harness was modified; only the gitignored build artifact `phase3rag.bin` was regenerated via `go build .`, its own documented regeneration mechanism per §11.4.77).
**Driver:** a scratchpad-only orchestration script that invokes the harness's own documented subcommands (`boot-up`/`boot-down`/`embed-corpus`/`embed-query`/`retrieve`/`checkretrieval`/`red`/`green`/`analyze`/`selfvalidate`) in the same sequence as the harness's own `run_proof.sh`, redirected to this fresh evidence directory instead of the harness's hardcoded evidence path. This keeps the submodule 100% untouched while producing genuinely fresh, independently-timestamped evidence for this re-validation session.

## Purpose

Re-prove, LIVE, that the RAG (Retrieval-Augmented Generation) pipeline genuinely composes CPU-tier TEI embeddings (`BAAI/bge-small-en-v1.5`, dim 384, port `:18440`) with the live coder LLM (`helixllm-coder`, Qwen3-Coder-30B-A3B-Instruct, port `:18434`) to ground generation in retrieved facts — with an unfakeable, invented-fact proof (§11.4.6 / §11.4.107): the coder cannot know the fact without retrieval (RED), and correctly produces the exact invented token only when grounded in the real top-1 retrieved document (GREEN).

## Hard constraints honored

- **TEI embeddings = CPU-tier, own port `:18440`, NO GPU contention.** The TEI container (`ghcr.io/huggingface/text-embeddings-inference:cpu-1.9`) was booted with no GPU device, `mem_limit=8g`, `cpus=8`, via the `digital.vasic.containers` orchestrator (§11.4.76), rootless podman (§11.4.161). The GPU vision-generation stream at `:18439` was never touched — see `00_preflight.txt` (pre-existing `:18439 LISTENING`) and `29b_post_teardown.txt` (still present after our teardown; it was a short-lived generation job from a sibling track that completed and self-cleaned independently of this run — confirmed via `podman ps -a` showing no residual `vision` container, and we issued zero commands against it).
- **Coder `:18434` READ-ONLY, never restarted.** Every interaction with the coder was a plain HTTP `GET /v1/models` or `POST /v1/chat/completions` call — no `podman restart`/`stop`/`kill` was ever issued against `helixllm-coder`. Confirmed running, `Up 44 minutes` at pre-flight and `Up 45 minutes` (still the SAME container, uptime only grew) at post-teardown, with `/v1/models` returning HTTP 200 both times (§11.4.122).
- **Harness run READ-ONLY; evidence committed only at ROOT `docs/qa`.** No file inside `submodules/helix_llm/` was edited. This deliverable is the ONLY new content, written to `docs/qa/rag_liveproof_20260711T133918Z/` at the outer/root repo.
- **Sibling ports `18435-18438` and CPU-caps range untouched** — confirmed free before and after (see `00_preflight.txt` / `29b_post_teardown.txt`).
- **Single-resource-owner (§11.4.119):** this run owned exactly one exclusive resource — the compose project `phase3ragliveproof` bound to port `:18440` — booted and torn down by this run alone; the coder and the sibling vision stream were passively observed only (read-only `GET`/`ps` probes), never driven.

## RED baseline (§11.4.115) — load-bearing anti-bluff proof

Both invented facts were asked of the **bare** coder with **zero retrieved context**, proving the facts are genuinely unknown to the model (not memorized/leaked) — this is the load-bearing precondition for the whole proof: if RED failed (coder produced the invented token unprompted), the GREEN pass below would prove nothing.

| qkey | Question | Invented token | RED answer (verbatim, no context) | Verdict |
|---|---|---|---|---|
| q1 | "What is the internal codename for the 2027 HelixCode release?" | `Borealis-9` | *"I don't have any information about a 2027 HelixCode release or its internal codename, as this appears to be fictional or hypothetical."* | **RED-OK** — token absent |
| q2 | "What is the name of the internal service-mesh sidecar used for HelixLLM routing?" | `Quillfeather-7` | *"The internal service-mesh sidecar used for HelixLLM routing is called \"HelixProxy\"."* | **RED-OK** — coder hallucinated a **different**, plausible-sounding name (`HelixProxy`) — not the invented token — proving the RED baseline is genuinely load-bearing (it isn't merely "I don't know"; the model actively confabulates something else entirely and still misses the real invented fact) |

Both `red_exit_q1=0` and `red_exit_q2=0` — the harness's own `containsToken()` check would have exited non-zero (`RED-VIOLATION`) had either answer contained the invented token. Raw evidence: `10_red_baseline.txt`, `red_response_q1.json`, `red_response_q2.json`.

## TEI boot (real container, real health)

```
[13:40:05] boot TEI lane model=BAAI/bge-small-en-v1.5 port=18440 project=phase3ragliveproof via containers submodule orchestrator
UP-OK: phase3ragliveproof tei-rag via containers submodule orchestrator
[13:40:10] health OK after 2 polls
```

Container state after boot (`24_container_state.txt`):
```
helixllm-coder                    localhost/helixllm/llamacpp-router:cuda12.8-sm120        Up 44 minutes    8080/tcp, 50052/tcp
helixllm_visiongen_visiongen_1    localhost/helixllm/llamacpp-router:cuda12.8-sm120        Up About a minute  0.0.0.0:18439->18439/tcp, 8080/tcp, 50052/tcp
phase3ragliveproof_tei-rag_1      ghcr.io/huggingface/text-embeddings-inference:cpu-1.9    Up 4 seconds     0.0.0.0:18440->80/tcp
```
Three independent containers coexisting cleanly on disjoint ports — proof of §11.4.119 partitioning. Raw evidence: `20_boot.txt`, `21_health.txt`, `24_container_state.txt`.

## GREEN — real embed → real cosine retrieve top-1 → grounded generate

Corpus (6 docs, 2 fact-bearing + 4 distractors) embedded via a REAL HTTP call to the live TEI container: `EMBED-CORPUS-OK: model=BAAI/bge-small-en-v1.5 dim=384 docs=6` (`30_embed_corpus.txt`, `corpus_embeddings.json` — 50008 bytes of real 384-dim float vectors).

### q1 — codename

| Step | Result |
|---|---|
| Query embed | REAL TEI call, dim=384 (`query_embedding_q1.json`) |
| Real cosine retrieval (all 6 docs ranked) | rank1 `doc_codename` score=**0.8852**, rank2 `doc_ci` 0.7166, rank3 `doc_sidecar` 0.6766, rank4 `doc_coffee` 0.4577, rank5 `doc_revenue` 0.3922, rank6 `doc_cat` 0.3029 |
| `checkretrieval` | **PASS** — top-1 = `doc_codename` (the fact-bearing doc), exit 0 |
| Grounded generation (top-2 context stuffed) | *"The internal codename for the 2027 HelixCode release is Borealis-9."* |
| `analyze` (RAG runtime signature) | **PASS** — `top1=doc_codename(score=0.8852, ok=true) tokenFound=true` |

### q2 — sidecar

| Step | Result |
|---|---|
| Query embed | REAL TEI call, dim=384 (`query_embedding_q2.json`) |
| Real cosine retrieval (all 6 docs ranked) | rank1 `doc_sidecar` score=**0.8855**, rank2 `doc_codename` 0.6819, rank3 `doc_ci` 0.6336, rank4 `doc_coffee` 0.4619, rank5 `doc_revenue` 0.4114, rank6 `doc_cat` 0.3477 |
| `checkretrieval` | **PASS** — top-1 = `doc_sidecar` (the fact-bearing doc), exit 0 |
| Grounded generation (top-2 context stuffed) | *"The internal service-mesh sidecar used internally for HelixLLM routing is called Quillfeather-7."* |
| `analyze` (RAG runtime signature) | **PASS** — `top1=doc_sidecar(score=0.8855, ok=true) tokenFound=true` |

Both retrieval rankings are REAL cosine similarity over REAL TEI-served embeddings (`main.go:cmdRetrieve`, sorted by score, never a hardcoded pick) — the distractor documents (`doc_cat`, `doc_revenue`, `doc_coffee`, `doc_ci`) score visibly lower for both queries, proving the retriever genuinely discriminates rather than always returning doc 0. Raw evidence: `11_green_proof_q1.txt`, `11_green_proof_q2.txt`, `retrieval_q1.json`, `retrieval_q2.json`, `green_response_q1.json`, `green_response_q2.json`.

## Self-validation of the analyzer (§11.4.107(10)) — golden-bad MUST FAIL

For both q1 and q2, the analyzer was mutation-tested against 3 deliberately-degraded fixtures derived from the SAME golden-good captured retrieval+generation:

| Fixture | q1 verdict | q2 verdict |
|---|---|---|
| GOLDEN-GOOD (expect PASS) | **PASS** | **PASS** |
| GOLDEN-BAD-NO-FACT — answer replaced with "I don't know based on the given context." (expect FAIL) | **FAIL** ✓ | **FAIL** ✓ |
| GOLDEN-BAD-WRONG-RETRIEVAL — top-1 swapped for an irrelevant distractor doc, same score (expect FAIL) | **FAIL** ✓ (`top1=doc_ci … ok=false`) | **FAIL** ✓ (`top1=doc_cat … ok=false`) |
| GOLDEN-BAD-EMPTY-ANSWER — answer blanked (expect FAIL) | **FAIL** ✓ | **FAIL** ✓ |

`[SELF-VALIDATION] PASS: analyzer PASSes golden-good and FAILs all golden-bad fixtures` for both qkeys (`selfvalidate_exit_q1=0`, `selfvalidate_exit_q2=0`). This proves the analyzer is not a bluff gate — it genuinely discriminates real success from every degraded-fixture class asked of it. Raw evidence: `12_self_validation_q1.txt`, `12_self_validation_q2.txt`.

## Coder-untouched proof (§11.4.119 / §11.4.122)

| Checkpoint | Coder state |
|---|---|
| Pre-flight (before TEI boot) | `helixllm-coder … Up 44 minutes`, `/v1/models` HTTP 200 |
| Post-teardown (after full RAG cycle + TEI teardown) | `helixllm-coder … Up 45 minutes` (same container, uptime advanced by exactly the run's wall-clock — never restarted), `/v1/models` HTTP 200 |

No `podman restart|stop|kill` was ever issued against the coder container in this run. Raw evidence: `00_preflight.txt`, `29b_post_teardown.txt`.

## Teardown (single-owner cleanup)

```
[13:40:10] teardown project=phase3ragliveproof (single-owner cleanup, coder untouched) ...
DOWN-OK: phase3ragliveproof tei-rag via containers submodule orchestrator
```
Post-teardown: `phase3ragliveproof_tei-rag_1` — **removed** (`podman ps -a` shows none matching `phase3ragliveproof_`); the shared `helixllm-tei-cache` volume was preserved (not deleted — `WithDownRemoveVolumes(false)`, a re-obtainable §11.4.77 artefact, not project-specific state); port `:18440` free again; sibling ports `18435-18438` still free; port `18439` returned to its own pre-existing state (the sibling vision-generation container had already self-completed, independent of this run). Raw evidence: `29_teardown.txt`, `29b_post_teardown.txt`.

## Verdict

**PASS — RAG capability live-re-validated.** RED baseline load-bearing (both invented tokens genuinely unknown to the bare coder — q2 even shows active confabulation of a different plausible name, `HelixProxy`, ruling out a lucky "I don't know" heuristic). GREEN pipeline exercises REAL TEI embeddings, REAL cosine retrieval (correct top-1 for both queries, distractors scored visibly lower), and REAL grounded generation from the live coder producing the exact invented token for both queries. Analyzer self-validated against 3 golden-bad fixture classes per query (6 total), all correctly FAIL. Coder never restarted; sibling GPU vision stream (`:18439`) and CPU-caps ports (`18436-18438`) never touched; TEI lane cleanly torn down.

## Evidence manifest

```
00_preflight.txt              pre-run state: coder up, TEI port free, siblings free/untouched
10_red_baseline.txt           RED baseline log (both qkeys)
red_response_{q1,q2}.json     raw coder chat-completion responses, no context
20_boot.txt / 21_health.txt   TEI container boot + health-poll log
24_container_state.txt        podman ps snapshot mid-run (3 containers coexisting)
30_embed_corpus.txt           corpus embedding call log
corpus_embeddings.json        real 384-dim vectors for all 6 corpus docs
query_embedding_{q1,q2}.json  real 384-dim query vectors
retrieval_{q1,q2}.json        real cosine-ranked retrieval (all 6 docs, scores)
11_green_proof_{q1,q2}.txt    full per-query pipeline log (embed→retrieve→check→green→analyze)
green_response_{q1,q2}.json   raw coder chat-completion responses, grounded
12_self_validation_{q1,q2}.txt   analyzer self-validation log (golden-good + 3 golden-bad)
29_teardown.txt / 29b_post_teardown.txt   teardown log + coder/siblings-untouched proof
```

## Sources verified

Harness source read directly: `submodules/helix_llm/docs/qa/phase3_rag_20260707/harness/main.go` (commit `875e53a`), `run_proof.sh`, `compose.phase3rag.yml` — no external documentation lookup was required for this re-validation (pure re-execution of an already-proven, already-documented harness against live infrastructure).
