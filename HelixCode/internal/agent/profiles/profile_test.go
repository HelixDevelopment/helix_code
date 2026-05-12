// profile_test.go — TDD tests for the verifier profile feature.
//
// These tests are written BEFORE the implementation and assert
// end-user-observable behaviour per CONST-035 / Article XI §11.9:
//   - the verifier profile EXISTS and is selectable by canonical name,
//   - its system prompt actually instructs the LLM to review/verify
//     (a hardcoded marker would be a bluff; we check semantic keywords),
//   - its tool filter REJECTS write-class tools by name and ACCEPTS
//     read-class tools (visible behaviour: FilterTools returns the
//     filtered slice that downstream code will register),
//   - its temperature preference is the deterministic 0.1 the spec
//     names (consumed by spawner LLMRequest construction),
//   - role->profile resolution is wired (RoleVerify → verifier profile).
package profiles_test

import (
	"strings"
	"testing"

	"dev.helix.code/internal/agent/profiles"
	"dev.helix.code/internal/agent/subagent"
)

// fakeTool is a minimal stand-in for the production tools.Tool interface
// that exposes only the two attributes the profile filter cares about:
// a stable name + a read-only flag. Production tool registration code
// will adapt its tools through profiles.ToolDescriptor; the filter logic
// itself is exercised here against these fakes so the test does not
// import the entire tools package graph.
type fakeTool struct {
	name     string
	readOnly bool
}

func (f fakeTool) ToolName() string { return f.name }
func (f fakeTool) IsReadOnly() bool { return f.readOnly }

// TestVerifierProfile_ExistsByName asserts the built-in verifier profile
// is registered under its canonical name and can be retrieved.
func TestVerifierProfile_ExistsByName(t *testing.T) {
	p, ok := profiles.Get(profiles.NameVerifier)
	if !ok {
		t.Fatalf("profiles.Get(NameVerifier) returned not-found; expected built-in profile to be registered")
	}
	if p.Name != profiles.NameVerifier {
		t.Fatalf("profile.Name = %q, want %q", p.Name, profiles.NameVerifier)
	}
}

// TestVerifierProfile_SystemPromptMentionsReviewOrVerify asserts the
// LLM-facing system prompt instructs the model to act as a reviewer/
// verifier. End-user observable: the LLM receives a prompt that
// actually shapes its behaviour. We check for either "review" or
// "verify" (case-insensitive) to allow phrasing flexibility but block
// the empty / bluff prompt.
func TestVerifierProfile_SystemPromptMentionsReviewOrVerify(t *testing.T) {
	p, ok := profiles.Get(profiles.NameVerifier)
	if !ok {
		t.Fatalf("verifier profile missing")
	}
	if strings.TrimSpace(p.SystemPrompt) == "" {
		t.Fatalf("verifier profile SystemPrompt is empty; LLM would receive no instructions")
	}
	lower := strings.ToLower(p.SystemPrompt)
	if !strings.Contains(lower, "review") && !strings.Contains(lower, "verify") {
		t.Fatalf("verifier SystemPrompt missing review/verify keyword; got: %q", p.SystemPrompt)
	}
}

// TestVerifierProfile_DeterministicTemperature asserts the profile
// requests low-temperature output (deterministic). End-user observable:
// the spawner passes Temperature 0.1 into LLMRequest, which actually
// changes provider behaviour.
func TestVerifierProfile_DeterministicTemperature(t *testing.T) {
	p, ok := profiles.Get(profiles.NameVerifier)
	if !ok {
		t.Fatalf("verifier profile missing")
	}
	const want = 0.1
	if p.Temperature != want {
		t.Fatalf("verifier Temperature = %v, want %v", p.Temperature, want)
	}
}

// TestVerifierProfile_FilterToolsRejectsWriteTools asserts FilterTools
// strips write-side tools when applied to a mixed list. This is the
// END-USER-OBSERVABLE guarantee that a verify-role subagent CAN NOT
// modify state: write tools never reach the registry.
func TestVerifierProfile_FilterToolsRejectsWriteTools(t *testing.T) {
	p, ok := profiles.Get(profiles.NameVerifier)
	if !ok {
		t.Fatalf("verifier profile missing")
	}
	tools := []profiles.ToolDescriptor{
		fakeTool{name: "fs_read", readOnly: true},
		fakeTool{name: "grep", readOnly: true},
		fakeTool{name: "lsp_definitions", readOnly: true},
		fakeTool{name: "web_fetch", readOnly: true},
		fakeTool{name: "fs_write", readOnly: false},
		fakeTool{name: "multiedit", readOnly: false},
		fakeTool{name: "shell", readOnly: false},
	}
	filtered := p.FilterTools(tools)

	// Build the set of allowed names for assertion.
	got := make(map[string]bool, len(filtered))
	for _, t := range filtered {
		got[t.ToolName()] = true
	}

	wantAllowed := []string{"fs_read", "grep", "lsp_definitions", "web_fetch"}
	for _, n := range wantAllowed {
		if !got[n] {
			t.Errorf("FilterTools dropped expected read-only tool %q; filtered=%v", n, got)
		}
	}
	wantRejected := []string{"fs_write", "multiedit", "shell"}
	for _, n := range wantRejected {
		if got[n] {
			t.Errorf("FilterTools KEPT write-side tool %q; verifier subagents MUST NOT have write tools (CONST-035)", n)
		}
	}
}

// TestProfileForRole_VerifyReturnsVerifier asserts the role->profile
// resolution wires RoleVerify to the verifier profile. End-user
// observable: dispatching with Role=RoleVerify activates the verifier
// posture without the caller having to name the profile explicitly.
func TestProfileForRole_VerifyReturnsVerifier(t *testing.T) {
	p, ok := profiles.ForRole(subagent.RoleVerify)
	if !ok {
		t.Fatalf("ForRole(RoleVerify) returned not-found")
	}
	if p.Name != profiles.NameVerifier {
		t.Fatalf("ForRole(RoleVerify).Name = %q, want %q", p.Name, profiles.NameVerifier)
	}
}

// TestProfileForRole_OtherRolesDoNotForceVerifier asserts non-verify
// roles do NOT collapse onto the verifier profile (would break the
// implement/explore postures).
func TestProfileForRole_OtherRolesDoNotForceVerifier(t *testing.T) {
	cases := []subagent.Role{
		subagent.RoleGeneral,
		subagent.RoleExplore,
		subagent.RoleImplement,
	}
	for _, r := range cases {
		t.Run(string(r), func(t *testing.T) {
			p, ok := profiles.ForRole(r)
			if ok && p.Name == profiles.NameVerifier {
				t.Fatalf("ForRole(%q) returned verifier profile; only RoleVerify should map to verifier", r)
			}
		})
	}
}
