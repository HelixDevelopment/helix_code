package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"dev.helix.code/internal/focus"
	"dev.helix.code/internal/hooks"
	"dev.helix.code/internal/llm/compression"
)

// ErrSessionNotFound is returned when a session lookup by id fails.
// Handlers MUST errors.Is-check this sentinel and return 404, not 500
// (CONST-035: 500 lies about the nature of a client-side missing-
// resource error).
var ErrSessionNotFound = errors.New("session not found")

// Manager manages development sessions
type Manager struct {
	sessions        map[string]*Session         // All sessions by ID
	activeSession   *Session                    // Currently active session
	focusManager    *focus.Manager              // Focus chain manager
	hooksManager    *hooks.Manager              // Hooks manager
	mu              sync.RWMutex                // Thread-safety
	onCreate        []SessionCallback           // Callbacks on session creation
	onStart         []SessionCallback           // Callbacks on session start
	onPause         []SessionCallback           // Callbacks on session pause
	onResume        []SessionCallback           // Callbacks on session resume
	onComplete      []SessionCallback           // Callbacks on session completion
	onDelete        []SessionCallback           // Callbacks on session deletion
	onSwitch        []SwitchCallback            // Callbacks on session switch
	maxHistory      int                         // Maximum sessions to keep
	thrashingGuard  *compression.ThrashingGuard // Optional auto-compaction thrashing tracker; nil = no-op
	currentWorktree string                      // P1-F04 — active worktree path; "" = main worktree
}

// SessionCallback is called for session lifecycle events
type SessionCallback func(*Session)

// SwitchCallback is called when switching sessions
type SwitchCallback func(from, to *Session)

// NewManager creates a new session manager
func NewManager() *Manager {
	return &Manager{
		sessions:     make(map[string]*Session),
		focusManager: focus.NewManager(),
		hooksManager: hooks.NewManager(),
		onCreate:     make([]SessionCallback, 0),
		onStart:      make([]SessionCallback, 0),
		onPause:      make([]SessionCallback, 0),
		onResume:     make([]SessionCallback, 0),
		onComplete:   make([]SessionCallback, 0),
		onDelete:     make([]SessionCallback, 0),
		onSwitch:     make([]SwitchCallback, 0),
		maxHistory:   100,
	}
}

// NewManagerWithIntegrations creates manager with existing focus and hooks managers
func NewManagerWithIntegrations(focusMgr *focus.Manager, hooksMgr *hooks.Manager) *Manager {
	m := NewManager()
	m.focusManager = focusMgr
	m.hooksManager = hooksMgr
	return m
}

// Create creates a new session. User-facing validation literals
// resolved through tr() (CONST-046 round-178 §11.4).
func (m *Manager) Create(projectID, name, description string, mode Mode) (*Session, error) {
	ctx := context.Background()
	if projectID == "" {
		return nil, errors.New(tr(ctx, "internal_session_project_id_empty", nil))
	}

	if name == "" {
		return nil, errors.New(tr(ctx, "internal_session_name_empty", nil))
	}

	if !mode.IsValid() {
		return nil, errors.New(tr(ctx, "internal_session_create_invalid_mode", map[string]any{
			"Mode": fmt.Sprintf("%s", mode),
		}))
	}

	m.mu.Lock()

	// Create session
	session := &Session{
		ID:          generateSessionID(),
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		Mode:        mode,
		Status:      StatusPaused,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Context:     make(map[string]interface{}),
		Metadata:    make(map[string]string),
		Tags:        make([]string, 0),
	}

	// Create dedicated focus chain for session
	chain, err := m.focusManager.CreateChain(fmt.Sprintf("session-%s", session.ID), false)
	if err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("failed to create focus chain: %w", err)
	}
	session.FocusChainID = chain.ID

	// Store session
	m.sessions[session.ID] = session

	// Snapshot callbacks under the lock, then invoke them OUTSIDE the lock
	// (see snapshotCallbacks for the deadlock rationale).
	onCreate := snapshotCallbacks(m.onCreate)
	m.mu.Unlock()

	for _, callback := range onCreate {
		callback(session)
	}

	// Emit hook outside m.mu: hook handlers may re-enter the session Manager.
	m.emitHook(hooks.HookTypeCustom, "session_created", session)

	return session, nil
}

