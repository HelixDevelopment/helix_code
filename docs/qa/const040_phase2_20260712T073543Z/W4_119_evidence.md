# HXC-119 Phase 4 — ACP Prompt turn-generation wiring — evidence

**Scope:** `helix_code/internal/acp/{agent.go,agent_test.go,doc.go}` +
`helix_code/cmd/cli/acp_cmd.go` only. `cmd/cli/main.go`, `internal/verifier`,
`internal/rag`, `internal/approval`, `internal/tools/permissions` were NOT
touched (permission-mapping stays deferred to Phase 5).

## What changed

- `Agent.Prompt` now builds a real `llm.LLMRequest` from the ACP prompt's
  text content and calls `provider.GenerateStream(turnCtx, req, chunkChan)`
  — the SAME `llm.Provider.GenerateStream` interface method
  `cmd/cli/main.go:handleGenerate`'s streaming branch calls against real
  providers (`internal/llm/missing_types.go:384`). Each non-empty streamed
  `LLMResponse.Content` chunk is forwarded live to the ACP client via
  `a.conn.SessionUpdate(...)` as a real `session/update`
  `agent_message_chunk` notification (`acpsdk.UpdateAgentMessageText`).
- `Agent` gained a `provider llm.Provider` field (constructor-injected via
  `NewAgent(provider)`) and a `conn *acpsdk.AgentSideConnection` field
  (injected post-construction via the new `Agent.SetConnection` method,
  mirroring the coder/acp-go-sdk's own `example/agent/main.go`
  `SetAgentConnection` convention).
- `Agent.Cancel` is now real: `sessionState.cancel` holds the
  `context.CancelFunc` for whatever turn is in flight; a `session/cancel`
  notification invokes it, unblocking `provider.GenerateStream` and causing
  the pending `Prompt` call to return
  `PromptResponse{StopReason: StopReasonCancelled}`.
- Round-46 partial-error chunks (`llm.ErrResponseTruncated` /
  `llm.ErrResponseContentBlocked`) are honestly reflected as
  `StopReasonMaxTokens` / `StopReasonRefusal` rather than silently
  swallowed into `StopReasonEndTurn`.
- `cmd/cli/acp_cmd.go`'s `ACPCommandDeps` gained a `Provider llm.Provider`
  field. When unset (production default), `newACPCommand`'s `RunE` calls
  the existing `buildSubagentLLMProvider(cmd.Context())` (already defined
  in `cmd/cli/main.go` for the analogous "no full CLI bootstrap available"
  subagent-child scenario) to construct a REAL provider
  (cloud-provider-from-config first, local Ollama fallback — never a stub).
  `main.go`'s dispatcher call site (`ACPCommandDeps{In: os.Stdin, Out:
  os.Stdout}`) was left untouched, matching the "do not edit main.go"
  constraint — it still compiles because `Provider` and `NewAgent` are just
  additional/changed struct fields with usable zero values.
  `newACPCommand` also now wires `agent.SetConnection(conn)` via a narrow
  type assertion immediately after constructing the connection, before
  `<-conn.Done()`.

## How Prompt now delegates to GenerateStream (cited interface)

`internal/llm/missing_types.go:368-400`:

```go
type Provider interface {
    ...
    Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error)
    GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error
    ...
}
```

`internal/acp/agent.go` (`Agent.Prompt`) calls this exact method:

```go
chunkChan := make(chan llm.LLMResponse, 100)
errCh := make(chan error, 1)
go func() {
    errCh <- provider.GenerateStream(turnCtx, req, chunkChan)
}()

