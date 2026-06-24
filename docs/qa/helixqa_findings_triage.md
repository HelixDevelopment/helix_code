# HelixQA bank-vs-server FAIL triage (V&V run, server :18080)

| Property | Value |
|---|---|
| Date | 2026-06-24 |
| Server | `http://localhost:18080` (fresh, IDLE) |
| Method | READ-ONLY: curl probes + source read (`internal/server/handlers.go`, `server.go`) + bank read (`submodules/helix_qa/banks/`) |
| Author | RESPAWN subagent (§11.4.147), data-only (§11.4.6/§11.4.120) |

## Captured actual responses

```
GET /api/v1/server/info →
{"info":{"database":{"connected":true,"enabled":true},
 "features":{"auth_enabled":true,"mcp_enabled":true,"notifications_enabled":true},
 "go_version":"1.24","name":"HelixCode Server","redis":{"connected":true,"enabled":true},
 "start_time":"...","uptime":"...","version":"1.0.0"},"status":"success"}

POST /api/v1/auth/refresh (no Authorization header) →
  HTTP 401  {"message":"Authorization header required","status":"error"}

POST /api/v1/auth/login (body `not json{`, Content-Type: application/json) →
  HTTP 400  {"error":"invalid character ... looking for beginning of object key string",
             "message":"Invalid request","status":"error"}
```

## Source facts

`getServerInfo` (`handlers.go:1077-1105`) emits `features` from exactly the three
capabilities wired into the `Server` struct (`server.go`): `auth`, `mcp`,
`notification`. There is NO struct field, NO route, and NO info-flag for
lsp / plugins / skills / ensemble. Streaming HAS a real route+handler
(`/api/v1/llm/stream` → `streamLLM`, `llm_generate.go:283`) but NO info flag.

Package presence in `internal/`: `mcp`✔ `agent`✔ `plugins`✔ ; `lsp`✘ `skills`✘ `ensemble`✘.

## Per-finding verdict

### 1. HXC-API-015 — refresh-without-auth → **BANK-DRIFT**
- Bank (`helixcode-full-qa-api.json:389`): `expect_status:401`, `expect_body_contains:"authorization"` (lowercase).
- Server: 401 ✔ but body says `"Authorization header required"` (capital A). The substring match is case-sensitive → fails on `"authorization"`.
- Server behavior is CORRECT (401 + clear message). The assertion's literal is wrong.
- **Fix (bank):** change `expect_body_contains` to `"Authorization"` (or `"Authorization header required"`).

### 2. HXC-SEC-010 — malformed-JSON login → **BANK-DRIFT**
- Bank (`helixcode-security-validation.json:208`): `expect_status:400`, `expect_body_contains:"invalid_request"`.
- Server: 400 ✔ (correctly NOT 500), body `{"message":"Invalid request",...}`. The token `invalid_request` (snake_case) does not appear; the server uses the human string `Invalid request`.
- Server behavior is CORRECT (the actual security property — 400 not 500 — holds). The assertion expects a snake_case error code the server never emits.
- **Fix (bank):** change `expect_body_contains` to `"Invalid request"`.

### 3. HXC-ENS-001 — ensemble in /server/info → **REAL SERVER GAP**
- Bank (`helixcode-ensemble-members.yaml:72`): `expect_body_contains:"ensemble"` in `$.info`.
- Server: not present. No ensemble field/route; `internal/ensemble` absent (ensemble surfaced via `internal/agent`/verifier, not advertised in info).
- **Fix (server):** add ensemble advertisement to `getServerInfo` `features` (e.g. `ensemble_enabled` / member descriptor) reflecting the real Helix Agent ensemble wiring, OR — if ensemble is genuinely not a server capability — this becomes BANK-DRIFT. Current code: ensemble is NOT a server-info capability, so server-side decision required (advertise it, or move the assertion off /server/info per §11.4.122 don't silently drop).

### 4. HXC-LSP-001 — lsp_enabled flag → **REAL SERVER GAP (conditional)**
- Bank (`helixcode-lsp.yaml:61`): `expect_body_contains:"lsp_enabled"`.
- Server: absent. No `lsp` struct field, no route, `internal/lsp` package absent.
- **Fix:** If LSP is a shipped server capability, add `lsp_enabled` to `features`. If LSP is NOT implemented in the server, the assertion is asserting an unimplemented feature → either implement+advertise, or reclassify the bank item honestly (NOT a metadata-only PASS).

### 5. HXC-PLUGIN-001 — plugins_enabled flag → **REAL SERVER GAP**
- Bank (`helixcode-plugins.yaml:49`): `expect_body_contains:"plugins_enabled"`.
- Server: absent from info. BUT `internal/plugins` package EXISTS — the capability is in the codebase, just not wired into the Server struct or advertised.
- **Fix (server):** wire plugins into `Server` and add `plugins_enabled` to `getServerInfo` `features`. This is a genuine info-exposure gap (feature exists, flag omitted).

### 6. HXC-SKILL-001 — skills_enabled flag → **REAL SERVER GAP (conditional)**
- Bank (`helixcode-skills.yaml:49`): `expect_body_contains:"skills_enabled"`.
- Server: absent. No `skills` field/route; `internal/skills` package absent.
- **Fix:** same shape as LSP — if skills is a server capability, advertise `skills_enabled`; otherwise implement+advertise or reclassify honestly.

### 7. HXC-STREAM-001 — streaming flag → **REAL SERVER GAP (strongest case)**
- Bank (`helixcode-streaming.yaml:57`): `expect_body_contains:"streaming"`.
- Server: streaming IS genuinely wired — real route `POST /api/v1/llm/stream` → `streamLLM` (`llm_generate.go:283`). The capability WORKS but `getServerInfo` omits any streaming flag.
- **Fix (server):** add `streaming_enabled: true` (capability is real and reachable) to `getServerInfo` `features`. Clearest "feature enabled but /server/info omits it" case.

## Summary

| Finding | Verdict | Fix side |
|---|---|---|
| HXC-API-015 | BANK-DRIFT | bank: `"authorization"`→`"Authorization"` |
| HXC-SEC-010 | BANK-DRIFT | bank: `"invalid_request"`→`"Invalid request"` |
| HXC-ENS-001 | REAL GAP (or bank if ensemble not a server capability) | server: advertise ensemble in /server/info |
| HXC-LSP-001 | REAL GAP (conditional on LSP being shipped) | server: add `lsp_enabled` or honestly reclassify |
| HXC-PLUGIN-001 | REAL GAP (pkg exists, flag omitted) | server: wire+advertise `plugins_enabled` |
| HXC-SKILL-001 | REAL GAP (conditional on skills being shipped) | server: add `skills_enabled` or honestly reclassify |
| HXC-STREAM-001 | REAL GAP (capability real, flag omitted) | server: add `streaming_enabled` |

**2 BANK-DRIFT, 5 REAL SERVER /server/info EXPOSURE GAPs.** STREAM and PLUGIN are
unambiguous (capability exists/works, flag omitted). ENS/LSP/SKILL are real gaps
IF the capability is a server feature; LSP/SKILL packages are absent from
`internal/`, so those two need a server-side decision: implement+advertise, or
reclassify the bank item honestly (never a metadata-only PASS).
