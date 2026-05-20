package llm

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// P3-T04 follow-up — unit tests for the LIVE tool-execution path
// (ToolCallingProvider.executeToolCalls) now routing through the parallel
// tool-dispatch facility via the llm.BatchToolExecutor seam.
//
// Mocks/fakes here are permitted: this is a *_test.go unit test invoked WITHOUT
// the integration build tag (CONST-050(A)). The real concrete
// llm.BatchToolExecutor is tools.ToolRegistry — exercised end-to-end by the
// internal/agent and internal/tools test suites; this file proves the
// internal/llm WIRING (the type-assertion seam + ordering contract).

// ---------------------------------------------------------------------------
// Fakes — a serial-only ToolExecutor and a batch-capable BatchToolExecutor.
// ---------------------------------------------------------------------------

// serialFakeExecutor implements only ToolExecutor — exercises the serial
// fallback path of executeToolCalls.
type serialFakeExecutor struct {
	mu    sync.Mutex
	order []string
}

func (s *serialFakeExecutor) Execute(_ context.Context, name string, _ map[string]interface{}) (interface{}, error) {
	s.mu.Lock()
	s.order = append(s.order, name)
	s.mu.Unlock()
	return "serial:" + name, nil
}

// batchFakeExecutor implements BatchToolExecutor. ExecuteToolBatch runs every
// call concurrently through a real goroutine wave (mirroring the
// tools.ToolRegistry parallel wave for read-only tools) so the concurrency
// probe below observes peak concurrency > 1. Results are assembled back in
// request order regardless of completion order.
type batchFakeExecutor struct {
	live  int32
	peak  int32
	calls int32
	delay time.Duration
}

// Execute satisfies the embedded ToolExecutor — used only by the serial
// fallback, never hit when ExecuteToolBatch is available.
func (b *batchFakeExecutor) Execute(_ context.Context, name string, _ map[string]interface{}) (interface{}, error) {
	return "serial:" + name, nil
}

func (b *batchFakeExecutor) ExecuteToolBatch(_ context.Context, calls []ToolCall, _ int) []ToolCallResult {
	out := make([]ToolCallResult, len(calls))
	var wg sync.WaitGroup
	for i, c := range calls {
		wg.Add(1)
		go func(i int, c ToolCall) {
			defer wg.Done()
			atomic.AddInt32(&b.calls, 1)
			cur := atomic.AddInt32(&b.live, 1)
			for {
				old := atomic.LoadInt32(&b.peak)
				if cur <= old || atomic.CompareAndSwapInt32(&b.peak, old, cur) {
					break
				}
			}
			time.Sleep(b.delay)
			atomic.AddInt32(&b.live, -1)
			// Slot write is disjoint per index — no mutex on out needed.
			out[i] = ToolCallResult{
				CallID:      c.ID,
				ToolName:    c.Function.Name,
				Result:      "batch:" + c.Function.Name,
				RanParallel: true,
			}
		}(i, c)
	}
	wg.Wait()
	return out
}

func batchTool(name string) Tool {
	return Tool{Type: "function", Function: ToolFunction{Name: name, Description: name}}
}

func batchCall(id, name string) ToolCall {
	return ToolCall{ID: id, Type: "function", Function: ToolCallFunc{Name: name}}
}

// TestExecuteToolCalls_RoutesThroughBatchExecutor — anti-bluff concurrency
// proof: when the wired ToolExecutor implements BatchToolExecutor, the LIVE
// executeToolCalls path dispatches the whole turn through it, and three
// independent calls reach peak concurrency > 1.
func TestExecuteToolCalls_RoutesThroughBatchExecutor(t *testing.T) {
	provider := NewToolCallingProvider(&MockProvider{})
	for i := 0; i < 3; i++ {
		require.NoError(t, provider.RegisterTool(batchTool(fmt.Sprintf("ro_%d", i))))
	}
	exec := &batchFakeExecutor{delay: 40 * time.Millisecond}
	provider.SetToolExecutor(exec)

	turn := []ToolCall{
		batchCall("t0", "ro_0"),
		batchCall("t1", "ro_1"),
		batchCall("t2", "ro_2"),
	}
	results, err := provider.executeToolCalls(context.Background(), turn)
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, int32(3), atomic.LoadInt32(&exec.calls))
	require.Greater(t, atomic.LoadInt32(&exec.peak), int32(1),
		"CONCURRENCY PROBE: the LIVE executeToolCalls path must run independent tool calls in parallel")
	for i := range results {
		require.True(t, results[i].RanParallel, "result[%d] must report RanParallel", i)
	}
}

// TestExecuteToolCalls_ResultOrderMatchesRequestOrder — the live path returns
// results in the LLM-requested order regardless of completion order, and a turn
// that calls the SAME tool twice yields TWO distinct entries (the pre-P3-T04
// name-keyed map silently merged them).
func TestExecuteToolCalls_ResultOrderMatchesRequestOrder(t *testing.T) {
	provider := NewToolCallingProvider(&MockProvider{})
	require.NoError(t, provider.RegisterTool(batchTool("fs_read")))
	require.NoError(t, provider.RegisterTool(batchTool("grep")))
	provider.SetToolExecutor(&batchFakeExecutor{delay: 5 * time.Millisecond})

	turn := []ToolCall{
		batchCall("call_a", "fs_read"),
		batchCall("call_b", "grep"),
		batchCall("call_c", "fs_read"), // SAME tool again — must NOT collapse.
	}
	results, err := provider.executeToolCalls(context.Background(), turn)
	require.NoError(t, err)
	require.Len(t, results, 3, "two calls to fs_read must yield two ordered entries")
	assert.Equal(t, "call_a", results[0].CallID)
	assert.Equal(t, "fs_read", results[0].ToolName)
	assert.Equal(t, "call_b", results[1].CallID)
	assert.Equal(t, "grep", results[1].ToolName)
	assert.Equal(t, "call_c", results[2].CallID)
	assert.Equal(t, "fs_read", results[2].ToolName)
}

