//go:build nogui

// CONST-046 (round-96 §11.4) call-site tests. Each injects a
// fakeTranslator that wraps every id in "<TRANSLATED:id>", captures
// stdout, and asserts the sentinel appears AND the original literal
// is absent. Anti-bluff: presence-of-sentinel + absence-of-literal
// jointly prove the translator was actually consulted. Mocks
// permitted per CONST-050(A) (unit tests only).
package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeTranslator struct{}

func (fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "<TRANSLATED:" + id + ">", nil
}

func (fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TRANSLATED:" + id + ">", nil
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	os.Stdout = orig
	select {
	case s := <-done:
		return s
	case <-time.After(5 * time.Second):
		t.Fatalf("captureStdout: timed out waiting for reader")
		return ""
	}
}

func newCLIAppForTest(t *testing.T) *HarmonyCLIApp {
	t.Helper()
	app := NewHarmonyCLIApp()
	app.SetTranslator(fakeTranslator{})
	// Initialize wires managers needed by cmd* methods. Backend
	// errors are non-fatal — section headers print BEFORE backend
	// access in every migrated call site.
	if err := app.Initialize(); err != nil {
		t.Logf("Initialize non-fatal (no docker): %v", err)
	}
	return app
}

// assertTranslated asserts the captured stdout contains the
// translator sentinel for msgID AND does NOT contain the original
// English literal — the joint anti-bluff invariant.
func assertTranslated(t *testing.T, out, msgID, originalLiteral string) {
	t.Helper()
	want := "<TRANSLATED:" + msgID + ">"
	if !strings.Contains(out, want) {
		t.Fatalf("output missing translator sentinel %q.\nFull:\n%s", want, out)
	}
	if strings.Contains(out, originalLiteral) {
		t.Fatalf("output still contains original literal %q — migration reverted or translator bypassed.\nFull:\n%s", originalLiteral, out)
	}
}

func TestCmdStatus_UsesTranslator_NotHardcodedHeader(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdStatus() })
	assertTranslated(t, out, "harmony_os_cli_status_header", "=== HelixCode Harmony OS Status ===")
}

func TestCmdSystem_UsesTranslator_NotHardcodedHeader(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdSystem() })
	assertTranslated(t, out, "harmony_os_cli_system_header", "=== Harmony OS System Information ===")
}

func TestCmdProjects_UsesTranslator_NotHardcodedHeader(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdProjects([]string{"list"}) })
	assertTranslated(t, out, "harmony_os_cli_projects_header", "=== Projects ===")
}

func TestCmdSessions_UsesTranslator_NotHardcodedHeader(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdSessions([]string{"list"}) })
	assertTranslated(t, out, "harmony_os_cli_sessions_header", "=== Sessions ===")
}

func TestCmdTasks_UsesTranslator_NotHardcodedHeader(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdTasks([]string{"list"}) })
	assertTranslated(t, out, "harmony_os_cli_tasks_header", "=== Tasks ===")
}

// TestSetTranslator_NilResetsToNoop: nil to SetTranslator MUST reset
// to NoopTranslator (loud echo) — silent translation failure would
// reintroduce the §11.4 PASS-bluff anti-pattern at the injection
// layer.
func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	app := NewHarmonyCLIApp()
	app.SetTranslator(fakeTranslator{})
	app.SetTranslator(nil)
	got := app.tr(context.Background(), "harmony_os_cli_status_header", nil)
	if got != "harmony_os_cli_status_header" {
		t.Fatalf("after SetTranslator(nil), tr returned %q, want loud message-ID echo", got)
	}
}
