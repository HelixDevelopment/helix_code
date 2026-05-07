# P2-F27 — Aider Voice Input + Repo-Map Implementation Plan

> **Programme position:** F27 is the **seventh** Phase 2 feature (after F21-F26).

**Goal:** Ship real voice input (speech-to-text) and tree-sitter repository mapping. 4 tools + /aider slash. New packages: `internal/voice/`, extends `internal/repomap/`.

**Spec:** `docs/superpowers/specs/2026-05-07-p2-f27-aider-voice-repomap-design.md`
**Q1-Q5:** Aider / Core / Hybrid Whisper / Tree-sitter / 4 tools + slash

**Zero new external deps** — go-tree-sitter already indirect, whisper.cpp is CLI binary not Go dep.

---

## Task list

- [ ] P2-F27-T01 — bootstrap F27 evidence + advance PROGRESS + CONTINUATION
- [ ] P2-F27-T02 — `internal/voice/types.go` + `recorder.go`: VoiceRecorder with arecord/sox (TDD)
- [ ] P2-F27-T03 — `internal/voice/transcriber.go`: Whisper API + whisper.cpp (TDD)
- [ ] P2-F27-T04 — `internal/voice/voice_tools.go`: voice_start/stop/transcribe (TDD)
- [ ] P2-F27-T05 — `internal/repomap/mapper.go` + `cache.go`: tree-sitter AST map (TDD)
- [ ] P2-F27-T06 — `internal/repomap/repomap_tool.go`: repomap tool (TDD)
- [ ] P2-F27-T07 — `/aider` slash command (TDD)
- [ ] P2-F27-T08 — main.go wiring + integration tests
- [ ] P2-F27-T09 — Challenge harness 6 phases + close-out + push 4 remotes

---

## Task 1: Bootstrap F27

Advance PROGRESS.md to "F27 in flight". Update CONTINUATION.md F27 mid-flight section.
Append evidence header.

## Task 2: voice/types.go + recorder.go

- `VoiceRecorder` with `Start(outputPath string)`, `Stop()`, `IsRecording() bool`
- Shell: `arecord -f cd -t wav <path>` (Linux) with `sox` fallback
- Subprocess management via `os/exec`, signal-based graceful stop
- WAV validation: check RIFF header, non-zero data chunk
- Tests: mock recorder for unit tests, real recorder gated (`SKIP-OK`)

## Task 3: voice/transcriber.go

- `VoiceTranscriber` with `Transcribe(audioPath string) (string, error)`
- Primary: OpenAI Whisper API (`POST /v1/audio/transcriptions`)
- Fallback: `whisper.cpp` CLI (`whisper-cli -m <model> -f <wav>`)
- Env vars: `OPENAI_API_KEY` for API, `HELIXCODE_WHISPER_MODEL` for local model path
- Tests: mock HTTP server for API, mock CLI for fallback

## Task 4: voice_tools.go

- `voice_start`: starts recording to temp WAV
- `voice_stop`: stops recording, returns path + duration
- `voice_transcribe`: transcribes stopped recording, returns text
- Category: `voice`. RequiresApproval: transcribe→LevelEdit, start/stop→LevelReadOnly

## Task 5: repomap/mapper.go + cache.go

- Extend existing `internal/repomap/` package
- Tree-sitter parser: for each source file, parse AST, extract function/class/import nodes
- Map struct: `RepoMap{Files []FileMap}` where `FileMap{Path, Functions, Classes, Imports}`
- Cache: `Cache{mu, entries map[string]*RepoMap}` keyed by `git rev-parse HEAD`
- Tests with real tempdir repos containing Go/Python/JS files

## Task 6: repomap_tool.go

- `repomap` tool: scans project dir, returns structured map as JSON
- Category: `mapping`. RequiresApproval: LevelReadOnly

## Task 7: /aider slash

- `/aider voice [start|stop|transcribe]` — voice control
- `/aider repomap [generate|show|clear]` — repo-map control
- Aliases: `/aider`, `/ai`

## Task 8: main.go wiring

Register 4 tools + /aider slash in main.go startup.

## Task 9: Challenge harness

6 phases:
- A: Voice record (gated on microphone) → WAV > 44 bytes
- B: Voice transcribe (gated on API key/model) → non-empty text
- C: Repo-map on tempdir with Go files → finds functions
- D: Repo-map cache → same commit returns cached, new commit invalidates
- E: Hybrid fallback → API absent exercises whisper.cpp path
- F: /aider slash subcommands → correct routing

---

*Plan written. Execute via TDD starting with T01.*
