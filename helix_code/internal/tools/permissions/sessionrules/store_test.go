package sessionrules

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
)

func TestStore_AddListRemove_RoundTrip(t *testing.T) {
	s := New()
	const sess = "sess-1"
	rule := permissions.Rule{Pattern: "Bash(git push:*)", Action: confirmation.ActionDeny, Priority: 10}

	require.False(t, s.Has(sess, rule.Pattern))
	s.Add(sess, rule)
	require.True(t, s.Has(sess, rule.Pattern))

	got := s.Rules(sess)
	require.Len(t, got, 1)
	assert.Equal(t, rule.Pattern, got[0].Pattern)
	assert.Equal(t, confirmation.ActionDeny, got[0].Action)
	assert.Equal(t, 10, got[0].Priority)
	// Source is forced to ScopeCLI for session rules.
	assert.Equal(t, permissions.ScopeCLI, got[0].Source)

	removed := s.Remove(sess, rule.Pattern)
	assert.True(t, removed)
	assert.False(t, s.Has(sess, rule.Pattern))
	assert.Empty(t, s.Rules(sess))
}

func TestStore_Remove_ReportsNotFound(t *testing.T) {
	s := New()
	assert.False(t, s.Remove("sess", "Bash(nonexistent:*)"),
		"removing a pattern that was never added must report false, not bluff a success")
}

func TestStore_Add_UpsertsByPattern(t *testing.T) {
	s := New()
	const sess = "s"
	s.Add(sess, permissions.Rule{Pattern: "Edit(*)", Action: confirmation.ActionAllow, Priority: 1})
	s.Add(sess, permissions.Rule{Pattern: "Edit(*)", Action: confirmation.ActionDeny, Priority: 5})
	got := s.Rules(sess)
	require.Len(t, got, 1, "re-adding the same pattern must upsert, not duplicate")
	assert.Equal(t, confirmation.ActionDeny, got[0].Action)
	assert.Equal(t, 5, got[0].Priority)
}

func TestStore_SessionsAreIsolated(t *testing.T) {
	s := New()
	s.Add("a", permissions.Rule{Pattern: "Bash(ls:*)", Action: confirmation.ActionAllow})
	assert.True(t, s.Has("a", "Bash(ls:*)"))
	assert.False(t, s.Has("b", "Bash(ls:*)"), "session b must not see session a's rules")
	assert.Empty(t, s.Rules("b"))
}

func TestStore_Rules_SortedByPriorityDescThenPattern(t *testing.T) {
	s := New()
	const sess = "s"
	s.Add(sess, permissions.Rule{Pattern: "Bash(b:*)", Action: confirmation.ActionAsk, Priority: 1})
	s.Add(sess, permissions.Rule{Pattern: "Bash(a:*)", Action: confirmation.ActionAsk, Priority: 5})
	s.Add(sess, permissions.Rule{Pattern: "Bash(c:*)", Action: confirmation.ActionAsk, Priority: 5})
	got := s.Rules(sess)
	require.Len(t, got, 3)
	assert.Equal(t, "Bash(a:*)", got[0].Pattern) // priority 5, pattern a < c
	assert.Equal(t, "Bash(c:*)", got[1].Pattern) // priority 5
	assert.Equal(t, "Bash(b:*)", got[2].Pattern) // priority 1
}

// TestStore_Decide proves the stored rule is genuinely CONSULTED in a real
// decision path — the action is not merely stored and ignored.
func TestStore_Decide(t *testing.T) {
	s := New()
	const sess = "s"

	// No rules: no match → Ask (caller falls through to base engine).
	d := s.Decide(sess, "Bash", "git push origin main")
	assert.Equal(t, confirmation.ActionAsk, d.Action)
	assert.Empty(t, d.MatchedPattern)

	// Add a deny rule; the same call now denies and cites the matched pattern.
	// (The arg-glob is matched literally against the rendered leaf command, so
	// "git push*" matches "git push origin main".)
	s.Add(sess, permissions.Rule{Pattern: "Bash(git push*)", Action: confirmation.ActionDeny, Priority: 10})
	d = s.Decide(sess, "Bash", "git push origin main")
	assert.Equal(t, confirmation.ActionDeny, d.Action)
	assert.Equal(t, "Bash(git push*)", d.MatchedPattern)

	// An unrelated command still has no match.
	d = s.Decide(sess, "Bash", "ls -la")
	assert.Equal(t, confirmation.ActionAsk, d.Action)
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			pat := "Bash(cmd" + string(rune('a'+n%26)) + ":*)"
			s.Add("sess", permissions.Rule{Pattern: pat, Action: confirmation.ActionAllow})
			_ = s.Rules("sess")
			_ = s.Has("sess", pat)
			_ = s.Decide("sess", "Bash", "cmd")
			s.Remove("sess", pat)
		}(i)
	}
	wg.Wait()
}
