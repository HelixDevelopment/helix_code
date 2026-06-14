//go:build helixagentlive

// LIVE anti-bluff embeddings test (CONST-035 / §11.4.5 captured-evidence).
// Exercises the Embedder against the REAL running HelixAgent server's
// /v1/embeddings endpoint:
//
//	go test -tags helixagentlive -run Live_Embed ./internal/llm/providers/helixagent/
//
// Honest-reporting contract: the OpenAI-compatible /v1/embeddings shape is
// asserted. If the live server's /v1/embeddings does NOT speak the OpenAI shape
// (e.g. it is an MCP/JSON-RPC transport, or 404s), the test FAILs loudly with
// the captured server response so the discrepancy is visible — never a faked
// PASS. If the server is entirely unreachable the test SKIPs with a reason.
//
// CAPTURED LIVE FINDING (2026-06-14, agent up on :7061): the live
// /v1/embeddings endpoint is an MCP JSON-RPC transport (serverInfo
// "helixagent-embeddings" v1.0.0) exposing an `embeddings_generate` MCP tool —
// NOT an OpenAI-shape REST endpoint. An OpenAI-shape POST returns
// {"jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found"}}, and
// the MCP tool itself returns only a text confirmation
// ("Generated embedding for text of length N"), never a numeric vector. So no
// real [][]float32 is obtainable from this live endpoint today; the adapter
// surfaces the "Method not found" error and this test FAILs honestly rather
// than fabricating a vector. The non-live httptest unit test proves the OpenAI
// /v1/embeddings parse path against the documented shape.
package helixagent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLive_Embed_AgainstRunningAgent(t *testing.T) {
	p := New(liveBaseURL())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !p.IsAvailable(ctx) {
		t.Skipf("SKIP-OK: HelixAgent not reachable at %s — start the agent on :7061 to run the live embeddings test", p.baseURL)
	}

	vecs, err := p.Embed(ctx, []string{"the quick brown fox", "a fast auburn fox"})
	if err != nil {
		// Honest reporting: surface the EXACT live failure. The endpoint on the
		// probed server is an MCP/JSON-RPC transport rather than an OpenAI-shape
		// /v1/embeddings REST endpoint; the adapter correctly reports that rather
		// than bluffing a vector. Failing here keeps the discrepancy visible.
		t.Fatalf("LIVE /v1/embeddings did not return an OpenAI-shape vector: %v", err)
	}

	require.Len(t, vecs, 2, "one vector per input")
	require.NotEmpty(t, vecs[0], "live vector is non-empty")
	require.Equal(t, len(vecs[0]), len(vecs[1]), "consistent dimension across inputs")

	sim := CosineSimilarity(vecs[0], vecs[1])
	t.Logf("LIVE embeddings PASS: dimension=%d, cosine(similar phrases)=%.4f", len(vecs[0]), sim)
	assert.GreaterOrEqual(t, sim, -1.0)
	assert.LessOrEqual(t, sim, 1.0)
}
