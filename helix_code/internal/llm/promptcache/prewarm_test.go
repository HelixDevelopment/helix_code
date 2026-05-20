package promptcache

import "testing"

// TestBuildWarmRequest_IsMaxTokensMinimal proves the warm request asks for the
// minimal completion budget — the request exists to cache the prefix, not to
// generate a real (billable) completion. CONST-050 unit-test coverage of the
// P1-T06 "warm request is max_tokens-minimal" invariant.
func TestBuildWarmRequest_IsMaxTokensMinimal(t *testing.T) {
	prefix := PrefixComponents{
		SystemPrompt: "You are HelixCode, an enterprise AI development platform.",
		Tools:        []interface{}{map[string]interface{}{"name": "read_file"}},
	}
	w := BuildWarmRequest(prefix)

	if w.MaxTokens != MinWarmTokens {
		t.Fatalf("warm request MaxTokens = %d, want MinWarmTokens (%d)", w.MaxTokens, MinWarmTokens)
	}
	if MinWarmTokens > 1 {
		t.Fatalf("MinWarmTokens = %d; a pre-warm must ask for near-zero output", MinWarmTokens)
	}
	if !w.IsMinimal() {
		t.Fatal("IsMinimal() = false for a freshly built warm request; want true")
	}
}

// TestBuildWarmRequest_CarriesStablePrefix proves the warm request carries the
// EXACT system prompt + tool set of the session's stable prefix, and that the
// warm request's prefix hash is byte-identical to the prefix hash a real
// request with the same prefix produces. That identity is the entire
// pre-warming mechanism: the warm call writes the cache entry, the first real
// call reads it — only possible if the two prefixes hash the same.
func TestBuildWarmRequest_CarriesStablePrefix(t *testing.T) {
	prefix := PrefixComponents{
		SystemPrompt: "You are HelixCode. Be precise.",
		Tools: []interface{}{
			map[string]interface{}{"name": "read_file", "required": []interface{}{"path"}},
			map[string]interface{}{"name": "write_file", "required": []interface{}{"path", "content"}},
		},
	}
	w := BuildWarmRequest(prefix)

	if w.SystemPrompt != prefix.SystemPrompt {
		t.Fatalf("warm SystemPrompt = %q, want %q", w.SystemPrompt, prefix.SystemPrompt)
	}
	if len(w.Tools) != len(prefix.Tools) {
		t.Fatalf("warm Tools len = %d, want %d", len(w.Tools), len(prefix.Tools))
	}

	// The hash of the warm request's prefix MUST equal the hash of the
	// real-request prefix — otherwise the first real request would miss the
	// cache the warm request created.
	warmHash, err := w.PrefixHash()
	if err != nil {
		t.Fatalf("WarmRequest.PrefixHash failed: %v", err)
	}
	realHash, err := prefix.Hash()
	if err != nil {
		t.Fatalf("PrefixComponents.Hash failed: %v", err)
	}
	if warmHash != realHash {
		t.Fatalf("warm prefix hash %s != real prefix hash %s — first real request would NOT hit the warmed cache",
			short(warmHash), short(realHash))
	}
}

// TestBuildWarmRequest_HasNonEmptyUserMessage proves the warm request carries
// a non-empty user turn — every provider rejects a request with zero
// messages, so a content-free-but-present user message is required for the
// warm-up call to be accepted (and thus to cache the prefix at all).
func TestBuildWarmRequest_HasNonEmptyUserMessage(t *testing.T) {
	w := BuildWarmRequest(PrefixComponents{SystemPrompt: "sys"})
	if w.WarmUserMessage == "" {
		t.Fatal("WarmUserMessage is empty; providers reject zero-message requests")
	}
}

// TestBuildWarmRequest_ToolsAreCopied proves BuildWarmRequest copies the tool
// slice — mutating the source prefix after building a warm request must not
// retroactively change the warm request (the warm request is meant to be a
// frozen snapshot of the prefix at session open).
func TestBuildWarmRequest_ToolsAreCopied(t *testing.T) {
	tools := []interface{}{map[string]interface{}{"name": "a"}}
	prefix := PrefixComponents{SystemPrompt: "sys", Tools: tools}
	w := BuildWarmRequest(prefix)

	tools[0] = map[string]interface{}{"name": "MUTATED"}
	got, _ := w.Tools[0].(map[string]interface{})
	if got["name"] == "MUTATED" {
		t.Fatal("BuildWarmRequest did not copy Tools; warm request mutated by source change")
	}
}

// TestWarmRequest_IsMinimal_RejectsNonMinimal proves IsMinimal correctly flags
// a hand-constructed warm request whose MaxTokens is NOT minimal — the
// session-open hook relies on this to refuse dispatching a request that would
// generate a real billable completion.
func TestWarmRequest_IsMinimal_RejectsNonMinimal(t *testing.T) {
	bad := WarmRequest{SystemPrompt: "sys", MaxTokens: 4096}
	if bad.IsMinimal() {
		t.Fatal("IsMinimal() = true for a 4096-token request; want false")
	}
}
