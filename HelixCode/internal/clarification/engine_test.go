package clarification

import (
	"context"
	"strings"
	"testing"
)

func TestDetectAmbiguity_NoLLM(t *testing.T) {
	engine := NewEngine(nil)
	questions := engine.DetectAmbiguity(context.Background(), "fix the bug")
	if len(questions) != 0 {
		t.Fatalf("expected 0 questions with no LLM, got %d", len(questions))
	}
}

func TestEngine_NewSession(t *testing.T) {
	engine := NewEngine(nil)
	s := engine.NewSession("test context")
	if s.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if s.Context != "test context" {
		t.Errorf("expected test context, got %s", s.Context)
	}
}

func TestEngine_Resolve(t *testing.T) {
	engine := NewEngine(nil)
	s := engine.NewSession("Do something")
	s.Questions = []Question{{ID: "target_file", Text: "Which file?", Type: FreeText}}
	result := engine.Resolve(s.ID, []Answer{{QuestionID: "target_file", Value: "main.go"}})
	if !strings.Contains(result, "main.go") {
		t.Errorf("expected resolved context to contain answer: %s", result)
	}
}

func TestEngine_GetSession(t *testing.T) {
	engine := NewEngine(nil)
	orig := engine.NewSession("test")
	got := engine.GetSession(orig.ID)
	if got == nil || got.ID != orig.ID {
		t.Error("expected to get the same session")
	}
}

func TestEngine_Resolve_NoSession(t *testing.T) {
	engine := NewEngine(nil)
	result := engine.Resolve("nonexistent", nil)
	if result != "" {
		t.Error("expected empty result for nonexistent session")
	}
}

func TestQuestionGenerator_NoLLM(t *testing.T) {
	g := NewQuestionGenerator(nil)
	questions := g.Generate("fix the bug")
	if len(questions) != 0 {
		t.Fatalf("expected 0 questions with no LLM, got %d", len(questions))
	}
}
