package llm

import "math"

// CharBasedTokenCount returns a conservative token estimate (1 token ≈ 3.5 chars).
// Used as a fallback by providers that don't have a native tokenizer reachable
// at runtime. Per CONST-035, individual providers SHOULD upgrade to their
// native tokenizer (Phase 3 sub-spec); this fallback keeps the Provider
// interface contract honoured in the meantime.
func CharBasedTokenCount(text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	return int(math.Ceil(float64(len(text)) / 3.5)), nil
}
