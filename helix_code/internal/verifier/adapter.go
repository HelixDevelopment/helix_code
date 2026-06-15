package verifier

import (
	"context"
	"sync"
	"time"
)

// Adapter bridges LLMsVerifier scores to HelixCode's llm package.
// Replicates HelixAgent's internal/services/llmsverifier_score_adapter.go.
type Adapter struct {
	client          *Client
	cache           *Cache
	health          *HealthMonitor
	config          *AdapterConfig
	events          *EventPublisher

	providerScores  map[string]float64
	modelScores     map[string]float64
	modelCodeScores map[string]float64
	modelRelScores  map[string]float64
	mu              sync.RWMutex

	lastRefresh     time.Time
	refreshInterval time.Duration
}

// AdapterConfig wraps the application-level VerifierConfig for use within
// the verifier package.
type AdapterConfig struct {
	Enabled         bool
	Endpoint        string
	APIKey          string
	Timeout         time.Duration
	CacheTTL        time.Duration
	PollingInterval time.Duration
	Scoring         ScoringAdapterConfig
	Health          HealthAdapterConfig
	Events          EventsAdapterConfig
	Providers       map[string]ProviderAdapterConfig
}

// ScoringAdapterConfig — runtime scoring configuration.
type ScoringAdapterConfig struct {
	Weights            ScoringWeights
	ModelsDevEnabled   bool
	ModelsDevEndpoint string
	MinAcceptableScore float64
}

// HealthAdapterConfig — runtime health configuration.
type HealthAdapterConfig struct {
	CheckInterval      time.Duration
	Timeout            time.Duration
	FailureThreshold   int
	RecoveryThreshold  int
	CircuitBreaker     CircuitBreakerConfig
}

// EventsAdapterConfig — runtime events configuration.
type EventsAdapterConfig struct {
	Enabled       bool
	WebSocket     bool
	WebSocketPath string
}

// ProviderAdapterConfig — runtime per-provider configuration.
type ProviderAdapterConfig struct {
	Enabled  bool
	APIKey   string
	BaseURL  string
	Models   []string
	Priority int
}

// CircuitBreakerConfig controls automatic failover behavior.
type CircuitBreakerConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	HalfOpenTimeout time.Duration `mapstructure:"half_open_timeout"`
}

// ScoringWeights defines the 5-component weight distribution.
type ScoringWeights struct {
	CodeCapability   float64 `mapstructure:"code_capability"`
	Responsiveness   float64 `mapstructure:"responsiveness"`
	Reliability      float64 `mapstructure:"reliability"`
	FeatureRichness  float64 `mapstructure:"feature_richness"`
	ValueProposition float64 `mapstructure:"value_proposition"`
}

// NewAdapter creates the verifier adapter.
func NewAdapter(client *Client, cache *Cache, health *HealthMonitor, cfg *AdapterConfig) *Adapter {
	if cfg == nil {
		cfg = &AdapterConfig{Enabled: false}
	}
	return &Adapter{
		client:          client,
		cache:           cache,
		health:          health,
		config:          cfg,
		providerScores:  make(map[string]float64),
		modelScores:     make(map[string]float64),
		modelCodeScores: make(map[string]float64),
		modelRelScores:  make(map[string]float64),
		refreshInterval: cfg.CacheTTL,
	}
}

// IsEnabled returns true if the verifier subsystem is enabled in config.
func (a *Adapter) IsEnabled() bool {
	return a.config != nil && a.config.Enabled
}

// IsReachable returns true if the verifier service is reachable (circuit not open).
func (a *Adapter) IsReachable() bool {
	if a.health == nil {
		return false
	}
	return a.health.AllowRequest()
}

// GetModelScore returns the overall verifier score (0-10) for a model.
func (a *Adapter) GetModelScore(modelID string) (float64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	score, ok := a.modelScores[modelID]
	if ok {
		return normalizeScore(score), true
	}
	// Try cache. The cached value MUST be normalized identically to the map
	// path: without this, the same modelID could resolve to 8.5 from the map
	// and 85.0 from the cache (a 0-10 vs 0-100 scale inconsistency).
	if a.cache != nil {
		if cached, found := a.cache.GetModelScore(modelID); found {
			return normalizeScore(cached), true
		}
	}
	return 0, false
}

