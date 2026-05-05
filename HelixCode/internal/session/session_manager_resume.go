package session

import (
	"context"
	"path/filepath"
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
//
// Metadata-preservation contract: Append reads the existing metadata sidecar
// (if any) and mutates ONLY LastActivity, MessageCount, and IsActive (the
// session is by definition active while Append is being called). All other
// persisted fields — ProjectPath, ProjectName, BranchName, StartedAt — are
// preserved byte-exact so that project-scoped lookup via
// ListSessionMetadata(ctx, projectPath) continues to return the session after
// it has been written to. Regression test:
// TestSessionManager_Append_PreservesProjectMetadata.
//
// On first append (no existing metadata), Append synthesises a fresh record
// using ComputeProjectIdentity() for ProjectPath/ProjectName so that the
// session is locatable by project from the start.
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

	// Read existing metadata so we can preserve ProjectPath, ProjectName,
	// BranchName, and the original StartedAt across writes. GetSessionMetadata
	// will resynthesise a record from the JSONL transcript if the sidecar is
	// missing (e.g. on first append where TranscriptStore.Append already wrote
	// a defaults-only sidecar) — that's fine; we still mutate it in place
	// rather than replacing it wholesale.
	existing, err := store.GetSessionMetadata(ctx, id)
	if err != nil || existing == nil {
		// No existing metadata and no transcript-derived synthesis was
		// possible. Build a fresh record with project identity attached so
		// the session is locatable by project path on later lookups.
		meta := SessionMetadata{
			SessionID:    id,
			StartedAt:    startedAt,
			LastActivity: time.Now(),
			MessageCount: count,
			IsActive:     true,
		}
		if meta.StartedAt.IsZero() {
			meta.StartedAt = time.Now()
		}
		if projPath, identityErr := ComputeProjectIdentity(); identityErr == nil && projPath != "" {
			meta.ProjectPath = projPath
			meta.ProjectName = filepath.Base(projPath)
		}
		return store.UpdateSessionMetadata(ctx, meta)
	}

	// Mutate only the activity-tracking fields; preserve everything else.
	existing.LastActivity = time.Now()
	existing.MessageCount = count
	existing.IsActive = true
	if existing.SessionID == "" {
		existing.SessionID = id
	}
	if existing.StartedAt.IsZero() {
		if !startedAt.IsZero() {
			existing.StartedAt = startedAt
		} else {
			existing.StartedAt = time.Now()
		}
	}
	return store.UpdateSessionMetadata(ctx, *existing)
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
