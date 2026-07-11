// Package rag provides HelixCode's adapter around the digital.vasic.rag
// submodule's retriever.Retriever / pipeline.Builder abstractions.
//
// # Scope (CONST-040 §HXC-118 Phase 1)
//
// The digital.vasic.rag submodule is a decoupled, project-not-aware
// leaf submodule (§11.4.28(B)): it ships the Retriever interface and
// pipeline machinery but deliberately no concrete, storage-backed
// implementation, because it cannot know HelixCode's embedding
// provider or persistence layer. Adapter is that missing, genuinely
// HelixCode-side piece — a thin wrapper that holds a caller-supplied
// retriever.Retriever (dependency injection; never a submodule
// modification per §11.4.28(B)) and gates retrieval behind an
// explicit enabled flag.
//
// Adapter is default-OFF: a freshly constructed Adapter never calls
// the underlying retriever until SetEnabled(true) is invoked
// explicitly. This is Phase 1 scope only — no request-flow wiring
// into cmd/cli exists yet (that is Phase 2, per the design doc at
// docs/research/const040_capability_model_20260712/DESIGN.md §3).
//
// ANTI-BLUFF (§11.4.6, CLAUDE.md Rule 2): Adapter never fabricates
// retrieval results. When disabled it reports a clear "did not run"
// signal (ran=false, err=nil) rather than either an error or a
// silently-empty-but-ambiguous result set. When enabled it always
// delegates to the real retriever.Retriever supplied by the caller.
package rag

import (
	"context"
	"errors"
	"sync"

	"digital.vasic.rag/pkg/retriever"
)

// ErrNoRetriever is returned when the adapter is enabled but was
// constructed without a backing retriever.Retriever (misconfiguration).
var ErrNoRetriever = errors.New("rag: adapter is enabled but no retriever.Retriever is configured")

// Adapter wraps a submodule retriever.Retriever and gates RAG
// retrieval behind an explicit, default-OFF enabled flag.
type Adapter struct {
	mu        sync.RWMutex
	retriever retriever.Retriever
	enabled   bool
}

// NewAdapter constructs a RAG adapter around the given retriever.
// r may be nil (e.g. while the concrete storage-backed retriever is
// still being wired); calling Retrieve while enabled with a nil
// retriever returns ErrNoRetriever rather than panicking.
//
// The returned Adapter is always default-OFF: Enabled() reports
// false until SetEnabled(true) is called explicitly by a caller
// (in a future phase, a --rag CLI flag or /rag on|off command).
func NewAdapter(r retriever.Retriever) *Adapter {
	return &Adapter{retriever: r}
}

// Enabled reports whether RAG retrieval is currently turned on.
func (a *Adapter) Enabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}

// SetEnabled turns RAG retrieval on or off.
func (a *Adapter) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
}

// Retrieve delegates to the underlying retriever.Retriever when the
// adapter is enabled, returning the real documents and errors it
// produces unmodified (ran=true in both the success and error case,
// since the retriever genuinely executed).
//
// When the adapter is disabled, Retrieve does NOT call the
// underlying retriever at all. It returns (nil, false, nil): no
// documents, a clear ran=false "did not run" signal, and no error —
// callers must treat this as "RAG is turned off," never as a
// failure.
//
// When the adapter is enabled but has no retriever configured,
// Retrieve returns (nil, false, ErrNoRetriever): this is a genuine
// misconfiguration (nothing could execute), distinct from the
// disabled case.
func (a *Adapter) Retrieve(
	ctx context.Context,
	query string,
	opts retriever.Options,
) ([]retriever.Document, bool, error) {
	a.mu.RLock()
	enabled := a.enabled
	r := a.retriever
	a.mu.RUnlock()

	if !enabled {
		return nil, false, nil
	}
	if r == nil {
		return nil, false, ErrNoRetriever
	}

	docs, err := r.Retrieve(ctx, query, opts)
	return docs, true, err
}
