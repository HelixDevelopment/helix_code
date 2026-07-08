# HelixLLM Coder Benchmark Bank — Live Run

## Test environment

- **Coder**: llama.cpp (GPU, Qwen2.5 0.5B Q4_K_M, 128 slots, contiguous batching)
- **Endpoint**: http://localhost:18434/v1/chat/completions
- **GPU**: NVIDIA (CUDA 12.8, ngl=99)
- **Tool**: `helixqa-verify-coder-bench` (HelixQA cmd/helixqa-verify-coder-bench)
- **Date**: 2026-07-09

## Levels tested

| Level | N  | OK | p50(ms) | p95(ms) | p99(ms) | RPS   | TPS   | TTFTp50(ms) | TTFTp95(ms) |
|-------|----|----|---------|---------|---------|-------|-------|-------------|-------------|
| 1     | 20 | 20 | 59      | 169     | 173     | 11.8  | 756   | 1           | 2           |
| 10    | 20 | 19 | 257     | 597     | 597     | 27.2  | 1590  | 6           | 9           |
| 50    | 20 | 20 | 472     | 705     | 713     | 28.0  | 1574  | 6           | 14          |
| 100   | 20 | 20 | 788     | 1252    | 1259    | 15.9  | 1113  | 81          | 89          |

## Verdict

PASS — all 4 concurrency levels within bounds (latency-p99-max=60000ms, throughput-min=0.1rps, ttft-p95-max=15000ms).

## Self-validation

Golden-bad fixture (latency-p99-max=1ms, expect-fail=true) correctly detected impossible threshold: `case_result=true` (expected failure confirmed).
