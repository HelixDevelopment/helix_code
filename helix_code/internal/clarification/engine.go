package clarification

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/litellm"
)

type Engine struct {
	llmProvider *litellm.UnifiedProvider
	mu          sync.RWMutex
	sessions    map[string]*Session
}

func NewEngine(llmProvider *litellm.UnifiedProvider) *Engine {
	return &Engine{
		llmProvider: llmProvider,
		sessions:    make(map[string]*Session),
	}
}

func (e *Engine) DetectAmbiguity(ctx context.Context, prompt string) []Question {
	if e.llmProvider == nil {
		return []Question{}
	}
	// CONST-046 round-222: system prompt resolved via i18n so non-English
	// operators get clarification questions phrased in their own language.
	system := tr(ctx, "internal_clarification_llm_system_prompt", nil)
	// CONST-046 round-222: user-request wrapper resolved via i18n with
	// the prompt interpolated as a named placeholder.
	userPrompt := tr(ctx, "internal_clarification_user_request_wrapper", map[string]any{"Prompt": prompt})
	req := &llm.LLMRequest{
		Model: "",
		Messages: []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3,
	}
	resp, err := e.llmProvider.Generate(ctx, req)
	if err != nil {
		return []Question{}
	}
	var questions []Question
	if err := json.Unmarshal([]byte(resp.Content), &questions); err != nil {
		return []Question{}
	}
	return questions
}

func (e *Engine) Resolve(sessionID string, answers []Answer) string {
	// HXC-clarif race fix: the session-state mutation (session.Answers =
	// answers) and the subsequent reads of session fields MUST hold the
	// engine lock. The *Session pointer is shared — GetSession/NewSession
	// hand the SAME pointer to other goroutines, and concurrent Resolve
	// calls on one session collide on session.Answers. The lock is held
	// for the whole field-touching span (write Answers, read Context)
	// so no reader observes a torn struct field. A write lock (not RLock)
	// is required because we mutate session.Answers under it.
	e.mu.Lock()
	defer e.mu.Unlock()
	session, ok := e.sessions[sessionID]
	if !ok {
		return ""
	}
	session.Answers = answers
	var resolved string
	if session.Context != "" {
		resolved = session.Context + "\n\n"
	}
	// CONST-046 round-222: resolved-context header resolved via i18n so
	// the user-facing summary is locale-aware.
	resolved += tr(context.Background(), "internal_clarification_clarifications_received_header", nil)
	for _, answer := range answers {
		resolved += fmt.Sprintf("- %s: %s\n", answer.QuestionID, answer.Value)
	}
	return resolved
}

func (e *Engine) NewSession(ctx string) *Session {
	// HXC-clarif race fix scope note (HONEST): NewSession intentionally
	// returns the LIVE *Session handle, NOT a snapshot. In-package callers
	// (engine_test.go TestEngine_Resolve, translator_test.go) rely on
	// mutating the returned pointer (e.g. `s.Questions = ...`) immediately
	// after creation; handing back a copy would silently turn those writes
	// into no-ops on a discarded value. This live-pointer return is a
	// SEPARATE, caller-owned-handle exposure OUTSIDE the scope of this fix:
	// a caller that retains the NewSession pointer and reads its mutable
	// fields concurrently with Resolve still races, because it aliases the
	// stored session. This fix eliminates the GetSession aliasing race (see
	// GetSession's defensive deep copy) and the Resolve off-lock write; it
	// does NOT claim to eliminate races for callers that deliberately hold
	// the live NewSession handle. Such callers own that synchronization.
	e.mu.Lock()
	defer e.mu.Unlock()
	id := fmt.Sprintf("session-%d", len(e.sessions)+1)
	s := &Session{
		ID:      id,
		Context: ctx,
	}
	e.sessions[id] = s
	return s
}

func (e *Engine) GetSession(id string) *Session {
	// HXC-clarif race fix: return a defensive DEEP copy, NOT the live
	// pointer and NOT a shallow `*session` copy. Returning the live
	// *Session let callers read mutable fields off-lock while Resolve
	// mutates them under the lock — a write/read data race caught by
	// `go test -race`. A SHALLOW `*session` copy is still unsafe: it
	// duplicates the slice HEADERS for Answers/Questions but SHARES their
	// backing arrays with the live session, so a caller ranging over
	// snapshot.Answers / snapshot.Questions elements still races a
	// concurrent in-place element mutation under Resolve. We therefore
	// deep-copy every reference-typed Session field (the Answers and
	// Questions slices — and each Question's Options slice) under the read
	// lock, mirroring internal/project's copyProject discipline. Context,
	// ID and Timeout are value/immutable fields and copy safely by value.
	// The returned snapshot shares NO mutable backing storage with the
	// engine's stored session, so the caller's reads can never alias a
	// concurrent write. Returns nil on a missing id (preserves the
	// existing contract).
	e.mu.RLock()
	defer e.mu.RUnlock()
	session, ok := e.sessions[id]
	if !ok {
		return nil
	}
	snapshot := *session
	if session.Answers != nil {
		snapshot.Answers = make([]Answer, len(session.Answers))
		copy(snapshot.Answers, session.Answers)
	}
	if session.Questions != nil {
		snapshot.Questions = make([]Question, len(session.Questions))
		copy(snapshot.Questions, session.Questions)
		for i := range session.Questions {
			if session.Questions[i].Options != nil {
				snapshot.Questions[i].Options = make([]string, len(session.Questions[i].Options))
				copy(snapshot.Questions[i].Options, session.Questions[i].Options)
			}
		}
	}
	return &snapshot
}