for chunk := range chunkChan {
    ...
    conn.SessionUpdate(turnCtx, acpsdk.SessionNotification{
        SessionId: params.SessionId,
        Update:    acpsdk.UpdateAgentMessageText(chunk.Content),
    })
}
```

This is the identical channel-consumption shape
`cmd/cli/main.go:handleGenerate`'s streaming branch already uses
(`chunkChan := make(chan llm.LLMResponse, 100); go func() { errCh <-
provider.GenerateStream(ctx, req, chunkChan) }()`, `main.go:1820-1824`) — no
new provider-calling convention was invented; the ACP path reuses the exact
production streaming contract (including the provider-owns-`close(ch)`
channel-ownership rule documented on the `Provider.GenerateStream` doc
comment).

## Confirmation: permission-flow untouched

- `git diff --stat` for this change touches ONLY
  `helix_code/internal/acp/{agent.go,agent_test.go,doc.go}` and
  `helix_code/cmd/cli/acp_cmd.go`.
- `grep -rn "approval\|permissions" helix_code/internal/acp/*.go
  helix_code/cmd/cli/acp_cmd.go` returns exactly two hits, BOTH doc
  comments citing `internal/approval` / `internal/tools/permissions` as the
  still-deferred Phase 5 target — zero CODE references (no import, no call)
  to either package (verified below).
- `Agent.Prompt` never calls `acp.RequestPermission` / `session/request_permission`
  — it only streams model text, exactly as documented in `doc.go` and in the
  new `Prompt` doc comment ("This method never needs to request permission
  because it does not yet perform any tool-call/file-write action").
- `internal/approval`, `internal/tools/permissions`, `internal/verifier`,
  `internal/rag`, and `cmd/cli/main.go` were not opened for writing in this
  session (only `internal/acp/*.go` and `cmd/cli/acp_cmd.go` were edited).

```
$ grep -rn "approval\|permissions" internal/acp/*.go cmd/cli/acp_cmd.go
internal/acp/agent.go:156:// onto HelixCode's internal/approval / internal/tools/permissions) remains
internal/acp/doc.go:33://     internal/approval / internal/tools/permissions subsystems —
```

Both hits are `//`-prefixed doc-comment lines (confirmed by the `:156:` /
`:33:` line content above, each beginning with `//`), not `import` lines or
function calls — i.e. exactly the citations that document the Phase 5
deferral, with no actual dependency on either package.

## RED -> GREEN (§11.4.115)

RED: `internal/acp/agent.go` + `doc.go` were stashed back to the committed
Phase 1-3 state (`git stash push -- internal/acp/agent.go internal/acp/doc.go`)
while the new Phase 4 test file (`agent_test.go`, requiring
`NewAgent(provider)` + `Agent.SetConnection`) stayed in place:

```
$ go vet ./internal/acp/...
# dev.helix.code/internal/acp
# [dev.helix.code/internal/acp]
vet: internal/acp/agent_test.go:203:20: too many arguments in call to NewAgent
	have (llm.Provider)
	want ()
```

This is a genuine RED: the new tests do not compile against the old
`NewAgent()` (no-arg) / no-`SetConnection` Phase 1-3 `Agent`, proving they
exercise new code.

`git stash pop` restored the Phase 4 implementation.

GREEN: `go test -tags=nogui ./internal/acp/... -count=1 -v`

```
=== RUN   TestNewAgentSideConnection_WiresWithoutPanic
--- PASS: TestNewAgentSideConnection_WiresWithoutPanic (0.00s)
=== RUN   TestAgent_RealHandshake
--- PASS: TestAgent_RealHandshake (0.00s)
=== RUN   TestAgent_NewSessionThenPromptHonestErrors
--- PASS: TestAgent_NewSessionThenPromptHonestErrors (0.00s)
=== RUN   TestAgent_PromptDelegatesToRealGenerateStream
--- PASS: TestAgent_PromptDelegatesToRealGenerateStream (0.00s)
=== RUN   TestAgent_PromptTruncatedStopReason
--- PASS: TestAgent_PromptTruncatedStopReason (0.00s)
=== RUN   TestAgent_CancelStopsRealInFlightGeneration
--- PASS: TestAgent_CancelStopsRealInFlightGeneration (0.05s)
PASS
ok  	dev.helix.code/internal/acp	0.176s
```

`TestAgent_PromptDelegatesToRealGenerateStream` is the anti-bluff core: it
constructs a unit-test-only `fakeStreamingProvider` (§11.4.27(A): fakes
permitted only in unit tests) that streams TEST-CHOSEN content
(`REAL-STREAM-CHUNK-A-7c1`, `-B-9d2`, `-C-e01`) and records the real
`*llm.LLMRequest` it received; it then asserts, over a REAL ACP JSON-RPC
`io.Pipe` round trip:

1. The LLM request `Agent.Prompt` built carries the EXACT ACP prompt text
   this test sent (`HXC-119-PHASE-4-FORWARD-PROBE-3f9a`) — proving genuine
   ACP-prompt -> LLM-request translation, not a canned request.
2. The real ACP client on the other end of the wire received EXACTLY the
   fake provider's streamed chunks as `session/update` notifications — a
   value `Prompt` could not have hardcoded, since it was chosen by the test
   at run time.

`TestAgent_CancelStopsRealInFlightGeneration` proves `session/cancel`
genuinely stops a real in-flight `Prompt` call (not a no-op): the fake
provider blocks until context-cancelled, a real ACP `Cancel` notification is
sent while `Prompt` is provably still in flight, and the pending `Prompt`
call returns `StopReason: StopReasonCancelled` within the timeout.

## Build

```
$ cd helix_code && go build -tags=nogui ./internal/acp/... ./cmd/cli/...
(exit 0, no output)

$ go test -tags=nogui ./internal/acp/... -count=1
ok  	dev.helix.code/internal/acp	0.176s

$ go test -tags=nogui ./cmd/cli/... -count=1 -run ACP
ok  	dev.helix.code/cmd/cli	0.064s (TestNewACPCommand_RealHandshakeOverStdioTransport PASS)

$ go vet -tags=nogui ./internal/acp/... ./cmd/cli/...
(exit 0, no output)

$ go build -tags=nogui ./...
(exit 0, no output — full inner-app build, including the unrelated
concurrent HXC-117/HXC-118 track work already present in
cmd/cli/main.go / internal/verifier / internal/rag, none of which this
Phase-4 change touched)
```

No changes were committed (per dispatch instructions).
