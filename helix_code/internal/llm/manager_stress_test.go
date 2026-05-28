package llm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the in-process LLM ModelManager registry/selector.
//
// The unit under stress is the REAL *ModelManager — its RWMutex-guarded
// `providers` + `modelRegistry` maps and the register/lookup/select/capability
// paths that read and write them. No LLM network calls are made: the harness
// drives registration, model-registry lookup, capability filtering and
// criteria-based selection, all of which are pure in-process map operations.
//
// The Provider implementation used here (stressTestProvider) is a real,
// deterministic, in-process Provider — it makes no network calls, so it is a
// legitimate test-file Provider per §11.4.85 / CONST-050(A) (a real minimal
// in-process impl in a *_test.go). It is registered through the REAL registry
// API (ModelManager.RegisterProvider), so the registry logic under test is real.
//
// These tests MUST run under -race: concurrent RegisterProvider (write-lock) +
// SelectOptimalModel / GetAvailableModels / GetModelsByCapability (read-lock) is
// exactly the contention pattern that exposes map-mutation / locking defects.

// stressTestProvider is a real, deterministic, network-free Provider used to
// populate the real registry. It is NOT a mock of the system under test (the
// ModelManager) — it is concrete input data for it.
type stressTestProvider struct {
	pType  ProviderType
	pName  string
	models []ModelInfo
}

func (p *stressTestProvider) GetType() ProviderType            { return p.pType }
func (p *stressTestProvider) GetName() string                  { return p.pName }
func (p *stressTestProvider) GetModels() []ModelInfo           { return p.models }
func (p *stressTestProvider) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityCodeGeneration, CapabilityPlanning}
}
func (p *stressTestProvider) Generate(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	return nil, nil // never invoked by registry/selection stress paths
}
func (p *stressTestProvider) GenerateStream(ctx context.Context, req *LLMRequest, ch chan<- LLMResponse) error {
	return nil
}
func (p *stressTestProvider) IsAvailable(ctx context.Context) bool { return true }
func (p *stressTestProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", LastCheck: time.Now()}, nil
}
func (p *stressTestProvider) Close() error                   { return nil }
func (p *stressTestProvider) GetContextWindow() int          { return 200_000 }
func (p *stressTestProvider) CountTokens(text string) (int, error) {
	return CharBasedTokenCount(text)
}

// newStressTestProvider builds a real provider with a small deterministic model
// set carrying enough context size to clear the selection criteria below.
func newStressTestProvider(pType ProviderType, idx int) *stressTestProvider {
	return &stressTestProvider{
		pType: pType,
		pName: fmt.Sprintf("stress-%s-%d", pType, idx),
		models: []ModelInfo{
			{
				ID:           fmt.Sprintf("%s-model-a-%d", pType, idx),
				Name:         fmt.Sprintf("%s-7b", pType),
				Provider:     pType,
				ContextSize:  32768,
				MaxTokens:    8192,
				Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityPlanning},
			},
			{
				ID:           fmt.Sprintf("%s-model-b-%d", pType, idx),
				Name:         fmt.Sprintf("%s-13b", pType),
				Provider:     pType,
				ContextSize:  65536,
				MaxTokens:    8192,
				Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityDebugging},
			},
		},
	}
}

// distinct provider types used to spread legitimate concurrent registrations
// across goroutines (RegisterProvider rejects a duplicate type by design — that
// is not a bug, so each goroutine claims its own slice of the type space).
var stressProviderTypes = []ProviderType{
	ProviderTypeOpenAI, ProviderTypeAnthropic, ProviderTypeGemini, ProviderTypeVertexAI,
	ProviderTypeAzure, ProviderTypeBedrock, ProviderTypeGroq, ProviderTypeQwen,
	ProviderTypeCopilot, ProviderTypeOpenRouter, ProviderTypeCerebras, ProviderTypeXAI,
	ProviderTypeOllama, ProviderTypeLocal, ProviderTypeLlamaCpp, ProviderTypeVLLM,
	ProviderTypeLocalAI, ProviderTypeFastChat, ProviderTypeTextGen, ProviderTypeLMStudio,
	ProviderTypeJan, ProviderTypeKoboldAI, ProviderTypeGPT4All, ProviderTypeTabbyAPI,
	ProviderTypeMLX, ProviderTypeMistralRS, ProviderTypeMemGPT, ProviderTypeCrewAI,
	ProviderTypeMistral, ProviderTypeDeepSeek, ProviderTypeCohere, ProviderTypeHuggingFace,
}

// populatedStressManager returns a real ModelManager pre-loaded with every
// stressProviderType registered, so read-path stress has a non-empty registry.
func populatedStressManager(t *testing.T) *ModelManager {
	t.Helper()
	mm := NewModelManager()
	for i, pt := range stressProviderTypes {
		if err := mm.RegisterProvider(newStressTestProvider(pt, i)); err != nil {
			t.Fatalf("setup: RegisterProvider(%s) failed: %v", pt, err)
		}
	}
	return mm
}

// codeSelectCriteria is a realistic criteria object that several registered
// models satisfy, so a nil/empty selection result is a real failure.
var codeSelectCriteria = ModelSelectionCriteria{
	TaskType:             "code_generation",
	RequiredCapabilities: []ModelCapability{CapabilityCodeGeneration},
	MaxTokens:            4096,
	QualityPreference:    "balanced",
}

