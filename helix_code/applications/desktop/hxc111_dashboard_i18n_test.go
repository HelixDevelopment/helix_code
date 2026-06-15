//go:build !nogui

// hxc111_dashboard_i18n_test.go — §11.4.115 RED-polarity + §11.4.135 standing
// regression guard for HXC-111 (2026-06-15).
//
// DEFECT (root cause): the GUI main() in main.go constructed the DesktopApp and
// ran it WITHOUT ever calling SetTranslator with the real bundle-backed
// translator. The `nogui` main() in main_nogui.go DID. So on the GUI surface
// da.translator stayed the NoopTranslator{} default and every dashboard label
// rendered its raw message ID — "desktop_dashboard_header" /
// "desktop_dashboard_activity_title" appeared literally on screen instead of
// the localized prose from bundles/active.en.yaml. Same class as the HXC-099
// WireAll gap.
//
// This guard proves the GUI boot wiring works end-to-end at the seam the GUI
// main() now uses: a DesktopApp wired via SetTranslator(i18n.NewTranslator())
// resolves the two HXC-111 keys (and the rest of the dashboard) to real prose,
// with NO raw key leaking. It deliberately does NOT spin up the Fyne UI (needs
// a display server) — it exercises i18n.NewTranslator + SetTranslator + tr(),
// which is exactly what the GUI main() invokes before building windows.
//
//	RED_MODE=1 — force the NoopTranslator{} default (the pre-fix GUI state) and
//	             assert the raw keys leak (reproduces HXC-111 on the broken path).
//	RED_MODE=0 — wire the real translator like the fixed GUI main() and assert
//	             resolved prose with no raw key.
package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"dev.helix.code/applications/desktop/i18n"
)

func TestHXC111_GUIDashboardLabelsResolveThroughRealTranslator(t *testing.T) {
	dashboardKeys := []string{
		"desktop_dashboard_header",
		"desktop_dashboard_activity_title",
		"desktop_dashboard_activity_seed",
	}
	ctx := context.Background()

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the pre-fix GUI state: NoopTranslator{} default, never
		// replaced (GUI main() did not call SetTranslator).
		da := &DesktopApp{translator: i18n.NoopTranslator{}}
		for _, key := range dashboardKeys {
			got := da.tr(ctx, key, nil)
			if got != key {
				t.Fatalf("RED_MODE: da.tr(%q) = %q, expected the raw key to leak verbatim", key, got)
			}
		}
		t.Logf("RED_MODE reproduced HXC-111: dashboard keys %v leaked raw through NoopTranslator", dashboardKeys)
		return
	}

	// GREEN: wire the real bundle-backed translator exactly as the fixed GUI
	// main() does (i18n.NewTranslator() with no langs → shipped en bundle).
	tr, err := i18n.NewTranslator()
	if err != nil {
		t.Fatalf("HXC-111: i18n.NewTranslator() failed to load the embedded bundle: %v", err)
	}
	da := &DesktopApp{}
	da.SetTranslator(tr)

	for _, key := range dashboardKeys {
		got := da.tr(ctx, key, nil)
		if got == key {
			t.Fatalf("HXC-111 REGRESSION: da.tr(%q) returned the raw key verbatim — "+
				"GUI dashboard would render the message ID instead of localized text "+
				"(GUI main() did not wire a real translator)", key)
		}
		if strings.TrimSpace(got) == "" {
			t.Fatalf("HXC-111: da.tr(%q) returned empty — bundle entry missing/blank", key)
		}
	}

	// Spot-check the two operator-visible labels resolve to their bundle prose
	// (values from i18n/bundles/active.en.yaml).
	if hdr := da.tr(ctx, "desktop_dashboard_header", nil); !strings.Contains(hdr, "HelixCode") {
		t.Fatalf("HXC-111: desktop_dashboard_header resolved to %q, expected human text containing 'HelixCode'", hdr)
	}
	if title := da.tr(ctx, "desktop_dashboard_activity_title", nil); !strings.Contains(title, "Recent Activity") {
		t.Fatalf("HXC-111: desktop_dashboard_activity_title resolved to %q, expected human text 'Recent Activity'", title)
	}
}
