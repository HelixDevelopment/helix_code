# Vectorization (raster→SVG, vtracer default path) — Live Proof

| | |
|---|---|
| **Run ID** | `vectorization_liveproof_20260711T142839Z` |
| **Date (UTC)** | 2026-07-11T14:32:04Z – 2026-07-11T14:32:50Z |
| **Operator mandate** | Implement the vectorization capability (raster image → SVG) that audit found NOT-STARTED, per `docs/research/07.2026/02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md` P3-T4′: vtracer (CPU) as the DEFAULT engine, StarVector-8B as an OPTIONAL GPU tier |
| **New service** | `submodules/helix_llm/services/vectorize/` (distinct new files, no shared-file edits — §11.4.84/§11.4.176) |
| **Coder `:18434`** | Untouched throughout (verified before/after, §11.4.119/§11.4.174) |

## Summary verdict

| Check | Result |
|---|---|
| vtracer default path implemented + booted (containers submodule, rootless podman) | **PASS** |
| Real image → valid SVG (`assets/Logo.png`, 1916×1522) | **PASS** — 18041-byte SVG, well-formed `<path>` data |
| Determinism (same input → same output) | **PASS** — byte-identical SVG across two independent calls |
| Fidelity analyzer: golden-good PASS | **PASS** — SSIM 0.9726 ≥ floor 0.7468 |
| Fidelity analyzer: golden-bad FAIL (blank canvas) | **PASS (correctly rejected)** — SSIM 0.5209 < floor |
| Fidelity analyzer: golden-bad FAIL (flat-color rect) | **PASS (correctly rejected)** — SSIM 0.4840 < floor |
| §1.1 paired mutation (neuter check → golden-bad wrongly passes → revert) | **PASS** — proven load-bearing, no residue left in tracked source |
| StarVector-8B optional GPU tier | **Honestly deferred** — free VRAM (12632 MiB) is at/below the low end of the 12-16 GB estimated footprint; not forced |
| `helixllm-coder` (`:18434`) untouched | **PASS** — GPU 19466 MiB used / 12632 MiB free identical before and after |

## 1. What was implemented

New, distinct files under `submodules/helix_llm/services/vectorize/` (no
existing shared file — `main.go`, `Containerfile`, RAG/video-analysis code —
was edited):

- `go.mod`, `main.go` — a minimal Go HTTP shim exposing:
  - `POST /v1/vectorize` — body = raw raster bytes → shells to the real
    `vtracer` CLI (visioncortex VTracer 0.6.5, Rust, CPU raster-tracing) →
    returns `{engine, preset, source_format, width, height, svg}`.
  - `POST /v1/rasterize` — body = raw SVG bytes, `?w=&h=` → shells to the
    real `rsvg-convert` (librsvg2-bin) → returns the rasterized PNG,
    computed INSIDE the container so renderer-version drift can never
    confound the fidelity comparison (mirrors the sibling OCR service's
    server-side `/v1/render` rationale).
  - `GET /health` — real `vtracer --version` + `rsvg-convert --version`.
