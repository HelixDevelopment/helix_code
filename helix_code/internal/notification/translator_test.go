// Unit tests for the internal/notification package-level translator +
// tr() helper (CONST-046 round-167 seam; round-385 §11.4 anti-bluff
// sweep extends coverage to the event_handler.go Message bodies,
// 2026-05-20).
//
// Paired-mutation discipline per §11.4: a planted vs. unplanted
// Translator yields distinguishable output at every migrated call
// site, and an interpolating translator proves the {{.Placeholder}}
// substitution actually reaches end users (the round-167 migration
// only covered Title fields — round-385 migrated the Message bodies
// that previously used fmt.Sprintf English literals). Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package notification

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	"dev.helix.code/internal/event"
	notificationi18n "dev.helix.code/internal/notification/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests can
// assert tr() actually went through Translator.T rather than returning
// a hardcoded literal that happened to match the bundle value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

// errTranslator forces the failure path: tr() MUST degrade to the
// message ID, never return an empty string (a §11.4 PASS-bluff at the
// i18n layer would surface a blank notification to the end user).
type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// interpolatingTranslator renders a minimal subset of the round-385
// bundle with real {{.Placeholder}} substitution. It proves the
// migrated Message bodies actually carry the runtime data through to
// the user — the anti-bluff guarantee that the feature works
// end-to-end, not merely that a message ID is echoed.
type interpolatingTranslator struct{}

func (interpolatingTranslator) T(_ stdctx.Context, id string, data map[string]any) (string, error) {
	render := map[string]string{
		"internal_notification_message_task_completed":         "Task {{.TaskID}} completed successfully",
		"internal_notification_message_duration_suffix":        " in {{.Duration}}",
		"internal_notification_message_task_failed":            "Task {{.TaskID}} failed: {{.Error}}",
		"internal_notification_message_workflow_completed":     "Workflow '{{.WorkflowName}}' completed successfully",
		"internal_notification_message_workflow_failed":        "Workflow '{{.WorkflowName}}' failed: {{.Error}}",
		"internal_notification_message_worker_disconnected":    "Worker {{.WorkerID}} ({{.Host}}) disconnected: {{.Reason}}",
		"internal_notification_message_worker_health_degraded": "Worker {{.WorkerID}} ({{.Host}}) health status: {{.HealthStatus}}",
		"internal_notification_message_system_error":           "Error in {{.Component}}: {{.Error}}",
		"internal_notification_message_system_started":         "HelixCode system started",
		"internal_notification_message_version_suffix":         " (version {{.Version}})",
		"internal_notification_message_system_shutdown":        "HelixCode system shutting down: {{.Reason}}",
		"internal_notification_value_unknown_error":            "Unknown error",
	}
	tmpl, ok := render[id]
	if !ok {
		return id, nil
	}
	out := tmpl
	for k, v := range data {
		out = strings.ReplaceAll(out, "{{."+k+"}}", toStr(v))
	}
	return out, nil
}
func (interpolatingTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}

func toStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_notification_message_task_failed", nil)
	if got != "internal_notification_message_task_failed" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_notification_message_system_error", nil)
	if got != "<TR:internal_notification_message_system_error>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (the user would see a blank notification). The
	// implementation MUST degrade to the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_notification_message_workflow_failed", nil)
	if got != "internal_notification_message_workflow_failed" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_notification_message_system_shutdown", nil)
	if got != "internal_notification_message_system_shutdown" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(notificationi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_notification_message_task_completed", nil)
	if got != "internal_notification_message_task_completed" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestEventHandler_TaskCompleted_InterpolatesPlaceholders proves the
// round-385 migration is not a §11.4 PASS-bluff: with a real
// interpolating translator wired, the notification Message MUST carry
// the runtime task ID + duration through to the end user.
func TestEventHandler_TaskCompleted_InterpolatesPlaceholders(t *testing.T) {
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

	h := NewEventNotificationHandler(NewNotificationEngine())
	evt := event.Event{
		Type:     event.EventTaskCompleted,
		Severity: event.SeverityInfo,
		Data:     map[string]interface{}{"task_id": "task-7788", "duration": "3m12s"},
		TaskID:   "task-7788",
	}
	notif := h.eventToNotification(stdctx.Background(), evt)
	if notif == nil {
		t.Fatal("eventToNotification returned nil for EventTaskCompleted")
	}
	want := "Task task-7788 completed successfully in 3m12s"
	if notif.Message != want {
		t.Fatalf("task-completed Message = %q, want %q — interpolation bypassed", notif.Message, want)
	}
}

// TestEventHandler_TaskFailed_InterpolatesPlaceholders proves the
// error-bearing failure notification renders both the task ID and the
// real error text for the end user.
func TestEventHandler_TaskFailed_InterpolatesPlaceholders(t *testing.T) {
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

	h := NewEventNotificationHandler(NewNotificationEngine())
	evt := event.Event{
		Type:     event.EventTaskFailed,
		Severity: event.SeverityError,
		Data:     map[string]interface{}{"task_id": "task-9001", "error": "disk full"},
		TaskID:   "task-9001",
	}
	notif := h.eventToNotification(stdctx.Background(), evt)
	if notif == nil {
		t.Fatal("eventToNotification returned nil for EventTaskFailed")
	}
	want := "Task task-9001 failed: disk full"
	if notif.Message != want {
		t.Fatalf("task-failed Message = %q, want %q — interpolation bypassed", notif.Message, want)
	}
}

// TestEventHandler_TaskFailed_DefaultErrorTranslated proves the
// fallback error value ("Unknown error") is itself a translated
// message ID, not a hardcoded English literal — round-385 migrated the
// getDataString default arguments too.
func TestEventHandler_TaskFailed_DefaultErrorTranslated(t *testing.T) {
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

	h := NewEventNotificationHandler(NewNotificationEngine())
	evt := event.Event{
		Type:     event.EventTaskFailed,
		Severity: event.SeverityError,
		Data:     map[string]interface{}{"task_id": "task-9002"}, // no "error" key
		TaskID:   "task-9002",
	}
	notif := h.eventToNotification(stdctx.Background(), evt)
	if notif == nil {
		t.Fatal("eventToNotification returned nil for EventTaskFailed")
	}
	want := "Task task-9002 failed: Unknown error"
	if notif.Message != want {
		t.Fatalf("task-failed default-error Message = %q, want %q", notif.Message, want)
	}
}

// TestEventHandler_WorkerDisconnected_InterpolatesPlaceholders proves
// the worker notification renders id, host and reason.
func TestEventHandler_WorkerDisconnected_InterpolatesPlaceholders(t *testing.T) {
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

	h := NewEventNotificationHandler(NewNotificationEngine())
	evt := event.Event{
		Type:     event.EventWorkerDisconnected,
		Severity: event.SeverityWarning,
		Data: map[string]interface{}{
			"worker_id": "w-42", "host": "node-3.local", "reason": "timeout",
		},
		WorkerID: "w-42",
	}
	notif := h.eventToNotification(stdctx.Background(), evt)
	if notif == nil {
		t.Fatal("eventToNotification returned nil for EventWorkerDisconnected")
	}
	want := "Worker w-42 (node-3.local) disconnected: timeout"
	if notif.Message != want {
		t.Fatalf("worker-disconnected Message = %q, want %q — interpolation bypassed", notif.Message, want)
	}
}

// TestEventHandler_SystemShutdown_InterpolatesReason proves the
// system-shutdown notification renders the shutdown reason.
func TestEventHandler_SystemShutdown_InterpolatesReason(t *testing.T) {
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

	h := NewEventNotificationHandler(NewNotificationEngine())
	evt := event.Event{
		Type:     event.EventSystemShutdown,
		Severity: event.SeverityWarning,
		Data:     map[string]interface{}{"reason": "operator request"},
	}
	notif := h.eventToNotification(stdctx.Background(), evt)
	if notif == nil {
		t.Fatal("eventToNotification returned nil for EventSystemShutdown")
	}
	want := "HelixCode system shutting down: operator request"
	if notif.Message != want {
		t.Fatalf("system-shutdown Message = %q, want %q — interpolation bypassed", notif.Message, want)
	}
}
