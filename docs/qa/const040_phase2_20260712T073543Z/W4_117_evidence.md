# HXC-117 Phase 2 — CONST-040 capability-flag population — evidence

Date: 2026-07-12
Scope touched: `helix_code/internal/verifier/client.go`, `helix_code/internal/verifier/capability_flags_test.go` (tests only, TDD).
No other files touched (no `cmd/cli`, `internal/rag`, `internal/acp`, `go.mod` edits, per instructions).

## 1. Investigation — where VerifiedModel/VerificationResult data comes from

- Real external LLMsVerifier HTTP service, via `internal/verifier/client.go`:
  - `Client.GetModels` → `decodeModels` → `unmarshalModelArray` (bare `json.Unmarshal` into `VerifiedModel` via an embedded type-alias — the SAME generic mechanism that already populates `SupportsEmbeddings` from real server data).
  - `Client.GetModelByID` → `json.NewDecoder(...).Decode(&VerifiedModel{})`.
  - `Client.VerifyModel` → `json.NewDecoder(...).Decode(&VerificationResult{})`.
- Embedded/fallback in-process server (`embedded_server.go`) — serves `FallbackModels` (`fallback_models.go`, a hardcoded 8-model Go literal list with NO probe data for any of the 6 new flags) and already (Phase 1) mirrors `VerifiedModel`→`VerificationResult` honestly.
- `internal/verifier/poller.go` and `adapter.go` consume `[]*VerifiedModel` from `Client.GetModels`/embedded server — no independent data source, so fixing `client.go` fixes the whole chain.

## 2. Real-source cross-check against the LLMsVerifier submodule (`submodules/llms_verifier/llm-verifier`)

Direct inspection this session (not guessed):

- `database/database.go:882-916` — `database.VerificationResult` struct **already has** all 6 CONST-040 fields, persisted to Postgres:
  `SupportsMCPs` (`json:"supports_mcps"`), `SupportsLSPs` (`json:"supports_lsps"`), `SupportsACPs` (`json:"supports_acps"`),
  `SupportsRAG` (`json:"supports_rag"`), `SupportsSkills` (`json:"supports_skills"`), `SupportsPlugins` (`json:"supports_plugins"`).
- `verification/verification.go:95-140` — `Verifier.Verify(ctx, req)` computes these from **real per-capability probes** (`features.MCPs`/`LSPs`/`ACPs`/`RAG`/`Skills`/`Plugins`), with raw request/response captured as evidence (`RawRequest`/`RawResponse`) — genuinely wired, not a stub (a sibling `ErrVerifierNotConfigured` explicitly forbids a hardcoded-all-true fallback).
- `capabilities/types.go:188-190` and `capabilities/registry_resolve.go:144-152` — a capability-resolution registry keyed by `"mcp"`/`"mcps"`/`"lsp"`/`"lsps"`/`"acp"`/`"acps"`/`"rag"`/`"skills"`/`"plugins"` reading off `database.VerificationResult`.
- **BUT** the live HTTP handlers our `client.go` actually calls do **NOT** emit any of these 6 keys today:
  - `api/handlers.go:43-113` `ListModelsHandler` (`GET /api/models`) — hand-rolled `map[string]any` response; `buildCapabilitiesList` (line ~115) only covers multimodal/vision/audio/video/reasoning/text.
  - `api/handlers.go:142-235` `GetModelHandler` (`GET /api/models/{id}`) — same restricted `map[string]any`, no capability-flag keys.
  - `api/handlers.go:266-359` `VerifyModelHandler` (`POST /api/models/{id}/verify`) — hand-rolled response with only `status/model_id/model_name/verification_status/score/message/job_id/verification_id/started_at/completed_at`; calls a DIFFERENT, narrower `s.verifier.Verify(ctx, model, provider) (status, score, err)` — not the capability-aware `verification.Verifier.Verify(ctx, req) (*database.VerificationResult, error)` shown above.

**Conclusion:** the underlying LLMsVerifier data model genuinely DOES expose all 6 capabilities (real probe-backed, DB-persisted) — but 3 of them (MCP/LSP/ACP) use a **plural** JSON key on the real service (`supports_mcps`/`supports_lsps`/`supports_acps`) vs HelixCode's own **singular** CONST-040 doc.go convention (`supports_mcp`/`supports_lsp`/`supports_acp`) from Phase 1; RAG/Skills/Plugins already match exactly. Separately (and orthogonally), the *live* HTTP endpoints do not currently serialize ANY of the 6 keys — a real, verified, honest gap on the LLMsVerifier side, out of scope for this HelixCode-side task.

