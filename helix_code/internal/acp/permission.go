// HXC-119 Phase 5: ACP permission-request mapping onto HelixCode's
// internal/tools/permissions engine (Option B — tool-execution gate).
//
// When an ACP-aware editor (Zed/JetBrains) requests a tool call via
// session/prompt, the agent MUST request permission before executing.
// This adapter maps ACP's session/request_permission onto HelixCode's
// confirmation.PolicyEngine + permissions.Engine, so existing permission
// rules (auto-allow, auto-deny, ask-user) apply consistently to ACP
// tool calls.
//
// Flow:
//  1. Agent receives tool call from ACP client
//  2. BuildConfirmationRequest maps it to a ConfirmationRequest
//  3. permissions.Engine.Decide evaluates rules → Decision
//  4. If ActionAllow → execute without asking client
//  5. If ActionDeny → reject the tool call
//  6. If ActionAsk → call conn.RequestPermission → user decides
//  7. Map user's choice back to execute-or-reject
//
// SECURITY: this is the ONLY path from ACP tool calls to HelixCode's
// permission system. No bypass, no shortcut, no "auto-approve when
// engine is unavailable" — engine unavailable = ActionAsk (fail-closed).

package acp

import (
	"context"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
)

// PermissionAdapter bridges ACP's session/request_permission to
// HelixCode's permissions.Engine. It is project-agnostic (§11.4.28):
// the engine is injected at construction, never hardcoded.
type PermissionAdapter struct {
	engine *permissions.Engine
}

// NewPermissionAdapter creates an adapter wrapping the given engine.
// If engine is nil, all requests default to ActionAsk (fail-closed).
func NewPermissionAdapter(engine *permissions.Engine) *PermissionAdapter {
	return &PermissionAdapter{engine: engine}
}

// CheckPermission evaluates whether a tool call is permitted.
// Returns (allowed bool, err error). If allowed=false and err=nil,
// the caller should request user permission via ACP. If err != nil,
// the request failed and the tool call should be rejected.
func (a *PermissionAdapter) CheckPermission(ctx context.Context, toolName string, params map[string]any) (bool, error) {
	req := BuildConfirmationRequest(toolName, params)

	if a.engine == nil {
		// No engine → fail-closed: must ask user (§11.4.6 no-guessing)
		return false, nil
	}

	decision := a.engine.Decide(req)
	switch decision.Action {
	case confirmation.ActionAllow:
		return true, nil
	case confirmation.ActionDeny:
		return false, nil
	case confirmation.ActionAsk:
		return false, nil // caller should ask user via ACP
	default:
		return false, nil
	}
}

// RequestACPPermission calls ACP's session/request_permission on the
// client, presenting the tool call and permission options. Returns
// true if user allows, false if user rejects or cancels.
func (a *PermissionAdapter) RequestACPPermission(
	ctx context.Context,
	conn *acpsdk.AgentSideConnection,
	sessionId acpsdk.SessionId,
	toolName string,
	params map[string]any,
) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("ACP connection not available")
	}

	// Build permission options (allow_once, allow_always, reject_once, reject_always)
	options := []acpsdk.PermissionOption{
		{
			Kind:     acpsdk.PermissionOptionKindAllowOnce,
			Name:     fmt.Sprintf("Allow %s once", toolName),
			OptionId: acpsdk.PermissionOptionId("allow-once"),
		},
		{
			Kind:     acpsdk.PermissionOptionKindAllowAlways,
			Name:     fmt.Sprintf("Always allow %s", toolName),
			OptionId: acpsdk.PermissionOptionId("allow-always"),
		},
		{
			Kind:     acpsdk.PermissionOptionKindRejectOnce,
			Name:     fmt.Sprintf("Reject %s once", toolName),
			OptionId: acpsdk.PermissionOptionId("reject-once"),
		},
		{
			Kind:     acpsdk.PermissionOptionKindRejectAlways,
			Name:     fmt.Sprintf("Always reject %s", toolName),
			OptionId: acpsdk.PermissionOptionId("reject-always"),
		},
	}

	// Build tool call details for the permission request (per SDK example)
	kind := classifyToolKind(toolName)
	toolCall := acpsdk.ToolCallUpdate{
		ToolCallId: acpsdk.ToolCallId(fmt.Sprintf("perm-%s-%d", toolName, len(toolName))),
		Title:      acpsdk.Ptr(fmt.Sprintf("Permission required: %s", toolName)),
		Kind:       acpsdk.Ptr(kind),
		Status:     acpsdk.Ptr(acpsdk.ToolCallStatusPending),
		RawInput:   params,
	}

	req := acpsdk.RequestPermissionRequest{
		SessionId: sessionId,
		Options:   options,
		ToolCall:  toolCall,
	}

	resp, err := conn.RequestPermission(ctx, req)
	if err != nil {
		return false, fmt.Errorf("ACP request_permission failed: %w", err)
	}

	// Map outcome to allowed/rejected
	if resp.Outcome.Cancelled != nil {
		return false, nil // user cancelled
	}
	if resp.Outcome.Selected != nil {
		switch resp.Outcome.Selected.OptionId {
		case acpsdk.PermissionOptionId("allow-once"),
			acpsdk.PermissionOptionId("allow-always"):
			return true, nil
		default:
			return false, nil
		}
	}

	return false, nil // unknown outcome → reject (fail-closed)
}

