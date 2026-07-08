# Lane-B Mistral-Nemo-12B Re-Run — Phase 1 QA Evidence

**Date:** 2026-07-08  
**Run-ID:** phase1_laneb_rerun_20260708T203302Z  
**Model:** Mistral-Nemo-Instruct-2407-Q4_K_M.gguf (bartowski, ~6.96 GiB weights)  
**Backend:** llama.cpp via llama-server (container, rootless podman, NVIDIA CDI)  
**GPU:** NVIDIA GeForce RTX 5090 (32 GiB VRAM)  
**Port:** :18435 (Lane B) — :18434 (coder) untouched  
**Boot harness:** `agentgen-boot` (`submodules/helix_llm/cmd/agentgen-boot/`)

## Workflow Executed

1. **`agentgen-boot admit-check`** — VRAM admit via vrambroker (ClassAgent, warm tier)
   - Total VRAM: 32607 MiB | Free: 32119 MiB | Need: 9216 MiB | Headroom: 2048 MiB
   - **ADMIT-OK** — Lane-B footprint admitted co-resident

2. **`agentgen-boot boot compose.agent.yml lane-b`** — compose up + health poll
   - Container UP on :18435 after 3 health polls (~9s)
   - Coder :18434 untouched

3. **50-prompt benchmark** via OpenAI-compatible `/v1/completions`
   - 50 varied prompts (coding, math, creative, system design, security, ...)
   - max_tokens=256, temp=0.7

## Benchmark Results

| Metric | Value |
|---|---|
| Successful requests | 50 / 50 (100%) |
| Errors | 0 |
| Total tokens generated | 10,982 |
| Total wall-clock time | 79.1 s |
| **Mean token rate** | **151.63 tok/s** |
| **Median token rate** | **163.24 tok/s** |
| **Steady-state token rate** | **~163 tok/s** |
| Min token rate (outlier) | 46.95 tok/s (prompt ~25-29 — context refill phase) |
| Max token rate | 165.32 tok/s |
| StdDev | 29.85 tok/s |
| **Aggregate (overall) tok/s** | **138.78 tok/s** |
| Mean prompt latency | 1.58 s |
| Mean tokens/request | 220 |

### Notable Observations

- Prompts 25-29 showed lower tok/s (47-121) — this corresponds to the server's context-refill phase when the model's KV cache had to process several long contexts sequentially. Steady-state after prompt 30 returns to ~163 tok/s.
- Short prompts (16-18: poem/haiku/dialog) showed lower tok/s due to the fixed overhead of prompt processing dominating the generation time.
- The overall aggregate of 139 tok/s across the full 79s run accounts for both prompt processing and generation.

## Teardown

- `agentgen-boot down compose.agent.yml lane-b` executed.
- :18434 coder untouched throughout.
- :18435 port freed.
