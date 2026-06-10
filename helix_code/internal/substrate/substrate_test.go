package substrate

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSubstrate_DispatchUnitThroughConcurrencyPool is the load-bearing RED→GREEN
// test for SP5 / D-7: it proves helix_code can import the own-org
// digital.vasic.concurrency submodule (via the substrate) AND actually run a
// Unit on its real worker pool.
//
// RED state (before the go.mod replace + require for digital.vasic.concurrency):
// this test file does not compile because the substrate package cannot import
// digital.vasic.concurrency/pkg/{pool,queue} — `go test ./internal/substrate/...`
// FAILs at the build step.
//
// GREEN state (after the replace + scaffold): the Unit's Execute runs on a real
// concurrency worker goroutine and the returned value is observed here.
func TestSubstrate_DispatchUnitThroughConcurrencyPool(t *testing.T) {
	resolver := NewResolver() // no capability requirement
	d := NewDispatcher(2, resolver)
	defer func() { _ = d.Shutdown(2 * time.Second) }()

	var ran int32
	u := NewUnitFunc(
		"unit-1", "", PriorityNormal,
		func(ctx context.Context) (interface{}, error) {
			atomic.AddInt32(&ran, 1)
			return "executed-by-concurrency-pool", nil
		},
	)

	res := d.Dispatch(context.Background(), u)

	require.NoError(t, res.Err, "unit must run on the concurrency worker pool")
	require.Equal(t, "unit-1", res.UnitID)
	require.Equal(t, "executed-by-concurrency-pool", res.Value,
		"the Unit's real Execute output must be returned through the pool")
	require.Equal(t, int32(1), atomic.LoadInt32(&ran),
		"Execute must have actually run exactly once on a worker goroutine")
}

// TestSubstrate_SchedulerPriorityOrdering proves the Scheduler honours the
// concurrency PriorityQueue ordering (Critical > High > Normal > Low, FIFO
// tie-break) — the §11.4.42 priority-first / §11.4.72 audio-first ordering
// reused from concurrency, not reimplemented.
func TestSubstrate_SchedulerPriorityOrdering(t *testing.T) {
	s := NewScheduler(0)

	mk := func(id string, p Priority) Unit {
		return NewUnitFunc(id, "", p,
			func(ctx context.Context) (interface{}, error) { return id, nil })
	}

	// Enqueue out of priority order.
	s.Enqueue(mk("low", PriorityLow))
	s.Enqueue(mk("critical", PriorityCritical))
	s.Enqueue(mk("normal", PriorityNormal))
	s.Enqueue(mk("high", PriorityHigh))

	var order []string
	for {
		u, ok := s.Next()
		if !ok {
			break
		}
		order = append(order, u.ID())
	}

	require.Equal(t,
		[]string{"critical", "high", "normal", "low"}, order,
		"scheduler must drain highest-priority first")
}

// TestSubstrate_DrainRunsAllUnitsOnPool proves the Dispatcher.Drain actually
// pulls every Unit out of the Scheduler in priority order and runs each on the
// real worker pool — the parallel-drain the in-app coordinator.go lacks.
func TestSubstrate_DrainRunsAllUnitsOnPool(t *testing.T) {
	d := NewDispatcher(4, NewResolver())
	defer func() { _ = d.Shutdown(2 * time.Second) }()

	s := NewScheduler(0)
	var executed int32
	for i, p := range []Priority{PriorityLow, PriorityCritical, PriorityNormal} {
		id := []string{"a", "b", "c"}[i]
		s.Enqueue(NewUnitFunc(id, "", p,
			func(ctx context.Context) (interface{}, error) {
				atomic.AddInt32(&executed, 1)
				return id, nil
			}))
	}

	results := d.Drain(context.Background(), s)

	require.Len(t, results, 3)
	require.Equal(t, int32(3), atomic.LoadInt32(&executed),
		"every drained unit must actually run on the pool")
	// Priority order: critical(b) first.
	require.Equal(t, "b", results[0].UnitID)
	for _, r := range results {
		require.NoError(t, r.Err)
	}
}

// TestSubstrate_CapabilityGateBlocksUnsatisfiableUnit is the paired §1.1
// anti-bluff assertion for the Resolver: a Unit requiring a capability no
// executor advertises MUST be refused with a real error — never silently
// dropped, never falsely run. Mutating CanRun to always-true (the bluff) would
// make this test FAIL because the unit's Execute would run and Err would be nil.
func TestSubstrate_CapabilityGateBlocksUnsatisfiableUnit(t *testing.T) {
	// Resolver advertises only "code-gen"; the unit requires "vision".
	d := NewDispatcher(2, NewResolver("code-gen"))
	defer func() { _ = d.Shutdown(2 * time.Second) }()

	var ranAnyway int32
	u := NewUnitFunc("vision-unit", Capability("vision"), PriorityHigh,
		func(ctx context.Context) (interface{}, error) {
			atomic.AddInt32(&ranAnyway, 1)
			return "should-not-run", nil
		})

	res := d.Dispatch(context.Background(), u)

	require.Error(t, res.Err,
		"unsatisfiable capability must produce a real error, not a silent pass")
	require.Equal(t, int32(0), atomic.LoadInt32(&ranAnyway),
		"the unit must NOT execute when no executor advertises its capability")

	// And the positive case: a satisfiable capability runs.
	ok := NewUnitFunc("codegen-unit", Capability("code-gen"), PriorityHigh,
		func(ctx context.Context) (interface{}, error) { return "ok", nil })
	res2 := d.Dispatch(context.Background(), ok)
	require.NoError(t, res2.Err)
	require.Equal(t, "ok", res2.Value)
}

// TestSubstrate_UnitErrorPropagates proves a Unit's real error surfaces through
// the pool unchanged (no swallowing).
func TestSubstrate_UnitErrorPropagates(t *testing.T) {
	d := NewDispatcher(1, NewResolver())
	defer func() { _ = d.Shutdown(2 * time.Second) }()

	sentinel := errors.New("boom")
	u := NewUnitFunc("err-unit", "", PriorityNormal,
		func(ctx context.Context) (interface{}, error) { return nil, sentinel })

	res := d.Dispatch(context.Background(), u)
	require.Error(t, res.Err)
	require.Contains(t, res.Err.Error(), "boom")
}
