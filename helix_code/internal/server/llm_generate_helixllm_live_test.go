package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/google/uuid"
)

// llm_generate_helixllm_live_test.go — LIVE round-trip proof (§11.4.5 /
// §11.4.69 / §11.4.107) that a REAL completion flows through
// resolveLLMProvider's new "helixllm" local-coder route all the way to the
// actual llama.cpp OpenAI-compatible sidecar (default
// http://localhost:18434), closing the gap
// scratchpad/phase1_dual_wire_facade.md flagged: "resolveLLMProvider ... has
// NO path to the LOCAL HelixLLM coder".
//
// This test is READ-ONLY against the coder — a single real Generate() call,
// no config changes, no writes (§11.4.122) — and honestly SKIPs (never
// fake-PASSes, CONST-035/§11.4.3) when the coder is not reachable, so it runs
// safely as part of the default `go test ./internal/server/...` invocation in
// every environment. No build tag is used: unlike the CONST-039 hosted-cloud
// harness in internal/llm/provider_live_proof_test.go (which build-tags
// itself out of default runs to avoid real API cost), a local loopback call
// to this coder carries zero API cost.
//
// Run:
//
//	cd helix_code && go test -v -run TestResolveLLMProvider_HelixLLMLocal_LiveRoundTrip ./internal/server/...
func helixLLMLocalReachable(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, envHelixLLMLocalEndpoint()+"/v1/models", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

func TestResolveLLMProvider_HelixLLMLocal_LiveRoundTrip(t *testing.T) {
	if !helixLLMLocalReachable(t) {
		t.Skip("SKIP: local HelixLLM coder not reachable at " + envHelixLLMLocalEndpoint() +
			" (set HELIX_LLM_LOCAL_OPENAI_ENDPOINT or start the coder to exercise this proof)")
	}

	// Exercise the EXACT production seam the HTTP handlers use
	// (llmProviderResolver, not resolveLLMProvider directly) so this proof
	// covers the real dispatch path generateLLM/streamLLM/wire_facade calls.
	provider, err := llmProviderResolver("helixllm", "")
	if err != nil {
		t.Fatalf("resolveLLMProvider(\"helixllm\", \"\") failed although the coder answered /v1/models: %v", err)
	}
	defer func() { _ = provider.Close() }()

	model := resolveDefaultModel(provider, "")
	if model == "" {
		t.Fatalf("resolveDefaultModel returned empty model although the coder's /v1/models catalog should be non-empty")
	}

	// Fresh per-run nonce (§11.4.2/§11.4.5 anti-bluff, same technique as
	// internal/llm/provider_live_proof_test.go's providerLiveNonce): a
	// cached/mocked/hardcoded response cannot possibly contain a token that
	// did not exist until this call executed.
	nonceBuf := make([]byte, 6)
	if _, err := rand.Read(nonceBuf); err != nil {
		t.Fatalf("nonce generation failed: %v", err)
	}
	nonce := "HELIXCODE-CODER-ROUTE-" + hex.EncodeToString(nonceBuf)

	req := &llm.LLMRequest{
		ID: uuid.New(),
		Messages: []llm.Message{{
			Role: "user",
			Content: fmt.Sprintf(
				"This is an automated liveness probe for the HelixCode->coder route. "+
					"Reply with EXACTLY this token and nothing else: %s", nonce),
		}},
		Model:       model,
		MaxTokens:   32,
		Temperature: 0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, genErr := provider.Generate(ctx, req)
	if genErr != nil {
		t.Fatalf("provider.Generate against the live coder failed: %v", genErr)
	}
	if resp.Content == "" {
		t.Fatalf("live coder returned empty content — no real completion produced")
	}
	if !strings.Contains(resp.Content, nonce) {
		t.Fatalf("response did not echo nonce %q (got %q) — cannot prove this is a live, non-cached answer", nonce, resp.Content)
	}

	t.Logf(
		"PASS: HelixCode resolveLLMProvider(\"helixllm\") -> REAL completion from the live coder: "+
			"model=%q finish_reason=%q tokens(prompt=%d,completion=%d,total=%d) content=%q",
		model, resp.FinishReason, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, resp.Content,
	)
}
