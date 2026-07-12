package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/rag"
	"digital.vasic.rag/pkg/retriever"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// llm_rag_test.go — reproduce-first (§11.4.115) coverage for HXC-118's
// server-side RAG wiring.
//
// CONFIRMED GAP (the reason this file exists): internal/server had ZERO RAG
// integration prior to this change — `grep -rl rag internal/server/`
// returned no hits — even though the RAG module (internal/rag/) was fully
// implemented and already wired into the CLI generate path
// (cmd/cli/main.go handleGenerate, behind HELIXCODE_RAG_ENABLED). A user
// calling POST /api/v1/llm/generate or /api/v1/llm/stream never got
// retrieval-augmentation, even with RAG enabled. applyRAGContext + the
// ragAdapterResolver seam (llm_generate.go) close that gap by mirroring the
// CLI's wiring exactly.
//
// CONST-050(A): fixtures below live ONLY in this *_test.go unit file.
// Production code never references them.

// fakeRAGRetriever is a deterministic, network-free retriever.Retriever
// fixture (the same "unit-test-only fake" pattern internal/rag's own
// vectorstore_test.go deterministicEmbedder uses). It records every query
// it was called with — proof RAG genuinely ran vs. was skipped — and
// returns a fixed doc set or a fixed error.
type fakeRAGRetriever struct {
	docs    []retriever.Document
	err     error
	queries []string
}

func (f *fakeRAGRetriever) Retrieve(_ context.Context, query string, _ retriever.Options) ([]retriever.Document, error) {
	f.queries = append(f.queries, query)
	if f.err != nil {
		return nil, f.err
	}
	return f.docs, nil
}

// withRAGAdapter temporarily points the package-level ragAdapterResolver
// seam at a fixed Adapter, restoring the real (rag.NewFromEnv) resolver on
// cleanup — the same test-injection pattern withFakeResolver
// (llm_generate_regression_test.go) already uses for llmProviderResolver.
func withRAGAdapter(t *testing.T, a *rag.Adapter) {
	t.Helper()
	prev := ragAdapterResolver
	ragAdapterResolver = func() *rag.Adapter { return a }
	t.Cleanup(func() { ragAdapterResolver = prev })
}

// --- Unit-level coverage of applyRAGContext directly ------------------------

// TestApplyRAGContext_Disabled_LeavesRequestByteIdentical proves the
// default-OFF posture: a disabled Adapter must never call the underlying
// retriever and must leave the request's prompt byte-identical.
func TestApplyRAGContext_Disabled_LeavesRequestByteIdentical(t *testing.T) {
	fake := &fakeRAGRetriever{docs: []retriever.Document{{ID: "d1", Content: "must never be used", Source: "src"}}}
	adapter := rag.NewAdapter(fake) // SetEnabled never called -> default-OFF

	req := &llm.LLMRequest{Messages: []llm.Message{{Role: "user", Content: "what is 2+2?"}}}
	applyRAGContext(context.Background(), adapter, req)

	assert.Equal(t, "what is 2+2?", req.Messages[0].Content,
		"disabled adapter must leave the prompt byte-identical")
	assert.Empty(t, fake.queries, "disabled adapter must never call the underlying retriever")
}

// TestApplyRAGContext_Enabled_AugmentsPromptWithRetrievedContext proves the
// enabled path performs a real retrieval and prepends the retrieved
// document content ahead of the original (still-present) prompt, using
// rag.PrependContext's exact contract.
func TestApplyRAGContext_Enabled_AugmentsPromptWithRetrievedContext(t *testing.T) {
	fake := &fakeRAGRetriever{docs: []retriever.Document{
		{ID: "d1", Content: "HelixCode uses Go 1.26.", Source: "docs/go-version.md"},
	}}
	adapter := rag.NewAdapter(fake)
	adapter.SetEnabled(true)

	prompt := "what Go version does HelixCode use?"
	req := &llm.LLMRequest{Messages: []llm.Message{{Role: "user", Content: prompt}}}
	applyRAGContext(context.Background(), adapter, req)

	require.Len(t, fake.queries, 1, "enabled adapter must call the real retriever exactly once")
	assert.Equal(t, prompt, fake.queries[0], "retriever must be queried with the real user prompt")

	got := req.Messages[0].Content
	assert.Contains(t, got, "HelixCode uses Go 1.26.", "augmented prompt must contain the retrieved document content")
	assert.Contains(t, got, prompt, "augmented prompt must still contain the ORIGINAL prompt verbatim")
	assert.Less(t, strings.Index(got, "HelixCode uses Go 1.26."), strings.Index(got, prompt),
		"retrieved context must be PREPENDED ahead of the original prompt (rag.PrependContext contract)")
}

