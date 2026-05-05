// inprocess_spawner.go — P1-F15-T03 InProcessSpawner.
//
// Implements SubagentSpawner by launching the subagent as a goroutine in the
// parent process and invoking the supplied llm.Provider directly. The Spawn
// caller drains the returned buffered (cap=1) channel for exactly one result,
// after which the channel is closed.
//
// Anti-bluff guarantee: the goroutine ALWAYS calls llmProvider.Generate. The
// in-process spawner does NOT fabricate results. If the provider panics, the
// panic is captured and surfaced as StateFailed; if Generate returns an error,
// it surfaces as StateFailed; if ctx is canceled or the per-task timeout
// fires, the result reflects StateCanceled / StateTimedOut and Duration
// reflects how long Generate actually ran. See spec §4.1 (in-process flow).
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f15-subagent-team-design.md
// Plan: docs/superpowers/plans/2026-05-06-p1-f15-subagent-team.md
package subagent

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"dev.helix.code/internal/llm"
)

// InProcessSpawner launches subagents as goroutines within the parent process.
// Each subagent's Generate call goes through the supplied llm.Provider. This
// spawner is suitable for read-only / analytical subagents and is the default
// for IsolationNone tasks.
//
// The struct is stateless: a single InProcessSpawner instance can be shared
// across the manager and concurrent Spawn invocations safely.
type InProcessSpawner struct{}

// NewInProcessSpawner constructs the in-process spawner. It is stateless;
// callers MAY share a single instance across goroutines.
func NewInProcessSpawner() *InProcessSpawner {
	return &InProcessSpawner{}
}

// Kind returns the spawner's identifier "in-process". Used by the manager
// for routing decisions and by Challenge harnesses for evidence collection.
func (s *InProcessSpawner) Kind() string {
	return "in-process"
}

// Spawn launches a goroutine that invokes llmProvider.Generate with a
// single-message LLMRequest carrying task.Prompt, honoring ctx cancellation
// and task.Timeout. It returns a buffered channel (capacity 1) that will
// receive exactly one SubagentResult before being closed.
//
// The goroutine catches panics from the provider, converting them to
// StateFailed. Time accounting uses time.Now() at start + completion. The
// buffered cap-1 channel lets the goroutine send + close without the receiver
// being ready, so callers can drain at their own pace.
//
// Returns (nil, error) ONLY for argument-validation failures (currently:
// llmProvider == nil). All runtime errors during Generate are surfaced via
// the result channel, not as a return error.
func (s *InProcessSpawner) Spawn(ctx context.Context, task SubagentTask, llmProvider llm.Provider) (<-chan SubagentResult, error) {
	if llmProvider == nil {
		return nil, errors.New("InProcessSpawner.Spawn: llmProvider must not be nil")
	}

	out := make(chan SubagentResult, 1)

	go s.run(ctx, task, llmProvider, out)

	return out, nil
}

// run executes the subagent body. Always sends exactly one result and closes
// the channel.
func (s *InProcessSpawner) run(ctx context.Context, task SubagentTask, provider llm.Provider, out chan<- SubagentResult) {
	startedAt := time.Now()

	// Build the per-call context, honoring task.Timeout when > 0.
	callCtx := ctx
	var cancel context.CancelFunc
	if task.Timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, task.Timeout)
	}
	if cancel != nil {
		defer cancel()
	}

	req := &llm.LLMRequest{
		Model: task.SubagentType,
		Messages: []llm.Message{
			{Role: "user", Content: task.Prompt},
		},
	}

	// Invoke the provider with panic recovery. resp/err/panicErr capture the
	// outcome; the classification block below converts them to a state.
	var (
		resp     *llm.LLMResponse
		callErr  error
		panicErr error
	)

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = fmt.Errorf("subagent provider panicked: %v\n%s", r, debug.Stack())
			}
		}()
		resp, callErr = provider.Generate(callCtx, req)
	}()

	completedAt := time.Now()
	duration := completedAt.Sub(startedAt)

	result := SubagentResult{
		TaskID:      task.ID,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Duration:    duration,
		Isolation:   task.Isolation,
	}

	switch {
	case panicErr != nil:
		result.State = StateFailed
		result.Error = panicErr.Error()

	case callErr != nil:
		// Distinguish ctx-driven failures from real provider errors. We
		// inspect both the call error AND the original parent ctx so that
		// "ctx canceled while task.Timeout was also active" classifies as
		// StateCanceled (parent intent) rather than StateTimedOut.
		switch {
		case ctx.Err() == context.Canceled:
			result.State = StateCanceled
			result.Error = callErr.Error()
		case errors.Is(callErr, context.Canceled):
			result.State = StateCanceled
			result.Error = callErr.Error()
		case errors.Is(callErr, context.DeadlineExceeded):
			result.State = StateTimedOut
			result.Error = callErr.Error()
		default:
			result.State = StateFailed
			result.Error = callErr.Error()
		}

	case resp == nil:
		// Defensive: a provider that returns (nil, nil) is non-conforming.
		result.State = StateFailed
		result.Error = "subagent provider returned nil response with nil error"

	default:
		result.State = StateSucceeded
		result.Output = resp.Content
	}

	// Buffered cap-1 channel: this send never blocks even if no receiver is
	// ready. Closing immediately afterward signals "no further results".
	out <- result
	close(out)
}
