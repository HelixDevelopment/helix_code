package llm

// deepseek_stream_guard_test.go — standing regression guard (§11.4.135) for the
// DeepSeek streaming SSE-parse fix (commit 74077736, "fix(llm): DeepSeek
// streaming — parse SSE data: lines"), which shipped WITHOUT a test. Added per
// §11.4.135 (every fixed defect gets a permanent regression test) after the
// §11.4.153 feature-video sweep surfaced the defect on a STALE bin/cli built
// before 74077736 (a §11.4.108 source-vs-artifact gap: the source fix existed,
// the running binary predated it).
//
// DEFECT (reproduced): DeepSeek (OpenAI-compatible) streams Server-Sent Events —
// each event a `data: {json}` line, blank-line separated, terminated by a
// `data: [DONE]` sentinel. The pre-fix path fed that body to a raw json.Decoder,
// which chokes on the SSE field prefix with "invalid character 'd' looking for
// beginning of value" (the 'd' of `data:`), so `cli -stream` failed entirely
// while non-stream worked.
//
// §11.4.115 polarity switch via RED_MODE:
//   RED_MODE=1 : drive the FAITHFUL pre-fix logic (raw json.Decoder over the SSE
//                body) and PASS when it fails with the SSE-prefix error — proving
//                the guard reproduces a real defect, not a synthetic one.
//   RED_MODE=0 (default) : drive the REAL fixed makeOpenAIStreamRequest (via
//                GenerateStream) against an httptest SSE server and assert the
//                streamed deltas are correctly stripped, [DONE]-terminated, and
//                concatenated — the standing GREEN regression guard.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// sseStreamBody is a faithful DeepSeek/OpenAI-compatible streaming response:
// two `data: {json}` content deltas then the `data: [DONE]` sentinel.
const sseStreamBody = "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n" +
	"data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}]}\n\n" +
	"data: [DONE]\n\n"

func TestDeepSeekStreamSSEParseGuard(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// RED: the pre-fix path = a raw json.Decoder over the SSE body. It MUST
		// fail on the `data:` prefix — exactly the defect the video surfaced.
		var anyResp OpenAIStreamResponse
		err := json.NewDecoder(strings.NewReader(sseStreamBody)).Decode(&anyResp)
		if err == nil {
			t.Fatal("RED_MODE: raw json.Decoder unexpectedly parsed an SSE `data:` body — " +
				"the reproduction is blind (the pre-fix defect requires this to fail)")
		}
		if !strings.Contains(err.Error(), "invalid character 'd'") {
			t.Fatalf("RED_MODE: expected the SSE-prefix decode error \"invalid character 'd'\", got %q", err.Error())
		}
		t.Logf("RED_MODE: pre-fix raw-decode correctly fails on the SSE body: %v (defect reproduced)", err)
		return
	}

	// GREEN (default): the REAL fixed makeOpenAIStreamRequest (reached via
	// GenerateStream) must parse the SSE stream — strip `data:`, honor `[DONE]`,
	// and emit the concatenated content deltas on the channel.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseStreamBody))
	}))
	defer srv.Close()

	dp, err := NewDeepSeekProvider(ProviderConfigEntry{Endpoint: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewDeepSeekProvider: %v", err)
	}

	ch := make(chan LLMResponse, 16)
	req := &LLMRequest{
		ID:       uuid.New(),
		Model:    "deepseek-v4-flash",
		Messages: []Message{{Role: "user", Content: "hi"}},
		Stream:   true,
	}
	// GenerateStream is synchronous here and closes ch on return; the two content
	// deltas fit the buffered channel so it never blocks without a reader.
	if err := dp.GenerateStream(context.Background(), req, ch); err != nil {
		t.Fatalf("GenerateStream over the SSE httptest server failed (the very defect this guards): %v", err)
	}
	var got strings.Builder
	for resp := range ch {
		got.WriteString(resp.Content)
	}
	if got.String() != "Hello world" {
		t.Fatalf("SSE deltas mis-parsed: want %q, got %q", "Hello world", got.String())
	}
	t.Logf("GREEN: makeOpenAIStreamRequest correctly parsed the SSE stream → %q", got.String())
}
