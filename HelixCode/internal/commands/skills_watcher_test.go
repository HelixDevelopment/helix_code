package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSkillsWatcher_DebouncesAndReloads(t *testing.T) {
	dir := t.TempDir()
	skills := filepath.Join(dir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))

	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())

	w, err := NewSkillsWatcher(loader, []string{skills})
	require.NoError(t, err)
	w.SetDebounce(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, os.WriteFile(filepath.Join(skills, "added.md"),
		[]byte("---\ndescription: a\ntriggers: [\"^a\"]\n---\nbody"), 0644))
	require.Eventually(t, func() bool {
		_, ok := reg.Get("added")
		return ok
	}, 3*time.Second, 25*time.Millisecond)
}

func TestSkillsWatcher_StopsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	skills := filepath.Join(dir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))
	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, skills, "")
	w, err := NewSkillsWatcher(loader, []string{skills})
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

func TestSkillsWatcher_HandlesMissingDirGracefully(t *testing.T) {
	w, err := NewSkillsWatcher(nil, []string{"/tmp/p1f10-skills-does-not-exist-12345"})
	require.NoError(t, err)
	w.Close()
}
