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
// transport using github.com/coder/acp-go-sdk — it is not a simulation. As
// of HXC-119 Phase 4, it also wires a REAL LLM provider into the agent so
// Prompt turn generation genuinely delegates to
// llm.Provider.GenerateStream (see buildProductionACPProvider below).
// Permission-request mapping remains explicitly deferred to a later,
// separately reviewed phase (HXC-119 Phase 5) and is not touched by this
// file.
//
// Anti-bluff anchor: no fabricated I/O, no in-memory shadow protocol state,
// no fabricated LLM output. deps.In/deps.Out are wired to os.Stdin/
// os.Stdout in production (newACPCommand's dispatcher call in main.go) and
// to a real io.Pipe in tests (acp_cmd_test.go) — never a stand-in fake
// transport. deps.Provider, when unset, is a REAL provider constructed by
// buildSubagentLLMProvider (cmd/cli/main.go) — never a stub or fake outside
// *_test.go.

import (
	"fmt"
	"io"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/spf13/cobra"

	"dev.helix.code/internal/acp"
	"dev.helix.code/internal/llm"
)

// ACPCommandDeps wires test seams for the "acp" cobra command.
type ACPCommandDeps struct {
	// In is the transport HelixCode reads incoming ACP JSON-RPC messages
	// from (the editor's outbound stream). Production wires os.Stdin.
	In io.Reader
	// Out is the transport HelixCode writes outgoing ACP JSON-RPC messages
	// to (the editor's inbound stream). Production wires os.Stdout.
	Out io.Writer
	// NewAgent, when set, overrides the ACP Agent CONSTRUCTOR the command
	// wires up (receiving the resolved Provider, see below). Production
	// leaves this nil so newACPCommand always constructs the real
	// internal/acp.Agent; tests may substitute a narrower double to isolate
	// the cobra/transport wiring from the agent's own (independently
	// tested) behavior.
	NewAgent func(provider llm.Provider) acpsdk.Agent
	// Provider, when set, overrides the REAL LLM provider newACPCommand
	// wires into the ACP Agent's turn-generation path (HXC-119 Phase 4:
	// Agent.Prompt -> Provider.GenerateStream). Production leaves this nil
	// so newACPCommand constructs a REAL provider via
	// buildSubagentLLMProvider (cmd/cli/main.go) — the same config-file +
	// HELIX_LLM_PROVIDER-env-driven, cloud-provider-first /
	// local-Ollama-fallback construction already used for the analogous
	// "no full CLI bootstrap available" subagent-child scenario. Tests may
	// substitute a narrower double (§11.4.27(A): unit tests only) to
	// isolate the cobra/transport wiring from real generation.
	Provider llm.Provider
}

// newACPCommand builds the `helixcode acp` cobra command. Running it
// resolves a REAL LLM provider (deps.Provider if set, otherwise a real
// provider from buildSubagentLLMProvider), constructs the ACP Agent wired
// to that provider, establishes a real acp.AgentSideConnection over
// deps.In/deps.Out, wires the connection back into the agent so Prompt can
// emit real session/update notifications, and blocks until the peer
// disconnects (conn.Done() closes) — the same stdio-subprocess lifecycle
// every ACP agent (Zed, JetBrains-compatible agents) implements.
func newACPCommand(deps ACPCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acp",
		Short: trc("cli_acp_root_short", nil),
		Long:  trc("cli_acp_root_long", nil),
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider := deps.Provider
			if provider == nil {
				p, err := buildSubagentLLMProvider(cmd.Context())
				if err != nil {
					return fmt.Errorf("acp: failed to construct LLM provider: %w", err)
				}
				provider = p
			}

			newAgent := deps.NewAgent
			if newAgent == nil {
				newAgent = func(provider llm.Provider) acpsdk.Agent { return acp.NewAgent(provider) }
			}
			agent := newAgent(provider)

			conn := acpsdk.NewAgentSideConnection(agent, deps.Out, deps.In)
			// Wire the live connection back into the agent BEFORE it can
			// dispatch any incoming request, so a Prompt call always has a
			// real conn.SessionUpdate path available (see
			// internal/acp.Agent.SetConnection's doc comment for the
			// ordering contract this mirrors from the SDK's own
			// example/agent/main.go).
			if setter, ok := agent.(interface {
				SetConnection(*acpsdk.AgentSideConnection)
			}); ok {
				setter.SetConnection(conn)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), trc("cli_acp_listening", nil))
			<-conn.Done()
			return nil
		},
	}
	return cmd
}
