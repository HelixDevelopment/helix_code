package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaybePersist_BelowThresholdIsInline(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)
	output := strings.Repeat("X", PersistThreshold-1)
	res, err := m.MaybePersist("Bash", "call-1", output)
	require.NoError(t, err)
	assert.False(t, res.WasPersisted)
	assert.Equal(t, output, res.Output)
	assert.Empty(t, res.PersistedOutputPath)
	assert.Equal(t, "Bash", res.ToolName)
	assert.Equal(t, "call-1", res.ToolCallID)
}

func TestMaybePersist_AtThresholdIsInline(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)
	output := strings.Repeat("X", PersistThreshold)
	res, err := m.MaybePersist("Bash", "call-1", output)
	require.NoError(t, err)
	assert.False(t, res.WasPersisted, "len == PersistThreshold must stay inline (boundary strictly greater)")
	assert.Equal(t, output, res.Output)
}

func TestMaybePersist_AboveThresholdIsPersisted(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)
	output := strings.Repeat("X", PersistThreshold+1)
	res, err := m.MaybePersist("Bash", "call-1", output)
	require.NoError(t, err)
	assert.True(t, res.WasPersisted)
	assert.Empty(t, res.Output)
	assert.NotEmpty(t, res.PersistedOutputPath)
	assert.Equal(t, PersistThreshold+1, res.PersistedOutputSize)

	body, err := os.ReadFile(res.PersistedOutputPath)
	require.NoError(t, err)
	assert.Equal(t, output, string(body))
}

func TestMaybePersist_HashIdempotence(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)
	output := strings.Repeat("Y", PersistThreshold+10)

	r1, err := m.MaybePersist("Bash", "call-1", output)
	require.NoError(t, err)
	require.True(t, r1.WasPersisted)

	r2, err := m.MaybePersist("Bash", "call-2", output)
	require.NoError(t, err)
	require.True(t, r2.WasPersisted)

	hash := sha256.Sum256([]byte(output))
	expectedHashPrefix := hex.EncodeToString(hash[:8])
	assert.Contains(t, r1.PersistedOutputPath, expectedHashPrefix,
		"filename must include sha256[:16] of content")
	assert.Contains(t, r2.PersistedOutputPath, expectedHashPrefix)
}

func TestMaybePersist_FilenameSanitises(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)
	output := strings.Repeat("Z", PersistThreshold+1)

	res, err := m.MaybePersist("../etc/passwd", "call-1", output)
	require.NoError(t, err)
	require.True(t, res.WasPersisted)

	base := filepath.Base(res.PersistedOutputPath)
	assert.NotContains(t, base, "..")
	assert.NotContains(t, base, "/")
	assert.NotContains(t, base, "\\")

	dir := filepath.Dir(res.PersistedOutputPath)
	expectedBase := filepath.Join(tmp, PersistDir)
	assert.Equal(t, expectedBase, dir)
}

func TestMaybePersist_EmptyOutputIsInline(t *testing.T) {
	tmp := t.TempDir()
	m := NewManager(tmp)
	res, err := m.MaybePersist("Bash", "call-1", "")
	require.NoError(t, err)
	assert.False(t, res.WasPersisted)
	assert.Empty(t, res.Output)
	assert.Equal(t, "Bash", res.ToolName)
}

func TestMaybePersist_NilManagerIsSafe(t *testing.T) {
	var m *Manager
	res, err := m.MaybePersist("Bash", "call-1", strings.Repeat("X", PersistThreshold+1))
	require.NoError(t, err)
	assert.False(t, res.WasPersisted, "nil Manager passes through inline")
	assert.Equal(t, strings.Repeat("X", PersistThreshold+1), res.Output)
}

func TestMaybePersist_DiskFullFallsBackToInline(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("SKIP-OK: #permissions-as-root — root bypasses chmod 0500 in this test")
	}
	tmp := t.TempDir()
	m := NewManager(tmp)

	// MkdirAll with 0500 on a two-level path fails because the intermediate
	// directory is also created with 0500 (no execute/write). Create the
	// parent normally, then the leaf with restricted permissions.
	persistRoot := filepath.Join(tmp, PersistDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(persistRoot), 0o755))
	require.NoError(t, os.Mkdir(persistRoot, 0o500))

	output := strings.Repeat("X", PersistThreshold+1)
	res, err := m.MaybePersist("Bash", "call-1", output)
	require.NoError(t, err, "disk-full must NOT propagate as error — fall back to inline")
	assert.False(t, res.WasPersisted)
	assert.Equal(t, output, res.Output)
}
