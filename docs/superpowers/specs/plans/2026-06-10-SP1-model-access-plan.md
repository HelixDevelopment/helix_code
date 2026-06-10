# SP1 — Model-Access Refinement: Implementation Plan

| Field | Value |
|-------|-------|
| Revision | 1 |
| Created | 2026-06-10 |
| Last modified | 2026-06-10 |
| Status | draft |
| Maintainer | SP1 planning subagent (READ-ONLY planning pass) |
| Source spec | `docs/superpowers/specs/2026-06-10-llms-access-master-roadmap.md` §SP1 |
| Evidence base | `docs/superpowers/specs/analysis/2026-06-10-B-model-access.md` |
| Authority | Cascades from `constitution/` + root `CLAUDE.md`/`CONSTITUTION.md`; anti-bluff §11.4 governs every closure |

> **Anti-bluff note (§11.4 / §11.4.123):** This is a *plan*, not a claim of done work. No task below is
> "complete" until it carries captured runtime evidence per §11.4.5 / §11.4.69 / §11.4.107. Brainstorming
> HARD-GATE: no implementation, no push until this plan is operator-approved. Subagent-driven execution is the
> default (§11.4.70). TDD RED-first discipline (§11.4.43 / §11.4.115); every closed defect gets a standing
> regression guard (§11.4.135).

---

## Table of contents

