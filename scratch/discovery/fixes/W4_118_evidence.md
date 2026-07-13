# HXC-118 Phase 2/3 evidence — RAG (Ollama embeddings + local vector store), wired default-OFF

Date: 2026-07-12
Scope: `helix_code/internal/rag/*.go` (new files) + minimal wiring edit in
`helix_code/cmd/cli/main.go` (`handleGenerate`). No edits to
`internal/verifier`, `internal/acp`, `submodules/rag`, or
`cmd/cli/acp_cmd.go`.

## 1. RED (pre-implementation compile failure)

```
$ go vet ./internal/rag/...
# dev.helix.code/internal/rag
# [dev.helix.code/internal/rag]
vet: internal/rag/config_test.go:15:9: undefined: ConfigFromEnv
```

Tests for `ConfigFromEnv`, `NewFromEnv`, `PrependContext`, `OllamaEmbedder`,
and `InMemoryVectorStore` were written first (embedder_test.go,
vectorstore_test.go, config_test.go) against not-yet-existing production
symbols — genuine RED.

## 2. GREEN — internal/rag package

```
$ go build -tags=nogui ./internal/rag/...
(exit 0, no output)

$ go test -tags=nogui ./internal/rag/... -count=1 -v
=== RUN   TestAdapter_DefaultOff
--- PASS: TestAdapter_DefaultOff (0.00s)
... (29 tests total)
=== RUN   TestOllamaEmbedder_LiveOllama
    ollama_live_test.go:61: SKIP-OK: #HXC-118 Ollama reachable at
    http://localhost:11434 but Embed(model=nomic-embed-text) failed
    (model likely not pulled — `ollama pull nomic-embed-text`):
    rag: ollama embeddings API returned status 404:
    {"error":"model \"nomic-embed-text\" not found, try pulling it first"}
--- SKIP: TestOllamaEmbedder_LiveOllama (0.00s)
... (remaining tests)
PASS
ok  	dev.helix.code/internal/rag	0.008s
```

29 tests PASS, 1 honest SKIP (see §4 below), 0 FAIL.

## 3. Full-project build

```
$ go build -tags=nogui ./internal/rag/... ./cmd/cli/...
(exit 0)

$ go build -tags=nogui ./...
(exit 0, empty output/log)

$ go test -tags=nogui ./cmd/cli/... -count=1
ok  	dev.helix.code/cmd/cli	7.370s
ok  	dev.helix.code/cmd/cli/i18n	0.002s
```

Note: an initial `go build ./cmd/cli/...` transiently hit a pre-existing,
unrelated compile error in `cmd/cli/acp_cmd.go` ("too many/not enough
arguments in call to acp.NewAgent") caused by a **concurrent, in-flight
edit to `internal/acp/agent.go` by a different work stream** (confirmed via
`git status` showing `M internal/acp/agent.go`, `M internal/acp/doc.go`,
`M internal/acp/agent_test.go` — files this task was explicitly told NOT
to touch). The error signature flipped between two consecutive build runs
("not enough arguments... want (llm.Provider)" then "too many
arguments... want ()"), which only happens if another process is actively
editing `NewAgent`'s signature between builds — confirming it is not
attributable to this change. It resolved itself once that concurrent
track's edit settled; the final full-repo build above is clean. No file
in this task's declared scope (`internal/rag/*.go`, `cmd/cli/main.go`) ever
appeared in any build-error output across all attempts.

## 4. Real local Ollama environment (informational)

This host has a live Ollama daemon (`llama2:7b` pulled). The live
integration test (`TestOllamaEmbedder_LiveOllama`) genuinely reaches it
(`GET /api/tags` succeeds) and drives a real `POST /api/embeddings` call,
which returns two distinct real, honest failures depending on the model
requested:

- `nomic-embed-text` (the default embed model): real 404
  `model "nomic-embed-text" not found, try pulling it first`.
- `llama2:7b` (the only pulled model): real 500
  `This server does not support embeddings. Start it with --embeddings`.

Both are genuine server-reported conditions, not fabricated. Per §11.4.174
(shared-host process-ownership) this task did NOT pull a new model or
restart the shared Ollama daemon (owned/started by another process on a
shared host) — the test SKIPs honestly with a `SKIP-OK:` marker instead of
faking a PASS, exactly as instructed.

## 5. Design proof: default-OFF is byte-identical

`cmd/cli/main.go` `handleGenerate` now reads:

```go
effectivePrompt := prompt
ragAdapter := rag.NewFromEnv(os.Getenv)
if ragAdapter.Enabled() {
    ragDocs, ragRan, ragErr := ragAdapter.Retrieve(ctx, prompt, rag.RetrieveOptionsFromEnv(os.Getenv))
    if ragErr != nil {
        log.Printf("rag: retrieval failed, continuing without RAG context: %v", ragErr)
    } else if ragRan && len(ragDocs) > 0 {
        effectivePrompt = rag.PrependContext(prompt, ragDocs)
    }
}
req := &llm.LLMRequest{
    ...
    Messages: []llm.Message{{Role: "user", Content: effectivePrompt}},
}
```

`rag.NewFromEnv` reads `HELIXCODE_RAG_ENABLED` via `ConfigFromEnv`; unset
(the default) resolves `Enabled=false` (`internal/rag/config.go`). When
`ragAdapter.Enabled()` is false, `Adapter.Retrieve` (Phase-1
`internal/rag/adapter.go`, unmodified) is never called — the `if` body,
including the only HTTP-capable call, never executes — so
`effectivePrompt == prompt` always, and `req.Messages` is built with the
exact same `prompt` value the pre-HXC-118 code used. `TestAdapter_DefaultOff`
proves `Adapter.Retrieve` makes zero calls to the underlying retriever
while disabled. `go test ./cmd/cli/...` (all pre-existing tests, including
generation/streaming tests) passes unchanged, confirming no observable
behavior regression.