// normalizeScore coerces a raw verifier score onto the canonical 0-10 scale.
// Values above 10 are assumed to be on a 0-100 scale and divided by 10. This is
// the single chokepoint applied to every GetModelScore source (in-memory map
// and cache fallback) so the score scale is consistent regardless of origin.
func normalizeScore(score float64) float64 {
	if score > 10 {
		return score / 10.0
	}
	return score
}

// GetProviderScore returns the best score for any model of this provider.
func (a *Adapter) GetProviderScore(providerType string) (float64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	score, ok := a.providerScores[providerType]
	if ok {
		return normalizeScore(score), true
	}
	return 0, false
}

// GetModelCodeCapabilityScore returns the code capability score for a model.
func (a *Adapter) GetModelCodeCapabilityScore(modelID string) (float64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	score, ok := a.modelCodeScores[modelID]
	return score, ok
}

// GetModelReliabilityScore returns the reliability score for a model.
func (a *Adapter) GetModelReliabilityScore(modelID string) (float64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	score, ok := a.modelRelScores[modelID]
	return score, ok
}

// GetMinAcceptableScore returns the configured minimum score threshold.
func (a *Adapter) GetMinAcceptableScore() float64 {
	if a.config == nil || a.config.Scoring.MinAcceptableScore == 0 {
		return 6.0
	}
	return a.config.Scoring.MinAcceptableScore
}

// GetVerifiedModels returns all models from verifier, filtered by provider config.
func (a *Adapter) GetVerifiedModels(ctx context.Context) ([]*VerifiedModel, error) {
	if !a.IsEnabled() {
		return nil, ErrVerifierDisabled
	}
	if a.health != nil && !a.health.AllowRequest() {
		return a.getFallbackModels()
	}

	// Try cache first
	if a.cache != nil {
		if models, ok := a.cache.GetModels("all"); ok {
			return a.filterByProviderConfig(models), nil
		}
	}

	// Fetch from verifier
	models, err := a.client.GetModels(ctx)
	if err != nil {
		if a.health != nil {
			a.health.RecordFailure()
		}
		// Try stale cache (up to 2x TTL)
		if a.cache != nil {
			if stale, ok := a.cache.GetModelsStale("all"); ok {
				return stale, ErrUsingStaleCache
			}
		}
		return a.getFallbackModels()
	}

	if a.health != nil {
		a.health.RecordSuccess()
	}
	a.refreshScores(models)

	// Update cache
	if a.cache != nil {
		a.cache.SetModels("all", models)
	}

	return a.filterByProviderConfig(models), nil
}

// GetWorkingModels returns the subset of verifier models that are actually
// usable by the end user (D-4 / SP1 working-model funnel). A model survives iff:
//
//	present[m.Provider]                            (key-presence gate — a key for
//	                                                that provider was recognized)
//	AND m.Verified == true                         (types.go Verified)
//	AND m.VerificationStatus == "verified"         (types.go VerificationStatus)
//	AND m.OverallScore >= a.GetMinAcceptableScore()(adapter.go:175 — now APPLIED)
//	AND the provider is not disabled in config     (preserved via GetVerifiedModels)
//
// present maps a provider type (e.g. "anthropic", "openai") to whether a key
// for it was recognized at startup. A nil/empty present map drops every model
// (no key recognized ⇒ no working models — never a §11.4 / CONST-035 bluff).
//
// Anti-bluff (§11.4 / CONST-035): failed / pending / rate-limited / sub-threshold
// / no-key models are NEVER returned — they are not usable, so presenting them
// as available would be a PASS-bluff at the model-listing layer (D-2).
//
func (a *Adapter) GetWorkingModels(ctx context.Context, present map[string]bool) ([]*VerifiedModel, error) {
	models, err := a.GetVerifiedModels(ctx)
	if err != nil {
		return nil, err
	}
	minScore := a.GetMinAcceptableScore()
	working := make([]*VerifiedModel, 0, len(models))
	for _, m := range models {
		if !present[m.Provider] {
			continue // key-presence gate
		}
		if !m.Verified {
			continue
		}
		if m.VerificationStatus != "verified" {
			continue
		}
		if m.OverallScore < minScore {
			continue
		}
		working = append(working, m)
	}
	return working, nil
}

