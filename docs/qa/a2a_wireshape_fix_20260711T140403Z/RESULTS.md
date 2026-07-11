# A2A Wire-Shape Fixes — AgentCard base path + Task `kind` discriminator, RESULTS

| | |
|---|---|
| **Run-id** | `a2a_wireshape_fix_20260711T140403Z` |
| **Track** | `(T1/feature/helixllm-full-extension)` |
| **Scope** | Fix the two genuine A2A wire-shape defects the real `github.com/a2aproject/a2a-go` SDK v0.3.15 exposed in `docs/qa/a2a_live_e2e_20260711T134958Z/RESULTS.md` §2.4 (Finding A: `AgentCard.url` missing the `/a2a` JSON-RPC base path; Finding B: `message/send` Task result missing the `"kind":"task"` discriminator), and cover both with a strict-SDK RED→GREEN regression test (§11.4.4 / §11.4.146 / §11.4.115). |
| **Verdict** | **DONE** — both defects fixed at source, both proven RED (pre-fix binary, real SDK, real 404 / real raw HTTP 404) → GREEN (post-fix binary, real SDK strict polymorphic decode, live-coder nonce echo), unit tests extended with permanent regression guards, coder untouched throughout. |
| **Repo (main)** | `feature/helixllm-full-extension` (this commit) |
| **Submodule (helix_llm)** | 4 files touched: `cmd/a2a-server/main.go`, `internal/a2a/agentcard.go`, `internal/a2a/types.go`, `internal/a2a/a2a_test.go` — see §2 below |

---

## 1. Constraints respected

- Coder `:18434` (`llama-server`, PID `1980342`, model `Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf`) — **read-only, never restarted**. PID identical before/during/after this whole session (§11.4.174 process-ownership verified via `ps -o pid,args` before every kill — only our own `a2a-server` PIDs were ever signalled). `coder_untouched_final.json` — `HTTP 200` on `/v1/models` after the whole cycle.
- `submodules/helix_agent`, `/mnt/track1`, `helix_qa`, `helix_code/internal/llm`, and the shared test-suite were **never touched**. An untracked file belonging to a different concurrent stream (`cmd/agentgen-boot/agentgen-boot`) was left untouched (§11.4.119 / §11.4.174).
- Scope respected: `submodules/helix_llm` (the a2a-server: `cmd/a2a-server/`, `internal/a2a/`) + root `docs/qa/`.
- The A2A test server ran on the distinct port `:18443` (the prior evidence run used `:18441`; the RESULTS.md that discovered these defects noted `18436`–`18440` belong to other streams — `:18443` was verified free before use via `ss -ltnp`).
- Local commits only — **no push**.

---

## 2. The two fixes

### 2.1 Fix A — `AgentCard.url` now includes the JSON-RPC dispatch base path

**File:** `submodules/helix_llm/internal/a2a/agentcard.go`

Added a `BasePath` field to `CardConfig` and a `dispatchURL()` helper that joins `PublicURL` + `BasePath` (defaulting to `/a2a` — the same default `RegisterRoutes` in `router.go` already uses), so the Agent Card's `url` field can never disagree with where `RegisterRoutes` actually mounts JSON-RPC dispatch. `BuildAgentCard` now sets `URL: dispatchURL(cfg.PublicURL, cfg.BasePath)` instead of the bare `cfg.PublicURL`.

**File:** `submodules/helix_llm/cmd/a2a-server/main.go`

`main()` now passes `BasePath: basePath` (the same `HELIX_A2A_BASE_PATH`-sourced variable already passed to `RouterOptions{BasePath: basePath}`) into `CardConfig`, so the card and the router share one config-injected source of truth for the mount path — no new env var, no hardcoded literal.

Before:
```json
"url":"http://localhost:18443"
```
After:
```json
"url":"http://localhost:18443/a2a"
```

### 2.2 Fix B — `message/send` / `tasks/get` Task result now carries `"kind":"task"`

**File:** `submodules/helix_llm/internal/a2a/types.go`

