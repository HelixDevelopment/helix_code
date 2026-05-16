package roocode

import (
	"fmt"
	"sync"
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
	return conv
}

func (cs *ConversationStore) AddMessage(convID, role, content string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	conv, ok := cs.conversations[convID]
	if !ok {
		conv = &Conversation{
			ID:        convID,
			Title:     "imported",
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

func (cs *ConversationStore) Get(convID string) (*Conversation, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	conv, ok := cs.conversations[convID]
	if !ok {
		return nil, ErrTaskDelegationFailed
	}
	return conv, nil
}

func (cs *ConversationStore) List() []*Conversation {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var result []*Conversation
	for _, c := range cs.conversations {
		result = append(result, c)
	}
	return result
}

var idCounter int

func generateID() string {
	idCounter++
	return fmt.Sprintf("conv-%d", idCounter)
}
