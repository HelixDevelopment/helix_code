package rag

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"digital.vasic.rag/pkg/retriever"
)

// fakeRetriever is a test double satisfying retriever.Retriever. Fakes are
// permitted in unit tests only (§11.4.27) — production code (this package)
// never fabricates results; it delegates to a real retriever.Retriever
// supplied by the caller (dependency injection).
type fakeRetriever struct {
	called      bool
	calledQuery string
	calledOpts  retriever.Options
	docs        []retriever.Document
	err         error
}

func (f *fakeRetriever) Retrieve(_ context.Context, query string, opts retriever.Options) ([]retriever.Document, error) {
	f.called = true
	f.calledQuery = query
	f.calledOpts = opts
	return f.docs, f.err
}

func TestAdapter_DefaultOff(t *testing.T) {
	fake := &fakeRetriever{docs: []retriever.Document{{ID: "1", Content: "hello"}}}
	a := NewAdapter(fake)

	if a.Enabled() {
		t.Fatal("expected adapter to be disabled by default (CONST-040 Phase 1: default-OFF)")
	}

	docs, ran, err := a.Retrieve(context.Background(), "query", retriever.DefaultOptions())
	if err != nil {
		t.Fatalf("a disabled adapter must return a clear disabled signal, not an error; got err=%v", err)
	}
	if ran {
		t.Fatal("expected ran=false when adapter is disabled")
	}
	if docs != nil {
		t.Fatalf("expected no context/documents when disabled, got: %v", docs)
	}
	if fake.called {
		t.Fatal("underlying retriever.Retrieve must NOT be invoked while the adapter is disabled")
	}
}

func TestAdapter_EnabledDelegatesToRetriever(t *testing.T) {
	wantDocs := []retriever.Document{{ID: "doc-1", Content: "real retrieved content", Score: 0.91}}
	fake := &fakeRetriever{docs: wantDocs}
	a := NewAdapter(fake)
	a.SetEnabled(true)

	if !a.Enabled() {
		t.Fatal("expected adapter to report enabled after SetEnabled(true)")
	}

	opts := retriever.Options{TopK: 5, MinScore: 0.1}
	docs, ran, err := a.Retrieve(context.Background(), "real query", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ran {
		t.Fatal("expected ran=true when adapter is enabled and delegates")
	}
	if !fake.called {
		t.Fatal("expected the adapter to delegate to the underlying retriever.Retrieve")
	}
	if fake.calledQuery != "real query" {
		t.Fatalf("expected the exact query to be delegated, got %q", fake.calledQuery)
	}
	if !reflect.DeepEqual(fake.calledOpts, opts) {
		t.Fatalf("expected the exact options to be delegated, got %+v want %+v", fake.calledOpts, opts)
	}
	if len(docs) != 1 || docs[0].ID != "doc-1" {
		t.Fatalf("expected the real delegated documents to be returned unmodified, got: %v", docs)
	}
}

func TestAdapter_EnabledPropagatesRetrieverError(t *testing.T) {
	wantErr := errors.New("boom: real backing store failure")
	fake := &fakeRetriever{err: wantErr}
	a := NewAdapter(fake)
	a.SetEnabled(true)

	docs, ran, err := a.Retrieve(context.Background(), "q", retriever.DefaultOptions())
	if !ran {
		t.Fatal("expected ran=true: the adapter genuinely attempted delegation")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the underlying retriever error to be propagated unmodified, got %v", err)
	}
	if docs != nil {
		t.Fatalf("expected no documents alongside a propagated error, got: %v", docs)
	}
}

func TestAdapter_EnabledButNoRetrieverConfigured(t *testing.T) {
	a := NewAdapter(nil)
	a.SetEnabled(true)

	docs, ran, err := a.Retrieve(context.Background(), "q", retriever.DefaultOptions())
	if ran {
		t.Fatal("expected ran=false: nothing could actually execute with no retriever configured")
	}
	if err == nil {
		t.Fatal("expected a clear misconfiguration error when enabled but no retriever is wired")
	}
	if !errors.Is(err, ErrNoRetriever) {
		t.Fatalf("expected ErrNoRetriever, got %v", err)
	}
	if docs != nil {
		t.Fatalf("expected no documents, got: %v", docs)
	}
}

func TestAdapter_SetEnabledToggle(t *testing.T) {
	fake := &fakeRetriever{docs: []retriever.Document{{ID: "x"}}}
	a := NewAdapter(fake)

	a.SetEnabled(true)
	if !a.Enabled() {
		t.Fatal("expected Enabled() true after SetEnabled(true)")
	}

	a.SetEnabled(false)
	if a.Enabled() {
		t.Fatal("expected Enabled() false after SetEnabled(false)")
	}
	if _, ran, err := a.Retrieve(context.Background(), "q", retriever.DefaultOptions()); ran || err != nil {
		t.Fatalf("expected disabled behavior after toggling back off, got ran=%v err=%v", ran, err)
	}
}
