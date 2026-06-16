package permissions_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
	"dev.helix.code/internal/tools/permissions/sessionrules"
)

// newBaseEngine builds a live permissions.Engine over an empty file-loaded
// rule set (the FileLoader resolves to non-existent paths so only built-in
// defaults / mode apply). The default mode leaves Bash at ActionAsk, which is
// the realistic "no base rule covers this command" state where a session add
// must be the deciding factor.
func newBaseEngine(t *testing.T, store *sessionrules.Store) *permissions.Engine {
	t.Helper()
	loader := &permissions.FileLoader{
		UserPath:    t.TempDir() + "/nonexistent-user.yaml",
		ProjectPath: t.TempDir() + "/nonexistent-project.yaml",
		Mode:        "",
	}
	var opts []permissions.EngineOption
	if store != nil {
		opts = append(opts, permissions.WithSessionDecider(store.Decide))
	}
	eng, err := permissions.NewEngine(context.Background(), loader, confirmation.NewPolicyEngine(), opts...)
	require.NoError(t, err)
	return eng
}

func bashReq(session, command string) confirmation.ConfirmationRequest {
	return confirmation.ConfirmationRequest{
		ToolName:   "Bash",
		Parameters: map[string]interface{}{"command": command},
		Context:    confirmation.ExecutionContext{SessionID: session},
	}
}

// TestEngine_SessionRule_LiveGate is the RED/GREEN polarity guard (§11.4.115).
//
// RED_MODE=1 (default): reproduces the defect on the CURRENT broken wiring —
// the live Engine is built WITHOUT a session decider (mirroring the pre-fix
// state where cmd/cli/main.go's Engine and the slash command's store were two
// unconnected objects). A deny rule added to a session store is NOT honoured by
// the live Engine.Decide → the call still resolves to Ask, not Deny. The test
// asserts that broken state is genuinely present (the defect reproduces).
//
// RED_MODE=0: the standing GREEN regression guard — the live Engine IS wired to
// the SAME store the writer mutates. A session deny rule added to that store is
// honoured by the live gate immediately (Decide → Deny). Removing it reverts the
// live gate to Ask. This is the genuine /permissions add → live gating link.
func TestEngine_SessionRule_LiveGate(t *testing.T) {
	const session = "live-sess"
	const cmd = "git push origin main"
	redMode := os.Getenv("RED_MODE") != "0" // default RED

	// The store the `/permissions add|remove` writer mutates.
	store := sessionrules.New()

	if redMode {
		// CURRENT broken wiring: live engine has NO session decider.
		eng := newBaseEngine(t, nil)

		// Operator runs `/permissions add Bash(git push*) deny` — writes to store.
		store.Add(session, permissions.Rule{
			Pattern: "Bash(git push*)", Action: confirmation.ActionDeny, Priority: 10,
		})

		// Sanity: the store itself honours the rule (the just-built feature works).
		require.Equal(t, confirmation.ActionDeny,
			store.Decide(session, "Bash", cmd).Action,
			"store must honour its own added rule")

		// DEFECT: the LIVE engine that actually gates execution does NOT consult
		// the store, so the added deny rule is ignored — the call is still Ask.
		d := eng.Decide(bashReq(session, cmd))
		assert.NotEqual(t, confirmation.ActionDeny, d.Action,
			"RED: defect must reproduce — live engine ignores the session-added deny rule")
		assert.Equal(t, confirmation.ActionAsk, d.Action,
			"RED: with no session decider wired, the unmatched command falls through to Ask")
		return
	}

	// GREEN guard: live engine wired to the SAME store the writer mutates.
	eng := newBaseEngine(t, store)

	// Before any add: no session rule, falls through to base → Ask.
	require.Equal(t, confirmation.ActionAsk, eng.Decide(bashReq(session, cmd)).Action,
		"with no rule added the live engine falls through to base (Ask)")

	// Operator runs `/permissions add Bash(git push*) deny`.
	store.Add(session, permissions.Rule{
		Pattern: "Bash(git push*)", Action: confirmation.ActionDeny, Priority: 10,
	})

	// The live gate now denies immediately — no restart.
	d := eng.Decide(bashReq(session, cmd))
	assert.Equal(t, confirmation.ActionDeny, d.Action,
		"GREEN: session-added deny rule must be honoured by the live engine")
	assert.Equal(t, "Bash(git push*)", d.MatchedPattern,
		"GREEN: the live decision must cite the session-added pattern")

	// Operator runs `/permissions remove Bash(git push*)`.
	require.True(t, store.Remove(session, "Bash(git push*)"))

	// The live gate reverts to base behaviour (Ask) immediately.
	assert.Equal(t, confirmation.ActionAsk, eng.Decide(bashReq(session, cmd)).Action,
		"GREEN: after remove, the live engine reverts to base (Ask)")
}

// TestEngine_SessionRule_AllowOverlay proves a session ALLOW rule also reaches
// the live gate (not only deny), and that a session decider that does not match
// falls through to the file-loaded base rules.
func TestEngine_SessionRule_AllowOverlay(t *testing.T) {
	const session = "allow-sess"
	store := sessionrules.New()
	eng := newBaseEngine(t, store)

	// Unrelated command with no session rule → falls through to base (Ask).
	assert.Equal(t, confirmation.ActionAsk,
		eng.Decide(bashReq(session, "ls -la")).Action)

	// Add an allow rule; the live gate now allows that command.
	store.Add(session, permissions.Rule{
		Pattern: "Bash(ls*)", Action: confirmation.ActionAllow, Priority: 5,
	})
	d := eng.Decide(bashReq(session, "ls -la"))
	assert.Equal(t, confirmation.ActionAllow, d.Action,
		"session-added allow rule must reach the live gate")
	assert.Equal(t, "Bash(ls*)", d.MatchedPattern)

	// A different command still has no session match → base (Ask).
	assert.Equal(t, confirmation.ActionAsk,
		eng.Decide(bashReq(session, "rm -rf /")).Action,
		"non-matching command must fall through to base, not be swallowed")
}

// TestEngine_SessionRule_IsolatedPerSession proves the live gate keys session
// rules by the request's SessionID — session A's rule must not gate session B.
func TestEngine_SessionRule_IsolatedPerSession(t *testing.T) {
	store := sessionrules.New()
	eng := newBaseEngine(t, store)

	store.Add("A", permissions.Rule{
		Pattern: "Bash(curl*)", Action: confirmation.ActionDeny, Priority: 1,
	})

	assert.Equal(t, confirmation.ActionDeny,
		eng.Decide(bashReq("A", "curl http://x")).Action,
		"session A's deny rule must gate session A")
	assert.Equal(t, confirmation.ActionAsk,
		eng.Decide(bashReq("B", "curl http://x")).Action,
		"session A's rule must NOT gate session B")
}
