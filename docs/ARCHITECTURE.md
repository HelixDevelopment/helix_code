# HelixCode Architecture

**Scope:** Factual, verified architecture map of the HelixCode repository. This document is the authority referenced by `CLAUDE.md` §11 for architecture questions. It describes what EXISTS in the live tree (verified by inspecting `.gitmodules`, both `go.mod` files, and directory listings); it is a structural map, not a status report. Known-incomplete areas are flagged honestly in the final section.

> **Note on CLAUDE.md drift:** `CLAUDE.md` §3.2 describes an older, partly flat submodule layout (e.g. `challenges/`, `containers/`, `security/`, `helix_qa/`, `helix_agent/` at repo root, and a `dependencies/` group for the HelixDevelopment modules). The live tree has moved nearly all owned submodules under `submodules/<name>/`. Where §3.2 and the live tree disagree, this document follows the live tree and notes the discrepancy inline.

---

## 1. Repository topology

HelixCode is a **governance / meta-repository** that contains a thin root Go module, a tracked inner Go application, and a large set of Git submodules.

### 1.1 Two Go modules — both `module dev.helix.code`

| Layer | Path | `module` | `go` version | Role |
|-------|------|----------|--------------|------|
| Root (thin) | `go.mod` | `dev.helix.code` | `1.25.2` | Governance/meta module. Only 3 require lines (`google/uuid`, `pkg/errors`, `yaml.v2`), all `// indirect`. Hosts root-level helpers (`internal/`, `cmd/`) and governance gates. |
| Inner (full app) | `helix_code/go.mod` | `dev.helix.code` | `1.26` | The real HelixCode Go application: server, CLI, internal domain packages, applications, tests, and all transitive third-party + own-org dependencies. |

Both modules declare the same module path `dev.helix.code`. They are distinct modules living one directory apart.

### 1.2 `helix_code/` is a tracked subdirectory, NOT a submodule

Verified: `.gitmodules` contains **zero** entries for `helix_code` (`grep -c helix_code .gitmodules` → `0`). The inner application is a normal tracked subdirectory of the meta-repo, consistent with CLAUDE.md §3.2.1's "circular reference if promoted" note. Instructions that reference `internal/auth`, `cmd/cli`, etc. almost always mean `helix_code/internal/...` / `helix_code/cmd/...`.

### 1.3 Submodule layout — `submodules/<name>/` (live tree), differs from CLAUDE.md §3.2

`.gitmodules` defines submodules across several path families:

- **`submodules/<name>/`** — the grouped layout for owned modules. 63 directories present under `submodules/`. NOTE the `.gitmodules` *section names* are inconsistent with the *paths*: e.g. the section `[submodule "containers"]` has `path = submodules/containers`, and `[submodule "dependencies/HelixDevelopment/llm_orchestrator"]` has `path = submodules/llm_orchestrator`. The **path** is authoritative; the live checkout puts these under `submodules/`. This is the principal divergence from CLAUDE.md §3.2, which still lists `challenges/`, `containers/`, `security/`, `helix_qa/`, `helix_agent/` as flat repo-root entries.
- **`dependencies/<name>/`** — third-party upstream mirrors: `dependencies/LLama_CPP` (ggml-org/llama.cpp), `dependencies/Ollama` (ollama/ollama), `dependencies/HuggingFace_Hub` (huggingface/huggingface_hub).
- **`cli_agents/<name>/`** — 50 third-party reference CLI-agent repos (aider, crush, gemini-cli, qwen-code, plandex, open-interpreter, swe-agent, gpt-engineer, claude-code, etc.).
- **`cli_agents_resources/<name>/`** — reference resource repos (Awesome-AI-Agents, Awesome-AI-GPTs, OpenAI-Cookbook, Cheshire-Cat-Ai docs, GitHub-Awesome-Copilot, Taches-CC-Resources).
- **Repo-root single submodules** — `github_pages_website` (HelixDevelopment-Code/Welcome marketing site) and `awesome-ai-memory` (topoteretes/awesome-ai-memory).

