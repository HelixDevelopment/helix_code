# HXC-044 — cognee AMD-GPU rocm-smi JSON parser returns -1 sentinel instead of parsed value
Found by isolated-worktree `go test -count=1 ./internal/...` (HEAD 54ab4e95, go 1.26.2).
internal/cognee TestProbeAMDGPU_ParsesRocmSmiJSON — performance_optimizer_gpu_amd_test.go:111:
"Max difference between 42 and -1 ... difference was 43" — hermetic test (fake rocm-smi on PATH via t.Setenv,
no infra) drove real queryAMDGPUUsage() with {"card0":{"GPU use (%)":"42"}} and got -1 (unavailable sentinel)
instead of 42. The rocm-smi single-card JSON parse path does not surface the reading. Genuine parser defect.

## RECLASSIFIED OBSOLETE — does NOT reproduce in main tree (not a real product defect)
S4 caught the failure ONLY in an isolated git-worktree (which also had 6 unrelated setup-failed packages from a
missing submodules/challenges checkout). In the real main tree: `go test -count=10 -run TestProbeAMDGPU ./internal/cognee/`
→ ok 10/10; full cognee package green. The parser parseRocmSmiUtilization correctly returns 42.0 for
{"card0":{"GPU use (%)":"42"}}. Root cause of the worktree failure = environment (PATH-scrub / missing shell for the
hermetic fake rocm-smi echo script in that worktree), NOT the code. No fix fabricated (§11.4.6 — fixing a
non-reproducing bug would itself be a bluff). Obsolete-Details: Since 2026-06-09; Reason: not-reproducible-environment-artifact
(isolated-worktree PATH/shell); Superseding-item: none; Evidence: 10/10 main-tree PASS above.
