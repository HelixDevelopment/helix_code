// Unit tests for the internal/event package-level translator +
// tr() helper (CONST-046 round-156 §11.4 anti-bluff sweep,
// 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator
// yields distinguishable output at every migrated call site. Mocks
// ALLOWED per CONST-050(A) (unit tests only).
package event

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	eventi18n "dev.helix.code/internal/event/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests
// can assert tr() actually went through Translator.T rather than
// returning a hardcoded literal that happened to match the bundle
// value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_event_handling_errors", nil)
	if got != "internal_event_handling_errors" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_event_async_handler_error", nil)
	if got != "<TR:internal_event_async_handler_error>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — user sees blank output). Implementation MUST degrade
	// to the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_event_sync_handler_error", nil)
	if got != "internal_event_sync_handler_error" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_event_no_subscribers_log", nil)
	if got != "internal_event_no_subscribers_log" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(eventi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_event_handler_subscribed_log", nil)
	if got != "internal_event_handler_subscribed_log" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestPublish_SyncHandlerError_GoesThroughTranslator covers the
// returned error path on EventBus.Publish (sync mode). With a
// sentinel translator wired, the surfaced error MUST contain the
// sentinel-wrapped message ID — proving the literal is NOT
// hardcoded on the path.
func TestPublish_SyncHandlerError_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	bus := NewEventBus(false)
	bus.Subscribe(EventTaskCompleted, func(_ stdctx.Context, _ Event) error {
		return errors.New("planted handler failure")
	})

	err := bus.Publish(stdctx.Background(), Event{Type: EventTaskCompleted, Source: "test", Severity: SeverityInfo})
	if err == nil {
		t.Fatal("Publish with failing handler returned no error")
	}
	want := "<TR:internal_event_handling_errors>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Publish error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
	// Inner per-handler error message: the sync handler logs via
	// bus.logError using internal_event_sync_handler_error. The
	// sentinel translator returns "<TR:" + id + ">" which the
	// wrapper Errorf preserves verbatim. Inspect captured errors.
	logged := bus.GetErrors()
	if len(logged) == 0 {
		t.Fatal("Publish failure path did not record any errors via logError")
	}
	wantInner := "<TR:internal_event_sync_handler_error>"
	foundInner := false
	for _, le := range logged {
		if strings.Contains(le.Error(), wantInner) {
			foundInner = true
			break
		}
	}
	if !foundInner {
		t.Fatalf("logError captures = %v, want contain %q — per-handler err formatting bypassed tr()", logged, wantInner)
	}
}

// TestPublish_SyncHandlerError_RawTextByDefault asserts that with
// no translator wired (NoopTranslator), the returned error contains
// the raw message ID — confirming the migration didn't
// accidentally drop the format-arg or pass an empty string.
func TestPublish_SyncHandlerError_RawTextByDefault(t *testing.T) {
	resetTranslator(t)

	bus := NewEventBus(false)
	bus.Subscribe(EventTaskCompleted, func(_ stdctx.Context, _ Event) error {
		return errors.New("planted handler failure")
	})

	err := bus.Publish(stdctx.Background(), Event{Type: EventTaskCompleted, Source: "test", Severity: SeverityInfo})
	if err == nil {
		t.Fatal("Publish with failing handler returned no error")
	}
	if !strings.Contains(err.Error(), "internal_event_handling_errors") {
		t.Fatalf("Publish error = %q, want raw message ID (Noop echo)", err.Error())
	}
	// Per-handler err is captured via bus.logError using
	// internal_event_sync_handler_error; under Noop that is the
	// raw message ID echoed verbatim by fmt.Errorf("%s", tr(...)).
	logged := bus.GetErrors()
	foundInner := false
	for _, le := range logged {
		if strings.Contains(le.Error(), "internal_event_sync_handler_error") {
			foundInner = true
			break
		}
	}
	if !foundInner {
		t.Fatalf("logError captures = %v, want raw inner-handler ID", logged)
	}
}

// TestPublishAndWait_HandlerError_GoesThroughTranslator covers the
// async/PublishAndWait code path. The returned error MUST surface
// through tr() — the wait variant has its own message ID
// (internal_event_wait_handler_error) plus the wrapper ID
// (internal_event_handling_errors).
func TestPublishAndWait_HandlerError_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	bus := NewEventBus(true)
	bus.Subscribe(EventTaskCompleted, func(_ stdctx.Context, _ Event) error {
		return errors.New("planted handler failure")
	})

	err := bus.PublishAndWait(stdctx.Background(), Event{Type: EventTaskCompleted, Source: "test", Severity: SeverityInfo})
	if err == nil {
		t.Fatal("PublishAndWait with failing handler returned no error")
	}
	want := "<TR:internal_event_handling_errors>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("PublishAndWait error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
	// Per-handler err: wait variant stores tr-built errors via
	// bus.logError after wg.Wait. Inspect captured slice.
	logged := bus.GetErrors()
	if len(logged) == 0 {
		t.Fatal("PublishAndWait failure path did not record any errors via logError")
	}
	wantInner := "<TR:internal_event_wait_handler_error>"
	foundInner := false
	for _, le := range logged {
		if strings.Contains(le.Error(), wantInner) {
			foundInner = true
			break
		}
	}
	if !foundInner {
		t.Fatalf("logError captures = %v, want contain %q — per-handler err formatting bypassed tr()", logged, wantInner)
	}
}

// TestPublishAndWait_HandlerError_RawTextByDefault asserts the
// Noop-default surface for the wait path.
func TestPublishAndWait_HandlerError_RawTextByDefault(t *testing.T) {
	resetTranslator(t)

	bus := NewEventBus(true)
	bus.Subscribe(EventTaskCompleted, func(_ stdctx.Context, _ Event) error {
		return errors.New("planted handler failure")
	})

	err := bus.PublishAndWait(stdctx.Background(), Event{Type: EventTaskCompleted, Source: "test", Severity: SeverityInfo})
	if err == nil {
		t.Fatal("PublishAndWait with failing handler returned no error")
	}
	if !strings.Contains(err.Error(), "internal_event_handling_errors") {
		t.Fatalf("PublishAndWait error = %q, want raw message ID (Noop echo)", err.Error())
	}
	// Per-handler err captured by bus.logError under Noop produces
	// the raw message ID verbatim.
	logged := bus.GetErrors()
	foundInner := false
	for _, le := range logged {
		if strings.Contains(le.Error(), "internal_event_wait_handler_error") {
			foundInner = true
			break
		}
	}
	if !foundInner {
		t.Fatalf("logError captures = %v, want raw per-handler ID", logged)
	}
}
