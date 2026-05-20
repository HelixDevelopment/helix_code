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

// P3-T04 — unit tests for the agent-side tool-dispatch driver.
//
// Mocks/fakes here are permitted: this is a *_test.go unit test invoked
// WITHOUT the integration build tag (CONST-050(A)).

// dispatchProbe is a parallel-safe (read-only) probe tool that observes peak
// concurrency the same way internal/tools' probeTool does.
type dispatchProbe struct {
	name  string
	live  *int32
	peak  *int32
	calls *int32
	delay time.Duration
	ro    bool
}

func (d *dispatchProbe) Name() string                          { return d.name }
func (d *dispatchProbe) Description() string                   { return "dispatch-probe" }
func (d *dispatchProbe) Schema() tools.ToolSchema              { return tools.ToolSchema{Type: "object"} }
func (d *dispatchProbe) Category() tools.ToolCategory          { return tools.CategoryFileSystem }
func (d *dispatchProbe) Validate(map[string]interface{}) error { return nil }

func (d *dispatchProbe) RequiresApproval() approval.ApprovalLevel {
	if d.ro {
		return approval.LevelReadOnly
	}
	return approval.LevelRun
}

func (d *dispatchProbe) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if d.calls != nil {
		atomic.AddInt32(d.calls, 1)
	}
	cur := atomic.AddInt32(d.live, 1)
	for {
		old := atomic.LoadInt32(d.peak)
		if cur <= old || atomic.CompareAndSwapInt32(d.peak, old, cur) {
			break
		}
	}
	time.Sleep(d.delay)
	atomic.AddInt32(d.live, -1)
	return "done:" + d.name, nil
}

func newDispatchRegistry(t *testing.T) *tools.ToolRegistry {
	t.Helper()
	r, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	return r
}

func toolCall(id, name string, args map[string]interface{}) llm.ToolCall {
	return llm.ToolCall{
		ID:   id,
		Type: "function",
		Function: llm.ToolCallFunc{Name: name, Arguments: args},
	}
}

// TestDispatchTurn_IndependentCallsRunConcurrently — anti-bluff concurrency
// proof: three read-only tool calls in one turn reach peak concurrency > 1.
func TestDispatchTurn_IndependentCallsRunConcurrently(t *testing.T) {
	r := newDispatchRegistry(t)
	var live, peak, calls int32
	for i := 0; i < 3; i++ {
		r.Register(&dispatchProbe{
			name: fmt.Sprintf("ro_%d", i), live: &live, peak: &peak, calls: &calls,
			delay: 40 * time.Millisecond, ro: true,
		})
	}
	turn := []llm.ToolCall{
		toolCall("t0", "ro_0", map[string]interface{}{}),
		toolCall("t1", "ro_1", map[string]interface{}{}),
		toolCall("t2", "ro_2", map[string]interface{}{}),
	}
	results, summary := DispatchTurn(context.Background(), r, turn, 0)

	require.Len(t, results, 3)
	require.Equal(t, int32(3), atomic.LoadInt32(&calls))
	require.Greater(t, atomic.LoadInt32(&peak), int32(1),
		"CONCURRENCY PROBE: independent tool calls in a turn must run in parallel")
	require.Equal(t, 3, summary.ParallelCount)
	require.Equal(t, 0, summary.SerialCount)
	require.Equal(t, 3, summary.TotalCalls)
}

// TestDispatchTurn_DependentCallsSerialise — two writes to the same file in a
// turn must serialise: peak concurrency stays 1.
func TestDispatchTurn_DependentCallsSerialise(t *testing.T) {
	r := newDispatchRegistry(t)
	var live, peak, calls int32
	r.Register(&dispatchProbe{
		name: "fs_write", live: &live, peak: &peak, calls: &calls,
		delay: 30 * time.Millisecond, ro: false,
	})
	turn := []llm.ToolCall{
		toolCall("w0", "fs_write", map[string]interface{}{"path": "shared.go", "content": "a"}),
		toolCall("w1", "fs_write", map[string]interface{}{"path": "shared.go", "content": "b"}),
	}
	results, summary := DispatchTurn(context.Background(), r, turn, 0)
	require.Len(t, results, 2)
	require.Equal(t, int32(1), atomic.LoadInt32(&peak),
		"conflicting same-file writes must serialise — peak concurrency must be 1")
	require.Equal(t, 0, summary.ParallelCount)
	require.Equal(t, 2, summary.SerialCount)
}

