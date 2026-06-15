// Package substrate provides the shared parallel-dispatch substrate for
// HelixCode's multi-agent / parallel-work coordination (SP5).
//
// Per docs/research/2026-06-10-parallel-agent-coordination.md, the substrate
// is the convergent four-role abstraction every surveyed agentic stack
// (Temporal, Ray, LangGraph, OpenAI Agents SDK, MetaGPT) settles on:
//
//	Unit       — a self-describing dispatchable piece of work (id, payload,
//	             declared capability requirement, priority, resource class).
//	Scheduler  — decides WHICH units run now + in what order (priority).
//	Dispatcher — maps a ready unit onto an executor with the concurrency
//	             worker pool (bounded concurrency, retry/timeout mechanics).
//	Resolver   — picks the right executor for a unit (capability match) and
//	             reconciles results from parallel units.
//
// Per §11.4.74 (catalogue-first, extend-don't-reimplement) the mechanical
// substrate is REUSED from the own-org submodule digital.vasic.concurrency
// (pool.WorkerPool + queue.PriorityQueue + semaphore.Semaphore) rather than
// reimplemented here. This package is the thin coordination layer that wires
// HelixCode's Unit/Resolver concepts onto that proven concurrency floor —
// closing the "own-org concurrency unused" gap (D-7) without duplicating a
// fourth coordinator.
package substrate

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/pool"
	"digital.vasic.concurrency/pkg/queue"
)

// ErrDispatcherShutdown is returned by Dispatch when the Dispatcher has already
// been shut down. Dispatching onto a shut-down pool must fail cleanly, never
// panic with "send on closed channel".
var ErrDispatcherShutdown = errors.New("substrate: dispatcher is shut down")

// Priority mirrors the concurrency queue's four-level priority so callers of
// the substrate do not need to import the concurrency package directly.
type Priority = queue.Priority

// Re-export the priority levels for ergonomic use by substrate consumers.
const (
	PriorityLow      = queue.Low
	PriorityNormal   = queue.Normal
	PriorityHigh     = queue.High
	PriorityCritical = queue.Critical
)

// Capability is a declared requirement of a Unit OR a declared competency of
// an executor. The Resolver matches a Unit's required capability against the
// executors that advertise it. The empty Capability ("") means "no specific
// capability required" and matches any executor.
type Capability string

// Unit is the substrate's self-describing dispatchable piece of work. It
// extends the concurrency pool.Task contract (ID + Execute) with the agentic
// metadata the Scheduler and Resolver need: a required Capability and a
// dispatch Priority.
//
// Unit is intentionally an interface (not a struct) so HelixCode's existing
// task/agent types can satisfy it directly and be dispatched through the
// concurrency-backed substrate without a wrapper.
type Unit interface {
	// ID returns a unique identifier for the unit (pool.Task contract).
	ID() string
	// Execute runs the unit and returns its result (pool.Task contract).
	Execute(ctx context.Context) (interface{}, error)
	// Requires declares the capability an executor must advertise to run
	// this unit. The empty Capability matches any executor.
	Requires() Capability
	// Priority returns the dispatch priority used by the Scheduler.
	Priority() Priority
}

// Result is the outcome of executing a Unit. It wraps the concurrency pool
// Result so substrate consumers get a stable type independent of the pool.
type Result struct {
	UnitID string
	Value  interface{}
	Err    error
}

// UnitFunc is a convenience implementation of Unit wrapping a plain function,
// analogous to concurrency's pool.TaskFunc but carrying capability + priority.
type UnitFunc struct {
	id       string
	requires Capability
	priority Priority
	fn       func(ctx context.Context) (interface{}, error)
}

// NewUnitFunc builds a Unit from a function plus its dispatch metadata.
func NewUnitFunc(
	id string,
	requires Capability,
	priority Priority,
	fn func(ctx context.Context) (interface{}, error),
) *UnitFunc {
	return &UnitFunc{id: id, requires: requires, priority: priority, fn: fn}
}

func (u *UnitFunc) ID() string                                       { return u.id }
func (u *UnitFunc) Requires() Capability                             { return u.requires }
func (u *UnitFunc) Priority() Priority                               { return u.priority }
func (u *UnitFunc) Execute(ctx context.Context) (interface{}, error) { return u.fn(ctx) }

// unitTask adapts a substrate Unit to the concurrency pool.Task interface so
// the real worker pool can execute it. This is the seam that makes a Unit
// "actually run via the concurrency pool" per the SP5 brief.
type unitTask struct {
	unit Unit
}

func (t unitTask) ID() string { return t.unit.ID() }
func (t unitTask) Execute(ctx context.Context) (interface{}, error) {
	return t.unit.Execute(ctx)
}

// Scheduler decides which Units run now and in what order. It is backed by the
// concurrency PriorityQueue (heap, four levels, stable FIFO tie-break) so the
// §11.4.42 priority-first / §11.4.72 audio-first ordering is honoured by
// construction rather than reimplemented.
type Scheduler struct {
	pq *queue.PriorityQueue[Unit]
}

// NewScheduler creates a Scheduler with an optional initial queue capacity.
func NewScheduler(initialCap int) *Scheduler {
	return &Scheduler{pq: queue.New[Unit](initialCap)}
}

// Enqueue admits a Unit into the scheduler at its declared priority.
func (s *Scheduler) Enqueue(u Unit) {
	s.pq.Push(u, u.Priority())
}

// Next removes and returns the highest-priority ready Unit, or false if empty.
func (s *Scheduler) Next() (Unit, bool) {
	return s.pq.Pop()
}

// Len reports how many Units are waiting.
func (s *Scheduler) Len() int { return s.pq.Len() }

