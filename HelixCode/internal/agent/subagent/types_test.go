package subagent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"dev.helix.code/internal/llm"
)

// TestIsolation_String exercises every Isolation enum value to prevent silent
// drift when new isolation modes are added in later phases.
func TestIsolation_String(t *testing.T) {
	cases := []struct {
		iso  Isolation
		want string
	}{
		{IsolationNone, "none"},
		{IsolationWorktree, "worktree"},
	}
	for _, tc := range cases {
		t.Run(string(tc.iso), func(t *testing.T) {
			if got := string(tc.iso); got != tc.want {
				t.Fatalf("Isolation %q: got %q want %q", tc.iso, got, tc.want)
			}
		})
	}
}

// TestState_String exercises every State enum value.
func TestState_String(t *testing.T) {
	cases := []struct {
		st   State
		want string
	}{
		{StatePending, "pending"},
		{StateRunning, "running"},
		{StateSucceeded, "succeeded"},
		{StateFailed, "failed"},
		{StateCanceled, "canceled"},
		{StateTimedOut, "timed-out"},
	}
	for _, tc := range cases {
		t.Run(string(tc.st), func(t *testing.T) {
			if got := string(tc.st); got != tc.want {
				t.Fatalf("State %q: got %q want %q", tc.st, got, tc.want)
			}
		})
	}
}

// TestSubagentTask_JSONRoundTrip ensures every documented field survives a
// marshal/unmarshal cycle. Catches accidental json-tag drift.
func TestSubagentTask_JSONRoundTrip(t *testing.T) {
	original := SubagentTask{
		ID:             "task-123",
		Description:    "fix typo",
		Prompt:         "Replace 'teh' with 'the' in README.md",
		Isolation:      IsolationWorktree,
		SubagentType:   "code-fixer",
		Timeout:        7 * time.Minute,
		BaseBranch:     "main",
		MergeOnSuccess: true,
	}
	raw, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got SubagentTask
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != original {
		t.Fatalf("round-trip mismatch:\nwant %#v\ngot  %#v", original, got)
	}
}

// TestSubagentResult_JSONRoundTrip ensures every documented field survives a
// marshal/unmarshal cycle.
func TestSubagentResult_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	original := SubagentResult{
		TaskID:        "task-7",
		State:         StateSucceeded,
		Output:        "done",
		Error:         "",
		Duration:      450 * time.Millisecond,
		StartedAt:     now,
		CompletedAt:   now.Add(450 * time.Millisecond),
		Isolation:     IsolationNone,
		WorktreePath:  "",
		WorktreeDiff:  "",
		ToolCallCount: 3,
	}
	raw, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got SubagentResult
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.StartedAt.Equal(original.StartedAt) || !got.CompletedAt.Equal(original.CompletedAt) {
		t.Fatalf("time fields not equal after round trip:\nwant %#v\ngot  %#v", original, got)
	}
	// Compare other fields verbatim.
	got.StartedAt = original.StartedAt
	got.CompletedAt = original.CompletedAt
	if got != original {
		t.Fatalf("round-trip mismatch:\nwant %#v\ngot  %#v", original, got)
	}
}

// TestFakeLLMProvider_GetTypeIsSentinel locks down the sentinel ProviderType
// that the production factory MUST never produce. If this test breaks, the
// factory might be wiring the test provider into a real code path.
func TestFakeLLMProvider_GetTypeIsSentinel(t *testing.T) {
	p := NewFakeLLMProvider(nil)
	if got := p.GetType(); got != llm.ProviderType("fake-test-only") {
		t.Fatalf("GetType: got %q want %q", got, "fake-test-only")
	}
}

// TestFakeLLMProvider_GetNameNonEmpty guards against a regression that turns
// the name into an empty string (which would let it impersonate a real
// provider in registries that key by name).
func TestFakeLLMProvider_GetNameNonEmpty(t *testing.T) {
	p := NewFakeLLMProvider(nil)
	if name := p.GetName(); name == "" {
		t.Fatalf("GetName: empty")
	}
}

// TestFakeLLMProvider_ImplementsLLMProvider is a compile-time and runtime
// assertion that FakeLLMProvider satisfies llm.Provider exactly. The compile-
// time interface assignment is the real teeth; the runtime line keeps the
// import non-blank.
func TestFakeLLMProvider_ImplementsLLMProvider(t *testing.T) {
	var _ llm.Provider = (*FakeLLMProvider)(nil)
	var p llm.Provider = NewFakeLLMProvider(nil)
	if p == nil {
		t.Fatalf("nil provider")
	}
}