Only a subset of the 63 `submodules/*` directories are wired as formal `.gitmodules` entries; the remainder are present in the working tree (see §7 for the honest gap on this).

### 1.4 Inner module wiring via `replace` directives

`helix_code/go.mod` consumes own-org submodules through local `replace` directives pointing at `../submodules/<name>` — e.g.:

```
replace digital.vasic.containers     => ../submodules/containers
replace digital.vasic.helixqa        => ../submodules/helix_qa
replace digital.vasic.docprocessor   => ../submodules/doc_processor
replace digital.vasic.llmorchestrator => ../submodules/llm_orchestrator
replace digital.vasic.visionengine   => ../submodules/vision_engine
replace digital.vasic.challenges     => ../submodules/challenges
replace digital.vasic.security       => ../submodules/security
replace digital.vasic.debate         => ../submodules/debate_orchestrator
replace digital.vasic.helixspecifier => ../submodules/helix_specifier
replace digital.vasic.lazy           => ../submodules/lazy
```

The inner `require` block lists own-org modules `digital.vasic.containers`, `digital.vasic.debate`, `digital.vasic.helixqa`, `digital.vasic.helixspecifier`, `digital.vasic.lazy` (versions `v0.0.0-...` resolved via the local `replace`).

---

## 2. Inner application structure (`helix_code/`)

### 2.1 `helix_code/cmd/*` — entry points and command surfaces

Verified contents (mix of subdirectories and root-level `.go` files):

- `cli/` — CLI client entry.
- `server/` — HTTP server entry.
- `helix_config/`, `config_test/` — config tool and config validator.
- `security_fix/`, `security_fix_standalone/`, `security_scan/`, `security_test/` — security tooling.
- `performance_optimization/` — performance tools.
- `infrastructure/`, `i18n/` — infrastructure and i18n command surfaces.
- Root-level `.go` files: `root.go`, `main_commands.go`, `other_commands.go`, `local_llm.go`, `local_llm_advanced.go`, `i18n_seam.go`, plus `*_test.go` files (e.g. `local_llm_i18n_test.go`, `cmd_residual_i18n_round453_test.go`).

### 2.2 `helix_code/internal/*` — domain packages (74 verified)

Each is the real domain implementation (the `internal/mocks/` package is unit-test-only):

```
adapters     agent        approval     approvalwire   auth
autocommit   cache        clarification cognee        commands
config       context      continua     database       deployment
discovery    editor       event        fix            focus
hardware     helixqa      hooks        i18n_wiring    i18nwiring
kilocode     llm          logging      logo           mcp
memory       mocks        monitoring   notification   performance
persistence  planner      plantree     plugins        pprofutil
project      projectmemory provider    providers      quality
redis        render       repomap      roocode        rules
secrets      security     server       session        task
telemetry    template     testutil     theme          tools
verifier     version      voice        worker         workflow
workspace
```

Role groupings:
- **LLM / providers:** `llm`, `provider`, `providers`, `verifier` (LLMsVerifier integration, CONST-036–040), `cognee`.
- **Agent / orchestration:** `agent`, `commands`, `task`, `worker`, `workflow`, `planner`, `plantree`, `continua`, `self`-improvement-adjacent `quality`.
- **Context / code intel:** `context`, `repomap`, `discovery`, `project`, `projectmemory`, `memory`, `editor`.
- **IO / integration surfaces:** `mcp`, `hooks`, `plugins`, `tools`, `voice`, `render`.
- **Platform / infra:** `database`, `redis`, `persistence`, `cache`, `server`, `deployment`, `hardware`, `monitoring`, `telemetry`, `performance`, `pprofutil`, `logging`, `event`.
- **Security / auth:** `auth`, `security`, `secrets`, `approval`, `approvalwire`.
- **i18n / theming / branding:** `i18n_wiring`, `i18nwiring`, `theme`, `logo`.
- **Adapter shims for ported agents:** `kilocode`, `roocode`, `adapters`, `focus`, `clarification`, `autocommit`.
- **Test-only:** `mocks`, `testutil`.

