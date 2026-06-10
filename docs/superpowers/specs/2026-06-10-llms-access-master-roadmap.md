# Master Roadmap — Models-Access Refinement & Agents-Work Enforcement

| Field | Value |
|-------|-------|
| Revision | 3 |
| Last modified | 2026-06-10 |
| Status summary | PLANNING COMPLETE — 6 analyses + 5 SP plans written; constitution synced; SP3 plan + bundle code-review in flight; awaiting operator on decision register (G-1..G-5) |
| Source request | `docs/requests/helix_code_request_llms_access.txt` |
| Delivery model | **Independent sub-programs** (operator decision 2026-06-10) — this doc is the index/roadmap; each sub-program gets its own spec → plan → release cycle |
| Authority | Cascades from `constitution/` + root `CLAUDE.md`/`CONSTITUTION.md`; anti-bluff §11.4 family governs every closure |

> **Anti-bluff note (§11.4 / §11.4.123):** This roadmap is a *plan*, not a claim of done work. No phase below is "complete" until it carries captured runtime evidence per §11.4.5 / §11.4.69 / §11.4.107. The brainstorming HARD-GATE applies: **no implementation, fork creation, or push happens until each sub-program's design+plan is approved.**

---

## 0. Operator decisions baked in (2026-06-10)

1. **Fork strategy:** Fork **ALL** ~50 `cli_agents/*` third-party submodules **now** under the `vasic-digital` org with prefix **`caf-`** (cli-agent-fork) → e.g. `git@github.com:vasic-digital/caf-claude-code.git`. Swap each submodule's URL to our fork. Auto-merge-from-upstream for every fork.
2. **Delivery:** Independent sub-programs; this is the master index.
3. **Bridge exposure:** **One provider per CLI agent** — each working CLI agent surfaces its own dynamically-discovered model list + power-features (Vision / Generative / RAG / Memory / MCP / LSP / ACP).
4. **New "ways-of-working" submodules** (parallel-agents, dynamic-flows, …): under the **HelixDevelopment** org.
5. **Constitution DESCOPED (2026-06-10):** no governance-rule authoring, no constitution push — **sync only** (fetch+pull to current). Reusable fork/merge scripts live **local to HelixCode** (`scripts/caf/`), not in `constitution/scripts/`.

---

## 1. Architecture grounding (verified from repo, 2026-06-10)

- **HelixAgent is the "root"** the request extends — `submodules/helix_agent/internal/` already contains `ensemble`, `llm`, `router`, `verifier`, `providers`, `modelsdev`, `models`, `clis`, `mcp`, `rag`, `memory`, `agentic`, `agents`, `concurrency`, `planning`, `background`. (Verified by `ls`.)
- **LLMsVerifier** (`submodules/llms_verifier`) is the single source of truth for working models (CONST-036/040) — all dynamic discovery flows through it; hardcoded model lists are violations.
- **Inner Go app** (`helix_code/`) has `internal/llm/*_provider.go` (anthropic, azure, bedrock, copilot, deepseek, gemini, groq, …), `internal/provider`, `internal/providers`, `internal/agent`.
- **Constitution scripts** live in `constitution/scripts/` (codegraph, action-prefix, subagent-tiering, token-accounting, workable-items, hooks). **No fork-maintenance scripts exist yet** — Sub-program SP0 adds them.
- **~50 CLI agents** under `cli_agents/` as third-party submodules (verified `.gitmodules`).

Per-workstream evidence-backed analyses (produced by 6 parallel subagents, 2026-06-10) land in `docs/superpowers/specs/analysis/2026-06-10-{A..F}-*.md` and feed the detailed sub-program specs.

---

## 1a. Real defects surfaced during planning (evidence-backed — to be fixed inside the relevant SP, RED-first §11.4.115)

These are concrete §11.4 anti-bluff / CONST violations found by the analysis subagents with `file:line` evidence — independent of the new features, they are pre-existing defects:

