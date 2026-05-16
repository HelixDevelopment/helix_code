// Package projectmemory — watcher_test.go (P2-F24-T05).
//
// All watcher tests use REAL fsnotify against REAL tempdirs. No mock
// fsnotify — the integration is the contract.
package projectmemory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// waitForCondition polls until cond() is true or the deadline expires.
// Used to assert "fsnotify event eventually triggered Reload".
func waitForCondition(t *testing.T, deadline time.Duration, cond func() bool) bool {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

func TestWatcher_FileWriteTriggersReload_Real(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "helixcode.md")
	require.NoError(t, os.WriteFile(file, []byte("V1"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
	_, err := r.Reload(context.Background())
	require.NoError(t, err)
	require.Equal(t, "V1", r.Snapshot().Project)

	w := NewMemoryWatcher(r, zap.NewNop())
	require.NoError(t, w.Start(context.Background()))
	defer w.Close()

	// Rewrite the file. fsnotify Write event must fire → debounce timer →
	// registry.Reload → Snapshot returns new content.
	require.NoError(t, os.WriteFile(file, []byte("V2"), 0644))

	updated := waitForCondition(t, 1500*time.Millisecond, func() bool {
		return r.Snapshot().Project == "V2"
	})
	require.True(t, updated, "registry never picked up new content; current=%q", r.Snapshot().Project)
}

func TestWatcher_RapidWrites_DebounceCoalesces(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "helixcode.md")
	require.NoError(t, os.WriteFile(file, []byte("V0"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
	_, _ = r.Reload(context.Background())

	w := NewMemoryWatcher(r, zap.NewNop())
	require.NoError(t, w.Start(context.Background()))
	defer w.Close()

	// Spam writes; debounce should coalesce — final state is the LAST written.
	for i := 1; i <= 10; i++ {
		require.NoError(t, os.WriteFile(file, []byte("VFINAL"), 0644))
		time.Sleep(10 * time.Millisecond)
	}

	updated := waitForCondition(t, 1500*time.Millisecond, func() bool {
		return r.Snapshot().Project == "VFINAL"
	})
	require.True(t, updated, "registry never picked up VFINAL; current=%q", r.Snapshot().Project)
}

func TestWatcher_Close_Idempotent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte("X"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
	_, _ = r.Reload(context.Background())
	w := NewMemoryWatcher(r, zap.NewNop())
	require.NoError(t, w.Start(context.Background()))
	require.NoError(t, w.Close())
	// Second close: must not panic, must not block.
	require.NoError(t, w.Close())
}

func TestWatcher_Close_BeforeStart_NoOp(t *testing.T) {
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), t.TempDir())
	w := NewMemoryWatcher(r, zap.NewNop())
	require.NoError(t, w.Close())
}

func TestWatcher_NoMemoryFile_GracefulStart(t *testing.T) {
	// Empty tempdir — no files to watch. Start should succeed (nil error)
	// and Close should be safe.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
	_, _ = r.Reload(context.Background())
	w := NewMemoryWatcher(r, zap.NewNop())
	require.NoError(t, w.Start(context.Background()))
	require.NoError(t, w.Close())
}

func TestWatcher_StartTwice_NoOp(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte("X"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
	_, _ = r.Reload(context.Background())
	w := NewMemoryWatcher(r, zap.NewNop())
	require.NoError(t, w.Start(context.Background()))
	defer w.Close()
	// Second Start must be a no-op (no error, no second goroutine).
	require.NoError(t, w.Start(context.Background()))
}

func TestWatcher_NewWithNilLog_Safe(t *testing.T) {
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), t.TempDir())
	w := NewMemoryWatcher(r, nil)
	require.NotNil(t, w)
}

func TestWatcher_UserOverlayChange_TriggersReload_Real(t *testing.T) {
	proj := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(proj, "helixcode.md"), []byte("P"), 0644))
	xdg := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(xdg, "helixcode"), 0755))
	userFile := filepath.Join(xdg, "helixcode", "memory.md")
	require.NoError(t, os.WriteFile(userFile, []byte("U_INITIAL"), 0644))
	t.Setenv("XDG_CONFIG_HOME", xdg)

	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), proj)
	_, err := r.Reload(context.Background())
	require.NoError(t, err)
	require.Equal(t, "U_INITIAL", r.Snapshot().User)

	w := NewMemoryWatcher(r, zap.NewNop())
	require.NoError(t, w.Start(context.Background()))
	defer w.Close()

	require.NoError(t, os.WriteFile(userFile, []byte("U_NEW"), 0644))

	updated := waitForCondition(t, 1500*time.Millisecond, func() bool {
		return r.Snapshot().User == "U_NEW"
	})
	require.True(t, updated, "user overlay reload never fired; user=%q", r.Snapshot().User)
}
