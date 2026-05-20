// Unit tests for the internal/server CONST-046 i18n seam helpers
// (round-350 §11.4 anti-bluff sweep, 2026-05-20, CONST-046 Phase 4).
// Mocks ALLOWED per CONST-050(A) — unit-test file invoked without the
// integration build tag.
package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeTranslator is a unit-test-only Translator that resolves a
// fixed message ID to a fixed locale-specific string, proving the
// seam genuinely routes through the wired Translator rather than
// echoing the raw ID. CONST-050(A): mock confined to *_test.go.
type fakeTranslator struct {
	resolved map[string]string
	failID   string
}

func (f fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	if id == f.failID {
		return "", errors.New("forced translator failure")
	}
	if v, ok := f.resolved[id]; ok {
		return v, nil
	}
	return id, nil
}

func (f fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return f.T(context.Background(), id, nil)
}

// TestReqCtx_NilSafe is the paired mutation guard for the latent
// nil-Request panic: gin.CreateTestContext leaves c.Request nil, and
// reqCtx MUST degrade to context.Background() instead of panicking.
func TestReqCtx_NilSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w) // c.Request is nil here

	// Before the round-350 fix this line panicked with a nil-pointer
	// dereference inside net/http.(*Request).Context.
	got := reqCtx(c)
	if got == nil {
		t.Fatalf("reqCtx returned nil context for nil c.Request")
	}

	// Fully nil gin.Context also degrades safely.
	if reqCtx(nil) == nil {
		t.Fatalf("reqCtx(nil) returned nil context")
	}
}

// TestReqCtx_UsesRequestContext proves reqCtx returns the real
// request-scoped context when c.Request is present (not always
// Background) — the paired half of TestReqCtx_NilSafe.
func TestReqCtx_UsesRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	type ctxKey struct{}
	parent := context.WithValue(context.Background(), ctxKey{}, "round-350")
	req, _ := http.NewRequestWithContext(parent, "GET", "/", nil)
	c.Request = req

	if v := reqCtx(c).Value(ctxKey{}); v != "round-350" {
		t.Fatalf("reqCtx did not return the request-scoped context: value=%v", v)
	}
}

// TestTr_RoutesThroughWiredTranslator is the anti-bluff paired
// mutation for the CONST-046 migration: a wired Translator MUST
// change the output. If tr() were a no-op (the bluff this guards
// against), the assertion below would fail because the output would
// equal the raw message ID instead of the locale string.
func TestTr_RoutesThroughWiredTranslator(t *testing.T) {
	defer SetTranslator(nil) // restore NoopTranslator for other tests

	const id = "internal_server_invalid_request"
	SetTranslator(fakeTranslator{resolved: map[string]string{
		id: "Zahtjev nije ispravan", // Serbian-locale resolution
	}})

	got := tr(context.Background(), id, nil)
	if got != "Zahtjev nije ispravan" {
		t.Fatalf("tr did not route through wired Translator: got %q, want locale string", got)
	}

	// Paired mutation: an ID the translator does not know falls back
	// to the raw ID (loud echo) — never an empty string.
	if echo := tr(context.Background(), "internal_server_unknown_id", nil); echo != "internal_server_unknown_id" {
		t.Fatalf("tr unknown-id fallback: got %q, want loud echo", echo)
	}
}

// TestTr_TranslatorErrorFallsBackToID proves a Translator error
// degrades to the loud message-ID echo rather than an empty string
// (an empty user-facing string would be a §11.4 PASS-bluff).
func TestTr_TranslatorErrorFallsBackToID(t *testing.T) {
	defer SetTranslator(nil)

	const id = "internal_server_failed_generate_token"
	SetTranslator(fakeTranslator{failID: id})

	if got := tr(context.Background(), id, nil); got != id {
		t.Fatalf("tr did not fall back to message ID on translator error: got %q", got)
	}
}

// round408MessageIDs is the closed set of message IDs introduced by
// the round-408 §11.4 CONST-046 genuine-UI residual sweep (auth,
// project, task and worker HTTP-handler response messages). Every ID
// migrated in handlers.go MUST have a backing bundle entry — a
// missing entry is a §11.4 PASS-bluff because the real
// *i18nadapter.Translator would silently echo the raw ID to API
// consumers.
var round408MessageIDs = []string{
	"internal_server_invalid_authorization_header",
	"internal_server_logged_out_successfully",
	"internal_server_authorization_header_required",
	"internal_server_invalid_or_expired_token",
	"internal_server_failed_create_project_directory",
	"internal_server_failed_create_project",
	"internal_server_failed_update_project",
	"internal_server_failed_delete_project",
	"internal_server_project_deleted",
	"internal_server_failed_list_tasks",
	"internal_server_failed_create_task",
	"internal_server_invalid_task_status",
	"internal_server_failed_update_task",
	"internal_server_task_updated_failed_retrieve",
	"internal_server_failed_delete_task",
	"internal_server_task_deleted",
	"internal_server_failed_list_workers",
	"internal_server_worker_hostname_too_long",
	"internal_server_worker_hostname_in_use",
	"internal_server_failed_register_worker",
	"internal_server_failed_update_worker",
	"internal_server_failed_delete_worker",
	"internal_server_worker_deleted_successfully",
	"internal_server_heartbeat_received",
	"internal_server_failed_update_heartbeat",
}

// TestRound408BundleCoverage is the paired-mutation guard for the
// round-408 CONST-046 migration: it asserts every message ID newly
// referenced in handlers.go has a corresponding key in the active
// English bundle. If a future edit deletes a bundle entry (the bluff
// this guards against), the real translator would echo the raw ID to
// every API consumer — this test FAILs and catches it pre-build.
func TestRound408BundleCoverage(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("i18n", "bundles", "active.en.yaml"))
	if err != nil {
		t.Fatalf("read active.en.yaml bundle: %v", err)
	}
	bundle := string(raw)
	for _, id := range round408MessageIDs {
		if !strings.Contains(bundle, id+":") {
			t.Errorf("round-408 message ID %q missing from active.en.yaml bundle", id)
		}
	}
}

// TestRound408IDsRouteThroughTranslator proves each round-408 ID is
// genuinely resolved by a wired Translator — not a no-op. If tr()
// regressed to echoing IDs, the asserted locale strings below would
// not appear and this test FAILs.
func TestRound408IDsRouteThroughTranslator(t *testing.T) {
	defer SetTranslator(nil)

	resolved := make(map[string]string, len(round408MessageIDs))
	for i, id := range round408MessageIDs {
		// Distinct synthetic locale string per ID so a swapped or
		// dropped mapping is detectable.
		resolved[id] = "loc-" + id + "-" + string(rune('A'+i%26))
	}
	SetTranslator(fakeTranslator{resolved: resolved})

	for _, id := range round408MessageIDs {
		got := tr(context.Background(), id, nil)
		if got != resolved[id] {
			t.Errorf("tr(%q) did not route through wired Translator: got %q, want %q", id, got, resolved[id])
		}
	}
}
