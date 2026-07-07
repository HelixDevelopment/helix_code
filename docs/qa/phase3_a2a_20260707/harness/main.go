// Phase-3 A2A (Google Agent-to-Agent) end-to-end proof harness.
//
// Deterministic, reproducible harness for the §11.4.108 runtime signature of
// the HelixLLM A2A server (design: docs/research/07.2026/00_master/
// ACP_A2A_PROVIDER.md §4, implemented in submodules/helix_llm/internal/a2a +
// cmd/a2a-server per the §11.4.101 documented scope note in that package's
// doc comment). It is a BLACK-BOX A2A client — it never imports the server's
// Go types (a real external A2A peer wouldn't either); it speaks only the
// public JSON-RPC 2.0 + Agent Card wire protocol (§11.4.99 latest-source,
// a2a-protocol.org, accessed 2026-07-07).
//
// Subcommands:
//
//	stub-serve <addr>                                   RED-baseline broken A2A server (foreground)
//	discover <base-url> <out.json>                       GET the Agent Card
//	send <base-url> <bearer|-> <prompt> <out.json>       POST message/send (bearer "-" = omit header)
//	send-malformed <base-url> <bearer> <out.json>        POST a malformed JSON-RPC body
//	get <base-url> <bearer> <task-id> <out.json>         POST tasks/get
//	analyze-card <captured.json>                         validate Agent Card schema -> PASS/FAIL
//	analyze-completed <captured.json> <tok1,tok2,...>    validate a completed Task + content oracle -> PASS/FAIL
//	analyze-rejected <captured.json>                     validate the request was correctly rejected -> PASS/FAIL
//	selfvalidate                                         §11.4.107(10) golden-good/golden-bad self-validation (no network)
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: phase3a2a <subcommand> [args...]")
	}
	switch os.Args[1] {
	case "stub-serve":
		cmdStubServe()
	case "discover":
		cmdDiscover()
	case "send":
		cmdSend()
	case "send-malformed":
		cmdSendMalformed()
	case "get":
		cmdGet()
	case "analyze-card":
		cmdAnalyzeCard()
	case "analyze-completed":
		cmdAnalyzeCompleted()
	case "analyze-rejected":
		cmdAnalyzeRejected()
	case "selfvalidate":
		cmdSelfValidate()
	default:
		fatal("unknown subcommand: %s", os.Args[1])
	}
}

// ---- captured envelope: every HTTP round trip records BOTH the status code
// and the raw body, so rejection-vs-processed can be judged from real wire
// evidence, never guessed. ----

type captured struct {
	HTTPStatus int    `json:"http_status"`
	Raw        string `json:"raw"`
}

func writeCaptured(path string, c captured) {
	b, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		fatal("write %s: %v", path, err)
	}
}

func readCaptured(path string) captured {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	var c captured
	if err := json.Unmarshal(b, &c); err != nil {
		fatal("parse captured envelope %s: %v", path, err)
	}
	return c
}

// ---- HTTP client subcommands (real, over-the-wire calls) ----

func cmdDiscover() {
	if len(os.Args) < 4 {
		fatal("usage: discover <base-url> <out.json>")
	}
	base, out := os.Args[2], os.Args[3]
	resp, err := http.Get(base + "/.well-known/agent-card.json")
	if err != nil {
		fatal("GET agent-card: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	writeCaptured(out, captured{HTTPStatus: resp.StatusCode, Raw: string(body)})
	fmt.Printf("DISCOVER-OK: status=%d bytes=%d wrote %s\n", resp.StatusCode, len(body), out)
}

func cmdSend() {
	if len(os.Args) < 6 {
		fatal("usage: send <base-url> <bearer|-> <prompt> <out.json>")
	}
	base, bearer, prompt, out := os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	reqBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  "message/send",
		"params": map[string]any{
			"message": map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{"kind": "text", "text": prompt},
				},
			},
		},
	})
	status, body := postJSON(base+"/a2a", bearer, reqBody)
	writeCaptured(out, captured{HTTPStatus: status, Raw: string(body)})
	fmt.Printf("SEND-OK: status=%d bytes=%d wrote %s\n", status, len(body), out)
}

