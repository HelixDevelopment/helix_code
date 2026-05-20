package promptcache

import "testing"

// BenchmarkCanonicalJSONSorted measures the per-call cost of deterministic
// tool-schema serialization. The cost is paid once per request prefix; it is
// negligible against the ~85% TTFT win on a prompt-cache hit.
func BenchmarkCanonicalJSONSorted(b *testing.B) {
	schema := buildSchemaFromMaps()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := CanonicalJSONSorted(schema); err != nil {
			b.Fatalf("CanonicalJSONSorted failed: %v", err)
		}
	}
}

// BenchmarkPrefixHash measures the cost of hashing a full request prefix
// (system prompt + one tool definition).
func BenchmarkPrefixHash(b *testing.B) {
	prefix := makePrefix("You are HelixCode, an enterprise AI development platform.")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := prefix.Hash(); err != nil {
			b.Fatalf("Hash failed: %v", err)
		}
	}
}

// BenchmarkCacheBreakCheck measures the cost of a per-turn cache-break check.
func BenchmarkCacheBreakCheck(b *testing.B) {
	det := NewCacheBreakDetector()
	if _, err := det.Freeze(makePrefix("You are HelixCode.")); err != nil {
		b.Fatalf("freeze failed: %v", err)
	}
	prefix := makePrefix("You are HelixCode.")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := det.Check(prefix); err != nil {
			b.Fatalf("Check failed: %v", err)
		}
	}
}
