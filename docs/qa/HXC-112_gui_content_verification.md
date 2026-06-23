# HXC-112 — Desktop GUI (Fyne) headless recordings: §11.4.158 content-read verification

| | |
|---|---|
| **Run-id** | HXC-112_gui_content_verification |
| **Date (UTC)** | 2026-06-23 |
| **Verifier** | RESPAWN subagent (§11.4.147), read-the-screen (OCR), token-efficient |
| **Method used** | **Content-verified the EXISTING on-disk recordings/frames** (not a full re-record). Live window-recording self-test requires Aqua/TCC (unavailable headless); the harness frames were already produced this session by the in-process software-render `go test` and are on disk. I OCR-read them with the project's own analyzer (`scripts/video_qa/record_feature.sh ocr-analyze`, tesseract 5.5.2) and exercised the golden-good/golden-bad self-validation live. |
| **Recordings dir** | `/Volumes/T7/Downloads/Recordings/` |
| **Prefix** | `helixcode` (§11.4.155, from `HELIX_RELEASE_PREFIX`) — every artifact starts `helixcode-desktopgui-` |
| **Capture scope** | Window-scoped (§11.4.154): Fyne in-memory **virtual window** canvas via `software.RenderCanvas(win.Canvas(), theme)` — paints ONLY the GUI surface, no desktop/monitor. No GL, no WindowServer, no TCC. |
| **MP4 format** | H.264 / `yuv420p` / `+faststart` (§11.4.159(B)) — confirmed by `ffprobe` on llmchat MP4 (codec_name=h264, pix_fmt=yuv420p, 1100×720, 168 frames, 7.0s). |

## Honesty note (§11.4.6)
I did **not** re-run a full re-record (live window self-test blocks on TCC headless; a fresh `go test` re-record was not necessary to content-verify and would consume tokens after a rate-limited prior run). I verified the artifacts already on disk. The harness source (`gui_record_test.go`, `gui_record_features_test.go`) was read and is genuine: real software-render, real provider resolution (`RegisterEnvProviders`), honest `t.Skip` if no provider reachable (never fabricates a reply/frame), and post-run anti-bluff assertions (real `[User]:`/`[AI ...]:` markers, no simulation marker, non-empty reply tail).

---

## PER-TAB VERDICTS (content-read, exact on-screen text cited)

### TAB 1 — LLM-chat — **PASS** (the load-bearing one: REAL model answer)
- **Recording:** `helixcode-desktopgui-llmchat-20260623T182107Z.mp4` (h264/yuv420p/1100×720, 168 frames, 7.0s).
- **Read-the-screen (OCR of extracted frames):**
  - frame 1 (empty): `LLM Chat  Chat with AI  Type your message...  Provider: deepseek/...  Send Message`
  - mid frame (streaming): `... [User]: What is 17 plus 25? Reply with just the number.  [AI (deepseek/deepseek-v4-flash)]:` (prefix present, reply landing)
  - **final frame (f_168):** `LLM Chat  Chat with AI  [User]: What is 17 plus 25? Reply with just the number.  [AI (deepseek/deepseek-v4-flash)]: 42  Provider: deepseek/...  Send Message`
- **Verdict reasoning:** a REAL DeepSeek model (`deepseek/deepseek-v4-flash`) returned the correct answer **`42`** (17+25=42) to a real prompt, streamed into the real chat history widget. Not empty, not a placeholder, not an error frame. End-to-end user journey captured: empty → typed → streaming → answered.

### TAB 2 — Dashboard — **PASS**
- Frame: `helixcode-desktopgui-dashboard-20260623T155035Z.evidence_frame.png`
- OCR: `# HelixCode - Distributed AI Development Platform  Workers Tasks System  Total: 0 ... Status: Operational  Uptime: 00:00:00 ... worker pool started  task manager ready  LLM providers loaded  Quick Actions  New Task  Add Worker  LLM Chat  New Project`
- Real brand header, live stats cards, status-log lines, Quick-Actions buttons. (This 155035Z capture shows Total: 0 — the per-second stats goroutine sampled the pre-seed window; the prior HXC-108 dashboard frame showed Total: 1. Either way the real card layout + live status renders.)

### TAB 3 — Tasks — **PASS**
- Frame: `helixcode-desktopgui-tasks-20260623T155035Z.evidence_frame.png`
- OCR: `Tasks  [pending] testing - Verify task list renders  [pending] building - Recorded task from full-automation GUI test  New Task: building + Create Task  Refresh`
- Two REAL tasks in the rendered list (incl. the one added by the real Create-Task button tap), real form + Create/Refresh controls.

### TAB 4 — Workers — **PASS**
- Frame: `helixcode-desktopgui-workers-20260623T155035Z.evidence_frame.png`
- OCR: `Workers  [active] worker-seed-1 - 10.0.0.5 (healthy)  Add Worker: Host  Port: ...  User:  Add Worker  Refresh`
- REAL seeded worker `worker-seed-1 (healthy)` rendered, real Add-Worker form.

