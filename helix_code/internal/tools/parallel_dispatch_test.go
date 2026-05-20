package tools

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/approval"
	"github.com/stretchr/testify/require"
)

// P3-T04 — unit tests for aggressive tool-call parallelism.
//
// Mocks/fakes in this file are permitted: this is a *_test.go unit test
// invoked WITHOUT the integration build tag (CONST-050(A)).

// ---- test tool doubles --------------------------------------------------

// probeTool is a parallel-safe (read-only) test tool. Its Execute increments a
// live concurrency counter on entry, records the running peak, sleeps, then
// decrements — so a test can observe whether two probeTools ever ran at once.
type probeTool struct {
	name    string
	live    *int32 // shared live-concurrency counter
	peak    *int32 // shared observed-peak counter
	delay   time.Duration
	calls   *int32 // shared total-call counter
	readOnly bool
}

func (p *probeTool) Name() string        { return p.name }
func (p *probeTool) Description() string { return "probe" }
func (p *probeTool) Schema() ToolSchema  { return ToolSchema{Type: "object"} }
func (p *probeTool) Category() ToolCategory {
	return CategoryFileSystem
}
func (p *probeTool) Validate(map[string]interface{}) error { return nil }

func (p *probeTool) RequiresApproval() approval.ApprovalLevel {
	if p.readOnly {
		return approval.LevelReadOnly
	}
	return approval.LevelRun
}

func (p *probeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if p.calls != nil {
		atomic.AddInt32(p.calls, 1)
	}
	cur := atomic.AddInt32(p.live, 1)
	// Record the peak observed concurrency.
	for {
		old := atomic.LoadInt32(p.peak)
		if cur <= old || atomic.CompareAndSwapInt32(p.peak, old, cur) {
			break
		}
	}
	time.Sleep(p.delay)
	atomic.AddInt32(p.live, -1)
	return "ok:" + p.name, nil
}

func newBatchTestRegistry(t *testing.T) *ToolRegistry {
	t.Helper()
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	return r
}

// ---- TestExecuteBatch_IndependentCallsRunConcurrently -------------------
// Anti-bluff proof: three read-only probe tools dispatched in one batch MUST
// reach a peak observed concurrency > 1 — proving real parallel execution.
func TestExecuteBatch_IndependentCallsRunConcurrently(t *testing.T) {
	r := newBatchTestRegistry(t)
	var live, peak, calls int32
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("probe_ro_%d", i)
		r.Register(&probeTool{
			name: name, live: &live, peak: &peak, calls: &calls,
			delay: 40 * time.Millisecond, readOnly: true,
		})
	}
	reqs := []ToolCallRequest{
		{ID: "c0", Name: "probe_ro_0", Params: map[string]interface{}{}},
		{ID: "c1", Name: "probe_ro_1", Params: map[string]interface{}{}},
		{ID: "c2", Name: "probe_ro_2", Params: map[string]interface{}{}},
	}
	results := r.ExecuteBatch(context.Background(), reqs, 0)

	require.Len(t, results, 3)
	require.Equal(t, int32(3), atomic.LoadInt32(&calls), "all 3 tools must execute")
	require.Greater(t, atomic.LoadInt32(&peak), int32(1),
		"CONCURRENCY PROBE: peak observed concurrency must exceed 1 — independent read-only calls did NOT run in parallel")
	for i, res := range results {
		require.NoError(t, res.Err)
		require.True(t, res.RanParallel, "call %d should have run in the parallel wave", i)
	}
}

// ---- TestExecuteBatch_ConflictingCallsSerialise ------------------------
// Two writes to the SAME file MUST never parallelise: peak concurrency stays 1.
func TestExecuteBatch_ConflictingCallsSerialise(t *testing.T) {
	r := newBatchTestRegistry(t)
	var live, peak, calls int32
	// fs_write is a real tool name in derivePaths' mutated-path table. Use a
	// non-read-only probe under that name so conflictKeys keys on params["path"].
	r.Register(&probeTool{
		name: "fs_write", live: &live, peak: &peak, calls: &calls,
		delay: 30 * time.Millisecond, readOnly: false,
	})
	reqs := []ToolCallRequest{
		{ID: "w0", Name: "fs_write", Params: map[string]interface{}{"path": "same.txt", "content": "a"}},
		{ID: "w1", Name: "fs_write", Params: map[string]interface{}{"path": "same.txt", "content": "b"}},
	}
	results := r.ExecuteBatch(context.Background(), reqs, 0)

	require.Len(t, results, 2)
	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&peak),
		"two writes to the same file must run strictly serially — peak concurrency must be 1")
	for _, res := range results {
		require.False(t, res.RanParallel, "conflicting calls must run in the serial wave")
	}
}

