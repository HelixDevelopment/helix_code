package rag

import (
	"context"
	"errors"
	"testing"

	"digital.vasic.rag/pkg/retriever"
)

// deterministicEmbedder is a UNIT-TEST-ONLY fake (§11.4.27 — mocks/fakes
// permitted only in unit tests). It stands in for the real OllamaEmbedder
// so InMemoryVectorStore's search-ranking logic (cosine similarity, TopK,
// MinScore, dimension-mismatch handling) can be tested deterministically
// without a live Ollama instance. Production code (embedder.go,
// vectorstore.go, config.go) never references this type — the production
// path always uses the real OllamaEmbedder, proven by embedder_test.go's
// real-HTTP-round-trip tests.
type deterministicEmbedder struct {
	vectors map[string][]float64
	err     error
}

func (d *deterministicEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	if d.err != nil {
		return nil, d.err
	}
	v, ok := d.vectors[text]
	if !ok {
		return nil, errors.New("deterministicEmbedder: no fixture vector for text")
	}
	return v, nil
}

func TestInMemoryVectorStore_EmptyStoreReturnsNoDocsNoError(t *testing.T) {
	embedder := &deterministicEmbedder{vectors: map[string][]float64{"q": {1, 0}}}
	store := NewInMemoryVectorStore(embedder)

	docs, err := store.Retrieve(context.Background(), "q", retriever.DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error on empty store: %v", err)
	}
	if docs != nil {
		t.Fatalf("expected no documents from an empty store, got: %v", docs)
	}
	// An empty store must not even attempt to embed the query — nothing
	// to compare against.
}

func TestInMemoryVectorStore_AddAndRetrieveRanksBySimilarity(t *testing.T) {
	embedder := &deterministicEmbedder{vectors: map[string][]float64{
		"exact match content": {1, 0, 0},
		"orthogonal content":  {0, 1, 0},
		"query":               {1, 0, 0},
	}}
	store := NewInMemoryVectorStore(embedder)

	if err := store.AddDocument(context.Background(), retriever.Document{ID: "a", Content: "exact match content", Source: "doc-a"}); err != nil {
		t.Fatalf("AddDocument a failed: %v", err)
	}
	if err := store.AddDocument(context.Background(), retriever.Document{ID: "b", Content: "orthogonal content", Source: "doc-b"}); err != nil {
		t.Fatalf("AddDocument b failed: %v", err)
	}
	if store.Len() != 2 {
		t.Fatalf("expected 2 stored documents, got %d", store.Len())
	}

	docs, err := store.Retrieve(context.Background(), "query", retriever.Options{TopK: 5, MinScore: 0})
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 ranked documents, got %d: %v", len(docs), docs)
	}
	if docs[0].ID != "a" {
		t.Fatalf("expected the exact-match document ranked first, got %q first", docs[0].ID)
	}
	if docs[0].Score <= docs[1].Score {
		t.Fatalf("expected doc a's score (%v) > doc b's score (%v)", docs[0].Score, docs[1].Score)
	}
	if docs[0].Score < 0.99 {
		t.Fatalf("expected near-1.0 cosine similarity for the exact-match vector, got %v", docs[0].Score)
	}
	if docs[0].Content != "exact match content" {
		t.Fatalf("expected the REAL stored content returned, got %q", docs[0].Content)
	}
}

