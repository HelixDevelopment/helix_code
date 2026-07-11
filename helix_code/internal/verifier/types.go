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
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	DisplayName        string  `json:"display_name"`
	Provider           string  `json:"provider"`
	ProviderType       string  `json:"provider_type"`
	Score              float64 `json:"score"`
	Verified           bool    `json:"verified"`
	VerificationStatus string  `json:"verification_status"` // pending, verified, failed, rate_limited
	ContextSize        int     `json:"context_window_tokens"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
	SupportsStreaming  bool    `json:"supports_streaming"`
	SupportsTools      bool    `json:"supports_tool_use"`
	SupportsFunctions  bool    `json:"supports_functions"`
	SupportsCode       bool    `json:"supports_code_generation"`
	SupportsVision     bool    `json:"supports_vision"`
	SupportsAudio      bool    `json:"supports_audio"`
	SupportsVideo      bool    `json:"supports_video"`
	SupportsReasoning  bool    `json:"supports_reasoning"`
	SupportsEmbeddings bool    `json:"supports_embeddings"`
	SupportsJSONMode   bool    `json:"supports_json_mode"`
	// CONST-040 capability flags — MCP / LSP / ACP / RAG / Skills / Plugins.
	// Mirrors the same fields on VerificationResult (see the doc comment
	// there for the false-is-"not verified", never-"actively disabled"
	// contract). Honest as-of-2026-07-12 status: no populator in this
	// codebase sets these yet (neither the embedded fallback server nor
	// the external LLMsVerifier HTTP client currently reports them), so
	// every VerifiedModel in the fallback list and every model returned by
	// the embedded server carries the zero value (false) until a real
	// verifier-side probe exists (tracked as later HXC-117 phases).
	SupportsMCP           bool          `json:"supports_mcp"`
	SupportsLSP           bool          `json:"supports_lsp"`
	SupportsACP           bool          `json:"supports_acp"`
	SupportsRAG           bool          `json:"supports_rag"`
	SupportsSkills        bool          `json:"supports_skills"`
	SupportsPlugins       bool          `json:"supports_plugins"`
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
	ModelID                string  `json:"model_id"`
	Status                 string  `json:"status"` // started, completed, failed
	OverallScore           float64 `json:"overall_score"`
	CodeCapabilityScore    float64 `json:"code_capability_score"`
	ResponsivenessScore    float64 `json:"responsiveness_score"`
	ReliabilityScore       float64 `json:"reliability_score"`
	FeatureRichnessScore   float64 `json:"feature_richness_score"`
	ValuePropositionScore  float64 `json:"value_proposition_score"`
	ModelExists            *bool   `json:"model_exists,omitempty"`
	Responsive             *bool   `json:"responsive,omitempty"`
	Overloaded             *bool   `json:"overloaded,omitempty"`
	SupportsToolUse        bool    `json:"supports_tool_use"`
	SupportsCodeGeneration bool    `json:"supports_code_generation"`
	SupportsEmbeddings     bool    `json:"supports_embeddings"`
	SupportsStreaming      bool    `json:"supports_streaming"`
	SupportsJSONMode       bool    `json:"supports_json_mode"`
	SupportsReasoning      bool    `json:"supports_reasoning"`
	// CONST-040 capability flags — MCP / LSP / ACP / RAG / Skills / Plugins.
	// Each reports whether the verified model/provider combination has been
	// confirmed (by LLMsVerifier, the CONST-036 single source of truth) to
	// support the corresponding integration surface. False/zero-value means
	// "not verified as supporting" — NEVER "verified as NOT supporting"; a
	// verifier that has not run the relevant probe MUST report false, and
	// callers MUST treat false as "fall back to config-driven behavior",
	// never as "actively disabled" (no user-facing capability may regress
	// silently because the verifier hasn't probed it yet).
	//
	// Honest as-of-2026-07-12 status: no populator in this codebase sets
	// these fields yet — client.go's VerifyModel does a bare json.Unmarshal
	// of the external LLMsVerifier HTTP response (these fields will
	// populate automatically once that service starts reporting them), and
	// embedded_server.go's handleModelDetail mirrors them from the
	// zero-valued VerifiedModel it already holds. Wiring an actual verifier
	// probe for MCP/LSP/ACP/RAG/Skills/Plugins, and wiring the subsystem
	// read-points that consume these flags, is later HXC-117/118/119 phase
	// work (see docs/research/const040_capability_model_20260712/DESIGN.md)
	// — deliberately out of scope for this additive struct-field change.
	SupportsMCP             bool      `json:"supports_mcp"`
	SupportsLSP             bool      `json:"supports_lsp"`
	SupportsACP             bool      `json:"supports_acp"`
	SupportsRAG             bool      `json:"supports_rag"`
	SupportsSkills          bool      `json:"supports_skills"`
	SupportsPlugins         bool      `json:"supports_plugins"`
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
	Type      string          `json:"type"` // model.discovered, model.score_changed, model.status_changed, model.removed
	Model     *VerifiedModel  `json:"model,omitempty"`
	Provider  *ProviderStatus `json:"provider,omitempty"`
	OldScore  float64         `json:"old_score,omitempty"`
	OldStatus string          `json:"old_status,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// FallbackModels is defined in fallback_models.go.
