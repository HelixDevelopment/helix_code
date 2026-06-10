# SP2 — HelixAgent Exposure Extension — Implementation Plan

| Field | Value |
|-------|-------|
| Revision | 1 |
| Last modified | 2026-06-10 |
| Status summary | PLANNING — TDD/anti-bluff plan; NOT approved for implementation (brainstorming HARD-GATE per master roadmap §0/§4) |
| Sub-program | SP2 (depends on SP1; feeds from analysis `…-C-…`) |
| Source design | `docs/superpowers/specs/analysis/2026-06-10-C-helixagent-exposure.md` |
| Roadmap | `docs/superpowers/specs/2026-06-10-llms-access-master-roadmap.md` §SP2 |
| Scope root | `/Volumes/T7/Projects/HelixCode/submodules/helix_agent` (module `dev.helix.agent`, `go 1.26`) + `helix_code/go.mod` (SDK currency only) |
| Authority | constitution/ + root CLAUDE.md/CONSTITUTION.md; anti-bluff §11.4 family governs every closure |

> **Anti-bluff note (§11.4 / §11.4.123):** This is a *plan*, not a claim of done work. No
> task below is "done" until it carries captured runtime evidence per §11.4.5 / §11.4.69 /
> §11.4.107. Every fix is RED-first (§11.4.115): reproduce the defect on the pre-fix artifact
> with a `RED_MODE` polarity switch BEFORE writing the fix. Code-review gate (§11.4.125/§11.4.134)
> runs before pre-build + main build. Regression guards (§11.4.135) registered in the same commit
> as each fix.

---

## 0. Goal (restated from roadmap §SP2)

Under **ONE root**, expose as uniformly-named, individually-addressable selection targets:
1. the **AI-debate ensemble** (aggregate + named presets),
2. **HelixLLM** promoted to a first-class root entry (when `USE_HELIX_LLM=true`),
3. **every discovered provider individually**, and
4. **each provider's WORKING (verified) models** (`DiscoveredModel.Verified == true`).

Plus: update provider/API SDKs to latest (deep-web-research §11.4.99); resolve the
gin `1.11.0 ↔ 1.12.0` module skew; ship docs/guides/graphs/SQL/website/video-course
deliverables and the full test-type matrix.

Naming scheme (analysis-C, confirmed against existing conventions):
`ensemble` · `ensemble/<preset>` · `helixllm` · `helixllm/<model>` ·
`<provider>` · `<provider>/<model_id>` (preserving already-namespaced ids like
`openrouter/x-ai/grok-4`).

---

## 1. Verified architecture grounding (re-confirmed against live source, 2026-06-10)

All re-opened and confirmed this session; line numbers are current HEAD (a few drifted
±5 lines from the analysis doc — corrected here):

| Fact | Evidence (`file:line`) |
|------|------------------------|
| Exposure surface = Gin routes, all under `/v1` | `internal/router/router.go` (route block from ~:471) |
| **Ensemble** route (inline closure → `GetEnsembleService().RunEnsemble`) | `internal/router/router.go:676` (call at `:730-731`; object tag `"ensemble.completion"` `:740`; default `EnsembleConfig.Strategy="confidence_weighted"` `:686`) |
| **Providers** list (reads live `GetCapabilities().SupportedModels`) | `internal/router/router.go:773` (GET `""` closure `:773-797`, models field `:783`) |
| **Discovery** group (models/selected/stats/trigger/ensemble/debate-model) | `internal/router/router.go:1306-1313` |
| **Verification** group (LLMsVerifier) | `internal/router/router.go:1378-1387`; bridge importing live providers `:1356-1379` |
| Runtime registry `services.ProviderRegistry` | `internal/services/provider_registry.go:82` |
| HelixLLM = gated provider `"helixllm"` only (no root) | `internal/services/provider_registry.go:750-764` (`USE_HELIX_LLM` `:755`, model id `helixllm-default` `:757`) |
| `LLMProvider` interface — **NO `GetModels()`** | `internal/llm/provider.go:9-14` (only `Complete/CompleteStream/HealthCheck/GetCapabilities/ValidateConfig`) |
| "Working model" flag (verifier authoritative) | `internal/verifier/discovery.go:56` (`DiscoveredModel.Verified`; `OverallScore` `:59`); ensemble member shape `SelectedModel` `:66-72` |
| Discovery service API | `internal/verifier/discovery.go:23`; `GetSelectedModels()` / `GetDiscoveredModels()` |
| models.dev reference catalog (distinct from runtime lists) | `internal/modelsdev/service.go:13-23`, `client.go:14` |
| **D-1 BLUFF: `/v1/completion/models` hardcoded 3-model list** | `internal/handlers/completion.go:411` (`Models` func; literal maps `:413-436`: deepseek-coder, claude-3-sonnet-20240229, gemini-pro, `time.Now()` stamps) |
| helix_agent has **zero vendor SDKs** (48 hand-rolled `net/http` providers) | `submodules/helix_agent/go.mod` (no aws/azure/openai/anthropic/genai) |
| Cloud SDKs live in inner app | `helix_code/go.mod` (azcore v1.16.0 `:12`, azidentity v1.8.0 `:13`, aws-sdk-go-v2 v1.32.7 `:15`, bedrockruntime v1.23.1 `:18`, zep-go/v3 v3.10.0 `:26`, tree-sitter Aug-2024 pin `:39`, gin v1.11.0 `:27`, pgx v5.7.6, go-redis v9.17.2) |
| gin module skew | `helix_code/go.mod` gin **v1.11.0** vs `submodules/helix_agent/go.mod` gin **v1.12.0** (line 5) |

