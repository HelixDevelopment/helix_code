// Package agentbridge wires HelixCode to the real HelixAgent module
// (dev.helix.agent) via its public Go SDK. It is the consuming seam that
// proves the `replace dev.helix.agent => ../submodules/helix_agent` directive
// resolves and that a real helix_agent type can be constructed and exercised
// from inside dev.helix.code.
//
// This bridge deliberately imports a real, non-internal helix_agent package
// (the LLMsVerifier Go SDK at dev.helix.agent/pkg/sdk/go/verifier). Per
// CONST-036/037, LLMsVerifier is the single source of truth for model and
// verification metadata; routing HelixCode's verifier access through the real
// HelixAgent SDK keeps that single-source-of-truth guarantee intact rather than
// duplicating a parallel client.
package agentbridge

import (
	"context"
	"time"

	agentverifier "dev.helix.agent/pkg/sdk/go/verifier"
)

// VerifierBridge is a thin adapter around the real HelixAgent verifier SDK
// client. It exists so HelixCode code depends on this bridge rather than on the
// foreign module path directly, keeping the coupling point in one place.
type VerifierBridge struct {
	client *agentverifier.Client
	baseURL string
}

// Config configures the bridge. BaseURL points at a running LLMsVerifier
// endpoint; when empty the SDK defaults to its own localhost endpoint.
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// NewVerifierBridge constructs a bridge backed by the REAL helix_agent verifier
// SDK client (agentverifier.NewClient). No simulation, no local re-implementation.
func NewVerifierBridge(cfg Config) *VerifierBridge {
	client := agentverifier.NewClient(agentverifier.ClientConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Timeout: cfg.Timeout,
	})
	return &VerifierBridge{client: client, baseURL: cfg.BaseURL}
}

// VerifyModel forwards a verification request to the real HelixAgent verifier
// SDK and returns its real VerificationResult. The request/response types are
// the helix_agent SDK's own types — proving cross-module type interchange works.
func (b *VerifierBridge) VerifyModel(ctx context.Context, modelID, provider string, tests []string) (*agentverifier.VerificationResult, error) {
	return b.client.VerifyModel(ctx, agentverifier.VerificationRequest{
		ModelID:  modelID,
		Provider: provider,
		Tests:    tests,
	})
}

// Client exposes the underlying real SDK client for callers that need the full
// surface (health checks, batch verification, etc.).
func (b *VerifierBridge) Client() *agentverifier.Client { return b.client }
