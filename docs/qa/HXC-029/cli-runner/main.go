// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
//
// HXC-029 standalone HelixQA CLI-class bank runner (§11.4.98).
//
// Sibling of docs/qa/HXC-029/full-qa-api/runner/main.go (the HTTP-only
// runner). This runner drives the CLI-class banks — cli-agents-comprehensive,
// aichat-bash-tools-comprehensive, cli-agents-test-helixagent — which carry
// `shell:` actions (real host commands) in addition to `http:` actions.
//
// WHY standalone (same reason as the HTTP runner): in this checkout
// helix_qa's Go module `replace` directives point at owned submodules
// (DocProcessor, LLMOrchestrator, …) that are NOT checked out, so
// `go build ./pkg/autonomous/` fails at module-resolution. This runner
// re-implements the SAME documented HelixQA executor contract
// (helix_qa/pkg/testbank/schema.go ActionTypeShell / ActionTypeHTTP +
// _skip/_skip_reason) using only the Go standard library, driving REAL
// host commands and the REAL configured server. It is NOT a mock: every
// shell step is run via os/exec and asserts on the real exit code +
// captured combined output.
//
// Action grammar (matches helix_qa/pkg/testbank/schema.go):
//
//	"shell: <command>"        — run `sh -c <command>` in the configured
//	                            working dir; assert exit code + output.
//	"http: METHOD /path"      — real HTTP request to HELIXQA_HTTP_BASE_URL.
//
// Per-step assertion fields:
//
//	expect_exit            int     — required shell exit code (default 0)
//	expect_output_contains string  — substring that MUST appear in stdout+stderr
//	expect_output_absent   string  — substring that MUST NOT appear
//	expect_status          int     — required HTTP status (http: only)
//	expect_body_contains   string  — substring in HTTP body
//	_skip / _skip_reason           — honest non-execution (tool/service absent)
//
// SKIPs never fail the run (honest non-execution per Article XI §11.2.2 /
// §11.4 / §11.4.98). Exit code 0 only if zero FAILs.
//
// Usage:
//
//	HELIXQA_HTTP_BASE_URL=http://localhost:8080 \
//	HXC_CLI_WORKDIR=/abs/path/to/helix_code \
//	  go run ./main.go -bank <path-to-bank.json>
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type step struct {
	Name                 string            `json:"name"`
	Action               string            `json:"action"`
	Expected             string            `json:"expected"`
	Body                 any               `json:"body,omitempty"`
	Headers              map[string]string `json:"headers,omitempty"`
	Auth                 string            `json:"auth,omitempty"`
	ExpectExit           *int              `json:"expect_exit,omitempty"`
	ExpectOutputContains string            `json:"expect_output_contains,omitempty"`
	ExpectOutputAbsent   string            `json:"expect_output_absent,omitempty"`
	ExpectStatus         int               `json:"expect_status,omitempty"`
	ExpectBodyContains   string            `json:"expect_body_contains,omitempty"`
	Skip                 bool              `json:"_skip,omitempty"`
	SkipReason           string            `json:"_skip_reason,omitempty"`
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
	workdir := os.Getenv("HXC_CLI_WORKDIR")
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

	fmt.Printf("=== HXC-029 CLI bank run: %s (%s) ===\n", b.Name, b.Version)
	fmt.Printf("=== base=%q workdir=%q started %s ===\n\n", base, workdir, time.Now().UTC().Format(time.RFC3339))

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
			switch {
			case strings.HasPrefix(s.Action, "shell:"):
				cmd := strings.TrimSpace(strings.TrimPrefix(s.Action, "shell:"))
				res := execShell(workdir, cmd, s)
				if res == "" {
					pass++
					fmt.Printf("PASS %s — shell: %s\n", label, trunc([]byte(cmd), 80))
				} else {
					fail++
					failures = append(failures, label+" — "+res)
					fmt.Printf("FAIL %s — %s\n", label, res)
				}
			case strings.HasPrefix(s.Action, "http:"):
				method, path, ok := parseHTTP(s.Action)
				if !ok || base == "" {
					fail++
					msg := fmt.Sprintf("%s — http action but base empty or unparseable %q", label, s.Action)
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
			default:
				// Non-shell, non-http, non-skip prose is a §11.4.98 violation
				// if it ever appears — surface as FAIL, never silent pass.
				fail++
				msg := fmt.Sprintf("%s — non-executable prose action %q (not shell:, not http:, not _skip)", label, s.Action)
				failures = append(failures, msg)
				fmt.Printf("FAIL %s\n", msg)
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

// execShell runs one shell step via os/exec. Returns "" on PASS, else detail.
func execShell(workdir, command string, s step) string {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if workdir != "" {
		cmd.Dir = workdir
	}
	out, runErr := cmd.CombinedOutput()
	exitCode := cmd.ProcessState.ExitCode()

	wantExit := 0
	if s.ExpectExit != nil {
		wantExit = *s.ExpectExit
	}
	if exitCode != wantExit {
		return fmt.Sprintf("exit %d, expected %d (err=%v, output: %s)",
			exitCode, wantExit, runErr, trunc(out, 200))
	}
	if s.ExpectOutputContains != "" && !strings.Contains(string(out), s.ExpectOutputContains) {
		return fmt.Sprintf("output missing %q (output: %s)",
			s.ExpectOutputContains, trunc(out, 200))
	}
	if s.ExpectOutputAbsent != "" && strings.Contains(string(out), s.ExpectOutputAbsent) {
		return fmt.Sprintf("output unexpectedly contains %q (output: %s)",
			s.ExpectOutputAbsent, trunc(out, 200))
	}
	return ""
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

// execHTTP runs one http step. Returns "" on PASS, else a failure detail.
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
	return ""
}

func trunc(b []byte, n int) string {
	s := strings.ReplaceAll(string(b), "\n", "\\n")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
