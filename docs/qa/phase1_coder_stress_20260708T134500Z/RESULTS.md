# Coder Concurrent Load Test — §11.4.85
- **Date**: 2026-07-08 ~13:45 UTC
- **Coder**: Qwen3-Coder-30B-A3B-Q4_K_M @ 0.0.0.0:18434
- **GPU**: 32607 MiB total, 19440 used, 12681 free

## Results
| Test | Concurrent | All Correct | Latency |
|------|-----------|-------------|---------|
| Baseline | 1 | yes | ~50ms |
| 10 parallel | 10/10 | yes | sub-100ms |
| 50 parallel | 50/50 | yes | 481ms total (~104 req/s) |

## Verdict
Coder handles at least 50 concurrent requests cleanly. No errors, no OOM, GPU stable. All responses individually correct.