**Design consequence:** the unified-catalog work is a NEW handler + a join over three
already-existing sources (`ProviderRegistry.ListProviders()` → `GetCapabilities().SupportedModels`,
`verifier` discovery `Verified`/`OverallScore`, and the ensemble service). No source is
removed (§11.4.122); all current endpoints stay intact (additive).

---

## 2. Operator-input decisions (FLAGGED — resolve before implementation)

These are design forks the agent must NOT decide silently (§11.4.101 — reversible/low-blast
items defaulted; high-blast items blocked for operator):

| # | Decision | Default proposed (if operator silent) | Why it needs operator |
|---|----------|---------------------------------------|------------------------|
| OP-1 | **Root path for the unified catalog.** `GET /v1/catalog` (analysis-C illustrative) vs `/v1/exposure` vs `/v1/targets`. | `GET /v1/catalog` | Public API surface naming; appears in docs/website/SDK; hard to rename later. |
| OP-2 | **Add `GetModels(ctx) ([]Model, error)` to the `LLMProvider` interface** (T2.2.2) vs keep reading `GetCapabilities().SupportedModels` + verifier join only. | Keep interface unchanged for SP2; join via verifier (smaller blast radius — 48 providers would each need the new method). | Interface change touches all 48 providers + tests; CONST-036 single-source. |
| OP-3 | **Ensemble preset enumeration source.** Expose only `ensemble` + `ensemble/confidence_weighted`, or enumerate every `EnsembleConfig.Strategy` value as a preset. | Expose `ensemble` (alias `ensemble/auto`) + presets derived from the strategies actually wired in code. | Avoids advertising presets that aren't implemented (would be a §11.4 capability bluff). |
| OP-4 | **SDK update blast radius (P2.3).** Update ALL flagged SDKs to latest now, or only the high-value/security ones (bedrockruntime, azidentity) this cycle. | Update security/auth + high-value (azidentity, azcore, bedrockruntime, aws-sdk-go-v2 core) this cycle; defer cosmetic minors with a tracked item. | Major-version bumps can break build; §11.4.92 multi-pass + §11.4.108 four-layer needed per bump. |
| OP-5 | **gin skew resolution direction (P2.3).** Bump `helix_code` 1.11.0 → 1.12.0 to match helix_agent, or pin both to a single researched-latest. | Align both to the deep-web-researched latest gin v1.x (likely ≥1.12.0). | Touches both modules' build + middleware; needs the §11.4.99 latest-source check first. |
| OP-6 | **Video-course + website production** (P2.4) — full production vs scripted-outline + asset stubs this cycle. | Author script/storyboard + doc-graph assets; flag full video render as operator-gated. | Time/resource budget; may need external tooling/credentials. |

---

## 3. Phase / task structure (each task = RED → impl → GREEN+evidence → rollback)

Subagent-driven by default (§11.4.70/§11.4.103); ≥3 parallel non-contending streams.
Phases ordered by dependency; P2.0 (D-1 bluff) lands FIRST as the highest-risk item
(§11.4.132 risk-ordered).

