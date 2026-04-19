package ingestion

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
	upserted []model.Memory
	err      error
}

func (f *fakeStore) Upsert(_ context.Context, memories []model.Memory) error {
	f.upserted = append(f.upserted, memories...)
	return f.err
}

func (f *fakeStore) Search(_ context.Context, _ model.MemorySearchQuery) ([]model.MemoryMatch, error) {
	return nil, nil
}

func TestServiceIngest_TXT_OK(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(fakeEmbedder{}, store, Config{Enabled: true, SegmentMaxChars: 500, MergeWindowSeconds: 90, EmbedBatchSize: 2, AllowedExtensions: []string{"txt", "json"}})
	fixedNow := time.Unix(2000, 0)
	svc.now = func() time.Time { return fixedNow }

	input := "2026-04-11 10:00:00 Alice: 你好\n继续一行\n2026-04-11 10:00:20 Alice: [图片]\n2026-04-11 10:01:10 Bob: 收到"
	result, err := svc.Ingest(context.Background(), Request{
		SessionID: "s1",
		Filename:  "chat.txt",
		Data:      []byte(input),
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if result.Accepted != 2 {
		t.Fatalf("expected accepted=2, got %d", result.Accepted)
	}
	if result.Rejected != 1 {
		t.Fatalf("expected rejected=1, got %d", result.Rejected)
	}
	if result.Stored != 2 {
		t.Fatalf("expected stored=2, got %d", result.Stored)
	}
	if len(store.upserted) != 2 {
		t.Fatalf("expected upserted=2, got %d", len(store.upserted))
	}
	if store.upserted[0].Importance <= 0 || store.upserted[0].Importance > 1 {
		t.Fatalf("expected clamped importance, got %f", store.upserted[0].Importance)
	}
}

func TestServiceIngest_ImportanceVariesByContent(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(fakeEmbedder{}, store, Config{Enabled: true, SegmentMaxChars: 500, MergeWindowSeconds: 90, EmbedBatchSize: 4, AllowedExtensions: []string{"txt", "json"}})

	input := "2026-04-11 10:00:00 Alice: 你好\n2026-04-11 10:00:20 Bob: 我计划下周完成迁移，记得提醒我"
	_, err := svc.Ingest(context.Background(), Request{SessionID: "s1", Filename: "chat.txt", Data: []byte(input)})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(store.upserted) != 2 {
		t.Fatalf("expected upserted=2, got %d", len(store.upserted))
	}
	if store.upserted[1].Importance <= store.upserted[0].Importance {
		t.Fatalf("expected second segment importance higher, got first=%f second=%f", store.upserted[0].Importance, store.upserted[1].Importance)
	}
}

func TestServiceIngest_JSON_DryRun(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(fakeEmbedder{}, store, Config{Enabled: true, AllowedExtensions: []string{"txt", "json"}})

	data := `[{"sender":"alice","msg":"hello","time":"1712800000"}]`
	result, err := svc.Ingest(context.Background(), Request{
		SessionID: "s1",
		Filename:  "chat.json",
		Format:    "json",
		Data:      []byte(data),
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !result.DryRun {
		t.Fatalf("expected dry_run=true")
	}
	if result.Stored != 0 {
		t.Fatalf("expected stored=0, got %d", result.Stored)
	}
	if len(store.upserted) != 0 {
		t.Fatalf("expected no upsert in dry run")
	}
}

func TestServiceIngest_UnsupportedFileExt(t *testing.T) {
	svc := NewService(fakeEmbedder{}, &fakeStore{}, Config{Enabled: true, AllowedExtensions: []string{"txt", "json"}})
	_, err := svc.Ingest(context.Background(), Request{SessionID: "s1", Filename: "chat.csv", Data: []byte("x")})
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestServiceIngest_Disabled(t *testing.T) {
	svc := NewService(fakeEmbedder{}, &fakeStore{}, Config{Enabled: false})
	_, err := svc.Ingest(context.Background(), Request{SessionID: "s1", Data: []byte("x")})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}
