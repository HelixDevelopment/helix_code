package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// wire_facade_live_e2e_test.go — full-HTTP round-trip proof (§11.4.5 /
// §11.4.69 / §11.4.107 / §11.4.108) that an authenticated OpenAI/Anthropic-
// shaped request to HelixCode's OWN server flows through the REAL
// wireFacadeAuthMiddleware + the REAL chatCompletions/anthropicMessages
// handlers + the REAL resolveLLMProvider("helixllm") routing, all the way to
// the REAL local HelixLLM coder (llama.cpp OpenAI-compatible sidecar,
// default http://localhost:18434), and back — closing the full-HTTP gap the
// routing-to-coder task (commit a21ad7ca, llm_generate_helixllm_test.go /
// llm_generate_helixllm_live_test.go) deferred: that work proved the
// resolveLLMProvider("helixllm") SEAM reaches the coder directly; this file
// proves the SAME thing is reachable by an actual OpenAI/Anthropic client
// speaking real HTTP to this server's real routes.
//
// STAND-UP PATH: httptest.NewServer wrapping the REAL *Server built via
// server.New()-equivalent construction (&Server{config,router} +
// srv.setupRoutes(), the identical fixture wire_facade_auth_test.go already
// uses) — NOT the full daemon binary, which requires a live PostgreSQL +
// Redis this environment does not provision. This is not a shortcut: gin's
// router, wireFacadeAuthMiddleware, chatCompletions/anthropicMessages, and
// llmProviderResolver are ALL the real, unmodified production code; only the
// database/redis-dependent route groups (auth/users/tasks/etc., which the
// wire-facade routes do not depend on) are absent because db==nil. The HTTP
// transport itself is genuinely real: httptest.NewServer opens a real TCP
// listener and serves real net/http requests, and every request below is
// driven by the actual `curl` binary via os/exec — a real HTTP client
// process, not an in-process handler call.
//
// Anti-bluff (§11.4/§11.4.1): no handler-direct call bypassing the router or
// the auth middleware; every assertion below is against real curl output
// captured from a real TCP round trip to a real gin.Engine talking to the
// real, live coder process on :18434 (read-only against the coder — a single
// Generate() call per subtest, no config changes, no writes, §11.4.122). The
// whole test honestly SKIPs (never fake-PASSes) when the coder is
// unreachable, mirroring llm_generate_helixllm_live_test.go's established
// pattern in this same package.
//
// Run:
//
//	cd helix_code && go test -v -run TestWireFacade_FullHTTP_E2E_LiveRoundTrip ./internal/server/...

const wireFacadeE2ETestAPIKey = "test-only-fullhttp-e2e-wire-facade-key-not-a-real-secret"

// wireFacadeE2ENonce returns a fresh per-run nonce token (same technique as
// llm_generate_helixllm_live_test.go's providerLiveNonce / nonceBuf): a
// cached/mocked/hardcoded response cannot possibly contain a token that did
// not exist until this call executed.
func wireFacadeE2ENonce(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	require.NoError(t, err, "nonce generation failed")
	return "HELIXCODE-FULLHTTP-E2E-" + hex.EncodeToString(buf)
}

// wireFacadeE2EEvidenceDir creates (once per test run) the docs/qa evidence
// directory this task's report cites (§11.4.83 — full end-to-end
// communication transcript committed in-repo). Path is relative to this
// package directory (helix_code/internal/server) up to the meta-repo root:
// internal/server -> helix_code (inner app root) -> meta-repo root -> docs/qa.
func wireFacadeE2EEvidenceDir(t *testing.T) string {
	t.Helper()
	ts := time.Now().UTC().Format("20060102T150405Z")
	dir := filepath.Join("..", "..", "..", "docs", "qa", "phase1_fullhttp_e2e_"+ts)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	return dir
}

