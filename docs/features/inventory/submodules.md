## Owned-submodule capabilities

Inventory of the discrete features each owned (own-org `vasic-digital` /
`HelixDevelopment`) submodule under `submodules/*` provides to HelixCode.
Assessed from each submodule's `README.md`, exported package surface, and
`*_test.go` presence (CONST-035 / §11.4.107 — evidence-based, no bluffs).

> **Wiring model (load-bearing).** HelixCode's inner Go module
> (`helix_code/go.mod`) `replace`s + imports only **3** submodules directly
> — `helix_agent` (`dev.helix.agent`), `dag_orchestrator` (`dev.helix.dag`),
> `pipeline_runtime` (`dev.helix.pipeline`). HelixCode source additionally
> imports a handful by module path (`containers`, `helixspecifier`, `helixqa`,
> `concurrency`, `helixmemory`, `debate`, `lazy`, `llmsverifier`, `memory`,
> `llmprovider`). A large set is wired **transitively** because
> `helix_agent/go.mod` requires ~35 own-org submodules — these are reachable
> through the agent dependency but NOT imported by HelixCode directly
> (`Wired=partial`). The remainder are present in the repo as equal-codebase
> submodules (CONST-051) but are NOT in HelixCode's Go build graph at all
> (`Wired=no`). `📹 Video=no` for every row (conductor owns video confirmation;
> nothing here is `confirmed`). Origin=`native` throughout (all own-org).

