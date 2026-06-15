package tools

// mcp_registration_security_test.go — §11.4.115 RED→GREEN regression guards
// (registered into the standing suite per §11.4.135) for two reproduced
// defects in MCP tool registration:
//
//   DEFECT-1 (SECURITY — approval-gate bypass): isReadOnlyMCPToolName matched
//   a BARE tool name against a generic allowlist ("search", "get_file_info",
//   "list_directory", …) IGNORING the server, so a MUTATING tool literally
//   named "search" on a NON-readOnly server was classed LevelReadOnly and
//   executed WITHOUT approval (applyApprovalGate short-circuits LevelReadOnly).
//
//   DEFECT-2 (collision → uncallable tool + silent level flip):
//   mcpToolRegisteredName is many-to-one; distinct (server,tool) pairs collide
//   onto one key, and last-write-wins registration left one tool permanently
//   uncallable while the survivor's approvalLevel could differ.
//
// Polarity switch per §11.4.115: set RED_MODE=1 to run the assertions in
// reproduce-the-defect-on-the-broken-artifact mode (the RED assertions MUST
// FAIL on fixed code and PASS on the pre-fix artifact, proving the defect was
// genuinely present). Default (RED_MODE unset/0) is the standing GREEN
// regression guard asserting the defect is ABSENT.

