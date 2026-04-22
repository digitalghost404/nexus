package embed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedSingleText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"nomic-embed-text","embeddings":[[0.1,0.2,0.3]]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text", server.Client())
	vec, err := client.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(vec))
	}
}

func TestEmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"nomic-embed-text","embeddings":[[0.1,0.2,0.3],[0.4,0.5,0.6]]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text", server.Client())
	vecs, err := client.EmbedBatch(context.Background(), []string{"text1", "text2"})
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != 2 {
		t.Errorf("expected 2 vectors, got %d", len(vecs))
	}
}

func TestEmbedUnavailable(t *testing.T) {
	client := NewClient("http://localhost:99999", "nomic-embed-text", &http.Client{Timeout: 1})
	_, err := client.Embed(context.Background(), "test")
	if err == nil {
		t.Error("expected error when ollama unavailable")
	}
}

func TestClientModel(t *testing.T) {
	client := NewClient("http://localhost:11434", "nomic-embed-text", &http.Client{})
	if client.Model() != "nomic-embed-text" {
		t.Errorf("expected model 'nomic-embed-text', got %s", client.Model())
	}
}
