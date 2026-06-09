# HXC-064 — cognee AMD-GPU parser tests flake under parallel load (§11.4.50)
**Captured:** 2026-06-09T16:25:30Z · Bug · Fixed (→ Fixed.md)
## RED (captured by D-8 inner-app sweep)
go test internal/cognee under heavy parallel batch load: TestProbeAMDGPU_HandlesAltKeyName +
_GpuUtilization FAIL — performance_optimizer_gpu_amd_test.go:129 "Max difference between 33 and -1
allowed is 0.0001, but difference was 34" (and :138 expecting 77, got -1). Cause: fake rocm-smi echo
subprocess signal-killed before the 2s const timeout under host saturation → product correctly returns
sentinel -1; parser test asserts 33/77 → non-deterministic FAIL. Product correct; test timeout load-fragile.
## Fix (§11.4.50 determinism)
performance_optimizer.go:1259 const rocmSmiQueryTimeout → var (production default 2s UNCHANGED).
The 2 parser tests raise it to 30s with t.Cleanup-restore (they exercise the PARSER, not the timeout).
TestProbeAMDGPU_HandlesTimeout still reads the restored 2s (tests run serially, cleanup restores).
## GREEN (captured)
COGNEE_BUILD_OK; the 2 parser tests + HandlesTimeout -count=10 -parallel 16 → ok 23.175s (10/10);
whole cognee package -count=1 → ok 20.734s. Deterministic under load.
