# §11.4.107(13) Analyzer-Calibration Closure — vidanalyzer `mean_adj_luma_dist` Floor Recalibrated on Real WAN2.2 Fixture

**Run-id:** `videogen_recalibration_20260711_144236Z`
**Scope:** `submodules/helix_llm/docs/qa/phase4_videogen_20260707/harness/vidanalyzer/main.go`
(the vidanalyzer source) + `docs/qa` ONLY. `submodules/helix_agent`, `/mnt/track1`,
`helix_qa`, `helix_code/internal/*`, `scripts/`, `services/vectorize/`, any
`rag-qdrant` dir — untouched. Coder daemon (`:18434`) untouched — this task used
CPU-only `ffmpeg`/`ffprobe`/Go, no GPU, no LLM calls.

## 1. Defect closed (from the prior stream's finding)

`docs/qa/videogen_analysis_20260711T141601Z/RESULTS.md` (previous PWU, analysis-only,
explicitly deferred recalibration as "a separate, tracked decision") documented: the
real WAN2.2 TI2V-5B clip `docs/qa/wan2_generation_20260709T0010Z/helix_code_wan2_generation.mp4`
(1280x704, 24fps, 121 frames) — independently confirmed genuinely LIVE via ffmpeg
`freezedetect` (zero freeze windows at two sensitivity thresholds) and via
`distinct_adjacent_frac=1.000` (literally every one of its 120 adjacent frame-pairs
differs above epsilon) — was FALSE-FLAGGED `DEGENERATE` by the vidanalyzer's
`mean_adj_luma_dist` signal alone (1.098 < the then-floor of 1.5). The floor was
calibrated only on the harness's own synthetic `ffmpeg testsrc2` fixture (a
high-contrast, large-block moving test pattern scoring 4.19), which is NOT
representative of smoother real cinematic AI-generated content. This is exactly the
§11.4.107(13) violation this run closes: *"thresholds MUST be calibrated on the
project's OWN real fixtures, not literature/synthetic values."*

## 2. Source located

`submodules/helix_llm/docs/qa/phase4_videogen_20260707/harness/vidanalyzer/main.go`
— the single-file Go CLI that computes `mean_adj_luma_dist` (freeze-detection oracle,
mean per-pixel |Δluma| between temporally-adjacent frames at a 64×64 downscale) and
the multi-signal-AND liveness verdict. Constant block at `main.go:70-78` (pre-change)
declared `advanceDiffFloor = 1.5` with a doc-comment explicitly anticipating this
exact recalibration moment: *"A real generated WAN/LTX MP4 at runtime-proof time
re-calibrates these against captured outputs."*

## 3. Real-fixture measurements (full float64 precision, `analyze` JSON output)

Built with `go build .` (`go1.26.4`, module declares `go 1.24`, forward-compatible;
clean build, zero errors). `ffmpeg`/`ffprobe` present on `PATH` (`ffmpeg 8.1.2-alt1`),
so `requireFFmpeg()`'s honest-SKIP short-circuit never fired — every result below is
a real ffmpeg decode + analysis, not a skip.

| Fixture | `mean_adj_luma_dist` | `distinct_adjacent_frac` | `mean_frame_entropy_bits` | Source |
|---|---|---|---|---|
| WAN2.2 real clip (LIVE reference) | **1.098228624131945** | 1 | 6.788813709519886 | real generated `.mp4`, 121 frames |
| golden-bad `frozen` (DEGENERATE reference) | **0** (exact) | 0 | 4.721171054373908 | one real `testsrc2` frame looped 20× |
| golden-bad `solid` (DEGENERATE reference) | **0** (exact) | 0 | 0 | blank-colour `.mp4`, 20 frames |
| golden-good `good` (LIVE reference, synthetic) | 4.193350808662283 | 1 | 4.919392884112303 | `testsrc2` moving pattern, 20 frames |

Both golden-bad fixtures measure `mean_adj_luma_dist` at EXACTLY `0` — ffmpeg's
deterministic re-encode of a byte-identical looped source frame produces
byte-identical decoded frames, zero decode/compression noise. This gives an
effectively unbounded separation margin between "genuinely frozen" (0.000000...)
and "genuinely live" (1.098228...) on this metric alone.

