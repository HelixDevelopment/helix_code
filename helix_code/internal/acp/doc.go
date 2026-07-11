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
// §4 and §5): this package is the Phase 1-3 scaffold — it establishes a real
// ACP handshake (initialize / session/new) over stdio and tracks real session
// state. Two capabilities are DELIBERATELY deferred to later, separately
// reviewed phases and are NOT implemented here:
//
//   - Prompt turn generation wired to HelixCode's real LLM provider path
//     (streaming session/update notifications over the existing
//     GenerateStream call) — HXC-119 Phase 4.
//   - ACP permission requests mapped onto HelixCode's existing
//     internal/approval / internal/tools/permissions subsystems —
//     HXC-119 Phase 5 (explicitly flagged medium-high risk, requires its own
//     security-focused review pass before any code lands).
//
// Because those two capabilities are not implemented, Prompt returns a real,
// honest JSON-RPC protocol error (acp.NewInternalError) rather than a
// fabricated completion. This keeps the scaffold anti-bluff-clean per
// CLAUDE.md §3.3 / Article XI §11.9: no fabricated LLM output is ever
// returned to an ACP client from this package.
//
// Constitutional anchors: CONST-040 (capability integration — see HXC-117 for
// the VerificationResult.SupportsACP field this package will read from in a
// later phase), §11.4.112 (this feature was deep-researched and found
// IMPLEMENTABLE, not structurally impossible — see DESIGN.md §4.3),
// §11.4.150 (deep multi-angle research performed before any code was
// written — see DESIGN.md "Sources verified").
package acp
