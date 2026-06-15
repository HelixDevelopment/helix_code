package hardware

// Regression guards (§11.4.115 RED→GREEN, §11.4.135) for two reproduced
// defects in detector.go:
//
//   DEFECT-2 (CanRunModel map-miss): sizeOrder[modelSize] returns 0 on an
//     unknown/typo'd size, so requestedOrder(0) <= optimalOrder is always
//     true and an unknown size is wrongly reported runnable.
//
//   DEFECT-3 (parseMemorySize sign/notation): the regex
//     `(\d+(?:\.\d+)?)\s*(GB|MB|TB|G|M|T)?` drops a leading '-' ("-5GB"→5)
//     and truncates scientific notation ("1e308TB"→1), so a negative or
//     malformed size silently becomes positive memory.
//
// Each test carries a RED_MODE polarity switch (default "1" = reproduce the
// defect on the pre-fix artifact). Run with RED_MODE=1 against the broken
// detector.go to SEE the defect; the default (post-fix) RED_MODE=0 asserts the
// defect is ABSENT — the standing regression guard.

import (
	"os"
	"testing"
)

// redMode reports whether the test should reproduce the historical defect
// (RED_MODE=1) rather than assert the fixed behaviour (RED_MODE=0, default).
func redMode() bool {
	return os.Getenv("RED_MODE") == "1"
}

// newLowEndDetector returns a Detector whose memory profile yields the smallest
// optimal model size ("3B"), so any *correctly* unknown size must be reported
// NOT runnable. With empty VRAM/RAM, GetOptimalModelSize() falls through to the
// "3B" default branch.
func newLowEndDetector() *Detector {
	return &Detector{
		info: &HardwareInfo{
			GPU:    GPUInfo{VRAM: ""},
			Memory: MemoryInfo{TotalRAM: ""},
		},
	}
}

// TestCanRunModel_UnknownSizeNotRunnable — DEFECT-2.
//
//   - RED_MODE=1 (broken artifact): a garbage model size is wrongly reported
//     runnable (true) because the map miss yields order 0; this test asserts
//     that broken behaviour, proving the defect is real.
//   - RED_MODE=0 (fixed artifact, default): an unknown size MUST be reported
//     NOT runnable (false) — the standing guard.
func TestCanRunModel_UnknownSizeNotRunnable(t *testing.T) {
	d := newLowEndDetector()

	for _, garbage := range []string{"garbage", "8b", "999B", "", "13", "B13"} {
		got := d.CanRunModel(garbage)
		if redMode() {
			// On the broken artifact the map miss makes order 0 <= optimalOrder,
			// so every unknown size is (wrongly) runnable.
			if !got {
				t.Fatalf("RED_MODE: expected broken CanRunModel(%q)==true on pre-fix artifact, got false", garbage)
			}
		} else {
			if got {
				t.Fatalf("CanRunModel(%q) = true; unknown/unrecognised model size must be reported NOT runnable", garbage)
			}
		}
	}

	// A known, supported size MUST stay runnable in BOTH modes — the fix must
	// not break the legitimate path.
	if !d.CanRunModel("3B") {
		t.Fatalf("CanRunModel(%q) = false; a known size at/below the optimal must remain runnable", "3B")
	}
	// A known size ABOVE the low-end optimum must be NOT runnable in both modes.
	if d.CanRunModel("70B") {
		t.Fatalf("CanRunModel(%q) = true on a low-end detector; a too-large known size must be NOT runnable", "70B")
	}
}

// TestParseMemorySize_NegativeAndMalformed — DEFECT-3.
//
//   - RED_MODE=1 (broken artifact): "-5GB" parses to a positive 5 and
//     "1e308TB" truncates to 1; this test asserts that broken behaviour.
//   - RED_MODE=0 (fixed artifact, default): a negative or malformed size MUST
//     parse to 0 — never a positive memory figure.
func TestParseMemorySize_NegativeAndMalformed(t *testing.T) {
	d := &Detector{info: &HardwareInfo{}}

	if redMode() {
		// Pre-fix behaviour captured on the broken artifact.
		if got := d.parseMemorySize("-5GB"); got != 5 {
			t.Fatalf("RED_MODE: expected broken parseMemorySize(\"-5GB\")==5 on pre-fix artifact, got %d", got)
		}
		if got := d.parseMemorySize("1e308TB"); got != 1 {
			t.Fatalf("RED_MODE: expected broken parseMemorySize(\"1e308TB\")==1 on pre-fix artifact, got %d", got)
		}
		return
	}

	// Fixed behaviour: negatives and malformed strings yield 0, never positive.
	negativeOrMalformed := []string{"-5GB", "-1TB", "-0.5GB", "1e308TB", "1e10GB", "NaNGB", "InfGB", "--3GB"}
	for _, in := range negativeOrMalformed {
		if got := d.parseMemorySize(in); got != 0 {
			t.Fatalf("parseMemorySize(%q) = %d; a negative/malformed size must parse to 0 (never positive memory)", in, got)
		}
	}

	// The fix must NOT regress valid inputs.
	valid := map[string]int{
		"16GB":   16,
		"8192MB": 8,
		"1TB":    1024,
		"4G":     4,
		"2.0GB":  2,
		"":       0,
	}
	for in, want := range valid {
		if got := d.parseMemorySize(in); got != want {
			t.Fatalf("parseMemorySize(%q) = %d; want %d (valid input must be unaffected by the fix)", in, got, want)
		}
	}
}