| Area | Component | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| submodule | helix_agent | Ensemble multi-LLM agent service (response fusion) | done | yes | yes | unit,integ,e2e | no | no | no | native | working-untaped |
| submodule | helix_agent | ReAct/tool-calling agent runtime (consumed by HelixCode `internal/agent`) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| submodule | helix_agent | Skill registry + debate + verifier wiring (umbrella, 1583 test files) | done | yes | unknown | unit,integ,e2e | no | no | no | native | partial |
| submodule | dag_orchestrator | Agent-free DAG scheduler (topo dispatch, worker pool, retry/backoff) | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | pipeline_runtime | Staged streaming dataflow runtime (push operators + FBP backpressure) | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | helix_specifier | Spec-driven dev fusion (SpecKit 7-phase + TDD + GSD milestones) | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | helix_specifier | 10 power features (parallel exec, constitution-as-code, debate phases) | partial | partial | unknown | unit | no | no | no | native | partial |
| submodule | helix_memory | Unified cognitive memory (Mem0+Cognee+Letta+Graphiti fusion) | done | yes | unknown | unit,integ | no | no | no | native | partial |
| submodule | helix_memory | Parallel cross-backend search + 3-stage fusion/dedup/re-rank | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | debate_orchestrator | Multi-agent debate orchestration (consensus + dissent, LessonBank) | done | yes | unknown | unit | no | no | no | native | partial |
| submodule | debate_orchestrator | Aux packages (validation/audit/evaluation/reflexion/tools) | stub | partial | no | unit | no | no | no | native | gap |
| submodule | concurrency | Worker pools, priority queues, rate limiters, circuit breakers, semaphores | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | lazy | Type-safe lazy-init primitives (sync.Once generics) | done | yes | yes | unit | no | no | no | native | working-untaped |
| submodule | memory | Mem0-style scoped memory + entity extraction + knowledge graph + leak detect | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | llm_provider | LLMProvider interface + circuit breaker + health monitor + retry + lazy | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | llms_verifier | LLM verification platform (existence/latency/streaming/vision/embeddings) | done | yes | unknown | unit,integ,e2e | no | no | no | native | partial |
| submodule | llms_verifier | Single-source-of-truth model/provider metadata (CONST-036/037) | done | yes | unknown | unit,integ | no | no | no | native | partial |
| submodule | containers | Container lifecycle (boot/compose/health) for infra-on-demand (§11.4.76) | done | yes | unknown | unit,integ | no | no | no | native | partial |
| submodule | helix_qa | Autonomous QA / Challenge orchestration (test-bank runner) [QA infra] | done | yes | unknown | unit,integ,e2e | no | n/a | no | native | partial |
| submodule | panoptic | Recording-validator / observation harness [QA infra] | done | partial | unknown | unit,integ | no | n/a | no | native | partial |
| submodule | agentic | Graph-based agentic workflow engine (branch/checkpoint/self-correct) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | auth | JWT / API-key / OAuth2 / HTTP auth middleware / token store | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | cache | Multi-backend cache (mem/Redis/PG) + distributed patterns + TTL/eviction | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | storage | Object storage (S3/MinIO/local) + cloud credential mgmt | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | vector_db | Unified vector store (Qdrant/Pinecone/Milvus/pgvector) | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | embeddings | Text embeddings across 7 providers (OpenAI/Cohere/Voyage/Jina/Google/Bedrock) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | event_bus | Typed pub/sub bus + glob/prefix/metadata filtering + middleware | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | messaging | Message-broker abstraction (in-mem/Kafka/RabbitMQ) + producer/consumer patterns | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | streaming | SSE / WebSocket(rooms) / gRPC streaming / webhook(HMAC) / HTTP+breaker / Gin | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | rag | RAG pipeline (3 chunkers, BM25+semantic+hybrid retrieve, MMR rerank, RRF fusion) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | tool_schema | Tool schema/validation/exec + 14 built-in safe tool handlers | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | plugins | Plugin lifecycle (dep-ordered) + .so/process loaders + sandbox + output parse | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | planning | AI planning algorithms (HiPlan + MCTS + Tree-of-Thoughts) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | optimization | Semantic cache + prompt compression + structured-output + SGLang/LlamaIndex/LangChain | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | self_improve | RLAIF pipeline (reward models, feedback, policy/prompt optimization) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | benchmark | LLM benchmarking (SWE-bench/HumanEval/MMLU/GSM8K) + leaderboard | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | llm_ops | LLMOps (continuous eval, A/B experiments, prompt versioning, alerting) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | mcp_module | MCP server+client (JSON-RPC 2.0 stdio+HTTP/SSE) + adapter registry | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | models | Shared AI/LLM data types (LLMRequest/Response, MCP/LSP/ACP, code-intelligence) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | formatters | Pluggable code-formatter registry + engine + cache + native-binary shims | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | vision_engine | CV + LLM-vision UI analysis + navigation-graph construction | partial | partial | unknown | unit | no | no | no | native | partial |
| submodule | doc_processor | Doc-feature-map extraction + verification-coverage tracking (QA-oriented) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | security | Security tooling module (round-300 deep-doc) — needs deeper inventory | done | partial | unknown | unit,integ | no | no | no | native | partial |
| submodule | red_team | YAML-driven adversarial-prompt fixture harness (defensive guardrail regression) | done | partial | unknown | unit | no | no | no | native | partial |
| submodule | llm_orchestrator | Headless CLI-agent orchestration (OpenCode/Claude/Gemini/Junie/Qwen) | done | no | no | unit | no | no | no | native | gap |
| submodule | helix_llm | Distributed LLM service (OpenAI/Anthropic APIs, llama.cpp, RAG, ReAct, HTTP/3) | done | no | no | unit,integ,e2e | no | no | no | native | gap |
| submodule | conversation | Infinite-context (event-sourcing) + LLM compression + LRU cache | done | no | no | unit | no | no | no | native | gap |
| submodule | database | Driver-agnostic DB (PG/SQLite) + pool + migrations + repo + query builder | done | no | no | unit,integ | no | no | no | native | gap |
| submodule | document | Document model (18-format detect, change tracking, JSON serialize) | done | no | no | unit | no | no | no | native | gap |
| submodule | filesystem | Multi-protocol FS client (SMB/FTP/NFS/WebDAV/Local) | done | no | no | unit | no | no | no | native | gap |
| submodule | config | Config mgmt (JSON files, env binding, validation, 8 storage-protocol types) | done | no | no | unit | no | no | no | native | gap |
| submodule | i18n | i18n library (bundles, loader, HTTP middleware) | done | no | no | unit | no | no | no | native | gap |
| submodule | observability | Distributed tracing + metrics + structured logging + health + analytics | done | no | no | unit | no | no | no | native | gap |
| submodule | middleware | Reusable net/http middleware (requestid/logging/recovery/cors/chain) | done | no | no | unit | no | no | no | native | gap |
| submodule | rate_limiter | Rate limiting (in-mem + Redis) + HTTP middleware | done | no | no | unit,integ | no | no | no | native | gap |
| submodule | recovery | Named circuit breakers + periodic health checks + resilience facade | done | no | no | unit | no | no | no | native | gap |
| submodule | watcher | Filesystem change monitoring (debounce, filters, handler chains) | done | no | no | unit | no | no | no | native | gap |
| submodule | background_tasks | Persistent task queue (PG) + worker pool + stuck-detect + DLQ + progress | done | no | no | unit | no | no | no | native | gap |
| submodule | skill_registry | Skill mgmt (load YAML/JSON/MD, register, execute, validate, store) | done | no | no | unit | no | no | no | native | gap |
| submodule | embeddings | (see embeddings row — module also standalone) | — | — | — | — | — | no | — | native | — |
| submodule | auto_temp | LLM temperature auto-tuning (multi-temp run + multi-judge scoring) | done | no | no | unit | no | no | no | native | gap |
| submodule | hyper_tune | LLM hyperparameter optimization (random/grid/Bayesian-lite) | done | no | no | unit | no | no | no | native | gap |
| submodule | i_llm | Structured-reasoning patterns (CoT/ToT/ReAct/few-shot/prompt-chain) | done | no | no | unit | no | no | no | native | gap |
| submodule | veritas | AI-truthfulness verification, fact-check, hallucination detection | done | no | no | unit | no | no | no | native | gap |
| submodule | leak_hub | System-prompt-leak detection + searchable archive | done | no | no | unit | no | no | no | native | gap |
| submodule | claritas | Defensive guardrail (leaked-prompt archive + extraction-attempt detector) | done | no | no | unit | no | no | no | native | gap |
| submodule | gandalf_solutions | Read-only Gandalf prompt-hacking solutions archive (research) | done | no | no | unit | no | no | no | native | gap |
| submodule | ouroborous | Self-referential AI-safety (recursive self-improve, runaway-loop detect) | done | no | no | unit | no | no | no | native | gap |
| submodule | normalize | Adversarial-input canonicalization (base64/leet/homoglyph/NFKC/ROT13…) | done | no | no | unit | no | no | no | native | gap |
| submodule | plinius_common | Plinius shared lib (config validators, error types, gRPC client, i18n, types) | done | no | no | unit | no | no | no | native | gap |
| submodule | toon | Token-Oriented Object Notation encode/decode | stub | no | no | unit | no | no | no | native | gap |

