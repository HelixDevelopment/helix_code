package browser

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvVarHeadedMode_Pin(t *testing.T) {
	require.Equal(t, "HELIXCODE_BROWSER_HEADED", EnvVarHeadedMode)
}

func TestMaxSnapshotBytes_Pin(t *testing.T) {
	require.Equal(t, 64*1024, MaxSnapshotBytes)
}

func TestMaxScreenshotBytes_Pin(t *testing.T) {
	require.Equal(t, int64(5*1024*1024), MaxScreenshotBytes)
}

func TestErrorSentinels_DistinctErrorsIs(t *testing.T) {
	for _, e := range []error{
		ErrNoActiveSession,
		ErrChromiumNotFound,
		ErrNavigationTimeout,
		ErrSelectorNotFound,
		ErrScreenshotTooLarge,
	} {
		wrapped := fmt.Errorf("wrapped: %w", e)
		require.ErrorIs(t, wrapped, e, "sentinel %v must be assertable via errors.Is", e)
	}
}

func TestSentinels_NotEqualToEachOther(t *testing.T) {
	// errors.Is reflexivity is a per-error pointer; ensure each sentinel
	// is distinct so callers can branch reliably.
	all := []error{
		ErrNoActiveSession,
		ErrChromiumNotFound,
		ErrNavigationTimeout,
		ErrSelectorNotFound,
		ErrScreenshotTooLarge,
	}
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			require.NotSame(t, all[i], all[j])
			require.NotEqual(t, all[i].Error(), all[j].Error())
		}
	}
}

func TestSnapshot_ZeroValue(t *testing.T) {
	var s Snapshot
	require.Equal(t, "", s.Mode)
	require.False(t, s.Truncated)
}

func TestScreenshotResult_ZeroValue(t *testing.T) {
	var r ScreenshotResult
	require.Equal(t, int64(0), r.Bytes)
}

func TestManagerStatus_ZeroValue(t *testing.T) {
	var s ManagerStatus
	require.False(t, s.Active)
	require.False(t, s.Headed)
}
