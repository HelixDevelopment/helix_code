package acp

// permission_test.go — HXC-119 Phase 5: tests for the ACP permission
// adapter mapping onto internal/tools/permissions (Option B).
//
// CONST-050(A): unit tests with real types, no mocks beyond the
// permissions engine's own test seam.

import (
	"testing"

	"dev.helix.code/internal/tools/confirmation"

	"github.com/stretchr/testify/require"
)

func TestBuildConfirmationRequest_ReadTool(t *testing.T) {
	req := BuildConfirmationRequest("read_file", map[string]any{
		"path": "/tmp/test.go",
	})

	require.Equal(t, "read_file", req.ToolName)
	require.Equal(t, confirmation.OpRead, req.Operation.Type)
	require.Equal(t, "/tmp/test.go", req.Operation.Target)
	require.Equal(t, confirmation.RiskLow, req.Operation.Risk)
	require.True(t, req.Operation.Reversible)
}

func TestBuildConfirmationRequest_WriteTool(t *testing.T) {
	req := BuildConfirmationRequest("write_file", map[string]any{
		"path":    "/tmp/test.go",
		"content": "package main",
	})

	require.Equal(t, confirmation.OpWrite, req.Operation.Type)
	require.Equal(t, "/tmp/test.go", req.Operation.Target)
	require.Equal(t, confirmation.RiskMedium, req.Operation.Risk)
	require.True(t, req.Operation.Reversible)
}

func TestBuildConfirmationRequest_DeleteTool(t *testing.T) {
	req := BuildConfirmationRequest("delete_file", map[string]any{
		"path": "/tmp/test.go",
	})

	require.Equal(t, confirmation.OpDelete, req.Operation.Type)
	require.Equal(t, confirmation.RiskHigh, req.Operation.Risk)
	require.False(t, req.Operation.Reversible)
}

func TestBuildConfirmationRequest_ExecTool(t *testing.T) {
	req := BuildConfirmationRequest("execute_command", map[string]any{
		"command": "go test ./...",
	})

	require.Equal(t, confirmation.OpExecute, req.Operation.Type)
	require.Equal(t, "go test ./...", req.Operation.Target)
	require.Equal(t, confirmation.RiskHigh, req.Operation.Risk)
}

func TestBuildConfirmationRequest_GitPush(t *testing.T) {
	req := BuildConfirmationRequest("git_push", map[string]any{
		"remote": "origin",
	})

	require.Equal(t, confirmation.OpGit, req.Operation.Type)
	require.Equal(t, confirmation.RiskCritical, req.Operation.Risk)
	require.False(t, req.Operation.Reversible) // git push is NOT reversible (remote state)
}

func TestBuildConfirmationRequest_NetworkTool(t *testing.T) {
	req := BuildConfirmationRequest("fetch_url", map[string]any{
		"url": "https://example.com",
	})

	require.Equal(t, confirmation.OpNetwork, req.Operation.Type)
	require.Equal(t, "https://example.com", req.Operation.Target)
}

func TestBuildConfirmationRequest_UnknownTool(t *testing.T) {
	req := BuildConfirmationRequest("custom_tool", map[string]any{})

	require.Equal(t, confirmation.OpFileSystem, req.Operation.Type)
	require.Equal(t, "custom_tool", req.Operation.Target) // falls back to tool name
	require.Equal(t, confirmation.RiskMedium, req.Operation.Risk)
}

func TestPermissionAdapter_NilEngine_FailClosed(t *testing.T) {
	adapter := NewPermissionAdapter(nil)
	allowed, err := adapter.CheckPermission(nil, "read_file", map[string]any{"path": "/tmp"})
	require.NoError(t, err)
	require.False(t, allowed, "nil engine must fail-closed (ActionAsk)")
}

func TestClassifyOperation_AllTypes(t *testing.T) {
	tests := []struct {
		tool string
		want confirmation.OperationType
	}{
		{"read_file", confirmation.OpRead},
		{"write_config", confirmation.OpWrite},
		{"create_directory", confirmation.OpWrite},
		{"save_document", confirmation.OpWrite},
		{"delete_item", confirmation.OpDelete},
		{"remove_file", confirmation.OpDelete},
		{"exec_command", confirmation.OpExecute},
		{"run_script", confirmation.OpExecute},
		{"terminal_open", confirmation.OpExecute},
		{"git_commit", confirmation.OpGit},
		{"network_fetch", confirmation.OpNetwork},
		{"http_request", confirmation.OpNetwork},
		{"unknown_tool", confirmation.OpFileSystem},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := classifyOperation(tt.tool, nil)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestClassifyRisk_Levels(t *testing.T) {
	tests := []struct {
		tool string
		want confirmation.RiskLevel
	}{
		{"read_file", confirmation.RiskLow},
		{"write_file", confirmation.RiskMedium},
		{"delete_file", confirmation.RiskHigh},
		{"exec_command", confirmation.RiskHigh},
		{"git_push", confirmation.RiskCritical},
		{"git_force_push", confirmation.RiskCritical},
		{"unknown", confirmation.RiskMedium},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := classifyRisk(tt.tool, nil)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestExtractTarget_FallbackToToolName(t *testing.T) {
	target := extractTarget("my_tool", map[string]any{})
	require.Equal(t, "my_tool", target)
}

func TestExtractTarget_FromParams(t *testing.T) {
	tests := []struct {
		key  string
		val  string
		want string
	}{
		{"path", "/tmp/test.go", "/tmp/test.go"},
		{"file", "main.go", "main.go"},
		{"url", "https://example.com", "https://example.com"},
		{"command", "go test", "go test"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := extractTarget("tool", map[string]any{tt.key: tt.val})
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsReversible(t *testing.T) {
	tests := []struct {
		tool string
		want bool
	}{
		{"read_file", true},
		{"write_file", true},
		{"delete_file", false},
		{"remove_item", false},
		{"git_push", false},
		{"git_force_push", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := isReversible(tt.tool, nil)
			require.Equal(t, tt.want, got)
		})
	}
}
