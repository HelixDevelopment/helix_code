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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// redactCredentials replaces JWT-shaped tokens and JSON `*_token` values
// with REDACTED markers BEFORE the body is written to disk. The QA
// harness only needs to prove a token was present and well-formed (e.g.,
// starts with "eyJ"); it does NOT need to log the actual credential
// value. Defense-in-depth per CONST-042 / §12.1 (No-Secret-Leak): even
// gitignored on-disk artefacts must not contain real credentials.
//
// Patterns redacted (in order):
//   - JWT tokens (eyJ-prefixed bearer headers + bodies)
//   - JSON fields ending in `_token` / `token` with string values
//   - JSON fields named `session_token`, `refresh_token`, `access_token`,
//     `api_key`, `password`, `secret`
var (
	jwtRe        = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]+`)
	tokenFieldRe = regexp.MustCompile(`"(session_token|refresh_token|access_token|api_key|password|secret|token)"\s*:\s*"[^"]+"`)
)

func redactCredentials(s string) string {
	s = jwtRe.ReplaceAllString(s, "eyJ.REDACTED.JWT")
	s = tokenFieldRe.ReplaceAllStringFunc(s, func(m string) string {
		i := strings.Index(m, ":")
		if i < 0 {
			return m
		}
		field := strings.TrimSpace(m[:i])
		return field + ": \"[REDACTED]\""
	})
	return s
}

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

// requireDeepJSONValue asserts the value at path equals want (any-typed eq).
func requireDeepJSONValue(want any, path ...string) func([]byte) error {
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
		if fmt.Sprintf("%v", cur) != fmt.Sprintf("%v", want) {
			return fmt.Errorf("at %v: got %v (%T), want %v (%T)",
				path, cur, cur, want, want)
		}
		return nil
	}
}

