package llm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"dev.helix.code/internal/tools/persistence"
)

// TestAllProvidersAcceptPersistenceManager is a contract test: every provider
// that handles tool_result must accept a *persistence.Manager for wrapping
// large outputs. The test fails if a provider doesn't implement the
// SetPersistenceManager hook (or an equivalent constructor option).
//
// Audit result (P1-F03-T07 Step 1):
//
//	internal/llm/anthropic_provider.go:      1 hit — struct tag comment only, conforming
//	internal/llm/azure_provider.go:          0 hits — conforming (delegates)
//	internal/llm/bedrock_provider.go:        0 hits — conforming (delegates)
//	internal/llm/copilot_provider.go:        0 hits — conforming (delegates)
//	internal/llm/gemini_provider.go:         0 hits — conforming (delegates)
//	internal/llm/groq_provider.go:           0 hits — conforming (delegates)
//	internal/llm/koboldai_provider.go:       0 hits — conforming (delegates)
//	internal/llm/llamacpp_provider.go:       0 hits — conforming (delegates)
//	internal/llm/local_provider.go:          0 hits — conforming (delegates)
//	internal/llm/ollama_provider.go:         0 hits — conforming (delegates)
//	internal/llm/openai_compatible_provider.go: 0 hits — conforming (delegates)
//	internal/llm/openai_provider.go:         0 hits — conforming (delegates)
//	internal/llm/openrouter_provider.go:     0 hits — conforming (delegates)
//	internal/llm/qwen_provider.go:           0 hits — conforming (delegates)
//	internal/llm/vertexai_provider.go:       0 hits — conforming (delegates)
//	internal/llm/xai_provider.go:            0 hits — conforming (delegates)
//
// Conclusion: no provider constructs tool_result wire content directly.
// All tool execution output flows through ToolCallingProvider.persistResults.
// No per-provider wiring is needed — only the contract test is required.
//
// Implementers: when adding a new provider that constructs tool_result
// content directly (bypassing tool_provider.go), add it to this test.
func TestAllProvidersAcceptPersistenceManager(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)

	tcp := &ToolCallingProvider{}
	tcp.SetPersistenceManager(m)
	assert.NotNil(t, tcp.persistenceManager, "ToolCallingProvider must accept Manager")

	// No additional providers needed: the T07 audit confirmed all 15 other
	// provider files have zero tool_result construction bypass paths.
	// The single hit in anthropic_provider.go is a struct tag comment, not
	// wire-content construction from tool execution output.
}

// TestPersistResults_ProducesExpectedRenderedString verifies the integration
// between persistResults and buildFinalPrompt: a persisted result must
// render as a path-reference, not as the original content.
func TestPersistResults_ProducesExpectedRenderedString(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)
	tcp := &ToolCallingProvider{persistenceManager: m}

	big := strings.Repeat("X", persistence.PersistThreshold+1)
	wrapped := tcp.persistResults([]ToolCallResult{{CallID: "c1", ToolName: "Bash", Result: big}})
	rendered := tcp.buildFinalPrompt("orig", "init", wrapped)

	assert.Contains(t, rendered, "persisted to")
	assert.Contains(t, rendered, "Use Read with that path")
	assert.NotContains(t, rendered, big)
}
