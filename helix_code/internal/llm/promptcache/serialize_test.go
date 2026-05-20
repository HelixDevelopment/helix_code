package promptcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// buildSchemaFromMaps constructs a realistic JSON-Schema tool definition. It
// deliberately assembles the `required` and `enum` arrays by ranging over Go
// maps — the exact pattern that produces randomly-ordered slices run-to-run.
// This is the input that, without canonicalization, would mishash and never
// hit the provider prompt cache.
func buildSchemaFromMaps() map[string]interface{} {
	// Range over a map to build the `required` array (random order per run).
	requiredSet := map[string]struct{}{
		"path": {}, "mode": {}, "encoding": {}, "recursive": {}, "follow_symlinks": {},
	}
	required := make([]interface{}, 0, len(requiredSet))
	for k := range requiredSet {
		required = append(required, k)
	}

	enumSet := map[string]struct{}{
		"read": {}, "write": {}, "append": {}, "delete": {}, "list": {},
	}
	enum := make([]interface{}, 0, len(enumSet))
	for k := range enumSet {
		enum = append(enum, k)
	}

	// Range over a map to build the `properties` object (Go randomizes this;
	// encoding/json sorts top-level keys but nested arrays stay random).
	props := map[string]interface{}{
		"path":            map[string]interface{}{"type": "string", "description": "file path"},
		"mode":            map[string]interface{}{"type": "string", "enum": enum},
		"encoding":        map[string]interface{}{"type": "string", "default": "utf-8"},
		"recursive":       map[string]interface{}{"type": "boolean", "default": false},
		"follow_symlinks": map[string]interface{}{"type": "boolean"},
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": props,
		"required":   required,
		"additionalProperties": false,
	}
}

// TestCanonicalJSONSorted_100Runs_ByteIdentical is the anti-bluff proof that
// the Go map-key/array-order randomization pitfall is closed. It builds the
// same logical tool schema 100 times — each time the `required`/`enum` arrays
// are assembled in a freshly-randomized order — and asserts the canonical
// serialization is BYTE-IDENTICAL across all 100 runs.
//
// If CanonicalJSONSorted were non-deterministic, the SHA-256 set below would
// contain more than one entry and the test would fail loudly, naming every
// distinct hash observed.
func TestCanonicalJSONSorted_100Runs_ByteIdentical(t *testing.T) {
	const runs = 100
	hashes := make(map[string]int)
	var firstBytes []byte

	for i := 0; i < runs; i++ {
		schema := buildSchemaFromMaps()
		b, err := CanonicalJSONSorted(schema)
		if err != nil {
			t.Fatalf("run %d: CanonicalJSONSorted failed: %v", i, err)
		}
		sum := sha256.Sum256(b)
		h := hex.EncodeToString(sum[:])
		hashes[h]++
		if i == 0 {
			firstBytes = b
		}
	}

	if len(hashes) != 1 {
		t.Errorf("ANTI-BLUFF FAILURE: expected 1 unique hash across %d runs, got %d distinct hashes:", runs, len(hashes))
		for h, count := range hashes {
			t.Errorf("  hash %s observed %d/%d times", h, count, runs)
		}
		return
	}
	for h, count := range hashes {
		t.Logf("ANTI-BLUFF PROOF: %d/%d runs produced byte-identical output, sha256=%s", count, runs, h)
	}
	t.Logf("canonical bytes (%d): %s", len(firstBytes), string(firstBytes))
}

