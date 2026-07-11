# Video-Gen §11.4.107 Liveness-Analysis Closure — WAN2.2 TI2V-5B Real Clip

**Run-id:** `videogen_analysis_20260711T141601Z`
**Scope:** `submodules/helix_llm` (vidanalyzer) + `docs/qa` only. `submodules/helix_agent`,
`/mnt/track1`, `submodules/helix_qa`, `helix_code/internal/{security,tools/browser}`,
`scripts/opendesign` — untouched. No GPU used (CPU-only ffmpeg/ffprobe decode +
analysis). Coder daemon (`:18434`) untouched/irrelevant to this task.

## 1. Gap being closed

The 2026-07-09 WAN2.2 TI2V-5B generation (`docs/qa/wan2_generation_20260709T0010Z/`,
proof commit context `f2c71d07`) produced a real `.mp4` off-broker via a raw
generation script, but that clip was **never run through the §11.4.107 video
liveness analyzer** (`submodules/helix_llm/docs/qa/phase4_videogen_20260707/harness/vidanalyzer`).
This run closes exactly that analysis gap — it does **not** touch the
broker-integrated generation path (separate GPU/coder-pause-gated concern, VAE
OOM at ~32 GiB, out of scope here).

## 2. Existing clip located + ffprobe geometry (honest §11.4.6 discovery, not fabricated)

Located via `find docs/qa -iname '*video*' -o -iname '*wan*'` →
`docs/qa/wan2_generation_20260709T0010Z/helix_code_wan2_generation.mp4`
(11,172,956 bytes, alongside `generation.log` — a genuine WAN TI2V-5B run log,
50/50 diffusion steps, prompt `"A futuristic AI coding assistant interface with
glowing green matrix-style code rain..."`, base_seed=42, `frame_num=121`,
`sample_fps: 24`, saved 2026-07-09 00:34:51 UTC).

`ffprobe -show_format -show_streams`:

```
codec_name=h264, profile=High, pix_fmt=yuv420p
width=1280, height=704
r_frame_rate=24/1, avg_frame_rate=24/1
duration=5.041667 (s)
nb_frames=121
bit_rate=17725138
format=mov,mp4,m4a,3gp,3g2,mj2 (isom/avc1/mp41)
size=11172956 bytes
```

Geometry/FPS/frame-count are internally consistent (121 frames / 24 fps =
5.0417 s = the reported duration exactly) — real, decodable H.264 content,
not a truncated/corrupt artefact.

## 3. Analyzer build

```
$ cd submodules/helix_llm/docs/qa/phase4_videogen_20260707/harness/vidanalyzer
$ go build -o <scratchpad>/vidanalyzer .
```

Built clean, no errors, `go1.26.4` toolchain (module declares `go 1.24`, forward
compatible). Binary requires `ffmpeg`/`ffprobe` on `PATH` — both present
(`ffmpeg version 8.1.2-alt1`), so no `SKIP: ... not on PATH` short-circuit fired.

## 4. Self-validation (§11.4.107(10)) — proves the oracle is not itself a bluff gate

```
$ vidanalyzer selfvalidate
[GOLDEN-GOOD good (expect LIVE)] -> LIVE  frames=20 ptsMono=true adjDist=4.19 distinctFrac=1.000 entropy=4.92
    reason: all liveness signals satisfied (real live generated video)
[GOLDEN-BAD frozen (expect DEGENERATE)] -> DEGENERATE  frames=20 ptsMono=true adjDist=0.00 distinctFrac=0.000 entropy=4.72
    reason: mean adjacent-frame diff 0.000 < floor 1.5 (frozen / single-repeated-frame / static loop)
    reason: distinct adjacent-frame fraction 0.000 < floor 0.50 (too many identical frames = static loop)
[GOLDEN-BAD solid (expect DEGENERATE)] -> DEGENERATE  frames=20 ptsMono=true adjDist=0.00 distinctFrac=0.000 entropy=0.00
    reason: mean adjacent-frame diff 0.000 < floor 1.5 (frozen / single-repeated-frame / static loop)
    reason: distinct adjacent-frame fraction 0.000 < floor 0.50 (too many identical frames = static loop)
    reason: mean frame entropy 0.000 < floor 3.0 (solid/blank frames)
[SELF-VALIDATION] PASS: oracle classifies golden-good LIVE and all golden-bad DEGENERATE
Exit code: 0
```