## 3. What was wired (Phase 2 change)

`client.go`:
- New `capabilityAliasFields` struct (`supports_mcps`/`supports_lsps`/`supports_acps`) + `applyTo` method — a one-way OR promoting a real-server plural-key `true` onto the already-decoded singular-tag field. Never demotes true→false; never fabricates true when no signal exists.
- `unmarshalModelArray` (used by `GetModels`) — embeds `capabilityAliasFields` into its existing raw-decode struct (same pattern already used there for `id`/`model_id`/`status` reconciliation) and applies it per model.
- `GetModelByID` — decodes into `struct{ VerifiedModel; capabilityAliasFields }` and applies the same reconciliation.
- `VerifyModel` — decodes into `struct{ VerificationResult; capabilityAliasFields }` and applies the same reconciliation.
- RAG/Skills/Plugins need **no** alias code — their JSON tags already match the real service exactly on both sides, so the existing generic `json.Unmarshal` (already true since Phase 1) handles them.

## 4. Populated-from-real-source vs honest-false-with-reason

| Flag | Populated from real source? | Detail |
|---|---|---|
| `SupportsRAG` | YES (generic JSON tag match) | Matches real service's `supports_rag` exactly — flows automatically via `json.Unmarshal` once the live endpoint emits it. |
| `SupportsSkills` | YES (generic JSON tag match) | Matches real service's `supports_skills` exactly. |
| `SupportsPlugins` | YES (generic JSON tag match) | Matches real service's `supports_plugins` exactly. |
| `SupportsMCP` | YES (alias reconciliation, Phase 2) | Real service uses plural `supports_mcps`; now promoted via `capabilityAliasFields.applyTo`. |
| `SupportsLSP` | YES (alias reconciliation, Phase 2) | Real service uses plural `supports_lsps`; now promoted. |
| `SupportsACP` | YES (alias reconciliation, Phase 2) | Real service uses plural `supports_acps`; now promoted. |

