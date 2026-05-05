// Package subagent defines the foundational types, lifecycle enums, spawner
// contract, and TEST-ONLY FakeLLMProvider used by HelixCode's subagent team
// feature (P1-F15).
//
// This file is type-only: every spawner (in-process / subprocess), the
// manager, the F04 worktree integration, the helper-mode dispatch, the
// `task` agent tool, and the `/subagents` slash command all consume the
// types declared here. Behaviour (spawn dispatch, streaming aggregation,
// kill, helper-mode re-exec) lives in sibling files added by later
// T03-T10 tasks.
//
// The TEST-ONLY FakeLLMProvider lives here (not in `_test.go`) because the
// in-tree Challenge harness binary at `tests/integration/cmd/p1f15_challenge`
// must link it without import-test gymnastics. The sentinel
// `ProviderType("fake-test-only")` together with the
// "MUST NOT be referenced from production code" docstring anchor enforce
// that the bluff scanner can detect any accidental production wiring.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f15-subagent-team-design.md
// Plan: docs/superpowers/plans/2026-05-06-p1-f15-subagent-team.md
package subagent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"dev.helix.code/internal/llm"
)

// Isolation declares how a subagent should be sandboxed.
//
// `IsolationNone` runs the subagent as a goroutine in the parent process,
// sharing the parent's working directory and tool registry. Suitable for
// read-only / analytical subagents.
//
// `IsolationWorktree` runs the subagent in an F04 git worktree (separate
// branch, separate filesystem) via the subprocess spawner. Suitable for
// subagents that mutate the filesystem.
type Isolation string

const (
	// IsolationNone — in-process goroutine, shared cwd. Default.
	IsolationNone Isolation = "none"
	// IsolationWorktree — F04 worktree, subprocess spawner.
	IsolationWorktree Isolation = "worktree"
)

// State is the lifecycle state of a single subagent.
//
// Transitions: pending -> running -> {succeeded, failed, canceled, timed-out}.
// `pending` covers the brief window between dispatch and the spawner
// actually starting the inner agent loop.
type State string

const (
	// StatePending — registered with the manager but not yet running.
	StatePending State = "pending"
	// StateRunning — the inner agent loop is executing.
	StateRunning State = "running"
	// StateSucceeded — the inner loop returned a non-error result.
	StateSucceeded State = "succeeded"
	// StateFailed — the inner loop returned an error or panicked.
	StateFailed State = "failed"
	// StateCanceled — Kill(id) or parent ctx cancellation propagated.
	StateCanceled State = "canceled"
	// StateTimedOut — the per-task timeout fired before completion.
	StateTimedOut State = "timed-out"
)

// SubagentTask is the input the agent gives to the `task` tool.
//
// `ID` is assigned by the manager (UUIDv4); callers MAY leave it empty.
// `Timeout` of 0 falls back to the manager's configured default. `BaseBranch`
// and `MergeOnSuccess` are only consulted when `Isolation == IsolationWorktree`.
type SubagentTask struct {
	ID             string        `json:"id"`
	Description    string        `json:"description"`
	Prompt         string        `json:"prompt"`
	Isolation      Isolation     `json:"isolation"`
	SubagentType   string        `json:"subagent_type,omitempty"`
	Timeout        time.Duration `json:"timeout"`
	BaseBranch     string        `json:"base_branch,omitempty"`
	MergeOnSuccess bool          `json:"merge_on_success,omitempty"`
}

