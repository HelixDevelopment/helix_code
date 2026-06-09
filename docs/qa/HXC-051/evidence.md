# HXC-051 — helix_llm + helix_memory go.mod CONST-052 build-breaks
**Captured:** 2026-06-09T14:54:03Z · Bug · Fixed (→ Fixed.md)
## helix_llm
36 capitalised `replace => ../../vasic-digital/<Cap>` directives repointed to flat lowercase siblings (e.g. `=> ../config`, `=> ../event_bus`, `=> ../mcp_module`, `=> ../vector_db`). Resolving paths exposed a missing toon go.sum entry → `go mod download`+`tidy` (1 indirect require added). `go build ./...` → exit 0 (LLM_BUILD_OK). Commit `72a1fdf`, pushed ff `32334d5..72a1fdf` (github+gitlab tips confirmed).
## helix_memory
`digital.vasic.memory => ../../vasic-digital/Memory` → `=> ../memory`. `go build ./...` → exit 0 (MEMORY_BUILD_OK). Commit `a677adc`, pushed ff `5f1dcb6..a677adc`.