**Golden-good (moving `testsrc2` pattern) → LIVE. PASS.**
**Golden-bad `frozen` (one real frame looped, the stale-frame trap named in
§11.4.107) → DEGENERATE. FAIL (as required).**
**Golden-bad `solid` (blank colour) → DEGENERATE. FAIL (as required).**

This is the §11.4.107(10) mutation-style proof that the analyzer itself cannot
rubber-stamp a bad clip: it correctly rejects both the "single repeated frame"
stale-frame trap and a blank/solid clip while accepting genuine motion.

## 5. Real-clip liveness analysis — the actual verdict (no fabrication)

```
$ RED_MODE=0 vidanalyzer analyze docs/qa/wan2_generation_20260709T0010Z/helix_code_wan2_generation.mp4
{
  "live_video": false,
  "reasons": [
    "mean adjacent-frame diff 1.098 < floor 1.5 (frozen / single-repeated-frame / static loop)"
  ],
  "metrics": {
    "frame_count": 121,
    "pts_monotonic": true,
    "mean_adj_luma_dist": 1.098228624131945,
    "distinct_adjacent_frac": 1,
    "mean_frame_entropy_bits": 6.788813709519886
  }
}
[RED_MODE=0] GREEN-FAIL: clip is a DEGENERATE video — generation did not produce a real live video
Exit code: 1
```

**Honest, as-shipped verdict: the current analyzer classifies this real clip as
DEGENERATE — but on a SINGLE narrow-margin signal, not a genuine freeze.**

