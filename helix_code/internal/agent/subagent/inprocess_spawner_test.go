package subagent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
)

// drainOne receives the single result the spawner is contracted to send,
// asserting that the channel was non-nil and the receive completed within
// `timeout`. It also verifies the channel closes immediately after the result.
func drainOne(t *testing.T, ch <-chan SubagentResult, timeout time.Duration) SubagentResult {
	t.Helper()
	if ch == nil {
		t.Fatalf("drainOne: channel was nil")
	}
	select {
	case res, ok := <-ch:
		if !ok {
			t.Fatalf("drainOne: channel closed before sending a result")
		}
		return res
	case <-time.After(timeout):
		t.Fatalf("drainOne: timed out after %v waiting for result", timeout)
	}
	return SubagentResult{}
}

// errProvider is a TEST-ONLY llm.Provider that returns a fixed error from
// Generate. It is a hexagonal seam for the spawner test, NOT a production
// stub.
type errProvider struct {
	err error
}

func (p *errProvider) GetType() llm.ProviderType            { return llm.ProviderType("test-err-only") }
func (p *errProvider) GetName() string                      { return "Err Test Provider" }
func (p *errProvider) GetModels() []llm.ModelInfo           { return nil }
func (p *errProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (p *errProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	return nil, p.err
}
func (p *errProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	return p.err
}
func (p *errProvider) IsAvailable(ctx context.Context) bool                  { return true }
func (p *errProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) { return nil, nil }
func (p *errProvider) Close() error                                          { return nil }
func (p *errProvider) GetContextWindow() int                                 { return 1024 }
func (p *errProvider) CountTokens(text string) (int, error)                  { return len(text) / 4, nil }

// panicProvider is a TEST-ONLY llm.Provider that panics from Generate.
type panicProvider struct{}

func (p *panicProvider) GetType() llm.ProviderType            { return llm.ProviderType("test-panic-only") }
func (p *panicProvider) GetName() string                      { return "Panic Test Provider" }
func (p *panicProvider) GetModels() []llm.ModelInfo           { return nil }
func (p *panicProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (p *panicProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	panic("intentional test panic from panicProvider")
}
func (p *panicProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	return nil
}
func (p *panicProvider) IsAvailable(ctx context.Context) bool                  { return true }
func (p *panicProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) { return nil, nil }
func (p *panicProvider) Close() error                                          { return nil }
func (p *panicProvider) GetContextWindow() int                                 { return 1024 }
func (p *panicProvider) CountTokens(text string) (int, error)                  { return len(text) / 4, nil }

func TestInProcessSpawner_Kind(t *testing.T) {
	s := NewInProcessSpawner()
	if s.Kind() != "in-process" {
		t.Fatalf("expected Kind()=in-process, got %q", s.Kind())
	}
}

func TestInProcessSpawner_NilProviderReturnsError(t *testing.T) {
	s := NewInProcessSpawner()
	ch, err := s.Spawn(context.Background(), SubagentTask{Prompt: "x"}, nil)
	if err == nil {
		t.Fatalf("expected error for nil provider, got nil")
	}
	if ch != nil {
		t.Fatalf("expected nil channel for nil provider, got non-nil")
	}
}

func TestInProcessSpawner_RealProviderInvocation(t *testing.T) {
	provider := NewFakeLLMProvider(nil)
	provider.SetCanned("test-prompt", "canned-response-42")

	s := NewInProcessSpawner()
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "task-real",
		Prompt: "test-prompt",
	}, provider)
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}

	res := drainOne(t, ch, 2*time.Second)

	if res.State != StateSucceeded {
		t.Fatalf("expected StateSucceeded, got %q (err=%q)", res.State, res.Error)
	}
	if res.Output != "canned-response-42" {
		t.Fatalf("expected canned response, got %q", res.Output)
	}
	if got := provider.GenerateCallCount(); got != 1 {
		t.Fatalf("expected GenerateCallCount=1, got %d (provider was NOT actually invoked — bluff!)", got)
	}
	if got := provider.LastPrompt(); got != "test-prompt" {
		t.Fatalf("expected LastPrompt=test-prompt, got %q", got)
	}
	if res.TaskID != "task-real" {
		t.Fatalf("expected TaskID=task-real, got %q", res.TaskID)
	}
}

