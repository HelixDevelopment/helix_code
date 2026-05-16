//go:build integration
// +build integration

// HelixCode/tests/integration/auto_compaction_integration_test.go
package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/compression"
	"dev.helix.code/internal/llm/compressioniface"
)

// TestAutoCompaction_IntegrationLargeConversation drives a conversation
// deliberately built to exceed 80% of a 200k context window through a real
// Anthropic provider, and asserts that AutoCompactor:
//  1. detects the threshold breach,
//  2. invokes the underlying CompressionCoordinator,
//  3. returns a result with TokensAfter < TokensBefore,
//  4. attaches CompactionMetadata to the surviving assistant message
//     OR reduces the message count (one or both is acceptable).
//
// SKIP-OK: #P1-F01-INT — when HELIX_LLM_ANTHROPIC_KEY is unset, the
// test cannot exercise the real provider; it skips gracefully.
func TestAutoCompaction_IntegrationLargeConversation(t *testing.T) {
	apiKey := os.Getenv("HELIX_LLM_ANTHROPIC_KEY")
	if apiKey == "" {
		t.Skip("SKIP-OK: #P1-F01-INT — HELIX_LLM_ANTHROPIC_KEY not set; integration test requires real Anthropic credentials")
	}

	// Real Anthropic provider — uses ProviderConfigEntry (the actual constructor signature).
	prov, err := llm.NewAnthropicProvider(llm.ProviderConfigEntry{
		Type:    llm.ProviderTypeAnthropic,
		APIKey:  apiKey,
		Enabled: true,
		Models:  []string{"claude-3-5-sonnet-20241022"},
	})
	require.NoError(t, err)
	defer func() { _ = prov.Close() }()

	// Build a conversation with ~165k tokens (~3.5 chars per token):
	// 116 messages × ~5 000 chars = ~580k chars ≈ 165k tokens.
	// 80% of 200k = 160k → 165k exceeds the threshold.
	conv := &compressioniface.Conversation{
		ID:        "p1-f01-int-test",
		CreatedAt: time.Now(),
		Messages:  make([]*compressioniface.Message, 0, 116),
	}
	for i := 0; i < 116; i++ {
		role := compressioniface.RoleUser
		if i%2 == 1 {
			role = compressioniface.RoleAssistant
		}
		conv.Messages = append(conv.Messages, &compressioniface.Message{
			ID:      fmt.Sprintf("msg-%d", i),
			Role:    role,
			Content: strings.Repeat("This is filler content to consume tokens. ", 120),
		})
	}

	// Coordinator + AutoCompactor wiring — real provider, no mocks.
	coord := compression.NewCompressionCoordinator(
		prov,
		compression.WithStrategy(compression.StrategySemanticSummarization),
	)
	guard := compression.NewThrashingGuard(3)
	ac := compression.NewAutoCompactor(prov, coord, guard, 0.80)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	originalCount := len(conv.Messages)
	result, err := ac.MaybeCompact(ctx, conv)
	require.NoError(t, err, "MaybeCompact must not error on a deliberately-oversized conversation")
	require.True(t, result.WasCompacted, "conversation at ~165k tokens MUST trigger compaction at 80% of 200k window")
	require.Less(t, result.TokensAfter, result.TokensBefore, "post-compaction tokens must be less than pre-compaction tokens")

	// Acceptance: either fewer messages OR explicit CompactionMetadata.
	hasMetadata := false
	for _, m := range conv.Messages {
		if _, ok := compression.ReadCompactionMetadata(m); ok {
			hasMetadata = true
			break
		}
	}
	require.True(t, hasMetadata || len(conv.Messages) < originalCount,
		"after compaction, conversation must have either CompactionMetadata or fewer messages")
}
