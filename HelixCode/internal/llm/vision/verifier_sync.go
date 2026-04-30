package vision

import (
	"context"
	"fmt"
	"time"

	"dev.helix.code/internal/verifier"
)

// SyncWithVerifier queries the LLMsVerifier for vision-capable models and
// populates the registry. Models already in the registry are updated;
// new models are added. This is the single point where vision model data
// transitions from hardcoded fallback to verifier-authoritative.
//
// Authority: CONST-036, CONST-041
func (r *ModelRegistry) SyncWithVerifier(ctx context.Context, adapter *verifier.Adapter) error {
	if adapter == nil || !adapter.IsEnabled() {
		return fmt.Errorf("verifier adapter not available")
	}

	models, err := adapter.GetVerifiedModels(ctx)
	if err != nil && err != verifier.ErrUsingFallback {
		return fmt.Errorf("failed to fetch verified models: %w", err)
	}

	for _, vm := range models {
		if !vm.SupportsVision {
			continue
		}

		model := convertVerifiedModelToVisionModel(vm)
		r.Register(model)
	}

	return nil
}

// convertVerifiedModelToVisionModel transforms a verifier VerifiedModel into
// a vision.Model. Vision-specific defaults are used when the verifier does
// not provide them.
func convertVerifiedModelToVisionModel(vm *verifier.VerifiedModel) *Model {
	// Default vision parameters (reasonable defaults for most vision models)
	maxImageSize := int64(20 * 1024 * 1024) // 20MB
	maxImages := 10
	supportedFormats := []string{"jpg", "jpeg", "png", "gif", "webp"}

	// Provider-specific adjustments
	switch vm.Provider {
	case "anthropic":
		maxImageSize = 10 * 1024 * 1024
		maxImages = 20
	case "gemini", "google":
		maxImageSize = 4 * 1024 * 1024
		maxImages = 16
	case "qwen":
		maxImageSize = 10 * 1024 * 1024
		maxImages = 8
		supportedFormats = []string{"jpg", "jpeg", "png"}
	}

	return &Model{
		ID:       vm.ID,
		Name:     vm.DisplayName,
		Provider: vm.Provider,
		Capabilities: &Capabilities{
			SupportsVision:   vm.SupportsVision,
			MaxImageSize:     maxImageSize,
			MaxImages:        maxImages,
			SupportedFormats: supportedFormats,
			ContextWindow:    vm.ContextSize,
			OutputTokens:     vm.MaxOutputTokens,
			FunctionCalling:  vm.SupportsTools || vm.SupportsFunctions,
			StreamingSupport: vm.SupportsStreaming,
		},
		Metadata: &ModelMetadata{
			Version:    "verifier",
			Released:   time.Now().UTC(),
			Deprecated: vm.Deprecated,
			Pricing: &Pricing{
				InputCost:  vm.CostPerInputToken * 1e6,
				OutputCost: vm.CostPerOutputToken * 1e6,
			},
		},
	}
}
