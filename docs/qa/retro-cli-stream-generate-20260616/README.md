# Retro-capture: CLI streaming LLM generation (`-stream`)

**Retro-captured 2026-06-16 vs HEAD `9e6e0458` (`9e6e0458f93b7e50ee6d47d8324b34f02e77b31c`).**
Fresh CURRENT run on the running System — explicitly a retro-capture on
2026-06-16, NOT original/historical evidence (§11.4.6 / §11.4.123). Backfills
the §11.4.83 docs/qa gap (G7) for the streaming-generation surface.

## What was exercised (real end-user CLI path)

Provider: DeepSeek.

| File | Feature | Real result |
|---|---|---|
| `cli_stream_generate.txt` | `cli -non-interactive -provider deepseek -stream -prompt "Count from 1 to 5..."` | Real streamed model output `1\n2\n3\n4\n5` + `✅ Generation completed`. |

## Anti-bluff notes

- Real streamed provider response over the `-stream` path; output is the
  model's, not a placeholder (§11.4 / §11.4.2).