// Resolver picks the executor (capability match) for a Unit and reconciles
// results. In this minimal scaffold a Unit is self-executing (it carries its
// own Execute), so resolution is a capability gate: the Resolver verifies the
// dispatcher advertises the Unit's required Capability before dispatch.
//
// Capability merge / voting / scatter-gather is the genuinely-new layer the
// research flags; the gate below is the first concrete slice, extended as the
// agentic surface grows.
type Resolver struct {
	capabilities map[Capability]bool
}

// NewResolver builds a Resolver advertising the given executor capabilities.
func NewResolver(advertised ...Capability) *Resolver {
	caps := make(map[Capability]bool, len(advertised))
	for _, c := range advertised {
		caps[c] = true
	}
	return &Resolver{capabilities: caps}
}

// CanRun reports whether this resolver's executors can satisfy the unit's
// required capability. The empty Capability always resolves true.
func (r *Resolver) CanRun(u Unit) bool {
	req := u.Requires()
	if req == "" {
		return true
	}
	return r.capabilities[req]
}

// Dispatcher maps ready Units onto the concurrency WorkerPool with bounded
// concurrency. It owns the real pool lifecycle (Start/Shutdown) and turns each
// Unit into a pool.Task so it actually runs on a worker goroutine.
type Dispatcher struct {
	pool     *pool.WorkerPool
	resolver *Resolver

	// mu serializes Dispatch against Shutdown. Dispatch holds RLock across the
	// pool submit (the `p.tasks <- task` send inside SubmitWait); Shutdown holds
	// the exclusive write Lock across the pool teardown (the `close(p.tasks)`).
	// Because RLock excludes the write Lock, a send can never run concurrently
	// with the close — closing the "panic: send on closed channel" race that a
	// concurrent Dispatch/Shutdown otherwise hits (DATA RACE reproduced under
	// `go test -race`). Concurrent Dispatches still run in parallel (shared
	// RLock); only Shutdown is exclusive.
	//
	// KNOWN LIMITATION (not reachable by any current code, §11.4.6): a Unit whose
	// Execute() calls d.Shutdown() on THIS dispatcher would deadlock (the running
	// Dispatch holds RLock; Shutdown waits for the write Lock; the unit can't
	// finish until Shutdown returns). No Unit in the codebase does this — the
	// only Unit.Execute impls (UnitFunc, unitTask) run caller fns that dispatch
	// work, never tear down their own dispatcher. Documented rather than guarded
	// against with extra machinery that would add its own failure surface.
	mu           sync.RWMutex
	shutdown     bool
	shutdownOnce sync.Once
	shutdownErr  error
}

// NewDispatcher creates a Dispatcher backed by a real concurrency worker pool
// of the given size, gated by the given Resolver. It starts the pool.
func NewDispatcher(workers int, resolver *Resolver) *Dispatcher {
	cfg := pool.DefaultPoolConfig()
	if workers > 0 {
		cfg.Workers = workers
	}
	if resolver == nil {
		resolver = NewResolver()
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()
	return &Dispatcher{pool: wp, resolver: resolver}
}

// Dispatch runs a single Unit synchronously on the worker pool and returns its
// Result. It first verifies the Resolver can satisfy the Unit's capability;
// an unsatisfiable capability is a real error, never a silent drop.
func (d *Dispatcher) Dispatch(ctx context.Context, u Unit) Result {
	if !d.resolver.CanRun(u) {
		return Result{
			UnitID: u.ID(),
			Err: fmt.Errorf(
				"substrate: no executor advertises capability %q required by unit %q",
				u.Requires(), u.ID(),
			),
		}
	}
	// Hold RLock across the submit so Shutdown's pool teardown (write Lock)
	// cannot close the task channel underneath an in-flight send.
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.shutdown {
		return Result{UnitID: u.ID(), Err: ErrDispatcherShutdown}
	}
	res, err := d.pool.SubmitWait(ctx, unitTask{unit: u})
	if err != nil {
		return Result{UnitID: u.ID(), Err: err}
	}
	return Result{UnitID: res.TaskID, Value: res.Value, Err: res.Error}
}

// Drain pulls every Unit out of the Scheduler in priority order and dispatches
// it on the worker pool, returning the Results in completion order. This is
// the parallel-drain the in-app coordinator.go lacks (its taskQueue is never
// drained) — here it is mechanically backed by the concurrency pool.
func (d *Dispatcher) Drain(ctx context.Context, s *Scheduler) []Result {
	var units []Unit
	for {
		u, ok := s.Next()
		if !ok {
			break
		}
		units = append(units, u)
	}
	results := make([]Result, 0, len(units))
	for _, u := range units {
		results = append(results, d.Dispatch(ctx, u))
	}
	return results
}

// Shutdown gracefully stops the underlying worker pool, waiting up to timeout
// for in-flight units to finish. A non-positive timeout uses the pool's
// configured shutdown grace period.
// Shutdown is idempotent and safe to call concurrently with Dispatch. It takes
// the exclusive write Lock (which waits for every in-flight Dispatch to release
// its RLock), marks the dispatcher shut down so subsequent Dispatch calls fail
// cleanly with ErrDispatcherShutdown, and tears the pool down exactly once via
// shutdownOnce. Holding the write Lock across the teardown guarantees no
// concurrent Dispatch is mid-submit when the pool closes its task channel. A
// second (or concurrent) Shutdown returns the same teardown result without
// re-closing anything.
func (d *Dispatcher) Shutdown(timeout time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.shutdown = true
	d.shutdownOnce.Do(func() {
		d.shutdownErr = d.pool.Shutdown(timeout)
	})
	return d.shutdownErr
}
