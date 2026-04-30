// Package verifier integrates LLMsVerifier as the single source of truth
// for model metadata, provider health, verification status, and scoring
// within HelixCode.
//
// Architecture:
//
//	HelixCode (cmd/cli, cmd/server, internal/llm)
//	       |
//	       | REST API calls (HTTP/json)
//	       v
//	internal/verifier/Client -> LLMsVerifier service (localhost:8081)
//	       |
//	       | SQLite DB read
//	       v
//	LLMsVerifier (submodule) -> Provider APIs
//
// Key types:
//   - Client: HTTP REST client for verifier API
//   - Adapter: Bridges verifier scores to HelixCode's ModelManager
//   - Cache: Two-tier cache (in-memory LRU + Redis)
//   - HealthMonitor: Circuit breaker for verifier availability
//   - Poller: Background goroutine for real-time updates
//   - EventPublisher: Publishes changes to HelixCode event bus
//
// The verifier is disabled by default (verifier.enabled=false). When disabled,
// all model operations fall back to legacy behavior.
//
// Constitutional compliance:
//   - CONST-036: LLMsVerifier single source of truth
//   - CONST-037: Anti-bluff guarantee (no hardcoded models)
//   - CONST-038: Real-time status accuracy
//   - CONST-039: All providers integration
//   - CONST-040: MCP/LSP/ACP/Embedding/RAG/Skills/Plugins
package verifier