### P2.0 — D-1 bluff remediation: `/v1/completion/models` hardcoded list

Highest-risk, highest-visibility; a live CONST-036 / BLUFF-002 violation. Do first.

- **T2.0.1 RED — reproduce the bluff.** New test `internal/handlers/completion_models_test.go`
  with `RED_MODE` polarity switch (§11.4.115). `RED_MODE=1`: assert the response body is
  EXACTLY the 3 hardcoded ids (deepseek-coder / claude-3-sonnet-20240229 / gemini-pro) and
  that the set does NOT change when the registry/verifier is reconfigured → proves the list is
  static (the defect present on the pre-fix artifact). Capture output to
  `docs/qa/SP2-T2.0.1/`.
- **T2.0.2 IMPL — verifier-sourced list.** Rewrite `CompletionHandler.Models`
  (`internal/handlers/completion.go:411`) to return the live list from the verifier
  (`GetDiscoveredModels()` filtered `Verified==true`) joined with `ProviderRegistry.ListProviders()`
  capabilities — never a literal map (CONST-036). Inject the registry/verifier into the handler
  (constructor wiring) rather than hardcode. Define cold/verifier-down behaviour: return the
  registry capabilities set with `verified:false` markers, never a fabricated "working" list
  (§11.4 PASS-bluff guard, mirrors SP1 P1.3.3).
- **T2.0.3 GREEN + evidence.** Flip `RED_MODE=0`: the same test becomes the standing
  regression guard (§11.4.135) — asserts the list now reflects registry/verifier state and
  CHANGES when a provider is enabled/disabled. Live integration run against a real verifier
  (real key where available) captured to `docs/qa/SP2-T2.0.2/`. Register guard
  `ATM-SP2-D1` in the standing suite.
- **Rollback:** revert `completion.go` to the literal-map version (single-commit, behind the
  test) if GREEN cannot be achieved; tracked item stays open. No data migration involved.

### P2.1 — Unified catalog (the missing join layer)

- **T2.1.1 RED — absence test.** New test asserting there is currently NO single endpoint
  returning ensemble + helixllm + every provider + every verified model as uniformly-named
  entries (`RED_MODE=1` proves the 404 / absence). Evidence `docs/qa/SP2-T2.1.1/`.
- **T2.1.2 IMPL — catalog assembler.** New `internal/catalog/catalog.go`:
  `type Entry struct { Name string; Kind string /*ensemble|provider|model*/; Provider string;
  Verified bool; OverallScore float64; Enabled bool }` and `BuildCatalog(reg, verifier, ensembleSvc) []Entry`.
  Join rules: (a) one `ensemble` entry (+ presets per OP-3); (b) one `helixllm` entry +
  `helixllm/<model>` when enabled (OP from §P2.2.4); (c) one entry per
  `ProviderRegistry.ListProviders()`; (d) one `<provider>/<model_id>` entry per
  `DiscoveredModel` with `Verified==true` (NEVER static `SupportedModels`, NEVER hardcoded).
- **T2.1.3 IMPL — handler + route.** New `internal/handlers/catalog.go` `CatalogHandler.List`;
  register `GET /v1/catalog` (OP-1) inside the protected group near
  `internal/router/router.go:773` (alongside `/v1/providers`). Additive; no existing route changed.
- **T2.1.4 IMPL — selector resolution.** Extend the existing free-text `model` field
  (`CompletionRequest.Model`, `internal/handlers/completion.go:28`; ensemble closure
  `router.go:702`) so any catalog `Name` resolves to the right target: `ensemble[/preset]` →
  ensemble service; `helixllm[/model]` → helixllm provider; `<provider>[/<model>]` → that
  provider. Unknown selector → 400 with the candidate list (no silent guess, §11.4.6/§11.4.105).
- **T2.1.5 GREEN + evidence.** `RED_MODE=0` guard asserts the catalog lists all four classes
  and that each `Name` is selectable end-to-end (real proxied completion where a key exists).
  Liveness/captured evidence per §11.4.107 → `docs/qa/SP2-T2.1/`. Guard `ATM-SP2-CATALOG`.
- **Rollback:** new package + route are isolated; remove the route registration line + revert
  selector-resolution shim. Existing endpoints unaffected.

### P2.2 — Per-provider individual exposure + HelixLLM promotion

