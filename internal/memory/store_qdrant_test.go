package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"persona_agent/internal/model"
)

func TestQdrantStore_Upsert(t *testing.T) {
	var gotCollectionGet, gotCollectionPut, gotPoints bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/persona_memories":
			gotCollectionGet = true
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":{"error":"Not found"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/persona_memories":
			gotCollectionPut = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/persona_memories/points":
			gotPoints = true
			if r.URL.RawQuery != "wait=true" {
				t.Fatalf("expected wait=true query, got %q", r.URL.RawQuery)
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode upsert body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{"status":"acknowledged"}}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	store := NewQdrantStore(srv.URL, "persona_memories", "", 4)
	err := store.Upsert(context.Background(), []model.Memory{{
		ID:         "550e8400-e29b-41d4-a716-446655440000",
		SessionID:  "s1",
		Type:       model.MemoryTypeEpisodic,
		Content:    "hello",
		Embedding:  []float64{1, 0, 0, 0},
		Emotion:    "happy",
		Importance: 0.6,
		Timestamp:  100,
	}})
	if err != nil {
		t.Fatalf("unexpected upsert err: %v", err)
	}
	if !gotCollectionGet || !gotCollectionPut || !gotPoints {
		t.Fatalf("expected collection get/put and points endpoints called")
	}
}

func TestQdrantStore_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/persona_memories":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/persona_memories/points/search":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode search body: %v", err)
			}
			if _, ok := body["filter"]; !ok {
				t.Fatalf("expected filter in search body")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"result": [
					{
						"id": "550e8400-e29b-41d4-a716-446655440001",
						"score": 0.98,
						"payload": {
							"session_id": "s1",
							"type": "episodic",
							"content": "User: hi",
							"emotion": "",
							"timestamp": 100,
							"importance": 0.5
						}
					}
				]
			}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	store := NewQdrantStore(strings.TrimRight(srv.URL, "/"), "persona_memories", "", 4)
	matches, err := store.Search(context.Background(), model.MemorySearchQuery{
		SessionID:      "s1",
		QueryEmbedding: []float64{1, 0, 0, 0},
		TopK:           3,
	})
	if err != nil {
		t.Fatalf("unexpected search err: %v", err)
	}
	if len(matches) != 1 || matches[0].Memory.ID != "550e8400-e29b-41d4-a716-446655440001" {
		t.Fatalf("unexpected matches: %+v", matches)
	}
	if matches[0].Memory.Emotion != "" {
		t.Fatalf("expected empty emotion round-trip, got %q", matches[0].Memory.Emotion)
	}
}

func TestQdrantStore_RecentBySession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/persona_memories":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/persona_memories/points/scroll":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode scroll body: %v", err)
			}
			if body["limit"] != float64(3) {
				t.Fatalf("expected limit=3, got %#v", body["limit"])
			}
			if body["with_payload"] != true {
				t.Fatalf("expected with_payload=true, got %#v", body["with_payload"])
			}
			if body["with_vector"] != false {
				t.Fatalf("expected with_vector=false, got %#v", body["with_vector"])
			}
			filter, ok := body["filter"].(map[string]any)
			if !ok {
				t.Fatalf("expected filter object, got %#v", body["filter"])
			}
			must, ok := filter["must"].([]any)
			if !ok || len(must) == 0 {
				t.Fatalf("expected must filter, got %#v", filter["must"])
			}
			cond, ok := must[0].(map[string]any)
			if !ok {
				t.Fatalf("expected filter condition map, got %#v", must[0])
			}
			if cond["key"] != "session_id" {
				t.Fatalf("expected session_id filter key, got %#v", cond["key"])
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"result": {
					"points": [
						{
							"id": "p3",
							"payload": {
								"session_id": "s1",
								"type": "episodic",
								"content": "old",
								"emotion": "",
								"timestamp": 100,
								"importance": 0.1
							}
						},
						{
							"id": "p1",
							"payload": {
								"session_id": "s1",
								"type": "summary",
								"content": "new",
								"emotion": "",
								"timestamp": 300,
								"importance": 0.9
							}
						},
						{
							"id": "p2",
							"payload": {
								"session_id": "s1",
								"type": "episodic",
								"content": "mid",
								"emotion": "",
								"timestamp": 200,
								"importance": 0.5
							}
						}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	store := NewQdrantStore(strings.TrimRight(srv.URL, "/"), "persona_memories", "", 4)
	memories, err := store.RecentBySession(context.Background(), "s1", 3)
	if err != nil {
		t.Fatalf("unexpected recent err: %v", err)
	}
	if len(memories) != 3 {
		t.Fatalf("expected 3 memories, got %+v", memories)
	}
	if memories[0].ID != "p1" || memories[1].ID != "p2" || memories[2].ID != "p3" {
		t.Fatalf("expected timestamp-desc order, got %+v", memories)
	}
	if memories[0].Type != model.MemoryTypeSummary || memories[0].Timestamp != 300 {
		t.Fatalf("unexpected mapping for first memory: %+v", memories[0])
	}
}