## 4. Recalibration performed

**Old floor: `advanceDiffFloor = 1.5`** (calibrated on synthetic `testsrc2` only).
**New floor: `advanceDiffFloor = 0.55`** (calibrated on the real WAN clip + both
golden-bad fixtures).

Rationale (documented inline in `main.go` at the constant block, `§11.4.107(13)
RECALIBRATION` comment): `0.55` is ~50% of the single captured real-live sample
(1.098228624131945 / 2 ≈ 0.549114...), giving:

- **~2x safety margin BELOW the known-live WAN measurement** — 1.098 vs floor 0.55
  is a 0.548 absolute / ~100% relative margin (the WAN clip clears the new floor by
  roughly double).
- **An effectively unbounded margin ABOVE the golden-bad fixtures** — 0.55 vs the
  golden-bads' exact `0` is not "close" by any measure; the golden-bads are not near
  the new floor in any statistical sense.

`distinctFracLow` (0.50) and `entropyFloor` (3.0) were **NOT changed** — both already
comfortably separated the WAN clip (`distinctFrac=1.000`, `entropy=6.789`) from the
golden-bads (`distinctFrac=0.000`, `entropy=0.000` for `solid`) with wide margin; the
false-positive was isolated entirely to the single `mean_adj_luma_dist` signal, so no
composite/multi-signal-corroboration rule was needed — the metric genuinely CAN
separate real-live from frozen/solid cleanly once calibrated on real, not only
synthetic, data. (See §7 "Honest gap" below — this is an n=1 real-fixture
calibration, not a distribution fit.)

## 5. Post-recalibration verdicts — all four required checks, full evidence

### 5a. `selfvalidate` (§11.4.107(10)) — run twice for determinism

```
$ vidanalyzer selfvalidate            (run 1)
[GOLDEN-GOOD good (expect LIVE)] -> LIVE  frames=20 ptsMono=true adjDist=4.19 distinctFrac=1.000 entropy=4.92
    reason: all liveness signals satisfied (real live generated video)
[GOLDEN-BAD frozen (expect DEGENERATE)] -> DEGENERATE  frames=20 ptsMono=true adjDist=0.00 distinctFrac=0.000 entropy=4.72
    reason: mean adjacent-frame diff 0.000 < floor 0.6 (frozen / single-repeated-frame / static loop)
    reason: distinct adjacent-frame fraction 0.000 < floor 0.50 (too many identical frames = static loop)
[GOLDEN-BAD solid (expect DEGENERATE)] -> DEGENERATE  frames=20 ptsMono=true adjDist=0.00 distinctFrac=0.000 entropy=0.00
    reason: mean adjacent-frame diff 0.000 < floor 0.6 (frozen / single-repeated-frame / static loop)
    reason: distinct adjacent-frame fraction 0.000 < floor 0.50 (too many identical frames = static loop)
    reason: mean frame entropy 0.000 < floor 3.0 (solid/blank frames)
[SELF-VALIDATION] PASS: oracle classifies golden-good LIVE and all golden-bad DEGENERATE
Exit code: 0

$ vidanalyzer selfvalidate            (run 2 — determinism)
... IDENTICAL output to run 1, byte-for-byte (frames/ptsMono/adjDist/distinctFrac/entropy all match) ...
[SELF-VALIDATION] PASS: oracle classifies golden-good LIVE and all golden-bad DEGENERATE
Exit code: 0
```

(Printed floor rounds to `0.6` at the 1-decimal `%.1f` display format used by the
verdict-reason string; the underlying constant and comparison use the full
`0.55` value — confirmed via the full-precision JSON below.)

**Golden-good → LIVE. Golden-bad `frozen` → DEGENERATE. Golden-bad `solid` →
DEGENERATE. Deterministic across 2 runs.**

### 5b. WAN2.2 real clip — `RED_MODE=0` (standing regression-guard, post-fix expectation), run twice

