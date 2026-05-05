package commands

import (
	"context"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// SkillsWatcher uses fsnotify to detect changes in skill directories and
// triggers SkillLoader.Reload() with a debounce so rapid filesystem activity
// (e.g., editor saves) collapses to a single reload. Mirrors MarkdownWatcher.
type SkillsWatcher struct {
	loader   *SkillLoader
	dirs     []string
	debounce time.Duration
	w        *fsnotify.Watcher
	log      *zap.Logger

	mu      sync.Mutex
	pending *time.Timer
}

// NewSkillsWatcher creates a new watcher that monitors dirs for filesystem
// changes and calls loader.Reload() (debounced) on any event. Dirs that do
// not exist are silently skipped; the loader may be nil (useful for tests
// that only need to verify lifecycle behaviour).
func NewSkillsWatcher(loader *SkillLoader, dirs []string) (*SkillsWatcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	sw := &SkillsWatcher{
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
	return sw, nil
}

// SetDebounce overrides the default 200 ms debounce window. Useful in tests
// to keep wall-clock time short.
func (sw *SkillsWatcher) SetDebounce(d time.Duration) { sw.debounce = d }

// SetLogger installs a non-noop logger.
func (sw *SkillsWatcher) SetLogger(log *zap.Logger) { sw.log = log }

// Close shuts down the underlying fsnotify watcher. Safe to call even after
// Run has returned.
func (sw *SkillsWatcher) Close() error { return sw.w.Close() }

// Run blocks until ctx is cancelled, processing fsnotify events and scheduling
// debounced reloads. The underlying watcher is closed when Run returns.
func (sw *SkillsWatcher) Run(ctx context.Context) {
	defer sw.w.Close()
	for {
		select {
		case <-ctx.Done():
			sw.cancelPending()
			return
		case ev, ok := <-sw.w.Events:
			if !ok {
				return
			}
			if ev.Op == 0 {
				continue
			}
			sw.scheduleReload()
		case err, ok := <-sw.w.Errors:
			if !ok {
				return
			}
			sw.log.Warn("skills watcher: fsnotify error", zap.Error(err))
		}
	}
}

// scheduleReload resets the debounce timer so that only one Reload call is
// issued after a burst of filesystem events.
func (sw *SkillsWatcher) scheduleReload() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.pending != nil {
		sw.pending.Stop()
	}
	sw.pending = time.AfterFunc(sw.debounce, func() {
		if sw.loader == nil {
			return
		}
		if err := sw.loader.Reload(); err != nil {
			sw.log.Warn("skills watcher: reload failed", zap.Error(err))
		}
	})
}

// cancelPending stops a pending debounce timer, if any.
func (sw *SkillsWatcher) cancelPending() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.pending != nil {
		sw.pending.Stop()
		sw.pending = nil
	}
}
