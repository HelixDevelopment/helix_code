package clientcore

import (
	"strings"
	"testing"

	"dev.helix.code/internal/agent"
	"dev.helix.code/internal/approval"
)

// TestWireAgenticTools_ReadOnlyCore proves the shared agentic wiring produces a
// registry containing the read-only core tools (git_status + fs_read/glob/grep)
// — the exact capability the desktop + TUI both reuse (§11.4.74). It also proves
// the read-only core tools report LevelReadOnly so the ReadOnlyOnly tool loop
// reaches nothing destructive (§11.4.133).
func TestWireAgenticTools_ReadOnlyCore(t *testing.T) {
	at, err := WireAgenticTools(".helixcode/mcp.yml")
	if err != nil {
		t.Fatalf("WireAgenticTools: %v", err)
	}
	if at == nil || at.Registry == nil {
		t.Fatalf("WireAgenticTools returned nil registry")
	}
	defer at.Close()

	levels := map[string]approval.ApprovalLevel{}
	for _, tool := range at.Registry.List() {
		levels[tool.Name()] = tool.RequiresApproval()
	}
	for _, want := range []string{"git_status", "fs_read", "glob", "grep"} {
		lvl, ok := levels[want]
		if !ok {
			t.Errorf("read-only tool %q not wired", want)
			continue
		}
		if lvl != approval.LevelReadOnly {
			t.Errorf("tool %q should be LevelReadOnly, got %v", want, lvl)
		}
	}
}

// TestBuildToolLoopSystemPrompt_NamesLiveTools proves the system prompt is
// composed from the LIVE registry tool names (CONST-046 structural, not a
// hardcoded literal) — a model can never claim "I cannot see your files".
func TestBuildToolLoopSystemPrompt_NamesLiveTools(t *testing.T) {
	at, err := WireAgenticTools(".helixcode/mcp.yml")
	if err != nil {
		t.Fatalf("WireAgenticTools: %v", err)
	}
	defer at.Close()

	prompt := BuildToolLoopSystemPrompt(at.Registry)
	for _, want := range []string{"git_status", "glob", "Helix coding agent", "Prefer calling a tool"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("system prompt missing %q\nprompt: %s", want, prompt)
		}
	}
}

// TestAdaptToolTrace_PreservesFields proves the agent-loop trace entries are
// adapted 1:1 into the shared ensembleui view (the bridge that lets every client
// render the SAME tool trace).
func TestAdaptToolTrace_PreservesFields(t *testing.T) {
	in := []agent.ToolTraceEntry{
		{ToolName: "git_status", Output: "clean", Err: "", Arguments: map[string]interface{}{"dir": "."}},
		{ToolName: "fs_read", Output: "", Err: "no such file", Arguments: map[string]interface{}{"path": "/nope"}},
	}
	out := AdaptToolTrace(in)
	if len(out) != len(in) {
		t.Fatalf("adapt length mismatch: got %d want %d", len(out), len(in))
	}
	if out[0].ToolName != "git_status" || out[0].Output != "clean" {
		t.Errorf("entry 0 not preserved: %+v", out[0])
	}
	if out[1].Err != "no such file" || out[1].Arguments["path"] != "/nope" {
		t.Errorf("entry 1 not preserved: %+v", out[1])
	}
}
