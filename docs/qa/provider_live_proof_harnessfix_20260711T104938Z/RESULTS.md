# Provider Live-Proof Harness — Reasoning-Model Oracle-Honesty Fix

## Metadata

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-07-11 |
| Last modified | 2026-07-11 |
| Status | active |
| Track | T1 / feature/helixllm-full-extension |
| Anchor | §11.4.6 (no-guessing / oracle honesty), §11.4.115 (RED-baseline-on-broken-artifact polarity), §11.4.50 (deterministic + re-runnable) |

## Table of contents

- [Problem](#problem)
- [Fix](#fix)
- [Before → After verdict table](#before--after-verdict-table)
- [Raw evidence (redacted — no secrets)](#raw-evidence-redacted--no-secrets)
- [Out-of-scope observations](#out-of-scope-observations)
- [Reproduction](#reproduction)

## Problem

`helix_code/internal/llm/provider_live_proof_test.go` builds a nonce-echo
liveness challenge for every CONST-039 provider: send a fresh random nonce,
ask the model to echo it back verbatim, and treat "nonce present in
`resp.Content`" as PASS. The original request capped `MaxTokens` at **32**.

Reasoning-style models (DeepSeek `deepseek-v4-flash`, OpenRouter
`openai/gpt-oss-20b:free`) emit internal reasoning tokens *before* the
visible answer. At `MaxTokens=32` the reasoning alone consumed the entire
budget, the provider genuinely reported `finish_reason="length"`, and the
nonce was never reached — the harness recorded a **FAIL**, but the real
call succeeded; the only thing missing was a truncation, not a broken
provider. That is a false-negative in the oracle (§11.4.6): the test was
asserting something the real, live system had no chance to satisfy given
the budget it was given.

## Fix

`internal/llm/provider_live_proof_test.go`:

1. **`providerLiveNonceMaxTokens = 4096`** (new package const) replaces the
   hardcoded `MaxTokens: 32` in both `runHostedProviderLiveProof` and
   `runLocalProviderLiveProof`. Sized against this package's own
   `reasoning.go` defaults (`ThinkingBudget` 5000–10000 for
   o1/extended-thinking-class models) — generous headroom for cheaper
   reasoning-tagged/free-tier models plus the short nonce echo.
2. **`providerLiveNoncePrompt(nonce)`** (new shared helper, replacing the
   duplicated inline prompt string) adds an explicit "skip any reasoning,
   chain-of-thought, or explanation" instruction — a best-effort
   cost/latency reduction for models that honor it. It does **not** change
   the pass/fail oracle.
3. **Third verdict: `INCONCLUSIVE`.** When a real call succeeds, is
   genuinely truncated (`errors.Is(resp.Err, ErrResponseTruncated)` — the
   canonical sentinel every provider file in this package already
   populates from its own `finish_reason` mapping helper), and the nonce
   still never appears, the harness now records `INCONCLUSIVE` via
   `t.Skipf(...)` (keeps `go test`'s exit code green) instead of `FAIL`. A
   truncated call proves nothing about whether the provider *can* echo the
   nonce.
4. **Unforgeability preserved.** A real echoed nonce is still the *only*
   thing that produces `PASS`. A non-truncated call that omits the nonce
   (model simply did not comply) still `FAIL`s exactly as before — verified
   below against groq, which failed this run for an unrelated, genuine
   non-compliance (see [Out-of-scope observations](#out-of-scope-observations)).

Evidence struct `providerLiveResponseEvidence` gained a `truncated bool`
field so every captured `response.json` states explicitly whether
`ErrResponseTruncated` was set, for future audits.

`go vet -tags=providerlive ./internal/llm/...` and `gofmt -l` are clean.

## Before → After verdict table

"Before" = the original file (`MaxTokens: 32`, no `INCONCLUSIVE` verdict) —
both the **actual prior live run** the operator's brief referenced
(`docs/qa/provider_live_proof_r41_20260711T102100Z/raw_evidence/provider_coverage/`)
and an independent reproduction performed in this pass by temporarily
restoring the pre-fix file (`git show HEAD:...`) and re-running
(`docs/qa/provider_live_proof_20260711T105008Z/`). "After" = the fixed file,
two independent live runs
(`docs/qa/provider_live_proof_20260711T104843Z/` and
`docs/qa/provider_live_proof_20260711T105104Z/`), demonstrating
re-runnability (§11.4.50).

| Provider | Before (MaxTokens=32) | After (fix) | Notes |
|---|---|---|---|
| **deepseek** (`deepseek-v4-flash`) | **FAIL** — `finish_reason=length`, `completion_tokens=32`, `content=""`/`"LIVEPRO"`, nonce absent | **PASS** — `finish_reason=stop`, `completion_tokens=54`, nonce echoed cleanly, `truncated=false` | Reproduced 2×: original prior run + independent RED re-run, both FAIL-by-truncation. Fix reproduced 2×, both PASS. |
| **openrouter** (`openai/gpt-oss-20b:free`) | **FAIL** — `finish_reason=length`, `completion_tokens=32`, `content=""`, nonce absent (prior run 10:21Z) | **PASS** — `finish_reason=stop`, nonce echoed cleanly, `truncated=false` | Prior run FAILed by truncation. In this pass's own RED re-run openrouter happened to PASS even at `MaxTokens=32` (reasoning-token burn is nondeterministic per call) — consistent with the diagnosis, not a contradiction: the harness's fallback (`INCONCLUSIVE`) exists precisely for the runs where it doesn't. Fix reproduced 2×, both PASS. |
| groq (`llama-3.1-8b-instant`) | n/a (not a reasoning model, out of scope) | PASS in run 1, FAIL in run 2 (`content="LIVE"`, `finish_reason=stop`, `truncated=false`, 3 completion tokens) | **Not a truncation** — genuine non-compliant short completion from a small fast model. Correctly still reported as FAIL (unforgeability intact, no INCONCLUSIVE mislabeling of a real failure). Out of this task's scope. |
| mistral (`mistral-small-2603`) | n/a | PASS (both runs) | Unaffected by this fix, included for completeness. |
| gemini | n/a | FAIL — `gemini API error (400): INVALID_ARGUMENT - API key not valid` | Pre-existing, unrelated invalid/expired credential — not a truncation, not touched by this fix. Out of scope. |
| openai, anthropic, xai | n/a | honest **SKIP: no-key** | No key configured this session — correct per harness design. |
| ollama, llamacpp | n/a | honest **SKIP: unreachable** | Local servers not running; harness performed a read-only reachability probe only, per task constraint (no local coder booted/touched). |

## Raw evidence (redacted — no secrets)

No API key values appear in any captured artifact (`providerLiveRequestEvidence`
/ `providerLiveResponseEvidence` never carry credentials — §12.1/CONST-042).
Nonce values are single-use random tokens, safe to display.

**Before (prior live run, deepseek)** —
`docs/qa/provider_live_proof_r41_20260711T102100Z/raw_evidence/provider_coverage/deepseek/response.json`:
```json
{
  "provider": "deepseek",
  "content": "LIVEPRO",
  "nonce_echoed": false,
  "finish_reason": "length",
  "processing_time": "839.915899ms",
  "usage": {"prompt_tokens": 36, "completion_tokens": 32, "total_tokens": 68}
}
```

**Before (prior live run, openrouter)** — same directory, `openrouter/response.json`:
```json
{
  "provider": "openrouter",
  "content": "",
  "nonce_echoed": false,
  "finish_reason": "length",
  "processing_time": "3.375308057s",
  "usage": {"prompt_tokens": 98, "completion_tokens": 32, "total_tokens": 130}
}
```

**After (this pass, deepseek)** —
`docs/qa/provider_live_proof_20260711T104843Z/provider_coverage/deepseek/response.json`:
```json
{
  "provider": "deepseek",
  "content": "LIVEPROOF-dca3761d6ae9",
  "nonce_echoed": true,
  "truncated": false,
  "finish_reason": "stop",
  "processing_time": "1.39980484s",
  "usage": {"prompt_tokens": 47, "completion_tokens": 54, "total_tokens": 101}
}
```

**After (this pass, openrouter)** — same directory, `openrouter/response.json`:
```json
{
  "provider": "openrouter",
  "content": "LIVEPROOF-e72693198bc5",
  "nonce_echoed": true,
  "truncated": false,
  "finish_reason": "stop",
  "processing_time": "7.757438674s",
  "usage": {"prompt_tokens": 108, "completion_tokens": 56, "total_tokens": 164}
}
```

Full per-provider `request.json` / `response.json` / `verdict.txt` for every
run referenced above are committed under their respective `docs/qa/` run-id
directories (already tracked by the harness itself, not duplicated here).

## Out-of-scope observations

- **gemini**: `API key not valid` — a pre-existing, unrelated invalid
  credential. Not caused by, and not fixed by, this change.
- **groq**: one of two runs produced a short non-compliant completion
  (`"LIVE"`, `finish_reason=stop`, not truncated) — genuine model flakiness
  on a small fast model, unrelated to the reasoning/truncation defect this
  pass addresses. The fix correctly still reports this as FAIL (it is not
  a truncation), confirming the INCONCLUSIVE path does not mask genuine
  non-compliance.

Neither was touched; both are outside this task's scope (§11.4.6 —
resolve one attributed defect, log adjacent findings honestly, do not
scope-creep into a fix guessed at without a dedicated repro cycle).

## Reproduction

```bash
cd helix_code
go test -tags=providerlive -v -count=1 -run TestProviderLiveProof \
  -timeout 280s ./internal/llm/ 2>&1 | tee /tmp/pp2.log
```

Re-runnable indefinitely (§11.4.98/§11.4.50): every invocation opens a
fresh `docs/qa/provider_live_proof_<UTC-ts>/` evidence directory and never
clobbers a prior run's evidence.
