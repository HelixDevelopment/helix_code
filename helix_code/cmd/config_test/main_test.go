// Sentinel call-site tests for the round-141 CONST-046 migration of
// helix_code/cmd/config_test/main.go. Mocks ALLOWED per CONST-050(A)
// (unit tests only).
//
// Each test:
//   1. Wires a fakeTranslator that wraps every message ID as
//      "<TRANSLATED:%s>".
//   2. Invokes the relevant entry point (the tr() helper, or
//      printConfigInfo() for the formatter path).
//   3. Asserts the captured output contains the sentinel-wrapped IDs
//      we expect — proving the call site actually went through the
//      translator instead of a hardcoded literal that happens to
//      match the bundle value (§11.4 anti-bluff).
//
// Round-141 also asserts SetTranslator(nil) resets to NoopTranslator
// (loud echo), guarding against the §11.4 i18n-layer PASS-bluff of
// silently disabling translation.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"dev.helix.code/cmd/config_test/i18n"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
)

// minimalConfig builds the smallest *config.Config that exercises
// every printConfigInfo formatter branch (Server / Database / Redis /
// Auth / LLM). All values are obvious sentinels so a regression in
// the call-site wiring would change the stdout fingerprint.
func minimalConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Address: "127.0.0.1", Port: 8080},
		Database: database.Config{
			Host:   "db.test",
			Port:   5432,
			DBName: "helix_test",
		},
		Redis: config.RedisConfig{
			Host:    "redis.test",
			Port:    6379,
			Enabled: true,
		},
		Auth: config.AuthConfig{JWTSecret: "secret-of-length-22--"},
		LLM: config.LLMConfig{
			DefaultProvider: "ollama",
			MaxTokens:       4096,
			Temperature:     0.7,
		},
	}
}

// fakeTranslator wraps every message ID so call sites can be detected
// by sentinel substring search rather than relying on the bundle's
// English value (which would render the test indistinguishable from a
// hardcoded literal that happens to match — a §11.4 PASS-bluff).
type fakeTranslator struct {
	seen     map[string]int
	failOnID string
}

func newFake() *fakeTranslator {
	return &fakeTranslator{seen: make(map[string]int)}
}

func (f *fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	f.seen[id]++
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

func (f *fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	f.seen[id]++
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

// captureStdout redirects os.Stdout, runs fn, then returns the
// captured bytes as a string. Used to exercise the printConfigInfo
// call site which writes through fmt.Println.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	return buf.String()
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	SetTranslator(nil) // explicit reset
	got := tr(context.Background(), "config_test_header_title", nil)
	if got != "config_test_header_title" {
		t.Fatalf("tr() with NoopTranslator returned %q, want loud echo of message ID", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	// Wire fake then explicitly reset to nil. Guard against §11.4 PASS-
	// bluff where SetTranslator(nil) might silently disable translation
	// (returning empty strings) instead of resetting to loud echo.
	SetTranslator(newFake())
	SetTranslator(nil)
	got := tr(context.Background(), "config_test_press_ctrl_c", nil)
	if got != "config_test_press_ctrl_c" {
		t.Fatalf("after SetTranslator(nil) reset, tr() returned %q, want loud echo of message ID", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	fake := newFake()
	SetTranslator(fake)
	defer SetTranslator(nil)

	got := tr(context.Background(), "config_test_initial_loaded", nil)
	want := "<TRANSLATED:config_test_initial_loaded>"
	if got != want {
		t.Fatalf("tr() returned %q, want %q", got, want)
	}
	if fake.seen["config_test_initial_loaded"] != 1 {
		t.Fatalf("fakeTranslator.T called %d times for config_test_initial_loaded, want 1", fake.seen["config_test_initial_loaded"])
	}
}

func TestTr_TranslatorErrorFallsBackToID(t *testing.T) {
	// §11.4 anti-bluff: a translator error MUST NOT swallow output
	// (silent empty line) — it MUST degrade to the message ID so the
	// production stream stays loud + obvious.
	fake := newFake()
	fake.failOnID = "config_test_shutting_down"
	SetTranslator(fake)
	defer SetTranslator(nil)

	got := tr(context.Background(), "config_test_shutting_down", nil)
	if got != "config_test_shutting_down" {
		t.Fatalf("tr() with translator error returned %q, want loud fallback to message ID", got)
	}
}

func TestTr_SatisfiesTranslatorInterface(t *testing.T) {
	// Compile-time guard: the fakeTranslator we use throughout these
	// tests MUST satisfy the same i18n.Translator interface used by
	// the production wire-in path. A drift in either direction would
	// silently break the call-site assertions.
	var _ i18n.Translator = newFake()
	var _ i18n.Translator = i18n.NoopTranslator{}
}

// TestPrintConfigInfo_AllCallSitesUseTranslator captures stdout +
// asserts every formatter message ID appears sentinel-wrapped in the
// output stream. This is the sentinel call-site proof for the
// printConfigInfo() formatter path migrated in round 141.
func TestPrintConfigInfo_AllCallSitesUseTranslator(t *testing.T) {
	fake := newFake()
	SetTranslator(fake)
	defer SetTranslator(nil)

	// Build a minimal in-memory Config sufficient to drive every
	// printConfigInfo formatter branch. We deliberately do NOT call
	// config.Load() because that would require a real config file /
	// env on the test host — unit tests stay hermetic per CONST-050.
	cfg := minimalConfig()

	out := captureStdout(t, func() {
		printConfigInfo(context.Background(), cfg)
	})

	wantIDs := []string{
		"config_test_info_server",
		"config_test_info_database",
		"config_test_info_redis",
		"config_test_info_auth",
		"config_test_info_llm",
	}
	for _, id := range wantIDs {
		sentinel := "<TRANSLATED:" + id + ">"
		if !strings.Contains(out, sentinel) {
			t.Errorf("printConfigInfo output missing sentinel for %s\noutput:\n%s", id, out)
		}
		if fake.seen[id] != 1 {
			t.Errorf("printConfigInfo invoked Translator.T %d times for %s, want 1", fake.seen[id], id)
		}
	}
}
