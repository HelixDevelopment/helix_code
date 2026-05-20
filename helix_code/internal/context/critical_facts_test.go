package context

// Unit tests for task-critical fact extraction (speed programme P3-T05).
//
// These tests pin the no-regression core: ExtractCriticalFacts must NOT drop
// the active task, decisions, files, open questions, or errors from a
// representative history.

import (
	"strings"
	"testing"
)

func TestExtractCriticalFacts_AllCategories(t *testing.T) {
	turns := []HistoryTurn{
		{Role: "assistant", Content: "Working on: the P3-T05 condenser implementation"},
		{Role: "assistant", Content: "Decided to keep the most recent turns verbatim."},
		{Role: "assistant", Content: "We will use a token threshold of 24000."},
		{Role: "tool", Content: "edited internal/context/condenser.go\nedited internal/session/condense.go"},
		{Role: "assistant", Content: "Error: build failed due to an import cycle"},
		{Role: "user", Content: "Open question: should the threshold be per-provider?"},
	}

	f := ExtractCriticalFacts(turns)

	if f.IsEmpty() {
		t.Fatalf("expected non-empty facts")
	}
	if !strings.Contains(f.ActiveTask, "condenser") {
		t.Fatalf("active task not extracted: %q", f.ActiveTask)
	}
	if len(f.Decisions) < 2 {
		t.Fatalf("expected >=2 decisions, got %v", f.Decisions)
	}
	if !containsSubstr(f.FilesTouched, "condenser.go") || !containsSubstr(f.FilesTouched, "condense.go") {
		t.Fatalf("file paths not extracted: %v", f.FilesTouched)
	}
	if len(f.OpenQuestions) == 0 {
		t.Fatalf("open question not extracted")
	}
	if len(f.Errors) == 0 {
		t.Fatalf("error not extracted")
	}
}

func TestExtractCriticalFacts_ActiveTask_LastWins(t *testing.T) {
	turns := []HistoryTurn{
		{Role: "assistant", Content: "Working on: the first task"},
		{Role: "assistant", Content: "Working on: the second and most recent task"},
	}
	f := ExtractCriticalFacts(turns)
	if !strings.Contains(f.ActiveTask, "second") {
		t.Fatalf("most recent active task should win, got %q", f.ActiveTask)
	}
}

func TestExtractCriticalFacts_EmptyAndNoise(t *testing.T) {
	// History with no task-critical content must yield empty facts (no false
	// positives that would bloat the summary needlessly).
	turns := []HistoryTurn{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello there"},
		{Role: "user", Content: ""},
	}
	f := ExtractCriticalFacts(turns)
	if f.ActiveTask != "" {
		t.Fatalf("expected no active task for noise input, got %q", f.ActiveTask)
	}
}

func TestExtractCriticalFacts_DeduplicatesFiles(t *testing.T) {
	turns := []HistoryTurn{
		{Role: "tool", Content: "edited main.go"},
		{Role: "tool", Content: "edited main.go again"},
		{Role: "tool", Content: "edited main.go once more"},
	}
	f := ExtractCriticalFacts(turns)
	count := 0
	for _, fp := range f.FilesTouched {
		if fp == "main.go" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("main.go should appear exactly once after dedupe, got %d", count)
	}
}

func TestExtractCriticalFacts_LongFactTruncated(t *testing.T) {
	long := "Working on: " + strings.Repeat("a", 1000)
	f := ExtractCriticalFacts([]HistoryTurn{{Role: "assistant", Content: long}})
	if len(f.ActiveTask) > 300 {
		t.Fatalf("long fact not truncated: len=%d", len(f.ActiveTask))
	}
}

func TestExtractCriticalFacts_CapsPerCategory(t *testing.T) {
	var turns []HistoryTurn
	for i := 0; i < maxFactsPerCategory*3; i++ {
		turns = append(turns, HistoryTurn{
			Role:    "assistant",
			Content: "Decided to do thing number " + string(rune('A'+i%26)) + strings.Repeat("x", i),
		})
	}
	f := ExtractCriticalFacts(turns)
	if len(f.Decisions) > maxFactsPerCategory {
		t.Fatalf("decisions exceeded the per-category cap: %d", len(f.Decisions))
	}
}
