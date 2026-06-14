package main

import (
	"fmt"
	"sort"
	"strings"
)

// ensemble_render.go — PURE display-formatting helpers for the terminal UI.
//
// These functions perform NO I/O, NO network, and depend on NO TUI widget
// types. They take plain data (the ensemble ProviderMetadata map produced by
// internal/llm/ensemble_provider.go, and a lightweight tool-trace slice) and
// return []string display lines that a caller renders however it likes. Keeping
// them pure makes the multi-member visibility (the operator SEEING every
// ensemble member's answer) and the agentic tool-trace deterministically
// testable.
//
// The labels here are minimal structural diagnostics composed from metadata
// (e.g. "ensemble: 4/4 members", "[winner] Groq") — not locale-sensitive prose
// — so they stay CONST-046-safe.

// ToolTraceLine is a local, decoupled view of one agentic tool-call step. The
// caller adapts whatever the agent loop produces (e.g. agent.ToolTraceEntry)
// into this struct, so this package never imports internal/agent.
type ToolTraceLine struct {
	ToolName  string
	Output    string
	Err       string
	Arguments map[string]interface{}
}

// metaInt reads an integer-ish metadata value defensively: it may arrive as an
// int (in-process call) or a float64 (after a trip through JSON or an
// interface{} channel). Returns (value, true) when present and numeric.
func metaInt(meta map[string]interface{}, key string) (int, bool) {
	v, ok := meta[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	}
	return 0, false
}

// metaString reads a string metadata value; returns "" when absent/wrong type.
func metaString(meta map[string]interface{}, key string) string {
	if s, ok := meta[key].(string); ok {
		return s
	}
	return ""
}

// metaStringSlice reads a []string participant list defensively, tolerating the
// []interface{} shape produced after a JSON round-trip.
func metaStringSlice(meta map[string]interface{}, key string) []string {
	switch raw := meta[key].(type) {
	case []string:
		out := make([]string, len(raw))
		copy(out, raw)
		return out
	case []interface{}:
		out := make([]string, 0, len(raw))
		for _, e := range raw {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// metaFloatMap reads a map[string]float64 (scores) defensively, tolerating the
// map[string]interface{} shape produced after a JSON round-trip.
func metaFloatMap(meta map[string]interface{}, key string) map[string]float64 {
	switch raw := meta[key].(type) {
	case map[string]float64:
		out := make(map[string]float64, len(raw))
		for k, v := range raw {
			out[k] = v
		}
		return out
	case map[string]interface{}:
		out := make(map[string]float64, len(raw))
		for k, v := range raw {
			switch n := v.(type) {
			case float64:
				out[k] = n
			case float32:
				out[k] = float64(n)
			case int:
				out[k] = float64(n)
			case int64:
				out[k] = float64(n)
			}
		}
		return out
	}
	return nil
}

// metaStringMap reads a map[string]string (excerpts) defensively, tolerating
// the map[string]interface{} shape produced after a JSON round-trip.
func metaStringMap(meta map[string]interface{}, key string) map[string]string {
	switch raw := meta[key].(type) {
	case map[string]string:
		out := make(map[string]string, len(raw))
		for k, v := range raw {
			out[k] = v
		}
		return out
	case map[string]interface{}:
		out := make(map[string]string, len(raw))
		for k, v := range raw {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
		return out
	}
	return nil
}

// isEnsemble reports whether meta carries a positive "ensemble" flag.
func isEnsemble(meta map[string]interface{}) bool {
	b, ok := meta["ensemble"].(bool)
	return ok && b
}

// FormatEnsemblePanel renders the multi-provider ensemble's per-member answers,
// scores, and the winning vote into display lines. It returns an EMPTY slice for
// any non-ensemble metadata (nil, "ensemble" absent, or "ensemble"==false) so
// the caller renders nothing special for ordinary single-provider responses.
//
// For a real ensemble response the panel lists EVERY participant with its score,
// its answer excerpt, and a clear [winner] marker on the selected provider, plus
// a "successful/total members" count header — making every member's response
// visible to the operator.
func FormatEnsemblePanel(meta map[string]interface{}) []string {
	if meta == nil || !isEnsemble(meta) {
		return []string{}
	}

	total, _ := metaInt(meta, "ensemble_total_providers")
	successful, _ := metaInt(meta, "ensemble_successful_providers")
	strategy := metaString(meta, "ensemble_strategy")
	selected := metaString(meta, "ensemble_selected_provider")
	participants := metaStringSlice(meta, "ensemble_participants")
	scores := metaFloatMap(meta, "ensemble_scores")
	excerpts := metaStringMap(meta, "ensemble_excerpts")

	// Determine the iteration order: prefer the (already-sorted) participant
	// list; otherwise derive a stable order from whatever keys we do have.
	order := participants
	if len(order) == 0 {
		seen := map[string]bool{}
		for k := range scores {
			if !seen[k] {
				order = append(order, k)
				seen[k] = true
			}
		}
		for k := range excerpts {
			if !seen[k] {
				order = append(order, k)
				seen[k] = true
			}
		}
		sort.Strings(order)
	}

	lines := make([]string, 0, len(order)+2)

	header := fmt.Sprintf("ensemble: %d/%d members", successful, total)
	if strategy != "" {
		header += " (strategy: " + strategy + ")"
	}
	lines = append(lines, header)

	for _, name := range order {
		marker := ""
		if name == selected && selected != "" {
			marker = "[winner] "
		}
		score, hasScore := scores[name]
		var head string
		if hasScore {
			head = fmt.Sprintf("  %s%s  score=%.2f", marker, name, score)
		} else {
			head = fmt.Sprintf("  %s%s", marker, name)
		}
		lines = append(lines, head)
		if ex := strings.TrimSpace(excerpts[name]); ex != "" {
			lines = append(lines, "    "+ex)
		}
	}

	return lines
}

// FormatToolTrace renders an agentic tool-call trace into display lines. Each
// entry shows the tool name, its arguments, and either the real output or the
// error it surfaced. Returns an empty slice for an empty trace.
func FormatToolTrace(entries []ToolTraceLine) []string {
	if len(entries) == 0 {
		return []string{}
	}

	lines := make([]string, 0, len(entries)*3)
	for i, e := range entries {
		header := fmt.Sprintf("[%d] tool: %s", i+1, e.ToolName)
		if args := formatArgs(e.Arguments); args != "" {
			header += "  args: " + args
		}
		lines = append(lines, header)

		if strings.TrimSpace(e.Err) != "" {
			lines = append(lines, "    error: "+e.Err)
		}
		if out := strings.TrimRight(e.Output, "\n"); out != "" {
			for _, ol := range strings.Split(out, "\n") {
				lines = append(lines, "    "+ol)
			}
		}
	}
	return lines
}

// formatArgs renders a tool's arguments deterministically (sorted by key) as a
// compact key=value list.
func formatArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, args[k]))
	}
	return strings.Join(parts, " ")
}
