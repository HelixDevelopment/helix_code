# HXC-108 — §11.4.107 Liveness / Freeze-Oracle Verification of Committed Durable Recordings

**Date:** 2026-06-23
**Scope:** Confirm the committed durable `docs/qa/HXC-108_*` mp4 recordings show GENUINELY ADVANCING frames
(not a frozen/stale single image), per Constitution §11.4.107 (liveness battery / freeze-detection oracle).
**Tooling:** `ffprobe` + `ffmpeg` (`/opt/homebrew/bin`, host).
**Read-only on recordings.** No commit performed.

---

## Method (§11.4.107 freeze oracle, multi-signal — §11.4.6 honest judgment)

A single captured frame is not proof (§11.4.107). For each mp4 we captured THREE independent signals
and combined them rather than trusting any one in isolation:

1. **Frame count + duration + fps** (`ffprobe -count_frames`) — a 1-frame / single-image video FAILs liveness.
2. **Distinct-frame count** via `mpdecimate` (drops near-duplicate frames; survivors = genuinely distinct
   content frames) — this is the perceptual freeze oracle. **1 distinct frame ⇒ frozen-stale.**
3. **First-frame vs last-frame distinctness** (frame-accurate PNG extract + md5) — first≠last ⇒ content
   genuinely changed across the window (advancing), not one stale image.
4. **`freezedetect=n=-60dB:d=0.5`** captured for reference. NOTE (§11.4.6 honest boundary): the default
   −60 dB threshold is extremely sensitive and flags a legitimately-static TUI/status screen as "frozen";
   freezedetect ALONE cannot distinguish *rendered-but-static real UI* from *frozen-stale-bluff*, so it is
   used only as a corroborating signal — the distinct-frame count + first≠last are the decisive oracles.

### Verdict vocabulary
- **live** — many distinct frames advancing across the window (genuinely-advancing).
- **static-real** — real genuine content, few changes (a status/list screen that renders then sits);
  ≥2 distinct frames AND first≠last ⇒ NOT frozen, acceptable per §11.4.107.
- **FROZEN-BLUFF** — exactly 1 distinct frame the whole duration / first==last ⇒ real defect, re-record.

---

## Per-mp4 results (§11.4.118 enumerated coverage — 12 of 12 committed mp4s checked)

| # | Recording | Frames | Dur (s) | fps | Distinct (mpdecimate) | first≠last | freezedetect | Verdict |
|---|-----------|-------:|--------:|----:|----------------------:|:----------:|:------------:|:-------:|
| 1 | HXC-108_android/helixcode-android-client.mp4 | 33 | 19.92 | ~1.7 avg | 25 (76%) | DIFFERENT | minimal | **live** |
| 2 | HXC-108_cli/helixcode-cli-command_exec-…mp4 | 33 | 3.30 | 10 | 4 (12%) | DIFFERENT | partial | **live** |
| 3 | HXC-108_cli/helixcode-cli-generate-…mp4 | 68 | 6.80 | 10 | 7 (10%) | DIFFERENT | partial | **live** |
| 4 | HXC-108_cli/helixcode-cli-version-…mp4 | 35 | 3.50 | 10 | 5 (14%) | DIFFERENT | partial | **live** |
| 5 | HXC-108_tui_server/helixcode-server-api-…mp4 | 48 | 4.80 | 10 | 5 (10%) | DIFFERENT | partial | **live** |
| 6 | HXC-108_tui_server/helixcode-tui-dashboard-…mp4 | 131 | 13.10 | 10 | 6 (5%) | DIFFERENT | periodic | **live** (updates @0,0.1,5.1,10.1s) |
| 7 | HXC-108_tui_views/helixcode-tui-projects-…mp4 | 50 | 5.00 | 10 | 3 (6%) | DIFFERENT | mostly static | **static-real** |
| 8 | HXC-108_tui_views/helixcode-tui-qa-…mp4 | 50 | 5.00 | 10 | 3 (6%) | DIFFERENT | mostly static | **static-real** |
| 9 | HXC-108_tui_views/helixcode-tui-sessions-…mp4 | 50 | 5.00 | 10 | 3 (6%) | DIFFERENT | mostly static | **static-real** |
| 10 | HXC-108_tui_views/helixcode-tui-tasks-…mp4 | 50 | 5.00 | 10 | 3 (6%) | DIFFERENT | mostly static | **static-real** |
| 11 | HXC-108_tui_views/helixcode-tui-workers-…mp4 | 50 | 5.00 | 10 | 3 (6%) | DIFFERENT | mostly static | **static-real** |
| 12 | HXC-108_web/helixcode-web-llmchat-…mp4 | 419 | 13.97 | 30 | 8 (2%) | DIFFERENT | streaming | **live** (updates ~every 2s @0,2,4,6,8,10s — LLM token stream) |

**iOS:** no committed mp4 — evidence is a single frame + `ffprobe` stamp (per the run notes). Honestly noted:
liveness oracle N/A (no video to analyze). Not counted in the 12; not a FROZEN-BLUFF (it never claimed a video).

---

## Interpretation (§11.4.6)

- **Zero FROZEN-BLUFF findings.** No recording is a single frozen/stale image. The decisive proof:
  **every one of the 12 has first-frame ≠ last-frame AND ≥3 distinct (mpdecimate-survivor) frames** — a
  frozen-stale bluff would show exactly 1 distinct frame and identical first/last.
- The **6 "live"** recordings (android, 3× cli, server-api, dashboard, web-llmchat) show clear content
  advancement: the Android client at 76% distinct frames is unambiguously live; the dashboard updates
  periodically (0/0.1/5.1/10.1s); the web LLM-chat advances every ~2s consistent with token streaming.
- The **5 TUI-view "static-real"** recordings (projects/qa/sessions/tasks/workers) legitimately render the
  view once then sit — 3 distinct frames (initial blank → rendered view → minor change) over a 5s window.
  This is *rendered-but-static real UI*, NOT a frozen black/stale frame: each has a genuine content
  transition (first≠last) and the OCR-content-verification (prior run) already confirmed the right content
  is on screen. Per §11.4.107 honest boundary, a deliberately-static real screen with a real distinct
  frame count is acceptable and MUST NOT be false-FAILed as frozen.

---

## Overall verdict

**PASS — liveness confirmed for all 12 committed durable mp4s.** 6 genuinely-advancing (live), 5 static-but-real
(real content, expected few changes for a TUI status/list screen), 1 (android) strongly live. **No
frozen-stale-bluff detected; no re-record required.** iOS honestly carries no committed mp4 (frame +
ffprobe-stamp only) — liveness oracle not applicable there.

This confirmation COMPLEMENTS (does not replace) the prior OCR content-verification and the §11.4.5 captured-evidence layer.
