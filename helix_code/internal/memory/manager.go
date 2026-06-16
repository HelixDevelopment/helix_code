package memory

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Manager manages conversation memory
type Manager struct {
	conversations    map[string]*Conversation // All conversations by ID
	activeConv       *Conversation            // Currently active conversation
	maxMessages      int                      // Maximum messages per conversation
	maxTokens        int                      // Maximum tokens per conversation
	maxConversations int                      // Maximum conversations to keep
	mu               sync.RWMutex             // Thread-safety
	onCreate         []ConversationCallback   // Callbacks on conversation creation
	onMessage        []MessageCallback        // Callbacks on message addition
	onClear          []ConversationCallback   // Callbacks on conversation clear
	onDelete         []ConversationCallback   // Callbacks on conversation deletion
	persist          *persistenceState        // Durable write-through provider (nil = legacy in-memory only)
}

// ConversationCallback is called for conversation events
type ConversationCallback func(*Conversation)

// MessageCallback is called for message events
type MessageCallback func(*Conversation, *Message)

// NewManager creates a new memory manager
func NewManager() *Manager {
	return &Manager{
		conversations:    make(map[string]*Conversation),
		maxMessages:      1000,   // Default max messages
		maxTokens:        100000, // Default max tokens (~25K words)
		maxConversations: 100,    // Default max conversations
		onCreate:         make([]ConversationCallback, 0),
		onMessage:        make([]MessageCallback, 0),
		onClear:          make([]ConversationCallback, 0),
		onDelete:         make([]ConversationCallback, 0),
	}
}

// CreateConversation creates a new conversation
func (m *Manager) CreateConversation(title string) (*Conversation, error) {
	if title == "" {
		return nil, fmt.Errorf("conversation title cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	conv := NewConversation(title)
	m.conversations[conv.ID] = conv

	// Trigger callbacks
	for _, callback := range m.onCreate {
		callback(conv)
	}

	return conv, nil
}

// GetConversation gets a conversation by ID.
//
// Concurrency contract (HXC-014 §11.4.85 chaos fix): the returned *Conversation
// is a deep CLONE, never the live stored pointer. Returning the live pointer
// previously let a caller read conv.Messages / conv.MessageCount concurrently
// with a writer's AddMessage / ClearConversation (which mutate the same struct
// under the Manager write lock) — a genuine data race the Manager's RWMutex did
// NOT protect against, because the mutex only guards the conversations map, not
// the contents of a pointer that has already escaped the critical section.
// Cloning under the read lock makes the snapshot immutable from the caller's
// perspective, so reads and writes can no longer alias the same memory.
func (m *Manager) GetConversation(id string) (*Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, exists := m.conversations[id]
	if !exists {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}

	return conv.Clone(), nil
}

// GetActive returns a race-free clone of the active conversation (or nil).
// See Manager.GetConversation for the HXC-014 §11.4.85 cloning rationale.
func (m *Manager) GetActive() *Conversation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.activeConv == nil {
		return nil
	}
	return m.activeConv.Clone()
}

// SetActive sets the active conversation
func (m *Manager) SetActive(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[id]
	if !exists {
		return fmt.Errorf("conversation not found: %s", id)
	}

	m.activeConv = conv
	return nil
}

// AddMessage adds a message to a conversation
func (m *Manager) AddMessage(convID string, message *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[convID]
	if !exists {
		return fmt.Errorf("conversation not found: %s", convID)
	}

	conv.AddMessage(message)
	conv.Version++
	conv.UpdatedAt = time.Now()

	// Check limits and truncate if needed
	m.enforceConversationLimits(conv)

	// Trigger callbacks
	for _, callback := range m.onMessage {
		callback(conv, message)
	}

	return nil
}

// AddMessageToActive adds a message to the active conversation
func (m *Manager) AddMessageToActive(message *Message) error {
	m.mu.RLock()
	active := m.activeConv
	m.mu.RUnlock()

	if active == nil {
		return fmt.Errorf("no active conversation")
	}

	return m.AddMessage(active.ID, message)
}

// DeleteConversation deletes a conversation
func (m *Manager) DeleteConversation(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[id]
	if !exists {
		return fmt.Errorf("conversation not found: %s", id)
	}

	// Clear active if deleting active conversation
	if m.activeConv != nil && m.activeConv.ID == id {
		m.activeConv = nil
	}

	delete(m.conversations, id)

	// Trigger callbacks
	for _, callback := range m.onDelete {
		callback(conv)
	}

	return nil
}

// ClearConversation clears all messages from a conversation
func (m *Manager) ClearConversation(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[id]
	if !exists {
		return fmt.Errorf("conversation not found: %s", id)
	}

	conv.Clear()
	conv.Version++
	conv.UpdatedAt = time.Now()

	// Trigger callbacks
	for _, callback := range m.onClear {
		callback(conv)
	}

	return nil
}

// GetAll returns all conversations
func (m *Manager) GetAll() []*Conversation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conversations := make([]*Conversation, 0, len(m.conversations))
	for _, conv := range m.conversations {
		// HXC-014 §11.4.85: hand out race-free clones, not live pointers.
		conversations = append(conversations, conv.Clone())
	}

	return conversations
}

