package llm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/persistence"
)

func TestToolCallingProvider_SetsPersistenceManager(t *testing.T) {
	p := &ToolCallingProvider{}
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)
	p.SetPersistenceManager(m)
	assert.NotNil(t, p.persistenceManager)
}

func TestPersistResults_BelowThresholdInline(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)
	p := &ToolCallingProvider{persistenceManager: m}

	results := map[string]interface{}{
		"Bash": "small output",
	}
	wrapped := p.persistResults(results)
	require.Len(t, wrapped, 1)
	assert.False(t, wrapped["Bash"].WasPersisted)
	assert.Equal(t, "small output", wrapped["Bash"].Output)
}

func TestPersistResults_AboveThresholdPersisted(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)
	p := &ToolCallingProvider{persistenceManager: m}

	big := strings.Repeat("Z", persistence.PersistThreshold+10)
	results := map[string]interface{}{
		"Bash": big,
	}
	wrapped := p.persistResults(results)
	require.Len(t, wrapped, 1)
	assert.True(t, wrapped["Bash"].WasPersisted)
	assert.Empty(t, wrapped["Bash"].Output)
	assert.NotEmpty(t, wrapped["Bash"].PersistedOutputPath)
	assert.Equal(t, persistence.PersistThreshold+10, wrapped["Bash"].PersistedOutputSize)
}

func TestPersistResults_NilManagerPassthrough(t *testing.T) {
	p := &ToolCallingProvider{persistenceManager: nil}

	big := strings.Repeat("Z", persistence.PersistThreshold+10)
	results := map[string]interface{}{
		"Bash": big,
	}
	wrapped := p.persistResults(results)
	require.Len(t, wrapped, 1)
	assert.False(t, wrapped["Bash"].WasPersisted)
	assert.Equal(t, big, wrapped["Bash"].Output)
}

func TestPersistResults_NonStringResultStringified(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)
	p := &ToolCallingProvider{persistenceManager: m}

	results := map[string]interface{}{
		"Calc": 42,
	}
	wrapped := p.persistResults(results)
	require.Len(t, wrapped, 1)
	assert.False(t, wrapped["Calc"].WasPersisted)
	assert.Equal(t, "42", wrapped["Calc"].Output)
}
