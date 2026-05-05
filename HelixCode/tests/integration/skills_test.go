//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/agent"
	"dev.helix.code/internal/commands"
)

// TestSkills_LoadAndAutoMatch exercises the full F10 path: real .md file in
// a real tempdir, loaded via SkillLoader, auto-matched via SkillDispatcher,
// rendered with named-capture groups.
func TestSkills_LoadAndAutoMatch(t *testing.T) {
	dir := t.TempDir()
	skills := filepath.Join(dir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))
	body := "---\ndescription: refactor\ntriggers:\n  - \"refactor (?P<comp>[A-Z][A-Za-z]+) component\"\n---\n\nRefactoring {{ARG.comp}}\n"
	require.NoError(t, os.WriteFile(filepath.Join(skills, "refactor.md"), []byte(body), 0644))

	reg := commands.NewSkillRegistry()
	loader := commands.NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())

	dispatcher := agent.NewSkillDispatcher(reg, nil)
	rendered, skill, caps, ok, err := dispatcher.Match(
		context.Background(),
		"please refactor LoginButton component now",
		"", "")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "refactor", skill.Name())
	assert.Equal(t, "LoginButton", caps["comp"])
	assert.Contains(t, rendered, "Refactoring LoginButton")
}

// TestSkills_RequiresIsolation_Flag confirms that the RequiresIsolation flag
// flows through SkillLoader → SkillRegistry → SkillDispatcher.Match.
func TestSkills_RequiresIsolation_Flag(t *testing.T) {
	dir := t.TempDir()
	skills := filepath.Join(dir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))
	body := "---\ndescription: iso skill\ntriggers:\n  - \"^iso$\"\nrequires_isolation: true\n---\n\nisolated body\n"
	require.NoError(t, os.WriteFile(filepath.Join(skills, "iso.md"), []byte(body), 0644))

	reg := commands.NewSkillRegistry()
	loader := commands.NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())

	dispatcher := agent.NewSkillDispatcher(reg, nil)
	_, skill, _, ok, err := dispatcher.Match(context.Background(), "iso", "", "")
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, skill.RequiresIsolation())
}

// TestSkills_WatcherReloadsOnFileWrite uses real fsnotify with a real tempdir
// and asserts the registry picks up newly-written skill files.
func TestSkills_WatcherReloadsOnFileWrite(t *testing.T) {
	dir := t.TempDir()
	skills := filepath.Join(dir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))

	reg := commands.NewSkillRegistry()
	loader := commands.NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())

	w, err := commands.NewSkillsWatcher(loader, []string{skills})
	require.NoError(t, err)
	w.SetDebounce(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	body := "---\ndescription: late-add\ntriggers:\n  - \"^late$\"\n---\n\nadded body\n"
	require.NoError(t, os.WriteFile(filepath.Join(skills, "late.md"), []byte(body), 0644))
	require.Eventually(t, func() bool {
		_, ok := reg.Get("late")
		return ok
	}, 3*time.Second, 25*time.Millisecond)
}
