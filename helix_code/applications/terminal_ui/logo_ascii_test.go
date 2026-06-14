package main

import (
	"os"
	"strings"
	"testing"
)

// TestEmbeddedLogoASCII_ColoredAndFits is the anti-bluff guard for the dashboard
// logo: it must be the COLORED (tview-tagged) nautilus art derived from
// assets/Logo.png, fitting the dashboard's bordered 15-row header (≤13 content
// rows), with the green→teal gradient present — not the old clipped monochrome
// pyramid. Mirrors the runtime override file with the embedded package copy.
func TestEmbeddedLogoASCII_ColoredAndFits(t *testing.T) {
	if strings.TrimSpace(embeddedLogoASCII) == "" {
		t.Fatal("embeddedLogoASCII is empty — the //go:embed of logo-ascii.txt failed")
	}
	lines := strings.Split(strings.TrimRight(embeddedLogoASCII, "\n"), "\n")
	if len(lines) > 13 {
		t.Fatalf("logo art is %d lines; must be <=13 to fit the bordered 15-row dashboard header (was clipping)", len(lines))
	}
	// Must carry tview color tags (colored, not monochrome).
	if !strings.Contains(embeddedLogoASCII, "[#") {
		t.Fatal("logo art has no [#rrggbb] colour tags — it must be COLOURED to match assets/Logo.png")
	}
	// Gradient sanity: greens (low blue) on one side, teals (high blue) somewhere —
	// i.e. a real spread of colours, not a single flat colour.
	if strings.Count(embeddedLogoASCII, "[#") < 10 {
		t.Fatalf("expected many distinct colour tags for the gradient; got too few")
	}
	// The runtime override file (read by loadASCIIArt) must match the embed copy.
	if data, err := os.ReadFile("logo-ascii.txt"); err == nil {
		if string(data) != embeddedLogoASCII {
			t.Fatal("package logo-ascii.txt (embed source) and embeddedLogoASCII differ — regenerate via scripts/gen_logo_ascii.go")
		}
	}
}