func cmdSendMalformed() {
	if len(os.Args) < 5 {
		fatal("usage: send-malformed <base-url> <bearer> <out.json>")
	}
	base, bearer, out := os.Args[2], os.Args[3], os.Args[4]
	// Deliberately missing "method" — a genuinely malformed JSON-RPC 2.0
	// request per the envelope the server requires.
	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"params":{}}`)
	status, body := postJSON(base+"/a2a", bearer, reqBody)
	writeCaptured(out, captured{HTTPStatus: status, Raw: string(body)})
	fmt.Printf("SEND-MALFORMED-OK: status=%d bytes=%d wrote %s\n", status, len(body), out)
}

func cmdGet() {
	if len(os.Args) < 6 {
		fatal("usage: get <base-url> <bearer> <task-id> <out.json>")
	}
	base, bearer, taskID, out := os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	reqBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  "tasks/get",
		"params":  map[string]any{"id": taskID},
	})
	status, body := postJSON(base+"/a2a", bearer, reqBody)
	writeCaptured(out, captured{HTTPStatus: status, Raw: string(body)})
	fmt.Printf("GET-OK: status=%d bytes=%d wrote %s\n", status, len(body), out)
}

func postJSON(url, bearer string, reqBody []byte) (int, []byte) {
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		fatal("build request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if bearer != "-" && bearer != "" {
		httpReq.Header.Set("Authorization", "Bearer "+bearer)
	}
	httpc := &http.Client{Timeout: 5 * time.Minute}
	resp, err := httpc.Do(httpReq)
	if err != nil {
		fatal("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// ---- analyzers (pure functions over captured JSON — the mutation-proofed
// core reused identically for real evidence AND self-validation fixtures) ----

type analysis struct {
	pass    bool
	reasons []string
}

func (a *analysis) fail(format string, args ...any) {
	a.pass = false
	a.reasons = append(a.reasons, fmt.Sprintf(format, args...))
}

func printAnalysis(tag string, a analysis) {
	verdict := "PASS"
	if !a.pass {
		verdict = "FAIL"
	}
	fmt.Printf("[%s] %s\n", tag, verdict)
	for _, r := range a.reasons {
		fmt.Printf("    reason: %s\n", r)
	}
}

// analyzeCard validates the required Agent Card fields (spec §4.4.1 /
// v0.3.0 binding, cited in ACP_A2A_PROVIDER.md §1.2). Structural validation,
// not just "HTTP 200" — an HTTP-200 empty object must FAIL this.
func analyzeCard(raw string) analysis {
	a := analysis{pass: true}
	var card map[string]any
	if err := json.Unmarshal([]byte(raw), &card); err != nil {
		a.fail("agent card is not valid JSON: %v", err)
		return a
	}
	requireNonEmptyString := func(field string) {
		v, ok := card[field].(string)
		if !ok || strings.TrimSpace(v) == "" {
			a.fail("missing/empty required string field %q", field)
		}
	}
	requireNonEmptyString("name")
	requireNonEmptyString("description")
	requireNonEmptyString("version")
	requireNonEmptyString("url")

	skills, ok := card["skills"].([]any)
	if !ok || len(skills) == 0 {
		a.fail("skills[] must be a non-empty array")
	} else {
		for i, sRaw := range skills {
			s, ok := sRaw.(map[string]any)
			if !ok {
				a.fail("skills[%d] is not an object", i)
				continue
			}
			for _, f := range []string{"id", "name", "description"} {
				if v, ok := s[f].(string); !ok || strings.TrimSpace(v) == "" {
					a.fail("skills[%d] missing/empty %q", i, f)
				}
			}
		}
	}

	caps, ok := card["capabilities"].(map[string]any)
	if !ok {
		a.fail("capabilities object is missing")
	} else {
		for _, f := range []string{"streaming", "pushNotifications", "extendedAgentCard"} {
			if _, ok := caps[f]; !ok {
				a.fail("capabilities.%s is missing", f)
			}
		}
	}

	if dim, ok := card["defaultInputModes"].([]any); !ok || len(dim) == 0 {
		a.fail("defaultInputModes[] must be a non-empty array")
	}
	if dom, ok := card["defaultOutputModes"].([]any); !ok || len(dom) == 0 {
		a.fail("defaultOutputModes[] must be a non-empty array")
	}
	return a
}

func cmdAnalyzeCard() {
	if len(os.Args) < 3 {
		fatal("usage: analyze-card <captured.json>")
	}
	c := readCaptured(os.Args[2])
	a := analyzeCard(c.Raw)
	if c.HTTPStatus != http.StatusOK {
		a.fail("http_status=%d, want 200", c.HTTPStatus)
	}
	printAnalysis("AGENT-CARD", a)
	exitOn(a)
}

// rpcResult extracts the "result" object of a JSON-RPC response, and reports
// whether an "error" member was present instead.
func rpcResult(raw string) (result map[string]any, hasError bool, parseErr error) {
	var env map[string]any
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil, false, err
	}
	if _, ok := env["error"]; ok {
		return nil, true, nil
	}
	res, ok := env["result"].(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("no result object in response")
	}
	return res, false, nil
}

// analyzeCompleted is the CONTENT ORACLE + task-lifecycle assertion: the
// task MUST have reached the terminal "completed" state, MUST carry a
// non-empty artifact, and that artifact MUST contain every required token
// (case-insensitive substring match) — proving REAL generated content, not
// an empty/placeholder body (design §4 bullet 4 / task instructions).
func analyzeCompleted(httpStatus int, raw string, tokens []string) analysis {
	a := analysis{pass: true}
	if httpStatus != http.StatusOK {
		a.fail("http_status=%d, want 200", httpStatus)
	}
	result, hasErr, err := rpcResult(raw)
	if err != nil {
		a.fail("could not parse JSON-RPC response: %v", err)
		return a
	}
	if hasErr {
		a.fail("response is a JSON-RPC error, expected a completed Task")
		return a
	}
	status, ok := result["status"].(map[string]any)
	if !ok {
		a.fail("task has no status object")
		return a
	}
	state, _ := status["state"].(string)
	if state != "completed" {
		a.fail("task state = %q, want \"completed\" (non-terminal or wrong state)", state)
	}
	artifacts, ok := result["artifacts"].([]any)
	if !ok || len(artifacts) == 0 {
		a.fail("artifacts[] is missing/empty")
		return a
	}
	var combinedText strings.Builder
	for _, artRaw := range artifacts {
		art, ok := artRaw.(map[string]any)
		if !ok {
			continue
		}
		parts, ok := art["parts"].([]any)
		if !ok {
			continue
		}
		for _, pRaw := range parts {
			p, ok := pRaw.(map[string]any)
			if !ok {
				continue
			}
			if txt, ok := p["text"].(string); ok {
				combinedText.WriteString(txt)
				combinedText.WriteString("\n")
			}
		}
	}
	text := combinedText.String()
	if strings.TrimSpace(text) == "" {
		a.fail("artifact text is empty")
		return a
	}
	lower := strings.ToLower(text)
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(tok)) {
			a.fail("artifact text does not contain required token %q", tok)
		}
	}
	return a
}

func cmdAnalyzeCompleted() {
	if len(os.Args) < 4 {
		fatal("usage: analyze-completed <captured.json> <tok1,tok2,...>")
	}
	c := readCaptured(os.Args[2])
	tokens := strings.Split(os.Args[3], ",")
	a := analyzeCompleted(c.HTTPStatus, c.Raw, tokens)
	printAnalysis("TASK-COMPLETED", a)
	exitOn(a)
}

// analyzeRejected is the auth/malformed golden-bad discriminator: the
// request MUST NOT have been processed to a completed Task. It PASSes
// (correctly rejected) when either the transport rejected it (non-200) or
// the JSON-RPC layer returned an error — and FAILs (a real bug: an
// unauthorized/malformed request was silently processed) if a completed
// Task with content is present.
func analyzeRejected(httpStatus int, raw string) analysis {
	a := analysis{pass: true}
	if httpStatus == http.StatusOK {
		if result, hasErr, err := rpcResult(raw); err == nil && !hasErr {
			if status, ok := result["status"].(map[string]any); ok {
				if state, _ := status["state"].(string); state == "completed" {
					a.fail("request was PROCESSED to a completed Task (http_status=200) — should have been rejected")
					return a
				}
			}
		}
	}
	// Anything else (non-200, or a 200 carrying a JSON-RPC error / no
	// completed task) counts as a correct rejection.
	return a
}

func cmdAnalyzeRejected() {
	if len(os.Args) < 3 {
		fatal("usage: analyze-rejected <captured.json>")
	}
	c := readCaptured(os.Args[2])
	a := analyzeRejected(c.HTTPStatus, c.Raw)
	printAnalysis("CORRECTLY-REJECTED", a)
	exitOn(a)
}

func exitOn(a analysis) {
	if !a.pass {
		os.Exit(1)
	}
}

// ---- §11.4.107(10) self-validation: golden-good/golden-bad, no network ----

func cmdSelfValidate() {
	ok := true

	// ---- Agent Card ----
	goodCard := `{"name":"helixllm-coder-a2a","description":"desc","version":"0.1.0","url":"http://localhost:18441",` +
		`"capabilities":{"streaming":false,"pushNotifications":false,"extendedAgentCard":false},` +
		`"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],` +
		`"skills":[{"id":"generate-code","name":"Generate code","description":"d"}]}`
	r := analyzeCard(goodCard)
	printAnalysis("GOLDEN-GOOD-CARD(expect PASS)", r)
	if !r.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: golden-good card did not PASS")
	}

	badCard := `{"name":"helixllm-coder-a2a","description":"desc","version":"0.1.0","url":"http://localhost:18441",` +
		`"capabilities":{"streaming":false,"pushNotifications":false,"extendedAgentCard":false},` +
		`"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[]}`
	rb := analyzeCard(badCard)
	printAnalysis("GOLDEN-BAD-CARD-EMPTY-SKILLS(expect FAIL)", rb)
	if rb.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: malformed card (empty skills[]) PASSed the analyzer")
	}

	// ---- Completed-task content oracle ----
	goodTask := `{"jsonrpc":"2.0","id":1,"result":{"id":"t1","status":{"state":"completed"},` +
		`"artifacts":[{"parts":[{"kind":"text","text":"func Fibonacci(n int) int { return n }"}]}]}}`
	rg := analyzeCompleted(200, goodTask, []string{"func", "fibonacci"})
	printAnalysis("GOLDEN-GOOD-TASK(expect PASS)", rg)
	if !rg.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: golden-good task did not PASS")
	}

	// golden-bad: stuck in a non-terminal state.
	nonTerminal := `{"jsonrpc":"2.0","id":1,"result":{"id":"t2","status":{"state":"working"},"artifacts":[]}}`
	rn := analyzeCompleted(200, nonTerminal, []string{"func"})
	printAnalysis("GOLDEN-BAD-NONTERMINAL(expect FAIL)", rn)
	if rn.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: non-terminal task PASSed the analyzer")
	}

	// golden-bad: empty/missing artifact.
	emptyArtifact := `{"jsonrpc":"2.0","id":1,"result":{"id":"t3","status":{"state":"completed"},"artifacts":[]}}`
	re := analyzeCompleted(200, emptyArtifact, []string{"func"})
	printAnalysis("GOLDEN-BAD-EMPTY-ARTIFACT(expect FAIL)", re)
	if re.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: empty-artifact task PASSed the analyzer")
	}

	// golden-bad: placeholder artifact (completed, non-empty, but lacks the
	// required content tokens — the regressed-handler-fakes-success case).
	placeholder := `{"jsonrpc":"2.0","id":1,"result":{"id":"t4","status":{"state":"completed"},` +
		`"artifacts":[{"parts":[{"kind":"text","text":"(placeholder response)"}]}]}}`
	rp := analyzeCompleted(200, placeholder, []string{"func", "fibonacci"})
	printAnalysis("GOLDEN-BAD-PLACEHOLDER-ARTIFACT(expect FAIL)", rp)
	if rp.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: placeholder artifact PASSed the content oracle")
	}

	// ---- Rejection discriminator ----
	// golden-good: a genuinely rejected (401) request.
	rejected401 := `{"error":{"message":"invalid API key","type":"invalid_request_error"}}`
	rr := analyzeRejected(401, rejected401)
	printAnalysis("GOLDEN-GOOD-REJECTED(expect PASS)", rr)
	if !rr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: a genuine 401 rejection did not PASS analyze-rejected")
	}

	// golden-bad: an unauthorized/malformed request that was WRONGLY
	// processed to a completed Task — analyze-rejected MUST catch this.
	wronglyProcessed := `{"jsonrpc":"2.0","id":1,"result":{"id":"t5","status":{"state":"completed"},` +
		`"artifacts":[{"parts":[{"kind":"text","text":"func Fibonacci(n int) int { return n }"}]}]}}`
	rw := analyzeRejected(200, wronglyProcessed)
	printAnalysis("GOLDEN-BAD-WRONGLY-PROCESSED(expect FAIL)", rw)
	if rw.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: a wrongly-processed unauthorized/malformed request PASSed analyze-rejected")
	}

	if !ok {
		fmt.Println("[SELF-VALIDATION] FAIL")
		os.Exit(1)
	}
	fmt.Println("[SELF-VALIDATION] PASS: analyzer PASSes every golden-good fixture and FAILs every golden-bad fixture")
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", a...)
	os.Exit(2)
}

