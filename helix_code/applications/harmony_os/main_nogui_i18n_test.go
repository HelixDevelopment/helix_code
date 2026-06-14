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

	"dev.helix.code/internal/hardware"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/project"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/task"
	"dev.helix.code/internal/worker"
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
	// Initialize wires managers needed by cmd* methods. In CI without
	// a populated config, config.Load() fails and Initialize returns
	// early leaving managers nil — which crashes cmd* call sites. To
	// keep the i18n seam tests deterministic without real
	// infrastructure (CONST-050(A): mocks/fakes permitted in unit
	// tests), wire the managers directly when Initialize fails. Every
	// manager here uses in-memory backends — no docker / network.
	if err := app.Initialize(); err != nil {
		t.Logf("Initialize non-fatal (no docker/config): %v — wiring in-memory managers", err)
	}
	if app.taskManager == nil {
		app.taskManager = NewCLITaskManager(task.NewTaskManager(nil, nil))
	}
	if app.workerManager == nil {
		wr := worker.NewInMemoryWorkerRepository()
		app.workerManager = NewCLIWorkerManager(worker.NewWorkerManager(wr, 30*time.Second))
	}
	if app.projectManager == nil {
		app.projectManager = project.NewManager()
	}
	if app.sessionManager == nil {
		app.sessionManager = session.NewManager()
	}
	if app.llmManager == nil {
		app.llmManager = llm.NewModelManager()
	}
	if app.hardwareDetector == nil {
		app.hardwareDetector = hardware.NewHardwareDetector()
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

// --- Round-330 §11.4 paired-mutation seam tests ---
// Each asserts a round-330-migrated call site consults the injected
// Translator (sentinel present) and no longer emits the original
// English literal (literal absent). Joint invariant = anti-bluff.

func TestPrintHelp_UsesTranslator_NotHardcodedBody(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { app.printHelp() })
	assertTranslated(t, out, "harmony_os_cli_help_body", "HelixCode Harmony OS CLI (nogui mode)")
}

func TestRun_UnknownCommand_UsesTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.Run([]string{"definitely-not-a-command"}) })
	want := "<TRANSLATED:harmony_os_cli_unknown_command>"
	if !strings.Contains(out, want) {
		t.Fatalf("unknown-command output missing translator sentinel %q.\nFull:\n%s", want, out)
	}
	if strings.Contains(out, "Unknown command: definitely-not-a-command") {
		t.Fatalf("output still contains original literal — migration reverted.\nFull:\n%s", out)
	}
}

func TestCmdStatus_StatusLines_UseTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdStatus() })
	// Status info lines (placeholder-bearing) MUST route through the
	// Translator — sentinel for the platform + LLM-models IDs present.
	for _, id := range []string{
		"harmony_os_cli_status_platform",
		"harmony_os_cli_status_workers",
		"harmony_os_cli_status_llm_models",
	} {
		if !strings.Contains(out, "<TRANSLATED:"+id+">") {
			t.Fatalf("cmdStatus output missing sentinel for %q.\nFull:\n%s", id, out)
		}
	}
}

func TestCmdProjects_UnknownSubcommand_UsesTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdProjects([]string{"bogus-subcmd"}) })
	want := "<TRANSLATED:harmony_os_cli_unknown_subcommand>"
	if !strings.Contains(out, want) {
		t.Fatalf("unknown-subcommand output missing translator sentinel %q.\nFull:\n%s", want, out)
	}
	if strings.Contains(out, "Unknown subcommand: bogus-subcmd") {
		t.Fatalf("output still contains original literal — migration reverted.\nFull:\n%s", out)
	}
}

func TestCmdProjects_CreateMissingArgs_UsesTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdProjects([]string{"create"}) })
	assertTranslated(t, out, "harmony_os_cli_err_name_path_required", "Error: --name and --path are required")
}

// --- Round-369 §11.4 paired-mutation seam tests ---
// cmdSystem report body, cmdLLM subcommand output, cmdDistributed
// default branch, and cmdInteractive banner. Each asserts the
// round-369-migrated call sites consult the injected Translator
// (sentinel present) AND no longer emit the original English literal.

func TestCmdSystem_ReportBody_UsesTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdSystem() })
	for _, id := range []string{
		"harmony_os_cli_system_hw_profile",
		"harmony_os_cli_system_os_info",
		"harmony_os_cli_system_capabilities",
		"harmony_os_cli_system_runtime_stats",
		"harmony_os_cli_system_cap_distributed",
	} {
		if !strings.Contains(out, "<TRANSLATED:"+id+">") {
			t.Fatalf("cmdSystem output missing sentinel for %q.\nFull:\n%s", id, out)
		}
	}
	for _, lit := range []string{"Hardware Profile:", "OS Information:", "Harmony OS Capabilities:", "Runtime Statistics:"} {
		if strings.Contains(out, lit) {
			t.Fatalf("cmdSystem output still contains original literal %q — migration reverted.\nFull:\n%s", lit, out)
		}
	}
}

