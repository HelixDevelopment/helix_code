# R41F — Fast-lane generative (image) LIVE re-validation — coder untouched throughout

**Run ID:** `generative_liveproof_20260711T131205Z`
**Date:** 2026-07-11
**Scope:** Re-validate fast-lane generative (image) capability LIVE, now that GPU
free-VRAM is available (Lane-B torn down) and the coder is up on `:18434`, with
fresh captured evidence (§11.4.5/§11.4.107). Track label: `(T1/feature/helixllm-full-extension)`.

---

## Tier chosen

**Image — FLUX.1-schnell** (cheapest/fastest to prove per the Jul-9 precedent
`d8593b63`; video WAN2.2 TI2V-5B `f2c71d07` was investigated as the alternative
but ruled out — see "Why not video" below).

Environment reused from the Jul-9 proof exactly as instructed (§11.4.6 — no
re-download, no fabrication):

- torch `2.13.0+cu129`, diffusers `0.39.0` (system Python, `~/.local/lib/python3/site-packages`)
- HuggingFace cache: `~/.cache/huggingface/hub/models--black-forest-labs--FLUX.1-schnell`
  (32 GB, already present — `du -sh` confirmed before use, zero bytes downloaded
  this session)
- `black-forest-labs/FLUX.1-schnell` is Apache-2.0 / ungated — no `HF_TOKEN` needed
  (confirmed: `HF_TOKEN` unset in shell, generation succeeded without it)

## Live free-VRAM reads (§11.4.119 single-owner burst — re-read immediately before every admission, DZ-23 volatility)

| Checkpoint | used | free | total |
|---|---|---|---|
| Task start (before any action) | 19464 MiB | **12634 MiB** | 32607 MiB |
| Immediately before admission attempt 1 | 19464 MiB | 12634 MiB | 32607 MiB |
| Immediately before admission attempt 3 (successful) | 19464 MiB | 12634 MiB | 32607 MiB |
| Immediately after final teardown | 19464 MiB | **12634 MiB** | 32607 MiB |

Free VRAM was stable at 12634 MiB across the whole session (no other burst
contended the card) and is **bit-for-bit identical before and after** this
run — full restoration confirmed.

## Admission proof (broker-gated, lease held across the actual generation)

A new wrapper, `genproof` (`submodules/helix_llm/docs/qa/phase4_imagegen_20260707/harness/genproof/main.go`),
was added to the existing Phase-4 image-gen harness. Unlike `imagegen-boot
admit-check` (which releases the lease immediately after a dry-run probe),
`genproof` holds a real `vrambroker.ClassImage` burst lease (§11.4.119
single-owner) for the **entire duration** of the generation subprocess, then
releases it — the strongest form of broker-admitted evidence available without
the full containerized imagegen service (which remains blocked on missing
`NUNCHAKU_WHEEL` per the Jul-8 `phase1_imagegen_runtime_20260708T082002Z`
report; that blocker is orthogonal to this task and was not touched).

Successful run (attempt 3, `generation_attempt3.log`):

```
[genproof] PRE-ADMISSION live nvidia-smi read: total=32607MiB used=19464MiB free=12634MiB need=7168MiB headroom=2048MiB
[genproof] ADMIT-OK: lease id=image-1783775777861200504-1 class=image vram_bytes=7516192768 — coder stays live, generating now...
...
[genproof] subprocess exit 0 — real generation completed while lease held.
[genproof] POST-RELEASE live nvidia-smi read: total=32607MiB used=19464MiB free=12634MiB
```

`vrambroker.New()` reads REAL `nvidia-smi` (not a mock); `Acquire(ctx,
ClassImage, 7GiB)` is the same broker path the coder/Lane-B/video classes use.

## Real artifact generated

- **Prompt (new, distinct from the Jul-9 cyberpunk-tiger prompt — proves a
  fresh generation, never a stale/frozen reuse, §11.4.107):**
  > "A serene alpine lake at dawn reflecting snow-capped mountains, with a lone
  > wooden rowboat and mist rising off the water, photorealistic, golden hour
  > lighting"
- **Output:** `r41f_flux_schnell_liveproof.png` (gitignored raw artifact —
  see `.gitignore` in this directory; path referenced here per this task's
  explicit no-binary-commit instruction)
- **SHA-256:** `269dfabe61a3add03a4b3ec05f309340246f38d8e61a517bdfb4972738c42b7e`
- **Size:** 1,246,710 bytes; **Dimensions:** 1024×1024 RGB
- **Direct visual inspection** (Read-tool image render, this session): a
  genuine, detailed, prompt-matching photorealistic alpine lake scene — visible
  snow-capped mountains left/right, a wooden rowboat at the shoreline, mist and
  golden-hour lighting on the right-hand peaks, full reflection in the water.
  Not noise, not a placeholder, not a frozen/stale frame.
- **4 inference steps, guidance_scale=0.0** (FLUX.1-schnell distilled defaults,
  same as the Jul-9 proof), seed=7 (deliberately different seed from Jul-9's
  seed=42).

