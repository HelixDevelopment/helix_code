package main

// Anti-bluff anchor: this test proves the `helixcode acp` cobra command
// really wires acpsdk.NewAgentSideConnection over the deps.In/deps.Out
// transport (a real io.Pipe here, os.Stdin/os.Stdout in production) rather
// than merely returning nil without touching the SDK. It drives a genuine
// ACP `initialize` JSON-RPC round trip through the command's RunE via a
// real acpsdk.ClientSideConnection on the other end of the pipe, then
// closes the transport to unblock RunE's <-conn.Done() wait.

import (
	"context"
	"io"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func TestNewACPCommand_RealHandshakeOverStdioTransport(t *testing.T) {
	// cmdIn/cmdOut model the command's stdio: the command reads from
	// cmdIn (fed by the test-side client's writes) and writes to cmdOut
	// (read by the test-side client).
	cmdInR, cmdInW := io.Pipe()
	cmdOutR, cmdOutW := io.Pipe()

	cmd := newACPCommand(ACPCommandDeps{In: cmdInR, Out: cmdOutW})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	// Real client-side connection driving the other end of the same pipes
	// the command's RunE just wired via acpsdk.NewAgentSideConnection.
	client := acpsdk.NewClientSideConnection(testNoopClient{}, cmdInW, cmdOutR)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Initialize(ctx, acpsdk.InitializeRequest{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
	})
	require.NoError(t, err, "real ACP initialize round-trip through the cobra command failed")
	require.Equal(t, acpsdk.ProtocolVersion(acpsdk.ProtocolVersionNumber), resp.ProtocolVersion)
	require.NotNil(t, resp.AgentInfo)
	require.Equal(t, "helixcode", resp.AgentInfo.Name)

	// Closing the transport lets the command's RunE observe peer
	// disconnect (conn.Done()) and return, proving RunE genuinely blocks on
	// the real connection rather than returning immediately.
	_ = cmdInW.Close()
	_ = cmdOutW.Close()
	_ = cmdInR.Close()
	_ = cmdOutR.Close()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("helixcode acp command did not return after peer disconnect")
	}
}

// testNoopClient is a minimal real (not mocked-out) acpsdk.Client used only
// to drive the peer side of the wire in this test.
type testNoopClient struct{}

func (testNoopClient) ReadTextFile(context.Context, acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	return acpsdk.ReadTextFileResponse{}, acpsdk.NewMethodNotFound("fs/read_text_file")
}

func (testNoopClient) WriteTextFile(context.Context, acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	return acpsdk.WriteTextFileResponse{}, acpsdk.NewMethodNotFound("fs/write_text_file")
}

func (testNoopClient) RequestPermission(context.Context, acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	return acpsdk.RequestPermissionResponse{}, acpsdk.NewMethodNotFound("session/request_permission")
}

func (testNoopClient) SessionUpdate(context.Context, acpsdk.SessionNotification) error { return nil }

func (testNoopClient) CreateTerminal(context.Context, acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
	return acpsdk.CreateTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/create")
}

func (testNoopClient) KillTerminal(context.Context, acpsdk.KillTerminalRequest) (acpsdk.KillTerminalResponse, error) {
	return acpsdk.KillTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/kill")
}

func (testNoopClient) TerminalOutput(context.Context, acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error) {
	return acpsdk.TerminalOutputResponse{}, acpsdk.NewMethodNotFound("terminal/output")
}

func (testNoopClient) ReleaseTerminal(context.Context, acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error) {
	return acpsdk.ReleaseTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/release")
}

func (testNoopClient) WaitForTerminalExit(context.Context, acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error) {
	return acpsdk.WaitForTerminalExitResponse{}, acpsdk.NewMethodNotFound("terminal/wait_for_exit")
}

var _ acpsdk.Client = testNoopClient{}
