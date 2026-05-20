package promptcache

import (
	"testing"
)

// makePrefix builds a PrefixComponents with a tool whose schema arrays are
// map-derived (random order per construction).
func makePrefix(systemPrompt string) PrefixComponents {
	return PrefixComponents{
		SystemPrompt: systemPrompt,
		Tools: []interface{}{
			map[string]interface{}{
				"name":         "read_file",
				"description":  "Read a file from disk",
				"input_schema": buildSchemaFromMaps(),
			},
		},
	}
}

// TestPrefixHash_StableAcross100Runs proves the prefix hash — the value the
// cache-break detector compares — is byte-deterministic. The same logical
// prefix, rebuilt 100 times with freshly-randomized schema arrays, MUST hash
// to a single value.
func TestPrefixHash_StableAcross100Runs(t *testing.T) {
	const runs = 100
	hashes := make(map[string]int)
	for i := 0; i < runs; i++ {
		h, err := makePrefix("You are HelixCode.").Hash()
		if err != nil {
			t.Fatalf("run %d: Hash failed: %v", i, err)
		}
		hashes[h]++
	}
	if len(hashes) != 1 {
		t.Errorf("ANTI-BLUFF FAILURE: prefix hash not stable — %d distinct hashes across %d runs:", len(hashes), runs)
		for h, c := range hashes {
			t.Errorf("  %s x%d", h, c)
		}
		return
	}
	for h, c := range hashes {
		t.Logf("ANTI-BLUFF PROOF: %d/%d prefix hashes identical, sha256=%s", c, runs, h)
	}
}

// TestCacheBreakDetector_QuietWhenStable asserts the detector does NOT report a
// break when the prefix is unchanged across many turns — even though each turn
// rebuilds the prefix with re-randomized schema arrays. A false-positive break
// would needlessly alarm operators; the detector must be silent when the cache
// will genuinely hit.
func TestCacheBreakDetector_QuietWhenStable(t *testing.T) {
	det := NewCacheBreakDetector()
	baseline, err := det.Freeze(makePrefix("You are HelixCode."))
	if err != nil {
		t.Fatalf("freeze failed: %v", err)
	}
	if !det.IsFrozen() {
		t.Fatal("detector should report frozen after Freeze")
	}
	t.Logf("frozen baseline hash: %s", baseline)

	for turn := 1; turn <= 25; turn++ {
		// Same logical prefix, freshly rebuilt (schema arrays re-randomized).
		res, err := det.Check(makePrefix("You are HelixCode."))
		if err != nil {
			t.Fatalf("turn %d: check failed: %v", turn, err)
		}
		if res.Broken {
			t.Fatalf("turn %d: FALSE-POSITIVE cache break on a stable prefix: %s", turn, res.Reason)
		}
		if res.ObservedHash != baseline {
			t.Fatalf("turn %d: observed hash %s != baseline %s on a stable prefix", turn, res.ObservedHash, baseline)
		}
	}
	t.Logf("ANTI-BLUFF PROOF: 25 turns with a stable (re-randomized) prefix — zero false-positive breaks")
}

// TestCacheBreakDetector_FiresWhenSystemPromptChanges asserts the detector
// reports a break when the system prompt is mutated mid-session.
func TestCacheBreakDetector_FiresWhenSystemPromptChanges(t *testing.T) {
	det := NewCacheBreakDetector()
	if _, err := det.Freeze(makePrefix("You are HelixCode.")); err != nil {
		t.Fatalf("freeze failed: %v", err)
	}
	res, err := det.Check(makePrefix("You are a DIFFERENT assistant."))
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if !res.Broken {
		t.Fatal("detector FAILED to report a cache break after the system prompt changed")
	}
	if res.Reason == "" {
		t.Error("broken result must carry a non-empty Reason")
	}
	if res.BaselineHash == res.ObservedHash {
		t.Error("baseline and observed hashes should differ when the prefix changed")
	}
	t.Logf("cache break correctly detected: %s", res.Reason)
}

// TestCacheBreakDetector_FiresWhenToolsetChanges asserts the detector reports a
// break when a tool is added/removed mid-session.
func TestCacheBreakDetector_FiresWhenToolsetChanges(t *testing.T) {
	det := NewCacheBreakDetector()
	if _, err := det.Freeze(makePrefix("You are HelixCode.")); err != nil {
		t.Fatalf("freeze failed: %v", err)
	}

	mutated := makePrefix("You are HelixCode.")
	mutated.Tools = append(mutated.Tools, map[string]interface{}{
		"name":         "write_file",
		"description":  "Write a file to disk",
		"input_schema": buildSchemaFromMaps(),
	})

	res, err := det.Check(mutated)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if !res.Broken {
		t.Fatal("detector FAILED to report a cache break after a tool was added")
	}
	t.Logf("toolset-change cache break correctly detected: %s", res.Reason)
}

// TestCacheBreakDetector_NoBaselineIsNotBroken asserts Check before Freeze
// returns a non-broken result (nothing to break against yet).
func TestCacheBreakDetector_NoBaselineIsNotBroken(t *testing.T) {
	det := NewCacheBreakDetector()
	if det.IsFrozen() {
		t.Fatal("a fresh detector must not be frozen")
	}
	res, err := det.Check(makePrefix("anything"))
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if res.Broken {
		t.Error("Check before Freeze must not report a break")
	}
}

// TestCacheBreakDetector_ConcurrentSafe exercises the detector from multiple
// goroutines to confirm the RWMutex guards hold (run with -race).
func TestCacheBreakDetector_ConcurrentSafe(t *testing.T) {
	det := NewCacheBreakDetector()
	if _, err := det.Freeze(makePrefix("You are HelixCode.")); err != nil {
		t.Fatalf("freeze failed: %v", err)
	}
	done := make(chan struct{})
	for g := 0; g < 8; g++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 50; i++ {
				_, _ = det.Check(makePrefix("You are HelixCode."))
				_ = det.IsFrozen()
				_ = det.Baseline()
			}
		}()
	}
	for g := 0; g < 8; g++ {
		<-done
	}
}
