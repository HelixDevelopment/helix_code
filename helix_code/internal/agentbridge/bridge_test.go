package agentbridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	agentverifier "dev.helix.agent/pkg/sdk/go/verifier"
	"github.com/stretchr/testify/require"
)

// TestVerifierBridge_RealRoundTrip proves that HelixCode (dev.helix.code) can
// import and exercise a REAL helix_agent (dev.helix.agent) type end-to-end.
//
// The system-under-test is the real helix_agent verifier SDK client, reached
// through the bridge. A local httptest server stands in for the LLMsVerifier
// HTTP endpoint (a real HTTP server fixture — allowed for unit tests; the SDK
// client itself is NOT mocked, it makes a genuine HTTP request). The assertions
// verify the cross-module wiring: the request the helix_agent SDK emits, and
// the helix_agent VerificationResult type it decodes.
func TestVerifierBridge_RealRoundTrip(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody agentverifier.VerificationRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)

		// Respond with a real VerificationResult the SDK will decode.
		resp := agentverifier.VerificationResult{
			ModelID:      "llama3.2",
			Provider:     "ollama",
			Verified:     true,
			Score:        0.91,
			OverallScore: 0.91,
			Tests:        map[string]bool{"reasoning": true},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	bridge := NewVerifierBridge(Config{
		BaseURL: srv.URL,
		APIKey:  "test-key-123",
		Timeout: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := bridge.VerifyModel(ctx, "llama3.2", "ollama", []string{"reasoning"})
	require.NoError(t, err)
	require.NotNil(t, result)

	// The real helix_agent SDK made a real HTTP POST to the verifier route.
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "/api/v1/verifier/verify", gotPath)
	require.Equal(t, "Bearer test-key-123", gotAuth)
	require.Equal(t, "llama3.2", gotBody.ModelID)
	require.Equal(t, "ollama", gotBody.Provider)
	require.Equal(t, []string{"reasoning"}, gotBody.Tests)

	// The bridge returned the real helix_agent VerificationResult type.
	require.Equal(t, "llama3.2", result.ModelID)
	require.Equal(t, "ollama", result.Provider)
	require.True(t, result.Verified)
	require.InDelta(t, 0.91, result.OverallScore, 0.0001)
	require.True(t, result.Tests["reasoning"])

	// Underlying client is the real helix_agent SDK client type.
	require.IsType(t, &agentverifier.Client{}, bridge.Client())
}