// TestApplyRAGContext_Enabled_RetrievalError_DegradesGracefully proves the
// ANTI-BLUFF graceful-degrade requirement (§11.4.6): a retrieval failure
// must never panic and must never corrupt/replace the original prompt —
// generation proceeds on the unaugmented prompt.
func TestApplyRAGContext_Enabled_RetrievalError_DegradesGracefully(t *testing.T) {
	fake := &fakeRAGRetriever{err: errors.New("embedding endpoint unreachable")}
	adapter := rag.NewAdapter(fake)
	adapter.SetEnabled(true)

	original := "what is 2+2?"
	req := &llm.LLMRequest{Messages: []llm.Message{{Role: "user", Content: original}}}

	require.NotPanics(t, func() { applyRAGContext(context.Background(), adapter, req) },
		"a retrieval failure must NEVER panic the request")
	assert.Equal(t, original, req.Messages[0].Content,
		"a retrieval failure must degrade gracefully — the ORIGINAL prompt reaches the provider unmodified")
}

// TestApplyRAGContext_Enabled_EmptyMessages_NoOp proves the defensive nil/
// empty-messages guard does not panic on a malformed/empty request.
func TestApplyRAGContext_Enabled_EmptyMessages_NoOp(t *testing.T) {
	fake := &fakeRAGRetriever{docs: []retriever.Document{{ID: "d1", Content: "x"}}}
	adapter := rag.NewAdapter(fake)
	adapter.SetEnabled(true)

	req := &llm.LLMRequest{}
	require.NotPanics(t, func() { applyRAGContext(context.Background(), adapter, req) })
	assert.Empty(t, fake.queries, "no messages -> no query to retrieve for")
}

// --- End-to-end coverage: RED->GREEN through the real HTTP handlers --------

// TestGenerateLLM_RAG_DisabledByDefault_PromptByteIdentical proves the
// default (HELIXCODE_RAG_ENABLED unset) POST /api/v1/llm/generate request
// reaches the provider with the EXACT same prompt buildLLMRequest produced
// — no RAG augmentation, no behavior change for callers who never opt in.
func TestGenerateLLM_RAG_DisabledByDefault_PromptByteIdentical(t *testing.T) {
	t.Setenv(rag.EnvEnabled, "") // explicit: the safe, unset default
	withRAGAdapter(t, rag.NewFromEnv(os.Getenv))

	fake := &wireFacadeFakeProvider{content: "4", finish: "stop"}
	withFakeResolver(t, fake)

	srv := &Server{}
	w, body := postJSON(t, "/api/v1/llm/generate", srv.generateLLM, `{"prompt":"what is 2+2?"}`)

	require.Equal(t, http.StatusOK, w.Code, "body=%v", body)
	require.NotNil(t, fake.gotReq, "handler must have called the provider")
	require.Len(t, fake.gotReq.Messages, 1)
	assert.Equal(t, "what is 2+2?", fake.gotReq.Messages[0].Content,
		"RAG disabled by default: the prompt reaching the provider must be byte-identical")
}

