# Phase 1 — Fast-lane image-gen runtime proof — BLOCKED on missing prerequisites (honest, not VRAM)

Task asked for a download of a fast-lane image model that fits the co-residence
ceiling: "SDXL-Turbo or FLUX.1-schnell GGUF-Q4 / SD.cpp-compatible (~7-9 GiB)".
Investigation of the **actual existing scaffold** at
`submodules/helix_llm/{cmd/imagegen-boot,services/imagegen,docs/qa/phase4_imagegen_20260707}`
(commit `0f07559` lineage, confirmed present at HEAD `c92fb16`) found a real
mismatch with that description — reported honestly rather than worked around
by fabricating a different pipeline under time pressure (§11.4.6/§11.4.145).

## What the scaffold actually is (verified by reading the real code, not assumed)

- `services/imagegen/imagegen_server.py`: a FastAPI shim around a REAL
  **diffusers `FluxPipeline` + Nunchaku `NunchakuFluxTransformer2dModel`**
  (NVFP4 SVDQuant) pipeline for **`black-forest-labs/FLUX.1-dev`** — a
  **gated** HuggingFace repo requiring license acceptance + `HF_TOKEN`.
- It is NOT a GGUF / llama.cpp-family (SD.cpp) server. There is no code path
  in this scaffold that loads a `.gguf` diffusion checkpoint. Substituting
  SDXL-Turbo or FLUX.1-schnell GGUF would require writing a NEW server
  backend (a different Python dependency stack, a different container image,
  a different `/v1/images/generations` implementation) — that is new
  engineering work, not "download a model and run the existing harness," and
  is out of scope for a download-and-prove task without the independent
  review + impact-research gauntlet (§11.4.125/§11.4.142/§11.4.145) a new
  backend would require.
- Building/running the EXISTING scaffold as designed requires:
  1. `HF_TOKEN` (HuggingFace access token with FLUX.1-dev license accepted) —
     **verified UNSET** in both the shell environment and `submodules/helix_llm/.env`
     (grep for `^HF_TOKEN=` in both returned nothing).
  2. `NUNCHAKU_WHEEL` (a custom pip wheel for the Nunchaku NVFP4 quantization
     library, not on PyPI as a stable release for this stack) — **verified
     UNSET / not present**.
  3. Building the CUDA 12.8 + torch-cu128 + Nunchaku container image
     (`services/imagegen/Containerfile`) — a heavy, likely multi-GB, multi-
     step build not attempted this session given (1) and (2) already block
     the pipeline from running even if the image built.

Neither prerequisite is something this agent can supply autonomously
(§11.4.10 — credentials/tokens are operator-provisioned, never invented or
guessed; §11.4.6 — no fabricated workaround).

## What WAS proven for real this session (no fabrication)

### 1. Broker VRAM admission — REAL, live, against the running coder

```
cd submodules/helix_llm/docs/qa/phase4_imagegen_20260707/harness
./run_proof.sh admit-check
```
Output (real `nvidia-smi` read through the vrambroker, coder untouched):
```
VRAM budget (nvidia-smi): total=32607MiB used=19444MiB free=12677MiB need=7168MiB headroom=2048MiB
ADMIT-OK: NVFP4 footprint admitted co-resident (coder stays live) — fast path
```
This confirms the master-plan §6.8 co-residence math holds RIGHT NOW
(need=7 GiB + 2 GiB headroom = 9 GiB ≤ 12.677 GiB free) — i.e. **if** the
FLUX.1-dev+Nunchaku pipeline could be booted, the broker would admit it
alongside the live coder without a pause. The admission check does not boot
any container and does not touch the coder (confirmed by construction:
`admit-check` only calls `vrambroker.Acquire`+immediate `Release`, no
`compose up`).

### 2. Harness compiles — REAL

```
go build -o /dev/null ./cmd/imagegen-boot   -> exit 0 (BUILD OK)
```

### 3. Model download — NOT ATTEMPTED (honest, reasoned)

No SDXL-Turbo / FLUX.1-schnell GGUF download was started, because:
- The existing, already-built scaffold (`imagegen_server.py`,
  `cmd/imagegen-boot`, `compose.imagegen.yml`) has NO code path that would
  consume such a file — downloading it would produce an unused ~7-9 GiB
  artifact and zero runtime proof, which would itself be a bluff (spending
  hours of real download bandwidth for a file nothing can load is not
  "keeping honest progress," it is busywork dressed as progress).
- Running the scaffold AS DESIGNED (FLUX.1-dev) is blocked on missing
  `HF_TOKEN` + `NUNCHAKU_WHEEL`, not on VRAM or download bandwidth — a
  different blocker class than Part A.

## Verdict: BLOCKED-on-prerequisites (NOT VRAM, NOT download bandwidth)

- VRAM ceiling check: **PASS** (ADMIT-OK, real broker evidence above) — this
  is NOT the blocker.
- Missing `HF_TOKEN` (gated FLUX.1-dev license acceptance): **BLOCKING**.
- Missing `NUNCHAKU_WHEEL`: **BLOCKING**.
- Container build (CUDA 12.8 + Nunchaku): not attempted (blocked upstream by
  the two items above; would also need to be verified against the ~10.39 GiB
  co-reside ceiling once a real needBytes is measured — currently only a
  ~7 GiB placeholder per the scaffold's own README).
- No image was generated. No CLIPScore analyzer run was performed against a
  real generation (the analyzer's own golden-good/golden-bad self-validation
  is documented as already real/independent of a live model in the
  scaffold's README — that self-validation is unaffected by this blocker but
  was NOT re-run this session since it produces no NEW evidence relevant to
  this task's ask, which is a real generation + verdict).

## Coder untouched — confirmed (§11.4.122)

```
curl :18434/health -> {"status":"ok"}
nvidia-smi: 32607 MiB total, 19444 MiB used, 12677 MiB free   <- unchanged throughout
```
No `compose up` was ever invoked (admit-check only), so no container of any
kind was booted this session for Part B.

## What's needed to unblock

1. Operator provisions `HF_TOKEN` (with FLUX.1-dev license accepted on
   huggingface.co) and `NUNCHAKU_WHEEL` (built/obtained per
   `services/imagegen/Containerfile` + `.gitignore-meta/flux_nunchaku_nvfp4.yaml`)
   into `submodules/helix_llm/.env` (mode 0600, gitignored, §11.4.10).
2. Re-run `./run_proof.sh admit-check` (fast, safe) then `./run_proof.sh boot`
   (builds/boots the real container — first real GPU test of this pipeline)
   then `./run_proof.sh generate "<prompt>"` for the real image + analyzer
   verdict, then `./run_proof.sh down`.
3. Alternatively, if the operator prefers the GGUF/SD.cpp fast-lane path
   described in this task over the existing FLUX.1-dev+Nunchaku scaffold,
   that is a NEW backend and should be scoped as its own feature work-stream
   (§11.4.167) with its own design + review, not folded into this
   download-and-prove task.
