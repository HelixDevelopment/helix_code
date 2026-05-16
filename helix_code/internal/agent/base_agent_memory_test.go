// Package agent — base_agent_memory_test.go (P2-F24-T07).
//
// Tests for the optional project-memory prepend in getSystemPrompt. The
// existing TestBaseAgentGetSystemPrompt in base_agent_extended_test.go
// constructs WITHOUT SetMemoryRegistry — those tests stay green
// byte-for-byte. These tests verify the new behaviour.
package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/projectmemory"
)

// stubSnapshotter implements projectmemory.MemorySnapshotter for tests.
// The mutable .m field lets a single test mutate it between getSystemPrompt
// calls and assert that the second call sees the new value (proves no
// construct-time caching).
type stubSnapshotter struct{ m projectmemory.Memory }

func (s *stubSnapshotter) Snapshot() projectmemory.Memory { return s.m }

func TestBaseAgent_GetSystemPrompt_NoMemoryRegistry_Unchanged(t *testing.T) {
	a := NewBaseAgent("id", "name", AgentTypeCoordinator, nil)
	out := a.getSystemPrompt()
	require.NotContains(t, out, "USER MEMORY OVERLAY")
	require.Contains(t, out, "You are a")
	require.Contains(t, out, "agent named name")
}

func TestBaseAgent_GetSystemPrompt_PrependsCurrentMemory(t *testing.T) {
	a := NewBaseAgent("id", "name", AgentTypeCoordinator, nil)
	s := &stubSnapshotter{m: projectmemory.Memory{Project: "PROJECT_FIXTURE_24"}}
	a.SetMemoryRegistry(s)
	out := a.getSystemPrompt()
	require.Contains(t, out, "PROJECT_FIXTURE_24")
	// Memory must come before the base prompt's signature line.
	require.Less(t, strings.Index(out, "PROJECT_FIXTURE_24"), strings.Index(out, "agent named name"))
}

func TestBaseAgent_GetSystemPrompt_LiveSnapshot_NotCached(t *testing.T) {
	// Anti-bluff: getSystemPrompt MUST read registry.Snapshot() per call.
	// Caching at construct time would mean editing helixcode.md mid-session
	// + fsnotify-reloading + /memory show new content all PASS, but the
	// next LLM call still sees the OLD blob — the canonical bluff (#3).
	a := NewBaseAgent("id", "name", AgentTypeCoordinator, nil)
	s := &stubSnapshotter{m: projectmemory.Memory{Project: "OLD_24"}}
	a.SetMemoryRegistry(s)
	require.Contains(t, a.getSystemPrompt(), "OLD_24")

	s.m = projectmemory.Memory{Project: "NEW_24"}
	out := a.getSystemPrompt()
	require.Contains(t, out, "NEW_24")
	require.NotContains(t, out, "OLD_24")
}

func TestBaseAgent_GetSystemPrompt_BothProjectAndUser_Prepended(t *testing.T) {
	a := NewBaseAgent("id", "name", AgentTypeCoordinator, nil)
	s := &stubSnapshotter{m: projectmemory.Memory{Project: "P_24", User: "U_24"}}
	a.SetMemoryRegistry(s)
	out := a.getSystemPrompt()
	require.Contains(t, out, "P_24")
	require.Contains(t, out, "U_24")
	require.Contains(t, out, "USER MEMORY OVERLAY")
	// Project before user before base.
	require.Less(t, strings.Index(out, "P_24"), strings.Index(out, "U_24"))
	require.Less(t, strings.Index(out, "U_24"), strings.Index(out, "agent named name"))
}

func TestBaseAgent_GetSystemPrompt_EmptyMemory_NoPrepend(t *testing.T) {
	// Memory is wired but currently empty — must not add a stray "---\n\n"
	// separator to the base prompt.
	a := NewBaseAgent("id", "name", AgentTypeCoordinator, nil)
	a.SetMemoryRegistry(&stubSnapshotter{m: projectmemory.Memory{}})
	out := a.getSystemPrompt()
	require.NotContains(t, out, "USER MEMORY OVERLAY")
	// Must still contain the base body.
	require.Contains(t, out, "agent named name")
}

func TestBaseAgent_SetMemoryRegistry_Nil_RestoresUnchanged(t *testing.T) {
	a := NewBaseAgent("id", "name", AgentTypeCoordinator, nil)
	a.SetMemoryRegistry(&stubSnapshotter{m: projectmemory.Memory{Project: "X_24"}})
	require.Contains(t, a.getSystemPrompt(), "X_24")
	a.SetMemoryRegistry(nil)
	require.NotContains(t, a.getSystemPrompt(), "X_24")
}