// TestModelManager_Stress_SustainedSelect drives SelectOptimalModel under
// sustained load (N>=100) over a populated real registry, recording per-call
// latency. The registry holds capability-matching models, so a returned error
// is a real failure surfaced to the harness.
func TestModelManager_Stress_SustainedSelect(t *testing.T) {
	mm := populatedStressManager(t)

	stresschaos.RunSustainedLoad(t, "model_manager_sustained_select",
		stresschaos.SustainedConfig{N: 3000, MaxErrorRate: 0.0},
		func(i int) error {
			model, err := mm.SelectOptimalModel(codeSelectCriteria)
			if err != nil {
				return err
			}
			if model == nil {
				return context.DeadlineExceeded
			}
			return nil
		})
}

// TestModelManager_Stress_ConcurrentReadPaths hammers every read path
// (SelectOptimalModel, GetAvailableModels, GetModelsByCapability,
// GetProviderForModel) from N>=10 goroutines concurrently against a populated
// real registry. This is the direct regression guard for concurrent
// registry-map reads — it MUST pass clean under -race with no deadlock/leak.
func TestModelManager_Stress_ConcurrentReadPaths(t *testing.T) {
	mm := populatedStressManager(t)

	stresschaos.RunConcurrent(t, "model_manager_concurrent_reads",
		stresschaos.ConcurrencyConfig{Parallelism: 24, IterationsPerGoroutine: 250, Timeout: 20 * time.Second},
		func(g, it int) error {
			switch it % 4 {
			case 0:
				if _, err := mm.SelectOptimalModel(codeSelectCriteria); err != nil {
					return err
				}
			case 1:
				if models := mm.GetAvailableModels(); len(models) == 0 {
					return fmt.Errorf("GetAvailableModels returned empty on populated registry")
				}
			case 2:
				_ = mm.GetModelsByCapability([]ModelCapability{CapabilityCodeGeneration})
			case 3:
				pt := stressProviderTypes[g%len(stressProviderTypes)]
				modelName := fmt.Sprintf("%s-7b", pt)
				if _, err := mm.GetProviderForModel(modelName, pt); err != nil {
					return err
				}
			}
			return nil
		})
}

// TestModelManager_Stress_ConcurrentRegisterAndRead is the core write/read
// contention test: many goroutines each register their OWN disjoint slice of
// provider types (legitimate concurrent writes under write-lock) while other
// goroutines concurrently read the registry. Under -race this surfaces any
// unguarded map mutation/read on `providers` / `modelRegistry`. No deadlock,
// no leak, no race is the PASS bar.
func TestModelManager_Stress_ConcurrentRegisterAndRead(t *testing.T) {
	// Fresh empty manager so registrations are the real write workload.
	mm := NewModelManager()

	// Partition the type space so concurrent registers never collide on a type
	// (a duplicate-type register returns a benign error by design, not a bug).
	const writers = 8
	perWriter := len(stressProviderTypes) / writers

	stresschaos.RunConcurrent(t, "model_manager_concurrent_register_read",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 60, Timeout: 25 * time.Second},
		func(g, it int) error {
			if g < writers {
				// Writer goroutine: register one of its owned types (idempotent
				// after first iteration — duplicate returns a benign error which
				// we tolerate; the point is concurrent write-lock contention).
				lo := g * perWriter
				idx := lo + (it % maxInt(perWriter, 1))
				if idx < len(stressProviderTypes) {
					_ = mm.RegisterProvider(newStressTestProvider(stressProviderTypes[idx], idx))
				}
				return nil
			}
			// Reader goroutine: read the registry mid-mutation. An empty result
			// is acceptable early on (registry still filling); the assertion is
			// purely that reads never race/panic/deadlock — surfaced by -race
			// and the harness deadlock guard.
			_ = mm.GetAvailableModels()
			_ = mm.GetModelsByCapability([]ModelCapability{CapabilityCodeGeneration})
			_, _ = mm.SelectOptimalModel(codeSelectCriteria)
			return nil
		})
}

// TestModelManager_Stress_BoundaryCriteria exercises §11.4.85 boundary
// conditions for selection: empty criteria, zero MaxTokens, an impossibly large
// MaxTokens (exceeds every model context — must return the documented
// "insufficient context" no-match), and an unknown required capability. Each
// boundary must produce a deterministic, non-panicking result.
func TestModelManager_Stress_BoundaryCriteria(t *testing.T) {
	mm := populatedStressManager(t)

	cases := []struct {
		name      string
		criteria  ModelSelectionCriteria
		wantMatch bool // true => expect a model; false => expect a clean no-match error
	}{
		{"empty_criteria", ModelSelectionCriteria{}, true},
		{"zero_max_tokens", ModelSelectionCriteria{TaskType: "code_generation", MaxTokens: 0}, true},
		{"oversized_max_tokens", ModelSelectionCriteria{MaxTokens: 1 << 30}, false},
		{"unknown_capability", ModelSelectionCriteria{RequiredCapabilities: []ModelCapability{ModelCapability("nonexistent-cap")}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := mm.SelectOptimalModel(tc.criteria)
			if tc.wantMatch {
				if err != nil {
					t.Fatalf("%s: expected a match, got error: %v", tc.name, err)
				}
				if model == nil {
					t.Fatalf("%s: expected a non-nil model", tc.name)
				}
			} else {
				if err == nil {
					t.Fatalf("%s: expected a clean no-match error, got model=%v", tc.name, model)
				}
			}
		})
	}
}
