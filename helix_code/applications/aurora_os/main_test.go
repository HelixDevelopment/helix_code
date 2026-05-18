//go:build !nogui

package main

import (
	"context"
	"errors"
	"testing"

	"dev.helix.code/applications/aurora_os/i18n"
	"github.com/stretchr/testify/assert"
)

// sentinelTranslator is a unit-test-only stub (CONST-050(A): mocks
// allowed in *_test.go) that wraps every message ID in a recognisable
// sentinel envelope. Call-site tests use this to PROVE the migrated
// expression actually goes through Translator.T — a bluff-resistant
// alternative to grepping for "i18n" in the source.
type sentinelTranslator struct {
	calls []string
}

func (s *sentinelTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	s.calls = append(s.calls, id)
	return "<SENTINEL:" + id + ">", nil
}

func (s *sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	s.calls = append(s.calls, id)
	return "<SENTINEL:" + id + ">", nil
}

// errTranslator simulates a Translator.T failure so we can assert
// auroraApp.t() falls back to the literal message ID (loud echo)
// rather than returning an empty string and silently breaking the
// UI — that would be a §11.4 PASS-bluff at the i18n error path.
type errTranslator struct{}

func (errTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "", errors.New("translator boom")
}
func (errTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("translator boom")
}

// minimalAuroraAppForT builds the smallest AuroraApp value needed to
// exercise auroraApp.t() without booting Fyne. The full NewAuroraApp
// constructor calls app.New() which requires a display environment;
// these tests target ONLY the i18n helper so they construct the
// struct directly with the NoopTranslator default.
func minimalAuroraAppForT() *AuroraApp {
	return &AuroraApp{translator: i18n.NoopTranslator{}}
}

func TestAuroraApp_NoopTranslator_DefaultIsLoudEcho(t *testing.T) {
	auroraApp := minimalAuroraAppForT()
	// Round-140 §11.4: the NoopTranslator default MUST loud-echo
	// the message ID so a missing SetTranslator call never produces
	// a silently-empty UI element.
	assert.Equal(t, "aurora_os_window_title", auroraApp.t("aurora_os_window_title"))
}

func TestAuroraApp_SetTranslator_InstallsCustom(t *testing.T) {
	auroraApp := minimalAuroraAppForT()
	tr := &sentinelTranslator{}
	auroraApp.SetTranslator(tr)
	got := auroraApp.t("aurora_os_tab_aurora_dashboard")
	assert.Equal(t, "<SENTINEL:aurora_os_tab_aurora_dashboard>", got,
		"auroraApp.t() must route through the injected Translator")
	assert.Equal(t, []string{"aurora_os_tab_aurora_dashboard"}, tr.calls,
		"sentinelTranslator must record the message ID it was asked to resolve")
}

func TestAuroraApp_SetTranslator_NilPreservesDefault(t *testing.T) {
	auroraApp := minimalAuroraAppForT()
	defaultTr := auroraApp.translator
	auroraApp.SetTranslator(nil)
	assert.Equal(t, defaultTr, auroraApp.translator,
		"SetTranslator(nil) must NOT destroy the NoopTranslator safety net")
	// And auroraApp.t() must still yield a loud echo of the ID.
	assert.Equal(t, "aurora_os_window_title", auroraApp.t("aurora_os_window_title"))
}

func TestAuroraApp_T_FallsBackToIDOnError(t *testing.T) {
	auroraApp := minimalAuroraAppForT()
	auroraApp.SetTranslator(errTranslator{})
	got := auroraApp.t("aurora_os_tab_security")
	assert.Equal(t, "aurora_os_tab_security", got,
		"auroraApp.t() must loud-echo the ID when Translator.T returns an error")
}

// Ensure the i18n package's NoopTranslator (the constructor default)
// echoes the ID — paired sanity check against the in-package test in
// applications/aurora_os/i18n/translator_test.go.
func TestAuroraApp_NoopTranslator_LoudEcho(t *testing.T) {
	tr := i18n.NoopTranslator{}
	got, err := tr.T(context.Background(), "aurora_os_tab_aurora_dashboard", nil)
	assert.NoError(t, err)
	assert.Equal(t, "aurora_os_tab_aurora_dashboard", got)
}