// BuildConfirmationRequest maps an ACP tool call to a
// confirmation.ConfirmationRequest for the permissions engine.
func BuildConfirmationRequest(toolName string, params map[string]any) confirmation.ConfirmationRequest {
	opType := classifyOperation(toolName, params)
	risk := classifyRisk(toolName, params)

	return confirmation.ConfirmationRequest{
		ToolName: toolName,
		Operation: confirmation.Operation{
			Type:        opType,
			Description: fmt.Sprintf("ACP tool call: %s", toolName),
			Target:      extractTarget(toolName, params),
			Risk:        risk,
			Reversible:  isReversible(toolName, params),
		},
		Parameters: params,
	}
}

// classifyOperation maps tool names to operation types.
func classifyOperation(toolName string, params map[string]any) confirmation.OperationType {
	switch {
	case strings.Contains(strings.ToLower(toolName), "read"):
		return confirmation.OpRead
	case strings.Contains(strings.ToLower(toolName), "write"),
		strings.Contains(strings.ToLower(toolName), "create"),
		strings.Contains(strings.ToLower(toolName), "save"):
		return confirmation.OpWrite
	case strings.Contains(strings.ToLower(toolName), "delete"),
		strings.Contains(strings.ToLower(toolName), "remove"):
		return confirmation.OpDelete
	case strings.Contains(strings.ToLower(toolName), "exec"),
		strings.Contains(strings.ToLower(toolName), "run"),
		strings.Contains(strings.ToLower(toolName), "terminal"):
		return confirmation.OpExecute
	case strings.Contains(strings.ToLower(toolName), "git"):
		return confirmation.OpGit
	case strings.Contains(strings.ToLower(toolName), "network"),
		strings.Contains(strings.ToLower(toolName), "fetch"),
		strings.Contains(strings.ToLower(toolName), "http"):
		return confirmation.OpNetwork
	default:
		return confirmation.OpFileSystem
	}
}

// classifyRisk assigns risk levels based on tool characteristics.
func classifyRisk(toolName string, params map[string]any) confirmation.RiskLevel {
	lower := strings.ToLower(toolName)
	switch {
	case strings.Contains(lower, "delete") || strings.Contains(lower, "remove"):
		return confirmation.RiskHigh
	case strings.Contains(lower, "exec") || strings.Contains(lower, "run"):
		return confirmation.RiskHigh
	case strings.Contains(lower, "write") || strings.Contains(lower, "create"):
		return confirmation.RiskMedium
	case strings.Contains(lower, "git") && (strings.Contains(lower, "push") || strings.Contains(lower, "force")):
		return confirmation.RiskCritical
	case strings.Contains(lower, "read"):
		return confirmation.RiskLow
	default:
		return confirmation.RiskMedium
	}
}

// extractTarget extracts the primary target from tool parameters.
func extractTarget(toolName string, params map[string]any) string {
	for _, key := range []string{"path", "file", "url", "target", "command"} {
		if v, ok := params[key]; ok {
			return fmt.Sprintf("%v", v)
		}
	}
	return toolName
}

// isReversible determines if the operation is reversible.
func isReversible(toolName string, params map[string]any) bool {
	lower := strings.ToLower(toolName)
	switch {
	case strings.Contains(lower, "delete") || strings.Contains(lower, "remove"):
		return false
	case strings.Contains(lower, "push") || strings.Contains(lower, "force"):
		return false
	case strings.Contains(lower, "read"):
		return true
	default:
		return true
	}
}

// classifyToolKind maps tool names to ACP ToolKind values for the
// permission request's ToolCallUpdate.Kind field.
func classifyToolKind(toolName string) acpsdk.ToolKind {
	lower := strings.ToLower(toolName)
	switch {
	case strings.Contains(lower, "read"):
		return acpsdk.ToolKindRead
	case strings.Contains(lower, "write") || strings.Contains(lower, "edit") ||
		strings.Contains(lower, "create") || strings.Contains(lower, "save"):
		return acpsdk.ToolKindEdit
	case strings.Contains(lower, "delete") || strings.Contains(lower, "remove"):
		return acpsdk.ToolKindDelete
	default:
		return acpsdk.ToolKindOther
	}
}