### 2.3 `helix_code/applications/*` — platform front-ends

`android`, `aurora_os`, `desktop` (Fyne GUI), `harmony_os`, `ios`, `terminal_ui` (tview/tcell TUI), plus a `README.md`. NOTE the directory names use snake_case (`aurora_os`, `harmony_os`, `terminal_ui`) per CONST-052, differing from CLAUDE.md §3.2.1's hyphenated `terminal-ui` / `aurora-os` / `harmony-os` listing.

### 2.4 `helix_code/tests/*` — test layers

`unit` (mocks allowed here only), `integration` (`-tags=integration`), `e2e`, `automation`, `regression`, `security`, `performance`, `stresschaos`, `memory`, `qa`, `infrastructure`, `testinfra`, plus `COVERAGE_REPORT.md`, `TEST_COVERAGE_REPORT.md`, `TESTING_INFRASTRUCTURE.md`, `README.md`.

### 2.5 Other inner directories

`api/`, `pkg/`, `shared/`, `config/`, `docker/`, `scripts/`, `security/`, `assets/`, `examples/`, `benchmarks/`, `packaging/`. Several `docker-compose.*.yml` files exist at the inner root, including `docker-compose.full-test.yml` (the zero-skip integration stack referenced by the test targets).

---

## 3. Submodule catalogue (`submodules/*`)

The grouped owned-module layout. Roles inferred from each submodule's own `README.md` first heading and directory name (verified live). Modules whose README opens with a `digital.vasic.*` / `Helix*` identifier are owned-org reusable libraries.

| Submodule | Role (one-line) |
|-----------|-----------------|
| `helix_qa` | QA / Challenge-orchestration platform (HelixDevelopment/HelixQA). |
| `challenges` | Cross-cutting Challenge bank (vasic-digital/Challenges). |
| `containers` | Container/runtime abstraction (Docker/Podman/Qemu) — vasic-digital/Containers. |
| `security` | Security tooling (vasic-digital/Security). |
| `helix_agent` | AI-powered ensemble LLM service (HelixDevelopment/HelixAgent). |
| `llms_verifier` | Enterprise LLM verification platform — single source of truth for model metadata. |
| `doc_processor` | Document processing module (HelixDevelopment/DocProcessor). |
| `llm_orchestrator` | LLM orchestration module. |
| `llm_provider` | LLM provider abstraction module. |
| `vision_engine` | Vision / image-analysis engine. |
| `debate_orchestrator` | Multi-agent debate orchestration. |
| `helix_specifier` | Specification-handling module. |
| `helix_memory` | Memory subsystem. |
| `helix_llm` | LLM helper module. |
| `rag` | Retrieval-Augmented Generation module. |
| `vector_db` | Vector database abstraction. |
| `embeddings` | Embeddings generation/handling. |
| `memory` | Generic memory store. |
| `agentic` | Agentic-workflow primitives. |
| `auth` | Authentication module. |
| `auto_temp` | Auto-temperature tuning for LLM calls. |
| `background_tasks` | Background task execution (digital.vasic.background). |
| `benchmark` | Benchmarking utilities. |
| `cache` | Caching abstraction. |
| `claritas` | digital.vasic.claritas (clarity/analysis utility). |
| `concurrency` | Concurrency primitives (digital.vasic.concurrency). |
| `config` | Configuration module (digital.vasic.config). |
| `conversation` | Conversation handling (digital.vasic.conversation). |
| `database` | Database abstraction (digital.vasic.database). |
| `document` | Document model (digital.vasic.document). |
| `event_bus` | Event bus (digital.vasic.eventbus). |
| `filesystem` | Filesystem abstraction (digital.vasic.filesystem). |
| `formatters` | Output/text formatters. |
| `gandalf_solutions` | GandalfSolutions utility module. |
| `hyper_tune` | Hyperparameter tuning. |
| `i_llm` | I-LLM interface module. |
| `i18n` | Internationalization (digital.vasic.i18n). |
| `lazy` | Lazy-evaluation utilities. |
| `leak_hub` | Leak/secret detection hub. |
| `llm_ops` | LLM operations module. |
| `mcp_module` | Model Context Protocol module. |
| `messaging` | Messaging module. |
| `middleware` | Middleware (digital.vasic.middleware). |
| `models` | Models module. |
| `normalize` | Normalization utilities (digital.vasic.normalize). |
| `observability` | Observability (digital.vasic.observability). |
| `optimization` | Optimization utilities. |
| `ouroborous` | Self-referential/loop orchestration module. |
| `panoptic` | Panoptic monitoring/observation module. |
| `planning` | Planning module. |
| `plinius_common` | PliniusCommon shared utilities. |
| `plugins` | Plugin framework. |
| `rate_limiter` | Rate limiting (digital.vasic.ratelimiter). |
| `recovery` | Application-level fault tolerance for Go. |
| `red_team` | Red-team / adversarial testing (digital.vasic.redteam). |
| `self_improve` | Self-improvement module. |
| `skill_registry` | Skill registry module. |
| `storage` | Storage abstraction (digital.vasic.storage). |
| `streaming` | Streaming module. |
| `tool_schema` | Tool schema definitions (digital.vasic.toolschema). |
| `toon` | digital.vasic.toon utility module. |
| `veritas` | Veritas verification/truth module. |
| `watcher` | File/resource watcher (digital.vasic.watcher). |

