package smartedit

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// TestMarkers_AreLiteralStrings asserts the SEARCH/REPLACE marker constants
// are byte-for-byte the strings the spec mandates. The hashes count matters:
// 7 left-angle / right-angle brackets and exactly 7 equals signs.
func TestMarkers_AreLiteralStrings(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"MarkerSearch", MarkerSearch, "<<<<<<< SEARCH"},
		{"MarkerDivider", MarkerDivider, "======="},
		{"MarkerReplace", MarkerReplace, ">>>>>>> REPLACE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Fatalf("%s = %q, want %q", c.name, c.got, c.want)
			}
			// byte-by-byte equality (catches whitespace / similar-glyph mishaps).
			if len(c.got) != len(c.want) {
				t.Fatalf("%s len=%d, want %d", c.name, len(c.got), len(c.want))
			}
			for i := 0; i < len(c.got); i++ {
				if c.got[i] != c.want[i] {
					t.Fatalf("%s byte %d = 0x%02x, want 0x%02x", c.name, i, c.got[i], c.want[i])
				}
			}
		})
	}
}

// TestMarkers_NotEqualToEachOther guards against future copy-paste errors
// where two of the three markers accidentally collapse to the same literal.
func TestMarkers_NotEqualToEachOther(t *testing.T) {
	if MarkerSearch == MarkerDivider {
		t.Fatal("MarkerSearch == MarkerDivider")
	}
	if MarkerSearch == MarkerReplace {
		t.Fatal("MarkerSearch == MarkerReplace")
	}
	if MarkerDivider == MarkerReplace {
		t.Fatal("MarkerDivider == MarkerReplace")
	}
}

// TestSizeLimits_AreReasonable table-checks the documented values.
func TestSizeLimits_AreReasonable(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"MaxPromptBytes", MaxPromptBytes, 1 << 20},
		{"MaxBlocksPerPrompt", MaxBlocksPerPrompt, 100},
		{"MaxFileBytes", MaxFileBytes, 10 << 20},
		{"MaxSearchBytes", MaxSearchBytes, 1 << 16},
		{"MaxReplaceBytes", MaxReplaceBytes, 1 << 16},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Fatalf("%s = %d, want %d", c.name, c.got, c.want)
			}
			if c.got <= 0 {
				t.Fatalf("%s = %d, must be positive", c.name, c.got)
			}
		})
	}
	// Sanity: per-section limits should not exceed the per-prompt cap.
	if MaxSearchBytes > MaxPromptBytes {
		t.Fatalf("MaxSearchBytes (%d) > MaxPromptBytes (%d)", MaxSearchBytes, MaxPromptBytes)
	}
	if MaxReplaceBytes > MaxPromptBytes {
		t.Fatalf("MaxReplaceBytes (%d) > MaxPromptBytes (%d)", MaxReplaceBytes, MaxPromptBytes)
	}
}

// TestEditBlock_ZeroValueIsEmpty asserts the zero value has empty fields.
func TestEditBlock_ZeroValueIsEmpty(t *testing.T) {
	var b EditBlock
	if b.Path != "" {
		t.Errorf("zero EditBlock.Path = %q, want empty", b.Path)
	}
	if b.Search != "" {
		t.Errorf("zero EditBlock.Search = %q, want empty", b.Search)
	}
	if b.Replace != "" {
		t.Errorf("zero EditBlock.Replace = %q, want empty", b.Replace)
	}
	if b.LineStart != 0 {
		t.Errorf("zero EditBlock.LineStart = %d, want 0", b.LineStart)
	}
	if b.LineEnd != 0 {
		t.Errorf("zero EditBlock.LineEnd = %d, want 0", b.LineEnd)
	}
}

// TestEditPlan_ZeroValueIsEmpty asserts the zero value has nil maps/slices.
func TestEditPlan_ZeroValueIsEmpty(t *testing.T) {
	var p EditPlan
	if len(p.Blocks) != 0 {
		t.Errorf("zero EditPlan.Blocks len = %d, want 0", len(p.Blocks))
	}
	if len(p.PerFile) != 0 {
		t.Errorf("zero EditPlan.PerFile len = %d, want 0", len(p.PerFile))
	}
	if p.SourceBytes != 0 {
		t.Errorf("zero EditPlan.SourceBytes = %d, want 0", p.SourceBytes)
	}
}

