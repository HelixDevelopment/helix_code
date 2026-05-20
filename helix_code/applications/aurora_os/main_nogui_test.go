//go:build nogui

package main

import (
	"context"
	"errors"
	"testing"

	"dev.helix.code/applications/aurora_os/i18n"
	"github.com/stretchr/testify/assert"
)

func TestCLIAppCreation(t *testing.T) {
	app := NewCLIApp()

	assert.NotNil(t, app)
	assert.NotNil(t, app.securityManager)
	assert.NotNil(t, app.diagnosticsLog)
}

// fakeTranslator is a unit-test-only translator (CONST-050(A): fakes
// permitted in *_test.go). calls captures the message IDs the CLI
// resolves so the paired-mutation tests below can assert the seam
// actually routes through Translator.T rather than echoing a literal.
type fakeTranslator struct {
	prefix string
	fail   bool
	calls  []string
}

func (f *fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	f.calls = append(f.calls, id)
	if f.fail {
		return "", errors.New("translate failed")
	}
	return f.prefix + id, nil
}

func (f *fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	f.calls = append(f.calls, id)
	if f.fail {
		return "", errors.New("translate failed")
	}
	return f.prefix + id, nil
}

// TestCLIAppTranslatorDefault verifies NewCLIApp installs a non-nil
// NoopTranslator (loud-echo safety net) per CONST-046 round-327.
func TestCLIAppTranslatorDefault(t *testing.T) {
	app := NewCLIApp()
	assert.NotNil(t, app.translator)
	assert.Equal(t, "aurora_os_cli_status_header", app.t("aurora_os_cli_status_header"))
}

// TestCLIAppSetTranslator is the positive case: a wired translator
// IS consulted and its output replaces the message ID.
func TestCLIAppSetTranslator(t *testing.T) {
	app := NewCLIApp()
	ft := &fakeTranslator{prefix: "XL:"}
	app.SetTranslator(ft)

	got := app.t("aurora_os_cli_version_banner")
	assert.Equal(t, "XL:aurora_os_cli_version_banner", got)
	assert.Equal(t, []string{"aurora_os_cli_version_banner"}, ft.calls)
}

// TestCLIAppSetTranslatorNilNoop is the paired-mutation guard:
// passing nil MUST NOT clear the NoopTranslator default — the
// loud-echo safety net must never disappear silently.
func TestCLIAppSetTranslatorNilNoop(t *testing.T) {
	app := NewCLIApp()
	ft := &fakeTranslator{prefix: "XL:"}
	app.SetTranslator(ft)
	app.SetTranslator(nil) // no-op — must NOT wipe ft

	got := app.t("aurora_os_cli_help_body")
	assert.Equal(t, "XL:aurora_os_cli_help_body", got,
		"SetTranslator(nil) must be a no-op, not a reset")
}

// TestCLIAppTranslatorFallbackOnError is the paired-mutation guard
// for the error path: when Translator.T returns an error the helper
// MUST fall back to the literal message ID (loud echo), never an
// empty string.
func TestCLIAppTranslatorFallbackOnError(t *testing.T) {
	app := NewCLIApp()
	app.SetTranslator(&fakeTranslator{fail: true})

	got := app.t("aurora_os_cli_no_projects")
	assert.Equal(t, "aurora_os_cli_no_projects", got,
		"on translate error the helper must echo the message ID")
}

// TestCLIAppTranslatorNoopType confirms i18n.NoopTranslator
// satisfies the i18n.Translator contract used by SetTranslator.
func TestCLIAppTranslatorNoopType(t *testing.T) {
	var _ i18n.Translator = i18n.NoopTranslator{}
	app := NewCLIApp()
	app.SetTranslator(i18n.NoopTranslator{})
	assert.Equal(t, "aurora_os_cli_workers_header", app.t("aurora_os_cli_workers_header"))
}

// TestCLIAppRound354IDsResolveThroughTranslator is the positive case
// for the round-354 §11.4 residual migration: every newly migrated
// message ID MUST route through Translator.T (not echo a literal).
func TestCLIAppRound354IDsResolveThroughTranslator(t *testing.T) {
	app := NewCLIApp()
	ft := &fakeTranslator{prefix: "R354:"}
	app.SetTranslator(ft)

	ids := []string{
		"aurora_os_cli_security_status_header",
		"aurora_os_cli_sessions_header",
		"aurora_os_cli_no_sessions",
		"aurora_os_cli_err_name_project_required",
		"aurora_os_cli_err_session_id_required",
		"aurora_os_cli_tasks_header",
		"aurora_os_cli_no_tasks",
		"aurora_os_cli_err_worker_id_required",
		"aurora_os_cli_llm_providers_header",
		"aurora_os_cli_no_providers",
		"aurora_os_cli_models_header",
		"aurora_os_cli_no_models",
		"aurora_os_cli_llm_chat_requires_provider",
		"aurora_os_cli_llm_chat_configure_hint",
		"aurora_os_cli_info_header",
		"aurora_os_cli_diagnostics_header",
		"aurora_os_cli_running_system_checks",
		"aurora_os_cli_optimization_header",
		"aurora_os_cli_running_gc",
		"aurora_os_cli_performance_mode_enabled",
		"aurora_os_cli_optimization_complete",
		"aurora_os_cli_access_control_roles_label",
		"aurora_os_cli_audit_log_header",
		"aurora_os_cli_no_audit_entries",
		"aurora_os_cli_encryption_enabled",
		"aurora_os_cli_encryption_disabled",
		"aurora_os_cli_unknown_encryption_command",
		"aurora_os_cli_exiting",
		"aurora_os_cli_goodbye",
	}
	for _, id := range ids {
		got := app.t(id)
		assert.Equal(t, "R354:"+id, got, "id %q must route through Translator.T", id)
	}
	assert.Equal(t, ids, ft.calls, "every round-354 id must be consulted in order")
}

