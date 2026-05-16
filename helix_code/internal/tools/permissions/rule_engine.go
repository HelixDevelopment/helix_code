package permissions

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"dev.helix.code/internal/tools/confirmation"
)

// ParsedPattern is a decomposed permission rule pattern.
type ParsedPattern struct {
	ToolName   string
	ArgPattern string
}

// Decision is the outcome of evaluating a request against the rule engine.
type Decision struct {
	Action         confirmation.Action
	MatchedPattern string // empty if no rule matched
	Reason         string // human-readable explanation for audit logs
}

// patternRegex parses "ToolName(arg-pattern)" with optional whitespace.
var patternRegex = regexp.MustCompile(`^([A-Za-z0-9_\*]+)\s*\(([^()]*)\)$`)

// ParsePattern decomposes "ToolName(arg)" into its parts.
// Returns an error for malformed input.
func ParsePattern(pattern string) (ParsedPattern, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ParsedPattern{}, fmt.Errorf("empty pattern")
	}
	m := patternRegex.FindStringSubmatch(pattern)
	if m == nil {
		return ParsedPattern{}, fmt.Errorf("malformed pattern %q (expected ToolName(arg-pattern))", pattern)
	}
	return ParsedPattern{ToolName: m[1], ArgPattern: m[2]}, nil
}

// RuleEngine holds parsed rules sorted by priority.
type RuleEngine struct {
	rules []parsedRule
}

type parsedRule struct {
	parsed ParsedPattern
	rule   Rule
}

// NewRuleEngine parses every rule pattern and sorts by descending priority.
// Returns an error if any pattern is malformed (fail-fast at load time).
func NewRuleEngine(rules []Rule) (*RuleEngine, error) {
	out := make([]parsedRule, 0, len(rules))
	for _, r := range rules {
		p, err := ParsePattern(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", r.Pattern, err)
		}
		out = append(out, parsedRule{parsed: p, rule: r})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].rule.Priority > out[j].rule.Priority
	})
	return &RuleEngine{rules: out}, nil
}

// Evaluate resolves a tool call to a single Decision. For Bash, the input is
// split into leaf calls and aggregated to the most-restrictive (deny > ask >
// allow). A shell parse error is treated as deny (fail-closed).
func (e *RuleEngine) Evaluate(toolName, input string) Decision {
	if toolName == "Bash" {
		leaves, err := SplitCommands(input)
		if err != nil {
			return Decision{
				Action: confirmation.ActionDeny,
				Reason: fmt.Sprintf("shell parse error: %v", err),
			}
		}
		if len(leaves) == 0 {
			leaves = []string{input}
		}
		return e.aggregate(toolName, leaves)
	}
	return e.evaluateLeaf(toolName, input)
}

func (e *RuleEngine) evaluateLeaf(toolName, leaf string) Decision {
	for _, pr := range e.rules {
		if !toolNameMatches(pr.parsed.ToolName, toolName) {
			continue
		}
		if !wildcardMatchString(pr.parsed.ArgPattern, leaf) {
			continue
		}
		return Decision{
			Action:         pr.rule.Action,
			MatchedPattern: pr.rule.Pattern,
			Reason:         fmt.Sprintf("matched rule %q (%s)", pr.rule.Pattern, pr.rule.Source),
		}
	}
	return Decision{Action: confirmation.ActionAsk, Reason: "no rule matched"}
}

func (e *RuleEngine) aggregate(toolName string, leaves []string) Decision {
	worst := confirmation.ActionAllow
	var matched string
	var reason string
	hadDeny, hadAsk := false, false
	for _, leaf := range leaves {
		// Defensive: skip empty leaves (can occur if a CallExpr's only args
		// are pure CmdSubsts; the splitter avoids this but belt-and-braces).
		if strings.TrimSpace(leaf) == "" {
			continue
		}
		d := e.evaluateLeaf(toolName, leaf)
		switch d.Action {
		case confirmation.ActionDeny:
			if !hadDeny {
				matched = d.MatchedPattern
				reason = d.Reason
			}
			hadDeny = true
		case confirmation.ActionAsk:
			if !hadDeny && !hadAsk {
				matched = d.MatchedPattern
				reason = d.Reason
			}
			hadAsk = true
		case confirmation.ActionAllow:
			if !hadDeny && !hadAsk && matched == "" {
				matched = d.MatchedPattern
				reason = d.Reason
			}
		}
	}
	switch {
	case hadDeny:
		worst = confirmation.ActionDeny
	case hadAsk:
		worst = confirmation.ActionAsk
	}
	return Decision{Action: worst, MatchedPattern: matched, Reason: reason}
}

// toolNameMatches treats "*" as a wildcard tool-name match.
func toolNameMatches(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	return pattern == name
}

// wildcardMatchString matches the arg-pattern against an input string.
// Empty arg-pattern (Tool()) matches only an empty input.
// "*" matches everything.
func wildcardMatchString(pattern, input string) bool {
	if pattern == "" {
		return input == ""
	}
	return globMatchPermissions(pattern, input)
}

// globMatchPermissions is a glob matcher with *, ?, and [abc] support.
// Duplicated from confirmation.globMatch to keep the package boundary clean.
func globMatchPermissions(pattern, s string) bool {
	pi, si := 0, 0
	starPi, starSi := -1, 0
	for si < len(s) {
		switch {
		case pi < len(pattern) && pattern[pi] == '\\' && pi+1 < len(pattern) && pattern[pi+1] == s[si]:
			pi += 2
			si++
		case pi < len(pattern) && pattern[pi] == '?':
			pi++
			si++
		case pi < len(pattern) && pattern[pi] == '[':
			closeIdx := pi + 1
			for closeIdx < len(pattern) && pattern[closeIdx] != ']' {
				closeIdx++
			}
			if closeIdx >= len(pattern) {
				return false
			}
			class := pattern[pi+1 : closeIdx]
			matched := false
			for j := 0; j < len(class); j++ {
				if class[j] == s[si] {
					matched = true
					break
				}
			}
			if matched {
				pi = closeIdx + 1
				si++
			} else if starPi != -1 {
				pi = starPi + 1
				starSi++
				si = starSi
			} else {
				return false
			}
		case pi < len(pattern) && pattern[pi] == '*':
			starPi = pi
			starSi = si
			pi++
		case pi < len(pattern) && pattern[pi] == s[si]:
			pi++
			si++
		case starPi != -1:
			pi = starPi + 1
			starSi++
			si = starSi
		default:
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}