func TestAuroraAppCreation(t *testing.T) {
	// Test that we can create the basic structs without panicking
	// Note: Full GUI testing would require a display environment

	aurora := &AuroraIntegration{
		nativeServices: make(map[string]interface{}),
	}

	assert.NotNil(t, aurora)
	assert.NotNil(t, aurora.nativeServices)
}

func TestAuroraSystemMonitor(t *testing.T) {
	monitor := &AuroraSystemMonitor{
		networkStats: make(map[string]interface{}),
	}

	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.networkStats)
}

func TestAuroraSecurityManager(t *testing.T) {
	security := NewAuroraSecurityManager()

	assert.NotNil(t, security)
	assert.NotNil(t, security.accessControl)
	assert.NotNil(t, security.auditLog)
	assert.True(t, security.encryptionEnabled)
	assert.Equal(t, "AES-256-GCM", security.encryptionAlgo)
}

func TestAuroraSecurityManagerAuditLog(t *testing.T) {
	security := NewAuroraSecurityManager()

	// Add audit entry
	security.AddAuditEntry("test_action", "test_user", "test details", "info")

	entries := security.GetAuditLog()
	assert.Len(t, entries, 1)
	assert.Equal(t, "test_action", entries[0].Action)
	assert.Equal(t, "test_user", entries[0].User)
	assert.Equal(t, "test details", entries[0].Details)
	assert.Equal(t, "info", entries[0].Severity)
}

func TestAuroraTaskManager(t *testing.T) {
	tm := NewAuroraTaskManager(nil)

	assert.NotNil(t, tm)
	assert.Empty(t, tm.GetAllTasks())

	stats := tm.GetStats()
	assert.Equal(t, 0, stats.TotalTasks)
	assert.Equal(t, 0, stats.CompletedTasks)
	assert.Equal(t, 0, stats.RunningTasks)
	assert.Equal(t, 0, stats.PendingTasks)
}

func TestAuroraWorkerManager(t *testing.T) {
	wm := NewAuroraWorkerManager(nil)

	assert.NotNil(t, wm)
	assert.Empty(t, wm.GetWorkers())
}

func TestUIWorker(t *testing.T) {
	worker := &UIWorker{
		ID:      "test-worker",
		Host:    "192.168.1.100",
		Port:    "22",
		User:    "deploy",
		Status:  "active",
		Healthy: true,
	}

	assert.Equal(t, "test-worker", worker.ID)
	assert.Equal(t, "192.168.1.100", worker.Host)
	assert.True(t, worker.Healthy)
}

func TestUITask(t *testing.T) {
	task := &UITask{
		ID:          "test-task",
		Type:        "building",
		Description: "Test task",
		Status:      "pending",
		Priority:    "high",
	}

	assert.Equal(t, "test-task", task.ID)
	assert.Equal(t, "building", task.Type)
	assert.Equal(t, "pending", task.Status)
}

func TestThemeManager(t *testing.T) {
	tm := NewThemeManager()

	assert.NotNil(t, tm)
	assert.NotNil(t, tm.GetCurrentTheme())

	themes := tm.GetAvailableThemes()
	assert.NotEmpty(t, themes)
}

func TestThemeManagerSetTheme(t *testing.T) {
	tm := NewThemeManager()

	// Test setting different themes
	success := tm.SetTheme("dark")
	assert.True(t, success)
	assert.Equal(t, "Dark", tm.GetCurrentTheme().Name)

	success = tm.SetTheme("light")
	assert.True(t, success)
	assert.Equal(t, "Light", tm.GetCurrentTheme().Name)

	// Test invalid theme
	success = tm.SetTheme("nonexistent")
	assert.False(t, success)
}

func TestThemeManagerGetColor(t *testing.T) {
	tm := NewThemeManager()
	tm.SetTheme("aurora")

	primary := tm.GetColor("primary")
	assert.NotEmpty(t, primary)
	assert.True(t, primary[0] == '#')
}
