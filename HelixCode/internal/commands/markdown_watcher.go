package commands

import (
	"context"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// MarkdownWatcher uses fsnotify to detect changes in command directories and
// triggers loader.Reload() with a debounce so rapid filesystem activity
// (e.g., editor saves) collapses to a single reload.
type MarkdownWatcher struct {
	loader   *MarkdownLoader
	dirs     []string
	debounce time.Duration
	w        *fsnotify.Watcher
	log      *zap.Logger

	mu      sync.Mutex
	pending *time.Timer
}

// NewMarkdownWatcher creates a new watcher that monitors dirs for filesystem
// changes and calls loader.Reload() (debounced) on any event. Dirs that do
// not exist are silently skipped; the loader may be nil (useful for tests
// that only need to verify lifecycle behaviour).
func NewMarkdownWatcher(loader *MarkdownLoader, dirs []string) (*MarkdownWatcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	mw := &MarkdownWatcher{
		loader:   loader,
		dirs:     dirs,
		debounce: 200 * time.Millisecond,
		w:        fw,
		log:      zap.NewNop(),
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		// Non-fatal: directory may not exist yet.
		_ = fw.Add(d)
	}
	return mw, nil
}

// SetDebounce overrides the default 200 ms debounce window.  Useful in tests
// to keep wall-clock time short.
func (mw *MarkdownWatcher) SetDebounce(d time.Duration) { mw.debounce = d }

// SetLogger installs a non-noop logger.
func (mw *MarkdownWatcher) SetLogger(log *zap.Logger) { mw.log = log }

// Close shuts down the underlying fsnotify watcher.  Safe to call even after
// Run has returned.
func (mw *MarkdownWatcher) Close() error { return mw.w.Close() }

// Run blocks until ctx is cancelled, processing fsnotify events and scheduling
// debounced reloads.  The underlying watcher is closed when Run returns.
func (mw *MarkdownWatcher) Run(ctx context.Context) {
	defer mw.w.Close()
	for {
		select {
		case <-ctx.Done():
			mw.cancelPending()
			return
		case ev, ok := <-mw.w.Events:
			if !ok {
				return
			}
			if ev.Op == 0 {
				continue
			}
			mw.scheduleReload()
		case err, ok := <-mw.w.Errors:
			if !ok {
				return
			}
			mw.log.Warn("markdown watcher: fsnotify error", zap.Error(err))
		}
	}
}

// scheduleReload resets the debounce timer so that only one Reload call is
// issued after a burst of filesystem events.
func (mw *MarkdownWatcher) scheduleReload() {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	if mw.pending != nil {
		mw.pending.Stop()
	}
	mw.pending = time.AfterFunc(mw.debounce, func() {
		if mw.loader == nil {
			return
		}
		if err := mw.loader.Reload(); err != nil {
			mw.log.Warn("markdown watcher: reload failed", zap.Error(err))
		}
	})
}

// cancelPending stops a pending debounce timer, if any.
func (mw *MarkdownWatcher) cancelPending() {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	if mw.pending != nil {
		mw.pending.Stop()
		mw.pending = nil
	}
}
