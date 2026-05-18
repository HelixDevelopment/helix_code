// Round-54 §11.4 anti-bluff tests for Replicate LLMResponse.Err
// propagation. Replicate's prediction-completion envelope uses a
// `status` field rather than a finish_reason; round-54 added
// ErrReplicatePredictionFailed (in helix_code/internal/llm/missing_types.go)
// and mapReplicateStatusToErr (in this package's client.go) to map
// status="failed" → ErrReplicatePredictionFailed (wrapping the upstream
// error message via fmt.Errorf %w so errors.Is(...) succeeds AND the
// human-readable upstream message survives for diagnostics).
//
// Replicate does NOT natively expose truncation or content-filter
// signals on its prediction envelope — ErrResponseTruncated and
// ErrResponseContentBlocked will NOT fire from this provider unless
// the underlying model writes them into the output payload. Documented
// for downstream callers + future-proofed if Replicate adds those
// signals.
//
// CONST-035 / CONST-050(A)+(B) / Article XI §11.9: every PASS in this
// file is backed by an httptest fixture that exercises the real HTTP
// round-trip (real http.Client, real JSON encode/decode, real
// LLMResponse construction). No mocks of internal helpers.
package replicate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/llm"
)

// TestRound54_Replicate_Generate_StatusFailed_PopulatesErr asserts that
// a Replicate prediction with status="failed" populates LLMResponse.Err
// with ErrReplicatePredictionFailed wrapping the upstream error message.
func TestRound54_Replicate_Generate_StatusFailed_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/models/") && strings.HasSuffix(r.URL.Path, "/predictions") {
			// Initial POST: return a prediction with id but status=processing
			// (forces a polling round).
			resp := map[string]interface{}{
				"id": "pred-round54-failed", "status": "processing",
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/predictions/") {
			// Poll GET: return failed status with upstream error message.
			resp := map[string]interface{}{
				"id":     "pred-round54-failed",
				"status": "failed",
				"error":  "out of memory loading model",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClientWithBaseURL("test-key", server.URL+"/v1")
	client.SetPollInterval(5 * time.Millisecond) // speed up the polling loop
	resp, err := client.Generate(context.Background(), &llm.LLMRequest{
		ID: uuid.New(), Model: "meta/meta-llama-3-70b-instruct",
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err, "Generate MUST NOT return a hard error for status=failed (round-54 contract: failure surfaces via LLMResponse.Err)")
	require.NotNil(t, resp)
	require.NotNil(t, resp.Err, "LLMResponse.Err MUST be populated for prediction status=failed")
	assert.True(t, errors.Is(resp.Err, llm.ErrReplicatePredictionFailed),
		"resp.Err MUST be ErrReplicatePredictionFailed; got %v", resp.Err)
	assert.Contains(t, resp.Err.Error(), "out of memory loading model",
		"resp.Err message MUST include the upstream error string for diagnostics")
}

// TestRound54_Replicate_Generate_StatusSucceeded_LeavesErrNil asserts
// the backward-compat invariant — status="succeeded" leaves Err nil.
func TestRound54_Replicate_Generate_StatusSucceeded_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/models/") && strings.HasSuffix(r.URL.Path, "/predictions") {
			resp := map[string]interface{}{"id": "pred-round54-ok", "status": "processing"}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/predictions/") {
			resp := map[string]interface{}{
				"id":     "pred-round54-ok",
				"status": "succeeded",
				"output": "Replicate completion text",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClientWithBaseURL("test-key", server.URL+"/v1")
	client.SetPollInterval(5 * time.Millisecond)
	resp, err := client.Generate(context.Background(), &llm.LLMRequest{
		ID: uuid.New(), Model: "meta/meta-llama-3-70b-instruct",
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Err, "Err MUST be nil for status=succeeded")
	assert.Contains(t, resp.Content, "Replicate completion text",
		"Content MUST carry the prediction output")
}

// TestRound54_Replicate_Generate_StatusCanceled_LeavesErrNil asserts
// that status="canceled" (caller-initiated cancellation, not an LLM
// error) leaves Err nil.
func TestRound54_Replicate_Generate_StatusCanceled_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/models/") && strings.HasSuffix(r.URL.Path, "/predictions") {
			resp := map[string]interface{}{"id": "pred-round54-canceled", "status": "processing"}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/predictions/") {
			resp := map[string]interface{}{
				"id":     "pred-round54-canceled",
				"status": "canceled",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClientWithBaseURL("test-key", server.URL+"/v1")
	client.SetPollInterval(5 * time.Millisecond)
	resp, err := client.Generate(context.Background(), &llm.LLMRequest{
		ID: uuid.New(), Model: "meta/meta-llama-3-70b-instruct",
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Err, "Err MUST be nil for status=canceled (caller-initiated, not an LLM error)")
}

// TestRound54_Replicate_StatusMapper_AllCases pins the round-54-new
// mapReplicateStatusToErr helper. Per CONST-050(B) paired-mutation: if
// Replicate adds a new terminal status (e.g., "timed_out"), this test
// MUST be extended in the same commit.
func TestRound54_Replicate_StatusMapper_AllCases(t *testing.T) {
	// status=succeeded → nil
	assert.Nil(t, mapReplicateStatusToErr("succeeded", ""))

	// status=canceled → nil (caller-initiated, not an LLM error)
	assert.Nil(t, mapReplicateStatusToErr("canceled", ""))

	// status=failed (no message) → bare ErrReplicatePredictionFailed
	err := mapReplicateStatusToErr("failed", "")
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, llm.ErrReplicatePredictionFailed))
	assert.Equal(t, llm.ErrReplicatePredictionFailed.Error(), err.Error(),
		"bare ErrReplicatePredictionFailed MUST round-trip when no upstream message")

	// status=failed (with message) → wrapped, errors.Is still succeeds
	err = mapReplicateStatusToErr("failed", "model out of memory")
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, llm.ErrReplicatePredictionFailed),
		"wrapped failure MUST still satisfy errors.Is(err, ErrReplicatePredictionFailed)")
	assert.Contains(t, err.Error(), "model out of memory",
		"wrapped failure MUST surface the upstream message for diagnostics")

	// status=processing / starting / empty → nil (non-terminal)
	assert.Nil(t, mapReplicateStatusToErr("processing", ""))
	assert.Nil(t, mapReplicateStatusToErr("starting", ""))
	assert.Nil(t, mapReplicateStatusToErr("", ""))
}
