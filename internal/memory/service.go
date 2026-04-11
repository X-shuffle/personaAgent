package memory

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"persona_agent/internal/model"
)

// DefaultService implements memory retrieval and turn storage.
type DefaultService struct {
	store         Store
	embedder      Embedder
	topK          int
	minImportance float64
	now           func() time.Time
}

func NewService(store Store, embedder Embedder, topK int, minImportance float64) *DefaultService {
	if topK <= 0 {
		topK = 3
	}
	return &DefaultService{
		store:         store,
		embedder:      embedder,
		topK:          topK,
		minImportance: minImportance,
		now:           time.Now,
	}
}

func (s *DefaultService) Retrieve(ctx context.Context, sessionID, userInput string) ([]model.Memory, error) {
	sessionID = strings.TrimSpace(sessionID)
	userInput = strings.TrimSpace(userInput)
	if sessionID == "" || userInput == "" {
		return nil, nil
	}

	vectors, err := s.embedder.Embed(ctx, []string{userInput})
	if err != nil {
		return nil, fmt.Errorf("embed retrieve query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, nil
	}

	matches, err := s.store.Search(ctx, model.MemorySearchQuery{
		SessionID:      sessionID,
		QueryEmbedding: vectors[0],
		TopK:           s.topK,
		MinImportance:  s.minImportance,
	})
	if err != nil {
		return nil, fmt.Errorf("search memory: %w", err)
	}

	out := make([]model.Memory, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.Memory)
	}
	return out, nil
}

func (s *DefaultService) StoreTurn(ctx context.Context, sessionID, userInput, assistantOutput string, emotion model.EmotionState) error {
	sessionID = strings.TrimSpace(sessionID)
	userInput = strings.TrimSpace(userInput)
	assistantOutput = strings.TrimSpace(assistantOutput)
	if sessionID == "" || userInput == "" || assistantOutput == "" {
		return nil
	}

	content := "User: " + userInput + "\nAssistant: " + assistantOutput
	vectors, err := s.embedder.Embed(ctx, []string{content})
	if err != nil {
		return fmt.Errorf("embed memory turn: %w", err)
	}
	if len(vectors) == 0 {
		return nil
	}

	memory := model.Memory{
		ID:         newMemoryID(),
		SessionID:  sessionID,
		Type:       model.MemoryTypeEpisodic,
		Content:    content,
		Embedding:  vectors[0],
		Emotion:    strings.TrimSpace(emotion.Label),
		Timestamp:  s.now().Unix(),
		Importance: 0.5,
	}
	if err := s.store.Upsert(ctx, []model.Memory{memory}); err != nil {
		return fmt.Errorf("upsert memory: %w", err)
	}
	return nil
}

func newMemoryID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16],
	)
}
