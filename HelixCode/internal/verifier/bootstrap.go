package verifier

import (
	"context"
	"fmt"
	"log"

	"dev.helix.code/internal/config"
)

// BootstrapResult holds all verifier components ready for injection.
type BootstrapResult struct {
	Client   *Client
	Adapter  *Adapter
	Poller   *Poller
	Config   *AdapterConfig
}

// Bootstrap creates the full verifier subsystem from application config.
// If verifier is disabled in config, returns nil, nil.
func Bootstrap(cfg *config.VerifierConfig) (*BootstrapResult, error) {
	if cfg == nil || !cfg.Enabled {
		log.Println("ℹ️  LLMsVerifier disabled in configuration")
		return nil, nil
	}

	if cfg.Mode != "remote" {
		return nil, fmt.Errorf("verifier mode %q not yet supported (only 'remote' is implemented)", cfg.Mode)
	}

	// Build adapter config from application config
	adapterCfg := &AdapterConfig{
		Enabled:         cfg.Enabled,
		Endpoint:        cfg.Endpoint,
		APIKey:          cfg.APIKey,
		Timeout:         cfg.Timeout,
		CacheTTL:        cfg.CacheTTL,
		PollingInterval: cfg.PollingInterval,
		Scoring: ScoringAdapterConfig{
			Weights: ScoringWeights{
				CodeCapability:   cfg.Scoring.Weights.CodeCapability,
				Responsiveness:   cfg.Scoring.Weights.Responsiveness,
				Reliability:      cfg.Scoring.Weights.Reliability,
				FeatureRichness:  cfg.Scoring.Weights.FeatureRichness,
				ValueProposition: cfg.Scoring.Weights.ValueProposition,
			},
			ModelsDevEnabled:   cfg.Scoring.ModelsDevEnabled,
			ModelsDevEndpoint: cfg.Scoring.ModelsDevEndpoint,
			MinAcceptableScore: cfg.Scoring.MinAcceptableScore,
		},
		Health: HealthAdapterConfig{
			CheckInterval:     cfg.Health.CheckInterval,
			Timeout:           cfg.Health.Timeout,
			FailureThreshold:  cfg.Health.FailureThreshold,
			RecoveryThreshold: cfg.Health.RecoveryThreshold,
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:         cfg.Health.CircuitBreaker.Enabled,
				HalfOpenTimeout: cfg.Health.CircuitBreaker.HalfOpenTimeout,
			},
		},
		Events: EventsAdapterConfig{
			Enabled:       cfg.Events.Enabled,
			WebSocket:     cfg.Events.WebSocket,
			WebSocketPath: cfg.Events.WebSocketPath,
		},
	}

	// Create REST client
	client := NewClient(cfg.Endpoint, cfg.APIKey, cfg.Timeout)

	// Create cache (no Redis backing yet)
	cache := NewCache(cfg.CacheTTL, nil)

	// Create health monitor (circuit breaker)
	health := NewHealthMonitor(
		cfg.Health.FailureThreshold,
		cfg.Health.RecoveryThreshold,
		cfg.Health.CircuitBreaker.HalfOpenTimeout,
	)

	// Create adapter
	adapter := NewAdapter(client, cache, health, adapterCfg)

	// Create and start poller if enabled
	var poller *Poller
	if cfg.Events.Enabled && cfg.PollingInterval > 0 {
		poller = NewPoller(adapter, cfg.PollingInterval)
		poller.Start()
		log.Printf("🔄 Verifier poller started (interval: %s)", cfg.PollingInterval)
	}

	// Perform immediate health check
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if _, err := client.Health(ctx); err != nil {
		log.Printf("⚠️  Verifier health check failed: %v (will use fallback models)", err)
		health.RecordFailure()
	} else {
		log.Printf("✅ LLMsVerifier connected: %s", cfg.Endpoint)
		health.RecordSuccess()
	}

	return &BootstrapResult{
		Client:  client,
		Adapter: adapter,
		Poller:  poller,
		Config:  adapterCfg,
	}, nil
}

// Shutdown gracefully stops the verifier subsystem.
func (r *BootstrapResult) Shutdown() {
	if r == nil {
		return
	}
	if r.Poller != nil {
		r.Poller.Stop()
		log.Println("🛑 Verifier poller stopped")
	}
}
