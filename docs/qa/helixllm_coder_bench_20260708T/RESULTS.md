# HelixQA Benchmarking Bank: HelixLLM Coder — EVIDENCE

**Bank**: `banks/helixllm_coder_bench.yaml`
**Analyzer**: `cmd/helixqa-verify-coder-bench/`
**Date**: 2026-07-08
**Run-ID**: `helixllm_coder_bench_20260708T`
**Precondition**: Coder at :18434 (llama.cpp, `llama3.2` or configured model) MUST already be running.

## Test-case summary

| Case | Concurrency | N | Baseline threshold | Status |
|---|---|---|---|---|
| BENCH-CODER-001 | 1 | 20 | p99<30000ms, throughput>0.3rps | GREEN |
| BENCH-CODER-002 | 10 | 30 | p99<30000ms, throughput>1.0rps | GREEN |
| BENCH-CODER-003 | 50 | 30 | p99<60000ms, throughput>2.0rps | GREEN |
| BENCH-CODER-004 | 100 | 30 | p99<120000ms, throughput>3.0rps | GREEN |
| BENCH-CODER-005 | 1(stream) | 10 | TTFT p95<5000ms | GREEN |
| SELF-VALIDATE-GOOD | 1 | 3 | throughput>0.01rps (trivial) | PASS |
| SELF-VALIDATE-BAD | 1 | 3 | p99<1ms (impossible) | raw FAIL, case PASS (`--expect-fail`) |

## Verdict: ALL GREEN

All six test cases PASS against the live coder at :18434. Baseline thresholds
are current-iteration targets; regressions below these thresholds are findings
per §11.4.132 risk-ordered validation priority.

## Anti-bluff

Every case recorded genuine measured latency/throughput/TTFT from real
concurrent HTTP round-trips against the live llama.cpp coder process.
The golden-bad case (p99 < 1ms) correctly FAILs at the raw assertion level,
proving the threshold comparisons are load-bearing (§11.4.107(10)
self-validation).

Evidence paths: `qa-results/helixllm_coder_bench/*.verdict.json`
