package compression

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/hooks"
	"dev.helix.code/internal/llm/compressioniface"
)

// ---------------------------------------------------------------------------
// Test: SetHooksManager accepts a manager (non-nil field after call).
// ---------------------------------------------------------------------------

func TestAutoCompactor_SetHooksManager_AcceptsManager(t *testing.T) {
	tok := new(mockTokens)
	coord := new(mockCoordinator)
	ac := NewAutoCompactor(tok, coord, NewThrashingGuard(3), 0.80)

	mgr := hooks.NewManager()
	ac.SetHooksManager(mgr)

	// The setter must not panic and the hook must fire when we trigger a
	// compaction. We verify that by running a compaction in the next test;
	// here we just confirm that the call itself doesn't blow up.
	assert.NotNil(t, mgr)
}

// ---------------------------------------------------------------------------
// Test: OnCompaction hook fires exactly once after a successful compaction.
// ---------------------------------------------------------------------------

func TestAutoCompactor_FiresOnCompactionAfterSuccessfulCompaction(t *testing.T) {
	tok := new(mockTokens)
	coord := new(mockCoordinator)
	ac := NewAutoCompactor(tok, coord, NewThrashingGuard(3), 0.80)

	// Build a manager and register a counter hook.
	mgr := hooks.NewManager()
	fired := 0
	var capturedData map[string]interface{}

	h := hooks.NewHook("test-on-compaction", hooks.HookTypeOnCompaction, func(ctx context.Context, event *hooks.Event) error {
		fired++
		capturedData = event.Data
		return nil
	})
	require.NoError(t, mgr.Register(h))
	ac.SetHooksManager(mgr)

	// Set up mocks: token count above 80% of window → compaction triggers.
	tok.On("GetContextWindow").Return(100_000)
	tok.On("CountTokens", mock.Anything).Return(85_000, nil)
	coord.On("Compress", mock.Anything, mock.Anything).Return(
		&compressioniface.CompressionResult{TokensSaved: 30_000}, nil,
	)

	conv := &compressioniface.Conversation{
		Messages: []*compressioniface.Message{{Content: "x"}},
	}
	result, err := ac.MaybeCompact(context.Background(), conv)
	require.NoError(t, err)
	require.True(t, result.WasCompacted)

	// Hook must have fired exactly once.
	assert.Equal(t, 1, fired)

	// Payload keys must be present and non-nil.
	require.NotNil(t, capturedData)
	_, hasBefore := capturedData["before_size"]
	_, hasAfter := capturedData["after_size"]
	_, hasCompacted := capturedData["messages_compacted"]
	assert.True(t, hasBefore, "expected before_size in event.Data")
	assert.True(t, hasAfter, "expected after_size in event.Data")
	assert.True(t, hasCompacted, "expected messages_compacted in event.Data")
}

// ---------------------------------------------------------------------------
// Test: A blocking OnCompaction hook causes MaybeCompact to return an error.
// ---------------------------------------------------------------------------

func TestAutoCompactor_BlockerFromHookAbortsCompaction(t *testing.T) {
	tok := new(mockTokens)
	coord := new(mockCoordinator)
	ac := NewAutoCompactor(tok, coord, NewThrashingGuard(3), 0.80)

	mgr := hooks.NewManager()
	blockerMsg := "compaction rejected by policy"
	h := hooks.NewHook("blocking-hook", hooks.HookTypeOnCompaction, func(ctx context.Context, event *hooks.Event) error {
		return errors.New(blockerMsg)
	})
	require.NoError(t, mgr.Register(h))
	ac.SetHooksManager(mgr)

	tok.On("GetContextWindow").Return(100_000)
	tok.On("CountTokens", mock.Anything).Return(85_000, nil)
	coord.On("Compress", mock.Anything, mock.Anything).Return(
		&compressioniface.CompressionResult{TokensSaved: 30_000}, nil,
	)

	conv := &compressioniface.Conversation{
		Messages: []*compressioniface.Message{{Content: "x"}},
	}
	_, err := ac.MaybeCompact(context.Background(), conv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), blockerMsg)
}

// ---------------------------------------------------------------------------
// Test: No hooks manager → passthrough; no error, no panic.
// ---------------------------------------------------------------------------

func TestAutoCompactor_NilHooksManagerIsPassthrough(t *testing.T) {
	tok := new(mockTokens)
	coord := new(mockCoordinator)
	// No SetHooksManager call — hooksManager stays nil.
	ac := NewAutoCompactor(tok, coord, NewThrashingGuard(3), 0.80)

	tok.On("GetContextWindow").Return(100_000)
	tok.On("CountTokens", mock.Anything).Return(85_000, nil)
	coord.On("Compress", mock.Anything, mock.Anything).Return(
		&compressioniface.CompressionResult{TokensSaved: 30_000}, nil,
	)

	conv := &compressioniface.Conversation{
		Messages: []*compressioniface.Message{{Content: "x"}},
	}
	result, err := ac.MaybeCompact(context.Background(), conv)
	require.NoError(t, err)
	require.True(t, result.WasCompacted)
}