// Start starts a session and makes it active
func (m *Manager) Start(sessionID string) error {
	m.mu.Lock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	if session.Status == StatusCompleted {
		m.mu.Unlock()
		return errors.New(tr(context.Background(), "internal_session_start_completed", nil))
	}

	if session.Status == StatusFailed {
		m.mu.Unlock()
		return errors.New(tr(context.Background(), "internal_session_start_failed", nil))
	}

	oldActive := m.activeSession
	session.Status = StatusActive
	session.UpdatedAt = time.Now()
	session.StartedAt = time.Now()
	m.activeSession = session

	// Set focus chain as active
	if session.FocusChainID != "" {
		m.focusManager.SetActiveChain(session.FocusChainID)
	}

	// Snapshot callbacks under the lock, then invoke them OUTSIDE the lock.
	onStart := snapshotCallbacks(m.onStart)
	switched := oldActive != session
	var onSwitch []SwitchCallback
	if switched {
		onSwitch = snapshotSwitchCallbacks(m.onSwitch)
	}
	m.mu.Unlock()

	// Trigger callbacks
	for _, callback := range onStart {
		callback(session)
	}

	// Trigger switch callbacks
	if switched {
		for _, callback := range onSwitch {
			callback(oldActive, session)
		}
	}

	// Emit hook outside m.mu: hook handlers may re-enter the session Manager.
	m.emitHook(hooks.HookTypeCustom, "session_started", session)

	return nil
}

// Pause pauses the active session
func (m *Manager) Pause(sessionID string) error {
	m.mu.Lock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	if session.Status != StatusActive {
		m.mu.Unlock()
		return fmt.Errorf("session is not active")
	}

	session.Status = StatusPaused
	session.UpdatedAt = time.Now()

	// Update duration
	if !session.StartedAt.IsZero() {
		session.Duration += time.Since(session.StartedAt)
		session.StartedAt = time.Time{} // Reset
	}

	if m.activeSession == session {
		m.activeSession = nil
	}

	// Snapshot callbacks under the lock, then invoke them OUTSIDE the lock.
	onPause := snapshotCallbacks(m.onPause)
	m.mu.Unlock()

	// Trigger callbacks
	for _, callback := range onPause {
		callback(session)
	}

	// Emit hook outside m.mu: hook handlers may re-enter the session Manager.
	m.emitHook(hooks.HookTypeCustom, "session_paused", session)

	return nil
}

// Resume resumes a paused session
func (m *Manager) Resume(sessionID string) error {
	m.mu.Lock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	if session.Status != StatusPaused {
		m.mu.Unlock()
		return fmt.Errorf("session is not paused")
	}

	oldActive := m.activeSession
	session.Status = StatusActive
	session.UpdatedAt = time.Now()
	session.StartedAt = time.Now()
	m.activeSession = session

	// Set focus chain as active
	if session.FocusChainID != "" {
		m.focusManager.SetActiveChain(session.FocusChainID)
	}

	// Snapshot callbacks under the lock, then invoke them OUTSIDE the lock.
	onResume := snapshotCallbacks(m.onResume)
	switched := oldActive != session
	var onSwitch []SwitchCallback
	if switched {
		onSwitch = snapshotSwitchCallbacks(m.onSwitch)
	}
	m.mu.Unlock()

	// Trigger callbacks
	for _, callback := range onResume {
		callback(session)
	}

	// Trigger switch callbacks
	if switched {
		for _, callback := range onSwitch {
			callback(oldActive, session)
		}
	}

	// Emit hook outside m.mu: hook handlers may re-enter the session Manager.
	m.emitHook(hooks.HookTypeCustom, "session_resumed", session)

	return nil
}

// Complete marks a session as completed
func (m *Manager) Complete(sessionID string) error {
	m.mu.Lock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	if session.Status == StatusCompleted {
		m.mu.Unlock()
		return fmt.Errorf("session already completed")
	}

	session.Status = StatusCompleted
	session.UpdatedAt = time.Now()
	session.CompletedAt = time.Now()

	// Update duration for active sessions
	if !session.StartedAt.IsZero() {
		session.Duration += time.Since(session.StartedAt)
		session.StartedAt = time.Time{} // Reset
	}

	if m.activeSession == session {
		m.activeSession = nil
	}

	// Snapshot callbacks under the lock, then invoke them OUTSIDE the lock.
	onComplete := snapshotCallbacks(m.onComplete)
	m.mu.Unlock()

	// Trigger callbacks
	for _, callback := range onComplete {
		callback(session)
	}

	// Emit hook outside m.mu: hook handlers may re-enter the session Manager.
	m.emitHook(hooks.HookTypeCustom, "session_completed", session)

	return nil
}

