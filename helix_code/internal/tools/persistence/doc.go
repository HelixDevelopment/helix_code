// Package persistence implements claude-code-style tool-result persistence.
//
// When a tool's output exceeds PersistThreshold characters, the runtime
// writes the raw content to <projectRoot>/.helix/tool-results/ and the
// LLM payload carries a path-reference instead of inline content. The
// LLM reads back via the existing Read tool. A 7-day age-based sweep
// runs lazily at startup via Manager.CleanupOld.
//
// See: docs/superpowers/specs/2026-05-05-p1-f03-tool-result-persistence-design.md
package persistence
