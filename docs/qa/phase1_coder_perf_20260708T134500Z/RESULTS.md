# Coder Concurrent Throughput — §11.4.85
**Coder**: Qwen3-Coder-30B-A3B-Q4_K_M @ 0.0.0.0:18434
**GPU**: 32607 MiB, ~19440 used, ~12680 free
**180 total requests, 0 errors, 0 timeouts**

| Concurrency | Avg | P50 | P95 | P99 | Max |
|------------|-----|-----|-----|-----|-----|
| 10 | 99ms | 100ms | 112ms | 112ms | 112ms |
| 20 | 104ms | 111ms | 141ms | 141ms | 141ms |
| 50 | 191ms | 196ms | 292ms | 293ms | 293ms |
| 100 | 297ms | 297ms | 453ms | 591ms | 591ms |

Latency scales sub-linearly (2× concurrency → ~1.5× P50 at 50→100).
Single-inference-worker bottleneck visible but smooth — no saturation cliff.
Tail tight: P99 ≈ 1.5-2× P50 across all levels.
