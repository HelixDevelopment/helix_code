package main

// acp_cmd.go (HXC-119 Phase 1-3): cobra subcommand surface for `helixcode acp`.
//
// This is the stdio entrypoint an ACP-aware editor (Zed, JetBrains — see
// agentclientprotocol.com) launches as a subprocess to talk to HelixCode as
// an ACP agent. It mirrors the exact `newMCPCommand`/`MCPCommandDeps`
// pattern already established in mcp_cmd.go: a Deps struct carrying test
// seams, a constructor building a cobra.Command, and a dispatcher block in
// main.go that intercepts `os.Args[1] == "acp"` before flag.Parse() (see
// main.go, same shape as the existing "mcp" dispatcher).
//
// The command is opt-in / default-off: it only runs when the operator (or
// an editor's ACP integration) explicitly invokes `helixcode acp`. It does
// not alter the default `helixcode` CLI behavior in any other invocation.
//
// Scope boundary (see internal/acp/doc.go for the full citation): this
// entrypoint wires a REAL ACP agent-side connection over a REAL stdio
// transport using github.com/coder/acp-go-sdk — it is not a simulation.
// Prompt-turn generation and permission-request mapping are explicitly
// deferred to later, separately reviewed phases (HXC-119 Phase 4 and 5)
// and are not touched by this file.
//
// Anti-bluff anchor: no fabricated I/O, no in-memory shadow protocol state.
// deps.In/deps.Out are wired to os.Stdin/os.Stdout in production
// (newACPCommand's dispatcher call in main.go) and to a real io.Pipe in
// tests (acp_cmd_test.go) — never a stand-in fake transport.

import (
	"fmt"
	"io"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/spf13/cobra"

	"dev.helix.code/internal/acp"
)

// ACPCommandDeps wires test seams for the "acp" cobra command.
type ACPCommandDeps struct {
	// In is the transport HelixCode reads incoming ACP JSON-RPC messages
	// from (the editor's outbound stream). Production wires os.Stdin.
	In io.Reader
	// Out is the transport HelixCode writes outgoing ACP JSON-RPC messages
	// to (the editor's inbound stream). Production wires os.Stdout.
	Out io.Writer
	// NewAgent, when set, overrides the ACP Agent implementation the
	// command wires up. Production leaves this nil so newACPCommand always
	// constructs the real internal/acp.Agent; tests may substitute a
	// narrower double to isolate the cobra/transport wiring from the
	// agent's own (independently tested) behavior.
	NewAgent func() acpsdk.Agent
}

// newACPCommand builds the `helixcode acp` cobra command. Running it
// establishes a real acp.AgentSideConnection over deps.In/deps.Out and
// blocks until the peer disconnects (conn.Done() closes) — the same
// stdio-subprocess lifecycle every ACP agent (Zed, JetBrains-compatible
// agents) implements.
func newACPCommand(deps ACPCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acp",
		Short: trc("cli_acp_root_short", nil),
		Long:  trc("cli_acp_root_long", nil),
		RunE: func(cmd *cobra.Command, _ []string) error {
			newAgent := deps.NewAgent
			if newAgent == nil {
				newAgent = func() acpsdk.Agent { return acp.NewAgent() }
			}
			agent := newAgent()

			conn := acpsdk.NewAgentSideConnection(agent, deps.Out, deps.In)
			fmt.Fprintln(cmd.ErrOrStderr(), trc("cli_acp_listening", nil))
			<-conn.Done()
			return nil
		},
	}
	return cmd
}
