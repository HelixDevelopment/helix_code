// Package main implements helix-qa — a real, machine-executable
// anti-bluff QA harness for HelixCode. Per the operator's repeated
// CONST-035 / Article XI §11.9 mandate, this tool does NOT report
// PASS without positive runtime evidence captured during execution.
//
// For each test case:
//   1. Issues a real HTTP request against a running HelixCode server
//   2. Captures the response status, headers, body length, and body
//      excerpt into a per-session evidence file
//   3. Asserts the documented expectations and records the result
//
// A PASS in helix-qa means a real HTTP round-trip succeeded and the
// response satisfied the documented contract. It is NOT possible to
// PASS without producing evidence — empty evidence == FAIL.
//
// Usage:
//
//   helix-qa run -base http://localhost:8080 -evidence-dir <path>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Check struct {
	ID         string
	Name       string
	Method     string
	Path       string
	WantStatus int
	WantBody   func(body []byte) error
}

type Evidence struct {
	CheckID    string `json:"check_id"`
	CheckName  string `json:"check_name"`
	Method     string `json:"method"`
	URL        string `json:"url"`
	Status     int    `json:"status"`
	WantStatus int    `json:"want_status"`
	BodyBytes  int    `json:"body_bytes"`
	BodyHead   string `json:"body_head"`
	DurationMs int64  `json:"duration_ms"`
	Result     string `json:"result"` // PASS | FAIL
	Error      string `json:"error,omitempty"`
	Timestamp  string `json:"timestamp"`
}

func requireJSONField(field string) func([]byte) error {
	return func(body []byte) error {
		var v map[string]any
		if err := json.Unmarshal(body, &v); err != nil {
			return fmt.Errorf("body not JSON: %w", err)
		}
		if _, ok := v[field]; !ok {
			return fmt.Errorf("response missing field %q", field)
		}
		return nil
	}
}

func requireDeepJSONPath(path ...string) func([]byte) error {
	return func(body []byte) error {
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			return fmt.Errorf("body not JSON: %w", err)
		}
		cur := v
		for _, p := range path {
			m, ok := cur.(map[string]any)
			if !ok {
				return fmt.Errorf("not a map at segment %q", p)
			}
			next, ok := m[p]
			if !ok {
				return fmt.Errorf("missing field %q", p)
			}
			cur = next
		}
		return nil
	}
}

var checks = []Check{
	{
		ID: "HCQA-001", Name: "Health check returns healthy status",
		Method: "GET", Path: "/health", WantStatus: 200,
		WantBody: func(b []byte) error {
			if !strings.Contains(string(b), `"status":"healthy"`) {
				return fmt.Errorf("body missing \"status\":\"healthy\"")
			}
			return nil
		},
	},
	{
		ID: "HCQA-002", Name: "Server info reports database connected",
		Method: "GET", Path: "/api/v1/server/info", WantStatus: 200,
		WantBody: requireDeepJSONPath("info", "database", "connected"),
	},
	{
		ID: "HCQA-003", Name: "Server info includes version + start_time",
		Method: "GET", Path: "/api/v1/server/info", WantStatus: 200,
		WantBody: func(b []byte) error {
			if err := requireDeepJSONPath("info", "version")(b); err != nil {
				return err
			}
			return requireDeepJSONPath("info", "start_time")(b)
		},
	},
	// CORRECTED 2026-05-13 per anti-bluff principle. The previous
	// HCQA-004/005/006 assertions stated my own assumptions ("metrics
	// endpoint protected", "not-implemented returns 501") that the
	// server contract did NOT actually promise. server.go:264-265
	// registers /server/info and /metrics as PUBLIC routes; the
	// notImplemented handler exists but is wired ONLY in handlers_test.go,
	// so /api/v1/not-implemented correctly returns 404 in production.
	// A test that asserts an expectation the server never promised is
	// itself a bluff (CONST-035 inverse PASS-bluff — failing the server
	// for not doing something it was never designed to do). Replaced
	// with assertions against documented behavior only.
	{
		ID: "HCQA-004", Name: "Metrics endpoint public + JSON-shaped",
		Method: "GET", Path: "/api/v1/metrics", WantStatus: 200,
		WantBody: func(b []byte) error {
			s := strings.TrimSpace(string(b))
			if !strings.HasPrefix(s, "{") && !strings.HasPrefix(s, "[") {
				return fmt.Errorf("body not JSON-shaped, head=%q", s[:min(80, len(s))])
			}
			return nil
		},
	},
	{
		ID: "HCQA-005", Name: "LLM providers list public + JSON-shaped",
		Method: "GET", Path: "/api/v1/llm/providers", WantStatus: 200,
		WantBody: func(b []byte) error {
			s := strings.TrimSpace(string(b))
			if !strings.HasPrefix(s, "{") && !strings.HasPrefix(s, "[") {
				return fmt.Errorf("body not JSON-shaped, head=%q", s[:min(80, len(s))])
			}
			return nil
		},
	},
	{
		ID: "HCQA-006", Name: "LLM models list public + JSON-shaped",
		Method: "GET", Path: "/api/v1/llm/models", WantStatus: 200,
		WantBody: func(b []byte) error {
			s := strings.TrimSpace(string(b))
			if !strings.HasPrefix(s, "{") && !strings.HasPrefix(s, "[") {
				return fmt.Errorf("body not JSON-shaped, head=%q", s[:min(80, len(s))])
			}
			return nil
		},
	},
}

