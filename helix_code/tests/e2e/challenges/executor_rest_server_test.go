package challenges

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// executor_rest_server_test.go — HXC-146 reproduce-first regression guard
// (§11.4.115 RED-baseline-on-the-broken-artifact / §11.4.135 standing guard).
//
// RED (historical/pre-fix reproduction, captured as FACT rather than a
// runtime toggle — see docs investigation report referenced by HXC-146 and
// the git diff of this commit for executor.go/executeREST): before this
// fix, executeREST built an interface-flavored prompt and called
// NewLLMClient(execution.Provider, execution.Model, e.apiKeys, ...).Complete(),
// which issues an HTTP POST directly to the raw LLM provider's own public API
// (e.g. https://api.openai.com/..., http://localhost:11434/api/generate for
// Ollama). It NEVER read ChallengeConfig.HelixCodeHost/HelixCodePort and
// NEVER issued any HTTP request against the configured HelixCode server. Had
// TestExecuteREST_DrivesRealHelixCodeServer below been run against that
// pre-fix executeREST, the "handler invoked exactly once" assertion at
// generateHits would have FAILed (generateHits stays 0 forever — the httptest
// server standing in for the HelixCode server is simply never contacted),
// which is the FACTUAL reproduction of the HXC-146 defect.
//
// GREEN (this commit / post-fix, standing regression guard): the same test
// proves executeREST now genuinely POSTs to the configured
// HelixCodeHost:HelixCodePort/api/v1/llm/generate endpoint (the real server's
// own HTTP surface — internal/server/llm_generate.go Server.generateLLM),
// with the expected JSON body, and that the server's response content flows
// all the way through parseAndSaveCode into the result directory.
//
// No live provider or live server network calls are made anywhere in this
// file — every server interaction is against an in-process httptest.Server.

func newHelixCodeTestConfig(t *testing.T, serverURL string) *ChallengeConfig {
	t.Helper()

	host, portStr, err := net.SplitHostPort(strings.TrimPrefix(serverURL, "http://"))
	if err != nil {
		t.Fatalf("failed to split httptest server URL %q: %v", serverURL, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port from httptest server URL %q: %v", serverURL, err)
	}

	config := DefaultChallengeConfig()
	config.HelixCodeHost = host
	config.HelixCodePort = port
	config.ResultsBaseDir = t.TempDir()
	config.LogsBaseDir = t.TempDir()
	config.DefaultTimeout = 10 * time.Second
	// Compilation/test/run validation is out of scope for HXC-146 (which is
	// about wiring the transport, not the generated code's quality) and
	// would make this test depend on the host's Go toolchain behavior.
	config.ValidateCompilation = false
	config.ValidateTests = false
	config.ValidateRun = false
	config.StrictValidation = false

	return config
}

// newTestExecution builds a minimal ChallengeExecution + open log files,
// mirroring the setup ChallengeExecutor.Execute performs, so tests can call
// an interface method (e.g. executeREST) directly without going through
// Execute()'s downstream validation pipeline.
func newTestExecution(t *testing.T, config *ChallengeConfig, iface ChallengeInterface, provider LLMProviderType, model string) (*ChallengeExecution, *os.File, *os.File) {
	t.Helper()

	execution := &ChallengeExecution{
		ID:           "test-exec-id",
		ChallengeID:  "hxc146-test-challenge",
		Interface:    iface,
		Distribution: DistributionSingle,
		Provider:     provider,
		Model:        model,
		StartTime:    time.Now(),
		Status:       StatusRunning,
		Metadata:     make(map[string]interface{}),
	}

	resultDir := filepath.Join(config.ResultsBaseDir, "result")
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		t.Fatalf("failed to create result dir: %v", err)
	}
	execution.ResultDir = resultDir

	logDir := filepath.Join(config.LogsBaseDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}
	execution.LogFile = filepath.Join(logDir, "execution.log")
	execution.RequestLog = filepath.Join(logDir, "requests.log")
	execution.ValidationLog = filepath.Join(logDir, "validation.log")

	logFile, err := os.Create(execution.LogFile)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	t.Cleanup(func() { logFile.Close() })

	requestLog, err := os.Create(execution.RequestLog)
	if err != nil {
		t.Fatalf("failed to create request log: %v", err)
	}
	t.Cleanup(func() { requestLog.Close() })

	return execution, logFile, requestLog
}

