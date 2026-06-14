package llm

import "context"

// Embedder is an OPTIONAL capability interface a provider MAY implement to turn
// text into dense vector embeddings. It is deliberately kept SEPARATE from the
// core Provider interface so that the vast majority of providers (which only do
// chat completion) are unaffected: a provider opts in simply by adding an Embed
// method, and a caller discovers the capability with a type assertion:
//
//	if e, ok := provider.(llm.Embedder); ok {
//	    vecs, err := e.Embed(ctx, []string{"some text"})
//	}
//
// Implementations MUST return one vector per input string, in input order, all
// of the SAME dimension. An empty input slice returns an empty (non-nil) result
// and a nil error.
type Embedder interface {
	// Embed returns one embedding vector per input string, in order. All
	// returned vectors share the same dimension. The error is non-nil only on a
	// transport/decoding/remote failure.
	Embed(ctx context.Context, input []string) ([][]float32, error)
}
