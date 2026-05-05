package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSystemPrompt_IncludesPersistedOutputNote(t *testing.T) {
	a := &BaseAgent{
		agentType:    "coding",
		name:         "test",
		capabilities: []Capability{CapabilityCodeGeneration, CapabilityCodeAnalysis},
	}
	prompt := a.getSystemPrompt()

	assert.Contains(t, prompt, "persistedOutputPath",
		"system prompt must teach the LLM about persisted outputs")
	assert.Contains(t, prompt, "Read",
		"system prompt must instruct the LLM to use the Read tool")
	assert.Contains(t, prompt, "50,000",
		"system prompt must reference the threshold so the LLM understands the trigger")
}

// Existing assertions remain — the prompt still describes the agent type/name.
func TestGetSystemPrompt_StillDescribesAgent(t *testing.T) {
	a := &BaseAgent{
		agentType:    "coding",
		name:         "test",
		capabilities: []Capability{CapabilityCodeGeneration},
	}
	prompt := a.getSystemPrompt()
	assert.Contains(t, prompt, "coding")
	assert.Contains(t, prompt, "test")
	// Look for either a JSON instruction or a closing instruction
	assert.True(t, strings.Contains(prompt, "JSON") || strings.Contains(prompt, "Read"))
}