// GetProviderStatus returns the health and score status for a provider.
func (a *Adapter) GetProviderStatus(providerType string) (*ProviderStatus, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	score, hasScore := a.providerScores[providerType]
	healthy := a.health != nil && a.health.AllowRequest() && hasScore

	status := &ProviderStatus{
		Type:        providerType,
		Score:       score,
		Healthy:     healthy,
		LastChecked: a.lastRefresh,
	}
	if hasScore {
		return status, true
	}
	return status, false
}

// ForceRefresh invalidates cache and re-fetches from verifier.
func (a *Adapter) ForceRefresh(ctx context.Context) error {
	if a.cache != nil {
		a.cache.Invalidate("all")
	}
	_, err := a.GetVerifiedModels(ctx)
	return err
}

// refreshScores updates internal score maps from model list.
func (a *Adapter) refreshScores(models []*VerifiedModel) {
	a.mu.Lock()
	defer a.mu.Unlock()

	providerBest := make(map[string]float64)

	for _, m := range models {
		a.modelScores[m.ID] = m.OverallScore
		a.modelCodeScores[m.ID] = m.CodeCapabilityScore
		a.modelRelScores[m.ID] = m.ReliabilityScore

		if current, ok := providerBest[m.Provider]; !ok || m.OverallScore > current {
			providerBest[m.Provider] = m.OverallScore
		}
	}

	a.providerScores = providerBest
	a.lastRefresh = time.Now()
}

// filterByProviderConfig removes models from disabled providers.
func (a *Adapter) filterByProviderConfig(models []*VerifiedModel) []*VerifiedModel {
	if len(a.config.Providers) == 0 {
		return models
	}
	filtered := make([]*VerifiedModel, 0, len(models))
	for _, m := range models {
		if pc, ok := a.config.Providers[m.Provider]; ok && !pc.Enabled {
			continue
		}
		filtered = append(filtered, m)
	}
	return filtered
}

// getFallbackModels returns the hardcoded fallback list.
func (a *Adapter) getFallbackModels() ([]*VerifiedModel, error) {
	result := make([]*VerifiedModel, len(FallbackModels))
	for i, m := range FallbackModels {
		copy := *m
		copy.Source = "fallback"
		result[i] = &copy
	}
	return result, ErrUsingFallback
}

// EventPublisher publishes verifier change events.
type EventPublisher struct {
	subscribers []func(ChangeEvent)
	mu          sync.RWMutex
}

// NewEventPublisher creates an event publisher.
func NewEventPublisher() *EventPublisher {
	return &EventPublisher{
		subscribers: make([]func(ChangeEvent), 0),
	}
}

// Subscribe registers a callback for change events.
func (ep *EventPublisher) Subscribe(fn func(ChangeEvent)) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.subscribers = append(ep.subscribers, fn)
}

// Publish emits a change event to all subscribers.
//
// Each subscriber runs in its own goroutine and is wrapped in a recover() guard:
// a panic in one subscriber callback (e.g. a faulty third-party handler) must not
// crash the process or starve sibling subscribers. A panicking subscriber is
// isolated and dropped for that event; healthy subscribers still receive it.
func (ep *EventPublisher) Publish(event ChangeEvent) error {
	ep.mu.RLock()
	defer ep.mu.RUnlock()
	for _, fn := range ep.subscribers {
		go func(handler func(ChangeEvent)) {
			defer func() {
				_ = recover() // isolate a panicking subscriber from the process/siblings
			}()
			handler(event)
		}(fn)
	}
	return nil
}
