package agent

import (
	"context"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools"
)

// P3-T04 — Aggressive tool-call parallelism (speed programme Phase 3).
//
// This file is the agent-side driver for the tool-dispatch loop. When an LLM
// turn returns multiple tool calls, DispatchTurn classifies them and routes the
// batch through tools.ToolRegistry.ExecuteBatch: independent (read-only /
// side-effect-free / non-conflicting) calls run concurrently through a bounded
// worker pool, while conflicting or ordering-dependent calls run serially in
// request order. Results are assembled back in the LLM-requested order so the
// turn outcome is identical to fully serial dispatch.
//
// R2 #5 (Claude Code /batch up to 10×); R3 §2.1; R4 O12.

// ToolDispatchResult is one tool call's outcome inside a dispatched turn. The
// slice returned by DispatchTurn has one entry per requested call, in the SAME
// order as the input (the LLM-requested order), never completion order.
type ToolDispatchResult struct {
	// CallID is the LLM-assigned tool-call ID (passed through for correlation).
	CallID string
	// ToolName is the dispatched tool name.
	ToolName string
	// Result is the tool's return value (nil on error).
	Result interface{}
	// Err is the tool's error (nil on success).
	Err error
	// RanParallel is true when the call ran in the concurrent wave.
	RanParallel bool
}

// ToolDispatchSummary carries turn-level diagnostics for the dispatched batch.
// It is the anti-bluff evidence surface: ParallelCount > 1 with a measured
// Wall-clock proves real concurrency happened.
type ToolDispatchSummary struct {
	// TotalCalls is the number of tool calls in the turn.
	TotalCalls int
	// ParallelCount is how many calls ran in the concurrent wave.
	ParallelCount int
	// SerialCount is how many calls ran in the serial wave.
	SerialCount int
	// Wallclock is the total wall-clock time spent dispatching the turn.
	Wallclock time.Duration
}

// DispatchTurn executes every tool call requested in a single LLM turn.
//
// Independent calls (read-only / side-effect-free tools whose mutation targets
// do not conflict with any other call in the turn) are run concurrently through
// the registry's bounded worker pool. Calls that conflict (writes to the same
// file, run/shell calls, calls that may depend on a prior call's output) run
// serially in the LLM-requested order.
//
// GUARANTEE: the returned []ToolDispatchResult is in the SAME order as calls,
// and the turn outcome is identical to fully serial execution — only genuinely
// independent calls parallelise. maxConcurrency <= 0 selects the registry
// default (10, matching R2 #5).
//
// A nil registry returns a nil slice + a zero summary (caller has no tools
// wired). A turn with a single call degrades to a plain serial dispatch.
func DispatchTurn(ctx context.Context, registry *tools.ToolRegistry, calls []llm.ToolCall, maxConcurrency int) ([]ToolDispatchResult, ToolDispatchSummary) {
	if registry == nil || len(calls) == 0 {
		return nil, ToolDispatchSummary{TotalCalls: len(calls)}
	}

	reqs := make([]tools.ToolCallRequest, len(calls))
	for i, c := range calls {
		reqs[i] = tools.ToolCallRequest{
			ID:     c.ID,
			Name:   c.Function.Name,
			Params: c.Function.Arguments,
		}
	}

	start := time.Now()
	batch := registry.ExecuteBatch(ctx, reqs, maxConcurrency)
	wall := time.Since(start)

	results := make([]ToolDispatchResult, len(batch))
	summary := ToolDispatchSummary{TotalCalls: len(batch), Wallclock: wall}
	for i, b := range batch {
		results[i] = ToolDispatchResult{
			CallID:      b.ID,
			ToolName:    b.Name,
			Result:      b.Result,
			Err:         b.Err,
			RanParallel: b.RanParallel,
		}
		if b.RanParallel {
			summary.ParallelCount++
		} else {
			summary.SerialCount++
		}
	}
	return results, summary
}

// DispatchTurnWithRegistry is the BaseAgent-bound convenience wrapper around
// DispatchTurn — it dispatches the turn through the agent's configured tool
// registry. Returns a nil slice when the agent has no registry wired.
func (a *BaseAgent) DispatchTurnWithRegistry(ctx context.Context, calls []llm.ToolCall, maxConcurrency int) ([]ToolDispatchResult, ToolDispatchSummary) {
	return DispatchTurn(ctx, a.GetToolRegistry(), calls, maxConcurrency)
}