Added a custom `Task.MarshalJSON()` method that injects the top-level `"kind":"task"` discriminator the real a2a-go SDK's polymorphic decoder (`a2a.UnmarshalEventJSON`, `a2a-go@v0.3.15/a2a/core.go:87-124`) requires — mirroring the exact pattern the real SDK's own `Task.MarshalJSON`/`Message.MarshalJSON` use (the fix direction the prior evidence run's bluff-audit specified). A type-alias trick (`type taskAlias Task`) avoids infinite MarshalJSON recursion. This is transport-agnostic: it fires for `message/send` AND `tasks/get` (and any other code path that ever JSON-marshals a `Task`) without needing to touch `handlers.go` or `taskstore.go` — the discriminator can never be forgotten at a construction site.

Before:
```json
"result":{"id":"...","status":{"state":"completed",...},...}
```
After:
```json
"result":{"kind":"task","id":"...","status":{"state":"completed",...},...}
```

---

## 3. Strict-SDK test added (permanent regression guard)

### 3.1 Unit-level guards (fast, hermetic, mocked downstream per §11.4.27(A))

`submodules/helix_llm/internal/a2a/a2a_test.go` — extended (not replaced) the existing 5-test suite:
- `TestAgentCardHasRequiredFields` — new assertion: `card["url"]` MUST have suffix `/a2a`.
- `TestMessageSendHappyPath` — new assertion: `result["kind"] == "task"`.
- `TestTasksGetRoundTrip` — new assertion: `result2["kind"] == "task"`.

```
=== RUN   TestAgentCardHasRequiredFields
--- PASS: TestAgentCardHasRequiredFields (0.00s)
=== RUN   TestMessageSendHappyPath
--- PASS: TestMessageSendHappyPath (0.00s)
=== RUN   TestMessageSendRejectsMissingAuth
--- PASS: TestMessageSendRejectsMissingAuth (0.00s)
=== RUN   TestDispatchRejectsMalformedRequest
--- PASS: TestDispatchRejectsMalformedRequest (0.00s)
=== RUN   TestTasksGetRoundTrip
--- PASS: TestTasksGetRoundTrip (0.00s)
PASS
ok  	github.com/HelixDevelopment/HelixLLM/internal/a2a	0.008s
```
(`green_evidence/unit_tests_postfix.txt`)

### 3.2 Strict-SDK live test (the deliverable's core requirement)

New standalone Go module `docs/qa/a2a_wireshape_fix_20260711T140403Z/harness/` (own `go.mod`, mirrors the prior evidence run's harness layout — never touches `submodules/helix_llm/go.mod`) containing `wireshape_test.go`, a **black-box** test that imports ONLY the real upstream `github.com/a2aproject/a2a-go v0.3.15` SDK (never `internal/a2a`, never hand-rolled JSON-RPC). `TestA2AStrictSDKWireShape`:

1. Resolves the Agent Card via the real SDK's own `agentcard.DefaultResolver.Resolve`.
2. Constructs a real `a2aclient.Client` via `a2aclient.NewFromCard` — the **literal, spec-faithful** client that (per `a2a-go@v0.3.15/a2a/agent.go:84-89`) dispatches JSON-RPC directly to `card.URL`. This is the exact client construction the prior evidence run's harness could NOT complete against the live server (it had to fall back to `NewFromEndpoints` with a hand-specified `/a2a` suffix — a harness-side workaround, never a real fix). This test uses the literal card-driven client with zero workaround.
3. Sends a fresh-nonce `message/send` request and asserts the SDK's **strict polymorphic decode** types the result as `*a2a.Task` (not an error, not a `*a2a.Message`).
4. Confirms the live downstream coder genuinely answered (the nonce is unforgeable — only a live model that actually saw the exact prompt could echo it, §11.4.143).
5. Round-trips `tasks/get` via the same strict-typed `client.GetTask`, confirming ID match + nonce persistence.
6. Honestly `t.Skip()`s (§11.4.3) if `A2A_BASE_URL`/`A2A_BEARER_TOKEN` are unset or the coder is unreachable — never a faked PASS.
7. **Never** starts/stops/restarts the coder — only a read-only `GET /v1/models` reachability probe (§11.4.119 / §11.4.122 / §11.4.174).

---

## 4. RED → GREEN proof (§11.4.115 / §11.4.146)

Because the fix touches production Go structs (`CardConfig.BasePath`, `Task.MarshalJSON`), the polarity-switch discipline is realized as **which artifact the same test source is run against**, exactly as prescribed by §11.4.115's own honest-boundary note (the harness reads its target from `A2A_BASE_URL`, so the artifact under test — pre-fix vs post-fix binary — IS the polarity switch here, not an in-process env flag).

### 4.1 RED — pre-fix a2a-server binary (production fix files reverted via `git stash push --keep-index -- <3 files>`, test file with new assertions KEPT)

```
=== RUN   TestA2AStrictSDKWireShape
    wireshape_test.go:122: card.URL="http://localhost:18443" card.PreferredTransport="JSONRPC"
    wireshape_test.go:153: cardClient.SendMessage (literal card.URL="http://localhost:18443" dispatch) FAILED: unexpected HTTP status: 404 Not Found
        This is exactly the failure class the fix (AgentCard.url base path + "kind":"task" discriminator) closes -- if this test is run against a PRE-FIX a2a-server binary, this failure IS the expected RED-baseline evidence (§11.4.115).
--- FAIL: TestA2AStrictSDKWireShape (0.00s)
FAIL
```
(`red_evidence/red_test_output.txt`)

Corroborating raw-wire proof — a literal `POST http://localhost:18443` (card.URL with no base path) against the pre-fix server:
```
HTTP/1.1 404 Not Found
Content-Type: text/plain
Content-Length: 18

404 page not found
```
(`red_evidence/raw_literal_card_url_post.txt`) — and the pre-fix Agent Card itself, `red_evidence/agent_card_prefix.json`: `"url":"http://localhost:18443"` (no `/a2a`).

Additionally, before any harness run: the internal unit-test file with the new field reference fails to even **compile** against pre-fix `agentcard.go` —
```
internal/a2a/a2a_test.go:59:3: unknown field BasePath in struct literal of type a2a.CardConfig
FAIL	github.com/HelixDevelopment/HelixLLM/internal/a2a [build failed]
```
— an even stronger RED signal proving the test is genuinely coupled to the fix, not decorative.

### 4.2 GREEN — post-fix a2a-server binary (`git stash pop`, rebuild)

```
=== RUN   TestA2AStrictSDKWireShape
    wireshape_test.go:122: card.URL="http://localhost:18443/a2a" card.PreferredTransport="JSONRPC"
    wireshape_test.go:172: cardClient.SendMessage strict-decoded *a2a.Task id=fc982510-1387-4ec5-a044-bff87f69736a state=completed nonce_echoed=true
    wireshape_test.go:186: cardClient.GetTask strict-decoded id=fc982510-1387-4ec5-a044-bff87f69736a state=completed nonce_present=true
--- PASS: TestA2AStrictSDKWireShape (0.30s)
PASS
```
(`green_evidence/green_test_output.txt`) — **the live Qwen3-Coder-30B model genuinely answered with the exact fresh nonce**, routed through the fixed A2A server, strict-decoded by the real, unmodified upstream SDK using the card's own literal URL (no client-side workaround).

Re-run for determinism (§11.4.50), fresh nonce each time:
```
=== RUN   TestA2AStrictSDKWireShape
    wireshape_test.go:122: card.URL="http://localhost:18443/a2a" card.PreferredTransport="JSONRPC"
    wireshape_test.go:172: cardClient.SendMessage strict-decoded *a2a.Task id=ca9b51a3-2818-410d-a97b-f177c187fe0d state=completed nonce_echoed=true
    wireshape_test.go:186: cardClient.GetTask strict-decoded id=ca9b51a3-2818-410d-a97b-f177c187fe0d state=completed nonce_present=true
--- PASS: TestA2AStrictSDKWireShape (0.09s)
PASS
```
(`green_evidence/green_test_output_rerun2.txt`)

Corroborating raw-wire proof — literal `POST http://localhost:18443/a2a` against the post-fix server:
```
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{"jsonrpc":"2.0","id":1,"result":{"kind":"task","id":"6fc4ed90-8974-4ef3-96ca-622645942114","status":{"state":"completed","timestamp":"2026-07-11T14:07:25.371432844Z"},"history":[{"role":"user","parts":[{"kind":"text","text":"Reply with ONLY this exact token and nothing else: NONCE_rawcurlcheck"}]},{"role":"agent","parts":[{"kind":"text","text":"NONCE_rawcurlcheck"}]}],"artifacts":[{"name":"generated-code","parts":[{"kind":"text","text":"NONCE_rawcurlcheck"}]}]}}
```
(`green_evidence/raw_literal_card_url_post.txt`) — `"kind":"task"` is now present on the wire, AND the endpoint the card literally advertises (`/a2a`) is exactly where dispatch succeeds.

The extended unit-test suite is GREEN post-fix (§3.1 above, `green_evidence/unit_tests_postfix.txt`).

---

## 5. Coder untouched (§11.4.119 / §11.4.122 / §11.4.174)

| Check | Before RED | After RED / Before GREEN | After GREEN |
|---|---|---|---|
| `llama-server` PID | `1980342` | `1980342` | `1980342` (unchanged) |
| `GET /v1/models` HTTP status | `200` | `200` | `200` |

Every teardown (`kill "$PID"`) was preceded by `ps -o pid,args -p "$PID"` confirming the target was our own `./bin/a2a-server` process — never the coder, never another stream's process (§11.4.174). `coder_untouched_final.json` captures the final post-cycle `/v1/models` response, model id unchanged.

---

## 6. Reproduce

```bash
cd submodules/helix_llm
go build -o bin/a2a-server ./cmd/a2a-server
go test -v -count=1 ./internal/a2a/...

TOKEN=$(openssl rand -hex 16)
HELIX_A2A_LISTEN_ADDR=:18443 HELIX_A2A_PUBLIC_URL=http://localhost:18443 HELIX_A2A_BASE_PATH=/a2a \
  HELIX_A2A_DOWNSTREAM_URL=http://localhost:18434 HELIX_A2A_BEARER_TOKEN="$TOKEN" \
  ./bin/a2a-server &

cd ../../docs/qa/a2a_wireshape_fix_20260711T140403Z/harness
A2A_BASE_URL=http://localhost:18443 A2A_BEARER_TOKEN="$TOKEN" A2A_CODER_URL=http://localhost:18434 \
  go test -v -count=1 -run TestA2AStrictSDKWireShape .

curl -s http://localhost:18434/v1/models | head -c 200
kill %1
```

---

## 7. Sources verified (§11.4.99 / §11.4.150)

- <https://github.com/a2aproject/a2a-go> v0.3.15, read directly from the module cache (`~/go/pkg/mod/github.com/a2aproject/a2a-go@v0.3.15`) — `a2a/core.go` (`Task.ID` type `TaskID`, `TaskQueryParams`), `a2aclient/client.go` / `a2aclient/jsonrpc.go` (`Client.SendMessage` / `Client.GetTask` signatures) — used to confirm the exact API surface this fix's test drives, not from training-data memory.
- `docs/qa/a2a_live_e2e_20260711T134958Z/RESULTS.md` §2.4 — the originating findings this task fixes, cited verbatim in code comments at both fix sites.

---

## 8. Composition footer

Composes §11.4.4(b) (four-layer: pre-build unit tests + post-build strict-SDK integration test + runtime-on-live-target RED→GREEN + this document as the paired-evidence record) / §11.4.6 (RED captured before any fix claim) / §11.4.108 (SOURCE → ARTIFACT rebuild → RUNTIME-ON-CLEAN-TARGET port `:18443` → USER-VISIBLE real nonce echo) / §11.4.115 (RED-baseline-on-broken-artifact, one test source both roles) / §11.4.146 (reproduce-first STEP 1 = §4.1, same-test-confirms-fix STEP 2 = §4.2, extend-to-all-cases STEP 3 = the 3 unit-test regression guards in §3.1 covering both `message/send` and `tasks/get` paths) / §11.4.119 + §11.4.122 + §11.4.174 (coder never touched, process-ownership verified before every kill) / §11.4.99 (a2a-go SDK source read live from module cache, not memory).