// TestEditOutcome_String_Values asserts each documented outcome enumerates
// to the exact string literal the spec requires.
func TestEditOutcome_String_Values(t *testing.T) {
	cases := []struct {
		name string
		got  EditOutcome
		want string
	}{
		{"OutcomeApplied", OutcomeApplied, "applied"},
		{"OutcomeNotFound", OutcomeNotFound, "not-found"},
		{"OutcomeAmbiguous", OutcomeAmbiguous, "ambiguous"},
		{"OutcomeBinary", OutcomeBinary, "binary"},
		{"OutcomeReadFailed", OutcomeReadFailed, "read-failed"},
		{"OutcomeWriteFailed", OutcomeWriteFailed, "write-failed"},
		{"OutcomeTooLarge", OutcomeTooLarge, "too-large"},
	}
	if len(cases) != 7 {
		t.Fatalf("expected 7 outcomes, got %d", len(cases))
	}
	seen := make(map[EditOutcome]string, len(cases))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if string(c.got) != c.want {
				t.Fatalf("%s = %q, want %q", c.name, string(c.got), c.want)
			}
		})
		if prior, dup := seen[c.got]; dup {
			t.Errorf("outcome %q used by both %s and %s", c.got, prior, c.name)
		}
		seen[c.got] = c.name
	}
}

// TestEditResult_JSONRoundTrip verifies the JSON tags survive marshal+unmarshal.
func TestEditResult_JSONRoundTrip(t *testing.T) {
	orig := EditResult{
		Block: EditBlock{
			Path:      "internal/foo/bar.go",
			Search:    "old text",
			Replace:   "new text",
			LineStart: 4,
			LineEnd:   8,
		},
		Outcome: OutcomeApplied,
		Error:   "",
		Diff:    "--- a/foo\n+++ b/foo\n@@ -1 +1 @@\n-old text\n+new text\n",
	}

	data, err := json.Marshal(&orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got EditResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Block.Path != orig.Block.Path {
		t.Errorf("Block.Path: got %q, want %q", got.Block.Path, orig.Block.Path)
	}
	if got.Block.Search != orig.Block.Search {
		t.Errorf("Block.Search: got %q, want %q", got.Block.Search, orig.Block.Search)
	}
	if got.Block.Replace != orig.Block.Replace {
		t.Errorf("Block.Replace: got %q, want %q", got.Block.Replace, orig.Block.Replace)
	}
	if got.Block.LineStart != orig.Block.LineStart {
		t.Errorf("Block.LineStart: got %d, want %d", got.Block.LineStart, orig.Block.LineStart)
	}
	if got.Outcome != orig.Outcome {
		t.Errorf("Outcome: got %q, want %q", got.Outcome, orig.Outcome)
	}
	if got.Diff != orig.Diff {
		t.Errorf("Diff mismatch: got %q, want %q", got.Diff, orig.Diff)
	}

	// Empty Error+Diff should be omitted from the JSON output.
	empty := EditResult{Block: orig.Block, Outcome: OutcomeNotFound}
	emptyData, err := json.Marshal(&empty)
	if err != nil {
		t.Fatalf("Marshal empty: %v", err)
	}
	if got, want := string(emptyData), `"error"`; containsSubstr(got, want) {
		t.Errorf("expected empty Error to be omitted, got %s", got)
	}
	if got, want := string(emptyData), `"diff"`; containsSubstr(got, want) {
		t.Errorf("expected empty Diff to be omitted, got %s", got)
	}
}

// TestSmartEditResult_JSONRoundTrip verifies the aggregate result marshals
// and unmarshals losslessly with stable JSON tags.
func TestSmartEditResult_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	orig := SmartEditResult{
		Results: []EditResult{
			{
				Block:   EditBlock{Path: "a.go", Search: "x", Replace: "y", LineStart: 1, LineEnd: 5},
				Outcome: OutcomeApplied,
				Diff:    "diff-text-a",
			},
			{
				Block:   EditBlock{Path: "b.go", Search: "p", Replace: "q", LineStart: 7, LineEnd: 11},
				Outcome: OutcomeNotFound,
				Error:   "SEARCH not present",
			},
		},
		AppliedCount: 1,
		FailedCount:  1,
		Diff:         "concatenated-diff",
		StartedAt:    now,
		CompletedAt:  now.Add(2 * time.Second),
		Atomic:       true,
	}

	data, err := json.Marshal(&orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got SmartEditResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Results) != len(orig.Results) {
		t.Fatalf("Results len: got %d, want %d", len(got.Results), len(orig.Results))
	}
	if got.AppliedCount != orig.AppliedCount {
		t.Errorf("AppliedCount: got %d, want %d", got.AppliedCount, orig.AppliedCount)
	}
	if got.FailedCount != orig.FailedCount {
		t.Errorf("FailedCount: got %d, want %d", got.FailedCount, orig.FailedCount)
	}
	if got.Diff != orig.Diff {
		t.Errorf("Diff: got %q, want %q", got.Diff, orig.Diff)
	}
	if !got.StartedAt.Equal(orig.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", got.StartedAt, orig.StartedAt)
	}
	if !got.CompletedAt.Equal(orig.CompletedAt) {
		t.Errorf("CompletedAt: got %v, want %v", got.CompletedAt, orig.CompletedAt)
	}
	if got.Atomic != orig.Atomic {
		t.Errorf("Atomic: got %v, want %v", got.Atomic, orig.Atomic)
	}

	// AtomicError empty should be omitted.
	if containsSubstr(string(data), `"atomic_error"`) {
		t.Errorf("expected empty AtomicError to be omitted, got %s", string(data))
	}
}

