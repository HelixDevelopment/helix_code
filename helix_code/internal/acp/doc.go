// Package acp implements HelixCode's agent-side role in the Agent Client
// Protocol (ACP) — a JSON-RPC-2.0, LSP-modeled protocol (agentclientprotocol.com,
// jointly maintained by Zed and JetBrains) that standardizes communication
// between editors ("clients") and AI coding agents ("agents"). HelixCode is
// always the ACP *agent* side, analogous to how Claude Code / Gemini CLI act
// as ACP agents that editors connect to.
//
// This package wraps the maintained third-party Go SDK
// github.com/coder/acp-go-sdk (pinned at v0.13.5; see go.mod), which supplies
// the acp.Agent interface, JSON-RPC transport, and typed request/response
// plumbing. HelixCode supplies the Agent implementation and CLI stdio
// entrypoint (cmd/cli/acp_cmd.go).
//
// Scope note (HXC-119, docs/research/const040_capability_model_20260712/DESIGN.md
// §4 and §5): this package establishes a real ACP handshake
// (initialize / session/new) over stdio, tracks real session state, and
// (as of Phase 4) routes Prompt turn generation through HelixCode's real LLM
// provider path: Agent.Prompt builds an llm.LLMRequest from the prompt's
// text content and calls provider.GenerateStream — the SAME
// llm.Provider.GenerateStream method cmd/cli/main.go:handleGenerate's
// streaming branch calls against real providers — and forwards each
// streamed chunk to the connected ACP client as a real `session/update`
// agent_message_chunk notification. The injected provider is supplied by
// the caller (cmd/cli/acp_cmd.go's newACPCommand, via
// buildSubagentLLMProvider in production); this package never constructs
// its own provider, keeping provider-selection policy out of the protocol
// layer.
//
// As of Phase 5 (HXC-119, Option B — tool-execution gate), ACP permission
// requests are mapped onto HelixCode's internal/tools/permissions engine via
// the PermissionAdapter (permission.go). When the agent receives a tool call
// from the ACP client, it evaluates the call against the permissions engine:
// ActionAllow → execute without asking; ActionDeny → reject; ActionAsk →
// call conn.RequestPermission on the client, presenting allow/reject options.
// If the engine is unavailable, all requests default to ActionAsk (fail-closed,
// never auto-approve). The adapter is injected by the caller via
// SetPermissionAdapter, keeping this package decoupled from engine construction.
//
// Constitutional anchors: CONST-040 (capability integration — see HXC-117 for
// the VerificationResult.SupportsACP field this package will read from in a
// later phase), §11.4.112 (this feature was deep-researched and found
// IMPLEMENTABLE, not structurally impossible — see DESIGN.md §4.3),
// §11.4.150 (deep multi-angle research performed before any code was
// written — see DESIGN.md "Sources verified"), §11.4.6/§11.4.107 (Prompt's
// GenerateStream delegation is genuine wiring, never a fabricated or
// hardcoded response — see agent_test.go's real-provider-double assertion).
package acp