| ID | Defect | Evidence | Owner SP |
|----|--------|----------|----------|
| D-1 | `/v1/completion/models` returns a **hardcoded 3-model list** | `helix_agent/internal/handlers/completion.go:406` | SP2 (CONST-036/BLUFF-002) |
| D-2 | CLI `handleListModels` lists `failed`/`pending` models as **available** | `helix_code/.../cmd/cli/main.go:1361` | SP1 |
| D-3 | `secrets.LoadAPIKeys` is **DEAD code** (never called in prod) | `helix_code/internal/secrets/loader.go:30` | SP1 |
| D-4 | "Working-model" filter (`Verified ∧ score≥min`) **loaded but never applied** | `adapter.go:175` | SP1 |
| D-5 | **Hardcoded provider model lists** (OpenAI/Anthropic/DeepSeek/Mistral +~8) | `openai_provider.go:203`, `anthropic_provider.go:205`, … | SP1 (CONST-036) |
| D-6 | Stub CLI-agent packages return **hardcoded strings, never exec** the binary | `helix_agent/internal/clis/agents/qwencode/qwencode.go:101/115/137` | SP4 (CONST-035) |
| D-7 | `helix_code/go.mod` lacks `replace dev.helix.agent` → can't import the real `clis`/`agentic` substrate | `helix_code/go.mod` | SP4/SP5 |
| D-8 | Constitution `CLAUDE.md` Revision-table drift | `constitution/CLAUDE.md` | ~~SP0~~ resolved by sync (now Rev 21 @ f26368b) |
| D-9 | **Pooled dispatch methods are stubs too** — return `"<Agent> execution completed"`, no exec | `helix_agent/internal/clis/instance_manager.go:906-942` (executeAider/ClaudeCode/Codex/Cline/OpenHands) | ✅ FIXED (SP4, real os/exec, pin guards GREEN) |
| D-6 | qwencode stub returns templated strings, never execs | `helix_agent/.../qwencode/qwencode.go:101` | ✅ FIXED (SP4, real os/exec) |
| D-10 | 2 pre-existing flaky tests (nanosecond ID-collision) | `helix_agent` `TestCompletionHandler_IDGeneration`, `TestGenerateTeamID` | open (pre-existing, low-pri) |
| D-11 | pre-existing `"For now, allow all types"` bluff | `helix_agent/internal/clis/instance_manager.go:389` (IsAgentTypeAvailable) | open (SP4 follow-up) |
| D-12 | ~34 more `execute*` dispatch methods still stubbed | `helix_agent/internal/clis/instance_manager.go` (executeKiro…executeLlamaCode) | open (SP4 follow-up wave) |
| D-13 | CONST-066 doc-export gap: 95+ `docs/**/*.md` (+ new specs/research) lack `.html`/`.pdf` siblings | repo-wide `docs/**` | open (D-13; F1 hook scoped too leniently — reconcile) |

## 2. Sub-program decomposition

Each sub-program (SP) is independent: own spec, own plan, own evidence-backed release. Dependency order shown.

```
SP0 Constitution + fork-mandate + reusable scripts   (must land first — CONST-049)
        │
        ├──> SP1 Model-access refinement (keys → working models via LLMsVerifier)
        │        │
        │        └──> SP2 HelixAgent exposure extension (each provider+models, ensemble, HelixLLM)
        │                 │
        ├──> SP3 Fork-ALL mechanism + auto-merge scripts ──┐
        │                                                  ▼
        │                                          SP4 CLI-agent bridge providers
        │                                            (one provider per agent + power features)
        │
        ├──> SP5 Parallel/subagent-driven enforcement (coordinated multi-agent execution)
        │
        └──> SP6 Other execution models (dynamic flows…) as new HelixDevelopment submodules

SP7 Testing & QA  ── cross-cutting, binds every SP (100% test-type coverage + Challenges + HelixQA + autonomous QA)
```

---

## SP0 — Constitution SYNC ONLY (DESCOPED per operator 2026-06-10)
**Operator decision 2026-06-10:** *"We dont have need to change the constitution Submodule at all… just make constitution Submodule up to date, fetch and pull."* → **No `§11.4.N` authoring, no constitution push.** The reusable fork/merge scripts relocate to **`scripts/caf/` local to HelixCode** (not `constitution/scripts/`). Analysis-A's drafted anchor is archived, unused (and `§11.4.142` was in any case already taken upstream by the Universal-code-review mandate).