- **T2.2.1 Provider catalog entry per discovered provider** (verifier-sourced) — covered by
  the catalog join (T2.1.2 clause c). Test: every `ListProviders()` name appears as a `kind:provider`
  entry with correct `enabled` flag.
- **T2.2.2 Per-provider working-model list** (`Verified==true`). Decision OP-2: default = derive
  from verifier join (no interface change). If operator chooses to add `GetModels(ctx)` to
  `LLMProvider` (`internal/llm/provider.go:9`), that becomes a SEPARATE sub-plan updating all
  48 providers + their tests + the registry — RED-first per provider, evidence per provider.
- **T2.2.3 Naming scheme enforcement.** Test the canonical grammar against fixtures:
  `ensemble`, `ensemble/confidence_weighted`, `helixllm`, `helixllm/helixllm-default`,
  `anthropic/claude-3-sonnet-20240229`, `openrouter/x-ai/grok-4`, `groq`, `deepseek/deepseek-coder`.
  Truth-table test + a cell-flip mutation (§1.1) forcing FAIL (e.g. an UpperCase provider name must
  be rejected/normalised). Reuse existing lowercase names + the `vendor/model` slash form already
  present at `provider_registry.go:791` (`x-ai/grok-4`).
- **T2.2.4 Promote HelixLLM to first-class root.** Catalog emits a top-level `helixllm` entry
  (and `helixllm/<model>`) when `USE_HELIX_LLM=true` (`provider_registry.go:755`), NOT buried.
  Test: with the flag on, `helixllm` is a selectable root target; with it off, it is absent
  (no fabrication). Evidence `docs/qa/SP2-T2.2/`.
- **Rollback:** entries are produced by the join in T2.1.2; reverting that function reverts these.

### P2.3 — SDK / API currency (clearly-bounded sub-plan)

> **Scope nuance (load-bearing):** `submodules/helix_agent` has **NO vendor SDKs** — its 48
> providers are hand-rolled `net/http`, so "update provider SDKs" there means **API-drift review
> of the hand-written clients vs each vendor's CURRENT API reference**, not a `go.mod` bump.
> The cloud SDKs that CAN age out live in **`helix_code/go.mod`**. This sub-plan therefore spans
> both modules.

- **T2.3.1 Deep-web-research latest versions (§11.4.99).** For each flagged dep determine the
  TRUE latest via authoritative source (Go module proxy / vendor release notes), capture URLs +
  date in a `## Sources verified <date>` footer. Targets: `azcore` (v1.16.0), `azidentity`
  (v1.8.0 — auth/security, high priority), `aws-sdk-go-v2` core (v1.32.7), `aws-sdk-go-v2/config`
  (v1.28.7), `credentials` (v1.17.48), `bedrockruntime` (v1.23.1 — high value, new-model access),
  `getzep/zep-go/v3` (v3.10.0), `smacker/go-tree-sitter` (Aug-2024 pseudo-version — stale-likely;
  same pin in BOTH go.mods), `gin` (v1.11.0 helix_code / v1.12.0 helix_agent), `pgx/v5`
  (v5.7.6 / v5.9.2), `go-redis/v9` (v9.17.2 / v9.18.0). Output: `docs/research/SP2-sdk-currency/`.
- **T2.3.2 Hand-rolled provider API-drift audit.** For each of the 48 `internal/llm/providers/*`,
  diff the implemented request/response shape vs the vendor's current API reference; open a
  tracked item per drift found (no silent assumption — §11.4.6). Prioritise providers with live
  keys for real-call verification.
- **T2.3.3 Bump cloud SDKs in `helix_code/go.mod`** per OP-4 decision. Per bump: RED (capture
  current build/behaviour) → `go get <dep>@<latest>` → `go mod tidy` → build + full unit/integration
  sweep → §11.4.92 multi-pass blast-radius → §11.4.108 four-layer (source/artifact/runtime/user-visible).
  Each bump is its OWN commit (revertible). Evidence: build logs + a real Bedrock/Azure call where
  credentials exist.
- **T2.3.4 Resolve gin skew (OP-5).** Align `helix_code` (v1.11.0) and `helix_agent` (v1.12.0) to
  the researched latest gin v1.x. Run BOTH modules' route + middleware tests post-bump; capture
  green output. RED: a test asserting the two go.mods currently disagree (`RED_MODE=1`), GREEN:
  they agree post-fix.
