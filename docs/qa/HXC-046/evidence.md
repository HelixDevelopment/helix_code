# HXC-046 — generateThreadID() non-unique under fast back-to-back calls (uniqueness contract violated)
internal/memory/providers TestGenerateThreadID — zep_provider_test.go:628: Should not be:
"thread-1781003815158614000" — two consecutive generateThreadID() calls returned IDENTICAL IDs because the
generator derives the ID purely from a nanosecond timestamp; on fast Apple Silicon both landed in the same
nanosecond. Fix: add a monotonic counter / entropy suffix. Deterministically reproducible. (HEAD 54ab4e95)

## FIXED — RED→GREEN
RED (S4 capture): TestGenerateThreadID zep_provider_test.go:628 "Should not be: thread-1781003815158614000" (two calls identical).
FIX: added package-level `var threadIDCounter atomic.Uint64`; generateThreadID now returns
"thread-%d-%d" (UnixNano, threadIDCounter.Add(1)) — unique even within the same nanosecond. +import sync/atomic.
GREEN (this session): `go test -count=1 -run TestGenerateThreadID ./internal/memory/providers/` → ok 1.094s; package build OK.
