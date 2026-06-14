//go:build helixagentlive

// LIVE anti-bluff test (CONST-035 / §11.4.5 captured-evidence): exercises the
// HelixAgent adapter against the REAL running HelixAgent server. Build-tagged
// so it runs ONLY when explicitly requested:
//
//	go test -tags helixagentlive -run Live ./internal/llm/providers/helixagent/
//
// Base URL is resolved from HELIXAGENT_BASE_URL (default DefaultBaseURL). If
// the agent is not reachable the test SKIPs with a reason (§11.4.3) — the
// non-live unit tests still prove the mapping.
package helixagent

import (
	"context"
	"os"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func liveBaseURL() string {
	if v := os.Getenv("HELIXAGENT_BASE_URL"); v != "" {
		return v
	}
	return DefaultBaseURL
}

func TestLive_GenerateAgainstRunningAgent(t *testing.T) {
	p := New(liveBaseURL())

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if !p.IsAvailable(ctx) {
		t.Skipf("SKIP-OK: HelixAgent not reachable at %s — start the agent on :7061 to run the live test", p.baseURL)
	}

	// Real model discovery from the live engine.
	models := p.GetModels()
	require.NotEmpty(t, models, "live /v1/models returned at least one model")
	t.Logf("LIVE models (%d): %+v", len(models), modelIDs(models))

	// Real chat completion through the real 25-provider engine.
	req := &llm.LLMRequest{
		ID:        uuid.New(),
		Model:     "helixagent-llm",
		Messages:  []llm.Message{{Role: "user", Content: "Reply with exactly: PONG"}},
		MaxTokens: 64,
	}
	resp, err := p.Generate(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content, "live HelixAgent returned non-empty content")
	t.Logf("LIVE HelixAgent response: %q (finish=%s, usage=%+v, model=%v)",
		resp.Content, resp.FinishReason, resp.Usage, resp.ProviderMetadata["helixagent_model"])
}

func modelIDs(models []llm.ModelInfo) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		out = append(out, m.ID)
	}
	return out
}