// TestDispatchTurn_ResultOrderMatchesRequestOrder — results in request order
// regardless of completion order (first call sleeps longest).
func TestDispatchTurn_ResultOrderMatchesRequestOrder(t *testing.T) {
	r := newDispatchRegistry(t)
	var live, peak int32
	delays := []time.Duration{60 * time.Millisecond, 30 * time.Millisecond, 5 * time.Millisecond}
	for i, d := range delays {
		r.Register(&dispatchProbe{
			name: fmt.Sprintf("o_%d", i), live: &live, peak: &peak, delay: d, ro: true,
		})
	}
	turn := []llm.ToolCall{
		toolCall("first", "o_0", map[string]interface{}{}),
		toolCall("second", "o_1", map[string]interface{}{}),
		toolCall("third", "o_2", map[string]interface{}{}),
	}
	results, _ := DispatchTurn(context.Background(), r, turn, 0)
	require.Len(t, results, 3)
	require.Equal(t, "first", results[0].CallID)
	require.Equal(t, "done:o_0", results[0].Result)
	require.Equal(t, "second", results[1].CallID)
	require.Equal(t, "done:o_1", results[1].Result)
	require.Equal(t, "third", results[2].CallID)
	require.Equal(t, "done:o_2", results[2].Result)
}

// TestDispatchTurn_OutputEqualsSerial — a mixed turn produces results identical
// to executing each call serially.
func TestDispatchTurn_OutputEqualsSerial(t *testing.T) {
	build := func() *tools.ToolRegistry {
		r := newDispatchRegistry(t)
		var live, peak int32
		r.Register(&dispatchProbe{name: "ro_0", live: &live, peak: &peak, delay: 5 * time.Millisecond, ro: true})
		r.Register(&dispatchProbe{name: "ro_1", live: &live, peak: &peak, delay: 5 * time.Millisecond, ro: true})
		r.Register(&dispatchProbe{name: "fs_write", live: &live, peak: &peak, delay: 5 * time.Millisecond, ro: false})
		return r
	}
	turn := []llm.ToolCall{
		toolCall("a", "ro_0", map[string]interface{}{}),
		toolCall("b", "fs_write", map[string]interface{}{"path": "x.go"}),
		toolCall("c", "ro_1", map[string]interface{}{}),
	}

	rSerial := build()
	serial := make([]interface{}, len(turn))
	for i, c := range turn {
		res, err := rSerial.Execute(context.Background(), c.Function.Name, c.Function.Arguments)
		require.NoError(t, err)
		serial[i] = res
	}

	rBatch := build()
	results, _ := DispatchTurn(context.Background(), rBatch, turn, 0)
	require.Len(t, results, len(turn))
	for i := range turn {
		require.NoError(t, results[i].Err)
		require.Equal(t, serial[i], results[i].Result,
			"dispatched result[%d] must equal the serial-execution result", i)
	}
}

// TestDispatchTurn_NilRegistryAndEmptyTurn — defensive paths.
func TestDispatchTurn_NilRegistryAndEmptyTurn(t *testing.T) {
	res, sum := DispatchTurn(context.Background(), nil, []llm.ToolCall{toolCall("x", "y", nil)}, 0)
	require.Nil(t, res)
	require.Equal(t, 1, sum.TotalCalls)

	r := newDispatchRegistry(t)
	res2, sum2 := DispatchTurn(context.Background(), r, nil, 0)
	require.Nil(t, res2)
	require.Equal(t, 0, sum2.TotalCalls)
}