**Status: DONE (this session).**
- ✅ Fetched all 8 constitution remotes; **fast-forwarded `60e2d66 → f26368b` on `main`** (0 ahead / 4 behind / ancestor → no loss, §11.4.113 no-force).
- ✅ Recursive check: **no nested constitution submodules** (root-only, CONST-051 clean); `submodules/*/CONSTITUTION.md` are cascade file-copies.
- ✅ New anchors now in force: **§11.4.142 Universal code-review** (every change → independent review before commit/build — applies to all SP implementation), **§11.4.143 video-journey** (N/A to HelixCode).
- ⏳ **Pending follow-up:** bump superproject submodule pointer `60e2d66 → f26368b` (a meta-repo commit; itself subject to §11.4.142 review) — batch with next commit.
- ⏳ **Requested but not yet run** (request "Before we begin"): CONST-055 post-pull full-rule validation sweep across project + submodules; correct any violation found. → tracked as **SP0.b**.

### ~~Original SP0 (superseded)~~ — Constitution mandate + scripts
**Goal:** Land the new mandatory rule + generic reusable scripts in the constitution submodule BEFORE building on them (CONST-049 ordering). Feeds: analysis `…-A-…`. *(Superseded by the descope above; the script designs from analysis-A/E are reused but land in `scripts/caf/` local to HelixCode under SP3.)*

- **P0.1 Fetch/pull constitution (deep, recursive).**
  - T0.1.1 `git -C constitution fetch --all --prune`; pull; capture `HEAD..@{u}`.
  - T0.1.2 Recurse: `git submodule foreach --recursive` locate every nested `constitution`/governance copy; fetch/pull each. Evidence: per-path log.
  - T0.1.3 Run `scripts/verify-all-constitution-rules.sh` (CONST-055) post-pull; record verdict.
- **P0.2 Audit project + submodules against ALL mandatory rules** (request §"Before we begin").
  - T0.2.1 Run cascade verifier; enumerate any skipping/violation; open tracker items.
  - T0.2.2 Correct every detected violation (own sub-tasks per finding).
- **P0.3 Author the new fork-and-maintain anchor — §11.4.142** (next free number CONFIRMED by analysis-A; gate `CM-COVENANT-114-142-PROPAGATION`). ⚠️ Constitution submodule is **detached HEAD @ `60e2d66`** → `git checkout main` inside it FIRST; fix `CLAUDE.md` Revision-table drift (lags at Rev 23) per §11.4.44 in the same edit.
  - T0.3.1 Use analysis-A's drafted anchor (forensic anchor = verbatim request lines 14–15; 6-clause body: fork+prefix → pointer-swap → recursive nested fork → automated upstream-merge → §11.4.28(B)-preserving edit allow-list [gitignore/remote-config/CI-disable/governance-pointers ONLY] → anti-bluff; composes §11.4.113 no-force-push).
  - T0.3.2 Cascade into `constitution/{Constitution,CLAUDE,AGENTS}.md` + `QWEN.md`; consumer-side index updates.
  - T0.3.3 Meta-test + paired §1.1 mutation for the propagation gate + 3 recommended per-family gates.
- **P0.4 Reusable generic scripts** in `constitution/scripts/` (decoupled CONST-051; inherited-by-reference §11.4.80 — never copied). Names per analysis-A:
  - T0.4.1 `fork_third_party_submodule.sh` (gh+glab fork-or-create under org+prefix `caf-`, params only, no force §11.4.113).
  - T0.4.2 `update_fork_from_upstream.sh` (fetch upstream main/master → merge into fork → push fast-forward-only).
  - T0.4.3 `resolve_recursive_fork_deps.sh` (recursive nested-dep handling; own-org → root per CONST-051; 3rd-party nested → fork).
  - T0.4.4 Anti-bluff Challenge + §1.1 mutation per script (throwaway repo bootstrap → assert remotes/merge).
- **P0.5 Commit + push constitution to ALL upstreams; bump pointer** (CONST-049 steps 4–7).

