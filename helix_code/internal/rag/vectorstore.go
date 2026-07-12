// InMemoryVectorStore — the concrete, storage-backed retriever.Retriever
// implementation for CONST-040 §HXC-118 Phase 2/3.
//
// digital.vasic.rag (the submodule) ships the Retriever interface and
// pipeline machinery but deliberately no concrete implementation
// (§11.4.28(B): it cannot know HelixCode's embedding provider or
// persistence layer). InMemoryVectorStore is that missing HelixCode-side
// piece: real Ollama embeddings (via the injected Embedder) over an
// in-memory cosine-similarity index. Phase 2 scope is explicitly
// in-memory; a durable/indexed backing store is future work.
//
// ANTI-BLUFF (§11.4.6): AddDocument and Retrieve both call the injected
// real Embedder for every text they need a vector for — no
// fabricated/hash-based/random vector is ever substituted for a real
// embedding, and an empty store genuinely returns no results rather than
// synthesizing plausible-looking documents.
package rag

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"digital.vasic.rag/pkg/retriever"
)

// InMemoryVectorStore implements retriever.Retriever over an in-memory,
// cosine-similarity-ranked set of previously-added documents.
type InMemoryVectorStore struct {
	mu       sync.RWMutex
	embedder Embedder
	chunks   []storedChunk
}

type storedChunk struct {
	doc       retriever.Document
	embedding []float64
}

// NewInMemoryVectorStore constructs an empty vector store backed by the
// given Embedder (the real OllamaEmbedder in production; a fixture-backed
// fake only in unit tests per §11.4.27). A nil embedder is accepted at
// construction time and yields a clear error from AddDocument/Retrieve
// rather than a panic.
func NewInMemoryVectorStore(embedder Embedder) *InMemoryVectorStore {
	return &InMemoryVectorStore{embedder: embedder}
}

// AddDocument embeds doc.Content via the real Embedder and stores the
// resulting (document, embedding) pair for later retrieval.
func (s *InMemoryVectorStore) AddDocument(ctx context.Context, doc retriever.Document) error {
	if s.embedder == nil {
		return fmt.Errorf("rag: vector store has no embedder configured")
	}
	if doc.ID == "" {
		return fmt.Errorf("rag: document must have a non-empty ID")
	}

	embedding, err := s.embedder.Embed(ctx, doc.Content)
	if err != nil {
		return fmt.Errorf("rag: failed to embed document %q: %w", doc.ID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunks = append(s.chunks, storedChunk{doc: doc, embedding: embedding})
	return nil
}

// Len reports the number of documents currently stored.
func (s *InMemoryVectorStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.chunks)
}

// Retrieve embeds query via the real Embedder, scores every stored
// document by cosine similarity against the query embedding, and returns
// the top opts.TopK documents whose score is >= opts.MinScore, sorted by
// score descending. It implements retriever.Retriever.
//
// An empty store (nothing added yet) is not an error: it genuinely has
// nothing to retrieve, so Retrieve returns (nil, nil) without even
// attempting to embed the query — an honest "no context available"
// signal rather than a fabricated one.
func (s *InMemoryVectorStore) Retrieve(ctx context.Context, query string, opts retriever.Options) ([]retriever.Document, error) {
	if s.embedder == nil {
		return nil, fmt.Errorf("rag: vector store has no embedder configured")
	}

	s.mu.RLock()
	n := len(s.chunks)
	s.mu.RUnlock()
	if n == 0 {
		return nil, nil
	}

	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("rag: failed to embed query: %w", err)
	}

	s.mu.RLock()
	chunks := make([]storedChunk, len(s.chunks))
	copy(chunks, s.chunks)
	s.mu.RUnlock()

	scored := make([]retriever.Document, 0, len(chunks))
	for _, c := range chunks {
		score, err := cosineSimilarity(queryEmbedding, c.embedding)
		if err != nil {
			// A dimension mismatch means this chunk was embedded by a
			// different model than the current query embedding — skip it
			// rather than fabricate a score or fail the whole retrieval
			// (a genuinely-comparable subset of results is more honest
			// than either extreme).
			continue
		}
		if score < opts.MinScore {
			continue
		}
		d := c.doc
		d.Score = score
		scored = append(scored, d)
	}

	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })

	if opts.TopK > 0 && len(scored) > opts.TopK {
		scored = scored[:opts.TopK]
	}
	return scored, nil
}

// cosineSimilarity computes the cosine similarity between two real
// embedding vectors. Returns an error on dimension mismatch or a
// zero-magnitude vector (both indicate the vectors are not comparable) —
// never a guessed/clamped score.
func cosineSimilarity(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("rag: embedding dimension mismatch: %d vs %d", len(a), len(b))
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0, fmt.Errorf("rag: zero-magnitude embedding vector")
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB)), nil
}