// Fail marks a session as failed
func (m *Manager) Fail(sessionID string, reason string) error {
	m.mu.Lock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	session.Status = StatusFailed
	session.UpdatedAt = time.Now()
	session.SetMetadata("failure_reason", reason)

	// Update duration for active sessions
	if !session.StartedAt.IsZero() {
		session.Duration += time.Since(session.StartedAt)
		session.StartedAt = time.Time{} // Reset
	}

	if m.activeSession == session {
		m.activeSession = nil
	}

	m.mu.Unlock()

	// Emit hook outside m.mu: hook handlers may re-enter the session Manager.
	m.emitHook(hooks.HookTypeCustom, "session_failed", session)

	return nil
}

// Delete deletes a session
func (m *Manager) Delete(sessionID string) error {
	m.mu.Lock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	// Don't delete active session
	if session.Status == StatusActive {
		m.mu.Unlock()
		return fmt.Errorf("cannot delete active session")
	}

	// Delete focus chain
	if session.FocusChainID != "" {
		m.focusManager.DeleteChain(session.FocusChainID)
	}

	// Remove session
	delete(m.sessions, sessionID)

	// Snapshot callbacks under the lock, then invoke them OUTSIDE the lock.
	onDelete := snapshotCallbacks(m.onDelete)
	m.mu.Unlock()

	// Trigger callbacks
	for _, callback := range onDelete {
		callback(session)
	}

	// Emit hook outside m.mu: hook handlers may re-enter the session Manager.
	m.emitHook(hooks.HookTypeCustom, "session_deleted", session)

	return nil
}

// Get retrieves a session by ID
func (m *Manager) Get(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// GetActive returns the currently active session
func (m *Manager) GetActive() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.activeSession
}

