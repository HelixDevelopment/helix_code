//go:build nogui

package main

import (
	"context"
	"strings"
	"testing"

	i18n "dev.helix.code/applications/harmony_os/i18n"
)

// hxc102_interactive_i18n_test.go — HXC-102 regression guard (§11.4.135).
//
// The interactive REPL (cmdInteractive) used to print two user-facing strings
// with raw fmt — `fmt.Println("Goodbye!")` and `fmt.Printf("Error: %v\n", err)`
// — bypassing the translator (CONST-046 gap: identical English for every
// locale). They are now routed through cliApp.tr with the bundle keys
// harmony_os_cli_interactive_goodbye / _error_fmt. This guard proves, via the
// REAL embedded-bundle translator, that both keys resolve to prose (not the raw
// key) and that the error key BINDS its {{.Error}} param (no '<no value>' leak —
// the HXC-099 templating lesson).
func TestHXC102_InteractiveStringsResolveViaI18n(t *testing.T) {
	app := NewHarmonyCLIApp()
	tr, err := i18n.NewTranslator()
	if err != nil {
		t.Fatalf("real embedded-bundle translator must load: %v", err)
	}
	app.SetTranslator(tr)
	ctx := context.Background()

	gb := app.tr(ctx, "harmony_os_cli_interactive_goodbye", nil)
	if gb == "harmony_os_cli_interactive_goodbye" || strings.TrimSpace(gb) == "" {
		t.Fatalf("goodbye key did not resolve to prose (raw key leaked): %q", gb)
	}

	em := app.tr(ctx, "harmony_os_cli_interactive_error_fmt", map[string]any{"Error": "boom"})
	if em == "harmony_os_cli_interactive_error_fmt" {
		t.Fatalf("error key did not resolve to prose (raw key leaked): %q", em)
	}
	if strings.Contains(em, "<no value>") {
		t.Fatalf("error key leaked '<no value>' — {{.Error}} param not bound: %q", em)
	}
	if !strings.Contains(em, "boom") {
		t.Fatalf("error key did not bind the Error param: %q", em)
	}
}
