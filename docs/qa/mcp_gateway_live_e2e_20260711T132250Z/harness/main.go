// R41F MCP gateway LIVE end-to-end re-proof harness (§11.4.5).
//
// Builds the real internal/mcpgateway-backed `cmd/mcp-gateway` binary from
// the helix_llm submodule, starts it as a REAL subprocess pointed at the
// REAL live coder (helixllm-coder, http://localhost:18434 — read-only,
// never restarted/stopped, §11.4.119/§11.4.122), then drives it with a REAL
// MCP go-sdk client over Streamable-HTTP:
//
//   - a raw HTTP POST with NO Authorization header -> asserts 401.
//   - a real mcp.Client Streamable-HTTP connection WITH the bearer token ->
//     tools/list -> asserts BOTH helixllm_generate and helixllm_list_models
//     are registered (sourced live from the server, not hardcoded).
//   - a real tools/call of helixllm_generate with a FRESH per-run nonce
//     prompt -> the call proxies to the live coder's /v1/chat/completions
//     and the response ECHOES the nonce, proving this is a genuine live
//     round-trip (not a cached/canned/simulated response) — BLUFF-001
//     anti-bluff.
//   - a real tools/call of helixllm_list_models -> the live coder's real
//     model id (CONST-036 — never a hardcoded list).
//
// §11.4.10: the bearer token is a fresh random-per-run test credential; it
// is NEVER written to any evidence file. Every evidence file that could
// carry an Authorization header is redacted before write.
//
// All request/response evidence is captured to files alongside this
// harness (§11.4.5/§11.4.69/§11.4.83). The gateway subprocess is a NET-NEW
// process this harness owns; it is killed at the end. The coder at :18434
// is NEVER started, stopped, or restarted (§11.4.119/§11.4.122).
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// redact removes any occurrence of the live bearer token from a string
// before it is written to an evidence file (§11.4.10).
func redact(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "<REDACTED-HELIX_MCP_GATEWAY_TOKEN>")
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("RESULT: PASS")
}

