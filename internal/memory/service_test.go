package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

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
	matches           []model.MemoryMatch
	recent            []model.Memory
	searchErr         error
	recentErr         error
	upsertErr         error
	upserted          []model.Memory
	lastSearch        model.MemorySearchQuery
	lastRecentSession string
	lastRecentLimit   int
	recentCalls       int
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

func (f *fakeStore) RecentBySession(_ context.Context, sessionID string, limit int) ([]model.Memory, error) {
	f.recentCalls++
	f.lastRecentSession = sessionID
	f.lastRecentLimit = limit
	if f.recentErr != nil {
		return nil, f.recentErr
	}
	return f.recent, nil
}

func TestServiceRetrieve_OK(t *testing.T) {
	store := &fakeStore{matches: []model.MemoryMatch{{Memory: model.Memory{ID: "m1"}, Score: 0.9}}}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0.2, 0, 3)

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

func TestServiceRetrieve_SimilarityThreshold(t *testing.T) {
	store := &fakeStore{matches: []model.MemoryMatch{
		{Memory: model.Memory{ID: "low"}, Score: 0.19},
		{Memory: model.Memory{ID: "high"}, Score: 0.81},
	}}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0, 0.2, 3)

	memories, err := svc.Retrieve(context.Background(), "s1", "hello")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(memories) != 1 || memories[0].ID != "high" {
		t.Fatalf("unexpected memories: %+v", memories)
	}
}

func TestServiceRetrieve_FallbackRecentWhenFilteredEmpty(t *testing.T) {
	store := &fakeStore{
		matches: []model.MemoryMatch{{Memory: model.Memory{ID: "low"}, Score: 0.1}},
	}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0, 0.2, 2)
	_ = svc.StoreTurn(context.Background(), "s1", "u1", "a1", model.EmotionState{})
	_ = svc.StoreTurn(context.Background(), "s1", "u2", "a2", model.EmotionState{})

	memories, err := svc.Retrieve(context.Background(), "s1", "hello")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(memories) != 2 {
		t.Fatalf("unexpected fallback memories: %+v", memories)
	}
	if !strings.Contains(memories[0].Content, "u2") || !strings.Contains(memories[1].Content, "u1") {
		t.Fatalf("unexpected fallback order/content: %+v", memories)
	}
}

func TestServiceRetrieve_UsesRecentStoreWhenColdStart(t *testing.T) {
	store := &fakeStore{
		matches: []model.MemoryMatch{{Memory: model.Memory{ID: "low"}, Score: 0.1}},
		recent: []model.Memory{
			{ID: "r1", Type: model.MemoryTypeSummary, Timestamp: 100},
			{ID: "r2", Type: model.MemoryTypeSummary, Timestamp: 90},
			{ID: "r3", Type: model.MemoryTypeEpisodic, Timestamp: 80},
		},
	}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0, 0.2, 2)

	memories, err := svc.Retrieve(context.Background(), "s1", "hello")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if store.recentCalls != 1 || store.lastRecentSession != "s1" || store.lastRecentLimit != 3 {
		t.Fatalf("unexpected recent call: calls=%d session=%q limit=%d", store.recentCalls, store.lastRecentSession, store.lastRecentLimit)
	}
	if len(memories) != 2 {
		t.Fatalf("expected summary capped recent memories, got %+v", memories)
	}
	summaryCount := 0
	for _, m := range memories {
		if m.Type == model.MemoryTypeSummary {
			summaryCount++
		}
	}
	if summaryCount > 1 {
		t.Fatalf("expected at most one summary in recent fallback, got %d", summaryCount)
	}
}

func TestServiceRetrieve_ShortTermPreferredOverRecentStore(t *testing.T) {
	store := &fakeStore{
		matches: []model.MemoryMatch{{Memory: model.Memory{ID: "low"}, Score: 0.1}},
		recent:  []model.Memory{{ID: "r1"}},
	}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0, 0.2, 2)
	_ = svc.StoreTurn(context.Background(), "s1", "u1", "a1", model.EmotionState{})

	memories, err := svc.Retrieve(context.Background(), "s1", "hello")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(memories) == 0 {
		t.Fatalf("expected short-term fallback memories")
	}
	if store.recentCalls != 0 {
		t.Fatalf("expected no recent store call, got %d", store.recentCalls)
	}
}

func TestServiceRetrieve_RecentStoreErrorGraceful(t *testing.T) {
	store := &fakeStore{
		matches:   []model.MemoryMatch{{Memory: model.Memory{ID: "low"}, Score: 0.1}},
		recentErr: errors.New("recent boom"),
	}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0, 0.2, 2)

	memories, err := svc.Retrieve(context.Background(), "s1", "hello")
	if err != nil {
		t.Fatalf("expected graceful recent fallback error, got: %v", err)
	}
	if len(memories) != 0 {
		t.Fatalf("expected empty memories on recent fallback error, got %+v", memories)
	}
	if store.recentCalls != 1 {
		t.Fatalf("expected one recent call, got %d", store.recentCalls)
	}
}


