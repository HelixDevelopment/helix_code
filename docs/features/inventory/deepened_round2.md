## Deepened inventory (round 2)

Round-2 deepening of packages flagged shallow/partial/unknown in round 1
(`internal_services.md` coverage notes + the owned-submodule umbrella rows).
Each previously-thin entry is enumerated into discrete features in the SAME
12-column schema as `internal_services.md` (legend in `docs/features/Status.md`
§ "Status dimensions"). Assessed from real code evidence (impl reality, wiring,
`*_test.go` presence) per CONST-035 / §11.4.107 anti-bluff.

> **Anti-bluff caveats (read before trusting a row).**
> - `📹 Video` is **`no` for every row** — no analyzed recording exists; the
>   conductor owns video confirmation. `Overall` is therefore **never
>   `confirmed`**; honest rollups are `working-untaped` / `partial` / `gap`.
> - `Origin` is `native` (all own-org code).
> - **Round-1 stub claims corrected here from direct source reads** (a round-2
>   exploration subagent reported these as stubs WITHOUT reading the code —
>   verified false): `workspace` docker `Stop`/`Remove` are real
>   (`docker stop` / `docker rm -f`, `manager.go:21-37`); `voice` recorder
>   `Stop` is real (`Process.Signal(os.Interrupt)` then `Kill()`,
>   `recorder.go:63-80`); `roocode` `CodeReviewer.Review` is real (reads the
>   file + scans for TODO/issue patterns, `reviewer.go:16-30`). They are
>   `done`/`partial` (impl real) but `Real-use=unknown` (shipped-flow wiring
>   unconfirmed), not stubs.
> - Submodule-side `Wired` follows `internal_services.md`'s wiring model:
>   `helix_agent` is imported directly (`Wired=yes` where HelixCode consumes it),
>   transitively-reachable own-org deps are `Wired=partial`, repo-present-but-not-
>   in-build-graph are `Wired=no`. `helix_specifier`/`helix_qa` are imported by
>   module path (`Wired=yes`); `panoptic` is a Challenge/recording submodule not
>   in HelixCode's Go build graph (`Wired=no`).
> - Submodule `Tests` reflects `*_test.go` presence in/near the package; it is
>   NOT a coverage-percentage claim. README-cited coverage figures
>   (helix_agent "65.6%", panoptic "78%") are NOT used as evidence — unverified.

### Internal packages (round-1 `partial`/`unknown` → deepened)

| Area | Component | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| infrastructure | internal/clientcore | agentic tool registry wiring (git/fs/LSP/MCP, live-built) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/clientcore | LSP read-only diagnostics tool wiring | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | MCP config-merge + server startup + tool registration | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | tool-loop system-prompt generation (from registry names) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/clientcore | tool-trace adapter (agent → ensembleui) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/clientcore | skills + plugins loader (graceful-on-failure) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | verifier adapter wiring (CONST-036/037 single-source) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | env-provider registration (cloud API keys, zero-hardcode) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | HelixAgent provider registration (when server reachable) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/agentbridge | verifier bridge config wrapper | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/agentbridge | VerifyModel real HTTP call to verifier service | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/agentbridge | test request/result marshal+unmarshal | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/checkpoint | git-backed snapshot (stash/write-tree/read-tree) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | file-copy fallback backend (real os.Read/Write) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | checkpoint create (capture) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | checkpoint restore (undo, writes real bytes) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | metadata persistence (label/timestamp JSON) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | multi-backend auto-selection (git else files) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/ensembleui | tool-trace formatting (text/markdown render) | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/ensembleui | metadata extraction helpers (int/string/slice) | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/ensembleui | multi-model response assembly | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/ensembleui | per-model token-usage aggregation | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/substrate | Unit interface (task contract) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/substrate | queue-based scheduler (enqueue + dispatch) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/substrate | serial FIFO execution (single worker) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/substrate | context-cancellation propagation to Execute | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | workspace create (project bootstrap) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | docker runner start/stop/remove/list (real os/exec) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | workspace status tracking (enum + DB persist) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | CLI tools (create/delete/list registration) | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | i18n translator seam | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/workspace | auto-cleanup / TTL teardown | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/voice | audio device detection (arecord/sox/parec probe) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | audio capture start (real exec.Command) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | audio capture stop (Signal(Interrupt)→Kill) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | Whisper API transcription (real multipart HTTP POST) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | WAV validation before transcribe | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | local whisper.cpp fallback transcription | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/roocode | in-memory conversation store (create/add-message) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/roocode | task delegation (TaskSpec build; no real dispatch) | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/roocode | code generation (Generate/Bootstrap scaffold) | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/roocode | code review (file read + TODO/issue scan) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/roocode | i18n translator seam | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | OpenTelemetry provider bootstrap (OTLP exporter) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | agent instrumentation (spans + iteration counter) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | LLM instrumentation (traced-provider wrapper) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | tool instrumentation (call spans) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | config-from-env (OTEL_* vars + exporter-kind resolve) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | flexible timestamp parse (Unix + RFC3339) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | adapter facade (cache+health+client, IsEnabled/Reachable) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | two-tier cache (TTL + Redis) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | health monitor (background liveness goroutine) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | background poller (interval model-sync) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | bootstrap (embedded-or-remote stack init) | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | embedded verifier server (fallback) | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | fallback model list (offline mode) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | real-server integration test (live LLMsVerifier) | done | yes | unknown | integ | no | no | no | native | working-untaped |
| service | internal/worker | consensus manager (Raft state machine, election/heartbeat) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | vote-transport abstraction (decoupled from wire) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | single-node fallback (nil-transport safe step-down) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | worker isolation: sandbox dir create (0750) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | worker isolation: resource limits (cgroup writes) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | SSH client pool (persistent connection reuse) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | consensus stress + chaos tests (concurrent votes, SIGKILL) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | worker-pool isolation stress test | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/server | Gin server + JWT auth middleware | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| service | internal/server | project list/register handlers (user-from-context) | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server | LLM generate endpoint (real provider stream) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/server | LLM model-list endpoint (verifier-queried) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/server | worker/task/project CRUD handlers | done | yes | yes | integ | no | no | no | native | working-untaped |
| service | internal/server | QA session start handler (helix_qa integration) | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/server | specify endpoint (spec from prompt) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/server | stats / pprof / error-categorize (400-vs-500) helpers | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/server | i18n translator seam (context-aware) | done | yes | yes | unit | no | no | no | native | working-untaped |

