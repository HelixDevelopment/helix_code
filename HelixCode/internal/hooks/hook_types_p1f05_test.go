package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestF05HookTypes_AreDistinct(t *testing.T) {
	newTypes := []HookType{
		HookTypeBeforeToolCall,
		HookTypeAfterToolCall,
		HookTypeBeforeBash,
		HookTypeAfterBash,
		HookTypeOnCompaction,
		HookTypeOnPlanApproval,
	}
	seen := map[HookType]bool{}
	for _, ht := range newTypes {
		assert.NotEmpty(t, string(ht), "HookType must have a non-empty string value")
		assert.False(t, seen[ht], "duplicate HookType value: %q", ht)
		seen[ht] = true
	}
}

func TestF05HookTypes_StringValues(t *testing.T) {
	cases := map[HookType]string{
		HookTypeBeforeToolCall: "before_tool_call",
		HookTypeAfterToolCall:  "after_tool_call",
		HookTypeBeforeBash:     "before_bash",
		HookTypeAfterBash:      "after_bash",
		HookTypeOnCompaction:   "on_compaction",
		HookTypeOnPlanApproval: "on_plan_approval",
	}
	for ht, expected := range cases {
		assert.Equal(t, expected, string(ht), "HookType %q has wrong string value", expected)
	}
}

func TestF05HookTypes_DoNotCollideWithExisting(t *testing.T) {
	existing := []HookType{
		HookTypeBeforeTask, HookTypeAfterTask,
		HookTypeBeforeLLM, HookTypeAfterLLM,
		HookTypeBeforeEdit, HookTypeAfterEdit,
		HookTypeBeforeBuild, HookTypeAfterBuild,
		HookTypeBeforeTest, HookTypeAfterTest,
		HookTypeOnError, HookTypeOnSuccess,
		HookTypeCustom,
	}
	new := []HookType{
		HookTypeBeforeToolCall, HookTypeAfterToolCall,
		HookTypeBeforeBash, HookTypeAfterBash,
		HookTypeOnCompaction, HookTypeOnPlanApproval,
	}
	all := append([]HookType{}, existing...)
	all = append(all, new...)
	seen := map[HookType]bool{}
	for _, ht := range all {
		assert.False(t, seen[ht], "HookType collision: %q already exists", ht)
		seen[ht] = true
	}
}
