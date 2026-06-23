# HXC-108 — TUI per-view recording evidence (§11.4.158)

**Date:** 2026-06-23 · **Client:** terminal-UI (tview/tcell) · **Mode:** fully automatic, no TCC/Aqua/human

## Harness
`scripts/video_qa/record_tui_views.sh` — launches the TUI binary in a detached
tmux session (fixed 200×50), drives view navigation via `tmux send-keys`
(programmatic key injection — no human, no TCC), captures each view, assembles
project-prefixed MP4s (§11.4.155) under `/Volumes/T7/Downloads/Recordings/`
(gitignored raw corpus, §11.4.128), and reaps the tmux session (§11.4.14).

## Views recorded + OCR-validated (real content)

| View | Recording | OCR-confirmed real content |
|------|-----------|----------------------------|
| Dashboard | `helixcode-tui-dashboard-20260622T213246Z.mp4` | (prior run) HelixCode v1.0.0, sidebar, Workers/Tasks counters, Status bar |
| Tasks | `helixcode-tui-tasks-20260623T133410Z.mp4` | `Task Management — Tasks: (1) Code Generation Task "Generate REST API endpoints" (2) Testing Task "Run unit tests" (3) …` — real seeded task list |
| Workers | `helixcode-tui-workers-20260623T133444Z.mp4` | `Worker Management — ID Hostname Status Health CPU% Memory% Tasks Last Heartbeat … No workers registered` — real worker table |
| Projects | `helixcode-tui-projects-20260623T133516Z.mp4` | `Project Management — Name Type Path Status Created Updated … No projects found. Click 'New Project'` — real projects view |
| Sessions | `helixcode-tui-sessions-20260623T133553Z.mp4` | `Session Management — Development Sessions: Name Project Mode Status Duration Created … No sessions found` — real sessions view |
| QA | `helixcode-tui-qa-20260623T133656Z.mp4` | `QA Dashboard — QA Engine: DISABLED … ID Status Phase Progress Platforms Banks Duration` — real QA view (DISABLED is the genuine state, no live QA session) |

All six TUI views (the sidebar nav `(d) Dashboard … (q) QA … (c) Settings` plus
each view's management panel) rendered genuine application output — not
simulated/placeholder. The empty-state rows ("No workers registered", "No
projects found") are the real rendered states of a freshly-launched TUI.

## Validation method (honest §11.4.6 note)
The harness reached its 82nd tool-step and produced all 5 per-view recordings,
but a transient API throttle killed the agent **before** it ran the harness's
own self-validated OCR analyzer + wrote this doc. The conductor therefore
finished the validation directly: extracted a frame from each of the 5 MP4s
(ffmpeg) and OCR-read it (tesseract), confirming the real per-view content above.
The harness's self-validated analyzer (`scripts/video_qa/record_feature.sh`
`selftest`: golden-good PASS / golden-bad FAIL — provably cannot bluff) remains
available for a future re-run; this evidence is the conductor's direct
OCR-confirmation of real rendered content, which the recordings (committed-by-
reference, gitignored) back.

## Status
TUI per-view coverage: **6/6 views recorded with real-content OCR confirmation.**
Fully automatic (tmux send-keys), no TCC/Aqua/human.
