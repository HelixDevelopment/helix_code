package clarification

import (
	"context"

	"dev.helix.code/internal/llm/litellm"
)

// QuestionGenerator uses an LLM to generate clarification questions dynamically.
// It delegates to the Engine's DetectAmbiguity method, which calls the LLM.
type QuestionGenerator struct {
	engine *Engine
}

// NewQuestionGenerator creates a new question generator that uses the
// given LiteLLM provider to produce questions adaptively in the user's language.
func NewQuestionGenerator(llmProvider *litellm.UnifiedProvider) *QuestionGenerator {
	return &QuestionGenerator{engine: NewEngine(llmProvider)}
}

// Generate returns clarification questions for the given prompt by calling
// the LLM, which generates them dynamically in the appropriate language.
func (g *QuestionGenerator) Generate(prompt string) []Question {
	return g.engine.DetectAmbiguity(context.Background(), prompt)
}