55 features across 50 submodules.

### Coverage-depth honesty

- **Shallow-inventoried (principal features only; deeper inventory warranted):**
  `helix_agent` (1583 test files — only top umbrella capabilities captured;
  internal package-level features not enumerated), `helix_qa` (646 test files,
  QA infra — single capability row), `helix_specifier` (37 tests, 10 power
  features collapsed to 2 rows), `security` (deep-doc module — only a single
  umbrella row; per-package security capabilities NOT enumerated, flagged as
  needs-deeper-inventory), `panoptic` (QA infra — single row).
- **`Real-use` is `unknown`** for nearly every transitively-wired (`partial`)
  and standalone (`no`) submodule because reachability through `helix_agent`'s
  go.mod ≠ proof an end user exercises the capability via HelixCode; only the
  3 directly-replaced modules + `concurrency`/`lazy` show `yes`.
- **`Dev` reflects README claims + test presence**, NOT runtime verification.
  `toon` is honestly `stub` (PENDING_IMPLEMENTATION per its own README);
  `debate_orchestrator` aux packages are `stub` (NotYetImplemented per ACK-STUB).
- **`Tests` column** marks `unit` baseline (all 70 own-org submodules have
  `*_test.go`); `integ`/`e2e` added only where README/dir evidence shows real
  infra/integration suites (cache, storage, vector_db, llms_verifier, helix_qa,
  helix_agent, containers, database, rate_limiter, helix_llm, observability-N/A).
- Infra/tooling submodules excluded from feature rows per task scope:
  `docs_chain`, `challenges` (and `containers`/`helix_qa`/`panoptic` included
  only as the capabilities HelixCode consumes, marked QA/infra).
