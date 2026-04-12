package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPEmbedder_Embed_OK(t *testing.T) {
	var gotAuth, gotFailover, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotFailover = r.Header.Get("X-Failover-Enabled")
		gotPath = r.URL.Path
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,2]},{"index":1,"embedding":[3,4]}]}`))
	}))
	defer srv.Close()

	embedder := HTTPEmbedder{
		Endpoint:    srv.URL + "/v1",
		APIKey:      "k",
		Model:       "bge-large-zh-v1.5",
		ExpectedDim: 2,
	}

	vectors, err := embedder.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(vectors) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vectors))
	}
	if gotAuth != "Bearer k" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotFailover != "true" {
		t.Fatalf("expected failover=true, got %q", gotFailover)
	}
	if gotPath != "/v1/embeddings" {
		t.Fatalf("expected /v1/embeddings, got %q", gotPath)
	}
}

func TestHTTPEmbedder_Embed_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream down"))
	}))
	defer srv.Close()

	_, err := HTTPEmbedder{Endpoint: srv.URL}.Embed(context.Background(), []string{"a"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestHTTPEmbedder_Embed_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	_, err := HTTPEmbedder{Endpoint: srv.URL}.Embed(context.Background(), []string{"a"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestHTTPEmbedder_Embed_CountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,2]}]}`))
	}))
	defer srv.Close()

	_, err := HTTPEmbedder{Endpoint: srv.URL}.Embed(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestHTTPEmbedder_Embed_DimMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,2,3]}]}`))
	}))
	defer srv.Close()

	_, err := HTTPEmbedder{Endpoint: srv.URL, ExpectedDim: 2}.Embed(context.Background(), []string{"a"})
	if err == nil {
		t.Fatalf("expected error")
	}
}