// SubagentResult is the output of a single subagent execution.
//
// `Output` is the LLM's final textual response (NOT the prompt echoed back —
// see anti-bluff (1) in the spec). `WorktreePath` and `WorktreeDiff` are
// populated only when `Isolation == IsolationWorktree`.
type SubagentResult struct {
	TaskID        string        `json:"task_id"`
	State         State         `json:"state"`
	Output        string        `json:"output"`
	Error         string        `json:"error,omitempty"`
	Duration      time.Duration `json:"duration"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   time.Time     `json:"completed_at"`
	Isolation     Isolation     `json:"isolation"`
	WorktreePath  string        `json:"worktree_path,omitempty"`
	WorktreeDiff  string        `json:"worktree_diff,omitempty"`
	ToolCallCount int           `json:"tool_call_count"`
}

// SubagentSpawner is the contract implemented by the in-process and
// subprocess spawners (T03 / T04). Spawn launches the subagent and returns a
// result-channel that the manager forwards to the coordinator. The channel
// is closed when the subagent terminates (success, error, cancel, timeout —
// every path closes).
type SubagentSpawner interface {
	Spawn(ctx context.Context, task SubagentTask, llmProvider llm.Provider) (<-chan SubagentResult, error)
	Kind() string
}

// Sentinel errors. Tests assert against these via errors.Is.
var (
	// ErrSubagentTimeout — the per-task timeout fired before the inner
	// agent loop completed.
	ErrSubagentTimeout = errors.New("subagent timeout")
	// ErrSubagentCanceled — the manager's Kill(id) or the parent ctx
	// cancellation propagated to the subagent.
	ErrSubagentCanceled = errors.New("subagent canceled")
	// ErrMaxConcurrency — Dispatch was called while the manager already
	// has its configured cap of running subagents.
	ErrMaxConcurrency = errors.New("max concurrency reached")
	// ErrSubagentRecursion — a child subagent attempted to register the
	// `task` tool. v1 caps recursion depth at 1 via the env-var sentinel.
	ErrSubagentRecursion = errors.New("subagent recursion not supported in v1")
	// ErrUnknownIsolation — the SubagentTask carried an Isolation value
	// the manager does not know how to dispatch.
	ErrUnknownIsolation = errors.New("unknown isolation mode")
)

// SubagentRecursionEnvVar is the env-var name set on a child process to
// signal that the helper-mode dispatch (F15 T08) has already been entered;
// when present, the inner agent's tool registry MUST refuse to register the
// `task` tool, which caps recursion depth at 1.
//
// Documented anti-bluff anchor: this constant exists so unit and challenge
// tests can assert "the cap is wired" without grepping the source.
const SubagentRecursionEnvVar = "HELIXCODE_SUBAGENT_NO_RECURSE"

// fakeProviderTypeSentinel is the ProviderType that the FakeLLMProvider
// reports. Production factories MUST never produce this value; the bluff
// scanner uses it as the single allowed test-provider marker.
const fakeProviderTypeSentinel llm.ProviderType = "fake-test-only"

// FakeLLMProvider is a TEST-ONLY llm.Provider implementation used by F15's
// challenge harness and unit tests. It returns canned responses based on the
// last message in `LLMRequest.Messages`, falling back to
// "FAKE-LLM-ECHO: <prompt>" when no canned response is registered.
//
// MUST NOT be referenced from production code paths (cmd/, applications/,
// internal/<pkg>/<file>.go that doesn't end in _test.go). The sentinel
// ProviderType "fake-test-only" prevents accidental wiring via the F12
// factory; the bluff scanner has a special-case rule that FakeLLMProvider is
// the ONE allowed test-provider type and that it MUST carry this anti-misuse
// comment. Verified by `TestFakeLLMProvider_HasAntiMisuseComment`.
//
// Lives in types.go (rather than in a `_test.go` file) so the in-tree
// Challenge harness binary at `tests/integration/cmd/p1f15_challenge` can
// link it without import-test gymnastics.
type FakeLLMProvider struct {
	mu         sync.Mutex
	canned     map[string]string
	callCount  atomic.Int64
	lastPrompt atomic.Value  // string
	delay      atomic.Int64  // time.Duration as int64; 0 = no delay
}

// Compile-time assertion that FakeLLMProvider implements llm.Provider in
// full. If this line breaks, the llm.Provider interface has changed and the
// fake needs updating in lockstep.
var _ llm.Provider = (*FakeLLMProvider)(nil)

// NewFakeLLMProvider returns a FakeLLMProvider seeded with the given
// canned prompt -> response map. A nil map is treated as empty.
func NewFakeLLMProvider(canned map[string]string) *FakeLLMProvider {
	cp := make(map[string]string, len(canned))
	for k, v := range canned {
		cp[k] = v
	}
	p := &FakeLLMProvider{canned: cp}
	p.lastPrompt.Store("")
	return p
}

// SetCanned adds or replaces a canned response for the given prompt.
// Safe for concurrent use.
func (p *FakeLLMProvider) SetCanned(prompt, response string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.canned == nil {
		p.canned = make(map[string]string)
	}
	p.canned[prompt] = response
}

// WithDelay configures Generate / GenerateStream to block for `d` before
// returning. The block IS context-aware: if the caller's ctx is canceled or
// times out before `d` elapses, Generate returns ctx.Err() immediately. This
// lets unit tests exercise the spawner's timeout / cancellation paths against
// a real time-based blocking provider rather than mocking ctx itself.
//
// Setting d <= 0 disables the delay (default behaviour). Safe for concurrent
// use; the most recent call wins.
func (p *FakeLLMProvider) WithDelay(d time.Duration) {
	p.delay.Store(int64(d))
}

// GenerateCallCount returns the total number of Generate invocations.
// Atomic and safe for concurrent reads.
func (p *FakeLLMProvider) GenerateCallCount() int64 {
	return p.callCount.Load()
}

// LastPrompt returns the most recently-received prompt body, or the empty
// string if Generate has never been called.
func (p *FakeLLMProvider) LastPrompt() string {
	if v := p.lastPrompt.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractPrompt returns the last user-or-system message content from the
// request, which is the closest analogue to "the prompt" given that
// llm.LLMRequest carries a Messages slice rather than a single Prompt field.
// An empty Messages slice yields the empty string.
func extractPrompt(req *llm.LLMRequest) string {
	if req == nil || len(req.Messages) == 0 {
		return ""
	}
	return req.Messages[len(req.Messages)-1].Content
}

// GetType returns the sentinel ProviderType "fake-test-only".
func (p *FakeLLMProvider) GetType() llm.ProviderType {
	return fakeProviderTypeSentinel
}

// GetName returns a human-readable name. Non-empty by contract.
func (p *FakeLLMProvider) GetName() string {
	return "Fake Test Provider"
}

// GetModels returns an empty slice — the fake exposes no real models.
func (p *FakeLLMProvider) GetModels() []llm.ModelInfo {
	return []llm.ModelInfo{}
}

// GetCapabilities returns an empty slice — the fake declares no capabilities.
func (p *FakeLLMProvider) GetCapabilities() []llm.ModelCapability {
	return []llm.ModelCapability{}
}

// Generate returns the canned response for the prompt extracted from the
// last message in `request.Messages`, or "FAKE-LLM-ECHO: <prompt>" when
// no canned response is registered.
//
// The unique FAKE-LLM-ECHO prefix lets tests positively distinguish "the
// provider was actually invoked" from "an upstream layer just echoed the
// prompt without calling the LLM".
func (p *FakeLLMProvider) Generate(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
	p.callCount.Add(1)
	prompt := extractPrompt(request)
	p.lastPrompt.Store(prompt)

	if d := time.Duration(p.delay.Load()); d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	p.mu.Lock()
	canned, ok := p.canned[prompt]
	p.mu.Unlock()

	var content string
	if ok {
		content = canned
	} else {
		content = "FAKE-LLM-ECHO: " + prompt
	}

	var reqID uuid.UUID
	if request != nil {
		reqID = request.ID
	}

	return &llm.LLMResponse{
		ID:           uuid.New(),
		RequestID:    reqID,
		Content:      content,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
		Usage: llm.Usage{
			PromptTokens:     len(prompt) / 4,
			CompletionTokens: len(content) / 4,
			TotalTokens:      (len(prompt) + len(content)) / 4,
		},
	}, nil
}

// GenerateStream emits the full canned/echo response as a single chunk on
// the channel and then closes it. The channel must be writable; if the
// caller passes a nil channel, the function returns an error.
func (p *FakeLLMProvider) GenerateStream(ctx context.Context, request *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	if ch == nil {
		return errors.New("FakeLLMProvider.GenerateStream: nil channel")
	}
	defer close(ch)

	resp, err := p.Generate(ctx, request)
	if err != nil {
		return err
	}
	select {
	case ch <- *resp:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// IsAvailable always returns true — the fake has no external dependencies.
func (p *FakeLLMProvider) IsAvailable(ctx context.Context) bool {
	return true
}

// GetHealth returns a healthy status with zero latency.
func (p *FakeLLMProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{
		Status:     "healthy",
		LastCheck:  time.Now(),
		Latency:    0,
		ModelCount: 0,
		ErrorCount: 0,
		Message:    "fake test provider — always healthy",
	}, nil
}

// Close is a no-op; the fake holds no external resources.
func (p *FakeLLMProvider) Close() error {
	return nil
}

// GetContextWindow returns a fixed 100k tokens — large enough that callers
// never hit the cap during tests.
func (p *FakeLLMProvider) GetContextWindow() int {
	return 100000
}

// CountTokens returns a conservative char-based estimate (1 token ≈ 4 chars).
// Empty input returns (0, nil) per the llm.Provider contract.
func (p *FakeLLMProvider) CountTokens(text string) (int, error) {
	if len(text) == 0 {
		return 0, nil
	}
	count := len(text) / 4
	if count == 0 {
		count = 1
	}
	return count, nil
}
