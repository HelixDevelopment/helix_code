package main

import (
	"context"
	"errors"
	"testing"

	"dev.helix.code/applications/terminal_ui/i18n"
	"github.com/stretchr/testify/assert"
)

func TestNewTerminalUI(t *testing.T) {
	tui := NewTerminalUI()
	assert.NotNil(t, tui)
	assert.NotNil(t, tui.app)
	// Round-137 §11.4: NewTerminalUI MUST install the
	// NoopTranslator default so tui.t() never panics on a nil
	// translator. Loud-echo safety net.
	assert.NotNil(t, tui.translator, "NewTerminalUI must install a non-nil Translator default")
}

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

func TestTerminalUI_SetTranslator_InstallsCustom(t *testing.T) {
	tui := NewTerminalUI()
	tr := &sentinelTranslator{}
	tui.SetTranslator(tr)
	got := tui.t("terminal_ui_sidebar_title")
	assert.Equal(t, "<SENTINEL:terminal_ui_sidebar_title>", got,
		"tui.t() must route through the injected Translator")
	assert.Equal(t, []string{"terminal_ui_sidebar_title"}, tr.calls,
		"sentinelTranslator must record the message ID it was asked to resolve")
}

func TestTerminalUI_SetTranslator_NilPreservesDefault(t *testing.T) {
	tui := NewTerminalUI()
	defaultTr := tui.translator
	tui.SetTranslator(nil)
	assert.Equal(t, defaultTr, tui.translator,
		"SetTranslator(nil) must NOT destroy the NoopTranslator safety net")
	// And tui.t() must still yield a loud echo of the ID.
	assert.Equal(t, "terminal_ui_sidebar_title", tui.t("terminal_ui_sidebar_title"))
}

// errTranslator simulates a Translator.T failure so we can assert
// tui.t() falls back to the literal message ID (loud echo) rather
// than returning an empty string and silently breaking the UI —
// that would be a §11.4 PASS-bluff at the i18n error path.
type errTranslator struct{}

func (errTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "", errors.New("translator boom")
}
func (errTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("translator boom")
}

func TestTerminalUI_T_FallsBackToIDOnError(t *testing.T) {
	tui := NewTerminalUI()
	tui.SetTranslator(errTranslator{})
	got := tui.t("terminal_ui_status_bar_default")
	assert.Equal(t, "terminal_ui_status_bar_default", got,
		"tui.t() must loud-echo the ID when Translator.T returns an error")
}

// Ensure the i18n package's NoopTranslator (the constructor default)
// echoes the ID — paired sanity check against the in-package test in
// applications/terminal_ui/i18n/translator_test.go.
func TestTerminalUI_NoopTranslator_LoudEcho(t *testing.T) {
	tr := i18n.NoopTranslator{}
	got, err := tr.T(context.Background(), "terminal_ui_sidebar_dashboard_desc", nil)
	assert.NoError(t, err)
	assert.Equal(t, "terminal_ui_sidebar_dashboard_desc", got)
}

func TestNewThemeManager(t *testing.T) {
	tm := NewThemeManager()
	assert.NotNil(t, tm)
	assert.NotNil(t, tm.currentTheme)
	assert.NotEmpty(t, tm.themes)
}

func TestThemeManager_GetAvailableThemes(t *testing.T) {
	tm := NewThemeManager()
	themes := tm.GetAvailableThemes()
	assert.Contains(t, themes, "dark")
	assert.Contains(t, themes, "light")
	assert.Contains(t, themes, "helix")
}

func TestThemeManager_SetTheme(t *testing.T) {
	tm := NewThemeManager()

	// Test valid theme
	assert.True(t, tm.SetTheme("light"))
	assert.Equal(t, "Light", tm.GetCurrentTheme().Name)

	// Test invalid theme
	assert.False(t, tm.SetTheme("invalid"))
}

func TestThemeManager_GetColor(t *testing.T) {
	tm := NewThemeManager()

	// Test valid color types
	assert.NotEmpty(t, tm.GetColor("primary"))
	assert.NotEmpty(t, tm.GetColor("text"))
	assert.NotEmpty(t, tm.GetColor("background"))

	// Test invalid color type (should return text color)
	assert.NotEmpty(t, tm.GetColor("invalid"))
}

func TestNewUIComponents(t *testing.T) {
	tui := NewTerminalUI()
	components := NewUIComponents(tui)
	assert.NotNil(t, components)
	assert.Equal(t, tui, components.tui)
}

func TestUIComponents_CreateForm(t *testing.T) {
	tui := NewTerminalUI()
	components := NewUIComponents(tui)

	fields := []FormField{
		{Type: "text", Label: "Name", DefaultValue: "test"},
	}

	form := components.CreateForm("Test Form", fields)
	assert.NotNil(t, form)
}

func TestUIComponents_CreateList(t *testing.T) {
	tui := NewTerminalUI()
	components := NewUIComponents(tui)

	items := []ListItem{
		{MainText: "Item 1", SecondaryText: "Description 1"},
		{MainText: "Item 2", SecondaryText: "Description 2"},
	}

	list := components.CreateList("Test List", items)
	assert.NotNil(t, list)
}

func TestUIComponents_CreateTable(t *testing.T) {
	tui := NewTerminalUI()
	components := NewUIComponents(tui)

	headers := []string{"Col1", "Col2"}
	data := [][]string{
		{"Row1Col1", "Row1Col2"},
		{"Row2Col1", "Row2Col2"},
	}

	table := components.CreateTable("Test Table", headers, data)
	assert.NotNil(t, table)
}

func TestUIComponents_CreateProgressBar(t *testing.T) {
	tui := NewTerminalUI()
	components := NewUIComponents(tui)

	progress := components.CreateProgressBar("Test Progress", 50, 100)
	assert.NotNil(t, progress)
}

func TestUIComponents_CreateModal(t *testing.T) {
	tui := NewTerminalUI()
	components := NewUIComponents(tui)

	buttons := []ModalButton{
		{Label: "OK", OnClick: func() {}},
		{Label: "Cancel", OnClick: func() {}},
	}

	modal := components.CreateModal("Test Modal", "Test message", buttons)
	assert.NotNil(t, modal)
}

func TestUIComponents_CreateStatusBar(t *testing.T) {
	tui := NewTerminalUI()
	components := NewUIComponents(tui)

	status := components.CreateStatusBar()
	assert.NotNil(t, status)
}

func TestUIComponents_CreateLogView(t *testing.T) {
	tui := NewTerminalUI()
	components := NewUIComponents(tui)

	logView := components.CreateLogView("Test Log")
	assert.NotNil(t, logView)
}