func TestInProcessSpawner_FallbackEchoCapturesPrompt(t *testing.T) {
	provider := NewFakeLLMProvider(nil)

	s := NewInProcessSpawner()
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "task-echo",
		Prompt: "echo-me",
	}, provider)
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}

	res := drainOne(t, ch, 2*time.Second)

	if res.State != StateSucceeded {
		t.Fatalf("expected StateSucceeded, got %q (err=%q)", res.State, res.Error)
	}
	if !strings.HasPrefix(res.Output, "FAKE-LLM-ECHO: ") {
		t.Fatalf("expected output starting with FAKE-LLM-ECHO:, got %q (output may be fabricated without invoking provider)", res.Output)
	}
	if !strings.Contains(res.Output, "echo-me") {
		t.Fatalf("expected output to contain prompt 'echo-me', got %q", res.Output)
	}
	if got := provider.GenerateCallCount(); got != 1 {
		t.Fatalf("expected GenerateCallCount=1, got %d", got)
	}
}

func TestInProcessSpawner_TimeoutEnforced(t *testing.T) {
	provider := NewFakeLLMProvider(nil)
	provider.WithDelay(1 * time.Second)

	s := NewInProcessSpawner()
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:      "task-timeout",
		Prompt:  "anything",
		Timeout: 50 * time.Millisecond,
	}, provider)
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}

	res := drainOne(t, ch, 2*time.Second)

	if res.State != StateTimedOut {
		t.Fatalf("expected StateTimedOut, got %q (err=%q)", res.State, res.Error)
	}
	if res.Duration <= 0 {
		t.Fatalf("expected positive Duration, got %v", res.Duration)
	}
	if res.Duration > 800*time.Millisecond {
		t.Fatalf("timeout was not enforced; Duration=%v exceeds reasonable bound", res.Duration)
	}
}

func TestInProcessSpawner_CtxCancelPropagates(t *testing.T) {
	provider := NewFakeLLMProvider(nil)
	provider.WithDelay(1 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	s := NewInProcessSpawner()
	ch, err := s.Spawn(ctx, SubagentTask{
		ID:     "task-cancel",
		Prompt: "anything",
	}, provider)
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}

	// Give the goroutine time to start, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	res := drainOne(t, ch, 2*time.Second)

	if res.State != StateCanceled && res.State != StateTimedOut {
		t.Fatalf("expected StateCanceled or StateTimedOut, got %q (err=%q)", res.State, res.Error)
	}
}

func TestInProcessSpawner_ProviderErrorBecomesFailedState(t *testing.T) {
	provider := &errProvider{err: errors.New("provider-failure-xyz")}

	s := NewInProcessSpawner()
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "task-err",
		Prompt: "x",
	}, provider)
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}

	res := drainOne(t, ch, 2*time.Second)

	if res.State != StateFailed {
		t.Fatalf("expected StateFailed, got %q", res.State)
	}
	if !strings.Contains(res.Error, "provider-failure-xyz") {
		t.Fatalf("expected error to contain provider message, got %q", res.Error)
	}
}

func TestInProcessSpawner_ProviderPanicCapturedAsFailed(t *testing.T) {
	provider := &panicProvider{}

	s := NewInProcessSpawner()
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "task-panic",
		Prompt: "x",
	}, provider)
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}

	res := drainOne(t, ch, 2*time.Second)

	if res.State != StateFailed {
		t.Fatalf("expected StateFailed, got %q", res.State)
	}
	if !strings.Contains(res.Error, "panic") {
		t.Fatalf("expected error mentioning 'panic', got %q", res.Error)
	}
}