// requireNonEmptyArray asserts the value at path is a non-empty []any.
func requireNonEmptyArray(path ...string) func([]byte) error {
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
		arr, ok := cur.([]any)
		if !ok {
			return fmt.Errorf("at %v: not an array (got %T)", path, cur)
		}
		if len(arr) == 0 {
			return fmt.Errorf("at %v: array is empty (count=0 is a bluff for a populated platform)", path)
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

	// Deep-content checks — added 2026-05-13 after discovering that
	// "JSON-shape only" assertions pass for a server that returns
	// {"status":"ok","providers":[]}, which is a populated-platform
	// bluff. CONST-035 requires real evidence of working data.
	{
		ID: "HCQA-007", Name: "Server info reports database.connected==true",
		Method: "GET", Path: "/api/v1/server/info", WantStatus: 200,
		WantBody: requireDeepJSONValue(true, "info", "database", "connected"),
	},
	{
		ID: "HCQA-008", Name: "LLM providers list is non-empty",
		Method: "GET", Path: "/api/v1/llm/providers", WantStatus: 200,
		WantBody: requireNonEmptyArray("providers"),
	},
	{
		ID: "HCQA-009", Name: "LLM models list is non-empty",
		Method: "GET", Path: "/api/v1/llm/models", WantStatus: 200,
		WantBody: requireNonEmptyArray("models"),
	},
	{
		ID: "HCQA-010", Name: "Memory systems list is non-empty",
		Method: "GET", Path: "/api/v1/memory/systems", WantStatus: 200,
		WantBody: requireNonEmptyArray("systems"),
	},
	// HCQA-014b: catches the BUG #14 lie. /memory/systems used to
	// claim status="available" for all 6 entries while /memory/stats
	// simultaneously reported systems_connected=0 — direct contradiction
	// (no memory manager wired in the Server struct, yet every entry
	// claimed it was up). This check asserts the contradiction is
	// resolved: status must NOT be "available" for any entry until the
	// corresponding manager is wired AND a real reachability probe runs.
	{
		ID: "HCQA-MEM-BLUFF", Name: "Memory systems status agrees with /memory/stats systems_connected",
		Method: "GET", Path: "/api/v1/memory/systems", WantStatus: 200,
		WantBody: func(b []byte) error {
			var v map[string]any
			if err := json.Unmarshal(b, &v); err != nil {
				return fmt.Errorf("body not JSON: %w", err)
			}
			arr, _ := v["systems"].([]any)
			availableCount := 0
			for _, s := range arr {
				m, _ := s.(map[string]any)
				if m == nil {
					continue
				}
				if m["status"] == "available" {
					availableCount++
				}
			}
			// Until a real memory manager is wired AND a real probe
			// runs, no entry should claim "available". An empty
			// availableCount is the honest state. If this assertion
			// ever fails, either the manager IS wired (good — update
			// the test to count the actually-running ones) or the
			// status field was re-hardcoded to "available" without
			// real backing (bad — CONST-035 violation).
			if availableCount > 0 {
				return fmt.Errorf("%d systems claim status=\"available\" but no memory manager is wired (CONST-035 bluff)",
					availableCount)
			}
			return nil
		},
	},
	{
		ID: "HCQA-011", Name: "Known provider (openai) returns 200 with id matching",
		Method: "GET", Path: "/api/v1/llm/providers/openai", WantStatus: 200,
		WantBody: requireDeepJSONValue("openai", "provider", "id"),
	},

	// Anti-bluff bug-reproduction check — added 2026-05-13.
	// Verifies the BLUFF-002-class fix in handlers.go:getLLMProvider:
	// unknown provider IDs MUST return 404, not a fabricated
	// "status: available" stub. Pre-fix the server returned 200 for
	// "does-not-exist-xyz" which silently lied about platform state.
	{
		ID: "HCQA-012", Name: "Unknown provider 404 (BLUFF-002 reproduction)",
		Method: "GET", Path: "/api/v1/llm/providers/does-not-exist-xyz",
		WantStatus: 404,
		WantBody: func(b []byte) error {
			var v map[string]any
			if err := json.Unmarshal(b, &v); err != nil {
				return fmt.Errorf("body not JSON: %w", err)
			}
			if v["status"] != "error" {
				return fmt.Errorf("status field=%v, want \"error\"", v["status"])
			}
			es, _ := v["error"].(string)
			if !strings.Contains(es, "does-not-exist-xyz") {
				return fmt.Errorf("error %q does not mention the bogus id", es)
			}
			return nil
		},
	},
	{
		ID: "HCQA-013", Name: "Metrics endpoint includes resources.goroutines>0",
		Method: "GET", Path: "/api/v1/metrics", WantStatus: 200,
		WantBody: func(b []byte) error {
			var v map[string]any
			if err := json.Unmarshal(b, &v); err != nil {
				return fmt.Errorf("body not JSON: %w", err)
			}
			m, _ := v["metrics"].(map[string]any)
			if m == nil {
				return fmt.Errorf("metrics field missing or wrong type")
			}
			r, _ := m["resources"].(map[string]any)
			if r == nil {
				return fmt.Errorf("metrics.resources missing")
			}
			g, _ := r["goroutines"].(float64)
			if g < 1 {
				return fmt.Errorf("goroutines=%v, must be >0 for a live process", g)
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
		ev.BodyHead = redactCredentials(string(body))
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

	// Sequenced auth-flow probes — exercise register → login → protected
	// → logout against the live server. Each step writes its own evidence
	// file. Failure of any step does NOT abort the suite; subsequent
	// steps still attempt and record evidence, since "step N failed and
	// then we never ran N+1" is itself useful diagnostic data.
	authResults, ap, af := runAuthFlow(client, *base, *dir)
	results = append(results, authResults...)
	passed += ap
	failed += af

	total := len(checks) + len(authResults)

	mdReport := generateMarkdown(passed, failed, results, *base, *dir)
	_ = os.WriteFile(filepath.Join(*dir, "qa-report.md"), []byte(mdReport), 0o644)

	// Update summary now that auth-flow results are merged.
	summary["total"] = total
	summary["passed"] = passed
	summary["failed"] = failed
	summary["results"] = results
	summary["const_035"] = passed > 0 && failed == 0 && passed == total
	summaryJSON, _ = json.MarshalIndent(summary, "", "  ")
	_ = os.WriteFile(filepath.Join(*dir, "summary.json"), summaryJSON, 0o644)

	fmt.Println()
	fmt.Printf("=== helix-qa: %d/%d passed, %d failed ===\n", passed, total, failed)
	fmt.Printf("Evidence: %s\n", *dir)
	if failed > 0 {
		os.Exit(1)
	}
}

// authStep performs one HTTP request as part of the sequenced auth flow,
// records evidence, and returns the parsed JSON body + the populated
// Evidence record. The caller stitches outputs of step N into the input
// of step N+1.
func authStep(client *http.Client, dir, id, name, method, url string,
	body any, headers map[string]string, wantStatus int,
	check func(map[string]any, []byte) error,
) (Evidence, map[string]any, bool) {
	ev := Evidence{
		CheckID: id, CheckName: name,
		Method: method, URL: url,
		WantStatus: wantStatus,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	var reqBody io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		reqBody = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		ev.Result, ev.Error = "FAIL", err.Error()
		return ev, nil, false
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	start := time.Now()
	resp, err := client.Do(req)
	ev.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		ev.Result, ev.Error = "FAIL", err.Error()
		fmt.Printf("  [%s] FAIL: %v\n", id, err)
		writeEvidence(dir, ev)
		return ev, nil, false
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	ev.Status = resp.StatusCode
	ev.BodyBytes = len(raw)
	ev.BodyHead = redactCredentials(string(raw))
	if len(ev.BodyHead) > 600 {
		ev.BodyHead = ev.BodyHead[:600] + "...(truncated)"
	}

	parsed := map[string]any{}
	_ = json.Unmarshal(raw, &parsed)

	if resp.StatusCode != wantStatus {
		ev.Result = "FAIL"
		ev.Error = fmt.Sprintf("status %d != want %d", resp.StatusCode, wantStatus)
		fmt.Printf("  [%s] FAIL: %s\n", id, ev.Error)
		writeEvidence(dir, ev)
		return ev, parsed, false
	}
	if check != nil {
		if err := check(parsed, raw); err != nil {
			ev.Result = "FAIL"
			ev.Error = err.Error()
			fmt.Printf("  [%s] FAIL: %s\n", id, err)
			writeEvidence(dir, ev)
			return ev, parsed, false
		}
	}
	ev.Result = "PASS"
	fmt.Printf("  [%s] PASS: %d %s (%d bytes)\n", id, resp.StatusCode, name, len(raw))
	writeEvidence(dir, ev)
	return ev, parsed, true
}

func writeEvidence(dir string, ev Evidence) {
	path := filepath.Join(dir, "evidence", ev.CheckID+".json")
	b, _ := json.MarshalIndent(ev, "", "  ")
	_ = os.WriteFile(path, b, 0o644)
}

// runAuthFlow exercises register → login → protected → logout end-to-end.
// Returns the per-step evidence and pass/fail counts.
//
// Anti-bluff invariants enforced (CONST-035):
//   - Register must return 201 with user.id non-empty (real persisted user)
//   - Login must return 200 with token non-empty (real JWT, not "")
//   - /users/me with Bearer token must return the SAME username we registered
//     (proves the token actually authenticated the right user, not just
//      that the endpoint returns some-user-shaped JSON)
//   - /users/me with garbage token must 401 (proves auth middleware is real)
//   - Logout must return 200
func runAuthFlow(client *http.Client, base, dir string) ([]Evidence, int, int) {
	var results []Evidence
	passed, failed := 0, 0

	username := fmt.Sprintf("qa-flow-%d", time.Now().UnixNano())
	password := "Qa-AntiBluff-Flow-2026!"
	email := username + "@helix.local"

	// HCQA-014: unauthenticated /users/me must 401.
	ev, _, ok := authStep(client, dir, "HCQA-014",
		"Unauthenticated /users/me returns 401",
		"GET", base+"/api/v1/users/me", nil, nil, 401,
		func(v map[string]any, raw []byte) error {
			if v["status"] != "error" {
				return fmt.Errorf("status=%v want \"error\"", v["status"])
			}
			return nil
		})
	results = append(results, ev)
	if ok {
		passed++
	} else {
		failed++
	}

	// HCQA-015: register a fresh user.
	regBody := map[string]string{"username": username, "password": password, "email": email}
	ev, regResp, ok := authStep(client, dir, "HCQA-015",
		"Auth register creates user with UUID",
		"POST", base+"/api/v1/auth/register", regBody, nil, 201,
		func(v map[string]any, raw []byte) error {
			u, _ := v["user"].(map[string]any)
			if u == nil {
				return fmt.Errorf("user field missing")
			}
			id, _ := u["id"].(string)
			if len(id) < 32 {
				return fmt.Errorf("user.id=%q too short to be a UUID", id)
			}
			if u["username"] != username {
				return fmt.Errorf("user.username=%v want %q", u["username"], username)
			}
			return nil
		})
	results = append(results, ev)
	if ok {
		passed++
	} else {
		failed++
	}

	registeredID := ""
	if u, _ := regResp["user"].(map[string]any); u != nil {
		registeredID, _ = u["id"].(string)
	}

	// HCQA-016: login → JWT.
	loginBody := map[string]string{"username": username, "password": password}
	ev, loginResp, ok := authStep(client, dir, "HCQA-016",
		"Auth login returns JWT + session",
		"POST", base+"/api/v1/auth/login", loginBody, nil, 200,
		func(v map[string]any, raw []byte) error {
			tok, _ := v["token"].(string)
			if len(tok) < 32 {
				return fmt.Errorf("token=%q too short to be a JWT", tok)
			}
			if !strings.HasPrefix(tok, "eyJ") {
				return fmt.Errorf("token doesn't start with JWT header marker")
			}
			sess, _ := v["session"].(map[string]any)
			if sess == nil {
				return fmt.Errorf("session field missing")
			}
			sid, _ := sess["id"].(string)
			if len(sid) < 32 {
				return fmt.Errorf("session.id=%q too short to be a UUID", sid)
			}
			if sess["user_id"] != registeredID {
				return fmt.Errorf("session.user_id=%v != registered user.id=%q",
					sess["user_id"], registeredID)
			}
			return nil
		})
	results = append(results, ev)
	if ok {
		passed++
	} else {
		failed++
	}
	token, _ := loginResp["token"].(string)

	// HCQA-017: /users/me with valid Bearer.
	//
	// Anti-bluff invariants (tightened 2026-05-13 after discovering the
	// JWT-stub bug): the returned user must have REAL persisted state,
	// not the minimal stub the previous middleware returned from JWT
	// claims alone. Specifically:
	//   - id/username match the registered user (caught the wrong-user
	//     case already)
	//   - is_active==true (not the zero-value `false` stub)
	//   - created_at is non-zero (not "0001-01-01T00:00:00Z" stub)
	if token != "" {
		ev, _, ok := authStep(client, dir, "HCQA-017",
			"Authenticated /users/me returns the same user with real persisted state",
			"GET", base+"/api/v1/users/me", nil,
			map[string]string{"Authorization": "Bearer " + token}, 200,
			func(v map[string]any, raw []byte) error {
				u, _ := v["user"].(map[string]any)
				if u == nil {
					return fmt.Errorf("user field missing")
				}
				if u["username"] != username {
					return fmt.Errorf("user.username=%v want %q (auth returned wrong user!)",
						u["username"], username)
				}
				if u["id"] != registeredID {
					return fmt.Errorf("user.id=%v want %q", u["id"], registeredID)
				}
				if active, _ := u["is_active"].(bool); !active {
					return fmt.Errorf("user.is_active=false — stub-from-JWT-claims bluff (real user IS active)")
				}
				ca, _ := u["created_at"].(string)
				if ca == "" || strings.HasPrefix(ca, "0001-") {
					return fmt.Errorf("user.created_at=%q — zero-value stub bluff", ca)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
	} else {
		fmt.Println("  [HCQA-017] SKIP-OK: previous login failed; cannot exercise authenticated path")
	}

	// HCQA-018: garbage token must 401 (proves middleware verifies, not just looks).
	ev, _, ok = authStep(client, dir, "HCQA-018",
		"Garbage Bearer token returns 401",
		"GET", base+"/api/v1/users/me", nil,
		map[string]string{"Authorization": "Bearer not-a-real-token-xyz"}, 401,
		nil)
	results = append(results, ev)
	if ok {
		passed++
	} else {
		failed++
	}

	// HCQA-019..HCQA-024: tasks CRUD before logout. Real DB persistence.
	// Anti-bluff invariants:
	//   - empty list returns [] not null (JSON contract: list always array)
	//   - POST returns 201 with task.id (UUID) + data field present (the
	//     "task_data is NOT NULL" schema-violation bug that returned 500
	//     pre-fix would surface here as != 201)
	//   - subsequent GET-list contains the just-created task.id
	//   - GET /tasks/:id returns the same task we created
	//   - DELETE returns 200 and the task is gone (idempotent 404 next time)
	if token != "" {
		auth := map[string]string{"Authorization": "Bearer " + token}

		// HCQA-019: list empty (pre-create).
		ev, _, ok := authStep(client, dir, "HCQA-019",
			"Tasks list endpoint returns array (not null)",
			"GET", base+"/api/v1/tasks", nil, auth, 200,
			func(v map[string]any, raw []byte) error {
				// Must be present AND be an array. nil-slice serialization
				// to `null` is a JSON contract bluff — this check enforces
				// the array invariant.
				if !strings.Contains(string(raw), `"tasks":[`) {
					return fmt.Errorf("tasks field is not a JSON array (raw head=%.200s)", string(raw))
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-020: create.
		taskBody := map[string]any{
			"name":        "qa-anti-bluff-task-" + username,
			"type":        "qa",
			"priority":    "high",
			"description": "helix-qa anti-bluff probe task",
		}
		ev, createResp, ok := authStep(client, dir, "HCQA-020",
			"Create task returns 201 with UUID + data field",
			"POST", base+"/api/v1/tasks", taskBody, auth, 201,
			func(v map[string]any, raw []byte) error {
				t, _ := v["task"].(map[string]any)
				if t == nil {
					return fmt.Errorf("task field missing")
				}
				id, _ := t["id"].(string)
				if len(id) < 32 {
					return fmt.Errorf("task.id=%q too short to be a UUID", id)
				}
				// data must be present (proves the task_data NOT NULL
				// schema invariant held)
				if _, hasData := t["data"]; !hasData {
					return fmt.Errorf("task.data field missing — task_data schema invariant broken")
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		createdTaskID := ""
		if t, _ := createResp["task"].(map[string]any); t != nil {
			createdTaskID, _ = t["id"].(string)
		}

		// HCQA-021: list contains the just-created task.
		if createdTaskID != "" {
			ev, _, ok = authStep(client, dir, "HCQA-021",
				"List tasks contains the just-created task",
				"GET", base+"/api/v1/tasks", nil, auth, 200,
				func(v map[string]any, raw []byte) error {
					if !strings.Contains(string(raw), createdTaskID) {
						return fmt.Errorf("list response does not contain created task id %q",
							createdTaskID)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-022: GET /tasks/:id round-trips the same task.
			ev, _, ok = authStep(client, dir, "HCQA-022",
				"GET task by ID returns the same task",
				"GET", base+"/api/v1/tasks/"+createdTaskID, nil, auth, 200,
				func(v map[string]any, raw []byte) error {
					t, _ := v["task"].(map[string]any)
					if t == nil {
						return fmt.Errorf("task field missing")
					}
					if t["id"] != createdTaskID {
						return fmt.Errorf("task.id=%v != requested id %q", t["id"], createdTaskID)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-023: DELETE.
			ev, _, ok = authStep(client, dir, "HCQA-023",
				"DELETE task returns 200",
				"DELETE", base+"/api/v1/tasks/"+createdTaskID, nil, auth, 200,
				nil)
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-024: GET after DELETE → 404.
			ev, _, ok = authStep(client, dir, "HCQA-024",
				"GET deleted task returns 404",
				"GET", base+"/api/v1/tasks/"+createdTaskID, nil, auth, 404,
				nil)
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-PRE-projects: Unauthenticated POST /projects must 401 — the
		// previous routing had a `publicProjects` group with "no auth
		// for testing" comment, exposing POST and 4 workflow endpoints
		// without authentication. CONST-035 anti-bluff: route comments
		// that ship "for testing" become real production attack surface.
		ev, _, ok = authStep(client, dir, "HCQA-030",
			"Unauthenticated POST /projects must return 401",
			"POST", base+"/api/v1/projects",
			map[string]any{"name": "unauth-probe", "path": "/tmp/unauth-probe-qa"},
			nil, 401, nil)
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-CREATE-PROJ: Authenticated POST /projects must return 201
		// with project.id (UUID). Previously broken by:
		//   (a) routing through publicProjects (no middleware) → 401
		//       even with valid Bearer
		//   (b) projectManager.CreateProject hardcoding ownerID to
		//       "default-user" (12 chars, not a UUID) → 500
		// Both fixed in this round.
		projPath := fmt.Sprintf("/tmp/qa-create-proj-%d", time.Now().UnixNano())
		ev, _, ok = authStep(client, dir, "HCQA-031",
			"Auth POST /projects creates project with UUID + path",
			"POST", base+"/api/v1/projects",
			map[string]any{
				"name":        "qa-create-proj",
				"description": "qa anti-bluff project create probe",
				"path":        projPath,
				"type":        "go",
			},
			auth, 201,
			func(v map[string]any, raw []byte) error {
				p, _ := v["project"].(map[string]any)
				if p == nil {
					return fmt.Errorf("project field missing")
				}
				id, _ := p["id"].(string)
				if len(id) < 32 {
					return fmt.Errorf("project.id=%q too short to be a UUID", id)
				}
				if p["path"] != projPath {
					return fmt.Errorf("project.path=%v != requested %q", p["path"], projPath)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-025: list projects returns 200 with an array (not 401, not null).
		// Anti-bluff: pre-fix, this endpoint returned 401 even with a
		// valid JWT because listProjects looked up "user_id" via
		// c.GetString while authMiddleware sets "user" as *auth.User.
		// And even after the auth fix, an empty result serialized as
		// null instead of []. Both fixed; this check now guards both.
		ev, _, ok = authStep(client, dir, "HCQA-025",
			"List projects returns array (not 401, not null)",
			"GET", base+"/api/v1/projects", nil, auth, 200,
			func(v map[string]any, raw []byte) error {
				if !strings.Contains(string(raw), `"projects":[`) {
					return fmt.Errorf("projects field is not a JSON array (raw head=%.200s)",
						string(raw))
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-036..038: task-lifecycle probes — exercise the start/
		// complete/checkpoints endpoints on a fresh task. The checkpoints
		// probe catches BUG #12 (4th instance of the nil-slice→null
		// JSON contract bluff — checkpoints was null instead of []).
		// The retry-on-wrong-state probe catches BUG #13 (retry on a
		// completed task returned 500 — a server-error code lying about
		// what was a client-side state error; now correctly 422).
		lifecycleTaskBody := map[string]any{
			"name":     "qa-lifecycle-probe",
			"type":     "qa",
			"priority": "high",
		}
		var createR map[string]any
		ev, createR, ok = authStep(client, dir, "HCQA-036",
			"Create task for lifecycle probe (201 + UUID)",
			"POST", base+"/api/v1/tasks", lifecycleTaskBody, auth, 201,
			func(v map[string]any, raw []byte) error {
				t, _ := v["task"].(map[string]any)
				if t == nil {
					return fmt.Errorf("task field missing")
				}
				id, _ := t["id"].(string)
				if len(id) < 32 {
					return fmt.Errorf("task.id=%q too short to be a UUID", id)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		taskIDLC := ""
		if t, _ := createR["task"].(map[string]any); t != nil {
			taskIDLC, _ = t["id"].(string)
		}

		if taskIDLC != "" {
			// HCQA-037: empty checkpoints returns [] (catches BUG #12).
			ev, _, ok = authStep(client, dir, "HCQA-037",
				"GET /tasks/:id/checkpoints returns array (not null)",
				"GET", base+"/api/v1/tasks/"+taskIDLC+"/checkpoints",
				nil, auth, 200,
				func(v map[string]any, raw []byte) error {
					if !strings.Contains(string(raw), `"checkpoints":[`) {
						return fmt.Errorf("checkpoints field is not a JSON array (raw=%.200s)",
							string(raw))
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-038: start → complete → retry returns 422 (BUG #13).
			// Both start + complete handlers strictly require a JSON
			// body — pass an empty object explicitly so the gin
			// ShouldBindJSON doesn't reject with 400 "EOF".
			_, _, _ = authStep(client, dir, "HCQA-038-PRE-START",
				"Start task for retry probe", "POST",
				base+"/api/v1/tasks/"+taskIDLC+"/start",
				map[string]any{}, auth, 200, nil)
			_, _, _ = authStep(client, dir, "HCQA-038-PRE-COMPLETE",
				"Complete task for retry probe", "POST",
				base+"/api/v1/tasks/"+taskIDLC+"/complete",
				map[string]any{}, auth, 200, nil)
			ev, _, ok = authStep(client, dir, "HCQA-038",
				"POST /tasks/:id/retry on a completed task returns 422 (not 500)",
				"POST", base+"/api/v1/tasks/"+taskIDLC+"/retry", nil,
				auth, 422,
				func(v map[string]any, raw []byte) error {
					if v["status"] != "error" {
						return fmt.Errorf("status=%v want \"error\"", v["status"])
					}
					m, _ := v["message"].(string)
					if !strings.Contains(strings.ToLower(m), "retry") {
						return fmt.Errorf("message %q does not mention retry", m)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-039..041: project lifecycle (update + delete + 404-after-delete).
		// Catches BUG #15: PUT /projects/:id used a wrong column name
		// (`path` instead of `workspace_path`, and a nonexistent `type`
		// column) in the UPDATE ... RETURNING clause — every PUT
		// returned 500 "column path does not exist (SQLSTATE 42703)".
		// The renamed PR validates the column mapping by asserting that
		// the response carries the new name + description.
		projForLifecycleBody := map[string]any{
			"name":        "qa-update-lifecycle",
			"description": "original",
			"path":        fmt.Sprintf("/tmp/qa-update-lifecycle-%d", time.Now().UnixNano()),
			"type":        "go",
		}
		ev, projLcResp, ok := authStep(client, dir, "HCQA-039",
			"Create project for update-lifecycle probe",
			"POST", base+"/api/v1/projects", projForLifecycleBody, auth, 201,
			func(v map[string]any, raw []byte) error {
				p, _ := v["project"].(map[string]any)
				if p == nil || p["id"] == nil {
					return fmt.Errorf("project.id missing")
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		projLcID := ""
		if p, _ := projLcResp["project"].(map[string]any); p != nil {
			projLcID, _ = p["id"].(string)
		}

		if projLcID != "" {
			ev, _, ok = authStep(client, dir, "HCQA-040",
				"PUT /projects/:id renames and updates description (catches BUG #15)",
				"PUT", base+"/api/v1/projects/"+projLcID,
				map[string]any{"name": "qa-renamed", "description": "updated"},
				auth, 200,
				func(v map[string]any, raw []byte) error {
					p, _ := v["project"].(map[string]any)
					if p == nil {
						return fmt.Errorf("project field missing")
					}
					if p["name"] != "qa-renamed" {
						return fmt.Errorf("project.name=%v want \"qa-renamed\"", p["name"])
					}
					if p["description"] != "updated" {
						return fmt.Errorf("project.description=%v want \"updated\"", p["description"])
					}
					// path must round-trip (proves workspace_path<->Path
					// mapping in UpdateProject works as in GetProject)
					if path, _ := p["path"].(string); path == "" {
						return fmt.Errorf("project.path empty — UpdateProject column-mapping regression")
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-041: DELETE then GET → 404.
			ev, _, ok = authStep(client, dir, "HCQA-041-DEL",
				"DELETE /projects/:id returns 200",
				"DELETE", base+"/api/v1/projects/"+projLcID, nil, auth, 200, nil)
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
			ev, _, ok = authStep(client, dir, "HCQA-041",
				"GET deleted project returns 404",
				"GET", base+"/api/v1/projects/"+projLcID, nil, auth, 404, nil)
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-042..043: worker update + delete lifecycle.
		// Catches BUG #16: PUT /workers/:id triggered the SAME NULL-on-
		// capabilities constraint violation as RegisterWorker (round 5),
		// AND was overwriting non-omitted fields with empty strings.
		// Fix added COALESCE(NULLIF(...)) for hostname/display_name +
		// nil-defaults capabilities to empty slice.
		workerLcBody := map[string]any{
			"hostname":     fmt.Sprintf("qa-update-w-%d", time.Now().UnixNano()),
			"display_name": "qa-worker-original",
		}
		ev, workerLcResp, ok := authStep(client, dir, "HCQA-042-CREATE",
			"Create worker for update-lifecycle probe",
			"POST", base+"/api/v1/workers", workerLcBody, auth, 201,
			func(v map[string]any, raw []byte) error {
				w, _ := v["worker"].(map[string]any)
				if w == nil || w["id"] == nil {
					return fmt.Errorf("worker.id missing")
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		workerLcID := ""
		origHostname := ""
		if w, _ := workerLcResp["worker"].(map[string]any); w != nil {
			workerLcID, _ = w["id"].(string)
			origHostname, _ = w["hostname"].(string)
		}

		if workerLcID != "" {
			// HCQA-042: PUT with partial body — hostname must be preserved
			// (proves COALESCE pattern works), display_name must change,
			// nil capabilities must not 500 (proves the nil-default).
			ev, _, ok = authStep(client, dir, "HCQA-042",
				"PUT /workers/:id updates display_name without clobbering hostname",
				"PUT", base+"/api/v1/workers/"+workerLcID,
				map[string]any{"display_name": "qa-worker-renamed", "max_concurrent_tasks": 20},
				auth, 200,
				func(v map[string]any, raw []byte) error {
					w, _ := v["worker"].(map[string]any)
					if w == nil {
						return fmt.Errorf("worker field missing")
					}
					if w["display_name"] != "qa-worker-renamed" {
						return fmt.Errorf("worker.display_name=%v want \"qa-worker-renamed\"",
							w["display_name"])
					}
					if w["hostname"] != origHostname {
						return fmt.Errorf("worker.hostname=%v changed from %q (partial-update clobber bug)",
							w["hostname"], origHostname)
					}
					mct, _ := w["max_concurrent_tasks"].(float64)
					if mct != 20 {
						return fmt.Errorf("worker.max_concurrent_tasks=%v want 20", mct)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-043: DELETE → 200 and GET → 404.
			ev, _, ok = authStep(client, dir, "HCQA-043-DEL",
				"DELETE /workers/:id returns 200",
				"DELETE", base+"/api/v1/workers/"+workerLcID, nil, auth, 200, nil)
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
			ev, _, ok = authStep(client, dir, "HCQA-043",
				"GET deleted worker returns 404",
				"GET", base+"/api/v1/workers/"+workerLcID, nil, auth, 404, nil)
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-086..088: round-26 sessions+workers input validation.
		// (29) POST /sessions with bogus project_id → 404
		// (30) POST /sessions with malformed project_id → 400
		// (31) POST /workers with >255-char hostname → 400, no pg leak
		ev, _, ok = authStep(client, dir, "HCQA-086",
			"POST /sessions with bogus project_id returns 404 (was 201 silent)",
			"POST", base+"/api/v1/sessions",
			map[string]any{
				"project_id": "00000000-0000-0000-0000-000000000000",
				"mode":       "planning",
				"name":       "qa-anti-bluff",
			},
			auth, 404,
			func(v map[string]any, raw []byte) error {
				if v["status"] != "error" {
					return fmt.Errorf("status=%v want \"error\"", v["status"])
				}
				m, _ := v["message"].(string)
				if !strings.Contains(strings.ToLower(m), "project") {
					return fmt.Errorf("message %q does not mention project", m)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		ev, _, ok = authStep(client, dir, "HCQA-087",
			"POST /sessions with malformed project_id returns 400 (was 201 silent)",
			"POST", base+"/api/v1/sessions",
			map[string]any{
				"project_id": "not-a-uuid",
				"mode":       "planning",
				"name":       "qa-anti-bluff",
			},
			auth, 400,
			func(v map[string]any, raw []byte) error {
				m, _ := v["message"].(string)
				if !strings.Contains(strings.ToLower(m), "invalid") {
					return fmt.Errorf("message %q does not mention invalid input", m)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		// HCQA-088: 512-char hostname must 400 + NO postgres leak.
		longHost := strings.Repeat("a", 512)
		ev, _, ok = authStep(client, dir, "HCQA-088",
			"POST /workers with 512-char hostname returns 400 + no pg SQLSTATE 22001 leak",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": longHost}, auth, 400,
			func(v map[string]any, raw []byte) error {
				m, _ := v["message"].(string)
				if !strings.Contains(strings.ToLower(m), "hostname") {
					return fmt.Errorf("message %q does not mention hostname limit", m)
				}
				bodyStr := string(raw)
				for _, leak := range []string{"SQLSTATE", "22001", "character varying"} {
					if strings.Contains(bodyStr, leak) {
						return fmt.Errorf("response leaks postgres detail %q: %.200s", leak, bodyStr)
					}
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-081..085: round-25 — PUT/DELETE project malformed-UUID
		// (catches BUG #28 — pg SQLSTATE 22P02 leaked through), task
		// checkpoint handlers (createTaskCheckpoint + getTaskCheckpoints
		// missing respondInvalidID branches), and session PUT 404.
		bogusU := "not-a-uuid"
		for _, probe := range []struct{ id, method, path, name string }{
			{"HCQA-081", "PUT", "/api/v1/projects/" + bogusU, "project-update-400"},
			{"HCQA-082", "DELETE", "/api/v1/projects/" + bogusU, "project-delete-400"},
			{"HCQA-083", "POST", "/api/v1/tasks/" + bogusU + "/checkpoint", "checkpoint-create-400"},
			{"HCQA-084", "GET", "/api/v1/tasks/" + bogusU + "/checkpoints", "checkpoint-list-400"},
		} {
			body := map[string]any{}
			if probe.name == "project-update-400" {
				body["description"] = "qa-probe"
			} else if probe.name == "checkpoint-create-400" {
				body["checkpoint_name"] = "qa-probe"
			}
			ev, _, ok = authStep(client, dir, probe.id,
				probe.method+" /<malformed-uuid> returns 400 + no postgres leak ("+probe.name+")",
				probe.method, base+probe.path, body, auth, 400,
				func(v map[string]any, raw []byte) error {
					m, _ := v["message"].(string)
					if !strings.Contains(strings.ToLower(m), "invalid") {
						return fmt.Errorf("message %q does not mention invalid input", m)
					}
					bodyStr := string(raw)
					for _, leak := range []string{"SQLSTATE", "22P02", "fkey"} {
						if strings.Contains(bodyStr, leak) {
							return fmt.Errorf("postgres detail leak %q in: %.200s", leak, bodyStr)
						}
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}
		// HCQA-085: PUT /sessions/<bogus> with action=start returns 404
		// (was 500 — updateSession handler wasn't routing ErrSessionNotFound).
		ev, _, ok = authStep(client, dir, "HCQA-085",
			"PUT /sessions/<bogus> action=start returns 404 (was 500)",
			"PUT", base+"/api/v1/sessions/session-bogus-xyz",
			map[string]any{"action": "start"}, auth, 404,
			func(v map[string]any, raw []byte) error {
				m, _ := v["message"].(string)
				if !strings.Contains(strings.ToLower(m), "session") {
					return fmt.Errorf("message %q does not mention session", m)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-074..080: malformed-UUID coverage across the rest of the
		// matrix (sentinel-wired in round 24). Every endpoint accepting
		// :id must return 400 for invalid UUID — not 500 (CONST-035) and
		// not leaking pgx/uuid internals (CONST-042).
		bogusPath := "not-a-uuid-xyz"
		for _, probe := range []struct{ id, method, path, name string }{
			{"HCQA-074", "POST", "/api/v1/tasks/" + bogusPath + "/start", "task-start"},
			{"HCQA-075", "POST", "/api/v1/tasks/" + bogusPath + "/complete", "task-complete"},
			{"HCQA-076", "POST", "/api/v1/tasks/" + bogusPath + "/retry", "task-retry"},
			{"HCQA-077", "POST", "/api/v1/tasks/" + bogusPath + "/fail", "task-fail"},
			{"HCQA-078", "DELETE", "/api/v1/workers/" + bogusPath, "worker-delete"},
			{"HCQA-079", "PUT", "/api/v1/workers/" + bogusPath, "worker-update"},
			{"HCQA-080", "POST", "/api/v1/workers/" + bogusPath + "/heartbeat", "worker-heartbeat"},
		} {
			body := map[string]any{}
			if probe.name == "task-fail" {
				body["error_message"] = "qa-probe"
			} else if probe.name == "worker-update" {
				body["display_name"] = "qa"
			}
			ev, _, ok = authStep(client, dir, probe.id,
				probe.method+" /<malformed-uuid> returns 400 ("+probe.name+")",
				probe.method, base+probe.path, body, auth, 400,
				func(v map[string]any, raw []byte) error {
					m, _ := v["message"].(string)
					if !strings.Contains(strings.ToLower(m), "invalid") {
						return fmt.Errorf("message %q does not mention invalid input", m)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-073: malformed UUID on DELETE /tasks returns 400 (BUG #27).
		// Pre-fix: 500 with "invalid task ID: invalid UUID length: 10" —
		// CONST-035 wrong-HTTP-code (5xx for client-input error).
		// Introduced `task.ErrInvalidTaskID` sentinel + handler-level
		// `respondInvalidID` helper that maps to 400 Bad Request.
		ev, _, ok = authStep(client, dir, "HCQA-073",
			"DELETE /tasks/<malformed-uuid> returns 400 (was 500)",
			"DELETE", base+"/api/v1/tasks/not-a-uuid", nil, auth, 400,
			func(v map[string]any, raw []byte) error {
				if v["status"] != "error" {
					return fmt.Errorf("status=%v want \"error\"", v["status"])
				}
				m, _ := v["message"].(string)
				if !strings.Contains(strings.ToLower(m), "invalid") {
					return fmt.Errorf("message %q does not mention invalid input", m)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-071: duplicate hostname on worker create returns 409
		// with NO postgres SQLSTATE / constraint-name leakage (catches
		// BUG #25). Pre-fix: raw "duplicate key value violates unique
		// constraint \"workers_hostname_unique\" (SQLSTATE 23505)"
		// returned as HTTP 500 — CONST-042 schema leakage + CONST-035
		// wrong-HTTP-code.
		dupHost := fmt.Sprintf("qa-dup-host-%d", time.Now().UnixNano())
		_, _, _ = authStep(client, dir, "HCQA-071-PRE-W",
			"Create first worker (will be duplicated)",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": dupHost}, auth, 201, nil)
		ev, _, ok = authStep(client, dir, "HCQA-071",
			"POST /workers with duplicate hostname returns 409 (no postgres SQLSTATE leak)",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": dupHost}, auth, 409,
			func(v map[string]any, raw []byte) error {
				if v["status"] != "error" {
					return fmt.Errorf("status=%v want \"error\"", v["status"])
				}
				bodyStr := string(raw)
				// Anti-leak: response must NOT contain postgres internals.
				for _, leak := range []string{"workers_hostname_unique", "SQLSTATE", "duplicate key value"} {
					if strings.Contains(bodyStr, leak) {
						return fmt.Errorf("response leaks postgres detail %q: %.200s", leak, bodyStr)
					}
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-072: workflow planning on bogus project returns 404 (BUG #26).
		ev, _, ok = authStep(client, dir, "HCQA-072",
			"POST /projects/<bogus>/workflows/planning returns 404 (was 500)",
			"POST", base+"/api/v1/projects/00000000-0000-0000-0000-000000000000/workflows/planning",
			map[string]any{}, auth, 404,
			func(v map[string]any, raw []byte) error {
				if v["status"] != "error" {
					return fmt.Errorf("status=%v want \"error\"", v["status"])
				}
				m, _ := v["message"].(string)
				if !strings.Contains(strings.ToLower(m), "project") {
					return fmt.Errorf("message %q does not mention project", m)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-070: heartbeat on bogus worker returns 404 (catches BUG #24).
		// Pre-fix: leaked raw postgres FK constraint error as HTTP 500
		// (CONST-042 schema leakage + CONST-035 wrong-HTTP-code bluff).
		// UpdateWorkerHeartbeat now checks RowsAffected on the
		// workers-table UPDATE BEFORE attempting the worker_metrics
		// INSERT, surfacing ErrWorkerNotFound which the handler maps
		// to 404 with a clean message.
		ev, _, ok = authStep(client, dir, "HCQA-070",
			"POST /workers/<bogus>/heartbeat returns 404 (no postgres FK leak)",
			"POST", base+"/api/v1/workers/00000000-0000-0000-0000-000000000000/heartbeat",
			map[string]any{"metrics": map[string]any{"cpu_usage_percent": 50.0}},
			auth, 404,
			func(v map[string]any, raw []byte) error {
				if v["status"] != "error" {
					return fmt.Errorf("status=%v want \"error\"", v["status"])
				}
				// Anti-leak: response must NOT contain postgres-schema
				// detail like "worker_metrics_worker_id_fkey" or
				// "violates foreign key constraint" — those are
				// CONST-042 leakage of internal schema.
				bodyStr := string(raw)
				if strings.Contains(bodyStr, "fkey") || strings.Contains(bodyStr, "SQLSTATE") {
					return fmt.Errorf("response leaks postgres schema details: %.200s", bodyStr)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-065..069: 5 endpoints that previously returned HTTP 500
		// for "resource not found" (a client-side missing-id error)
		// now correctly return 404. Catches BUG #23.
		bogusID := "00000000-0000-0000-0000-000000000000"
		ev, _, ok = authStep(client, dir, "HCQA-065",
			"DELETE /tasks/<bogus> returns 404 (was 500)",
			"DELETE", base+"/api/v1/tasks/"+bogusID, nil, auth, 404,
			nil)
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		ev, _, ok = authStep(client, dir, "HCQA-066",
			"POST /tasks/<bogus>/fail returns 404 (was 500)",
			"POST", base+"/api/v1/tasks/"+bogusID+"/fail",
			map[string]any{"error_message": "qa-bogus"}, auth, 404, nil)
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		ev, _, ok = authStep(client, dir, "HCQA-067",
			"DELETE /workers/<bogus> returns 404 (was 500)",
			"DELETE", base+"/api/v1/workers/"+bogusID, nil, auth, 404, nil)
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		ev, _, ok = authStep(client, dir, "HCQA-068",
			"PUT /workers/<bogus> returns 404 (was 500)",
			"PUT", base+"/api/v1/workers/"+bogusID,
			map[string]any{"display_name": "x"}, auth, 404, nil)
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		ev, _, ok = authStep(client, dir, "HCQA-069",
			"DELETE /sessions/<bogus> returns 404 (was 500)",
			"DELETE", base+"/api/v1/sessions/session-bogus-id", nil, auth, 404, nil)
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-064: re-assign an already-assigned task returns 422 (catches BUG #23).
		// Same state-machine sentinel family as BUG #21 (start/complete) and
		// BUG #13 (retry). AssignTask now wraps ErrTaskInvalidStateTransition.
		_, reAssignTResp, _ := authStep(client, dir, "HCQA-064-PRE-TASK",
			"Create task for reassign-422 probe",
			"POST", base+"/api/v1/tasks",
			map[string]any{"name": "qa-reassign-422", "type": "qa"}, auth, 201, nil)
		reAssignTID := ""
		if t, _ := reAssignTResp["task"].(map[string]any); t != nil {
			reAssignTID, _ = t["id"].(string)
		}
		_, reAssignW1Resp, _ := authStep(client, dir, "HCQA-064-PRE-W1",
			"Create first worker for reassign-422 probe",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": fmt.Sprintf("qa-reassign-w1-%d", time.Now().UnixNano())},
			auth, 201, nil)
		reAssignW1ID := ""
		if w, _ := reAssignW1Resp["worker"].(map[string]any); w != nil {
			reAssignW1ID, _ = w["id"].(string)
		}
		_, reAssignW2Resp, _ := authStep(client, dir, "HCQA-064-PRE-W2",
			"Create second worker for reassign-422 probe",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": fmt.Sprintf("qa-reassign-w2-%d", time.Now().UnixNano())},
			auth, 201, nil)
		reAssignW2ID := ""
		if w, _ := reAssignW2Resp["worker"].(map[string]any); w != nil {
			reAssignW2ID, _ = w["id"].(string)
		}
		if reAssignTID != "" && reAssignW1ID != "" && reAssignW2ID != "" {
			_, _, _ = authStep(client, dir, "HCQA-064-PRE-ASSIGN1",
				"First assign (pending → assigned)",
				"POST", base+"/api/v1/tasks/"+reAssignTID+"/assign",
				map[string]any{"worker_id": reAssignW1ID}, auth, 200, nil)
			ev, _, ok = authStep(client, dir, "HCQA-064",
				"POST /tasks/:id/assign on already-assigned task returns 422 (not 500)",
				"POST", base+"/api/v1/tasks/"+reAssignTID+"/assign",
				map[string]any{"worker_id": reAssignW2ID}, auth, 422,
				func(v map[string]any, raw []byte) error {
					m, _ := v["message"].(string)
					if !strings.Contains(strings.ToLower(m), "pending") {
						return fmt.Errorf("message %q does not mention pending requirement", m)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-061..063: state-machine constraint enforcement (catches BUG #21).
		// Pre-fix: complete-on-pending, start-on-running, complete-on-completed
		// all returned 500 (server-error) for what are client-side state
		// errors. Now they return 422 with a clear "prerequisite state"
		// message via the ErrTaskInvalidStateTransition sentinel.
		_, smTResp, _ := authStep(client, dir, "HCQA-061-PRE-CREATE",
			"Create task for state-machine probe",
			"POST", base+"/api/v1/tasks",
			map[string]any{"name": "qa-state-machine", "type": "qa"}, auth, 201, nil)
		smTID := ""
		if t, _ := smTResp["task"].(map[string]any); t != nil {
			smTID, _ = t["id"].(string)
		}
		if smTID != "" {
			// HCQA-061: complete on pending task → 422 (catches BUG #21).
			ev, _, ok = authStep(client, dir, "HCQA-061",
				"POST /tasks/:id/complete on pending task returns 422 (not 500)",
				"POST", base+"/api/v1/tasks/"+smTID+"/complete",
				map[string]any{"result": map[string]any{}}, auth, 422,
				func(v map[string]any, raw []byte) error {
					if v["status"] != "error" {
						return fmt.Errorf("status=%v want \"error\"", v["status"])
					}
					m, _ := v["message"].(string)
					if !strings.Contains(strings.ToLower(m), "running") {
						return fmt.Errorf("message %q does not mention prerequisite state", m)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-062: start it (legitimate transition pending→running).
			_, _, _ = authStep(client, dir, "HCQA-062-PRE-START",
				"Start task for state-machine probe",
				"POST", base+"/api/v1/tasks/"+smTID+"/start",
				map[string]any{}, auth, 200, nil)
			// HCQA-062: start AGAIN (already running) → 422.
			ev, _, ok = authStep(client, dir, "HCQA-062",
				"POST /tasks/:id/start on already-running task returns 422 (not 500)",
				"POST", base+"/api/v1/tasks/"+smTID+"/start",
				map[string]any{}, auth, 422,
				func(v map[string]any, raw []byte) error {
					m, _ := v["message"].(string)
					if !strings.Contains(strings.ToLower(m), "pending") {
						return fmt.Errorf("message %q does not mention pending requirement", m)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-063: complete it then complete-again → 422.
			_, _, _ = authStep(client, dir, "HCQA-063-PRE-COMPLETE",
				"Complete task for state-machine probe",
				"POST", base+"/api/v1/tasks/"+smTID+"/complete",
				map[string]any{"result": map[string]any{}}, auth, 200, nil)
			ev, _, ok = authStep(client, dir, "HCQA-063",
				"POST /tasks/:id/complete on already-completed task returns 422 (not 500)",
				"POST", base+"/api/v1/tasks/"+smTID+"/complete",
				map[string]any{"result": map[string]any{}}, auth, 422,
				func(v map[string]any, raw []byte) error {
					m, _ := v["message"].(string)
					if !strings.Contains(strings.ToLower(m), "running") {
						return fmt.Errorf("message %q does not mention running requirement", m)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-058..060: positive-coverage for state-accuracy invariants.
		// These probe areas where bugs were FEARED but not found in
		// round 17. Mechanically asserting the invariants now prevents
		// regression into the bluff patterns we already fixed elsewhere.

		// HCQA-058: complete with result body round-trips into result_data.
		// (Field-naming asymmetry: request key is "result", response key
		// is "result_data" — both work, just inconsistent. Worth a probe
		// so any future "silently-discarded result" regression surfaces.)
		_, completeTResp, _ := authStep(client, dir, "HCQA-058-PRE-CREATE",
			"Create task for complete-result-data round-trip",
			"POST", base+"/api/v1/tasks",
			map[string]any{"name": "qa-result-data-task", "type": "qa"}, auth, 201, nil)
		completeTID := ""
		if t, _ := completeTResp["task"].(map[string]any); t != nil {
			completeTID, _ = t["id"].(string)
		}
		if completeTID != "" {
			_, _, _ = authStep(client, dir, "HCQA-058-PRE-START",
				"Start task for complete probe",
				"POST", base+"/api/v1/tasks/"+completeTID+"/start",
				map[string]any{}, auth, 200, nil)
			ev, _, ok = authStep(client, dir, "HCQA-058",
				"POST /tasks/:id/complete with result body round-trips into result_data",
				"POST", base+"/api/v1/tasks/"+completeTID+"/complete",
				map[string]any{"result": map[string]any{
					"output":    "qa-anti-bluff-output",
					"exit_code": 0,
				}}, auth, 200,
				func(v map[string]any, raw []byte) error {
					t, _ := v["task"].(map[string]any)
					if t == nil {
						return fmt.Errorf("task field missing")
					}
					if t["status"] != "completed" {
						return fmt.Errorf("task.status=%v want \"completed\"", t["status"])
					}
					rd, _ := t["result_data"].(map[string]any)
					if rd == nil {
						return fmt.Errorf("result_data missing — request->response field bridge broken")
					}
					if rd["output"] != "qa-anti-bluff-output" {
						return fmt.Errorf("result_data.output=%v did not round-trip", rd["output"])
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-059: /tasks list ordered DESC by created_at — first
		// element is the most recently created task. Catches any
		// regression that flips the ORDER BY direction or drops it.
		listOrderHost := fmt.Sprintf("qa-list-order-%d", time.Now().UnixNano())
		_, listOrderResp, _ := authStep(client, dir, "HCQA-059-PRE-CREATE",
			"Create task for list-ordering probe (most-recent-must-be-first)",
			"POST", base+"/api/v1/tasks",
			map[string]any{"name": listOrderHost, "type": "qa"}, auth, 201, nil)
		listOrderTID := ""
		if t, _ := listOrderResp["task"].(map[string]any); t != nil {
			listOrderTID, _ = t["id"].(string)
		}
		if listOrderTID != "" {
			ev, _, ok = authStep(client, dir, "HCQA-059",
				"GET /tasks list ordered DESC by created_at (first = most recent)",
				"GET", base+"/api/v1/tasks", nil, auth, 200,
				func(v map[string]any, raw []byte) error {
					arr, _ := v["tasks"].([]any)
					if len(arr) == 0 {
						return fmt.Errorf("tasks list is empty after just creating one (BUG #2 regression?)")
					}
					first, _ := arr[0].(map[string]any)
					if first == nil {
						return fmt.Errorf("first task is not a map")
					}
					if first["id"] != listOrderTID {
						return fmt.Errorf("first task id=%v != just-created %q (ORDER BY desc broken)",
							first["id"], listOrderTID)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-060: /system/stats correctly counts created tasks/workers.
		// Captures the BEFORE counts, creates one of each, asserts AFTER
		// counts rose by exactly 1. Catches any "stats hardcoded to 0"
		// or "stats stale" regression.
		_, beforeStatsResp, _ := authStep(client, dir, "HCQA-060-PRE-STATS",
			"Capture /system/stats counters before adding new task+worker",
			"GET", base+"/api/v1/system/stats", nil, auth, 200, nil)
		beforeTasks, beforeWorkers := 0.0, 0.0
		if s, _ := beforeStatsResp["stats"].(map[string]any); s != nil {
			if t, _ := s["tasks"].(map[string]any); t != nil {
				beforeTasks, _ = t["total"].(float64)
			}
			if w, _ := s["workers"].(map[string]any); w != nil {
				beforeWorkers, _ = w["total"].(float64)
			}
		}
		_, _, _ = authStep(client, dir, "HCQA-060-PRE-CREATE-T",
			"Create new task for stats-counter probe",
			"POST", base+"/api/v1/tasks",
			map[string]any{"name": "qa-stats-task", "type": "qa"}, auth, 201, nil)
		_, _, _ = authStep(client, dir, "HCQA-060-PRE-CREATE-W",
			"Create new worker for stats-counter probe",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": fmt.Sprintf("qa-stats-w-%d", time.Now().UnixNano())}, auth, 201, nil)
		ev, _, ok = authStep(client, dir, "HCQA-060",
			"GET /system/stats counters rose by 1 after creating new task+worker",
			"GET", base+"/api/v1/system/stats", nil, auth, 200,
			func(v map[string]any, raw []byte) error {
				s, _ := v["stats"].(map[string]any)
				if s == nil {
					return fmt.Errorf("stats field missing")
				}
				t, _ := s["tasks"].(map[string]any)
				w, _ := s["workers"].(map[string]any)
				afterTasks, _ := t["total"].(float64)
				afterWorkers, _ := w["total"].(float64)
				if afterTasks != beforeTasks+1 {
					return fmt.Errorf("tasks.total went %v→%v, expected +1 (BUG: stats stale or hardcoded)",
						beforeTasks, afterTasks)
				}
				if afterWorkers != beforeWorkers+1 {
					return fmt.Errorf("workers.total went %v→%v, expected +1 (BUG: stats stale or hardcoded)",
						beforeWorkers, afterWorkers)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-HB-*: heartbeat metrics persistence to snapshot columns.
		// Catches BUG #20: heartbeat updated workers.last_heartbeat +
		// inserted into worker_metrics time-series table, but NEVER
		// updated the worker's cpu_usage_percent / memory_usage_percent
		// / disk_usage_percent snapshot columns. GET /workers/:id
		// always returned 0 for those fields regardless of how many
		// heartbeats had landed. Schema has the columns; handler
		// claimed success without writing them.
		hbWorker := fmt.Sprintf("qa-hb-w-%d", time.Now().UnixNano())
		_, hbWResp, _ := authStep(client, dir, "HCQA-HB-PRE-W",
			"Create worker for heartbeat-snapshot probe",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": hbWorker}, auth, 201, nil)
		hbWID := ""
		if w, _ := hbWResp["worker"].(map[string]any); w != nil {
			hbWID, _ = w["id"].(string)
		}
		if hbWID != "" {
			_, _, _ = authStep(client, dir, "HCQA-HB-PRE-BEAT",
				"Send heartbeat with metrics",
				"POST", base+"/api/v1/workers/"+hbWID+"/heartbeat",
				map[string]any{"metrics": map[string]any{
					"cpu_usage_percent":    75.0,
					"memory_usage_percent": 60.0,
					"disk_usage_percent":   40.0,
				}}, auth, 200, nil)

			// HCQA-057: GET /workers/:id reflects the heartbeat values
			// in the snapshot columns (catches BUG #20).
			ev, _, ok = authStep(client, dir, "HCQA-057",
				"GET /workers/:id reflects heartbeat metrics in snapshot columns",
				"GET", base+"/api/v1/workers/"+hbWID, nil, auth, 200,
				func(v map[string]any, raw []byte) error {
					w, _ := v["worker"].(map[string]any)
					if w == nil {
						return fmt.Errorf("worker field missing")
					}
					cpu, _ := w["cpu_usage_percent"].(float64)
					mem, _ := w["memory_usage_percent"].(float64)
					disk, _ := w["disk_usage_percent"].(float64)
					if cpu < 1 || mem < 1 || disk < 1 {
						return fmt.Errorf("workers.cpu/mem/disk=%v/%v/%v — heartbeat metrics "+
							"not reaching snapshot columns (BUG #20 regression)", cpu, mem, disk)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-CKPT-*: checkpoint persistence end-to-end.
		// Catches BUG #19 — POST /tasks/:id/checkpoint used to return 201
		// "success" with `_, _ = m.db.Exec(...)` silently discarding the
		// task_checkpoints INSERT error (failing because the schema
		// required worker_id but the INSERT omitted it). GET checkpoints
		// stayed perpetually empty — triple bluff: discarded error,
		// fabricated success response, history table never populated.
		// Fix added ErrCheckpointRequiresAssignment sentinel + surfaces
		// the INSERT error + maps to 422 vs 500.
		ckptW := fmt.Sprintf("qa-ckpt-w-%d", time.Now().UnixNano())
		_, ckptWResp, _ := authStep(client, dir, "HCQA-CKPT-PRE-W",
			"Create worker for checkpoint lifecycle probe",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": ckptW}, auth, 201, nil)
		ckptWID := ""
		if w, _ := ckptWResp["worker"].(map[string]any); w != nil {
			ckptWID, _ = w["id"].(string)
		}
		_, ckptTResp, _ := authStep(client, dir, "HCQA-CKPT-PRE-T",
			"Create task for checkpoint lifecycle probe",
			"POST", base+"/api/v1/tasks",
			map[string]any{"name": "qa-ckpt-task", "type": "qa"}, auth, 201, nil)
		ckptTID := ""
		if t, _ := ckptTResp["task"].(map[string]any); t != nil {
			ckptTID, _ = t["id"].(string)
		}

		if ckptTID != "" {
			// HCQA-054: checkpoint on UNASSIGNED task returns 422 (catches
			// BUG #19 — was 201 fake-success). Run BEFORE the assign step.
			ev, _, ok = authStep(client, dir, "HCQA-054",
				"POST /tasks/:id/checkpoint on unassigned task returns 422 (not fake-201)",
				"POST", base+"/api/v1/tasks/"+ckptTID+"/checkpoint",
				map[string]any{"checkpoint_name": "qa-ckpt", "checkpoint_data": map[string]any{"step": "before-assign"}},
				auth, 422,
				func(v map[string]any, raw []byte) error {
					if v["status"] != "error" {
						return fmt.Errorf("status=%v want \"error\"", v["status"])
					}
					m, _ := v["message"].(string)
					if !strings.Contains(strings.ToLower(m), "assigned") {
						return fmt.Errorf("message %q does not mention assignment requirement", m)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// Assign then checkpoint → 201 → GET shows it.
			if ckptWID != "" {
				_, _, _ = authStep(client, dir, "HCQA-CKPT-PRE-ASSIGN",
					"Assign task for checkpoint history probe",
					"POST", base+"/api/v1/tasks/"+ckptTID+"/assign",
					map[string]any{"worker_id": ckptWID}, auth, 200, nil)

				// HCQA-055: checkpoint on assigned task returns 201.
				ev, _, ok = authStep(client, dir, "HCQA-055",
					"POST /tasks/:id/checkpoint on assigned task returns 201",
					"POST", base+"/api/v1/tasks/"+ckptTID+"/checkpoint",
					map[string]any{
						"checkpoint_name": "qa-anti-bluff-cp",
						"checkpoint_data": map[string]any{"step": "qa-probe"},
					},
					auth, 201,
					func(v map[string]any, raw []byte) error {
						if v["status"] != "success" {
							return fmt.Errorf("status=%v want \"success\"", v["status"])
						}
						return nil
					})
				results = append(results, ev)
				if ok {
					passed++
				} else {
					failed++
				}

				// HCQA-056: GET /tasks/:id/checkpoints now contains the
				// just-created checkpoint (proves history is actually
				// persisted — catches the silently-discarded-INSERT bug).
				ev, _, ok = authStep(client, dir, "HCQA-056",
					"GET /tasks/:id/checkpoints returns the just-created checkpoint (history persistence)",
					"GET", base+"/api/v1/tasks/"+ckptTID+"/checkpoints",
					nil, auth, 200,
					func(v map[string]any, raw []byte) error {
						arr, _ := v["checkpoints"].([]any)
						if len(arr) < 1 {
							return fmt.Errorf("checkpoints list is empty — BUG #19 regression " +
								"(INSERT into task_checkpoints silently failing again?)")
						}
						cp0, _ := arr[0].(map[string]any)
						if cp0["name"] != "qa-anti-bluff-cp" {
							return fmt.Errorf("checkpoint.name=%v want \"qa-anti-bluff-cp\"", cp0["name"])
						}
						return nil
					})
				results = append(results, ev)
				if ok {
					passed++
				} else {
					failed++
				}
			}
		}

		// HCQA-051..053: task fail → retry round-trip + project-sessions list.
		// Locks in correct behavior of the canonical fail/retry workflow
		// AND catches any regression in the round-1/2/3/8 fixes
		// (sentinel-based retry semantics, task_data NOT NULL, etc).
		_, failTResp, _ := authStep(client, dir, "HCQA-051-CREATE",
			"Create task for fail/retry lifecycle",
			"POST", base+"/api/v1/tasks",
			map[string]any{"name": "qa-fail-retry-task", "type": "qa"}, auth, 201, nil)
		failTID := ""
		if t, _ := failTResp["task"].(map[string]any); t != nil {
			failTID, _ = t["id"].(string)
		}
		if failTID != "" {
			_, _, _ = authStep(client, dir, "HCQA-051-START",
				"Start task for fail/retry lifecycle",
				"POST", base+"/api/v1/tasks/"+failTID+"/start",
				map[string]any{}, auth, 200, nil)

			// HCQA-051: fail with error_message persists status + error_message.
			ev, _, ok = authStep(client, dir, "HCQA-051",
				"POST /tasks/:id/fail persists status + error_message",
				"POST", base+"/api/v1/tasks/"+failTID+"/fail",
				map[string]any{"error_message": "qa-anti-bluff-injected-failure"},
				auth, 200,
				func(v map[string]any, raw []byte) error {
					t, _ := v["task"].(map[string]any)
					if t == nil {
						return fmt.Errorf("task field missing")
					}
					if t["status"] != "failed" {
						return fmt.Errorf("task.status=%v want \"failed\"", t["status"])
					}
					if t["error_message"] != "qa-anti-bluff-injected-failure" {
						return fmt.Errorf("error_message=%q did not round-trip", t["error_message"])
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-052: retry a real failed task transitions failed→pending
			// and increments retry_count (proves the round-8 sentinel
			// distinguishes legitimate retries from state errors).
			ev, _, ok = authStep(client, dir, "HCQA-052",
				"POST /tasks/:id/retry on failed task transitions to pending + increments retry_count",
				"POST", base+"/api/v1/tasks/"+failTID+"/retry", nil, auth, 200,
				func(v map[string]any, raw []byte) error {
					t, _ := v["task"].(map[string]any)
					if t == nil {
						return fmt.Errorf("task field missing")
					}
					if t["status"] != "pending" {
						return fmt.Errorf("task.status=%v want \"pending\"", t["status"])
					}
					rc, _ := t["retry_count"].(float64)
					if rc != 1 {
						return fmt.Errorf("retry_count=%v want 1 (proves real increment, not stub)", rc)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-053: GET /projects/:projectId/sessions lists the sessions
		// for the project (uses the sessions-create probe's project_id).
		// Catches any regression where session<->project linkage breaks.
		// The probe uses HCQA-031's project (which got HCQA-035's session
		// attached to it).
		var projForSessionsID string
		for _, r := range results {
			if r.CheckID == "HCQA-031" {
				if idx := strings.Index(r.BodyHead, `"id":"`); idx >= 0 {
					start := idx + len(`"id":"`)
					end := strings.Index(r.BodyHead[start:], `"`)
					if end > 0 {
						projForSessionsID = r.BodyHead[start : start+end]
					}
				}
				break
			}
		}
		if projForSessionsID != "" {
			ev, _, ok = authStep(client, dir, "HCQA-053",
				"GET /projects/:id/sessions returns array of sessions for project",
				"GET", base+"/api/v1/projects/"+projForSessionsID+"/sessions",
				nil, auth, 200,
				func(v map[string]any, raw []byte) error {
					if !strings.Contains(string(raw), `"sessions":[`) {
						return fmt.Errorf("sessions field is not a JSON array (raw=%.200s)",
							string(raw))
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-ASSIGN-*: task assignment + heartbeat lifecycle.
		// Catches BUG #18: GetTask/ListTasks scanned assigned_worker_id
		// from the DB but never set Task.AssignedWorker on the returned
		// struct — every response had `"assigned_worker": null` even
		// after a successful assign. Real persisted state in
		// distributed_tasks.assigned_worker_id was correct; only the
		// JSON response was wrong.
		assignWorkerHost := fmt.Sprintf("qa-assign-w-%d", time.Now().UnixNano())
		_, assignWResp, _ := authStep(client, dir, "HCQA-ASSIGN-PRE-W",
			"Create worker for assign-task lifecycle",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": assignWorkerHost}, auth, 201, nil)
		assignWID := ""
		if w, _ := assignWResp["worker"].(map[string]any); w != nil {
			assignWID, _ = w["id"].(string)
		}
		_, assignTResp, _ := authStep(client, dir, "HCQA-ASSIGN-PRE-T",
			"Create task for assign-task lifecycle",
			"POST", base+"/api/v1/tasks",
			map[string]any{"name": "qa-assign-task", "type": "qa"}, auth, 201, nil)
		assignTID := ""
		if t, _ := assignTResp["task"].(map[string]any); t != nil {
			assignTID, _ = t["id"].(string)
		}
		if assignWID != "" && assignTID != "" {
			ev, _, ok = authStep(client, dir, "HCQA-048",
				"POST /tasks/:id/assign populates assigned_worker (not null)",
				"POST", base+"/api/v1/tasks/"+assignTID+"/assign",
				map[string]any{"worker_id": assignWID}, auth, 200,
				func(v map[string]any, raw []byte) error {
					t, _ := v["task"].(map[string]any)
					if t == nil {
						return fmt.Errorf("task field missing")
					}
					if t["status"] != "assigned" {
						return fmt.Errorf("task.status=%v want \"assigned\"", t["status"])
					}
					aw, _ := t["assigned_worker"].(string)
					if aw == "" {
						return fmt.Errorf("assigned_worker is null/empty — BUG #18 regression " +
							"(GetTask not populating AssignedWorker from DB scan)")
					}
					if aw != assignWID {
						return fmt.Errorf("assigned_worker=%q != requested worker_id %q", aw, assignWID)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-049: GET /tasks/:id round-trips assigned_worker
			// (proves the fix works for GetTask too, not just the inline
			// re-fetch after assign).
			ev, _, ok = authStep(client, dir, "HCQA-049",
				"GET /tasks/:id after assign shows assigned_worker (BUG #18 round-trip)",
				"GET", base+"/api/v1/tasks/"+assignTID, nil, auth, 200,
				func(v map[string]any, raw []byte) error {
					t, _ := v["task"].(map[string]any)
					if t == nil {
						return fmt.Errorf("task field missing")
					}
					aw, _ := t["assigned_worker"].(string)
					if aw != assignWID {
						return fmt.Errorf("GET assigned_worker=%q != %q (BUG #18 regression in GetTask)",
							aw, assignWID)
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-050: POST /workers/:id/heartbeat returns 200 with success.
			ev, _, ok = authStep(client, dir, "HCQA-050",
				"POST /workers/:id/heartbeat returns 200",
				"POST", base+"/api/v1/workers/"+assignWID+"/heartbeat",
				map[string]any{"metrics": map[string]any{"cpu_usage_percent": 50.0}},
				auth, 200,
				func(v map[string]any, raw []byte) error {
					if v["status"] != "success" {
						return fmt.Errorf("status=%v want \"success\"", v["status"])
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-044..047: session action lifecycle + worker-metrics fixes.
		// HCQA-044 catches BUG #17 (worker-metrics null vs []).
		// HCQA-045..047 exercise the session action transitions
		// (start→active, pause→paused, complete→completed, invalid→400)
		// to lock the design in place.
		actSessProjBody := map[string]any{
			"name": "qa-act-session-proj",
			"path": fmt.Sprintf("/tmp/qa-act-session-proj-%d", time.Now().UnixNano()),
		}
		_, actSessProjResp, _ := authStep(client, dir, "HCQA-044-PRE-PROJ",
			"Create project for session-action lifecycle probe",
			"POST", base+"/api/v1/projects", actSessProjBody, auth, 201, nil)
		actSessProjID := ""
		if p, _ := actSessProjResp["project"].(map[string]any); p != nil {
			actSessProjID, _ = p["id"].(string)
		}
		actSessID := ""
		if actSessProjID != "" {
			_, actSessResp, _ := authStep(client, dir, "HCQA-044-PRE-SESS",
				"Create session for action lifecycle probe",
				"POST", base+"/api/v1/sessions",
				map[string]any{"project_id": actSessProjID, "mode": "planning", "name": "qa-act"},
				auth, 201, nil)
			if s, _ := actSessResp["session"].(map[string]any); s != nil {
				actSessID, _ = s["id"].(string)
			}
		}

		// HCQA-044: worker-metrics returns array (catches BUG #17 — 5th
		// instance of nil-slice→null JSON contract bluff). Create a
		// dedicated worker inline so HCQA-044 doesn't depend on the
		// source ordering of HCQA-032 (which runs LATER in the auth
		// flow). Decoupling avoids "evidence file not found" when the
		// probe ordering changes.
		metricsWorkerHost := fmt.Sprintf("qa-metrics-w-%d", time.Now().UnixNano())
		_, metricsWorkerResp, _ := authStep(client, dir, "HCQA-044-PRE-W",
			"Create worker for metrics probe",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": metricsWorkerHost}, auth, 201, nil)
		metricsWID := ""
		if w, _ := metricsWorkerResp["worker"].(map[string]any); w != nil {
			metricsWID, _ = w["id"].(string)
		}
		if metricsWID != "" {
			ev, _, ok = authStep(client, dir, "HCQA-044",
				"GET /workers/:id/metrics returns array (not null)",
				"GET", base+"/api/v1/workers/"+metricsWID+"/metrics", nil, auth, 200,
				func(v map[string]any, raw []byte) error {
					if !strings.Contains(string(raw), `"metrics":[`) {
						return fmt.Errorf("metrics field is not a JSON array (raw=%.200s)",
							string(raw))
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-045..047: session action transitions.
		if actSessID != "" {
			// HCQA-045: action=start transitions paused→active.
			ev, _, ok = authStep(client, dir, "HCQA-045",
				"PUT /sessions/:id action=start transitions to active",
				"PUT", base+"/api/v1/sessions/"+actSessID,
				map[string]any{"action": "start"}, auth, 200,
				func(v map[string]any, raw []byte) error {
					s, _ := v["session"].(map[string]any)
					if s == nil {
						return fmt.Errorf("session field missing")
					}
					if s["status"] != "active" {
						return fmt.Errorf("session.status=%v want \"active\"", s["status"])
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-046: action=complete transitions active→completed.
			ev, _, ok = authStep(client, dir, "HCQA-046",
				"PUT /sessions/:id action=complete transitions to completed",
				"PUT", base+"/api/v1/sessions/"+actSessID,
				map[string]any{"action": "complete"}, auth, 200,
				func(v map[string]any, raw []byte) error {
					s, _ := v["session"].(map[string]any)
					if s == nil {
						return fmt.Errorf("session field missing")
					}
					if s["status"] != "completed" {
						return fmt.Errorf("session.status=%v want \"completed\"", s["status"])
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}

			// HCQA-047: invalid action returns 400 with clear error.
			ev, _, ok = authStep(client, dir, "HCQA-047",
				"PUT /sessions/:id with invalid action returns 400",
				"PUT", base+"/api/v1/sessions/"+actSessID,
				map[string]any{"action": "invalid-xyz"}, auth, 400,
				func(v map[string]any, raw []byte) error {
					if v["status"] != "error" {
						return fmt.Errorf("status=%v want \"error\"", v["status"])
					}
					return nil
				})
			results = append(results, ev)
			if ok {
				passed++
			} else {
				failed++
			}
		}

		// HCQA-035: create session (requires a real project_id from
		// the round-4 create-project probe + a valid Mode). Catches
		// any regression where the sessions handler returns a session
		// stub without persisting the linked project_id, or rejects a
		// valid mode. We snapshot the just-created project's id from
		// the earlier list-projects step's body excerpt.
		projectsBody := ""
		for _, r := range results {
			if r.CheckID == "HCQA-025" {
				projectsBody = r.BodyHead
				break
			}
		}
		if idx := strings.Index(projectsBody, `"id":"`); idx >= 0 {
			start := idx + len(`"id":"`)
			end := strings.Index(projectsBody[start:], `"`)
			if end > 0 {
				projID := projectsBody[start : start+end]
				ev, _, ok = authStep(client, dir, "HCQA-035",
					"POST /sessions with valid project_id + mode returns 201",
					"POST", base+"/api/v1/sessions",
					map[string]any{
						"project_id":  projID,
						"mode":        "planning",
						"name":        "qa-anti-bluff-session",
						"description": "qa-anti-bluff probe",
					},
					auth, 201,
					func(v map[string]any, raw []byte) error {
						sess, _ := v["session"].(map[string]any)
						if sess == nil {
							return fmt.Errorf("session field missing")
						}
						if sess["project_id"] != projID {
							return fmt.Errorf("session.project_id=%v != requested %q",
								sess["project_id"], projID)
						}
						if sess["mode"] != "planning" {
							return fmt.Errorf("session.mode=%v != \"planning\"", sess["mode"])
						}
						return nil
					})
				results = append(results, ev)
				if ok {
					passed++
				} else {
					failed++
				}
			}
		}

		// HCQA-026: list sessions returns 200 with array.
		ev, _, ok = authStep(client, dir, "HCQA-026",
			"List sessions returns array",
			"GET", base+"/api/v1/sessions", nil, auth, 200,
			func(v map[string]any, raw []byte) error {
				if !strings.Contains(string(raw), `"sessions":[`) {
					return fmt.Errorf("sessions field is not a JSON array (raw head=%.200s)",
						string(raw))
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-027: list workers returns 200 with array (catches the
		// same nil-slice→null JSON contract bluff in listWorkers as
		// the listTasks/listProjects path).
		ev, _, ok = authStep(client, dir, "HCQA-027",
			"List workers returns array (not null)",
			"GET", base+"/api/v1/workers", nil, auth, 200,
			func(v map[string]any, raw []byte) error {
				if !strings.Contains(string(raw), `"workers":[`) {
					return fmt.Errorf("workers field is not a JSON array (raw head=%.200s)",
						string(raw))
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-033: POST /auth/refresh with valid Bearer issues a new JWT.
		// Pre-fix: refreshToken handler called c.Get("user") but the
		// /auth group has NO authMiddleware (register/login must stay
		// public) — every refresh attempt 401'd, even with a perfectly
		// valid token. Same context-key bug pattern as listProjects
		// (BUG #3). Now manually parses+verifies the Authorization
		// header via VerifyJWTWithDB, mirroring logout's pattern.
		// Note: JWT iat (issued-at) is in seconds-since-epoch. If login
		// and refresh happen within the same second, the new JWT can
		// be byte-identical to the old one — the timestamps + payload
		// are the same. So we assert "valid JWT is returned" but do
		// NOT assert "different from the input" (the pre-fix behavior
		// was 401 regardless of input, so "valid 200 with eyJ token"
		// is already sufficient evidence that the refresh path works).
		ev, refreshResp, ok := authStep(client, dir, "HCQA-033",
			"POST /auth/refresh with valid Bearer returns new JWT",
			"POST", base+"/api/v1/auth/refresh", nil, auth, 200,
			func(v map[string]any, raw []byte) error {
				tok, _ := v["token"].(string)
				if len(tok) < 32 || !strings.HasPrefix(tok, "eyJ") {
					return fmt.Errorf("refresh token=%q is not a JWT", tok)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
		_ = refreshResp

		// HCQA-034: POST /auth/refresh with garbage Bearer must 401.
		ev, _, ok = authStep(client, dir, "HCQA-034",
			"POST /auth/refresh with garbage Bearer must return 401",
			"POST", base+"/api/v1/auth/refresh", nil,
			map[string]string{"Authorization": "Bearer garbage-token"}, 401, nil)
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-032: POST /workers with empty ssh_config + capabilities
		// must succeed (catches the ssh_config/capabilities NOT NULL
		// constraint violation — same pattern as the task_data NOT NULL
		// bug). Pre-fix this was a 500 "null value violates not-null"
		// from postgres; post-fix the manager defaults nil to empty.
		workerHost := fmt.Sprintf("qa-worker-%d", time.Now().UnixNano())
		ev, _, ok = authStep(client, dir, "HCQA-032",
			"POST /workers with empty ssh_config + capabilities returns 201",
			"POST", base+"/api/v1/workers",
			map[string]any{"hostname": workerHost, "display_name": "qa-worker"},
			auth, 201,
			func(v map[string]any, raw []byte) error {
				w, _ := v["worker"].(map[string]any)
				if w == nil {
					return fmt.Errorf("worker field missing")
				}
				id, _ := w["id"].(string)
				if len(id) < 32 {
					return fmt.Errorf("worker.id=%q too short to be a UUID", id)
				}
				if w["hostname"] != workerHost {
					return fmt.Errorf("worker.hostname=%v != %q", w["hostname"], workerHost)
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-028: system stats endpoint returns expected sub-objects.
		ev, _, ok = authStep(client, dir, "HCQA-028",
			"System stats includes tasks + workers + system sub-objects",
			"GET", base+"/api/v1/system/stats", nil, auth, 200,
			func(v map[string]any, raw []byte) error {
				s, _ := v["stats"].(map[string]any)
				if s == nil {
					return fmt.Errorf("stats field missing")
				}
				for _, k := range []string{"tasks", "workers", "system"} {
					if _, ok := s[k]; !ok {
						return fmt.Errorf("stats.%s missing", k)
					}
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}

		// HCQA-029: logout (renumbered from former HCQA-019/HCQA-025/HCQA-027).
		ev, _, ok = authStep(client, dir, "HCQA-029",
			"Logout with valid token returns 200",
			"POST", base+"/api/v1/auth/logout", nil, auth, 200,
			func(v map[string]any, raw []byte) error {
				if v["status"] != "success" {
					return fmt.Errorf("status=%v want \"success\"", v["status"])
				}
				return nil
			})
		results = append(results, ev)
		if ok {
			passed++
		} else {
			failed++
		}
	}

	return results, passed, failed
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
