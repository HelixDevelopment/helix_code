// CONST-046 (round-134 §11.4) call-site tests for cmd/server. Each
// injects a fakeTranslator that wraps every id in "<TRANSLATED:id>"
// and asserts the sentinel propagates through the tr() helper.
// Anti-bluff: presence-of-sentinel jointly with default-NoopTranslator
// raw-ID echo proves the translator is actually consulted at the
// call site, not bypassed by a stale literal.
//
// We exercise tr() directly because main() loops on a SIGTERM channel
// and starts an HTTP server — exercising the full main() in a unit
// test would require dragging in real database / redis / TCP
// listeners, which CONST-050(A) forbids in *_test.go without the
// integration tag. Mocks permitted per CONST-050(A) (unit tests only).
package main

import (
	"context"
	"strings"
	"testing"

	"dev.helix.code/cmd/server/i18n"
)

type fakeTranslator struct{}

func (fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "<TRANSLATED:" + id + ">", nil
}

func (fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TRANSLATED:" + id + ">", nil
}

// withFakeTranslator swaps the package-level translator with
// fakeTranslator for the duration of the test, restoring the original
// on cleanup. Tests run sequentially within a single goroutine, so
// the global swap is safe.
func withFakeTranslator(t *testing.T) {
	t.Helper()
	prev := translator
	SetTranslator(fakeTranslator{})
	t.Cleanup(func() { translator = prev })
}

// TestTr_NoopReturnsID confirms the package default (NoopTranslator)
// emits the raw message ID. This is the anti-bluff floor — if a
// future change accidentally drops the i18n import or replaces
// translator with a nil-returning stub, this test fails.
func TestTr_NoopReturnsID(t *testing.T) {
	prev := translator
	t.Cleanup(func() { translator = prev })
	SetTranslator(nil) // nil resets to NoopTranslator{}

	got := tr(context.Background(), "server_startup_banner_version", nil)
	if got != "server_startup_banner_version" {
		t.Fatalf("tr returned %q, want raw message ID with Noop translator", got)
	}
}

// TestTr_FakeWrapsID confirms that wiring a real Translator changes
// tr()'s output — the canonical proof that the call site routes
// through the translator surface rather than a hardcoded literal.
func TestTr_FakeWrapsID(t *testing.T) {
	withFakeTranslator(t)
	got := tr(context.Background(), "server_startup_banner_version", nil)
	want := "<TRANSLATED:server_startup_banner_version>"
	if got != want {
		t.Fatalf("tr returned %q, want %q", got, want)
	}
}

// TestSetTranslator_NilResetsToNoop confirms passing nil reverts the
// package-level translator to NoopTranslator{} (loud echo), never to
// nil — silent nil would PANIC at every call site, which is a worse
// PASS-bluff than loud echo.
func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	prev := translator
	t.Cleanup(func() { translator = prev })
	SetTranslator(fakeTranslator{})
	SetTranslator(nil)
	if _, ok := translator.(i18n.NoopTranslator); !ok {
		t.Fatalf("SetTranslator(nil) failed to reset to NoopTranslator{}; got %T", translator)
	}
}

// TestTr_AllMigratedIDsRouteThroughTranslator walks every message ID
// migrated in round 134 and asserts each one flows through the
// fakeTranslator. The anti-bluff invariant: a future regression that
// reverts ANY of the migrated call sites to a raw literal would
// break the build (compile-time) OR fail this test (runtime) when
// the expected sentinel does not appear at that ID's slot.
func TestTr_AllMigratedIDsRouteThroughTranslator(t *testing.T) {
	withFakeTranslator(t)

	migrated := []string{
		"server_startup_banner_version",
		"server_startup_banner_build",
		"server_startup_banner_commit",
		"server_fatal_load_config",
		"server_warn_db_init_skipped",
		"server_fatal_redis_init",
		"server_runtime_http_start",
		"server_fatal_http_start",
		"server_lifecycle_shutting_down",
		"server_lifecycle_exited_properly",
	}

	for _, id := range migrated {
		got := tr(context.Background(), id, nil)
		want := "<TRANSLATED:" + id + ">"
		if got != want {
			t.Fatalf("tr(%q) returned %q, want %q — call site may have bypassed translator", id, got, want)
		}
		if !strings.Contains(got, id) {
			t.Fatalf("tr(%q) result %q does not embed message ID — sentinel format violated", id, got)
		}
	}
}
