package agent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools"
	"github.com/stretchr/testify/require"
)

// P3-T04 follow-up — integration-style proof that the REAL tools.ToolRegistry
// satisfies the llm.BatchToolExecutor interface and that the live-path seam
// (ToolCallingProvider.executeToolCalls -> BatchToolExecutor.ExecuteToolBatch)
// runs independent calls concurrently while producing output identical to
// serial dispatch.
//
// This test lives in internal/agent because internal/agent may import BOTH
// internal/llm and internal/tools — internal/llm itself cannot import
// internal/tools (import cycle), which is exactly why the BatchToolExecutor
// seam is consumer-defined in internal/llm.
//
// Mocks/fakes here are permitted: this is a *_test.go unit test invoked WITHOUT
// the integration build tag (CONST-050(A)). It exercises the real
// tools.ToolRegistry against real in-process tool implementations.

// TestToolRegistry_SatisfiesBatchToolExecutor is a compile-time + runtime
// assertion that *tools.ToolRegistry satisfies the llm.BatchToolExecutor
// interface — the contract that wires the parallel facility into the live path.
func TestToolRegistry_SatisfiesBatchToolExecutor(t *testing.T) {
	r, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	var _ llm.BatchToolExecutor = r // compile-time guarantee
	require.Implements(t, (*llm.BatchToolExecutor)(nil), r,
		"tools.ToolRegistry MUST satisfy llm.BatchToolExecutor for the live-path wiring")
}

// TestExecuteToolBatch_RealRegistryRunsIndependentCallsConcurrently — anti-bluff
// concurrency proof on the REAL registry bridging method ExecuteToolBatch:
// three read-only probe tools in one turn reach peak concurrency > 1.
func TestExecuteToolBatch_RealRegistryRunsIndependentCallsConcurrently(t *testing.T) {
	r := newDispatchRegistry(t)
	var live, peak, calls int32
	for i := 0; i < 3; i++ {
		r.Register(&dispatchProbe{
			name: fmt.Sprintf("lp_ro_%d", i), live: &live, peak: &peak, calls: &calls,
			delay: 40 * time.Millisecond, ro: true,
		})
	}
	turn := []llm.ToolCall{
		toolCall("t0", "lp_ro_0", map[string]interface{}{}),
		toolCall("t1", "lp_ro_1", map[string]interface{}{}),
		toolCall("t2", "lp_ro_2", map[string]interface{}{}),
	}
	results := r.ExecuteToolBatch(context.Background(), turn, 0)
	require.Len(t, results, 3)
	require.Equal(t, int32(3), atomic.LoadInt32(&calls))
	require.Greater(t, atomic.LoadInt32(&peak), int32(1),
		"CONCURRENCY PROBE: ExecuteToolBatch must run independent tool calls in parallel on the real registry")
	for i := range results {
		require.True(t, results[i].RanParallel, "result[%d] must report RanParallel", i)
		require.Equal(t, fmt.Sprintf("done:lp_ro_%d", i), results[i].Result)
	}
}

// TestExecuteToolBatch_RealRegistryDependentCallsSerialise — two writes to the
// same file must serialise: peak concurrency stays 1.
func TestExecuteToolBatch_RealRegistryDependentCallsSerialise(t *testing.T) {
	r := newDispatchRegistry(t)
	var live, peak, calls int32
	r.Register(&dispatchProbe{
		name: "lp_fs_write", live: &live, peak: &peak, calls: &calls,
		delay: 30 * time.Millisecond, ro: false,
	})
	turn := []llm.ToolCall{
		toolCall("w0", "lp_fs_write", map[string]interface{}{"path": "shared.go", "content": "a"}),
		toolCall("w1", "lp_fs_write", map[string]interface{}{"path": "shared.go", "content": "b"}),
	}
	results := r.ExecuteToolBatch(context.Background(), turn, 0)
	require.Len(t, results, 2)
	require.Equal(t, int32(1), atomic.LoadInt32(&peak),
		"conflicting same-file writes must serialise — peak concurrency must be 1")
	for i := range results {
		require.False(t, results[i].RanParallel)
	}
}

