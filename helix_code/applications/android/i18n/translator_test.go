// Unit tests for the android Translator interface + NoopTranslator
// default. Mocks ALLOWED per CONST-050(A) (unit tests only).
package i18n

import (
	"context"
	"errors"
	"testing"
)

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "android_connection_failed_title", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "android_connection_failed_title" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(context.Background(), "android_tasks_count", 3, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "android_tasks_count" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
	}
}

// fakeTranslator returns a sentinel-wrapped message ID so call-site
// tests can assert the lookup actually went through Translator.T,
// not a hardcoded literal that happens to match the bundle value.
type fakeTranslator struct {
	failOnID string
}

func (f fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

func (f fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

func TestFakeTranslator_T_WrapsID(t *testing.T) {
	tr := fakeTranslator{}
	got, err := tr.T(context.Background(), "android_connection_failed_title", nil)
	if err != nil {
		t.Fatalf("fakeTranslator.T returned error: %v", err)
	}
	want := "<TRANSLATED:android_connection_failed_title>"
	if got != want {
		t.Fatalf("fakeTranslator.T returned %q, want %q", got, want)
	}
}

// TestNoopTranslator_T_PreservesID_AcrossAllMigratedIDs is the
// round-139 sentinel: it locks in the bundle-declared message IDs
// (loud-echo on Noop, sentinel-wrap on fake) so a future round that
// removes or renames a documented ID without updating the bundle
// fails this test.
func TestNoopTranslator_T_PreservesID_AcrossAllMigratedIDs(t *testing.T) {
	declaredIDs := []string{
		"android_connection_status_connected",
		"android_connection_status_disconnected",
		"android_button_connect",
		"android_button_disconnect",
		"android_button_ok",
		"android_alert_connection_failed_title",
		"android_alert_connection_failed_message",
		"android_user_not_connected",
		"android_layout_task_name_default",
		"android_layout_task_status_default",
	}
	tr := NoopTranslator{}
	for _, id := range declaredIDs {
		got, err := tr.T(context.Background(), id, nil)
		if err != nil {
			t.Fatalf("NoopTranslator.T(%q) returned error: %v", id, err)
		}
		if got != id {
			t.Fatalf("NoopTranslator.T(%q) = %q, want loud-echo %q (CONST-046 regression — bundle ID renamed without test update)", id, got, id)
		}
	}
}

// TestFakeTranslator_FailOnID_PropagatesError verifies that an
// erroring Translator returns a non-nil error rather than silently
// returning empty string — the call-site degradation policy (raw ID
// loud-echo on error) lives in the consumer, not the Translator
// itself.
func TestFakeTranslator_FailOnID_PropagatesError(t *testing.T) {
	tr := fakeTranslator{failOnID: "android_button_connect"}
	got, err := tr.T(context.Background(), "android_button_connect", nil)
	if err == nil {
		t.Fatalf("expected error from fakeTranslator.T with failOnID match; got %q with nil error", got)
	}
	if got != "" {
		t.Fatalf("expected empty string on error; got %q", got)
	}
}
