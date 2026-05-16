package notification

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	metrics := NewMetrics()

	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.ChannelMetrics)
	assert.Equal(t, int64(0), metrics.TotalSent)
	assert.Equal(t, int64(0), metrics.TotalFailed)
	assert.Equal(t, time.Hour, metrics.MinResponseTime) // Initial high value
}

func TestMetricsRecordSent(t *testing.T) {
	metrics := NewMetrics()

	t.Run("record sent increments counter", func(t *testing.T) {
		metrics.RecordSent("slack", 100*time.Millisecond)

		assert.Equal(t, int64(1), metrics.TotalSent)
		assert.NotNil(t, metrics.ChannelMetrics["slack"])
		assert.Equal(t, int64(1), metrics.ChannelMetrics["slack"].Sent)
	})

	t.Run("record multiple sends", func(t *testing.T) {
		metrics.RecordSent("slack", 200*time.Millisecond)
		metrics.RecordSent("discord", 150*time.Millisecond)

		assert.Equal(t, int64(3), metrics.TotalSent)
		assert.Equal(t, int64(2), metrics.ChannelMetrics["slack"].Sent)
		assert.Equal(t, int64(1), metrics.ChannelMetrics["discord"].Sent)
	})
}

func TestMetricsRecordFailed(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordFailed("slack")

	assert.Equal(t, int64(1), metrics.TotalFailed)
	assert.NotNil(t, metrics.ChannelMetrics["slack"])
	assert.Equal(t, int64(1), metrics.ChannelMetrics["slack"].Failed)

	// Record another failure
	metrics.RecordFailed("slack")
	assert.Equal(t, int64(2), metrics.TotalFailed)
	assert.Equal(t, int64(2), metrics.ChannelMetrics["slack"].Failed)
}

func TestMetricsRecordRetry(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordRetry("email")

	assert.Equal(t, int64(1), metrics.TotalRetries)
	assert.NotNil(t, metrics.ChannelMetrics["email"])
	assert.Equal(t, int64(1), metrics.ChannelMetrics["email"].Retries)
}

func TestMetricsRecordQueued(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordQueued()
	metrics.RecordQueued()

	assert.Equal(t, int64(2), metrics.TotalQueued)
}

func TestMetricsRecordRateLimited(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordRateLimited("telegram")

	assert.Equal(t, int64(1), metrics.TotalRateLimited)
	assert.NotNil(t, metrics.ChannelMetrics["telegram"])
	assert.Equal(t, int64(1), metrics.ChannelMetrics["telegram"].RateLimited)
}

func TestMetricsRecordEventProcessed(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordEventProcessed()
	metrics.RecordEventProcessed()

	assert.Equal(t, int64(2), metrics.EventsProcessed)
}

func TestMetricsRecordEventIgnored(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordEventIgnored()

	assert.Equal(t, int64(1), metrics.EventsIgnored)
}

func TestMetricsGetMetrics(t *testing.T) {
	metrics := NewMetrics()

	// Record some data
	metrics.RecordSent("slack", 100*time.Millisecond)
	metrics.RecordFailed("slack")
	metrics.RecordEventProcessed()

	snapshot := metrics.GetMetrics()

	assert.Equal(t, int64(1), snapshot.TotalSent)
	assert.Equal(t, int64(1), snapshot.TotalFailed)
	assert.Equal(t, int64(1), snapshot.EventsProcessed)
	assert.NotNil(t, snapshot.ChannelMetrics)
}

func TestMetricsReset(t *testing.T) {
	metrics := NewMetrics()

	// Record some data
	metrics.RecordSent("slack", 100*time.Millisecond)
	metrics.RecordFailed("slack")
	metrics.RecordEventProcessed()

	// Reset
	metrics.Reset()

	assert.Equal(t, int64(0), metrics.TotalSent)
	assert.Equal(t, int64(0), metrics.TotalFailed)
	assert.Equal(t, int64(0), metrics.EventsProcessed)
	assert.Empty(t, metrics.ChannelMetrics)
}

func TestMetricsGetSuccessRate(t *testing.T) {
	metrics := NewMetrics()

	t.Run("no data returns 100 (default success)", func(t *testing.T) {
		rate := metrics.GetSuccessRate()
		// When there's no data, implementation returns 100%
		assert.Equal(t, float64(100), rate)
	})

	t.Run("all successful returns 100", func(t *testing.T) {
		metrics.RecordSent("slack", 100*time.Millisecond)
		metrics.RecordSent("discord", 100*time.Millisecond)

		rate := metrics.GetSuccessRate()
		assert.Equal(t, float64(100), rate)
	})

	t.Run("mixed results returns correct rate", func(t *testing.T) {
		metrics.Reset()
		metrics.RecordSent("slack", 100*time.Millisecond)
		metrics.RecordFailed("slack")

		rate := metrics.GetSuccessRate()
		assert.Equal(t, float64(50), rate)
	})
}

func TestMetricsGetChannelSuccessRate(t *testing.T) {
	metrics := NewMetrics()

	t.Run("unknown channel returns 100 (default success)", func(t *testing.T) {
		rate := metrics.GetChannelSuccessRate("unknown")
		// When there's no channel or no data, implementation returns 100%
		assert.Equal(t, float64(100), rate)
	})

	t.Run("channel with all successes returns 100", func(t *testing.T) {
		metrics.RecordSent("slack", 100*time.Millisecond)

		rate := metrics.GetChannelSuccessRate("slack")
		assert.Equal(t, float64(100), rate)
	})

	t.Run("channel with mixed results", func(t *testing.T) {
		metrics.RecordFailed("slack")

		rate := metrics.GetChannelSuccessRate("slack")
		assert.Equal(t, float64(50), rate)
	})
}