Third-party upstream mirrors live under `dependencies/` (LLama_CPP, Ollama, HuggingFace_Hub). Reference agents/resources live under `cli_agents/` (50) and `cli_agents_resources/`.

---

## 4. Technology stack

Verified from `helix_code/go.mod` (inner module is the real dependency owner; cross-checked against CLAUDE.md §3.1). Actual versions found:

- **Language:** Go `1.26` (inner), Go `1.25.2` (root meta module).
- **HTTP / API / RPC:** `gin-gonic/gin v1.11.0`, `gorilla/websocket v1.5.3`, `google.golang.org/grpc v1.80.0`, `grpc-ecosystem/grpc-gateway/v2 v2.28.0` (indirect).
- **Persistence:** `jackc/pgx/v5 v5.7.6` + `lib/pq v1.10.9` (PostgreSQL); `redis/go-redis/v9 v9.17.2` (Redis); `bradfitz/gomemcache`; `hashicorp/golang-lru/v2 v2.0.7`.
- **AuthN/Z:** `golang-jwt/jwt/v4 v4.5.2`, `golang.org/x/crypto v0.49.0` (bcrypt/argon2), `golang.org/x/oauth2 v0.36.0`.
- **Config / CLI:** `spf13/viper v1.21.0`, `spf13/cobra v1.8.0`, `spf13/pflag v1.0.10`, `fsnotify/fsnotify v1.9.0`.
- **LLM / cloud:** `aws-sdk-go-v2` (`v1.32.7`) incl. `service/bedrockruntime v1.23.1`, `aws/smithy-go v1.22.1`; `Azure azcore v1.16.0` + `azidentity v1.8.0`; `getzep/zep-go/v3 v3.10.0`; `smacker/go-tree-sitter` (tree-sitter parsing).
- **UI:** `fyne.io/fyne/v2 v2.7.0` (desktop GUI); `rivo/tview v0.42.0` + `gdamore/tcell/v2 v2.8.1` (terminal UI); `chromedp/chromedp v0.15.1` + `chromedp/cdproto` (headless browser).
- **LSP:** `go.lsp.dev/protocol v0.12.0`, `go.lsp.dev/jsonrpc2 v0.10.0`, `go.lsp.dev/uri v0.3.0`.
- **i18n / NLP:** `nicksnyder/go-i18n/v2 v2.5.1`, `jdkato/prose/v2 v2.0.0`.
- **Observability:** OpenTelemetry `otel v1.43.0` family (OTLP gRPC/HTTP exporters, stdout exporters, SDK, metric, trace); `go.uber.org/zap v1.28.0`.
- **Testing:** `stretchr/testify v1.11.1`.
- **Own-org (via `replace`):** `digital.vasic.containers`, `digital.vasic.helixqa`, `digital.vasic.debate`, `digital.vasic.helixspecifier`, `digital.vasic.lazy` (and others wired through `replace` to `../submodules/*`).