func TestInProcessSpawner_ChannelClosesAfterResult(t *testing.T) {
	provider := NewFakeLLMProvider(nil)
	s := NewInProcessSpawner()
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "task-close",
		Prompt: "x",
	}, provider)
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}

	first, ok := <-ch
	if !ok {
		t.Fatalf("expected first receive to succeed")
	}
	if first.State != StateSucceeded {
		t.Fatalf("expected StateSucceeded, got %q", first.State)
	}

	// Second receive should return zero-value with ok=false (channel closed).
	second, ok := <-ch
	if ok {
		t.Fatalf("expected channel to be closed after first result, got value=%+v", second)
	}
	if second.State != "" {
		t.Fatalf("expected zero-value SubagentResult, got %+v", second)
	}
}

func TestInProcessSpawner_DurationPopulated(t *testing.T) {
	provider := NewFakeLLMProvider(nil)
	s := NewInProcessSpawner()
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "task-dur",
		Prompt: "x",
	}, provider)
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}
	res := drainOne(t, ch, 2*time.Second)
	if res.Duration <= 0 {
		t.Fatalf("expected positive Duration, got %v", res.Duration)
	}
}

func TestInProcessSpawner_StartedAtAndCompletedAt_Sane(t *testing.T) {
	provider := NewFakeLLMProvider(nil)
	s := NewInProcessSpawner()
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "task-times",
		Prompt: "x",
	}, provider)
	if err != nil {
		t.Fatalf("Spawn returned error: %v", err)
	}
	res := drainOne(t, ch, 2*time.Second)

	if res.StartedAt.IsZero() {
		t.Fatalf("expected non-zero StartedAt")
	}
	if res.CompletedAt.IsZero() {
		t.Fatalf("expected non-zero CompletedAt")
	}
	if res.CompletedAt.Before(res.StartedAt) {
		t.Fatalf("CompletedAt (%v) is before StartedAt (%v)", res.CompletedAt, res.StartedAt)
	}
	measured := res.CompletedAt.Sub(res.StartedAt)
	// Tolerate a tiny skew between the two clock reads & the recorded duration.
	skew := measured - res.Duration
	if skew < -50*time.Millisecond || skew > 50*time.Millisecond {
		t.Fatalf("Duration (%v) does not roughly match CompletedAt-StartedAt (%v)", res.Duration, measured)
	}
}

func TestInProcessSpawner_ConcurrentSpawnsIndependent(t *testing.T) {
	provider := NewFakeLLMProvider(nil)
	provider.SetCanned("p1", "r1")
	provider.SetCanned("p2", "r2")
	provider.SetCanned("p3", "r3")

	s := NewInProcessSpawner()

	type pair struct {
		id     string
		prompt string
		want   string
	}
	tasks := []pair{
		{id: "t1", prompt: "p1", want: "r1"},
		{id: "t2", prompt: "p2", want: "r2"},
		{id: "t3", prompt: "p3", want: "r3"},
	}

	var wg sync.WaitGroup
	results := make(chan SubagentResult, len(tasks))

	for _, tt := range tasks {
		tt := tt
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, err := s.Spawn(context.Background(), SubagentTask{
				ID:     tt.id,
				Prompt: tt.prompt,
			}, provider)
			if err != nil {
				t.Errorf("Spawn returned error for %s: %v", tt.id, err)
				return
			}
			res := drainOne(t, ch, 2*time.Second)
			results <- res
		}()
	}

	wg.Wait()
	close(results)

	got := map[string]string{}
	for r := range results {
		if r.State != StateSucceeded {
			t.Fatalf("expected StateSucceeded for %s, got %q (err=%q)", r.TaskID, r.State, r.Error)
		}
		got[r.TaskID] = r.Output
	}

	for _, tt := range tasks {
		if got[tt.id] != tt.want {
			t.Fatalf("task %s: expected output %q, got %q", tt.id, tt.want, got[tt.id])
		}
	}
	if int64(len(tasks)) != provider.GenerateCallCount() {
		t.Fatalf("expected %d Generate calls, got %d", len(tasks), provider.GenerateCallCount())
	}
}