// curlCapture drives a REAL POST request via the actual `curl` binary
// (os/exec — a genuine external HTTP client process, not an in-process
// handler call) against the given httptest.Server URL, capturing the HTTP
// status code and response body. headers is a flat list of "Name: Value"
// strings passed via repeated curl -H flags.
func curlCapture(t *testing.T, url string, headers []string, body string) (status int, respBody string, rawTranscript string) {
	t.Helper()

	bodyFile, err := os.CreateTemp("", "wire-facade-e2e-body-*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(bodyFile.Name()) }()
	require.NoError(t, bodyFile.Close())

	args := []string{
		"-s", "-S",
		"-X", "POST",
		"-o", bodyFile.Name(),
		"-w", "%{http_code}",
		"--max-time", "60",
		"-H", "Content-Type: application/json",
	}
	for _, h := range headers {
		args = append(args, "-H", h)
	}
	args = append(args, "-d", body, url)

	cmd := exec.Command("curl", args...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "curl invocation failed: %v (output=%s)", err, string(out))

	statusStr := strings.TrimSpace(string(out))
	status, err = strconv.Atoi(statusStr)
	require.NoErrorf(t, err, "curl did not report a numeric HTTP status (got %q)", statusStr)

	respBytes, err := os.ReadFile(bodyFile.Name())
	require.NoError(t, err)
	respBody = string(respBytes)

	var reqHeaderLines strings.Builder
	for _, h := range headers {
		reqHeaderLines.WriteString(h)
		reqHeaderLines.WriteString("\n")
	}
	rawTranscript = fmt.Sprintf(
		"=== REQUEST ===\nPOST %s\nContent-Type: application/json\n%s\n%s\n\n=== RESPONSE ===\nHTTP %d\n%s\n",
		url, reqHeaderLines.String(), body, status, respBody)
	return status, respBody, rawTranscript
}

// wireFacadeE2EFixture builds a real *Server (the SAME construction
// wire_facade_auth_test.go's wireFacadeAuthFixture uses — real gin router +
// real setupRoutes(), so the two wire-facade routes, wireFacadeAuthMiddleware,
// and every other db/redis-independent route are ALL genuinely wired) and
// wraps it in a real httptest.Server (a real TCP listener + real net/http
// serving).
func wireFacadeE2EFixture(t *testing.T) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret-key-for-testing-only",
			TokenExpiry:       3600,
			BcryptCost:        4,
			WireFacadeAPIKeys: wireFacadeE2ETestAPIKey,
		},
		Logging: config.LoggingConfig{Level: "error"},
	}

	srv := &Server{
		config: cfg,
		router: gin.New(),
	}
	srv.setupRoutes()

	ts := httptest.NewServer(srv.router)
	t.Cleanup(ts.Close)
	return ts
}

