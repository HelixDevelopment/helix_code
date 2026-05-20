package fastapply

import (
	"context"
	"testing"

	"dev.helix.code/internal/llm/routing"
)

// staticResolver is a minimal routing.ModelResolver for unit tests. Mocks
// are permitted only in unit tests (CONST-050(A)); this lives in a _test.go
// file and is never imported by production code.
type staticResolver struct {
	small    string
	frontier string
}

func (r staticResolver) ResolveModel(_ context.Context, tier routing.ModelTier) (string, error) {
	if tier == routing.TierSmall {
		return r.small, nil
	}
	return r.frontier, nil
}

// TestApply_ThroughRouter proves the fast-apply path integrates with the
// P3-T01 routing.Router: the apply is dispatched through the router, the
// routing log captures the model used, and the result is still
// byte-verified against the reference apply.
func TestApply_ThroughRouter(t *testing.T) {
	// The policy maps the fast-apply task class to the small tier — file
	// apply is a mechanical transformation suited to the small/specialised
	// tier. A production caller declares this the same way when wiring.
	router, err := routing.NewRouter(
		routing.NewPolicy(routing.WithTierOverride(TaskClassFastApply, routing.TierSmall)),
		staticResolver{small: "fast-apply-small-v1", frontier: "frontier-v1"},
	)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	// The routed generate func: a confident small-model apply call. The
	// actual byte transformation is the speculative fast func wired below.
	applyGen := func(_ context.Context, modelID string, tier routing.ModelTier) (routing.Result, error) {
		return routing.Result{Content: "", Confidence: 1.0}, nil
	}

	a := NewApplier(DefaultConfig(), SpeculativeFastEditFunc()).
		WithRouting(router, applyGen)

	ctx := context.Background()
	for _, tc := range editCorpus() {
		t.Run(tc.name, func(t *testing.T) {
			refBytes, refErr := ReferenceApply(tc.instr, []byte(tc.original))
			if refErr != nil {
				t.Fatalf("reference apply error: %v", refErr)
			}
			out, err := a.Apply(ctx, tc.instr, []byte(tc.original))
			if err != nil {
				t.Fatalf("Apply error: %v", err)
			}
			if string(out.Content) != string(refBytes) {
				t.Fatalf("routed fast apply != reference\n ref:  %q\n got:  %q", string(refBytes), string(out.Content))
			}
			if !out.UsedFast() {
				t.Fatalf("expected fast route, got %s", out.Route)
			}
			if out.ModelID != "fast-apply-small-v1" {
				t.Fatalf("expected small-tier model, got %q", out.ModelID)
			}
		})
	}

	// The routing log must record one fast_apply event per corpus entry on
	// the small tier — the captured model-used evidence.
	log := router.Log()
	if len(log) != len(editCorpus()) {
		t.Fatalf("expected %d routing events, got %d", len(editCorpus()), len(log))
	}
	for i, ev := range log {
		if ev.Class != TaskClassFastApply {
			t.Fatalf("event %d: class = %q, want %q", i, ev.Class, TaskClassFastApply)
		}
		if ev.Tier != routing.TierSmall {
			t.Fatalf("event %d: tier = %s, want small", i, ev.Tier)
		}
	}
	stats := router.Stats()
	if stats.SmallTier != len(editCorpus()) {
		t.Fatalf("expected all applies on small tier, got %d", stats.SmallTier)
	}
}

// TestApply_RouterContentCarriesEditedFile proves the alternative routed
// mode where the routed model's Result.Content carries the edited file
// directly (no separate fast func). The Applier still byte-verifies it.
func TestApply_RouterContentCarriesEditedFile(t *testing.T) {
	router, err := routing.NewRouter(
		routing.NewPolicy(routing.WithTierOverride(TaskClassFastApply, routing.TierSmall)),
		staticResolver{small: "apply-model", frontier: "frontier"},
	)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	original := []byte("func old() {}\n")
	instr := &Instruction{Hunks: []Hunk{{Kind: EditReplace, Search: "old", Replace: "new"}}}
	want, _ := ReferenceApply(instr, original)

	// The routed model returns the (correct) edited file in Result.Content.
	applyGen := func(_ context.Context, _ string, _ routing.ModelTier) (routing.Result, error) {
		return routing.Result{Content: string(want), Confidence: 1.0}, nil
	}

	// No direct fast func — the routed Result.Content is the candidate.
	a := NewApplier(DefaultConfig(), nil).WithRouting(router, applyGen)

	out, err := a.Apply(context.Background(), instr, original)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if string(out.Content) != string(want) {
		t.Fatalf("routed-content apply wrong\n want: %q\n got:  %q", string(want), string(out.Content))
	}
	if !out.UsedFast() {
		t.Fatalf("expected fast route, got %s", out.Route)
	}
}

// TestApply_RouterContentWrongFallsBack proves that when the routed model's
// Result.Content is a WRONG edited file, the Applier rejects it via byte
// verification and ships the reference apply instead.
func TestApply_RouterContentWrongFallsBack(t *testing.T) {
	router, err := routing.NewRouter(
		routing.NewPolicy(routing.WithTierOverride(TaskClassFastApply, routing.TierSmall)),
		staticResolver{small: "apply-model", frontier: "frontier"},
	)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	original := []byte("func old() {}\n")
	instr := &Instruction{Hunks: []Hunk{{Kind: EditReplace, Search: "old", Replace: "new"}}}
	want, _ := ReferenceApply(instr, original)

	// The routed model hallucinates a wrong file.
	applyGen := func(_ context.Context, _ string, _ routing.ModelTier) (routing.Result, error) {
		return routing.Result{Content: "func WRONG() { /* hallucinated */ }\n", Confidence: 1.0}, nil
	}
	a := NewApplier(DefaultConfig(), nil).WithRouting(router, applyGen)

	out, err := a.Apply(context.Background(), instr, original)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if string(out.Content) != string(want) {
		t.Fatalf("wrong routed content was shipped\n want: %q\n got:  %q", string(want), string(out.Content))
	}
	if out.UsedFast() {
		t.Fatal("hallucinated routed content must NOT ship — expected reference fallback")
	}
	if out.Fallback != FallbackByteMismatch {
		t.Fatalf("expected FallbackByteMismatch, got %s", out.Fallback)
	}
}