// GetBySession returns conversations for a session
func (m *Manager) GetBySession(sessionID string) []*Conversation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conversations := make([]*Conversation, 0)
	for _, conv := range m.conversations {
		if conv.SessionID == sessionID {
			// HXC-014 §11.4.85: hand out race-free clones, not live pointers.
			conversations = append(conversations, conv.Clone())
		}
	}

	return conversations
}

// GetRecent returns the N most recently updated conversations
func (m *Manager) GetRecent(n int) []*Conversation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get all conversations
	all := make([]*Conversation, 0, len(m.conversations))
	for _, conv := range m.conversations {
		all = append(all, conv)
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

	// HXC-014 §11.4.85: hand out race-free clones, not live pointers.
	out := make([]*Conversation, n)
	for i := 0; i < n; i++ {
		out[i] = all[i].Clone()
	}
	return out
}

// Search searches for conversations containing query
func (m *Manager) Search(query string) []*Conversation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(query)
	conversations := make([]*Conversation, 0)

	for _, conv := range m.conversations {
		// Search in title
		if strings.Contains(strings.ToLower(conv.Title), query) {
			// HXC-014 §11.4.85: hand out race-free clones, not live pointers.
			conversations = append(conversations, conv.Clone())
			continue
		}

		// Search in messages
		for _, msg := range conv.Messages {
			if strings.Contains(strings.ToLower(msg.Content), query) {
				conversations = append(conversations, conv.Clone())
				break
			}
		}
	}

	return conversations
}

// SearchMessages searches for messages across all conversations
func (m *Manager) SearchMessages(query string) []*Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(query)
	messages := make([]*Message, 0)

	for _, conv := range m.conversations {
		for _, msg := range conv.Messages {
			if strings.Contains(strings.ToLower(msg.Content), query) {
				messages = append(messages, msg)
			}
		}
	}

	return messages
}

// Count returns the total number of conversations
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.conversations)
}

// TotalMessages returns the total number of messages across all conversations
func (m *Manager) TotalMessages() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for _, conv := range m.conversations {
		total += conv.MessageCount
	}

	return total
}

// TotalTokens returns the total number of tokens across all conversations
func (m *Manager) TotalTokens() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for _, conv := range m.conversations {
		total += conv.TokenCount
	}

	return total
}

// SetMaxMessages sets the maximum messages per conversation
func (m *Manager) SetMaxMessages(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maxMessages = max
}

// SetMaxTokens sets the maximum tokens per conversation
func (m *Manager) SetMaxTokens(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maxTokens = max
}

// SetMaxConversations sets the maximum conversations to keep
func (m *Manager) SetMaxConversations(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maxConversations = max
}