- **Rollback:** every SDK bump is an isolated commit; `git revert` the specific bump + `go mod tidy`.
  No data migration. The hand-rolled-client audit produces tracked items only (no code change in this task).

### P2.4 — Deliverables: docs / guides / graphs / SQL / website / video-course

All exported with `.html` + `.pdf` siblings via the Docs Chain engine (§11.4.106) + revision
headers (§11.4.44).

- **T2.4.1 API reference + user guide** for `GET /v1/catalog` + the selector grammar +
  HelixLLM-as-root + per-provider/per-model targeting. Cross-reference vendor API docs verified
  in T2.3.1 (§11.4.99 footer).
- **T2.4.2 Architecture graphs** (mermaid/graphviz): the unified-catalog join (registry +
  verifier + ensemble → catalog → selector resolution); the four item-classes namespace tree.
- **T2.4.3 SQL** — if the catalog is persisted/cached, schema + sample queries for the
  catalog/verified-model tables; otherwise an SQL view modelling the catalog join for the
  workable-items / reporting DB. (Confirm persistence need with OP-1 design.)
- **T2.4.4 Website** — update `github_pages_website/` provider/model exposure pages to list the
  new namespace + examples (`ensemble`, `helixllm`, `<provider>/<model>`).
- **T2.4.5 Video-course** — script/storyboard for "selecting an exposure target" (OP-6 gates full
  render). Assets under `docs/qa/SP2-T2.4/`.

### P2.5 — Full test-type matrix (cross-cutting, binds every task)

Per §11.4.27 / §11.4.50 / §11.4.85 / §11.4.98 / SP7. Each row = real infra (no mocks beyond
unit), captured evidence, paired §1.1 mutation, fully automated (§11.4.98).

| Test type | SP2 coverage |
|-----------|--------------|
| unit | catalog assembler join logic; selector grammar truth-table; Models-handler verifier-sourcing |
| integration | `GET /v1/catalog` against real `ProviderRegistry` + real verifier (real keys where available); selector → real completion |
| e2e | full user path: discover catalog → pick `ensemble` / `helixllm` / `<provider>/<model>` → real completion result |
| full-automation | self-driving, re-runnable `-count=3`, no human step (§11.4.98) |
| security | authz on `/v1/catalog` (behind `auth.Middleware`); no key/secret leakage in catalog output |
| ddos / scaling | catalog endpoint under load (delegated to HelixQA/Challenges per SP7 — evidence-backed, not config-only) |
| chaos / stress | verifier-down / registry-empty / partial-provider-failure → catalog degrades honestly (no fabricated "working" list); §11.4.85 injection |
| performance / benchmark | catalog assembly latency with N providers × M models |
| ui / ux | website exposure pages render the namespace correctly (CV/OCR per §11.4.117 if needed) |
| Challenges | `challenges/` bank entry exercising the unified catalog end-to-end |
| HelixQA | autonomous QA session over the new endpoints with captured wire evidence (SP7 P7.3) |
| regression-guards | `ATM-SP2-D1`, `ATM-SP2-CATALOG`, naming-grammar guard registered in standing suite (§11.4.135) |

### P2.6 — Code-review + release gating (§11.4.125 / §11.4.134)

- **T2.6.1** Dispatch code-review agent(s) over the full SP2 batch BEFORE pre-build + main build.
  Iterate until clean GO with ZERO findings AND ZERO warnings (§11.4.134), each round carrying
  captured physical evidence.
- **T2.6.2** Pre-build readiness verdict (§11.4.110) + clash detection (new handler ⇄ route
  registration ⇄ DI wiring). Coverage-completeness gate: every changed file → ≥1 gate + ≥1 test +
  ≥1 §1.1 mutation.
- **T2.6.3** Update `docs/CONTINUATION.md` + the session-resumption file (§11.4.131 / CONST-044)
  in the same commit as each state-advancing change.

---

## 4. Files to create / modify

