package memory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleEmbeddingProviderEmbed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":1,"embedding":[0.3,0.4]},{"index":0,"embedding":[0.1,0.2]}]}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleEmbeddingProvider()
	vectors, err := provider.Embed(context.Background(), EmbeddingProfile{
		BaseURL:    server.URL + "/v1",
		APIKey:     "test-key",
		Model:      "embedding-model",
		Dimensions: 2,
	}, []string{"first", "second"})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if len(vectors) != 2 || vectors[0][0] != 0.1 || vectors[1][0] != 0.3 {
		t.Fatalf("Embed() vectors = %#v", vectors)
	}
}

func TestVectorLiteral(t *testing.T) {
	if got := vectorLiteral([]float64{0.25, -1.5}); got != "[0.25,-1.5]" {
		t.Fatalf("vectorLiteral() = %q", got)
	}
}
