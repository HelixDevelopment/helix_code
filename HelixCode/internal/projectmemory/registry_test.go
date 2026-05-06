// Package projectmemory — registry_test.go (P2-F24-T04).
//
// Tests run with -race in CI / close-out (T08). Concurrent read+write must
// not produce data races; Reload-on-error must preserve previous value.
package projectmemory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRegistry_Snapshot_NilCurrent_ZeroValue(t *testing.T) {
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), t.TempDir())
	m := r.Snapshot()
	require.Empty(t, m.Project)
	require.Empty(t, m.ProjectPath)
	require.True(t, m.LoadedAt.IsZero())
}

func TestRegistry_Set_Snapshot_Roundtrip(t *testing.T) {
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), t.TempDir())
	r.Set(Memory{Project: "ROUNDTRIP_24"})
	require.Equal(t, "ROUNDTRIP_24", r.Snapshot().Project)
}

func TestRegistry_Snapshot_ReturnsValueCopy_NotPointer(t *testing.T) {
	// Mutating the returned Memory must NOT affect subsequent Snapshots.
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), t.TempDir())
	r.Set(Memory{Project: "ORIGINAL"})
	m := r.Snapshot()
	m.Project = "MUTATED"
	require.Equal(t, "ORIGINAL", r.Snapshot().Project)
}

func TestRegistry_Reload_RealTempdir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte("RELOAD_24"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
	m, err := r.Reload(context.Background())
	require.NoError(t, err)
	require.Contains(t, m.Project, "RELOAD_24")
	require.Contains(t, r.Snapshot().Project, "RELOAD_24")
}

func TestRegistry_Reload_PicksUpNewContent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "helixcode.md")
	require.NoError(t, os.WriteFile(file, []byte("V1"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
	_, err := r.Reload(context.Background())
	require.NoError(t, err)
	require.Equal(t, "V1", r.Snapshot().Project)

	require.NoError(t, os.WriteFile(file, []byte("V2"), 0644))
	_, err = r.Reload(context.Background())
	require.NoError(t, err)
	require.Equal(t, "V2", r.Snapshot().Project)
}

func TestRegistry_Reload_KeepsPreviousOnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("SKIP-OK: #P2-F24 root bypasses chmod 0000")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "helixcode.md")
	require.NoError(t, os.WriteFile(file, []byte("GOOD"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
	_, err := r.Reload(context.Background())
	require.NoError(t, err)
	require.Contains(t, r.Snapshot().Project, "GOOD")

	// Make file unreadable; Reload must err and registry must keep "GOOD".
	require.NoError(t, os.Chmod(file, 0000))
	defer os.Chmod(file, 0644)

	_, err = r.Reload(context.Background())
	require.Error(t, err)
	require.Contains(t, r.Snapshot().Project, "GOOD")
}

func TestRegistry_Reload_NilLoader_Err(t *testing.T) {
	r := NewMemoryRegistry(nil, t.TempDir())
	_, err := r.Reload(context.Background())
	require.Error(t, err)
}

func TestRegistry_Reload_CancelledCtx_Err(t *testing.T) {
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Reload(ctx)
	require.Error(t, err)
}

// TestRegistry_Concurrent_ReadWrite_NoDataRace exercises the lock-free read +
// mu-serialised write path under -race. Run via:
//
//	go test -race -count=1 ./internal/projectmemory/
func TestRegistry_Concurrent_ReadWrite_NoDataRace(t *testing.T) {
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), t.TempDir())
	r.Set(Memory{Project: "INITIAL"})

	const readers = 5
	const writes = 200

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = r.Snapshot()
				}
			}
		}()
	}

	for i := 0; i < writes; i++ {
		r.Set(Memory{Project: fmt.Sprintf("V%d", i)})
	}

	close(stop)
	wg.Wait()

	final := r.Snapshot().Project
	require.True(t, final == fmt.Sprintf("V%d", writes-1) || final == "INITIAL" || final == fmt.Sprintf("V%d", writes-2),
		"final value should be a recent write, got %q", final)
}

func TestRegistry_Implements_MemorySnapshotter(t *testing.T) {
	// Compile-time assertion that *MemoryRegistry implements the interface.
	var _ MemorySnapshotter = (*MemoryRegistry)(nil)
	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), t.TempDir())
	var s MemorySnapshotter = r
	require.NotNil(t, s)
	require.Empty(t, s.Snapshot().Project)
}