### Owned submodules (round-1 umbrella rows → deepened principal packages)

| Area | Component | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| submodule | helix_agent | internal/llm multi-provider interface (Claude/DeepSeek/Gemini/Mistral/Qwen/xAI/OpenRouter/Ollama/llama.cpp/Bedrock/Azure) | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/ensemble multi-model debate/fusion orchestrator | done | yes | yes | unit,integ,e2e | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/provider+providers per-provider abstraction layer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/verifier LLMsVerifier single-source-of-truth | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/server HTTP(Gin)+WebSocket API (completions/ensemble/stream) | done | yes | unknown | unit,integ,e2e | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/handlers per-route request/response handlers | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/database PostgreSQL persistence (pgx/v5, migrations) | done | partial | unknown | integ | no | no | no | native | partial |
| submodule | helix_agent | internal/redis caching layer (go-redis/v9) | done | partial | unknown | integ | no | no | no | native | partial |
| submodule | helix_agent | internal/auth JWT + bcrypt/argon2 + OAuth2 | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | helix_agent | internal/security PII redaction + guardrails bridge | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/streaming SSE/WebSocket token-level streaming | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_agent | internal/cache semantic (embeddings dedup) caching | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/knowledge RAG / vector-DB (zep) bridge | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/monitoring Prometheus metrics | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/logging structured JSON + gRPC tracing | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/mcp Model Context Protocol server/client | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/skills+plugins hot-reload plugin system | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | internal/analytics usage telemetry + warehouse export | partial | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_agent | pkg/sdk Go + Python client SDKs | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/guardrails content guardrail engine (severity rules) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/pii PII detect+redact (email/phone/SSN/CC-Luhn/IPv4) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/content composable filter chains (length/pattern/keyword) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/policy rule-based policy enforce (allow/deny/audit) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/scanner vuln-scan interface + report aggregation | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/headers HTTP security headers middleware (CSP/HSTS/...) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/securestorage AES-256-GCM + Argon2id key derivation | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/e2ee end-to-end encryption (ChaCha20-Poly1305) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/ssrf SSRF prevention (outbound HTTP guard) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | pkg/attestation + pkg/gpuattest platform/GPU attestation | partial | no | unknown | unit | no | no | no | native | partial |
| submodule | helix_specifier | parallel execution (bounded-concurrency dispatch) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | constitution-as-code (machine-readable rule enforce) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | Nyquist TDD (≥2x test-to-impl ratio enforce) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | debate architecture (multi-round spec refinement) | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | skill learning (running-avg proficiency tracking) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | brownfield analysis (legacy pattern/dep detection) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | predictive specification (future-req prediction) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | cross-project transfer (shared knowledge base) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | adaptive ceremony (dynamic level by quality metrics) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | spec memory (persistent index + semantic search) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/autonomous autonomous QA session engine (~9.5k loc, 21 tests) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/orchestrator QA run orchestration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/navigator UI navigation/driving (~3.8k loc, 16 tests) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/detector + pkg/issuedetector issue/error detection | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/recordingqa recording-validator (anti-bluff §11.4.107) | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/regression standing regression-guard suite | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/evidence captured-evidence management | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/challengegen Challenge generation | partial | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/reporter QA reporting | done | yes | unknown | unit | no | no | no | native | working-untaped |
| submodule | helix_qa | pkg/conduit + pkg/bridge sync-channel / agent bridge (§11.4.116) | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/replay + pkg/reproduce replay / defect reproduction | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | pkg/distributed + pkg/maestro distributed/maestro coordination | partial | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_qa | banks/ test-bank corpus (JSON/YAML suites) | done | yes | unknown | none | no | no | no | native | partial |
| submodule | helix_qa | cmd/* tooling (axtree/capture/recvalidate/omniparser/uitars/lpips/...) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/launcher multi-platform test launcher (web/desktop/mobile) | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/executor UI automation executor | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/vision computer-vision element detection | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/ocr OCR text extraction | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/recvalidate recording validator | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/ai AI test generation/enhancement | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/cloud cloud storage/integration | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | internal/platforms per-platform automation adapters | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | screenshot + video recording capture | done | no | unknown | unit | no | no | no | native | partial |
| submodule | panoptic | cmd/record + cmd/vision + cmd/testgen CLI tooling | done | no | unknown | unit | no | no | no | native | partial |

### Round-2 coverage notes

- **Internal deepening:** the 12 round-1-flagged packages
  (`clientcore`, `agentbridge`, `checkpoint`, `ensembleui`, `substrate`,
  `workspace`, `voice`, `roocode`, `telemetry`, `verifier`, `worker`, `server`)
  are now broken out into 73 discrete feature rows. `checkpoint`, `telemetry`,
  most of `verifier`, `server`, and several `worker` features upgrade from
  round-1 `partial` to `working-untaped` on direct-source evidence (real git
  plumbing, real OTEL exporter, real Raft + cgroup writes, real Gin handlers
  with integration tests). `roocode` task-delegation/code-gen and
  `voice`/`workspace` auto-cleanup remain honest `gap` (real types, no shipped
  wiring / incomplete impl).
- **Stub-claim corrections** (see top caveat): three round-2-explorer "stub"
  reports were verified FALSE by direct read and corrected to `done`/`partial`.
  Logged here so the conductor does not propagate the wrong call.
- **Submodule `Real-use=unknown` throughout:** each submodule package compiles
  and has tests, but genuine end-user reachability THROUGH HelixCode (vs the
  submodule's own standalone tests) was not confirmed by static inspection —
  honestly `unknown`, never green.
- **`helix_agent` `Wired`:** `yes` for the packages HelixCode consumes via the
  direct `dev.helix.agent` import (llm/ensemble/server/provider/verifier/
  streaming surface reachable through the agent dependency); `partial` for
  packages reachable only transitively (database/redis/auth/monitoring/etc).
- **`panoptic` `Wired=no`:** present as an equal-codebase submodule
  (CONST-051) but NOT in HelixCode's Go build graph — it is a
  Challenge/recording framework consumed out-of-process, not imported.
- Every `Real-use=unknown` / `working-untaped` row is a candidate for a
  recorded scenario; none is video-confirmed (📹 `no` throughout).

**Round-2 deepened feature count: 145 rows** (73 internal across 12 packages +
72 submodule across helix_agent 19, security 11, helix_specifier 10,
helix_qa 14, panoptic 10, distributed across the wiring model).

## Sources verified 2026-06-22: helix_code/internal/* , submodules/{helix_agent,security,helixspecifier,helix_qa,panoptic}/

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced repo trees,
following the `docs/ARCHITECTURE.md` precedent — no external service documented).
This round-2 deepening re-reads packages round-1 flagged shallow; cross-referenced
against the live tree on 2026-06-22:
- The deepened rows resolve against real `helix_code/internal/*` package source and
  the named own-org submodules under `submodules/*` (`helix_agent`, `security`,
  `helix_qa`, `panoptic`, helix_specifier).
- The doc already records its own source-read corrections (round-1 stub claims
  verified false by direct source reads: `workspace` docker stop/remove, `voice`
  recorder) — those ARE the §11.4.99 negative findings for this slice, captured
  inline at the head of the file.
- No external-service version claims in this doc → no staleness check applies;
  the evidence is structural (impl reality / wiring / `_test.go` presence).
