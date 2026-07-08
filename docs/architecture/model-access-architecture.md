# Model-Access Architecture — Diagrams (Key-Recognition · Unified Catalog · CLI-Bridge)

| Field | Value |
|-------|-------|
| Revision | 1 |
| Created | 2026-06-10 |
| Last modified | 2026-06-10 |
| Status | DESIGN — describes target architecture; nodes labelled IMPLEMENTED vs PLANNED |
| Maintainer | DOCUMENTATION subagent (READ-ONLY on code) |
| Authority | Cascades from `constitution/` + root `CLAUDE.md`/`CONSTITUTION.md`; CONST-036 (verifier single source of truth), CONST-040 (capabilities from `VerificationResult`), §11.4 anti-bluff |
| Sources | SP1 plan (`docs/superpowers/specs/plans/2026-06-10-SP1-model-access-plan.md`), SP2 plan (`…/2026-06-10-SP2-helixagent-exposure-plan.md`), SP4 plan (`…/2026-06-10-SP4-cli-bridge-plan.md`); analyses B/C/D (`docs/superpowers/specs/analysis/2026-06-10-{B,C,D}-*.md`); master roadmap (`docs/superpowers/specs/2026-06-10-llms-access-master-roadmap.md`) |

---

> **§11.4 / §11.4.123 anti-bluff honesty note (read first).** This document describes the
> **target architecture** of the model-access + provider-exposure + CLI-bridge system. It mixes
> **IMPLEMENTED** components (real code, cited `file:line`, confirmed in the analysis passes) with
> **PLANNED** components (specified in SP1/SP2/SP4 but **not yet built** — no captured runtime
> evidence exists). Every diagram node carries an explicit `[IMPLEMENTED …]` or `[PLANNED …]`
> tag. A node tagged PLANNED is a design intent, **not** a claim of working software. Diagrams
> are documentation, not proof of behaviour; the runtime-evidence bar (§11.4.5 / §11.4.69 /
> §11.4.107) is met only by the test/Challenge evidence each SP plan requires at execution time.

Legend used in every diagram:

- **IMPLEMENTED** — real code exists today; cited to a real `file:line` read during the
  2026-06-10 analysis passes.
- **PLANNED** — specified in an SP plan / task (e.g. `SP1·T1.2.1`); **not built**; no runtime
  evidence.
- **DEAD/UNWIRED** — code exists but has zero production call-sites (a §11.4.124 investigate-then-wire
  target, **not** removed).

Paths: inner Go app code is under `helix_code/` (module `dev.helix.code`); the HelixAgent
exposure surface is under `submodules/helix_agent/` (module `dev.helix.agent`).

---

## Table of contents