// TestFakeLLMProvider_GenerateReturnsCanned proves the canned-response path:
// a registered prompt MUST round-trip its registered response verbatim.
func TestFakeLLMProvider_GenerateReturnsCanned(t *testing.T) {
	p := NewFakeLLMProvider(map[string]string{
		"hello": "world",
	})
	resp, err := p.Generate(context.Background(), &llm.LLMRequest{
		ID: uuid.New(),
		Messages: []llm.Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Content != "world" {
		t.Fatalf("Content: got %q want %q", resp.Content, "world")
	}
}

// TestFakeLLMProvider_GenerateFallsBackToEchoWithPrefix proves the fallback
// path: an unknown prompt MUST be echoed with the FAKE-LLM-ECHO prefix so
// upstream tests can positively distinguish "the fake provider was actually
// invoked" from "an upstream bluff just echoed the prompt".
func TestFakeLLMProvider_GenerateFallsBackToEchoWithPrefix(t *testing.T) {
	p := NewFakeLLMProvider(nil)
	resp, err := p.Generate(context.Background(), &llm.LLMRequest{
		ID: uuid.New(),
		Messages: []llm.Message{
			{Role: "user", Content: "unknown-prompt"},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	const prefix = "FAKE-LLM-ECHO: "
	if !strings.HasPrefix(resp.Content, prefix) {
		t.Fatalf("Content does not start with %q: got %q", prefix, resp.Content)
	}
	if !strings.Contains(resp.Content, "unknown-prompt") {
		t.Fatalf("echo did not contain original prompt: got %q", resp.Content)
	}
}

// TestFakeLLMProvider_CallCountIncrementsAtomically proves the atomic counter
// is honest under concurrent load. A naive non-atomic int would race-detect
// here under -race.
func TestFakeLLMProvider_CallCountIncrementsAtomically(t *testing.T) {
	p := NewFakeLLMProvider(nil)
	var wg sync.WaitGroup
	const N = 32
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = p.Generate(context.Background(), &llm.LLMRequest{
				ID:       uuid.New(),
				Messages: []llm.Message{{Role: "user", Content: "x"}},
			})
		}()
	}
	wg.Wait()
	if got := p.GenerateCallCount(); got != int64(N) {
		t.Fatalf("GenerateCallCount: got %d want %d", got, N)
	}
}

// TestFakeLLMProvider_LastPromptUpdated proves that LastPrompt() reflects the
// most recent prompt, which is what subagent tests use to assert "the
// subagent really sent the prompt to the LLM".
func TestFakeLLMProvider_LastPromptUpdated(t *testing.T) {
	p := NewFakeLLMProvider(nil)
	if got := p.LastPrompt(); got != "" {
		t.Fatalf("LastPrompt before any call: got %q want empty", got)
	}
	_, err := p.Generate(context.Background(), &llm.LLMRequest{
		ID:       uuid.New(),
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got := p.LastPrompt(); got != "hello" {
		t.Fatalf("LastPrompt: got %q want %q", got, "hello")
	}
}

// TestFakeLLMProvider_HasAntiMisuseComment grep-self-tests the source file to
// guarantee the documented anti-misuse anchor stays present (per spec §5.2).
// The bluff scanner whitelists FakeLLMProvider only when this comment is
// present, so deleting it would silently re-open the door to production
// wiring.
func TestFakeLLMProvider_HasAntiMisuseComment(t *testing.T) {
	const anchor = "MUST NOT be referenced from production code"
	src, err := os.ReadFile("types.go")
	if err != nil {
		t.Fatalf("read types.go: %v", err)
	}
	if !strings.Contains(string(src), anchor) {
		t.Fatalf("types.go missing anti-misuse anchor %q", anchor)
	}
}

// TestFakeLLMProvider_OtherProviderMethodsDoNotPanic exercises the lower-
// traffic Provider methods so a future refactor can't sneak a panic past CI.
func TestFakeLLMProvider_OtherProviderMethodsDoNotPanic(t *testing.T) {
	p := NewFakeLLMProvider(nil)

	if !p.IsAvailable(context.Background()) {
		t.Fatalf("IsAvailable: got false want true")
	}
	health, err := p.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth: %v", err)
	}
	if health == nil || health.Status != "healthy" {
		t.Fatalf("GetHealth: %#v", health)
	}
	if got := p.GetContextWindow(); got != 100000 {
		t.Fatalf("GetContextWindow: got %d want 100000", got)
	}
	if got := len(p.GetModels()); got != 0 {
		t.Fatalf("GetModels: got %d models, want 0", got)
	}
	if got := len(p.GetCapabilities()); got != 0 {
		t.Fatalf("GetCapabilities: got %d caps, want 0", got)
	}
	count, err := p.CountTokens("hello world")
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if count <= 0 {
		t.Fatalf("CountTokens: got %d want positive", count)
	}

	// GenerateStream MUST run a producer goroutine and close ch.
	ch := make(chan llm.LLMResponse, 4)
	err = p.GenerateStream(context.Background(),
		&llm.LLMRequest{
			ID:       uuid.New(),
			Messages: []llm.Message{{Role: "user", Content: "hi"}},
		},
		ch,
	)
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}
	// Drain the channel; it must close.
	timeout := time.After(2 * time.Second)
	got := 0
drain:
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				break drain
			}
			got++
		case <-timeout:
			t.Fatalf("GenerateStream channel did not close within 2s")
		}
	}
	if got == 0 {
		t.Fatalf("GenerateStream: no responses sent")
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestSubagentRecursionEnvVar locks the sentinel env-var name so a future
// rename forces a deliberate, documented change.
func TestSubagentRecursionEnvVar(t *testing.T) {
	if SubagentRecursionEnvVar != "HELIXCODE_SUBAGENT_NO_RECURSE" {
		t.Fatalf("SubagentRecursionEnvVar: got %q", SubagentRecursionEnvVar)
	}
}

// TestSentinelErrors_Distinct guards against any future merge accidentally
// pointing two sentinels at the same errors.New value.
func TestSentinelErrors_Distinct(t *testing.T) {
	errs := []error{
		ErrSubagentTimeout,
		ErrSubagentCanceled,
		ErrMaxConcurrency,
		ErrSubagentRecursion,
		ErrUnknownIsolation,
	}
	for i := range errs {
		if errs[i] == nil {
			t.Fatalf("sentinel %d is nil", i)
		}
		for j := i + 1; j < len(errs); j++ {
			if errs[i] == errs[j] {
				t.Fatalf("sentinels %d and %d alias the same value: %v", i, j, errs[i])
			}
		}
	}
}
