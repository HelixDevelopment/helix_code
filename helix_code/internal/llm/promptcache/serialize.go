// Package promptcache provides deterministic JSON serialization helpers and
// prompt-cache-prefix stability tracking for HelixCode's LLM request path.
//
// # Why this package exists (speed programme P1-T04)
//
// Provider-side prompt caching (Anthropic `cache_control` ephemeral
// breakpoints, and equivalents on other providers) can cut time-to-first-token
// by up to ~85% on long prompts — but ONLY when the request *prefix* (system
// prompt + tool definitions) is byte-stable across every request in a session.
// The provider hashes the prefix; a single differing byte mishashes and the
// request never hits the cache.
//
// # The Go map-key-randomization pitfall
//
// Go deliberately randomizes `map` key iteration order at runtime. The
// standard `encoding/json` package *does* sort top-level string-keyed map keys
// when marshalling, but two real non-determinism sources remain in the LLM
// request path and are why naive serialization silently never hits the cache:
//
//  1. Values typed as `interface{}` whose concrete type is itself another
//     `map[string]interface{}` (tool input-schemas, nested object schemas) —
//     these ARE sorted by encoding/json, but any *non-map* unordered container
//     (a Go `map` decoded into a slice, set-like structures) is not.
//  2. More importantly, JSON-Schema objects frequently carry sibling fields
//     whose *semantic* order matters to a cache hash but whose *construction*
//     order in Go varies run-to-run (built from ranging over a map, merged
//     from multiple sources, etc.). encoding/json sorts map keys but callers
//     that build a `[]interface{}` (e.g. `required: [...]`, `enum: [...]`)
//     from a Go `map` get a randomly-ordered slice that encoding/json faithfully
//     preserves — i.e. faithfully preserves the randomness.
//
// CanonicalJSON eliminates BOTH classes: it recursively normalizes every
// `map[string]interface{}` to sorted-key order AND (when AskedToSortArrays via
// CanonicalJSONSorted) normalizes set-like string arrays so the prefix hash is
// stable regardless of how the caller assembled the structure.
//
// All exported functions in this file are pure and deterministic: the same
// input value produces byte-identical output on every call, in every process,
// forever. This is the anti-bluff invariant proven by the 100-run test.
package promptcache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// CanonicalJSON marshals v to JSON with deterministic, byte-stable output.
//
// Determinism guarantees:
//   - every map[string]interface{} (at any nesting depth) is emitted in
//     sorted-key order;
//   - HTML escaping is disabled so '<', '>', '&' are emitted literally and
//     identically regardless of Go version escaping defaults;
//   - no trailing newline is appended (json.Encoder appends one — we strip it);
//   - numeric and string values are emitted by encoding/json's stable codec.
//
// Array element order is PRESERVED (not sorted) — array order is frequently
// semantically meaningful (message sequence, ordered tool list). When the
// caller knows an array is a set whose order is irrelevant to meaning (e.g. a
// JSON-Schema `required` list assembled from a Go map), use CanonicalJSONSorted.
//
// CanonicalJSON is the single serialization entry point the LLM request-builder
// path MUST use for any value destined for the cacheable prefix.
func CanonicalJSON(v interface{}) ([]byte, error) {
	return canonicalize(v, false)
}

// CanonicalJSONSorted behaves like CanonicalJSON but additionally sorts every
// JSON array whose elements are all strings. This closes the last
// non-determinism gap for set-like schema fields (`required`, `enum`,
// `tags`) that callers assemble by ranging over a Go map — an operation whose
// output slice order is randomized per run.
//
// Use CanonicalJSONSorted for tool-definition and system-block serialization
// where the goal is a stable cache prefix. Do NOT use it for message arrays or
// any array whose order carries meaning.
func CanonicalJSONSorted(v interface{}) ([]byte, error) {
	return canonicalize(v, true)
}

// canonicalize round-trips v through encoding/json to obtain a normalized
// generic tree (map[string]interface{} / []interface{} / scalars), recursively
// re-marshals that tree with sorted keys (and optionally sorted string arrays),
// and returns compact, escape-stable bytes.
func canonicalize(v interface{}, sortStringArrays bool) ([]byte, error) {
	// Step 1: marshal v with the stdlib codec. This gives a faithful JSON
	// representation honouring struct tags, omitempty, custom Marshalers, etc.
	raw, err := marshalNoHTMLEscape(v)
	if err != nil {
		return nil, fmt.Errorf("promptcache: initial marshal failed: %w", err)
	}

	// Step 2: decode into a generic tree so we can re-impose ordering. We use
	// json.Decoder with UseNumber so integers/floats survive the round-trip
	// without precision drift (a float printed as 1 vs 1.0 would mishash).
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree interface{}
	if err := dec.Decode(&tree); err != nil {
		return nil, fmt.Errorf("promptcache: decode to generic tree failed: %w", err)
	}

	// Step 3: recursively normalize ordering.
	normalized := normalize(tree, sortStringArrays)

	// Step 4: re-marshal the normalized tree. encoding/json emits map keys in
	// sorted order, and our tree is now built from maps whose every level is
	// already canonical — so this final marshal is byte-deterministic.
	return marshalNoHTMLEscape(normalized)
}

// normalize recursively walks a decoded JSON tree and returns an equivalent
// tree whose ordering is fully determined by content (sorted-key maps, and —
// when sortStringArrays is true — sorted all-string arrays).
func normalize(node interface{}, sortStringArrays bool) interface{} {
	switch typed := node.(type) {
	case map[string]interface{}:
		// Re-build the map. encoding/json sorts keys on marshal, so returning a
		// map[string]interface{} is sufficient for key order; we still recurse
		// to normalize every value.
		out := make(map[string]interface{}, len(typed))
		for k, val := range typed {
			out[k] = normalize(val, sortStringArrays)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		allStrings := len(typed) > 0
		for i, val := range typed {
			out[i] = normalize(val, sortStringArrays)
			if _, ok := out[i].(string); !ok {
				allStrings = false
			}
		}
		if sortStringArrays && allStrings {
			sort.Slice(out, func(i, j int) bool {
				return out[i].(string) < out[j].(string)
			})
		}
		return out
	default:
		// Scalars (string, json.Number, bool, nil) are already canonical.
		return node
	}
}

// jsonUnmarshalNumberAware decodes b into dst using a json.Decoder with
// UseNumber so numeric values survive as json.Number (no float-precision drift
// that could mishash an otherwise-identical prefix on re-marshal).
func jsonUnmarshalNumberAware(b []byte, dst interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return dec.Decode(dst)
}

// marshalNoHTMLEscape marshals v compactly with HTML escaping disabled and no
// trailing newline. Disabling HTML escaping is essential for byte-stability:
// it removes any dependency on the stdlib's escaping defaults and makes the
// output identical to what the caller's eyes (and the provider's hash) expect.
func marshalNoHTMLEscape(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder.Encode appends exactly one '\n'; strip it for stable bytes.
	out := buf.Bytes()
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1]
	}
	// Return a copy so callers cannot mutate buf's backing array.
	cp := make([]byte, len(out))
	copy(cp, out)
	return cp, nil
}