// TrimConversations removes old conversations beyond maxConversations
func (m *Manager) TrimConversations() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.conversations) <= m.maxConversations {
		return 0
	}

	// Get all conversations
	all := make([]*Conversation, 0, len(m.conversations))
	for _, conv := range m.conversations {
		all = append(all, conv)
	}

	// Sort by UpdatedAt (oldest first)
	for i := 0; i < len(all)-1; i++ {
		for j := i + 1; j < len(all); j++ {
			if all[i].UpdatedAt.After(all[j].UpdatedAt) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	// Remove oldest conversations
	removed := 0
	toRemove := len(all) - m.maxConversations

	for i := 0; i < toRemove; i++ {
		conv := all[i]

		// Don't remove active conversation
		if m.activeConv != nil && m.activeConv.ID == conv.ID {
			continue
		}

		delete(m.conversations, conv.ID)
		removed++
	}

	return removed
}

// enforceConversationLimits enforces message and token limits (internal, no lock)
func (m *Manager) enforceConversationLimits(conv *Conversation) {
	// Check message limit
	if m.maxMessages > 0 && len(conv.Messages) > m.maxMessages {
		keepLast := m.maxMessages / 2 // Keep last 50%
		conv.Truncate(keepLast)
	}

	// Check token limit
	if m.maxTokens > 0 && conv.TokenCount > m.maxTokens {
		// Truncate to 75% of limit
		targetTokens := m.maxTokens * 3 / 4
		keepMessages := 0

		// Count backwards to find how many messages to keep
		tokenCount := 0
		for i := len(conv.Messages) - 1; i >= 0; i-- {
			tokenCount += conv.Messages[i].TokenCount
			if tokenCount > targetTokens {
				keepMessages = len(conv.Messages) - i
				break
			}
		}

		if keepMessages > 0 {
			conv.Truncate(keepMessages)
		}
	}
}

// Clear removes all conversations
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.conversations = make(map[string]*Conversation)
	m.activeConv = nil
}

// GetStatistics returns manager statistics
func (m *Manager) GetStatistics() *ManagerStatistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ManagerStatistics{
		TotalConversations: len(m.conversations),
		TotalMessages:      0,
		TotalTokens:        0,
		ByRole:             make(map[Role]int),
	}

	for _, conv := range m.conversations {
		stats.TotalMessages += conv.MessageCount
		stats.TotalTokens += conv.TokenCount

		for _, msg := range conv.Messages {
			stats.ByRole[msg.Role]++
		}
	}

	if stats.TotalMessages > 0 {
		stats.AverageMessagesPerConv = float64(stats.TotalMessages) / float64(stats.TotalConversations)
		stats.AverageTokensPerMessage = float64(stats.TotalTokens) / float64(stats.TotalMessages)
	}

	return stats
}

// OnCreate registers a callback for conversation creation.
//
// The callback slices are read under m.mu by the operation methods
// (CreateConversation/AddMessage/ClearConversation/DeleteConversation range
// them while holding the lock), so registration MUST take the write lock to
// append — otherwise a concurrent register-during-operation is an
// unsynchronised read/write of the slice header (data race).
func (m *Manager) OnCreate(callback ConversationCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onCreate = append(m.onCreate, callback)
}

// OnMessage registers a callback for message addition. See OnCreate for the
// locking rationale.
func (m *Manager) OnMessage(callback MessageCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMessage = append(m.onMessage, callback)
}

// OnClear registers a callback for conversation clear. See OnCreate for the
// locking rationale.
func (m *Manager) OnClear(callback ConversationCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onClear = append(m.onClear, callback)
}

// OnDelete registers a callback for conversation deletion. See OnCreate for the
// locking rationale.
func (m *Manager) OnDelete(callback ConversationCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onDelete = append(m.onDelete, callback)
}

// Export exports a conversation
func (m *Manager) Export(convID string) (*ConversationSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, exists := m.conversations[convID]
	if !exists {
		return nil, fmt.Errorf("conversation not found: %s", convID)
	}

	return &ConversationSnapshot{
		Conversation: conv.Clone(),
		ExportedAt:   time.Now(),
	}, nil
}

// Import imports a conversation
func (m *Manager) Import(snapshot *ConversationSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate ID
	if _, exists := m.conversations[snapshot.Conversation.ID]; exists {
		return fmt.Errorf("conversation with ID '%s' already exists", snapshot.Conversation.ID)
	}

	// HXC-014 §11.4.85: store a deep CLONE, never the caller's live pointer.
	// Storing snapshot.Conversation directly would let the caller's retained
	// reference alias the manager-owned struct — a subsequent manager-side
	// mutation (AddMessage under the write lock) concurrent with a caller-side
	// read of the same struct is a data race the Manager RWMutex cannot guard,
	// because the mutex only protects the conversations map, not the contents of
	// a pointer that escaped the critical section. Cloning on the store path
	// mirrors the read-path clone in GetConversation/GetAll and closes the
	// aliasing hole.
	m.conversations[snapshot.Conversation.ID] = snapshot.Conversation.Clone()

	return nil
}

// ManagerStatistics contains manager statistics
type ManagerStatistics struct {
	TotalConversations      int          // Total conversations
	TotalMessages           int          // Total messages
	TotalTokens             int          // Total tokens
	ByRole                  map[Role]int // Message count by role
	AverageMessagesPerConv  float64      // Average messages per conversation
	AverageTokensPerMessage float64      // Average tokens per message
}