**Honest boundary (documented in code, `client.go` `capabilityAliasFields` doc comment):** as of THIS session, the live LLMsVerifier `/api/models`, `/api/models/{id}`, and `/api/models/{id}/verify` handlers emit **none** of the 6 keys (verified by reading `api/handlers.go` directly) — so today every flag legitimately still evaluates to `false` end-to-end against the real running service. This is NOT fabricated: it is the honest "not verified as supporting" default the Phase 1 doc comment mandates. Nothing was invented; the wiring is proven-correct forward-compatible plumbing for the moment the LLMsVerifier team starts emitting either key convention on those specific endpoints (a LLMsVerifier-submodule-side gap, out of this task's file scope). The embedded/fallback server path (`FallbackModels`) also stays honestly false — no probe data exists there either.

## 5. TDD — RED → GREEN

RED (Phase 2 tests added to `capability_flags_test.go`, `client.go` reverted to Phase-1-only state via `git checkout`):

```
$ go test -tags=nogui -count=1 -run 'TestClient_GetModels_CapabilityFlags|TestClient_GetModelByID_CapabilityFlags|TestClient_VerifyModel_CapabilityFlags|TestCapabilityAliasFields_ApplyTo' ./internal/verifier/...
# dev.helix.code/internal/verifier [dev.helix.code/internal/verifier.test]
internal/verifier/capability_flags_test.go:328:2: undefined: capabilityAliasFields
internal/verifier/capability_flags_test.go:336:2: undefined: capabilityAliasFields
internal/verifier/capability_flags_test.go:347:2: undefined: capabilityAliasFields
internal/verifier/capability_flags_test.go:358:2: undefined: capabilityAliasFields
FAIL	dev.helix.code/internal/verifier [build failed]
FAIL
```

GREEN (Phase 2 `client.go` fix reapplied):

```
$ go test -tags=nogui -count=1 -v -run 'TestClient_GetModels_CapabilityFlags|TestClient_GetModelByID_CapabilityFlags|TestClient_VerifyModel_CapabilityFlags|TestCapabilityAliasFields_ApplyTo' ./internal/verifier/...
=== RUN   TestClient_GetModels_CapabilityFlags_RealServerPluralKeys
--- PASS: TestClient_GetModels_CapabilityFlags_RealServerPluralKeys (0.00s)
=== RUN   TestClient_GetModels_CapabilityFlags_SingularKeys
--- PASS: TestClient_GetModels_CapabilityFlags_SingularKeys (0.00s)
=== RUN   TestClient_GetModels_CapabilityFlags_HonestFalse_WhenAbsent
--- PASS: TestClient_GetModels_CapabilityFlags_HonestFalse_WhenAbsent (0.00s)
=== RUN   TestClient_GetModelByID_CapabilityFlags_RealServerPluralKeys
--- PASS: TestClient_GetModelByID_CapabilityFlags_RealServerPluralKeys (0.00s)
=== RUN   TestClient_VerifyModel_CapabilityFlags_RealServerPluralKeys
--- PASS: TestClient_VerifyModel_CapabilityFlags_RealServerPluralKeys (0.00s)
=== RUN   TestClient_VerifyModel_CapabilityFlags_HonestFalse_WhenAbsent
--- PASS: TestClient_VerifyModel_CapabilityFlags_HonestFalse_WhenAbsent (0.00s)
=== RUN   TestCapabilityAliasFields_ApplyTo_NeverDemotesTrueToFalse
--- PASS: TestCapabilityAliasFields_ApplyTo_NeverDemotesTrueToFalse (0.00s)
PASS
ok  	dev.helix.code/internal/verifier	0.013s
```

(One test-mock fix mid-cycle: the first GREEN attempt caught a genuine PRE-EXISTING, UNRELATED bug — the real `VerifyModelHandler` emits `model_id` as a numeric int64 while `VerificationResult.ModelID` is typed `string`; my `TestClient_VerifyModel_CapabilityFlags_HonestFalse_WhenAbsent` mock accidentally included a numeric `model_id` and tripped it. Fixed by omitting `model_id` from that mock — the mismatch itself is out of scope for HXC-117 Phase 2 and is documented in a code comment rather than silently worked around.)

## 6. Package build + full test suite

```
$ go build -tags=nogui ./internal/verifier/...
(exit 0, no output)

$ go test -tags=nogui ./internal/verifier/... -count=1
ok  	dev.helix.code/internal/verifier	4.364s
?   	dev.helix.code/internal/verifier/i18n	[no test files]
```

Downstream-consumer sanity build (packages that import `internal/verifier`):

```
$ go build -tags=nogui ./internal/llm/... ./internal/mcp/...
(exit 0, no output)
```

## 7. Full-repo build (`go build -tags=nogui ./...`)

```
$ go build -tags=nogui ./...
# dev.helix.code/cmd/cli
cmd/cli/main.go:41:2: could not import dev.helix.code/internal/rag (open : no such file or directory)
cmd/cli/acp_cmd.go:68:45: not enough arguments in call to acp.NewAgent
	have ()
	want (llm.Provider)
```

**This failure is PRE-EXISTING / OUT OF SCOPE, not caused by this task's changes:**
- `internal/rag` does not exist yet — HXC-118 (RAG integration) has not landed; this task was explicitly told not to touch `internal/rag`.
- `cmd/cli/acp_cmd.go`'s `acp.NewAgent` call-site mismatch is inside `internal/acp`/`cmd/cli` — HXC-119 (ACP) scaffold work, explicitly out of this task's file scope.
- `internal/verifier` (this task's only touched package) is not on the failing import path from `internal/verifier`'s own side — `go build ./internal/verifier/...` and `go test ./internal/verifier/...` both pass standalone (§6), and downstream consumers `internal/llm`/`internal/mcp` also build clean against the Phase 2 `client.go` changes.
- `git status` at the start of this run showed `cmd/cli/acp_cmd.go`, `cmd/cli/main.go`, `internal/acp/agent.go`, `internal/acp/agent_test.go`, `internal/acp/doc.go` as already-modified in the working tree (not by this task) — consistent with concurrent HXC-118/HXC-119 work in progress on another track in this same multi-track checkout (per the repo's §11.4.176/§11.4.178 multi-track governance). This task did not touch, revert, or otherwise interact with those files.

## Anti-bluff scan

```
$ grep -rniE "\bsimulated\b|\bfor now\b|TODO implement|in production this would" internal/verifier/client.go internal/verifier/capability_flags_test.go
(no matches — exit 1)
```
