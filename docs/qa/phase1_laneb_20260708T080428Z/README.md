# Phase 1 — Lane-B second-instance benchmark spike (Task 2.1) — BLOCKED on download

Attempted per serving plan Task 2.1 / master plan §6.1-6.2. Priority-1 candidate:
Mistral-Nemo-Instruct-2407 Q4_K_M (bartowski/Mistral-Nemo-Instruct-2407-GGUF),
content-length 7477208192 bytes (6.964 GiB).

## Broker admission — PROVEN (real, live nvidia-smi)

Built `cmd/agentgen-boot` (new, mirrors cmd/visiongen-boot exactly) — the
on-demand boot+health harness for Lane B using vrambroker.ClassAgent (landed
commit 2726cb2). `admit-check` ran against the LIVE running coder:

```
VRAM budget (nvidia-smi): total=32607MiB used=19444MiB free=12677MiB need=9216MiB headroom=2048MiB
ADMIT-OK: Lane-B footprint admitted co-resident (coder stays live) — warm tier
exit:0
```

This confirms the plan's VRAM math: needBytes=9 GiB (Mistral-Nemo weights
~6.96 GiB + modest 16384-ctx/4-parallel q8_0 KV + activation headroom) fits
comfortably under free=12677 MiB with the 2 GiB safety headroom, well inside
the plan's ~10.39 GiB Lane-B co-reside ceiling.

## Download — HONEST BLOCKED (§11.4.6 / §12.12, hard-bounded per task instructions)

No cached GGUF found on disk (checked ~/models, ~/.cache/huggingface — neither
had the file; `hf`/`huggingface-cli` not installed on host). Attempted a
resumable HTTPS download (`curl -C -`) directly from HuggingFace
(bartowski/Mistral-Nemo-Instruct-2407-GGUF).

Measured real sustained throughput over multiple samples: ~1.0-2.2 MB/s
(volatile, network-bound — NOT a guess, three consecutive real measurements:
289,378,520 -> 447,242,456 -> 539,091,160 -> 587,022,552 bytes across
~13:59-13:03, averaging ~1.3-1.8 MB/s).

At this rate, ETA for the remaining ~6.46 GiB was **~60-90 minutes** — this is
NOT a "reasonable single attempt" per the task's hard bound ("do NOT get stuck
on a slow download... prior attempts died on session-limit crashes"). Per the
task's explicit instruction, STOPPED the download at **637,022,424 / 7,477,208,192
bytes (8.52%)** rather than looping, and preserved the resumable partial file at
`~/models/Mistral-Nemo-Instruct-2407-Q4_K_M.gguf.part` (curl -C - will resume
it in a future session/attempt without re-downloading the first 637 MB).

**Verdict: BLOCKED-on-download.** The benchmark (tok/s, concurrent tok/s,
tool-calling correctness, coder co-residence proof) could NOT be run this
session because the model artifact never finished downloading. This is an
honest, evidence-backed blocker per §11.4.6/§11.4.123 — not a bluff.

## Coder untouched — confirmed before AND after (§11.4.122)

Before (baseline, matches plan's cited free-VRAM figure exactly):
```
32607, 19444, 12677
```

After stopping the download attempt and killing the download processes:
```
health: {"status":"ok"}
chat completion: "CODER_OK" (real, real model=/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf)
nvidia-smi: 32607, 19444, 12677   <- IDENTICAL to baseline, no VRAM drift
podman ps: helixllm-coder Up 2 hours, 8080/tcp, 50052/tcp
```

No Lane-B container was ever booted this session (admit-check only — no
`compose up` was invoked), so §11.4.119 single-resource-owner + §11.4.122
no-silent-removal are trivially satisfied: nothing besides the coder ever ran.

## What's needed to unblock

1. Complete the download of `Mistral-Nemo-Instruct-2407-Q4_K_M.gguf` to
   `~/models/` (resume via `curl -C - -o ~/models/Mistral-Nemo-Instruct-2407-Q4_K_M.gguf.part <url>`,
   then rename to strip `.part`) — either from a host with faster/more stable
   egress bandwidth, or over a longer background window explicitly budgeted by
   the operator (§11.4.89 background test execution — this download itself
   should run detached across a session boundary next time, not inline in an
   interactive spike).
2. Re-run `cmd/agentgen-boot admit-check` (already proven working) then
   `cmd/agentgen-boot boot cmd/agentgen-boot/compose.agent.yml laneb-spike`.
3. Run the benchmark (single-stream + concurrent tok/s, tool-call/structured-
   output correctness sample, concurrent coder completion) against
   `http://localhost:18435`.
4. `cmd/agentgen-boot down cmd/agentgen-boot/compose.agent.yml laneb-spike`
   to tear down (§11.4.119).

## Harness delivered this session (ready for the completed-download re-run)

- `submodules/helix_llm/cmd/agentgen-boot/main.go` — admit-check/boot/down/status,
  mirrors cmd/visiongen-boot exactly, ClassAgent, port :18435.
- `submodules/helix_llm/cmd/agentgen-boot/compose.agent.yml` — reuses the
  already-built router image, no rebuild, port :18435, env-parameterized model
  selection (§CONST-045/§11.4.28).

Both compile cleanly (`go build ./cmd/agentgen-boot`) and the admit-check path
is proven against the live broker + live nvidia-smi (not a mock).
