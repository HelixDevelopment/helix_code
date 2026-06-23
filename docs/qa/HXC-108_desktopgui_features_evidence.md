# HXC-108 — HelixCode Desktop GUI (Fyne): Real Video-QA Recordings of the non-LLM-chat features (curated evidence)

| | |
|---|---|
| **Run-id** | HXC-108_desktopgui_features |
| **Date (UTC)** | 2026-06-23 |
| **HEAD at recording** | `9301a264` |
| **Surface** | HelixCode **desktop GUI** (Fyne v2.7.0) — `helix_code/applications/desktop` — the six tabs BEYOND LLM-chat: **Dashboard, Tasks, Workers, Projects, Sessions, Settings** (LLM-chat already covered by `HXC-108_*` LLM-chat slice via `gui_record_test.go`) |
| **Capture method** | **Fully in-process** Fyne software renderer + in-memory test driver (`fyne.io/fyne/v2/test` + `.../driver/software`). `test.NewApp()` → no real window / no WindowServer / no GL / **no TCC**; each frame software-painted via `software.RenderCanvas(canvas, theme)` (== `Canvas.Capture()` under a software painter). Runs in ANY launchd domain incl. Background — satisfies §11.4.98 (no Aqua, no human, rerunnable). Identical mechanism to the proven LLM-chat harness `gui_record_test.go`. |
| **Harness** | `helix_code/applications/desktop/gui_record_features_test.go` (build tag `!nogui`, zero production weight) — extends the `gui_record_test.go` pattern |
| **Wrapper** | `scripts/video_qa/record_gui_features_inprocess.sh` (assembles per-feature MP4 + OCR-validates) |
| **Recordings dir** | `/Volumes/T7/Downloads/Recordings/` (gitignored raw corpus, §11.4.128) |
| **Prefix** | `helixcode` (from `HELIX_RELEASE_PREFIX` in `<repo-root>/.env`, §11.4.155/.151) |
| **MP4 format** | H.264 / `yuv420p` / `+faststart`, 1200×800 (§11.4.159(B)) |
| **Validator** | harness `scripts/video_qa/record_feature.sh` `ocr-analyze` (tesseract OCR), self-validated golden-good/golden-bad per §11.4.107(10) |
| **Recording resource ownership** | §11.4.119 — this agent rendered ENTIRELY in-process (Fyne software canvas, no terminal). A concurrent TUI recorder owns the tmux/asciinema resource; **no contention** (disjoint resources). |
| **Anti-bluff** | each rendered frame OCR-scanned for `simulated`/`simulate`/`TODO implement`/`placeholder`/`for now`/`in production this would` by `ocr-analyze` → **all clean**; every feature uses REAL managers + REAL bundle-backed i18n + REAL seeded data (no fabrication) |

## What is REAL here (no simulation — §11.4.2 / §11.4.107 / §11.4.158)

- Each tab is built by the **same production `create<Tab>()` method** `main.go`'s `setupUI()` calls — `createDashboardTab` / `createTasksTab` / `createWorkersTab` / `createProjectsTab` / `createSessionsTab` / `createSettingsTab`. The captured frames are the REAL rendered Fyne widget trees, NOT hand-rebuilt facsimiles, so they cannot drift from production.
- The `DesktopApp` is wired with the REAL managers production uses (`task.NewTaskManager` + `worker.NewWorkerManager`(in-memory repo) + `project.NewManager` + `session.NewManager` + `NewThemeManager`) and the REAL bundle-backed i18n translator (`i18n.NewTranslator()`), exactly as the fixed GUI `main()` does — so labels render localized prose, not raw message-ID echoes (HXC-111 guard composes).
- Where a feature involves data, REAL data is seeded through the REAL manager APIs (`CreateTask` / `AddWorker` / `CreateProject` — against a real on-disk path / session `Create`) and the REAL interaction is driven (`test.Type`/`test.Tap`/`Select.SetSelected`/`List.OnSelected`). Each test ALSO hard-asserts the real manager state changed (e.g. ≥1 task/worker/project/session present, theme switch took) — proving the production closure ran, not just rendered.

## Analyzer self-validation (the validator provably cannot bluff) — §11.4.107(10)

