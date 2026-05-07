# Phase 2 / Feature 27 — Aider Voice Input + Repo-Map

**Date:** 2026-05-07
**Status:** Approved
**Programme:** CLI-Agent Fusion — Phase 2 port (aider: voice input + repo-map)

> **Programme position:** F27 is the **seventh** Phase 2 feature (F21-F26 shipped).

---

## 1. Goal

Ship real voice input (speech-to-text) and tree-sitter-based repository mapping for the HelixCode CLI agent. Voice: arecord/sox capture → OpenAI Whisper API (with local whisper.cpp fallback). Repo-map: tree-sitter AST parsing for function/class/import declarations, cached per-commit SHA. Four tools: voice_start, voice_stop, voice_transcribe, repomap. Slash: /aider (voice/repomap subcommands).

---

## 2. Architecture

### 2.1 Voice Pipeline

```
  Microphone → arecord/sox → temp WAV → Whisper API → text
                                          ↓ (fallback)
                                      whisper.cpp → text
```

- `internal/voice/` (NEW) — VoiceRecorder (shell capture), VoiceTranscriber (Whisper API + local fallback)
- Audio format: 16kHz mono WAV via `arecord` (Linux) or `sox` (cross-platform)
- Temp files at `$XDG_RUNTIME_DIR/helixcode/voice/` (auto-cleaned)
- Transcription: OpenAI Whisper API (env: `OPENAI_API_KEY`) → local `whisper.cpp` CLI fallback

### 2.2 Repo-Map Pipeline

```
  Project dir → glob files → tree-sitter parse → extract defs → JSON map → cache
```

- `internal/repomap/` (NEW — extends existing `internal/repomap/` stub)
- AST parsing via `go-tree-sitter` (already indirect dep in go.sum)
- Map includes: functions, classes, imports, exports per file
- Cache keyed by `git rev-parse HEAD`, invalidated on commit
- Output format: structured JSON map

---

## 3. Components

| File | Purpose |
|------|---------|
| `internal/voice/types.go` | VoiceRecorder, VoiceTranscriber, config, sentinels |
| `internal/voice/recorder.go` | Shell-based audio capture (arecord/sox) |
| `internal/voice/transcriber.go` | Whisper API + whisper.cpp fallback |
| `internal/voice/voice_tools.go` | voice_start/stop/transcribe tools |
| `internal/repomap/mapper.go` | Tree-sitter AST parser + map builder |
| `internal/repomap/cache.go` | Per-commit map cache |
| `internal/repomap/repomap_tool.go` | repomap tool implementation |
| `internal/commands/aider_command.go` | /aider slash command |

---

## 4. Anti-Bluff Hot Zone

1. **Voice recorded but 0 bytes** — WAV file size > 44 bytes (header minimum)
2. **Transcription returns empty** — text output is non-empty
3. **Whisper API fallback silent** — fallback path is exercised when API key absent
4. **Repo-map returns empty for non-empty repo** — at least 1 function/class discovered
5. **Cache returns stale data after commit** — map differs pre/post commit

---

## 5. Task Breakdown

| # | Task | Description |
|---|------|------------|
| T01 | Bootstrap | F27 evidence + advance PROGRESS + CONTINUATION |
| T02 | voice/types.go + recorder.go | VoiceRecorder with arecord/sox (TDD) |
| T03 | voice/transcriber.go | Whisper API + whisper.cpp fallback (TDD) |
| T04 | voice/voice_tools.go | voice_start/stop/transcribe tools (TDD) |
| T05 | repomap/mapper.go + cache.go | Tree-sitter AST map + commit cache (TDD) |
| T06 | repomap/repomap_tool.go | repomap tool (TDD) |
| T07 | /aider slash | voice/repomap subcommands (TDD) |
| T08 | main.go wiring | Register tools + slash |
| T09 | Challenge harness + close-out | 6 phases + push 4 remotes |

---

*F27 spec — Aider Voice Input + Repo-Map.*
