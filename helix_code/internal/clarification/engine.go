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
	system := `You are an AI assistant that helps clarify user requests. 
Given the user's request, generate a list of clarification questions that would help clarify the request. 
Each question should be short and open-ended. 
Return a JSON array of objects with the following fields:
- id: a unique string identifier for the question
- text: the question text
- type: one of "yes_no", "multiple_choice", "free_text"
- options: (optional) array of strings for multiple choice questions
- default: (optional) default value for the question
Only return the JSON array, nothing else.`
	userPrompt := fmt.Sprintf("User request: %s", prompt)
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
	resolved += "Clarifications received:\n"
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
