package permissions

import (
	"context"
	"fmt"

	"dev.helix.code/internal/tools/confirmation"
)

// Engine is the public facade. Construct with NewEngine; it registers a
// confirmation.Policy with the supplied PolicyEngine and is ready to use.
type Engine struct {
	ruleEngine   *RuleEngine
	policyEngine *confirmation.PolicyEngine
	loader       *FileLoader
	ruleSet      *RuleSet
}

// PolicyName is the registered policy name in the confirmation.PolicyEngine.
const PolicyName = "permissions/rule-engine"

// NewEngine loads rules via the loader, builds a RuleEngine, and registers a
// confirmation.Policy as the default policy in the supplied PolicyEngine.
// The policy is installed as the default (applies to every tool name) using
// SetDefaultPolicy.
func NewEngine(ctx context.Context, loader *FileLoader, pe *confirmation.PolicyEngine) (*Engine, error) {
	rs, err := loader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading permission rules: %w", err)
	}
	re, err := NewRuleEngine(rs.Rules)
	if err != nil {
		return nil, fmt.Errorf("building rule engine: %w", err)
	}
	eng := &Engine{
		ruleEngine:   re,
		policyEngine: pe,
		loader:       loader,
		ruleSet:      rs,
	}
	policy := eng.buildPolicy()
	if err := pe.SetDefaultPolicy(policy); err != nil {
		return nil, fmt.Errorf("registering permissions policy: %w", err)
	}
	return eng, nil
}

// RuleSet returns the loaded RuleSet (for inspection by `permissions list`).
func (e *Engine) RuleSet() *RuleSet { return e.ruleSet }

// Decide is the public single-shot decision API used by the dispatch rule's
// custom condition; it is also what the slash command and CLI subcommands call.
func (e *Engine) Decide(req confirmation.ConfirmationRequest) Decision {
	primary := primaryParam(req)
	return e.ruleEngine.Evaluate(req.ToolName, primary)
}

func primaryParam(req confirmation.ConfirmationRequest) string {
	if cmd, ok := req.Parameters["command"].(string); ok {
		return cmd
	}
	if p, ok := req.Parameters["path"].(string); ok {
		return p
	}
	if p, ok := req.Parameters["file_path"].(string); ok {
		return p
	}
	return req.Operation.Target
}

func (e *Engine) buildPolicy() *confirmation.Policy {
	deny := func(req confirmation.ConfirmationRequest) bool {
		return e.Decide(req).Action == confirmation.ActionDeny
	}
	allow := func(req confirmation.ConfirmationRequest) bool {
		return e.Decide(req).Action == confirmation.ActionAllow
	}
	ask := func(req confirmation.ConfirmationRequest) bool {
		return e.Decide(req).Action == confirmation.ActionAsk
	}
	return &confirmation.Policy{
		Name:               PolicyName,
		Description:        "claude-code-style permission rules (mode " + e.ruleSet.Mode + ")",
		Enabled:            true,
		DefaultAction:      confirmation.ActionAsk,
		BatchDefaultAction: confirmation.ActionDeny,
		Rules: []confirmation.Rule{
			{
				Name:      PolicyName + "/deny",
				Priority:  1_000_000_002,
				Condition: confirmation.Condition{Custom: deny},
				Action:    confirmation.ActionDeny,
			},
			{
				Name:      PolicyName + "/allow",
				Priority:  1_000_000_001,
				Condition: confirmation.Condition{Custom: allow},
				Action:    confirmation.ActionAllow,
			},
			{
				Name:      PolicyName + "/ask",
				Priority:  1_000_000_000,
				Condition: confirmation.Condition{Custom: ask},
				Action:    confirmation.ActionAsk,
			},
		},
	}
}
