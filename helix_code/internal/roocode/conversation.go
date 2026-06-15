package roocode

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConversationStore struct {
	mu            sync.RWMutex
	conversations map[string]*Conversation
}

func NewConversationStore() *ConversationStore {
	return &ConversationStore{conversations: make(map[string]*Conversation)}
}

func (cs *ConversationStore) Create(title string) *Conversation {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	conv := &Conversation{
		ID:        generateID(),
		Title:     title,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	cs.conversations[conv.ID] = conv
	// Returns the live pointer by design: Create hands the creating caller a
	// usable handle to the just-created conversation (callers observe later
	// state through it). The read-getters Get/List snapshot because they hand
	// the pointer to ARBITRARY later callers while concurrent writers exist; the
	// creating caller holds the only reference at this point. A caller that
	// shares this handle across goroutines AND reads it concurrently with
	// AddMessage must Get() a snapshot instead.
	return conv
}

func (cs *ConversationStore) AddMessage(convID, role, content string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	conv, ok := cs.conversations[convID]
	if !ok {
		conv = &Conversation{
			ID:        convID,
			Title:     tr(context.Background(), "internal_roocode_conversation_imported_default_title", nil),
			CreatedAt: time.Now().UTC(),
		}
		cs.conversations[convID] = conv
	}

	conv.Messages = append(conv.Messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().UTC(),
	})
	conv.UpdatedAt = time.Now().UTC()
	return nil
}

// Get returns a deep snapshot of the stored conversation. Returning the
// LIVE stored *Conversation would let the caller read conv.Messages /
// conv.UpdatedAt while a concurrent AddMessage appends to / mutates the
// same struct under cs.mu — a data race the caller cannot guard because
// it never sees cs.mu. snapshotConversation copies the header fields and
// clones the Messages slice so the returned value is fully detached.
func (cs *ConversationStore) Get(convID string) (*Conversation, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	conv, ok := cs.conversations[convID]
	if !ok {
		return nil, ErrTaskDelegationFailed
	}
	return snapshotConversation(conv), nil
}

func (cs *ConversationStore) List() []*Conversation {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := make([]*Conversation, 0, len(cs.conversations))
	for _, c := range cs.conversations {
		result = append(result, snapshotConversation(c))
	}
	return result
}

// snapshotConversation deep-copies a conversation (header + Messages
// slice) so the returned value shares no mutable backing array with the
// stored original. Caller MUST hold cs.mu (read or write) while calling.
func snapshotConversation(conv *Conversation) *Conversation {
	cp := *conv
	if conv.Messages != nil {
		cp.Messages = make([]Message, len(conv.Messages))
		copy(cp.Messages, conv.Messages)
	}
	return &cp
}

// idCounter is process-global (IDs must be unique across every
// ConversationStore in the process), so each ConversationStore's own
// cs.mu does NOT serialize it — two distinct stores calling Create
// concurrently would race a plain `idCounter++`. atomic.Int64 makes the
// increment safe regardless of which (or no) store mutex is held.
var idCounter atomic.Int64

func generateID() string {
	return fmt.Sprintf("conv-%d", idCounter.Add(1))
}
