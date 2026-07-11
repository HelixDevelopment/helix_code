// Package harness contains a strict-SDK regression test for the two A2A
// wire-shape defects documented in
// docs/qa/a2a_live_e2e_20260711T134958Z/RESULTS.md §2.4:
//
//	Finding A — AgentCard.url omits the JSON-RPC dispatch base path, so a
//	spec-faithful client (a2aclient.NewFromCard, which POSTs directly to
//	card.URL per the a2a-go@v0.3.15 a2a/agent.go:84-89 PreferredTransport
//	doc contract) 404s.
//	Finding B — the message/send Task result lacks the top-level
//	"kind":"task" discriminator the real SDK's polymorphic result decoder
//	(a2a.UnmarshalEventJSON) requires to type the JSON as a Task.
//
// This is a BLACK-BOX client test: it never imports HelixLLM's internal/a2a
// package, only the real upstream github.com/a2aproject/a2a-go v0.3.15 SDK +
// net/http (§11.4.146 / §11.4.115 / §11.4.4 — strict-SDK proof, not a
// hand-rolled JSON-RPC/curl fake).
//
// RED-baseline-on-broken-artifact (§11.4.115): this exact test source, run
// against a pre-fix a2a-server binary (card.URL = "http://host:port", no
// base path; Task JSON with no "kind" field), FAILS at
// client.SendMessage(...) with either an HTTP 404 (literal card-driven
// dispatch never reaches the server's real JSON-RPC route) or a
// "could not determine type: unknown event kind" decode error. Run against
// the post-fix binary the SAME test source passes -- one source, two roles,
// per §11.4.115's polarity discipline (this harness reads the artifact
// under test from A2A_BASE_URL, so its own "polarity switch" is simply
// which binary is listening there when the test runs -- captured explicitly
// in RESULTS.md rather than an in-process RED_MODE env var, since the
// artifact itself, not the test's internal logic, is what changes between
// runs).
package harness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2aclient"
	"github.com/a2aproject/a2a-go/a2aclient/agentcard"
)

func genNonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "NONCE_" + hex.EncodeToString(b)
}

func taskArtifactText(t *a2a.Task) string {
	var sb strings.Builder
	for _, art := range t.Artifacts {
		if art == nil {
			continue
		}
		for _, p := range art.Parts {
			if tp, ok := p.(a2a.TextPart); ok {
				sb.WriteString(tp.Text)
			}
		}
	}
	return sb.String()
}

