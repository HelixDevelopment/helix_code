# Retro-capture: CLI real LLM generate + models list + health

**Retro-captured 2026-06-16 vs HEAD `9e6e0458` (`9e6e0458f93b7e50ee6d47d8324b34f02e77b31c`).**
This is a CURRENT, fresh end-to-end run of the shipped feature on the running
System — it is explicitly a **retro-capture done on 2026-06-16**, NOT a
reconstruction of the original feature-shipping moment, and NOT claimed as
historical evidence (§11.4.6 / §11.4.123). The original LLM-generate feature is
the anti-BLUFF-001/002 resolution in `helix_code/cmd/cli/main.go`; this directory
backfills the §11.4.83 docs/qa gap (G7) with a real run.

## What was exercised (real end-user CLI path)

Provider: DeepSeek (`. ~/api_keys.sh; export HELIX_LLM_PROVIDER=deepseek`).

| File | Feature | Real result |
|---|---|---|
| `cli_generate_run.txt` | `cli -non-interactive -provider deepseek -prompt "What is 2+2?..."` | Model returned `4`; real token accounting `in=17 out=30 total=47`, `time=1.298s`, `finish=length`. |
| `cli_models_list.txt` | `cli -list-models -provider deepseek` (CONST-036 LLMsVerifier source-of-truth; anti-BLUFF-002) | Live `/models` catalog refresh → 2 real models (`deepseek-v4-flash`, `deepseek-v4-pro`), real context size 128000. NOT a hardcoded list. |
| `cli_health.txt` | `cli -health` | Real health check: `⚠️ Worker Pool: No healthy workers` + `✅ System is operational` (honest live state). |

## Anti-bluff notes

- Every transcript is a REAL run with REAL provider responses — no simulated /
  placeholder output (§11.4 / §11.4.1 / §11.4.2).
- The `-list-models` run proves models come from a live provider query
  (`catalog refreshed ... (live /models)`), not a static array — the canonical
  BLUFF-002 anti-pattern guard.
- Raft/consensus log noise is the binary's normal single-node startup; the
  feature output lines (model reply, token table, model list, health verdict)
  are the load-bearing evidence.
