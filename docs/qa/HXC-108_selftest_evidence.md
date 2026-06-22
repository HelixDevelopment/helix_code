# HXC-108 — Video-QA Harness Self-Test Evidence

**Harness:** `scripts/video_qa/record_feature.sh selftest`
**Captured:** 2026-06-22 (UTC), dev host macOS 15.5
**Revision:** 1
**Discipline:** §11.4.6 real evidence · §11.4.3 honest env-gap · §11.4.107(10) self-validated analyzer

## What the self-test proves

The self-test proves the harness mechanics on a NON-GUI target (no HXC-112 contention),
per the HXC-108 brief: record a deterministic on-screen string, then prove the OCR analyzer
READS that known string back (golden-good PASS) AND FAILS on a wrong expected-pattern
(golden-bad FAIL). Scope-prefix: `helixcode-harness_selftest-*`.

## Result summary

| Check | Verdict | Evidence |
|-------|---------|----------|
| Project-prefix resolution | PASS | `resolve-prefix` → `helixcode` (from `.env` `HELIX_RELEASE_PREFIX`) |
| Analyzer golden-good (expect real string) | **PASS (rc=0)** | OCR read `HELIXCODE_OCR_PROBE_4242`, all expected patterns present |
| Analyzer golden-bad (expect absent string) | **FAIL (rc=1)** | OCR read real string; `HELIXCODE_WRONG_PATTERN_9999` correctly NOT matched |
| Analyzer bluff-catch (string present + `TODO implement`) | **FAIL (rc=1)** | anti-bluff pattern `todo implement` detected → FAIL even though expected pattern present |
| Analyzer self-validation overall | **PASS** | golden-good rc=0 AND golden-bad rc=1 → analyzer provably cannot bluff |
| Live window-scoped recording | **SKIP (§11.4.3 env-gap)** | both window capture primitives TCC-blocked on host; harness refuses whole-desktop fallback |

Self-test exit code: **0** (`PARTIAL-PASS`: analyzer self-validation PASS; live recording an honest env-gap).

## Captured run — analyzer self-validation (the load-bearing anti-bluff proof)

```
[20:18:39] ANALYZER SELF-VALIDATION: PASS (golden-good PASS rc=0, golden-bad FAIL rc=1) — analyzer cannot bluff
```

## Captured run — standalone golden-good / golden-bad / bluff-catch

```
--- render a known string into an image (simulating an on-screen frame) ---
frame: PNG image data, 1100 x 220, 8-bit grayscale, non-interlaced

--- GOLDEN-GOOD: ocr-analyze expects the real string -> MUST PASS ---
ocr-analyze: image=frame.png extracted_chars=24
OCR-VERDICT: PASS (all expected patterns present, no bluff): HELIXCODE_OCR_PROBE_4242
  golden-good exit: 0 (0=PASS expected)

--- GOLDEN-BAD: ocr-analyze expects a string NOT on screen -> MUST FAIL ---
ocr-analyze: image=frame.png extracted_chars=24
OCR-VERDICT: FAIL (missing expected pattern(s): HELIXCODE_WRONG_PATTERN_9999)
  golden-bad exit: 1 (1=FAIL expected)

--- BLUFF-CATCH: a frame with 'TODO implement' -> MUST FAIL even if expected pattern present ---
ocr-analyze: image=bluff.png extracted_chars=39
OCR-VERDICT: FAIL (anti-bluff pattern present: "todo implement")
  bluff-catch exit: 1 (1=FAIL expected)
```

## Captured run — live recording honest SKIP (§11.4.3, never faked)

```
[live] launching Terminal window (marker=HELIXCODE_WIN_...) running: echo HELIXCODE_OCR_PROBE_4242; sleep 5
[live] discover: owner+title miss (titles likely hidden by privacy grant); owner-only fallback for 'Terminal'
[live] discovered window id=27097; capturing window-scoped for 6s
[live] native window video unavailable (screencapture: capture error The operation could not be completed); trying window-scoped STILL timelapse fallback
[live] SKIP (honest env-gap §11.4.3): record: window-scoped capture blocked by host
       (screencapture -v video AND -l<id> still both failed: 'could not create image from window').
       Reproduce: screencapture -v -V2 -l27097 /tmp/x.mov ; screencapture -o -l27097 /tmp/x.png .
       Refusing whole-desktop fallback.
```

## Host capability probe (re-runnable)

`scripts/video_qa/record_feature.sh probe-caps`:

```
host capture capabilities (probed 2026-06-22T20:16:19Z):
  [OK ] screencapture -R region still
  [GAP] screencapture -l<id> window still (id=27097) — window-scoped recording requires this OR window video
  [GAP] screencapture -v window video (macOS Screen-Recording video TCC grant)
  [OK ] OCR (magick render + tesseract read) under TMPDIR=/Volumes/T7/tmp
```

## Root-cause findings (systematic-debugging §11.4.102, fixed during build)

1. **Selftest aborted before analyzer self-validation** — `record_feature` uses `skip()`/`fail()` which
   `exit`; a function-level `exit` killed the whole script before the analyzer half. **Fix:** run the
   live-recording attempt in an isolated subshell `( ... )` so its SKIP is contained.
2. **OCR silently read empty under `/tmp`** — this host's Homebrew tesseract/leptonica 1.87.0 is DENIED
   read access to `/tmp` (=`/private/tmp`) image files ("findFileFormat: image file not found" on a file
   that exists) but reads files under `$TMPDIR` (`/Volumes/T7/tmp`) and `$HOME` fine. **Fix:** all scratch
   files moved to a `$TMPDIR`-rooted work dir; never `/tmp`. (This was a hidden FALSE-NEGATIVE bluff
   surface — OCR would have reported "no content" on perfectly valid frames.)
3. **`magick -font Courier` failed** — the `Courier` font alias is unresolvable on this host. **Fix:**
   render with a real font file (`/System/Library/Fonts/Menlo.ttc`), default-font fallback otherwise.
4. **Window-title discovery missed** — macOS returns EMPTY window titles to a CLI process without the
   privacy grant. **Fix:** owner+title first, then documented owner-only fallback (still window-scoped — a
   single window id — never whole-desktop).
5. **Window capture itself TCC-blocked** — both `screencapture -v` (video) and `-l<id>` (window still)
   fail on this host. **Outcome:** honest SKIP with reproducer; never a whole-desktop fallback (§11.4.154).

## Sources verified

- Self-test + standalone analyzer runs captured live this session — 2026-06-22.
