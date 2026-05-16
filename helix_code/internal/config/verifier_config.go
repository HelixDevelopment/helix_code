package config

import "time"

// VerifierConfig controls LLMsVerifier integration.
type VerifierConfig struct {
	Enabled         bool                              `mapstructure:"enabled"`
	Mode            string                            `mapstructure:"mode"`        // "remote" | "embedded"
	Endpoint        string                            `mapstructure:"endpoint"`    // REST API URL
	APIKey          string                            `mapstructure:"api_key"`     // Auth key for verifier API
	Timeout         time.Duration                     `mapstructure:"timeout"`
	CacheTTL        time.Duration                     `mapstructure:"cache_ttl"`
	PollingInterval time.Duration                     `mapstructure:"polling_interval"`
	Scoring         VerifierScoringConfig             `mapstructure:"scoring"`
	Health          VerifierHealthConfig              `mapstructure:"health"`
	Events          VerifierEventsConfig              `mapstructure:"events"`
	Providers       map[string]VerifierProviderConfig `mapstructure:"providers"`
}

// VerifierScoringConfig mirrors HelixAgent's scoring weights.
type VerifierScoringConfig struct {
	Weights            ScoringWeights `mapstructure:"weights"`
	ModelsDevEnabled   bool           `mapstructure:"models_dev_enabled"`
	ModelsDevEndpoint string        `mapstructure:"models_dev_endpoint"`
	MinAcceptableScore float64       `mapstructure:"min_acceptable_score"`
}

// ScoringWeights defines the 5-component weight distribution.
// Weights MUST sum to exactly 1.0 (validated at startup).
type ScoringWeights struct {
	CodeCapability   float64 `mapstructure:"code_capability"`   // default: 0.40
	Responsiveness   float64 `mapstructure:"responsiveness"`    // default: 0.20
	Reliability      float64 `mapstructure:"reliability"`       // default: 0.20
	FeatureRichness  float64 `mapstructure:"feature_richness"`  // default: 0.15
	ValueProposition float64 `mapstructure:"value_proposition"` // default: 0.05
}

// VerifierHealthConfig mirrors HelixAgent's circuit breaker config.
type VerifierHealthConfig struct {
	CheckInterval     time.Duration        `mapstructure:"check_interval"`
	Timeout           time.Duration        `mapstructure:"timeout"`
	FailureThreshold  int                  `mapstructure:"failure_threshold"`
	RecoveryThreshold int                  `mapstructure:"recovery_threshold"`
	CircuitBreaker    CircuitBreakerConfig `mapstructure:"circuit_breaker"`
}

// CircuitBreakerConfig controls automatic failover behavior.
type CircuitBreakerConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	HalfOpenTimeout time.Duration `mapstructure:"half_open_timeout"`
}

// VerifierEventsConfig controls event publishing.
type VerifierEventsConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	WebSocket     bool   `mapstructure:"websocket"`
	WebSocketPath string `mapstructure:"websocket_path"`
}

// VerifierProviderConfig — per-provider override.
type VerifierProviderConfig struct {
	Enabled   bool     `mapstructure:"enabled"`
	APIKey    string   `mapstructure:"api_key" json:"-" yaml:"api_key"` // json:"-" prevents serialization
	BaseURL   string   `mapstructure:"base_url"`
	Models    []string `mapstructure:"models"`
	Priority  int      `mapstructure:"priority"`
}