func TestInMemoryVectorStore_MinScoreFiltersLowRankedDocs(t *testing.T) {
	embedder := &deterministicEmbedder{vectors: map[string][]float64{
		"a": {1, 0},
		"b": {0, 1},
		"q": {1, 0},
	}}
	store := NewInMemoryVectorStore(embedder)
	if err := store.AddDocument(context.Background(), retriever.Document{ID: "a", Content: "a"}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := store.AddDocument(context.Background(), retriever.Document{ID: "b", Content: "b"}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	docs, err := store.Retrieve(context.Background(), "q", retriever.Options{TopK: 10, MinScore: 0.5})
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if len(docs) != 1 || docs[0].ID != "a" {
		t.Fatalf("expected only doc a to pass MinScore=0.5, got: %v", docs)
	}
}

func TestInMemoryVectorStore_TopKTruncates(t *testing.T) {
	embedder := &deterministicEmbedder{vectors: map[string][]float64{
		"a": {1, 0}, "b": {0.9, 0.1}, "c": {0.8, 0.2}, "q": {1, 0},
	}}
	store := NewInMemoryVectorStore(embedder)
	for _, id := range []string{"a", "b", "c"} {
		if err := store.AddDocument(context.Background(), retriever.Document{ID: id, Content: id}); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}
	docs, err := store.Retrieve(context.Background(), "q", retriever.Options{TopK: 2, MinScore: 0})
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected TopK=2 to truncate to 2 docs, got %d", len(docs))
	}
}

func TestInMemoryVectorStore_AddDocumentPropagatesEmbedderError(t *testing.T) {
	embedder := &deterministicEmbedder{err: errors.New("boom: ollama unreachable")}
	store := NewInMemoryVectorStore(embedder)
	if err := store.AddDocument(context.Background(), retriever.Document{ID: "a", Content: "x"}); err == nil {
		t.Fatal("expected AddDocument to propagate the real embedder error")
	}
}

func TestInMemoryVectorStore_AddDocumentRejectsEmptyID(t *testing.T) {
	embedder := &deterministicEmbedder{vectors: map[string][]float64{"x": {1}}}
	store := NewInMemoryVectorStore(embedder)
	if err := store.AddDocument(context.Background(), retriever.Document{ID: "", Content: "x"}); err == nil {
		t.Fatal("expected AddDocument to reject a document with an empty ID")
	}
}

func TestInMemoryVectorStore_RetrievePropagatesEmbedderError(t *testing.T) {
	working := &deterministicEmbedder{vectors: map[string][]float64{"a": {1, 0}}}
	store := NewInMemoryVectorStore(working)
	if err := store.AddDocument(context.Background(), retriever.Document{ID: "a", Content: "a"}); err != nil {
		t.Fatalf("setup AddDocument failed: %v", err)
	}

	// Swap in a failing embedder for the query-embedding step (same-package
	// test, direct field access) to prove Retrieve propagates a REAL
	// embedder error rather than silently returning stale/fabricated
	// results.
	store.embedder = &deterministicEmbedder{err: errors.New("boom: query embed failed")}

	if _, err := store.Retrieve(context.Background(), "q", retriever.DefaultOptions()); err == nil {
		t.Fatal("expected Retrieve to propagate the real embedder error")
	}
}

func TestInMemoryVectorStore_DimensionMismatchSkipsChunkNotFail(t *testing.T) {
	embedder := &deterministicEmbedder{vectors: map[string][]float64{
		"a": {1, 0, 0}, // 3-dim (e.g. embedded with a different model)
		"b": {1, 0},    // 2-dim
		"q": {1, 0},    // 2-dim query
	}}
	store := NewInMemoryVectorStore(embedder)
	if err := store.AddDocument(context.Background(), retriever.Document{ID: "a", Content: "a"}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := store.AddDocument(context.Background(), retriever.Document{ID: "b", Content: "b"}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	docs, err := store.Retrieve(context.Background(), "q", retriever.DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 || docs[0].ID != "b" {
		t.Fatalf("expected only the dimension-matching doc b, got: %v", docs)
	}
}

func TestInMemoryVectorStore_NilEmbedderReturnsErrorNotPanic(t *testing.T) {
	store := NewInMemoryVectorStore(nil)
	if err := store.AddDocument(context.Background(), retriever.Document{ID: "a", Content: "x"}); err == nil {
		t.Fatal("expected AddDocument with a nil embedder to error, not panic")
	}
	if _, err := store.Retrieve(context.Background(), "q", retriever.DefaultOptions()); err == nil {
		t.Fatal("expected Retrieve with a nil embedder to error, not panic")
	}
}

// TestInMemoryVectorStore_ImplementsRetrieverInterface is a compile-time +
// runtime proof that InMemoryVectorStore satisfies retriever.Retriever, so
// it can be handed directly to rag.NewAdapter (Phase 1) without any
// adapter shim.
func TestInMemoryVectorStore_ImplementsRetrieverInterface(t *testing.T) {
	var _ retriever.Retriever = NewInMemoryVectorStore(&deterministicEmbedder{})
}