Versions found in go.mod match the CLAUDE.md §3.1 stated versions where both list a number (Gin v1.11.0, gorilla/websocket v1.5.3, gRPC v1.80.0, JWT v4.5.2, Viper v1.21.0, Cobra v1.8.0, pflag v1.0.10, fsnotify v1.9.0, Fyne v2.7.0, testify v1.11.1).

---

## 5. Build & test entry points

Two Makefiles. The **root** Makefile runs governance gates only; the **inner** `helix_code/Makefile` does real builds/tests.

### 5.1 Root governance Makefile (verified targets)

`bluff-detector`, `no-silent-skips` (+ `-warn`), `demo-all` (+ `-warn`), `demo-one`, `ci-validate-all`, `verify-foundation`, `verify-governance-cascade`, `verify-llmsverifier-pin-parity`, `scan-all`, `scan-gosec`, `scan-secrets` (+ `-root`), `scan-snyk`, `scan-sonarqube`, `scan-trivy`.

### 5.2 Inner `helix_code/Makefile` (verified targets)

- **Build:** `all`, `build`, `verify-compile`, `clean`, `clean-all`, `dev`, `dev-setup`, `prod`, `release`, `deps-scan`, `setup-deps`, `pgo-refresh`.
- **Quality:** `fmt`, `lint`.
- **Test:** `test`, `test-all`, `test-complete`, `test-coverage`, `coverage-full`, `test-benchmark`, `test-full`, `test-unit-full`, `test-integration-full`, `test-e2e-full`, `test-security-full`, `test-load-full`, `test-docs`.
- **Test infra:** `test-infra-up`, `test-infra-status`, `test-infra-down`.
- **Verifier:** `test-verifier-unit`, `test-verifier-integration`, `test-verifier-challenges`, `test-verifier-capability`, `test-verifier-hardcode`.
- **Stress/chaos:** `stress-chaos`, `stress-chaos-infra`, `stress-chaos-meta`.
- **HelixQA:** `helixqa-build`, `helixqa-test`, `helixqa-challenge`, `helixqa-bump-submodules`.
- **Platforms:** `desktop` (+ `-all`/`-linux`/`-macos`/`-windows`/`-nogui`), `mobile` (+ `-init`/`-ios`/`-android`), `aurora-os`, `harmony-os`, `aurora-harmony`, `windows`.
- **Packaging:** `installers`, `deb`, `rpm`, `homebrew`, `assets`, `logo-assets`, `logo-deps`, `docs`, `manual-html`, `sync-manual`.
- **Containers (inner Makefile):** `container-build`, `container-builder-image`, `container-clean`, `container-dev-up`, `container-dev-down`, `container-lint`, `container-release`, `container-shell`, `container-test`, `container-test-full`. NOTE CLAUDE.md §3.4 (note 2026-05-29) states `make container-*` targets do not exist; the live inner Makefile DOES define them. Discrepancy flagged for reconciliation — per project policy containerized workflows go through the `./helix` facade / `containers` submodule, but the targets are present in the file.
- **Security scans:** `security-scan` (+ `-all`/`-gosec`/`-grype`/`-kics`/`-semgrep`/`-snyk`/`-sonarqube`/`-trivy`), `secrets-scan`, `scan-start-sonar`, `scan-stop`.

### 5.3 `./helix` container facade

A repo-root bash script (`helix`) providing a unified interface to the containerized stack (`start`/`stop`/`status`/`logs`/`restart`/`shell`). NOTE the script's own header says "Docker Facade" and its `check_docker()` invokes `docker`; CLAUDE.md states the host uses podman, not docker. The script as written checks for `docker` on PATH — discrepancy flagged (the documented podman workflow and the script's `docker` invocation are not aligned in the inspected header).

