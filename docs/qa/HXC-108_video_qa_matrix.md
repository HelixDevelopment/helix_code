# HXC-108 — Video-QA Recording Harness: Clients × Features Matrix

**Owner:** HXC-108 (video-QA recording harness)
**Harness:** `scripts/video_qa/record_feature.sh` (+ `scripts/video_qa/find_window_id.swift`)
**Revision:** 1
**Last modified:** 2026-06-22
**Status:** harness mechanics PROVEN (analyzer self-validation PASS); live window-scoped
recording is an honest host TCC env-gap on the current dev host (§11.4.3) — see "Host capability gap" below.

This document enumerates HelixCode's client surfaces × the features recordable per client,
per §11.4.153 (per-feature status) / §11.4.158 (intensive recording coverage) / §11.4.143
(real-user-journey for app surfaces). It marks which features are recordable NOW via the
CLI/TUI non-interactive paths versus which are GUI-input-dependent and therefore owned by the
concurrent **HXC-112** desktop-GUI stream (this harness deliberately stays clear of HXC-112's
`helix_code/applications/desktop/`, `helix_code/scripts/testing/`, and any
`helix_code-hxc112-*` recording — §11.4.119 single-resource-owner).

> **Coordination note.** Two recording streams exist. HXC-112 drives the desktop Fyne GUI under
> `helix_code/scripts/testing/`. HXC-108 (this harness) lives under `scripts/video_qa/` and uses
> the recording scope-prefix `helixcode-<client>-<feature>-*` (self-test: `helixcode-harness_selftest-*`),
> so corpus rotation (§11.4.154(B)) only ever touches **this** harness's own files.

## Resolved facts (FACT, §11.4.6)

| Fact | Value | Source |
|------|-------|--------|
| Project prefix (§11.4.151/.155) | `helixcode` | `HELIX_RELEASE_PREFIX` in `<root>/.env` (env-first; dir-name fallback would be `helix_code`) |
| Recordings dir (§11.4.158 host override) | `/Volumes/T7/Downloads/Recordings` | root `CLAUDE.md` §11.4.158 project instantiation |
| Built client binaries | `helix_code/bin/{helixcode, cli, tui/helix-tui, helix-desktop, helixcode-web-fresh, helixcode-infra}` | `ls helix_code/bin/` |
| Client app sources | `helix_code/applications/{desktop, terminal_ui, android, ios, aurora_os, harmony_os}` | `ls helix_code/applications/` |

## Clients × recordable features

Legend — **Input class:** `CLI`/`TUI` = scriptable non-interactive, recordable by THIS harness now ·
`GUI` = GUI-input-dependent, **HXC-112-gated** (not recorded here) · `SERVER` = HTTP/API, recordable via curl-driven terminal · `MOBILE` = device/emulator, future phase.

