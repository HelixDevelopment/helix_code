//go:build integration

package persistence_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/persistence"
)

// TestIntegration_LargeOutputIsPersistedAndReloadable proves that a >50K
// output is written to disk under .helix/tool-results/ and the resulting
// PersistedResult.PersistedOutputPath is a real file whose content matches
// the original byte-for-byte. NO mocks.
func TestIntegration_LargeOutputIsPersistedAndReloadable(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)

	output := strings.Repeat("X", 60_000)
	res, err := m.MaybePersist("Bash", "call-1", output)
	require.NoError(t, err)
	require.True(t, res.WasPersisted)

	info, err := os.Stat(res.PersistedOutputPath)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
	assert.Equal(t, int64(60_000), info.Size())

	body, err := os.ReadFile(res.PersistedOutputPath)
	require.NoError(t, err)
	assert.Equal(t, output, string(body))

	loaded, err := m.LoadPersisted(res.PersistedOutputPath)
	require.NoError(t, err)
	assert.Equal(t, output, loaded)

	expectedDir := filepath.Join(tmp, persistence.PersistDir)
	assert.Equal(t, expectedDir, filepath.Dir(res.PersistedOutputPath))
}

// TestIntegration_BelowThresholdNeverWritesToDisk proves that a small
// output never creates a file under .helix/tool-results/.
func TestIntegration_BelowThresholdNeverWritesToDisk(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)

	output := strings.Repeat("Y", persistence.PersistThreshold-1)
	res, err := m.MaybePersist("Bash", "call-1", output)
	require.NoError(t, err)
	require.False(t, res.WasPersisted)

	_, err = os.Stat(filepath.Join(tmp, persistence.PersistDir))
	assert.True(t, os.IsNotExist(err),
		"below-threshold persist must not create the .helix/tool-results dir")
}

// TestIntegration_PathTraversalIsRejected proves that LoadPersisted refuses
// to read files outside its baseDir, even if a malicious path is provided.
func TestIntegration_PathTraversalIsRejected(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)

	sensitive := filepath.Join(tmp, "secrets.txt")
	require.NoError(t, os.WriteFile(sensitive, []byte("topsecret"), 0o600))

	_, err := m.LoadPersisted(sensitive)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path outside persistence directory")
}
