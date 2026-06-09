# HXC-054 — memory leak_detector parallel flake (§11.4.50)
**Captured:** 2026-06-09T14:54:03Z · Bug · Fixed (→ Fixed.md)
**Submodule:** submodules/memory · test `TestLeakDetector_FalsePositive_HeapOnly` (pkg/memory/leak_detector_edge_test.go)
## Reproduction (RED, pre-fix)
`go test ./pkg/memory -count=5 -race -parallel=16 -cpu=8` × 25 fresh runs → 21 PASS / 4 FAIL (ratios 101.57/104.71/105.69/106.21 just above 100.0 threshold).
## FACT root cause (§11.4.102)
HeapGrowthRatio uses process-global runtime.MemStats.HeapAlloc. Under t.Parallel(), sibling edge tests (50MB allocation etc.) inflate the shared heap; baseline captured at a post-GC trough (~1.5MB) vs live (~230MB) → ratio crosses 100× though this detector allocated nothing. Instrumented: baseline=1517176 live=237499520 ratio=156.54.
## Fix (root cause, not symptom)
Dropped t.Parallel() (so the heap-growth window isn't polluted by siblings) + runtime.GC()×2 before Start() (settled baseline). Assertion contract unchanged. +34/-3.
## GREEN evidence (post-fix)
- exact scenario -count=5 -race -parallel=16 -cpu=8 → 25/25 PASS
- mandated -count=50 -race → 50/50 PASS (160.8s)
- with large-allocator siblings under 22× CPU saturation, 50 runs → 50/50 PASS
- go vet clean
Commit `dfc8f03`, pushed ff `004d7b5..dfc8f03` (both remotes confirmed).