`record_feature.sh selftest`, captured this session:
```
[gui-feat] self-validating OCR analyzer (golden-good/golden-bad)…
[gui-feat] analyzer self-validation: PASS
```
The golden-good fixture (expected probe string) PASSes; the golden-bad fixture (a string NOT on screen, `HELIXCODE_WRONG_PATTERN_9999`) FAILs — so the analyzer reading back every PASS below provably cannot bluff.

## Calibration note (§11.4.6 — patterns calibrated on the project's own frames)

Expected OCR patterns per feature were chosen from the project's OWN rendered frames (OCR-confirmed below), favouring tokens that survive tesseract on the Fyne software-render AND uniquely prove the real feature. `ocr-analyze` is ALL-must-match. This calibrates the *patterns*, never the *content*.

---

## PER-FEATURE RESULTS — all six **PASS** (recorded + OCR content-validated)

In-process harness (all green):
```
--- PASS: TestRecordDesktopGUIDashboard (5.48s)   RECORD-OK dashboard: 5 frames
--- PASS: TestRecordDesktopGUITasks     (0.20s)   RECORD-OK tasks: 4 frames, 2 real tasks
--- PASS: TestRecordDesktopGUIWorkers   (0.21s)   RECORD-OK workers: 4 frames, 2 real workers
--- PASS: TestRecordDesktopGUIProjects  (0.23s)   RECORD-OK projects: 4 frames
--- PASS: TestRecordDesktopGUISessions  (0.24s)   RECORD-OK sessions: 4 frames
--- PASS: TestRecordDesktopGUISettings  (0.27s)   RECORD-OK settings: 4 frames
```

### GUI-1. Dashboard — real brand header + live stats cards + Quick Actions — **PASS**
- **Real data:** a real task + a real worker seeded; the per-second stats goroutine repaints non-zero totals over a 4-sample window.
- **MP4:** `helixcode-desktopgui-dashboard-20260623T110359Z.mp4` (md5 `c302e2b8a032f62384a468d1c54e0c95`, 5 frames → H.264/yuv420p/1200×800).
- **OCR expected (all-must-match):** `HelixCode|Workers|Quick Actions` → **PASS**.
- **Real-content excerpt (OCR of a captured frame):** `# HelixCode - Distributed AI Development Platform  Workers Tasks System  Total: 1 ... Status: Operational Uptime: 00:00:05 ... Quick Actions  New Task ... Add Worker  LLM Chat  New Project`

### GUI-2. Tasks — real task list + real "Create Task" through the production button closure — **PASS**
- **Real interaction:** seeded one task, then `test.Type` into the description entry + `test.Tap` the real Create button; harness asserts the real manager now holds 2 tasks.
- **MP4:** `helixcode-desktopgui-tasks-20260623T110359Z.mp4` (md5 `327319062af6a51de55f15d7ec16fca3`, 4 frames). OCR matched on `frame_0002.png` (post-create-tap).
- **OCR expected:** `Tasks|Recorded task|Create Task` → **PASS**.
- **Real-content excerpt:** `Tasks  [pending] testing - Verify task list renders  [pending] building - Recorded task from full-automation GUI test  New Task: building + Create Task  Refresh` — the task added by the real button-tap is genuinely in the rendered list.

### GUI-3. Workers — real worker list + real "Add Worker" through the production closure — **PASS**
- **Real interaction:** seeded `worker-seed-1`, then typed a host + tapped the real Add Worker button; harness asserts ≥1 worker (2 present).
- **MP4:** `helixcode-desktopgui-workers-20260623T110359Z.mp4` (md5 `cfb90f8a1c7f182bffa993efec90f319`, 4 frames).
- **OCR expected:** `Workers|worker-seed-1|Add Worker` → **PASS**.
- **Real-content excerpt (final driven frame, `frame_0003.png`):** `Workers  [active] worker-seed-1 - 10.0.0.5 (healthy)  [pending] worker-192.168.1.42-... - 192.168.1.42 (unhealthy)  Add Worker: Host ... Add Worker  Refresh` — BOTH the seeded worker AND the worker added by the real Add-Worker tap are rendered (interaction genuinely captured, not initial-frame-only).

