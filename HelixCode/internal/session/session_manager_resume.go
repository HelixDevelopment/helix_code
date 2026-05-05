package session

import (
	"context"
	"sync"
	"time"
)

// SessionManager is a lightweight session state holder for F11 transcript
// persistence and resume. It is intentionally separate from the heavier
// Manager type (which manages session lifecycle via focus chains and hooks)
// so that the transcript-persistence surface can be used without pulling in
// database, Redis, or other production infrastructure.
//
// Production callers create one SessionManager per active session and wire a
// SessionStore via SetStore. The /sessions command (F11-T06) and the
// --resume / --continue flags (F11-T07) use this type to persist and reload
// conversation transcripts.
type SessionManager struct {
	mu             sync.Mutex
	store          SessionStore
	currentID      string
	loadedMessages []Message
	startedAt      time.Time
}

// NewSessionManager constructs a SessionManager without a wired store.
// Call SetStore to enable persistence.
func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

// NewSessionManagerForTestF11 constructs a zeroed SessionManager suitable for
// F11 unit tests. It is functionally identical to NewSessionManager; the
// distinct name signals test-only usage and prevents accidental reliance on
// any test-only reset semantics in production code.
func NewSessionManagerForTestF11() *SessionManager {
	return &SessionManager{}
}

// SetStore wires a SessionStore for transcript persistence. Pass nil to
// disable persistence (Append becomes a no-op).
func (m *SessionManager) SetStore(store SessionStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = store
}

// setCurrentID records the active session ID. This is a test helper used by
// unit tests to establish an active session without going through a full
// session-creation flow.
func (m *SessionManager) setCurrentID(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentID = id
	if m.startedAt.IsZero() {
		m.startedAt = time.Now()
	}
}

// CurrentID returns the active session ID, or "" if no session is active.
func (m *SessionManager) CurrentID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentID
}

// Append persists msg to the wired store and updates session metadata.
// The message is also appended to the in-memory loadedMessages slice so that
// LoadedMessageCountForTestF11 (and any future in-process inspection) reflects
// the current state without requiring an additional ReadTranscript call.
//
// If no store is wired, or if the current session ID is empty, Append is a
// silent no-op (returns nil).
func (m *SessionManager) Append(ctx context.Context, msg Message) error {
	m.mu.Lock()
	store := m.store
	id := m.currentID
	startedAt := m.startedAt
	m.loadedMessages = append(m.loadedMessages, msg)
	count := len(m.loadedMessages)
	m.mu.Unlock()

	if store == nil || id == "" {
		return nil
	}

	if err := store.Append(ctx, id, msg); err != nil {
		return err
	}

	meta := SessionMetadata{
		SessionID:    id,
		StartedAt:    startedAt,
		LastActivity: time.Now(),
		MessageCount: count,
		IsActive:     true,
	}
	return store.UpdateSessionMetadata(ctx, meta)
}

// Resume loads the stored metadata and transcript for sessionID into the
// manager's in-memory state and sets the current session ID to sessionID.
//
// After Resume returns successfully, CurrentID() == sessionID and
// LoadedMessageCountForTestF11() reflects the number of messages read from
// the store.
//
// If no store is wired, Resume is a no-op (updates only currentID).
func (m *SessionManager) Resume(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	store := m.store
	m.mu.Unlock()

	if store == nil {
		m.mu.Lock()
		m.currentID = sessionID
		m.mu.Unlock()
		return nil
	}

	meta, err := store.GetSessionMetadata(ctx, sessionID)
	if err != nil {
		return err
	}

	msgs, err := store.ReadTranscript(ctx, sessionID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.currentID = sessionID
	m.loadedMessages = msgs
	if meta != nil && !meta.StartedAt.IsZero() {
		m.startedAt = meta.StartedAt
	}
	m.mu.Unlock()

	return nil
}

// LoadedMessageCountForTestF11 returns the number of messages currently held
// in the in-memory loadedMessages slice. This is a test-helper accessor; it
// reflects both messages Appended in this session and messages loaded via
// Resume.
func (m *SessionManager) LoadedMessageCountForTestF11() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.loadedMessages)
}