func TestWireFacade_FullHTTP_E2E_LiveRoundTrip(t *testing.T) {
	if !helixLLMLocalReachable(t) {
		t.Skip("SKIP: local HelixLLM coder not reachable at " + envHelixLLMLocalEndpoint() +
			" (set HELIX_LLM_LOCAL_OPENAI_ENDPOINT or start the coder to exercise this proof)")
	}

	// Route resolveLLMProvider's provider selection to the coder: server.go's
	// wire-facade handlers call llmProviderResolver("", llmReq.Model) — an
	// EMPTY provider-name argument — so resolution falls through to
	// HELIX_LLM_PROVIDER (see resolveLLMProvider's doc-comment: "Honour
	// HELIX_LLM_PROVIDER only when the request did not name a provider").
	t.Setenv("HELIX_LLM_PROVIDER", "helixllm")
	t.Setenv("HELIX_LLM_LOCAL_OPENAI_ENDPOINT", envHelixLLMLocalEndpoint())

	ts := wireFacadeE2EFixture(t)
	evidenceDir := wireFacadeE2EEvidenceDir(t)

	t.Run("no_auth_chat_completions_401", func(t *testing.T) {
		body := `{"messages":[{"role":"user","content":"hi"}]}`
		status, respBody, transcript := curlCapture(t, ts.URL+"/v1/chat/completions", nil, body)
		_ = os.WriteFile(filepath.Join(evidenceDir, "01_no_auth_401.txt"), []byte(transcript), 0o644)

		require.Equalf(t, 401, status, "unauthenticated POST /v1/chat/completions must be rejected 401 (fail-closed) — got %d body=%s", status, respBody)

		var errBody map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(respBody), &errBody))
		errObj, ok := errBody["error"].(map[string]interface{})
		require.True(t, ok, "expected an {error:{...}} body, got %s", respBody)
		require.Equal(t, "authentication_error", errObj["type"])

		t.Logf("PASS: no-auth request to /v1/chat/completions correctly rejected: status=%d body=%s", status, respBody)
	})

	t.Run("openai_chat_completions_authenticated_200", func(t *testing.T) {
		nonce := wireFacadeE2ENonce(t)
		reqBody := fmt.Sprintf(`{"messages":[{"role":"user","content":"This is an automated full-HTTP e2e liveness probe. Reply with EXACTLY this token and nothing else: %s"}],"max_tokens":32,"temperature":0}`, nonce)

		headers := []string{"Authorization: Bearer " + wireFacadeE2ETestAPIKey}
		status, respBody, transcript := curlCapture(t, ts.URL+"/v1/chat/completions", headers, reqBody)
		_ = os.WriteFile(filepath.Join(evidenceDir, "02_openai_chat_completions_200.txt"), []byte(transcript), 0o644)

		require.Equalf(t, 200, status, "authenticated POST /v1/chat/completions must succeed — got %d body=%s", status, respBody)

		var parsed struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}
		require.NoErrorf(t, json.Unmarshal([]byte(respBody), &parsed), "response body must be valid OpenAI-shaped JSON: %s", respBody)

		require.Equal(t, "chat.completion", parsed.Object, "must be OpenAI Chat Completions wire shape")
		require.NotEmpty(t, parsed.ID)
		require.Len(t, parsed.Choices, 1)
		require.Equal(t, "assistant", parsed.Choices[0].Message.Role)
		require.Containsf(t, parsed.Choices[0].Message.Content, nonce,
			"real coder completion must echo the fresh nonce (proves a live, non-cached answer): got %q", parsed.Choices[0].Message.Content)
		require.Containsf(t, strings.ToLower(parsed.Model), "qwen3-coder",
			"resolved model must be the real coder model id, got %q", parsed.Model)
		require.Greater(t, parsed.Usage.PromptTokens, 0, "usage.prompt_tokens must be a real positive count")
		require.Greater(t, parsed.Usage.CompletionTokens, 0, "usage.completion_tokens must be a real positive count")

		t.Logf("PASS: OpenAI-shaped 200 via real HTTP+curl+router+middleware+coder: model=%q finish_reason=%q usage=%+v content=%q",
			parsed.Model, parsed.Choices[0].FinishReason, parsed.Usage, parsed.Choices[0].Message.Content)
	})

	t.Run("anthropic_messages_authenticated_200", func(t *testing.T) {
		nonce := wireFacadeE2ENonce(t)
		reqBody := fmt.Sprintf(`{"messages":[{"role":"user","content":"This is an automated full-HTTP e2e liveness probe. Reply with EXACTLY this token and nothing else: %s"}],"max_tokens":32,"temperature":0}`, nonce)

		headers := []string{"x-api-key: " + wireFacadeE2ETestAPIKey}
		status, respBody, transcript := curlCapture(t, ts.URL+"/v1/messages", headers, reqBody)
		_ = os.WriteFile(filepath.Join(evidenceDir, "03_anthropic_messages_200.txt"), []byte(transcript), 0o644)

		require.Equalf(t, 200, status, "authenticated POST /v1/messages must succeed — got %d body=%s", status, respBody)

		var parsed struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Role    string `json:"role"`
			Model   string `json:"model"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			StopReason string `json:"stop_reason"`
			Usage      struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		require.NoErrorf(t, json.Unmarshal([]byte(respBody), &parsed), "response body must be valid Anthropic-shaped JSON: %s", respBody)

		require.Equal(t, "message", parsed.Type, "must be Anthropic Messages wire shape")
		require.Equal(t, "assistant", parsed.Role)
		require.NotEmpty(t, parsed.ID)
		require.NotEmpty(t, parsed.StopReason)
		require.GreaterOrEqual(t, len(parsed.Content), 1)

		var fullText strings.Builder
		for _, block := range parsed.Content {
			fullText.WriteString(block.Text)
		}
		require.Containsf(t, fullText.String(), nonce,
			"real coder completion must echo the fresh nonce (proves a live, non-cached answer): got %q", fullText.String())
		require.Containsf(t, strings.ToLower(parsed.Model), "qwen3-coder",
			"resolved model must be the real coder model id, got %q", parsed.Model)
		require.Greater(t, parsed.Usage.InputTokens, 0, "usage.input_tokens must be a real positive count")
		require.Greater(t, parsed.Usage.OutputTokens, 0, "usage.output_tokens must be a real positive count")

		t.Logf("PASS: Anthropic-shaped 200 via real HTTP+curl+router+middleware+coder: model=%q stop_reason=%q usage=%+v content=%q",
			parsed.Model, parsed.StopReason, parsed.Usage, fullText.String())
	})

	t.Logf("evidence captured under %s", evidenceDir)
}