**Done when:** new anchor present + cascade-verified across fleet; scripts pass their Challenges with captured output; constitution pushed to all upstreams; meta-repo `.gitmodules` pointer bumped in same commit.

---

## SP1 — Model-access refinement
**Goal:** For every provider, an API key supplied OR auto-recognized from `.env` / `.api_keys` / shell exports makes that provider's **working** models available — dynamically obtained, validated, verified via LLMsVerifier. Feeds: analysis `…-B-…`.

> **Verified facts (analysis-B):** verifier consumption is REAL (`verifier/client.go` live HTTP; `adapter.GetVerifiedModels:183` cache→live→stale→fallback). But: `secrets.LoadAPIKeys` (`loader.go:30`, reads `api_keys.sh`+`.env`) is **DEAD code** (zero production call-sites); no key-presence gate; the "working-model" predicate (`Verified==true ∧ status=="verified" ∧ score≥GetMinAcceptableScore()`) is loaded (`adapter.go:175`) but **never applied**; CLI `handleListModels:1361` lists `failed`/`pending` as available. **The target flow already exists in `helix_agent` (`verifier/provider_types.go:349` `SupportedProviders.EnvVars` multi-alias + `startup.go:389` per-key→`DiscoverModels`) + `llms_verifier/api_keys/`** → lift/extend per §11.4.74.