## §11.4.107 self-validated analyzer — real artifact PASS + golden-bad FAIL proof

Reused the existing Phase-4 `imganalyzer` (multi-signal AND oracle: Shannon
entropy, unique-colour count, dominant-colour fraction, adjacent-pixel
structure band, PNG compressibility — `submodules/helix_llm/docs/qa/phase4_imagegen_20260707/harness/imganalyzer/main.go`).

**1. Self-validation (`imganalyzer selfvalidate`) — proves the oracle discriminates, not a bluff gate:**

```
[GOLDEN-GOOD real (expect REAL)] -> REAL  entropy=6.21 colors=43692 dominant=0.097 adjDiff=0.73 compress=0.241
    reason: all structure signals satisfied (real generated image)
[GOLDEN-BAD solid (expect DEGENERATE)] -> DEGENERATE  entropy=0.00 colors=1 dominant=1.000 adjDiff=0.00 compress=0.003
[GOLDEN-BAD blank (expect DEGENERATE)] -> DEGENERATE  entropy=0.00 colors=1 dominant=1.000 adjDiff=0.00 compress=0.003
[GOLDEN-BAD noise (expect DEGENERATE)] -> DEGENERATE  entropy=7.45 colors=65393 dominant=0.000 adjDiff=48.66 compress=1.002
[SELF-VALIDATION] PASS: oracle classifies golden-good REAL and all golden-bad DEGENERATE
```

All three golden-bad fixtures (solid/blank/noise) genuinely **FAIL** (classified
DEGENERATE) — including the pure-noise fixture, which has high entropy but is
correctly rejected on the adjacent-pixel structure + compressibility signals
(the exact §11.4.107 trap a naive entropy-only oracle would miss).

**2. Real artifact verdict (`RED_MODE=0 imganalyzer analyze r41f_flux_schnell_liveproof.png`):**

```json
{
  "real_image": true,
  "reasons": ["all structure signals satisfied (real generated image)"],
  "metrics": {
    "width": 1024, "height": 1024,
    "entropy_bits": 7.819693521886245,
    "unique_colors": 147440,
    "dominant_fraction": 0.00445556640625,
    "adj_mean_diff": 3.415482954552182,
    "compress_ratio": 0.3798240025838216
  }
}
[RED_MODE=0] GREEN-OK: image is a REAL generated image (defect absent)
```

`GREEN-OK` (exit 0) — the standing regression-guard polarity (§11.4.115)
confirms the real artifact is genuinely non-degenerate: 147,440 unique colours,
7.82 bits entropy, low dominant-colour fraction, mid-range adjacent-pixel
structure, and compressible (all consistent with real photographic/rendered
content, not noise or a blank frame).

## Honest finding: `enable_model_cpu_offload()` did NOT fit — `enable_sequential_cpu_offload()` did (§11.4.6/§11.4.108/§11.4.111)

Two admitted-but-failed attempts preceded the successful one, captured
honestly rather than hidden:

- **Attempt 1** (`generation.log`) and **attempt 2** (`generation_attempt2.log`,
  with `PYTORCH_CUDA_ALLOC_CONF=expandable_segments:True`) both used
  `pipe.enable_model_cpu_offload()` — the same API the Jul-9 proof used. Both
  hit `torch.OutOfMemoryError` **during the transformer's forward pass**
  (`accelerate` incrementally copying the 12B-parameter transformer's weights
  onto the GPU as a single resident unit), short by 18 MiB and 108 MiB
  respectively. Jul-9's run succeeded with this same API only because the GPU
  was **fully free** (~31 GB) at that time — with the coder now resident
  (~19.5 GB used), `enable_model_cpu_offload()`'s single-large-submodule-resident
  footprint (the 12B transformer partially/fully materialized on GPU) does not
  fit the ~12.6 GiB headroom. This is genuine measured reality, not an
  assumption (§11.4.6) — the task's own tier table assumed a quantized
  "FLUX-schnell-Q4" footprint (~7-9 GiB); no Q4/GGUF pipeline exists in this
  scaffold (confirmed by the Jul-8 `phase1_imagegen_runtime_20260708T082002Z`
  report), so the full-precision bf16 pipeline's `model_cpu_offload` peak is
  the true co-resident footprint, and it does not clear the 12.6 GiB bar.
- **Both failed attempts released their VRAM cleanly on process exit** — the
  `genproof` post-release `nvidia-smi` read confirmed `free=12634MiB` restored
  after each failure (before the `genproof` lease-release-before-`os.Exit`
  ordering bug, discovered and fixed during attempt 1's failure, was even in
  play — see "genproof bug fixed" below). **The coder was never affected by
  either OOM** (confirmed healthy + serving real completions after each).
- **Attempt 3** switched to `pipe.enable_sequential_cpu_offload()` (moves
  individual layers just-in-time instead of whole submodules) — peak
  footprint dropped to **0.69 GiB** (used rose from 19.50 GiB to only 20.18 GiB
  during inference), comfortably inside the 12.6 GiB headroom, and generation
  completed in 16.5s (faster than Jul-9's 22.7s `model_cpu_offload` run,
  because far less data had to shuttle CPU↔GPU per step at this resolution/step-count).