// TestCLIAppRound354FallbackOnError is the paired-mutation guard for
// the round-354 IDs: on translate error the helper MUST echo the
// literal message ID (loud echo), never an empty string.
func TestCLIAppRound354FallbackOnError(t *testing.T) {
	app := NewCLIApp()
	app.SetTranslator(&fakeTranslator{fail: true})

	for _, id := range []string{
		"aurora_os_cli_tasks_header",
		"aurora_os_cli_no_audit_entries",
		"aurora_os_cli_goodbye",
	} {
		assert.Equal(t, id, app.t(id),
			"on translate error the helper must echo the message ID")
	}
}

// round361IDs is the closed set of message IDs migrated by the
// round-361 §11.4 residual sweep (status-report + create/start/pause/
// complete/cancel/add/remove confirmation + aurora optimize lines).
var round361IDs = []string{
	"aurora_os_cli_status_platform",
	"aurora_os_cli_status_performance_mode",
	"aurora_os_cli_status_workers",
	"aurora_os_cli_status_tasks",
	"aurora_os_cli_status_projects",
	"aurora_os_cli_status_sessions",
	"aurora_os_cli_status_llm_models",
	"aurora_os_cli_audit_log_entries",
	"aurora_os_cli_created_project",
	"aurora_os_cli_set_active_project",
	"aurora_os_cli_deleted_project",
	"aurora_os_cli_created_session",
	"aurora_os_cli_started_session",
	"aurora_os_cli_paused_session",
	"aurora_os_cli_completed_session",
	"aurora_os_cli_created_task",
	"aurora_os_cli_cancelled_task",
	"aurora_os_cli_added_worker",
	"aurora_os_cli_removed_worker",
	"aurora_os_cli_performance_mode_toggle",
	"aurora_os_cli_memory_freed",
	"aurora_os_cli_setting_gomaxprocs",
}

// TestCLIAppRound361IDsResolveThroughTranslator is the positive case
// for the round-361 §11.4 residual migration: every newly migrated
// format-string ID MUST route through Translator.T (not echo a
// literal). These IDs carry %-verb placeholders bound by fmt.Printf
// at the call site; this test asserts the seam, not the binding.
func TestCLIAppRound361IDsResolveThroughTranslator(t *testing.T) {
	app := NewCLIApp()
	ft := &fakeTranslator{prefix: "R361:"}
	app.SetTranslator(ft)

	for _, id := range round361IDs {
		got := app.t(id)
		assert.Equal(t, "R361:"+id, got, "id %q must route through Translator.T", id)
	}
	assert.Equal(t, round361IDs, ft.calls, "every round-361 id must be consulted in order")
}

// TestCLIAppRound361FallbackOnError is the paired-mutation guard for
// the round-361 IDs: on translate error the helper MUST echo the
// literal message ID (loud echo), never an empty string — otherwise
// fmt.Printf would receive an empty format string and silently drop
// the runtime values, a §11.4 PASS-bluff at the i18n layer.
func TestCLIAppRound361FallbackOnError(t *testing.T) {
	app := NewCLIApp()
	app.SetTranslator(&fakeTranslator{fail: true})

	for _, id := range round361IDs {
		assert.Equal(t, id, app.t(id),
			"on translate error the helper must echo the message ID for %q", id)
	}
}

func TestCLISecurityManager(t *testing.T) {
	sm := NewAuroraSecurityManager()

	assert.NotNil(t, sm)
	assert.True(t, sm.encryptionEnabled)
	assert.NotNil(t, sm.accessControl)
	assert.NotNil(t, sm.auditLog)

	// Test adding audit entry
	sm.AddAuditEntry("test_action", "test_user", "test details", "info")

	entries := sm.GetAuditLog()
	assert.Len(t, entries, 1)
	assert.Equal(t, "test_action", entries[0].Action)
	assert.Equal(t, "test_user", entries[0].User)
	assert.Equal(t, "test details", entries[0].Details)
	assert.Equal(t, "info", entries[0].Severity)
}

