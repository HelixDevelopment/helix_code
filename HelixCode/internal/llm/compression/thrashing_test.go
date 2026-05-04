package compression

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThrashingGuard_AllowsFirstThreeCompactions(t *testing.T) {
	g := NewThrashingGuard(3)
	for i := 0; i < 3; i++ {
		err := g.RecordCompaction()
		require.NoError(t, err, "compaction %d should not error", i+1)
	}
}

func TestThrashingGuard_AbortsFourthConsecutive(t *testing.T) {
	g := NewThrashingGuard(3)
	for i := 0; i < 3; i++ {
		_ = g.RecordCompaction()
	}
	err := g.RecordCompaction()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrThrashing), "fourth consecutive must be ErrThrashing")
}

func TestThrashingGuard_ResetOnUserMessage(t *testing.T) {
	g := NewThrashingGuard(3)
	for i := 0; i < 3; i++ {
		_ = g.RecordCompaction()
	}
	g.NoteUserMessage()
	err := g.RecordCompaction()
	require.NoError(t, err, "after NoteUserMessage, the guard should reset")
}

func TestThrashingGuard_ZeroLimitAllowsNothing(t *testing.T) {
	g := NewThrashingGuard(0)
	err := g.RecordCompaction()
	require.Error(t, err)
}
