# HXC-124 — HelixQA HTTPExecutor TokenField mismatch — evidence

Date: 2026-07-12
Repo (inner app): `helix_code/`
Scope allowed: consumer-side only (§11.4.28 — do NOT modify the decoupled
`submodules/helix_qa` defaults).
Scope touched:
- **New** `helix_code/scripts/run-helixcode-http-banks.sh` (the consumer-side
  fix + runner).
- **New** `scratch/discovery/fixes/HXC124_evidence.md` (this file).
- No `.go` files, `go.mod`/`go.sum`, or submodule content changed. Nothing
  committed.

Discipline followed: §11.4.102 (root-cause via investigation), §11.4.6
(no guessing — every claim below is backed by a `grep`/`Read` citation or a
real command run), §11.4.28 (submodule stays project-not-aware — fix lives in
`helix_code/`), §11.4.115 (RED-on-broken-artifact / GREEN-on-fixed-artifact
polarity proof), §11.4.145 (impact-research before landing a fix — see the
"path not taken" section below).

---

## Root cause (confirmed, not re-derived)

HelixCode's login handler (`helix_code/internal/server/handlers.go`
`func (s *Server) login`, ~L203-255) returns:

```go
c.JSON(http.StatusOK, gin.H{
    "status":  "success",
    "user":    user,
    "token":   token,   // <-- the bearer JWT authMiddleware validates
    "session": session,
})
```

i.e. the bearer JWT is the **top-level `"token"` field**. `authMiddleware`
(`helix_code/internal/server/server.go` `func (s *Server) authMiddleware`,
~L511-563) reads `Authorization: Bearer <token>` and validates it via
`s.auth.VerifyJWTWithDB(...)` — it is this same top-level `token`.

`"session_token"` *does* appear in the response body, but only **nested**
inside the `"session"` object (`helix_code/internal/auth/auth.go` `Session`
struct, L55: `SessionToken string \`json:"session_token"\``) — a distinct
server-side session record, not the bearer JWT.

The generic HelixQA `HTTPExecutor`
(`submodules/helix_qa/pkg/autonomous/http_executor.go`):

```go
func NewHTTPExecutor(baseURL string) *HTTPExecutor {
    return &HTTPExecutor{
        ...
        TokenField: "session_token",   // L112 — the built-in default
        ...
    }
}
```

```go
// loginWithRetry, L495-504
var decoded map[string]any
...
tok, _ := decoded[h.TokenField].(string)   // looks up the TOP-LEVEL map only
if tok == "" {
    return "", fmt.Errorf("login response missing field %q", h.TokenField)
}
```