---

## 6. Governance / constitution cascade

The **constitution submodule** at `constitution/` is the canonical root of governance (per CONST-059 / §11.4.35). Verified present: `constitution/Constitution.md`, `constitution/CLAUDE.md`, `constitution/AGENTS.md` (each with `.html`/`.pdf`/`.docx` sibling exports).

The consumer-side repo-root files — `CLAUDE.md`, `AGENTS.md`, `CONSTITUTION.md`, plus sibling agent manuals `CRUSH.md`, `QWEN.md` — are **extensions** that inherit from the canonical root. `CLAUDE.md` opens with `@constitution/CLAUDE.md` / an `## INHERITED FROM constitution/CLAUDE.md` pointer. Project-specific rules live consumer-side; universal rules live in the constitution submodule. Cascade is verified via `./scripts/verify-governance-cascade.sh` (root governance gate `verify-governance-cascade`).

---

## 7. Known-incomplete / honest gaps

This section records areas the live tree shows as in-progress or inconsistent. It does NOT claim any feature works; it records observable structural facts.

1. **CLAUDE.md §3.2 vs live submodule layout.** §3.2 documents a flat repo-root layout for several owned submodules; the live tree uses `submodules/<name>/`. CLAUDE.md §3.2 is stale relative to the tree and should be reconciled.
2. **`.gitmodules` section-name vs path mismatch.** Several entries have section names like `[submodule "dependencies/HelixDevelopment/llm_orchestrator"]` but `path = submodules/llm_orchestrator`. The naming is inconsistent (legacy section labels not updated after the relayout). Paths are authoritative.
3. **`submodules/*` directories not all in `.gitmodules`.** 63 directories exist under `submodules/`, but only a subset have formal `.gitmodules` entries (e.g. `containers`, `helix_qa`, `challenges`, `security`, `llms_verifier`, `helix_agent`, `doc_processor`, `llm_orchestrator`, `llm_provider`, `vision_engine`). The remainder are present in the working tree without verified `.gitmodules` wiring in the file inspected — the submodule registry and the on-disk tree are not fully in sync.
4. **Inner `container-*` Makefile targets vs CLAUDE.md §3.4.** CLAUDE.md says these targets were removed and do not exist; the inner Makefile still defines them. One of the two is out of date.
5. **`./helix` facade uses `docker`, policy mandates podman.** The script header and `check_docker()` invoke `docker`; project rules (Rule 4 / §11.4.76) mandate podman via the orchestrator. The facade script is not aligned with the stated podman workflow.
6. **Applications directory naming.** Live dirs use snake_case (`aurora_os`, `harmony_os`, `terminal_ui`); CLAUDE.md §3.2.1 lists hyphenated names. CLAUDE.md is stale here (the live tree is the CONST-052-compliant form).
7. **Large volume of root-level status/report `.md` files.** The repo root holds many `*_COMPLETION_*`, `*_REPORT.md`, `PHASE_*` documents. These are historical narrative artifacts; they are NOT authoritative for current architecture (this document is). Their accuracy is not verified here.
8. **Duplicate-looking internal packages.** `internal/i18n_wiring` and `internal/i18nwiring` both exist; `internal/provider` and `internal/providers` both exist. Whether this is intentional layering or in-progress consolidation was not determined from directory listing alone.

---

## Sources verified 2026-06-04: /Users/milosvasic/Projects/HelixCode/.gitmodules, /Users/milosvasic/Projects/HelixCode/go.mod, /Users/milosvasic/Projects/HelixCode/helix_code/go.mod, /Users/milosvasic/Projects/HelixCode/Makefile, /Users/milosvasic/Projects/HelixCode/helix_code/Makefile, /Users/milosvasic/Projects/HelixCode/helix (facade script header), ls of repo root, ls of submodules/, ls of helix_code/{cmd,internal,applications,tests}, ls of cli_agents/ + submodules/ (counts), README.md first headings of submodules/* for the catalogue, ls of constitution/.
