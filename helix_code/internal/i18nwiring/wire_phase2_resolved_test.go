// wire_phase2_resolved_test.go — HXC-036 Phase 2 acceptance test.
//
// Anti-bluff (§11.4 / §11.9): proves that newly-wired Phase-2 packages
// render REAL resolved + interpolated text through their boot-time
// NewTranslator constructor — NOT the NoopTranslator{} raw-message-ID
// echo that every package shipped before this wiring existed. Each
// sub-test loads the package's own embedded active.en.yaml via the
// constructor added in Phase 2, resolves a known message ID, and asserts
// the resolved English string (with template interpolation applied).
//
// A NoopTranslator{} would echo the message ID verbatim (e.g.
// "internal_auth_failed_hash_password"); these assertions therefore fail
// loudly if a package ever silently regresses to the no-op default.
package i18nwiring

import (
	"context"
	"strings"
	"testing"

	authi18n "dev.helix.code/internal/auth/i18n"
	configi18n "dev.helix.code/internal/config/i18n"
	llmi18n "dev.helix.code/internal/llm/i18n"
)

func TestPhase2_WireAll_Succeeds(t *testing.T) {
	if err := WireAll(); err != nil {
		t.Fatalf("WireAll() returned error (a package failed to build a real translator): %v", err)
	}
}

func TestPhase2_Auth_ResolvesInterpolatedText(t *testing.T) {
	tr, err := authi18n.NewTranslator()
	if err != nil {
		t.Fatalf("auth NewTranslator: %v", err)
	}
	got, err := tr.T(context.Background(), "internal_auth_failed_hash_password", map[string]any{"Err": "boom"})
	if err != nil {
		t.Fatalf("auth T: %v", err)
	}
	want := "failed to hash password: boom"
	if got != want {
		t.Fatalf("auth resolved text mismatch:\n got=%q\nwant=%q\n(raw-key echo would be the message ID — that is the bluff this guards against)", got, want)
	}
	if strings.Contains(got, "internal_auth_") {
		t.Fatalf("auth rendered a raw message-ID key (NoopTranslator regression): %q", got)
	}
	t.Logf("auth resolved: %q", got)
}

func TestPhase2_LLM_ResolvesInterpolatedText(t *testing.T) {
	tr, err := llmi18n.NewTranslator()
	if err != nil {
		t.Fatalf("llm NewTranslator: %v", err)
	}
	got, err := tr.T(context.Background(), "internal_llm_wizard_form_title", map[string]any{"Provider": "Anthropic"})
	if err != nil {
		t.Fatalf("llm T: %v", err)
	}
	want := " Configure Anthropic "
	if got != want {
		t.Fatalf("llm resolved text mismatch:\n got=%q\nwant=%q", got, want)
	}
	if strings.Contains(got, "internal_llm_") {
		t.Fatalf("llm rendered a raw message-ID key (NoopTranslator regression): %q", got)
	}
	t.Logf("llm resolved: %q", got)
}

func TestPhase2_Config_ResolvesInterpolatedText(t *testing.T) {
	tr, err := configi18n.NewTranslator()
	if err != nil {
		t.Fatalf("config NewTranslator: %v", err)
	}
	got, err := tr.T(context.Background(), "internal_config_info_using_config_file", map[string]any{"Path": "/etc/helix/config.yaml"})
	if err != nil {
		t.Fatalf("config T: %v", err)
	}
	want := "INFO: Using config file: /etc/helix/config.yaml"
	if got != want {
		t.Fatalf("config resolved text mismatch:\n got=%q\nwant=%q", got, want)
	}
	if strings.Contains(got, "internal_config_") {
		t.Fatalf("config rendered a raw message-ID key (NoopTranslator regression): %q", got)
	}
	t.Logf("config resolved: %q", got)
}
