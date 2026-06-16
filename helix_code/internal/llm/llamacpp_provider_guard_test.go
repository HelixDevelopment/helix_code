package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// llamacpp_provider_guard_test.go — STANDING regression guard (§11.4.135) for
// the round-63 LlamaCPPProvider.Generate ProcessingTime defect.
//
// DEFECT: LlamaCPPProvider.Generate populated the response with
//   ProcessingTime: time.Since(time.Now())
// which always evaluates to ~0 (now − now). Every llama.cpp response therefore
// reported a meaningless near-zero processing time, corrupting downstream
// latency consumers — notably ensemble_provider.go's quality_weighted speed
// bonus (`resp.ProcessingTime > 0 && resp.ProcessingTime < time.Second`) and
// usage analytics. The fix captures a startTime before the HTTP round-trip and
// uses time.Since(startTime).
//
// §11.4.115 polarity switch (env RED_MODE):
//   RED_MODE=1 — reproduce the historical defect on a FAITHFUL pre-fix
//                stand-in (the exact `time.Since(time.Now())` expression the
//                buggy code used) and PROVE it yields a near-zero duration even
//                though the simulated request took a real, measurable amount of
//                wall-clock time. This is the captured reproduction.
//   RED_MODE=0 (DEFAULT, no env) — drive the REAL, fixed Generate against an
//                httptest server that delays its response by a known floor, and
//                assert the reported ProcessingTime reflects that real latency.
//
// The server's deliberate response delay is the ground-truth latency floor: a
// correct ProcessingTime MUST be at least that floor; the buggy ~0 value cannot.
const llamacppLatencyFloor = 60 * time.Millisecond

func TestLlamaCPPProvider_Generate_ProcessingTimeReflectsRealLatency_Guard(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LlamaCPP guard in short mode (SKIP-OK: #short-mode)")
	}

	if os.Getenv("RED_MODE") == "1" {
		// RED reproduction: replay the EXACT pre-fix expression against a real
		// elapsed interval. Simulate the request taking `llamacppLatencyFloor`
		// of wall-clock time, then compute ProcessingTime the buggy way.
		simulatedRequestStart := time.Now()
		time.Sleep(llamacppLatencyFloor)
		// This is verbatim the defective line that shipped in Generate.
		buggyProcessingTime := time.Since(time.Now())
		// Sanity: real wall-clock since the simulated start IS at least the floor...
		require.GreaterOrEqual(t, time.Since(simulatedRequestStart), llamacppLatencyFloor,
			"RED setup invalid: the simulated request did not actually take the floor latency")
		// ...yet the buggy expression reports a near-zero duration, NOT the floor.
		// The reproduction PASSES by demonstrating the defect is present.
		assert.Less(t, buggyProcessingTime, llamacppLatencyFloor,
			"RED reproduction: time.Since(time.Now()) must yield a near-zero duration, proving the defect")
		t.Logf("RED reproduction: simulated request took >=%v but buggy ProcessingTime=%v (defect present)",
			llamacppLatencyFloor, buggyProcessingTime)
		return
	}

	// GREEN guard (default): the REAL fixed code path must report a
	// ProcessingTime that is at least the server's deliberate response delay.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberate latency floor: a correct ProcessingTime cannot be below this.
		time.Sleep(llamacppLatencyFloor)
		response := map[string]interface{}{
			"content": "Hello! How can I help you today?",
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := LlamaConfig{
		Model:         "/path/to/model.gguf",
		ContextSize:   4096,
		ServerHost:    server.URL,
		ServerTimeout: 10 * time.Second,
	}

	provider, err := NewLlamaCPPProvider(config)
	require.NoError(t, err)

	request := &LLMRequest{
		Model:       "llama-7b",
		Messages:    []Message{{Role: "user", Content: "Hello"}},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	response, err := provider.Generate(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, response)

	// The real fixed code MUST report a ProcessingTime at least as large as the
	// server's deliberate delay. The buggy time.Since(time.Now()) ~0 fails this.
	assert.GreaterOrEqual(t, response.ProcessingTime, llamacppLatencyFloor,
		"ProcessingTime (%v) must reflect the real request latency (>= %v); a near-zero value is the round-63 defect",
		response.ProcessingTime, llamacppLatencyFloor)

	// And it must satisfy the ensemble quality_weighted speed-bonus precondition
	// meaningfully — strictly positive (not a few-nanosecond artifact).
	assert.Positive(t, response.ProcessingTime, "ProcessingTime must be strictly positive")
}
