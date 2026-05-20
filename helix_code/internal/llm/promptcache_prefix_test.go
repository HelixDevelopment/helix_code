package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toolSchemaWithMapDerivedArrays builds a JSON-Schema tool-parameter map whose
// `required` array is assembled by ranging over a Go map — the exact
// randomization pitfall speed-programme P1-T04 closes. Each call produces a
// logically-identical schema with a freshly-randomized `required` order.
func toolSchemaWithMapDerivedArrays() map[string]interface{} {
	requiredSet := map[string]struct{}{
		"path": {}, "content": {}, "mode": {}, "encoding": {}, "create_dirs": {},
	}
	required := make([]interface{}, 0, len(requiredSet))
	for k := range requiredSet {
		required = append(required, k)
	}
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":        map[string]interface{}{"type": "string"},
			"content":     map[string]interface{}{"type": "string"},
			"mode":        map[string]interface{}{"type": "string"},
			"encoding":    map[string]interface{}{"type": "string"},
			"create_dirs": map[string]interface{}{"type": "boolean"},
		},
		"required": required,
	}
}

func twoToolRequest() *LLMRequest {
	return &LLMRequest{
		ID:    uuid.New(),
		Model: "claude-3-5-sonnet-latest",
		Messages: []Message{
			{Role: "system", Content: "You are HelixCode, an enterprise AI development platform."},
			{Role: "user", Content: "Write a file."},
		},
		MaxTokens: 1024,
		Tools: []Tool{
			{Type: "function", Function: ToolFunction{
				Name: "read_file", Description: "Read a file",
				Parameters: toolSchemaWithMapDerivedArrays(),
			}},
			{Type: "function", Function: ToolFunction{
				Name: "write_file", Description: "Write a file",
				Parameters: toolSchemaWithMapDerivedArrays(),
			}},
		},
	}
}

// TestPromptCache_Integration_CacheControlOnStablePrefix sends real HTTP
// requests through AnthropicProvider.Generate to an Anthropic-shim httptest
// server and verifies, on the captured wire bytes:
//
//  1. cache_control ephemeral breakpoints land on the STABLE PREFIX only —
//     the system block and the last (cache-boundary) tool definition.
//  2. the serialized prefix (system + tools) is BYTE-IDENTICAL across two
//     requests in the same session — the precondition for a provider cache hit.
//
// This is the P1-T04 anti-bluff proof: the request shape carries cache_control
// on the prefix, and the prefix does not drift.
func TestPromptCache_Integration_CacheControlOnStablePrefix(t *testing.T) {
	var capturedBodies [][]byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		capturedBodies = append(capturedBodies, body)

		resp := anthropicResponse{
			ID: "msg_test", Type: "message", Role: "assistant",
			Content:    []anthropicContentBlock{{Type: "text", Text: "ok"}},
			Model:      "claude-3-5-sonnet-latest",
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 100, OutputTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewAnthropicProvider(ProviderConfigEntry{
		Type: "anthropic", Endpoint: server.URL, APIKey: "test-key",
	})
	require.NoError(t, err)

	// Two requests in the same session, each rebuilding tools from a freshly-
	// randomized schema map.
	for turn := 0; turn < 2; turn++ {
		_, err := provider.Generate(context.Background(), twoToolRequest())
		require.NoError(t, err, "turn %d generate failed", turn)
	}
	require.Len(t, capturedBodies, 2, "expected 2 captured request bodies")

	// --- Assertion 1: cache_control on the stable prefix only ---
	var first anthropicRequest
	require.NoError(t, json.Unmarshal(capturedBodies[0], &first))

	// System must be a []anthropicSystemBlock carrying an ephemeral breakpoint.
	sysBlocks := decodeSystemBlocks(t, capturedBodies[0])
	require.Len(t, sysBlocks, 1, "system must be a single cacheable block")
	require.NotNil(t, sysBlocks[0].CacheControl, "system block must carry cache_control")
	assert.Equal(t, "ephemeral", sysBlocks[0].CacheControl.Type)

	// The last tool (cache boundary) carries cache_control; earlier tools do not.
	require.Len(t, first.Tools, 2, "expected 2 tools on the wire")
	assert.Nil(t, first.Tools[0].CacheControl, "non-boundary tool must NOT carry cache_control")
	require.NotNil(t, first.Tools[1].CacheControl, "boundary (last) tool must carry cache_control")
	assert.Equal(t, "ephemeral", first.Tools[1].CacheControl.Type)

	// --- Assertion 2: prefix is byte-identical across the two requests ---
	prefix0 := extractPrefixBytes(t, capturedBodies[0])
	prefix1 := extractPrefixBytes(t, capturedBodies[1])
	assert.Equal(t, string(prefix0), string(prefix1),
		"ANTI-BLUFF: prompt-cache prefix (system+tools) must be byte-identical across requests")
	t.Logf("ANTI-BLUFF PROOF: prefix byte-identical across 2 requests (%d bytes); cache_control ephemeral on system block + boundary tool", len(prefix0))
}

// decodeSystemBlocks extracts the `system` field of a raw Anthropic request
// body as a []anthropicSystemBlock.
func decodeSystemBlocks(t *testing.T, body []byte) []anthropicSystemBlock {
	t.Helper()
	var wire struct {
		System json.RawMessage `json:"system"`
	}
	require.NoError(t, json.Unmarshal(body, &wire))
	var blocks []anthropicSystemBlock
	require.NoError(t, json.Unmarshal(wire.System, &blocks), "system field must be a block array")
	return blocks
}

// extractPrefixBytes returns the canonical JSON of just the cacheable prefix
// (system + tools) of a raw Anthropic request body. Used to assert byte
// stability of the prefix across requests.
func extractPrefixBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var wire struct {
		System json.RawMessage `json:"system"`
		Tools  json.RawMessage `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(body, &wire))
	prefix := map[string]json.RawMessage{
		"system": wire.System,
		"tools":  wire.Tools,
	}
	out, err := json.Marshal(prefix)
	require.NoError(t, err)
	return out
}

// TestPromptCache_Integration_PrefixDetectorFreezesOnFirstRequest verifies the
// AnthropicProvider's prefix detector freezes on the first request and reports
// the prefix as stable (not broken) on the second — the no-regression proof
// that ordinary multi-turn sessions do not trip the cache-break detector.
func TestPromptCache_Integration_PrefixDetectorFreezesOnFirstRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID: "msg", Type: "message", Role: "assistant",
			Content:    []anthropicContentBlock{{Type: "text", Text: "ok"}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 50, OutputTokens: 3},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewAnthropicProvider(ProviderConfigEntry{
		Type: "anthropic", Endpoint: server.URL, APIKey: "test-key",
	})
	require.NoError(t, err)

	require.False(t, provider.prefixDetector.IsFrozen(), "detector must start unfrozen")

	_, err = provider.Generate(context.Background(), twoToolRequest())
	require.NoError(t, err)
	require.True(t, provider.prefixDetector.IsFrozen(), "first request must freeze the prefix")
	baseline := provider.prefixDetector.Baseline()
	require.NotEmpty(t, baseline)

	// Second request, same logical prefix (schema re-randomized) — must not break.
	_, err = provider.Generate(context.Background(), twoToolRequest())
	require.NoError(t, err)
	assert.Equal(t, baseline, provider.prefixDetector.Baseline(),
		"baseline must be unchanged by a stable second request")
	t.Logf("prefix detector froze baseline %s and stayed stable on turn 2", baseline[:12])
}
