package helixagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbed_ParsesOpenAIShapeFromHTTPTestServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/embeddings", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		var req embeddingsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Len(t, req.Input, 2, "both inputs forwarded")

		// Return a deterministic OpenAI-shape response: a known 3-dim vector per
		// input. data is intentionally returned OUT of order to prove index
		// reassembly.
		resp := map[string]any{
			"model": "helixagent-embed",
			"data": []map[string]any{
				{"index": 1, "embedding": []float32{0, 1, 0}},
				{"index": 0, "embedding": []float32{1, 0, 0}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := New(srv.URL)
	vecs, err := p.Embed(context.Background(), []string{"first", "second"})
	require.NoError(t, err)
	require.Len(t, vecs, 2)

	// Index reassembly: input[0] -> index 0 vector, input[1] -> index 1 vector.
	assert.Equal(t, []float32{1, 0, 0}, vecs[0])
	assert.Equal(t, []float32{0, 1, 0}, vecs[1])
	assert.Len(t, vecs[0], 3, "consistent dimension")
	assert.Len(t, vecs[1], 3)

	// Orthogonal known vectors -> cosine 0.
	assert.InDelta(t, 0.0, CosineSimilarity(vecs[0], vecs[1]), 1e-9)
	// Identical vector -> cosine 1.
	assert.InDelta(t, 1.0, CosineSimilarity(vecs[0], vecs[0]), 1e-9)
}

func TestEmbedText_ReportsDimensionAndSimilarity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": []float32{1, 1, 0, 0}},
				{"index": 1, "embedding": []float32{1, 1, 0, 0}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := New(srv.URL)
	res, err := EmbedText(context.Background(), p, "hello", "hello again")
	require.NoError(t, err)
	assert.Equal(t, 4, res.Dimension, "vector dimension reported")
	assert.InDelta(t, 1.0, res.CosineToFirst, 1e-9, "identical vectors are maximally similar")
	assert.Equal(t, "hello", res.Text)
	assert.Equal(t, "hello again", res.CompareText)
}

func TestEmbed_EmptyInputIsEmptyResult(t *testing.T) {
	p := New("http://127.0.0.1:0") // never contacted
	vecs, err := p.Embed(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, vecs)
	assert.NotNil(t, vecs)
}

func TestEmbed_HTTPErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := New(srv.URL)
	_, err := p.Embed(context.Background(), []string{"x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}
