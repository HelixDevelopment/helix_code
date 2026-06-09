# HXC-044 — cognee AMD-GPU rocm-smi JSON parser returns -1 sentinel instead of parsed value
Found by isolated-worktree `go test -count=1 ./internal/...` (HEAD 54ab4e95, go 1.26.2).
internal/cognee TestProbeAMDGPU_ParsesRocmSmiJSON — performance_optimizer_gpu_amd_test.go:111:
"Max difference between 42 and -1 ... difference was 43" — hermetic test (fake rocm-smi on PATH via t.Setenv,
no infra) drove real queryAMDGPUUsage() with {"card0":{"GPU use (%)":"42"}} and got -1 (unavailable sentinel)
instead of 42. The rocm-smi single-card JSON parse path does not surface the reading. Genuine parser defect.