- **P1.1 Key auto-recognition.**
  - T1.1.1 **Wire `secrets.LoadAPIKeys` at startup** in `cmd/server` + `cmd/cli/main.go` BEFORE config load (§11.4.124 — wire, don't delete the dead code).
  - T1.1.2 Multi-alias key-recognition table (reuse helix_agent `SupportedProviders.EnvVars`): `.env` → `.api_keys`/`api_keys.sh` → shell-exported precedence; per-provider detection.
  - T1.1.3 Tests (unit: precedence + absence; integration: real key → provider enabled).
- **P1.2 Key-presence → provider-availability gate** — filter the verifier catalog by present keys (no key ⇒ provider hidden); fix `factory.go:9` (single-provider) → enumerate every key-present provider.
- **P1.3 Dynamic working-model exposure via LLMsVerifier.**
  - T1.3.1 Apply the unused "working-model" filter (key-present ∧ `Verified==true` ∧ status verified ∧ score≥min); poll ≤60s (CONST-038); verified ≤24h (CONST-037).
  - T1.3.2 Replace hardcoded model lists (OpenAI:203, Anthropic:205, DeepSeek, Mistral, +~8) with verifier-sourced / dynamic fetch (CONST-036).
  - T1.3.3 Define safe verifier-disabled/cold behaviour — `FallbackModels` (CLI:1387) must NOT be shown as "working" without validation (else §11.4 PASS-bluff).
- **P1.4 Docs/guides/manuals** for key setup + model availability + verification flow.
- **P1.5 Test coverage** (unit/integration/e2e/Challenge) — real keys where available, anti-bluff liveness (§11.4.107).

---

## SP2 — HelixAgent exposure extension
**Goal:** Under one root expose: AI debate ensemble + HelixLLM (if enabled) + **every** discovered provider individually + each provider's working models; update provider SDKs to latest. Naming fits existing conventions. Feeds: analysis `…-C-…`.

> **Verified facts (analysis-C):** exposure surface = `helix_agent/internal/router/router.go` (Gin `/v1`): ensemble `POST /v1/ensemble/completions:676`, providers `GET /v1/providers:773` (reads `GetCapabilities().SupportedModels`), discovery `:1306`, verification `:1378`; runtime registry `services.ProviderRegistry` (`provider_registry.go:82`). `LLMProvider` interface has **no `GetModels()`** — models only via `GetCapabilities().SupportedModels`. 48 hand-rolled HTTP providers, no vendor SDKs. HelixLLM buried as gated provider `"helixllm"` (`USE_HELIX_LLM=true`). "Working model" = `DiscoveredModel.Verified == true` (verifier). Building blocks exist; **no unified catalog joins ensemble + HelixLLM + each provider + each verified model**. Inner Go module is at `helix_code/go.mod` (not `helix_code/helix_code/go.mod`).

- **P2.0 Bluff remediation:** `GET /v1/completion/models` (`handlers/completion.go:406`) returns a **hardcoded 3-model list** → replace with verifier-sourced (CONST-036 / BLUFF-002). RED→GREEN + §11.4.135 guard.
- **P2.1 Unified catalog** joining ensemble + HelixLLM + each provider + each verified model as uniformly-named selectable targets (the missing layer).
- **P2.2 Per-provider individual exposure** under the root alongside ensemble + HelixLLM.
  - T2.2.1 Provider catalog entry per discovered provider (verifier-sourced).
  - T2.2.2 Per-provider working-model list (`Verified==true`); consider adding `GetModels()` to the provider interface.
  - T2.2.3 Naming scheme (analysis-C): `ensemble`, `ensemble/confidence_weighted`, `helixllm`, `helixllm/helixllm-default`, `anthropic/claude-3-sonnet-20240229`, `openrouter/x-ai/grok-4` (reuses existing lowercase + `vendor/model` slash form).
  - T2.2.4 Promote HelixLLM to first-class root entry (not buried).
- **P2.3 SDK/API currency** — deep web research (§11.4.99 latest-source) → update each provider SDK to latest (bedrockruntime/azcore/azidentity/tree-sitter pin; resolve gin 1.11.0↔1.12.0 skew across helix_code/helix_agent); bridged CLI agents + their models up to date.
- **P2.4 Docs/guides/manuals/graphs/schemes/SQL** + website + video-course updates (request-mandated deliverables).
- **P2.5 Full test coverage.**

---

## SP3 — Fork-ALL mechanism + auto-merge
**Goal:** Fork every `cli_agents/*` under `vasic-digital/caf-*`, swap submodules to forks, recursively handle nested deps, auto-merge upstream regularly. Feeds: analysis `…-E-…`. Uses SP0 scripts.

> **Verified facts (analysis-E, parsed from 381-line `.gitmodules`):** 50 fork entries (49 top-level + 1 nested). **EXCLUSIONS:** `cli_agents/claude-code-source` is already the operator's own GitLab mirror (`milos85vasic/ccode-private`) → **skip fork** (default-excluded). Only ONE nested submodule fleet-wide: `cline/.gitmodules → cline-bench` (HTTPS → SSH `caf-cline-bench`); max depth 1, no own-org nested chains (CONST-051 clean). OpenAI-Cookbook uses `org-NNNN@github.com:` alias (out of scope, resources dir) — parser must normalize.

- **P3.1 Fork mapping table** — built (49 GitHub upstreams → `vasic-digital/caf-<name>`, kebab names preserved); GitHub+GitLab dual-remote.
- **P3.2 Execute fork-all** via the SP0 fork script (operator-gated; **high blast radius → §11.4.101 explicit confirm before any external mutation**; async fork-readiness polling).
- **P3.3 Submodule swap** (.gitmodules URL rewrite + `git submodule sync --recursive`), per-fork validate buildable.
- **P3.4 Nested-submodule resolution** (just `cline-bench` today; own-org → root per CONST-051; 3rd-party nested → fork).
- **P3.5 Auto-merge automation** (scheduled merge-into-fork-only, **no-force §11.4.113**, clean commits §11.4.30).
- **P3.6 Build + install each locally** with correct git-ignore hygiene (no stray files committed).
- **P3.7 Reconcile script naming** between analysis-A (`fork_third_party_submodule.sh`) and analysis-E (`caf_fork_all.sh` + `caf_lib.sh`) into one unified, decoupled set before authoring.

---

## SP4 — CLI-agent bridge providers (one provider per agent)
**Goal:** Each working CLI agent = its own provider proxying prompts via CLI + exposing power-features (Vision/Generative/RAG/Memory/MCP/LSP/ACP). Primary = system-installed; fallback = submodule-built. Re-export+install+validate every CLI config. Feeds: analysis `…-D-…`.

> **Verified facts (analysis-D):** 8 agents system-installed NOW (claude/qwen/opencode/gemini/crush/codex/goose/copilot — versions captured). The real `exec.LookPath` orchestration lives in `helix_agent/internal/clis` (`agents/base/base.go:120`, `instance_manager.go`), NOT in `helix_code/internal/llm` (its `copilot_provider.go` is HTTP/token, not CLI-exec). `llm.Provider` contract at `missing_types.go:356`. Re-export exists (`helixagent --generate-all-agents`, `main.go:4522`).

- **P4.0 Pre-req remediation (anti-bluff, BLOCKS the rest).**
  - T4.0.1 **Fix stub per-agent packages** in `helix_agent/internal/clis` that return hardcoded strings without exec (e.g. `agents/qwencode/qwencode.go:101/115/137`) — these are live §11.4 / CONST-035 bluffs. RED-on-stub → GREEN-on-real-exec (§11.4.115).
  - T4.0.2 Add `replace dev.helix.agent` wiring so HelixCode can import the `clis` layer (or thin adapter per CONST-051/§11.4.74).
- **P4.1 Per-agent provider contract** — one parametrized `CLIAgentProvider` implementing `llm.Provider`; reuse the `clis` layer via adapter, don't reimplement.
- **P4.2 Primary-vs-fallback selection** (real `exec.LookPath` system-installed → submodule-built).
- **P4.3 Power-feature passthrough** (RAG/Memory/MCP/LSP/ACP/Vision) + dynamic `GetModels` per agent (CONST-036) + capabilities from LLMsVerifier (CONST-040) — research per agent (§11.4.99): OpenCode/Zen non-interactive flags, Claude Code `-p/--output-format`, Qwen Code.
- **P4.4 Config re-export+install+validate** — re-export exists; **add the ABSENT filesystem-install + LIVE post-install validation** (each installed config drives a real proxied prompt → real result, captured evidence). Note: current configs point agents *at* HelixAgent (reverse) — D adds the forward bridge direction.
- **P4.5 Tier-1 agents first** (the 8 installed) → expand to ~43 candidates.
- **P4.6 Full test coverage** per agent (real proxied prompt → real result, anti-bluff §11.4.98 fully automated).

---

## SP5 — Parallel / subagent-driven enforcement
**Goal:** All HelixCode work can be done by coordinated parallel agents; subagents-driven used maximally without harming the main stream. Feeds: analysis `…-F-…`. As a new HelixDevelopment submodule if warranted.

> **Verified facts (analysis-F):** capability **exists twice, duplicated not shared** — HelixCode (`internal/agent/coordinator.go:11-50` registry+queue+circuit-breakers+voting, `agent/subagent/` in-process+subprocess+worktree spawners, `worker/` SSH pool, `workflow/` DAG) vs HelixAgent (`internal/agentic/workflow.go:28` LangGraph-style, `agents/swarm/`, `ensemble/`, `planning/{tree_of_thoughts,mcts,hiplan}`). **GAP:** `helix_code/go.mod` doesn't import the own-org agentic/concurrency/planning substrate; no universal "every operation is a parallel-dispatchable agent unit."

- **P5.1 Map existing multi-agent/worker/queue capability.** (done — analysis-F)
- **P5.2 De-duplicate into a shared substrate** (reuse before reimplement §11.4.74) — universal parallel-dispatchable agent-unit abstraction.
- **P5.3 Coordination layer** (dispatch, isolation, quiescence §11.4.84, PWU §11.4.58, §11.4.103 ≥3 streams).
- **P5.4 Wire into HelixCode + HelixAgent** (add the missing own-org imports).
- **P5.5 Full test coverage** (stress/chaos §11.4.85 for concurrency).

---

## SP6 — Other execution models (dynamic flows, …)
**Goal:** Dynamic flows + other execution models as NEW decoupled public submodules under HelixDevelopment, wired into HelixCode + HelixAgent. Feeds: analysis `…-F-…`.

> **Verified facts (analysis-F):** existing models = DAG workflow (HelixCode), graph/state-machine (`helix_agent/agentic`), ToT/MCTS/hierarchical planning. **ABSENT as named packages:** `pipeline`, standalone `dag`, dataflow, reactive, saga, behavior-tree. Reuse candidates already in tree. Proposed NEW HelixDevelopment submodules: **flow_engine, dag_orchestrator, pipeline_runtime, agent_mesh/swarm_kit, flow_dsl**; 12 deep-research topics enumerated.

- **P6.1 Deep web research** on execution models (dynamic flow + others) — 12 topics in analysis-F (§11.4.99 latest-source).
- **P6.2 Create public submodules** (gh+glab) under HelixDevelopment: flow_engine / dag_orchestrator / pipeline_runtime / agent_mesh / flow_dsl; decoupled (CONST-051), `helix-deps.yaml` (CONST-054). Reuse existing own-org pieces first (§11.4.74).
- **P6.3 Integration analysis + wiring** into both consumers (shared `Node`/`Scheduler`/`Dispatcher`/`Resolver` interfaces + `go.mod` replace).
- **P6.4 Full test coverage + Challenges.**

---

## SP7 — Testing & QA (cross-cutting)
**Goal:** 100% coverage across every supported test type + Challenges + HelixQA banks + fully-autonomous QA sessions; no bluff; independent-agent confirmation (CONST-050/048/098). Feeds: analysis `…-F-…`.

> **Verified facts (analysis-F):** HelixCode strong on unit/integration/e2e/security/chaos/stress/performance/benchmarking + §1.1 mutation meta-tests (`tests/stresschaos/stresschaos_meta_test.go`) + regression dir. **HelixCode-local GAPS:** ddos, scaling, ux have no local harness (HelixQA/challenge-only); ui thin → delegation risks a coverage bluff unless evidence-backed. ⚠️ **HelixQA is STALE** (README round 219 / 2026-05-19).

- **P7.1 Update HelixQA** submodule to latest (fetch/pull + pointer bump).
- **P7.2 Test-type coverage matrix** (unit/integration/e2e/full-automation/security/ddos/scaling/chaos/stress/performance/benchmark/ui/ux/Challenges/HelixQA) × every new feature.
- **P7.3 Autonomous QA sessions** executing every registered bank with captured wire evidence.
- **P7.4 Independent code-review agents** confirm each closure (§11.4.125/§11.4.134).
- **P7.5 Regression-guard registration** for every fixed defect (§11.4.135).

---

## 3. Cross-cutting mandatory constraints (apply to every SP)

- **Anti-bluff §11.4 family** — captured evidence on every closure; no metadata-only PASS.
- **No-force-push §11.4.113**; merge-onto-latest-main for every repo/submodule.
- **CONST-049** ordering for all constitution edits.
- **Subagent-driven default §11.4.70 / §11.4.103** — ≥3 parallel background streams while actionable items exist.
- **Fully-automated tests §11.4.98**; **rock-solid-proof-or-research §11.4.123**.
- **§11.4.122** — no silent removal of existing capabilities without operator confirmation.
- **CONST-046** — no hardcoded user-facing content; **CONST-036** — no hardcoded model lists.
- **Continuation §13.1 (CONST-044)** kept in sync each state-advancing commit.

---

## 4. Detailed implementation plans (written 2026-06-10)

| SP | Plan doc | Scope |
|----|----------|-------|
| SP1 | `plans/2026-06-10-SP1-model-access-plan.md` | 6 phases / 18 TDD tasks / 4 decisions |
| SP2 | `plans/2026-06-10-SP2-helixagent-exposure-plan.md` | 7 phases / 21 tasks / 6 decisions |
| SP3 | `plans/2026-06-10-SP3-fork-mechanism-plan.md` | 50-fork map + 3 scripts at `scripts/caf/` / tasks T3.0–T3.7 / G-1 gated |
| SP4 | `plans/2026-06-10-SP4-cli-bridge-plan.md` | 7 phases / 34 tasks / 4 decisions |
| SP5+SP6 | `plans/2026-06-10-SP5-SP6-parallel-dynamic-plan.md` | reuse-first; create 2 repos / 5 decisions |
| SP7 | `plans/2026-06-10-SP7-testing-qa-plan.md` | phases A–E / 6 gaps / 5 decisions |

## 5. Consolidated decision register

**(a) Operator-GATED — irreversible / high-blast / genuinely ambiguous (I will NOT proceed without explicit go):**
- **G-1** Execute **fork-ALL 49 repos** under `vasic-digital/caf-*` (SP3) — irreversible external. **✅ PRE-AUTHORIZED by operator 2026-06-10** — may run once the SP3 scripts pass their anti-bluff Challenge (no re-ask).
- **G-2** Create **2 new HelixDevelopment repos** `dag_orchestrator` + `pipeline_runtime` (SP6) — irreversible external. **Still requires explicit go.**
- **G-3** **SDK-update blast radius** (SP2) — bumping bedrockruntime/azcore/tree-sitter + resolving gin 1.11.0↔1.12.0 across both modules can break builds → operator scope/sequence call.
- **G-4** **Video-course / website production scope** (SP2 OP-6) — defer vs in-scope.
- **G-5** Whether to **de-bluff all ~70 agent packages now** vs tier-1-first + track-rest (SP4 D-C; D-6/D-9 live in `helix_agent`, a CONST-047 submodule).

**(b) Reversible / clear-default — I'll proceed on the subagents' recommended defaults unless you object (§11.4.101):**
- Lift a decoupled key-recognition table from helix_agent `SupportedProviders.EnvVars` (SP1); LoadAPIKeys precedence = gap-fill (don't override already-exported shell env).
- Verifier-disabled = honest empty + opt-in `--include-unverified` live path (never show `FallbackModels` as "working").
- Unified catalog at `internal/catalog/` + `GET /v1/catalog`; promote HelixLLM to first-class; naming `ensemble//helixllm//<provider>/<model>`.
- De-hardcode funnel-first, then OpenAI/Anthropic/DeepSeek/Mistral.
- SP5 substrate = EXTEND `Concurrency`/`Agentic`, REUSE `Planning`, FOLD agent_mesh in; first task adds the `replace` directives (closes D-7). **[M-1 fix] SP5 T5.2 is the SOLE owner of the `replace dev.helix.agent` + `replace dev.helix.substrate` `helix_code/go.mod` edits; SP4 T4.0.x CONSUMES them as a prerequisite (depends-on SP5 wiring) and NEVER re-edits go.mod** — resolves the parallel-stream edit-collision.
- New local harnesses for ddos/scaling/ux/ui with anti-delegation-bluff gates; HelixQA fetch-to-latest (§11.4.71 merge-onto-main).

## 5a. Bundle code-review (§11.4.142) — verdict + fixes applied

Independent review `2026-06-10-planning-bundle-review.md`: **GO-WITH-FIXES** (0 BLOCKER · 1 MAJOR · 5 MINOR); 18 anchors spot-checked incl. all of D-1..D-9 — zero fabricated/drifted. Fixes:
- **M-1 (MAJOR) — FIXED:** `replace dev.helix.agent`/`dev.helix.substrate` go.mod edits sole-owned by SP5 T5.2; SP4 consumes as prerequisite (see §5(b)).
- **m-1 — N/A:** analysis-A's `§11.4.142` choice is moot (SP0 constitution authoring descoped; upstream already has §11.4.142).
- **m-2 — noted:** SP4's "beyond roadmap" label for D-9 is cosmetic; D-9 is in the ledger (§1a).
- **m-3 — TODO in SP7:** add a D-9 regression-guard row to the registry (carried into SP7 execution).
- **m-4 — RESOLVED:** SP3 plan now written.
- **m-5 — tracked:** CONST-055 full-rule audit = SP0.b (still unrun; offered to operator).

## 6. Next actions

1. ✅ Planning complete: 6 analyses + 5 SP plans + defect ledger D-1..D-9. SP3 plan + planning-bundle code-review in flight.
2. **▶ HERE:** operator confirms the **decision register** (esp. the 5 gated items G-1..G-5).
3. Commit the planning bundle (needs §11.4.142 independent review GO + §11.4.75 `.html`/`.pdf` siblings + superproject pointer bump) — **operator: commit now?**
4. On go: implement RED-first per SP, each change through the §11.4.142 review gate; parallel non-contending streams (§11.4.103); irreversible externals only after G-1/G-2 go.