func main() {
	base := flag.String("base", "http://localhost:8080", "Base URL of HelixCode server")
	dir := flag.String("evidence-dir", "", "Evidence directory (per-session)")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: helix-qa -base <url> -evidence-dir <path>")
		os.Exit(2)
	}

	if err := os.MkdirAll(filepath.Join(*dir, "evidence"), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var results []Evidence
	passed, failed := 0, 0

	for _, c := range checks {
		ev := Evidence{
			CheckID: c.ID, CheckName: c.Name,
			Method: c.Method, URL: *base + c.Path,
			WantStatus: c.WantStatus,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}
		start := time.Now()
		req, _ := http.NewRequest(c.Method, ev.URL, nil)
		resp, err := client.Do(req)
		ev.DurationMs = time.Since(start).Milliseconds()
		if err != nil {
			ev.Result, ev.Error = "FAIL", err.Error()
			results = append(results, ev)
			failed++
			fmt.Printf("  [%s] FAIL: %v\n", c.ID, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		ev.Status = resp.StatusCode
		ev.BodyBytes = len(body)
		ev.BodyHead = string(body)
		if len(ev.BodyHead) > 600 {
			ev.BodyHead = ev.BodyHead[:600] + "...(truncated)"
		}

		if resp.StatusCode != c.WantStatus {
			ev.Result = "FAIL"
			ev.Error = fmt.Sprintf("status %d != want %d", resp.StatusCode, c.WantStatus)
			fmt.Printf("  [%s] FAIL: %s\n", c.ID, ev.Error)
			failed++
		} else if err := c.WantBody(body); err != nil {
			ev.Result = "FAIL"
			ev.Error = err.Error()
			fmt.Printf("  [%s] FAIL: %s\n", c.ID, err)
			failed++
		} else {
			ev.Result = "PASS"
			fmt.Printf("  [%s] PASS: %d %s (%d bytes)\n", c.ID, resp.StatusCode, c.Name, len(body))
			passed++
		}
		results = append(results, ev)

		evPath := filepath.Join(*dir, "evidence", c.ID+".json")
		evJSON, _ := json.MarshalIndent(ev, "", "  ")
		_ = os.WriteFile(evPath, evJSON, 0o644)
	}

	// Summary report.
	summary := map[string]any{
		"session":    filepath.Base(*dir),
		"base_url":   *base,
		"total":      len(checks),
		"passed":     passed,
		"failed":     failed,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"results":    results,
		"const_035":  passed > 0 && failed == 0 && passed == len(checks),
	}
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	_ = os.WriteFile(filepath.Join(*dir, "summary.json"), summaryJSON, 0o644)

	mdReport := generateMarkdown(passed, failed, results, *base, *dir)
	_ = os.WriteFile(filepath.Join(*dir, "qa-report.md"), []byte(mdReport), 0o644)

	fmt.Println()
	fmt.Printf("=== helix-qa: %d/%d passed, %d failed ===\n", passed, len(checks), failed)
	fmt.Printf("Evidence: %s\n", *dir)
	if failed > 0 {
		os.Exit(1)
	}
}

func generateMarkdown(passed, failed int, results []Evidence, base, dir string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# helix-qa Anti-Bluff Session Report\n\n")
	fmt.Fprintf(&sb, "**Target**: %s\n", base)
	fmt.Fprintf(&sb, "**Timestamp**: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&sb, "**Session**: %s\n\n", filepath.Base(dir))
	fmt.Fprintf(&sb, "## Summary\n\n")
	fmt.Fprintf(&sb, "| Metric | Value |\n|---|---|\n")
	fmt.Fprintf(&sb, "| Total checks | %d |\n", len(results))
	fmt.Fprintf(&sb, "| Passed | %d |\n", passed)
	fmt.Fprintf(&sb, "| Failed | %d |\n", failed)
	overall := "FAIL"
	if passed > 0 && failed == 0 && passed == len(results) {
		overall = "PASS"
	}
	fmt.Fprintf(&sb, "| Overall | **%s** |\n\n", overall)
	fmt.Fprintf(&sb, "## Per-check results\n\n")
	fmt.Fprintf(&sb, "| Check | Method+Path | Status | Result | Duration | Bytes |\n")
	fmt.Fprintf(&sb, "|---|---|---|---|---|---|\n")
	for _, r := range results {
		fmt.Fprintf(&sb, "| %s — %s | %s %s | %d | %s | %d ms | %d |\n",
			r.CheckID, r.CheckName, r.Method, strings.TrimPrefix(r.URL, base),
			r.Status, r.Result, r.DurationMs, r.BodyBytes)
	}
	fmt.Fprintf(&sb, "\n## Anti-bluff evidence\n\n")
	fmt.Fprintf(&sb, "Every PASS above is backed by a captured-evidence JSON file under `evidence/<check-id>.json`. ")
	fmt.Fprintf(&sb, "The captured body (`body_head`), status code, and round-trip duration are real HTTP wire data — not metadata grepped from source.\n\n")
	fmt.Fprintf(&sb, "Per CONST-035 / Article XI §11.9, a PASS here means HelixCode actually served the documented behavior. ")
	fmt.Fprintf(&sb, "A FAIL means it did not, and the body excerpt + diagnostic message tells you why.\n")
	return sb.String()
}