// GetAll returns all sessions
func (m *Manager) GetAll() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// GetByProject returns all sessions for a project
func (m *Manager) GetByProject(projectID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0)
	for _, session := range m.sessions {
		if session.ProjectID == projectID {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// GetByMode returns all sessions with a specific mode
func (m *Manager) GetByMode(mode Mode) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0)
	for _, session := range m.sessions {
		if session.Mode == mode {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// GetByStatus returns all sessions with a specific status
func (m *Manager) GetByStatus(status Status) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0)
	for _, session := range m.sessions {
		if session.Status == status {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// GetByTag returns all sessions with a specific tag
func (m *Manager) GetByTag(tag string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0)
	for _, session := range m.sessions {
		if session.HasTag(tag) {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// GetRecent returns the N most recently updated sessions
func (m *Manager) GetRecent(n int) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get all sessions
	all := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		all = append(all, session)
	}

	// Sort by UpdatedAt (descending)
	for i := 0; i < len(all)-1; i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].UpdatedAt.After(all[i].UpdatedAt) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	// Return top N
	if n <= 0 || n > len(all) {
		n = len(all)
	}

	return all[:n]
}

// FindByName finds sessions by name (case-insensitive substring match)
func (m *Manager) FindByName(nameSubstring string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0)
	lowerSearch := toLower(nameSubstring)

	for _, session := range m.sessions {
		if contains(toLower(session.Name), lowerSearch) {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// Count returns the total number of sessions
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.sessions)
}

// CountByStatus returns count of sessions by status
func (m *Manager) CountByStatus(status Status) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, session := range m.sessions {
		if session.Status == status {
			count++
		}
	}

	return count
}

// Clear removes all sessions
func (m *Manager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Don't clear if there's an active session
	if m.activeSession != nil {
		return fmt.Errorf("cannot clear while session is active")
	}

	// Delete all focus chains
	for _, session := range m.sessions {
		if session.FocusChainID != "" {
			m.focusManager.DeleteChain(session.FocusChainID)
		}
	}

	m.sessions = make(map[string]*Session)

	return nil
}

// GetFocusManager returns the focus manager
func (m *Manager) GetFocusManager() *focus.Manager {
	return m.focusManager
}

// GetHooksManager returns the hooks manager
func (m *Manager) GetHooksManager() *hooks.Manager {
	return m.hooksManager
}

// GetStatistics returns session statistics
func (m *Manager) GetStatistics() *Statistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Statistics{
		Total:    len(m.sessions),
		ByStatus: make(map[Status]int),
		ByMode:   make(map[Mode]int),
	}

	var totalDuration time.Duration
	for _, session := range m.sessions {
		stats.ByStatus[session.Status]++
		stats.ByMode[session.Mode]++

		// Calculate total duration
		duration := session.Duration
		if !session.StartedAt.IsZero() {
			// Session is currently active
			duration += time.Since(session.StartedAt)
		}
		totalDuration += duration
	}

	if stats.Total > 0 {
		stats.AverageDuration = totalDuration / time.Duration(stats.Total)
	}

	return stats
}

// snapshotCallbacks returns an independent copy of a SessionCallback slice.
//
// Lifecycle methods (Create/Start/Pause/Resume/Complete/Delete) take this
// copy while holding m.mu, release the lock, then range the copy with NO lock
// held. Invoking callbacks under m.mu would deadlock any user callback that
// re-enters the same *Manager (m.Get, m.GetAll, m.OnCreate, …) because
// sync.RWMutex is not reentrant. The copy is independent of the live slice so
// a concurrent On* append (which grows via copy-on-grow under the lock) cannot
// race the lock-free range. Returns nil for an empty/nil input so the range is
// a no-op.
func snapshotCallbacks(cbs []SessionCallback) []SessionCallback {
	if len(cbs) == 0 {
		return nil
	}
	out := make([]SessionCallback, len(cbs))
	copy(out, cbs)
	return out
}

// snapshotSwitchCallbacks is snapshotCallbacks for the SwitchCallback slice
// (onSwitch). Same deadlock + race rationale.
func snapshotSwitchCallbacks(cbs []SwitchCallback) []SwitchCallback {
	if len(cbs) == 0 {
		return nil
	}
	out := make([]SwitchCallback, len(cbs))
	copy(out, cbs)
	return out
}

// OnCreate registers a callback for session creation.
//
// Callback-registration mutates the same slice the lifecycle methods snapshot
// while holding m.mu (e.g. Create's onCreate snapshot). Registering without
// the lock is a data race against a concurrent lifecycle call, so every On*
// registrar takes the write lock. (Fixed: previously these appended without
// m.mu, a race reproduced by TestManagerCallbackRegistration_Race.)
//
// The lifecycle methods snapshot the callback slice under the lock and invoke
// the callbacks OUTSIDE the lock (see snapshotCallbacks) so a callback that
// re-enters the Manager cannot deadlock
// (TestManagerCallbackReentry_NoDeadlock).
func (m *Manager) OnCreate(callback SessionCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onCreate = append(m.onCreate, callback)
}

// OnStart registers a callback for session start
func (m *Manager) OnStart(callback SessionCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStart = append(m.onStart, callback)
}

// OnPause registers a callback for session pause
func (m *Manager) OnPause(callback SessionCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onPause = append(m.onPause, callback)
}

// OnResume registers a callback for session resume
func (m *Manager) OnResume(callback SessionCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onResume = append(m.onResume, callback)
}

// OnComplete registers a callback for session completion
func (m *Manager) OnComplete(callback SessionCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onComplete = append(m.onComplete, callback)
}

// OnDelete registers a callback for session deletion
func (m *Manager) OnDelete(callback SessionCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onDelete = append(m.onDelete, callback)
}

// OnSwitch registers a callback for session switching
func (m *Manager) OnSwitch(callback SwitchCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSwitch = append(m.onSwitch, callback)
}

// SetMaxHistory sets the maximum number of sessions to keep
func (m *Manager) SetMaxHistory(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maxHistory = max
}

// SetThrashingGuard wires the auto-compaction thrashing guard into the session
// manager. Pass nil to disable. When set, every call to NoteUserMessage will
// reset the guard's consecutive-compaction counter.
func (m *Manager) SetThrashingGuard(g *compression.ThrashingGuard) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.thrashingGuard = g
}