| Path | Action | Purpose |
|------|--------|---------|
| `submodules/helix_agent/internal/handlers/completion.go` (:411 `Models`) | MODIFY | Replace hardcoded 3-model list (D-1) with verifier/registry-sourced list (CONST-036) |
| `submodules/helix_agent/internal/handlers/completion_models_test.go` | CREATE | RED→GREEN polarity test for D-1 (§11.4.115) |
| `submodules/helix_agent/internal/catalog/catalog.go` | CREATE | `Entry` type + `BuildCatalog()` join (ensemble+helixllm+providers+verified models) |
| `submodules/helix_agent/internal/catalog/catalog_test.go` | CREATE | Join logic + naming-grammar truth-table + §1.1 mutation |
| `submodules/helix_agent/internal/handlers/catalog.go` | CREATE | `CatalogHandler.List` for `GET /v1/catalog` |
| `submodules/helix_agent/internal/handlers/catalog_test.go` | CREATE | Handler integration test (real registry+verifier) |
| `submodules/helix_agent/internal/router/router.go` (near :773) | MODIFY | Register `GET /v1/catalog` (additive); wire selector resolution into ensemble/completion closures (:676/:702) |
| `submodules/helix_agent/internal/handlers/completion.go` (:28 selector) | MODIFY | Selector resolution for catalog `Name` → target |
| `submodules/helix_agent/internal/services/provider_registry.go` (:750-764) | MODIFY (minimal) | Surface helixllm enabled-state for catalog promotion (read-only; no removal §11.4.122) |
| `submodules/helix_agent/internal/llm/provider.go` (:9) | MODIFY *(OP-2 only)* | Add `GetModels(ctx)` to interface — gated on OP-2; then 48 provider impls + tests |
| `helix_code/go.mod` / `go.sum` | MODIFY | SDK bumps (azidentity/azcore/bedrockruntime/aws-sdk-go-v2/zep/tree-sitter/pgx/go-redis) per OP-4; gin align per OP-5 |
| `submodules/helix_agent/go.mod` / `go.sum` | MODIFY | gin align per OP-5; tree-sitter pin per T2.3.1 |
| `docs/research/SP2-sdk-currency/*.md` | CREATE | §11.4.99 latest-source research + `Sources verified` footers |
| `docs/guides/SP2-exposure-catalog.md` (+ `.html`/`.pdf`) | CREATE | API reference + selector grammar + user guide |
| `docs/diagrams/SP2-catalog-join.*` | CREATE | Architecture graphs |
| `docs/sql/SP2-catalog-view.sql` | CREATE | Catalog view / cache schema (if persisted — confirm OP-1) |
| `github_pages_website/...` | MODIFY | Exposure-namespace pages |
| `challenges/...` | CREATE | End-to-end Challenge for the unified catalog |
| `docs/qa/SP2-*/` | CREATE | Captured runtime evidence per task |
| `docs/CONTINUATION.md` + session-resumption file | MODIFY | CONST-044 / §11.4.131 sync per state-advancing commit |

---

## 5. Ordered fine-grained task list

1. **T2.0.1** RED: reproduce D-1 hardcoded-list bluff (polarity `RED_MODE=1`), capture evidence.
2. **T2.0.2** IMPL: verifier/registry-source `CompletionHandler.Models`; define verifier-down honest behaviour.
3. **T2.0.3** GREEN: flip guard, live run, register `ATM-SP2-D1` in standing suite.
4. **T2.1.1** RED: catalog-absence test.
5. **T2.1.2** IMPL: `catalog.BuildCatalog()` join (4 item classes; `Verified==true` only).
6. **T2.1.3** IMPL: `CatalogHandler.List` + register `GET /v1/catalog` (OP-1).
7. **T2.1.4** IMPL: selector resolution for catalog `Name` into ensemble/helixllm/provider targets.
8. **T2.1.5** GREEN: end-to-end selectability, evidence, guard `ATM-SP2-CATALOG`.
9. **T2.2.1** Provider-entry-per-provider test.
10. **T2.2.4** HelixLLM first-class root promotion (flag-on/flag-off honesty test).
11. **T2.2.3** Naming-grammar truth-table + §1.1 mutation.
12. **T2.2.2** *(OP-2 gated)* per-provider working-model list; if interface change chosen, spawn the 48-provider `GetModels` sub-plan.
13. **T2.3.1** Deep-web-research latest SDK/API versions (§11.4.99); capture sources.
14. **T2.3.2** Hand-rolled 48-provider API-drift audit → tracked items.
15. **T2.3.4** Resolve gin 1.11.0↔1.12.0 skew (OP-5), both modules' tests green.
16. **T2.3.3** Bump cloud SDKs in `helix_code/go.mod` per OP-4 (each bump = own commit + four-layer verify).
17. **T2.4.1–T2.4.5** Docs / graphs / SQL / website / video-course deliverables (OP-6 gates render).
18. **P2.5** Full test-type matrix execution (real infra, captured evidence, automated `-count=3`).
19. **T2.6.1** Code-review iterate-until-GO (§11.4.134).
20. **T2.6.2** Pre-build readiness verdict + clash + coverage-completeness gates.
21. **T2.6.3** CONTINUATION + resumption-file sync; commit/push merge-onto-latest-main (§11.4.113, no force).

