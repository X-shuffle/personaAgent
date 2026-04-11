package memory

import (
	"context"
	"testing"

	"persona_agent/internal/model"
)

func TestInMemoryStore_SearchRanksBySimilarity(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	err := store.Upsert(ctx, []model.Memory{
		{ID: "1", SessionID: "s1", Content: "a", Embedding: []float64{1, 0}, Importance: 0.5, Timestamp: 10},
		{ID: "2", SessionID: "s1", Content: "b", Embedding: []float64{0, 1}, Importance: 0.5, Timestamp: 20},
		{ID: "3", SessionID: "s2", Content: "c", Embedding: []float64{1, 0}, Importance: 0.5, Timestamp: 30},
	})
	if err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	matches, err := store.Search(ctx, model.MemorySearchQuery{
		SessionID:      "s1",
		QueryEmbedding: []float64{1, 0},
		TopK:           2,
		MinImportance:  0.1,
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Memory.ID != "1" {
		t.Fatalf("expected first match id=1, got %s", matches[0].Memory.ID)
	}
}

func TestInMemoryStore_SearchRespectsImportance(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, []model.Memory{
		{ID: "1", SessionID: "s1", Embedding: []float64{1, 0}, Importance: 0.2},
		{ID: "2", SessionID: "s1", Embedding: []float64{1, 0}, Importance: 0.9},
	})

	matches, err := store.Search(ctx, model.MemorySearchQuery{
		SessionID:      "s1",
		QueryEmbedding: []float64{1, 0},
		TopK:           10,
		MinImportance:  0.5,
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(matches) != 1 || matches[0].Memory.ID != "2" {
		t.Fatalf("expected only memory id=2, got %+v", matches)
	}
}