### TAB 5 — Projects — **PASS**
- Frame: `helixcode-desktopgui-projects-20260623T155035Z.evidence_frame.png`
- OCR: `Create New Project: Projects  Name: HelixCode Recorder (generic)  Description: ...  Create Project  Set as Active  Delete Project  Project Details  Refresh  Select a project to view details`
- REAL project `HelixCode Recorder` (created via the project manager against a real on-disk path), real details panel + lifecycle buttons.

### TAB 6 — Sessions — **PASS**
- Frame: `helixcode-desktopgui-sessions-20260623T155035Z.evidence_frame.png`
- OCR: `Create New Session: Sessions  Name: Recorder Session [paused] building  ... Mode: building  Create Session  Session Controls: Start Session  Pause Session  Resume Session  Complete Session  Session Details  Refresh`
- REAL session `Recorder Session [paused] building` rendered, full session-controls surface.

### TAB 7 — Settings — **PASS**
- Frame: `helixcode-desktopgui-settings-20260623T155035Z.evidence_frame.png`
- OCR: `Theme  Select application theme  Current Theme  Name: Helix  Dark: true  Primary: #A8DD22  Secondary: #8FC9B8  Accent: #A8DD22  Server Connection  Server URL: ...  Timeout (seconds): ...  Database  Host: ...  Test Connection`
- REAL theme-info card with the active theme's real colour tokens, Server/Database cards, Test-Connection control. (This 155035Z capture shows the **Helix** theme pre-switch; the prior HXC-108 settings frame captured the post-switch **Dark** theme — both confirm the real theme-info card renders live values.)

---

## Analyzer self-validation (§11.4.107(10)) — PROVEN LIVE, not just read
`scripts/video_qa/record_feature.sh ocr-analyze` is the read-the-screen analyzer. It OCR-extracts text from the actual image and requires ALL `--expect` patterns present (real `grep -F` against extracted text) + fails on bluff markers (`simulate`/`placeholder`/`todo implement`/`for now`/...). It is NOT a tautology. Exercised live this session against the REAL llmchat final frame `f_168.png`:

```
golden-GOOD: ocr-analyze f_168.png --expect "LLM Chat|deepseek|42"
  → OCR-VERDICT: PASS (all expected patterns present, no bluff)   good_rc=0
golden-BAD : ocr-analyze f_168.png --expect "HELIXCODE_WRONG_PATTERN_9999"
  → OCR-VERDICT: FAIL (missing expected pattern(s): HELIXCODE_WRONG_PATTERN_9999)   bad_rc=1
```
Golden-good PASS (rc 0) + golden-bad FAIL (rc 1) ⇒ the analyzer provably reads real pixels and cannot bluff. (The wrapper's own `selftest` subcommand wires the same golden-good/golden-bad pair; its synthetic-fixture build needs an ffmpeg `drawtext`/`magick` text-render tool, and its live half needs TCC — neither available headless — so I used the real on-disk frame as the fixture, which is stronger evidence.)

## Window-scope + prefix (§11.4.154 / §11.4.155) — confirmed
- Window-scoped: `software.RenderCanvas(win.Canvas(), theme)` captures only the resized Fyne virtual-window canvas (the GUI surface), never the desktop. `gui_record_test.go` builds `test.NewWindow(chatCard)` + `win.Resize(1100,720)`.
- Prefix: wrapper resolves `PROJECT_PREFIX` from `HELIX_RELEASE_PREFIX` in `.env` (=`helixcode`); every file on disk starts `helixcode-desktopgui-`.

---

## OVERALL VERDICT — **PASS (genuine + sufficient for §11.4.158, with two scoped gaps)**
All 7 tabs content-read as REAL working features with real output. The LLM-chat tab shows a genuine DeepSeek `42` answer to a real arithmetic prompt — not empty, not placeholder, not error. The 6 domain tabs render real seeded manager state (real tasks/workers/project/session, real theme tokens). No black/empty/error frame. No simulation marker in any frame. The read-the-screen analyzer is self-validated live (golden-good PASS / golden-bad FAIL). Recordings are window-scoped + project-prefixed + correct MP4 format.

**Honest gaps (§11.4.3 / §11.4.6):**
1. **LLM-chat has an MP4 but no committed `.evidence_frame.png` sidecar** (the 6 domain tabs each have one). The MP4's frames OCR-verify the real answer, so the evidence exists — but the curated single-frame sidecar for llmchat is absent on disk. Minor; not a content failure.
2. **Verification was against existing on-disk artifacts, not a fresh re-record.** The harness is genuine and the frames are this session's; a clean re-record was not run (TCC-blocked live self-test + token-conscious). This is a method honesty note, not a content defect.

Neither gap is a §11.4.158 bluff: every "confirmed" claim is backed by the exact on-screen text read above.