// TestSmartEditResult_IsZero_Empty asserts the zero value reports IsZero.
func TestSmartEditResult_IsZero_Empty(t *testing.T) {
	var r SmartEditResult
	if !r.IsZero() {
		t.Fatal("zero SmartEditResult.IsZero() = false, want true")
	}
}

// TestSmartEditResult_IsZero_NonEmpty asserts a populated result is not zero.
func TestSmartEditResult_IsZero_NonEmpty(t *testing.T) {
	cases := []struct {
		name string
		r    SmartEditResult
	}{
		{"with-results", SmartEditResult{Results: []EditResult{{Outcome: OutcomeApplied}}}},
		{"with-applied-count", SmartEditResult{AppliedCount: 1}},
		{"with-failed-count", SmartEditResult{FailedCount: 1}},
		{"with-diff", SmartEditResult{Diff: "x"}},
		{"with-started-at", SmartEditResult{StartedAt: time.Now()}},
		{"with-atomic-true", SmartEditResult{Atomic: true}},
		{"with-atomic-error", SmartEditResult{AtomicError: "boom"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.r.IsZero() {
				t.Fatalf("SmartEditResult.IsZero() = true for %s, want false", c.name)
			}
		})
	}
}

// TestSentinelErrors_Distinct asserts each sentinel error has a unique message.
func TestSentinelErrors_Distinct(t *testing.T) {
	all := []error{
		ErrInvalidBlockStructure,
		ErrSearchEmpty,
		ErrSearchNotFound,
		ErrSearchAmbiguous,
		ErrPromptTooLarge,
		ErrTooManyBlocks,
		ErrFileTooLarge,
		ErrSearchTooLarge,
		ErrReplaceTooLarge,
		ErrBinaryFile,
		ErrPathRequired,
	}

	seen := make(map[string]int, len(all))
	for i, e := range all {
		if e == nil {
			t.Fatalf("sentinel #%d is nil", i)
		}
		msg := e.Error()
		if msg == "" {
			t.Fatalf("sentinel #%d has empty message", i)
		}
		if prior, dup := seen[msg]; dup {
			t.Errorf("duplicate sentinel message %q at indices %d and %d", msg, prior, i)
		}
		seen[msg] = i
	}

	// errors.Is should distinguish the sentinels from each other.
	if errors.Is(ErrSearchEmpty, ErrSearchNotFound) {
		t.Errorf("ErrSearchEmpty must not match ErrSearchNotFound via errors.Is")
	}
	if errors.Is(ErrFileTooLarge, ErrPromptTooLarge) {
		t.Errorf("ErrFileTooLarge must not match ErrPromptTooLarge via errors.Is")
	}
	// Each sentinel matches itself via errors.Is.
	for _, e := range all {
		if !errors.Is(e, e) {
			t.Errorf("errors.Is(%v, %v) = false; want true", e, e)
		}
	}
}

// containsSubstr is a tiny dependency-free substring check used only by the
// JSON-omission assertions. Avoids pulling in `strings` for one call.
func containsSubstr(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