// String returns a string representation
func (s *ManagerStatistics) String() string {
	return fmt.Sprintf("Conversations: %d, Messages: %d, Tokens: %d",
		s.TotalConversations, s.TotalMessages, s.TotalTokens)
}

// ConversationSnapshot represents exported conversation data
type ConversationSnapshot struct {
	Conversation *Conversation `json:"conversation"`
	ExportedAt   time.Time     `json:"exported_at"`
}

// ConflictResolution represents the result of a conflict resolution attempt
type ConflictResolution struct {
	Resolved   bool          // Whether the conflict was resolved
	Current    *Conversation // Current version in memory
	Incoming   *Conversation // Incoming version that caused conflict
	Resolution *Conversation // Resolved version (if auto-resolved)
	Error      error         // Error if resolution failed
}

// UpdateConversationWithVersion updates a conversation with version conflict detection
func (m *Manager) UpdateConversationWithVersion(id string, updated *Conversation, expectedVersion int64) (*ConflictResolution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, exists := m.conversations[id]
	if !exists {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}

	// Check version conflict
	if current.Version != expectedVersion {
		return &ConflictResolution{
			Resolved: false,
			Current:  current,
			Incoming: updated,
			Error:    fmt.Errorf("version conflict: expected %d, got %d", expectedVersion, current.Version),
		}, nil
	}

	// Update version and timestamp
	updated.Version = current.Version + 1
	updated.UpdatedAt = time.Now()

	// Update in memory
	m.conversations[id] = updated

	// Update active conversation if needed
	if m.activeConv != nil && m.activeConv.ID == id {
		m.activeConv = updated
	}

	return &ConflictResolution{
		Resolved: true,
		Current:  updated,
	}, nil
}

// ResolveConflict attempts to auto-resolve a version conflict
func (m *Manager) ResolveConflict(id string, incoming *Conversation, strategy string) (*ConflictResolution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, exists := m.conversations[id]
	if !exists {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}

	switch strategy {
	case "overwrite":
		// Overwrite with incoming version
		incoming.Version = current.Version + 1
		incoming.UpdatedAt = time.Now()
		m.conversations[id] = incoming

		if m.activeConv != nil && m.activeConv.ID == id {
			m.activeConv = incoming
		}

		return &ConflictResolution{
			Resolved:   true,
			Current:    incoming,
			Incoming:   incoming,
			Resolution: incoming,
		}, nil

	case "merge":
		// Simple merge: keep current title, append new messages
		merged := &Conversation{
			ID:           current.ID,
			Title:        current.Title, // Keep current title
			SessionID:    current.SessionID,
			CharacterID:  current.CharacterID,
			UserID:       current.UserID,
			Messages:     append(current.Messages, incoming.Messages...), // Append messages
			CharMessages: append(current.CharMessages, incoming.CharMessages...),
			Metadata:     mergeMetadata(current.Metadata, incoming.Metadata),
			CreatedAt:    current.CreatedAt,
			UpdatedAt:    time.Now(),
			Version:      current.Version + 1,
			Status:       incoming.Status, // Use incoming status
			Summary:      incoming.Summary,
			TokenCount:   current.TokenCount + incoming.TokenCount,
			MessageCount: current.MessageCount + incoming.MessageCount,
		}

		m.conversations[id] = merged

		if m.activeConv != nil && m.activeConv.ID == id {
			m.activeConv = merged
		}

		return &ConflictResolution{
			Resolved:   true,
			Current:    merged,
			Incoming:   incoming,
			Resolution: merged,
		}, nil

	default:
		return &ConflictResolution{
			Resolved: false,
			Current:  current,
			Incoming: incoming,
			Error:    fmt.Errorf("unknown conflict resolution strategy: %s", strategy),
		}, nil
	}
}

// mergeMetadata merges two metadata maps
func mergeMetadata(current, incoming map[string]string) map[string]string {
	merged := make(map[string]string)

	// Copy current
	for k, v := range current {
		merged[k] = v
	}

	// Override with incoming
	for k, v := range incoming {
		merged[k] = v
	}

	return merged
}

// GetConversationVersion returns the current version of a conversation
func (m *Manager) GetConversationVersion(id string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, exists := m.conversations[id]
	if !exists {
		return 0, fmt.Errorf("conversation not found: %s", id)
	}

	return conv.Version, nil
}