// TestExecuteToolCalls_BatchOutputEqualsSerial — output-equality proof: the
// parallel-dispatched live path produces results identical to executing each
// call serially through the same logic.
func TestExecuteToolCalls_BatchOutputEqualsSerial(t *testing.T) {
	turn := []ToolCall{
		batchCall("a", "ro_0"),
		batchCall("b", "ro_1"),
		batchCall("c", "ro_2"),
	}

	// Serial reference: ToolExecutor without batch capability.
	serialProvider := NewToolCallingProvider(&MockProvider{})
	for i := 0; i < 3; i++ {
		require.NoError(t, serialProvider.RegisterTool(batchTool(fmt.Sprintf("ro_%d", i))))
	}
	serialProvider.SetToolExecutor(&serialFakeExecutor{})
	serialResults, err := serialProvider.executeToolCalls(context.Background(), turn)
	require.NoError(t, err)

	// Batch path: same turn through a BatchToolExecutor.
	batchProvider := NewToolCallingProvider(&MockProvider{})
	for i := 0; i < 3; i++ {
		require.NoError(t, batchProvider.RegisterTool(batchTool(fmt.Sprintf("ro_%d", i))))
	}
	batchProvider.SetToolExecutor(&batchFakeExecutor{delay: 5 * time.Millisecond})
	batchResults, err := batchProvider.executeToolCalls(context.Background(), turn)
	require.NoError(t, err)

	require.Len(t, batchResults, len(serialResults))
	for i := range serialResults {
		// CallID + ToolName + ordering must be identical; only the Result
		// payload prefix differs because the two fakes label differently —
		// assert the structural equality that the LLM actually depends on.
		assert.Equal(t, serialResults[i].CallID, batchResults[i].CallID,
			"CallID at position %d must match serial", i)
		assert.Equal(t, serialResults[i].ToolName, batchResults[i].ToolName,
			"ToolName at position %d must match serial", i)
	}
}

// TestExecuteToolCalls_SerialFallbackPreservesOrder — when no batch-capable
// executor is wired the serial fallback still returns an ordered slice in
// request order (the no-regression contract for non-registry executors).
func TestExecuteToolCalls_SerialFallbackPreservesOrder(t *testing.T) {
	provider := NewToolCallingProvider(&MockProvider{})
	require.NoError(t, provider.RegisterTool(batchTool("alpha")))
	require.NoError(t, provider.RegisterTool(batchTool("beta")))
	exec := &serialFakeExecutor{}
	provider.SetToolExecutor(exec)

	turn := []ToolCall{
		batchCall("1", "alpha"),
		batchCall("2", "beta"),
		batchCall("3", "alpha"),
	}
	results, err := provider.executeToolCalls(context.Background(), turn)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, []string{"alpha", "beta", "alpha"}, exec.order,
		"serial fallback must run calls in request order")
	assert.Equal(t, "1", results[0].CallID)
	assert.Equal(t, "3", results[2].CallID)
	for i := range results {
		assert.False(t, results[i].RanParallel,
			"serial-fallback results must not claim RanParallel")
	}
}

// BenchmarkExecuteToolCalls_SerialVsBatch — live multi-tool turn wall-clock,
// serial-fallback baseline vs batch-dispatched. Run: go test -bench=ExecuteToolCalls.
func BenchmarkExecuteToolCalls_SerialVsBatch(b *testing.B) {
	const delay = 5 * time.Millisecond
	mkTurn := func() []ToolCall {
		turn := make([]ToolCall, 6)
		for i := 0; i < 6; i++ {
			turn[i] = batchCall(fmt.Sprintf("c%d", i), fmt.Sprintf("bm_%d", i))
		}
		return turn
	}
	mkProvider := func(exec ToolExecutor) *ToolCallingProvider {
		p := NewToolCallingProvider(&MockProvider{})
		for i := 0; i < 6; i++ {
			_ = p.RegisterTool(batchTool(fmt.Sprintf("bm_%d", i)))
		}
		p.SetToolExecutor(exec)
		return p
	}

	b.Run("serial", func(b *testing.B) {
		// slowSerialExecutor sleeps per call so the 6-call turn costs 6×delay.
		p := mkProvider(&slowSerialExecutor{delay: delay})
		turn := mkTurn()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			_, _ = p.executeToolCalls(context.Background(), turn)
		}
	})
	b.Run("batch", func(b *testing.B) {
		p := mkProvider(&batchFakeExecutor{delay: delay})
		turn := mkTurn()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			_, _ = p.executeToolCalls(context.Background(), turn)
		}
	})
}

// slowSerialExecutor is a benchmark-only ToolExecutor whose per-call sleep makes
// the serial baseline's cost proportional to the turn size.
type slowSerialExecutor struct{ delay time.Duration }

func (s *slowSerialExecutor) Execute(_ context.Context, name string, _ map[string]interface{}) (interface{}, error) {
	time.Sleep(s.delay)
	return "serial:" + name, nil
}
