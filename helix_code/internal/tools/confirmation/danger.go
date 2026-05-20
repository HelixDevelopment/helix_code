package confirmation

import (
	"context"
	"strings"
)

// DangerPattern defines a dangerous pattern
type DangerPattern struct {
	Name        string
	Description string
	Risk        RiskLevel
	Reversible  bool
	Match       func(ConfirmationRequest) bool
}

// DangerAssessment contains risk assessment
type DangerAssessment struct {
	Risk       RiskLevel
	Dangers    []string
	Reversible bool
}

// DangerDetector identifies dangerous operations
type DangerDetector struct {
	patterns []DangerPattern
}

// NewDangerDetector creates a new danger detector with default patterns
func NewDangerDetector() *DangerDetector {
	return &DangerDetector{
		patterns: defaultDangerPatterns(),
	}
}

// Detect checks if operation is dangerous
func (dd *DangerDetector) Detect(req ConfirmationRequest) *DangerAssessment {
	assessment := &DangerAssessment{
		Risk:       RiskLow,
		Dangers:    []string{},
		Reversible: true,
	}

	for _, pattern := range dd.patterns {
		if pattern.Match(req) {
			assessment.Risk = maxRisk(assessment.Risk, pattern.Risk)
			assessment.Dangers = append(assessment.Dangers, pattern.Description)
			if !pattern.Reversible {
				assessment.Reversible = false
			}
		}
	}

	return assessment
}

// AddPattern adds a custom danger pattern
func (dd *DangerDetector) AddPattern(pattern DangerPattern) {
	dd.patterns = append(dd.patterns, pattern)
}

