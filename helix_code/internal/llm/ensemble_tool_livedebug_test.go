//go:build ensembletooldebug

package llm

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ensemble_tool_livedebug_test.go — gated LIVE root-cause harness for the
// ensemble-WITH-TOOLS failure (build tag `ensembletooldebug` keeps it out of the
// default unit run). It mirrors the TUI's provider construction EXACTLY
// (applications/terminal_ui/env_providers.go), warm-caches, dumps per-member
// resolution state, then calls ensemble.Generate with a REAL git_status Tools
// schema (the same shape internal/agent/tool_loop.go builds) and logs each
// member's outcome.
//
// Run:
//   source /tmp/helix_keys.sh
//   go test -tags=ensembletooldebug -v -count=1 -run TestEnsembleToolLiveDebug ./internal/llm/
func TestEnsembleToolLiveDebug(t *testing.T) {
	// Mirror env_providers.go envProviderCandidates (the cloud quartet that has
	// live keys per /tmp/helix_keys.sh).
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
		t.Fatalf("LIVE harness requires >=2 real providers with keys; got %d", len(members))
	}

	ens := NewEnsembleProvider(EnsembleProviderConfig{
		Members:  members,
		Strategy: "confidence_weighted",
		Timeout:  90 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// 1) WarmCache exactly as the TUI does, and wait for it (WarmCache is blocking
	//    in-body; the TUI runs it in a goroutine but here we want it complete).
	fmt.Println("================ WARM CACHE ================")
	wcStart := time.Now()
	ens.WarmCache(ctx)
	fmt.Printf("WarmCache completed in %s\n", time.Since(wcStart))

	dumpState := func(label string) {
		fmt.Printf("---------------- %s ----------------\n", label)
		for _, mr := range ens.MemberResolutions(ctx) {
			fmt.Printf("[%s] type=%s verifierModel=%q cached=%q\n",
				mr.ProviderName, mr.ProviderType, mr.VerifierModel, mr.Cached)
			fmt.Printf("    candidates(%d): %v\n", len(mr.Candidates), mr.Candidates)
		}
		fmt.Printf("DeadModelCount : %v\n", ens.DeadModelCount())
		fmt.Printf("MemberCallCounts: %v\n", ens.MemberCallCounts())
	}
	dumpState("STATE AFTER WARM CACHE")

	// 2) Build a REAL git_status Tools schema mirroring tool_loop.go
	//    registryToLLMTools output shape.
	gitStatusTool := Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "git_status",
			Description: "Show the working tree status of the current git repository.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	req := &LLMRequest{
		ID:    uuid.New(),
		Model: EnsembleModelName,
		Messages: []Message{
			{Role: "system", Content: "You are a coding agent. When the user asks about repository status, you MUST call the git_status tool."},
			{Role: "user", Content: "What is the current git status of this repo? Use the git_status tool."},
		},
		MaxTokens:   256,
		Temperature: 0.0,
		Tools:       []Tool{gitStatusTool},
		ToolChoice:  "auto",
	}

	fmt.Println("================ ENSEMBLE GENERATE *WITH TOOLS* ================")
	genStart := time.Now()
	resp, err := ens.Generate(ctx, req)
	fmt.Printf("Generate WITH TOOLS completed in %s\n", time.Since(genStart))
	if err != nil {
		fmt.Printf("GENERATE ERROR: %v\n", err)
	} else {
		fmt.Printf("CONTENT  : %q\n", resp.Content)
		fmt.Printf("TOOLCALLS: %d\n", len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			fmt.Printf("  toolcall[%d]: name=%s args=%v\n", i, tc.Function.Name, tc.Function.Arguments)
		}
		md := resp.ProviderMetadata
		fmt.Printf("total_providers      : %v\n", md["ensemble_total_providers"])
		fmt.Printf("successful_providers : %v\n", md["ensemble_successful_providers"])
		fmt.Printf("failed_providers     : %v\n", md["ensemble_failed_providers"])
		fmt.Printf("participants         : %v\n", md["ensemble_participants"])
		fmt.Printf("selected_provider    : %v\n", md["ensemble_selected_provider"])
		if ex, ok := md["ensemble_excerpts"].(map[string]string); ok {
			for name, e := range ex {
				fmt.Printf("  excerpt[%s] %s\n", name, e)
			}
		}
	}

	dumpState("STATE AFTER GENERATE-WITH-TOOLS")

	// 3) Per-member DIRECT probe WITH tools, bypassing the resilient loop, to
	//    isolate whether the cached/working model itself rejects a tool request.
	fmt.Println("================ PER-MEMBER DIRECT TOOL PROBE ================")
	for _, m := range members {
		ens.mu.RLock()
		cached := ens.workingModel[m.GetName()]
		ens.mu.RUnlock()
		if cached == "" {
			fmt.Printf("[%s] NO cached model — skipping direct probe\n", m.GetName())
			continue
		}
		direct := &LLMRequest{
			ID:          uuid.New(),
			Model:       cached,
			Messages:    req.Messages,
			MaxTokens:   256,
			Temperature: 0.0,
			Tools:       []Tool{gitStatusTool},
			ToolChoice:  "auto",
		}
		r, e := m.Generate(ctx, direct)
		if e != nil {
			fmt.Printf("[%s] cached=%s DIRECT-WITH-TOOLS err=%v\n", m.GetName(), cached, e)
			continue
		}
		fmt.Printf("[%s] cached=%s DIRECT-WITH-TOOLS content=%q toolcalls=%d finish=%q\n",
			m.GetName(), cached, strings.TrimSpace(r.Content), len(r.ToolCalls), r.FinishReason)
	}
	fmt.Println("=============================================================")
}
