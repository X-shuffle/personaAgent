package memory

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"persona_agent/internal/model"
)

// DefaultService implements memory retrieval and turn storage.
type DefaultService struct {
	store    Store
	embedder Embedder
	// logger 用于输出检索命中/过滤细节，便于排查“为何某条记忆未被使用”。
	logger        *zap.Logger
	topK          int
	minImportance float64
	minSimilarity float64
	shortTermSize int
	now           func() time.Time

	cacheMu         sync.RWMutex
	shortTermBySess map[string][]model.Memory
}

func NewService(store Store, embedder Embedder, logger *zap.Logger, topK int, minImportance, minSimilarity float64, shortTermSize int) *DefaultService {
	if logger == nil {
		// 统一要求外部注入 logger，避免运行时到处做 nil 判断。
		panic("memory logger is nil")
	}
	if topK <= 0 {
		topK = 3
	}
	if minSimilarity < 0 {
		minSimilarity = 0
	}
	if minSimilarity > 1 {
		minSimilarity = 1
	}
	if shortTermSize <= 0 {
		shortTermSize = topK
	}
	return &DefaultService{
		store:           store,
		embedder:        embedder,
		logger:          logger,
		topK:            topK,
		minImportance:   minImportance,
		minSimilarity:   minSimilarity,
		shortTermSize:   shortTermSize,
		now:             time.Now,
		shortTermBySess: make(map[string][]model.Memory),
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
	s.logger.Debug("memory retrieve raw matches",
		zap.String("session_id", sessionID),
		zap.Int("count", len(matches)),
	)

	out := make([]model.Memory, 0, len(matches))
	for _, m := range matches {
		if m.Score < s.minSimilarity {
			// 分数低于阈值的记忆会被过滤，不参与后续 prompt 组装。
			s.logger.Debug("memory retrieve filtered",
				zap.Float64("score", m.Score),
				zap.Float64("threshold", s.minSimilarity),
			)
			continue
		}
		s.logger.Debug("memory retrieve kept",
			zap.Float64("score", m.Score),
			zap.Any("memory", m.Memory.Content),
		)
		out = append(out, m.Memory)
	}
	if len(out) > 0 {
		return out, nil
	}

	recent := s.loadShortTerm(sessionID, s.topK)
	if len(recent) > 0 {
		s.logger.Debug("memory retrieve fallback short-term cache",
			zap.String("session_id", sessionID),
			zap.Int("count", len(recent)),
		)
	}
	return recent, nil
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
	s.pushShortTerm(memory)
	if err := s.store.Upsert(ctx, []model.Memory{memory}); err != nil {
		return fmt.Errorf("upsert memory: %w", err)
	}
	return nil
}

func (s *DefaultService) pushShortTerm(memory model.Memory) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	items := append([]model.Memory{memory}, s.shortTermBySess[memory.SessionID]...)
	if len(items) > s.shortTermSize {
		items = items[:s.shortTermSize]
	}
	s.shortTermBySess[memory.SessionID] = items
}

func (s *DefaultService) loadShortTerm(sessionID string, limit int) []model.Memory {
	if limit <= 0 {
		limit = 3
	}

	s.cacheMu.RLock()
	items := s.shortTermBySess[sessionID]
	if len(items) == 0 {
		s.cacheMu.RUnlock()
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	out := append([]model.Memory(nil), items...)
	s.cacheMu.RUnlock()
	return out
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
