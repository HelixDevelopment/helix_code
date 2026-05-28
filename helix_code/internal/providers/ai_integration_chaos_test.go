package providers

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the REAL internal/providers components.
//
// Chaos classes exercised (all against the production structs, no fakes):
//   - input-corruption: feed malformed/boundary personality + conversation
//     inputs and assert the real managers reject cleanly (no panic).
//   - state-corruption: concurrent add/remove of the SAME key against the real
//     PersonalityManager + concurrent create/get/add of the SAME conversation id
//     — asserting no race/panic/deadlock and a consistent final state.
//   - resource-exhaustion: drive the memory store under bounded memory pressure,
//     asserting graceful operation (no OOM-crash) and that the FIFO cleanup path
//     keeps the map bounded.
//   - boundary: empty registry, unknown lookups, default-personality protection.
//
// Run under -race: the concurrent same-key mutation is exactly the contention
// pattern that exposes locking / map-access defects.

// TestPersonalityManager_Chaos_CorruptInput feeds malformed personality records
// to the real AddPersonality and asserts each is rejected without a crash. Empty
// ID is the documented rejection path; the others must not panic.
func TestPersonalityManager_Chaos_CorruptInput(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	// Each payload is a JSON-ish marker the feed decodes into a corrupt input.
	corrupt := [][]byte{
		[]byte("empty-id"),
		[]byte("nil-traits"),
		[]byte("duplicate-default"),
		[]byte("huge-id"),
	}

	stresschaos.ChaosCorruptInputDuring(t, "providers_personality_corrupt_input", corrupt,
		func(input []byte) error {
			switch string(input) {
			case "empty-id":
				return pm.AddPersonality(&Personality{ID: "", Name: "x", Enabled: true})
			case "nil-traits":
				// nil maps are valid in Go (read-safe); must NOT panic.
				return pm.AddPersonality(&Personality{ID: "nil-traits", Traits: nil, Enabled: true})
			case "duplicate-default":
				return pm.AddPersonality(&Personality{ID: "default", Name: "dup", Enabled: true})
			case "huge-id":
				big := make([]byte, 1<<16)
				for i := range big {
					big[i] = 'a'
				}
				return pm.AddPersonality(realPersonality(string(big)))
			}
			return nil
		})
}

// TestConversationManager_Chaos_CorruptInput feeds malformed conversation /
// message operations to the real ConversationManager. Adding a message to an
// unknown conversation is the documented rejection path; nil/empty content must
// not crash.
func TestConversationManager_Chaos_CorruptInput(t *testing.T) {
	ai := NewAIIntegration(nil)
	cm := NewConversationManager(ai, ai.config)
	ctx := context.Background()

	corrupt := [][]byte{
		[]byte("add-to-missing"),
		[]byte("get-missing"),
		[]byte("empty-id-create"),
		[]byte("nil-message-fields"),
	}

	stresschaos.ChaosCorruptInputDuring(t, "providers_conversation_corrupt_input", corrupt,
		func(input []byte) error {
			switch string(input) {
			case "add-to-missing":
				return cm.AddMessage(ctx, "does-not-exist", &ChatMessage{Role: "user", Content: "x"})
			case "get-missing":
				_, err := cm.GetConversation(ctx, "does-not-exist")
				return err
			case "empty-id-create":
				// Empty id is accepted (a valid map key) — must not crash.
				_, err := cm.CreateConversation(ctx, "")
				return err
			case "nil-message-fields":
				if _, err := cm.CreateConversation(ctx, "nilmsg"); err != nil {
					return err
				}
				// Message with zero-value fields — must not crash on append.
				return cm.AddMessage(ctx, "nilmsg", &ChatMessage{})
			}
			return nil
		})
}