// TestDispatchTurnWithRegistry — the BaseAgent-bound wrapper drives the agent's
// configured registry.
func TestDispatchTurnWithRegistry(t *testing.T) {
	r := newDispatchRegistry(t)
	var live, peak, calls int32
	for i := 0; i < 2; i++ {
		r.Register(&dispatchProbe{
			name: fmt.Sprintf("ag_%d", i), live: &live, peak: &peak, calls: &calls,
			delay: 20 * time.Millisecond, ro: true,
		})
	}
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID: "dispatch-agent", Type: AgentTypePlanning, Name: "Dispatch Agent",
	})
	agent.SetToolRegistry(r)

	turn := []llm.ToolCall{
		toolCall("g0", "ag_0", map[string]interface{}{}),
		toolCall("g1", "ag_1", map[string]interface{}{}),
	}
	results, summary := agent.DispatchTurnWithRegistry(context.Background(), turn, 0)
	require.Len(t, results, 2)
	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
	require.Equal(t, 2, summary.ParallelCount)

	// Agent with no registry returns nil.
	bare := NewBaseAgentFromConfig(&AgentConfig{ID: "bare", Type: AgentTypePlanning, Name: "Bare"})
	r2, s2 := bare.DispatchTurnWithRegistry(context.Background(), turn, 0)
	require.Nil(t, r2)
	require.Equal(t, 2, s2.TotalCalls)
}

// TestDispatchTurn_ParallelFasterThanSerial — wall-clock benchmark-style proof
// that parallelising a 3-call read-only turn beats serial execution.
func TestDispatchTurn_ParallelFasterThanSerial(t *testing.T) {
	const delay = 50 * time.Millisecond
	r := newDispatchRegistry(t)
	var live, peak int32
	for i := 0; i < 3; i++ {
		r.Register(&dispatchProbe{
			name: fmt.Sprintf("sp_%d", i), live: &live, peak: &peak, delay: delay, ro: true,
		})
	}
	turn := []llm.ToolCall{
		toolCall("s0", "sp_0", map[string]interface{}{}),
		toolCall("s1", "sp_1", map[string]interface{}{}),
		toolCall("s2", "sp_2", map[string]interface{}{}),
	}

	// Serial reference: 3 × delay.
	serialStart := time.Now()
	for _, c := range turn {
		_, err := r.Execute(context.Background(), c.Function.Name, c.Function.Arguments)
		require.NoError(t, err)
	}
	serialWall := time.Since(serialStart)

	// Parallel dispatch: ~1 × delay.
	_, summary := DispatchTurn(context.Background(), r, turn, 0)
	require.Less(t, summary.Wallclock, serialWall,
		"WALL-CLOCK PROOF: parallel turn (%v) must be faster than serial (%v)",
		summary.Wallclock, serialWall)
	// Parallel wall-clock should be well under 2× the single-call delay.
	require.Less(t, summary.Wallclock, 2*delay,
		"parallel turn wall-clock %v should be near a single call's %v", summary.Wallclock, delay)
}

// BenchmarkDispatchTurn_SerialVsParallel — multi-tool turn wall-clock,
// serial baseline vs parallel dispatch. Run with: go test -bench=DispatchTurn.
func BenchmarkDispatchTurn_SerialVsParallel(b *testing.B) {
	const delay = 5 * time.Millisecond
	mk := func() (*tools.ToolRegistry, []llm.ToolCall) {
		r, _ := tools.NewToolRegistry(tools.DefaultRegistryConfig())
		var live, peak int32
		for i := 0; i < 6; i++ {
			r.Register(&dispatchProbe{
				name: fmt.Sprintf("bm_%d", i), live: &live, peak: &peak, delay: delay, ro: true,
			})
		}
		turn := make([]llm.ToolCall, 6)
		for i := 0; i < 6; i++ {
			turn[i] = toolCall(fmt.Sprintf("c%d", i), fmt.Sprintf("bm_%d", i), map[string]interface{}{})
		}
		return r, turn
	}

	b.Run("serial", func(b *testing.B) {
		r, turn := mk()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			for _, c := range turn {
				_, _ = r.Execute(context.Background(), c.Function.Name, c.Function.Arguments)
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		r, turn := mk()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			DispatchTurn(context.Background(), r, turn, 0)
		}
	})
}