- [1. Goal & scope](#1-goal--scope)
- [2. Confirmed evidence anchors (re-verified this pass)](#2-confirmed-evidence-anchors)
- [3. Target design (the working-model funnel)](#3-target-design)
- [4. Decisions needing operator input](#4-decisions-needing-operator-input)
- [5. Files to create / modify](#5-files-to-create--modify)
- [6. Phased TDD task list](#6-phased-tdd-task-list)
- [7. Test-type matrix for SP1](#7-test-type-matrix-for-sp1)
- [8. Docs & guides deliverables](#8-docs--guides-deliverables)
- [9. Reuse/lift ledger (§11.4.74)](#9-reuselift-ledger)
- [10. Summary](#10-summary)

---

## 1. Goal & scope

**Goal (verbatim from request):** *"For every provider, if an API key is provided OR recognized from `.env` /
`.api_keys` / shell-exported, that provider's WORKING models become available — everything dynamically obtained
+ validated + verified via LLMsVerifier."*

SP1 closes five evidence-backed defects (roadmap §1a):
- **D-2** CLI `handleListModels` lists `failed`/`pending` models as available (`cmd/cli/main.go:1361`/`:1392`).
- **D-3** `secrets.LoadAPIKeys` is DEAD code, never called in prod (`internal/secrets/loader.go:30`).
- **D-4** working-model filter (`Verified ∧ score≥min`) loaded but never applied (`adapter.go:175`).
- **D-5** hardcoded provider model lists (CONST-036) (`openai_provider.go:203`, `anthropic_provider.go:205`, …).
- Plus the **no-key-presence-gate** gap (a user with one key sees every provider) and the **single-provider
  factory** gap (`factory.go:9` builds ONE provider by `config.Type`, no key-present enumerator).

Out of scope (owned by other SPs): HelixAgent `/v1/completion/models` hardcoded list (D-1 → SP2); CLI-agent
stub packages (D-6 → SP4); `go.mod replace dev.helix.agent` wiring (D-7 → SP4/SP5); constitution Rev-table
drift (D-8 → SP0).

---

## 2. Confirmed evidence anchors

All anchors below were opened and confirmed in this planning pass (file:line as read).

| Anchor | Confirmed fact |
|---|---|
| `helix_code/internal/secrets/loader.go:30` | `func LoadAPIKeys() error` reads `$HOME/api_keys.sh` (L33-37 `loadFromShell`), else walks up to `.env` (L119 `findEnvFile`, L84 `loadFromEnv`), `os.Setenv` each (L78/L103). CONST-042-clean (no value logging). Zero prod call-sites = DEAD. |
| `helix_code/internal/config/config.go:307-345` | `v.AutomaticEnv()` + `SetEnvPrefix("HELIX")` (L308-309); 8 single-alias per-provider bindings (L338-345: openai/anthropic/gemini/deepseek/groq/mistral/xai/openrouter) — narrower than helix_agent's multi-alias. |
| `helix_code/cmd/cli/main.go:1355-1390` | `handleListModels`: P1 verifier `GetVerifiedModels` (L1361) → `printVerifiedModels` (prints ALL, no filter); P2 single `c.llmProvider.GetModels()` (L1372); P3 `verifier.FallbackModels` (L1387). |
| `helix_code/cmd/cli/main.go:1392-1414` | `printVerifiedModels` renders `✓ verified` / `○ pending` / `✗ failed` / `⏳ rate-limited` — i.e. prints failed/pending rows as "available" (D-2). |
| `helix_code/internal/verifier/adapter.go:175-180` | `GetMinAcceptableScore()` default 6.0 from `Scoring.MinAcceptableScore`. **Never invoked** in any listing path. |
| `helix_code/internal/verifier/adapter.go:183-224` | `GetVerifiedModels`: gated on `IsEnabled()`; cache→live→stale→`getFallbackModels()`; returns `filterByProviderConfig(models)`. No key-presence, no Verified/score filter. |
| `helix_code/internal/verifier/adapter.go:277-289` | `filterByProviderConfig` drops only `config.Providers[m.Provider].Enabled == false` (manual flag). |
| `helix_code/internal/verifier/types.go:27/31/32/33/48/54` | `VerifiedModel{DisplayName, Verified bool, VerificationStatus string, ContextSize, OverallScore float64, LastVerified}` — the "working" fields. |
| `helix_code/internal/llm/openai_provider.go:201-239` | `initializeModels()` hardcodes 4 models (gpt-4o/4-turbo/4/3.5-turbo). CONST-036 risk. |
| `helix_code/internal/llm/anthropic_provider.go:203-205+` | `getAnthropicModels()` hardcodes claude-4-sonnet/opus, 3.x families. CONST-036 risk. |
| `helix_code/internal/llm/factory.go:9-101` | `NewProvider(config)` switch builds ONE provider by `config.Type`. `InitializeModelManager` (L135) loops `configs` but only those with `config.Enabled`. No "build every key-present provider" enumerator. |
| `submodules/helix_agent/internal/verifier/provider_types.go:348-385+` | `SupportedProviders map[string]*ProviderTypeInfo` with per-provider multi-alias `EnvVars` (claude `{ANTHROPIC_API_KEY,CLAUDE_API_KEY}`, gemini `{GEMINI_API_KEY,GOOGLE_API_KEY,ApiKey_Gemini}`, …). THE reuse template. |
| `submodules/helix_agent/internal/verifier/startup.go:389-414` | Per-provider loop: for each `EnvVar`, `os.Getenv`; if non-empty `&& !isPlaceholder` → `DiscoverModels(ctx, providerType, apiKey)` dynamic-first, static `info.Models` fallback. THE key→discover flow. |
| `submodules/llms_verifier/llm-verifier/api_keys/{env_scanner.go,manager.go,priority.go}` | `EnvVarScanner` + faulty-key registry. Imported in helix_agent as `digital.vasic.llmsverifier/api_keys`. |

---

## 3. Target design

The working-model funnel (analysis §4.1, anti-bluff-tightened):

```
startup (cmd/server/main.go AND cmd/cli/main.go) — BEFORE config load
  └─ secrets.LoadAPIKeys()                         [D-3 — WIRE; loader.go:30]
        reads $HOME/api_keys.sh → else walked-up .env → os.Setenv each
        (precedence note: already-exported shell env is NOT overwritten by loader? — see DECISION-1)
  └─ config.Load()  (viper AutomaticEnv + BindEnv — now sees loader-populated env)
  └─ keyrec.PresentProviders()                     [NEW — internal/llm/keyrecognition.go]
        for each provider, any(os.Getenv(alias)) != "" && !isPlaceholder(v)
        alias table sourced from helix_agent SupportedProviders.EnvVars (§11.4.74 reuse)
        → set[providerType]bool
  └─ verifier.Adapter.GetWorkingModels(ctx, present) [NEW METHOD — adapter.go]
        m kept  iff  present[m.Provider]                          (key-presence gate — NEW)
                AND  m.Verified == true                           (types.go:31)
                AND  m.VerificationStatus == "verified"           (types.go:32)
                AND  m.OverallScore >= GetMinAcceptableScore()    (adapter.go:175 — now APPLIED, D-4)
                AND  config.Providers[m.Provider].Enabled != false (existing flag preserved)
  └─ expose:
        - CLI handleListModels (main.go:1361) → GetWorkingModels; printVerifiedModels prints only working
        - server handlers (internal/server/handlers.go:1049/1140/1206)
        - model_manager registry (model_manager.go:132 SetVerifierAdapter / :202 getAvailableModels)
  └─ verifier-DISABLED / cold path  [DECISION-2]
        NOT FallbackModels-as-working (that = §11.4 / CONST-035 PASS-bluff)
```

De-hardcoding (D-5, CONST-036): convert each hardcoded `initializeModels()` to either (a) live catalog fetch
(mirror OpenRouter `fetchCatalog` / Ollama `/api/tags`) OR (b) defer to the verifier catalog. The working-model
funnel above already routes the *displayed* list through the verifier, so the provider `GetModels()` lists stop
being the user-facing source — but they remain a CONST-036 violation as long as they ship literal slices and are
reachable via CLI P2 fallback (main.go:1372). See [DECISION-3](#4-decisions-needing-operator-input).

---

## 4. Decisions needing operator input

These are genuine ambiguities the agent must NOT guess (§11.4.6 / §11.4.101 — surface via `AskUserQuestion`
per §11.4.66 before the affected task starts):

- **DECISION-1 — LoadAPIKeys precedence vs shell-exported env.** `loader.go` calls `os.Setenv`
  unconditionally, so `.env`/`api_keys.sh` values would OVERRIDE an already-exported shell var. The request says
  "provided OR recognized from `.env` / `.api_keys` / shell-exported" without ranking them. Options: (a)
  shell-export wins (loader only fills GAPS — safest, least surprising); (b) file wins (loader overrides —
  current loader behaviour); (c) explicit precedence `.env` < `api_keys.sh` < shell. **Recommended: (a)
  gap-fill** — least destructive, matches "recognize if not already set". Affects T1.1.1 + a loader change.

- **DECISION-2 — verifier-disabled behaviour** (`HELIX_VERIFIER_ENABLED=false` default, `.env.example:30`).
  When the verifier is off there is NO authoritative "working" signal. Options: (a) "working models" require the
  verifier — disabled ⇒ empty list + honest notice ("enable HELIX_VERIFIER_ENABLED to see working models"); (b)
  fall back to per-provider LIVE `GetModels()` for key-present providers only (never hardcoded fallback),
  labelled `unverified`; (c) keep today's `FallbackModels` path. (c) is a §11.4/CONST-035 PASS-bluff and is
  REJECTED. **Recommended: (a)** for the headline "working models" command + (b) behind an explicit
  `--include-unverified` flag. Affects T1.3.3.

- **DECISION-3 — de-hardcoding strategy per provider** (CONST-036). Options: (a) delete the literal
  `initializeModels` slices and have `GetModels()` fetch live (OpenAI `/v1/models`, Anthropic `/v1/models`,
  etc.) — most faithful to CONST-036 but adds network dependency + needs the API key at construction; (b) keep a
  thin static slice ONLY as an offline fallback but route ALL user-facing listing through the verifier funnel
  (§3) so the literal is never shown as authoritative; (c) full per-provider live fetch now for the 8 cloud
  providers, defer Azure/Bedrock/VertexAI/Copilot/Qwen. **Recommended: (b)+(c) staged** — funnel first (removes
  the user-facing bluff cheaply), then convert OpenAI/Anthropic/DeepSeek/Mistral to live-fetch. Note §11.4.124:
  the hardcoded slices are NOT dead code (CLI P2 reaches them) — investigate-before-remove is satisfied; we
  convert, not silently delete.

- **DECISION-4 — key-recognition table source.** Options: (a) `import` helix_agent `SupportedProviders` directly
  (needs `replace dev.helix.agent` — D-7, owned by SP4/SP5, not yet wired); (b) lift a decoupled copy of just
  the `{provider → []envVarAlias}` map into a new HelixCode `internal/llm/keyrecognition.go` now, with a
  forward-note to converge on the shared substrate when D-7 lands. **Recommended: (b)** — unblocks SP1 without
  taking the SP4/SP5 dependency; the map is small, data-only, and CONST-051-decoupled. Record `Catalogue-Check:
  extend helix_agent@<sha>` per §11.4.74.

---

## 5. Files to create / modify

| Path | Action | Purpose |
|---|---|---|
| `helix_code/internal/llm/keyrecognition.go` | **create** | `PresentProviders()` + multi-alias `{provider→[]envVar}` table + `isPlaceholder()`; lifted/decoupled from helix_agent `SupportedProviders.EnvVars` (§11.4.74, DECISION-4). |
| `helix_code/internal/llm/keyrecognition_test.go` | **create** | unit: alias precedence, absence, placeholder rejection (mocks OK — unit only). |
| `helix_code/cmd/server/main.go` | **modify** | call `secrets.LoadAPIKeys()` first in `main()` (D-3, T1.1.1). |
| `helix_code/cmd/cli/main.go` | **modify** | call `secrets.LoadAPIKeys()` first in `main()` (D-3); `handleListModels:1361` → `GetWorkingModels`; `printVerifiedModels:1392` stops emitting failed/pending as available (D-2); P3 fallback gated by DECISION-2. |
| `helix_code/internal/secrets/loader.go` | **modify (DECISION-1 only)** | optional gap-fill precedence (skip `os.Setenv` when var already set). |
| `helix_code/internal/verifier/adapter.go` | **modify** | new `GetWorkingModels(ctx, present map[string]bool) ([]*VerifiedModel, error)` applying key∧Verified∧status∧min-score (D-4, uses `GetMinAcceptableScore:175`); keep `GetVerifiedModels` for raw catalog. |
| `helix_code/internal/verifier/adapter_test.go` | **modify/create** | unit: filter predicate truth-table + paired §1.1 mutation (strip key-gate → no-key provider leaks → FAIL). |
| `helix_code/internal/llm/factory.go` | **modify** | add `BuildPresentProviders(configs, present)` enumerator (build every key-present provider, not ONE by `config.Type`). |
| `helix_code/internal/llm/openai_provider.go` | **modify (D-5/DECISION-3)** | convert `initializeModels:201` to live-fetch or verifier-deferred. |
| `helix_code/internal/llm/anthropic_provider.go` | **modify (D-5/DECISION-3)** | convert `getAnthropicModels:203` likewise. |
| `helix_code/internal/llm/{deepseek,mistral}_provider.go` | **modify (D-5)** | convert hardcoded lists. |
| `helix_code/internal/server/handlers.go` | **modify** | `:1049/:1140/:1206` route through `GetWorkingModels` + present-set. |
| `helix_code/internal/llm/model_manager.go` | **modify** | `:132 SetVerifierAdapter` / `:202 getAvailableModels` populate registry from working-model funnel, not hardcoded `GetModels()`. |
| `helix_code/tests/integration/sp1_model_access_test.go` | **create** | `-tags=integration`, REAL verifier + REAL provider key, NO mocks (CONST-050). |
| `helix_code/tests/e2e/challenges/sp1_working_models/...` | **create** | one-key→only-that-provider→real-generate E2E (§11.4.107 liveness). |
| `helix_code/tests/stresschaos/sp1_model_access_stress_test.go` | **create** | verifier-unreachable / concurrent-list (§11.4.85). |
| `helix_code/tests/regression/sp1_working_model_guard_test.go` | **create** | standing RED_MODE guards for D-2/D-3/D-4 (§11.4.135). |
| `challenges/banks/sp1_model_access/...` | **create** | cross-cutting Challenge bank entry (§11.4.83 transcript). |
| `helix_code/docs/guides/model-access.md` (+ `.html`/`.pdf`) | **create** | user manual (CONST-066 always-sync). |
| `.env.example` | **modify** | document `api_keys.sh` + `.env` auto-load + per-provider aliases + verifier-disabled behaviour. |
| `docs/ARCHITECTURE.md` | **modify** | working-model funnel flow. |
| `docs/Issues.md` / `docs/Issues_Summary.md` / `docs/CONTINUATION.md` | **modify** | ATM-NNN entries per §11.4.54 / CONST-044. |

---

## 6. Phased TDD task list

Each task: **(a) RED** (reproduce gap on current code, `RED_MODE=1`) → **(b) IMPL** (exact file/func) →
**(c) GREEN+evidence** (`RED_MODE=0` guard + captured runtime proof) → **(d) rollback**.

### Phase P1.0 — Pre-work (blocking)
- **T1.0.1** Resolve DECISION-1..4 via `AskUserQuestion` (§11.4.66). Capture answers into this plan (bump Rev).
- **T1.0.2** Catalogue-Check (§11.4.74): record `extend helix_agent@<sha>` + `reuse llms_verifier/api_keys@<sha>`
  in the ATM-NNN tracker entry.
- **T1.0.3** `git fetch --all --prune` + `git log HEAD..@{u}` (§11.4.37) before any edit; capture output.

### Phase P1.1 — Key auto-recognition (D-3)
- **T1.1.1 — Wire `secrets.LoadAPIKeys` at startup.**
  - (a) RED: `tests/integration/sp1_model_access_test.go::TestLoadAPIKeys_WiredAtStartup` — set ONLY a key in a
    temp `.env` (no shell export), boot CLI/server entrypoint, assert the provider becomes key-present.
    `RED_MODE=1` on current tree FAILS (loader never called → key not recognized).
  - (b) IMPL: call `secrets.LoadAPIKeys()` as the first statement of `main()` in `cmd/server/main.go` and
    `cmd/cli/main.go`, BEFORE `config.Load()`. Apply DECISION-1 precedence in `loader.go` if chosen.
  - (c) GREEN: `RED_MODE=0` guard passes; capture stdout proving the temp-`.env` key was recognized (value never
    printed — CONST-042). Evidence path `docs/qa/<run-id>/loadapikeys/`.
  - (d) Rollback: remove the two `main()` calls (+ any `loader.go` precedence change); loader returns to DEAD.
- **T1.1.2 — Multi-alias key-recognition table.**
  - (a) RED: `keyrecognition_test.go::TestPresentProviders_AliasAndPlaceholder` — set `CLAUDE_API_KEY` (alias),
    assert anthropic present; set a placeholder value, assert NOT present. RED FAILS (no table exists).
  - (b) IMPL: create `internal/llm/keyrecognition.go` with `PresentProviders() map[string]bool`, the
    `{provider→[]envVarAlias}` map (lifted decoupled from helix_agent `provider_types.go:349`), `isPlaceholder()`.
  - (c) GREEN: unit table-driven pass; paired §1.1 mutation (drop an alias → that alias no longer recognized →
    test FAILs) captured.
  - (d) Rollback: delete `keyrecognition.go` + test.

### Phase P1.2 — Key-presence → provider-availability gate + factory enumerator
- **T1.2.1 — Key-presence gate in the verifier path.**
  - (a) RED: `adapter_test.go::TestGetWorkingModels_KeyPresenceGate` — verifier returns OpenAI+Anthropic
    verified models, `present={anthropic:true}`; assert OpenAI models DROPPED. RED FAILS (no `GetWorkingModels`).
  - (b) IMPL: add `GetWorkingModels(ctx, present)` to `adapter.go`; it calls `GetVerifiedModels` then filters by
    `present[m.Provider]` (+ the D-4 predicate, T1.3.1).
  - (c) GREEN: predicate truth-table pass; **paired §1.1 mutation** (strip the `present[...]` check → no-key
    provider leaks → FAIL) captured.
  - (d) Rollback: remove `GetWorkingModels`; callers fall back to `GetVerifiedModels`.
- **T1.2.2 — Key-present provider enumerator (`factory.go:9`).**
  - (a) RED: `factory_test.go::TestBuildPresentProviders_EnumeratesAllKeyed` — present={openai,anthropic};
    assert TWO providers built (current `NewProvider` builds one by `config.Type` → RED FAILS for the set case).
  - (b) IMPL: add `BuildPresentProviders(configs, present) ([]Provider, error)` looping every key-present
    provider, reusing `NewProvider` per type.
  - (c) GREEN: capture the enumerated provider list; mutation (force single-provider → set incomplete → FAIL).
  - (d) Rollback: remove enumerator; `InitializeModelManager` unchanged.

### Phase P1.3 — Dynamic working-model exposure (D-4, D-2, D-5)
- **T1.3.1 — Apply the working-model filter (D-4).**
  - (a) RED: `adapter_test.go::TestGetWorkingModels_VerifiedAndScore` — feed a `failed` model, a `verified`
    model with score 4.0 (<min 6.0), and a `verified` score 8.0; assert only the last survives. RED FAILS today
    (no path applies `GetMinAcceptableScore:175` / Verified / status).
  - (b) IMPL: inside `GetWorkingModels`, apply `m.Verified && m.VerificationStatus=="verified" &&
    m.OverallScore >= a.GetMinAcceptableScore()`. Honour CONST-038 poll ≤60s + CONST-037 verified ≤24h via
    existing cache TTL.
  - (c) GREEN: truth-table + paired §1.1 mutation (drop the score check → low-score model leaks → FAIL).
  - (d) Rollback: filter reverts to key-presence-only.
- **T1.3.2 — CLI/server/registry exposure switch (D-2).**
  - (a) RED: `e2e/.../sp1_working_models` — set ONLY `ANTHROPIC_API_KEY`, run `cli --list-models`; assert NO
    `failed`/`pending` rows and NO non-Anthropic providers. RED FAILS today (`printVerifiedModels:1392` shows
    failed/pending; P1 lists whole catalog).
  - (b) IMPL: `handleListModels:1361` → `GetWorkingModels(ctx, present)`; `printVerifiedModels:1392` renders only
    working rows; server `handlers.go:1049/1140/1206` + `model_manager.go:132/202` likewise.
  - (c) GREEN: captured CLI transcript shows only Anthropic verified models; then a REAL `generate` returns REAL
    output (§11.4.107 liveness, not metadata). Evidence `docs/qa/<run-id>/working-models/`.
  - (d) Rollback: revert call-sites to `GetVerifiedModels` + old printer.
- **T1.3.3 — Safe verifier-disabled / cold behaviour (DECISION-2).**
  - (a) RED: `TestListModels_VerifierDisabled_NoBluff` — `HELIX_VERIFIER_ENABLED=false`; assert the command does
    NOT present `FallbackModels` as "working". RED FAILS today (`main.go:1387` prints fallback list).
  - (b) IMPL: per DECISION-2 — empty + honest notice (recommended) or `--include-unverified` live path.
  - (c) GREEN: capture the honest-notice output; mutation (re-enable fallback-as-working → FAIL).
  - (d) Rollback: restore P3 fallback (NOT recommended — keeps the bluff).
- **T1.3.4 — De-hardcode provider model lists (D-5 / CONST-036, DECISION-3).**
  - (a) RED: `TestProviderModels_NotHardcoded_OpenAI/Anthropic` — assert `GetModels()` does not return the exact
    literal slice when a live source is reachable (or that the user-facing funnel never surfaces the literal).
    RED FAILS today (literal slices at `openai_provider.go:201`, `anthropic_provider.go:203`).
  - (b) IMPL: per DECISION-3 — funnel-first (route all listing through verifier) then convert
    OpenAI/Anthropic/DeepSeek/Mistral `initializeModels` to live-fetch (mirror OpenRouter `fetchCatalog` /
    Ollama `/api/tags`). §11.4.124: convert, don't silently delete.
  - (c) GREEN: capture a live model list from a real provider endpoint; anti-bluff smoke (`grep` for
    `simulated|for now|placeholder`) clean.
  - (d) Rollback: restore literal slices (CONST-036 violation returns).

### Phase P1.4 — Docs & guides (CONST-066 / §11.4.65)
- **T1.4.1** Write `docs/guides/model-access.md` (key setup, `.env`/`api_keys.sh` auto-load, alias table,
  working-model definition, verifier-disabled behaviour); export `.html`+`.pdf`.
- **T1.4.2** Update `.env.example` (auto-load note + aliases + verifier behaviour) and `docs/ARCHITECTURE.md`
  (funnel flow). Update README Documentation Map (CONST-062).

### Phase P1.5 — Full test-type coverage + regression guards (§11.4.135, CONST-050)
- **T1.5.1** Stress/chaos (`tests/stresschaos/sp1_model_access_stress_test.go`): verifier-unreachable →
  honest degradation (not fallback-as-working); 10 concurrent `GetWorkingModels`; §11.4.85 evidence.
- **T1.5.2** Standing regression guards (`tests/regression/sp1_working_model_guard_test.go`) registering D-2,
  D-3, D-4 with `RED_MODE` polarity per §11.4.115; wire into the release-gate suite (§11.4.40).
- **T1.5.3** Challenge bank (`challenges/banks/sp1_model_access/`) + HelixQA registration; captured bidirectional
  transcript under `docs/qa/<run-id>/` (§11.4.83).
- **T1.5.4** Code-review gate (§11.4.125/§11.4.134) before pre-build + main build; iterate-until-GO.

---

## 7. Test-type matrix for SP1

| Test type | Harness | Anti-bluff requirement |
|---|---|---|
| Unit | `keyrecognition_test.go`, `adapter_test.go`, `factory_test.go` | mocks OK (unit only); paired §1.1 mutation per filter. |
| Integration (`-tags=integration`) | `tests/integration/sp1_model_access_test.go` | REAL verifier (`make test-verifier-integration`) + REAL provider key; NO mocks (CONST-050). |
| E2E | `tests/e2e/challenges/sp1_working_models` | one-key→only-that-provider→REAL generate returns REAL output (§11.4.107 liveness). |
| Full-automation | self-driving, `-count=3` re-runnable | no operator typing (§11.4.98). |
| Stress + chaos | `tests/stresschaos/sp1_model_access_stress_test.go` | verifier-down honest degradation; concurrency (§11.4.85). |
| Security | reuse `make test-security-full` | key never logged/committed (CONST-042); placeholder rejection. |
| Regression guard | `tests/regression/sp1_working_model_guard_test.go` | RED_MODE polarity; release-gate blocker (§11.4.135). |
| Challenge / HelixQA | `challenges/banks/sp1_model_access/` + HelixQA bank | captured transcript (§11.4.83). |
| Performance/benchmark | `go test -bench` on `GetWorkingModels` | filter is O(n), no regression. |

UI/UX/DDoS/scaling: N/A-or-delegated for this server/CLI-only surface — record honest coverage note (§11.4.3),
do not fake (SP7 owns the cross-cutting matrix).

---

## 8. Docs & guides deliverables

- `docs/guides/model-access.md` (+`.html`/`.pdf`) — user manual.
- `.env.example` — auto-load + aliases + verifier-disabled behaviour.
- `docs/ARCHITECTURE.md` — funnel flow.
- README Documentation Map + revision header (CONST-062/064).
- `docs/Issues.md`/`Issues_Summary.md`/`Fixed.md`/`CONTINUATION.md` — ATM-NNN entries, type-aware closure
  (CONST-057), kept in sync each state-advancing commit (CONST-044/063).

---

## 9. Reuse/lift ledger

Per §11.4.74 (record in tracker `Catalogue-Check:`):
- **Key-recognition table** → `extend helix_agent@<sha>` — lift the decoupled `{provider→[]envVar}` map from
  `provider_types.go:349 SupportedProviders.EnvVars` into `internal/llm/keyrecognition.go` (DECISION-4(b));
  converge on the shared substrate when D-7 (`replace dev.helix.agent`) lands under SP4/SP5.
- **Key→discover flow** → `reuse helix_agent@<sha>` pattern from `startup.go:389` (per-env-var → DiscoverModels
  dynamic-first) as the de-hardcoding template (T1.3.4).
- **Faulty-key registry / scanner** → `reuse llms_verifier/api_keys@<sha>` (`env_scanner.go`/`manager.go`/
  `priority.go`) when D-7 wiring exists; until then, key-presence is sufficient for SP1.
- **Verifier client/adapter** → already real (`verifier/client.go` live HTTP) — extend, don't replace.

---

## 10. Summary

1. SP1 makes a provider's WORKING models appear iff a key is recognized (`.env`/`api_keys.sh`/shell) AND the
   verifier validates them — closing D-2, D-3, D-4, D-5 plus the no-key-gate and single-factory gaps.
2. Root cause is wiring, not new architecture: `secrets.LoadAPIKeys` (loader.go:30) is DEAD; the working-model
   predicate exists in pieces (`GetMinAcceptableScore:175`, `VerifiedModel` fields) but is never assembled.
3. The fix adds a `keyrecognition.go` present-providers table (lifted decoupled from helix_agent
   `SupportedProviders.EnvVars:349`), a `GetWorkingModels` filter (key∧Verified∧status∧min-score), a
   key-present factory enumerator, and routes CLI/server/registry through the funnel.
4. De-hardcoding (CONST-036) converts OpenAI/Anthropic/DeepSeek/Mistral literal lists to live/verifier-sourced;
   §11.4.124 satisfied — convert, never silently delete (CLI P2 reaches them, so not dead code).
5. Verifier-disabled MUST NOT present `FallbackModels` as "working" (that is a CONST-035/§11.4 PASS-bluff) —
   DECISION-2 specifies honest empty-or-unverified behaviour.
6. Every task is TDD RED-first (§11.4.43/§11.4.115) with a `RED_MODE` polarity switch and a paired §1.1 mutation;
   every closure registers a standing regression guard (§11.4.135).
7. Tests beyond unit hit REAL verifier + REAL provider endpoints (CONST-050); "working" models are proven by a
   REAL generate returning REAL output (§11.4.107 liveness), never metadata-only.
8. Four operator decisions are flagged (precedence, verifier-disabled behaviour, de-hardcode strategy, table
   source) — surfaced via AskUserQuestion (§11.4.66) BEFORE the affected tasks; no guessing (§11.4.6).
9. Deliverables include docs/guides (`model-access.md`, `.env.example`, ARCHITECTURE), the full SP1 test-type
   matrix, Challenge/HelixQA banks with captured transcripts, and synced Issues/Continuation (CONST-044/063/066).
10. Reuse-first per §11.4.74: extend helix_agent + llms_verifier rather than re-implement; converge on the shared
    substrate when the SP4/SP5 `replace dev.helix.agent` wiring (D-7) lands.

**Ordered task count: 18 tasks** across 6 phases (P1.0: 3, P1.1: 2, P1.2: 2, P1.3: 4, P1.4: 2, P1.5: 4) +
4 operator decisions (DECISION-1..4).
