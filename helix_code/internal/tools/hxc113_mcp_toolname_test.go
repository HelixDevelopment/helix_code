package tools

import (
	"regexp"
	"strings"
	"testing"
)

// hxc113_mcp_toolname_test.go — regression guard (§11.4.135) for HXC-113:
// MCP tools were registered as "server:name" (colon), which OpenAI-compatible
// providers (DeepSeek, OpenAI, Groq, Mistral, …) reject — the function name
// must match ^[A-Za-z0-9_-]+$ — returning HTTP 400 and breaking LLM chat
// whenever any MCP tool was registered. mcpToolRegisteredName now sanitises
// the name to be provider-compatible.
func TestMCPToolRegisteredName_OpenAICompatible(t *testing.T) {
	re := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

	// RED anchor: the OLD "server:name" join is NOT provider-compatible — prove
	// the constraint the fix must satisfy (the colon is exactly what broke chat).
	if re.MatchString("fs:read_file") {
		t.Fatal("guard invalid: the old colon-joined name must fail ^[A-Za-z0-9_-]+$")
	}

	cases := []struct{ server, tool string }{
		{"fs", "read_file"},
		{"git", "status"},
		{"my server", "weird:name"},   // spaces + colon in inputs
		{"a.b", "c/d"},                 // dots + slash
		{"filesystem", "list_directory"},
	}
	for _, c := range cases {
		got := mcpToolRegisteredName(c.server, c.tool)
		if !re.MatchString(got) {
			t.Fatalf("mcpToolRegisteredName(%q,%q)=%q is NOT OpenAI-compatible (must match ^[A-Za-z0-9_-]+$)", c.server, c.tool, got)
		}
		if strings.ContainsAny(got, ":/. ") {
			t.Fatalf("name %q still contains a provider-rejected character (HXC-113)", got)
		}
		if got == "" {
			t.Fatalf("mcpToolRegisteredName(%q,%q) returned empty", c.server, c.tool)
		}
	}
}