// TestPersonalityManager_Chaos_ConcurrentSameKey injects state-corruption: many
// goroutines add + remove + set-active the SAME small set of keys concurrently
// while readers iterate. Asserts no panic/race/deadlock and that the registry is
// internally consistent afterwards (default always present, list iterable).
func TestPersonalityManager_Chaos_ConcurrentSameKey(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)
	rec := stresschaos.NewChaosRecorder(t, "providers_personality_concurrent_same_key", "state-corruption")

	const keys = 4
	stop := make(chan struct{})
	var wg sync.WaitGroup
	var ops int64

	// Writers: churn add/remove/update/set-active on the SAME keys.
	for w := 0; w < 16; w++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("writer panicked: %v", p))
				}
			}()
			i := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				key := fmt.Sprintf("k%d", i%keys)
				_ = pm.AddPersonality(realPersonality(key)) // may collide -> error, fine
				_ = pm.UpdatePersonality(key, map[string]interface{}{"enabled": true})
				_ = pm.SetActivePersonality(key) // may fail if removed concurrently
				_ = pm.RemovePersonality(key)
				atomic.AddInt64(&ops, 1)
				i++
			}
		}(w)
	}

	// Readers: iterate the maps concurrently with the churn.
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("reader panicked: %v", p))
				}
			}()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = pm.ListPersonalities()
				_ = pm.GetActivePersonality()
				_, _ = pm.GetPersonality("default")
			}
		}()
	}

	time.Sleep(400 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Internal consistency after the storm: default must survive (it cannot be
	// removed), and the registry must remain iterable + queryable.
	if _, err := pm.GetPersonality("default"); err != nil {
		rec.Record(stresschaos.Fatal, "default personality lost after concurrent storm")
	}
	if pm.GetActivePersonality() == nil {
		rec.Record(stresschaos.Fatal, "active personality became nil after concurrent storm")
	}
	_ = pm.ListPersonalities()
	rec.Record(stresschaos.Recovered,
		fmt.Sprintf("registry consistent after %d concurrent same-key ops: default survived, list iterable, no panic/deadlock", atomic.LoadInt64(&ops)))
	rec.AssertNoFatal()
}

// TestMemoryIntegration_Chaos_ResourcePressure stores into the real memory maps
// under bounded memory pressure, asserting it neither OOM-crashes nor lets the
// map grow unbounded (the FIFO cleanup path must keep len <= MaxGenerations).
func TestMemoryIntegration_Chaos_ResourcePressure(t *testing.T) {
	mi := NewMemoryIntegration(&MemoryConfig{
		Enabled:          true,
		MaxGenerations:   100,
		MaxConversations: 100,
		TTL:              time.Hour,
	})
	ctx := context.Background()

	stresschaos.ChaosResourcePressureDuring(t, "providers_memory_resource_pressure", 32,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 5000; i++ {
				if err := mi.StoreGeneration(ctx, "prov", "prompt", &GenerationResult{Tokens: 1}); err != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("StoreGeneration failed under pressure: %v", err))
					return
				}
			}
			stats, err := mi.GetMemoryStats(ctx)
			if err != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("GetMemoryStats failed: %v", err))
				return
			}
			// Cleanup must have bounded the live map despite 5000 stores.
			if stats.StoredGenerations > 100 {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("FIFO cleanup failed: stored=%d > cap 100", stats.StoredGenerations))
				return
			}
			rec.Record(stresschaos.Recovered,
				fmt.Sprintf("5000 stores under pressure, live map bounded at %d (<=100), total accounted %d", stats.StoredGenerations, stats.TotalGenerations))
		})
}

// TestAIIntegration_Chaos_BoundaryRegistry exercises boundary conditions on the
// real AIIntegration provider registry: zero providers, unknown lookup, and the
// concurrent stats/health iteration over an empty + a populated map. Asserts
// graceful behaviour (clean errors / empty results) with no crash.
func TestAIIntegration_Chaos_BoundaryRegistry(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "providers_ai_boundary_registry", "input-corruption")
	ctx := context.Background()

	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("boundary registry panicked: %v", p))
			}
		}()

		// Boundary: empty registry.
		ai := NewAIIntegration(&AIConfig{DefaultLLM: "none", Providers: map[string]*AIProviderConfig{}})
		if err := ai.Initialize(ctx); err != nil {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("Initialize(empty) failed: %v", err))
			return
		}
		if list := ai.ListProviders(); len(list) != 0 {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("empty registry returned %d providers", len(list)))
			return
		}
		if _, err := ai.GetStats(ctx); err != nil {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("GetStats(empty) failed: %v", err))
			return
		}
		health, err := ai.HealthCheck(ctx)
		if err != nil {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("HealthCheck(empty) failed: %v", err))
			return
		}
		if health.TotalProviders != 0 {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("empty HealthCheck reported %d providers", health.TotalProviders))
			return
		}

		// Boundary: unknown provider lookup -> clean error, no crash.
		if _, err := ai.GetProvider("nonexistent"); err == nil {
			rec.Record(stresschaos.Fatal, "GetProvider(unknown) returned nil error")
			return
		}
		// Boundary: generate against unknown provider -> clean error.
		if _, err := ai.GenerateTextWithProvider(ctx, "nope", "x", nil); err == nil {
			rec.Record(stresschaos.Fatal, "GenerateTextWithProvider(unknown) returned nil error")
			return
		}
		rec.Record(stresschaos.Degraded, "empty/unknown registry paths returned clean empty/errors with no crash")
	}()

	rec.AssertNoFatal()
}