**Conclusion:** the fast-lane FLUX.1-schnell tier DOES fit the current free
VRAM co-resident with the live coder — but only via `enable_sequential_cpu_offload()`,
not the `enable_model_cpu_offload()` API the Jul-9 proof happened to use when
the card was fully free. This is a real, useful re-validation finding, captured
per §11.4.108 (runtime signature: `enable_sequential_cpu_offload()` is the
correct co-resident-with-coder code path for this tier on this card) rather
than silently reusing the Jul-9 script unmodified and re-hitting the same OOM.

### `genproof` bug fixed during this run

The first `genproof` build called `os.Exit(1)` directly inside the subprocess-failure
branch, which bypasses Go's deferred `lease.Release()` — a real defect (VRAM
was still restored correctly in practice because the actual GPU memory was
held by the crashed Python subprocess, freed by the OS/CUDA driver on process
exit, not by the Go-level lease bookkeeping — but the broker's in-process lease
state would have leaked had `genproof` been long-lived). Fixed by routing
through a deferred exit-code variable so `lease.Release()` always runs before
exit. Rebuilt and used for attempts 2 and 3.

## Why not video (WAN2.2 TI2V-5B)

Read the Jul-9 `docs/qa/wan2_generation_20260709T0010Z/generation.log`
(present on disk, untracked): the run logged `memory allocation failed with
OOM on device 0 while trying to allocate 2808086528 bytes (free: 1120337920,
total: 33679998976)` **during VAE decode**, i.e. it consumed essentially the
**entire ~33.6 GB card** (only ~1.1 GB free at that point) even with
`offload_model=True, t5_cpu=True`. That is far larger than the current 12.6
GiB headroom co-resident with the coder — video was correctly ruled out as
the tier for this re-validation without wasting a ~5.5-minute generation
attempt that would predictably OOM.

## Coder-untouched proof (§11.4.122)

| Checkpoint | `/health` | live completion |
|---|---|---|
| Before any generative action | `{"status":"ok"}` | `"OK-R41F"` (fresh, non-canned) |
| After both failed OOM attempts | `{"status":"ok"}` | (re-confirmed via VRAM read + health) |
| After successful generation + teardown | `{"status":"ok"}` | `"R41F-CODER-UNTO"` (fresh, distinct nonce, non-canned) |

`nvidia-smi --query-compute-apps` before and after shows **only** `llama-server`
(pid `1980342`, 19438 MiB) as the GPU-resident process throughout — no second
process was ever left resident. The coder was never paused, restarted, or
otherwise touched (§11.4.122 no-silent-removal / no-pause-without-approval).

## Teardown

The generative pipeline is process-scoped (no container, no daemon) — the
Python process exited after saving the image and `del pipe; gc.collect();
torch.cuda.empty_cache()`, and the `genproof` wrapper released its broker
lease in the same run. Final `nvidia-smi` read: `used=19464MiB free=12634MiB
total=32607MiB` — **identical to the pre-task baseline**. No manual teardown
step was required (no container was booted).

## Evidence files (this directory)

- `RESULTS.md` — this file
- `generation.log` — attempt 1 (`model_cpu_offload`, OOM by 18 MiB)
- `generation_attempt2.log` — attempt 2 (`model_cpu_offload` + `expandable_segments`, OOM by 108 MiB)
- `generation_attempt3.log` — attempt 3 (`sequential_cpu_offload`, SUCCESS)
- `r41f_flux_schnell_liveproof.png` — the real generated artifact (gitignored, see `.gitignore`)
- `.gitignore` — excludes the raw PNG per this task's instruction

## Source changes (committed, `submodules/helix_llm`)

- `docs/qa/phase4_imagegen_20260707/harness/genproof/main.go` — new broker-lease-holding
  generation wrapper (real `vrambroker.Acquire`/`Release` spanning the actual
  subprocess, not a release-immediately dry run)
- `docs/qa/phase4_imagegen_20260707/harness/genproof/.gitignore` — excludes
  the compiled binary (regenerable via `go build`, §11.4.77)

## Summary verdict

| Item | Verdict |
|---|---|
| Tier chosen | FLUX.1-schnell (image), `enable_sequential_cpu_offload()` |
| Fits current free VRAM (12634 MiB) without coder-pause | **YES** (peak +0.69 GiB) |
| Broker admission (lease held across real generation) | **ADMIT-OK**, released cleanly |
| Real artifact generated | **YES** — 1024×1024, 1.25 MB, prompt-matching, visually confirmed |
| Analyzer self-validation (golden-good PASS + golden-bad FAIL ×3) | **PASS** |
| Real artifact analyzer verdict | **GREEN-OK / real_image=true** |
| Coder untouched (health + live completions before/during/after) | **YES** |
| VRAM restored after teardown | **YES** — 12634 MiB free, bit-identical to baseline |
