package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/confirmation"
)

func TestParsePattern_Valid(t *testing.T) {
	tests := []struct {
		in       string
		toolName string
		argPat   string
	}{
		{"Bash(git status:*)", "Bash", "git status:*"},
		{"Read(*.go)", "Read", "*.go"},
		{"Edit(internal/auth/*)", "Edit", "internal/auth/*"},
		{"Write()", "Write", ""},
		{"*(*)", "*", "*"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParsePattern(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.toolName, got.ToolName)
			assert.Equal(t, tt.argPat, got.ArgPattern)
		})
	}
}

func TestParsePattern_Malformed(t *testing.T) {
	bad := []string{"Bash", "Bash(", "Bash)", "(foo)", "", "Bash()(extra)"}
	for _, b := range bad {
		t.Run(b, func(t *testing.T) {
			_, err := ParsePattern(b)
			assert.Error(t, err)
		})
	}
}

func TestEvaluate_PriorityOrder(t *testing.T) {
	rules := []Rule{
		{Pattern: "Bash(git push*)", Action: confirmation.ActionDeny, Priority: 1000},
		{Pattern: "Bash(*)", Action: confirmation.ActionAllow, Priority: 1},
	}
	eng, err := NewRuleEngine(rules)
	require.NoError(t, err)
	got := eng.Evaluate("Bash", "git push origin main")
	assert.Equal(t, confirmation.ActionDeny, got.Action)
	assert.Equal(t, "Bash(git push*)", got.MatchedPattern)
}

func TestEvaluate_NoMatchReturnsAsk(t *testing.T) {
	rules := []Rule{
		{Pattern: "Bash(ls*)", Action: confirmation.ActionAllow},
	}
	eng, err := NewRuleEngine(rules)
	require.NoError(t, err)
	got := eng.Evaluate("Read", "/etc/passwd")
	assert.Equal(t, confirmation.ActionAsk, got.Action)
	assert.Empty(t, got.MatchedPattern)
}

func TestEvaluate_CompoundAggregation_DenyWins(t *testing.T) {
	rules := []Rule{
		{Pattern: "Bash(echo*)", Action: confirmation.ActionAllow},
		{Pattern: "Bash(rm*)", Action: confirmation.ActionDeny, Priority: 100},
	}
	eng, err := NewRuleEngine(rules)
	require.NoError(t, err)
	got := eng.Evaluate("Bash", "echo hello && rm -rf /tmp/x")
	assert.Equal(t, confirmation.ActionDeny, got.Action)
}

func TestEvaluate_CompoundAggregation_AskOverAllow(t *testing.T) {
	rules := []Rule{
		{Pattern: "Bash(echo*)", Action: confirmation.ActionAllow},
		{Pattern: "Bash(curl*)", Action: confirmation.ActionAsk},
	}
	eng, err := NewRuleEngine(rules)
	require.NoError(t, err)
	got := eng.Evaluate("Bash", "echo hi && curl example.com")
	assert.Equal(t, confirmation.ActionAsk, got.Action)
}

func TestEvaluate_CommandSubstitutionIsAggregated(t *testing.T) {
	rules := []Rule{
		{Pattern: "Bash(echo*)", Action: confirmation.ActionAllow},
		{Pattern: "Bash(rm*)", Action: confirmation.ActionDeny, Priority: 100},
	}
	eng, err := NewRuleEngine(rules)
	require.NoError(t, err)
	got := eng.Evaluate("Bash", "echo $(rm -rf /tmp/x)")
	assert.Equal(t, confirmation.ActionDeny, got.Action,
		"smuggled rm inside $() must propagate deny to compound")
}

func TestEvaluate_ShellParseErrorIsDeny(t *testing.T) {
	rules := []Rule{
		{Pattern: "Bash(*)", Action: confirmation.ActionAllow},
	}
	eng, err := NewRuleEngine(rules)
	require.NoError(t, err)
	got := eng.Evaluate("Bash", `echo "unclosed`)
	assert.Equal(t, confirmation.ActionDeny, got.Action,
		"shell parse error must fail-closed to deny")
}

func TestNewRuleEngine_RejectsMalformedPatterns(t *testing.T) {
	rules := []Rule{
		{Pattern: "Bash(", Action: confirmation.ActionAllow},
	}
	_, err := NewRuleEngine(rules)
	require.Error(t, err)
}