```
$ RED_MODE=0 vidanalyzer analyze docs/qa/wan2_generation_20260709T0010Z/helix_code_wan2_generation.mp4   (run 1)
{
  "live_video": true,
  "reasons": [
    "all liveness signals satisfied (real live generated video)"
  ],
  "metrics": {
    "frame_count": 121,
    "pts_monotonic": true,
    "mean_adj_luma_dist": 1.098228624131945,
    "distinct_adjacent_frac": 1,
    "mean_frame_entropy_bits": 6.788813709519886
  }
}
[RED_MODE=0] GREEN-OK: clip is a REAL live generated video (defect absent)
Exit code: 0

$ RED_MODE=0 vidanalyzer analyze <same clip>   (run 2 — determinism)
... IDENTICAL JSON, byte-for-byte ...
[RED_MODE=0] GREEN-OK: clip is a REAL live generated video (defect absent)
Exit code: 0
```

**The WAN clip now correctly classifies LIVE — no simulated/frozen/degenerate
result, this is the real ffmpeg decode + real-clip analysis of the real WAN2.2
generation, deterministic across 2 runs.**

For completeness, `RED_MODE=1` (the reproduce-the-defect polarity) on the SAME clip
now correctly reports `RED-VIOLATION` (exit 1) — because the defect (false-DEGENERATE
misclassification) is genuinely absent post-fix, which is the polarity switch
(§11.4.115) behaving exactly as designed:

```
$ RED_MODE=1 vidanalyzer analyze <same clip>
{ ... same metrics, live_video: true ... }
[RED_MODE=1] RED-VIOLATION: clip is a REAL live generated video but a defect was expected (a real service must exist)
Exit code: 1
```

### 5c. All four required verdicts — summary table

| Fixture | Verdict required | Verdict measured | Match? |
|---|---|---|---|
| WAN2.2 real clip | LIVE | **LIVE** (`live_video: true`) | ✅ |
| golden-bad `frozen` | DEGENERATE | **DEGENERATE** | ✅ |
| golden-bad `solid` | DEGENERATE | **DEGENERATE** | ✅ |
| golden-good `good` (moving) | LIVE | **LIVE** | ✅ |

**Separation margin (measured, `mean_adj_luma_dist`):**
- WAN clip (1.098228624131945) vs new floor (0.55): **+0.548228624... absolute,
  +99.7% relative** — WAN clears the floor by essentially double.
- New floor (0.55) vs golden-bad frozen/solid (0 exact): **the golden-bads are at
  the metric's absolute floor of 0 — not near 0.55 by any measure.**

## 6. §1.1 paired mutation — proves the guard is load-bearing, not weakened to pass-all

Per §1.1 / §11.4.107(10): mutate → assert FAIL → restore → assert PASS. Performed on
an isolated scratchpad copy of the vidanalyzer source (never touched the tracked
`main.go` — `git status --porcelain` in the submodule confirms only the intended
`main.go` recalibration diff, zero mutation residue, per §11.4.84 quiescence).

**Mutation:** `advanceDiffFloor` and `distinctFracLow` both set to `-1000.0` in the
isolated scratch copy only (annotated inline as a temporary, paired-§1.1-test
change on that disposable copy — never staged, never part of the tracked diff) —
simulates the exact bug class this self-validation harness exists to catch: a
weakening of the freeze-detection guard so it accepts everything unconditionally.

```
$ vidanalyzer_MUTATED selfvalidate
[GOLDEN-GOOD good (expect LIVE)] -> LIVE  frames=20 ptsMono=true adjDist=4.19 distinctFrac=1.000 entropy=4.92
    reason: all liveness signals satisfied (real live generated video)
[GOLDEN-BAD frozen (expect DEGENERATE)] -> LIVE  frames=20 ptsMono=true adjDist=0.00 distinctFrac=0.000 entropy=4.72
    reason: all liveness signals satisfied (real live generated video)
    SELF-VALIDATION VIOLATION: golden-bad "frozen" PASSed as a live video (bluff gate)
[GOLDEN-BAD solid (expect DEGENERATE)] -> DEGENERATE  frames=20 ptsMono=true adjDist=0.00 distinctFrac=0.000 entropy=0.00
    reason: mean frame entropy 0.000 < floor 3.0 (solid/blank frames)
[SELF-VALIDATION] FAIL
Exit code: 1
```

