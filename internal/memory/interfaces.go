package memory

import (
	"context"

	"persona_agent/internal/model"
)

// Embedder converts text into vector embeddings.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// Store persists memories and supports vector search.
type Store interface {
	Upsert(ctx context.Context, memories []model.Memory) error
	Search(ctx context.Context, query model.MemorySearchQuery) ([]model.MemoryMatch, error)
	// RecentBySession 拉取指定 session 的最近记忆（按时间倒序，limit 为上限）。
	RecentBySession(ctx context.Context, sessionID string, limit int) ([]model.Memory, error)
}

// Service is the orchestration-facing memory API.
type Service interface {
	Retrieve(ctx context.Context, sessionID, userInput string) ([]model.Memory, error)
	StoreTurn(ctx context.Context, sessionID, userInput, assistantOutput string, emotion model.EmotionState) error
}
