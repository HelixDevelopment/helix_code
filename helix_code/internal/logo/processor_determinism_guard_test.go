package logo

// processor_determinism_guard_test.go — standing regression guard (§11.4.135)
// for the ExtractColors non-determinism defect found by the §11.4.118
// discovery sweep (2026-06-16).
//
// DEFECT (reproduced): ExtractColors built its dominant-color slice by
// iterating a Go map (`colorCounts`), whose iteration order is randomized per
// run. The slice order determines Primary/Secondary/Accent, so the SAME logo
// produced a DIFFERENT brand palette on each call — non-deterministic output
// that then propagated into the generated icons, CSS theme, and Go theme
// constants.
//
// FIX: collect (color,count) pairs and sort by descending count with the hex
// value as a stable tiebreaker before assigning the scheme.
//
// §11.4.115 polarity switch via RED_MODE:
//   RED_MODE=1 : reproduce the defect on a FAITHFUL pre-fix stand-in (raw map
//                iteration, exactly the removed logic) and PASS when the
//                non-determinism is observed — proves the guard is real.
//   RED_MODE=0 (default / no env) : drive the REAL fixed ExtractColors and
//                assert the extracted scheme is identical across many runs.

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// fourBandPNG writes a PNG with four equally-sized horizontal bands of four
// distinct fully-opaque colors, each band far exceeding the count>50 dominance
// threshold so all four qualify — maximizing the slice-order ambiguity the
// defect depended on.
func fourBandPNG(t *testing.T, path string) {
	t.Helper()
	const w, h = 300, 300
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	cols := []color.RGBA{
		{255, 0, 0, 255},   // red
		{0, 255, 0, 255},   // green
		{0, 0, 255, 255},   // blue
		{255, 255, 0, 255}, // yellow
	}
	band := h / len(cols)
	for i, c := range cols {
		for y := i * band; y < (i+1)*band; y++ {
			for x := 0; x < w; x++ {
				img.Set(x, y, c)
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

// preFixDominantOrder reproduces the EXACT removed logic: sample the image,
// threshold, and collect dominant colors by RAW MAP ITERATION (no sort). It
// returns the hex of whichever color would have become Primary.
func preFixDominantOrder(img image.Image) string {
	bounds := img.Bounds()
	colorCounts := make(map[color.Color]int)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 10 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 10 {
			colorCounts[img.At(x, y)]++
		}
	}
	var dominantColors []color.Color
	for c, count := range colorCounts {
		if count > 50 {
			dominantColors = append(dominantColors, c)
		}
	}
	if len(dominantColors) == 0 {
		return ""
	}
	return colorToHex(dominantColors[0]) // pre-fix: this was map-order-dependent
}

func TestExtractColorsDeterminismGuard(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "four_band.png")
	fourBandPNG(t, src)

	if os.Getenv("RED_MODE") == "1" {
		// RED: prove the pre-fix (raw-map-iteration) logic is non-deterministic.
		f, err := os.Open(src)
		if err != nil {
			t.Fatalf("open src: %v", err)
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			t.Fatalf("decode src: %v", err)
		}
		// Go spec §6.2 explicitly does NOT guarantee map iteration order and the
		// runtime randomizes it per-iteration (https://go.dev/ref/spec#For_range).
		// With 4 equally-counted colors the pre-fix dominantColors[0] is whichever
		// the runtime placed first; P(all redTrials calls pick the same one) is at
		// most (1/4)^(redTrials-1) ≈ (0.25)^499 ≈ 10^-300 — a cryptographically
		// negligible false-negative floor, so this reproduction is not blind.
		const redTrials = 500
		seen := map[string]struct{}{}
		for trial := 0; trial < redTrials; trial++ {
			seen[preFixDominantOrder(img)] = struct{}{}
		}
		if len(seen) <= 1 {
			t.Fatalf("RED_MODE: expected pre-fix map-iteration to be non-deterministic, "+
				"but Primary color was stable across %d trials (%d distinct) — "+
				"the reproduction is blind", redTrials, len(seen))
		}
		t.Logf("RED_MODE: pre-fix logic produced %d distinct Primary colors over %d trials (defect reproduced)", len(seen), redTrials)
		return
	}

	// GREEN (default): the REAL fixed ExtractColors must yield an identical
	// scheme on every call for the identical input.
	var first string
	const runs = 60
	for trial := 0; trial < runs; trial++ {
		lp := NewLogoProcessor(src, dir)
		if err := lp.ExtractColors(); err != nil {
			t.Fatalf("ExtractColors: %v", err)
		}
		got := lp.Colors.Primary + "/" + lp.Colors.Secondary + "/" + lp.Colors.Accent
		if trial == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("ExtractColors non-deterministic: run %d produced %q, run 0 produced %q",
				trial, got, first)
		}
	}
	// The most-dominant band (count-tie -> hex tiebreaker) must be a stable,
	// known value, not merely "stable at some random tuple".
	lp := NewLogoProcessor(src, dir)
	if err := lp.ExtractColors(); err != nil {
		t.Fatalf("ExtractColors: %v", err)
	}
	// All four bands tie on count; hex-ascending tiebreaker => #0000FF first.
	if lp.Colors.Primary != "#0000FF" {
		t.Fatalf("expected deterministic Primary #0000FF (lowest hex among tied dominants), got %q", lp.Colors.Primary)
	}
	t.Logf("GREEN: ExtractColors stable across %d runs => %s", runs, first)
}
