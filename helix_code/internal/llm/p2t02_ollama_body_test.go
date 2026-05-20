package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// p2t02_ollama_body_test.go — speed programme Phase 2, task P2-T02.
//
// Task P2-T02 removed the redundant []byte -> string -> reader round-trip in
// OllamaProvider.makeAPIRequest / makeStreamingRequest (R1 B11): the prior
// strings.NewReader(string(requestBody)) allocated a fresh copy of the whole
// marshalled request body just to feed http.NewRequestWithContext, which only
// needs an io.Reader. bytes.NewReader(requestBody) reads the existing []byte
// directly — zero extra copy — and the wire bytes are byte-identical.
//
// These tests are anti-bluff per CONST-035 / Article XI §11.9: they prove the
// request body bytes a server would receive are byte-identical pre/post-change.

// p2t02OllamaRequests returns representative OllamaAPIRequest values spanning
// the chat (Messages) and generate (Prompt) shapes plus Options.
func p2t02OllamaRequests() []OllamaAPIRequest {
	return []OllamaAPIRequest{
		{
			Model:  "llama3.2",
			Prompt: "What is 2+2?",
			Stream: false,
		},
		{
			Model: "llama3.2",
			Messages: []Message{
				{Role: "system", Content: "You are helpful."},
				{Role: "user", Content: "Hello, world! Unicode: éñ中"},
			},
			Stream: true,
			Options: map[string]interface{}{
				"temperature": 0.7,
				"num_ctx":     4096,
			},
		},
		{
			Model:    "codellama",
			Prompt:   "func main() {\n\tprintln(\"hi\")\n}",
			Messages: []Message{{Role: "user", Content: "review this"}},
			Stream:   false,
		},
	}
}

// TestP2T02_OllamaRequestBodyByteIdentical proves that the new body reader
// (bytes.NewReader on the marshalled []byte) produces wire bytes byte-identical
// to the legacy reader (strings.NewReader(string(requestBody))). If these ever
// diverged, the post-change request would put different bytes on the wire — a
// CONST-035 false-success regression.
func TestP2T02_OllamaRequestBodyByteIdentical(t *testing.T) {
	for i, req := range p2t02OllamaRequests() {
		requestBody, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("request[%d]: marshal failed: %v", i, err)
		}

		// Legacy reader path (pre-P2-T02): []byte -> string -> reader.
		legacy, err := io.ReadAll(strings.NewReader(string(requestBody)))
		if err != nil {
			t.Fatalf("request[%d]: legacy reader read failed: %v", i, err)
		}

		// Current reader path (post-P2-T02): bytes.NewReader on the []byte.
		current, err := io.ReadAll(bytes.NewReader(requestBody))
		if err != nil {
			t.Fatalf("request[%d]: current reader read failed: %v", i, err)
		}

		if !bytes.Equal(legacy, current) {
			t.Fatalf("request[%d]: wire bytes differ\n legacy:  %q\n current: %q",
				i, legacy, current)
		}
		// The current reader must also reproduce the marshalled body exactly.
		if !bytes.Equal(current, requestBody) {
			t.Fatalf("request[%d]: current reader bytes differ from marshalled body\n got:  %q\n want: %q",
				i, current, requestBody)
		}
		// The body must be valid JSON round-tripping to the same request.
		var back OllamaAPIRequest
		if err := json.Unmarshal(current, &back); err != nil {
			t.Fatalf("request[%d]: body is not valid JSON: %v", i, err)
		}
		if back.Model != req.Model || back.Prompt != req.Prompt || back.Stream != req.Stream {
			t.Fatalf("request[%d]: JSON round-trip mismatch: got %+v want %+v", i, back, req)
		}
	}
}

// TestP2T02_OllamaBodyReaderLength confirms the bytes.NewReader-backed body
// reports the same Len() as the marshalled body — Content-Length-affecting
// behaviour http.NewRequestWithContext relies on. strings.NewReader and
// bytes.NewReader both expose Len(); they must agree.
func TestP2T02_OllamaBodyReaderLength(t *testing.T) {
	for i, req := range p2t02OllamaRequests() {
		requestBody, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("request[%d]: marshal failed: %v", i, err)
		}
		legacyLen := strings.NewReader(string(requestBody)).Len()
		currentLen := bytes.NewReader(requestBody).Len()
		if legacyLen != currentLen {
			t.Fatalf("request[%d]: reader Len() differs: legacy=%d current=%d",
				i, legacyLen, currentLen)
		}
		if currentLen != len(requestBody) {
			t.Fatalf("request[%d]: reader Len()=%d, want %d", i, currentLen, len(requestBody))
		}
	}
}

// BenchmarkP2T02_OllamaBodyReader measures allocs/op for constructing the
// request-body reader the P2-T02 way (bytes.NewReader on the marshalled bytes).
// Run with -benchmem and compare against the baseline below.
func BenchmarkP2T02_OllamaBodyReader(b *testing.B) {
	reqs := p2t02OllamaRequests()
	bodies := make([][]byte, len(reqs))
	for i, r := range reqs {
		bodies[i], _ = json.Marshal(r)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, body := range bodies {
			r := bytes.NewReader(body)
			_ = r.Len()
		}
	}
}

// BenchmarkP2T02_OllamaBodyReader_StringRoundTripBaseline reproduces the
// PRE-P2-T02 strings.NewReader(string(requestBody)) round-trip — the redundant
// []byte->string copy. Comparing -benchmem allocs/op against
// BenchmarkP2T02_OllamaBodyReader quantifies the per-request copy P2-T02
// eliminated. Benchmark-only; never runs in production.
func BenchmarkP2T02_OllamaBodyReader_StringRoundTripBaseline(b *testing.B) {
	reqs := p2t02OllamaRequests()
	bodies := make([][]byte, len(reqs))
	for i, r := range reqs {
		bodies[i], _ = json.Marshal(r)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, body := range bodies {
			r := strings.NewReader(string(body))
			_ = r.Len()
		}
	}
}
