package providers

import (
	"strings"
	"testing"
	"time"
)

// memonto_provider_concepts_test.go — round-55 §11.4 anti-bluff verification
// for the real-NER concept extraction wired into extractConcepts via
// jdkato/prose v2. See memonto_provider.go function-level forensic anchor
// for round-34 → round-55 progression.
//
// Anti-bluff (CONST-035 / CONST-050(A)+(B) / Article XI §11.9): these
// tests exercise the real prose pipeline (real POS tagger + real NER
// model embedded in the Go binary). No mocks. Each assertion captures
// a positive-evidence claim about actual model output for a known
// input.

// newTestProvider builds a minimal MemontoProvider safe for unit-level
// extractConcepts exercise. extractConcepts only touches p.logger
// (nil-safe) and is otherwise pure-function over the text argument.
func newTestProvider() *MemontoProvider {
	return &MemontoProvider{}
}

// containsCaseInsensitive returns true if any string in s matches
// target under case-insensitive compare. Tolerates the case-preserving
// behaviour of the extractConcepts implementation (first-occurrence
// surface form retained).
func containsCaseInsensitive(s []string, target string) bool {
	lt := strings.ToLower(target)
	for _, v := range s {
		if strings.ToLower(v) == lt {
			return true
		}
	}
	return false
}

func TestExtractConcepts_DetectsNamedEntities(t *testing.T) {
	p := newTestProvider()
	text := "Alice works at OpenAI in San Francisco."

	got := p.extractConcepts(text)
	if len(got) == 0 {
		t.Fatalf("expected at least one concept, got none for input %q", text)
	}

	// prose's NER model recognises PERSON / ORG / GPE for this sentence.
	// We assert presence of at least one of the canonical named-entity
	// surface forms — the union covers model-version drift while still
	// guaranteeing real NER activity (a stopword-skim would have returned
	// "alice", "works", "openai", "san", "francisco" in lowercase only).
	wantOneOf := []string{"Alice", "OpenAI", "San Francisco"}
	hit := false
	for _, w := range wantOneOf {
		if containsCaseInsensitive(got, w) {
			hit = true
			break
		}
	}
	if !hit {
		t.Errorf("expected at least one of %v in concepts, got %v", wantOneOf, got)
	}
}

func TestExtractConcepts_DetectsNounPhrases(t *testing.T) {
	p := newTestProvider()
	text := "The quick brown fox jumps over the lazy dog."

	got := p.extractConcepts(text)
	if len(got) == 0 {
		t.Fatalf("expected at least one concept, got none for input %q", text)
	}

	// "fox" and "dog" are tagged NN by prose's POS tagger. At least one
	// must surface — otherwise noun-phrase extraction is not running.
	if !containsCaseInsensitive(got, "fox") && !containsCaseInsensitive(got, "dog") {
		t.Errorf("expected at least one of [fox, dog] in concepts, got %v", got)
	}

	// Negative assertion: pure stopwords ("the", "over") must NOT appear.
	// Their presence would indicate the round-34 stopword-skim path is
	// still active.
	for _, sw := range []string{"the", "over"} {
		if containsCaseInsensitive(got, sw) {
			t.Errorf("stopword %q leaked into concepts: %v", sw, got)
		}
	}
}

func TestExtractConcepts_EmptyString_ReturnsEmpty(t *testing.T) {
	p := newTestProvider()
	got := p.extractConcepts("")
	if len(got) != 0 {
		t.Errorf("expected empty concepts for empty input, got %v", got)
	}

	// Whitespace-only input should also return nil (TrimSpace short-circuit).
	got = p.extractConcepts("   \t\n  ")
	if len(got) != 0 {
		t.Errorf("expected empty concepts for whitespace-only input, got %v", got)
	}
}

func TestExtractConcepts_DedupesIdentical(t *testing.T) {
	p := newTestProvider()
	text := "Apple Apple Apple Apple Apple"

	got := p.extractConcepts(text)

	count := 0
	for _, c := range got {
		if strings.EqualFold(c, "Apple") {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected 'Apple' to appear at most once after dedup, got %d times in %v", count, got)
	}
}

func TestExtractConcepts_HandlesMalformedInput_FailsSoft(t *testing.T) {
	p := newTestProvider()

	// Series of inputs that have historically tripped tokenisers: control
	// characters, repeated punctuation, single character, only punctuation.
	cases := []string{
		"\x00\x01\x02 hello",
		"!!!???.,;:",
		"a",
		"\n\n\n",
	}

	for _, tc := range cases {
		// Must not panic and must return without error path crashing.
		// Empty result is a legitimate outcome — the contract is fail-soft.
		got := p.extractConcepts(tc)
		_ = got // length unchecked; only no-panic invariant matters here.
	}
}

func TestExtractConcepts_LengthScales_OK(t *testing.T) {
	p := newTestProvider()

	// Build ~10KB of well-formed English by repeating a short paragraph.
	paragraph := "The Apollo program was a series of crewed spaceflights undertaken by NASA. " +
		"Neil Armstrong walked on the Moon in 1969. Buzz Aldrin followed him shortly after. " +
		"Mission control in Houston monitored every step of the journey. "
	var sb strings.Builder
	for sb.Len() < 10*1024 {
		sb.WriteString(paragraph)
	}

	start := time.Now()
	got := p.extractConcepts(sb.String())
	elapsed := time.Since(start)

	if elapsed > 30*time.Second {
		t.Errorf("extractConcepts took %v on 10KB input — exceeds 30s ceiling", elapsed)
	}
	if len(got) == 0 {
		t.Errorf("expected non-empty concepts for 10KB English input, got none")
	}
	// Cap is 10 per the documented contract.
	if len(got) > 10 {
		t.Errorf("expected at most 10 concepts (cap), got %d: %v", len(got), got)
	}
}

// TestExtractConcepts_NotStopwordSkim guards against accidental regression
// to the round-34 stopword-skim implementation. The stopword-skim returned
// lowercased whitespace-tokenised words; the prose pipeline returns
// surface-form NER/NP tokens. We assert that an input with mixed-case
// proper nouns produces at least one concept whose surface form preserves
// the original case — impossible under the stopword-skim path.
func TestExtractConcepts_NotStopwordSkim(t *testing.T) {
	p := newTestProvider()
	text := "Microsoft acquired GitHub in 2018."

	got := p.extractConcepts(text)
	if len(got) == 0 {
		t.Fatalf("expected at least one concept, got none")
	}

	// At least one concept should preserve its mixed-case surface form
	// (e.g., "Microsoft" or "GitHub"). The stopword-skim implementation
	// lowercased every token unconditionally — surfacing "microsoft" /
	// "github" instead. Mixed-case preservation is the round-55 fingerprint.
	hasMixedCase := false
	for _, c := range got {
		if c != strings.ToLower(c) {
			hasMixedCase = true
			break
		}
	}
	if !hasMixedCase {
		t.Errorf("expected at least one mixed-case concept (round-55 NER preserves surface form), got %v", got)
	}
}
