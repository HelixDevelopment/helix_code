package permissions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/confirmation"
)

func TestNewEngine_RegistersPolicy(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.permissions/v1
mode: default
rules:
  - pattern: "Bash(git status*)"
    action: allow
    priority: 100
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing")}

	policyEngine := confirmation.NewPolicyEngine()
	eng, err := NewEngine(context.Background(), loader, policyEngine)
	require.NoError(t, err)
	require.NotNil(t, eng)

	req := confirmation.ConfirmationRequest{
		ToolName:   "Bash",
		Parameters: map[string]interface{}{"command": "git status -sb"},
	}
	decision, err := policyEngine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, confirmation.ActionAllow, decision.Action)
}

func TestNewEngine_EvaluatesViaRuleEngine(t *testing.T) {
	tmp := t.TempDir()
	loader := &FileLoader{
		UserPath:    filepath.Join(tmp, "user.yaml"),
		ProjectPath: filepath.Join(tmp, "project.yaml"),
		Mode:        "dontAsk",
	}
	pe := confirmation.NewPolicyEngine()
	_, err := NewEngine(context.Background(), loader, pe)
	require.NoError(t, err)

	req := confirmation.ConfirmationRequest{
		ToolName:   "Read",
		Parameters: map[string]interface{}{"path": "/etc/hosts"},
	}
	decision, err := pe.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, confirmation.ActionAllow, decision.Action)
}

func TestNewEngine_DenyRuleBlocksMatch(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.permissions/v1
mode: auto
rules:
  - pattern: "Bash(rm*)"
    action: deny
    priority: 1000
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing")}
	pe := confirmation.NewPolicyEngine()
	_, err := NewEngine(context.Background(), loader, pe)
	require.NoError(t, err)

	req := confirmation.ConfirmationRequest{
		ToolName:   "Bash",
		Parameters: map[string]interface{}{"command": "rm -rf /tmp/x"},
	}
	decision, err := pe.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, confirmation.ActionDeny, decision.Action)
}
