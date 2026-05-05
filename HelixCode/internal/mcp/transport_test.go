package mcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackoffSchedule_Sequence(t *testing.T) {
	bs := NewBackoffSchedule()
	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for i, base := range want {
		got := bs.Next()
		// jitter ±20% means got ∈ [base*0.8, base*1.2]
		require.GreaterOrEqual(t, got, time.Duration(float64(base)*0.8), "step %d", i)
		require.LessOrEqual(t, got, time.Duration(float64(base)*1.2), "step %d", i)
	}
}

func TestBackoffSchedule_ResetAfterSuccess(t *testing.T) {
	bs := NewBackoffSchedule()
	bs.Next()
	bs.Next()
	bs.Next()
	bs.Reset()
	got := bs.Next()
	assert.GreaterOrEqual(t, got, time.Duration(float64(time.Second)*0.8))
	assert.LessOrEqual(t, got, time.Duration(float64(time.Second)*1.2))
}

func TestTransportType_Validate(t *testing.T) {
	cases := map[TransportType]bool{
		TransportStdio:         true,
		TransportHTTP:          true,
		TransportSSE:           true,
		TransportWS:            true,
		TransportType("bogus"): false,
		TransportType(""):      false,
	}
	for tt, ok := range cases {
		err := tt.Validate()
		if ok {
			assert.NoError(t, err, string(tt))
		} else {
			assert.Error(t, err, string(tt))
		}
	}
}

func TestClientState_String(t *testing.T) {
	assert.Equal(t, "disconnected", StateDisconnected.String())
	assert.Equal(t, "ready", StateReady.String())
	assert.Equal(t, "reconnecting", StateReconnecting.String())
	assert.Equal(t, "closed", StateClosed.String())
}
