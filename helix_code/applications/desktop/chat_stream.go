package main

import (
	"context"

	"dev.helix.code/internal/llm"
)

// consumeDesktopChatStream drives one desktop-chat turn over the provider's
// streaming API (P1-T07, speed programme Phase 1).
//
// It calls provider.GenerateStream and invokes onChunk for every non-empty
// chunk the instant it arrives — so the caller can render token-by-token
// instead of buffering the whole reply. The provider error (if any) is
// returned.
//
// This helper carries NO build tag (unlike main.go, which is `!nogui`) so the
// streaming-consumption logic is unit-testable without an X11 display: a test
// supplies a fake llm.Provider plus a recording onChunk and asserts that N
// chunks produce N incremental callbacks before the stream completes.
//
// Channel-close robustness: the llm.Provider streaming contract is not uniform
// (Anthropic/OpenAI/Groq/DeepSeek close the channel from GenerateStream;
// Ollama and the OpenAI-compatible provider do not). drainDesktopProviderStream
// selects on both the chunk channel and the provider's return signal so it
// terminates for either provider family — a naive `for range` would deadlock
// against the non-closing providers.
//
// No-regression: the concatenation of every onChunk argument is byte-identical
// to the buffered Generate result for any conformant provider — each provider's
// GenerateStream emits the same total content as Generate. Only WHEN the bytes
// appear changes, not WHAT appears.
func consumeDesktopChatStream(ctx context.Context, provider llm.Provider, request *llm.LLMRequest, onChunk func(content string)) error {
	chunkChan := make(chan llm.LLMResponse, 100)
	errCh := make(chan error, 1)
	go func() { errCh <- provider.GenerateStream(ctx, request, chunkChan) }()

	return drainDesktopProviderStream(chunkChan, errCh, func(chunk llm.LLMResponse) {
		if chunk.Content != "" {
			onChunk(chunk.Content)
		}
	})
}

// drainDesktopProviderStream consumes every chunk a provider's GenerateStream
// emits onto chunkChan, invoking onChunk for each, and returns the provider's
// error.
//
// It copes with the non-uniform channel-close contract across llm.Provider
// implementations: some close chunkChan from inside GenerateStream, some return
// without closing. It selects on both chunkChan and errCh — on channel close it
// joins errCh; on an errCh send it drains the remaining buffered chunks
// non-blockingly and returns. Terminates for both provider families.
func drainDesktopProviderStream(chunkChan chan llm.LLMResponse, errCh chan error, onChunk func(llm.LLMResponse)) error {
	for {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				return <-errCh
			}
			onChunk(chunk)
		case provErr := <-errCh:
			for {
				select {
				case chunk, ok := <-chunkChan:
					if !ok {
						return provErr
					}
					onChunk(chunk)
				default:
					return provErr
				}
			}
		}
	}
}