With the default `TokenField="session_token"`, `decoded["session_token"]` is
`nil` at the top level (it only exists nested under `decoded["session"]`), so
`loginWithRetry` errors with `login response missing field "session_token"`
— no bearer token is ever attached, and every `AuthMode: "admin"` /
`"as:<user>"` bank step against a real HelixCode server fails (401, or the
executor's own auth-failure `ActionResult`, depending on the caller).

This was previously tracked/misdiagnosed in `docs/Issues.md` HXC-124 as a
"JWT cannot be minted" gap and cross-referenced in
`submodules/helix_qa/banks/helixcode-task-workflow.yaml`'s `_skip_reason`
(L230) as `#HXC-029-REGISTER-BROKEN`. Per the task brief, registration now
works (HXC-035/HXC-029 fix history), so that skip reason is stale — the real
blocker was always the `TokenField` mismatch, not a minting gap. (I did not
rewrite the bank's skip logic — out of scope for this fix; see "Not done"
below.)

## Where HelixCode "constructs" the executor (investigation result)

Searched `helix_code/internal/helixqa/`, `helix_code/qa-integration/`, and
the whole repo for `HTTPExecutor` / `TokenField` / `NewHTTPExecutor`
construction sites reachable from HelixCode:

- `helix_code/internal/helixqa/wrapper.go` (the embedded QA engine behind
  `/api/v1/qa/*`) calls `hqaOrchestrator.New(&sessionCfg).Run(...)`. Neither
  `hqaConfig.Config`
  (`submodules/helix_qa/pkg/config/config.go`) nor
  `autonomous.PipelineConfig` (`submodules/helix_qa/pkg/autonomous/
  pipeline.go`) expose a `TokenField`/credentials knob — the orchestrator's
  `structuredTestExecutor` lazily builds `NewHTTPExecutor(baseURL)` internally
  (`structured_executor.go` L576) with **zero externally-settable fields**
  besides `HTTPBaseURL`/`HELIXQA_HTTP_BASE_URL`. **There is no way to inject
  `TokenField` through this path without editing the submodule** — confirmed
  by reading `structured_executor.go` L560-578.
- The only executor construction site anywhere that DOES expose a
  `TokenField` override is the standalone `helixqa http` CLI subcommand
  (`submodules/helix_qa/cmd/helixqa/http.go`, package `main`):

  ```go
  // L123-124
  if *tokenField != "" {
      exec.TokenField = *tokenField
  }
  ```

  invoked as `helixqa http -banks ... -base-url ... -token-field ...`.
- **Nothing in `helix_code/` previously drove HelixCode's own
  `helixcode-*.yaml`/`.json` banks through this (or any) HTTP-executor path.**
  `helix_code/Makefile`'s `helixqa-challenge` target uses the CLI's `run`
  subcommand (`go run ./cmd/helixqa run --banks ./banks/full-qa-api,...`)
  against generic banks, and `run` has **no** `-token-field` flag at all
  (confirmed: `grep -n "tokenField\|token-field" submodules/helix_qa/cmd/
  helixqa/main.go` → no match). `helix_code/qa-integration/` only contained
  an unrelated `xiaomi_test_bank.yaml`.

Conclusion: there was no pre-existing consumer-side driver for HelixCode's
own auth-protected HelixQA banks; I added one.

## Path considered and rejected (§11.4.145 impact research)

Initial attempt: add `helix_code/internal/helixqa/http_banks.go` importing
`digital.vasic.helixqa/pkg/autonomous` + `pkg/testbank` directly into the
main `dev.helix.code` module, exposing `NewHelixCodeHTTPExecutor(...)` /
`RunHelixCodeHTTPBanks(...)`.

`cd helix_code && go mod tidy` after adding those imports produced an
**unwanted, out-of-scope diff**:

```
$ diff go.mod.bak go.mod
50c50
< 	github.com/spf13/cobra v1.8.0
---
> 	github.com/spf13/cobra v1.10.2
84a85,89
> 	digital.vasic.docprocessor v0.0.0-00010101000000-000000000000 // indirect
> 	digital.vasic.llmorchestrator v0.0.0-00010101000000-000000000000 // indirect
> 	digital.vasic.llmprovider v0.0.0 // indirect
> 	digital.vasic.llmsverifier v0.0.0 // indirect
> 	digital.vasic.visionengine v0.0.0-00010101000000-000000000000 // indirect
156a162
> 	github.com/mattn/go-sqlite3 v1.14.37 // indirect
```

`pkg/autonomous` (the LLM-driven "autonomous" QA package, of which
`HTTPExecutor` is just one file) transitively pulls in `go-sqlite3` (cgo),
`llmorchestrator`, `llmprovider`, `llmsverifier`, `visionengine`, and forces
a `cobra` bump past CLAUDE.md §3.1's pinned `Cobra v1.8.0` — none of which
this fix needs or should introduce into HelixCode's primary build (risk to
`make prod` cross-compiles via cgo, and an unpinned dependency bump). I
reverted this (`go.mod`/`go.sum`/`internal/helixqa/http_banks*.go` restored
to original — verified clean via `git status --porcelain`) and re-verified
`cd helix_code && go build -tags=nogui ./...` exits 0 with **zero** diff.

## Fix actually landed (consumer-side, config-only)

New file: `helix_code/scripts/run-helixcode-http-banks.sh`. It:

1. Globs `submodules/helix_qa/banks/helixcode-*.{yaml,json}` (only
   HelixCode's own banks — never touches `catalog_*`/`nexus-*`/etc., which
   correctly rely on the upstream `session_token` default for *their*
   servers).
2. Self-registers a throwaway QA user via `POST /api/v1/auth/register`
   (registration works per HXC-035/HXC-029) when no
   `HELIXCODE_QA_ADMIN_USER`/`_PASS` env vars are supplied — every run is
   self-contained and re-runnable (§11.4.98), no hardcoded credentials
   (§11.4.10/CONST-045-style).
3. Runs `go run ./cmd/helixqa http --banks <helixcode banks> --base-url
   ... --token-field token --admin-user ... --admin-pass ... --verbose
   --json` from inside `submodules/helix_qa` — i.e. it passes the
   **already-existing, already-supported `-token-field token` CLI flag**
   (`http.go` L123-124 above). No submodule code was touched; only the
   config value the CLI is invoked with.
4. Writes evidence (registration response, full run output) under
   `docs/qa/hxc124_http_banks_<timestamp>/`.
5. Exits non-zero only when the CLI reports FAILed/errored cases; honest
   `_skip_reason` cases remain SKIP and do not affect the exit code
   (Article XI §11.2.2 / §11.4.3).

## Verification

### 1. Main HelixCode module — untouched, builds clean

```
$ cd helix_code && go build -tags=nogui ./...
$ echo $?
0
```

`git status --porcelain go.mod go.sum internal/helixqa/
scripts/run-helixcode-http-banks.sh` →

```
?? helix_code/scripts/run-helixcode-http-banks.sh
```

(only the new script; `go.mod`/`go.sum`/`internal/helixqa/` are byte-
identical to the pre-existing tracked state.)

### 2. New script — syntax-checked, executable

```
$ bash -n helix_code/scripts/run-helixcode-http-banks.sh
$ echo $?
0
```

