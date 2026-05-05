package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeProjectIdentity_GitRoot(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init").Run())
	sub := filepath.Join(dir, "sub", "deep")
	require.NoError(t, os.MkdirAll(sub, 0755))
	saved, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(saved)
	require.NoError(t, os.Chdir(sub))

	id, err := ComputeProjectIdentity()
	require.NoError(t, err)
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(id)
	assert.Equal(t, want, got)
}

func TestComputeProjectIdentity_NoGitFallback(t *testing.T) {
	dir := t.TempDir()
	saved, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(saved)
	require.NoError(t, os.Chdir(dir))

	id, err := ComputeProjectIdentity()
	require.NoError(t, err)
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(id)
	assert.Equal(t, want, got)
}