### GUI-4. Projects — real project created via the project manager + real row-selection details panel — **PASS**
- **Real data:** `project.Manager.CreateProject` against a REAL on-disk path (the manager validates the path exists — no fabrication); cache refreshed; real `List.OnSelected(0)` drives the details panel.
- **MP4:** `helixcode-desktopgui-projects-20260623T110359Z.mp4` (md5 `5fb40e98625b428ab27f6b284a4c355d`, 4 frames).
- **OCR expected:** `Projects|HelixCode Recorder|Project Details` → **PASS**.
- **Real-content excerpt:** `Projects  Create New Project: ... HelixCode Recorder (generic) ... Create Project  Set as Active  Delete Project  Project Details  Name: HelixCode Recorder ... /Volumes/T7/tmp/.../helixcode-rec  Description: GUI recording target  Created: 23 Jun 26 14:02 MSK` — the real created project's details rendered after a real selection.

### GUI-5. Sessions — real session created via the session manager + real row-selection details panel — **PASS**
- **Real data:** `session.Manager.Create(...)`; cache refreshed; real `List.OnSelected(0)` drives the details panel.
- **MP4:** `helixcode-desktopgui-sessions-20260623T110359Z.mp4` (md5 `565b00b863f12d4cf0e4ca0aebfa254c`, 4 frames).
- **OCR expected:** `Sessions|Recorder Session|Session Details` → **PASS**.
- **Real-content excerpt:** `Sessions  Recorder Session [paused] building  Session Details  Name: Recorder Session  Mode: building  Status: paused  Project ID: proj-rec-1  Description: GUI recording session  Created: 23 Jun 26 14:02 MSK ... Create New Session: ... Session Controls: Start Session Pause Session Resume Session Complete Session` — the real session's details rendered after a real selection.

### GUI-6. Settings — real theme/server/db/LLM/about cards + real theme switch through the production Select closure — **PASS**
- **Real interaction:** the production theme `Select`'s `OnChanged` closure driven via `SetSelected` to a different available theme; harness asserts `ThemeManager.GetCurrentTheme()` changed (Helix → Dark). The theme-info card repaints with the new theme's real colours.
- **MP4:** `helixcode-desktopgui-settings-20260623T110359Z.mp4` (md5 `5f1b2b78cd12e785c6e45b555d8e4463`, 4 frames).
- **OCR expected:** `Current Theme|Primary|Server Connection` → **PASS**.
- **Real-content excerpt (final driven frame, `frame_0003.png`):** `Theme  Select application theme  dark  Current Theme  Name: Dark  Dark: true  Primary: #2E86AB  Secondary: #A23B72  Accent: #F18F01  Server Connection  Server URL: ...  Timeout (seconds): ...  Database  Host: ... Test Connection` — the real theme switch to **Dark** is reflected in the re-rendered theme-info card.

---

## Honest gaps / notes (§11.4.3 / §11.4.6)

- **No SKIPs.** Unlike the LLM-chat slice (which honestly SKIPs without an API key), these six tabs render local domain state and need NO LLM provider — so all six recorded unconditionally.
- **Dialogs not exercised.** Buttons that open Fyne modal dialogs (`dialog.ShowInformation` / `ShowConfirm` / `ShowError`) are not driven here — modal dialogs render on the window overlay; the in-process software canvas capture path was kept to the tab content. The tabs' primary feature surfaces (lists, forms, details panels, theme switch, live stats) ARE driven and captured. Driving overlay dialogs is a documented follow-up, not a faked pass.
- **Raw frames are ephemeral** (the wrapper rotates its own scope per §11.4.154 and cleans its temp frames base on exit). The committable record is this curated doc + the gitignored MP4s under the recordings dir; the harness + wrapper regenerate them deterministically (`scripts/video_qa/record_gui_features_inprocess.sh`).

## Reproduce

```bash
# Full pipeline (build harness → 6 MP4s → analyzer self-validate → OCR-validate each):
scripts/video_qa/record_gui_features_inprocess.sh
# Harness only (frames to a chosen dir):
cd helix_code && HELIX_GUI_FEATURES_FRAMES_DIR=/tmp/feat \
  go test -count=1 -run 'TestRecordDesktopGUI(Dashboard|Tasks|Workers|Projects|Sessions|Settings)' -v ./applications/desktop/
```