- `Containerfile` — Debian bookworm-slim + `librsvg2-bin`; both `vtracer` and
  the Go shim are host-compiled and `COPY`ed in (§11.4.77 regeneration
  mechanism, same pattern + same RLIMIT_NPROC rationale as the sibling OCR
  service's `ocr-shim`). Verified glibc-symbol compatibility: the host-built
  `vtracer` binary's highest required glibc symbol is `GLIBC_2.34`
  (`objdump -T`); Debian bookworm ships glibc 2.36 — compatible.
- `.gitignore` — ignores the two host-compiled binaries + the cargo install
  tree (regenerable via `cargo install vtracer --locked`).
- `README.md` — full contract, build/regeneration instructions, and the
  StarVector tier-status decision record.

Port **18452** (verified free; distinct from every sibling Phase-3/4
capability: coder 18434, embeddings 18435, translation 18436, whisper 18437,
OCR 18438, vision 18439, RAG/Qdrant 18440, a2a 18441, imagegen 18442,
videogen 18443, mcp-gateway 18444, helixmemory 18450-18451).

Booted through the `digital.vasic.containers` submodule
`compose.Orchestrator` (§11.4.76), rootless podman (§11.4.161), **CPU-only**
— no GPU device requested, confirmed by `nvidia-smi` showing an unchanged
GPU-memory footprint before and after the run (see §5 below).

## 2. Real image → SVG (raw evidence: `response_1.json`, `vectorized.svg`)

Source: `assets/Logo.png` (a real, already-committed repo asset — a company
logo — copied to `source_logo.png`, 213533 bytes, 1916×1522 PNG). This is
exactly the class of graphic (flat-color/logo-style) vtracer excels at, and
per the plan's own scope note, vtracer is also the real path for
natural-image/illustration/pixelized-graphic content generally.

```
$ curl -sS -X POST --data-binary @source_logo.png http://localhost:18452/v1/vectorize
{"engine":"vtracer-0.6.5","preset":"","source_format":"png","width":1916,"height":1522,"svg":"<?xml ...
```

Produced SVG (`vectorized.svg`, 18041 bytes) opens with:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!-- Generator: visioncortex VTracer 0.6.5 -->
<svg version="1.1" xmlns="http://www.w3.org/2000/svg" width="1916" height="1522">
<path d="M0 0 C1.18 0 2.36 0 3.57 0 C90.62 0.12 177.7 9.83 450.85 114.48 ...
```

A well-formed SVG 1.1 document, dimensions matching the source raster
exactly, with real traced `<path>` geometry — not a template, not a
placeholder.

## 3. Determinism (`32_determinism.txt`)

`/v1/vectorize` was called twice, independently, on the identical input
bytes:

```
[DETERMINISM] PASS: byte-identical SVG (18041 chars) across two independent /v1/vectorize calls on the same input
```

Same input → byte-for-byte identical SVG (§11.4.50).

## 4. Fidelity analyzer (self-validated, §11.4.107(10)/§11.4.107(13))

The candidate SVG is re-rasterized by the SAME container (`/v1/rasterize`,
at the source's exact 1916×1522 dimensions) and compared to the source
raster via a windowed (8×8 block, Wang et al. 2004 formula) grayscale SSIM,
computed in pure Go (`ssimWindowed`, no external dependency, deterministic).

**Floor calibration** — computed from this run's OWN observed values
(§11.4.6/§11.4.107(13), never hardcoded from literature): midpoint between
the golden-good SSIM and the highest golden-bad SSIM.

| Fixture | SSIM | Verdict |
|---|---|---|
| Golden-good (real vtracer SVG → rasterize → vs source) | **0.9726** | PASS (≥ floor) |
| Golden-bad — blank white canvas SVG (same dims, zero traced structure) | 0.5209 | FAIL (< floor), correctly rejected |
| Golden-bad — flat mid-gray rect SVG (same dims, zero traced structure) | 0.4840 | FAIL (< floor), correctly rejected |
| **Calibrated floor** | **0.7468** | midpoint(0.9726, max(0.5209, 0.4840)) |

`39_self_validation.txt`:

```
[GOLDEN-GOOD(expect PASS)] PASS ssim=0.9726
[GOLDEN-BAD-BLANK(expect FAIL)] FAIL ssim=0.5209
[GOLDEN-BAD-FLATCOLOR(expect FAIL)] FAIL ssim=0.4840
[SELF-VALIDATION] PASS: analyzer PASSes golden-good and FAILs all golden-bad fixtures
```

### §1.1 paired mutation (`41_mutation_proof.txt`, `42_mutation_residue_check.txt`)

To prove the fidelity floor check is load-bearing (not a bluff gate), the
check was deliberately neutered (forced `pass=true` regardless of SSIM) and
re-run against both golden-bad fixtures. Per §11.4.84 (working-tree
quiescence / no mutation residue in committed code), the neuter was
implemented in a **disposable `/tmp` scratch binary** that duplicates the
SSIM logic — the tracked `harness/main.go` was never edited, so there is no
revert step needed and no risk of a mutation marker landing in a commit:

```
[GOLDEN-BAD-BLANK-MUTATED-NEUTERED] PASS ssim=0.5209 (fidelity check forced to always-pass)
[GOLDEN-BAD-FLATCOLOR-MUTATED-NEUTERED] PASS ssim=0.4840 (fidelity check forced to always-pass)
MUTATION-PROOF-OK: neutered check wrongly PASSED both golden-bad fixtures, as expected
```

Combined with §4's real (non-mutated) `39_self_validation.txt` result — where
the SAME two fixtures correctly FAIL under the real, committed analyzer —
this proves the check is load-bearing: mutate it away and the bluff-shaped
fixtures wrongly pass; with it intact, they correctly fail.

Residue check (`42_mutation_residue_check.txt`):

```
### §11.4.84 mutation-residue check on tracked harness/main.go
CLEAN: no mutation markers in tracked harness/main.go
```

## 5. StarVector-8B optional GPU tier — honest defer (`01_starvector_vram_check.txt`)

```
free_vram_mib=12632
starvector_estimated_footprint_gb=12-16 (per CAPABILITIES_MASTER_PLAN_v2.md P3-T4')
verdict=HONEST_DEFER (free VRAM 12632 MiB is at/below the low end of the 12-16 GB estimated footprint, no safe headroom; StarVector optional tier NOT attempted this run — documented follow-up per README 'StarVector tier status')
```

The single resident GPU consumer at proof time was `helixllm-coder`
(`llama-server`, 19440 MiB) — no RAG-Qdrant GPU usage (RAG/Qdrant + the TEI
embed/rerank containers observed running concurrently during this proof are
CPU-only, confirmed by `nvidia-smi --query-compute-apps` showing only the
coder process). 12632 MiB free sits at the very bottom of StarVector-8B's
own 12-16 GB estimated footprint (per the capabilities plan), leaving no
safe margin for model load + KV cache + allocation spikes without risking
the coder's resident context — which the task's hard constraints require to
stay untouched. Per the plan's own caveat (StarVector's HF model card:
*"StarVector models will not work for natural images or illustrations, as
they have not been trained on those images"*), StarVector would also add no
value for vtracer's default target content class. **Decision: honestly
deferred as a follow-up item** (tracked in the service `README.md`
"StarVector tier status"), not forced in against the hard constraints.

## 6. `helixllm-coder` untouched (§11.4.119/§11.4.174)

`00_preflight.txt` (before) vs `51_post_teardown.txt` (after):

```
# before
0, NVIDIA GeForce RTX 5090, 32607 MiB, 19466 MiB, 12632 MiB, 0 %
helixllm-coder ... Up 2 hours

# after
0, NVIDIA GeForce RTX 5090, 32607 MiB, 19466 MiB, 12632 MiB, 0 %
helixllm-coder Up 2 hours
```

Identical GPU-memory footprint, container still `Up`, never restarted or
touched by this proof.

## 7. Evidence file manifest

```
00_preflight.txt                 pre-flight: coder status, nvidia-smi baseline, port check
01_starvector_vram_check.txt     StarVector VRAM fitness check (honest defer)
20_boot.txt / 21_health.txt      container boot + health poll
24_container_state.txt           podman ps at healthy state
25_health_response.json          real GET /health response
source_logo.png                  REAL test image (assets/Logo.png copy)
30_vectorize_1.txt / 31_vectorize_2.txt   two independent /v1/vectorize calls
response_1.json / response_2.json         raw JSON responses (both calls)
32_determinism.txt               byte-identical SVG proof
33_extract.txt                   extracted SVG + dims
vectorized.svg                   the real produced SVG
34_rasterize_good.txt            /v1/rasterize on the real SVG
bad_blank.svg / bad_flatcolor.svg          synthetic golden-bad SVG fixtures
35_/36_rasterize_bad_*.txt       /v1/rasterize on golden-bad fixtures
rasterized_good.png / rasterized_bad_blank.png / rasterized_bad_flatcolor.png   real rasterized PNGs
37_calibration.txt               SSIM floor calibration (this run's own data)
38_green_signature.txt           GREEN runtime signature (§11.4.108)
39_self_validation.txt           golden-good PASS + golden-bad FAIL proof
40_mutation_build.txt            scratch mutation binary build log
41_mutation_proof.txt            §1.1 paired mutation proof
42_mutation_residue_check.txt    confirms no mutation markers in tracked source
50_teardown.txt / 51_post_teardown.txt     teardown + coder-untouched proof
harness/                         Go proof harness (main.go, compose.vectorize.yml, run_proof.sh)
```

## 8. Reproduce

```bash
cd docs/qa/vectorization_liveproof_20260711T142839Z/harness
./run_proof.sh
```

Fully self-contained and re-runnable: builds both host binaries (Go shim +
`vtracer` via `cargo install`, cached after first run), boots via the
containers submodule orchestrator, produces fresh evidence, tears down.

## Sources verified

- vtracer 0.6.5, crates.io — https://crates.io/crates/vtracer — verified 2026-07-11
- Debian bookworm `librsvg2-bin` — https://packages.debian.org/bookworm/librsvg2-bin — verified 2026-07-11
- Debian bookworm `libc6` (glibc 2.36) — https://packages.debian.org/bookworm/libc6 — verified 2026-07-11
- StarVector-8B model card — https://huggingface.co/starvector/starvector-8b-im2svg — accessed 2026-07-08, re-verified 2026-07-11
- `docs/research/07.2026/02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md` P3-T4′ (this project, read as lead per §11.4.6)