Breaking down the five-signal AND (§11.4.107 "multi-signal AND — a single
metric is never proof"):

| Signal | Value | Floor/Ceiling | Verdict |
|---|---|---|---|
| `frame_count` | 121 | ≥ 8 | **PASS** (15× the floor) |
| `pts_monotonic` (independent frame-advance counter, demux-domain) | `true` | strictly increasing | **PASS** |
| `mean_adj_luma_dist` (freeze-detection oracle, 64×64 downscaled Δluma) | 1.098 | ≥ 1.5, ≤ 80.0 | **FAIL** — 73% of the floor, i.e. a near-miss, not zero |
| `distinct_adjacent_frac` (not-a-static-loop) | **1.000** | ≥ 0.50 | **PASS** — every single one of the 120 adjacent frame-pairs differs above the 0.75 per-pixel epsilon |
| `mean_frame_entropy_bits` (not-solid/blank) | 6.789 | ≥ 3.0 | **PASS** (2.3× the floor) |

4 of 5 signals pass comfortably; only `mean_adj_luma_dist` misses, and misses
narrowly (not zero — 1.098, not the ~0.00 seen on the golden-bad `frozen`/`solid`
fixtures above).

## 6. Independent cross-check (§11.4.107 "independent frame-advance counter …
different oracle domain") — is this clip actually frozen?

Rather than accept the single-signal DEGENERATE verdict as proof of an actual
freeze (§11.4.6 no-guessing — a threshold miss is not automatically "the video
is broken"), the clip was cross-checked with ffmpeg's own `freezedetect` filter
— an established, independent, full-resolution (not 64×64-downscaled) liveness
oracle, run at two different sensitivity thresholds:

```
$ ffmpeg -loglevel info -i <clip> -vf "freezedetect=n=-40dB:d=0.3" -an -f null -
  → NO freeze_start/freeze_duration/freeze_end events

$ ffmpeg -loglevel info -i <clip> -vf "freezedetect=n=-60dB:d=0.2" -an -f null -
  (more sensitive: -60dB noise floor, 0.2s min duration)
  → NO freeze_start/freeze_duration/freeze_end events
```

**Zero freeze windows detected across the full 5.04 s / 121-frame clip at either
threshold.** This is a different-domain oracle from the analyzer's own 64×64
Δluma metric (full native 1280×704 resolution, ffmpeg's mature/standard freeze
heuristic) and it independently confirms the clip contains **no** frozen or
stale-repeated-frame segment anywhere.

### RED_MODE=1 (polarity-switch, §11.4.115) cross-run

```
$ RED_MODE=1 vidanalyzer analyze <clip>
... same metrics ...
[RED_MODE=1] RED-OK: clip is a DEGENERATE video (defect reproduced — not a real live generated video)
Exit code: 0
```

At `RED_MODE=1` (the polarity that expects a degenerate/no-real-service clip)
the tool reports RED-OK because its own DEGENERATE classification matches the
expected pre-fix state — this is the polarity switch behaving exactly as
designed (§11.4.115), not a second independent confirmation of an actual
freeze.

## 7. Honest interpretation — finding, not a fix

**Finding:** the vidanalyzer's `advanceDiffFloor = 1.5` threshold was
calibrated (per the tool's own doc-comment, `main.go:66-68`) against
deterministic synthetic `ffmpeg testsrc2` fixtures — a high-contrast,
large-block moving test pattern. Real WAN2.2 TI2V-5B output (smoother
cinematic AI-generated content: "glowing... code rain," "holographic data
panels," "neon... particles") produces genuinely continuous motion
(`distinct_adjacent_frac = 1.000`, i.e. every adjacent frame pair differs) but
with a smaller mean per-pixel luma delta after the 64×64 analysis downscale
than the synthetic fixture. The tool's own comment names this exact moment:
*"A real generated WAN/LTX MP4 at runtime-proof time re-calibrates these
against captured outputs."* This is that runtime-proof moment.

**This analysis-only task does NOT recalibrate `advanceDiffFloor`** —
threshold changes to the shared anti-bluff harness affect every future
verdict it produces and are a separate, tracked decision (recalibration
evidence, updated self-validation fixtures, §11.4.107(13) calibration-on-own-
fixtures compliance, code review). Recalibrating unilaterally inside a
narrowly-scoped analysis PWU would itself be a scope violation. This is
flagged here as a follow-up item, not performed.

**Bottom line, stated without hedging:**
- **As-shipped analyzer verdict on this real clip: DEGENERATE** (fails 1 of 5
  liveness signals, a near-miss on `mean_adj_luma_dist`).
- **Independent freeze-detection cross-check: the clip is NOT actually frozen**
  (zero freeze windows at two sensitivity thresholds; `pts_monotonic=true`;
  `distinct_adjacent_frac=1.000`; entropy well above the blank/solid floor).
- The gap that motivated this task — "a real generated `.mp4` was never run
  through the §11.4.107 analyzer" — is **closed**: the clip has now been
  analyzed, the analyzer's own self-validation is proven anti-bluff (golden-good
  PASS, golden-bad FAIL), and the resulting DEGENERATE-by-narrow-margin verdict
  is reported honestly, together with the independent evidence that
  contextualises it, rather than being silently discarded or overridden.

## 8. §1.1 mutation note

The analyzer's own self-validation (§5 above) already IS the paired-mutation
proof this run required: the `frozen` golden-bad fixture (a single real frame
looped — the exact stale-frame trap named in §11.4.107) is correctly rejected
(DEGENERATE), and the `solid` golden-bad fixture is correctly rejected. A
mutation that weakened `mean_adj_luma_dist < advanceDiffFloor` to always pass
would make `selfvalidate` FAIL on the `frozen` fixture — the pairing already
exists in the shipped `cmdSelfValidate()` implementation; no new mutation
harness was authored in this PWU (out of scope: no source change to
`vidanalyzer` was made or needed to close the analysis gap).

## 9. Evidence inventory

| Artefact | Location |
|---|---|
| Real WAN2.2 clip (git-ignored per §11.4.30, NOT committed here) | `docs/qa/wan2_generation_20260709T0010Z/helix_code_wan2_generation.mp4` |
| Generation log (already committed, pre-existing) | `docs/qa/wan2_generation_20260709T0010Z/generation.log` |
| This report | `docs/qa/videogen_analysis_20260711T141601Z/RESULTS.md` |
| vidanalyzer source (pre-existing, unmodified) | `submodules/helix_llm/docs/qa/phase4_videogen_20260707/harness/vidanalyzer/main.go` |

## 10. Constitution composition

§11.4.107 (AV liveness battery, multi-signal AND, independent freeze-detection
oracle, self-validated golden-good/golden-bad), §11.4.107(10) (analyzer
self-validation), §11.4.6 (no-guessing — cross-checked with an independent
oracle rather than accepting a single-signal miss as proof of freeze),
§11.4.102 (root-cause investigation before any fix — no fix was applied;
recalibration correctly identified and deferred as a separate tracked item),
§11.4.30 (raw `.mp4` git-ignored, not committed), §11.4.69 (`video_display`
sink-side evidence class — this report + the ffprobe/analyzer/freezedetect
transcripts above ARE the captured evidence).
