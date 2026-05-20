// Package autocommit — summariser.go (P2-F22-T04).
//
// MessageSummariser turns a staged diff into a short imperative-voice
// commit subject (≤72 chars). Two implementations:
//   - LLMSummariser: calls llm.Provider.Generate with a fixed prompt,
//     5-second timeout, then byte-truncates to 72 chars.
//   - DeterministicFallback: format string only — used when no provider
//     is configured, when Generate errors, or when the LLM returns empty.
//
// NewSummariser is the public constructor; it picks the fallback automatically
// when provider is nil. Errors and empty results from the LLM trigger the
// fallback chain — never propagated.
//
// The summariser does NOT redact secrets — that's secret_filter.go's job.
// committer.go runs the secret filter over the summary AFTER it's produced.
package autocommit

import (
	"context"
	"strings"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/routing"
)

// MessageSummariser is the contract the committer depends on. Pure
// function: same input → same output.
type MessageSummariser interface {
	// Summarise returns a 1-line commit subject ≤72 chars. Never
	// returns an error — fallbacks are internal.
	Summarise(ctx context.Context, diff, toolName string, paths []string) string
}

// summarisePrompt is the canonical prompt sent to the LLM. Kept short to
// minimise prompt tokens; the diff itself is truncated to maxDiffBytes
// before being appended.
const summarisePrompt = "Summarise this diff in 50-72 chars (imperative voice, no period):\n\n"

// maxDiffBytes caps the diff size sent to the LLM. Larger diffs are
// truncated to keep the prompt within sane token limits and to prevent a
// stuck LLM call from holding up every commit.
const maxDiffBytes = 8 * 1024

// maxSubjectChars is git's de-facto subject convention. Truncated, no
// ellipsis. Per spec §11 #7.
const maxSubjectChars = 72

// llmTimeout is the bounded blocking window for a single LLM call. Per
// spec §11 #6 — 5 seconds is long enough for a healthy provider, short
// enough to feel responsive when the provider is stuck.
const llmTimeout = 5 * time.Second

// LLMSummariser is the production summariser. Uses the configured
// llm.Provider with a fixed prompt and timeout.
//
// Commit-message generation is a trivial LLM subtask — the speed-programme
// model-cascade lever (Phase 3, task P3-T01). When a subtask router is
// wired, the Summarise call routes through the small/cheap model tier and
// escalates to the frontier tier only on a low-confidence small-model
// result. With no router wired the call runs unchanged on the provider's
// first model (the no-regression default).
type LLMSummariser struct {
	provider llm.Provider
	timeout  time.Duration
	router   *routing.Router
}

// DeterministicFallback is a zero-state summariser that synthesises a
// "Auto-edit: <toolName> on <comma-separated paths>" subject. Used when
// the LLM is unavailable or returns garbage.
type DeterministicFallback struct{}

// NewSummariser returns LLMSummariser when provider is non-nil, else
// DeterministicFallback. Callers should treat the returned interface
// uniformly.
func NewSummariser(p llm.Provider) MessageSummariser {
	if p == nil {
		return &DeterministicFallback{}
	}
	return &LLMSummariser{provider: p, timeout: llmTimeout}
}

// NewRoutedSummariser returns an LLMSummariser whose commit-message
// generation routes through the supplied small-model router (Phase 3,
// task P3-T01). A nil provider yields a DeterministicFallback; a nil router
// behaves identically to NewSummariser (routing disabled).
func NewRoutedSummariser(p llm.Provider, router *routing.Router) MessageSummariser {
	if p == nil {
		return &DeterministicFallback{}
	}
	return &LLMSummariser{provider: p, timeout: llmTimeout, router: router}
}

// SetRouter installs (or clears, with nil) the small-model subtask router on
// an LLMSummariser. nil-safe.
func (s *LLMSummariser) SetRouter(r *routing.Router) {
	s.router = r
}

// Summarise (LLMSummariser) calls the provider, falls back on any error
// or empty result, and truncates to 72 chars.
func (s *LLMSummariser) Summarise(ctx context.Context, diff, toolName string, paths []string) string {
	if len(diff) > maxDiffBytes {
		diff = diff[:maxDiffBytes]
	}
	cctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	models := s.provider.GetModels()
	modelID := ""
	if len(models) > 0 {
		modelID = models[0].ID
	}
	req := &llm.LLMRequest{
		Model:    modelID,
		Messages: []llm.Message{{Role: "user", Content: summarisePrompt + diff}},
	}

	// Route commit-message generation through the small-model cascade when a
	// router is wired; otherwise call the provider directly on its first
	// model (the no-regression default). Per speed-programme task P3-T01.
	var resp *llm.LLMResponse
	var err error
	if s.router != nil {
		resp, err = s.routeSummarise(cctx, req)
	} else {
		resp, err = s.provider.Generate(cctx, req)
	}
	if err != nil || resp == nil {
		return (&DeterministicFallback{}).Summarise(ctx, diff, toolName, paths)
	}
	out := strings.TrimSpace(resp.Content)
	if out == "" {
		return (&DeterministicFallback{}).Summarise(ctx, diff, toolName, paths)
	}
	if len(out) > maxSubjectChars {
		out = out[:maxSubjectChars]
	}
	return out
}

// routeSummarise dispatches the commit-message generation request through
// the subtask router. The router resolves the policy's model tier
// (TaskCommitMessage routes to the small tier) to a concrete verifier-sourced
// model, runs the generation, and escalates to the frontier model when the
// small-model result is low-confidence (truncated / content-filtered finish).
// The request's Model field is overwritten with the routed model per attempt.
func (s *LLMSummariser) routeSummarise(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	var final *llm.LLMResponse
	gen := func(ctx context.Context, modelID string, _ routing.ModelTier) (routing.Result, error) {
		req.Model = modelID
		resp, err := s.provider.Generate(ctx, req)
		if err != nil {
			return routing.Result{}, err
		}
		final = resp
		return routing.Result{
			Content:    resp.Content,
			Confidence: summariseConfidence(resp),
		}, nil
	}
	if _, err := s.router.Route(ctx, routing.TaskCommitMessage, gen); err != nil {
		return nil, err
	}
	return final, nil
}

// summariseConfidence maps a commit-message response's finish reason to a
// routing confidence. A clean stop is confident (1.0); a truncated or
// blocked finish is low-confidence (0.0) so the router escalates to the
// frontier model rather than committing a degraded subject line.
func summariseConfidence(resp *llm.LLMResponse) float64 {
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return 0.0
	}
	switch resp.FinishReason {
	case "", "stop", "end_turn", "complete", "completed":
		return 1.0
	default:
		return 0.0
	}
}

// Summarise (DeterministicFallback) returns "Auto-edit: <tool> on <paths>".
// Truncates to 72 chars without ellipsis.
//
// CONST-046: the subject literal is user-facing (it appears verbatim
// in every `git log` output when the LLM is unavailable) — resolved
// via tr() with the active locale + caller-supplied placeholders.
func (DeterministicFallback) Summarise(ctx context.Context, _, toolName string, paths []string) string {
	msg := tr(ctx, "internal_autocommit_subject_auto_edit_prefix", map[string]any{
		"ToolName": toolName,
		"Paths":    strings.Join(paths, ", "),
	})
	if len(msg) > maxSubjectChars {
		msg = msg[:maxSubjectChars]
	}
	return msg
}
