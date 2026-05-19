// Package i18n declares internal/projectmemory's hardcoded-content
// abstraction per CONST-046 (round-235 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..226 (most recently
// internal/voice 226, internal/secrets 225, internal/render 224,
// internal/quality 223, internal/clarification 222, internal/approval
// 221).
//
// Wire-in path at boot: consuming binary builds an *i18n.Localizer
// (loaded with active.en.yaml from internal/projectmemory/i18n/bundles),
// wraps it in *i18nadapter.Translator, stores via
// projectmemory.SetTranslator. The package-level tr() helper falls
// back to NoopTranslator{} when not wired — loud message-ID echo,
// never silent swallow (which would be a §11.4 PASS-bluff at the
// i18n layer).
//
// Round-235 status: NO-OP INFRA — the audit gate
// (scripts/audit-const046-hardcoded-content.sh) reports zero
// hardcoded-content violations in internal/projectmemory/ at HEAD.
// The package's surface is entirely structural:
//
//   - Discovery filename tokens ("helixcode.md", "codex.md",
//     "AGENTS.md" in DiscoveryFilenames) — filesystem identifiers
//     consumed by os.Stat / case-insensitive comparison, not
//     user-display content. Translating them would break the
//     parent-walk lookup contract.
//
//   - Path-suffix tokens ("helixcode", "memory.md") used to build
//     $XDG_CONFIG_HOME/helixcode/memory.md — filesystem path
//     fragments, not translatable.
//
//   - Memory.Render() concatenation delimiter
//     "\n\n--- USER MEMORY OVERLAY ---\n\n" — load-bearing protocol
//     marker pinned by tests (per types.go:106 contract) and consumed
//     by LLM-side parsing to distinguish project- vs user-memory
//     source. Identity marker, NOT user-narrative; round-158
//     identifier-token precedent applies.
//
//   - Wrapped-error technical strings ("projectmemory: read %s: %w",
//     "projectmemory: resolve %s: %w", "projectmemory: registry has
//     no loader", "projectmemory: no memory file found",
//     "projectmemory: memory file exceeds MaxMemoryBytes") — surfaced
//     to developer logs / error chains, not end-user UI narrative.
//
//   - WARN-log messages ("project memory file truncated", "user
//     memory file truncated", "projectmemory: fsnotify new watcher
//     failed; degrading to slash-only reload", "projectmemory:
//     fsnotify add failed", "projectmemory: reload after fsnotify
//     failed") — zap.Logger.Warn surface for ops / debug, not end-user
//     content per CONST-042 (logger receives paths and byte counts,
//     never content body).
//
// Per round-158's hardware identifier-token precedent, identity keys
// consumed by downstream parsers / equality checks are explicitly OUT
// of CONST-046 scope. Translating the Memory.Render delimiter or
// DiscoveryFilenames tokens would silently rewrite the contract with
// the LLM system-prompt prepend pipeline (F24 P2-F24-T07) and the
// fsnotify watch-target pipeline (P2-F24-T05).
//
// The scaffold lands now so any FUTURE user-facing string added to
// internal/projectmemory/ (e.g. a /memory status TUI banner, a CLI
// summary line surfacing TruncatedProject / TruncatedUser flags to
// end users, a /memory edit guidance line) inherits the standard
// migration pattern without further infra work.
package i18n

import "context"

// Translator is the contract internal/projectmemory uses for every
// CONST-046-migrated user-facing string.
type Translator interface {
	// T resolves messageID against the active locale. templateData
	// supplies named placeholders for go-i18n style interpolation;
	// pass nil when the message has no placeholders.
	T(ctx context.Context, messageID string, templateData map[string]any) (string, error)

	// TPlural resolves messageID with plural-form selection driven
	// by count. templateData carries any non-count placeholders.
	TPlural(ctx context.Context, messageID string, count int, templateData map[string]any) (string, error)
}

// NoopTranslator returns the messageID verbatim. SAFETY default for
// unit tests within this package + backward-compat for callers who
// have not yet wired a real Translator. Production paths MUST inject
// a real Translator (helix_code wires *i18nadapter.Translator at
// boot).
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}

// TPlural returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}
