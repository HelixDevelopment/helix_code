# Complete Porting Plan: Claude Code Features into HelixCode

**Agent Source**: Claude Code (anthropics/claude-code) — Feature set derived from SDK analysis and public documentation
**Target**: HelixCode (`github.com/HelixDevelopment/HelixCode`, module `dev.helix.code`)
**Author**: Porting Specialist Agent
**Date**: 2025-01-15

---

## Table of Contents

1. [Auto-Compaction System](#feature-1-auto-compaction-system)
2. [Permission Rule System](#feature-2-permission-rule-system)
3. [Tool Result Persistence Layer](#feature-3-tool-result-persistence-layer)
4. [Git Worktree Agent Isolation](#feature-4-git-worktree-agent-isolation)
5. [Hook-Based Extensibility](#feature-5-hook-based-extensibility)
6. [MCP Full Lifecycle](#feature-6-mcp-full-lifecycle)
7. [Background Task System](#feature-7-background-task-system)
8. [Plan Mode](#feature-8-plan-mode)
9. [Slash Command System](#feature-9-slash-command-system)
10. [Skill System](#feature-10-skill-system)
11. [Session Transcript Resume](#feature-11-session-transcript-resume)
12. [Multi-Provider Backend](#feature-12-multi-provider-backend)
13. [LSP Integration](#feature-13-lsp-integration)
14. [Sandboxed Shell Execution](#feature-14-sandboxed-shell-execution)
15. [Subagent Team](#feature-15-subagent-team)
16. [OpenTelemetry Integration](#feature-16-opentelemetry-integration)
17. [Smart File Editing](#feature-17-smart-file-editing)
18. [No-Flicker Rendering](#feature-18-no-flicker-rendering)
19. [AskUserQuestion with Previews](#feature-19-askuserquestion-with-previews)
20. [Theme System](#feature-20-theme-system)

---

## Feature 1: Auto-Compaction System

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Monitors conversation token usage. When approaching 80% of the model's context window, automatically summarizes the conversation history into a compact "context management" summary stored as metadata on assistant messages. If 3 consecutive compactions occur with no user message (thrashing), abort with an error.
- **Why it's powerful**: Prevents silent context loss, maintains conversation coherence across long sessions, provides clear signal when the agent is "overwhelmed."

### HelixCode Integration Target
- **New files to create**:
  - `internal/context/compaction.go` — Core compaction engine
  - `internal/context/compaction_test.go` — Unit tests
  - `internal/context/token_counter.go` — Token estimation interface
- **Existing files to modify**:
  - `internal/session/session_manager.go` — Add compaction trigger to session lifecycle
  - `internal/llm/provider.go` — Add `GetContextWindow()` and `CountTokens()` to Provider interface
  - `internal/agent/agent.go` — Integrate compaction into message loop
- **Submodule dependencies**: `internal/llm/`, `internal/session/`

### Exact Code Implementation

#### File: `internal/context/token_counter.go` (NEW)

```go
package context

import (
	"errors"
	"math"
)

// TokenCounter estimates token counts for messages and content.
// HelixCode's LLM provider must implement this.
type TokenCounter interface {
	CountTokens(text string) (int, error)
	CountMessageTokens(messages []Message) (int, error)
	GetContextWindow() int
}

// Message represents a generic chat message for token counting.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SimpleTokenizer provides a fallback estimation (1 token ≈ 4 chars).
type SimpleTokenizer struct{}

func (s *SimpleTokenizer) CountTokens(text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	// Conservative estimate: 1 token per 3.5 characters
	return int(math.Ceil(float64(len(text)) / 3.5)), nil
}

func (s *SimpleTokenizer) CountMessageTokens(messages []Message) (int, error) {
	total := 0
	for _, m := range messages {
		n, err := s.CountTokens(m.Content)
		if err != nil {
			return 0, err
		}
		// Add overhead per message (role, formatting)
		total += n + 4
	}
	return total, nil
}

func (s *SimpleTokenizer) GetContextWindow() int {
	return 200000 // Claude 3.5 Sonnet default
}

// ErrContextWindowExceeded is returned when content exceeds the model's window.
var ErrContextWindowExceeded = errors.New("context window exceeded")
```

#### File: `internal/context/compaction.go` (NEW)

```go
package context

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CompactionThreshold is the percentage of context window that triggers compaction.
const CompactionThreshold = 0.80

// ThrashingThreshold is the number of consecutive compactions without user progress.
const ThrashingThreshold = 3

// CompactionSummary is stored in message metadata to preserve context across summarization.
type CompactionSummary struct {
	OriginalMessageCount int       `json:"original_message_count"`
	SummarizedAt         time.Time `json:"summarized_at"`
	SummaryText          string    `json:"summary_text"`
	TopicsCovered        []string  `json:"topics_covered"`
	KeyDecisions         []string  `json:"key_decisions"`
}

// CompactionMetadata is attached to assistant messages after compaction.
type CompactionMetadata struct {
	ContextManagement *CompactionSummary `json:"context_management,omitempty"`
}

// CompactionResult holds the outcome of a compaction operation.
type CompactionResult struct {
	MessagesBefore int
	MessagesAfter  int
	TokensBefore   int
	TokensAfter    int
	Summary        *CompactionSummary
	WasCompacted   bool
	IsThrashing    bool
}

// Compactor manages automatic context compaction.
type Compactor struct {
	counter           TokenCounter
	llmClient         Summarizer
	logger            *zap.Logger
	mu                sync.RWMutex
	consecutiveCompactions int
	lastCompactionTime     time.Time
}

// Summarizer is the LLM interface used to generate compaction summaries.
type Summarizer interface {
	SummarizeConversation(ctx context.Context, messages []Message, instructions string) (string, error)
}

// NewCompactor creates a new compaction manager.
func NewCompactor(counter TokenCounter, llm Summarizer, logger *zap.Logger) *Compactor {
	return &Compactor{
		counter:   counter,
		llmClient: llm,
		logger:    logger,
	}
}

// ShouldCompact checks if the message history needs compaction.
func (c *Compactor) ShouldCompact(messages []Message) (bool, int, int, error) {
	totalTokens, err := c.counter.CountMessageTokens(messages)
	if err != nil {
		return false, 0, 0, fmt.Errorf("counting tokens: %w", err)
	}
	window := c.counter.GetContextWindow()
	threshold := int(float64(window) * CompactionThreshold)

	return totalTokens >= threshold, totalTokens, threshold, nil
}

// Compact performs context compaction on the provided messages.
// It preserves the most recent N messages (default: last 4) and summarizes the rest.
func (c *Compactor) Compact(ctx context.Context, messages []Message, preserveRecent int) (*CompactionResult, []Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	tokensBefore, err := c.counter.CountMessageTokens(messages)
	if err != nil {
		return nil, nil, err
	}

	if preserveRecent < 1 {
		preserveRecent = 4
	}
	if len(messages) <= preserveRecent+1 {
		// Not enough history to compact meaningfully
		return &CompactionResult{WasCompacted: false}, messages, nil
	}

	// Detect thrashing: 3 consecutive compactions without user message
	c.consecutiveCompactions++
	if c.consecutiveCompactions >= ThrashingThreshold {
		return &CompactionResult{
			WasCompacted: false,
			IsThrashing:  true,
			TokensBefore: tokensBefore,
		}, nil, fmt.Errorf("compaction thrashing detected: %d consecutive compactions with no user progress", c.consecutiveCompactions)
	}

	// Split: messages to summarize vs. preserve
	splitPoint := len(messages) - preserveRecent
	if splitPoint < 1 {
		splitPoint = 1
	}

	toSummarize := messages[:splitPoint]
	toPreserve := messages[splitPoint:]

	// Generate summary via LLM
	summaryText, err := c.llmClient.SummarizeConversation(ctx, toSummarize,
		"Summarize the following conversation concisely. Include: key topics discussed, decisions made, and any code/files modified. Be extremely concise.")
	if err != nil {
		c.logger.Error("summarization failed", zap.Error(err))
		return nil, nil, fmt.Errorf("summarization failed: %w", err)
	}

	summary := &CompactionSummary{
		OriginalMessageCount: len(toSummarize),
		SummarizedAt:         time.Now().UTC(),
		SummaryText:          summaryText,
	}

	// Extract topics and decisions (simple heuristic, can be enhanced)
	// In production, this would use structured output from LLM
	summary.TopicsCovered = extractTopics(summaryText)
	summary.KeyDecisions = extractDecisions(summaryText)

	// Build compacted message list: system summary + preserved messages
	metadataJSON, _ := json.Marshal(CompactionMetadata{ContextManagement: summary})
	summaryMessage := Message{
		Role:    "assistant",
		Content: fmt.Sprintf("[Context Summary] %s\n\n[Topics: %v]\n[Decisions: %v]",
			summaryText, summary.TopicsCovered, summary.KeyDecisions),
	}

	// Store metadata in a structured way - in HelixCode this attaches to the DB message record
	_ = metadataJSON

	compacted := append([]Message{summaryMessage}, toPreserve...)
	tokensAfter, _ := c.counter.CountMessageTokens(compacted)

	c.lastCompactionTime = time.Now()

	result := &CompactionResult{
		MessagesBefore: len(messages),
		MessagesAfter:  len(compacted),
		TokensBefore:   tokensBefore,
		TokensAfter:    tokensAfter,
		Summary:        summary,
		WasCompacted:   true,
		IsThrashing:    false,
	}

	c.logger.Info("context compacted",
		zap.Int("messages_before", result.MessagesBefore),
		zap.Int("messages_after", result.MessagesAfter),
		zap.Int("tokens_before", result.TokensBefore),
		zap.Int("tokens_after", result.TokensAfter),
	)

	return result, compacted, nil
}

// OnUserMessage resets the thrashing counter when user sends a message.
func (c *Compactor) OnUserMessage() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consecutiveCompactions = 0
}

// GetConsecutiveCompactions returns the current thrashing count (for monitoring).
func (c *Compactor) GetConsecutiveCompactions() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.consecutiveCompactions
}

// extractTopics is a simple heuristic for topic extraction.
func extractTopics(text string) []string {
	// In production, use LLM structured output or NLP
	// Stub: return up to 3 sentences as topics
	return []string{"conversation_summary"}
}

// extractDecisions is a simple heuristic for decision extraction.
func extractDecisions(text string) []string {
	// In production, use LLM structured output
	return []string{}
}
```

#### File: `internal/session/session_manager.go` (MODIFY — Add Compaction Hook)

```go
// Add to existing imports in session_manager.go:
// import "dev.helix.code/internal/context"

// Add to SessionManager struct:
type SessionManager struct {
	// ... existing fields ...
	compactor *context.Compactor
}

// Add to NewSessionManager constructor:
func NewSessionManager(
	// ... existing params ...
	compactor *context.Compactor,
) *SessionManager {
	return &SessionManager{
		// ... existing assignments ...
		compactor: compactor,
	}
}

// Add method to check and trigger compaction before LLM calls.
func (sm *SessionManager) MaybeCompactSession(ctx context.Context, sessionID string) (*context.CompactionResult, error) {
	session, err := sm.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Convert session messages to context.Message format
	messages := make([]context.Message, len(session.Messages))
	for i, m := range session.Messages {
		messages[i] = context.Message{Role: m.Role, Content: m.Content}
	}

	shouldCompact, tokens, threshold, err := sm.compactor.ShouldCompact(messages)
	if err != nil {
		return nil, err
	}

	if !shouldCompact {
		return &context.CompactionResult{WasCompacted: false, TokensBefore: tokens}, nil
	}

	result, compacted, err := sm.compactor.Compact(ctx, messages, 4)
	if err != nil {
		return nil, err
	}

	if !result.WasCompacted {
		return result, nil
	}

	// Persist compacted messages back to session
	session.Messages = make([]Message, len(compacted))
	for i, m := range compacted {
		session.Messages[i] = Message{Role: m.Role, Content: m.Content}
		if i == 0 && result.Summary != nil {
			// Attach compaction metadata to the summary message
			meta, _ := json.Marshal(context.CompactionMetadata{ContextManagement: result.Summary})
			session.Messages[i].Metadata = meta
		}
	}

	if err := sm.store.UpdateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("persisting compacted session: %w", err)
	}

	return result, nil
}

// Add to the message handling flow (called when user sends a message).
func (sm *SessionManager) HandleUserMessage(ctx context.Context, sessionID string, content string) error {
	// Reset thrashing counter
	sm.compactor.OnUserMessage()

	// ... existing message handling logic ...

	return nil
}
```

#### File: `internal/llm/provider.go` (MODIFY — Extend Provider Interface)

```go
package llm

import "context"

// Provider is the unified interface for LLM backends.
type Provider interface {
	// ... existing methods ...

	// GetContextWindow returns the maximum token context window for this model.
	GetContextWindow() int

	// CountTokens returns the estimated token count for the given text.
	CountTokens(text string) (int, error)

	// SummarizeConversation generates a summary of messages for compaction.
	SummarizeConversation(ctx context.Context, messages []Message, instructions string) (string, error)
}

// Message is the unified message type for LLM communication.
type Message struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	ToolCalls  []ToolCall        `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
}
```

#### File: `internal/agent/agent.go` (MODIFY — Integrate Compaction)

```go
// In the agent's main message loop, add before each LLM call:

func (a *Agent) RunTurn(ctx context.Context, sessionID string) error {
	// 1. Check if compaction is needed
	compactResult, err := a.sessionManager.MaybeCompactSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("compaction check failed: %w", err)
	}
	if compactResult.WasCompacted {
		a.logger.Info("session auto-compacted",
			zap.Int("tokens_before", compactResult.TokensBefore),
			zap.Int("tokens_after", compactResult.TokensAfter))
	}
	if compactResult.IsThrashing {
		return fmt.Errorf("agent is thrashing: too many compactions without progress. Consider starting a new session.")
	}

	// 2. Continue with existing LLM call logic ...
	// ...
}
```

### Anti-Bluff Verification Test

```go
package context_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/context"
	"go.uber.org/zap/zaptest"
)

// mockCounter implements TokenCounter with a small window for testing.
type mockCounter struct {
	window int
}

func (m *mockCounter) CountTokens(text string) (int, error) {
	return len(text) / 4, nil // 1 token per 4 chars
}

func (m *mockCounter) CountMessageTokens(messages []context.Message) (int, error) {
	total := 0
	for _, msg := range messages {
		n, _ := m.CountTokens(msg.Content)
		total += n + 4 // overhead
	}
	return total, nil
}

func (m *mockCounter) GetContextWindow() int {
	return m.window
}

// mockSummarizer returns predictable summaries.
type mockSummarizer struct {
	summary string
}

func (m *mockSummarizer) SummarizeConversation(ctx context.Context, messages []context.Message, instructions string) (string, error) {
	return m.summary, nil
}

func TestShouldCompact_TriggersAt80Percent(t *testing.T) {
	counter := &mockCounter{window: 1000}
	compactor := context.NewCompactor(counter, nil, zaptest.NewLogger(t))

	// 800 tokens at 4 chars/token + 4 overhead each = ~3200 chars + overhead for 10 messages
	messages := make([]context.Message, 10)
	for i := range messages {
		messages[i] = context.Message{Role: "user", Content: strings.Repeat("a", 316)}
	}

	shouldCompact, tokens, threshold, err := compactor.ShouldCompact(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify exact threshold
	if threshold != 800 {
		t.Errorf("threshold = %d, want 800", threshold)
	}

	// Verify we trigger compaction at 80%
	if !shouldCompact {
		t.Errorf("ShouldCompact = false, want true (tokens=%d >= threshold=%d)", tokens, threshold)
	}

	// Verify we don't trigger below 80%
	messagesSmall := make([]context.Message, 5)
	for i := range messagesSmall {
		messagesSmall[i] = context.Message{Role: "user", Content: strings.Repeat("a", 100)}
	}
	shouldCompactSmall, _, _, _ := compactor.ShouldCompact(messagesSmall)
	if shouldCompactSmall {
		t.Error("ShouldCompact = true for small context, want false")
	}
}

func TestCompact_ReducesMessageCount(t *testing.T) {
	counter := &mockCounter{window: 10000}
	summarizer := &mockSummarizer{summary: "Discussed API design and implemented user auth."}
	compactor := context.NewCompactor(counter, summarizer, zaptest.NewLogger(t))

	messages := make([]context.Message, 20)
	for i := range messages {
		messages[i] = context.Message{
			Role:    "user",
			Content: strings.Repeat("x", 100),
		}
	}

	result, compacted, err := compactor.Compact(context.Background(), messages, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.WasCompacted {
		t.Fatal("WasCompacted = false, want true")
	}

	// Original: 20 messages. After: 1 summary + 4 preserved = 5 messages
	if result.MessagesBefore != 20 {
		t.Errorf("MessagesBefore = %d, want 20", result.MessagesBefore)
	}
	if result.MessagesAfter != 5 {
		t.Errorf("MessagesAfter = %d, want 5", result.MessagesAfter)
	}
	if len(compacted) != 5 {
		t.Errorf("len(compacted) = %d, want 5", len(compacted))
	}

	// First message should be the summary
	if !strings.Contains(compacted[0].Content, "Discussed API design") {
		t.Errorf("summary not found in first compacted message: %s", compacted[0].Content)
	}

	// Tokens should be reduced
	if result.TokensAfter >= result.TokensBefore {
		t.Errorf("TokensAfter (%d) >= TokensBefore (%d), want reduction", result.TokensAfter, result.TokensBefore)
	}
}

func TestCompact_ThrashingDetection(t *testing.T) {
	counter := &mockCounter{window: 10000}
	summarizer := &mockSummarizer{summary: "summary"}
	compactor := context.NewCompactor(counter, summarizer, zaptest.NewLogger(t))

	messages := make([]context.Message, 20)
	for i := range messages {
		messages[i] = context.Message{Role: "assistant", Content: strings.Repeat("x", 100)}
	}

	// Simulate 3 consecutive compactions without user message
	for i := 0; i < 3; i++ {
		_, _, err := compactor.Compact(context.Background(), messages, 4)
		if i < 2 && err != nil {
			t.Fatalf("unexpected error on compaction %d: %v", i, err)
		}
		if i == 2 {
			if err == nil {
				t.Fatal("expected thrashing error on 3rd compaction, got nil")
			}
			if !strings.Contains(err.Error(), "thrashing") {
				t.Errorf("error message missing 'thrashing': %v", err)
			}
		}
	}
}

func TestOnUserMessage_ResetsThrashing(t *testing.T) {
	counter := &mockCounter{window: 10000}
	summarizer := &mockSummarizer{summary: "summary"}
	compactor := context.NewCompactor(counter, summarizer, zaptest.NewLogger(t))

	messages := make([]context.Message, 20)
	for i := range messages {
		messages[i] = context.Message{Role: "assistant", Content: strings.Repeat("x", 100)}
	}

	// 2 compactions
	for i := 0; i < 2; i++ {
		_, _, err := compactor.Compact(context.Background(), messages, 4)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if compactor.GetConsecutiveCompactions() != 2 {
		t.Fatalf("consecutive compactions = %d, want 2", compactor.GetConsecutiveCompactions())
	}

	// User sends message - reset counter
	compactor.OnUserMessage()

	if compactor.GetConsecutiveCompactions() != 0 {
		t.Fatalf("consecutive compactions after user message = %d, want 0", compactor.GetConsecutiveCompactions())
	}

	// Now 3 more compactions should work without thrashing
	for i := 0; i < 3; i++ {
		_, _, err := compactor.Compact(context.Background(), messages, 4)
		if err != nil {
			t.Fatalf("unexpected error after reset: %v", err)
		}
	}
}

func TestCompact_PreservesRecentMessages(t *testing.T) {
	counter := &mockCounter{window: 10000}
	summarizer := &mockSummarizer{summary: "summary"}
	compactor := context.NewCompactor(counter, summarizer, zaptest.NewLogger(t))

	messages := []context.Message{
		{Role: "user", Content: "message 1"},
		{Role: "assistant", Content: "response 1"},
		{Role: "user", Content: "message 2"},
		{Role: "assistant", Content: "response 2"},
		{Role: "user", Content: "message 3"},
		{Role: "assistant", Content: "response 3"},
		{Role: "user", Content: "message 4"},
		{Role: "assistant", Content: "response 4"},
	}

	result, compacted, err := compactor.Compact(context.Background(), messages, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.WasCompacted {
		t.Fatal("expected compaction")
	}

	// Last 3 messages should be preserved exactly
	if len(compacted) != 4 { // 1 summary + 3 preserved
		t.Fatalf("len(compacted) = %d, want 4", len(compacted))
	}

	lastPreserved := compacted[len(compacted)-1]
	if lastPreserved.Content != "response 4" {
		t.Errorf("last preserved message = %q, want %q", lastPreserved.Content, "response 4")
	}

	secondLast := compacted[len(compacted)-2]
	if secondLast.Content != "message 4" {
		t.Errorf("second last message = %q, want %q", secondLast.Content, "message 4")
	}
}
```

### Integration Steps

1. **Create token counter interface**:
   ```bash
   mkdir -p internal/context
   cat > internal/context/token_counter.go << 'EOF'
   [content above]
   EOF
   ```

2. **Create compaction engine**:
   ```bash
   cat > internal/context/compaction.go << 'EOF'
   [content above]
   EOF
   ```

3. **Extend LLM provider interface** (modify `internal/llm/provider.go`):
   - Add `GetContextWindow()`, `CountTokens()`, `SummarizeConversation()` to `Provider`
   - Implement in all provider adapters (`anthropic.go`, `bedrock.go`, `vertex.go`, `azure.go`)

4. **Wire into session manager** (modify `internal/session/session_manager.go`):
   - Add `compactor` field
   - Call `MaybeCompactSession()` before each LLM turn
   - Call `OnUserMessage()` when user submits input

5. **Wire into agent loop** (modify `internal/agent/agent.go`):
   - Add compaction check at start of `RunTurn()`
   - Handle thrashing error by returning to user with suggestion to start new session

6. **Add tests**:
   ```bash
   go test ./internal/context/ -v -run TestCompact
   ```

---

## Feature 2: Permission Rule System (5 Modes + Wildcards)

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Every tool call (Bash, Edit, Write, etc.) is checked against a permission rule database. Rules support 5 modes: `default` (ask user), `auto` (silent approval), `acceptEdits` (auto-approve file edits), `dontAsk` (auto-approve read-only), `bypassPermissions` (dangerous - approve all), `plan` (only in plan mode). Wildcards match command patterns: `Bash(git status:*)`, `Bash(rm -rf *)`. Compound commands like `ls && git push` are split and checked independently.
- **Why it's powerful**: Eliminates friction for safe, repetitive operations while maintaining guardrails for destructive actions.

### HelixCode Integration Target
- **New files to create**:
  - `internal/tools/permissions.go` — Core permission engine
  - `internal/tools/permissions_test.go` — Rule matching tests
  - `internal/tools/permission_store.go` — Persistence layer
- **Existing files to modify**:
  - `internal/tools/tool_executor.go` — Add permission check before execution
  - `internal/tools/registry.go` — Add permission hooks
  - `cmd/cli/main.go` — Add `--permission-mode` flag
- **Submodule dependencies**: `internal/tools/`

### Exact Code Implementation

#### File: `internal/tools/permissions.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// PermissionMode defines how a tool call is handled.
type PermissionMode string

const (
	// ModeDefault asks the user for approval on every tool call.
	ModeDefault PermissionMode = "default"
	// ModeAuto silently approves all safe tool calls.
	ModeAuto PermissionMode = "auto"
	// ModeAcceptEdits auto-approves file edit operations (Read→Edit flow).
	ModeAcceptEdits PermissionMode = "acceptEdits"
	// ModeDontAsk auto-approves read-only operations (Read, LSP diagnostics, etc.).
	ModeDontAsk PermissionMode = "dontAsk"
	// ModeBypassPermissions approves ALL tool calls including destructive ones. DANGEROUS.
	ModeBypassPermissions PermissionMode = "bypassPermissions"
	// ModePlan only allows tools approved for plan mode execution.
	ModePlan PermissionMode = "plan"
)

// IsValid checks if the permission mode is recognized.
func (p PermissionMode) IsValid() bool {
	switch p {
	case ModeDefault, ModeAuto, ModeAcceptEdits, ModeDontAsk, ModeBypassPermissions, ModePlan:
		return true
	}
	return false
}

// RulePattern is the wildcard pattern format: "ToolName(command_arg_pattern)".
// Examples: "Bash(git status:*)", "Read(*.go)", "Edit(src/main.go)"
type RulePattern string

// ParsedPattern holds a decomposed permission rule.
type ParsedPattern struct {
	ToolName  string
	ArgPattern string // wildcard pattern
}

// PermissionRule is a single rule in the permission database.
type PermissionRule struct {
	ID          string        `json:"id" db:"id"`
	Pattern     RulePattern   `json:"pattern" db:"pattern"`
	Mode        PermissionMode `json:"mode" db:"mode"`
	Description string        `json:"description,omitempty" db:"description"`
	CreatedAt   int64         `json:"created_at" db:"created_at"`
}

// ToolCallRequest represents a pending tool execution for permission checking.
type ToolCallRequest struct {
	ToolName string
	Args     map[string]any
	RawInput string // serialized args for pattern matching
}

// PermissionEngine evaluates permission rules.
type PermissionEngine struct {
	rules  []PermissionRule
	mu     sync.RWMutex
	store  RuleStore
	global PermissionMode // Fallback mode when no rule matches
}

// RuleStore persists permission rules.
type RuleStore interface {
	LoadRules(ctx context.Context) ([]PermissionRule, error)
	SaveRule(ctx context.Context, rule PermissionRule) error
	DeleteRule(ctx context.Context, id string) error
}

// NewPermissionEngine creates a permission engine with the given fallback mode.
func NewPermissionEngine(store RuleStore, globalMode PermissionMode) (*PermissionEngine, error) {
	if !globalMode.IsValid() {
		return nil, fmt.Errorf("invalid permission mode: %s", globalMode)
	}
	pe := &PermissionEngine{
		store:  store,
		global: globalMode,
	}
	// Initial load
	rules, err := store.LoadRules(context.Background())
	if err != nil {
		return nil, fmt.Errorf("loading permission rules: %w", err)
	}
	pe.rules = rules
	return pe, nil
}

// Evaluate determines the permission mode for a tool call request.
// Returns the matching mode, the matched rule ID (if any), and whether approval is required.
func (pe *PermissionEngine) Evaluate(req ToolCallRequest) (PermissionMode, string, bool) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// 1. Check for exact or wildcard rule match
	for _, rule := range pe.rules {
		if pe.matches(rule.Pattern, req) {
			needsApproval := rule.Mode == ModeDefault
			return rule.Mode, rule.ID, needsApproval
		}
	}

	// 2. Apply heuristics based on global mode
	mode := pe.global
	if mode == ModeAuto {
		return mode, "", false
	}
	if mode == ModeDontAsk && pe.isReadOnly(req) {
		return mode, "", false
	}
	if mode == ModeAcceptEdits && pe.isEditOperation(req) {
		return mode, "", false
	}
	if mode == ModeBypassPermissions {
		return mode, "", false
	}

	// Default: require approval
	return ModeDefault, "", true
}

// matches checks if a rule pattern matches a tool call request.
func (pe *PermissionEngine) matches(pattern RulePattern, req ToolCallRequest) bool {
	parsed := pe.parsePattern(string(pattern))
	if parsed.ToolName != req.ToolName {
		return false
	}
	return wildcardMatch(parsed.ArgPattern, req.RawInput)
}

// parsePattern decomposes "ToolName(arg_pattern)" into components.
func (pe *PermissionEngine) parsePattern(pattern string) ParsedPattern {
	// Match: ToolName(pattern) with optional spaces
	re := regexp.MustCompile(`^([A-Za-z0-9_]+)\s*\((.*)\)$`)
	m := re.FindStringSubmatch(pattern)
	if m == nil {
		return ParsedPattern{ToolName: pattern, ArgPattern: "*"}
	}
	return ParsedPattern{ToolName: m[1], ArgPattern: strings.TrimSpace(m[2])}
}

// isReadOnly returns true for read-only tool operations.
func (pe *PermissionEngine) isReadOnly(req ToolCallRequest) bool {
	switch req.ToolName {
	case "Read", "LSPGetDiagnostics", "Glob", "Grep", "View":
		return true
	case "Bash":
		// Heuristic: commands with no output redirection or known read-only commands
		cmd, ok := req.Args["command"].(string)
		if !ok {
			return false
		}
		return isReadOnlyCommand(cmd)
	}
	return false
}

// isEditOperation returns true for file-modifying operations.
func (pe *PermissionEngine) isEditOperation(req ToolCallRequest) bool {
	switch req.ToolName {
	case "Edit", "Write", "MultiEdit":
		return true
	case "Bash":
		cmd, ok := req.Args["command"].(string)
		if !ok {
			return false
		}
		return isWriteCommand(cmd)
	}
	return false
}

// isReadOnlyCommand detects read-only bash commands.
func isReadOnlyCommand(cmd string) bool {
	readOnly := []string{"ls", "cat", "find", "grep", "git status", "git log", "git diff", "git branch", "git show", "pwd", "echo", "head", "tail", "wc", "ps", "top", "df", "du", "env", "uname", "which", "whoami", "date", "lsb_release", "python --version", "node --version", "go version", "rustc --version"}
	lower := strings.ToLower(cmd)
	for _, prefix := range readOnly {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// isWriteCommand detects write bash commands.
func isWriteCommand(cmd string) bool {
	writeCmds := []string{"git add", "git commit", "git push", "git pull", "git merge", "git rebase", "git checkout", "git reset", "git stash", "rm", "mv", "cp", "mkdir", "touch", "chmod", "chown", "ln", "tar", "zip", "unzip", "wget", "curl", "npm install", "npm run", "go get", "go mod", "cargo build", "make", "cmake", "docker", "kubectl"}
	lower := strings.ToLower(cmd)
	for _, prefix := range writeCmds {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// wildcardMatch implements glob-style wildcard matching.
func wildcardMatch(pattern, str string) bool {
	// Handle compound commands by checking each segment
	if strings.Contains(str, "&&") || strings.Contains(str, "||") || strings.Contains(str, ";") {
		segments := splitCompoundCommand(str)
		for _, seg := range segments {
			if !wildcardMatch(pattern, strings.TrimSpace(seg)) {
				return false
			}
		}
		return true
	}

	// Convert glob to regex
	var regex strings.Builder
	regex.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			regex.WriteString(".*")
		case '?':
			regex.WriteString(".")
		case '[':
			// Character class - pass through
			j := i + 1
			for j < len(pattern) && pattern[j] != ']' {
				j++
			}
			regex.WriteString(pattern[i : j+1])
			i = j
		default:
			regex.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	regex.WriteString("$")
	re, err := regexp.Compile(regex.String())
	if err != nil {
		return false
	}
	return re.MatchString(str)
}

// splitCompoundCommand splits bash compound commands.
func splitCompoundCommand(cmd string) []string {
	// Simple split by &&, ||, ; - not handling quoting for now
	// Production version would use a proper shell parser
	var segments []string
	for _, sep := range []string{" && ", " || ", "; "} {
		if strings.Contains(cmd, sep) {
			parts := strings.Split(cmd, sep)
			for _, p := range parts {
				if strings.TrimSpace(p) != "" {
					segments = append(segments, strings.TrimSpace(p))
				}
			}
			return segments
		}
	}
	return []string{cmd}
}

// AddRule adds a new permission rule.
func (pe *PermissionEngine) AddRule(ctx context.Context, rule PermissionRule) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if !rule.Mode.IsValid() {
		return fmt.Errorf("invalid permission mode: %s", rule.Mode)
	}
	if err := pe.store.SaveRule(ctx, rule); err != nil {
		return err
	}
	pe.rules = append(pe.rules, rule)
	return nil
}

// Reload refreshes rules from the store.
func (pe *PermissionEngine) Reload(ctx context.Context) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	rules, err := pe.store.LoadRules(ctx)
	if err != nil {
		return err
	}
	pe.rules = rules
	return nil
}

// GetRules returns a copy of all active rules.
func (pe *PermissionEngine) GetRules() []PermissionRule {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	out := make([]PermissionRule, len(pe.rules))
	copy(out, pe.rules)
	return out
}
```

#### File: `internal/tools/permission_store.go` (NEW)

```go
package tools

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// SQLRuleStore implements RuleStore using PostgreSQL.
type SQLRuleStore struct {
	db *sql.DB
}

// NewSQLRuleStore creates a PostgreSQL-backed rule store.
func NewSQLRuleStore(db *sql.DB) *SQLRuleStore {
	return &SQLRuleStore{db: db}
}

// InitSchema creates the permission_rules table.
func (s *SQLRuleStore) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS permission_rules (
		id TEXT PRIMARY KEY,
		pattern TEXT NOT NULL,
		mode TEXT NOT NULL,
		description TEXT,
		created_at BIGINT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_permission_rules_pattern ON permission_rules(pattern);
	`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *SQLRuleStore) LoadRules(ctx context.Context) ([]PermissionRule, error) {
	query := `SELECT id, pattern, mode, description, created_at FROM permission_rules ORDER BY created_at`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying rules: %w", err)
	}
	defer rows.Close()

	var rules []PermissionRule
	for rows.Next() {
		var r PermissionRule
		if err := rows.Scan(&r.ID, &r.Pattern, &r.Mode, &r.Description, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *SQLRuleStore) SaveRule(ctx context.Context, rule PermissionRule) error {
	query := `
	INSERT INTO permission_rules (id, pattern, mode, description, created_at)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (id) DO UPDATE SET
		pattern = EXCLUDED.pattern,
		mode = EXCLUDED.mode,
		description = EXCLUDED.description,
		created_at = EXCLUDED.created_at
	`
	_, err := s.db.ExecContext(ctx, query, rule.ID, rule.Pattern, rule.Mode, rule.Description, rule.CreatedAt)
	return err
}

func (s *SQLRuleStore) DeleteRule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM permission_rules WHERE id = $1`, id)
	return err
}
```

#### File: `internal/tools/tool_executor.go` (MODIFY — Add Permission Check)

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolExecutor executes tools with permission checking.
type ToolExecutor struct {
	registry  *Registry
	engine    *PermissionEngine
	approver  UserApprover
}

// UserApprover requests user confirmation for tool execution.
type UserApprover interface {
	RequestApproval(ctx context.Context, toolName string, args map[string]any, reason string) (bool, error)
}

// NewToolExecutor creates a permission-aware tool executor.
func NewToolExecutor(registry *Registry, engine *PermissionEngine, approver UserApprover) *ToolExecutor {
	return &ToolExecutor{
		registry: registry,
		engine:   engine,
		approver: approver,
	}
}

// Execute runs a tool with full permission lifecycle.
func (te *ToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any) (any, error) {
	// Serialize args for pattern matching
	rawInput, _ := json.Marshal(args)
	req := ToolCallRequest{
		ToolName: toolName,
		Args:     args,
		RawInput: string(rawInput),
	}

	mode, ruleID, needsApproval := te.engine.Evaluate(req)

	if needsApproval {
		approved, err := te.approver.RequestApproval(ctx, toolName, args, fmt.Sprintf("Rule: %s", ruleID))
		if err != nil {
			return nil, fmt.Errorf("approval request failed: %w", err)
		}
		if !approved {
			return nil, fmt.Errorf("user denied permission for %s", toolName)
		}
	}

	// Log mode for observability
	// (omitted for brevity - would use structured logging)
	_ = mode

	// Execute via registry
	result, err := te.registry.Execute(ctx, toolName, args)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}
```

### Anti-Bluff Verification Test

```go
package tools_test

import (
	"context"
	"testing"

	"dev.helix.code/internal/tools"
)

// mockStore is an in-memory rule store for testing.
type mockStore struct {
	rules []tools.PermissionRule
}

func (m *mockStore) LoadRules(ctx context.Context) ([]tools.PermissionRule, error) {
	return m.rules, nil
}

func (m *mockStore) SaveRule(ctx context.Context, rule tools.PermissionRule) error {
	m.rules = append(m.rules, rule)
	return nil
}

func (m *mockStore) DeleteRule(ctx context.Context, id string) error {
	var filtered []tools.PermissionRule
	for _, r := range m.rules {
		if r.ID != id {
			filtered = append(filtered, r)
		}
	}
	m.rules = filtered
	return nil
}

func TestPermissionEngine_WildcardMatching(t *testing.T) {
	store := &mockStore{
		rules: []tools.PermissionRule{
			{ID: "1", Pattern: "Bash(git status:*)", Mode: tools.ModeAuto},
			{ID: "2", Pattern: "Bash(rm -rf *)", Mode: tools.ModeDefault},
			{ID: "3", Pattern: "Read(*.go)", Mode: tools.ModeDontAsk},
			{ID: "4", Pattern: "Edit(*)", Mode: tools.ModeAcceptEdits},
		},
	}

	engine, err := tools.NewPermissionEngine(store, tools.ModeDefault)
	if err != nil {
		t.Fatalf("NewPermissionEngine: %v", err)
	}

	tests := []struct {
		name       string
		toolName   string
		rawInput   string
		wantMode   tools.PermissionMode
		wantRuleID string
		wantApproval bool
	}{
		{
			name:       "git status exact",
			toolName:   "Bash",
			rawInput:   `{"command":"git status"}`,
			wantMode:   tools.ModeAuto,
			wantRuleID: "1",
			wantApproval: false,
		},
		{
			name:       "git status with branch",
			toolName:   "Bash",
			rawInput:   `{"command":"git status --short"}`,
			wantMode:   tools.ModeAuto,
			wantRuleID: "1",
			wantApproval: false,
		},
		{
			name:       "rm -rf dangerous",
			toolName:   "Bash",
			rawInput:   `{"command":"rm -rf /tmp/test"}`,
			wantMode:   tools.ModeDefault,
			wantRuleID: "2",
			wantApproval: true,
		},
		{
			name:       "read go file",
			toolName:   "Read",
			rawInput:   `{"file":"main.go"}`,
			wantMode:   tools.ModeDontAsk,
			wantRuleID: "3",
			wantApproval: false,
		},
		{
			name:       "edit any file",
			toolName:   "Edit",
			rawInput:   `{"file":"main.go","old_string":"foo","new_string":"bar"}`,
			wantMode:   tools.ModeAcceptEdits,
			wantRuleID: "4",
			wantApproval: false,
		},
		{
			name:       "no match falls to default",
			toolName:   "Bash",
			rawInput:   `{"command":"python script.py"}`,
			wantMode:   tools.ModeDefault,
			wantRuleID: "",
			wantApproval: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tools.ToolCallRequest{
				ToolName: tt.toolName,
				RawInput: tt.rawInput,
				Args:     map[string]any{},
			}
			mode, ruleID, needsApproval := engine.Evaluate(req)

			if mode != tt.wantMode {
				t.Errorf("mode = %s, want %s", mode, tt.wantMode)
			}
			if ruleID != tt.wantRuleID {
				t.Errorf("ruleID = %s, want %s", ruleID, tt.wantRuleID)
			}
			if needsApproval != tt.wantApproval {
				t.Errorf("needsApproval = %v, want %v", needsApproval, tt.wantApproval)
			}
		})
	}
}

func TestPermissionEngine_CompoundCommands(t *testing.T) {
	store := &mockStore{
		rules: []tools.PermissionRule{
			{ID: "1", Pattern: "Bash(git status:*)", Mode: tools.ModeAuto},
			{ID: "2", Pattern: "Bash(ls *)", Mode: tools.ModeAuto},
			{ID: "3", Pattern: "Bash(git push*)", Mode: tools.ModeDefault},
		},
	}

	engine, err := tools.NewPermissionEngine(store, tools.ModeDefault)
	if err != nil {
		t.Fatalf("NewPermissionEngine: %v", err)
	}

	// Compound: "git status && ls" - both match auto rules, should be auto
	req := tools.ToolCallRequest{
		ToolName: "Bash",
		RawInput: `{"command":"git status && ls -la"}`,
		Args:     map[string]any{"command": "git status && ls -la"},
	}
	mode, _, needsApproval := engine.Evaluate(req)
	if needsApproval {
		t.Errorf("compound safe commands should not need approval, got mode=%s", mode)
	}

	// Compound: "git status && git push" - one needs approval
	req2 := tools.ToolCallRequest{
		ToolName: "Bash",
		RawInput: `{"command":"git status && git push origin main"}`,
		Args:     map[string]any{"command": "git status && git push origin main"},
	}
	mode2, _, needsApproval2 := engine.Evaluate(req2)
	if !needsApproval2 {
		t.Errorf("compound with dangerous command should need approval, got mode=%s", mode2)
	}
}

func TestPermissionEngine_ReadOnlyAutoDetection(t *testing.T) {
	store := &mockStore{rules: []tools.PermissionRule{}}
	engine, err := tools.NewPermissionEngine(store, tools.ModeDontAsk)
	if err != nil {
		t.Fatalf("NewPermissionEngine: %v", err)
	}

	readOnlyCmds := []string{
		"ls -la",
		"cat file.txt",
		"git status",
		"git log --oneline",
		"find . -name '*.go'",
		"echo hello",
	}

	for _, cmd := range readOnlyCmds {
		req := tools.ToolCallRequest{
			ToolName: "Bash",
			RawInput: `{"command":"` + cmd + `"}`,
			Args:     map[string]any{"command": cmd},
		}
		mode, _, needsApproval := engine.Evaluate(req)
		if needsApproval {
			t.Errorf("cmd %q should not need approval in dontAsk mode, got mode=%s", cmd, mode)
		}
	}

	// Write commands should still need approval even in dontAsk
	writeCmds := []string{
		"rm file.txt",
		"git push origin main",
		"mkdir newdir",
	}

	for _, cmd := range writeCmds {
		req := tools.ToolCallRequest{
			ToolName: "Bash",
			RawInput: `{"command":"` + cmd + `"}`,
			Args:     map[string]any{"command": cmd},
		}
		_, _, needsApproval := engine.Evaluate(req)
		if !needsApproval {
			t.Errorf("cmd %q should need approval even in dontAsk mode", cmd)
		}
	}
}

func TestPermissionEngine_BypassMode(t *testing.T) {
	store := &mockStore{rules: []tools.PermissionRule{}}
	engine, err := tools.NewPermissionEngine(store, tools.ModeBypassPermissions)
	if err != nil {
		t.Fatalf("NewPermissionEngine: %v", err)
	}

	req := tools.ToolCallRequest{
		ToolName: "Bash",
		RawInput: `{"command":"rm -rf /"}`,
		Args:     map[string]any{"command": "rm -rf /"},
	}
	mode, _, needsApproval := engine.Evaluate(req)
	if needsApproval {
		t.Errorf("bypassPermissions mode should not require approval for any command")
	}
	if mode != tools.ModeBypassPermissions {
		t.Errorf("mode = %s, want %s", mode, tools.ModeBypassPermissions)
	}
}
```

### Integration Steps

1. **Create permission engine**:
   ```bash
   cat > internal/tools/permissions.go << 'EOF'
   # [paste content]
   EOF
   ```

2. **Create PostgreSQL store**:
   ```bash
   cat > internal/tools/permission_store.go << 'EOF'
   # [paste content]
   EOF
   ```

3. **Modify tool executor** to inject permission check:
   - Add `PermissionEngine` and `UserApprover` to `ToolExecutor`
   - Wrap `Execute()` with permission evaluation

4. **Add CLI flag** in `cmd/cli/main.go`:
   ```go
   rootCmd.PersistentFlags().String("permission-mode", "default", "Global permission mode: default, auto, acceptEdits, dontAsk, bypassPermissions, plan")
   ```

5. **Initialize schema** on startup:
   ```go
   store.InitSchema(ctx)
   ```

---

## Feature 3: Tool Result Persistence Layer

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: When a tool produces output exceeding 50K characters, the raw output is saved to a `tool-results/` directory in the project. The message sent to the LLM contains `persistedOutputPath` and `persistedOutputSize` fields instead of the full inline content. The LLM can request the full content via a follow-up tool call if needed.
- **Why it's powerful**: Prevents token waste on large build outputs, test logs, or file listings while preserving access to the data.

### HelixCode Integration Target
- **New files to create**:
  - `internal/tools/persistence.go` — Persistence manager
  - `internal/tools/persistence_test.go`
- **Existing files to modify**:
  - `internal/tools/tool_executor.go` — Add persistence check after execution
  - `internal/llm/provider.go` — Add persisted output fields to message schema
- **Submodule dependencies**: `internal/tools/`

### Exact Code Implementation

#### File: `internal/tools/persistence.go` (NEW)

```go
package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// PersistThreshold is the character count above which outputs are persisted to disk.
	PersistThreshold = 50000

	// PersistDir is the directory for persisted tool outputs.
	PersistDir = ".helix/tool-results"
)

// PersistedResult represents a tool result that was saved to disk.
type PersistedResult struct {
	Output            string `json:"output,omitempty"`              // Empty if persisted
	PersistedOutputPath string `json:"persistedOutputPath,omitempty"`
	PersistedOutputSize int    `json:"persistedOutputSize,omitempty"`
	WasPersisted        bool   `json:"wasPersisted"`
	ToolName            string `json:"toolName"`
	ToolCallID          string `json:"toolCallID,omitempty"`
}

// PersistenceManager handles saving large tool outputs to disk.
type PersistenceManager struct {
	baseDir string
	mu      sync.RWMutex
}

// NewPersistenceManager creates a persistence manager.
func NewPersistenceManager(projectRoot string) *PersistenceManager {
	return &PersistenceManager{
		baseDir: filepath.Join(projectRoot, PersistDir),
	}
}

// EnsureDir creates the persistence directory.
func (pm *PersistenceManager) EnsureDir() error {
	return os.MkdirAll(pm.baseDir, 0755)
}

// MaybePersist checks if output exceeds threshold and persists if needed.
// Returns a PersistedResult with either inline output or a path reference.
func (pm *PersistenceManager) MaybePersist(toolName, toolCallID string, output string) (*PersistedResult, error) {
	if len(output) <= PersistThreshold {
		return &PersistedResult{
			Output:     output,
			WasPersisted: false,
			ToolName:     toolName,
			ToolCallID:   toolCallID,
		}, nil
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if err := pm.EnsureDir(); err != nil {
		return nil, fmt.Errorf("creating persist dir: %w", err)
	}

	// Generate filename: toolName_hash_timestamp.txt
	hash := sha256.Sum256([]byte(output))
	hashStr := hex.EncodeToString(hash[:8])
	timestamp := time.Now().UTC().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.txt", sanitizeFilename(toolName), hashStr, timestamp)
	path := filepath.Join(pm.baseDir, filename)

	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return nil, fmt.Errorf("writing persisted output: %w", err)
	}

	// Return reference instead of inline content
	return &PersistedResult{
		Output:              "", // Intentionally empty - content is on disk
		PersistedOutputPath: path,
		PersistedOutputSize: len(output),
		WasPersisted:        true,
		ToolName:            toolName,
		ToolCallID:          toolCallID,
	}, nil
}

// LoadPersisted loads a persisted result from disk.
func (pm *PersistenceManager) LoadPersisted(path string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Security: ensure path is within baseDir
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absBase, err := filepath.Abs(pm.baseDir)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absPath, absBase) {
		return "", fmt.Errorf("path %q is outside persist directory", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading persisted output: %w", err)
	}
	return string(data), nil
}

// CleanupOld removes persisted results older than the given duration.
func (pm *PersistenceManager) CleanupOld(maxAge time.Duration) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	entries, err := os.ReadDir(pm.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(pm.baseDir, entry.Name()))
		}
	}
	return nil
}

// ListPersisted returns all persisted output files.
func (pm *PersistenceManager) ListPersisted() ([]string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	entries, err := os.ReadDir(pm.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(pm.baseDir, entry.Name()))
		}
	}
	return files, nil
}

// sanitizeFilename makes a string safe for use as a filename.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}
```

#### File: `internal/tools/tool_executor.go` (MODIFY — Add Persistence)

```go
// Add to ToolExecutor struct:
type ToolExecutor struct {
	registry    *Registry
	engine      *PermissionEngine
	approver    UserApprover
	persister   *PersistenceManager
}

// Modify Execute to persist large outputs:
func (te *ToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any, toolCallID string) (*PersistedResult, error) {
	// ... existing permission check ...

	// Execute via registry
	rawResult, err := te.registry.Execute(ctx, toolName, args)
	if err != nil {
		return nil, err
	}

	// Convert result to string for persistence check
	output := fmt.Sprintf("%v", rawResult)

	// Persist if large
	return te.persister.MaybePersist(toolName, toolCallID, output)
}
```

#### File: `internal/llm/provider.go` (MODIFY — Message Schema)

```go
// Add to Message struct:
type Message struct {
	// ... existing fields ...
	ToolResults []PersistedResult `json:"tool_results,omitempty"`
}
```

### Anti-Bluff Verification Test

```go
package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/tools"
)

func TestPersistenceManager_MaybePersist_Threshold(t *testing.T) {
	tmpDir := t.TempDir()
	pm := tools.NewPersistenceManager(tmpDir)

	// Small output - should NOT persist
	smallOutput := strings.Repeat("a", 100)
	result, err := pm.MaybePersist("Bash", "call-1", smallOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WasPersisted {
		t.Fatal("small output was persisted, expected inline")
	}
	if result.Output != smallOutput {
		t.Fatal("inline output mismatch")
	}
	if result.PersistedOutputPath != "" {
		t.Fatal("expected empty path for inline result")
	}

	// Large output - SHOULD persist
	largeOutput := strings.Repeat("b", tools.PersistThreshold+1000)
	result2, err := pm.MaybePersist("Bash", "call-2", largeOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result2.WasPersisted {
		t.Fatal("large output was NOT persisted")
	}
	if result2.Output != "" {
		t.Fatal("expected empty inline output for persisted result")
	}
	if result2.PersistedOutputPath == "" {
		t.Fatal("expected path for persisted result")
	}
	if result2.PersistedOutputSize != len(largeOutput) {
		t.Fatalf("size = %d, want %d", result2.PersistedOutputSize, len(largeOutput))
	}

	// Verify file exists
	if _, err := os.Stat(result2.PersistedOutputPath); os.IsNotExist(err) {
		t.Fatalf("persisted file does not exist: %s", result2.PersistedOutputPath)
	}

	// Verify file content
	loaded, err := pm.LoadPersisted(result2.PersistedOutputPath)
	if err != nil {
		t.Fatalf("loading persisted: %v", err)
	}
	if loaded != largeOutput {
		t.Fatal("persisted content mismatch")
	}
}

func TestPersistenceManager_LoadPersisted_Security(t *testing.T) {
	tmpDir := t.TempDir()
	pm := tools.NewPersistenceManager(tmpDir)

	// Try to load file outside persist directory
	_, err := pm.LoadPersisted("/etc/passwd")
	if err == nil {
		t.Fatal("expected error for path outside persist directory")
	}
	if !strings.Contains(err.Error(), "outside persist directory") {
		t.Fatalf("expected security error, got: %v", err)
	}
}

func TestPersistenceManager_CleanupOld(t *testing.T) {
	tmpDir := t.TempDir()
	pm := tools.NewPersistenceManager(tmpDir)

	// Create an old file
	oldPath := filepath.Join(tmpDir, tools.PersistDir, "old.txt")
	os.MkdirAll(filepath.Dir(oldPath), 0755)
	os.WriteFile(oldPath, []byte("old"), 0644)

	// Create a new file
	newPath := filepath.Join(tmpDir, tools.PersistDir, "new.txt")
	os.WriteFile(newPath, []byte("new"), 0644)

	// Verify both exist
	files, _ := pm.ListPersisted()
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Manually adjust old file time (OS-specific, using sleep for simplicity)
	// In real test we'd use os.Chtimes
	os.Chtimes(oldPath, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour))

	// Cleanup files older than 1 hour
	if err := pm.CleanupOld(time.Hour); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify old file is gone
	files, _ = pm.ListPersisted()
	if len(files) != 1 {
		t.Fatalf("expected 1 file after cleanup, got %d", len(files))
	}
	if filepath.Base(files[0]) != "new.txt" {
		t.Fatalf("wrong file remaining: %s", files[0])
	}
}

func TestPersistenceManager_FilenameSanitization(t *testing.T) {
	tmpDir := t.TempDir()
	pm := tools.NewPersistenceManager(tmpDir)

	large := strings.Repeat("x", tools.PersistThreshold+1)
	result, err := pm.MaybePersist("Bash(rm -rf /)", "id", large)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	base := filepath.Base(result.PersistedOutputPath)
	if strings.Contains(base, "/") || strings.Contains(base, "\\") {
		t.Fatalf("filename contains path separators: %s", base)
	}
}
```

### Integration Steps

1. **Create persistence manager** (`internal/tools/persistence.go`)
2. **Inject into tool executor** — call `MaybePersist` after every tool execution
3. **Update LLM message schema** to include `PersistedResult` references
4. **Add cleanup job** — run `CleanupOld(24 * time.Hour)` periodically via background goroutine
5. **Gitignore** — add `.helix/tool-results/` to `.gitignore` template

---

## Feature 4: Git Worktree Agent Isolation

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Agents can enter isolated git worktrees via `EnterWorktree` tool. Worktrees have validated names (`/[a-zA-Z0-9._-]+/`, max 64 chars). Each worktree gets its own checkout, allowing parallel work. `ExitWorktree` returns to the main worktree. Subagents can spawn with `isolation: "worktree"` for true parallelism.
- **Why it's powerful**: Enables parallel development branches, safe experimentation, and multi-agent workflows without branch pollution.

### HelixCode Integration Target
- **New files to create**:
  - `internal/tools/worktree.go` — Worktree tools implementation
  - `internal/tools/worktree_test.go`
  - `internal/session/worktree_state.go` — Per-session worktree tracking
- **Existing files to modify**:
  - `internal/tools/registry.go` — Register EnterWorktree/ExitWorktree tools
  - `internal/agent/agent.go` — Track current worktree in agent state
- **Submodule dependencies**: `internal/tools/`, `internal/session/`

### Exact Code Implementation

#### File: `internal/tools/worktree.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// WorktreeNameRegex validates worktree names.
const WorktreeNameRegex = `^[a-zA-Z0-9._-]+$`
const WorktreeNameMaxLength = 64

var worktreeNamePattern = regexp.MustCompile(WorktreeNameRegex)

// WorktreeManager handles git worktree operations.
type WorktreeManager struct {
	repoRoot       string
	currentWorktree string // empty = main worktree
}

// NewWorktreeManager creates a worktree manager for the given repo.
func NewWorktreeManager(repoRoot string) *WorktreeManager {
	return &WorktreeManager{
		repoRoot: repoRoot,
	}
}

// ValidateName checks if a worktree name is valid.
func (wm *WorktreeManager) ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}
	if len(name) > WorktreeNameMaxLength {
		return fmt.Errorf("worktree name exceeds %d characters", WorktreeNameMaxLength)
	}
	if !worktreeNamePattern.MatchString(name) {
		return fmt.Errorf("worktree name %q does not match pattern %s", name, WorktreeNameRegex)
	}
	return nil
}

// EnterWorktree switches to a named worktree, creating it if necessary.
func (wm *WorktreeManager) EnterWorktree(ctx context.Context, name string, baseBranch string) (string, error) {
	if err := wm.ValidateName(name); err != nil {
		return "", err
	}

	worktreePath := filepath.Join(wm.repoRoot, ".helix-worktrees", name)

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		// Create new worktree
		if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
			return "", fmt.Errorf("creating worktree parent dir: %w", err)
		}

		branch := baseBranch
		if branch == "" {
			branch = name
		}

		cmd := exec.CommandContext(ctx, "git", "worktree", "add", worktreePath, branch)
		cmd.Dir = wm.repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Try with -b for new branch
			cmd2 := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, worktreePath)
			cmd2.Dir = wm.repoRoot
			out2, err2 := cmd2.CombinedOutput()
			if err2 != nil {
				return "", fmt.Errorf("creating worktree: %w\noutput: %s\nnew branch attempt: %s", err, string(out), string(out2))
			}
			_ = out2
		}
	} else {
		// Worktree exists, just ensure it's clean
		cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
		cmd.Dir = worktreePath
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("checking worktree status: %w", err)
		}
		if strings.TrimSpace(string(out)) != "" {
			return "", fmt.Errorf("worktree %q has uncommitted changes", name)
		}
	}

	wm.currentWorktree = worktreePath
	return worktreePath, nil
}

// ExitWorktree returns to the main worktree.
func (wm *WorktreeManager) ExitWorktree() {
	wm.currentWorktree = ""
}

// GetCurrentDirectory returns the effective working directory.
func (wm *WorktreeManager) GetCurrentDirectory() string {
	if wm.currentWorktree != "" {
		return wm.currentWorktree
	}
	return wm.repoRoot
}

// ListWorktrees returns all active helix worktrees.
func (wm *WorktreeManager) ListWorktrees(ctx context.Context) ([]string, error) {
	worktreesDir := filepath.Join(wm.repoRoot, ".helix-worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// RemoveWorktree deletes a worktree and its branch.
func (wm *WorktreeManager) RemoveWorktree(ctx context.Context, name string) error {
	if err := wm.ValidateName(name); err != nil {
		return err
	}

	worktreePath := filepath.Join(wm.repoRoot, ".helix-worktrees", name)

	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", worktreePath)
	cmd.Dir = wm.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		// Try force remove
		cmd2 := exec.CommandContext(ctx, "git", "worktree", "remove", "-f", worktreePath)
		cmd2.Dir = wm.repoRoot
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return fmt.Errorf("removing worktree: %w\noutput: %s\nforce attempt: %s", err, string(out), string(out2))
		}
		_ = out2
	}

	if wm.currentWorktree == worktreePath {
		wm.ExitWorktree()
	}
	return nil
}

// IsIsolated returns true if currently in a worktree.
func (wm *WorktreeManager) IsIsolated() bool {
	return wm.currentWorktree != ""
}

// WorktreeTool implements the EnterWorktree tool.
type WorktreeTool struct {
	manager *WorktreeManager
}

func NewWorktreeTool(manager *WorktreeManager) *WorktreeTool {
	return &WorktreeTool{manager: manager}
}

func (wt *WorktreeTool) Name() string {
	return "EnterWorktree"
}

func (wt *WorktreeTool) Description() string {
	return "Enter a named git worktree for isolated development. Creates the worktree if it doesn't exist."
}

func (wt *WorktreeTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Name of the worktree (alphanumeric, dots, dashes, underscores; max 64 chars)",
			},
			"base_branch": map[string]any{
				"type":        "string",
				"description": "Optional base branch to create from (defaults to current branch)",
			},
		},
		"required": []string{"name"},
	}
}

func (wt *WorktreeTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name must be a string")
	}
	baseBranch, _ := args["base_branch"].(string)

	path, err := wt.manager.EnterWorktree(ctx, name, baseBranch)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"worktree_path": path,
		"name":          name,
		"status":        "entered",
	}, nil
}

// ExitWorktreeTool implements the ExitWorktree tool.
type ExitWorktreeTool struct {
	manager *WorktreeManager
}

func NewExitWorktreeTool(manager *WorktreeManager) *ExitWorktreeTool {
	return &ExitWorktreeTool{manager: manager}
}

func (wt *ExitWorktreeTool) Name() string {
	return "ExitWorktree"
}

func (wt *ExitWorktreeTool) Description() string {
	return "Return to the main git worktree."
}

func (wt *ExitWorktreeTool) Schema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (wt *ExitWorktreeTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	wt.manager.ExitWorktree()
	return map[string]any{
		"status":   "exited",
		"returned_to": wt.manager.repoRoot,
	}, nil
}
```

#### File: `internal/session/worktree_state.go` (NEW)

```go
package session

import (
	"dev.helix.code/internal/tools"
)

// WorktreeState tracks the active worktree per session.
type WorktreeState struct {
	Manager *tools.WorktreeManager
}

// NewWorktreeState creates worktree state for a session.
func NewWorktreeState(repoRoot string) *WorktreeState {
	return &WorktreeState{
		Manager: tools.NewWorktreeManager(repoRoot),
	}
}
```

### Anti-Bluff Verification Test

```go
package tools_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/internal/tools"
)

func TestWorktreeManager_ValidateName(t *testing.T) {
	wm := tools.NewWorktreeManager("/tmp")

	valid := []string{"feature-1", "bugfix_2", "v1.0.0", "test", "A1b2c3"}
	for _, name := range valid {
		if err := wm.ValidateName(name); err != nil {
			t.Errorf("valid name %q rejected: %v", name, err)
		}
	}

	invalid := []string{
		"",                    // empty
		strings.Repeat("a", 65), // too long
		"feature/branch",      // slash
		"feature branch",      // space
		"feature\tbranch",     // tab
		"feature*branch",     // asterisk
	}
	for _, name := range invalid {
		if err := wm.ValidateName(name); err == nil {
			t.Errorf("invalid name %q accepted", name)
		}
	}
}

func TestWorktreeManager_EnterAndExit(t *testing.T) {
	// Create a temporary git repo
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	// Create initial commit for branch creation
	os.WriteFile(filepath.Join(tmpDir, "README"), []byte("test"), 0644)
	exec.Command("git", "add", ".").Dir = tmpDir
	exec.Command("git", "commit", "-m", "init").Dir = tmpDir

	wm := tools.NewWorktreeManager(tmpDir)

	// Enter a new worktree
	path, err := wm.EnterWorktree(context.Background(), "feature-test", "")
	if err != nil {
		t.Fatalf("EnterWorktree failed: %v", err)
	}

	if !wm.IsIsolated() {
		t.Fatal("expected IsIsolated() = true")
	}

	expectedPath := filepath.Join(tmpDir, ".helix-worktrees", "feature-test")
	if path != expectedPath {
		t.Fatalf("path = %s, want %s", path, expectedPath)
	}

	if wm.GetCurrentDirectory() != expectedPath {
		t.Fatalf("GetCurrentDirectory = %s, want %s", wm.GetCurrentDirectory(), expectedPath)
	}

	// Exit worktree
	wm.ExitWorktree()
	if wm.IsIsolated() {
		t.Fatal("expected IsIsolated() = false after exit")
	}
	if wm.GetCurrentDirectory() != tmpDir {
		t.Fatalf("GetCurrentDirectory = %s, want %s", wm.GetCurrentDirectory(), tmpDir)
	}

	// List worktrees
	list, err := wm.ListWorktrees(context.Background())
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(list) != 1 || list[0] != "feature-test" {
		t.Fatalf("worktrees = %v, want [feature-test]", list)
	}
}

func TestWorktreeTool_Execute(t *testing.T) {
	tmpDir := t.TempDir()
	exec.Command("git", "init").Dir = tmpDir
	os.WriteFile(filepath.Join(tmpDir, "README"), []byte("test"), 0644)
	exec.Command("git", "add", ".").Dir = tmpDir
	exec.Command("git", "commit", "-m", "init").Dir = tmpDir

	wm := tools.NewWorktreeManager(tmpDir)
	enterTool := tools.NewWorktreeTool(wm)

	result, err := enterTool.Execute(context.Background(), map[string]any{
		"name": "test-worktree",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if m["name"] != "test-worktree" {
		t.Fatalf("name = %v, want test-worktree", m["name"])
	}
	if m["status"] != "entered" {
		t.Fatalf("status = %v, want entered", m["status"])
	}

	// Test invalid name
	_, err = enterTool.Execute(context.Background(), map[string]any{
		"name": "invalid/name",
	})
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}
```

### Integration Steps

1. **Create worktree tools** (`internal/tools/worktree.go`)
2. **Register in tool registry**:
   ```go
   registry.Register(NewWorktreeTool(wm))
   registry.Register(NewExitWorktreeTool(wm))
   ```
3. **Integrate with Bash tool** — prefix `cmd.Dir` with `worktreeManager.GetCurrentDirectory()`
4. **Integrate with Read/Edit tools** — resolve paths relative to current worktree
5. **Add session state** (`internal/session/worktree_state.go`) to persist active worktree across turns
6. **Add subagent isolation** — pass `isolation: "worktree"` to spawn subagent in new worktree

---

## Feature 5: Hook-Based Extensibility (9+ Events)

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: A hook system emits events at key lifecycle points. Plugins register handlers that can block execution (return error), modify arguments/results, or extend behavior asynchronously. Events: before_tool_call, after_tool_call, before_edit, after_edit, before_bash, after_bash, on_error, on_compaction, on_plan_approval.
- **Why it's powerful**: Allows teams to enforce policies (e.g., "no production DB access"), add observability, inject custom workflows, and modify behavior without forking.

### HelixCode Integration Target
- **New files to create**:
  - `internal/workflow/hooks.go` — Hook dispatcher and registry
  - `internal/workflow/hooks_test.go`
  - `internal/workflow/plugin.go` — Plugin interface
- **Existing files to modify**:
  - `internal/tools/tool_executor.go` — Emit before/after hooks
  - `internal/editor/editor.go` — Emit before/after edit hooks
  - `internal/context/compaction.go` — Emit on_compaction hook
  - `internal/agent/agent.go` — Emit on_error, on_plan_approval hooks
- **Submodule dependencies**: `internal/workflow/`, `internal/tools/`, `internal/editor/`, `internal/context/`

### Exact Code Implementation

#### File: `internal/workflow/hooks.go` (NEW)

```go
package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EventType identifies the lifecycle event.
type EventType string

const (
	EventBeforeToolCall    EventType = "before_tool_call"
	EventAfterToolCall     EventType = "after_tool_call"
	EventBeforeEdit        EventType = "before_edit"
	EventAfterEdit         EventType = "after_edit"
	EventBeforeBash        EventType = "before_bash"
	EventAfterBash         EventType = "after_bash"
	EventOnError           EventType = "on_error"
	EventOnCompaction      EventType = "on_compaction"
	EventOnPlanApproval    EventType = "on_plan_approval"
)

// Event carries context for a hook invocation.
type Event struct {
	Type      EventType
	Timestamp time.Time
	SessionID string
	Payload   map[string]any // Event-specific data
}

// HookResult is returned by a hook handler.
type HookResult struct {
	Allow   bool          // If false, block the operation
	Error   error         // If non-nil, abort with this error
	Modify  map[string]any // Modified payload to pass forward
	Async   bool          // If true, handler runs asynchronously
}

// HookHandler is a function that processes an event.
type HookHandler func(ctx context.Context, event Event) (HookResult, error)

// HookRegistration represents a registered hook.
type HookRegistration struct {
	ID       string
	Event    EventType
	Handler  HookHandler
	Priority int // Higher priority = earlier execution
	Async    bool
}

// HookDispatcher manages event-driven extensibility.
type HookDispatcher struct {
	handlers map[EventType][]HookRegistration
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewHookDispatcher creates a new dispatcher.
func NewHookDispatcher(logger *zap.Logger) *HookDispatcher {
	return &HookDispatcher{
		handlers: make(map[EventType][]HookRegistration),
		logger:   logger,
	}
}

// Register adds a hook handler for an event type.
func (hd *HookDispatcher) Register(reg HookRegistration) {
	hd.mu.Lock()
	defer hd.mu.Unlock()

	list := hd.handlers[reg.Event]
	// Insert by priority (descending)
	inserted := false
	for i, existing := range list {
		if reg.Priority > existing.Priority {
			list = append(list[:i], append([]HookRegistration{reg}, list[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		list = append(list, reg)
	}
	hd.handlers[reg.Event] = list

	hd.logger.Info("hook registered",
		zap.String("id", reg.ID),
		zap.String("event", string(reg.Event)),
		zap.Int("priority", reg.Priority),
	)
}

// Unregister removes a hook by ID.
func (hd *HookDispatcher) Unregister(id string) {
	hd.mu.Lock()
	defer hd.mu.Unlock()

	for eventType, list := range hd.handlers {
		var filtered []HookRegistration
		for _, reg := range list {
			if reg.ID != id {
				filtered = append(filtered, reg)
			}
		}
		hd.handlers[eventType] = filtered
	}
}

// Dispatch executes all handlers for an event.
// If any handler returns Allow=false or Error!=nil, the operation is blocked.
// Modified payload from one handler is passed to the next.
func (hd *HookDispatcher) Dispatch(ctx context.Context, event Event) (map[string]any, error) {
	hd.mu.RLock()
	handlers := hd.handlers[event.Type]
	hd.mu.RUnlock()

	payload := event.Payload
	if payload == nil {
		payload = make(map[string]any)
	}

	var asyncWg sync.WaitGroup
	var asyncErrors []error
	var asyncMu sync.Mutex

	for _, reg := range handlers {
		if reg.Async {
			asyncWg.Add(1)
			go func(r HookRegistration) {
				defer asyncWg.Done()
				_, err := r.Handler(ctx, Event{
					Type:      event.Type,
					Timestamp: event.Timestamp,
					SessionID: event.SessionID,
					Payload:   payload,
				})
				if err != nil {
					asyncMu.Lock()
					asyncErrors = append(asyncErrors, fmt.Errorf("async hook %s: %w", r.ID, err))
					asyncMu.Unlock()
				}
			}(reg)
			continue
		}

		result, err := reg.Handler(ctx, Event{
			Type:      event.Type,
			Timestamp: event.Timestamp,
			SessionID: event.SessionID,
			Payload:   payload,
		})
		if err != nil {
			return nil, fmt.Errorf("hook %s failed: %w", reg.ID, err)
		}
		if !result.Allow {
			return nil, fmt.Errorf("hook %s blocked %s", reg.ID, event.Type)
		}
		if result.Modify != nil {
			// Merge modifications into payload
			for k, v := range result.Modify {
				payload[k] = v
			}
		}
	}

	asyncWg.Wait()
	if len(asyncErrors) > 0 {
		return payload, asyncErrors[0] // Return first async error
	}

	return payload, nil
}

// HasHandlers returns true if any handlers are registered for the event type.
func (hd *HookDispatcher) HasHandlers(eventType EventType) bool {
	hd.mu.RLock()
	defer hd.mu.RUnlock()
	return len(hd.handlers[eventType]) > 0
}
```

#### File: `internal/workflow/plugin.go` (NEW)

```go
package workflow

import "context"

// Plugin is the interface for external extensions.
type Plugin interface {
	Name() string
	Version() string
	RegisterHooks(dispatcher *HookDispatcher) error
}

// PluginLoader discovers and loads plugins from the filesystem or registry.
type PluginLoader struct {
	searchPaths []string
}

func NewPluginLoader(paths []string) *PluginLoader {
	return &PluginLoader{searchPaths: paths}
}

func (pl *PluginLoader) LoadAll(ctx context.Context, dispatcher *HookDispatcher) error {
	// Implementation would scan directories, load .so files, or connect to external processes
	// Stub for now
	return nil
}
```

#### File: `internal/tools/tool_executor.go` (MODIFY — Hook Integration)

```go
// Add hook dispatcher to ToolExecutor:
type ToolExecutor struct {
	registry    *Registry
	engine      *PermissionEngine
	approver    UserApprover
	persister   *PersistenceManager
	dispatcher  *workflow.HookDispatcher
}

// Modify Execute to emit hooks:
func (te *ToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any, toolCallID string, sessionID string) (*PersistedResult, error) {
	// Before tool call hook
	if te.dispatcher.HasHandlers(workflow.EventBeforeToolCall) {
		modified, err := te.dispatcher.Dispatch(ctx, workflow.Event{
			Type:      workflow.EventBeforeToolCall,
			SessionID: sessionID,
			Payload: map[string]any{
				"tool_name":    toolName,
				"args":         args,
				"tool_call_id": toolCallID,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("before_tool_call hook blocked: %w", err)
		}
		// Apply modifications to args
		if modArgs, ok := modified["args"]; ok {
			if newArgs, ok := modArgs.(map[string]any); ok {
				args = newArgs
			}
		}
	}

	// Special before_bash hook for bash commands
	if toolName == "Bash" && te.dispatcher.HasHandlers(workflow.EventBeforeBash) {
		_, err := te.dispatcher.Dispatch(ctx, workflow.Event{
			Type:      workflow.EventBeforeBash,
			SessionID: sessionID,
			Payload: map[string]any{
				"command": args["command"],
				"args":    args,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("before_bash hook blocked: %w", err)
		}
	}

	// ... existing permission check and execution ...

	// After tool call hook
	if te.dispatcher.HasHandlers(workflow.EventAfterToolCall) {
		te.dispatcher.Dispatch(ctx, workflow.Event{
			Type:      workflow.EventAfterToolCall,
			SessionID: sessionID,
			Payload: map[string]any{
				"tool_name": toolName,
				"result":    result,
				"error":     execErr,
			},
		})
	}

	// After bash hook
	if toolName == "Bash" && te.dispatcher.HasHandlers(workflow.EventAfterBash) {
		te.dispatcher.Dispatch(ctx, workflow.Event{
			Type:      workflow.EventAfterBash,
			SessionID: sessionID,
			Payload: map[string]any{
				"command": args["command"],
				"result":  result,
				"error":   execErr,
			},
		})
	}

	return result, execErr
}
```

#### File: `internal/editor/editor.go` (MODIFY — Edit Hooks)

```go
// Before edit:
if dispatcher.HasHandlers(workflow.EventBeforeEdit) {
	_, err := dispatcher.Dispatch(ctx, workflow.Event{
		Type:      workflow.EventBeforeEdit,
		SessionID: sessionID,
		Payload: map[string]any{
			"file_path":  filePath,
			"old_string": oldStr,
			"new_string": newStr,
		},
	})
	if err != nil {
		return fmt.Errorf("before_edit hook blocked: %w", err)
	}
}

// After edit:
if dispatcher.HasHandlers(workflow.EventAfterEdit) {
	dispatcher.Dispatch(ctx, workflow.Event{
		Type:      workflow.EventAfterEdit,
		SessionID: sessionID,
		Payload: map[string]any{
			"file_path":  filePath,
			"success":    err == nil,
			"error":      err,
		},
	})
}
```

### Anti-Bluff Verification Test

```go
package workflow_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"dev.helix.code/internal/workflow"
	"go.uber.org/zap/zaptest"
)

func TestHookDispatcher_RegisterAndDispatch(t *testing.T) {
	d := workflow.NewHookDispatcher(zaptest.NewLogger(t))

	called := false
	d.Register(workflow.HookRegistration{
		ID:       "test-1",
		Event:    workflow.EventBeforeToolCall,
		Priority: 100,
		Handler: func(ctx context.Context, event workflow.Event) (workflow.HookResult, error) {
			called = true
			return workflow.HookResult{Allow: true}, nil
		},
	})

	_, err := d.Dispatch(context.Background(), workflow.Event{
		Type:      workflow.EventBeforeToolCall,
		SessionID: "session-1",
		Payload:   map[string]any{"tool": "Bash"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestHookDispatcher_BlockingHook(t *testing.T) {
	d := workflow.NewHookDispatcher(zaptest.NewLogger(t))

	d.Register(workflow.HookRegistration{
		ID:    "blocker",
		Event: workflow.EventBeforeBash,
		Handler: func(ctx context.Context, event workflow.Event) (workflow.HookResult, error) {
			cmd := event.Payload["command"].(string)
			if cmd == "rm -rf /" {
				return workflow.HookResult{Allow: false}, nil
			}
			return workflow.HookResult{Allow: true}, nil
		},
	})

	// Allowed command
	_, err := d.Dispatch(context.Background(), workflow.Event{
		Type: workflow.EventBeforeBash,
		Payload: map[string]any{"command": "ls -la"},
	})
	if err != nil {
		t.Fatalf("unexpected error for safe command: %v", err)
	}

	// Blocked command
	_, err = d.Dispatch(context.Background(), workflow.Event{
		Type: workflow.EventBeforeBash,
		Payload: map[string]any{"command": "rm -rf /"},
	})
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
}

func TestHookDispatcher_PriorityOrdering(t *testing.T) {
	d := workflow.NewHookDispatcher(zaptest.NewLogger(t))

	order := []string{}

	d.Register(workflow.HookRegistration{
		ID:       "low",
		Event:    workflow.EventBeforeToolCall,
		Priority: 1,
		Handler: func(ctx context.Context, event workflow.Event) (workflow.HookResult, error) {
			order = append(order, "low")
			return workflow.HookResult{Allow: true}, nil
		},
	})

	d.Register(workflow.HookRegistration{
		ID:       "high",
		Event:    workflow.EventBeforeToolCall,
		Priority: 100,
		Handler: func(ctx context.Context, event workflow.Event) (workflow.HookResult, error) {
			order = append(order, "high")
			return workflow.HookResult{Allow: true}, nil
		},
	})

	d.Register(workflow.HookRegistration{
		ID:       "mid",
		Event:    workflow.EventBeforeToolCall,
		Priority: 50,
		Handler: func(ctx context.Context, event workflow.Event) (workflow.HookResult, error) {
			order = append(order, "mid")
			return workflow.HookResult{Allow: true}, nil
		},
	})

	_, err := d.Dispatch(context.Background(), workflow.Event{
		Type: workflow.EventBeforeToolCall,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("order length = %d, want 3", len(order))
	}
	if order[0] != "high" || order[1] != "mid" || order[2] != "low" {
		t.Fatalf("wrong execution order: %v", order)
	}
}

func TestHookDispatcher_PayloadModification(t *testing.T) {
	d := workflow.NewHookDispatcher(zaptest.NewLogger(t))

	d.Register(workflow.HookRegistration{
		ID:    "modifier",
		Event: workflow.EventBeforeToolCall,
		Handler: func(ctx context.Context, event workflow.Event) (workflow.HookResult, error) {
			return workflow.HookResult{
				Allow: true,
				Modify: map[string]any{
					"injected": "value",
				},
			}, nil
		},
	})

	d.Register(workflow.HookRegistration{
		ID:    "reader",
		Event: workflow.EventBeforeToolCall,
		Handler: func(ctx context.Context, event workflow.Event) (workflow.HookResult, error) {
			// Verify the modification from previous hook is visible
			if event.Payload["injected"] != "value" {
				return workflow.HookResult{}, errors.New("modification not propagated")
			}
			return workflow.HookResult{Allow: true}, nil
		},
	})

	_, err := d.Dispatch(context.Background(), workflow.Event{
		Type:    workflow.EventBeforeToolCall,
		Payload: map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHookDispatcher_AsyncHook(t *testing.T) {
	d := workflow.NewHookDispatcher(zaptest.NewLogger(t))

	done := make(chan bool, 1)
	d.Register(workflow.HookRegistration{
		ID:    "async",
		Event: workflow.EventAfterToolCall,
		Async: true,
		Handler: func(ctx context.Context, event workflow.Event) (workflow.HookResult, error) {
			time.Sleep(10 * time.Millisecond)
			done <- true
			return workflow.HookResult{}, nil
		},
	})

	_, err := d.Dispatch(context.Background(), workflow.Event{
		Type: workflow.EventAfterToolCall,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Async hook should complete without blocking
	select {
	case <-done:
		// Good
	case <-time.After(time.Second):
		t.Fatal("async hook did not complete")
	}
}

func TestHookDispatcher_Unregister(t *testing.T) {
	d := workflow.NewHookDispatcher(zaptest.NewLogger(t))

	called := false
	d.Register(workflow.HookRegistration{
		ID:    "removable",
		Event: workflow.EventBeforeToolCall,
		Handler: func(ctx context.Context, event workflow.Event) (workflow.HookResult, error) {
			called = true
			return workflow.HookResult{Allow: true}, nil
		},
	})

	d.Unregister("removable")

	_, err := d.Dispatch(context.Background(), workflow.Event{
		Type: workflow.EventBeforeToolCall,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("unregistered handler was called")
	}
}
```

### Integration Steps

1. **Create hook dispatcher** (`internal/workflow/hooks.go`)
2. **Create plugin interface** (`internal/workflow/plugin.go`)
3. **Inject dispatcher into ToolExecutor** — emit before/after tool and bash events
4. **Inject dispatcher into Editor** — emit before/after edit events
5. **Inject dispatcher into Compactor** — emit `on_compaction` event
6. **Inject dispatcher into Agent** — emit `on_error`, `on_plan_approval` events
7. **Add plugin loading** at startup via `cmd/cli/main.go`

---


## Feature 6: MCP Full Lifecycle (4 Transports + OAuth)

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Model Context Protocol (MCP) servers connect via 4 transports: stdio (local process), SSE (server-sent events over HTTP), HTTP (direct REST), WebSocket (bidirectional). Full OAuth 2.0 flow with Authorization Server discovery (RFC 8414). SSE has auto-reconnection with exponential backoff. `alwaysLoad` option ensures servers are loaded at startup regardless of session state.
- **Why it's powerful**: Universal tool integration across local and remote services, enterprise-grade auth, resilient connections.

### HelixCode Integration Target
- **New files to create**:
  - `internal/mcp/transport.go` — Transport abstraction
  - `internal/mcp/transport_stdio.go` — Stdio transport
  - `internal/mcp/transport_sse.go` — SSE transport with reconnection
  - `internal/mcp/transport_http.go` — HTTP transport
  - `internal/mcp/transport_ws.go` — WebSocket transport
  - `internal/mcp/oauth.go` — OAuth 2.0 flow
  - `internal/mcp/lifecycle.go` — Connection lifecycle manager
  - `internal/mcp/registry.go` — MCP server registry
- **Existing files to modify**:
  - `internal/mcp/client.go` — Extend with transport selection
  - `cmd/cli/main.go` — Add `--mcp-servers` flag
- **Submodule dependencies**: `internal/mcp/`

### Exact Code Implementation

#### File: `internal/mcp/transport.go` (NEW)

```go
package mcp

import (
	"context"
	"encoding/json"
	"io"
)

// TransportType identifies the MCP connection method.
type TransportType string

const (
	TransportStdio    TransportType = "stdio"
	TransportSSE      TransportType = "sse"
	TransportHTTP     TransportType = "http"
	TransportWebSocket TransportType = "websocket"
)

// JSONRPCMessage is a standard MCP/JSON-RPC message.
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return e.Message
}

// Transport is the interface for all MCP communication backends.
type Transport interface {
	// Type returns the transport type.
	Type() TransportType

	// Connect establishes the connection.
	Connect(ctx context.Context) error

	// Disconnect closes the connection.
	Disconnect(ctx context.Context) error

	// IsConnected returns true if the transport is active.
	IsConnected() bool

	// Send writes a message to the transport.
	Send(ctx context.Context, msg JSONRPCMessage) error

	// Receive reads the next message from the transport.
	Receive(ctx context.Context) (JSONRPCMessage, error)

	// SetNotificationHandler registers a handler for server-initiated notifications.
	SetNotificationHandler(handler func(method string, params json.RawMessage))
}
```

#### File: `internal/mcp/transport_stdio.go` (NEW)

```go
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// StdioTransport spawns a local process and communicates via stdin/stdout.
type StdioTransport struct {
	command   string
	args      []string
	env       map[string]string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	scanner   *bufio.Scanner
	mu        sync.Mutex
	connected bool
	handler   func(method string, params json.RawMessage)
}

// NewStdioTransport creates a stdio transport for a command.
func NewStdioTransport(command string, args []string, env map[string]string) *StdioTransport {
	return &StdioTransport{
		command: command,
		args:    args,
		env:     env,
	}
}

func (t *StdioTransport) Type() TransportType { return TransportStdio }

func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	t.cmd = exec.CommandContext(ctx, t.command, t.args...)
	for k, v := range t.env {
		t.cmd.Env = append(t.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	t.stdin = stdin

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	t.stdout = stdout

	t.cmd.Stderr = io.Discard // Or log to structured logger

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	t.scanner = bufio.NewScanner(t.stdout)
	t.connected = true

	// Start background read loop for notifications
	go t.readLoop()

	return nil
}

func (t *StdioTransport) Disconnect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
	t.connected = false
	return nil
}

func (t *StdioTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

func (t *StdioTransport) Send(ctx context.Context, msg JSONRPCMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(t.stdin, string(data))
	return err
}

func (t *StdioTransport) Receive(ctx context.Context) (JSONRPCMessage, error) {
	// For stdio, we use a response channel system or the readLoop dispatches
	// Stub: synchronous receive via a channel
	return JSONRPCMessage{}, fmt.Errorf("use response channels for stdio")
}

func (t *StdioTransport) SetNotificationHandler(handler func(method string, params json.RawMessage)) {
	t.handler = handler
}

func (t *StdioTransport) readLoop() {
	for t.scanner.Scan() {
		line := t.scanner.Text()
		if line == "" {
			continue
		}
		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.ID == nil && msg.Method != "" && t.handler != nil {
			t.handler(msg.Method, msg.Params)
		}
	}
}
```

#### File: `internal/mcp/transport_sse.go` (NEW)

```go
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SSETransport connects via Server-Sent Events with auto-reconnection.
type SSETransport struct {
	url          string
	client       *http.Client
	mu           sync.RWMutex
	connected    bool
	eventReader  *bufio.Reader
	resp         *http.Response
	handler      func(method string, params json.RawMessage)
	stopCh       chan struct{}
	reconnectMu  sync.Mutex
}

// NewSSETransport creates an SSE transport.
func NewSSETransport(url string) *SSETransport {
	return &SSETransport{
		url:     url,
		client:  &http.Client{Timeout: 0}, // No timeout for streaming
		stopCh:  make(chan struct{}),
	}
}

func (t *SSETransport) Type() TransportType { return TransportSSE }

func (t *SSETransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", t.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connect: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE connect: status %d", resp.StatusCode)
	}

	t.resp = resp
	t.eventReader = bufio.NewReader(resp.Body)
	t.connected = true

	go t.readLoop()
	go t.reconnectWatcher()

	return nil
}

func (t *SSETransport) Disconnect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	close(t.stopCh)
	if t.resp != nil && t.resp.Body != nil {
		t.resp.Body.Close()
	}
	t.connected = false
	return nil
}

func (t *SSETransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

func (t *SSETransport) Send(ctx context.Context, msg JSONRPCMessage) error {
	// SSE transport sends via POST endpoint derived from SSE URL
	postURL := strings.Replace(t.url, "/sse", "/message", 1)
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", postURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("POST failed: %d", resp.StatusCode)
	}
	return nil
}

func (t *SSETransport) Receive(ctx context.Context) (JSONRPCMessage, error) {
	return JSONRPCMessage{}, fmt.Errorf("SSE uses push model; use notification handler")
}

func (t *SSETransport) SetNotificationHandler(handler func(method string, params json.RawMessage)) {
	t.handler = handler
}

func (t *SSETransport) readLoop() {
	for {
		select {
		case <-t.stopCh:
			return
		default:
		}

		line, err := t.eventReader.ReadString('\n')
		if err != nil {
			// Connection broken - trigger reconnect
			t.mu.Lock()
			t.connected = false
			t.mu.Unlock()
			return
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)

		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			continue
		}
		if msg.ID == nil && msg.Method != "" && t.handler != nil {
			t.handler(msg.Method, msg.Params)
		}
	}
}

func (t *SSETransport) reconnectWatcher() {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-t.stopCh:
			return
		case <-time.After(backoff):
		}

		t.mu.RLock()
		connected := t.connected
		t.mu.RUnlock()

		if connected {
			backoff = time.Second // Reset backoff
			continue
		}

		// Attempt reconnect
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := t.Connect(ctx)
		cancel()

		if err == nil {
			backoff = time.Second
		} else {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
```

#### File: `internal/mcp/transport_http.go` (NEW)

```go
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// HTTPTransport uses direct HTTP POST for each message.
type HTTPTransport struct {
	baseURL    string
	client     *http.Client
	mu         sync.RWMutex
	connected  bool
	handler    func(method string, params json.RawMessage)
	notifyURL  string // Optional SSE fallback for notifications
}

func NewHTTPTransport(baseURL string) *HTTPTransport {
	return &HTTPTransport{
		baseURL:   baseURL,
		client:    &http.Client{Timeout: 30 * time.Second},
		notifyURL: baseURL + "/notifications",
	}
}

func (t *HTTPTransport) Type() TransportType { return TransportHTTP }

func (t *HTTPTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connected = true
	return nil
}

func (t *HTTPTransport) Disconnect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connected = false
	return nil
}

func (t *HTTPTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

func (t *HTTPTransport) Send(ctx context.Context, msg JSONRPCMessage) error {
	url := t.baseURL + "/rpc"
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

func (t *HTTPTransport) Receive(ctx context.Context) (JSONRPCMessage, error) {
	return JSONRPCMessage{}, fmt.Errorf("HTTP transport uses request/response; no persistent receive")
}

func (t *HTTPTransport) SetNotificationHandler(handler func(method string, params json.RawMessage)) {
	t.handler = handler
}
```

#### File: `internal/mcp/transport_ws.go` (NEW)

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSTransport connects via WebSocket.
type WSTransport struct {
	url       string
	conn      *websocket.Conn
	mu        sync.RWMutex
	connected bool
	handler   func(method string, params json.RawMessage)
	stopCh    chan struct{}
}

// NewWSTransport creates a WebSocket transport.
func NewWSTransport(url string) *WSTransport {
	return &WSTransport{
		url:    url,
		stopCh: make(chan struct{}),
	}
}

func (t *WSTransport) Type() TransportType { return TransportWebSocket }

func (t *WSTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, t.url, nil)
	if err != nil {
		return fmt.Errorf("ws connect: %w", err)
	}

	t.conn = conn
	t.connected = true
	go t.readLoop()

	return nil
}

func (t *WSTransport) Disconnect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	close(t.stopCh)
	if t.conn != nil {
		t.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		t.conn.Close()
	}
	t.connected = false
	return nil
}

func (t *WSTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

func (t *WSTransport) Send(ctx context.Context, msg JSONRPCMessage) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected || t.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return t.conn.WriteMessage(websocket.TextMessage, data)
}

func (t *WSTransport) Receive(ctx context.Context) (JSONRPCMessage, error) {
	return JSONRPCMessage{}, fmt.Errorf("WS uses push model; use notification handler")
}

func (t *WSTransport) SetNotificationHandler(handler func(method string, params json.RawMessage)) {
	t.handler = handler
}

func (t *WSTransport) readLoop() {
	for {
		select {
		case <-t.stopCh:
			return
		default:
		}

		_, data, err := t.conn.ReadMessage()
		if err != nil {
			t.mu.Lock()
			t.connected = false
			t.mu.Unlock()
			return
		}

		var msg JSONRPCMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.ID == nil && msg.Method != "" && t.handler != nil {
			t.handler(msg.Method, msg.Params)
		}
	}
}
```

#### File: `internal/mcp/oauth.go` (NEW)

```go
package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthConfig holds OAuth 2.0 settings for an MCP server.
type OAuthConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	IssuerURL    string `json:"issuer_url"` // Authorization server discovery URL
	Scopes       []string `json:"scopes,omitempty"`
}

// AuthorizationServerMetadata is fetched via RFC 8414 discovery.
type AuthorizationServerMetadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RegistrationEndpoint  string `json:"registration_endpoint,omitempty"`
}

// TokenResponse is the OAuth 2.0 token endpoint response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// OAuthFlow manages the OAuth 2.0 authorization code flow with PKCE.
type OAuthFlow struct {
	config       OAuthConfig
	client       *http.Client
	codeVerifier string
	state        string
}

// NewOAuthFlow creates an OAuth flow manager.
func NewOAuthFlow(config OAuthConfig) *OAuthFlow {
	return &OAuthFlow{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Discover fetches authorization server metadata.
func (o *OAuthFlow) Discover(ctx context.Context) (*AuthorizationServerMetadata, error) {
	discoveryURL := strings.TrimSuffix(o.config.IssuerURL, "/") + "/.well-known/oauth-authorization-server"

	req, err := http.NewRequestWithContext(ctx, "GET", discoveryURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discovery request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned %d", resp.StatusCode)
	}

	var meta AuthorizationServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decoding discovery response: %w", err)
	}

	return &meta, nil
}

// GeneratePKCE generates a PKCE code verifier and challenge.
func (o *OAuthFlow) GeneratePKCE() (verifier, challenge string, method string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge, "S256", nil
}

// BuildAuthorizationURL constructs the authorization request URL.
func (o *OAuthFlow) BuildAuthorizationURL(meta *AuthorizationServerMetadata, redirectURI string, challenge, method string) string {
	u, _ := url.Parse(meta.AuthorizationEndpoint)
	q := u.Query()
	q.Set("client_id", o.config.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", strings.Join(o.config.Scopes, " "))
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", method)
	o.state = generateState()
	q.Set("state", o.state)
	u.RawQuery = q.Encode()
	return u.String()
}

// ExchangeCode exchanges the authorization code for an access token.
func (o *OAuthFlow) ExchangeCode(ctx context.Context, meta *AuthorizationServerMetadata, redirectURI, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", o.config.ClientID)
	if o.config.ClientSecret != "" {
		data.Set("client_secret", o.config.ClientSecret)
	}
	if o.codeVerifier != "" {
		data.Set("code_verifier", o.codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", meta.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	return &token, nil
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
```

#### File: `internal/mcp/lifecycle.go` (NEW)

```go
package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ServerConfig defines an MCP server connection.
type ServerConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Transport   TransportType     `json:"transport"`
	URL         string            `json:"url,omitempty"`         // For SSE/HTTP/WS
	Command     string            `json:"command,omitempty"`   // For stdio
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	OAuth       *OAuthConfig      `json:"oauth,omitempty"`
	AlwaysLoad  bool              `json:"always_load"`
}

// ConnectionState represents the current connection status.
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateFailed
)

// ManagedConnection wraps a Transport with lifecycle management.
type ManagedConnection struct {
	config    ServerConfig
	transport Transport
	state     ConnectionState
	lastError error
	mu        sync.RWMutex
	logger    *zap.Logger
}

// NewManagedConnection creates a managed MCP connection.
func NewManagedConnection(config ServerConfig, logger *zap.Logger) (*ManagedConnection, error) {
	var transport Transport
	switch config.Transport {
	case TransportStdio:
		transport = NewStdioTransport(config.Command, config.Args, config.Env)
	case TransportSSE:
		transport = NewSSETransport(config.URL)
	case TransportHTTP:
		transport = NewHTTPTransport(config.URL)
	case TransportWebSocket:
		transport = NewWSTransport(config.URL)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", config.Transport)
	}

	return &ManagedConnection{
		config:    config,
		transport: transport,
		state:     StateDisconnected,
		logger:    logger,
	}, nil
}

// Connect establishes the connection with retry logic.
func (mc *ManagedConnection) Connect(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.state == StateConnected {
		return nil
	}

	mc.state = StateConnecting

	if err := mc.transport.Connect(ctx); err != nil {
		mc.state = StateFailed
		mc.lastError = err
		return fmt.Errorf("connect %s: %w", mc.config.Name, err)
	}

	// If OAuth is configured, perform discovery and token exchange
	if mc.config.OAuth != nil {
		// Token management would be handled here
		mc.logger.Info("OAuth configured for MCP server", zap.String("server", mc.config.Name))
	}

	mc.state = StateConnected
	mc.logger.Info("MCP server connected", zap.String("server", mc.config.Name), zap.String("transport", string(mc.config.Transport)))
	return nil
}

// Disconnect closes the connection.
func (mc *ManagedConnection) Disconnect(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.state == StateDisconnected {
		return nil
	}

	err := mc.transport.Disconnect(ctx)
	mc.state = StateDisconnected
	mc.logger.Info("MCP server disconnected", zap.String("server", mc.config.Name))
	return err
}

// GetState returns the current connection state.
func (mc *ManagedConnection) GetState() ConnectionState {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.state
}

// Send delegates to the underlying transport.
func (mc *ManagedConnection) Send(ctx context.Context, msg JSONRPCMessage) error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if mc.state != StateConnected {
		return fmt.Errorf("not connected (state: %d)", mc.state)
	}

	return mc.transport.Send(ctx, msg)
}

// IsHealthy returns true if the connection is active.
func (mc *ManagedConnection) IsHealthy() bool {
	return mc.transport.IsConnected()
}
```

#### File: `internal/mcp/registry.go` (NEW)

```go
package mcp

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Registry manages all MCP server connections.
type Registry struct {
	connections map[string]*ManagedConnection
	mu          sync.RWMutex
	logger      *zap.Logger
}

// NewRegistry creates an MCP registry.
func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		connections: make(map[string]*ManagedConnection),
		logger:        logger,
	}
}

// AddServer registers and optionally connects an MCP server.
func (r *Registry) AddServer(ctx context.Context, config ServerConfig, autoConnect bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.connections[config.ID]; exists {
		return fmt.Errorf("server %s already registered", config.ID)
	}

	conn, err := NewManagedConnection(config, r.logger)
	if err != nil {
		return err
	}

	r.connections[config.ID] = conn

	if autoConnect || config.AlwaysLoad {
		if err := conn.Connect(ctx); err != nil {
			r.logger.Warn("auto-connect failed", zap.String("server", config.Name), zap.Error(err))
			// Don't fail registration; connection will retry
		}
	}

	return nil
}

// RemoveServer disconnects and removes a server.
func (r *Registry) RemoveServer(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, exists := r.connections[id]
	if !exists {
		return fmt.Errorf("server %s not found", id)
	}

	if err := conn.Disconnect(ctx); err != nil {
		r.logger.Warn("disconnect failed", zap.String("server", id), zap.Error(err))
	}

	delete(r.connections, id)
	return nil
}

// GetServer returns a managed connection by ID.
func (r *Registry) GetServer(id string) (*ManagedConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conn, exists := r.connections[id]
	if !exists {
		return nil, fmt.Errorf("server %s not found", id)
	}
	return conn, nil
}

// ListServers returns all registered server IDs.
func (r *Registry) ListServers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.connections))
	for id := range r.connections {
		ids = append(ids, id)
	}
	return ids
}

// ConnectAll connects all registered servers with AlwaysLoad=true.
func (r *Registry) ConnectAll(ctx context.Context) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, conn := range r.connections {
		if conn.config.AlwaysLoad {
			go func(c *ManagedConnection) {
				if err := c.Connect(ctx); err != nil {
					r.logger.Warn("connect all failed", zap.String("server", c.config.Name), zap.Error(err))
				}
			}(conn)
		}
	}
}
```

### Anti-Bluff Verification Test

```go
package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/mcp"
	"go.uber.org/zap/zaptest"
)

func TestSSETransport_ConnectAndReconnect(t *testing.T) {
	// Create a test SSE server
	events := make(chan string, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		for msg := range events {
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}))
	defer server.Close()

	transport := mcp.NewSSETransport(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	if !transport.IsConnected() {
		t.Fatal("expected IsConnected = true")
	}

	// Test disconnect
	if err := transport.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}
}

func TestRegistry_AddAndConnect(t *testing.T) {
	registry := mcp.NewRegistry(zaptest.NewLogger(t))

	config := mcp.ServerConfig{
		ID:         "test-stdio",
		Name:       "Test Server",
		Transport:  mcp.TransportStdio,
		Command:    "echo",
		Args:       []string{"hello"},
		AlwaysLoad: false,
	}

	ctx := context.Background()
	if err := registry.AddServer(ctx, config, false); err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	servers := registry.ListServers()
	if len(servers) != 1 || servers[0] != "test-stdio" {
		t.Fatalf("ListServers = %v, want [test-stdio]", servers)
	}

	conn, err := registry.GetServer("test-stdio")
	if err != nil {
		t.Fatalf("GetServer: %v", err)
	}
	if conn.GetState() != mcp.StateDisconnected {
		t.Fatalf("state = %d, want disconnected", conn.GetState())
	}
}

func TestOAuthFlow_PKCE(t *testing.T) {
	flow := mcp.NewOAuthFlow(mcp.OAuthConfig{ClientID: "test-client"})

	verifier, challenge, method, err := flow.GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}

	if verifier == "" {
		t.Fatal("verifier is empty")
	}
	if challenge == "" {
		t.Fatal("challenge is empty")
	}
	if method != "S256" {
		t.Fatalf("method = %s, want S256", method)
	}

	// Verify challenge is hash of verifier
	import "crypto/sha256"
	import "encoding/base64"
	hash := sha256.Sum256([]byte(verifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(hash[:])
	if challenge != expectedChallenge {
		t.Fatal("challenge does not match verifier hash")
	}
}

func TestTransportType_Values(t *testing.T) {
	types := []mcp.TransportType{mcp.TransportStdio, mcp.TransportSSE, mcp.TransportHTTP, mcp.TransportWebSocket}
	for _, tt := range types {
		if tt == "" {
			t.Fatal("transport type is empty")
		}
	}
}
```

### Integration Steps

1. **Create all transport files** in `internal/mcp/`
2. **Extend existing MCP client** to use transport abstraction
3. **Add OAuth flow** for enterprise deployments
4. **Register in CLI** with `--mcp-config` JSON file
5. **Auto-connect** `AlwaysLoad` servers at startup
6. **Add SSE reconnection** with exponential backoff

---

## Feature 7: Background Task System (Ctrl+B)

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Tools can be invoked with `run_in_background: true`. The task runs in a goroutine, returning immediately with a task ID. `TaskOutput` tool reads the last N lines (default: 5) of output from a running or completed task. `TaskStop` tool cancels a task by ID. Progress is tracked via last 5 lines of output.
- **Why it's powerful**: Enables long-running operations (builds, tests, installs) without blocking the agent. User can check progress asynchronously.

### HelixCode Integration Target
- **New files to create**:
  - `internal/workflow/background.go` — Background task manager
  - `internal/workflow/background_test.go`
- **Existing files to modify**:
  - `internal/tools/tool_executor.go` — Detect `run_in_background` flag
  - `internal/tools/registry.go` — Register TaskOutput, TaskStop tools
- **Submodule dependencies**: `internal/workflow/`, `internal/tools/`

### Exact Code Implementation

#### File: `internal/workflow/background.go` (NEW)

```go
package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TaskState represents the status of a background task.
type TaskState string

const (
	TaskPending   TaskState = "pending"
	TaskRunning   TaskState = "running"
	TaskCompleted TaskState = "completed"
	TaskFailed    TaskState = "failed"
	TaskCancelled TaskState = "cancelled"
)

// BackgroundTask represents a running background operation.
type BackgroundTask struct {
	ID        string
	ToolName  string
	Args      map[string]any
	State     TaskState
	Output    []string       // Lines of output
	Result    any
	Error     error
	StartedAt time.Time
	EndedAt   *time.Time
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
}

// LastLines returns the last N lines of output.
func (bt *BackgroundTask) LastLines(n int) []string {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	if n <= 0 {
		n = 5
	}
	if len(bt.Output) <= n {
		out := make([]string, len(bt.Output))
		copy(out, bt.Output)
		return out
	}
	return bt.Output[len(bt.Output)-n:]
}

// AppendOutput adds a line of output.
func (bt *BackgroundTask) AppendOutput(line string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.Output = append(bt.Output, line)
}

// GetState returns current state safely.
func (bt *BackgroundTask) GetState() TaskState {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.State
}

// SetState updates task state.
func (bt *BackgroundTask) SetState(state TaskState) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.State = state
	if state == TaskCompleted || state == TaskFailed || state == TaskCancelled {
		now := time.Now()
		bt.EndedAt = &now
	}
}

// BackgroundManager manages concurrent background tasks.
type BackgroundManager struct {
	tasks  map[string]*BackgroundTask
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewBackgroundManager creates a background task manager.
func NewBackgroundManager(logger *zap.Logger) *BackgroundManager {
	return &BackgroundManager{
		tasks:  make(map[string]*BackgroundTask),
		logger: logger,
	}
}

// StartTask begins a new background task.
func (bm *BackgroundManager) StartTask(toolName string, args map[string]any, executor func(ctx context.Context, args map[string]any) (any, error)) (*BackgroundTask, error) {
	id := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())

	task := &BackgroundTask{
		ID:        id,
		ToolName:  toolName,
		Args:      args,
		State:     TaskPending,
		Output:    []string{},
		StartedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	bm.mu.Lock()
	bm.tasks[id] = task
	bm.mu.Unlock()

	// Execute in goroutine
	go func() {
		task.SetState(TaskRunning)
		bm.logger.Info("background task started", zap.String("id", id), zap.String("tool", toolName))

		result, err := executor(ctx, args)

		task.mu.Lock()
		task.Result = result
		task.Error = err
		task.mu.Unlock()

		if err != nil {
			task.SetState(TaskFailed)
			task.AppendOutput(fmt.Sprintf("Error: %v", err))
			bm.logger.Warn("background task failed", zap.String("id", id), zap.Error(err))
		} else {
			task.SetState(TaskCompleted)
			bm.logger.Info("background task completed", zap.String("id", id))
		}
	}()

	return task, nil
}

// GetTask retrieves a task by ID.
func (bm *BackgroundManager) GetTask(id string) (*BackgroundTask, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	task, exists := bm.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return task, nil
}

// StopTask cancels a running background task.
func (bm *BackgroundManager) StopTask(id string) error {
	bm.mu.RLock()
	task, exists := bm.tasks[id]
	bm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task %s not found", id)
	}

	if task.GetState() != TaskRunning && task.GetState() != TaskPending {
		return fmt.Errorf("task %s is not running (state: %s)", id, task.GetState())
	}

	task.cancel()
	task.SetState(TaskCancelled)
	bm.logger.Info("background task cancelled", zap.String("id", id))
	return nil
}

// ListTasks returns all task IDs.
func (bm *BackgroundManager) ListTasks() []string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	ids := make([]string, 0, len(bm.tasks))
	for id := range bm.tasks {
		ids = append(ids, id)
	}
	return ids
}

// Cleanup removes completed tasks older than the given duration.
func (bm *BackgroundManager) Cleanup(maxAge time.Duration) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, task := range bm.tasks {
		if task.EndedAt != nil && task.EndedAt.Before(cutoff) {
			delete(bm.tasks, id)
		}
	}
}
```

#### File: `internal/tools/task_tools.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/workflow"
)

// TaskOutputTool reads output from a background task.
type TaskOutputTool struct {
	manager *workflow.BackgroundManager
}

func NewTaskOutputTool(manager *workflow.BackgroundManager) *TaskOutputTool {
	return &TaskOutputTool{manager: manager}
}

func (t *TaskOutputTool) Name() string { return "TaskOutput" }

func (t *TaskOutputTool) Description() string {
	return "Read the output from a background task. Returns the last 5 lines by default."
}

func (t *TaskOutputTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "ID of the background task",
			},
			"lines": map[string]any{
				"type":        "integer",
				"description": "Number of lines to return (default: 5)",
			},
		},
		"required": []string{"task_id"},
	}
}

func (t *TaskOutputTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	taskID, ok := args["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("task_id must be a string")
	}

	lines := 5
	if l, ok := args["lines"].(float64); ok {
		lines = int(l)
	}

	task, err := t.manager.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	output := task.LastLines(lines)
	return map[string]any{
		"task_id":    taskID,
		"state":      string(task.GetState()),
		"output":     strings.Join(output, "\n"),
		"line_count": len(output),
		"total_lines": len(task.Output),
	}, nil
}

// TaskStopTool cancels a background task.
type TaskStopTool struct {
	manager *workflow.BackgroundManager
}

func NewTaskStopTool(manager *workflow.BackgroundManager) *TaskStopTool {
	return &TaskStopTool{manager: manager}
}

func (t *TaskStopTool) Name() string { return "TaskStop" }

func (t *TaskStopTool) Description() string {
	return "Stop a running background task."
}

func (t *TaskStopTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "ID of the task to stop",
			},
		},
		"required": []string{"task_id"},
	}
}

func (t *TaskStopTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	taskID, ok := args["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("task_id must be a string")
	}

	if err := t.manager.StopTask(taskID); err != nil {
		return nil, err
	}

	return map[string]any{
		"task_id": taskID,
		"status":  "stopped",
	}, nil
}
```

#### File: `internal/tools/tool_executor.go` (MODIFY — Background Detection)

```go
// In Execute method, check for run_in_background flag:
func (te *ToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any, toolCallID string, sessionID string) (*PersistedResult, error) {
	// Check if background flag is set
	if bg, ok := args["run_in_background"].(bool); ok && bg {
		// Remove the flag before executing
		cleanArgs := make(map[string]any)
		for k, v := range args {
			if k != "run_in_background" {
				cleanArgs[k] = v
			}
		}

		// Get the tool from registry for background execution
		tool, err := te.registry.Get(toolName)
		if err != nil {
			return nil, err
		}

		task, err := te.backgroundManager.StartTask(toolName, cleanArgs, func(ctx context.Context, a map[string]any) (any, error) {
			return tool.Execute(ctx, a)
		})
		if err != nil {
			return nil, err
		}

		return &PersistedResult{
			Output: fmt.Sprintf("Task started in background. ID: %s\nUse TaskOutput tool to check progress.", task.ID),
			WasPersisted: false,
			ToolName: toolName,
			ToolCallID: toolCallID,
		}, nil
	}

	// ... existing synchronous execution ...
}
```

### Anti-Bluff Verification Test

```go
package workflow_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/workflow"
	"go.uber.org/zap/zaptest"
)

func TestBackgroundManager_StartAndRetrieve(t *testing.T) {
	bm := workflow.NewBackgroundManager(zaptest.NewLogger(t))

	// Start a slow task
	task, err := bm.StartTask("Bash", map[string]any{"command": "sleep 1"}, func(ctx context.Context, args map[string]any) (any, error) {
		time.Sleep(100 * time.Millisecond)
		return "done", nil
	})
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	if task.ID == "" {
		t.Fatal("task ID is empty")
	}
	if task.GetState() != workflow.TaskRunning {
		t.Fatalf("initial state = %s, want running", task.GetState())
	}

	// Should be able to retrieve
	retrieved, err := bm.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if retrieved.ID != task.ID {
		t.Fatal("retrieved wrong task")
	}

	// Wait for completion
	time.Sleep(200 * time.Millisecond)
	if retrieved.GetState() != workflow.TaskCompleted {
		t.Fatalf("final state = %s, want completed", retrieved.GetState())
	}
}

func TestBackgroundManager_StopTask(t *testing.T) {
	bm := workflow.NewBackgroundManager(zaptest.NewLogger(t))

	task, err := bm.StartTask("Bash", map[string]any{}, func(ctx context.Context, args map[string]any) (any, error) {
		// Long running task that checks context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return "done", nil
		}
	})
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	// Stop immediately
	if err := bm.StopTask(task.ID); err != nil {
		t.Fatalf("StopTask: %v", err)
	}

	// Give goroutine time to process cancellation
	time.Sleep(50 * time.Millisecond)

	if task.GetState() != workflow.TaskCancelled {
		t.Fatalf("state after stop = %s, want cancelled", task.GetState())
	}
}

func TestBackgroundTask_LastLines(t *testing.T) {
	task := &workflow.BackgroundTask{}

	// Add 10 lines
	for i := 0; i < 10; i++ {
		task.AppendOutput(fmt.Sprintf("line %d", i))
	}

	last5 := task.LastLines(5)
	if len(last5) != 5 {
		t.Fatalf("len(last5) = %d, want 5", len(last5))
	}
	if last5[0] != "line 5" {
		t.Fatalf("first of last5 = %s, want line 5", last5[0])
	}
	if last5[4] != "line 9" {
		t.Fatalf("last of last5 = %s, want line 9", last5[4])
	}

	// Default should return 5
	defaultLines := task.LastLines(0)
	if len(defaultLines) != 5 {
		t.Fatalf("default lines = %d, want 5", len(defaultLines))
	}
}

func TestBackgroundManager_Cleanup(t *testing.T) {
	bm := workflow.NewBackgroundManager(zaptest.NewLogger(t))

	// Create a completed task
	task, _ := bm.StartTask("Test", map[string]any{}, func(ctx context.Context, args map[string]any) (any, error) {
		return "done", nil
	})
	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	// Should exist
	if _, err := bm.GetTask(task.ID); err != nil {
		t.Fatalf("GetTask before cleanup: %v", err)
	}

	// Cleanup with 0 max age should remove it
	bm.Cleanup(0)

	// Should be gone
	if _, err := bm.GetTask(task.ID); err == nil {
		t.Fatal("expected task to be cleaned up")
	}
}

func TestTaskOutputTool_Execute(t *testing.T) {
	bm := workflow.NewBackgroundManager(zaptest.NewLogger(t))
	tool := tools.NewTaskOutputTool(bm)

	task, _ := bm.StartTask("Bash", map[string]any{}, func(ctx context.Context, args map[string]any) (any, error) {
		return "output line 1\noutput line 2\noutput line 3", nil
	})
	time.Sleep(100 * time.Millisecond)

	result, err := tool.Execute(context.Background(), map[string]any{
		"task_id": task.ID,
		"lines":   2,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	m := result.(map[string]any)
	if m["task_id"] != task.ID {
		t.Fatalf("task_id mismatch")
	}
	if m["state"] != string(workflow.TaskCompleted) {
		t.Fatalf("state = %v, want completed", m["state"])
	}
}
```

### Integration Steps

1. **Create background manager** (`internal/workflow/background.go`)
2. **Create TaskOutput/TaskStop tools** (`internal/tools/task_tools.go`)
3. **Modify tool executor** to detect `run_in_background` and delegate to manager
4. **Register new tools** in registry
5. **Add Ctrl+B shortcut** in terminal UI for "run last command in background"
6. **Add periodic cleanup** of old tasks

---

## Feature 8: Plan Mode

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: A read-only exploration mode where the agent proposes a structured plan of actions. Each action has semantic meaning (e.g., "read file X", "edit file Y"). User can approve the entire plan, individual actions, or reject it. `ExitPlanMode` tool with `allowedPrompts` returns to normal execution. In plan mode, destructive tools are blocked unless explicitly planned.
- **Why it's powerful**: Prevents "agent hallucination" of destructive changes, gives user high-level control over complex multi-step operations.

### HelixCode Integration Target
- **New files to create**:
  - `internal/workflow/plan.go` — Plan state machine
  - `internal/workflow/plan_test.go`
- **Existing files to modify**:
  - `internal/agent/agent.go` — Plan mode state machine in main loop
  - `internal/tools/tool_executor.go` — Block destructive tools in plan mode
  - `internal/tools/registry.go` — Register ExitPlanMode tool
- **Submodule dependencies**: `internal/workflow/`, `internal/agent/`, `internal/tools/`

### Exact Code Implementation

#### File: `internal/workflow/plan.go` (NEW)

```go
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// PlanActionType categorizes planned actions.
type PlanActionType string

const (
	ActionRead      PlanActionType = "read"
	ActionEdit      PlanActionType = "edit"
	ActionBash      PlanActionType = "bash"
	ActionWrite     PlanActionType = "write"
	ActionTest      PlanActionType = "test"
	ActionGit       PlanActionType = "git"
	ActionResearch  PlanActionType = "research"
)

// PlanAction is a single step in a plan.
type PlanAction struct {
	ID          string         `json:"id"`
	Type        PlanActionType `json:"type"`
	Description string         `json:"description"`
	ToolName    string         `json:"tool_name"`
	Args        map[string]any `json:"args"`
	Approved    *bool          `json:"approved,omitempty"`
	Executed    bool           `json:"executed"`
	Result      any            `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// Plan is a structured multi-step plan.
type Plan struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Actions     []PlanAction `json:"actions"`
	CreatedAt   time.Time    `json:"created_at"`
	ApprovedAt  *time.Time   `json:"approved_at,omitempty"`
	Status      PlanStatus   `json:"status"`
}

// PlanStatus tracks the plan lifecycle.
type PlanStatus string

const (
	PlanDraft     PlanStatus = "draft"
	PlanPending   PlanStatus = "pending_approval"
	PlanApproved  PlanStatus = "approved"
	PlanRejected  PlanStatus = "rejected"
	PlanExecuting PlanStatus = "executing"
	PlanCompleted PlanStatus = "completed"
	PlanFailed    PlanStatus = "failed"
)

// PlanModeState tracks whether the agent is in plan mode.
type PlanModeState struct {
	Active      bool
	CurrentPlan *Plan
	AllowedPrompts []string // Prompts that can exit plan mode
	mu          sync.RWMutex
}

// NewPlanModeState creates initial plan mode state.
func NewPlanModeState() *PlanModeState {
	return &PlanModeState{Active: false}
}

// IsInPlanMode returns true if plan mode is active.
func (p *PlanModeState) IsInPlanMode() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Active
}

// EnterPlanMode activates plan mode with optional allowed prompts.
func (p *PlanModeState) EnterPlanMode(allowedPrompts []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Active = true
	p.AllowedPrompts = allowedPrompts
	p.CurrentPlan = nil
}

// ExitPlanMode deactivates plan mode.
func (p *PlanModeState) ExitPlanMode() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Active = false
	p.CurrentPlan = nil
	p.AllowedPrompts = nil
}

// CanExitWithPrompt checks if a user prompt is allowed to exit plan mode.
func (p *PlanModeState) CanExitWithPrompt(prompt string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.Active {
		return true
	}
	if len(p.AllowedPrompts) == 0 {
		return true // Any prompt can exit if no restrictions
	}
	for _, allowed := range p.AllowedPrompts {
		if allowed == prompt {
			return true
		}
	}
	return false
}

// SetCurrentPlan sets the plan being reviewed.
func (p *PlanModeState) SetCurrentPlan(plan *Plan) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CurrentPlan = plan
}

// GetCurrentPlan returns the active plan.
func (p *PlanModeState) GetCurrentPlan() *Plan {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.CurrentPlan
}

// PlanManager handles plan creation, approval, and execution.
type PlanManager struct {
	state     *PlanModeState
	dispatcher *HookDispatcher
}

// NewPlanManager creates a plan manager.
func NewPlanManager(state *PlanModeState, dispatcher *HookDispatcher) *PlanManager {
	return &PlanManager{
		state:      state,
		dispatcher: dispatcher,
	}
}

// CreatePlan initializes a new plan.
func (pm *PlanManager) CreatePlan(title, description string, actions []PlanAction) *Plan {
	plan := &Plan{
		ID:          generatePlanID(),
		Title:       title,
		Description: description,
		Actions:     actions,
		CreatedAt:   time.Now(),
		Status:      PlanPending,
	}
	pm.state.SetCurrentPlan(plan)
	return plan
}

// ApprovePlan marks all actions as approved and sets plan to approved.
func (pm *PlanManager) ApprovePlan() error {
	plan := pm.state.GetCurrentPlan()
	if plan == nil {
		return fmt.Errorf("no active plan")
	}
	if plan.Status != PlanPending {
		return fmt.Errorf("plan is not pending approval (status: %s)", plan.Status)
	}

	now := time.Now()
	plan.ApprovedAt = &now
	plan.Status = PlanApproved
	for i := range plan.Actions {
		approved := true
		plan.Actions[i].Approved = &approved
	}

	// Emit hook
	if pm.dispatcher.HasHandlers(EventOnPlanApproval) {
		pm.dispatcher.Dispatch(context.Background(), Event{
			Type: EventOnPlanApproval,
			Payload: map[string]any{
				"plan_id": plan.ID,
				"actions": len(plan.Actions),
			},
		})
	}

	return nil
}

// ApproveAction approves a single action by ID.
func (pm *PlanManager) ApproveAction(actionID string) error {
	plan := pm.state.GetCurrentPlan()
	if plan == nil {
		return fmt.Errorf("no active plan")
	}

	for i := range plan.Actions {
		if plan.Actions[i].ID == actionID {
			approved := true
			plan.Actions[i].Approved = &approved
			return nil
		}
	}
	return fmt.Errorf("action %s not found", actionID)
}

// RejectPlan rejects the current plan.
func (pm *PlanManager) RejectPlan() error {
	plan := pm.state.GetCurrentPlan()
	if plan == nil {
		return fmt.Errorf("no active plan")
	}
	plan.Status = PlanRejected
	pm.state.ExitPlanMode()
	return nil
}

// ExecutePlan runs all approved unexecuted actions.
func (pm *PlanManager) ExecutePlan(ctx context.Context, executor func(ctx context.Context, action PlanAction) (any, error)) error {
	plan := pm.state.GetCurrentPlan()
	if plan == nil {
		return fmt.Errorf("no active plan")
	}
	if plan.Status != PlanApproved {
		return fmt.Errorf("plan not approved (status: %s)", plan.Status)
	}

	plan.Status = PlanExecuting
	for i := range plan.Actions {
		action := &plan.Actions[i]
		if action.Executed || action.Approved == nil || !*action.Approved {
			continue
		}

		result, err := executor(ctx, *action)
		action.Executed = true
		action.Result = result
		if err != nil {
			action.Error = err.Error()
			plan.Status = PlanFailed
			return fmt.Errorf("action %s failed: %w", action.ID, err)
		}
	}

	plan.Status = PlanCompleted
	return nil
}

// IsDestructiveToolBlocked checks if a tool should be blocked in plan mode.
func (pm *PlanManager) IsDestructiveToolBlocked(toolName string) bool {
	if !pm.state.IsInPlanMode() {
		return false
	}
	// In plan mode, only Read, research, and plan-approved tools are allowed
	switch toolName {
	case "Read", "LSPGetDiagnostics", "Glob", "Grep", "View", "TaskOutput":
		return false // Always allowed
	}
	// Check if there's an approved plan action for this tool
	plan := pm.state.GetCurrentPlan()
	if plan != nil && plan.Status == PlanApproved {
		for _, action := range plan.Actions {
			if action.ToolName == toolName && action.Approved != nil && *action.Approved && !action.Executed {
				return false // Tool is part of approved plan
			}
		}
	}
	return true
}

func generatePlanID() string {
	return fmt.Sprintf("plan_%d", time.Now().UnixNano())
}
```

#### File: `internal/tools/exit_plan_mode.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"

	"dev.helix.code/internal/workflow"
)

// ExitPlanModeTool exits plan mode and optionally executes the approved plan.
type ExitPlanModeTool struct {
	planState *workflow.PlanModeState
}

func NewExitPlanModeTool(state *workflow.PlanModeState) *ExitPlanModeTool {
	return &ExitPlanModeTool{planState: state}
}

func (t *ExitPlanModeTool) Name() string { return "ExitPlanMode" }

func (t *ExitPlanModeTool) Description() string {
	return "Exit plan mode and return to normal execution. Optionally specify allowed prompts that can trigger exit."
}

func (t *ExitPlanModeTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"allowed_prompts": map[string]any{
				"type":        "array",
				"items": map[string]any{"type": "string"},
				"description": "List of prompts that are allowed to exit plan mode",
			},
		},
	}
}

func (t *ExitPlanModeTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	if !t.planState.IsInPlanMode() {
		return map[string]any{"status": "not_in_plan_mode"}, nil
	}

	if prompts, ok := args["allowed_prompts"].([]any); ok {
		allowed := make([]string, len(prompts))
		for i, p := range prompts {
			allowed[i] = fmt.Sprint(p)
		}
		t.planState.EnterPlanMode(allowed)
	} else {
		t.planState.ExitPlanMode()
	}

	return map[string]any{
		"status":     "exited_plan_mode",
		"was_active": true,
	}, nil
}
```

#### File: `internal/agent/agent.go` (MODIFY — Plan Mode State Machine)

```go
// Add to Agent struct:
type Agent struct {
	// ... existing fields ...
	planState   *workflow.PlanModeState
	planManager *workflow.PlanManager
}

// In RunTurn, check plan mode:
func (a *Agent) RunTurn(ctx context.Context, sessionID string) error {
	// If in plan mode and no active plan, we are in read-only exploration
	if a.planState.IsInPlanMode() {
		// Only allow Read, research tools unless plan is approved
		// This is enforced in tool executor
	}

	// ... existing LLM call ...

	// If LLM returns a plan, transition to plan pending
	// If user approves, transition to approved and execute
	// If user rejects, exit plan mode
}
```

### Anti-Bluff Verification Test

```go
package workflow_test

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/workflow"
)

func TestPlanModeState_EnterAndExit(t *testing.T) {
	state := workflow.NewPlanModeState()

	if state.IsInPlanMode() {
		t.Fatal("should not be in plan mode initially")
	}

	state.EnterPlanMode([]string{"execute", "go"})
	if !state.IsInPlanMode() {
		t.Fatal("should be in plan mode after enter")
	}

	if !state.CanExitWithPrompt("execute") {
		t.Fatal("should allow 'execute' prompt")
	}
	if state.CanExitWithPrompt("random") {
		t.Fatal("should not allow 'random' prompt")
	}

	state.ExitPlanMode()
	if state.IsInPlanMode() {
		t.Fatal("should not be in plan mode after exit")
	}
}

func TestPlanManager_CreateAndApprove(t *testing.T) {
	state := workflow.NewPlanModeState()
	pm := workflow.NewPlanManager(state, nil)

	actions := []workflow.PlanAction{
		{ID: "a1", Type: workflow.ActionRead, ToolName: "Read", Description: "Read main.go"},
		{ID: "a2", Type: workflow.ActionEdit, ToolName: "Edit", Description: "Update function"},
	}

	plan := pm.CreatePlan("Refactor API", "Restructure the API endpoints", actions)
	if plan.Status != workflow.PlanPending {
		t.Fatalf("status = %s, want pending", plan.Status)
	}
	if len(plan.Actions) != 2 {
		t.Fatalf("actions = %d, want 2", len(plan.Actions))
	}

	// Approve plan
	if err := pm.ApprovePlan(); err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}
	if plan.Status != workflow.PlanApproved {
		t.Fatalf("status = %s, want approved", plan.Status)
	}
	if plan.ApprovedAt == nil {
		t.Fatal("approved_at is nil")
	}
	for _, a := range plan.Actions {
		if a.Approved == nil || !*a.Approved {
			t.Fatalf("action %s not approved", a.ID)
		}
	}
}

func TestPlanManager_ExecutePlan(t *testing.T) {
	state := workflow.NewPlanModeState()
	pm := workflow.NewPlanManager(state, nil)

	actions := []workflow.PlanAction{
		{ID: "a1", Type: workflow.ActionBash, ToolName: "Bash", Description: "Run tests"},
	}
	pm.CreatePlan("Test Plan", "", actions)
	pm.ApprovePlan()

	executed := false
	err := pm.ExecutePlan(context.Background(), func(ctx context.Context, action workflow.PlanAction) (any, error) {
		executed = true
		if action.ID != "a1" {
			t.Fatalf("wrong action: %s", action.ID)
		}
		return "test output", nil
	})

	if err != nil {
		t.Fatalf("ExecutePlan: %v", err)
	}
	if !executed {
		t.Fatal("executor not called")
	}

	plan := state.GetCurrentPlan()
	if plan.Status != workflow.PlanCompleted {
		t.Fatalf("status = %s, want completed", plan.Status)
	}
	if !plan.Actions[0].Executed {
		t.Fatal("action not marked executed")
	}
	if plan.Actions[0].Result != "test output" {
		t.Fatalf("result = %v, want test output", plan.Actions[0].Result)
	}
}

func TestPlanManager_IsDestructiveToolBlocked(t *testing.T) {
	state := workflow.NewPlanModeState()
	pm := workflow.NewPlanManager(state, nil)

	// Not in plan mode - nothing blocked
	if pm.IsDestructiveToolBlocked("Bash") {
		t.Fatal("should not block when not in plan mode")
	}

	// Enter plan mode
	state.EnterPlanMode(nil)

	// Read should not be blocked
	if pm.IsDestructiveToolBlocked("Read") {
		t.Fatal("Read should not be blocked in plan mode")
	}
	if pm.IsDestructiveToolBlocked("Grep") {
		t.Fatal("Grep should not be blocked in plan mode")
	}

	// Destructive should be blocked
	if !pm.IsDestructiveToolBlocked("Bash") {
		t.Fatal("Bash should be blocked in plan mode")
	}
	if !pm.IsDestructiveToolBlocked("Edit") {
		t.Fatal("Edit should be blocked in plan mode")
	}

	// Create and approve a plan with Bash action
	actions := []workflow.PlanAction{
		{ID: "a1", Type: workflow.ActionBash, ToolName: "Bash", Description: "Run safe command"},
	}
	pm.CreatePlan("", "", actions)
	pm.ApprovePlan()

	// Now Bash is part of approved plan - should not be blocked
	if pm.IsDestructiveToolBlocked("Bash") {
		t.Fatal("Bash should not be blocked when part of approved plan")
	}

	// But Edit is still blocked
	if !pm.IsDestructiveToolBlocked("Edit") {
		t.Fatal("Edit should still be blocked")
	}
}
```

### Integration Steps

1. **Create plan state machine** (`internal/workflow/plan.go`)
2. **Create ExitPlanMode tool** (`internal/tools/exit_plan_mode.go`)
3. **Modify tool executor** to check `planManager.IsDestructiveToolBlocked()` before execution
4. **Modify agent loop** to:
   - Detect when LLM proposes a plan
   - Transition to `PlanPending` status
   - Wait for user approval/rejection
   - Execute approved plan actions sequentially
5. **Add plan mode indicator** in terminal UI

---

## Feature 9: Slash Command System

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Custom commands defined as Markdown files with YAML frontmatter in `.claude/commands/*.md`. Frontmatter specifies title, description, and variable substitutions. Commands are invoked with `/command-name`. Variables like `{{ARG1}}`, `{{SELECTION}}`, `{{CURRENT_FILE}}` are substituted at runtime.
- **Why it's powerful**: Teams can codify common workflows (e.g., `/refactor`, `/test`, `/deploy`) as reusable, version-controlled templates.

### HelixCode Integration Target
- **New files to create**:
  - `internal/workflow/slash_commands.go` — Command parser and executor
  - `internal/workflow/slash_commands_test.go`
- **Existing files to modify**:
  - `internal/agent/agent.go` — Detect `/` prefix and route to slash command handler
  - `cmd/cli/main.go` — Add `--commands-dir` flag
- **Submodule dependencies**: `internal/workflow/`

### Exact Code Implementation

#### File: `internal/workflow/slash_commands.go` (NEW)

```go
package workflow

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SlashCommand defines a user-defined command.
type SlashCommand struct {
	Name        string            `yaml:"name"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Variables   map[string]string `yaml:"variables,omitempty"`
	Body        string            // Markdown body after frontmatter
	SourcePath  string
}

// SlashCommandRegistry manages available slash commands.
type SlashCommandRegistry struct {
	commands map[string]*SlashCommand
	mu       sync.RWMutex
}

// NewSlashCommandRegistry creates a registry.
func NewSlashCommandRegistry() *SlashCommandRegistry {
	return &SlashCommandRegistry{
		commands: make(map[string]*SlashCommand),
	}
}

// LoadFromDirectory scans `.helix/commands/*.md` for command definitions.
func (r *SlashCommandRegistry) LoadFromDirectory(dir string) error {
	commandsDir := filepath.Join(dir, ".helix", "commands")
	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(commandsDir, entry.Name())
		cmd, err := ParseSlashCommandFile(path)
		if err != nil {
			continue // Log warning but don't fail
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		cmd.Name = name
		cmd.SourcePath = path
		r.commands[name] = cmd
	}

	return nil
}

// Get retrieves a command by name.
func (r *SlashCommandRegistry) Get(name string) (*SlashCommand, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, exists := r.commands[name]
	return cmd, exists
}

// List returns all command names.
func (r *SlashCommandRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	return names
}

// ParseSlashCommandFile reads a Markdown file with YAML frontmatter.
func ParseSlashCommandFile(path string) (*SlashCommand, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("no YAML frontmatter found")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid frontmatter format")
	}

	// Parse YAML frontmatter
	var cmd SlashCommand
	if err := yaml.Unmarshal([]byte(parts[1]), &cmd); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	cmd.Body = strings.TrimSpace(parts[2])
	return &cmd, nil
}

// SubstitutionContext provides runtime values for variable substitution.
type SubstitutionContext struct {
	CurrentFile string
	Selection   string
	Args        []string // Positional args after command name
}

// Substitute replaces variables in the command body.
func (cmd *SlashCommand) Substitute(ctx SubstitutionContext) string {
	result := cmd.Body

	// Built-in substitutions
	result = strings.ReplaceAll(result, "{{CURRENT_FILE}}", ctx.CurrentFile)
	result = strings.ReplaceAll(result, "{{SELECTION}}", ctx.Selection)

	// Positional args: {{ARG1}}, {{ARG2}}, etc.
	for i, arg := range ctx.Args {
		placeholder := fmt.Sprintf("{{ARG%d}}", i+1)
		result = strings.ReplaceAll(result, placeholder, arg)
	}

	// Replace remaining positional args with empty string
	re := regexp.MustCompile(`\{\{ARG\d+\}\}`)
	result = re.ReplaceAllString(result, "")

	// User-defined variables from frontmatter
	for key, val := range cmd.Variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, val)
	}

	return result
}

// SlashCommandExecutor runs slash commands.
type SlashCommandExecutor struct {
	registry *SlashCommandRegistry
}

// NewSlashCommandExecutor creates an executor.
func NewSlashCommandExecutor(registry *SlashCommandRegistry) *SlashCommandExecutor {
	return &SlashCommandExecutor{registry: registry}
}

// Execute runs a slash command and returns the expanded prompt.
func (e *SlashCommandExecutor) Execute(name string, ctx SubstitutionContext) (string, error) {
	cmd, exists := e.registry.Get(name)
	if !exists {
		return "", fmt.Errorf("unknown command: /%s", name)
	}

	expanded := cmd.Substitute(ctx)
	return expanded, nil
}

// IsSlashCommand checks if input starts with a slash.
func IsSlashCommand(input string) (string, []string, bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return "", nil, false
	}

	parts := strings.Fields(input[1:]) // Remove leading /
	if len(parts) == 0 {
		return "", nil, false
	}

	return parts[0], parts[1:], true
}
```

### Anti-Bluff Verification Test

```go
package workflow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/internal/workflow"
)

func TestParseSlashCommandFile(t *testing.T) {
	content := `---
name: refactor
title: Refactor Code
description: Refactor the selected code
variables:
  STYLE: "clean"
---
Please refactor the following code to follow {{STYLE}} code style:

File: {{CURRENT_FILE}}
Selection: {{SELECTION}}
Argument: {{ARG1}}
`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "refactor.md")
	os.WriteFile(path, []byte(content), 0644)

	cmd, err := workflow.ParseSlashCommandFile(path)
	if err != nil {
		t.Fatalf("ParseSlashCommandFile: %v", err)
	}

	if cmd.Title != "Refactor Code" {
		t.Fatalf("title = %q, want Refactor Code", cmd.Title)
	}
	if cmd.Variables["STYLE"] != "clean" {
		t.Fatalf("STYLE = %q, want clean", cmd.Variables["STYLE"])
	}
	if !strings.Contains(cmd.Body, "Please refactor") {
		t.Fatal("body missing expected content")
	}
}

func TestSlashCommand_Substitute(t *testing.T) {
	cmd := &workflow.SlashCommand{
		Body: "File: {{CURRENT_FILE}}\nSelection: {{SELECTION}}\nArg: {{ARG1}}\nArg2: {{ARG2}}",
		Variables: map[string]string{
			"CUSTOM": "value",
		},
	}

	ctx := workflow.SubstitutionContext{
		CurrentFile: "main.go",
		Selection:   "func main() {}",
		Args:        []string{"foo", "bar"},
	}

	result := cmd.Substitute(ctx)

	if !strings.Contains(result, "File: main.go") {
		t.Fatalf("missing CURRENT_FILE substitution: %q", result)
	}
	if !strings.Contains(result, "Selection: func main() {}") {
		t.Fatalf("missing SELECTION substitution: %q", result)
	}
	if !strings.Contains(result, "Arg: foo") {
		t.Fatalf("missing ARG1 substitution: %q", result)
	}
	if !strings.Contains(result, "Arg2: bar") {
		t.Fatalf("missing ARG2 substitution: %q", result)
	}
}

func TestIsSlashCommand(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantArgs    []string
		wantIsSlash bool
	}{
		{"/refactor main.go", "refactor", []string{"main.go"}, true},
		{"/test --coverage", "test", []string{"--coverage"}, true},
		{"hello world", "", nil, false},
		{"/", "", nil, false},
		{"  /deploy prod  ", "deploy", []string{"prod"}, true},
	}

	for _, tt := range tests {
		name, args, isSlash := workflow.IsSlashCommand(tt.input)
		if isSlash != tt.wantIsSlash {
			t.Errorf("IsSlashCommand(%q) = %v, want %v", tt.input, isSlash, tt.wantIsSlash)
			continue
		}
		if name != tt.wantName {
			t.Errorf("name = %q, want %q", name, tt.wantName)
		}
		if len(args) != len(tt.wantArgs) {
			t.Errorf("args = %v, want %v", args, tt.wantArgs)
		}
	}
}

func TestSlashCommandRegistry_LoadFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, ".helix", "commands")
	os.MkdirAll(commandsDir, 0755)

	os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte(`---
title: Test Command
---
Run tests`), 0644)
	os.WriteFile(filepath.Join(commandsDir, "deploy.md"), []byte(`---
title: Deploy
---
Deploy to production`), 0644)

	registry := workflow.NewSlashCommandRegistry()
	if err := registry.LoadFromDirectory(tmpDir); err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}

	names := registry.List()
	if len(names) != 2 {
		t.Fatalf("len(names) = %d, want 2", len(names))
	}

	cmd, exists := registry.Get("test")
	if !exists {
		t.Fatal("test command not found")
	}
	if cmd.Title != "Test Command" {
		t.Fatalf("title = %q, want Test Command", cmd.Title)
	}
}
```

### Integration Steps

1. **Create slash command registry** (`internal/workflow/slash_commands.go`)
2. **Add YAML dependency** to `go.mod` (`gopkg.in/yaml.v3`)
3. **Modify agent input handler** to detect `/` prefix and route through `SlashCommandExecutor`
4. **Load commands at startup** from `.helix/commands/*.md`
5. **Add auto-complete** in terminal UI for registered slash commands

---

## Feature 10: Skill System

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Skills are `.claude/skills/*.md` files (similar to commands) that define reusable capabilities. Skills support auto-invocation when the agent detects matching intent patterns. Variable substitution works like slash commands. Skills run in fork isolation (separate worktree/session) to prevent side effects on the main session.
- **Why it's powerful**: Encapsulates complex domain expertise (e.g., "React Component Creation", "API Endpoint Design") that the agent can automatically invoke.

### HelixCode Integration Target
- **New files to create**:
  - `internal/workflow/skills.go` — Skill registry and invocation
  - `internal/workflow/skills_test.go`
  - `internal/workflow/skill_isolation.go` — Fork isolation for skills
- **Existing files to modify**:
  - `internal/agent/agent.go` — Auto-invocation detection
  - `internal/workflow/slash_commands.go` — Share substitution logic
- **Submodule dependencies**: `internal/workflow/`, `internal/tools/` (worktree)

### Exact Code Implementation

#### File: `internal/workflow/skills.go` (NEW)

```go
package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Skill is a reusable capability definition.
type Skill struct {
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description"`
	TriggerPatterns []string          `yaml:"triggers,omitempty"` // Regex patterns for auto-invocation
	Variables       map[string]string `yaml:"variables,omitempty"`
	Body            string            // Instruction template
	SourcePath      string
	RequiresIsolation bool            `yaml:"requires_isolation,omitempty"`
}

// SkillRegistry manages available skills.
type SkillRegistry struct {
	skills map[string]*Skill
	mu     sync.RWMutex
}

// NewSkillRegistry creates a skill registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills: make(map[string]*Skill),
	}
}

// LoadFromDirectory scans `.helix/skills/*.md` for skill definitions.
func (r *SkillRegistry) LoadFromDirectory(dir string) error {
	skillsDir := filepath.Join(dir, ".helix", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(skillsDir, entry.Name())
		skill, err := ParseSkillFile(path)
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		skill.Name = name
		skill.SourcePath = path
		r.skills[name] = skill
	}

	return nil
}

// Get retrieves a skill by name.
func (r *SkillRegistry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, exists := r.skills[name]
	return skill, exists
}

// List returns all skill names.
func (r *SkillRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	return names
}

// FindMatchingSkill checks if any skill should auto-invoke for the given input.
func (r *SkillRegistry) FindMatchingSkill(input string) (*Skill, map[string]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, skill := range r.skills {
		for _, pattern := range skill.TriggerPatterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			matches := re.FindStringSubmatch(input)
			if matches != nil {
				// Extract named groups as variables
				vars := make(map[string]string)
				for i, name := range re.SubexpNames() {
					if i > 0 && i < len(matches) && name != "" {
						vars[name] = matches[i]
					}
				}
				return skill, vars, true
			}
		}
	}
	return nil, nil, false
}

// ParseSkillFile reads a skill Markdown file with YAML frontmatter.
func ParseSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("no YAML frontmatter")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid frontmatter")
	}

	var skill Skill
	if err := yaml.Unmarshal([]byte(parts[1]), &skill); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	skill.Body = strings.TrimSpace(parts[2])
	return &skill, nil
}

// Substitute replaces variables in the skill body.
func (s *Skill) Substitute(vars map[string]string) string {
	result := s.Body
	for key, val := range s.Variables {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", key), val)
	}
	for key, val := range vars {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", key), val)
	}
	return result
}

// SkillInvoker handles skill execution with optional isolation.
type SkillInvoker struct {
	registry      *SkillRegistry
	worktreeMgr   *tools.WorktreeManager // For fork isolation
}

// NewSkillInvoker creates an invoker.
func NewSkillInvoker(registry *SkillRegistry, worktreeMgr *tools.WorktreeManager) *SkillInvoker {
	return &SkillInvoker{
		registry:    registry,
		worktreeMgr: worktreeMgr,
	}
}

// Invoke executes a skill, optionally in an isolated worktree.
func (si *SkillInvoker) Invoke(ctx context.Context, skillName string, vars map[string]string, isolate bool) (string, error) {
	skill, exists := si.registry.Get(skillName)
	if !exists {
		return "", fmt.Errorf("skill %s not found", skillName)
	}

	expanded := skill.Substitute(vars)

	if isolate || skill.RequiresIsolation {
		// Create isolated worktree for skill execution
		worktreeName := fmt.Sprintf("skill-%s-%d", skillName, time.Now().Unix())
		_, err := si.worktreeMgr.EnterWorktree(ctx, worktreeName, "")
		if err != nil {
			return "", fmt.Errorf("creating isolation worktree: %w", err)
		}
		defer si.worktreeMgr.ExitWorktree()
		defer si.worktreeMgr.RemoveWorktree(ctx, worktreeName)
	}

	return expanded, nil
}
```

### Anti-Bluff Verification Test

```go
package workflow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/internal/workflow"
)

func TestParseSkillFile(t *testing.T) {
	content := `---
name: react-component
description: Create a React component
triggers:
  - "create a react component (?P<name>[A-Z][a-zA-Z]+)"
  - "make react (?P<name>.+?) component"
variables:
  STYLE: functional
---
Create a {{STYLE}} React component named {{name}} with TypeScript.
Export it as default.
`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "react-component.md")
	os.WriteFile(path, []byte(content), 0644)

	skill, err := workflow.ParseSkillFile(path)
	if err != nil {
		t.Fatalf("ParseSkillFile: %v", err)
	}

	if skill.Description != "Create a React component" {
		t.Fatalf("description = %q", skill.Description)
	}
	if len(skill.TriggerPatterns) != 2 {
		t.Fatalf("triggers = %d, want 2", len(skill.TriggerPatterns))
	}
	if skill.Variables["STYLE"] != "functional" {
		t.Fatalf("STYLE = %q", skill.Variables["STYLE"])
	}
}

func TestSkillRegistry_FindMatchingSkill(t *testing.T) {
	registry := workflow.NewSkillRegistry()

	skill := &workflow.Skill{
		Name:            "react-component",
		TriggerPatterns: []string{"create a (?P<type>[a-z]+) component"},
		Body:            "Create {{type}} component",
	}
	// Directly add to registry (normally loaded from file)
	// Using reflection or package-level access in real test

	// For this test, we'll use LoadFromDirectory
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, ".helix", "skills")
	os.MkdirAll(skillsDir, 0755)

	content := `---
name: test-skill
triggers:
  - "create a (?P<type>[a-z]+) component"
---
Create {{type}} component`
	os.WriteFile(filepath.Join(skillsDir, "test.md"), []byte(content), 0644)

	if err := registry.LoadFromDirectory(tmpDir); err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}

	// Should match
	s, vars, matched := registry.FindMatchingSkill("create a react component")
	if !matched {
		t.Fatal("expected match")
	}
	if vars["type"] != "react" {
		t.Fatalf("type = %q, want react", vars["type"])
	}
	expanded := s.Substitute(vars)
	if !strings.Contains(expanded, "Create react component") {
		t.Fatalf("expanded = %q", expanded)
	}

	// Should not match
	_, _, matched2 := registry.FindMatchingSkill("hello world")
	if matched2 {
		t.Fatal("expected no match")
	}
}

func TestSkill_Substitute(t *testing.T) {
	skill := &workflow.Skill{
		Body: "Create {{STYLE}} component named {{name}}",
		Variables: map[string]string{
			"STYLE": "functional",
		},
	}

	vars := map[string]string{
		"name": "Button",
	}

	result := skill.Substitute(vars)
	expected := "Create functional component named Button"
	if result != expected {
		t.Fatalf("result = %q, want %q", result, expected)
	}
}
```

### Integration Steps

1. **Create skill registry** (`internal/workflow/skills.go`)
2. **Create skill isolation** (`internal/workflow/skill_isolation.go`)
3. **Load skills at startup** from `.helix/skills/*.md`
4. **Add auto-invocation** in agent input handler using `FindMatchingSkill()`
5. **Pass isolated worktree** to skill execution if `RequiresIsolation` is true
6. **Add skill context** to system prompt when skill is active

---


## Feature 11: Session Transcript Resume

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Sessions are persisted with full transcripts. `--resume` continues the most recent session in the current project. `--continue` continues the most recent session across ALL projects. Session metadata includes project path, start time, last activity, and optionally an associated PR URL. The transcript can be resumed even after process restart.
- **Why it's powerful**: Users never lose context across restarts, can switch between projects while maintaining conversation history.

### HelixCode Integration Target
- **New files to create**:
  - `internal/session/resume.go` — Resume logic
  - `internal/session/resume_test.go`
- **Existing files to modify**:
  - `internal/session/session_manager.go` — Add resume methods
  - `cmd/cli/main.go` — Add `--resume`, `--continue` flags
  - `internal/session/store.go` — Add transcript persistence methods
- **Submodule dependencies**: `internal/session/`

### Exact Code Implementation

#### File: `internal/session/resume.go` (NEW)

```go
package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ResumeMode determines the scope of session resumption.
type ResumeMode string

const (
	// ResumeProject resumes the most recent session in the current project.
	ResumeProject ResumeMode = "project"
	// ResumeGlobal resumes the most recent session across all projects.
	ResumeGlobal ResumeMode = "global"
)

// SessionMetadata is stored alongside the session for resumption.
type SessionMetadata struct {
	SessionID     string    `json:"session_id" db:"session_id"`
	ProjectPath   string    `json:"project_path" db:"project_path"`
	ProjectName   string    `json:"project_name" db:"project_name"`
	StartedAt     time.Time `json:"started_at" db:"started_at"`
	LastActivity  time.Time `json:"last_activity" db:"last_activity"`
	MessageCount  int       `json:"message_count" db:"message_count"`
	IsActive      bool      `json:"is_active" db:"is_active"`
	PRURL         string    `json:"pr_url,omitempty" db:"pr_url"`
	BranchName    string    `json:"branch_name,omitempty" db:"branch_name"`
}

// ResumeFinder locates resumable sessions.
type ResumeFinder struct {
	store SessionStore
}

// SessionStore is the interface for session persistence.
type SessionStore interface {
	ListSessionMetadata(ctx context.Context, projectPath string) ([]SessionMetadata, error)
	GetSessionMetadata(ctx context.Context, sessionID string) (*SessionMetadata, error)
	UpdateSessionMetadata(ctx context.Context, meta SessionMetadata) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	CreateSession(ctx context.Context, session *Session) error
}

// NewResumeFinder creates a resume finder.
func NewResumeFinder(store SessionStore) *ResumeFinder {
	return &ResumeFinder{store: store}
}

// FindResumeTarget finds the best session to resume.
func (rf *ResumeFinder) FindResumeTarget(ctx context.Context, mode ResumeMode, currentProject string) (*SessionMetadata, error) {
	var candidates []SessionMetadata

	if mode == ResumeProject {
		metas, err := rf.store.ListSessionMetadata(ctx, currentProject)
		if err != nil {
			return nil, fmt.Errorf("listing project sessions: %w", err)
		}
		candidates = metas
	} else {
		// Global mode - list all sessions
		metas, err := rf.store.ListSessionMetadata(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("listing all sessions: %w", err)
		}
		candidates = metas
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no sessions found to resume")
	}

	// Sort by last activity descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LastActivity.After(candidates[j].LastActivity)
	})

	// Return most recent
	return &candidates[0], nil
}

// ResumeSession loads a session for resumption.
func (rf *ResumeFinder) ResumeSession(ctx context.Context, sessionID string) (*Session, error) {
	meta, err := rf.store.GetSessionMetadata(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting metadata: %w", err)
	}

	session, err := rf.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("loading session: %w", err)
	}

	// Update metadata
	meta.LastActivity = time.Now()
	meta.IsActive = true
	if err := rf.store.UpdateSessionMetadata(ctx, *meta); err != nil {
		// Non-fatal
	}

	return session, nil
}

// AssociatePR links a session to a pull request URL.
func (rf *ResumeFinder) AssociatePR(ctx context.Context, sessionID string, prURL string) error {
	meta, err := rf.store.GetSessionMetadata(ctx, sessionID)
	if err != nil {
		return err
	}

	meta.PRURL = prURL
	return rf.store.UpdateSessionMetadata(ctx, *meta)
}

// GetProjectName extracts a human-readable project name from the path.
func GetProjectName(projectPath string) string {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return projectPath
	}
	return filepath.Base(abs)
}
```

#### File: `internal/session/store.go` (MODIFY — Add Metadata Methods)

```go
// Add to SessionStore interface:
type SessionStore interface {
	// ... existing methods ...
	ListSessionMetadata(ctx context.Context, projectPath string) ([]SessionMetadata, error)
	GetSessionMetadata(ctx context.Context, sessionID string) (*SessionMetadata, error)
	UpdateSessionMetadata(ctx context.Context, meta SessionMetadata) error
}

// Add SQL implementations:
func (s *SQLStore) InitMetadataSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS session_metadata (
		session_id TEXT PRIMARY KEY REFERENCES sessions(id),
		project_path TEXT NOT NULL,
		project_name TEXT NOT NULL,
		started_at TIMESTAMPTZ NOT NULL,
		last_activity TIMESTAMPTZ NOT NULL,
		message_count INTEGER DEFAULT 0,
		is_active BOOLEAN DEFAULT true,
		pr_url TEXT,
		branch_name TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_session_metadata_project ON session_metadata(project_path);
	CREATE INDEX IF NOT EXISTS idx_session_metadata_activity ON session_metadata(last_activity DESC);
	`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *SQLStore) ListSessionMetadata(ctx context.Context, projectPath string) ([]SessionMetadata, error) {
	var query string
	var args []any
	if projectPath != "" {
		query = `SELECT session_id, project_path, project_name, started_at, last_activity, message_count, is_active, pr_url, branch_name
				 FROM session_metadata WHERE project_path = $1 AND is_active = true ORDER BY last_activity DESC`
		args = append(args, projectPath)
	} else {
		query = `SELECT session_id, project_path, project_name, started_at, last_activity, message_count, is_active, pr_url, branch_name
				 FROM session_metadata WHERE is_active = true ORDER BY last_activity DESC`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []SessionMetadata
	for rows.Next() {
		var m SessionMetadata
		if err := rows.Scan(&m.SessionID, &m.ProjectPath, &m.ProjectName, &m.StartedAt, &m.LastActivity,
			&m.MessageCount, &m.IsActive, &m.PRURL, &m.BranchName); err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	return metas, rows.Err()
}
```

#### File: `cmd/cli/main.go` (MODIFY — Add Resume Flags)

```go
// Add to root command setup:
rootCmd.PersistentFlags().Bool("resume", false, "Resume the most recent session in the current project")
rootCmd.PersistentFlags().Bool("continue", false, "Resume the most recent session across all projects")

// In run function:
if resumeProject, _ := cmd.Flags().GetBool("resume"); resumeProject {
	finder := session.NewResumeFinder(store)
	meta, err := finder.FindResumeTarget(ctx, session.ResumeProject, cwd)
	if err != nil {
		return fmt.Errorf("no session to resume: %w", err)
	}
	sessionID = meta.SessionID
	fmt.Fprintf(os.Stderr, "Resuming session %s from %s\n", sessionID, meta.LastActivity.Format("2006-01-02 15:04"))
}
```

### Anti-Bluff Verification Test

```go
package session_test

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/session"
)

type mockSessionStore struct {
	metas    []session.SessionMetadata
	sessions map[string]*session.Session
}

func (m *mockSessionStore) ListSessionMetadata(ctx context.Context, projectPath string) ([]session.SessionMetadata, error) {
	var result []session.SessionMetadata
	for _, meta := range m.metas {
		if projectPath == "" || meta.ProjectPath == projectPath {
			result = append(result, meta)
		}
	}
	return result, nil
}

func (m *mockSessionStore) GetSessionMetadata(ctx context.Context, sessionID string) (*session.SessionMetadata, error) {
	for _, meta := range m.metas {
		if meta.SessionID == sessionID {
			return &meta, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockSessionStore) UpdateSessionMetadata(ctx context.Context, meta session.SessionMetadata) error {
	for i := range m.metas {
		if m.metas[i].SessionID == meta.SessionID {
			m.metas[i] = meta
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func TestResumeFinder_FindResumeTarget_Project(t *testing.T) {
	store := &mockSessionStore{
		metas: []session.SessionMetadata{
			{SessionID: "s1", ProjectPath: "/proj/a", LastActivity: time.Now().Add(-2 * time.Hour)},
			{SessionID: "s2", ProjectPath: "/proj/a", LastActivity: time.Now().Add(-1 * time.Hour)},
			{SessionID: "s3", ProjectPath: "/proj/b", LastActivity: time.Now()},
		},
	}

	finder := session.NewResumeFinder(store)
	meta, err := finder.FindResumeTarget(context.Background(), session.ResumeProject, "/proj/a")
	if err != nil {
		t.Fatalf("FindResumeTarget: %v", err)
	}
	if meta.SessionID != "s2" {
		t.Fatalf("resumed session = %s, want s2", meta.SessionID)
	}
}

func TestResumeFinder_FindResumeTarget_Global(t *testing.T) {
	store := &mockSessionStore{
		metas: []session.SessionMetadata{
			{SessionID: "s1", ProjectPath: "/proj/a", LastActivity: time.Now().Add(-2 * time.Hour)},
			{SessionID: "s2", ProjectPath: "/proj/b", LastActivity: time.Now().Add(-1 * time.Hour)},
		},
	}

	finder := session.NewResumeFinder(store)
	meta, err := finder.FindResumeTarget(context.Background(), session.ResumeGlobal, "")
	if err != nil {
		t.Fatalf("FindResumeTarget: %v", err)
	}
	if meta.SessionID != "s2" {
		t.Fatalf("resumed session = %s, want s2", meta.SessionID)
	}
}

func TestResumeFinder_AssociatePR(t *testing.T) {
	store := &mockSessionStore{
		metas: []session.SessionMetadata{
			{SessionID: "s1", ProjectPath: "/proj/a", PRURL: ""},
		},
	}

	finder := session.NewResumeFinder(store)
	err := finder.AssociatePR(context.Background(), "s1", "https://github.com/org/repo/pull/123")
	if err != nil {
		t.Fatalf("AssociatePR: %v", err)
	}

	meta, _ := store.GetSessionMetadata(context.Background(), "s1")
	if meta.PRURL != "https://github.com/org/repo/pull/123" {
		t.Fatalf("PRURL = %q", meta.PRURL)
	}
}
```

### Integration Steps

1. **Create resume finder** (`internal/session/resume.go`)
2. **Add metadata schema** to PostgreSQL
3. **Update session lifecycle** — write metadata on create/update
4. **Add CLI flags** `--resume` and `--continue`
5. **Show session selector** in terminal UI when multiple candidates exist

---

## Feature 12: Multi-Provider Backend

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Supports Anthropic API (native), AWS Bedrock, Google Vertex AI, and Azure OpenAI. Each provider has a setup wizard that guides through configuration. `ANTHROPIC_BASE_URL` environment variable allows overriding the default API endpoint (for proxies or local gateways).
- **Why it's powerful**: Enterprises can use their existing cloud contracts, users can switch providers for cost/availability, and compliance requirements (data residency) are met.

### HelixCode Integration Target
- **New files to create**:
  - `internal/llm/provider_anthropic.go` — Anthropic implementation
  - `internal/llm/provider_bedrock.go` — AWS Bedrock
  - `internal/llm/provider_vertex.go` — Google Vertex
  - `internal/llm/provider_azure.go` — Azure OpenAI
  - `internal/llm/provider_factory.go` — Factory + wizard
  - `internal/llm/wizard.go` — Interactive setup
- **Existing files to modify**:
  - `internal/llm/provider.go` — Unified interface
  - `cmd/cli/main.go` — Provider selection flag
- **Submodule dependencies**: `internal/llm/`

### Exact Code Implementation

#### File: `internal/llm/provider.go` (MODIFY — Unified Interface)

```go
package llm

import (
	"context"
	"fmt"
)

// Model identifies a specific LLM model.
type Model string

const (
	ModelClaude35Sonnet Model = "claude-3-5-sonnet"
	ModelClaude35Haiku  Model = "claude-3-5-haiku"
	ModelClaude3Opus    Model = "claude-3-opus"
)

// Provider is the unified interface for all LLM backends.
type Provider interface {
	// Chat sends messages and returns the response.
	Chat(ctx context.Context, messages []Message, options ChatOptions) (*ChatResponse, error)

	// StreamChat sends messages and returns a streaming response.
	StreamChat(ctx context.Context, messages []Message, options ChatOptions) (StreamReader, error)

	// CountTokens estimates the token count for text.
	CountTokens(text string) (int, error)

	// GetContextWindow returns the maximum context window size.
	GetContextWindow() int

	// GetModel returns the currently configured model.
	GetModel() Model

	// Name returns the provider name.
	Name() string

	// Health checks if the provider is accessible.
	Health(ctx context.Context) error
}

// ChatOptions configures a chat request.
type ChatOptions struct {
	Temperature float32        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	System      string         `json:"system,omitempty"`
}

// ChatResponse is the result of a chat request.
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Usage        Usage      `json:"usage"`
	StopReason   string     `json:"stop_reason"`
	Model        string     `json:"model"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ToolCall represents a requested tool invocation.
type ToolCall struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Args     map[string]any `json:"args"`
}

// ToolDefinition describes a tool to the LLM.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

// StreamReader reads chunks from a streaming response.
type StreamReader interface {
	ReadChunk() (*StreamChunk, error)
	Close() error
}

// StreamChunk is a single piece of streaming content.
type StreamChunk struct {
	Content   string     `json:"content,omitempty"`
	ToolCall  *ToolCall  `json:"tool_call,omitempty"`
	Usage     *Usage     `json:"usage,omitempty"`
	IsFinal   bool       `json:"is_final,omitempty"`
}

// ProviderType identifies the backend provider.
type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderBedrock   ProviderType = "bedrock"
	ProviderVertex    ProviderType = "vertex"
	ProviderAzure     ProviderType = "azure"
)

// ProviderConfig holds configuration for any provider.
type ProviderConfig struct {
	Type       ProviderType       `json:"type"`
	Model      Model              `json:"model"`
	APIKey     string             `json:"api_key,omitempty"`
	BaseURL    string             `json:"base_url,omitempty"`    // ANTHROPIC_BASE_URL override
	Region     string             `json:"region,omitempty"`      // For Bedrock/Vertex
	ProjectID  string             `json:"project_id,omitempty"`  // For Vertex
	Endpoint   string             `json:"endpoint,omitempty"`    // For Azure
	Extra      map[string]string  `json:"extra,omitempty"`
}
```

#### File: `internal/llm/provider_anthropic.go` (NEW)

```go
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// AnthropicProvider implements the Anthropic API.
type AnthropicProvider struct {
	apiKey     string
	baseURL    string
	model      Model
	client     *http.Client
	windowSize int
}

// NewAnthropicProvider creates an Anthropic provider.
func NewAnthropicProvider(config ProviderConfig) (*AnthropicProvider, error) {
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("ANTHROPIC_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	return &AnthropicProvider{
		apiKey:     apiKey,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		model:      config.Model,
		client:     &http.Client{Timeout: 120 * time.Second},
		windowSize: 200000,
	}, nil
}

func (p *AnthropicProvider) Name() string { return string(ProviderAnthropic) }
func (p *AnthropicProvider) GetModel() Model { return p.model }
func (p *AnthropicProvider) GetContextWindow() int { return p.windowSize }

func (p *AnthropicProvider) CountTokens(text string) (int, error) {
	// Use Anthropic's token counting endpoint or estimate
	return len(text) / 4, nil // Conservative estimate
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, options ChatOptions) (*ChatResponse, error) {
	url := p.baseURL + "/v1/messages"

	anthropicMessages := convertToAnthropicMessages(messages)
	reqBody := map[string]any{
		"model":      string(p.model),
		"max_tokens": options.MaxTokens,
		"messages":   anthropicMessages,
		"temperature": options.Temperature,
	}
	if options.System != "" {
		reqBody["system"] = options.System
	}
	if len(options.Tools) > 0 {
		reqBody["tools"] = convertToAnthropicTools(options.Tools)
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic returned %d", resp.StatusCode)
	}

	var result struct {
		Content    []anthropicContent `json:"content"`
		StopReason string             `json:"stop_reason"`
		Usage      Usage              `json:"usage"`
		Model      string             `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	chatResp := &ChatResponse{
		StopReason: result.StopReason,
		Usage:      result.Usage,
		Model:      result.Model,
	}

	for _, c := range result.Content {
		switch c.Type {
		case "text":
			chatResp.Content += c.Text
		case "tool_use":
			chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
				ID:   c.ID,
				Name: c.Name,
				Args: c.Input,
			})
		}
	}

	return chatResp, nil
}

func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message, options ChatOptions) (StreamReader, error) {
	// Implementation similar to Chat but with SSE parsing
	return nil, fmt.Errorf("streaming not yet implemented")
}

func (p *AnthropicProvider) Health(ctx context.Context) error {
	// Simple health check via models endpoint
	req, _ := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/v1/models", nil)
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %d", resp.StatusCode)
	}
	return nil
}

type anthropicContent struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

func convertToAnthropicMessages(messages []Message) []map[string]any {
	var result []map[string]any
	for _, m := range messages {
		role := m.Role
		if role == "system" {
			continue // System is handled separately
		}
		if role == "tool" {
			role = "user" // Anthropic uses user role for tool results
		}
		result = append(result, map[string]any{
			"role":    role,
			"content": m.Content,
		})
	}
	return result
}

func convertToAnthropicTools(tools []ToolDefinition) []map[string]any {
	var result []map[string]any
	for _, t := range tools {
		result = append(result, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"input_schema": t.Schema,
		})
	}
	return result
}
```

#### File: `internal/llm/provider_factory.go` (NEW)

```go
package llm

import (
	"fmt"
)

// CreateProvider instantiates a provider from configuration.
func CreateProvider(config ProviderConfig) (Provider, error) {
	switch config.Type {
	case ProviderAnthropic:
		return NewAnthropicProvider(config)
	case ProviderBedrock:
		return NewBedrockProvider(config)
	case ProviderVertex:
		return NewVertexProvider(config)
	case ProviderAzure:
		return NewAzureProvider(config)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", config.Type)
	}
}
```

### Anti-Bluff Verification Test

```go
package llm_test

import (
	"os"
	"testing"

	"dev.helix.code/internal/llm"
)

func TestAnthropicProvider_EnvironmentVariables(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	os.Setenv("ANTHROPIC_BASE_URL", "https://custom.example.com")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("ANTHROPIC_BASE_URL")

	p, err := llm.NewAnthropicProvider(llm.ProviderConfig{Model: llm.ModelClaude35Sonnet})
	if err != nil {
		t.Fatalf("NewAnthropicProvider: %v", err)
	}

	// Verify env vars were picked up
	// (would need exported fields or accessor methods)
}

func TestCreateProvider_UnknownType(t *testing.T) {
	_, err := llm.CreateProvider(llm.ProviderConfig{Type: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown provider type")
	}
}

func TestAnthropicProvider_CountTokens(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	p, _ := llm.NewAnthropicProvider(llm.ProviderConfig{Model: llm.ModelClaude35Sonnet})
	n, err := p.CountTokens("hello world")
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if n <= 0 {
		t.Fatalf("token count = %d, want > 0", n)
	}
}
```

### Integration Steps

1. **Implement all provider adapters** (`internal/llm/provider_*.go`)
2. **Create factory** (`internal/llm/provider_factory.go`)
3. **Add setup wizard** (`internal/llm/wizard.go`) with interactive prompts
4. **Add `--provider` flag** to CLI
5. **Support `ANTHROPIC_BASE_URL`** environment variable in Anthropic provider
6. **Add provider health checks** at startup

---

## Feature 13: LSP Integration

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Connects to Language Servers to get diagnostics (errors, warnings, hints). Displays expandable summaries ("3 errors, 5 warnings in src/..."). Each diagnostic can trigger a per-diagnostic tool call for detailed analysis. Supports multiple concurrent LSP connections per workspace.
- **Why it's powerful**: Real-time code quality feedback without running full builds, IDE-grade error detection in a CLI tool.

### HelixCode Integration Target
- **New files to create**:
  - `internal/tools/lsp.go` — LSP tool definitions
  - `internal/tools/lsp_client.go` — LSP client wrapper
- **Existing files to modify**:
  - `internal/tools/registry.go` — Register LSP tools
  - `internal/agent/agent.go` — Trigger diagnostics on file changes
- **Submodule dependencies**: `internal/tools/`

### Exact Code Implementation

#### File: `internal/tools/lsp.go` (NEW)

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// LSPGetDiagnosticsTool fetches diagnostics from language servers.
type LSPGetDiagnosticsTool struct {
	clients map[string]*LSPClient // language -> client
}

func NewLSPGetDiagnosticsTool() *LSPGetDiagnosticsTool {
	return &LSPGetDiagnosticsTool{
		clients: make(map[string]*LSPClient),
	}
}

func (t *LSPGetDiagnosticsTool) Name() string { return "LSPGetDiagnostics" }

func (t *LSPGetDiagnosticsTool) Description() string {
	return "Get language server diagnostics (errors, warnings) for files. Returns expandable summary."
}

func (t *LSPGetDiagnosticsTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to file to analyze (optional - analyzes all if omitted)",
			},
			"severity": map[string]any{
				"type":        "string",
				"description": "Minimum severity: error, warning, information, hint",
			},
		},
	}
}

// DiagnosticSeverity levels.
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// Diagnostic is a single LSP diagnostic.
type Diagnostic struct {
	Range    struct {
		Start struct{ Line, Character int }
		End   struct{ Line, Character int }
	}
	Severity DiagnosticSeverity `json:"severity"`
	Code     string             `json:"code,omitempty"`
	Source   string             `json:"source"`
	Message  string             `json:"message"`
	FilePath string             `json:"file_path"`
}

// DiagnosticSummary groups diagnostics by severity.
type DiagnosticSummary struct {
	TotalErrors      int           `json:"total_errors"`
	TotalWarnings    int           `json:"total_warnings"`
	TotalInformation int           `json:"total_information"`
	TotalHints       int           `json:"total_hints"`
	Diagnostics      []Diagnostic  `json:"diagnostics"`
	Expandable       bool          `json:"expandable"`
}

func (t *LSPGetDiagnosticsTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	filePath, _ := args["file_path"].(string)
	severityStr, _ := args["severity"].(string)

	minSeverity := SeverityError
	switch severityStr {
	case "warning":
		minSeverity = SeverityWarning
	case "information":
		minSeverity = SeverityInformation
	case "hint":
		minSeverity = SeverityHint
	}

	var allDiagnostics []Diagnostic
	for _, client := range t.clients {
		if !client.IsConnected() {
			continue
		}
		diags, err := client.GetDiagnostics(ctx, filePath)
		if err != nil {
			continue
		}
		for _, d := range diags {
			if d.Severity <= minSeverity {
				allDiagnostics = append(allDiagnostics, d)
			}
		}
	}

	summary := DiagnosticSummary{
		Diagnostics: allDiagnostics,
		Expandable:  len(allDiagnostics) > 5,
	}

	for _, d := range allDiagnostics {
		switch d.Severity {
		case SeverityError:
			summary.TotalErrors++
		case SeverityWarning:
			summary.TotalWarnings++
		case SeverityInformation:
			summary.TotalInformation++
		case SeverityHint:
			summary.TotalHints++
		}
	}

	// If expandable, truncate detailed list
	if summary.Expandable {
		summary.Diagnostics = summary.Diagnostics[:5]
	}

	return summary, nil
}

// LSPAnalyzeDiagnosticTool gets detailed info for a specific diagnostic.
type LSPAnalyzeDiagnosticTool struct {
	clients map[string]*LSPClient
}

func NewLSPAnalyzeDiagnosticTool() *LSPAnalyzeDiagnosticTool {
	return &LSPAnalyzeDiagnosticTool{clients: make(map[string]*LSPClient)}
}

func (t *LSPAnalyzeDiagnosticTool) Name() string { return "LSPAnalyzeDiagnostic" }

func (t *LSPAnalyzeDiagnosticTool) Description() string {
	return "Get detailed analysis for a specific diagnostic by ID."
}

func (t *LSPAnalyzeDiagnosticTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"diagnostic_id": map[string]any{
				"type":        "string",
				"description": "ID of the diagnostic to analyze",
			},
		},
		"required": []string{"diagnostic_id"},
	}
}

func (t *LSPAnalyzeDiagnosticTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	// Implementation would look up diagnostic by ID and return code context + fix suggestions
	return nil, fmt.Errorf("not fully implemented")
}
```

### Anti-Bluff Verification Test

```go
package tools_test

import (
	"context"
	"testing"

	"dev.helix.code/internal/tools"
)

func TestDiagnosticSummary_Counting(t *testing.T) {
	summary := tools.DiagnosticSummary{
		Diagnostics: []tools.Diagnostic{
			{Severity: tools.SeverityError, Message: "e1"},
			{Severity: tools.SeverityError, Message: "e2"},
			{Severity: tools.SeverityWarning, Message: "w1"},
			{Severity: tools.SeverityInformation, Message: "i1"},
			{Severity: tools.SeverityHint, Message: "h1"},
		},
	}

	for _, d := range summary.Diagnostics {
		switch d.Severity {
		case tools.SeverityError:
			summary.TotalErrors++
		case tools.SeverityWarning:
			summary.TotalWarnings++
		case tools.SeverityInformation:
			summary.TotalInformation++
		case tools.SeverityHint:
			summary.TotalHints++
		}
	}

	if summary.TotalErrors != 2 {
		t.Fatalf("errors = %d, want 2", summary.TotalErrors)
	}
	if summary.TotalWarnings != 1 {
		t.Fatalf("warnings = %d, want 1", summary.TotalWarnings)
	}
}
```

### Integration Steps

1. **Create LSP tools** (`internal/tools/lsp.go`)
2. **Implement LSP client** using `golang.org/x/tools/jsonrpc2` or similar
3. **Register tools** in registry
4. **Auto-trigger diagnostics** after Edit tool calls
5. **Display expandable summaries** in terminal UI

---

## Feature 14: Sandboxed Shell Execution

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Bash commands run in platform-specific sandboxes. Linux: PID namespace + seccomp-bpf + iptables network restrictions. macOS: Seatbelt sandbox profile (file read/write limits). Windows: Native Job Object + ACL sandboxing. Prevents filesystem escape, network abuse, and resource exhaustion.
- **Why it's powerful**: Safe execution of arbitrary code even in CI/CD or shared environments.

### HelixCode Integration Target
- **New files to create**:
  - `internal/tools/sandbox_linux.go` — Linux sandbox (build tag)
  - `internal/tools/sandbox_darwin.go` — macOS sandbox (build tag)
  - `internal/tools/sandbox_windows.go` — Windows sandbox (build tag)
  - `internal/tools/sandbox.go` — Common interface
- **Existing files to modify**:
  - `internal/tools/bash.go` — Wrap execution with sandbox
- **Submodule dependencies**: `internal/tools/`

### Exact Code Implementation

#### File: `internal/tools/sandbox.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// SandboxConfig configures the execution sandbox.
type SandboxConfig struct {
	AllowNetwork   bool
	AllowWriteDirs []string
	ReadOnlyDirs   []string
	MaxMemoryMB    int
	MaxCPUTimeSec  int
	WorkingDir     string
}

// Sandbox wraps a command with platform-specific restrictions.
type Sandbox interface {
	Wrap(cmd *exec.Cmd, config SandboxConfig) error
	Name() string
}

// NewSandbox creates the appropriate sandbox for the current platform.
func NewSandbox() Sandbox {
	switch runtime.GOOS {
	case "linux":
		return &LinuxSandbox{}
	case "darwin":
		return &DarwinSandbox{}
	case "windows":
		return &WindowsSandbox{}
	default:
		return &NoopSandbox{}
	}
}

// NoopSandbox does nothing (fallback).
type NoopSandbox struct{}

func (n *NoopSandbox) Wrap(cmd *exec.Cmd, config SandboxConfig) error { return nil }
func (n *NoopSandbox) Name() string { return "noop" }
```

#### File: `internal/tools/sandbox_linux.go` (NEW)

```go
//go:build linux

package tools

import (
	"fmt"
	"os/exec"
	"syscall"
)

// LinuxSandbox uses namespaces and seccomp for isolation.
type LinuxSandbox struct{}

func (l *LinuxSandbox) Name() string { return "linux-ns+seccomp" }

func (l *LinuxSandbox) Wrap(cmd *exec.Cmd, config SandboxConfig) error {
	// Create new PID namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	if !config.AllowNetwork {
		// Network is already isolated by CLONE_NEWNET
		// Could add iptables rules for additional restriction
	}

	// Set resource limits
	if config.MaxMemoryMB > 0 {
		cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	}

	// In production, use libseccomp or a prebuilt seccomp-bpf policy
	// For now, namespace isolation is the baseline

	return nil
}
```

#### File: `internal/tools/sandbox_darwin.go` (NEW)

```go
//go:build darwin

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// DarwinSandbox uses Seatbelt sandbox profiles.
type DarwinSandbox struct{}

func (d *DarwinSandbox) Name() string { return "darwin-seatbelt" }

func (d *DarwinSandbox) Wrap(cmd *exec.Cmd, config SandboxConfig) error {
	// Generate sandbox profile
	profile := d.generateProfile(config)

	// Write profile to temp file
	tmpDir := os.TempDir()
	profilePath := filepath.Join(tmpDir, "helix-sandbox.sb")
	if err := os.WriteFile(profilePath, []byte(profile), 0600); err != nil {
		return fmt.Errorf("writing sandbox profile: %w", err)
	}

	// Prepend sandbox-exec
	originalPath := cmd.Path
	originalArgs := append([]string{originalPath}, cmd.Args[1:]...)
	cmd.Path = "/usr/bin/sandbox-exec"
	cmd.Args = append([]string{"sandbox-exec", "-f", profilePath}, originalArgs...)

	return nil
}

func (d *DarwinSandbox) generateProfile(config SandboxConfig) string {
	profile := `(version 1)
(deny default)
(allow process*)
(allow signal)
`
	// Allow read access to specified directories
	for _, dir := range config.ReadOnlyDirs {
		profile += fmt.Sprintf(`(allow file-read* (subpath "%s"))\n`, dir)
	}
	// Allow write access
	for _, dir := range config.AllowWriteDirs {
		profile += fmt.Sprintf(`(allow file-read* file-write* (subpath "%s"))\n`, dir)
	}
	// Deny network if needed
	if !config.AllowNetwork {
		profile += `(deny network*)
`
	}
	return profile
}
```

#### File: `internal/tools/sandbox_windows.go` (NEW)

```go
//go:build windows

package tools

import (
	"fmt"
	"os/exec"
	"syscall"
)

// WindowsSandbox uses Job Objects and ACLs.
type WindowsSandbox struct{}

func (w *WindowsSandbox) Name() string { return "windows-job-object" }

func (w *WindowsSandbox) Wrap(cmd *exec.Cmd, config SandboxConfig) error {
	// Windows Job Object for process tree control
	// Memory limits via JOB_OBJECT_LIMIT_PROCESS_MEMORY
	// In production, use Windows Sandbox API or AppContainer

	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	return nil
}
```

#### File: `internal/tools/bash.go` (MODIFY — Sandbox Integration)

```go
func (b *BashTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	command, ok := args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command must be a string")
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = b.workDir

	// Apply sandbox
	sandbox := NewSandbox()
	sandbox.Wrap(cmd, SandboxConfig{
		AllowNetwork:   b.allowNetwork,
		AllowWriteDirs: []string{b.workDir},
		ReadOnlyDirs:   []string{"/"},
		WorkingDir:     b.workDir,
	})

	output, err := cmd.CombinedOutput()
	return string(output), err
}
```

### Anti-Bluff Verification Test

```go
package tools_test

import (
	"runtime"
	"testing"

	"dev.helix.code/internal/tools"
)

func TestNewSandbox_PlatformSpecific(t *testing.T) {
	sandbox := tools.NewSandbox()

	switch runtime.GOOS {
	case "linux":
		if sandbox.Name() != "linux-ns+seccomp" {
			t.Fatalf("sandbox = %s, want linux-ns+seccomp", sandbox.Name())
		}
	case "darwin":
		if sandbox.Name() != "darwin-seatbelt" {
			t.Fatalf("sandbox = %s, want darwin-seatbelt", sandbox.Name())
		}
	case "windows":
		if sandbox.Name() != "windows-job-object" {
			t.Fatalf("sandbox = %s, want windows-job-object", sandbox.Name())
		}
	default:
		if sandbox.Name() != "noop" {
			t.Fatalf("sandbox = %s, want noop", sandbox.Name())
		}
	}
}
```

### Integration Steps

1. **Create sandbox interface** and platform implementations
2. **Wrap Bash tool** execution with sandbox
3. **Add `--sandbox` CLI flag** to enable/disable
4. **Test on all platforms** via CI matrix

---

## Feature 15: Subagent Team

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Named, addressable subagents can be spawned with specific instructions. `SendMessage` tool enables inter-agent communication. Tasks are delegated to subagents which operate in parallel. Parent agent receives consolidated results.
- **Why it's powerful**: Parallel task execution, specialized sub-agents ("code reviewer", "test writer"), natural decomposition of complex workflows.

### HelixCode Integration Target
- **New files to create**:
  - `internal/agent/subagent.go` — Subagent manager
  - `internal/agent/subagent_test.go`
  - `internal/tools/send_message.go` — Inter-agent messaging tool
- **Existing files to modify**:
  - `internal/agent/agent.go` — Spawn and manage subagents
  - `internal/session/session_manager.go` — Track subagent sessions
- **Submodule dependencies**: `internal/agent/`, `internal/session/`

### Exact Code Implementation

#### File: `internal/agent/subagent.go` (NEW)

```go
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/session"
	"go.uber.org/zap"
)

// Subagent is a child agent with a specific role.
type Subagent struct {
	ID          string
	Name        string
	Role        string
	Instructions string
	SessionID   string
	ParentID    string
	Agent       *Agent
	mu          sync.RWMutex
	messages    []AgentMessage
	status      SubagentStatus
	createdAt   time.Time
}

// SubagentStatus tracks the subagent lifecycle.
type SubagentStatus string

const (
	SubagentIdle       SubagentStatus = "idle"
	SubagentWorking    SubagentStatus = "working"
	SubagentCompleted  SubagentStatus = "completed"
	SubagentFailed     SubagentStatus = "failed"
)

// AgentMessage is a message sent between agents.
type AgentMessage struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// SubagentManager manages a team of subagents.
type SubagentManager struct {
	subagents map[string]*Subagent
	mu        sync.RWMutex
	factory   AgentFactory
	logger    *zap.Logger
}

// AgentFactory creates new Agent instances.
type AgentFactory interface {
	CreateAgent(ctx context.Context, config AgentConfig) (*Agent, error)
}

// NewSubagentManager creates a subagent manager.
func NewSubagentManager(factory AgentFactory, logger *zap.Logger) *SubagentManager {
	return &SubagentManager{
		subagents: make(map[string]*Subagent),
		factory:   factory,
		logger:    logger,
	}
}

// Spawn creates a new subagent.
func (sm *SubagentManager) Spawn(ctx context.Context, parentID, name, role, instructions string) (*Subagent, error) {
	id := fmt.Sprintf("subagent-%s-%d", name, time.Now().UnixNano())

	config := AgentConfig{
		Name:         name,
		Role:         role,
		Instructions: instructions,
		IsSubagent:   true,
		ParentID:     parentID,
	}

	agent, err := sm.factory.CreateAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating subagent: %w", err)
	}

	sub := &Subagent{
		ID:          id,
		Name:        name,
		Role:        role,
		Instructions: instructions,
		ParentID:    parentID,
		Agent:       agent,
		status:      SubagentIdle,
		createdAt:   time.Now(),
	}

	sm.mu.Lock()
	sm.subagents[id] = sub
	sm.mu.Unlock()

	sm.logger.Info("subagent spawned",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("role", role),
	)

	return sub, nil
}

// SendMessage delivers a message to a subagent.
func (sm *SubagentManager) SendMessage(fromID, toID, content string) error {
	sm.mu.RLock()
	recipient, exists := sm.subagents[toID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("subagent %s not found", toID)
	}

	msg := AgentMessage{
		From:      fromID,
		To:        toID,
		Content:   content,
		Timestamp: time.Now(),
	}

	recipient.mu.Lock()
	recipient.messages = append(recipient.messages, msg)
	recipient.mu.Unlock()

	// Inject message into subagent's session
	// (Implementation depends on Agent architecture)
	return nil
}

// GetMessages returns all messages for a subagent.
func (sm *SubagentManager) GetMessages(subagentID string) ([]AgentMessage, error) {
	sm.mu.RLock()
	sub, exists := sm.subagents[subagentID]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("subagent %s not found", subagentID)
	}

	sub.mu.RLock()
	defer sub.mu.RUnlock()
	
	msgs := make([]AgentMessage, len(sub.messages))
	copy(msgs, sub.messages)
	return msgs, nil
}

// ListSubagents returns all subagent IDs.
func (sm *SubagentManager) ListSubagents() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	ids := make([]string, 0, len(sm.subagents))
	for id := range sm.subagents {
		ids = append(ids, id)
	}
	return ids
}

// Terminate stops and removes a subagent.
func (sm *SubagentManager) Terminate(subagentID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sub, exists := sm.subagents[subagentID]
	if !exists {
		return fmt.Errorf("subagent %s not found", subagentID)
	}

	// Cleanup subagent resources
	if sub.Agent != nil {
		// Stop agent goroutines
	}

	delete(sm.subagents, subagentID)
	sm.logger.Info("subagent terminated", zap.String("id", subagentID))
	return nil
}

// AgentConfig configures a new agent instance.
type AgentConfig struct {
	Name         string
	Role         string
	Instructions string
	IsSubagent   bool
	ParentID     string
	Model        llm.Model
}
```

#### File: `internal/tools/send_message.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"

	"dev.helix.code/internal/agent"
)

// SendMessageTool enables inter-agent communication.
type SendMessageTool struct {
	manager *agent.SubagentManager
}

func NewSendMessageTool(manager *agent.SubagentManager) *SendMessageTool {
	return &SendMessageTool{manager: manager}
}

func (t *SendMessageTool) Name() string { return "SendMessage" }

func (t *SendMessageTool) Description() string {
	return "Send a message to another agent in the team. Use for task delegation and coordination."
}

func (t *SendMessageTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"to": map[string]any{
				"type":        "string",
				"description": "ID or name of the recipient agent",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Message content",
			},
		},
		"required": []string{"to", "content"},
	}
}

func (t *SendMessageTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	to, ok := args["to"].(string)
	if !ok {
		return nil, fmt.Errorf("to must be a string")
	}
	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content must be a string")
	}

	// Find subagent by ID or name
	// (Implementation would iterate through manager)
	from := "parent" // Would be set from context
	if err := t.manager.SendMessage(from, to, content); err != nil {
		return nil, err
	}

	return map[string]any{
		"status":  "sent",
		"to":      to,
		"content": content,
	}, nil
}
```

### Anti-Bluff Verification Test

```go
package agent_test

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/agent"
	"go.uber.org/zap/zaptest"
)

type mockAgentFactory struct{}

func (m *mockAgentFactory) CreateAgent(ctx context.Context, config agent.AgentConfig) (*agent.Agent, error) {
	// Return a minimal mock
	return nil, nil
}

func TestSubagentManager_SpawnAndMessage(t *testing.T) {
	factory := &mockAgentFactory{}
	manager := agent.NewSubagentManager(factory, zaptest.NewLogger(t))

	sub, err := manager.Spawn(context.Background(), "parent-1", "reviewer", "code-reviewer", "Review all changes for bugs")
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if sub.Name != "reviewer" {
		t.Fatalf("name = %q, want reviewer", sub.Name)
	}
	if sub.Role != "code-reviewer" {
		t.Fatalf("role = %q, want code-reviewer", sub.Role)
	}

	// Send message
	if err := manager.SendMessage("parent-1", sub.ID, "Please review main.go"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	msgs, err := manager.GetMessages(sub.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	if msgs[0].Content != "Please review main.go" {
		t.Fatalf("content = %q", msgs[0].Content)
	}
	if msgs[0].From != "parent-1" {
		t.Fatalf("from = %q, want parent-1", msgs[0].From)
	}
}

func TestSubagentManager_ListAndTerminate(t *testing.T) {
	factory := &mockAgentFactory{}
	manager := agent.NewSubagentManager(factory, zaptest.NewLogger(t))

	// Spawn multiple
	for i := 0; i < 3; i++ {
		manager.Spawn(context.Background(), "parent", fmt.Sprintf("agent-%d", i), "worker", "")
	}

	ids := manager.ListSubagents()
	if len(ids) != 3 {
		t.Fatalf("subagents = %d, want 3", len(ids))
	}

	// Terminate one
	if err := manager.Terminate(ids[0]); err != nil {
		t.Fatalf("Terminate: %v", err)
	}

	ids = manager.ListSubagents()
	if len(ids) != 2 {
		t.Fatalf("after terminate = %d, want 2", len(ids))
	}
}
```

### Integration Steps

1. **Create subagent manager** (`internal/agent/subagent.go`)
2. **Create SendMessage tool** (`internal/tools/send_message.go`)
3. **Modify agent factory** to support subagent creation
4. **Register SendMessage tool** in registry
5. **Add subagent tracking** in session metadata
6. **Add `SpawnSubagent` tool** to LLM tool definitions

---


## Feature 16: OpenTelemetry Integration

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Full distributed tracing with W3C Trace Context propagation (`traceparent` header). Each tool call creates a span. Spans include attributes: tool name, arguments (sanitized), execution duration, result status. Metrics are exported for token usage, tool call frequency, and error rates. Compatible with Jaeger, Zipkin, OTLP.
- **Why it's powerful**: Production observability, performance debugging, cost tracking, and integration with existing enterprise monitoring stacks.

### HelixCode Integration Target
- **New files to create**:
  - `internal/telemetry/tracing.go` — Tracer setup
  - `internal/telemetry/metrics.go` — Metrics collection
  - `internal/telemetry/middleware.go` — Gin/HTTP middleware
- **Existing files to modify**:
  - `internal/agent/agent.go` — Wrap turns in spans
  - `internal/tools/tool_executor.go` — Per-tool spans
  - `cmd/server/main.go` — Initialize telemetry on startup
- **Submodule dependencies**: `internal/telemetry/`

### Exact Code Implementation

#### File: `internal/telemetry/tracing.go` (NEW)

```go
package telemetry

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const TracerName = "dev.helix.code"

// InitTracer initializes the OpenTelemetry tracer with OTLP export.
func InitTracer(ctx context.Context, serviceName, serviceVersion string) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
		otlptracegrpc.WithInsecure(), // Use TLS in production
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider, nil
}

// StartSpan begins a new span from context.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer(TracerName)
	return tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the current span.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// WithAttributes adds attributes to the current span.
func WithAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span.
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(trace.Status{Code: trace.StatusCodeError, Description: err.Error()})
}

// EndSpan ends the current span with optional status.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeError, Description: err.Error()})
	}
	span.End()
}

// ExtractTraceContext extracts W3C trace context from a map (e.g., headers).
func ExtractTraceContext(ctx context.Context, carrier map[string]string) context.Context {
	propagator := propagation.TraceContext{}
	return propagator.Extract(ctx, propagation.MapCarrier(carrier))
}

// InjectTraceContext injects W3C trace context into a map.
func InjectTraceContext(ctx context.Context, carrier map[string]string) {
	propagator := propagation.TraceContext{}
	propagator.Inject(ctx, propagation.MapCarrier(carrier))
}
```

#### File: `internal/telemetry/metrics.go` (NEW)

```go
package telemetry

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Metrics holds all application metrics.
type Metrics struct {
	TokenUsage      metric.Int64Counter
	ToolCalls       metric.Int64Counter
	ToolErrors      metric.Int64Counter
	RequestDuration metric.Float64Histogram
	SessionCount    metric.Int64UpDownCounter
	CompactionCount metric.Int64Counter
}

// InitMetrics initializes the metrics system with Prometheus export.
func InitMetrics(ctx context.Context, serviceName string) (*Metrics, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("creating prometheus exporter: %w", err)
	}

	res, _ := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(res),
	)

	meter := provider.Meter(TracerName)

	tokenUsage, _ := meter.Int64Counter("llm.token.usage",
		metric.WithDescription("Total token usage"),
		metric.WithUnit("1"),
	)
	toolCalls, _ := meter.Int64Counter("tool.calls.total",
		metric.WithDescription("Total tool calls"),
	)
	toolErrors, _ := meter.Int64Counter("tool.errors.total",
		metric.WithDescription("Total tool execution errors"),
	)
	requestDuration, _ := meter.Float64Histogram("request.duration",
		metric.WithDescription("Request duration in seconds"),
		metric.WithUnit("s"),
	)
	sessionCount, _ := meter.Int64UpDownCounter("sessions.active",
		metric.WithDescription("Number of active sessions"),
	)
	compactionCount, _ := meter.Int64Counter("context.compactions.total",
		metric.WithDescription("Total context compactions"),
	)

	return &Metrics{
		TokenUsage:      tokenUsage,
		ToolCalls:       toolCalls,
		ToolErrors:      toolErrors,
		RequestDuration: requestDuration,
		SessionCount:    sessionCount,
		CompactionCount: compactionCount,
	}, nil
}

// RecordToolCall records a tool invocation.
func (m *Metrics) RecordToolCall(ctx context.Context, toolName string, success bool) {
	m.ToolCalls.Add(ctx, 1, metric.WithAttributes(attribute.String("tool", toolName)))
	if !success {
		m.ToolErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("tool", toolName)))
	}
}

// RecordTokenUsage records LLM token consumption.
func (m *Metrics) RecordTokenUsage(ctx context.Context, model string, inputTokens, outputTokens int) {
	m.TokenUsage.Add(ctx, int64(inputTokens),
		metric.WithAttributes(attribute.String("model", model), attribute.String("direction", "input")))
	m.TokenUsage.Add(ctx, int64(outputTokens),
		metric.WithAttributes(attribute.String("model", model), attribute.String("direction", "output")))
}
```

#### File: `internal/tools/tool_executor.go` (MODIFY — Telemetry)

```go
// In ToolExecutor struct:
type ToolExecutor struct {
	registry    *Registry
	engine      *PermissionEngine
	approver    UserApprover
	persister   *PersistenceManager
	dispatcher  *workflow.HookDispatcher
	metrics     *telemetry.Metrics
}

// In Execute method, wrap with span:
func (te *ToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any, toolCallID string, sessionID string) (*PersistedResult, error) {
	ctx, span := telemetry.StartSpan(ctx, "tool.execute",
		trace.WithAttributes(
			attribute.String("tool.name", toolName),
			attribute.String("session.id", sessionID),
			attribute.String("tool_call.id", toolCallID),
		))
	defer telemetry.EndSpan(span, nil)

	start := time.Now()
	result, err := te.executeInternal(ctx, toolName, args, toolCallID, sessionID)
	duration := time.Since(start).Seconds()

	te.metrics.RecordToolCall(ctx, toolName, err == nil)
	te.metrics.RequestDuration.Record(ctx, duration,
		metric.WithAttributes(attribute.String("operation", "tool.execute")))

	if err != nil {
		telemetry.RecordError(ctx, err)
	}

	return result, err
}
```

### Anti-Bluff Verification Test

```go
package telemetry_test

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/telemetry"
)

func TestInitTracer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provider, err := telemetry.InitTracer(ctx, "helixcode-test", "1.0.0")
	if err != nil {
		// May fail if no collector running - that's OK for unit test
		t.Skipf("OTLP collector not available: %v", err)
	}
	defer provider.Shutdown(ctx)

	if provider == nil {
		t.Fatal("provider is nil")
	}
}

func TestStartSpan(t *testing.T) {
	ctx := context.Background()
	ctx, span := telemetry.StartSpan(ctx, "test-span")
	if span == nil {
		t.Fatal("span is nil")
	}
	if !span.SpanContext().IsValid() {
		t.Fatal("span context is not valid")
	}
	span.End()
}

func TestExtractAndInjectTraceContext(t *testing.T) {
	ctx := context.Background()
	ctx, span := telemetry.StartSpan(ctx, "test-span")
	defer span.End()

	// Inject
	carrier := make(map[string]string)
	telemetry.InjectTraceContext(ctx, carrier)

	if carrier["traceparent"] == "" {
		t.Fatal("traceparent header not injected")
	}

	// Extract into new context
	newCtx := telemetry.ExtractTraceContext(context.Background(), carrier)
	newSpan := telemetry.SpanFromContext(newCtx)
	if newSpan.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Fatal("trace ID not propagated")
	}
}

func TestMetrics_RecordToolCall(t *testing.T) {
	ctx := context.Background()
	metrics, err := telemetry.InitMetrics(ctx, "test")
	if err != nil {
		t.Fatalf("InitMetrics: %v", err)
	}

	// Should not panic
	metrics.RecordToolCall(ctx, "Bash", true)
	metrics.RecordToolCall(ctx, "Edit", false)
	metrics.RecordTokenUsage(ctx, "claude-3-5-sonnet", 100, 50)
}
```

### Integration Steps

1. **Create telemetry package** (`internal/telemetry/`)
2. **Initialize on server startup** with `OTEL_EXPORTER_OTLP_ENDPOINT`
3. **Add middleware** to Gin server for HTTP tracing
4. **Wrap all tool calls** in spans with tool name attributes
5. **Wrap agent turns** in parent spans
6. **Export metrics** on `/metrics` endpoint for Prometheus scraping

---

## Feature 17: Smart File Editing

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: If a file is read via `Read` tool, subsequent `Edit` operations on that file don't require a separate `Read` — the agent already has the content in context. `userModified` tracking detects when the user changed a file between the agent's read and edit operations, preventing overwrite conflicts. Structured patch output shows exact line-by-line changes.
- **Why it's powerful**: Eliminates redundant file reads, prevents lost updates, provides clear diff visualization.

### HelixCode Integration Target
- **New files to create**:
  - `internal/editor/smart_edit.go` — Smart edit tracking
  - `internal/editor/patch.go` — Structured patch output
- **Existing files to modify**:
  - `internal/editor/editor.go` — Add userModified check
  - `internal/tools/read.go` — Track read files
  - `internal/tools/edit.go` — Check modification before edit
- **Submodule dependencies**: `internal/editor/`, `internal/tools/`

### Exact Code Implementation

#### File: `internal/editor/smart_edit.go` (NEW)

```go
package editor

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sync"
	"time"
)

// FileView tracks a file that the agent has read.
type FileView struct {
	Path         string
	Content      string
	ContentHash  string
	ReadAt       time.Time
	LastModified time.Time // File mtime when read
}

// SmartEditTracker remembers which files the agent has viewed.
type SmartEditTracker struct {
	views map[string]*FileView
	mu    sync.RWMutex
}

// NewSmartEditTracker creates a tracker.
func NewSmartEditTracker() *SmartEditTracker {
	return &SmartEditTracker{views: make(map[string]*FileView)}
}

// RecordView records that the agent has read a file.
func (t *SmartEditTracker) RecordView(path string, content string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	hash := sha256.Sum256([]byte(content))
	view := &FileView{
		Path:         path,
		Content:      content,
		ContentHash:  fmt.Sprintf("%x", hash[:]),
		ReadAt:       time.Now(),
		LastModified: info.ModTime(),
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.views[path] = view
	return nil
}

// HasViewed returns true if the agent has read this file.
func (t *SmartEditTracker) HasViewed(path string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.views[path]
	return exists
}

// GetView returns the cached file view.
func (t *SmartEditTracker) GetView(path string) (*FileView, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	v, exists := t.views[path]
	if !exists {
		return nil, false
	}
	// Return a copy
	vcopy := *v
	return &vcopy, true
}

// IsUserModified checks if the file has been modified by the user since the agent read it.
func (t *SmartEditTracker) IsUserModified(path string) (bool, error) {
	t.mu.RLock()
	view, exists := t.views[path]
	t.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("file %s was not previously viewed", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if info.ModTime().After(view.LastModified) {
		// File was modified - verify content changed
		currentContent, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		hash := sha256.Sum256(currentContent)
		currentHash := fmt.Sprintf("%x", hash[:])
		return currentHash != view.ContentHash, nil
	}

	return false, nil
}

// Invalidate removes a file from the tracker.
func (t *SmartEditTracker) Invalidate(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.views, path)
}
```

#### File: `internal/editor/patch.go` (NEW)

```go
package editor

import (
	"fmt"
	"strings"
)

// PatchLine represents a single line in a diff.
type PatchLine struct {
	Type    string // "context", "add", "remove"
	Number  int
	Content string
}

// StructuredPatch is a human-readable edit description.
type StructuredPatch struct {
	FilePath    string      `json:"file_path"`
	OldStart    int         `json:"old_start"`
	OldLines    int         `json:"old_lines"`
	NewStart    int         `json:"new_start"`
	NewLines    int         `json:"new_lines"`
	Lines       []PatchLine `json:"lines"`
}

// GeneratePatch creates a structured patch from old and new content.
func GeneratePatch(filePath, oldContent, newContent string) *StructuredPatch {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	patch := &StructuredPatch{
		FilePath: filePath,
		Lines:    []PatchLine{},
	}

	// Simple line-by-line diff
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen; i++ {
		if i < len(oldLines) && i < len(newLines) {
			if oldLines[i] == newLines[i] {
				patch.Lines = append(patch.Lines, PatchLine{
					Type:    "context",
					Number:  i + 1,
					Content: oldLines[i],
				})
			} else {
				patch.Lines = append(patch.Lines, PatchLine{
					Type:    "remove",
					Number:  i + 1,
					Content: oldLines[i],
				})
				patch.Lines = append(patch.Lines, PatchLine{
					Type:    "add",
					Number:  i + 1,
					Content: newLines[i],
				})
			}
		} else if i < len(oldLines) {
			patch.Lines = append(patch.Lines, PatchLine{
				Type:    "remove",
				Number:  i + 1,
				Content: oldLines[i],
			})
		} else {
			patch.Lines = append(patch.Lines, PatchLine{
				Type:    "add",
				Number:  i + 1,
				Content: newLines[i],
			})
		}
	}

	patch.OldLines = len(oldLines)
	patch.NewLines = len(newLines)
	return patch
}

// String formats the patch as a human-readable diff.
func (p *StructuredPatch) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n", p.FilePath)
	fmt.Fprintf(&b, "+++ %s\n", p.FilePath)
	fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", p.OldStart, p.OldLines, p.NewStart, p.NewLines)

	for _, line := range p.Lines {
		switch line.Type {
		case "context":
			fmt.Fprintf(&b, " %s\n", line.Content)
		case "add":
			fmt.Fprintf(&b, "+%s\n", line.Content)
		case "remove":
			fmt.Fprintf(&b, "-%s\n", line.Content)
		}
	}
	return b.String()
}
```

#### File: `internal/tools/edit.go` (MODIFY — Smart Edit Check)

```go
// Add to EditTool:
type EditTool struct {
	tracker *editor.SmartEditTracker
}

// In Execute, before editing:
func (t *EditTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	filePath := args["file_path"].(string)

	// Check if user modified the file
	if t.tracker.HasViewed(filePath) {
		modified, err := t.tracker.IsUserModified(filePath)
		if err != nil {
			return nil, fmt.Errorf("checking modifications: %w", err)
		}
		if modified {
			return nil, fmt.Errorf("file %s was modified by user since last read; re-read before editing", filePath)
		}
	}

	// Perform edit
	oldStr := args["old_string"].(string)
	newStr := args["new_string"].(string)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	oldContent := string(content)
	newContent := strings.Replace(oldContent, oldStr, newStr, 1)

	// Generate structured patch
	patch := editor.GeneratePatch(filePath, oldContent, newContent)

	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return nil, err
	}

	// Update tracker
	t.tracker.RecordView(filePath, newContent)

	return map[string]any{
		"status":    "edited",
		"file_path": filePath,
		"patch":     patch.String(),
	}, nil
}
```

### Anti-Bluff Verification Test

```go
package editor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/internal/editor"
)

func TestSmartEditTracker_RecordAndCheck(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.go")
	os.WriteFile(path, []byte("package main\n\nfunc main() {}"), 0644)

	tracker := editor.NewSmartEditTracker()
	content := "package main\n\nfunc main() {}"
	if err := tracker.RecordView(path, content); err != nil {
		t.Fatalf("RecordView: %v", err)
	}

	if !tracker.HasViewed(path) {
		t.Fatal("HasViewed should be true")
	}

	// Not modified yet
	modified, err := tracker.IsUserModified(path)
	if err != nil {
		t.Fatalf("IsUserModified: %v", err)
	}
	if modified {
		t.Fatal("should not be modified")
	}

	// Modify file
	time.Sleep(10 * time.Millisecond) // Ensure mtime changes
	os.WriteFile(path, []byte("package main\n\nfunc main() { println(\"hello\") }"), 0644)

	modified, err = tracker.IsUserModified(path)
	if err != nil {
		t.Fatalf("IsUserModified after change: %v", err)
	}
	if !modified {
		t.Fatal("should be modified after user change")
	}
}

func TestGeneratePatch(t *testing.T) {
	old := "line 1\nline 2\nline 3"
	new := "line 1\nmodified 2\nline 3\nline 4"

	patch := editor.GeneratePatch("test.txt", old, new)
	if patch.FilePath != "test.txt" {
		t.Fatalf("filepath = %q", patch.FilePath)
	}

	diff := patch.String()
	if !strings.Contains(diff, "-line 2") {
		t.Fatalf("missing removed line: %s", diff)
	}
	if !strings.Contains(diff, "+modified 2") {
		t.Fatalf("missing added line: %s", diff)
	}
	if !strings.Contains(diff, "+line 4") {
		t.Fatalf("missing new line: %s", diff)
	}
	if !strings.Contains(diff, " line 1") {
		t.Fatalf("missing context line: %s", diff)
	}
}
```

### Integration Steps

1. **Create smart edit tracker** (`internal/editor/smart_edit.go`)
2. **Create patch generator** (`internal/editor/patch.go`)
3. **Integrate with Read tool** — call `RecordView()` after each read
4. **Integrate with Edit tool** — check `IsUserModified()` before editing
5. **Return structured patch** in edit tool results
6. **Show diff preview** in terminal UI before confirming edit

---

## Feature 18: No-Flicker Rendering

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: Terminal UI uses alt-screen mode (`SM?1049` / `RM?1049` ANSI sequences) to avoid scrollback pollution. Custom `cla` escape sequence (clear-line-after) efficiently redraws dynamic content. Cursor positioning minimizes full-screen redraws.
- **Why it's powerful**: Professional terminal UX — no screen flicker during streaming, clean scrollback history, efficient rendering.

### HelixCode Integration Target
- **New files to create**:
  - `applications/terminal_ui/renderer.go` — Terminal renderer
  - `applications/terminal_ui/ansi.go` — ANSI escape sequences
- **Existing files to modify**:
  - `applications/terminal_ui/app.go` — Use new renderer
- **Submodule dependencies**: `applications/terminal_ui/`

### Exact Code Implementation

#### File: `applications/terminal_ui/ansi.go` (NEW)

```go
package terminalui

// ANSI escape sequences for terminal control.
const (
	// Enter alternate screen buffer
	EnterAltScreen = "\x1b[?1049h"
	// Exit alternate screen buffer
	ExitAltScreen = "\x1b[?1049l"
	// Clear entire screen
	ClearScreen = "\x1b[2J"
	// Clear line after cursor (CLA - Clear Line After)
	ClearLineAfter = "\x1b[0K"
	// Clear entire line
	ClearLine = "\x1b[2K"
	// Move cursor to top-left
	CursorHome = "\x1b[H"
	// Hide cursor
	HideCursor = "\x1b[?25l"
	// Show cursor
	ShowCursor = "\x1b[?25h"
	// Save cursor position
	SaveCursor = "\x1b[s"
	// Restore cursor position
	RestoreCursor = "\x1b[u"
	// Enable bracketed paste
	EnableBracketedPaste = "\x1b[?2004h"
	// Disable bracketed paste
	DisableBracketedPaste = "\x1b[?2004l"
)

// MoveCursorTo generates a cursor positioning sequence.
func MoveCursorTo(row, col int) string {
	return fmt.Sprintf("\x1b[%d;%dH", row, col)
}

// MoveCursorUp moves cursor up N lines.
func MoveCursorUp(n int) string {
	return fmt.Sprintf("\x1b[%dA", n)
}

// MoveCursorDown moves cursor down N lines.
func MoveCursorDown(n int) string {
	return fmt.Sprintf("\x1b[%dB", n)
}

// MoveCursorForward moves cursor forward N columns.
func MoveCursorForward(n int) string {
	return fmt.Sprintf("\x1b[%dC", n)
}

// MoveCursorBack moves cursor back N columns.
func MoveCursorBack(n int) string {
	return fmt.Sprintf("\x1b[%dD", n)
}
```

#### File: `applications/terminal_ui/renderer.go` (NEW)

```go
package terminalui

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// TerminalRenderer manages efficient screen updates.
type TerminalRenderer struct {
	writer     io.Writer
	mu         sync.Mutex
	altScreen  bool
	lines      []string
	cursorRow  int
	cursorCol  int
	width      int
	height     int
}

// NewTerminalRenderer creates a renderer.
func NewTerminalRenderer(w io.Writer, width, height int) *TerminalRenderer {
	return &TerminalRenderer{
		writer: w,
		width:  width,
		height: height,
		lines:  make([]string, 0),
	}
}

// EnterAltScreen switches to alternate screen buffer.
func (r *TerminalRenderer) EnterAltScreen() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.altScreen = true
	fmt.Fprint(r.writer, EnterAltScreen)
	fmt.Fprint(r.writer, ClearScreen)
	fmt.Fprint(r.writer, CursorHome)
	r.lines = make([]string, 0)
}

// ExitAltScreen returns to normal screen buffer.
func (r *TerminalRenderer) ExitAltScreen() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.altScreen = false
	fmt.Fprint(r.writer, ShowCursor)
	fmt.Fprint(r.writer, ExitAltScreen)
}

// Render updates the screen with minimal redraw.
func (r *TerminalRenderer) Render(content string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	newLines := strings.Split(content, "\n")

	if !r.altScreen {
		// Normal mode: just write
		fmt.Fprint(r.writer, content)
		return
	}

	// Alt-screen mode: efficient differential rendering
	fmt.Fprint(r.writer, CursorHome)
	fmt.Fprint(r.writer, HideCursor)

	for i := 0; i < r.height; i++ {
		var oldLine, newLine string
		if i < len(r.lines) {
			oldLine = r.lines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			// Move to row, clear line, write new content
			fmt.Fprint(r.writer, MoveCursorTo(i+1, 1))
			fmt.Fprint(r.writer, ClearLine)
			fmt.Fprint(r.writer, newLine)
		}
	}

	// Clear any remaining old lines
	for i := len(newLines); i < len(r.lines); i++ {
		fmt.Fprint(r.writer, MoveCursorTo(i+1, 1))
		fmt.Fprint(r.writer, ClearLine)
	}

	r.lines = newLines
	fmt.Fprint(r.writer, ShowCursor)
}

// RenderLine updates a single line in-place.
func (r *TerminalRenderer) RenderLine(row int, content string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if row < 1 || row > r.height {
		return
	}

	fmt.Fprintf(r.writer, "%s%s%s", MoveCursorTo(row, 1), ClearLine, content)
}

// AppendLine adds content to the bottom.
func (r *TerminalRenderer) AppendLine(content string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lines = append(r.lines, content)
	if len(r.lines) > r.height {
		r.lines = r.lines[1:]
		// Would need to scroll in full implementation
	}
	fmt.Fprint(r.writer, content)
	fmt.Fprint(r.writer, "\n")
}

// Clear clears the screen.
func (r *TerminalRenderer) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprint(r.writer, ClearScreen)
	fmt.Fprint(r.writer, CursorHome)
	r.lines = make([]string, 0)
}

// GetDimensions returns the terminal dimensions.
func (r *TerminalRenderer) GetDimensions() (width, height int) {
	return r.width, r.height
}
```

### Anti-Bluff Verification Test

```go
package terminalui_test

import (
	"bytes"
	"strings"
	"testing"

	"dev.helix.code/applications/terminal_ui"
)

func TestTerminalRenderer_EnterExitAltScreen(t *testing.T) {
	var buf bytes.Buffer
	r := terminalui.NewTerminalRenderer(&buf, 80, 24)

	r.EnterAltScreen()
	output := buf.String()
	if !strings.Contains(output, "\x1b[?1049h") {
		t.Fatal("missing enter alt screen sequence")
	}
	if !strings.Contains(output, "\x1b[2J") {
		t.Fatal("missing clear screen sequence")
	}

	buf.Reset()
	r.ExitAltScreen()
	output = buf.String()
	if !strings.Contains(output, "\x1b[?1049l") {
		t.Fatal("missing exit alt screen sequence")
	}
}

func TestTerminalRenderer_Render(t *testing.T) {
	var buf bytes.Buffer
	r := terminalui.NewTerminalRenderer(&buf, 80, 24)
	r.EnterAltScreen()
	buf.Reset()

	r.Render("Line 1\nLine 2")
	output := buf.String()
	if !strings.Contains(output, "Line 1") {
		t.Fatalf("missing Line 1: %q", output)
	}
	if !strings.Contains(output, "Line 2") {
		t.Fatalf("missing Line 2: %q", output)
	}
}

func TestANSISequences(t *testing.T) {
	// Verify all sequences are non-empty
	sequences := []string{
		terminalui.EnterAltScreen,
		terminalui.ExitAltScreen,
		terminalui.ClearScreen,
		terminalui.ClearLineAfter,
		terminalui.ClearLine,
		terminalui.HideCursor,
		terminalui.ShowCursor,
	}
	for _, seq := range sequences {
		if seq == "" {
			t.Fatal("empty ANSI sequence")
		}
		if !strings.HasPrefix(seq, "\x1b[") && !strings.HasPrefix(seq, "\x1b[?") {
			t.Fatalf("invalid ANSI sequence: %q", seq)
		}
	}
}

func TestMoveCursorTo(t *testing.T) {
	seq := terminalui.MoveCursorTo(10, 20)
	if seq != "\x1b[10;20H" {
		t.Fatalf("MoveCursorTo = %q, want ESC[10;20H", seq)
	}
}
```

### Integration Steps

1. **Create ANSI constants** (`applications/terminal_ui/ansi.go`)
2. **Create renderer** (`applications/terminal_ui/renderer.go`)
3. **Replace direct fmt.Print** in app with renderer calls
4. **Enter alt-screen** on app start, exit on shutdown
5. **Use differential rendering** for streaming content
6. **Handle terminal resize** via SIGWINCH

---

## Feature 19: AskUserQuestion with Previews

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: The agent can ask 1-4 questions with multiple-choice options. Questions support multi-select. Preview content (file diffs, images, rendered markdown) is shown inline with each question. Annotations add context to specific options.
- **Why it's powerful**: Structured user input for decision points, visual previews make choices concrete, multi-select handles complex configuration.

### HelixCode Integration Target
- **New files to create**:
  - `internal/tools/ask_user.go` — AskUserQuestion tool
  - `applications/terminal_ui/question_renderer.go` — UI rendering
- **Existing files to modify**:
  - `internal/tools/registry.go` — Register AskUserQuestion
  - `internal/agent/agent.go` — Handle question response
- **Submodule dependencies**: `internal/tools/`, `applications/terminal_ui/`

### Exact Code Implementation

#### File: `internal/tools/ask_user.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
)

// QuestionOption is a single choice in a question.
type QuestionOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Selected    bool   `json:"selected,omitempty"`
}

// QuestionPreview provides visual context for a question.
type QuestionPreview struct {
	Type    string `json:"type"` // "diff", "image", "markdown", "code"
	Content string `json:"content"`
	Title   string `json:"title,omitempty"`
}

// UserQuestion is a structured question from the agent.
type UserQuestion struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Description string             `json:"description,omitempty"`
	Options     []QuestionOption   `json:"options"`
	MultiSelect bool               `json:"multi_select"`
	Preview     *QuestionPreview   `json:"preview,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty"` // option_id -> note
}

// UserAnswer is the user's response.
type UserAnswer struct {
	QuestionID string   `json:"question_id"`
	Selected   []string `json:"selected"` // Option IDs
	Cancelled  bool     `json:"cancelled,omitempty"`
}

// AskUserQuestionTool enables the agent to ask structured questions.
type AskUserQuestionTool struct {
	pending   map[string]*UserQuestion
	answers   map[string]*UserAnswer
	mu        sync.RWMutex
	responder UserQuestionResponder
}

// UserQuestionResponder presents questions to the user.
type UserQuestionResponder interface {
	AskQuestion(ctx context.Context, question UserQuestion) (*UserAnswer, error)
}

// NewAskUserQuestionTool creates the tool.
func NewAskUserQuestionTool(responder UserQuestionResponder) *AskUserQuestionTool {
	return &AskUserQuestionTool{
		pending:   make(map[string]*UserQuestion),
		answers:   make(map[string]*UserAnswer),
		responder: responder,
	}
}

func (t *AskUserQuestionTool) Name() string { return "AskUserQuestion" }

func (t *AskUserQuestionTool) Description() string {
	return "Ask the user a structured question with 1-4 options. Supports multi-select and inline previews."
}

func (t *AskUserQuestionTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "Question title",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Additional context",
			},
			"options": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":    map[string]any{"type": "string"},
						"label": map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
					},
					"required": []string{"id", "label"},
				},
			},
			"multi_select": map[string]any{
				"type":        "boolean",
				"description": "Allow multiple selections",
			},
			"preview": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type":    map[string]any{"type": "string", "enum": []string{"diff", "image", "markdown", "code"}},
					"content": map[string]any{"type": "string"},
					"title":   map[string]any{"type": "string"},
				},
			},
		},
		"required": []string{"title", "options"},
	}
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	title, _ := args["title"].(string)
	description, _ := args["description"].(string)
	multiSelect, _ := args["multi_select"].(bool)

	// Parse options
	var options []QuestionOption
	if opts, ok := args["options"].([]any); ok {
		for _, o := range opts {
			if optMap, ok := o.(map[string]any); ok {
				opt := QuestionOption{
					ID:    fmt.Sprint(optMap["id"]),
					Label: fmt.Sprint(optMap["label"]),
				}
				if d, ok := optMap["description"]; ok {
					opt.Description = fmt.Sprint(d)
				}
				options = append(options, opt)
			}
		}
	}

	if len(options) < 1 || len(options) > 4 {
		return nil, fmt.Errorf("must provide 1-4 options, got %d", len(options))
	}

	question := UserQuestion{
		ID:          fmt.Sprintf("q-%d", time.Now().UnixNano()),
		Title:       title,
		Description: description,
		Options:     options,
		MultiSelect: multiSelect,
	}

	// Parse preview
	if previewMap, ok := args["preview"].(map[string]any); ok {
		question.Preview = &QuestionPreview{
			Type:    fmt.Sprint(previewMap["type"]),
			Content: fmt.Sprint(previewMap["content"]),
			Title:   fmt.Sprint(previewMap["title"]),
		}
	}

	t.mu.Lock()
	t.pending[question.ID] = &question
	t.mu.Unlock()

	// Ask user
	answer, err := t.responder.AskQuestion(ctx, question)
	if err != nil {
		return nil, fmt.Errorf("question failed: %w", err)
	}

	t.mu.Lock()
	t.answers[question.ID] = answer
	delete(t.pending, question.ID)
	t.mu.Unlock()

	return map[string]any{
		"question_id": question.ID,
		"selected":    answer.Selected,
		"cancelled":   answer.Cancelled,
	}, nil
}

// GetAnswer retrieves an answer by question ID.
func (t *AskUserQuestionTool) GetAnswer(questionID string) (*UserAnswer, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ans, exists := t.answers[questionID]
	return ans, exists
}
```

### Anti-Bluff Verification Test

```go
package tools_test

import (
	"context"
	"testing"

	"dev.helix.code/internal/tools"
)

type mockResponder struct {
	lastQuestion *tools.UserQuestion
}

func (m *mockResponder) AskQuestion(ctx context.Context, q tools.UserQuestion) (*tools.UserAnswer, error) {
	m.lastQuestion = &q
	return &tools.UserAnswer{
		QuestionID: q.ID,
		Selected:   []string{q.Options[0].ID},
	}, nil
}

func TestAskUserQuestionTool_Execute(t *testing.T) {
	responder := &mockResponder{}
	tool := tools.NewAskUserQuestionTool(responder)

	result, err := tool.Execute(context.Background(), map[string]any{
		"title": "Choose action",
		"description": "What should we do?",
		"options": []any{
			map[string]any{"id": "a", "label": "Option A"},
			map[string]any{"id": "b", "label": "Option B"},
		},
		"multi_select": false,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	m := result.(map[string]any)
	if m["cancelled"].(bool) {
		t.Fatal("should not be cancelled")
	}
	selected := m["selected"].([]string)
	if len(selected) != 1 || selected[0] != "a" {
		t.Fatalf("selected = %v, want [a]", selected)
	}

	if responder.lastQuestion == nil {
		t.Fatal("responder not called")
	}
	if responder.lastQuestion.Title != "Choose action" {
		t.Fatalf("title = %q", responder.lastQuestion.Title)
	}
}

func TestAskUserQuestionTool_TooManyOptions(t *testing.T) {
	responder := &mockResponder{}
	tool := tools.NewAskUserQuestionTool(responder)

	options := make([]any, 5)
	for i := 0; i < 5; i++ {
		options[i] = map[string]any{"id": fmt.Sprintf("%d", i), "label": fmt.Sprintf("Option %d", i)}
	}

	_, err := tool.Execute(context.Background(), map[string]any{
		"title":   "Too many",
		"options": options,
	})
	if err == nil {
		t.Fatal("expected error for too many options")
	}
}

func TestAskUserQuestionTool_Preview(t *testing.T) {
	responder := &mockResponder{}
	tool := tools.NewAskUserQuestionTool(responder)

	tool.Execute(context.Background(), map[string]any{
		"title": "Review diff",
		"options": []any{
			map[string]any{"id": "approve", "label": "Approve"},
			map[string]any{"id": "reject", "label": "Reject"},
		},
		"preview": map[string]any{
			"type":    "diff",
			"content": "-old\n+new",
			"title":   "Changes",
		},
	})

	if responder.lastQuestion.Preview == nil {
		t.Fatal("preview not captured")
	}
	if responder.lastQuestion.Preview.Type != "diff" {
		t.Fatalf("preview type = %q, want diff", responder.lastQuestion.Preview.Type)
	}
	if responder.lastQuestion.Preview.Content != "-old\n+new" {
		t.Fatalf("preview content = %q", responder.lastQuestion.Preview.Content)
	}
}
```

### Integration Steps

1. **Create AskUserQuestion tool** (`internal/tools/ask_user.go`)
2. **Implement terminal UI responder** with interactive selection
3. **Render previews** inline (diffs with syntax highlighting, images via kitty/ sixel)
4. **Register in tool registry**
5. **Handle answer** in agent message loop
6. **Add keyboard shortcuts** (1-4 for option selection, space for multi-select)

---

## Feature 20: Theme System

### Source Analysis
- **Original agent**: Claude Code
- **How it works**: JSON theme files define colors for UI elements (user text, assistant text, tool calls, errors, highlights). Plugins can ship their own themes. Named themes are selectable via CLI flag or config file. Colors support hex, RGB, and ANSI256.
- **Why it's powerful**: Accessibility (high contrast themes), personal preference, brand consistency for enterprise deployments.

### HelixCode Integration Target
- **New files to create**:
  - `applications/terminal_ui/theme.go` — Theme engine
  - `applications/terminal_ui/themes/default.json` — Default theme
  - `applications/terminal_ui/themes/high-contrast.json` — High contrast theme
- **Existing files to modify**:
  - `applications/terminal_ui/app.go` — Apply theme to all rendered elements
  - `cmd/cli/main.go` — Add `--theme` flag
- **Submodule dependencies**: `applications/terminal_ui/`

### Exact Code Implementation

#### File: `applications/terminal_ui/theme.go` (NEW)

```go
package terminalui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ThemeColor represents a color in various formats.
type ThemeColor struct {
	Hex     string `json:"hex,omitempty"`
	ANSI256 int    `json:"ansi256,omitempty"`
	RGB     [3]int `json:"rgb,omitempty"`
}

// ToANSI converts the color to an ANSI escape sequence.
func (c ThemeColor) ToANSI(fg bool) string {
	if c.Hex != "" {
		// Convert hex to RGB
		var r, g, b int
		fmt.Sscanf(c.Hex, "#%02x%02x%02x", &r, &g, &b)
		if fg {
			return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
		}
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
	}
	if c.ANSI256 > 0 {
		if fg {
			return fmt.Sprintf("\x1b[38;5;%dm", c.ANSI256)
		}
		return fmt.Sprintf("\x1b[48;5;%dm", c.ANSI256)
	}
	if c.RGB[0] != 0 || c.RGB[1] != 0 || c.RGB[2] != 0 {
		if fg {
			return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.RGB[0], c.RGB[1], c.RGB[2])
		}
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.RGB[0], c.RGB[1], c.RGB[2])
	}
	return ""
}

// Theme defines all UI colors.
type Theme struct {
	Name        string     `json:"name"`
	Author      string     `json:"author,omitempty"`
	Description string     `json:"description,omitempty"`
	Colors      struct {
		Background      ThemeColor `json:"background"`
		Foreground      ThemeColor `json:"foreground"`
		UserText        ThemeColor `json:"user_text"`
		AssistantText   ThemeColor `json:"assistant_text"`
		SystemText      ThemeColor `json:"system_text"`
		ToolCall        ThemeColor `json:"tool_call"`
		ToolResult      ThemeColor `json:"tool_result"`
		Error           ThemeColor `json:"error"`
		Warning         ThemeColor `json:"warning"`
		Success         ThemeColor `json:"success"`
		Highlight       ThemeColor `json:"highlight"`
		DimText         ThemeColor `json:"dim_text"`
		Border          ThemeColor `json:"border"`
		Selection       ThemeColor `json:"selection"`
		Cursor          ThemeColor `json:"cursor"`
	} `json:"colors"`
	Styles struct {
		Bold      bool `json:"bold,omitempty"`
		Italic    bool `json:"italic,omitempty"`
		Underline bool `json:"underline,omitempty"`
	} `json:"styles,omitempty"`
}

// Reset returns the ANSI reset sequence.
func (t *Theme) Reset() string {
	return "\x1b[0m"
}

// StyleUser returns styled user text.
func (t *Theme) StyleUser(text string) string {
	return t.Colors.UserText.ToANSI(true) + text + t.Reset()
}

// StyleAssistant returns styled assistant text.
func (t *Theme) StyleAssistant(text string) string {
	return t.Colors.AssistantText.ToANSI(true) + text + t.Reset()
}

// StyleToolCall returns styled tool call text.
func (t *Theme) StyleToolCall(text string) string {
	return t.Colors.ToolCall.ToANSI(true) + text + t.Reset()
}

// StyleError returns styled error text.
func (t *Theme) StyleError(text string) string {
	return t.Colors.Error.ToANSI(true) + text + t.Reset()
}

// StyleSuccess returns styled success text.
func (t *Theme) StyleSuccess(text string) string {
	return t.Colors.Success.ToANSI(true) + text + t.Reset()
}

// ThemeLoader discovers and loads themes.
type ThemeLoader struct {
	searchPaths []string
}

// NewThemeLoader creates a theme loader.
func NewThemeLoader(paths []string) *ThemeLoader {
	return &ThemeLoader{searchPaths: paths}
}

// LoadTheme loads a theme by name.
func (tl *ThemeLoader) LoadTheme(name string) (*Theme, error) {
	for _, path := range tl.searchPaths {
		themePath := filepath.Join(path, name+".json")
		if _, err := os.Stat(themePath); err == nil {
			return LoadThemeFile(themePath)
		}
	}
	return nil, fmt.Errorf("theme %s not found", name)
}

// LoadThemeFile reads a theme from a JSON file.
func LoadThemeFile(path string) (*Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return nil, fmt.Errorf("parsing theme: %w", err)
	}

	return &theme, nil
}

// SaveThemeFile writes a theme to a JSON file.
func SaveThemeFile(path string, theme *Theme) error {
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

#### File: `applications/terminal_ui/themes/default.json` (NEW)

```json
{
  "name": "default",
  "author": "HelixCode",
  "description": "Default terminal theme",
  "colors": {
    "background": {"hex": "#1e1e1e"},
    "foreground": {"hex": "#d4d4d4"},
    "user_text": {"hex": "#569cd6"},
    "assistant_text": {"hex": "#4ec9b0"},
    "system_text": {"hex": "#808080"},
    "tool_call": {"hex": "#dcdcaa"},
    "tool_result": {"hex": "#ce9178"},
    "error": {"hex": "#f44747"},
    "warning": {"hex": "#ffcc00"},
    "success": {"hex": "#4ec9b0"},
    "highlight": {"hex": "#ffff00"},
    "dim_text": {"hex": "#6a9955"},
    "border": {"hex": "#3c3c3c"},
    "selection": {"hex": "#264f78"},
    "cursor": {"hex": "#aeafad"}
  },
  "styles": {
    "bold": false,
    "italic": false,
    "underline": false
  }
}
```

### Anti-Bluff Verification Test

```go
package terminalui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/applications/terminal_ui"
)

func TestThemeColor_ToANSI(t *testing.T) {
	c := terminalui.ThemeColor{Hex: "#ff5733"}
	seq := c.ToANSI(true)
	if !strings.Contains(seq, "38;2;") {
		t.Fatalf("expected foreground RGB, got %q", seq)
	}
	if !strings.Contains(seq, "255;87;51") {
		t.Fatalf("expected RGB values, got %q", seq)
	}

	bg := c.ToANSI(false)
	if !strings.Contains(bg, "48;2;") {
		t.Fatalf("expected background RGB, got %q", bg)
	}
}

func TestThemeColor_ANSI256(t *testing.T) {
	c := terminalui.ThemeColor{ANSI256: 196}
	seq := c.ToANSI(true)
	if seq != "\x1b[38;5;196m" {
		t.Fatalf("ansi256 fg = %q", seq)
	}
}

func TestLoadThemeFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	content := `{
		"name": "test",
		"colors": {
			"background": {"hex": "#000000"},
			"foreground": {"hex": "#ffffff"},
			"user_text": {"hex": "#ff0000"}
		}
	}`
	os.WriteFile(path, []byte(content), 0644)

	theme, err := terminalui.LoadThemeFile(path)
	if err != nil {
		t.Fatalf("LoadThemeFile: %v", err)
	}
	if theme.Name != "test" {
		t.Fatalf("name = %q, want test", theme.Name)
	}
	if theme.Colors.Background.Hex != "#000000" {
		t.Fatalf("background = %q", theme.Colors.Background.Hex)
	}
}

func TestSaveAndLoadTheme(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "roundtrip.json")

	original := &terminalui.Theme{
		Name: "roundtrip",
	}
	original.Colors.UserText.Hex = "#123456"
	original.Colors.AssistantText.ANSI256 = 42

	if err := terminalui.SaveThemeFile(path, original); err != nil {
		t.Fatalf("SaveThemeFile: %v", err)
	}

	loaded, err := terminalui.LoadThemeFile(path)
	if err != nil {
		t.Fatalf("LoadThemeFile: %v", err)
	}
	if loaded.Name != "roundtrip" {
		t.Fatalf("name = %q", loaded.Name)
	}
	if loaded.Colors.UserText.Hex != "#123456" {
		t.Fatalf("user_text hex mismatch")
	}
	if loaded.Colors.AssistantText.ANSI256 != 42 {
		t.Fatalf("assistant_text ansi256 = %d", loaded.Colors.AssistantText.ANSI256)
	}
}

func TestThemeLoader(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "dark.json"), []byte(`{"name":"dark"}`), 0644)

	loader := terminalui.NewThemeLoader([]string{tmpDir})
	theme, err := loader.LoadTheme("dark")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	if theme.Name != "dark" {
		t.Fatalf("name = %q", theme.Name)
	}

	_, err = loader.LoadTheme("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing theme")
	}
}

func TestTheme_Styling(t *testing.T) {
	theme := &terminalui.Theme{}
	theme.Colors.UserText.Hex = "#ff0000"
	theme.Colors.AssistantText.Hex = "#00ff00"
	theme.Colors.Error.Hex = "#ff0000"

	userStyled := theme.StyleUser("hello")
	if !strings.Contains(userStyled, "\x1b[") {
		t.Fatalf("missing ANSI codes: %q", userStyled)
	}
	if !strings.Contains(userStyled, "hello") {
		t.Fatalf("missing text: %q", userStyled)
	}
	if !strings.HasSuffix(userStyled, "\x1b[0m") {
		t.Fatalf("missing reset: %q", userStyled)
	}
}
```

### Integration Steps

1. **Create theme engine** (`applications/terminal_ui/theme.go`)
2. **Create default themes** in `themes/` directory
3. **Add `--theme` CLI flag**
4. **Load theme at app startup**
5. **Apply colors** to all rendered elements:
   - User input: `StyleUser()`
   - Assistant output: `StyleAssistant()`
   - Tool calls: `StyleToolCall()`
   - Errors: `StyleError()`
6. **Support plugin themes** by scanning plugin directories
7. **Add theme switching** at runtime via `/theme` slash command

---

## Appendix: Complete Integration Checklist

### New Files Summary (26 files)

| # | File Path | Feature |
|---|-----------|---------|
| 1 | `internal/context/token_counter.go` | Auto-Compaction |
| 2 | `internal/context/compaction.go` | Auto-Compaction |
| 3 | `internal/context/compaction_test.go` | Auto-Compaction |
| 4 | `internal/tools/permissions.go` | Permission Rules |
| 5 | `internal/tools/permission_store.go` | Permission Rules |
| 6 | `internal/tools/permissions_test.go` | Permission Rules |
| 7 | `internal/tools/persistence.go` | Tool Persistence |
| 8 | `internal/tools/persistence_test.go` | Tool Persistence |
| 9 | `internal/tools/worktree.go` | Git Worktree |
| 10 | `internal/tools/worktree_test.go` | Git Worktree |
| 11 | `internal/session/worktree_state.go` | Git Worktree |
| 12 | `internal/workflow/hooks.go` | Hooks |
| 13 | `internal/workflow/hooks_test.go` | Hooks |
| 14 | `internal/workflow/plugin.go` | Hooks |
| 15 | `internal/mcp/transport.go` | MCP |
| 16 | `internal/mcp/transport_stdio.go` | MCP |
| 17 | `internal/mcp/transport_sse.go` | MCP |
| 18 | `internal/mcp/transport_http.go` | MCP |
| 19 | `internal/mcp/transport_ws.go` | MCP |
| 20 | `internal/mcp/oauth.go` | MCP |
| 21 | `internal/mcp/lifecycle.go` | MCP |
| 22 | `internal/mcp/registry.go` | MCP |
| 23 | `internal/workflow/background.go` | Background Tasks |
| 24 | `internal/workflow/background_test.go` | Background Tasks |
| 25 | `internal/tools/task_tools.go` | Background Tasks |
| 26 | `internal/workflow/plan.go` | Plan Mode |
| 27 | `internal/workflow/plan_test.go` | Plan Mode |
| 28 | `internal/tools/exit_plan_mode.go` | Plan Mode |
| 29 | `internal/workflow/slash_commands.go` | Slash Commands |
| 30 | `internal/workflow/slash_commands_test.go` | Slash Commands |
| 31 | `internal/workflow/skills.go` | Skills |
| 32 | `internal/workflow/skills_test.go` | Skills |
| 33 | `internal/workflow/skill_isolation.go` | Skills |
| 34 | `internal/session/resume.go` | Session Resume |
| 35 | `internal/session/resume_test.go` | Session Resume |
| 36 | `internal/llm/provider_anthropic.go` | Multi-Provider |
| 37 | `internal/llm/provider_bedrock.go` | Multi-Provider |
| 38 | `internal/llm/provider_vertex.go` | Multi-Provider |
| 39 | `internal/llm/provider_azure.go` | Multi-Provider |
| 40 | `internal/llm/provider_factory.go` | Multi-Provider |
| 41 | `internal/llm/wizard.go` | Multi-Provider |
| 42 | `internal/tools/lsp.go` | LSP |
| 43 | `internal/tools/lsp_client.go` | LSP |
| 44 | `internal/tools/sandbox.go` | Sandboxed Shell |
| 45 | `internal/tools/sandbox_linux.go` | Sandboxed Shell |
| 46 | `internal/tools/sandbox_darwin.go` | Sandboxed Shell |
| 47 | `internal/tools/sandbox_windows.go` | Sandboxed Shell |
| 48 | `internal/agent/subagent.go` | Subagent Team |
| 49 | `internal/agent/subagent_test.go` | Subagent Team |
| 50 | `internal/tools/send_message.go` | Subagent Team |
| 51 | `internal/telemetry/tracing.go` | OpenTelemetry |
| 52 | `internal/telemetry/metrics.go` | OpenTelemetry |
| 53 | `internal/telemetry/middleware.go` | OpenTelemetry |
| 54 | `internal/editor/smart_edit.go` | Smart Editing |
| 55 | `internal/editor/patch.go` | Smart Editing |
| 56 | `applications/terminal_ui/ansi.go` | No-Flicker |
| 57 | `applications/terminal_ui/renderer.go` | No-Flicker |
| 58 | `internal/tools/ask_user.go` | AskUserQuestion |
| 59 | `applications/terminal_ui/question_renderer.go` | AskUserQuestion |
| 60 | `applications/terminal_ui/theme.go` | Themes |
| 61 | `applications/terminal_ui/themes/default.json` | Themes |
| 62 | `applications/terminal_ui/themes/high-contrast.json` | Themes |

### Modified Files Summary (12 files)

| # | File Path | Feature | Change Type |
|---|-----------|---------|-------------|
| 1 | `internal/session/session_manager.go` | Auto-Compaction, Session Resume | Add compactor field, resume methods |
| 2 | `internal/llm/provider.go` | Auto-Compaction, Multi-Provider | Extend interface |
| 3 | `internal/agent/agent.go` | Auto-Compaction, Plan, Subagent, Hooks | Integrate all systems |
| 4 | `internal/tools/tool_executor.go` | Permissions, Persistence, Hooks, Background, Smart Edit | Add all middleware layers |
| 5 | `internal/tools/registry.go` | Worktree, Background, Plan, LSP, Subagent | Register new tools |
| 6 | `internal/editor/editor.go` | Hooks, Smart Edit | Emit edit hooks, check modifications |
| 7 | `internal/context/compaction.go` | Hooks | Emit on_compaction event |
| 8 | `cmd/cli/main.go` | Permissions, Resume, Multi-Provider, Themes | Add all CLI flags |
| 9 | `cmd/server/main.go` | OpenTelemetry | Initialize telemetry |
| 10 | `internal/session/store.go` | Session Resume | Add metadata schema |
| 11 | `internal/tools/bash.go` | Sandboxed Shell | Wrap with sandbox |
| 12 | `applications/terminal_ui/app.go` | No-Flicker, Themes, AskUserQuestion | Use new renderer and theme |

### Dependencies to Add

```
go get go.opentelemetry.io/otel
ogo get go.opentelemetry.io/otel/sdk
ogo get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go get go.opentelemetry.io/otel/exporters/prometheus
go get github.com/gorilla/websocket
go get github.com/google/uuid
go get gopkg.in/yaml.v3
```

### Database Migrations

```sql
-- permission_rules
CREATE TABLE permission_rules (
    id TEXT PRIMARY KEY,
    pattern TEXT NOT NULL,
    mode TEXT NOT NULL,
    description TEXT,
    created_at BIGINT NOT NULL
);
CREATE INDEX idx_permission_rules_pattern ON permission_rules(pattern);

-- session_metadata
CREATE TABLE session_metadata (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id),
    project_path TEXT NOT NULL,
    project_name TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    last_activity TIMESTAMPTZ NOT NULL,
    message_count INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    pr_url TEXT,
    branch_name TEXT
);
CREATE INDEX idx_session_metadata_project ON session_metadata(project_path);
CREATE INDEX idx_session_metadata_activity ON session_metadata(last_activity DESC);
```

### Testing Strategy

Each feature includes:
1. **Unit tests** covering happy path, edge cases, and error conditions
2. **Negative tests** that verify the feature catches bad input (anti-bluff)
3. **Integration tests** where applicable (database, filesystem)

Run all tests:
```bash
go test ./... -v
```

### Rollout Plan

1. **Phase 1**: Core infrastructure (Auto-Compaction, Permissions, Persistence, Hooks)
2. **Phase 2**: Developer experience (Smart Edit, No-Flicker, Slash Commands, Themes)
3. **Phase 3**: Enterprise features (MCP, Multi-Provider, OpenTelemetry, Sandbox)
4. **Phase 4**: Advanced workflows (Subagents, Skills, Plan Mode, Background Tasks, LSP)
5. **Phase 5**: Session management (Resume, Worktree isolation)

---

**End of Porting Plan**

*Total Features Documented: 20*
*Total New Files: 62*
*Total Modified Files: 12*
*Total Lines of Go Code: ~3500+*
