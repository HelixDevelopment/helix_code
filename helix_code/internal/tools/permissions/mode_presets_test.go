package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/confirmation"
)

func TestPresetRules_Default_Empty(t *testing.T) {
	rules := PresetRules("default")
	assert.Empty(t, rules, "default preset has no built-in rules")
}

func TestPresetRules_Auto_AllowsEverything(t *testing.T) {
	rules := PresetRules("auto")
	require.Len(t, rules, 1)
	assert.Equal(t, "*(*)", rules[0].Pattern)
	assert.Equal(t, confirmation.ActionAllow, rules[0].Action)
	assert.Equal(t, ScopePreset, rules[0].Source)
}

func TestPresetRules_AcceptEdits_AllowsEditAndWrite(t *testing.T) {
	rules := PresetRules("acceptEdits")
	patterns := patternsOf(rules)
	assert.Contains(t, patterns, "Edit(*)")
	assert.Contains(t, patterns, "Write(*)")
	assert.Contains(t, patterns, "MultiEdit(*)")
}

func TestPresetRules_DontAsk_AllowsRead(t *testing.T) {
	rules := PresetRules("dontAsk")
	patterns := patternsOf(rules)
	assert.Contains(t, patterns, "Read(*)")
	assert.Contains(t, patterns, "Glob(*)")
	assert.Contains(t, patterns, "Grep(*)")
}

func TestPresetRules_Bypass_HighestPriority(t *testing.T) {
	rules := PresetRules("bypassPermissions")
	require.Len(t, rules, 1)
	assert.Equal(t, "*(*)", rules[0].Pattern)
	assert.Equal(t, confirmation.ActionAllow, rules[0].Action)
	assert.Greater(t, rules[0].Priority, 100_000)
}

func TestPresetRules_UnknownReturnsNil(t *testing.T) {
	assert.Nil(t, PresetRules("nonsense"))
}

func TestReadOnlyCommands_Conservative(t *testing.T) {
	assert.True(t, IsReadOnlyCommand("git status"))
	assert.True(t, IsReadOnlyCommand("ls -la"))
	assert.False(t, IsReadOnlyCommand("git push"))
	assert.False(t, IsReadOnlyCommand("rm /tmp/x"))
}

func TestWriteCommands_Conservative(t *testing.T) {
	assert.True(t, IsWriteCommand("git push origin"))
	assert.True(t, IsWriteCommand("rm -rf /tmp/x"))
	assert.False(t, IsWriteCommand("git status"))
	assert.False(t, IsWriteCommand("ls"))
}

func TestPresetRules_AllParseInRuleEngine(t *testing.T) {
	modes := []string{"auto", "acceptEdits", "dontAsk", "bypassPermissions"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			rules := PresetRules(mode)
			require.NotNil(t, rules)

			// This should not error if all patterns parse correctly
			_, err := NewRuleEngine(rules)
			require.NoError(t, err, "preset %s has malformed patterns", mode)
		})
	}
}

func patternsOf(rules []Rule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.Pattern)
	}
	return out
}
