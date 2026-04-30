package verifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallbackModels_HasSevenEntries(t *testing.T) {
	require.Len(t, FallbackModels, 7, "FallbackModels must have exactly 7 entries per CONST-035")
}

func TestFallbackModels_AllHaveRequiredFields(t *testing.T) {
	for _, m := range FallbackModels {
		assert.NotEmpty(t, m.ID, "model ID must not be empty")
		assert.NotEmpty(t, m.Name, "model Name must not be empty")
		assert.NotEmpty(t, m.Provider, "model Provider must not be empty")
		assert.Greater(t, m.ContextSize, 0, "model ContextSize must be > 0")
		assert.Equal(t, "fallback", m.Source, "fallback models must have Source='fallback'")
		assert.Greater(t, m.OverallScore, 0.0, "model OverallScore must be > 0")
	}
}

func TestFallbackModels_ContainsExpectedProviders(t *testing.T) {
	providers := make(map[string]bool)
	for _, m := range FallbackModels {
		providers[m.Provider] = true
	}
	expected := []string{"ollama", "openai", "anthropic", "mistral", "gemini", "deepseek", "xai"}
	for _, p := range expected {
		assert.True(t, providers[p], "fallback models must contain provider %s", p)
	}
}

func TestSentinelErrors(t *testing.T) {
	assert.EqualError(t, ErrVerifierDisabled, "verifier is disabled")
	assert.EqualError(t, ErrVerifierUnavailable, "verifier service is unavailable")
	assert.EqualError(t, ErrUsingStaleCache, "using stale cached verifier data")
	assert.EqualError(t, ErrUsingFallback, "using fallback model list")
}