// TestA2AStrictSDKWireShape drives the real a2a-go SDK client against a live
// A2A server (address supplied via A2A_BASE_URL / A2A_BEARER_TOKEN env vars
// -- never hardcoded, CONST-045/046). It:
//
//  1. Resolves the Agent Card via the real SDK's own DefaultResolver.
//  2. Constructs a real a2aclient.Client via a2aclient.NewFromCard, which per
//     spec dispatches JSON-RPC directly to card.URL (the literal, spec-
//     faithful path -- proves Finding A is closed: no 404).
//  3. Sends a fresh-nonce message/send request and asserts the SDK's STRICT
//     polymorphic decode types the result as *a2a.Task (proves Finding B is
//     closed: the "kind":"task" discriminator is present and understood).
//  4. Confirms the live downstream coder genuinely answered (the nonce is
//     unforgeable -- only a live model that actually saw this exact prompt
//     could echo it back, §11.4.143).
//  5. Round-trips tasks/get via the same strict-typed client.GetTask call.
//
// The coder itself is NEVER started, stopped, or reconfigured by this test
// -- only a read-only GET /v1/models reachability probe is issued before
// driving traffic through it (§11.4.119 / §11.4.122 / §11.4.174).
func TestA2AStrictSDKWireShape(t *testing.T) {
	baseURL := os.Getenv("A2A_BASE_URL")
	token := os.Getenv("A2A_BEARER_TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("SKIP-OK: §11.4.3 -- A2A_BASE_URL / A2A_BEARER_TOKEN not set (no live a2a-server target configured for this invocation)")
	}
	coderURL := os.Getenv("A2A_CODER_URL")
	if coderURL == "" {
		coderURL = "http://localhost:18434"
	}

	// Read-only reachability probe of the downstream coder ONLY -- never a
	// start/stop/restart (§11.4.119 / §11.4.122 read-only-downstream).
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer probeCancel()
	preq, perrBuild := http.NewRequestWithContext(probeCtx, http.MethodGet, coderURL+"/v1/models", nil)
	if perrBuild != nil {
		t.Fatalf("build coder probe request: %v", perrBuild)
	}
	presp, perr := http.DefaultClient.Do(preq)
	if perr != nil || presp == nil || presp.StatusCode != http.StatusOK {
		t.Skipf("SKIP-OK: §11.4.3 -- coder at %s not reachable (read-only probe: err=%v)", coderURL, perr)
	}
	_ = presp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// ---- (a) Resolve the real Agent Card via the real SDK resolver ----
	card, err := agentcard.DefaultResolver.Resolve(ctx, baseURL)
	if err != nil {
		t.Fatalf("agentcard.DefaultResolver.Resolve(%q): %v", baseURL, err)
	}
	t.Logf("card.URL=%q card.PreferredTransport=%q", card.URL, card.PreferredTransport)

	httpClient := &http.Client{Timeout: 90 * time.Second}
	credStore := a2aclient.NewInMemoryCredentialsStore()
	sid := a2aclient.SessionID("a2a-wireshape-strict-sdk")
	credStore.Set(sid, a2a.SecuritySchemeName("bearer"), a2aclient.AuthCredential(token))
	authInterceptor := &a2aclient.AuthInterceptor{Service: credStore}

	// ---- (a, cont'd) Construct the client STRICTLY from the card, per spec
	// ("clients should prefer this transport and URL combination" --
	// a2a-go@v0.3.15/a2a/agent.go:84-89). This is the literal, spec-faithful
	// dispatch path -- NOT a harness-side workaround. If Finding A (missing
	// base path in card.URL) is not fixed, EVERY call through cardClient
	// below 404s.
	cardClient, err := a2aclient.NewFromCard(ctx, card,
		a2aclient.WithJSONRPCTransport(httpClient),
		a2aclient.WithInterceptors(authInterceptor),
	)
	if err != nil {
		t.Fatalf("a2aclient.NewFromCard: %v", err)
	}
	defer func() { _ = cardClient.Destroy() }()
	ctx = a2aclient.WithSessionID(ctx, sid)

	nonce := genNonce()
	prompt := fmt.Sprintf("Reply with ONLY this exact token and nothing else: %s", nonce)
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.TextPart{Text: prompt})

	// ---- (b) full message/send via the literal card-driven client ----
	result, sendErr := cardClient.SendMessage(ctx, &a2a.MessageSendParams{Message: msg})
	if sendErr != nil {
		t.Fatalf(
			"cardClient.SendMessage (literal card.URL=%q dispatch) FAILED: %v\n"+
				"This is exactly the failure class the fix (AgentCard.url base path "+
				"+ \"kind\":\"task\" discriminator) closes -- if this test is run "+
				"against a PRE-FIX a2a-server binary, this failure IS the expected "+
				"RED-baseline evidence (§11.4.115).", card.URL, sendErr)
	}

	task, ok := result.(*a2a.Task)
	if !ok {
		t.Fatalf("cardClient.SendMessage returned %T, want *a2a.Task -- the SDK's "+
			"polymorphic decoder (a2a.UnmarshalEventJSON) failed to type the "+
			"result, i.e. the \"kind\":\"task\" discriminator is missing or wrong",
			result)
	}
	artifactText := taskArtifactText(task)
	if !strings.Contains(artifactText, nonce) {
		t.Fatalf("live coder did not echo the fresh nonce %q in the Task artifact (got %q)", nonce, artifactText)
	}
	t.Logf("cardClient.SendMessage strict-decoded *a2a.Task id=%s state=%s nonce_echoed=true",
		task.ID, task.Status.State)

	// ---- tasks/get strict round-trip via the same card-driven client ----
	got, getErr := cardClient.GetTask(ctx, &a2a.TaskQueryParams{ID: task.ID})
	if getErr != nil {
		t.Fatalf("cardClient.GetTask (tasks/get, literal card.URL dispatch) FAILED: %v", getErr)
	}
	if got.ID != task.ID {
		t.Errorf("tasks/get id mismatch: got %v want %v", got.ID, task.ID)
	}
	if !strings.Contains(taskArtifactText(got), nonce) {
		t.Errorf("tasks/get round-trip lost the nonce echo (want %q)", nonce)
	}
	t.Logf("cardClient.GetTask strict-decoded id=%s state=%s nonce_present=true",
		got.ID, got.Status.State)
}