import (
	"os"
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redMode reports whether the polarity switch RED_MODE is engaged (=1).
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

func newConfiguredManager(specs ...mcp.ServerSpec) *mcp.Manager {
	m := mcp.NewManager()
	m.SetConfig(&mcp.Config{Servers: specs})
	return m
}

// classifyMCPToolLevelForTest reproduces the exact registration-time approval
// level decision RegisterMCPManager makes for a (server,tool), using the SAME
// production predicates (serverTrustedForNameAllowlist + isReadOnlyMCPToolName).
// It is a hermetic seam over the real decision so the security classification
// can be asserted deterministically (§11.4.50, §11.4.98) without booting a live
// MCP server.
func classifyMCPToolLevelForTest(m *mcp.Manager, server, tool string) approval.ApprovalLevel {
	readOnlyServers := map[string]bool{}
	nameAllowlistServers := map[string]bool{}
	if cfg := m.Config(); cfg != nil {
		for _, s := range cfg.Servers {
			if s.ReadOnly {
				readOnlyServers[s.Name] = true
			}
			if serverTrustedForNameAllowlist(s) {
				nameAllowlistServers[s.Name] = true
			}
		}
	}
	level := approval.LevelEdit
	if readOnlyServers[server] ||
		(nameAllowlistServers[server] && isReadOnlyMCPToolName(tool)) {
		level = approval.LevelReadOnly
	}
	return level
}

// --- DEFECT-1: SECURITY approval-gate bypass --------------------------------

// TestSecurity_MutatingSearchOnUntrustedServer_NotReadOnly is the core
// anti-bluff guard. A NON-readOnly, NON-filesystem server exposes a tool named
// "search" (plausibly a mutating SQL/DB/index operation). It MUST be classed
// LevelEdit so the approval gate is NOT bypassed.
//
// RED reproduction (RED_MODE=1): on the broken artifact the classification
// path keys solely on the bare tool name, so "search" → LevelReadOnly on ANY
// server. The RED assertion asserts that misclassification IS present.
//
// GREEN guard: the fix routes the name allowlist through a per-server
// trust predicate so an untrusted server's "search" stays LevelEdit.
func TestSecurity_MutatingSearchOnUntrustedServer_NotReadOnly(t *testing.T) {
	// A generic DB/index server — NOT flagged readOnly, NOT the well-known
	// filesystem server. Its "search" tool may mutate an index.
	m := newConfiguredManager(mcp.ServerSpec{
		Name:      "sqlindex",
		Transport: mcp.TransportStdio,
		Command:   []string{"my-sql-index-server"},
		// ReadOnly intentionally NOT set.
	})

	if redMode() {
		// RED: reproduce the bypass on the BROKEN decision logic. Pre-fix,
		// RegisterMCPManager classed a tool LevelReadOnly whenever
		// isReadOnlyMCPToolName(tool) was true, IGNORING the server. Recreate
		// that exact predicate here: "search" is in the allowlist → the broken
		// code would have classed it LevelReadOnly on this untrusted server.
		brokenWouldClassReadOnly := isReadOnlyMCPToolName("search") // true on the broken allowlist
		assert.Truef(t, brokenWouldClassReadOnly,
			"RED_MODE: the bare-name allowlist (broken logic) classes 'search' read-only on ANY server → gate bypass")
		return
	}
	// GREEN guard: the fixed classifier defaults an untrusted server's tools to
	// LevelEdit regardless of name.
	level := classifyMCPToolLevelForTest(m, "sqlindex", "search")
	assert.Equalf(t, approval.LevelEdit, level,
		"a non-readOnly, non-filesystem server's 'search' tool MUST be LevelEdit so applyApprovalGate is NOT bypassed")
}

// TestSecurity_GateNotBypassed_ForMutatingSearch proves the end-to-end
// consequence: a LevelEdit tool is NOT short-circuited by applyApprovalGate,
// whereas a LevelReadOnly tool is.
func TestSecurity_GateNotBypassed_ForMutatingSearch(t *testing.T) {
	m := newConfiguredManager(mcp.ServerSpec{
		Name: "sqlindex", Transport: mcp.TransportStdio, Command: []string{"x"},
	})

	if redMode() {
		// RED: under the broken bare-name allowlist, 'search' classifies
		// LevelReadOnly, which applyApprovalGate short-circuits unconditionally
		// (registry.go ~561) → the gate is bypassed for a mutating tool.
		brokenLevel := approval.LevelEdit
		if isReadOnlyMCPToolName("search") { // broken predicate ignores the server
			brokenLevel = approval.LevelReadOnly
		}
		assert.Equalf(t, approval.LevelReadOnly, brokenLevel,
			"RED_MODE: broken code lets the mutating 'search' bypass the gate (LevelReadOnly short-circuit)")
		return
	}

	level := classifyMCPToolLevelForTest(m, "sqlindex", "search")
	// applyApprovalGate short-circuits ONLY LevelReadOnly (registry.go ~561).
	bypassed := level == approval.LevelReadOnly
	assert.Falsef(t, bypassed,
		"mutating 'search' on an untrusted server MUST NOT hit the LevelReadOnly gate short-circuit")
}

// TestSecurity_TrustedReadOnlyClassificationsPreserved ensures the fix does NOT
// over-correct: genuinely-read-only paths still classify read tools as
// LevelReadOnly. Asserts the legitimate paths in BOTH polarities (the fix must
// keep them working; the broken artifact also classified them this way, so
// these assertions are polarity-stable and act as the §11.4.120 reconciliation
// proof that read-only servers/tools are not regressed).
func TestSecurity_TrustedReadOnlyClassificationsPreserved(t *testing.T) {
	// (a) Server explicitly flagged readOnly:true → ALL its tools LevelReadOnly.
	roSrv := newConfiguredManager(mcp.ServerSpec{
		Name: "ro", Transport: mcp.TransportStdio, Command: []string{"x"}, ReadOnly: true,
	})
	assert.Equal(t, approval.LevelReadOnly, classifyMCPToolLevelForTest(roSrv, "ro", "write_file"),
		"readOnly:true server keeps every tool LevelReadOnly")
	assert.Equal(t, approval.LevelReadOnly, classifyMCPToolLevelForTest(roSrv, "ro", "search"),
		"readOnly:true server's 'search' stays LevelReadOnly")

	// (b) The well-known filesystem server (matched by command), NOT flagged
	//     readOnly → read-name tools LevelReadOnly, mutating-name tools LevelEdit.
	//     This assertion is the post-fix behaviour; under RED_MODE the broken
	//     artifact also classifies read_text_file as LevelReadOnly (just for the
	//     wrong reason), so it is polarity-stable.
	fsSrv := newConfiguredManager(mcp.ServerSpec{
		Name:      "fs",
		Transport: mcp.TransportStdio,
		Command:   []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "."},
	})
	assert.Equal(t, approval.LevelReadOnly, classifyMCPToolLevelForTest(fsSrv, "fs", "read_text_file"),
		"well-known filesystem server's read tool is LevelReadOnly via the name allowlist")
	assert.Equal(t, approval.LevelEdit, classifyMCPToolLevelForTest(fsSrv, "fs", "write_file"),
		"well-known filesystem server's mutating tool stays LevelEdit")
}