// TestExecuteREST_DrivesRealHelixCodeServer is the GREEN standing regression
// guard for HXC-146: executeREST must issue a real HTTP POST to the
// configured HelixCode server's /api/v1/llm/generate endpoint (never a raw
// LLM provider API), with the expected request shape, and the server's
// response content must flow through to the generated result files.
func TestExecuteREST_DrivesRealHelixCodeServer(t *testing.T) {
	var (
		healthHits   int
		generateHits int
		capturedReq  helixCodeGenerateRequest
		capturedAuth string
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET /health, got %s %s", r.Method, r.URL.Path)
		}
		healthHits++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/api/v1/llm/generate", func(w http.ResponseWriter, r *http.Request) {
		generateHits++

		if r.Method != http.MethodPost {
			t.Errorf("expected POST /api/v1/llm/generate, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}

		if err := json.NewDecoder(r.Body).Decode(&capturedReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		capturedAuth = r.Header.Get("Authorization")

		resp := helixCodeGenerateResponse{
			Status:       "success",
			Content:      "```main.go\npackage main\n\nfunc main() {}\n```",
			Provider:     "ollama",
			Model:        "llama3.2",
			FinishReason: "stop",
		}
		resp.Usage.PromptTokens = 42
		resp.Usage.CompletionTokens = 7
		resp.Usage.TotalTokens = 49

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	config := newHelixCodeTestConfig(t, ts.URL)
	config.HelixCodeAuth = "test-auth-token"

	executor := NewChallengeExecutor(config)

	spec := &ChallengeSpec{
		ID:     "hxc146-rest-guard",
		Name:   "HXC-146 REST Guard Challenge",
		Prompt: "Build a tiny REST service.",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// executeREST is called directly (not via Execute()) so this guard stays
	// scoped to HXC-146's transport-wiring concern — Execute()'s downstream
	// documentation/use-case content-quality validators are orthogonal to
	// whether the REST interface actually drives the real server and are
	// exercised by their own tests, not this one.
	execution, logFile, requestLog := newTestExecution(t, config, InterfaceREST, ProviderOllama, "llama3.2")

	execErr := executor.executeREST(ctx, spec, execution, logFile, requestLog)
	if execErr != nil {
		t.Fatalf("executeREST returned unexpected error: %v", execErr)
	}

	// --- GREEN assertion: the real server endpoint was actually hit. ---
	if healthHits < 1 {
		t.Errorf("expected GET /health to be hit at least once (reachability guard), got %d hits", healthHits)
	}
	if generateHits != 1 {
		t.Fatalf("expected POST /api/v1/llm/generate to be hit exactly once, got %d hits — executeREST did not drive the real HelixCode server", generateHits)
	}

	// --- Request shape: proves the call goes to OUR server, not a raw provider. ---
	if capturedReq.Provider != "ollama" {
		t.Errorf("expected provider %q in request body, got %q", "ollama", capturedReq.Provider)
	}
	if capturedReq.Model != "llama3.2" {
		t.Errorf("expected model %q in request body, got %q", "llama3.2", capturedReq.Model)
	}
	if len(capturedReq.Messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(capturedReq.Messages))
	}
	if capturedReq.Messages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got %q", capturedReq.Messages[0].Role)
	}
	if capturedReq.Messages[1].Role != "user" {
		t.Errorf("expected second message role 'user', got %q", capturedReq.Messages[1].Role)
	}
	if !strings.Contains(capturedReq.Messages[1].Content, spec.Prompt) {
		t.Errorf("expected user message to contain the original prompt %q, got %q", spec.Prompt, capturedReq.Messages[1].Content)
	}
	if !strings.Contains(capturedReq.Messages[1].Content, "REST API application") {
		t.Errorf("expected REST-flavored prompt augmentation in user message, got %q", capturedReq.Messages[1].Content)
	}

	// --- Auth header flows from ChallengeConfig.HelixCodeAuth. ---
	if capturedAuth != "Bearer test-auth-token" {
		t.Errorf("expected Authorization header %q, got %q", "Bearer test-auth-token", capturedAuth)
	}

	// --- The server's response content flowed through parseAndSaveCode. ---
	mainGoPath := filepath.Join(execution.ResultDir, "main.go")
	data, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("expected generated file %s from the server's response content, got error: %v", mainGoPath, err)
	}
	if !strings.Contains(string(data), "package main") {
		t.Errorf("expected generated main.go to contain the server's response content, got: %s", string(data))
	}

	if execution.Metrics.Requests != 1 {
		t.Errorf("expected Metrics.Requests == 1, got %d", execution.Metrics.Requests)
	}

	// --- Request/response log documents the real server URL, not a provider URL. ---
	reqLogData, err := os.ReadFile(execution.RequestLog)
	if err != nil {
		t.Fatalf("failed to read request log: %v", err)
	}
	reqLog := string(reqLogData)
	expectedURL := ts.URL + "/api/v1/llm/generate"
	if !strings.Contains(reqLog, expectedURL) {
		t.Errorf("expected request log to reference the real server URL %q, got:\n%s", expectedURL, reqLog)
	}
	for _, providerHost := range []string{"api.openai.com", "api.anthropic.com", "generativelanguage.googleapis.com", "api.x.ai", "api.deepseek.com"} {
		if strings.Contains(reqLog, providerHost) {
			t.Errorf("request log unexpectedly references a raw provider host %q — REST interface must only talk to the HelixCode server", providerHost)
		}
	}
	if !strings.Contains(reqLog, `"real_server": true`) {
		t.Errorf("expected request/response log to record real_server: true, got:\n%s", reqLog)
	}
}

// TestExecuteREST_SkipsWhenServerUnreachable proves the honest §11.4.3 SKIP
// guard: when the configured HelixCode server cannot be reached, executeREST
// must neither fail in a way that looks like a real bug nor fabricate a
// result — it must return a wrapped ErrHelixCodeServerUnreachable, mark the
// execution StatusSkipped, log a SKIP-OK reason, and never write any
// generated files.
func TestExecuteREST_SkipsWhenServerUnreachable(t *testing.T) {
	// Start a server, capture its address, then close it immediately so the
	// address reliably refuses connections (a well-known Go testing idiom
	// for "nothing is listening here").
	ts := httptest.NewServer(http.NewServeMux())
	unreachableURL := ts.URL
	ts.Close()

	config := newHelixCodeTestConfig(t, unreachableURL)

	executor := NewChallengeExecutor(config)

	spec := &ChallengeSpec{
		ID:     "hxc146-rest-skip-guard",
		Name:   "HXC-146 REST Skip Guard Challenge",
		Prompt: "Build a tiny REST service.",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	execution, err := executor.Execute(ctx, spec, InterfaceREST, DistributionSingle, ProviderOllama, "llama3.2")
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if execution == nil {
		t.Fatal("Execute returned nil execution")
	}

	if execution.Status != StatusSkipped {
		t.Errorf("expected execution status %q when server unreachable, got %q (error: %s)", StatusSkipped, execution.Status, execution.Error)
	}
	if !strings.Contains(execution.Error, ErrHelixCodeServerUnreachable.Error()) {
		t.Errorf("expected execution error to wrap ErrHelixCodeServerUnreachable, got: %s", execution.Error)
	}

	logData, err := os.ReadFile(execution.LogFile)
	if err != nil {
		t.Fatalf("failed to read execution log: %v", err)
	}
	if !strings.Contains(string(logData), "SKIP-OK") {
		t.Errorf("expected execution log to contain a SKIP-OK marker (§11.4.3), got:\n%s", string(logData))
	}

	// Never fabricate a result: no generated code files in the result
	// directory. "execution-metadata.json" is Execute()'s own bookkeeping
	// artifact (written for every execution, skipped or not) — not
	// generated code, so it is excluded from this check.
	entries, err := os.ReadDir(execution.ResultDir)
	if err != nil {
		t.Fatalf("failed to read result dir: %v", err)
	}
	var unexpected []string
	for _, e := range entries {
		if e.Name() != "execution-metadata.json" {
			unexpected = append(unexpected, e.Name())
		}
	}
	if len(unexpected) != 0 {
		t.Errorf("expected no generated code files when the server is unreachable, found: %v", unexpected)
	}
}

// TestCheckHelixCodeServerReachable_ErrorIsWrapped is a narrow unit-level
// guard that errors.Is(err, ErrHelixCodeServerUnreachable) actually works —
// this is the sentinel the Execute()/SKIP-routing switch depends on.
func TestCheckHelixCodeServerReachable_ErrorIsWrapped(t *testing.T) {
	config := DefaultChallengeConfig()
	config.HelixCodeHost = "127.0.0.1"
	config.HelixCodePort = 1 // reserved/unlikely-to-be-listening port
	config.DefaultTimeout = 2 * time.Second

	executor := NewChallengeExecutor(config)

	err := executor.checkHelixCodeServerReachable(context.Background())
	if err == nil {
		t.Fatal("expected an error probing an unreachable server, got nil")
	}
	if !errors.Is(err, ErrHelixCodeServerUnreachable) {
		t.Errorf("expected errors.Is(err, ErrHelixCodeServerUnreachable) to be true, got: %v", err)
	}
}
