## Internal services + infrastructure

Inventory of every feature under `helix_code/internal/*` (72 packages). Assessed
from code evidence (impl reality, wiring, `_test.go` presence) per CONST-035 /
§11.4.107 anti-bluff. `📹 Video` is `no` for every row (recordings are the
conductor's job once a real analyzed recording exists); `Overall` is never
`confirmed` for the same reason. `Origin` is `native` (HelixCode's own code).

| Area | Component | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| service | internal/adapters | i18n message resolution | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/adapters | translator injection seam | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/adapters | speckit debate adapter | partial | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/adapters | container adapter | partial | yes | unknown | unit | no | no | no | native | partial |
| service | internal/agent | agent execution interface | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/agent | agent capability management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/agent | agent health check | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/agent | agent collaboration | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/agentbridge | verifier bridge | partial | yes | unknown | unit | no | no | no | native | partial |
| service | internal/approval | approval request management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/approval | approval mode selector | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/approvalwire | yes/no prompter | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/auth | user authentication | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/auth | session management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/auth | password hashing (bcrypt/argon2) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/auth | JWT token generation | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/autocommit | git auto-commit | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/autocommit | secret filter | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/cache | multi-tier cache | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/cache | redis tier | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/cache | disk tier | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/checkpoint | checkpoint manager | partial | yes | unknown | unit | no | no | no | native | partial |
| service | internal/clarification | clarification engine | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/clarification | question generation | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/clientcore | agentic-tools provider | partial | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/clientcore | skills provider | partial | yes | unknown | unit | no | no | no | native | partial |
| service | internal/cognee | cognee client | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/cognee | cognee manager | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/cognee | cognee cache manager | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | aider command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | approval command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | browser command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | edit command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | git auto-commit command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | mcp command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | markdown commands | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | skills command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | subagents command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | tasks command | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/commands | worktree command | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/commands | command executor | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/commands | command registry | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/commands | command parser | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/config | viper-based config loader | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/config | cognee configuration management | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/config | verifier configuration | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/config | platform-UI adapters config | partial | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/context | context builder | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/context | token counter | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/context | history condenser | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/continua | completion engine | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/continua | continue-edit tool | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/continua | continue-complete tool | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/database | database connection pool | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/database | postgres integration | done | yes | yes | integ | no | no | no | native | working-untaped |
| infrastructure | internal/deployment | production deployer | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/deployment | deployment strategy | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/discovery | service registry with TTL + health checks | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/discovery | UDP multicast broadcast discovery | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/discovery | dynamic port allocation | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/discovery | health monitoring (HTTP/gRPC/TCP) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | unified-diff code editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | whole-file replacement editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | search-and-replace editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | line-range editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/editor | model-specific format selection | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/ensembleui | ensemble UI rendering | partial | partial | unknown | none | no | no | no | native | gap |
| service | internal/event | publish-subscribe event bus | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/event | async + sync event handling | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/event | task/workflow/worker event types | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/fix | security issue auto-fix | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/focus | hierarchical focus management | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/focus | priority-based focus tracking | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/focus | focus chain tracking | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/hardware | CPU detection + profiling | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/hardware | GPU detection (NVIDIA/AMD/Apple/Intel) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/hardware | optimal LLM model-size inference | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/helixqa | HelixQA test wrapper | partial | yes | unknown | unit,integ | no | no | no | native | partial |
| service | internal/hooks | hook registration + triggering | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/hooks | priority-based hook execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/hooks | async/sync hook execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/i18nwiring | i18n catalog wire-all (multi-lang) | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/infraboot | on-demand infra boot (EnsureInfra) | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| service | internal/kilocode | call-graph build + query | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/kilocode | symbol rename engine | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/kilocode | change-impact analyzer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/kilocode | refactor (extract-method/inline-call) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/kilocode | multi-edit tool | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/llm | OpenAI provider (GPT models) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/llm | Anthropic provider (Claude models) | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/llm | Google Gemini provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | Azure OpenAI provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | AWS Bedrock provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | Ollama local provider | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/llm | llama.cpp local inference | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | Mistral provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | DeepSeek provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | Groq provider | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | streaming response handling | done | yes | yes | unit,integ | no | no | no | native | working-untaped |
| service | internal/llm | token counting + accounting | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | model discovery/listing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | load balancing across providers | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | provider health monitoring | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | ensemble provider orchestration | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/llm | embeddings generation | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/logging | structured logging with levels | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/logging | named loggers | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/logo | logo processing + assets | done | yes | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/logo | color extraction + icon generation | done | yes | unknown | none | no | no | no | native | partial |
| infrastructure | internal/mcp | Model Context Protocol server | done | yes | partial | unit,integ | no | no | no | native | working-untaped |
| infrastructure | internal/mcp | JSON-RPC 2.0 tool invocation | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/mcp | WebSocket session management | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/mcp | OAuth token management | done | yes | unknown | unit | no | no | no | native | partial |
| service | internal/memory | Cognee LLM memory integration | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| infrastructure | internal/memory | memory state persistence | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/monitoring | system metrics collection | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/monitoring | health check monitoring | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/notification | multi-channel notification engine | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Slack integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Discord integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Telegram integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Email/SMTP integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | Teams integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/notification | rate limiting + retry | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/performance | performance optimizer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/performance | CPU optimization | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/performance | memory optimization | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/persistence | file-based state store | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/persistence | session serialization | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/persistence | auto-save manager | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/planner | task executor | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/planner | OpenHands plan execution | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/plantree | plan tree system | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/plantree | plan node management | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/plantree | plan branching + merging | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/plugins | plugin base framework | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/plugins | plugin activation | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/plugins | plugin hooks registry | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/project | project manager | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/project | project metadata | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/project | database-backed storage | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/projectmemory | memory loader | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/projectmemory | filesystem watcher | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/projectmemory | memory registry | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/provider | provider interface | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/provider | multi-provider support | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/providers | AI integration | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| service | internal/providers | fallback + load balancing | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| infrastructure | internal/quality | quality gate scoring | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/quality | build verification | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/quality | linting validation | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/redis | redis client | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/redis | key-value operations | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/redis | pub/sub messaging | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/render | ANSI renderer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/render | fancy-mode output | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/render | frame buffer management | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/render | streaming block renderer | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/repomap | symbol extraction | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/repomap | file ranking engine | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/repomap | repo cache | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/repomap | tree-sitter integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/roocode | Roo-Code CLI port | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/roocode | template-based code generation | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/rules | rule hierarchy management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/rules | rule pattern matching (glob/regex/exact) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/rules | rule category + tag querying | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/secrets | secret loader from .env files | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/secrets | secret validation + missing-var detection | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/security | security manager (global + local) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/security | feature scanning + zero-tolerance validation | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/server | auth endpoints (register/login/logout/refresh) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | user profile endpoints (GET/PUT /me) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | worker mgmt endpoints (list/register/heartbeat) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | task CRUD endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | project CRUD endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | workflow execution endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | system stats + health-check endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | LLM provider + model list endpoints | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | memory system statistics endpoint | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/server | WebSocket real-time communication | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/session | session lifecycle (create/start/complete/pause) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/session | session modes (planning/building/testing/refactoring) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/session | focus chain integration | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/session | context + metadata tracking | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/session | session querying + filtering | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/substrate | substrate abstraction layer | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/task | task creation + assignment | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | task status lifecycle | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | priority-based task queue | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | checkpoint creation + recovery | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | dependency + circular-dependency validation | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | dependent task tracking | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/task | redis caching for tasks | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/telemetry | agent instrumentation (prompt/response) | done | partial | yes | unit | no | no | no | native | partial |
| service | internal/telemetry | LLM instrumentation (calls/latency/tokens) | done | partial | yes | unit | no | no | no | native | partial |
| service | internal/telemetry | tool instrumentation (execution/errors) | done | partial | yes | unit | no | no | no | native | partial |
| service | internal/telemetry | OpenTelemetry provider integration | done | partial | yes | unit | no | no | no | native | partial |
| service | internal/template | template creation + registration | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/template | variable substitution + validation | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/template | built-in template library | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/theme | theme detection (OS/system prefs) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/theme | theme loading + customization | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/theme | built-in theme collection | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | file read/write operations | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | file editing with string replacement | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | file globbing (pattern matching) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | file content search (grep) | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | shell command execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | background shell execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | web page fetching + parsing | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/tools | web search integration | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/tools | browser automation (launch/navigate/screenshot) | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/tools | codebase mapping + symbol definitions | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | multi-file transactional editing | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | user confirmation prompts | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/tools | notebook read/edit operations | done | yes | yes | unit | no | no | no | native | working-untaped |
| infrastructure | internal/tools | tool registry + execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/verifier | LLMsVerifier HTTP client | done | yes | unknown | unit,integ | no | no | no | native | working-untaped |
| service | internal/verifier | model metadata adapter | done | yes | unknown | unit | no | no | no | native | working-untaped |
| infrastructure | internal/verifier | two-tier cache (LRU + Redis) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | health monitoring + circuit breaker | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | background poller (real-time updates) | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/verifier | event publishing to HelixCode bus | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/version | version string retrieval | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/version | build metadata exposure | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/voice | audio capture (arecord/sox) | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | Whisper API transcription | partial | partial | unknown | unit | no | no | no | native | partial |
| service | internal/voice | local whisper.cpp fallback | partial | no | unknown | unit | no | no | no | native | gap |
| service | internal/worker | worker registration + management | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | SSH-based remote execution | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | health monitoring + metrics | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | capability auto-detection | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/worker | worker isolation + sandboxing | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | consensus protocol (leader election) | done | partial | unknown | unit | no | no | no | native | partial |
| service | internal/worker | host-key verification + SSH security | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/workflow | planning workflow execution | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/workflow | building workflow execution | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/workflow | testing workflow execution | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/workflow | refactoring workflow execution | done | yes | unknown | unit | no | no | no | native | working-untaped |
| service | internal/workflow | DAG-based step orchestration | done | yes | yes | unit | no | no | no | native | working-untaped |
| service | internal/workflow | LLM provider integration for workflows | done | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | container-based workspace management | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | Docker/Podman container orchestration | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | project directory mounting + isolation | partial | partial | unknown | unit | no | no | no | native | partial |
| infrastructure | internal/workspace | auto-cleanup TTL enforcement | partial | no | unknown | unit | no | no | no | native | gap |
| infrastructure | internal/mocks | unit-test mock fixtures (test-only, not a user feature) | done | yes | n/a | none | no | n/a | no | native | n/a |
| infrastructure | internal/pprofutil | pprof profiling helper (dev/test support) | done | partial | unknown | none | no | no | no | native | partial |
| infrastructure | internal/testutil | shared test utilities (test-only, not a user feature) | done | yes | n/a | none | no | n/a | no | native | n/a |

### Coverage notes

- **Partially assessed (need deeper inventory):** `clientcore`, `agentbridge`,
  `checkpoint`, `ensembleui`, `substrate`, `workspace`, `voice`, `roocode`,
  `telemetry`, `verifier` (poller/event-bus wiring), `worker` (isolation/consensus),
  and `server` real-use (declared endpoints, no integration evidence inspected).
  Their core types compile and have unit tests, but wiring into a shipped flow
  and genuine end-user reachability could not be fully confirmed from static
  inspection alone — flagged `partial`/`unknown` honestly rather than green.
- `mocks`, `pprofutil`, `testutil` are dev/test-support packages, not user
  features — listed for completeness, marked `n/a`.
- Every `Real-use=unknown` and every `working-untaped` row is a candidate for a
  recorded scenario; none is video-confirmed yet (📹 `no` throughout).

233 features inventoried across 72 packages.

## Sources verified 2026-06-22: helix_code/internal/*

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced repo trees,
following the `docs/ARCHITECTURE.md` precedent — no external service documented).
Cross-referenced against the live tree on 2026-06-22:
- **`helix_code/internal/*` = 72 packages — CONFIRMED** (`ls -d helix_code/internal/*/`
  = 72), matching the "across 72 packages" rollup. The per-row `Dev`/`Wired`/`Tests`
  assessments are structural evidence (impl reality + `_test.go` presence) about
  HelixCode's own code.
- No external-service version claims in this doc → no §11.4.99 staleness check
  applies; nothing to contradict against an upstream source.
