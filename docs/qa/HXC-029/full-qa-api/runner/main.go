// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
//
// HXC-029 standalone HelixQA bank runner.
//
// WHY a standalone runner instead of helix_qa/pkg/autonomous's
// TestBankRealBinary_FullQAAPI: in this checkout the helix_qa Go module's
// `replace` directives point at owned submodules (DocProcessor,
// LLMOrchestrator, LLMProvider, VisionEngine) that are NOT checked out, so
// `go build ./pkg/autonomous/` fails at module-resolution time — a topology
// issue unrelated to the HTTP bank. This runner re-implements the SAME
// documented HelixQA http:/assert executor contract (action "http: METHOD
// /path", body, headers, expect_status, expect_json_path, expect_body_contains,
// _skip/_skip_reason — see helix_qa/pkg/testbank/schema.go +
// helix_qa/pkg/autonomous/http_executor.go) using only the Go standard
// library, and drives the REAL configured server. It is NOT a mock: every
// step makes a real HTTP request and asserts on the real response.
//
// Usage:
//
//	HELIXQA_HTTP_BASE_URL=http://localhost:8080 \
//	  go run ./main.go -bank <path-to-bank.json>
//
// Exit code 0 only if zero FAILs. SKIPs do not fail the run (honest
// non-execution per Article XI §11.2.2 / §11.4.98).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type step struct {
	Name               string            `json:"name"`
	Action             string            `json:"action"`
	Expected           string            `json:"expected"`
	Body               any               `json:"body,omitempty"`
	Headers            map[string]string `json:"headers,omitempty"`
	Auth               string            `json:"auth,omitempty"`
	ExpectStatus       int               `json:"expect_status,omitempty"`
	ExpectJSONPath     string            `json:"expect_json_path,omitempty"`
	ExpectBodyContains string            `json:"expect_body_contains,omitempty"`
	Skip               bool              `json:"_skip,omitempty"`
	SkipReason         string            `json:"_skip_reason,omitempty"`
}

type testCase struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Steps []step `json:"steps"`
}

type bank struct {
	Version   string     `json:"version"`
	Name      string     `json:"name"`
	TestCases []testCase `json:"test_cases"`
}

func main() {
	bankPath := flag.String("bank", "", "path to bank JSON")
	flag.Parse()
	base := strings.TrimRight(os.Getenv("HELIXQA_HTTP_BASE_URL"), "/")
	if base == "" {
		fmt.Fprintln(os.Stderr, "FATAL: set HELIXQA_HTTP_BASE_URL")
		os.Exit(2)
	}
	if *bankPath == "" {
		fmt.Fprintln(os.Stderr, "FATAL: -bank <path> required")
		os.Exit(2)
	}

	raw, err := os.ReadFile(*bankPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: read bank: %v\n", err)
		os.Exit(2)
	}
	var b bank
	if err := json.Unmarshal(raw, &b); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: parse bank: %v\n", err)
		os.Exit(2)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Pre-flight: server reachable?
	preReq, _ := http.NewRequest(http.MethodGet, base+"/health", nil)
	preResp, preErr := client.Do(preReq)
	if preErr != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %s/health unreachable: %v\n", base, preErr)
		os.Exit(2)
	}
	preResp.Body.Close()

	fmt.Printf("=== HXC-029 bank run: %s (%s) vs %s ===\n", b.Name, b.Version, base)
	fmt.Printf("=== started %s ===\n\n", time.Now().UTC().Format(time.RFC3339))

	var pass, fail, skip int
	var failures []string

	for _, tc := range b.TestCases {
		for _, s := range tc.Steps {
			label := fmt.Sprintf("[%s] %s", tc.ID, s.Name)
			if s.Skip {
				skip++
				fmt.Printf("SKIP %s — %s\n", label, s.SkipReason)
				continue
			}
			method, path, ok := parseHTTP(s.Action)
			if !ok {
				// Non-http, non-skip prose is a §11.4.98 violation if it ever
				// appears — surface it as FAIL, never silent pass.
				fail++
				msg := fmt.Sprintf("%s — non-executable prose action %q (not http:, not _skip)", label, s.Action)
				failures = append(failures, msg)
				fmt.Printf("FAIL %s\n", msg)
				continue
			}
			res := execHTTP(client, base, method, path, s)
			if res == "" {
				pass++
				fmt.Printf("PASS %s — %s %s\n", label, method, path)
			} else {
				fail++
				failures = append(failures, label+" — "+res)
				fmt.Printf("FAIL %s — %s\n", label, res)
			}
		}
	}

	fmt.Printf("\n=== summary: %d PASS, %d FAIL, %d SKIP (of %d steps) ===\n",
		pass, fail, skip, pass+fail+skip)
	if fail > 0 {
		fmt.Printf("=== %d failure(s): ===\n", len(failures))
		for _, f := range failures {
			fmt.Printf("  - %s\n", f)
		}
		os.Exit(1)
	}
	fmt.Println("=== RESULT: all executed steps PASS, no FAIL ===")
}

// parseHTTP splits "http: METHOD /path" -> (METHOD, /path, true).
func parseHTTP(action string) (method, path string, ok bool) {
	if !strings.HasPrefix(action, "http:") {
		return "", "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(action, "http:"))
	parts := strings.Fields(rest)
	if len(parts) < 2 {
		return "", "", false
	}
	method = strings.ToUpper(parts[0])
	path = parts[1]
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return method, path, true
}

// execHTTP runs one step. Returns "" on PASS, else a failure detail.
func execHTTP(client *http.Client, base, method, path string, s step) string {
	var body io.Reader
	contentType := ""
	if s.Body != nil {
		bb, err := json.Marshal(s.Body)
		if err != nil {
			return fmt.Sprintf("body marshal failed: %v", err)
		}
		body = bytes.NewReader(bb)
		contentType = "application/json"
	}
	req, err := http.NewRequest(method, base+path, body)
	if err != nil {
		return fmt.Sprintf("build request failed: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}
	// auth "raw:<token>" -> Bearer; "none"/"" -> nothing.
	if strings.HasPrefix(s.Auth, "raw:") {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.Auth[len("raw:"):]))
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("request failed: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if s.ExpectStatus != 0 && resp.StatusCode != s.ExpectStatus {
		return fmt.Sprintf("%s %s -> status %d, expected %d (body: %s)",
			method, path, resp.StatusCode, s.ExpectStatus, trunc(respBody, 160))
	}
	if s.ExpectBodyContains != "" && !strings.Contains(string(respBody), s.ExpectBodyContains) {
		return fmt.Sprintf("response body missing %q (body: %s)",
			s.ExpectBodyContains, trunc(respBody, 160))
	}
	if s.ExpectJSONPath != "" {
		if !jsonPathExists(respBody, s.ExpectJSONPath) {
			return fmt.Sprintf("json_path %q not found in response (body: %s)",
				s.ExpectJSONPath, trunc(respBody, 160))
		}
	}
	return ""
}

// jsonPathExists supports the simple "$.field" / "$.a.b" dotted form used by
// the bank (same subset as helix_qa's http_executor jsonPathExists for
// top-level/nested object keys).
func jsonPathExists(body []byte, path string) bool {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return false
	}
	p := strings.TrimPrefix(path, "$")
	p = strings.TrimPrefix(p, ".")
	if p == "" {
		return true
	}
	cur := v
	for _, key := range strings.Split(p, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		next, ok := m[key]
		if !ok {
			return false
		}
		cur = next
	}
	return true
}

func trunc(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
