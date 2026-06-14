package main

// ensemble_render.go — TUI-local thin re-export of the PROMOTED, shared
// ensemble/tool-trace rendering helpers.
//
// The pure display-formatting logic was promoted out of this package (where it
// was un-importable package-main code) into internal/ensembleui so BOTH the
// terminal UI and the desktop GUI render the IDENTICAL ensemble panel + tool
// trace (§11.4.74 reuse-over-reimplement; CONST-051(B) decoupling). This file
// keeps the TUI's existing call-sites (FormatEnsemblePanel, FormatToolTrace,
// ToolTraceLine) compiling unchanged by aliasing them onto the shared package —
// a zero-behaviour-change repoint, proven by internal/ensembleui's own tests.

import "dev.helix.code/internal/ensembleui"

// ToolTraceLine aliases the shared decoupled tool-trace view so the TUI's
// adaptToolTrace and FormatToolTrace call-sites are unchanged.
type ToolTraceLine = ensembleui.ToolTraceLine

// FormatEnsemblePanel re-exports the shared ensemble panel formatter.
func FormatEnsemblePanel(meta map[string]interface{}) []string {
	return ensembleui.FormatEnsemblePanel(meta)
}

// FormatToolTrace re-exports the shared tool-trace formatter.
func FormatToolTrace(entries []ToolTraceLine) []string {
	return ensembleui.FormatToolTrace(entries)
}
