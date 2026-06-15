//go:build integration

package integration

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// llm_stream_e2e_test.go — REAL end-to-end exercise of the streaming
// POST /api/v1/llm/stream web endpoint (server.streamLLM) against a LIVE local
// Ollama, asserting genuine STREAMED LLM output reaches the HTTP caller over the
// wire as it is produced — not as one buffered write.
//
// Anti-bluff (CONST-035 / BLUFF-001 / Article XI §11.9 / CONST-050(A) /
// §11.4 / §11.4.107): this test makes NO stub, NO fake provider, NO canned
// response, and NO mock. It boots the REAL HTTP server via
// server.New(...) (which registers the real /api/v1/llm/stream route through
// setupRoutes) on a real TCP port, then issues a REAL http.Client POST and reads
// the response body INCREMENTALLY. The server-side path is the exact production
// code:
//
//   streamLLM -> resolveLLMProvider (default local Ollama, no provider named)
//     -> llm.NewOllamaProvider -> provider.GenerateStream -> REAL HTTP POST to
//     http://localhost:11434/api/chat (stream=true) -> a real model produces
//     real tokens -> streamProviderToSSE forwards each non-empty chunk as an SSE
//     `data: <content>\n\n` frame (newlines in a chunk escaped to "\n") and ends
//     with a terminal `data: [DONE]\n\n`.
//
// Framing found in internal/server/llm_generate.go:
//   - streamLLM             (llm_generate.go:196) sets Content-Type
//     text/event-stream and pumps provider chunks via streamProviderToSSE.
//   - streamProviderToSSE   (llm_generate.go:247) writes one `data: <chunk>\n\n`
//     SSE frame per non-empty chunk.Content, flushing after each, then
//     `data: [DONE]\n\n` when the provider channel drains.
//   - sseEscape             (llm_generate.go:280) replaces "\n" with "\\n" inside
//     a chunk so a multi-line token stays one logical SSE data line.
//
// This is the streaming counterpart to TestLLMGenerateE2E (the non-streaming
// generate endpoint) in llm_generate_e2e_test.go and reuses that file's
// package-level helpers (ollamaEndpoint, liveOllamaModel, freePort,
// minimalServerConfig, itoa) — they are declared once for package `integration`.
//
// Run:
//   go test -tags=integration -run TestLLMStreamE2E ./tests/integration/ -count=1 -v
//
// Per CONST-050(A) integration tests exercise the real system. If Ollama is not
// reachable OR no model is installed, the test SKIPs with an explicit reason
// (SKIP-OK) per §11.4.3 rather than bluffing a PASS — an honest documented
// absence, never a fabricated success.

// sseFrame is a single decoded SSE `data:` payload, with the streamLLM newline
// escaping (\n -> literal "\n") reversed so the original chunk text is restored.
type sseFrame struct {
	data string // the model-produced chunk content (newlines un-escaped)
	done bool   // true for the terminal `data: [DONE]` sentinel
}

// readSSEFrames consumes a text/event-stream body line by line, returning every
// `data:` frame in arrival order. It reads the live stream incrementally via a
// bufio.Scanner so frames are captured as they are flushed by the server — the
// genuine streaming path, not a single buffered read.
func readSSEFrames(t *testing.T, body *bufio.Reader) (frames []sseFrame, errorEvent string) {
	t.Helper()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var lastEvent string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			// SSE event boundary (blank line) — nothing to collect.
			continue
		case strings.HasPrefix(line, "event:"):
			lastEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			payload := strings.TrimPrefix(line, "data:")
			payload = strings.TrimPrefix(payload, " ")
			if lastEvent == "error" {
				errorEvent = payload
				lastEvent = ""
				continue
			}
			if payload == "[DONE]" {
				frames = append(frames, sseFrame{done: true})
				continue
			}
			// Reverse sseEscape (llm_generate.go:280): literal "\n" -> newline.
			frames = append(frames, sseFrame{data: strings.ReplaceAll(payload, "\\n", "\n")})
		}
	}
	// scanner.Err() of nil (incl. io.EOF) means the stream closed cleanly.
	require.NoError(t, scanner.Err(), "reading the live SSE stream must not error at the transport level")
	return frames, errorEvent
}