// TestExecuteToolBatch_RealRegistryOutputEqualsSerial — output-equality proof:
// a mixed turn through ExecuteToolBatch produces results identical to executing
// each call serially through tools.ToolRegistry.Execute.
func TestExecuteToolBatch_RealRegistryOutputEqualsSerial(t *testing.T) {
	build := func() *tools.ToolRegistry {
		r := newDispatchRegistry(t)
		var live, peak int32
		r.Register(&dispatchProbe{name: "eq_ro_0", live: &live, peak: &peak, delay: 5 * time.Millisecond, ro: true})
		r.Register(&dispatchProbe{name: "eq_ro_1", live: &live, peak: &peak, delay: 5 * time.Millisecond, ro: true})
		r.Register(&dispatchProbe{name: "eq_fs_write", live: &live, peak: &peak, delay: 5 * time.Millisecond, ro: false})
		return r
	}
	turn := []llm.ToolCall{
		toolCall("a", "eq_ro_0", map[string]interface{}{}),
		toolCall("b", "eq_fs_write", map[string]interface{}{"path": "x.go"}),
		toolCall("c", "eq_ro_1", map[string]interface{}{}),
	}

	rSerial := build()
	serial := make([]interface{}, len(turn))
	for i, c := range turn {
		res, err := rSerial.Execute(context.Background(), c.Function.Name, c.Function.Arguments)
		require.NoError(t, err)
		serial[i] = res
	}

	rBatch := build()
	results := rBatch.ExecuteToolBatch(context.Background(), turn, 0)
	require.Len(t, results, len(turn))
	for i := range turn {
		require.Equal(t, turn[i].ID, results[i].CallID,
			"CallID at position %d must match the request", i)
		require.Equal(t, serial[i], results[i].Result,
			"ExecuteToolBatch result[%d] must equal the serial-execution result", i)
	}
}

// TestExecuteToolBatch_SameToolTwiceNotMerged — the "keyed by name" fix proven
// on the real registry: a turn calling the same read-only tool twice yields two
// distinct ordered entries keyed by call ID.
func TestExecuteToolBatch_SameToolTwiceNotMerged(t *testing.T) {
	r := newDispatchRegistry(t)
	var live, peak int32
	r.Register(&dispatchProbe{name: "twice", live: &live, peak: &peak, delay: 2 * time.Millisecond, ro: true})

	turn := []llm.ToolCall{
		toolCall("first", "twice", map[string]interface{}{}),
		toolCall("second", "twice", map[string]interface{}{}),
	}
	results := r.ExecuteToolBatch(context.Background(), turn, 0)
	require.Len(t, results, 2, "two calls to the same tool must NOT collapse into one entry")
	require.Equal(t, "first", results[0].CallID)
	require.Equal(t, "second", results[1].CallID)
}

// TestExecuteToolBatch_ErrorNormalisation — an unresolvable tool surfaces as a
// "Tool error:" string at its deterministic request position, matching the
// serial-path shape the final-prompt builder expects.
func TestExecuteToolBatch_ErrorNormalisation(t *testing.T) {
	r := newDispatchRegistry(t)
	turn := []llm.ToolCall{
		toolCall("x", "no_such_tool", map[string]interface{}{}),
	}
	results := r.ExecuteToolBatch(context.Background(), turn, 0)
	require.Len(t, results, 1)
	require.Equal(t, "no_such_tool", results[0].ToolName)
	s, ok := results[0].Result.(string)
	require.True(t, ok, "error result must be a string")
	require.Contains(t, s, "Tool error:")
}

// TestExecuteToolBatch_EmptyTurn — defensive: an empty turn yields an empty
// (non-nil) slice.
func TestExecuteToolBatch_EmptyTurn(t *testing.T) {
	r := newDispatchRegistry(t)
	results := r.ExecuteToolBatch(context.Background(), nil, 0)
	require.NotNil(t, results)
	require.Len(t, results, 0)
}

// ensure approval import is used (probe RequiresApproval references it).
var _ = approval.LevelReadOnly
