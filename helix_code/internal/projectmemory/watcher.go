// Package projectmemory — watcher.go (P2-F24-T05).
//
// MemoryWatcher wraps github.com/fsnotify/fsnotify to fire MemoryRegistry.Reload
// when the underlying memory file changes mid-session. Editors (vim, emacs)
// commonly write atomically via rename; to survive that, we watch the PARENT
// directory and filter events for the specific paths.
//
// Successive events within DebounceWindow (200 ms) coalesce into ONE Reload
// — a single :w in vim emits 3-5 events.
//
// Graceful degrade: if fsnotify is unavailable on this platform / volume,
// Start logs WARN and returns nil. The registry continues to work via the
// /memory reload slash; users just don't get hot-reload.
package projectmemory

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// MemoryWatcher is the fsnotify-driven hot-reload trigger. Holds a reference
// to the registry it reloads + a fsnotify.Watcher + the debounce window.
//
// Lifecycle: Start (attach watcher, spawn goroutine) → many events → Close
// (closes watcher, drains goroutine).
type MemoryWatcher struct {
	registry *MemoryRegistry
	watcher  *fsnotify.Watcher
	log      *zap.Logger
	debounce time.Duration

	// Synchronisation: closeOnce guards Close() so calling it twice is a
	// no-op. done is closed when runEventLoop exits — Close blocks on it.
	// startMu serialises the started check+set in Start so concurrent Start
	// calls spawn AT MOST one event loop (no data race on `started`, no second
	// goroutine). doneOnce guards close(done) so a stray double-close — e.g. a
	// duplicate event loop on the pre-fix path, or Close racing the loop — can
	// never panic with "close of closed channel" (defense-in-depth).
	closeOnce sync.Once
	doneOnce  sync.Once
	startMu   sync.Mutex
	done      chan struct{}
	started   bool
}

// closeDone closes w.done exactly once. Safe to call from the event loop's
// defer, from Start's fsnotify-failure path, and from Close.
func (w *MemoryWatcher) closeDone() {
	w.doneOnce.Do(func() { close(w.done) })
}

// NewMemoryWatcher constructs a watcher; safe to construct without an
// active registry snapshot — Start() reads the registry's current snapshot
// to pick the watch targets.
func NewMemoryWatcher(r *MemoryRegistry, log *zap.Logger) *MemoryWatcher {
	if log == nil {
		log = zap.NewNop()
	}
	return &MemoryWatcher{
		registry: r,
		log:      log,
		debounce: DebounceWindow,
		done:     make(chan struct{}),
	}
}

// Start attaches the fsnotify watcher to the parent directories of the
// project + user memory paths recorded in the registry's CURRENT snapshot,
// then spawns the event-handling goroutine.
//
// Returns nil even if fsnotify initialisation OR fsnotify.Add fails — the
// rationale is graceful degrade: a CLI that crashes because inotify ran out
// of watches is worse than a CLI that quietly works without hot-reload.
// Failures are WARN-logged.
//
// Calling Start twice is a no-op (the second call returns nil without
// spawning a second goroutine).
func (w *MemoryWatcher) Start(ctx context.Context) error {
	// Serialise the started check+set: concurrent Start calls must spawn AT MOST
	// one event loop. Without this lock both callers could read started==false,
	// both set it true, and both spawn a runEventLoop sharing one w.done — the
	// data race + the cancel→double-close-of-w.done panic (DEFECT-1).
	w.startMu.Lock()
	defer w.startMu.Unlock()

	if w.started {
		return nil
	}
	w.started = true

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.log.Warn("projectmemory: fsnotify new watcher failed; degrading to slash-only reload", zap.Error(err))
		// Mark done as closed so Close() doesn't block.
		w.closeDone()
		return nil
	}
	w.watcher = fsw

	snap := w.registry.Snapshot()
	seen := make(map[string]struct{})
	for _, p := range []string{snap.ProjectPath, snap.UserPath} {
		if p == "" {
			continue
		}
		parent := filepath.Dir(p)
		if _, dup := seen[parent]; dup {
			continue
		}
		seen[parent] = struct{}{}
		if addErr := fsw.Add(parent); addErr != nil {
			w.log.Warn("projectmemory: fsnotify add failed",
				zap.String("dir", parent),
				zap.Error(addErr))
		}
	}

	go w.runEventLoop(ctx, snap)
	return nil
}

// runEventLoop is the fsnotify event consumer. It filters events for the
// exact target paths (snap.ProjectPath, snap.UserPath), debounces them via
// a 200 ms timer, and triggers registry.Reload on the trailing edge.
//
// Targets are computed at Start time (NOT re-derived per event) so that a
// rename event for the project path doesn't trick us into ignoring the
// follow-up create/write of the same path.
func (w *MemoryWatcher) runEventLoop(ctx context.Context, snap Memory) {
	defer w.closeDone()

	targets := map[string]struct{}{}
	if snap.ProjectPath != "" {
		targets[snap.ProjectPath] = struct{}{}
	}
	if snap.UserPath != "" {
		targets[snap.UserPath] = struct{}{}
	}

	var (
		timer *time.Timer
		mu    sync.Mutex
	)

	fire := func() {
		if _, err := w.registry.Reload(ctx); err != nil {
			w.log.Warn("projectmemory: reload after fsnotify failed", zap.Error(err))
		}
	}

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			// Only consider events for our exact paths. Editors that write
			// via rename produce events for both the old name and the new
			// name; both are filtered through this set.
			if _, hit := targets[ev.Name]; !hit {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			mu.Lock()
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, fire)
			mu.Unlock()

		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// Close releases the underlying fsnotify watcher and waits for the event
// loop to exit. Idempotent: a second Close is a no-op.
//
// If Start was never called, Close is a no-op and returns nil.
func (w *MemoryWatcher) Close() error {
	var firstErr error
	w.closeOnce.Do(func() {
		if w.watcher == nil {
			// Either Start was never called or Start hit fsnotify.NewWatcher
			// failure. closeDone is idempotent: if Start already closed done
			// (fsnotify-failure path) this is a no-op; if Start was never called
			// it closes done so Close returns without blocking.
			w.closeDone()
			return
		}
		firstErr = w.watcher.Close()
		<-w.done
	})
	return firstErr
}
