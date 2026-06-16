package permissions

import (
	"context"
	"fmt"

	"dev.helix.code/internal/tools/confirmation"
)

// SessionDecider evaluates a tool call against session-scoped rules added at
// runtime (e.g. via `/permissions add|remove`). It returns a Decision whose
// Action is ActionAsk (and MatchedPattern empty) when the session has no
// applicable rule, so the Engine falls through to its file-loaded base rules.
//
// This is a dependency-inversion seam: the Engine consults session rules
// through this function type rather than importing a concrete session-store
// package. The wiring layer (which may import both the store and this package)
// supplies the implementation — keeping this package free of an import cycle
// with the sessionrules store (which imports this package for Rule types).
type SessionDecider func(session, toolName, input string) Decision

// Engine is the public facade. Construct with NewEngine; it registers a
// confirmation.Policy with the supplied PolicyEngine and is ready to use.
type Engine struct {
	ruleEngine   *RuleEngine
	policyEngine *confirmation.PolicyEngine
	loader       *FileLoader
	ruleSet      *RuleSet
	// sessionDecide, when non-nil, is consulted FIRST at decision time so that
	// session-added rules (via `/permissions add|remove`) take effect on the
	// live gate immediately, with no restart. A nil sessionDecide is a no-op:
	// the Engine behaves exactly as before (file-loaded rules only).
	sessionDecide SessionDecider
}

// EngineOption configures an Engine at construction.
type EngineOption func(*Engine)

// WithSessionDecider wires a session-scoped rule decider that the Engine
// consults FIRST at decision time. The same decider MUST be backed by the same
// store the `/permissions add|remove` writer mutates, so that a rule added in
// the session is honoured by the live gate immediately. Passing nil leaves the
// Engine consulting only its file-loaded base rules.
func WithSessionDecider(d SessionDecider) EngineOption {
	return func(e *Engine) { e.sessionDecide = d }
}

// SetSessionDecider wires (or rewires) the session-scoped rule decider after
// construction. Used by hosts that build the Engine before the shared store is
// available. Passing nil clears the overlay.
func (e *Engine) SetSessionDecider(d SessionDecider) { e.sessionDecide = d }

// PolicyName is the registered policy name in the confirmation.PolicyEngine.
const PolicyName = "permissions/rule-engine"

// NewEngine loads rules via the loader, builds a RuleEngine, and registers a
// confirmation.Policy as the default policy in the supplied PolicyEngine.
// The policy is installed as the default (applies to every tool name) using
// SetDefaultPolicy.
func NewEngine(ctx context.Context, loader *FileLoader, pe *confirmation.PolicyEngine, opts ...EngineOption) (*Engine, error) {
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
	for _, opt := range opts {
		opt(eng)
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
//
// Session-added rules (via `/permissions add|remove`) are consulted FIRST when a
// session decider is wired: a session rule that matches (deny or allow) wins over
// the file-loaded base rules, so an add/remove takes effect on the live gate
// immediately. A session decider that does not match (ActionAsk, no matched
// pattern) falls through to the file-loaded base rules. Fail-closed is preserved:
// the session decider itself returns deny on a corrupt session rule set.
func (e *Engine) Decide(req confirmation.ConfirmationRequest) Decision {
	primary := primaryParam(req)
	if e.sessionDecide != nil {
		if d := e.sessionDecide(req.Context.SessionID, req.ToolName, primary); d.MatchedPattern != "" || d.Action == confirmation.ActionDeny {
			return d
		}
	}
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
