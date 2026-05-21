# HXC-010 — Kimi CLI + Qwen Code CodeGraph end-to-end evidence

Captured 2026-05-21 after the operator supplied OpenAI-compatible router
credentials. Both agents were re-driven through their canonical Challenge
scripts and produced **true tier-1 PASS** results — each agent genuinely
invoked a `codegraph_*` MCP tool and received real graph data from the
scanned HelixCode code-graph.

## Path used

The originally-blocking backends are still blocked:

- **Kimi** — the `KIMI_API_KEY` shares the same exhausted account-level
  monthly billing-cycle quota as the bundled OAuth credentials
  (`exceeded_current_quota_error` on `api.kimi.com/coding/v1`).
- **Qwen Code** — bundled OAuth free tier discontinued 2026-04-15.

Both agents were therefore driven against an **OpenAI-compatible router**
(SiliconFlow — `OPENROUTER_API_KEY` had insufficient credit, ~$0.0007):

- Kimi CLI: an `openai_legacy`-type provider (config-file carrying a
  placeholder api_key); the real key is injected at runtime via the
  `OPENAI_API_KEY` environment variable — never written to disk.
- Qwen Code: `--auth-type openai` with `OPENAI_API_KEY` / `OPENAI_BASE_URL`
  / `OPENAI_MODEL` environment variables — never written into the tracked
  `.qwen/settings.json`.

The Challenge scripts `cg-challenge-05-kimi.sh` / `cg-challenge-07-qwen.sh`
now honour `HELIX_CG_OPENAI_API_KEY` + `HELIX_CG_OPENAI_BASE_URL`
(+ optional `HELIX_CG_QWEN_MODEL`) environment variables for the
credentialed path, falling back to the bundled (quota-gated) provider when
those env vars are absent.

## Result

| Challenge | Agent | Model (via SiliconFlow) | Verdict |
|---|---|---|---|
| CG-CHALLENGE-05 | Kimi CLI | `moonshotai/Kimi-K2.6` | PASS (true end-to-end) |
| CG-CHALLENGE-07 | Qwen Code | `Qwen/Qwen3-Coder-30B-A3B-Instruct` | PASS (true end-to-end) |

Both transcripts in this directory show the MCP loader connecting to the
`codegraph` server, the agent invoking `codegraph_search` for symbol
`Provider`, the tool returning 10 real `.go` symbol paths, and the agent
answering with a real file path. No API-key value appears in any transcript.
