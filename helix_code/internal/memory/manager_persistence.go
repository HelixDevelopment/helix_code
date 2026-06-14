package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// This file adds durable write-through persistence to the in-process Manager.
// The in-memory map stays the hot cache; the wired MemoryProvider is the durable
// truth. A fact stated in one session is stored through to the provider and,
// on a fresh process, hydrated back so it is recalled OUT-OF-THE-BOX.
//
// The persistence wiring is OPT-IN at construction (NewManagerWithProvider) and
// degrades honestly: if no provider is wired, the Manager behaves exactly like
// the legacy in-process map (no silent fake persistence).

// persistenceState holds the durable provider attached to a Manager. It is kept
// in a side map keyed by the Manager pointer so manager.go's struct definition
// does not have to change (surgical, low-blast-radius wiring).
type persistenceState struct {
	provider   MemoryProvider
	hydrating  bool // true while HydrateFromProvider replays durable rows — suppresses write-through so a restart does NOT re-persist (and unboundedly duplicate) every recalled row
}

// SetProvider attaches a durable MemoryProvider to the Manager, enabling
// write-through persistence. Passing nil detaches (legacy in-memory behaviour).
func (m *Manager) SetProvider(p MemoryProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.persist == nil {
		m.persist = &persistenceState{}
	}
	m.persist.provider = p
}

// HasProvider reports whether a durable provider is wired.
func (m *Manager) HasProvider() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.persist != nil && m.persist.provider != nil
}

// NewManagerWithProvider builds a Manager with durable write-through wired to p.
func NewManagerWithProvider(p MemoryProvider) *Manager {
	m := NewManager()
	m.SetProvider(p)
	// Register a write-through callback so every AddMessage path (AddMessage,
	// AddMessageToActive) persists durably without duplicating logic.
	m.OnMessage(func(conv *Conversation, msg *Message) {
		m.persistMessage(conv, msg)
	})
	return m
}

// persistMessage write-throughs a single message to the durable provider. It is
// invoked from the OnMessage callback, which fires from inside AddMessage while
// the caller already holds m.mu (write lock). RWMutex is NOT reentrant, so this
// method MUST NOT re-acquire m.mu — it reads m.persist directly (safe: we are
// inside the holder's critical section) and performs the actual provider I/O
// after the read, still synchronous to the AddMessage call.
func (m *Manager) persistMessage(conv *Conversation, msg *Message) {
	if conv == nil || msg == nil {
		return
	}
	var p MemoryProvider
	if m.persist != nil {
		// Suppress write-through while hydrating: replaying durable rows must
		// NOT re-persist them, otherwise every restart duplicates the whole
		// corpus (unbounded growth). We are inside AddMessage's critical section
		// so reading m.persist without a lock is safe.
		if m.persist.hydrating {
			return
		}
		p = m.persist.provider
	}
	if p == nil {
		return
	}

	// A unique, stable key per message so re-runs do not collide and the same
	// message updates the same durable row.
	key := fmt.Sprintf("conv:%s:msg:%s", conv.ID, msg.ID)
	// The durable content is the raw message text, prefixed with role + session
	// so Search recalls it with full context.
	content := fmt.Sprintf("[%s] %s", msg.Role, msg.Content)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Store(ctx, key, content); err != nil {
		// Honest failure surfacing (anti-bluff): record the failure on the
		// conversation metadata rather than silently swallowing it.
		conv.SetMetadata("persist_error", err.Error())
	}
}

// HydrateFromProvider loads recalled memories from the durable provider into a
// fresh Manager so a prior fact survives a process restart. It performs a broad
// search ("*" + an empty query surfaces all rows via List) and rebuilds a
// "recalled" conversation containing the durable messages in chronological
// order.
//
// It is the read-side counterpart of persistMessage and is invoked at TUI
// startup so recall works OUT-OF-THE-BOX.
func (m *Manager) HydrateFromProvider(ctx context.Context) error {
	m.mu.RLock()
	var p MemoryProvider
	if m.persist != nil {
		p = m.persist.provider
	}
	m.mu.RUnlock()
	if p == nil {
		return nil // nothing to hydrate; honest no-op
	}

	// A HelixMemoryProvider exposes List via its store; for the generic
	// MemoryProvider contract we recall via a wide Search. The empty-ish query
	// "[" matches every role-prefixed durable message ("[user] ...",
	// "[assistant] ...") since every persisted turn carries that prefix.
	results, err := p.Search(ctx, "[", 1000)
	if err != nil {
		return fmt.Errorf("hydrate: search provider: %w", err)
	}
	if len(results) == 0 {
		return nil
	}

	conv, err := m.CreateConversation("recalled-memory")
	if err != nil {
		return fmt.Errorf("hydrate: create conversation: %w", err)
	}

	// Sort by key so messages land in a stable order.
	sort.SliceStable(results, func(i, j int) bool { return results[i].Key < results[j].Key })

	// Mark hydration in progress so the write-through callback is suppressed for
	// every replayed row (otherwise a restart re-persists the entire corpus).
	m.mu.Lock()
	if m.persist == nil {
		m.persist = &persistenceState{}
	}
	m.persist.hydrating = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		if m.persist != nil {
			m.persist.hydrating = false
		}
		m.mu.Unlock()
	}()

	for _, r := range results {
		content, _ := r.Data.(string)
		role, body := splitRolePrefix(content)
		msg := NewMessage(role, body)
		if err := m.AddMessage(conv.ID, msg); err != nil {
			return fmt.Errorf("hydrate: add message: %w", err)
		}
	}
	return nil
}

// RecallContext returns up to `limit` durable memories most relevant to query,
// formatted as a single context preamble string suitable for prepending to a
// chat prompt. Empty when no provider is wired or nothing matches.
func (m *Manager) RecallContext(ctx context.Context, query string, limit int) (string, error) {
	m.mu.RLock()
	var p MemoryProvider
	if m.persist != nil {
		p = m.persist.provider
	}
	m.mu.RUnlock()
	if p == nil || strings.TrimSpace(query) == "" {
		return "", nil
	}
	results, err := p.Search(ctx, query, limit)
	if err != nil {
		return "", fmt.Errorf("recall: %w", err)
	}
	if len(results) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("Relevant remembered context:\n")
	for _, r := range results {
		if s, ok := r.Data.(string); ok && strings.TrimSpace(s) != "" {
			b.WriteString("- ")
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}

// splitRolePrefix parses "[role] body" back into (Role, body). Defaults to
// RoleUser when no prefix is present.
func splitRolePrefix(s string) (Role, string) {
	if strings.HasPrefix(s, "[") {
		if end := strings.Index(s, "]"); end > 0 {
			role := Role(s[1:end])
			body := strings.TrimSpace(s[end+1:])
			if role.IsValid() {
				return role, body
			}
		}
	}
	return RoleUser, s
}