- [Diagram A — Key-recognition → verifier → working-model funnel (SP1)](#diagram-a--key-recognition--verifier--working-model-funnel-sp1)
- [Diagram B — Unified catalog under one root (SP2 + SP4)](#diagram-b--unified-catalog-under-one-root-sp2--sp4)
- [Diagram C — CLI-bridge selection & proxy sequence (SP4)](#diagram-c--cli-bridge-selection--proxy-sequence-sp4)
- [Node provenance ledger (IMPLEMENTED vs PLANNED)](#node-provenance-ledger-implemented-vs-planned)
- [Cross-references](#cross-references)

---

## Diagram A — Key-recognition → verifier → working-model funnel (SP1)

**What it shows.** How a single supplied/recognized API key turns into the set of WORKING
models a user sees. Source: SP1 plan §3 "Target design (the working-model funnel)"; analysis-B §4.1.

**Working-model predicate (SP1 §3, applied in the PLANNED `GetWorkingModels`):**
`present[provider]` ∧ `Verified==true` ∧ `VerificationStatus=="verified"` ∧
`OverallScore >= GetMinAcceptableScore()` ∧ `Providers[provider].Enabled != false`.

```mermaid
flowchart TD
    subgraph STARTUP["Process startup (cmd/server/main.go AND cmd/cli/main.go)"]
        A1["secrets.LoadAPIKeys()<br/>reads $HOME/api_keys.sh then walked-up .env, os.Setenv each<br/><b>[DEAD/UNWIRED — loader.go:30; zero prod call-sites]</b><br/>[PLANNED WIRE — SP1·T1.1.1]"]
        A2["config.Load() (viper AutomaticEnv + SetEnvPrefix HELIX)<br/>8 single-alias BindEnv<br/><b>[IMPLEMENTED — config.go:307-345]</b>"]
        A3["keyrec.PresentProviders() map[string]bool<br/>multi-alias {provider -> []envVarAlias} + isPlaceholder()<br/><b>[PLANNED — SP1·T1.1.2; internal/llm/keyrecognition.go]</b><br/>lifts decoupled from helix_agent SupportedProviders.EnvVars<br/>[IMPLEMENTED template — provider_types.go:348-385]"]
    end

    subgraph KEYSRC["Key sources (any one ⇒ provider present)"]
        K1["$HOME/api_keys.sh (export VAR=val)<br/>[IMPLEMENTED reader — loader.go:33-37]"]
        K2["walked-up .env<br/>[IMPLEMENTED reader — loader.go:84-129]"]
        K3["shell-exported env (OPENAI_API_KEY, ANTHROPIC_API_KEY, ...)<br/>[IMPLEMENTED path — per-provider os.Getenv]"]
    end

    subgraph VERIFIER["LLMsVerifier integration (CONST-036 single source of truth)"]
        V1["verifier.Adapter.GetVerifiedModels(ctx)<br/>cache -> live HTTP -> stale -> fallback; circuit-breaker<br/><b>[IMPLEMENTED — adapter.go:183-224, client.go GET /api/models]</b>"]
        V2["GetMinAcceptableScore() default 6.0 (HELIX_VERIFIER_MIN_SCORE)<br/><b>[IMPLEMENTED but NEVER INVOKED — adapter.go:175 (D-4)]</b>"]
        V3["GetWorkingModels(ctx, present)<br/>applies key-presence + Verified + status + min-score<br/><b>[PLANNED — SP1·T1.2.1/T1.3.1; adapter.go]</b>"]
        V4["filterByProviderConfig — drops only Enabled==false<br/><b>[IMPLEMENTED — adapter.go:277-289]</b>"]
    end

    subgraph EXPOSE["Exposure call-sites"]
        E1["CLI handleListModels<br/>P1 verifier GetVerifiedModels -> printVerifiedModels (prints ALL incl. failed/pending = D-2)<br/><b>[IMPLEMENTED-but-bluffy — main.go:1355-1414]</b><br/>[PLANNED switch -> GetWorkingModels — SP1·T1.3.2]"]
        E2["server handlers /api/models<br/><b>[IMPLEMENTED -> GetVerifiedModels — handlers.go:1049/1140/1206]</b><br/>[PLANNED -> GetWorkingModels — SP1·T1.3.2]"]
        E3["model_manager registry<br/>SetVerifierAdapter exists; registry populated from hardcoded GetModels()<br/><b>[IMPLEMENTED — model_manager.go:132/202]</b><br/>[PLANNED -> funnel-populated — SP1·T1.3.2]"]
    end

    subgraph FACTORY["Provider construction"]
        F1["factory.NewProvider(config) — builds ONE provider by config.Type<br/><b>[IMPLEMENTED — factory.go:9-101]</b>"]
        F2["BuildPresentProviders(configs, present) — build EVERY key-present provider<br/><b>[PLANNED — SP1·T1.2.2; factory.go]</b>"]
        F3["per-provider GetModels() — OpenAI/Anthropic/DeepSeek/Mistral HARDCODED (CONST-036, D-5)<br/>OpenRouter fetchCatalog / Ollama /api/tags already DYNAMIC<br/><b>[IMPLEMENTED — openai_provider.go:201, anthropic_provider.go:203, openrouter_provider.go:233, ollama_provider.go:225]</b><br/>[PLANNED de-hardcode — SP1·T1.3.4]"]
    end

    DISABLED["Verifier disabled / cold (HELIX_VERIFIER_ENABLED=false default)<br/>MUST NOT present FallbackModels as 'working' (CONST-035 PASS-bluff)<br/><b>[IMPLEMENTED bluff today — main.go:1387 prints fallback]</b><br/>[PLANNED honest empty/notice — SP1·T1.3.3 / DECISION-2]"]

    K1 --> A1
    K2 --> A1
    A1 --> A2
    K3 --> A2
    A2 --> A3
    A3 --> V3
    V1 --> V3
    V2 --> V3
    V3 --> V4
    V4 --> E1
    V4 --> E2
    V4 --> E3
    A3 --> F2
    F1 --> F2
    F2 --> F3
    A2 -. "verifier off" .-> DISABLED
    DISABLED -. "honest list" .-> E1
```

**Defect anchors closed by SP1 (analysis-B top-5 gaps):** D-2 (CLI shows failed/pending),
D-3 (`LoadAPIKeys` dead), D-4 (min-score never applied), D-5 (hardcoded lists), plus the
no-key-presence-gate gap and the single-provider-factory gap.

---

## Diagram B — Unified catalog under one root (SP2 + SP4)

**What it shows.** The PLANNED unified catalog that joins, **under one root**, the AI-debate
ensemble + HelixLLM + every discovered provider + every provider's verified model + (SP4) every
CLI-agent bridge provider and its models — each a uniformly-named, individually-addressable
selection target. Source: SP2 plan §P2.1/§P2.2; analysis-C §3; master roadmap §257; SP4 plan §P4.1/§P4.3.

**Naming grammar (analysis-C §3.2, master roadmap line 257):**
`ensemble` · `ensemble/<preset>` · `helixllm` · `helixllm/<model>` · `<provider>` ·
`<provider>/<model_id>` (preserving already-namespaced ids like `openrouter/x-ai/grok-4`) ·
**(SP4)** `cli/<agent>` · `cli/<agent>/<model>`.

```mermaid
flowchart LR
    subgraph SOURCES["Join sources (all IMPLEMENTED individually)"]
        S1["ProviderRegistry.ListProviders() + GetCapabilities().SupportedModels<br/><b>[IMPLEMENTED — provider_registry.go:82; router.go:773-797]</b>"]
        S2["verifier ModelDiscoveryService<br/>DiscoveredModel.Verified / OverallScore<br/><b>[IMPLEMENTED — discovery.go:23,50-63]</b>"]
        S3["Ensemble service GetEnsembleService().RunEnsemble<br/><b>[IMPLEMENTED — router.go:676-767; provider_registry.go:1066]</b>"]
        S4["HelixLLM gated provider 'helixllm' (USE_HELIX_LLM=true)<br/><b>[IMPLEMENTED-but-buried — provider_registry.go:750-764]</b>"]
        S5["CLI-agent bridge providers (claude/qwen/opencode/...)<br/><b>[PLANNED — SP4·P4.1; cli_agent_provider.go + registry]</b><br/>(today: clis layer real exec base.go:120-138 + STUBS qwencode.go:101)"]
    end

    subgraph CATALOG["Unified catalog (the missing join layer) — ONE ROOT"]
        C1["catalog.BuildCatalog(reg, verifier, ensembleSvc[, cliRegistry])<br/>Entry{Name, Kind(ensemble|provider|model|cli), Provider, Verified, OverallScore, Enabled}<br/><b>[PLANNED — SP2·T2.1.2; internal/catalog/catalog.go]</b>"]
        C2["CatalogHandler.List -> GET /v1/catalog<br/><b>[PLANNED — SP2·T2.1.3 (OP-1 root path); internal/handlers/catalog.go]</b>"]
        C3["Selector resolution: any catalog Name -> target<br/>unknown -> 400 + candidate list (no silent guess §11.4.6)<br/><b>[PLANNED — SP2·T2.1.4; completion.go:28 / router.go:702]</b>"]
    end

    subgraph NAMES["Uniformly-named selection targets (one namespace)"]
        N0["ensemble<br/>ensemble/confidence_weighted"]
        N1["helixllm<br/>helixllm/helixllm-default"]
        N2["anthropic<br/>anthropic/claude-3-sonnet-20240229"]
        N3["openrouter/x-ai/grok-4 (preserved namespacing)"]
        N4["cli/claude<br/>cli/claude/&lt;model&gt; (SP4 PLANNED)"]
    end

    BLUFF["GET /v1/completion/models — HARDCODED 3-model list (D-1)<br/>deepseek-coder / claude-3-sonnet-20240229 / gemini-pro<br/><b>[IMPLEMENTED bluff — completion.go:411-436 (CONST-036/BLUFF-002)]</b><br/>[PLANNED verifier-sourced rewrite — SP2·T2.0.2]"]

    S1 --> C1
    S2 --> C1
    S3 --> C1
    S4 --> C1
    S5 --> C1
    C1 --> C2
    C2 --> C3
    C1 --> N0
    C1 --> N1
    C1 --> N2
    C1 --> N3
    C1 --> N4
    BLUFF -. "remediated, joins same source" .-> C1
```

**Anti-bluff invariant (SP2 §7).** Catalog "model" entries are emitted **only** for
`DiscoveredModel.Verified == true` — never the static `SupportedModels` slice, never a hardcoded
literal. HelixLLM is **promoted, never removed** (§11.4.122); all existing `/v1/*` routes stay
intact (additive design).

---

## Diagram C — CLI-bridge selection & proxy sequence (SP4)

**What it shows.** How a `cli/<agent>` selection resolves a binary (PRIMARY system-installed →
FALLBACK submodule-built), proxies a prompt via real `exec`, parses the result, and surfaces
power-features. Source: SP4 plan §P4.2/§P4.3/§P4.4; analysis-D §5.

**Tier-1 agents system-installed today (analysis-D §2, captured `command -v`):** claude 2.1.170,
qwen 0.5.0, opencode 1.16.2, gemini 0.1.9, crush v0.22.2, codex, goose 1.8.0, copilot 1.0.46.
**Fallback-required (absent today):** aider, plandex, forge.

```mermaid
sequenceDiagram
    autonumber
    participant U as User / catalog selector (cli/{agent})
    participant P as CLIAgentProvider (llm.Provider)<br/>[PLANNED — SP4·T4.1.2]
    participant R as resolveBinary(spec)<br/>[PLANNED — SP4·T4.2.1]
    participant SYS as System PATH (exec.LookPath)<br/>[IMPLEMENTED primitive — base.go:120]
    participant FB as Submodule-built binary<br/>cli_agents/{agent}/ [PLANNED build wiring — SP4·T4.2.2]
    participant EX as exec.CommandContext(binary, non-interactive args)<br/>[IMPLEMENTED primitive — base.go:127-138]
    participant V as LLMsVerifier VerificationResult<br/>[IMPLEMENTED — CONST-040]

    U->>P: Generate(ctx, req) / GetModels() / GetCapabilities()
    P->>R: resolve binary for AgentSpec
    R->>SYS: exec.LookPath(spec.Binary) — PRIMARY
    alt PRIMARY found (tier-1: claude/qwen/opencode/...)
        SYS-->>R: /path/to/binary
    else PRIMARY absent (aider/plandex/forge)
        R->>FB: resolve submodule-built artifact (FALLBACK)
        FB-->>R: built bin path
    else both absent
        R-->>P: IsAvailable=false + SKIP-with-reason (binary_unavailable)<br/>[PLANNED — SP4·T4.2.3, NOT a fake PASS]
    end
    R-->>P: chosen path (recorded in GetHealth — anti-bluff provenance)

    Note over P,EX: NEVER the qwencode APIKey!="" bluff (qwencode.go:137)<br/>NEVER templated strings (qwencode.go:101 "// Generated by Qwen")<br/>[PLANNED de-bluff — SP4·T4.0.1/T4.0.3]

    P->>EX: prompt via stdin / --prompt, non-interactive flag (§11.4.99 per-agent research)
    EX-->>P: real stdout + real exit code
    P->>P: parse stdout -> LLMResponse (GenerateStream = line-buffered)

    par Power-features + dynamic models
        P->>EX: model-list subcommand -> parse -> []ModelInfo (CONST-036, no hardcode)<br/>[PLANNED — SP4·T4.3.1]
        P->>V: GetCapabilities() from VerificationResult (Vision/RAG/Memory/MCP/LSP/ACP)<br/>[PLANNED CONST-040 wiring — SP4·T4.3.2], runtime-probe + honest provenance if no verifier entry
    end

    Note over P: Config re-export -> INSTALL (backup-first §9.2) -> LIVE post-install validate<br/>real proxied prompt returns real result, transcript docs/qa/{run-id}/<br/>[re-export+static-validate IMPLEMENTED — unified_cli_generator.go:76/263]<br/>[INSTALL + LIVE-validate PLANNED/ABSENT — SP4·T4.4.2/T4.4.3]
    P-->>U: real result (USER-VISIBLE proof, §11.4.108 runtime signature: "2+2"->"4")
```

**Wiring prerequisite (SP4 §2.3, confirmed gap D-7).** `helix_code/go.mod` (replace block
lines 194-212) has **no** `replace dev.helix.agent` — so HelixCode cannot import the `clis`
substrate today. SP4·T4.0.5 PLANS to add it (or a narrow shim, decided on `go mod graph`
evidence). The exec primitive itself (`base.go:120-138`) is IMPLEMENTED and real; the
per-agent dispatch tier (`instance_manager.go:906+ executeClaudeCode/...`) and
`qwencode.go:101/115/137` are **STUBS** (a §11.4 bluff SP4·P4.0 RED-reproduces then fixes).

---

## Node provenance ledger (IMPLEMENTED vs PLANNED)

A single-table audit so a reader can verify each claim against real code or a real SP task.

| Component | State | Anchor (real `file:line`) or SP task |
|-----------|-------|--------------------------------------|
| `secrets.LoadAPIKeys` reader | IMPLEMENTED but DEAD/UNWIRED | `helix_code/internal/secrets/loader.go:30,33-37,84-129` |
| Wire `LoadAPIKeys` at startup | PLANNED | SP1·T1.1.1 (`cmd/server/main.go`, `cmd/cli/main.go`) |
| Viper `AutomaticEnv` + 8 `BindEnv` | IMPLEMENTED | `helix_code/internal/config/config.go:307-345` |
| Multi-alias key-recognition table | PLANNED | SP1·T1.1.2 (`internal/llm/keyrecognition.go`) |
| helix_agent `SupportedProviders.EnvVars` (reuse template) | IMPLEMENTED | `submodules/helix_agent/internal/verifier/provider_types.go:348-385` |
| `verifier.Adapter.GetVerifiedModels` (real HTTP) | IMPLEMENTED | `helix_code/internal/verifier/adapter.go:183-224`; `client.go` GET `/api/models` |
| `GetMinAcceptableScore` (loaded, never invoked) | IMPLEMENTED-unused (D-4) | `helix_code/internal/verifier/adapter.go:175-180` |
| `GetWorkingModels` filter | PLANNED | SP1·T1.2.1 / T1.3.1 (`adapter.go`) |
| `filterByProviderConfig` (Enabled flag only) | IMPLEMENTED | `helix_code/internal/verifier/adapter.go:277-289` |
| CLI `handleListModels` / `printVerifiedModels` (shows failed/pending, D-2) | IMPLEMENTED-bluffy | `helix_code/cmd/cli/main.go:1355-1414` |
| Switch CLI/server/registry → `GetWorkingModels` | PLANNED | SP1·T1.3.2 |
| Verifier-disabled honest behaviour | PLANNED | SP1·T1.3.3 / DECISION-2 (today bluff at `main.go:1387`) |
| `factory.NewProvider` (one by `config.Type`) | IMPLEMENTED | `helix_code/internal/llm/factory.go:9-101` |
| `BuildPresentProviders` enumerator | PLANNED | SP1·T1.2.2 |
| Hardcoded provider model lists (D-5/CONST-036) | IMPLEMENTED (violation) | `openai_provider.go:201`, `anthropic_provider.go:203`, `deepseek_provider.go`, `mistral_provider.go` |
| OpenRouter `fetchCatalog` / Ollama `/api/tags` (dynamic templates) | IMPLEMENTED | `openrouter_provider.go:233-295`, `ollama_provider.go:225` |
| `ProviderRegistry.ListProviders` + `GET /v1/providers` | IMPLEMENTED | `submodules/helix_agent/internal/services/provider_registry.go:82`; `internal/router/router.go:773-797` |
| Ensemble route `POST /v1/ensemble/completions` | IMPLEMENTED | `submodules/helix_agent/internal/router/router.go:676-767` |
| HelixLLM gated provider `helixllm` (buried) | IMPLEMENTED | `submodules/helix_agent/internal/services/provider_registry.go:750-764` |
| `DiscoveredModel.Verified` / `OverallScore` (working flag) | IMPLEMENTED | `submodules/helix_agent/internal/verifier/discovery.go:50-63` |
| `GET /v1/completion/models` hardcoded 3-model list (D-1) | IMPLEMENTED bluff | `submodules/helix_agent/internal/handlers/completion.go:411-436` |
| D-1 verifier-sourced rewrite | PLANNED | SP2·T2.0.2 |
| `catalog.BuildCatalog` join | PLANNED | SP2·T2.1.2 (`internal/catalog/catalog.go`) |
| `GET /v1/catalog` handler + route | PLANNED | SP2·T2.1.3 (OP-1 root path) |
| Selector resolution (Name → target) | PLANNED | SP2·T2.1.4 |
| HelixLLM first-class root promotion | PLANNED | SP2·T2.2.4 |
| `llm.Provider` contract (CLI bridge target) | IMPLEMENTED | `helix_code/internal/llm/missing_types.go:356-378` |
| CLI-exec primitive (`LookPath` + `CommandContext`) | IMPLEMENTED | `submodules/helix_agent/internal/clis/agents/base/base.go:120-138` |
| `qwencode` stub (templated strings, APIKey gate) | IMPLEMENTED bluff (D-6) | `submodules/helix_agent/internal/clis/agents/qwencode/qwencode.go:101,115,137` |
| `instance_manager` dispatch stubs | IMPLEMENTED bluff (wider than D-6) | `submodules/helix_agent/internal/clis/instance_manager.go:906-942` |
| `replace dev.helix.agent` wiring (D-7) | PLANNED/ABSENT | SP4·T4.0.5 (`helix_code/go.mod:194-212` has no such replace) |
| `CLIAgentProvider` + `AgentSpec` registry | PLANNED | SP4·T4.1.2 / T4.1.3 |
| `resolveBinary` PRIMARY→FALLBACK | PLANNED | SP4·T4.2.1 |
| Dynamic `GetModels` per agent (CONST-036) | PLANNED | SP4·T4.3.1 |
| Capabilities from `VerificationResult` (CONST-040) | PLANNED | SP4·T4.3.2 |
| Config re-export + static `Validate()` | IMPLEMENTED | `submodules/helix_agent/.../unified_cli_generator.go:76,212,263`; `cmd/helixagent/main.go:107,4522` |
| Filesystem INSTALL + LIVE post-install validate | PLANNED/ABSENT | SP4·T4.4.2 / T4.4.3 |

---

## Cross-references

- **SQL data model** for these flows: [`model-access-schema.sql`](./model-access-schema.sql).
- **Index / metadata header:** [`README.md`](./README.md).
- **SP plans (authoritative task source):**
  `docs/superpowers/specs/plans/2026-06-10-SP1-model-access-plan.md`,
  `…-SP2-helixagent-exposure-plan.md`, `…-SP4-cli-bridge-plan.md`.
- **Analysis (evidence base):** `docs/superpowers/specs/analysis/2026-06-10-B-model-access.md`,
  `…-C-helixagent-exposure.md`, `…-D-cli-bridge.md`.
- **Master roadmap:** `docs/superpowers/specs/2026-06-10-llms-access-master-roadmap.md`.

> Per CONST-066 / §11.4.65, this `.md` should ship synchronized `.html`/`.pdf` siblings at
> release-prep. They are **not** generated by this READ-ONLY documentation pass.
