// Unit tests for the internal/tools/askuser/i18n package — Translator
// contract + NoopTranslator behaviour + active.en.yaml bundle integrity
// (CONST-046 round-440 §11.4 anti-bluff sweep, 2026-05-20).
//
// Anti-bluff: the bundle test parses the SHIPPED active.en.yaml and asserts
// every message ID a stdin_prompter.go call site references is present —
// a missing key would silently degrade that prompt to a raw ID for every
// locale, an unusable CLI experience.
package i18n

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNoopTranslator_T_EchoesID(t *testing.T) {
	var n NoopTranslator
	out, err := n.T(context.Background(), "askuser_prompt_invalid_choice_hint", map[string]any{"Max": 3})
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if out != "askuser_prompt_invalid_choice_hint" {
		t.Fatalf("NoopTranslator.T = %q, want loud message-ID echo", out)
	}
}

func TestNoopTranslator_TPlural_EchoesID(t *testing.T) {
	var n NoopTranslator
	out, err := n.TPlural(context.Background(), "askuser_prompt_enter_choice_no_default", 2, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if out != "askuser_prompt_enter_choice_no_default" {
		t.Fatalf("NoopTranslator.TPlural = %q, want loud message-ID echo", out)
	}
}

func TestNoopTranslator_SatisfiesTranslatorInterface(t *testing.T) {
	var _ Translator = NoopTranslator{}
}

// TestActiveBundle_ContainsEveryCallSiteID parses the shipped en bundle and
// asserts every message ID referenced by stdin_prompter.go resolves to a
// non-empty "other" form.
func TestActiveBundle_ContainsEveryCallSiteID(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("bundles", "active.en.yaml"))
	if err != nil {
		t.Fatalf("read active.en.yaml: %v", err)
	}
	var bundle map[string]map[string]string
	if err := yaml.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("parse active.en.yaml: %v", err)
	}
	wantIDs := []string{
		"askuser_prompt_invalid_choice_hint",
		"askuser_prompt_choice_preview_label",
		"askuser_prompt_enter_choice_with_default",
		"askuser_prompt_enter_choice_no_default",
	}
	for _, id := range wantIDs {
		entry, ok := bundle[id]
		if !ok {
			t.Fatalf("active.en.yaml missing message ID %q", id)
		}
		if entry["other"] == "" {
			t.Fatalf("message ID %q has empty 'other' form", id)
		}
	}
}
