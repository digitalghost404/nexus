package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedSingleText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"nomic-embed-text","embeddings":[[0.1,0.2,0.3]]}`))
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
		_, _ = w.Write([]byte(`{"model":"nomic-embed-text","embeddings":[[0.1,0.2,0.3],[0.4,0.5,0.6]]}`))
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

func TestEmbedHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text", server.Client())
	_, err := client.Embed(context.Background(), "test")
	if err == nil {
		t.Error("expected error for non-200 response")
	}
}

func TestEmbedRequestFormation(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	var gotBody embedRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"nomic-embed-text","embeddings":[[0.1]]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text", server.Client())
	_, _ = client.Embed(context.Background(), "hello")

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/embed" {
		t.Errorf("expected /api/embed, got %s", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected application/json, got %s", gotContentType)
	}
	if gotBody.Model != "nomic-embed-text" {
		t.Errorf("expected model 'nomic-embed-text', got %s", gotBody.Model)
	}
	if len(gotBody.Input) != 1 || gotBody.Input[0] != "hello" {
		t.Errorf("expected input ['hello'], got %v", gotBody.Input)
	}
}