// NoteUserMessage notifies the session manager that a user message has been
// appended to the given session. If a ThrashingGuard is wired, its consecutive-
// compaction counter is reset to zero so that the auto-compactor does not
// incorrectly detect thrashing after normal user interaction.
//
// Returns an error only if the session does not exist. A nil ThrashingGuard is
// treated as a no-op so callers do not need to guard against the unset case.
func (m *Manager) NoteUserMessage(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[sessionID]; !exists {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	if m.thrashingGuard != nil {
		m.thrashingGuard.NoteUserMessage()
	}

	return nil
}

// TrimHistory removes old completed sessions beyond maxHistory
func (m *Manager) TrimHistory() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get completed sessions
	completed := make([]*Session, 0)
	for _, session := range m.sessions {
		if session.Status == StatusCompleted || session.Status == StatusFailed {
			completed = append(completed, session)
		}
	}

	// Sort by completion time (oldest first)
	for i := 0; i < len(completed)-1; i++ {
		for j := i + 1; j < len(completed); j++ {
			if completed[i].CompletedAt.After(completed[j].CompletedAt) {
				completed[i], completed[j] = completed[j], completed[i]
			}
		}
	}

	// Remove oldest if exceeds maxHistory
	removed := 0
	if len(completed) > m.maxHistory {
		toRemove := len(completed) - m.maxHistory
		for i := 0; i < toRemove; i++ {
			session := completed[i]
			// Delete focus chain
			if session.FocusChainID != "" {
				m.focusManager.DeleteChain(session.FocusChainID)
			}
			delete(m.sessions, session.ID)
			removed++
		}
	}

	return removed
}

// GetCurrentWorktree returns the absolute path of the active worktree, or
// "" if the session is in the main worktree.
func (m *Manager) GetCurrentWorktree() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentWorktree
}

// SetCurrentWorktree records the active worktree path. Pass "" to indicate
// the session has exited a worktree (returned to main).
func (m *Manager) SetCurrentWorktree(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentWorktree = path
}

// emitHook emits a hook event
func (m *Manager) emitHook(hookType hooks.HookType, eventName string, session *Session) {
	event := hooks.NewEventWithContext(context.Background(), hookType)
	event.SetData("event", eventName)
	event.SetData("session_id", session.ID)
	event.SetData("session_name", session.Name)
	event.SetData("project_id", session.ProjectID)
	event.SetData("mode", string(session.Mode))
	event.SetData("status", string(session.Status))
	event.Source = "session-manager"

	m.hooksManager.TriggerEvent(event)
}

// Statistics contains session statistics
type Statistics struct {
	Total           int            // Total sessions
	ByStatus        map[Status]int // Count by status
	ByMode          map[Mode]int   // Count by mode
	AverageDuration time.Duration  // Average session duration
}

// String returns a string representation of the statistics
func (s *Statistics) String() string {
	return fmt.Sprintf("Sessions: %d total, Avg Duration: %v", s.Total, s.AverageDuration)
}

// Export exports session metadata (without focus chain)
func (m *Manager) Export(sessionID string) (*SessionSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Export focus chain
	var focusChain *focus.ChainSnapshot
	if session.FocusChainID != "" {
		chain, err := m.focusManager.ExportChain(session.FocusChainID)
		if err == nil {
			focusChain = chain
		}
	}

	return &SessionSnapshot{
		Session:    session.Clone(),
		FocusChain: focusChain,
	}, nil
}

// Import imports a session from snapshot
func (m *Manager) Import(snapshot *SessionSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate ID
	if _, exists := m.sessions[snapshot.Session.ID]; exists {
		return fmt.Errorf("session with ID '%s' already exists", snapshot.Session.ID)
	}

	// Import focus chain if present
	if snapshot.FocusChain != nil {
		err := m.focusManager.ImportChain(snapshot.FocusChain, false)
		if err != nil {
			return fmt.Errorf("failed to import focus chain: %w", err)
		}
		// Use the chain ID from snapshot
		snapshot.Session.FocusChainID = snapshot.FocusChain.Chain.ID
	}

	// Store session
	m.sessions[snapshot.Session.ID] = snapshot.Session

	return nil
}

// SessionSnapshot represents exported session data
type SessionSnapshot struct {
	Session    *Session             `json:"session"`
	FocusChain *focus.ChainSnapshot `json:"focus_chain,omitempty"`
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase
func toLower(s string) string {
	result := ""
	for _, ch := range s {
		if ch >= 'A' && ch <= 'Z' {
			result += string(ch + 32)
		} else {
			result += string(ch)
		}
	}
	return result
}
