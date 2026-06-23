# HXC-108 — HelixCode TUI + Server: Real Video-QA Recordings (curated evidence)

| | |
|---|---|
| **Run-id** | HXC-108_tui_server_recordings |
| **Date (UTC)** | 2026-06-22 / 2026-06-23 |
| **HEAD at recording** | `1b820b50` (CLI-evidence commit); binaries built at prior HEAD; HEAD later advanced to `f012ac2f` via a parallel constitution-pointer bump that does not touch these binaries |
| **Surfaces** | HelixCode **terminal-UI** client (`helix_code/applications/terminal_ui` → built binary) + HelixCode **server** (`cmd/server` → `helix_code/bin/helixcode`, Gin API) |
| **Capture method** | **asciinema** (terminal session → text cast) → **agg** (GIF) → **ffmpeg** (MP4 H.264 `+faststart` `yuv420p`). **TCC-free**: needs NO macOS Screen-Recording grant. The harness's native `screencapture -l<wid>` window path is TCC-blocked on this host (confirmed again by `selftest` `[live] SKIP` below) — same env-gap as the CLI slice. |
| **Recordings dir** | `/Volumes/T7/Downloads/Recordings/` (gitignored raw corpus, §11.4.128) |
| **Prefix** | `helixcode` (from `HELIX_RELEASE_PREFIX` in `<repo-root>/.env`, §11.4.155/.151) |
| **Validator** | harness `scripts/video_qa/record_feature.sh` `ocr-analyze` (tesseract OCR), self-validated golden-good/golden-bad per §11.4.107(10) |
| **Recording resource ownership** | §11.4.119 — this agent was the SINGLE recording-resource owner for this slice (CLI slice already done); no other recorder ran concurrently |
| **Anti-bluff** | both captured casts scanned for `simulated`/`simulate`/`TODO implement`/`placeholder`/`for now`/`in production this would` → **all clean** |

## Analyzer self-validation (the validator provably cannot bluff)

`record_feature.sh selftest` (§11.4.107(10)), captured this session:
```
[golden-good] OCR-VERDICT: PASS (all expected patterns present, no bluff): HELIXCODE_OCR_PROBE_4242
[golden-bad ] OCR-VERDICT: FAIL (missing expected pattern(s): HELIXCODE_WRONG_PATTERN_9999)
ANALYZER SELF-VALIDATION: PASS (golden-good PASS rc=0, golden-bad FAIL rc=1) — analyzer cannot bluff
[live] SKIP (honest env-gap §11.4.3): screencapture window path TCC-blocked — refusing whole-desktop fallback
SELFTEST: PARTIAL-PASS (analyzer self-validation PASS; live screencapture an honest env-gap §11.4.3)
```
Every PASS below is read back by **this same** self-validated OCR analyzer.

## Calibration note (§11.4.6 — patterns calibrated on the project's own frames)

The agg monospace render produces stable OCR substitutions: a space-prefixed `0`→`@` (so `Total: 0`→`Total: @`), and long tokens that line-wrap (e.g. `total_tokens` breaks across two lines as `total_` + `tokens`). Expected patterns were therefore chosen from tokens that survive OCR **and** uniquely prove the real feature. The underlying real output is captured verbatim in the supplementary `.cast` files. This calibrates the *patterns*, never the *content*.

> **Render-path finding (this session):** the harness `record` subcommand uses `screencapture` (TCC-blocked). The CLI slice's TCC-free route was asciinema→agg→ffmpeg→OCR. agg's bitmap monospace IS tesseract-readable at the agg-default render size, but the TUI has a ~15–17 s provider-init period before the tview dashboard paints, so the harness's built-in fps=1/first-30-frames `validate` sampler misses the painted frames. A small deterministic helper (`/Volumes/T7/tmp/render_validate.sh`, ephemeral) renders the full cast and selects the frame with the most expected-pattern hits as the canonical evidence frame, then validates it with the SAME self-validated `ocr-analyze` analyzer. This is a sampling-window adaptation, not an analyzer change.

---

## Durable evidence (committed) — rotation-proof anchors (§11.4.83, HXC-108 audit F2 fix)

The raw corpus (`/Volumes/T7/Downloads/Recordings/`, §11.4.128/.154-rotatable) is the
secondary location. The two load-bearing recordings (TUI dashboard render + server
real-API flow) + their evidence frames are **copied into the committed tree** so a
rotation cannot dangle these citations:

