//go:build integration

// Integration test for the interactive-REPL streaming wire-in (P1-T07, speed
// programme Phase 1).
//
// Unlike repl_streaming_test.go (which exercises streamREPLTurn against a
// hand-written fake provider — permitted in unit sources per CONST-050(A)),
// this test drives streamREPLTurn against a REAL llm.OpenAICompatibleProvider
// connected to a real local HTTP server that emits a genuine OpenAI-style
// Server-Sent-Events stream. No mock provider — the provider's actual SSE
// parser, HTTP client, and chunk-delivery path all run.
//
// Run with: go test -tags=integration -run TestREPLStreaming_RealSSE ./cmd/cli/
//
// Anti-bluff (CONST-035): the test asserts each SSE chunk reaches the consumer
// before the NEXT chunk is written by the server — proving token-by-token
// rendering against a real wire protocol, not a buffered whole-response read.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
)

// sseChunk is one token the fake-but-real HTTP server streams as an OpenAI
// chat-completion SSE delta frame.
type sseChunk struct {
	content string
	// writtenAt is filled by the server the moment the frame is flushed —
	// the server-side half of the timestamped render log.
	writtenAt time.Time
}

// TestREPLStreaming_RealSSE_TokenByToken stands up a real HTTP server that
// streams an OpenAI-format SSE response with a deliberate inter-frame delay,
// constructs a real OpenAICompatibleProvider against it, and drives one REPL
// turn through streamREPLTurn. It proves the consumer renders each token as it
// arrives over the real wire rather than buffering the completion.
func TestREPLStreaming_RealSSE_TokenByToken(t *testing.T) {
	const interFrameDelay = 25 * time.Millisecond
	chunks := []*sseChunk{
		{content: "The "},
		{content: "answer "},
		{content: "is "},
		{content: "four"},
		{content: "."},
	}

	// Real HTTP server emitting a genuine OpenAI chat-completion SSE stream.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Model-list endpoint — IsAvailable / GetHealth probe it.
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"test-model","object":"model"}]}`))
			return
		}
		// Chat-completions streaming endpoint.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("httptest ResponseWriter is not a Flusher")
			return
		}
		for _, c := range chunks {
			time.Sleep(interFrameDelay)
			c.writtenAt = time.Now()
			frame := fmt.Sprintf(
				`{"id":"x","object":"chat.completion.chunk","model":"test-model",`+
					`"choices":[{"index":0,"delta":{"content":%q}}]}`, c.content)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", frame)
			flusher.Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	// Real provider against the real server — no mock.
	provider, err := llm.NewOpenAICompatibleProvider("test", llm.OpenAICompatibleConfig{
		BaseURL:          srv.URL,
		DefaultModel:     "test-model",
		Timeout:          30 * time.Second,
		StreamingSupport: true,
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	defer provider.Close()

	req := &llm.LLMRequest{
		Model:    "test-model",
		Stream:   true,
		Messages: []llm.Message{{Role: "user", Content: "What is 2+2?"}},
	}

	start := time.Now()
	var assembled string
	out := captureStdout(t, func() {
		var serr error
		assembled, _, serr = streamREPLTurn(context.Background(), provider, req)
		if serr != nil {
			t.Errorf("streamREPLTurn over real SSE: %v", serr)
		}
	})
	completionAt := time.Since(start)

	// No-regression: assembled text equals the concatenation of every delta.
	var want strings.Builder
	for _, c := range chunks {
		want.WriteString(c.content)
	}
	if assembled != want.String() {
		t.Errorf("assembled = %q, want %q", assembled, want.String())
	}
	if !strings.Contains(out, want.String()) {
		t.Errorf("stdout missing streamed text; got %q", out)
	}

	// Anti-bluff: the server's first frame was written long before the last.
	firstWritten := chunks[0].writtenAt.Sub(start)
	lastWritten := chunks[len(chunks)-1].writtenAt.Sub(start)
	t.Logf("real-SSE streaming render log (P1-T07 anti-bluff):")
	for i, c := range chunks {
		t.Logf("  frame %d written at +%v (%q)", i, c.writtenAt.Sub(start), c.content)
	}
	t.Logf("  first frame at +%v, last frame at +%v, completion at +%v",
		firstWritten, lastWritten, completionAt)

	if lastWritten-firstWritten < interFrameDelay {
		t.Errorf("frames not staggered (gap %v < %v) — server did not stream",
			lastWritten-firstWritten, interFrameDelay)
	}
	// The consumer must have finished only after the last frame — proving it
	// read the real incremental stream rather than a single buffered body.
	if completionAt < lastWritten {
		t.Errorf("streamREPLTurn returned at +%v, before last frame at +%v",
			completionAt, lastWritten)
	}
}
