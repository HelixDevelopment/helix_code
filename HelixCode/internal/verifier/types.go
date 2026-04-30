// Package verifier integrates LLMsVerifier as the single source of truth
// for model metadata, provider health, verification status, and scoring
// within HelixCode.
//
// It communicates with LLMsVerifier via REST API (not Go module import)
// to avoid the digital.vasic.llmprovider sibling-module dependency.
package verifier

import (
	"errors"
	"time"
)

// Sentinel errors for verifier operations.
var (
	ErrVerifierDisabled    = errors.New("verifier is disabled")
	ErrVerifierUnavailable = errors.New("verifier service is unavailable")
	ErrUsingStaleCache     = errors.New("using stale cached verifier data")
	ErrUsingFallback       = errors.New("using fallback model list")
	ErrNoSuitableModel     = errors.New("no model matches the given criteria")
)

// VerifiedModel is the unified model representation from LLMsVerifier.
type VerifiedModel struct {
	ID                    string        `json:"id"`
	Name                  string        `json:"name"`
	DisplayName           string        `json:"display_name"`
	Provider              string        `json:"provider"`
	ProviderType          string        `json:"provider_type"`
	Score                 float64       `json:"score"`
	Verified              bool          `json:"verified"`
	VerificationStatus    string        `json:"verification_status"` // pending, verified, failed, rate_limited
	ContextSize           int           `json:"context_window_tokens"`
	MaxOutputTokens       int           `json:"max_output_tokens"`
	SupportsStreaming     bool          `json:"supports_streaming"`
	SupportsTools         bool          `json:"supports_tool_use"`
	SupportsFunctions     bool          `json:"supports_functions"`
	SupportsCode          bool          `json:"supports_code_generation"`
	SupportsVision        bool          `json:"supports_vision"`
	SupportsAudio         bool          `json:"supports_audio"`
	SupportsVideo         bool          `json:"supports_video"`
	SupportsReasoning     bool          `json:"supports_reasoning"`
	SupportsEmbeddings    bool          `json:"supports_embeddings"`
	SupportsJSONMode      bool          `json:"supports_json_mode"`
	Latency               time.Duration `json:"latency_ms"`
	CostPerInputToken     float64       `json:"input_token_cost"`
	CostPerOutputToken    float64       `json:"output_token_cost"`
	OverallScore          float64       `json:"overall_score"`
	CodeCapabilityScore   float64       `json:"code_capability_score"`
	ResponsivenessScore   float64       `json:"responsiveness_score"`
	ReliabilityScore      float64       `json:"reliability_score"`
	FeatureRichnessScore  float64       `json:"feature_richness_score"`
	ValuePropositionScore float64       `json:"value_proposition_score"`
	LastVerified          time.Time     `json:"last_verified"`
	Source                string        `json:"source"` // "verifier", "cache", "fallback"
	OpenSource            bool          `json:"open_source"`
	Deprecated            bool          `json:"deprecated"`
	Tier                  int           `json:"tier"` // 1=Premium, 2=High-quality, 3=Fast, 4=Aggregator, 5=Free
	Capabilities          []string      `json:"capabilities"`
	Tags                  []string      `json:"tags"`
}

// ProviderStatus represents health and score of a provider.
type ProviderStatus struct {
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	DisplayName string        `json:"display_name"`
	Score       float64       `json:"score"`
	Verified    bool          `json:"verified"`
	Healthy     bool          `json:"healthy"`
	Status      string        `json:"status"` // unknown, healthy, degraded, unhealthy, offline
	ModelCount  int           `json:"model_count"`
	Tier        int           `json:"tier"`
	Priority    int           `json:"priority"`
	LastChecked time.Time     `json:"last_checked"`
	UptimePct   float64       `json:"uptime_pct"`
	Latency     time.Duration `json:"latency"`
}

// VerificationResult is the result of on-demand verification.
type VerificationResult struct {
	ModelID                 string    `json:"model_id"`
	Status                  string    `json:"status"` // started, completed, failed
	OverallScore            float64   `json:"overall_score"`
	CodeCapabilityScore     float64   `json:"code_capability_score"`
	ResponsivenessScore     float64   `json:"responsiveness_score"`
	ReliabilityScore        float64   `json:"reliability_score"`
	FeatureRichnessScore    float64   `json:"feature_richness_score"`
	ValuePropositionScore   float64   `json:"value_proposition_score"`
	ModelExists             *bool     `json:"model_exists,omitempty"`
	Responsive              *bool     `json:"responsive,omitempty"`
	Overloaded              *bool     `json:"overloaded,omitempty"`
	SupportsToolUse         bool      `json:"supports_tool_use"`
	SupportsCodeGeneration  bool      `json:"supports_code_generation"`
	SupportsEmbeddings      bool      `json:"supports_embeddings"`
	SupportsStreaming       bool      `json:"supports_streaming"`
	SupportsJSONMode        bool      `json:"supports_json_mode"`
	SupportsReasoning       bool      `json:"supports_reasoning"`
	CodeDebugging           bool      `json:"code_debugging"`
	CodeOptimization        bool      `json:"code_optimization"`
	TestGeneration          bool      `json:"test_generation"`
	DocumentationGeneration bool      `json:"documentation_generation"`
	ArchitectureDesign      bool      `json:"architecture_design"`
	SecurityAssessment      bool      `json:"security_assessment"`
	PatternRecognition      bool      `json:"pattern_recognition"`
	Error                   string    `json:"error,omitempty"`
	CompletedAt             time.Time `json:"completed_at"`
}

// HealthResponse represents verifier service health.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// RateLimitStatus represents rate limit information for a model.
type RateLimitStatus struct {
	ModelID string       `json:"model_id"`
	Limits  []LimitEntry `json:"limits"`
}

// LimitEntry is a single rate limit dimension.
type LimitEntry struct {
	Type      string    `json:"type"`
	Limit     int       `json:"limit"`
	Used      int       `json:"used"`
	Remaining int       `json:"remaining"`
	ResetTime time.Time `json:"reset_time"`
}

// CooldownInfo represents cooldown state for a model or provider.
type CooldownInfo struct {
	ModelID   string        `json:"model_id"`
	Provider  string        `json:"provider"`
	Reason    string        `json:"reason"` // rate-limited, quota-exceeded, cooldown
	ResetTime time.Time     `json:"reset_time"`
	Duration  time.Duration `json:"duration"`
}

// ChangeEvent is emitted when verifier data changes.
type ChangeEvent struct {
	Type      string         `json:"type"` // model.discovered, model.score_changed, model.status_changed, model.removed
	Model     *VerifiedModel `json:"model,omitempty"`
	Provider  *ProviderStatus `json:"provider,omitempty"`
	OldScore  float64        `json:"old_score,omitempty"`
	OldStatus string         `json:"old_status,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// FallbackModels is defined in fallback_models.go.
