package llm

import (
	"context"
	"dev.helix.code/internal/verifier"
)

// VerifierModelSource bridges the verifier adapter to HelixCode's model system.
type VerifierModelSource struct {
	adapter *verifier.Adapter
}

// NewVerifierModelSource creates a model source backed by LLMsVerifier.
func NewVerifierModelSource(adapter *verifier.Adapter) *VerifierModelSource {
	return &VerifierModelSource{adapter: adapter}
}

// IsAvailable returns true if the verifier adapter is enabled and reachable.
func (s *VerifierModelSource) IsAvailable() bool {
	return s.adapter != nil && s.adapter.IsEnabled() && s.adapter.IsReachable()
}

// FetchModels retrieves verified models from the verifier.
func (s *VerifierModelSource) FetchModels(ctx context.Context) ([]*ModelInfo, error) {
	if s.adapter == nil {
		return nil, verifier.ErrVerifierDisabled
	}
	verified, err := s.adapter.GetVerifiedModels(ctx)
	if err != nil && err != verifier.ErrUsingFallback {
		return nil, err
	}
	return ConvertVerifiedToModelInfo(verified), nil
}

// ConvertVerifiedToModelInfo transforms verifier models into llm package models.
func ConvertVerifiedToModelInfo(verified []*verifier.VerifiedModel) []*ModelInfo {
	result := make([]*ModelInfo, 0, len(verified))
	for _, v := range verified {
		mi := &ModelInfo{
			ID:             v.ID,
			Name:           v.DisplayName,
			Provider:       ProviderType(v.Provider),
			ContextSize:    v.ContextSize,
			MaxTokens:      v.MaxOutputTokens,
			SupportsTools:  v.SupportsTools,
			SupportsVision: v.SupportsVision,
			Metadata: map[string]interface{}{
				"score":               v.OverallScore,
				"verified":            v.Verified,
				"verification_status": v.VerificationStatus,
				"source":              v.Source,
				"tier":                v.Tier,
				"latency_ms":          v.Latency.Milliseconds(),
				"cost_input":          v.CostPerInputToken,
				"cost_output":         v.CostPerOutputToken,
				"open_source":         v.OpenSource,
				"deprecated":          v.Deprecated,
			},
		}
		// Map capabilities
		if v.SupportsCode {
			mi.Capabilities = append(mi.Capabilities, ModelCapabilityCodeGeneration)
		}
		if v.SupportsStreaming {
			mi.Capabilities = append(mi.Capabilities, ModelCapabilityStreaming)
		}
		if v.SupportsTools || v.SupportsFunctions {
			mi.Capabilities = append(mi.Capabilities, ModelCapabilityToolUse)
		}
		if v.SupportsReasoning {
			mi.Capabilities = append(mi.Capabilities, ModelCapabilityReasoning)
		}
		result = append(result, mi)
	}
	return result
}

// ModelCapability constants for mapping.
const (
	ModelCapabilityCodeGeneration ModelCapability = "code_generation"
	ModelCapabilityStreaming      ModelCapability = "streaming"
	ModelCapabilityToolUse        ModelCapability = "tool_use"
	ModelCapabilityReasoning      ModelCapability = "reasoning"
)
