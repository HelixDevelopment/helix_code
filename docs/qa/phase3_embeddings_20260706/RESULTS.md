# Phase-3 CPU Embeddings — end-to-end proof (§11.4.108 / §11.4.5 / §11.4.69)

**Date:** 2026-07-06 · Track `(T1/feature/helixllm-full-extension)` · Design: `docs/research/07.2026/00_master/EMBEDDINGS_PROVIDER.md`

## Verdict (honest, §11.4.6)

**CAPABILITY PROVEN.** A real CPU embeddings service booted via the **containers submodule
orchestrator** (§11.4.76, rootless podman §11.4.161, NO GPU) served real, deterministic,
semantically-ordered embeddings. The §11.4.108 runtime signature is **GREEN-OK**:

```
[RUNTIME-SIGNATURE] PASS  dim=384  |A|=1.0000 |A'|=1.0000 |U|=1.0000
                   cos(A,A')=0.7509   cos(A,U)=0.3931   margin=0.3578  (required ≥ 0.15)
[DETERMINISM]      PASS  3 vectors byte-identical across two identical requests
GREEN-OK
```
(`A` vs `A'` = related/paraphrase pair; `A` vs `U` = unrelated pair. Related similarity ≫ unrelated,
margin 0.3578 ≫ the 0.15 floor. Real HTTP 200, non-zero norm, evidence `11_green_proof.txt`,
`30_embed_1.txt`, `green_response_{1,2}.json`.)

## Lane (honest substitution, §11.4.6 — `23_substitution.txt`)

- **Primary (design default) `nomic-ai/nomic-embed-text-v1.5` FAILED to boot** — TEI `cpu-1.9`
  rejects its `config.json`: *"duplicate field `max_position_embeddings`"* (container exit 1,
  `22_teilogs_primary.txt`). Known nomic/TEI parser incompatibility.
- **Fell back to `BAAI/bge-small-en-v1.5` (dim 384) — TEI-native, healthy, served the proof.**
  Both lanes are in the design; the fallback is the ship-now CPU lane. Follow-up: pin a
  TEI-`cpu-1.9`-parseable nomic revision or upgrade TEI if 768-dim nomic is wanted.

## Analyzer is non-bluff (§11.4.107(10) / §11.4.115)

Proven to genuinely discriminate — it does NOT rubber-stamp:
- **RED baseline** (`10_red_baseline.txt`): the dim-1536 zero-vector gateway stub → correctly **FAIL**
  (zero-norm, NaN cosine). Defect reproduced (§11.4.115).
- **Golden-BAD fixtures** all correctly **FAIL**: zero-vector, shuffled-order (margin −0.3578),
  wrong-dim. The analyzer rejects every degraded input.
- **Real GREEN input** correctly **PASS** (above).

## Known limitation (tracked follow-up — NOT a capability or analyzer defect)

`12_self_validation.txt` reports the **golden-GOOD** self-validation FAIL: the analyzer's baked
golden-good fixture is **dim-768 (the nomic primary lane)**, so on the **bge-small 384** fallback
lane it fails the dim match (`expected 768, got 384`). This is a **fixture-dim limitation**, not a
bluff — the analyzer's discrimination is independently proven above (RED + 3 golden-BAD all FAIL,
real GREEN passes). **Follow-up P3-EMB-1:** make the golden-good fixture lane-dim-aware (or capture a
real 384-dim golden-good), so the full §11.4.107(10) self-validation is GREEN on whichever lane runs.

## Reproduce

`cd harness && ./run_proof.sh` (boots TEI via the containers submodule, POSTs the sentence triple,
asserts the cosine signature + determinism + self-validation, tears the container down single-owner
§11.4.119, leaves `helixllm-coder` untouched). `HEALTH_TIMEOUT` env tunes the per-lane health wait.
The `harness/phase3embed.bin` is a build artifact (gitignored §11.4.30; rebuild via `go build`).

## Composition

§11.4.76 (containers submodule) · §11.4.161 (rootless) · §11.4.108 (runtime signature) ·
§11.4.107(10) (self-validated analyzer) · §11.4.115 (RED-first) · §11.4.119 (single-owner teardown) ·
§11.4.6 (honest substitution + honest limitation) · §11.4.147 (conductor completion of the stopped subagent's proof).
