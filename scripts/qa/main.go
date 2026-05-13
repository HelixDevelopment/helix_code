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
	if token != "" {
		ev, _, ok := authStep(client, dir, "HCQA-017",
			"Authenticated /users/me returns the same user",
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

		// HCQA-025: logout (renumbered from former HCQA-019).
		ev, _, ok = authStep(client, dir, "HCQA-025",
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
