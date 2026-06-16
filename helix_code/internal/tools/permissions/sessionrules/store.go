// Package sessionrules provides an in-memory, session-scoped store of
// permission rules added at runtime via the `/permissions add|remove` slash
// command.
//
// Rationale (investigated, not guessed): the HelixCode CLI already treats
// `/permissions mode <preset>` as session-only, transient state held on the
// command struct (it is NOT written to disk; persistent edits go through the
// `helixcode permissions` Cobra subcommand). `/permissions add|remove` is
// documented identically ("add rule (session-only)"). This store mirrors that
// model: it holds the rules added during the live session in memory, layered
// over the file-loaded base rules, without touching the user/project YAML files.
//
// The package is deliberately decoupled from the consuming CLI: it imports only
// the permissions rule types and knows nothing about sessions, projects, or any
// HelixCode-specific context (CONST-051(B)). The owner of a session supplies its
// own key string; the store is concurrency-safe so background tool dispatch and
// the interactive command loop can share one instance.
package sessionrules

import (
	"sort"
	"sync"

	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
)

// Store holds session-scoped permission rules keyed by session id.
//
// The zero value is NOT ready for use; construct with New. A Store is safe for
// concurrent use by multiple goroutines.
type Store struct {
	mu sync.RWMutex
	// bySession maps a session key to that session's added rules, keyed by
	// pattern so a re-add of the same pattern replaces (upserts) rather than
	// duplicating — matching the FileLoader.Save upsert semantics.
	bySession map[string]map[string]permissions.Rule
}

// New constructs an empty Store ready for use.
func New() *Store {
	return &Store{bySession: make(map[string]map[string]permissions.Rule)}
}

// Add stores (or upserts) a rule for the given session. A rule with the same
// pattern already present for that session is replaced. The stored rule's
// Source is forced to ScopeCLI so list output and decisions attribute it to the
// live session, not a file.
func (s *Store) Add(session string, rule permissions.Rule) {
	rule.Source = permissions.ScopeCLI
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.bySession[session]
	if !ok {
		m = make(map[string]permissions.Rule)
		s.bySession[session] = m
	}
	m[rule.Pattern] = rule
}

// Remove deletes the rule with the given pattern from the session. It reports
// whether a rule was actually removed (false if no such pattern was present),
// so the caller can give the operator honest feedback rather than always
// claiming success.
func (s *Store) Remove(session, pattern string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.bySession[session]
	if !ok {
		return false
	}
	if _, present := m[pattern]; !present {
		return false
	}
	delete(m, pattern)
	if len(m) == 0 {
		delete(s.bySession, session)
	}
	return true
}

// Rules returns the session's stored rules sorted by descending priority then
// pattern (stable, deterministic order for list rendering and decisions). The
// returned slice is a fresh copy; mutating it does not affect the store.
func (s *Store) Rules(session string) []permissions.Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.bySession[session]
	out := make([]permissions.Rule, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].Pattern < out[j].Pattern
	})
	return out
}

// Has reports whether the session currently holds a rule for the given pattern.
func (s *Store) Has(session, pattern string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.bySession[session]
	if !ok {
		return false
	}
	_, present := m[pattern]
	return present
}

// Decide evaluates a tool call against the session's stored rules and returns
// the resulting Decision. If the session has no matching rule (or no rules at
// all) the Decision's MatchedPattern is empty and the Action is ActionAsk — the
// caller layers this over the file-loaded base engine. A malformed stored
// pattern is impossible here because Add only accepts rules whose patterns were
// already validated by the command (ParsePattern) before storage; NewRuleEngine
// fails closed on any malformed pattern regardless.
func (s *Store) Decide(session, toolName, input string) permissions.Decision {
	rules := s.Rules(session)
	if len(rules) == 0 {
		return permissions.Decision{Action: confirmation.ActionAsk}
	}
	eng, err := permissions.NewRuleEngine(rules)
	if err != nil {
		// Fail closed: a corrupt session rule set must not silently allow.
		return permissions.Decision{Action: confirmation.ActionDeny, Reason: "session rule engine error: " + err.Error()}
	}
	return eng.Evaluate(toolName, input)
}
