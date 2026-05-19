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
	e.mu.RLock()
	session, ok := e.sessions[sessionID]
	e.mu.RUnlock()
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
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sessions[id]
}
