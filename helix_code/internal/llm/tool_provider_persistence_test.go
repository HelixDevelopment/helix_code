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

	results := []ToolCallResult{
		{CallID: "c1", ToolName: "Bash", Result: "small output"},
	}
	wrapped := p.persistResults(results)
	require.Len(t, wrapped, 1)
	assert.Equal(t, "c1", wrapped[0].callID)
	assert.False(t, wrapped[0].persisted.WasPersisted)
	assert.Equal(t, "small output", wrapped[0].persisted.Output)
}

func TestPersistResults_AboveThresholdPersisted(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)
	p := &ToolCallingProvider{persistenceManager: m}

	big := strings.Repeat("Z", persistence.PersistThreshold+10)
	results := []ToolCallResult{
		{CallID: "c1", ToolName: "Bash", Result: big},
	}
	wrapped := p.persistResults(results)
	require.Len(t, wrapped, 1)
	assert.True(t, wrapped[0].persisted.WasPersisted)
	assert.Empty(t, wrapped[0].persisted.Output)
	assert.NotEmpty(t, wrapped[0].persisted.PersistedOutputPath)
	assert.Equal(t, persistence.PersistThreshold+10, wrapped[0].persisted.PersistedOutputSize)
}

func TestPersistResults_NilManagerPassthrough(t *testing.T) {
	p := &ToolCallingProvider{persistenceManager: nil}

	big := strings.Repeat("Z", persistence.PersistThreshold+10)
	results := []ToolCallResult{
		{CallID: "c1", ToolName: "Bash", Result: big},
	}
	wrapped := p.persistResults(results)
	require.Len(t, wrapped, 1)
	assert.False(t, wrapped[0].persisted.WasPersisted)
	assert.Equal(t, big, wrapped[0].persisted.Output)
}

func TestPersistResults_NonStringResultStringified(t *testing.T) {
	tmp := t.TempDir()
	m := persistence.NewManager(tmp)
	p := &ToolCallingProvider{persistenceManager: m}

	results := []ToolCallResult{
		{CallID: "c1", ToolName: "Calc", Result: 42},
	}
	wrapped := p.persistResults(results)
	require.Len(t, wrapped, 1)
	assert.False(t, wrapped[0].persisted.WasPersisted)
	assert.Equal(t, "42", wrapped[0].persisted.Output)
}

// TestPersistResults_PreservesOrderAndSameToolTwice is the P3-T04 follow-up
// regression guard for the "keyed by name, losing order" defect: a turn that
// calls the SAME tool twice must yield TWO distinct ordered entries — a
// name-keyed map would have silently merged them and randomised the order.
func TestPersistResults_PreservesOrderAndSameToolTwice(t *testing.T) {
	p := &ToolCallingProvider{persistenceManager: nil}
	results := []ToolCallResult{
		{CallID: "call_a", ToolName: "fs_read", Result: "first read"},
		{CallID: "call_b", ToolName: "grep", Result: "grep hit"},
		{CallID: "call_c", ToolName: "fs_read", Result: "second read"},
	}
	wrapped := p.persistResults(results)
	require.Len(t, wrapped, 3, "two calls to fs_read must NOT collapse into one entry")
	assert.Equal(t, "call_a", wrapped[0].callID)
	assert.Equal(t, "first read", wrapped[0].persisted.Output)
	assert.Equal(t, "call_b", wrapped[1].callID)
	assert.Equal(t, "call_c", wrapped[2].callID)
	assert.Equal(t, "second read", wrapped[2].persisted.Output)
}
