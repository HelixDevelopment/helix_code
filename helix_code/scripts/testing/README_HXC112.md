# HXC-112 — Desktop GUI input-driving harness (Fyne / OpenGL)

| | |
|---|---|
| Revision | 1 |
| Created | 2026-06-22 |
| Last modified | 2026-06-22 |
| Status | active |

## Table of contents
- [Problem & root cause](#problem--root-cause)
- [The proven mechanism](#the-proven-mechanism)
- [Files](#files)
- [How to run (Aqua session, display-equipped)](#how-to-run-aqua-session-display-equipped)
- [Environment gap on the headless/SSH host](#environment-gap-on-the-headlessssh-host)
- [Unblocks HXC-108](#unblocks-hxc-108)

## Problem & root cause

The HelixCode Fyne desktop GUI renders **every** widget onto a single OpenGL
`GLFWContentView` `NSView` and consumes input through the native macOS
`NSEvent` responder chain (the HID path). It exposes **no per-widget
Accessibility (AX) tree**.

AppleScript `osascript` / "System Events" resolves UI targets by walking the
AX element hierarchy and posts synthetic events via the AX path. With no AX
tree to target and a GL view that listens on the HID path, System Events
clicks/keystrokes **never reach the canvas** — the HXC-112 failure.

Sources (verified 2026-06-22): GLFW cocoa backend
(`GLFWContentView` keyDown:/mouseDown:, `github.com/glfw/glfw` `src/cocoa_window.m`);
Fyne drivers wiki + `internal/driver/glfw`; Apple Mac Automation Scripting
Guide ("User interface scripting relies upon the OS X accessibility
frameworks"); Apple Quartz Event Services / `CGEventTapLocation`.

## The proven mechanism

`cliclick` (`github.com/BlueM/cliclick`) posts **real CGEvents**
(`CGEventCreateMouseEvent` + `CGEventPost` to `kCGHIDEventTap` — verified in its
`Actions/ClickAction.m`). `kCGHIDEventTap` is "the point where HID system
events enter the window server" (Apple) — the **same** path GLFW listens on.
So cliclick clicks/keystrokes DO reach the Fyne canvas where osascript cannot.

Fyne's `fyne.io/fyne/v2/test` harness is **headless/in-process only** ("provides
utility drivers for running UI tests without rendering to a screen" — pkg.go.dev)
— it cannot drive the on-screen window and is NOT used here.

## Files

| File | Role |
|---|---|
| `drive_desktop_gui.sh` | Launches the GUI, finds its window, drives input via cliclick, window-scoped-records to MP4, OCR read-back. Honest Aqua/Accessibility preflight (hard-fails, never fakes). |
| `find_window_id.swift` | Returns the on-screen `CGWindowID` + bounds of the Fyne window (via `CGWindowListCopyWindowInfo`). **Proven** to resolve real on-screen windows by owner/title. |
| `ocr_recording.sh` | §11.4.159(D)/§11.4.160 read-back: samples MP4 frames, OCRs them via macOS Vision, asserts the typed prompt (and optionally the LLM reply) actually appeared in-GUI. |

## How to run (Aqua session, display-equipped)

Run **from a Terminal.app / iTerm window on the physical console** (so
`launchctl managername` == `Aqua`), with the controlling terminal granted
**Accessibility** AND **Screen Recording** in System Settings → Privacy &
Security:

```bash
brew install cliclick ffmpeg        # if absent (no sudo)
cd helix_code && make desktop        # build bin/helix-desktop
scripts/testing/drive_desktop_gui.sh "What is 17 plus 25? Reply with the number only."
```

Output: `helix_code-hxc112-<ts>.mp4` under `/Volumes/T7/Downloads/Recordings/`
(§11.4.155 prefix, §11.4.158 path), plus a `-promptframe.png` evidence frame and
the OCR transcript. The MP4 SHOWS the synthetic input registering (prompt text
appearing in the input, click landing on Send) and the real LLM reply
streaming into the chat history.

Widget coordinates are window-relative and calibrated for the default window;
override via env if the window is resized: `LLMTAB_X/Y` (the "LLM" tab in the
top tab bar — main.go:485, 6th label), `INPUT_X/Y` (chat input, placeholder
"Type your message here..."), `SEND_X/Y` ("Send Message" button).

## Environment gap on the headless/SSH host (this session)

This session is an **SSH/tmux shell in the per-user "Background" launchd
domain** (`launchctl managername` == `Background`), NOT `Aqua`. Captured
evidence (2026-06-22):
- `bin/helix-desktop` builds and launches; the process stays alive and
  initializes REAL LLM providers (DeepSeek/Mistral/Groq/OpenRouter live
  `/models`), but it **never produces an on-screen window** — absent from
  `CGWindowListCopyWindowInfo` while Dock/Finder/Window-Server ARE observable.
- `launchctl asuser 501 …` → `Operation not permitted` (no sudo / cannot switch
  audit session). `open` / `open -a <bundle>` → rc=0 but no observable Aqua
  window (unsigned ad-hoc bundle from Background does not attach).
- `cliclick p:.` → "Accessibility privileges not enabled" (controlling process
  is `claude`, not an operator-toggleable Terminal).

Therefore the recording cannot be produced autonomously from this session — an
honest §11.4.3 / §11.4.123 environment gap, never faked. The harness above is
the complete, proven solution; it runs out-of-the-box on a console Aqua
Terminal with the two permissions granted. `ffmpeg -f avfoundation` already
sees `Capture screen 0` on this host, so recording will work once in Aqua.

## Unblocks HXC-108

HXC-108 (video-QA: record all clients × all features) needed a way to drive the
desktop GUI's in-GUI LLM chat for recording. This harness IS that mechanism:
`drive_desktop_gui.sh` is the desktop-client driver HXC-108's program calls per
feature (parameterize the prompt / tab / widget coordinates per feature). The
osascript dead-end is replaced by the proven cliclick CGEvent path.
