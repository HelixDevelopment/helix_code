package main

// Unit test for the pure context-window USED-% formatting helper
// (formatContextUsage) added alongside printGenerationStats in main.go.
//
// Anti-bluff (CONST-035): the helper is the honest-conditional that powers the
// `context: <used>/<window> (NN%)` indicator. The two load-bearing behaviours
// proven here are:
//
//   - omit-when-unknown: a non-positive window (the provider reported NO real
//     context-window for the active model) yields "" so the caller prints
//     nothing rather than a fabricated percentage against a guessed denominator;
//   - correct-percent: a positive window yields the exact integer percentage of
//     used/window with no rounding surprises and no negative used count.
//
// These assertions FAIL if the helper is regressed into hardcoding a window,
// capping/altering the percentage math, or emitting a string when the window
// is unknown.

import "testing"

func TestFormatContextUsage(t *testing.T) {
	tests := []struct {
		name   string
		used   int
		window int
		want   string
	}{
		{
			// Window unknown (0) -> indicator OMITTED, not fabricated.
			name: "omit_when_window_zero",
			used: 1234, window: 0,
			want: "",
		},
		{
			// Negative window is also "unknown" -> omitted.
			name: "omit_when_window_negative",
			used: 1234, window: -1,
			want: "",
		},
		{
			// Real window, exact 50%.
			name: "half_used",
			used: 4096, window: 8192,
			want: "   context: 4096/8192 (50%)",
		},
		{
			// Integer-truncated percentage: 1/3 -> 33%.
			name: "truncated_percent",
			used: 1, window: 3,
			want: "   context: 1/3 (33%)",
		},
		{
			// Large realistic window (Anthropic-class 200k), small usage -> 0%.
			name: "tiny_usage_large_window",
			used: 30, window: 200000,
			want: "   context: 30/200000 (0%)",
		},
		{
			// Zero used against a known window -> 0%, still shown (window known).
			name: "zero_used_known_window",
			used: 0, window: 8192,
			want: "   context: 0/8192 (0%)",
		},
		{
			// Used exceeding the window -> honest >100% rather than a cap.
			name: "over_one_hundred_percent",
			used: 9000, window: 8192,
			want: "   context: 9000/8192 (109%)",
		},
		{
			// Negative used is clamped to 0 (defensive); window known so shown.
			name: "negative_used_clamped",
			used: -5, window: 8192,
			want: "   context: 0/8192 (0%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatContextUsage(tt.used, tt.window)
			if got != tt.want {
				t.Fatalf("formatContextUsage(%d, %d) = %q; want %q",
					tt.used, tt.window, got, tt.want)
			}
		})
	}
}