func TestCLITaskManager(t *testing.T) {
	tm := NewCLITaskManager(nil)

	assert.NotNil(t, tm)
	assert.Empty(t, tm.GetAllTasks())

	// Test stats
	total, completed, running := tm.GetStats()
	assert.Equal(t, 0, total)
	assert.Equal(t, 0, completed)
	assert.Equal(t, 0, running)

	// Test creating a task
	ctx := context.Background()
	task, err := tm.CreateTask(ctx, "building", "Test task", "high")
	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, "building", task.Type)
	assert.Equal(t, "Test task", task.Description)
	assert.Equal(t, "high", task.Priority)
	assert.Equal(t, "pending", task.Status)

	// Verify task was added
	total, _, _ = tm.GetStats()
	assert.Equal(t, 1, total)

	// Test cancel task
	err = tm.CancelTask(ctx, task.ID)
	assert.NoError(t, err)

	total, _, _ = tm.GetStats()
	assert.Equal(t, 0, total)

	// Test cancel non-existent task
	err = tm.CancelTask(ctx, "non-existent")
	assert.Error(t, err)
}

func TestCLIWorkerManager(t *testing.T) {
	wm := NewCLIWorkerManager(nil)

	assert.NotNil(t, wm)
	assert.Empty(t, wm.GetWorkers())

	// Test adding a worker
	worker := &CLIWorker{
		ID:      "test-worker",
		Host:    "192.168.1.100",
		Port:    "22",
		User:    "deploy",
		Status:  "pending",
		Healthy: false,
	}
	err := wm.AddWorker(worker)
	assert.NoError(t, err)

	workers := wm.GetWorkers()
	assert.Len(t, workers, 1)
	assert.Equal(t, "test-worker", workers[0].ID)

	// Test removing a worker
	err = wm.RemoveWorker("test-worker")
	assert.NoError(t, err)
	assert.Empty(t, wm.GetWorkers())

	// Test removing non-existent worker
	err = wm.RemoveWorker("non-existent")
	assert.Error(t, err)
}

func TestCLITask(t *testing.T) {
	task := CLITask{
		ID:          "test-id",
		Type:        "building",
		Description: "Test task",
		Status:      "pending",
		Priority:    "high",
	}

	assert.Equal(t, "test-id", task.ID)
	assert.Equal(t, "building", task.Type)
	assert.Equal(t, "Test task", task.Description)
	assert.Equal(t, "pending", task.Status)
	assert.Equal(t, "high", task.Priority)
}

func TestCLIWorker(t *testing.T) {
	worker := CLIWorker{
		ID:      "worker-1",
		Host:    "192.168.1.100",
		Port:    "22",
		User:    "deploy",
		Status:  "active",
		Healthy: true,
	}

	assert.Equal(t, "worker-1", worker.ID)
	assert.Equal(t, "192.168.1.100", worker.Host)
	assert.Equal(t, "22", worker.Port)
	assert.Equal(t, "deploy", worker.User)
	assert.Equal(t, "active", worker.Status)
	assert.True(t, worker.Healthy)
}

func TestAuditLogEntry(t *testing.T) {
	sm := NewAuroraSecurityManager()

	// Add multiple entries
	sm.AddAuditEntry("action1", "user1", "details1", "info")
	sm.AddAuditEntry("action2", "user2", "details2", "warning")
	sm.AddAuditEntry("action3", "user3", "details3", "error")

	entries := sm.GetAuditLog()
	assert.Len(t, entries, 3)

	assert.Equal(t, "action1", entries[0].Action)
	assert.Equal(t, "action2", entries[1].Action)
	assert.Equal(t, "action3", entries[2].Action)

	assert.Equal(t, "info", entries[0].Severity)
	assert.Equal(t, "warning", entries[1].Severity)
	assert.Equal(t, "error", entries[2].Severity)
}

func TestAccessControl(t *testing.T) {
	sm := NewAuroraSecurityManager()

	// Check default roles
	assert.Contains(t, sm.accessControl, "admin")
	assert.Contains(t, sm.accessControl, "developer")
	assert.Contains(t, sm.accessControl, "viewer")

	// Check admin permissions
	adminPerms := sm.accessControl["admin"]
	assert.Contains(t, adminPerms, "read")
	assert.Contains(t, adminPerms, "write")
	assert.Contains(t, adminPerms, "execute")
	assert.Contains(t, adminPerms, "admin")

	// Check developer permissions
	devPerms := sm.accessControl["developer"]
	assert.Contains(t, devPerms, "read")
	assert.Contains(t, devPerms, "write")
	assert.Contains(t, devPerms, "execute")
	assert.NotContains(t, devPerms, "admin")

	// Check viewer permissions
	viewerPerms := sm.accessControl["viewer"]
	assert.Contains(t, viewerPerms, "read")
	assert.NotContains(t, viewerPerms, "write")
}