// defaultDangerPatterns returns default danger detection patterns.
// Every Description is resolved through the CONST-046 i18n seam
// (tr) so the operator-facing Warnings section of the confirmation
// prompt adapts to the active locale. context.Background() is used
// because pattern construction is package-level with no request
// context; the active locale is read from the process-wide
// translator wired at boot via SetTranslator.
func defaultDangerPatterns() []DangerPattern {
	ctx := context.Background()
	return []DangerPattern{
		{
			Name:        "delete_operation",
			Description: tr(ctx, "internal_tools_confirmation_danger_delete_operation", nil),
			Risk:        RiskHigh,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				return req.Operation.Type == OpDelete
			},
		},
		{
			Name:        "system_files",
			Description: tr(ctx, "internal_tools_confirmation_danger_system_files", nil),
			Risk:        RiskCritical,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				systemPaths := []string{"/etc", "/sys", "/bin", "/usr", "/sbin", "/boot"}
				for _, path := range systemPaths {
					if strings.HasPrefix(req.Operation.Target, path) {
						return true
					}
				}
				return false
			},
		},
		{
			Name:        "rm_rf_command",
			Description: tr(ctx, "internal_tools_confirmation_danger_rm_rf", nil),
			Risk:        RiskCritical,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if req.ToolName == "bash" {
					if cmd, ok := req.Parameters["command"].(string); ok {
						return strings.Contains(cmd, "rm -rf") || strings.Contains(cmd, "rm -fr")
					}
				}
				return false
			},
		},
		{
			Name:        "git_force_push",
			Description: tr(ctx, "internal_tools_confirmation_danger_git_force_push", nil),
			Risk:        RiskHigh,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if req.ToolName == "git" {
					if cmd, ok := req.Parameters["command"].(string); ok {
						return strings.Contains(cmd, "push") && (strings.Contains(cmd, "--force") || strings.Contains(cmd, "-f"))
					}
				}
				return false
			},
		},
		{
			Name:        "main_branch_operation",
			Description: tr(ctx, "internal_tools_confirmation_danger_main_branch", nil),
			Risk:        RiskMedium,
			Reversible:  true,
			Match: func(req ConfirmationRequest) bool {
				if branch, ok := req.Parameters["branch"].(string); ok {
					return branch == "main" || branch == "master"
				}
				return false
			},
		},
		{
			Name:        "network_request",
			Description: tr(ctx, "internal_tools_confirmation_danger_network_request", nil),
			Risk:        RiskMedium,
			Reversible:  true,
			Match: func(req ConfirmationRequest) bool {
				return req.Operation.Type == OpNetwork
			},
		},
		{
			Name:        "sudo_command",
			Description: tr(ctx, "internal_tools_confirmation_danger_sudo", nil),
			Risk:        RiskHigh,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if req.ToolName == "bash" {
					if cmd, ok := req.Parameters["command"].(string); ok {
						return strings.HasPrefix(strings.TrimSpace(cmd), "sudo ")
					}
				}
				return false
			},
		},
		{
			Name:        "chmod_777",
			Description: tr(ctx, "internal_tools_confirmation_danger_chmod_777", nil),
			Risk:        RiskHigh,
			Reversible:  true,
			Match: func(req ConfirmationRequest) bool {
				if req.ToolName == "bash" {
					if cmd, ok := req.Parameters["command"].(string); ok {
						return strings.Contains(cmd, "chmod 777") || strings.Contains(cmd, "chmod -R 777")
					}
				}
				return false
			},
		},
		{
			Name:        "drop_table",
			Description: tr(ctx, "internal_tools_confirmation_danger_drop_table", nil),
			Risk:        RiskCritical,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if cmd, ok := req.Parameters["command"].(string); ok {
					cmdLower := strings.ToLower(cmd)
					return strings.Contains(cmdLower, "drop table") || strings.Contains(cmdLower, "drop database")
				}
				return false
			},
		},
		{
			Name:        "truncate_table",
			Description: tr(ctx, "internal_tools_confirmation_danger_truncate_table", nil),
			Risk:        RiskHigh,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if cmd, ok := req.Parameters["command"].(string); ok {
					cmdLower := strings.ToLower(cmd)
					return strings.Contains(cmdLower, "truncate table")
				}
				return false
			},
		},
		{
			Name:        "npm_publish",
			Description: tr(ctx, "internal_tools_confirmation_danger_npm_publish", nil),
			Risk:        RiskHigh,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if req.ToolName == "bash" {
					if cmd, ok := req.Parameters["command"].(string); ok {
						return strings.Contains(cmd, "npm publish")
					}
				}
				return false
			},
		},
		{
			Name:        "pip_upload",
			Description: tr(ctx, "internal_tools_confirmation_danger_pip_upload", nil),
			Risk:        RiskHigh,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if req.ToolName == "bash" {
					if cmd, ok := req.Parameters["command"].(string); ok {
						return strings.Contains(cmd, "twine upload") || strings.Contains(cmd, "python setup.py upload")
					}
				}
				return false
			},
		},
		{
			Name:        "docker_system_prune",
			Description: tr(ctx, "internal_tools_confirmation_danger_docker_prune", nil),
			Risk:        RiskMedium,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if req.ToolName == "bash" {
					if cmd, ok := req.Parameters["command"].(string); ok {
						return strings.Contains(cmd, "docker system prune")
					}
				}
				return false
			},
		},
		{
			Name:        "format_disk",
			Description: tr(ctx, "internal_tools_confirmation_danger_format_disk", nil),
			Risk:        RiskCritical,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if req.ToolName == "bash" {
					if cmd, ok := req.Parameters["command"].(string); ok {
						return strings.Contains(cmd, "mkfs") || strings.Contains(cmd, "format")
					}
				}
				return false
			},
		},
		{
			Name:        "kill_process",
			Description: tr(ctx, "internal_tools_confirmation_danger_kill_process", nil),
			Risk:        RiskMedium,
			Reversible:  false,
			Match: func(req ConfirmationRequest) bool {
				if req.ToolName == "bash" {
					if cmd, ok := req.Parameters["command"].(string); ok {
						return strings.Contains(cmd, "kill -9") || strings.Contains(cmd, "pkill")
					}
				}
				return false
			},
		},
	}
}

// maxRisk returns the higher risk level
func maxRisk(a, b RiskLevel) RiskLevel {
	if a > b {
		return a
	}
	return b
}