// ---- RED-baseline broken A2A server (§11.4.115 RED-on-broken-artifact) ----
//
// cmdStubServe boots a deliberately BROKEN A2A server: its Agent Card is
// missing required fields (empty skills[]/name/description/version/url) and
// its "message/send" handler NEVER reaches the terminal "completed" state
// (it echoes "working" with an empty artifacts[]). Pointed at by the SAME
// client + SAME analyzers used against the real HelixLLM A2A server, this
// proves the analyzer genuinely FAILs on broken behaviour BEFORE the real
// server is asked to PASS — the RED half of the RED->GREEN polarity pair
// (design ACP_A2A_PROVIDER.md §4.1; the GREEN half is the real a2a-server
// binary reached via "discover"/"send"/"get" above). It never imports the
// real server's code (black-box, wire-protocol-only, like a genuine broken
// peer would be).
func cmdStubServe() {
	if len(os.Args) < 3 {
		fatal("usage: stub-serve <addr>")
	}
	addr := os.Args[2]

	mux := http.NewServeMux()

	// Deliberately BROKEN Agent Card: present but missing every field
	// analyzeCard requires (name/description/version/url all empty,
	// skills[] empty, capabilities{} missing its three flags) — the
	// golden-bad-card pattern exercised live over the wire.
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"","description":"","version":"","url":"",` +
			`"capabilities":{},"defaultInputModes":[],"defaultOutputModes":[],"skills":[]}`))
	})

	// Deliberately BROKEN dispatch: any "message/send" is acknowledged but
	// NEVER completes — it stays "working" forever with zero artifacts, the
	// stuck-task / empty-artifact golden-bad pattern (design §4.1 bullet 1)
	// exercised live over the wire, not just as an in-process fixture.
	mux.HandleFunc("/a2a", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var env map[string]any
		_ = json.Unmarshal(body, &env) // best-effort id echo; malformed input still gets a stuck-task reply
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      env["id"],
			"result": map[string]any{
				"id":        "stuck-task-red-baseline",
				"status":    map[string]any{"state": "working"},
				"artifacts": []any{},
			},
		}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})

	fmt.Printf("STUB-SERVE listening on %s (deliberately BROKEN A2A server, RED-baseline §11.4.115)\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fatal("ListenAndServe %s: %v", addr, err)
	}
}