// TestGenerateLLM_RAG_Enabled_Augments_RegressionGuard is the RED->GREEN
// (§11.4.115) guard for the confirmed HXC-118 gap: internal/server's
// generate endpoint had ZERO RAG integration.
//
// RED_MODE=1 replicates the PRE-FIX server shape exactly: build the request
// the same way generateLLM does (buildLLMRequest) and call the provider
// directly with NO applyRAGContext step in between — the actual code path
// before this change (confirmed by `grep -rl rag internal/server/`
// returning zero hits) — and asserts the prompt reaching the provider is
// NEVER augmented, even with an enabled adapter holding a matching
// document. This is the historic gap.
//
// RED_MODE=0 (default, GREEN guard) drives the REAL, now-wired generateLLM
// handler over a real gin engine + httptest recorder and asserts the
// prompt reaching the provider IS augmented with the retrieved document's
// content.
func TestGenerateLLM_RAG_Enabled_Augments_RegressionGuard(t *testing.T) {
	fixtureDoc := retriever.Document{ID: "d1", Content: "The HelixCode server binary is bin/helixcode.", Source: "docs/build.md"}
	prompt := "what is the HelixCode server binary called?"

	if redMode(t) {
		// A real, enabled adapter WITH a matching document is available in
		// this scope — proving the RED expectation below is "the pre-fix
		// code path never consults it," not "no adapter was configured."
		fake := &fakeRAGRetriever{docs: []retriever.Document{fixtureDoc}}
		_ = rag.NewAdapter(fake) // deliberately constructed, deliberately never wired below

		req := llmGenerateRequest{Prompt: prompt}
		llmReq, verr := req.buildLLMRequest(false)
		require.Empty(t, verr)

		fakeProvider := &wireFacadeFakeProvider{content: "bin/helixcode", finish: "stop"}
		_, genErr := fakeProvider.Generate(context.Background(), llmReq)
		require.NoError(t, genErr)

		require.NotNil(t, fakeProvider.gotReq)
		got := fakeProvider.gotReq.Messages[len(fakeProvider.gotReq.Messages)-1].Content
		require.Equal(t, prompt, got,
			"RED expectation: the pre-fix handler sends the RAW prompt to the provider — no RAG wiring existed")
		require.NotContains(t, got, fixtureDoc.Content,
			"RED expectation: the retrieved document content must be ABSENT from the pre-fix request")
		return
	}

	fake := &fakeRAGRetriever{docs: []retriever.Document{fixtureDoc}}
	adapter := rag.NewAdapter(fake)
	adapter.SetEnabled(true)
	withRAGAdapter(t, adapter)

	fakeProvider := &wireFacadeFakeProvider{content: "bin/helixcode", finish: "stop"}
	withFakeResolver(t, fakeProvider)

	srv := &Server{}
	w, body := postJSON(t, "/api/v1/llm/generate", srv.generateLLM,
		`{"prompt":"`+prompt+`"}`)

	require.Equal(t, http.StatusOK, w.Code, "body=%v", body)
	require.NotNil(t, fakeProvider.gotReq)
	got := fakeProvider.gotReq.Messages[len(fakeProvider.gotReq.Messages)-1].Content
	assert.Contains(t, got, fixtureDoc.Content,
		"GREEN: the wired handler must augment the prompt with the retrieved document")
	assert.Contains(t, got, prompt, "augmented prompt must still contain the original prompt verbatim")
	require.Len(t, fake.queries, 1, "GREEN: the real retriever must have been called exactly once")
	assert.Equal(t, prompt, fake.queries[0])
}

// TestStreamLLM_RAG_Enabled_AugmentsPrompt proves streamLLM (POST
// /api/v1/llm/stream) applies the IDENTICAL RAG wiring as generateLLM —
// the task's "in BOTH generate and stream paths" requirement.
func TestStreamLLM_RAG_Enabled_AugmentsPrompt(t *testing.T) {
	fixtureDoc := retriever.Document{ID: "d1", Content: "Streaming uses Server-Sent Events.", Source: "docs/stream.md"}
	fake := &fakeRAGRetriever{docs: []retriever.Document{fixtureDoc}}
	adapter := rag.NewAdapter(fake)
	adapter.SetEnabled(true)
	withRAGAdapter(t, adapter)

	fakeProvider := &wireFacadeFakeProvider{content: "chunk"}
	withFakeResolver(t, fakeProvider)

	gin.SetMode(gin.TestMode)
	srv := &Server{}
	router := gin.New()
	router.POST("/api/v1/llm/stream", srv.streamLLM)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/api/v1/llm/stream",
		strings.NewReader(`{"prompt":"how does streaming work?"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	require.NotPanics(t, func() { router.ServeHTTP(w, req) })

	require.NotNil(t, fakeProvider.gotReq, "streamLLM must have called the provider")
	got := fakeProvider.gotReq.Messages[len(fakeProvider.gotReq.Messages)-1].Content
	assert.Contains(t, got, fixtureDoc.Content, "streamLLM must apply the same RAG wiring as generateLLM")
	assert.Contains(t, got, "how does streaming work?")
	require.Len(t, fake.queries, 1)

	body := w.Body.String()
	assert.Contains(t, body, "data: [DONE]", "stream must still terminate honestly")
}

// TestStreamLLM_RAG_DisabledByDefault_PromptByteIdentical mirrors the
// generate-path disabled-by-default guard for the stream endpoint.
func TestStreamLLM_RAG_DisabledByDefault_PromptByteIdentical(t *testing.T) {
	t.Setenv(rag.EnvEnabled, "")
	withRAGAdapter(t, rag.NewFromEnv(os.Getenv))

	fakeProvider := &wireFacadeFakeProvider{content: "chunk"}
	withFakeResolver(t, fakeProvider)

	gin.SetMode(gin.TestMode)
	srv := &Server{}
	router := gin.New()
	router.POST("/api/v1/llm/stream", srv.streamLLM)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/api/v1/llm/stream",
		strings.NewReader(`{"prompt":"how does streaming work?"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.NotNil(t, fakeProvider.gotReq)
	got := fakeProvider.gotReq.Messages[len(fakeProvider.gotReq.Messages)-1].Content
	assert.Equal(t, "how does streaming work?", got,
		"RAG disabled by default: streamLLM's prompt must be byte-identical")
}
