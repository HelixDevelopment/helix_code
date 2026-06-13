//go:build ensembleprobe

package llm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ensemble_provider_live_probe_test.go — gated, anti-bluff REAL ensemble proof.
//
// Build tag `ensembleprobe` keeps this out of the default unit run. It makes
// REAL HTTP calls to REAL cloud providers (no mocks, no simulation) using the
// env keys the operator has exported, then asserts the ensemble genuinely
// orchestrated >1 provider and returned a real combined response.
//
// Run:
//   source /tmp/helix_keys.sh
//   go test -tags=ensembleprobe -v -count=1 -run TestEnsembleProvider_LiveProbe ./internal/llm/
func TestEnsembleProvider_LiveProbe(t *testing.T) {
	candidates := []ProviderType{
		ProviderTypeDeepSeek,
		ProviderTypeMistral,
		ProviderTypeGroq,
		ProviderTypeOpenRouter,
	}

	var members []Provider
	for _, pt := range candidates {
		if !IsProviderKeyPresent(pt) {
			t.Logf("skip member %s: key not present", pt)
			continue
		}
		p, err := NewCloudProvider(pt, ProviderConfigEntry{Type: pt, Enabled: true})
		if err != nil {
			t.Logf("skip member %s: construct failed: %v", pt, err)
			continue
		}
		members = append(members, p)
		t.Logf("ensemble member ready: %s (%s)", p.GetName(), pt)
	}

	if len(members) < 2 {
		t.Fatalf("LIVE PROBE requires >=2 real providers with keys; got %d", len(members))
	}

	ens := NewEnsembleProvider(EnsembleProviderConfig{
		Members:  members,
		Strategy: "confidence_weighted",
		Timeout:  90 * time.Second,
	})

	req := &LLMRequest{
		ID:          uuid.New(),
		Model:       EnsembleModelName,
		Messages:    []Message{{Role: "user", Content: "Reply with exactly: OK"}},
		MaxTokens:   32,
		Temperature: 0.0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	resp, err := ens.Generate(ctx, req)
	if err != nil {
		t.Fatalf("ensemble Generate failed: %v", err)
	}

	fmt.Println("================ REAL ENSEMBLE GENERATION ================")
	fmt.Printf("COMBINED CONTENT: %q\n", resp.Content)
	md := resp.ProviderMetadata
	fmt.Printf("total_providers      : %v\n", md["ensemble_total_providers"])
	fmt.Printf("successful_providers : %v\n", md["ensemble_successful_providers"])
	fmt.Printf("failed_providers     : %v\n", md["ensemble_failed_providers"])
	fmt.Printf("participants         : %v\n", md["ensemble_participants"])
	fmt.Printf("selected_provider    : %v\n", md["ensemble_selected_provider"])
	fmt.Printf("scores               : %v\n", md["ensemble_scores"])
	fmt.Println("--- per-provider excerpts (proof each member really answered) ---")
	if ex, ok := md["ensemble_excerpts"].(map[string]string); ok {
		for name, e := range ex {
			fmt.Printf("  [%s] %s\n", name, e)
		}
	}
	fmt.Println("=========================================================")

	if resp.Content == "" {
		t.Fatalf("combined content is empty — ensemble produced no real output")
	}
	if n, _ := md["ensemble_successful_providers"].(int); n < 2 {
		t.Fatalf("ensemble must orchestrate >=2 real providers; successful=%v", md["ensemble_successful_providers"])
	}
	if parts, _ := md["ensemble_participants"].([]string); len(parts) < 2 {
		t.Fatalf("expected >=2 participating providers, got %v", md["ensemble_participants"])
	}
	if sel, _ := md["ensemble_selected_provider"].(string); sel == "" {
		t.Fatalf("ensemble must name a selected provider")
	}
}
