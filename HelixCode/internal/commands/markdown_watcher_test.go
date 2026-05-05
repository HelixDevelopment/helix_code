package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWatcher_DebouncesAndReloads(t *testing.T) {
	dir := t.TempDir()
	cmds := filepath.Join(dir, ".helix", "commands")
	require.NoError(t, os.MkdirAll(cmds, 0755))

	reg := NewRegistry()
	loader := NewMarkdownLoader(reg, cmds, "")
	require.NoError(t, loader.Load())

	w, err := NewMarkdownWatcher(loader, []string{cmds})
	require.NoError(t, err)
	w.SetDebounce(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, os.WriteFile(filepath.Join(cmds, "added.md"), []byte("hi"), 0644))
	require.Eventually(t, func() bool {
		_, ok := reg.Get("added")
		return ok
	}, 3*time.Second, 25*time.Millisecond)
}

func TestWatcher_StopsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	cmds := filepath.Join(dir, ".helix", "commands")
	require.NoError(t, os.MkdirAll(cmds, 0755))
	reg := NewRegistry()
	loader := NewMarkdownLoader(reg, cmds, "")
	w, err := NewMarkdownWatcher(loader, []string{cmds})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop on ctx cancel")
	}
}

func TestWatcher_HandlesMissingDirGracefully(t *testing.T) {
	w, err := NewMarkdownWatcher(nil, []string{"/tmp/p1f09-does-not-exist-12345"})
	require.NoError(t, err)
	w.Close()
}

func TestWatcher_DebounceCollapsesRapidWrites(t *testing.T) {
	dir := t.TempDir()
	cmds := filepath.Join(dir, ".helix", "commands")
	require.NoError(t, os.MkdirAll(cmds, 0755))
	reg := NewRegistry()
	loader := NewMarkdownLoader(reg, cmds, "")
	require.NoError(t, loader.Load())

	w, err := NewMarkdownWatcher(loader, []string{cmds})
	require.NoError(t, err)
	w.SetDebounce(80 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	path := filepath.Join(cmds, "x.md")
	for i := 0; i < 5; i++ {
		require.NoError(t, os.WriteFile(path, []byte("body"), 0644))
		time.Sleep(10 * time.Millisecond)
	}
	require.Eventually(t, func() bool {
		_, ok := reg.Get("x")
		return ok
	}, 2*time.Second, 25*time.Millisecond)
}