func TestCmdLLM_Providers_UsesTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdLLM([]string{"providers"}) })
	assertTranslated(t, out, "harmony_os_cli_llm_providers_header", "=== LLM Providers ===")
}

func TestCmdLLM_Models_UsesTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdLLM([]string{"models"}) })
	assertTranslated(t, out, "harmony_os_cli_llm_models_header", "=== Available Models ===")
}

// TestCmdLLM_Chat_EmptyPrompt_UsesTranslator: `llm chat` with no prompt
// is a usage error — it MUST print the translated usage notice (via the
// injected Translator) and MUST NOT reach the real Generate path. This
// replaces the pre-wiring placeholder test that asserted the old
// "LLM chat requires a running provider." stub message.
func TestCmdLLM_Chat_EmptyPrompt_UsesTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	var out string
	var err error
	out = captureStdout(t, func() { err = app.cmdLLM([]string{"chat"}) })
	if err == nil {
		t.Fatalf("cmdLLM chat with empty prompt: expected a usage error, got nil")
	}
	if !strings.Contains(out, "<TRANSLATED:harmony_os_cli_llm_chat_usage>") {
		t.Fatalf("cmdLLM chat empty-prompt output missing usage translator sentinel.\nFull:\n%s", out)
	}
	// The pre-wiring placeholder hint MUST be gone (the chat case no
	// longer tells the user to "use the GUI version").
	if strings.Contains(out, "use the GUI version for interactive chat") {
		t.Fatalf("output still contains the old placeholder hint — real-generation wiring reverted.\nFull:\n%s", out)
	}
}

// TestCmdLLM_Chat_RoutesToRealGenerate_HonestError: `llm chat <prompt>`
// MUST route to the REAL HarmonyLLMCore.Generate path. In a unit-test
// environment with no cloud credentials and no local Ollama reachable,
// Generate surfaces a genuine "no LLM provider available" transport
// error verbatim — proving the placeholder was replaced by the real
// pipeline (a placeholder would have returned nil / printed a canned
// hint instead of attempting a real provider call). Anti-bluff: we
// assert the error is the honest provider/transport error, NOT a fake
// success and NOT the old stub message.
func TestCmdLLM_Chat_RoutesToRealGenerate_HonestError(t *testing.T) {
	app := newCLIAppForTest(t)
	// No cloud provider configured; buildHarmonyLLMProvider falls back to
	// a local Ollama provider on http://localhost:11434. In a unit-test
	// environment that port is not serving Ollama, so provider.Generate
	// fails with a real connection error — the honest no-provider path.
	t.Setenv("HELIX_LLM_PROVIDER", "")
	out := captureStdout(t, func() {
		err := app.cmdLLM([]string{"chat", "hello", "harmony"})
		if err == nil {
			t.Fatalf("cmdLLM chat <prompt> with no reachable provider: expected honest provider error, got nil (fake success)")
		}
		if !strings.Contains(err.Error(), "llm chat:") {
			t.Fatalf("cmdLLM chat error not wrapped by the chat call site: %v", err)
		}
	})
	// The "generating" status line MUST come from the translator (the
	// real flow was entered), and the old stub hint MUST be absent.
	if !strings.Contains(out, "<TRANSLATED:harmony_os_cli_llm_chat_generating>") {
		t.Fatalf("cmdLLM chat did not enter the real-generation path (missing generating sentinel).\nFull:\n%s", out)
	}
	if strings.Contains(out, "use the GUI version for interactive chat") {
		t.Fatalf("output still contains the old placeholder hint — real-generation wiring reverted.\nFull:\n%s", out)
	}
}

func TestCmdLLM_UnknownSubcommand_UsesTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdLLM([]string{"bogus-subcmd"}) })
	if !strings.Contains(out, "<TRANSLATED:harmony_os_cli_unknown_subcommand>") {
		t.Fatalf("cmdLLM unknown-subcommand output missing translator sentinel.\nFull:\n%s", out)
	}
	if strings.Contains(out, "Unknown subcommand: bogus-subcmd") {
		t.Fatalf("output still contains original literal — migration reverted.\nFull:\n%s", out)
	}
}

func TestCmdDistributed_UnknownSubcommand_UsesTranslator(t *testing.T) {
	app := newCLIAppForTest(t)
	out := captureStdout(t, func() { _ = app.cmdDistributed([]string{"bogus-subcmd"}) })
	for _, id := range []string{"harmony_os_cli_unknown_subcommand", "harmony_os_cli_distributed_subcommands"} {
		if !strings.Contains(out, "<TRANSLATED:"+id+">") {
			t.Fatalf("cmdDistributed output missing sentinel for %q.\nFull:\n%s", id, out)
		}
	}
	if strings.Contains(out, "Available subcommands: status, discover, sync") {
		t.Fatalf("output still contains original literal — migration reverted.\nFull:\n%s", out)
	}
}
