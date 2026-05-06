// Package projectmemory — registry.go (P2-F24-T04).
//
// MemoryRegistry wraps an atomic.Pointer[Memory] so that BaseAgent's per-LLM-
// call Snapshot is lock-free, while Reload (rare; fired by fsnotify or the
// /memory reload slash) is mu-serialised so concurrent reloads collapse to
// one Loader.Discover call.
//
// MemorySnapshotter is the read-only interface BaseAgent depends on; it
// hides Set/Reload from consumers that should only observe the current
// memory.
package projectmemory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// MemorySnapshotter is the read-only contract for "give me the current
// memory blob". Defined as an interface so test fakes (and BaseAgent's
// optional dependency injection) don't need to construct a full
// MemoryRegistry. *MemoryRegistry satisfies this interface.
type MemorySnapshotter interface {
	Snapshot() Memory
}

// MemoryRegistry stores the currently-loaded Memory and exposes lock-free
// reads via Snapshot. Mutations (Set, Reload) are mu-serialised.
//
// The atomic-pointer pattern lets concurrent agent calls (F15 subagents)
// share one registry without contention on the read path. The mutex
// serialises only lifecycle transitions (Reload), which are rare —
// typically <1 per second under normal use.
type MemoryRegistry struct {
	current atomic.Pointer[Memory]
	loader  *MemoryLoader
	cwd     string
	mu      sync.Mutex
}

// NewMemoryRegistry constructs a registry with no current memory (Snapshot
// returns zero-value Memory{} until Reload or Set is called).
//
// cwd is captured here and re-used by every Reload — this prevents the
// "agent in a subagent worktree picks up parent worktree's memory" footgun.
// Callers who want different cwds construct different registries.
func NewMemoryRegistry(loader *MemoryLoader, cwd string) *MemoryRegistry {
	return &MemoryRegistry{
		loader: loader,
		cwd:    cwd,
	}
}

// Snapshot returns the currently-stored Memory via a lock-free atomic load.
// Returns the zero-value Memory{} when no memory has been loaded yet (e.g.
// before the first Reload). Callers MUST treat the result as immutable.
func (r *MemoryRegistry) Snapshot() Memory {
	if p := r.current.Load(); p != nil {
		return *p
	}
	return Memory{}
}

// Set atomically stores a new Memory. Useful for tests and for the watcher's
// debounce trigger. Last-write-wins under concurrent Set calls.
func (r *MemoryRegistry) Set(m Memory) {
	r.current.Store(&m)
}

// Reload runs Loader.Discover(cwd) and atomically stores the result. On
// Discover error, the previous value is PRESERVED — a transient I/O error
// should not drop the agent's known-good blob.
//
// The mutex serialises concurrent Reload calls so two simultaneous fsnotify
// debounce triggers + a /memory reload slash collapse into ONE Discover.
// Snapshot remains lock-free during Reload because it reads the atomic
// pointer (which is updated only at the end, after Discover succeeded).
func (r *MemoryRegistry) Reload(ctx context.Context) (Memory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loader == nil {
		return Memory{}, fmt.Errorf("projectmemory: registry has no loader")
	}
	if err := ctx.Err(); err != nil {
		return Memory{}, err
	}
	m, err := r.loader.Discover(r.cwd)
	if err != nil {
		// Preserve previous value on error. Caller will surface err.
		return Memory{}, err
	}
	r.current.Store(&m)
	return m, nil
}
