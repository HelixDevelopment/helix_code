package persistence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupOld_RemovesAgedFiles(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)
	require.NoError(t, os.MkdirAll(m.baseDir, 0o755))

	old := filepath.Join(m.baseDir, "Bash_aabbccddeeff0011_20200101_000000.txt")
	require.NoError(t, os.WriteFile(old, []byte(strings.Repeat("X", 100)), 0o644))
	require.NoError(t, os.Chtimes(old, time.Now().Add(-30*24*time.Hour), time.Now().Add(-30*24*time.Hour)))

	new := filepath.Join(m.baseDir, "Bash_001122334455_20260101_000000.txt")
	require.NoError(t, os.WriteFile(new, []byte(strings.Repeat("Y", 100)), 0o644))

	require.NoError(t, m.CleanupOld(7*24*time.Hour))

	_, errOld := os.Stat(old)
	assert.True(t, os.IsNotExist(errOld), "aged file must be deleted")
	_, errNew := os.Stat(new)
	assert.NoError(t, errNew, "fresh file must remain")
}

func TestCleanupOld_LeavesNonPatternFiles(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)
	require.NoError(t, os.MkdirAll(m.baseDir, 0o755))

	gitkeep := filepath.Join(m.baseDir, ".gitkeep")
	require.NoError(t, os.WriteFile(gitkeep, []byte(""), 0o644))
	require.NoError(t, os.Chtimes(gitkeep, time.Now().Add(-30*24*time.Hour), time.Now().Add(-30*24*time.Hour)))

	readme := filepath.Join(m.baseDir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("docs"), 0o644))
	require.NoError(t, os.Chtimes(readme, time.Now().Add(-30*24*time.Hour), time.Now().Add(-30*24*time.Hour)))

	require.NoError(t, m.CleanupOld(7*24*time.Hour))

	_, errKeep := os.Stat(gitkeep)
	assert.NoError(t, errKeep, ".gitkeep must be left alone")
	_, errReadme := os.Stat(readme)
	assert.NoError(t, errReadme, "README.md must be left alone")
}

func TestCleanupOld_LeavesDirectories(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)
	require.NoError(t, os.MkdirAll(filepath.Join(m.baseDir, "subdir"), 0o755))

	require.NoError(t, m.CleanupOld(7*24*time.Hour))

	info, err := os.Stat(filepath.Join(m.baseDir, "subdir"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestCleanupOld_MissingBaseDirIsNoOp(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp) // baseDir not created
	require.NoError(t, m.CleanupOld(7*24*time.Hour))
}