// ---- TestExecuteBatch_SideEffectToolsSerialise -------------------------
// Run-level tools (not read-only) must serialise even with distinct targets.
func TestExecuteBatch_SideEffectToolsSerialise(t *testing.T) {
	r := newBatchTestRegistry(t)
	var live, peak, calls int32
	for i := 0; i < 3; i++ {
		r.Register(&probeTool{
			name: fmt.Sprintf("run_%d", i), live: &live, peak: &peak, calls: &calls,
			delay: 20 * time.Millisecond, readOnly: false, // LevelRun
		})
	}
	reqs := []ToolCallRequest{
		{ID: "r0", Name: "run_0", Params: map[string]interface{}{}},
		{ID: "r1", Name: "run_1", Params: map[string]interface{}{}},
		{ID: "r2", Name: "run_2", Params: map[string]interface{}{}},
	}
	results := r.ExecuteBatch(context.Background(), reqs, 0)
	require.Equal(t, int32(3), atomic.LoadInt32(&calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&peak),
		"side-effecting (LevelRun) tools must never run concurrently")
	for _, res := range results {
		require.False(t, res.RanParallel)
	}
}

// ---- TestExecuteBatch_ResultOrderMatchesRequestOrder -------------------
// Anti-bluff proof: results assembled in REQUEST order regardless of completion
// order. The first probe sleeps longest so it finishes LAST — yet result[0]
// must still be its result.
func TestExecuteBatch_ResultOrderMatchesRequestOrder(t *testing.T) {
	r := newBatchTestRegistry(t)
	var live, peak int32
	// Descending delays: c0 finishes last, c2 finishes first.
	delays := []time.Duration{60 * time.Millisecond, 30 * time.Millisecond, 5 * time.Millisecond}
	for i, d := range delays {
		r.Register(&probeTool{
			name: fmt.Sprintf("ord_%d", i), live: &live, peak: &peak,
			delay: d, readOnly: true,
		})
	}
	reqs := []ToolCallRequest{
		{ID: "c0", Name: "ord_0", Params: map[string]interface{}{}},
		{ID: "c1", Name: "ord_1", Params: map[string]interface{}{}},
		{ID: "c2", Name: "ord_2", Params: map[string]interface{}{}},
	}
	results := r.ExecuteBatch(context.Background(), reqs, 0)

	require.Len(t, results, 3)
	require.Equal(t, "c0", results[0].ID)
	require.Equal(t, "ok:ord_0", results[0].Result)
	require.Equal(t, "c1", results[1].ID)
	require.Equal(t, "ok:ord_1", results[1].Result)
	require.Equal(t, "c2", results[2].ID)
	require.Equal(t, "ok:ord_2", results[2].Result)
}

// ---- TestExecuteBatch_SerialWaveAppliesInRequestOrder ------------------
// Conflicting edit tools must apply in exactly the LLM-requested order.
func TestExecuteBatch_SerialWaveAppliesInRequestOrder(t *testing.T) {
	r := newBatchTestRegistry(t)
	var order []string
	var mu sync.Mutex
	// Same fs_write target -> all three calls drop to the serial wave; the
	// single fs_write tool records the params["tag"] of each call in order.
	r.Register(&orderRecorder{order: &order, mu: &mu})
	reqs := []ToolCallRequest{
		{ID: "s0", Name: "fs_write", Params: map[string]interface{}{"path": "x.txt", "tag": "first"}},
		{ID: "s1", Name: "fs_write", Params: map[string]interface{}{"path": "x.txt", "tag": "second"}},
		{ID: "s2", Name: "fs_write", Params: map[string]interface{}{"path": "x.txt", "tag": "third"}},
	}
	r.ExecuteBatch(context.Background(), reqs, 0)
	require.Equal(t, []string{"first", "second", "third"}, order,
		"serial wave must apply conflicting same-target calls in request order")
}

// orderRecorder records the params["tag"] of each call under a mutex.
type orderRecorder struct {
	approval.DefaultLevelEdit
	order *[]string
	mu    *sync.Mutex
}

func (o *orderRecorder) Name() string                          { return "fs_write" }
func (o *orderRecorder) Description() string                   { return "rec" }
func (o *orderRecorder) Schema() ToolSchema                    { return ToolSchema{Type: "object"} }
func (o *orderRecorder) Category() ToolCategory                { return CategoryFileSystem }
func (o *orderRecorder) Validate(map[string]interface{}) error { return nil }
func (o *orderRecorder) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	tag, _ := params["tag"].(string)
	o.mu.Lock()
	*o.order = append(*o.order, tag)
	o.mu.Unlock()
	return tag, nil
}