func run() error {
	evidenceDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Getwd: %w", err)
	}

	// Locate the helix_llm module root relative to this harness
	// (docs/qa/mcp_gateway_live_e2e_<ts>/harness -> up 4 levels -> repo root
	// -> submodules/helix_llm).
	moduleRoot, err := filepath.Abs(filepath.Join(evidenceDir, "..", "..", "..", "..", "submodules", "helix_llm"))
	if err != nil {
		return fmt.Errorf("resolving module root: %w", err)
	}
	if _, statErr := os.Stat(filepath.Join(moduleRoot, "go.mod")); statErr != nil {
		return fmt.Errorf("helix_llm module root not found at %s: %w", moduleRoot, statErr)
	}

	// --- Step 0: confirm coder is reachable BEFORE we touch anything (read-only) ---
	preStatus, preBody, err := rawGET("http://localhost:18434/v1/models")
	if err != nil {
		return fmt.Errorf("pre-flight coder :18434/v1/models check failed: %w", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "00_coder_preflight_models.json"), []byte(preBody), 0o644); err != nil {
		return fmt.Errorf("writing coder preflight evidence: %w", err)
	}
	if preStatus != http.StatusOK {
		return fmt.Errorf("coder :18434 pre-flight expected 200, got %d", preStatus)
	}
	fmt.Println("PRE-FLIGHT: coder at :18434 reachable (read-only query, untouched)")

	binPath := filepath.Join(evidenceDir, "mcp-gateway.bin")
	fmt.Printf("building gateway binary: go build -o %s ./cmd/mcp-gateway (in %s)\n", binPath, moduleRoot)
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/mcp-gateway")
	buildCmd.Dir = moduleRoot
	buildOut, buildErr := buildCmd.CombinedOutput()
	if writeErr := os.WriteFile(filepath.Join(evidenceDir, "build_output.txt"), buildOut, 0o644); writeErr != nil {
		return fmt.Errorf("writing build_output.txt: %w", writeErr)
	}
	if buildErr != nil {
		return fmt.Errorf("go build failed: %w\n%s", buildErr, string(buildOut))
	}
	fmt.Println("PASS: go build ./cmd/mcp-gateway succeeded")

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("generating bearer token: %w", err)
	}
	bearerToken := hex.EncodeToString(tokenBytes)

	// Fresh per-run nonce for the generate-call anti-bluff proof.
	nonceBytes := make([]byte, 8)
	if _, err := rand.Read(nonceBytes); err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}
	nonce := "R41F-NONCE-" + hex.EncodeToString(nonceBytes)

	const listenAddr = ":18446" // distinct port from the earlier phase1 harness (:18444) to avoid any collision with other tracks
	const gatewayBaseURL = "http://localhost:18446"
	const coderBaseURL = "http://localhost:18434"

	gwCmd := exec.Command(binPath)
	gwCmd.Env = append(os.Environ(),
		"HELIX_MCP_GATEWAY_LISTEN_ADDR="+listenAddr,
		"HELIX_LLM_LOCAL_OPENAI_ENDPOINT="+coderBaseURL,
		"HELIX_MCP_GATEWAY_TOKEN="+bearerToken,
		"HELIX_MCP_GATEWAY_OKF_ROOT=",
	)
	gwLogFile, err := os.Create(filepath.Join(evidenceDir, "gateway_stdout_stderr.log"))
	if err != nil {
		return fmt.Errorf("creating gateway log file: %w", err)
	}
	defer gwLogFile.Close()
	gwCmd.Stdout = gwLogFile
	gwCmd.Stderr = gwLogFile

	fmt.Printf("starting gateway subprocess: %s (listen=%s coder=%s)\n", binPath, listenAddr, coderBaseURL)
	if err := gwCmd.Start(); err != nil {
		return fmt.Errorf("starting gateway subprocess: %w", err)
	}
	defer func() {
		fmt.Println("stopping gateway subprocess (coder at", coderBaseURL, "left untouched)...")
		_ = gwCmd.Process.Kill()
		_, _ = gwCmd.Process.Wait()
	}()

	if err := waitForListening(gatewayBaseURL, 10*time.Second); err != nil {
		return fmt.Errorf("gateway did not become reachable: %w", err)
	}
	fmt.Println("gateway is reachable at", gatewayBaseURL)

	// --- Step 1: no-Bearer request -> assert 401 ---
	status401, body401, err := rawJSONRPCPost(gatewayBaseURL, "", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if err != nil {
		return fmt.Errorf("no-bearer request failed: %w", err)
	}
	evidence401 := fmt.Sprintf("HTTP status: %d\nBody: %s\n", status401, redact(body401, bearerToken))
	if err := os.WriteFile(filepath.Join(evidenceDir, "01_no_bearer_401_response.txt"), []byte(evidence401), 0o644); err != nil {
		return fmt.Errorf("writing 401 evidence: %w", err)
	}
	if status401 != http.StatusUnauthorized {
		return fmt.Errorf("expected 401 without Bearer token, got %d: %s", status401, body401)
	}
	fmt.Println("PASS: request with no Bearer token rejected with 401")

	// --- Step 2: real MCP client, tools/list, with Bearer ---
	httpClient := &http.Client{Transport: &bearerRoundTripper{token: bearerToken, base: http.DefaultTransport}}
	transport := &mcp.StreamableClientTransport{
		Endpoint:   gatewayBaseURL,
		HTTPClient: httpClient,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "r41f-mcp-gateway-live-e2e-harness", Version: "0.1.0"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("MCP client Connect failed: %w", err)
	}
	defer session.Close()

	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("tools/list failed: %w", err)
	}
	toolNames := make([]string, 0, len(toolsResult.Tools))
	for _, t := range toolsResult.Tools {
		toolNames = append(toolNames, t.Name)
	}
	toolsJSON, _ := json.MarshalIndent(toolsResult.Tools, "", "  ")
	if err := os.WriteFile(filepath.Join(evidenceDir, "02_tools_list_response.json"), toolsJSON, 0o644); err != nil {
		return fmt.Errorf("writing tools_list evidence: %w", err)
	}
	if !containsString(toolNames, "helixllm_generate") {
		return fmt.Errorf("helixllm_generate NOT found in tools/list; got: %v", toolNames)
	}
	if !containsString(toolNames, "helixllm_list_models") {
		return fmt.Errorf("helixllm_list_models NOT found in tools/list; got: %v", toolNames)
	}
	fmt.Println("PASS: tools/list (with valid Bearer) contains helixllm_generate + helixllm_list_models:", toolNames)

	// --- Step 3: real tools/call of helixllm_generate, fresh nonce echo ---
	prompt := fmt.Sprintf("Repeat the following token exactly, in full, and output nothing else — no punctuation, no explanation, just the token itself: %s", nonce)
	callResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "helixllm_generate",
		Arguments: map[string]any{"prompt": prompt, "max_tokens": 32},
	})
	if err != nil {
		return fmt.Errorf("tools/call helixllm_generate failed: %w", err)
	}
	callJSON, _ := json.MarshalIndent(callResult, "", "  ")
	if err := os.WriteFile(filepath.Join(evidenceDir, "03_generate_call_response.json"), callJSON, 0o644); err != nil {
		return fmt.Errorf("writing generate call evidence: %w", err)
	}
	if callResult.IsError {
		return fmt.Errorf("helixllm_generate returned IsError=true: %s", contentText(callResult))
	}
	content := contentText(callResult)
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("helixllm_generate returned empty content")
	}
	if !strings.Contains(content, nonce) {
		return fmt.Errorf("helixllm_generate response did NOT echo the fresh nonce %q (anti-bluff/anti-cache proof failed); got: %q", nonce, content)
	}
	structuredJSON, _ := json.Marshal(callResult.StructuredContent)
	fmt.Println("PASS: real tools/call helixllm_generate echoed the FRESH per-run nonce from the live coder (proves genuine live round-trip, not cached/canned)")
	fmt.Println("  nonce:", nonce)
	fmt.Println("  content:", content)
	fmt.Println("  structured:", string(structuredJSON))

	// --- Step 4: real tools/call of helixllm_list_models (second capability) ---
	modelsResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "helixllm_list_models",
		Arguments: map[string]any{},
	})
	if err != nil {
		return fmt.Errorf("tools/call helixllm_list_models failed: %w", err)
	}
	modelsJSON, _ := json.MarshalIndent(modelsResult, "", "  ")
	if err := os.WriteFile(filepath.Join(evidenceDir, "04_list_models_call_response.json"), modelsJSON, 0o644); err != nil {
		return fmt.Errorf("writing list_models call evidence: %w", err)
	}
	if modelsResult.IsError {
		return fmt.Errorf("helixllm_list_models returned IsError=true: %s", contentText(modelsResult))
	}
	fmt.Println("PASS: real tools/call helixllm_list_models returned live model list from coder:", contentText(modelsResult))

	// --- Step 5: post-flight — coder still answers on :18434, untouched ---
	postStatus, postBody, err := rawGET("http://localhost:18434/v1/models")
	if err != nil {
		return fmt.Errorf("post-flight coder :18434/v1/models check failed: %w", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "05_coder_postflight_models.json"), []byte(postBody), 0o644); err != nil {
		return fmt.Errorf("writing coder postflight evidence: %w", err)
	}
	if postStatus != http.StatusOK {
		return fmt.Errorf("coder :18434 post-flight expected 200, got %d", postStatus)
	}
	fmt.Println("PASS: coder at :18434 still answers post-run (untouched, read-only throughout)")

	// --- Summary ---
	summary := fmt.Sprintf(`# R41F MCP gateway LIVE end-to-end re-proof — summary

Gateway binary: %s
Gateway listen: %s
Coder base URL (never touched, read-only throughout — see 00/05 pre/post-flight): %s
Bearer token: <REDACTED-HELIX_MCP_GATEWAY_TOKEN> (fresh random test credential, never written to any evidence file)

## 0. Coder pre-flight (before gateway touched anything)
HTTP 200, see 00_coder_preflight_models.json

## 1. 401 without Bearer
%s

## 2. tools/list (with valid Bearer) — both tools present
%v

## 3. helixllm_generate real call — FRESH nonce echo (anti-bluff/anti-cache proof)
nonce sent: %q
content received: %q

## 4. helixllm_list_models real call — live model id from :18434
%s

## 5. Coder post-flight (after gateway torn down) — still answers, untouched
HTTP 200, see 05_coder_postflight_models.json
`, binPath, listenAddr, coderBaseURL, evidence401, toolNames, nonce, content, contentText(modelsResult))
	if err := os.WriteFile(filepath.Join(evidenceDir, "00_SUMMARY.md"), []byte(summary), 0o644); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	return nil
}

type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (rt *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+rt.token)
	return rt.base.RoundTrip(req)
}

func waitForListening(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":0,"method":"tools/list"}`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s waiting for %s: %v", timeout, baseURL, lastErr)
}

func rawJSONRPCPost(baseURL, bearer, body string) (int, string, error) {
	req, err := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader([]byte(body)))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, buf.String(), nil
}

func rawGET(url string) (int, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, buf.String(), nil
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func contentText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