// TestLLMStreamE2E boots the real server and POSTs a real request to the live
// streaming endpoint, asserting a genuine text/event-stream of real model output
// arriving as multiple SSE `data:` frames terminated by `[DONE]`.
func TestLLMStreamE2E(t *testing.T) {
	model, reachable := liveOllamaModel(t)
	if !reachable {
		t.Skip("SKIP-OK: local Ollama not reachable at " + ollamaEndpoint + "; cannot exercise the real streaming path") //nolint
	}
	if model == "" {
		t.Skip("SKIP-OK: local Ollama is reachable but no model is installed; pull a model (e.g. `ollama pull qwen2.5:0.5b`) to exercise streaming") //nolint
	}
	t.Logf("targeting live Ollama model %q via %s", model, ollamaEndpoint)

	// Ensure no cloud provider is named so resolveLLMProvider falls to the local
	// Ollama default — the exact out-of-the-box server path.
	t.Setenv("HELIX_LLM_PROVIDER", "")

	port := freePort(t)
	srv := server.New(minimalServerConfig(port), nil, nil)

	// Start the real HTTP server in the background; stop it at test end.
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Start() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	base := "http://127.0.0.1:" + itoa(port)

	// Wait for the listener to accept connections (or fail fast on serve error).
	require.Eventually(t, func() bool {
		select {
		case err := <-serveErr:
			t.Fatalf("server failed to start: %v", err)
			return false
		default:
		}
		c, err := net.DialTimeout("tcp", "127.0.0.1:"+itoa(port), 200*time.Millisecond)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 10*time.Second, 100*time.Millisecond, "server must come up on its port")

	// Cap max_tokens so a small model cannot ramble past the window (§11.4.98
	// determinism): "count 1 to 5" + a 64-token ceiling streams several chunks
	// and terminates with [DONE] in a few seconds, not an open-ended generation.
	bodyJSON := `{"prompt":"Count from 1 to 5, one number per line.","model":"` + model + `","max_tokens":64}`
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/llm/stream", bytes.NewBufferString(bodyJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "the real HTTP POST to the stream endpoint must succeed at the transport level")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"live Ollama must yield a real 200 from the stream endpoint")
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream",
		"streaming endpoint must announce an SSE content type")

	// Read the STREAMED body incrementally — captures frames as the server flushes
	// them, proving genuine streaming rather than a single buffered write.
	frames, errorEvent := readSSEFrames(t, bufio.NewReader(resp.Body))
	require.Empty(t, errorEvent, "live streaming must not emit an SSE error event; got %q", errorEvent)

	// Separate content frames from the terminal [DONE] sentinel.
	var contentFrames []string
	sawDone := false
	var aggregated strings.Builder
	for _, f := range frames {
		if f.done {
			sawDone = true
			continue
		}
		contentFrames = append(contentFrames, f.data)
		aggregated.WriteString(f.data)
	}
	content := aggregated.String()

	t.Logf("received %d content SSE frame(s), DONE-sentinel=%v", len(contentFrames), sawDone)
	t.Logf("aggregated streamed content:\n%s", content)
	for i, cf := range contentFrames {
		t.Logf("  frame[%d]=%q", i, cf)
	}

	// Anti-bluff proof #1: the stream MUST terminate with the [DONE] sentinel,
	// proving the server reached streamProviderToSSE's clean channel-drain path.
	require.True(t, sawDone, "the SSE stream must end with a `data: [DONE]` sentinel")

	// Anti-bluff proof #2: real, non-empty model output reached the caller.
	require.NotEmpty(t, strings.TrimSpace(content),
		"real streamed model output must be non-empty")

	// Anti-bluff proof #3: it genuinely STREAMED. A single buffered write would
	// arrive as exactly one content frame; a real token stream flushes many. The
	// live model emits the count token-by-token, so MORE THAN ONE content frame
	// must arrive — this is the load-bearing "it streamed" assertion (§11.4.107).
	require.Greater(t, len(contentFrames), 1,
		"streaming must deliver MORE THAN ONE content frame (token-by-token); got %d frame(s): %q",
		len(contentFrames), contentFrames)

	// Anti-bluff proof #4: the aggregated content is the model's REAL answer to
	// "count from 1 to 5" — a canned/simulated response would not reliably contain
	// the counted digits. Assert on real model output, not metadata.
	assert.Contains(t, content, "1",
		"the live model's real count must contain '1'; got %q", content)
	assert.Contains(t, content, "5",
		"the live model's real count must contain '5'; got %q", content)

	// Anti-bluff proof #5: no fabricated/canned marker leaked into the stream
	// (guards against a regression to BLUFF-001 simulated output).
	lower := strings.ToLower(content)
	for _, marker := range []string{"simulated", "this is a simulated", "for now", "placeholder response"} {
		assert.NotContains(t, lower, marker,
			"streamed content must be real model output, not a canned marker %q; got %q", marker, content)
	}
}