// ---- TestExecuteBatch_OutputEqualsSerial -------------------------------
// Anti-bluff proof: a mixed batch (read-only + edit) produces the EXACT same
// per-call results as running each call one at a time through Execute.
func TestExecuteBatch_OutputEqualsSerial(t *testing.T) {
	build := func() *ToolRegistry {
		r := newBatchTestRegistry(t)
		var live, peak int32
		r.Register(&probeTool{name: "probe_ro_0", live: &live, peak: &peak, delay: 5 * time.Millisecond, readOnly: true})
		r.Register(&probeTool{name: "probe_ro_1", live: &live, peak: &peak, delay: 5 * time.Millisecond, readOnly: true})
		var order []string
		var mu sync.Mutex
		r.Register(&orderRecorder{order: &order, mu: &mu})
		return r
	}
	reqs := []ToolCallRequest{
		{ID: "a", Name: "probe_ro_0", Params: map[string]interface{}{}},
		{ID: "b", Name: "fs_write", Params: map[string]interface{}{"path": "p.txt", "tag": "T"}},
		{ID: "c", Name: "probe_ro_1", Params: map[string]interface{}{}},
	}

	// Serial reference run.
	rSerial := build()
	serialResults := make([]interface{}, len(reqs))
	for i, req := range reqs {
		res, err := rSerial.Execute(context.Background(), req.Name, req.Params)
		require.NoError(t, err)
		serialResults[i] = res
	}

	// Batch run.
	rBatch := build()
	batch := rBatch.ExecuteBatch(context.Background(), reqs, 0)
	require.Len(t, batch, len(reqs))
	for i := range reqs {
		require.NoError(t, batch[i].Err)
		require.Equal(t, serialResults[i], batch[i].Result,
			"batch result[%d] must equal the serial-execution result", i)
	}
}

// ---- TestExecuteBatch_EmptyAndSingle -----------------------------------
func TestExecuteBatch_EmptyAndSingle(t *testing.T) {
	r := newBatchTestRegistry(t)
	var live, peak int32
	r.Register(&probeTool{name: "probe_ro_0", live: &live, peak: &peak, delay: time.Millisecond, readOnly: true})

	require.Empty(t, r.ExecuteBatch(context.Background(), nil, 0))

	single := r.ExecuteBatch(context.Background(),
		[]ToolCallRequest{{ID: "x", Name: "probe_ro_0", Params: map[string]interface{}{}}}, 0)
	require.Len(t, single, 1)
	require.NoError(t, single[0].Err)
	require.Equal(t, "ok:probe_ro_0", single[0].Result)
}

// ---- TestExecuteBatch_UnknownToolSurfacesError -------------------------
func TestExecuteBatch_UnknownToolSurfacesError(t *testing.T) {
	r := newBatchTestRegistry(t)
	results := r.ExecuteBatch(context.Background(),
		[]ToolCallRequest{{ID: "u", Name: "does_not_exist", Params: map[string]interface{}{}}}, 0)
	require.Len(t, results, 1)
	require.Error(t, results[0].Err, "unknown tool must surface an error in its result slot")
}

// ---- TestExecuteBatch_ConcurrencyBound ---------------------------------
// The worker pool must never exceed the requested maxConcurrency.
func TestExecuteBatch_ConcurrencyBound(t *testing.T) {
	r := newBatchTestRegistry(t)
	var live, peak, calls int32
	for i := 0; i < 8; i++ {
		r.Register(&probeTool{
			name: fmt.Sprintf("bound_%d", i), live: &live, peak: &peak, calls: &calls,
			delay: 15 * time.Millisecond, readOnly: true,
		})
	}
	reqs := make([]ToolCallRequest, 8)
	for i := 0; i < 8; i++ {
		reqs[i] = ToolCallRequest{ID: fmt.Sprintf("b%d", i), Name: fmt.Sprintf("bound_%d", i), Params: map[string]interface{}{}}
	}
	r.ExecuteBatch(context.Background(), reqs, 3)
	require.Equal(t, int32(8), atomic.LoadInt32(&calls))
	require.LessOrEqual(t, atomic.LoadInt32(&peak), int32(3),
		"worker pool must not exceed the requested maxConcurrency")
	require.Greater(t, atomic.LoadInt32(&peak), int32(1), "but it must still parallelise")
}

// ---- isParallelSafe / ParallelClassifier ------------------------------

type classifierTool struct {
	approval.DefaultLevelEdit // LevelEdit would normally => not parallel-safe
	safe bool
}

func (c *classifierTool) Name() string                          { return "classifier" }
func (c *classifierTool) Description() string                   { return "c" }
func (c *classifierTool) Schema() ToolSchema                    { return ToolSchema{Type: "object"} }
func (c *classifierTool) Category() ToolCategory                { return CategoryFileSystem }
func (c *classifierTool) Validate(map[string]interface{}) error { return nil }
func (c *classifierTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return "ok", nil
}
func (c *classifierTool) ParallelSafe() bool { return c.safe }

func TestIsParallelSafe_ExplicitClassifierWins(t *testing.T) {
	// LevelEdit tool that explicitly declares itself parallel-safe.
	require.True(t, isParallelSafe(&classifierTool{safe: true}),
		"explicit ParallelClassifier(true) must override the LevelEdit default")
	require.False(t, isParallelSafe(&classifierTool{safe: false}),
		"explicit ParallelClassifier(false) must keep an edit tool serial")
}

func TestIsParallelSafe_LevelFallback(t *testing.T) {
	var live, peak int32
	require.True(t, isParallelSafe(&probeTool{readOnly: true, live: &live, peak: &peak}),
		"a read-only tool with no ParallelClassifier must be inferred parallel-safe")
	require.False(t, isParallelSafe(&probeTool{readOnly: false, live: &live, peak: &peak}),
		"a run-level tool with no ParallelClassifier must be inferred must-serialise")
}
