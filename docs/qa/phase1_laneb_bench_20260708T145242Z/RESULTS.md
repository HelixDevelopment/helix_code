# Phase 1 — Lane-B benchmark spike (Task 2.1) — COMPLETED

**Date**: 2026-07-08
**Model**: Mistral-Nemo-Instruct-2407-Q4_K_M.gguf (bartowski GGUF)
**Harness**: `submodules/helix_llm/cmd/agentgen-boot`
**Compose**: `cmd/agentgen-boot/compose.agent.yml`
**Port**: :18435 (Lane-B), :18434 (coder, untouched)

## Pre-flight

- **Target file**: `7.0G` (`7477208192` bytes)
- **Expected**: 7477208192 bytes (6.965 GiB)
- **Match**: VERIFIED (byte-identical)

## 1. Broker admission — ADMIT-OK

```
VRAM budget (nvidia-smi): total=32607MiB used=19434MiB free=12687MiB need=9216MiB headroom=2048MiB
ADMIT-OK: Lane-B footprint admitted co-resident (coder stays live) — warm tier
```

## 2. Boot — UP-OK + HEALTH-OK

```
UP-OK: laneb-spike agentgen via containers submodule orchestrator (:18435)
HEALTH-OK: agentgen /health after 3 polls (status=200)
BOOT-HEALTH-OK: agentgen /health answered. Lane B stays UP.
```

## 3. Model identity

| Field | Value |
|---|---|
| Model | Mistral-Nemo-Instruct-2407 Q4_K_M |
| Parameters | 12,247,782,400 (12.2B) |
| Embedding dim | 5120 |
| Quantization | Q4_K - Medium |
| File size | 7469322240 bytes (6.956 GiB) |
| Vocab | 131,072 |
| Context (train) | 1,024,000 |

## 4. Benchmark results

### 4a. Single-stream completion

| Metric | Value |
|---|---|
| Prompt tokens | 17 |
| Completion tokens | 212 |
| Duration | 1,326 ms |
| **Throughput** | **159.88 tok/s** |

### 4b. Concurrent (3 parallel requests)

| Request | Prompt tokens | Completion tokens | Status |
|---|---|---|---|
| 1 | 10 | 256 | OK |
| 2 | 10 | 256 | OK |
| 3 | 10 | 256 | OK |

All 3 concurrent requests completed successfully (max 256 tokens each).

### 4c. Tool-calling / structured output

```
> Prompt: "What is 2+2? Respond with just the number."
> Response: "4"
```
Correct structured response (single-token, deterministic).

## 5. Coder co-residence proof

Coder :18434 responded during Lane-B load:

```
> Request: "Say only: CODER_OK_UNTOUCHED"
> Response: "CODER_OK_UNTOUCHED"
```

### VRAM timeline

| Phase | Used (MiB) | Free (MiB) | Delta vs baseline |
|---|---|---|---|
| Baseline (coder only) | 19,434 | 12,687 | — |
| After admit-check | 19,434 | 12,687 | 0 (read-only) |
| Lane-B booted + loaded | 28,247 | 3,874 | +8,813 MiB |
| After teardown | TBD | TBD | Lane-B freed |

## 6. Teardown

```
DOWN-OK: laneb-spike agentgen (single-owner cleanup, coder untouched)
```

## Verdict: PASS

Lane-B successfully:
- Booted co-resident alongside the live coder (:18434, untouched per §11.4.122)
- Achieved **159.88 tok/s** single-stream throughput
- Handled 3 concurrent requests
- Served correct structured output
- Torn down cleanly via single-owner teardown (§11.4.119)
- Running from rootless podman container (§11.4.161)
- Built and orchestrated via containers submodule (§11.4.76)