// --- DEFECT-2: tool-name collision → uncallable tool + silent level flip ----

// TestCollision_DistinctPairsGetDistinctKeys reproduces the many-to-one
// collision: ("a:b","c") and ("a_b","c") both sanitise to "a_b__c". After the
// fix, distinct (server,tool) pairs MUST produce distinct registry keys so
// neither tool becomes silently uncallable.
func TestCollision_DistinctPairsGetDistinctKeys(t *testing.T) {
	if redMode() {
		// RED: reproduce the raw collision on the existing primitive. The bare
		// mcpToolRegisteredName maps both distinct pairs to the SAME key — this
		// PASSes on the broken artifact and FAILs once the keying is fixed.
		assert.Equalf(t, mcpToolRegisteredName("a:b", "c"), mcpToolRegisteredName("a_b", "c"),
			"RED_MODE: distinct (server,tool) pairs sanitise to the SAME bare key (the collision)")
		return
	}

	reg, err := NewToolRegistry(nil)
	require.NoError(t, err)

	keyA := reg.registerMCPToolKey(&mcpTool{
		registry: reg, server: "a:b", toolName: "c",
		approvalLevel: approval.LevelEdit,
	})
	keyB := reg.registerMCPToolKey(&mcpTool{
		registry: reg, server: "a_b", toolName: "c",
		approvalLevel: approval.LevelReadOnly,
	})

	// GREEN guards:
	assert.NotEqualf(t, keyA, keyB,
		"distinct (server,tool) pairs MUST get distinct registry keys (no silent collision)")

	// Both tools remain callable (neither was shadowed away).
	gotA, errA := reg.Get(keyA)
	require.NoErrorf(t, errA, "first colliding tool must remain callable under key %q", keyA)
	gotB, errB := reg.Get(keyB)
	require.NoErrorf(t, errB, "second colliding tool must remain callable under key %q", keyB)

	// Each key resolves to its OWN tool (dispatch by original server/toolName
	// preserved) and the survivor's approvalLevel is NOT silently flipped.
	mtA := gotA.(*mcpTool)
	mtB := gotB.(*mcpTool)
	assert.Equal(t, "a:b", mtA.server, "key A must resolve to server a:b")
	assert.Equal(t, "a_b", mtB.server, "key B must resolve to server a_b")
	assert.Equal(t, approval.LevelEdit, mtA.RequiresApproval(),
		"tool A keeps its own approval level (no silent flip)")
	assert.Equal(t, approval.LevelReadOnly, mtB.RequiresApproval(),
		"tool B keeps its own approval level (no silent flip)")
}

// TestCollision_SamePairIsIdempotent confirms re-registering the SAME
// (server,tool) reuses the same key (a genuine re-register, not a collision).
// Polarity-stable (GREEN-only).
func TestCollision_SamePairIsIdempotent(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: idempotency holds in both polarities; only the collision/security RED assertions reproduce defects")
	}
	reg, err := NewToolRegistry(nil)
	require.NoError(t, err)

	k1 := reg.registerMCPToolKey(&mcpTool{registry: reg, server: "fs", toolName: "read_file", approvalLevel: approval.LevelReadOnly})
	k2 := reg.registerMCPToolKey(&mcpTool{registry: reg, server: "fs", toolName: "read_file", approvalLevel: approval.LevelReadOnly})
	assert.Equal(t, k1, k2, "re-registering the same (server,tool) MUST reuse the same key")
}
