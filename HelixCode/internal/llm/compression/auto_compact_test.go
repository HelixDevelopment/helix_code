package compression

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/llm/compressioniface"
)

// mockTokens implements ProviderTokenCounter (the small interface AutoCompactor needs).
type mockTokens struct{ mock.Mock }

func (m *mockTokens) GetContextWindow() int { return m.Called().Int(0) }
func (m *mockTokens) CountTokens(text string) (int, error) {
	args := m.Called(text)
	return args.Int(0), args.Error(1)
}

// mockCoordinator stubs compressioniface.CompressionCoordinator.
type mockCoordinator struct{ mock.Mock }

func (m *mockCoordinator) Compress(ctx context.Context, conv *compressioniface.Conversation) (*compressioniface.CompressionResult, error) {
	args := m.Called(ctx, conv)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*compressioniface.CompressionResult), args.Error(1)
}
func (m *mockCoordinator) ShouldCompress(conv *compressioniface.Conversation) (bool, string) {
	return false, ""
}
func (m *mockCoordinator) EstimateCompression(conv *compressioniface.Conversation) (*compressioniface.CompressionEstimate, error) {
	return nil, nil
}
func (m *mockCoordinator) GetStats() *compressioniface.CompressionStats { return nil }
func (m *mockCoordinator) GetConfig() *compressioniface.Config          { return nil }
func (m *mockCoordinator) UpdateConfig(c *compressioniface.Config)      {}

func TestAutoCompactor_BelowThreshold_NoCompaction(t *testing.T) {
	tok := new(mockTokens)
	coord := new(mockCoordinator)
	ac := NewAutoCompactor(tok, coord, NewThrashingGuard(3), 0.80)

	tok.On("GetContextWindow").Return(100_000)
	tok.On("CountTokens", mock.Anything).Return(50_000, nil)

	conv := &compressioniface.Conversation{Messages: []*compressioniface.Message{{Content: "x"}}}
	result, err := ac.MaybeCompact(context.Background(), conv)
	require.NoError(t, err)
	require.False(t, result.WasCompacted)
}

func TestAutoCompactor_AboveThreshold_TriggersCompression(t *testing.T) {
	tok := new(mockTokens)
	coord := new(mockCoordinator)
	ac := NewAutoCompactor(tok, coord, NewThrashingGuard(3), 0.80)

	tok.On("GetContextWindow").Return(100_000)
	tok.On("CountTokens", mock.Anything).Return(85_000, nil)
	coord.On("Compress", mock.Anything, mock.Anything).Return(&compressioniface.CompressionResult{}, nil)

	conv := &compressioniface.Conversation{Messages: []*compressioniface.Message{{Content: "x"}}}
	result, err := ac.MaybeCompact(context.Background(), conv)
	require.NoError(t, err)
	require.True(t, result.WasCompacted)
	coord.AssertCalled(t, "Compress", mock.Anything, mock.Anything)
}

func TestAutoCompactor_ThrashingAfterFourth(t *testing.T) {
	tok := new(mockTokens)
	coord := new(mockCoordinator)
	guard := NewThrashingGuard(3)
	ac := NewAutoCompactor(tok, coord, guard, 0.80)

	tok.On("GetContextWindow").Return(100_000)
	tok.On("CountTokens", mock.Anything).Return(85_000, nil)
	coord.On("Compress", mock.Anything, mock.Anything).Return(&compressioniface.CompressionResult{}, nil)

	conv := &compressioniface.Conversation{Messages: []*compressioniface.Message{{Content: "x"}}}
	for i := 0; i < 3; i++ {
		_, err := ac.MaybeCompact(context.Background(), conv)
		require.NoError(t, err)
	}
	_, err := ac.MaybeCompact(context.Background(), conv)
	require.ErrorIs(t, err, ErrThrashing)
}