| Client | Binary / source | Feature | Recordable command (deterministic) | Input class | Owner |
|--------|-----------------|---------|-------------------------------------|-------------|-------|
| CLI | `helix_code/bin/cli` | version banner | `cli -version` | CLI | HXC-108 now |
| CLI | `helix_code/bin/cli` | health check | `cli -health` | CLI | HXC-108 now |
| CLI | `helix_code/bin/cli` | list models (CONST-036, LLMsVerifier) | `cli -list-models` | CLI | HXC-108 now (needs verifier/providers reachable) |
| CLI | `helix_code/bin/cli` | list workers | `cli -list-workers` | CLI | HXC-108 now |
| CLI | `helix_code/bin/cli` | non-interactive generate (real LLM) | `cli -non-interactive -prompt "What is 2+2?" -model <m>` | CLI | HXC-108 now (needs provider key/Ollama) |
| CLI | `helix_code/bin/cli` | streaming generate | `cli -non-interactive -stream -prompt "..."` | CLI | HXC-108 now (provider-gated) |
| CLI | `helix_code/bin/cli` | command execution (BLUFF-003 path) | `cli -command "echo hi"` | CLI | HXC-108 now |
| CLI | `helix_code/bin/cli` | QA bank list / run | `cli -qa-list` · `cli -qa-run -qa-banks <b>` | CLI | HXC-108 now (qa-server gated for -qa-run) |
| TUI | `helix_code/bin/tui` (`helix-tui`) | terminal-UI boot + panes | launch in Terminal window, drive keys | TUI | HXC-108 now (window-scoped) |
| Server | `helix_code/bin/helixcode` | HTTP boot + health endpoint | start server, `curl /health` in same window | SERVER | HXC-108 now |
| Server | `helix_code/bin/helixcode` | `/api/v1/llm/generate` real output | `curl -X POST .../generate -d '{...}'` | SERVER | HXC-108 now (infra+provider gated) |
| Desktop GUI | `helix_code/applications/desktop` (`helix-desktop`) | Fyne window, chat, settings, model picker | GUI click/type | **GUI** | **HXC-112** |
| Web | `helix_code/bin/helixcode-web-fresh` | browser UI | browser-driven (chromedp/playwright) | GUI | HXC-112 / future |
| Mobile (iOS) | `helix_code/applications/ios` | app launch + chat journey (§11.4.143) | simulator UI journey | MOBILE | future phase |
| Mobile (Android) | `helix_code/applications/android` | app launch + chat journey (§11.4.143) | emulator UI journey | MOBILE | future phase |
| Mobile (Aurora/Harmony) | `helix_code/applications/{aurora_os,harmony_os}` | app launch | device journey | MOBILE | future phase |

## Recording + validation contract (per feature)

For each recordable feature the harness produces, per §11.4.159:
1. **Window-scoped** capture only (`screencapture -l<id>` / `-v -l<id>`) — never whole-desktop (§11.4.154).
2. Output **MP4 H.264 `+faststart` `yuv420p`** at `/Volumes/T7/Downloads/Recordings/helixcode-<client>-<feature>-<ts>.mp4` (§11.4.155).
3. **Fresh-corpus rotation** of ONLY this harness's own scope-prefixed prior files first (§11.4.154(B)).
4. **Read-the-screen** OCR content-validation against `--expect` patterns, anti-bluff scan for `TODO implement`/`simulate`/`placeholder`/`for now` (§11.4.158/.160/.163); PASS only when expected content is genuinely read back AND no bluff pattern present.
5. **Self-validated analyzer** (golden-good PASS + golden-bad FAIL) so the analyzer itself cannot bluff (§11.4.107(10)).

## Host capability gap (§11.4.3 — FACT, captured, never faked)

`record_feature.sh probe-caps` on the current dev host (2026-06-22, macOS 15.5) reports:

```
  [OK ] screencapture -R region still
  [GAP] screencapture -l<id> window still — "could not create image from window"
  [GAP] screencapture -v window video    — "The operation could not be completed"
  [OK ] OCR (magick render + tesseract read) under TMPDIR=/Volumes/T7/tmp
```

**Interpretation:** macOS TCC on this host grants Screen-Recording for region/full-screen STILLS,
but blocks both window-by-id capture primitives (`-l<id>` still AND `-v` video). Because §11.4.154
forbids a whole-desktop/region substitute for a window, the harness SKIPs live recording with the
exact failing primitive + reproducer rather than fake it. The harness is otherwise fully proven (see
self-test evidence in `docs/qa/HXC-108_selftest_evidence.md`). To unblock live recording: grant the
controlling app full Screen-Recording (incl. video) in System Settings → Privacy & Security → Screen
Recording, then run from a GUI-foreground session — OR run the harness on a host where
`probe-caps` shows `[OK]` for the window primitives.

## Sources verified

- `helix_code/bin/cli -help` (flag inventory), `ls helix_code/bin`, `ls helix_code/applications` — 2026-06-22.
- Host capability probe `record_feature.sh probe-caps` — 2026-06-22.