| Committed artifact | sha256 | raw-corpus md5 (verified byte-identical pre-copy) |
|---|---|---|
| `docs/qa/HXC-108_tui_server/helixcode-tui-dashboard-20260622T213246Z.mp4` (h264 790×560 131f) | `e062b8df6eb25c8431604aad91ee604b92bc2f6724b6ccca1bd86c5d43314e61` | `607df3c63fcb5880aa32edaaf35cbc4d` |
| `docs/qa/HXC-108_tui_server/helixcode-tui-dashboard-20260622T213246Z.evidence_frame.png` | `cb87560b78ca3dc1ca583b2c0eaedb1a9554170afb96da4b82fcee55674eace7` | — |
| `docs/qa/HXC-108_tui_server/helixcode-server-api-20260622T214152Z.mp4` (h264 790×560 48f) | `98e32d4f9e6c6d3cd53dd70f308646a45dc899dc07d3f7b0c2a969c0b16b85c5` | `0aea0e9677d3214ea9679e101fab07f3` |
| `docs/qa/HXC-108_tui_server/helixcode-server-api-20260622T214152Z.evidence_frame.png` | `c4e060bcf09930db4d46e1cdc9f517c5d7a718d89eecb22a3ae7ed339dd3e755` | — |

Both MP4s ffprobe-verified valid H.264 after copy.

## PER-FEATURE RESULTS

### TUI-1. terminal-UI dashboard — launch → real tview dashboard render — **PASS**
- **Driver:** `cd helix_code && <tui-binary>` launched under asciinema's PTY; warm 24 s for live provider init + paint; `SIGINT` for clean exit (the TUI's only quit path — see interaction limit below).
- **MP4:** `helixcode-tui-dashboard-20260622T213246Z.mp4` (md5 `607df3c63fcb5880aa32edaaf35cbc4d`, H.264/yuv420p/131f)
- **Evidence frame:** `helixcode-tui-dashboard-20260622T213246Z.evidence_frame.png`
- **OCR-validated real output excerpt** (read back from the recording):
  ```
  HelixCode v1.0.0
  (d) Dashboard  (t) Tasks  (w) Workers  (p) Projects
  (s) Sessions   (l) LLM    (q) QA       (c) Settings
  HelixCode — Enterprise distributed AI development platform
  Workers  Total: 0      Tasks  Total: 0
  Status: Ready | User: Not logged in | Session: None
  ```
- **Validator verdict:** `OCR-VERDICT: PASS` — `--expect "HelixCode v1.0.0|Enterprise distributed AI development platform|Workers|Tasks|Sessions|Settings|Status"` → 7/7. This is the genuine tview-rendered dashboard (real sidebar nav, real empty worker/task pools, honest "Not logged in" state) — not simulated. The binary initialized real providers from `.env` (DeepSeek/Mistral/Groq/OpenRouter live `/models` catalogs visible on stderr) and ran in DB/Redis-degraded mode (chat/LLM available).
- **Interaction limit (honest, §11.4.3):** the TUI is a fully-interactive tview app with NO command-line flags and NO headless/non-interactive mode; sidebar navigation (`d/t/w/p/s/l/q/c`) requires live keypresses that a recorded shell cannot reliably inject into tcell's raw-mode input, and quit is signal-only (`SIGINT`/`SIGTERM`). The recordable-now non-interactive slice is **launch + dashboard render**, captured + validated above. Deeper per-view captures (Tasks/Workers/LLM-chat screens) would require a scripted PTY key-injection harness (e.g. tmux send-keys against the live tcell session) — a follow-up if the conductor wants per-view recordings.

### SRV-1. server — real `/health` + `/api/v1/server/info` + authenticated `/api/v1/llm/generate` (BLUFF-001) — **PASS**
- **Setup:** launched `helix_code/bin/helixcode` (HelixCode Server v3.0.0, build 2026-06-20, commit 454319f1) on port **18080** via a temp `HELIX_CONFIG` copy (no tracked-config edit; port 8080 was held by an unrelated foreign `htCore` process, left untouched). The server **auto-booted podman PostgreSQL (`:55432`) + Redis (`:56379`)** per §11.4.76 on-demand-infra, connected to both, and created/verified its schema. `/health` returned 200 in 1 s.
- **Real end-to-end flow (curl against the live server):**
  - `POST /api/v1/auth/register` → **201**, real user persisted to PostgreSQL (uuid `109b324c-76d5-408a-bd4d-103e1f8e112a`).
  - `POST /api/v1/auth/login` → **200**, real 288-char JWT.
  - `POST /api/v1/llm/generate` (Bearer JWT) → **200**, **real DeepSeek response**.
