package memory

import (
	"context"
	"math"
	"sort"
	"sync"

	"persona_agent/internal/model"
)

// InMemoryStore is a local vector store for tests/dev.
type InMemoryStore struct {
	mu       sync.RWMutex
	byID     map[string]model.Memory
	bySessID map[string][]string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		byID:     make(map[string]model.Memory),
		bySessID: make(map[string][]string),
	}
}

func (s *InMemoryStore) Upsert(_ context.Context, memories []model.Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range memories {
		old, exists := s.byID[m.ID]
		if exists {
			s.bySessID[old.SessionID] = removeID(s.bySessID[old.SessionID], m.ID)
		}
		s.byID[m.ID] = cloneMemory(m)
		s.bySessID[m.SessionID] = appendIfMissing(s.bySessID[m.SessionID], m.ID)
	}
	return nil
}

func (s *InMemoryStore) Search(_ context.Context, query model.MemorySearchQuery) ([]model.MemoryMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.bySessID[query.SessionID]
	matches := make([]model.MemoryMatch, 0, len(ids))
	for _, id := range ids {
		m, ok := s.byID[id]
		if !ok {
			continue
		}
		if m.Importance < query.MinImportance {
			continue
		}
		score := cosineSimilarity(query.QueryEmbedding, m.Embedding)
		matches = append(matches, model.MemoryMatch{Memory: cloneMemory(m), Score: score})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].Memory.Timestamp > matches[j].Memory.Timestamp
		}
		return matches[i].Score > matches[j].Score
	})

	if query.TopK > 0 && len(matches) > query.TopK {
		matches = matches[:query.TopK]
	}
	return matches, nil
}

func removeID(ids []string, target string) []string {
	for i, id := range ids {
		if id == target {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

func appendIfMissing(ids []string, target string) []string {
	for _, id := range ids {
		if id == target {
			return ids
		}
	}
	return append(ids, target)
}

func cloneMemory(m model.Memory) model.Memory {
	cp := m
	cp.Embedding = append([]float64(nil), m.Embedding...)
	return cp
}

func cosineSimilarity(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}

	var dot, na, nb float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
