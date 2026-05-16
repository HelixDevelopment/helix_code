package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPersisted_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)

	output := strings.Repeat("A", PersistThreshold+5)
	res, err := m.MaybePersist("Bash", "call-1", output)
	require.NoError(t, err)
	require.True(t, res.WasPersisted)

	got, err := m.LoadPersisted(res.PersistedOutputPath)
	require.NoError(t, err)
	assert.Equal(t, output, got)
}

func TestLoadPersisted_RejectsParentTraversal(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)

	sensitive := filepath.Join(tmp, "secrets.txt")
	require.NoError(t, os.WriteFile(sensitive, []byte("topsecret"), 0o600))

	traversal := filepath.Join(tmp, PersistDir, "..", "secrets.txt")
	_, err := m.LoadPersisted(traversal)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPathTraversal),
		"expected ErrPathTraversal, got %v", err)
}

func TestLoadPersisted_RejectsAbsoluteOutsideBase(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)

	sensitive := filepath.Join(tmp, "etc_passwd")
	require.NoError(t, os.WriteFile(sensitive, []byte("uid=0"), 0o600))

	_, err := m.LoadPersisted(sensitive)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPathTraversal),
		"absolute path outside base must be rejected with ErrPathTraversal, got %v", err)
}

func TestLoadPersisted_MissingFileWrapsErrNotExist(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)

	missing := filepath.Join(tmp, PersistDir, "nope.txt")
	_, err := m.LoadPersisted(missing)
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist),
		"missing file must wrap os.ErrNotExist, got %v", err)
}
