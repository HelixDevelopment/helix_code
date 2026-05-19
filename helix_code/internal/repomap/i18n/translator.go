// Package i18n declares internal/repomap's hardcoded-content
// abstraction per CONST-046 (round-198 §11.4 anti-bluff sweep,
// 2026-05-19; recovery from round-174 stall in which the scaffolding
// was dispatched but never written). Mirrors the "consumer defines
// its own Translator interface" pattern of rounds 93..182.
//
// Round-198 scope: ONE real migration target identified — the
// RepoMapTool.Description() string ("Generate a semantic map of the
// codebase using tree-sitter") emitted to end users via the tool
// registry. This is a user-facing narrative descriptor, not an
// identity token, so it is in scope for CONST-046 migration. The
// other strings in internal/repomap/ are either:
//
//   * Log framing strings (log.Printf format strings in cache.go —
//     debug/warn output, not user-facing UI per the round-162
//     logging-identifier-token precedent).
//
//   * Tool category identifiers (e.g. "mapping" in repomap_tool.go)
//     — structural lookup keys, never adapted per-locale per the
//     round-158/162 identifier-token precedent.
//
//   * doc.go comments — Go package documentation comments, never
//     emitted at runtime; rendered by godoc/pkgsite which is not
//     i18n-aware.
//
// Naming convention: internal_repomap_<area>_<short_description>.
// All IDs prefixed with internal_repomap_ to avoid namespace
// collision when loaded into the same go-i18n.Bundle alongside
// other packages' entries.
package i18n

import "context"

// Translator is the contract internal/repomap uses for every
// CONST-046-migrated user-facing string. Mirrors the verifier /
// hardware / logging / watcher precedents.
type Translator interface {
	// T resolves a singular message ID against the active locale,
	// optionally interpolating templateData fields. Returns the raw
	// message ID on lookup failure so production output remains loud +
	// obvious instead of silently empty.
	T(ctx context.Context, messageID string, templateData map[string]any) (string, error)

	// TPlural resolves a plural message ID against the active locale
	// with count-aware variant selection.
	TPlural(ctx context.Context, messageID string, count int, templateData map[string]any) (string, error)
}

// NoopTranslator returns the messageID verbatim. SAFETY default for
// unit tests + backward-compat for callers who have not yet wired a
// real Translator. Production paths MUST inject a real Translator
// (helix_code wires *i18nadapter.Translator at boot).
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}

// TPlural returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}