---

## 6. Rollback summary (per phase)

- **P2.0:** revert `completion.go:411` to literal-map (single commit behind the RED test).
- **P2.1/P2.2:** new isolated package `internal/catalog/` + one route registration + one selector
  shim — remove the route line + revert the shim; all existing endpoints untouched (additive design).
- **P2.3:** each SDK bump is its own revertible commit (`git revert` + `go mod tidy`); the API-drift
  audit emits tracked items only.
- **P2.4:** docs/website are additive; revert the doc commits.
- No destructive DB/data ops anywhere in SP2; no capability removed (§11.4.122).

---

## 7. Anti-bluff / constitutional binding

- **CONST-036 / BLUFF-002:** D-1 fix + all catalog model sources are verifier/registry-derived;
  zero hardcoded model lists. "Working" = `DiscoveredModel.Verified==true` ONLY.
- **§11.4.115:** every fix RED-on-broken-artifact first with `RED_MODE` polarity switch.
- **§11.4.135:** every closed defect registers a permanent regression guard in the same commit.
- **§11.4.107:** PASS requires captured liveness evidence, never a single screenshot / metadata-only.
- **§11.4.99:** SDK/API currency uses latest-source web research with cited `Sources verified` footers.
- **§11.4.122:** HelixLLM is PROMOTED, never removed; all existing routes preserved.
- **§11.4.125/§11.4.134:** code-review iterate-until-GO before pre-build + main build.
- **§11.4.113:** merge-onto-latest-main; no force-push on any repo/submodule.
- **CONST-044 / §11.4.131:** CONTINUATION + resumption file synced each state-advancing commit.

---

## 8. Summary (10 lines)

1. SP2 adds ONE unified-catalog root joining ensemble + HelixLLM + every provider + every VERIFIED model as uniformly-named selectable targets — the only missing layer; all building blocks exist.
2. Exposure surface confirmed = `submodules/helix_agent/internal/router/router.go` (Gin `/v1`); ensemble `:676`, providers `:773`, discovery `:1306-1313`, verification `:1378-1387`.
3. Highest-risk first: P2.0 fixes the D-1 hardcoded 3-model bluff at `internal/handlers/completion.go:411` (CONST-036/BLUFF-002), RED-first + standing guard.
4. New code is additive + isolated (`internal/catalog/` + `CatalogHandler` + `GET /v1/catalog` per OP-1); no existing route or capability removed (§11.4.122).
5. Naming scheme reuses existing lowercase provider names + the `vendor/model` slash form (`openrouter/x-ai/grok-4`); "working" = `DiscoveredModel.Verified==true`.
6. HelixLLM is promoted to a first-class root (`helixllm[/model]`) when `USE_HELIX_LLM=true`, flag-honest (no fabrication when off).
7. SDK currency is a bounded sub-plan: helix_agent has ZERO vendor SDKs (hand-rolled HTTP → API-drift audit), cloud SDKs bumped in `helix_code/go.mod`; gin 1.11.0↔1.12.0 skew resolved (OP-5).
8. Six operator-input decisions flagged (OP-1 root path, OP-2 interface `GetModels`, OP-3 presets, OP-4 SDK blast radius, OP-5 gin direction, OP-6 video/website production).
9. Full deliverable set: docs/guides/graphs/SQL/website/video-course + the complete test-type matrix with captured evidence, paired §1.1 mutations, and fully-automated re-runnable tests.
10. Every closure is RED-first (§11.4.115) + regression-guarded (§11.4.135) + code-reviewed-until-GO (§11.4.134) + evidence-backed (§11.4.107); merge-onto-latest-main, no force-push (§11.4.113).

**Task count:** 21 ordered fine-grained tasks across 7 phases (P2.0–P2.6); 6 flagged operator-input decisions; ~18 file create/modify targets.
