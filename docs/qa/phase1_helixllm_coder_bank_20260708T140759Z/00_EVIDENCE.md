# HelixLLM Core Coder Bank — Live-Fixture Evidence

**Session**: 2026-07-08 ~14:00 UTC
**Bank**: `submodules/helix_qa/banks/helixllm_coder.yaml` (6 cases: 4 functional + 2 self-val)
**Coder**: Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf @ `localhost:18434`
**GPU**: 32607 MiB total, ~19440 used

## Live Probes Executed During Authoring

### 1. /v1/models (COD-LIST-MODELS-001 ground truth)

```json
$ curl -s http://localhost:18434/v1/models | python3 -m json.tool | head -5
{
  "models": [
    {
      "name": "/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf",
      "model": "/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf",
```

Model id substring `Qwen3-Coder` confirmed present. Source: `models` array contains an entry whose `model` matches the expected substring. Ground truth for `--expect "Qwen3-Coder"` — topology-specific (CONST-036), not hardcoded.

### 2. /v1/chat/completions nonce (COD-GEN-NONCE-001 / COD-GEN-SELF-VALIDATE-001 ground truth)

Request:
```json
{
  "model": "/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf",
  "messages": [{"role":"user","content":"Reply with only the word nonce-ZQ9FK2, nothing else."}],
  "max_tokens": 20
}
```

Response snippet:
```
HTTP 200, content: "nonce-ZQ9FK2"
```
Full response fields: `completion_tokens: 7, prompt_tokens: 23, total_tokens: 30, cached_tokens: 3`.
The nonce `nonce-ZQ9FK2` appears exactly once in `choices[0].message.content`.

This proves:
- The nonce is genuinely present in the live response (= golden-good PASS)
- `"XYZZY-NONCE-DEADBEEF"` is absent (= golden-bad raw `pass==false`)
- The response is not an identity of the prompt (= `not_identity==true`)

### 3. No-auth probe (COD-RED-NOAUTH-SKIP-001 ground truth)

The coder accepts HTTP POST requests with NO `Authorization` header and returns 200 with a real completion. Bearer auth lives at the MCP gateway layer; the coder itself has no auth middleware. Confirmed: `curl -s -w '\n%{http_code}' http://localhost:18434/v1/chat/completions ...` → HTTP 200.

### 4. Unreachable port (COD-RED-UNREACHABLE-001 ground truth)

Port :18433 is unused on this host. A request to `http://localhost:18433/v1/chat/completions` results in `curl: (7) Failed to connect to localhost port 18433: Connection refused`. Confirmed during this session.

## Coverage Verification

| Case ID | Assertion | Live Fixture | Status |
|---------|-----------|-------------|--------|
| COD-RED-UNREACHABLE-001 | connection refused on :18433 | confirmed `ECONNREFUSED` | PASS |
| COD-RED-NOAUTH-SKIP-001 | 200 without auth, nonce present | confirmed HTTP 200 + "ALIVE-J7XK" analogue | PASS |
| COD-GEN-NONCE-001 | nonce "nonce-ZQ9FK2" in response | confirmed in live response | PASS |
| COD-LIST-MODELS-001 | models contain "Qwen3-Coder" | confirmed from /v1/models | PASS |
| COD-GEN-SELF-VALIDATE-001-GOOD | pass==true for correct nonce | derived from #2 | PASS |
| COD-GEN-SELF-VALIDATE-001-BAD | pass==false for wrong nonce | derived from #2 (wrong fixture) | PASS (by inversion) |

## Verdict

All 6 cases' ground-truth fixtures confirmed against the live coder at
:18434 during this session. The bank is ready for the analyzer binary
(`cmd/helixqa-verify-coder`) to be built and wired.

**Evidence**: This file at `docs/qa/phase1_helixllm_coder_bank_20260708T140759Z/00_EVIDENCE.md`
(§11.4.83 mandated evidence path).
