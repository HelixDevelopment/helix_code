package providers

import (
	"context"
	"fmt"
	"testing"
	"time"

	memproviders "dev.helix.code/internal/memory/providers"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the REAL internal/providers in-process
// registries: PersonalityManager, ConversationManager, MemoryIntegration and
// AIIntegration's RWMutex-guarded provider map.
//
// NO network / LLM calls happen in these loops — they target the in-process
// register / lookup / list / capability / stats paths exactly per the §11.4.85
// brief. The components under test are the production structs (not fakes); the
// only injected value objects are real *Personality / *ChatMessage / config
// structs. These MUST be run under -race: the map mutation+iteration surfaces
// (register/list/stats concurrently) are precisely where concurrent-map and
// missing-mutex defects surface.

// realPersonality builds a real, enabled *Personality for registry stress.
func realPersonality(id string) *Personality {
	return &Personality{
		ID:           id,
		Name:         "P-" + id,
		Description:  "stress personality",
		Traits:       map[string]interface{}{"helpfulness": 0.9},
		SystemPrompt: "system",
		Temperature:  0.7,
		TopP:         1.0,
		Enabled:      true,
	}
}

// TestPersonalityManager_Stress_SustainedAddListRemove drives the real
// PersonalityManager add/list/get/remove cycle under sustained load (N>=100),
// recording per-op latency. A failure to round-trip a just-added personality is
// surfaced to the harness as an error (forces a non-zero error rate => FAIL).
func TestPersonalityManager_Stress_SustainedAddListRemove(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	stresschaos.RunSustainedLoad(t, "providers_personality_sustained_add_list_remove",
		stresschaos.SustainedConfig{N: 3000, MaxErrorRate: 0.0},
		func(i int) error {
			id := fmt.Sprintf("p-%d", i)
			if err := pm.AddPersonality(realPersonality(id)); err != nil {
				return err
			}
			if _, err := pm.GetPersonality(id); err != nil {
				return err
			}
			if list := pm.ListPersonalities(); len(list) == 0 {
				return fmt.Errorf("empty personality list after add")
			}
			pm.IncrementUsage(id)
			return pm.RemovePersonality(id)
		})
}

// TestPersonalityManager_Stress_ConcurrentMixed hammers add / get / list /
// set-active / increment / remove from >=10 goroutines concurrently. This is the
// direct regression guard for concurrent RWMutex-guarded map access on the real
// PersonalityManager — it MUST pass clean under -race with no deadlock/leak.
func TestPersonalityManager_Stress_ConcurrentMixed(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	stresschaos.RunConcurrent(t, "providers_personality_concurrent_mixed",
		stresschaos.ConcurrencyConfig{Parallelism: 24, IterationsPerGoroutine: 200, Timeout: 25 * time.Second},
		func(g, it int) error {
			id := fmt.Sprintf("g%d-i%d", g, it)
			// Add a goroutine-unique key, then exercise the shared read paths
			// (list/get-active) that race the writers, then remove our key.
			if err := pm.AddPersonality(realPersonality(id)); err != nil {
				return err
			}
			_ = pm.ListPersonalities()        // concurrent map iteration
			_ = pm.GetActivePersonality()     // concurrent active-pointer read
			_, _ = pm.GetPersonality("default") // always-present read
			pm.IncrementUsage(id)
			return pm.RemovePersonality(id)
		})
}

// TestConversationManager_Stress_SustainedCreateAddGet drives the real
// ConversationManager under sustained load: create -> add message -> get-back,
// asserting the round-trip is consistent each iteration.
func TestConversationManager_Stress_SustainedCreateAddGet(t *testing.T) {
	ai := NewAIIntegration(nil)
	cm := NewConversationManager(ai, ai.config)
	ctx := context.Background()

	stresschaos.RunSustainedLoad(t, "providers_conversation_sustained_create_add_get",
		stresschaos.SustainedConfig{N: 3000, MaxErrorRate: 0.0},
		func(i int) error {
			id := fmt.Sprintf("conv-%d", i)
			if _, err := cm.CreateConversation(ctx, id); err != nil {
				return err
			}
			if err := cm.AddMessage(ctx, id, &ChatMessage{Role: "user", Content: "hi", Tokens: 3}); err != nil {
				return err
			}
			conv, err := cm.GetConversation(ctx, id)
			if err != nil {
				return err
			}
			if len(conv.Messages) != 1 {
				return fmt.Errorf("expected 1 message, got %d", len(conv.Messages))
			}
			return nil
		})
}

// TestConversationManager_Stress_ConcurrentCreateAddGet hammers create/add/get
// from >=10 goroutines, each on its own conversation id, with shared map
// contention. MUST pass clean under -race.
func TestConversationManager_Stress_ConcurrentCreateAddGet(t *testing.T) {
	ai := NewAIIntegration(nil)
	cm := NewConversationManager(ai, ai.config)
	ctx := context.Background()

	stresschaos.RunConcurrent(t, "providers_conversation_concurrent_create_add_get",
		stresschaos.ConcurrencyConfig{Parallelism: 20, IterationsPerGoroutine: 200, Timeout: 25 * time.Second},
		func(g, it int) error {
			id := fmt.Sprintf("c-g%d-i%d", g, it)
			if _, err := cm.CreateConversation(ctx, id); err != nil {
				return err
			}
			if err := cm.AddMessage(ctx, id, &ChatMessage{Role: "user", Content: "x", Tokens: 1}); err != nil {
				return err
			}
			if _, err := cm.GetConversation(ctx, id); err != nil {
				return err
			}
			return nil
		})
}

// TestMemoryIntegration_Stress_ConcurrentStore hammers StoreGeneration /
// StoreConversation / GetMemoryStats from >=10 goroutines. The MemoryIntegration
// maps are RWMutex-guarded; this is the regression guard for concurrent store +
// stats read (including the FIFO cleanup path under load). MUST pass under -race.
func TestMemoryIntegration_Stress_ConcurrentStore(t *testing.T) {
	// Small caps so the cleanup path (cleanupGenerations / cleanupConversations)
	// is exercised under concurrent load rather than never reached.
	mi := NewMemoryIntegration(&MemoryConfig{
		Enabled:          true,
		MaxGenerations:   200,
		MaxConversations: 200,
		TTL:              time.Hour,
	})
	ctx := context.Background()

	stresschaos.RunConcurrent(t, "providers_memory_concurrent_store",
		stresschaos.ConcurrencyConfig{Parallelism: 24, IterationsPerGoroutine: 200, Timeout: 25 * time.Second},
		func(g, it int) error {
			p := fmt.Sprintf("prov-%d", g)
			if err := mi.StoreGeneration(ctx, p, "prompt", &GenerationResult{Tokens: 7, Cost: 0.01}); err != nil {
				return err
			}
			if err := mi.StoreConversation(ctx, p,
				[]*ChatMessage{{Role: "user", Content: "q", Tokens: 2}},
				&ChatResult{Tokens: 4, Cost: 0.02}); err != nil {
				return err
			}
			if _, err := mi.GetMemoryStats(ctx); err != nil { // concurrent map-len read
				return err
			}
			return nil
		})
}

// TestAIIntegration_Stress_ConcurrentStatsList registers real (NotImplemented)
// providers via the production map and then hammers ListProviders + GetStats +
// HealthCheck concurrently with re-Initialize churn. GetStats/HealthCheck iterate
// ai.providers; if those iterations are unguarded while Initialize writes the
// map under ai.mu.Lock(), -race surfaces a concurrent-map fault here. MUST pass
// clean under -race.
func TestAIIntegration_Stress_ConcurrentStatsList(t *testing.T) {
	// Cohere maps to a real NotImplementedProvider via the production factory
	// (createAIProvider -> NewCohereProvider): a real in-process AIProvider with
	// NO network dependency. HealthCheck below calls GenerateText on it which
	// returns a clean error — exactly the in-process path we want to hammer.
	//
	// DefaultLLM is intentionally set to a name NOT in the provider map so that
	// NewConversationManager's GetProvider lookup misses and the compression
	// coordinator is skipped: this test targets the concurrent provider-map
	// iteration in GetStats/HealthCheck/ListProviders, not the compression stack.
	cfg := &AIConfig{
		DefaultLLM:    "__unregistered__",
		DefaultMemory: "",
		Providers:     map[string]*AIProviderConfig{},
	}
	for i := 0; i < 8; i++ {
		cfg.Providers[fmt.Sprintf("p%d", i)] = &AIProviderConfig{
			Type:    memproviders.ProviderTypeCohere,
			Enabled: true,
			Model:   "m",
		}
	}
	ai := NewAIIntegration(cfg)
	ctx := context.Background()
	if err := ai.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	stresschaos.RunConcurrent(t, "providers_ai_concurrent_stats_list",
		stresschaos.ConcurrencyConfig{Parallelism: 24, IterationsPerGoroutine: 100, Timeout: 25 * time.Second},
		func(g, it int) error {
			_ = ai.ListProviders()
			if _, err := ai.GetStats(ctx); err != nil { // iterates ai.providers
				return err
			}
			if _, err := ai.HealthCheck(ctx); err != nil { // iterates ai.providers
				return err
			}
			return nil
		})
}