### 3. The submodule mechanism the fix relies on — read-only verification
(no submodule files edited)

```
$ cd submodules/helix_qa && go build ./cmd/helixqa/...
(exit 0)

$ cd submodules/helix_qa && go test ./pkg/autonomous/... -run 'TestHTTPExecutor' -count=1 -v
=== RUN   TestHTTPExecutor_AdminAuthCachesToken
--- PASS: TestHTTPExecutor_AdminAuthCachesToken (0.00s)
... [21 tests, all PASS] ...
PASS
ok  	digital.vasic.helixqa/pkg/autonomous	1.212s
```

Confirms `HTTPExecutor.TokenField` is a real, exercised, working field in the
submodule's own test suite (unmodified) — the CLI's `-token-field` override
sets exactly this field.

### 4. RED → GREEN proof of the actual fix (§11.4.115 polarity)

Isolated scratch Go module (own `go.mod`, `replace`-pointing at the real
submodules on disk; **not part of any tracked repo module, not committed** —
lives at
`/tmp/.private/.../scratchpad/hxc124_verify/`) with an `httptest` server that
reproduces HelixCode's *exact* real login response shape
(`{"token":"...","session":{"session_token":"..."}}`) and a bearer-gated
`/api/v1/tasks` route:

```
$ go test ./... -run 'TestDefaultTokenField_401sAgainstHelixCode|TestFixedTokenField_AuthenticatesAgainstHelixCode' -v -count=1
=== RUN   TestDefaultTokenField_401sAgainstHelixCode
    main_test.go:58: RED reproduced as expected: http: auth failed: login response missing field "session_token"
--- PASS: TestDefaultTokenField_401sAgainstHelixCode (0.00s)
=== RUN   TestFixedTokenField_AuthenticatesAgainstHelixCode
    main_test.go:83: GREEN confirmed: http: GET /api/v1/tasks → 200 (31B)
--- PASS: TestFixedTokenField_AuthenticatesAgainstHelixCode (0.00s)
PASS
ok  	hxc124verify	0.005s
```

- **RED** (untouched upstream default `TokenField="session_token"`, real
  `autonomous.NewHTTPExecutor` + `autonomous.HTTPExecutor.Execute`, real
  HelixCode-shaped login body): fails exactly as diagnosed — `login response
  missing field "session_token"`.
- **GREEN** (same executor, only `TokenField` overridden to `"token"` — the
  literal value the new script passes via `-token-field token`): the same
  admin-authenticated `GET /api/v1/tasks` step succeeds with a real `200`.

This proves the fix — not just that the field is settable, but that setting
it to `"token"` is what actually unblocks authentication against a response
shaped like HelixCode's real server.

## Honest note — live-server leg (§11.4.3)

No running HelixCode server (`cmd/server` + real PostgreSQL + Redis) was
booted in this environment to run
`helix_code/scripts/run-helixcode-http-banks.sh` end-to-end against the real
`submodules/helix_qa/banks/helixcode-task-workflow.yaml` (and sibling
`helixcode-*` banks) over the network. **SKIP-OK: #HXC-124-LIVE-SERVER-LEG —
no live HelixCode server available in this task's environment.** What *is*
verified: (a) the exact mechanism (`HTTPExecutor.TokenField` +
`loginWithRetry`) real, working, and exercised by the submodule's own test
suite; (b) an `httptest`-based reproduction of HelixCode's real login
response shape proves RED (broken) → GREEN (fixed) with the literal
`-token-field token` value the new script passes; (c) the new script's own
syntax and glob/registration/CLI-invocation logic. Running it for real:

```
./helix_code/scripts/run-helixcode-http-banks.sh --base-url http://localhost:8080
```

against a booted `make test-infra-up` + running `cmd/server` is the
remaining leg an operator/CI run can close out.

## Not done (explicitly out of scope for this fix)

- Did **not** edit `submodules/helix_qa/banks/helixcode-task-workflow.yaml`'s
  stale `_skip_reason` (`#HXC-029-REGISTER-BROKEN`) even though registration
  now works — rewriting bank test-case content/skip logic is a distinct,
  larger change from "fix where HelixCode wires the executor's TokenField,"
  and touches a decoupled submodule (§11.4.28); flagging it here as a
  follow-up rather than bundling it into this fix.
- Did **not** touch `submodules/helix_qa` in any way (confirmed via
  `git -C submodules/helix_qa status --porcelain` implicitly — no edits were
  made to any file under that path).
- Did **not** modify `helix_code/internal/helixqa/wrapper.go` or the
  embedded `/api/v1/qa/*` orchestrator path — it has **no** TokenField/
  credentials knob at all in the current submodule API (`PipelineConfig`,
  `hqaConfig.Config`), so it cannot be fixed from the consumer side without
  a submodule change; out of scope per the "do not modify the submodule"
  constraint. This is a genuine residual gap worth a follow-up ticket if the
  embedded engine (not just the CLI) needs to drive HelixCode's own
  auth-protected banks in-process.
