package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"persona_agent/internal/model"
)

type fakeEmbedder struct {
	vectors [][]float64
	err     error
}

func (f fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.vectors != nil {
		return f.vectors, nil
	}
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = []float64{1, 0}
	}
	return out, nil
}

type fakeStore struct {
	matches    []model.MemoryMatch
	searchErr  error
	upsertErr  error
	upserted   []model.Memory
	lastSearch model.MemorySearchQuery
}

func (f *fakeStore) Upsert(_ context.Context, memories []model.Memory) error {
	f.upserted = append(f.upserted, memories...)
	if f.upsertErr != nil {
		return f.upsertErr
	}
	return nil
}

func (f *fakeStore) Search(_ context.Context, query model.MemorySearchQuery) ([]model.MemoryMatch, error) {
	f.lastSearch = query
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.matches, nil
}

func TestServiceRetrieve_OK(t *testing.T) {
	store := &fakeStore{matches: []model.MemoryMatch{{Memory: model.Memory{ID: "m1"}, Score: 0.9}}}
	svc := NewService(store, fakeEmbedder{}, 3, 0.2)

	memories, err := svc.Retrieve(context.Background(), "s1", "hello")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(memories) != 1 || memories[0].ID != "m1" {
		t.Fatalf("unexpected memories: %+v", memories)
	}
	if store.lastSearch.TopK != 3 {
		t.Fatalf("expected TopK=3, got %d", store.lastSearch.TopK)
	}
}

func TestServiceStoreTurn_OK(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store, fakeEmbedder{}, 3, 0)
	svc.now = func() time.Time { return time.Unix(1000, 0) }

	err := svc.StoreTurn(context.Background(), "s1", "u", "a", model.EmotionState{Label: "anxious", Intensity: 0.6})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("expected one upserted memory")
	}
	m := store.upserted[0]
	if m.SessionID != "s1" || m.Type != model.MemoryTypeEpisodic {
		t.Fatalf("unexpected memory: %+v", m)
	}
	if m.Timestamp != 1000 {
		t.Fatalf("expected timestamp=1000, got %d", m.Timestamp)
	}
	if m.Emotion != "anxious" {
		t.Fatalf("expected emotion anxious, got %q", m.Emotion)
	}
}

func TestServiceStoreTurn_EmbedError(t *testing.T) {
	svc := NewService(&fakeStore{}, fakeEmbedder{err: errors.New("boom")}, 3, 0)
	if err := svc.StoreTurn(context.Background(), "s1", "u", "a", model.EmotionState{}); err == nil {
		t.Fatalf("expected error")
	}
}
