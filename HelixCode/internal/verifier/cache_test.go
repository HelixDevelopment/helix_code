package verifier

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_GetModels_Miss(t *testing.T) {
	c := NewCache(5*time.Minute, nil)
	_, ok := c.GetModels("all")
	assert.False(t, ok)
}

func TestCache_GetModels_Hit(t *testing.T) {
	c := NewCache(5*time.Minute, nil)
	models := []*VerifiedModel{{ID: "gpt-4o", Provider: "openai"}}
	c.SetModels("all", models)

	got, ok := c.GetModels("all")
	require.True(t, ok)
	require.Len(t, got, 1)
	assert.Equal(t, "gpt-4o", got[0].ID)
}

func TestCache_GetModels_Expired(t *testing.T) {
	c := NewCache(50*time.Millisecond, nil)
	models := []*VerifiedModel{{ID: "gpt-4o", Provider: "openai"}}
	c.SetModels("all", models)

	_, ok := c.GetModels("all")
	require.True(t, ok)

	time.Sleep(60 * time.Millisecond)
	_, ok = c.GetModels("all")
	assert.False(t, ok)
}

func TestCache_GetModelsStale_SlightlyExpired(t *testing.T) {
	c := NewCache(50*time.Millisecond, nil)
	models := []*VerifiedModel{{ID: "gpt-4o", Provider: "openai"}}
	c.SetModels("all", models)

	time.Sleep(60 * time.Millisecond)
	// Normal Get should miss
	_, ok := c.GetModels("all")
	assert.False(t, ok)

	// Stale get should hit (up to 2x TTL)
	got, ok := c.GetModelsStale("all")
	require.True(t, ok)
	assert.Equal(t, "gpt-4o", got[0].ID)

	// After 2x TTL, stale should also miss
	time.Sleep(60 * time.Millisecond)
	_, ok = c.GetModelsStale("all")
	assert.False(t, ok)
}

func TestCache_Invalidate(t *testing.T) {
	c := NewCache(5*time.Minute, nil)
	c.SetModels("all", []*VerifiedModel{{ID: "gpt-4o"}})
	_, ok := c.GetModels("all")
	require.True(t, ok)

	c.Invalidate("all")
	_, ok = c.GetModels("all")
	assert.False(t, ok)
}

func TestCache_InvalidateAll(t *testing.T) {
	c := NewCache(5*time.Minute, nil)
	c.SetModels("all", []*VerifiedModel{{ID: "gpt-4o"}})
	c.SetModels("openai", []*VerifiedModel{{ID: "gpt-4"}})

	c.InvalidateAll()
	_, ok := c.GetModels("all")
	assert.False(t, ok)
	_, ok = c.GetModels("openai")
	assert.False(t, ok)
}

func TestCache_Eviction(t *testing.T) {
	c := NewCache(5*time.Minute, nil)
	c.maxSize = 3
	c.SetModels("a", []*VerifiedModel{{ID: "1"}})
	c.SetModels("b", []*VerifiedModel{{ID: "2"}})
	c.SetModels("c", []*VerifiedModel{{ID: "3"}})
	c.SetModels("d", []*VerifiedModel{{ID: "4"}})

	// One of the first three should be evicted
	count := 0
	for _, key := range []string{"a", "b", "c", "d"} {
		if _, ok := c.GetModels(key); ok {
			count++
		}
	}
	assert.Equal(t, 3, count)
}

func TestCache_GetModelScore(t *testing.T) {
	c := NewCache(5*time.Minute, nil)
	c.SetScores(map[string]float64{"gpt-4o": 9.1})

	score, ok := c.GetModelScore("gpt-4o")
	require.True(t, ok)
	assert.Equal(t, 9.1, score)

	_, ok = c.GetModelScore("missing")
	assert.False(t, ok)
}