**With the freeze-detection floor + distinct-fraction floor disabled, the `frozen`
golden-bad fixture is WRONGLY classified LIVE — `selfvalidate` correctly detects
this as a bluff gate and FAILs (exit 1).** This proves both guards are genuinely
load-bearing: a naive "weaken the threshold until the real clip passes" mutation
does NOT silently succeed — it is caught by the self-validation harness itself.
(Note: `solid` is STILL correctly caught even with both freeze signals disabled,
via the independent, untouched `entropyFloor` signal — defense-in-depth confirmed:
disabling one signal class does not blind the other classes.)

**Restore:**

```
$ vidanalyzer selfvalidate   (the real, recalibrated, non-mutated binary)
[GOLDEN-GOOD good (expect LIVE)] -> LIVE  ...
[GOLDEN-BAD frozen (expect DEGENERATE)] -> DEGENERATE  ...
[GOLDEN-BAD solid (expect DEGENERATE)] -> DEGENERATE  ...
[SELF-VALIDATION] PASS: oracle classifies golden-good LIVE and all golden-bad DEGENERATE
Exit code: 0

$ grep -n "MUTATED\|always pass" submodules/helix_llm/.../vidanalyzer/main.go
clean — no mutation markers in tracked source
```

Mutate → FAIL → restore → PASS, confirmed. The mutation scratch directory and
mutated binary were deleted after the test (never staged, never committed).

## 7. Honest gap (§11.4.6 / §11.4.107(13))

This is an **n=1 real-fixture calibration** — exactly one real WAN2.2 generation was
available to calibrate against. The new floor (0.55) is NOT a statistical
distribution fit; it is a principled ~50%-of-observed-value safety margin chosen
because (a) the two golden-bad fixtures measure EXACTLY 0 with zero decode noise
(giving effectively unbounded headroom on the low side), and (b) 0.55 leaves the
single known-live sample (1.098) a comfortable ~2x margin above the floor. As more
real WAN2.2/LTX generations are captured (across different prompts, motion levels,
scene types — e.g. a calmer/mostly-static real generation could legitimately measure
lower than 1.098), this floor SHOULD be revisited/tightened using a proper
distribution rather than a single sample. This is flagged here explicitly, not
silently glossed over. No composite/multi-signal-corroboration rule was needed for
THIS defect — the single `mean_adj_luma_dist` metric cleanly separates the one real
sample from both golden-bads once recalibrated off synthetic-only data — but a
composite rule (requiring the freeze verdict to be corroborated by `distinctFrac`
also being low, per the prior stream's suggestion) remains a reasonable future
hardening if/when a real sample is captured that measures uncomfortably close to
0.55.

## 8. Evidence inventory

| Artefact | Location |
|---|---|
| Recalibrated analyzer source (committed) | `submodules/helix_llm/docs/qa/phase4_videogen_20260707/harness/vidanalyzer/main.go` |
| This report | `docs/qa/videogen_recalibration_20260711_144236Z/RESULTS.md` |
| Real WAN2.2 clip (git-ignored per §11.4.30, NOT committed, pre-existing) | `docs/qa/wan2_generation_20260709T0010Z/helix_code_wan2_generation.mp4` |
| Prior analysis-only run (the defect this run closes) | `docs/qa/videogen_analysis_20260711T141601Z/RESULTS.md` |

## 9. Constitution composition

§11.4.107(13) (thresholds calibrated on the project's OWN real fixtures, not
literature/synthetic values — the mandate this run directly satisfies), §11.4.107
(multi-signal-AND liveness battery, self-validated golden-good/golden-bad),
§11.4.107(10) (analyzer self-validation — proven anti-bluff via the §1.1 mutation in
§6 above), §1.1 (paired mutation: mutate → FAIL → restore → PASS), §11.4.6 (no
guessing — recalibration grounded in measured real-fixture data, honest n=1 gap
disclosed in §7, never a silent "tune until it passes"), §11.4.84 (working-tree
quiescence — `git status --porcelain` confirmed zero mutation residue and zero
unaccounted files before commit), §11.4.115 (RED_MODE polarity switch — verified
both polarities post-fix), §11.4.69 (`video_display` sink-side captured evidence —
this report + the full-precision JSON/selfvalidate transcripts above ARE the
captured evidence), §11.4.30 (raw `.mp4` git-ignored, not committed).
