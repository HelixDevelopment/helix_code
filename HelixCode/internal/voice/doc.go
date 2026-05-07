// Package voice provides speech-to-text voice input for the HelixCode CLI agent.
// Audio capture via arecord/sox, transcription via OpenAI Whisper API with
// local whisper.cpp fallback. Implements the aider voice input port (P2-F27).
//
// Spec: docs/superpowers/specs/2026-05-07-p2-f27-aider-voice-repomap-design.md
// Plan: docs/superpowers/plans/2026-05-07-p2-f27-aider-voice-repomap.md
package voice