- **MP4:** `helixcode-server-api-20260622T214152Z.mp4` (md5 `0aea0e9677d3214ea9679e101fab07f3`, H.264/yuv420p/48f)
- **Evidence frame:** `helixcode-server-api-20260622T214152Z.evidence_frame.png` (whole session fits one 80×24 screen)
- **OCR-validated real output excerpt** (read back from the recording):
  ```
  $ curl http://localhost:18080/health
  {"status":"healthy","timestamp":"2026-06-22T21:41:53.30136Z","version":"1.0.0"}
  $ curl http://localhost:18080/api/v1/server/info
      "database": { "connected": true,
      "go_version": "1.24", "name": "HelixCode Server",
      "redis": { "connected": true, "version": "1.0.0"
  $ curl -X POST http://localhost:18080/api/v1/llm/generate  (auth, real DeepSeek)
  {"content":"4","finish_reason":"stop","model":"deepseek-chat","provider":"DeepSeek",
   "status":"success","usage":{"completion_tokens":1,"prompt_tokens":17,"total_tokens":18}}
  ```
- **Validator verdict:** `OCR-VERDICT: PASS` — `--expect "healthy|connected|HelixCode Server|go_version|deepseek-chat|DeepSeek|finish_reason|prompt_tokens"` → 8/8.
- **BLUFF-001 cleared:** the model genuinely answered `2+2 = 4` via a real DeepSeek call with genuine token accounting (`prompt_tokens:17 completion_tokens:1 total_tokens:18`) — impossible for a simulated response. The `/api/v1/server/info` real `database.connected:true` + `redis.connected:true` prove a live backing stack.
- **Teardown (§11.4.14):** `SIGINT` → server logged `✅ Server exited properly` / `✅ Database connection pool closed`; `:18080` freed; foreign `htCore` on `:8080` untouched; podman infra (already healthy before launch, orchestrator-managed) left running.

### SRV-2. server unauthenticated discovery endpoints (real provider/model data) — **PASS** (supplementary, captured as JSON not video)
Probed on the live server (not separately video-recorded; the SRV-1 recording is the canonical video):
- `GET /api/v1/llm/providers` → **200**, 8 real providers: OpenAI, Anthropic, Mistral, Gemini, DeepSeek, Xai, Xiaomi (+1), each with model counts + `status:available` (BLUFF-002 class — real provider data, not hardcoded).
- `GET /api/v1/llm/models` → **200**, real catalog with per-model `context_length`/`score`/`provider`/`tier`/`verified` metadata.
- `GET /api/v1/system/stats` → **401** (auth-guarded — honest).

---

## SKIP / not recorded here (honest §11.4.3)
- **TUI per-view screens** (Tasks/Workers/LLM-chat/Settings beyond the landing dashboard): the TUI has no scriptable navigation path; reliable per-view capture needs a PTY key-injection harness (tmux send-keys against the live tcell session). Recordable in a follow-up — honest interaction limit, NOT a faked PASS.
- **Server streaming generate** (`POST /api/v1/llm/stream`): not separately recorded; non-streaming generate already proves the real authenticated LLM path.
- **Desktop GUI / mobile**: out of this TUI+server slice (Desktop Fyne GUI is HXC-112-gated).

## Env gap (the reason for the asciinema route, §11.4.3)
The harness's native window-video path (`screencapture -v -V<n> -l<wid>` + still-timelapse fallback) is **TCC-blocked** on this host (`could not create image from window`). The harness correctly refuses a whole-desktop fallback (§11.4.154) and SKIPs its live path (confirmed by `selftest [live] SKIP`). The asciinema text-capture route needs **no** Screen-Recording grant and produces the same MP4 + OCR-validation, so both surfaces were recorded TCC-free.

## Sources verified
- `helix_code/applications/terminal_ui/main.go` (TUI entry: tview app, no flags, SIGINT-only quit) — 2026-06-23.
- `helix_code/internal/server/server.go` + `cmd/server/main.go` + `internal/server/handlers.go` + `internal/server/llm_generate.go` (real routes, auth-guarded generate, register/login) — 2026-06-23.
- Live runs of every feature before recording (confirmed genuinely working before any recording, §11.4.6) — 2026-06-23.
- `docs/qa/HXC-108_cli_recordings_evidence.md` (CLI slice; same TCC-free asciinema route + analyzer).