func TestServiceStoreTurn_OK(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0, 0, 3)
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
	if m.Importance <= 0 || m.Importance > 1 {
		t.Fatalf("expected importance in (0,1], got %f", m.Importance)
	}
}

func TestServiceStoreTurn_ImportanceVariesBySignal(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0, 0, 8)

	if err := svc.StoreTurn(context.Background(), "s1", "你好", "好的", model.EmotionState{Label: "neutral", Intensity: 0.1}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := svc.StoreTurn(context.Background(), "s1", "我计划下周完成迁移，记得提醒我", "收到", model.EmotionState{Label: "anxious", Intensity: 0.9}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(store.upserted) < 2 {
		t.Fatalf("expected at least 2 upserted memories")
	}
	low := store.upserted[0].Importance
	high := store.upserted[1].Importance
	if high <= low {
		t.Fatalf("expected high-signal turn importance > low-signal turn, low=%f high=%f", low, high)
	}
}

func TestServiceStoreTurn_GeneratesSummaryEveryTriggerTurns(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0, 0, 8)
	base := time.Unix(1000, 0)
	idx := int64(0)
	svc.now = func() time.Time {
		idx++
		return base.Add(time.Duration(idx) * time.Second)
	}

	for i := 0; i < summaryTriggerTurns; i++ {
		if err := svc.StoreTurn(context.Background(), "s1", "我计划明天完成任务", "收到", model.EmotionState{Label: "anxious", Intensity: 0.7}); err != nil {
			t.Fatalf("unexpected err at turn %d: %v", i, err)
		}
	}

	summaryCount := 0
	for _, m := range store.upserted {
		if m.Type == model.MemoryTypeSummary {
			summaryCount++
			if !strings.Contains(m.Content, "Summary of recent session context") {
				t.Fatalf("unexpected summary content: %s", m.Content)
			}
		}
	}
	if summaryCount != 1 {
		t.Fatalf("expected 1 summary memory, got %d", summaryCount)
	}
}

func TestServiceStoreTurn_SummaryFailureDoesNotFailMainFlow(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store, fakeEmbedder{err: errors.New("boom")}, zap.NewNop(), 3, 0, 0, 8)
	if err := svc.StoreTurn(context.Background(), "s1", "u", "a", model.EmotionState{}); err == nil {
		t.Fatalf("expected error when episodic embedding fails")
	}

	embed := fakeEmbedder{}
	store = &fakeStore{}
	svc = NewService(store, embed, zap.NewNop(), 3, 0, 0, 8)
	svc.now = func() time.Time { return time.Unix(1000, 0) }
	for i := 0; i < summaryTriggerTurns-1; i++ {
		if err := svc.StoreTurn(context.Background(), "s1", "u", "a", model.EmotionState{}); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	}

	store.upsertErr = errors.New("upsert boom")
	err := svc.StoreTurn(context.Background(), "s1", "u", "a", model.EmotionState{})
	if err == nil {
		t.Fatalf("expected store upsert error")
	}
}

func TestServiceRetrieve_CapsSummaryMemories(t *testing.T) {
	store := &fakeStore{matches: []model.MemoryMatch{
		{Memory: model.Memory{ID: "s1", Type: model.MemoryTypeSummary}, Score: 0.99},
		{Memory: model.Memory{ID: "s2", Type: model.MemoryTypeSummary}, Score: 0.98},
		{Memory: model.Memory{ID: "e1", Type: model.MemoryTypeEpisodic}, Score: 0.97},
	}}
	svc := NewService(store, fakeEmbedder{}, zap.NewNop(), 3, 0, 0, 8)

	memories, err := svc.Retrieve(context.Background(), "sess", "hello")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	summaryCount := 0
	for _, m := range memories {
		if m.Type == model.MemoryTypeSummary {
			summaryCount++
		}
	}
	if summaryCount > 1 {
		t.Fatalf("expected at most one summary memory, got %d", summaryCount)
	}
}

func TestServiceStoreTurn_EmbedError(t *testing.T) {
	svc := NewService(&fakeStore{}, fakeEmbedder{err: errors.New("boom")}, zap.NewNop(), 3, 0, 0, 3)
	if err := svc.StoreTurn(context.Background(), "s1", "u", "a", model.EmotionState{}); err == nil {
		t.Fatalf("expected error")
	}
}