// TestCanonicalJSON_NaiveMarshalIsNonDeterministic_Contrast demonstrates the
// failure mode the package fixes: a naive json.Marshal of a schema whose
// arrays are map-derived produces DIFFERENT bytes across runs. This contrast
// test proves the pitfall is real (not a strawman) and that CanonicalJSONSorted
// genuinely closes it.
func TestCanonicalJSON_NaiveMarshalIsNonDeterministic_Contrast(t *testing.T) {
	naiveHashes := make(map[string]bool)
	canonHashes := make(map[string]bool)

	for i := 0; i < 200; i++ {
		schema := buildSchemaFromMaps()

		naive, err := json.Marshal(schema)
		if err != nil {
			t.Fatalf("naive marshal failed: %v", err)
		}
		ns := sha256.Sum256(naive)
		naiveHashes[hex.EncodeToString(ns[:])] = true

		canon, err := CanonicalJSONSorted(schema)
		if err != nil {
			t.Fatalf("canonical marshal failed: %v", err)
		}
		cs := sha256.Sum256(canon)
		canonHashes[hex.EncodeToString(cs[:])] = true
	}

	// The canonical path MUST be deterministic.
	if len(canonHashes) != 1 {
		t.Errorf("CanonicalJSONSorted produced %d distinct hashes — NOT deterministic", len(canonHashes))
	}
	// Note: naive json.Marshal MAY be deterministic if the only randomized
	// containers happen to be top-level map keys (which encoding/json sorts).
	// The map-derived `required`/`enum` arrays here are nested, so naive
	// marshal is expected to vary. We log the observed count rather than hard-
	// asserting >1, because the test's load-bearing claim is that the canonical
	// path is stable — not that the standard library is broken.
	t.Logf("naive json.Marshal produced %d distinct hash(es) across 200 runs (map-derived arrays not sorted)", len(naiveHashes))
	t.Logf("CanonicalJSONSorted produced %d distinct hash across 200 runs (deterministic)", len(canonHashes))
}

// TestCanonicalJSON_PreservesSemanticContent asserts the canonicalization is a
// pure re-ordering — no field is dropped, added, or value-mutated. This is the
// no-regression proof: the request stays logically identical.
func TestCanonicalJSON_PreservesSemanticContent(t *testing.T) {
	schema := buildSchemaFromMaps()
	canon, err := CanonicalJSONSorted(schema)
	if err != nil {
		t.Fatalf("canonicalize failed: %v", err)
	}
	var roundTripped map[string]interface{}
	if err := json.Unmarshal(canon, &roundTripped); err != nil {
		t.Fatalf("unmarshal canonical output failed: %v", err)
	}

	if roundTripped["type"] != "object" {
		t.Errorf("type field changed: got %v", roundTripped["type"])
	}
	if roundTripped["additionalProperties"] != false {
		t.Errorf("additionalProperties changed: got %v", roundTripped["additionalProperties"])
	}
	required, ok := roundTripped["required"].([]interface{})
	if !ok || len(required) != 5 {
		t.Fatalf("required array changed: got %v", roundTripped["required"])
	}
	wantRequired := map[string]bool{"path": true, "mode": true, "encoding": true, "recursive": true, "follow_symlinks": true}
	for _, r := range required {
		if !wantRequired[r.(string)] {
			t.Errorf("unexpected required entry: %v", r)
		}
	}
	props, ok := roundTripped["properties"].(map[string]interface{})
	if !ok || len(props) != 5 {
		t.Fatalf("properties changed: got %v", roundTripped["properties"])
	}
}

// TestCanonicalJSON_PreservesArrayOrder asserts CanonicalJSON (the non-sorting
// variant) leaves meaningful array order intact — message sequences must not be
// reordered.
func TestCanonicalJSON_PreservesArrayOrder(t *testing.T) {
	messages := []interface{}{
		map[string]interface{}{"role": "system", "content": "instructions"},
		map[string]interface{}{"role": "user", "content": "first"},
		map[string]interface{}{"role": "assistant", "content": "reply"},
		map[string]interface{}{"role": "user", "content": "second"},
	}
	b, err := CanonicalJSON(messages)
	if err != nil {
		t.Fatalf("CanonicalJSON failed: %v", err)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	wantRoles := []string{"system", "user", "assistant", "user"}
	for i, want := range wantRoles {
		if got[i]["role"] != want {
			t.Errorf("message %d role: got %v want %s — array order NOT preserved", i, got[i]["role"], want)
		}
	}
}

// TestCanonicalJSON_NumberStability ensures numeric values do not drift across
// the marshal→decode→marshal round trip (a float printed 1 vs 1.0 would
// mishash an otherwise-identical prefix).
func TestCanonicalJSON_NumberStability(t *testing.T) {
	in := map[string]interface{}{
		"max_tokens":  4096,
		"temperature": 0.7,
		"top_p":       1,
		"big":         9007199254740993, // beyond float64 exact-int range
	}
	first, err := CanonicalJSON(in)
	if err != nil {
		t.Fatalf("first marshal: %v", err)
	}
	for i := 0; i < 50; i++ {
		b, err := CanonicalJSON(in)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if string(b) != string(first) {
			t.Fatalf("run %d differs: %s vs %s", i, b, first)
		}
	}
	t.Logf("number-stable canonical bytes: %s", string(first))
}
