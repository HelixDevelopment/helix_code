//go:build integration
// +build integration

// HelixCode/tests/integration/permissions/permissions_integration_test.go
//
// Integration tests for permissions.Engine + confirmation.PolicyEngine.
// NO mocks — constructs the real engine against a temp filesystem, issues
// real os/exec calls for the control case, and asserts decisions directly.
package permissions_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
)

// TestIntegration_DenyRuleBlocksRealOSExec proves that a deny rule for
// "Bash(rm*)" causes the PolicyEngine to return ActionDeny when evaluated
// against a real ConfirmationRequest. The marker file is NOT deleted (the test
// never invokes exec itself for the denied command — it only checks the
// decision). A separate control exec shows that the host can actually delete
// files, proving the deny is enforced by the policy, not by a broken test env.
func TestIntegration_DenyRuleBlocksRealOSExec(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "marker")
	require.NoError(t, os.WriteFile(marker, []byte("present"), 0o644))

	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.permissions/v1
mode: auto
rules:
  - pattern: "Bash(rm*)"
    action: deny
    priority: 1000
`), 0o600))

	loader := &permissions.FileLoader{
		UserPath:    userPath,
		ProjectPath: filepath.Join(tmp, "missing.yaml"),
	}
	pe := confirmation.NewPolicyEngine()
	_, err := permissions.NewEngine(context.Background(), loader, pe)
	require.NoError(t, err)

	// Evaluate: rm -rf <marker> must be denied.
	req := confirmation.ConfirmationRequest{
		ToolName:   "Bash",
		Parameters: map[string]interface{}{"command": "rm -rf " + marker},
	}
	decision, err := pe.Evaluate(req)
	require.NoError(t, err)
	require.Equal(t, confirmation.ActionDeny, decision.Action,
		"deny rule must cause PolicyEngine to return ActionDeny for 'rm -rf'")

	// Marker must still exist because we never executed the command.
	_, statErr := os.Stat(marker)
	assert.NoError(t, statErr, "marker must still exist; deny rule blocked exec")

	// Control: a real exec proves the host can delete files normally.
	allowMarker := filepath.Join(tmp, "allow-marker")
	require.NoError(t, os.WriteFile(allowMarker, []byte("present"), 0o644))
	out, execErr := exec.Command("rm", "-f", allowMarker).CombinedOutput()
	require.NoError(t, execErr, "control: real exec works: %s", out)
	_, statErr2 := os.Stat(allowMarker)
	assert.True(t, os.IsNotExist(statErr2), "control: rm actually deletes the allow-marker")
}

// TestIntegration_SmuggleViaCommandSubstitutionDenied ensures the engine
// cannot be tricked by a $() substitution that embeds a denied command inside
// an innocuous outer call. The shell splitter (mvdan.cc/sh/v3) extracts
// "rm -rf /tmp/smuggled" as an independent leaf; the aggregator then returns
// the most-restrictive (deny) over all leaves.
func TestIntegration_SmuggleViaCommandSubstitutionDenied(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.permissions/v1
mode: auto
rules:
  - pattern: "Bash(rm*)"
    action: deny
    priority: 1000
`), 0o600))

	loader := &permissions.FileLoader{
		UserPath:    userPath,
		ProjectPath: filepath.Join(tmp, "missing.yaml"),
	}
	pe := confirmation.NewPolicyEngine()
	_, err := permissions.NewEngine(context.Background(), loader, pe)
	require.NoError(t, err)

	req := confirmation.ConfirmationRequest{
		ToolName:   "Bash",
		Parameters: map[string]interface{}{"command": "echo hi $(rm -rf /tmp/smuggled)"},
	}
	decision, err := pe.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, confirmation.ActionDeny, decision.Action,
		"smuggled rm inside $() substitution must propagate to deny via leaf aggregation")
}

// TestIntegration_ReadOnlyDontAskAllowsLs confirms that the "dontAsk" preset
// auto-allows read-only commands (ls). No YAML rule file is needed — the
// preset alone is sufficient.
func TestIntegration_ReadOnlyDontAskAllowsLs(t *testing.T) {
	tmp := t.TempDir()
	loader := &permissions.FileLoader{
		UserPath:    filepath.Join(tmp, "user.yaml"),
		ProjectPath: filepath.Join(tmp, "project.yaml"),
		Mode:        "dontAsk",
	}
	pe := confirmation.NewPolicyEngine()
	_, err := permissions.NewEngine(context.Background(), loader, pe)
	require.NoError(t, err)

	req := confirmation.ConfirmationRequest{
		ToolName:   "Bash",
		Parameters: map[string]interface{}{"command": "ls -la /"},
	}
	decision, err := pe.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, confirmation.ActionAllow, decision.Action,
		"dontAsk preset must auto-allow 'ls -la /'")
}
