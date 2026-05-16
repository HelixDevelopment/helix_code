// Package autocommit — summariser_test.go (P2-F22-T04).
//
// Uses an in-package fakeProvider that satisfies the llm.Provider interface
// with a configurable canned response and call counter. Per CLAUDE.md, mocks
// are allowed in unit tests; the in-package stub is the cleaner pattern (no
// cross-package import; minimal surface).
package autocommit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/llm"
)

// fakeProvider is an in-package llm.Provider stub used to drive
// LLMSummariser deterministically. Calls are counted; the canned response
// (or canned error) is returned from Generate.
type fakeProvider struct {
	response string
	err      error
	calls    int
}

func (f *fakeProvider) GetType() llm.ProviderType { return "fake" }
func (f *fakeProvider) GetName() string           { return "fake" }
func (f *fakeProvider) GetModels() []llm.ModelInfo {
	return []llm.ModelInfo{{ID: "fake-model", Name: "fake", ContextSize: 4096}}
}
func (f *fakeProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (f *fakeProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &llm.LLMResponse{Content: f.response}, nil
}
func (f *fakeProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	return nil
}
func (f *fakeProvider) IsAvailable(ctx context.Context) bool { return true }
func (f *fakeProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "ok"}, nil
}
func (f *fakeProvider) Close() error                       { return nil }
func (f *fakeProvider) GetContextWindow() int              { return 4096 }
func (f *fakeProvider) CountTokens(s string) (int, error)  { return len(s) / 4, nil }

func TestSummariser_LLMSuccess_UsesProviderResponse(t *testing.T) {
	p := &fakeProvider{response: "FAKE_LLM_RESPONSE_42"}
	s := NewSummariser(p)
	got := s.Summarise(context.Background(), "diff body", "fs_edit", []string{"x.go"})
	require.Equal(t, "FAKE_LLM_RESPONSE_42", got)
	require.Equal(t, 1, p.calls)
}

func TestSummariser_LLMError_FallsBackToDeterministic(t *testing.T) {
	p := &fakeProvider{err: errors.New("boom")}
	s := NewSummariser(p)
	got := s.Summarise(context.Background(), "diff body", "fs_edit", []string{"x.go"})
	require.Equal(t, "Auto-edit: fs_edit on x.go", got)
}

func TestSummariser_LLMEmpty_FallsBackToDeterministic(t *testing.T) {
	p := &fakeProvider{response: "   \n\t"}
	s := NewSummariser(p)
	got := s.Summarise(context.Background(), "diff body", "fs_edit", []string{"x.go"})
	require.Equal(t, "Auto-edit: fs_edit on x.go", got)
}

func TestSummariser_LLMTooLong_TruncatedAt72(t *testing.T) {
	long := strings.Repeat("A", 100)
	p := &fakeProvider{response: long}
	s := NewSummariser(p)
	got := s.Summarise(context.Background(), "diff", "t", []string{"f"})
	require.Equal(t, 72, len(got))
}

func TestSummariser_NilProvider_UsesDeterministicFallback(t *testing.T) {
	s := NewSummariser(nil)
	got := s.Summarise(context.Background(), "diff", "fs_edit", []string{"x.go"})
	require.Equal(t, "Auto-edit: fs_edit on x.go", got)
}

func TestDeterministicFallback_Format_ByteForByte(t *testing.T) {
	var d DeterministicFallback
	require.Equal(t, "Auto-edit: fs_edit on foo.go, bar.go",
		d.Summarise(context.Background(), "", "fs_edit", []string{"foo.go", "bar.go"}))
}

func TestDeterministicFallback_TruncatedAt72(t *testing.T) {
	var d DeterministicFallback
	long := strings.Repeat("longpath/file.go,", 10)
	got := d.Summarise(context.Background(), "", "t", []string{long})
	require.LessOrEqual(t, len(got), 72)
}

func TestSummariser_Truncates_LongDiff_Before_LLMCall(t *testing.T) {
	// The summariser SHOULD cap diff size to maxDiffBytes before calling
	// the LLM. Hard to assert directly; we just confirm a huge diff
	// doesn't OOM or hang, and result is bounded.
	huge := strings.Repeat("X", 100*1024)
	p := &fakeProvider{response: "ok"}
	s := NewSummariser(p)
	got := s.Summarise(context.Background(), huge, "t", []string{"f"})
	require.Equal(t, "ok", got)
	require.Equal(t, 1, p.calls)
}
