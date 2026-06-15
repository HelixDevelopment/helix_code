package continua

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type ChatManager struct {
	mu       sync.RWMutex
	sessions map[string]*ChatSession
}

func NewChatManager() *ChatManager {
	return &ChatManager{sessions: make(map[string]*ChatSession)}
}

func (c *ChatManager) CreateSession(title, model string) *ChatSession {
	c.mu.Lock()
	defer c.mu.Unlock()

	session := &ChatSession{
		ID:    uuid.New().String(),
		Title: title,
		Model: model,
	}
	c.sessions[session.ID] = session
	// HXC-continua race fix: return a deep-copy snapshot, never the live
	// stored pointer. Handing the caller the live *ChatSession lets it read
	// session.Messages while a concurrent AddMessage appends under the write
	// lock — a data race on the slice header (caught by `go test -race`),
	// the GetSession-style getter race class fixed in internal/project.
	return snapshotSession(session)
}

func (c *ChatManager) AddMessage(ctx context.Context, sessionID, role, content string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, ok := c.sessions[sessionID]
	if !ok {
		session = &ChatSession{ID: sessionID, Title: "imported"}
		c.sessions[sessionID] = session
	}

	msg := ChatMessage{Role: role, Content: content}
	session.Messages = append(session.Messages, msg)
	return nil
}

func (c *ChatManager) GetSession(id string) (*ChatSession, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.sessions[id]
	if !ok {
		return nil, ErrChatFailed
	}
	// HXC-continua race fix: snapshot under the read lock. Returning the
	// live stored pointer escapes the lock — the caller reading
	// snapshot.Messages races a concurrent AddMessage append.
	return snapshotSession(s), nil
}

func (c *ChatManager) ListSessions() []*ChatSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []*ChatSession
	for _, s := range c.sessions {
		// HXC-continua race fix: snapshot each session, never the live pointer.
		result = append(result, snapshotSession(s))
	}
	return result
}

// snapshotSession returns a deep copy of s — a new ChatSession with its own
// Messages backing array — so a returned value never aliases the live stored
// session's slice. ChatMessage holds only value fields (strings), so a
// fresh slice with copied elements is a full deep copy. Callers MUST hold
// c.mu (read or write) while invoking this.
func snapshotSession(s *ChatSession) *ChatSession {
	if s == nil {
		return nil
	}
	cp := &ChatSession{
		ID:    s.ID,
		Title: s.Title,
		Model: s.Model,
	}
	if len(s.Messages) > 0 {
		cp.Messages = make([]ChatMessage, len(s.Messages))
		copy(cp.Messages, s.Messages)
	}
	return cp
}

func (c *ChatManager) SetModel(sessionID, model string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.sessions[sessionID]
	if !ok {
		return ErrChatFailed
	}
	s.Model = model
	return nil
}

// Diff produces a simple diff between two strings.
func Diff(oldContent, newContent string) *DiffResult {
	oldLines := stringsSplit(oldContent, "\n")
	newLines := stringsSplit(newContent, "\n")

	additions := 0
	deletions := 0
	var patch strings.Builder

	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen; i++ {
		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			if oldLine != "" {
				patch.WriteString(fmt.Sprintf("- %s\n", oldLine))
				deletions++
			}
			if newLine != "" {
				patch.WriteString(fmt.Sprintf("+ %s\n", newLine))
				additions++
			}
		}
	}

	return &DiffResult{
		Additions: additions,
		Deletions: deletions,
		Patch:     patch.String(),
	}
}

func stringsSplit(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	parts := strings.SplitN(s, sep, -1)
	for _, p := range parts {
		result = append(result, p)
	}
	return result
}
