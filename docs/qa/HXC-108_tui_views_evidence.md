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

## Durable evidence (committed) — rotation-proof anchors (§11.4.83, HXC-108 audit F2 fix)

The raw corpus (`/Volumes/T7/Downloads/Recordings/`, §11.4.128/.154-rotatable) is the
secondary location. All per-view recordings + evidence frames are **copied into the
committed tree** so a rotation cannot dangle these citations. All MP4s ffprobe-verified
valid H.264 after copy (the 5 driven views are 1604×868/50f; dashboard is the prior run):

| View | Committed MP4 sha256 | Committed frame sha256 |
|---|---|---|
| dashboard (prior) | (frame only) | `cb87560b78ca3dc1ca583b2c0eaedb1a9554170afb96da4b82fcee55674eace7` |
| tasks | `d0e01c4e5211e5929256a230f0528b5ea533f2860b2728a745ec13bee9cbb6af` | `319a71eda5c37bc3cbde8e89a6de5aa072972a9d1a3207f158970913edb22cd2` |
| workers | `383400681bf61ff24ba5e135b62779f9f8bf10af4705860c791c9ed1498b9fd0` | `d78fa46315e86daf988c5d9de320ae2c51b7c31f36b743cbc1d8019062ba0689` |
| projects | `413b4f0a9e2942de8b039614af5f8b51a8a09899019107358037c0dfc0f7a7d4` | `82e410d11b0e0fcc1f9393699872b270a4e044788c62cb9a4907c831a1f695bc` |
| sessions | `b35f96009c098bf468f37c58c6c4313cdd4ca3fc1aca6fa224322dd8cf947108` | `b5916796943addd681bcb2033d8a7b3f063ba8207a126e777e098f753d51e9fc` |
| qa | `90d60d8ee561e51efda94fa5ddc024598a31da9cabf82a4db4ad9a6a92df840e` | `05e9fbe9b81076254268ff1049db908550dc0e27329a0cd3c71a4f9e32fbd158` |

Committed under `docs/qa/HXC-108_tui_views/` (filenames preserve the `<run-id>` timestamps cited above).

## Status
TUI per-view coverage: **6/6 views recorded with real-content OCR confirmation.**
Fully automatic (tmux send-keys), no TCC/Aqua/human.
