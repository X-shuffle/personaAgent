package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"persona_agent/internal/model"
)

// QdrantStore is a vector store backed by Qdrant HTTP API.
type QdrantStore struct {
	URL        string
	Collection string
	APIKey     string
	VectorDim  int
	Client     *http.Client

	ensureMu sync.Mutex
	ensured  bool
}

func NewQdrantStore(urlValue, collection, apiKey string, vectorDim int) *QdrantStore {
	if vectorDim <= 0 {
		vectorDim = 256
	}
	return &QdrantStore{
		URL:        strings.TrimRight(strings.TrimSpace(urlValue), "/"),
		Collection: strings.TrimSpace(collection),
		APIKey:     strings.TrimSpace(apiKey),
		VectorDim:  vectorDim,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *QdrantStore) Upsert(ctx context.Context, memories []model.Memory) error {
	if len(memories) == 0 {
		return nil
	}

	dim := s.VectorDim
	if len(memories[0].Embedding) > 0 {
		dim = len(memories[0].Embedding)
	}
	if err := s.ensureCollection(ctx, dim); err != nil {
		return err
	}

	type point struct {
		ID      any            `json:"id"`
		Vector  []float64      `json:"vector"`
		Payload map[string]any `json:"payload"`
	}

	points := make([]point, 0, len(memories))
	for _, m := range memories {
		points = append(points, point{
			ID:     m.ID,
			Vector: m.Embedding,
			Payload: map[string]any{
				"session_id": m.SessionID,
				"type":       m.Type,
				"content":    m.Content,
				"emotion":    m.Emotion,
				"timestamp":  m.Timestamp,
				"importance": m.Importance,
			},
		})
	}

	body := map[string]any{"points": points}
	if err := s.doJSON(ctx, http.MethodPut, s.collectionPath("points")+"?wait=true", body, nil, http.StatusOK); err != nil {
		return fmt.Errorf("qdrant upsert points: %w", err)
	}
	return nil
}

func (s *QdrantStore) Search(ctx context.Context, query model.MemorySearchQuery) ([]model.MemoryMatch, error) {
	if len(query.QueryEmbedding) == 0 {
		return nil, nil
	}

	dim := s.VectorDim
	if len(query.QueryEmbedding) > 0 {
		dim = len(query.QueryEmbedding)
	}
	if err := s.ensureCollection(ctx, dim); err != nil {
		return nil, err
	}

	must := []map[string]any{
		{
			"key": "session_id",
			"match": map[string]any{
				"value": query.SessionID,
			},
		},
	}
	if query.MinImportance > 0 {
		must = append(must, map[string]any{
			"key": "importance",
			"range": map[string]any{
				"gte": query.MinImportance,
			},
		})
	}

	limit := query.TopK
	if limit <= 0 {
		limit = 3
	}

	body := map[string]any{
		"vector":       query.QueryEmbedding,
		"limit":        limit,
		"with_payload": true,
		"filter": map[string]any{
			"must": must,
		},
	}

	var resp struct {
		Result []struct {
			ID      any            `json:"id"`
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}

	if err := s.doJSON(ctx, http.MethodPost, s.collectionPath("points", "search"), body, &resp, http.StatusOK); err != nil {
		return nil, fmt.Errorf("qdrant search points: %w", err)
	}

	matches := make([]model.MemoryMatch, 0, len(resp.Result))
	for _, item := range resp.Result {
		m := model.Memory{
			ID:         toString(item.ID),
			SessionID:  toString(item.Payload["session_id"]),
			Type:       model.MemoryType(toString(item.Payload["type"])),
			Content:    toString(item.Payload["content"]),
			Emotion:    toString(item.Payload["emotion"]),
			Timestamp:  toInt64(item.Payload["timestamp"]),
			Importance: toFloat64(item.Payload["importance"]),
		}
		matches = append(matches, model.MemoryMatch{Memory: m, Score: item.Score})
	}
	return matches, nil
}

func (s *QdrantStore) ensureCollection(ctx context.Context, vectorDim int) error {
	if vectorDim <= 0 {
		vectorDim = s.VectorDim
	}

	s.ensureMu.Lock()
	if s.ensured {
		s.ensureMu.Unlock()
		return nil
	}
	s.ensureMu.Unlock()

	exists, err := s.collectionExists(ctx)
	if err != nil {
		return fmt.Errorf("check collection exists: %w", err)
	}
	if exists {
		s.ensureMu.Lock()
		s.ensured = true
		s.ensureMu.Unlock()
		return nil
	}

	body := map[string]any{
		"vectors": map[string]any{
			"size":     vectorDim,
			"distance": "Cosine",
		},
	}
	if err := s.doJSONStatus(ctx, http.MethodPut, s.collectionPath(), body, nil, map[int]struct{}{
		http.StatusOK:      {},
		http.StatusCreated: {},
	}); err != nil {
		return fmt.Errorf("create collection: %w", err)
	}

	s.ensureMu.Lock()
	s.ensured = true
	s.ensureMu.Unlock()
	return nil
}

func (s *QdrantStore) collectionExists(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+s.collectionPath(), nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	if s.APIKey != "" {
		req.Header.Set("api-key", s.APIKey)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func (s *QdrantStore) collectionPath(parts ...string) string {
	base := path.Join("collections", url.PathEscape(s.Collection))
	for _, p := range parts {
		base = path.Join(base, p)
	}
	return "/" + base
}

func (s *QdrantStore) doJSON(ctx context.Context, method, endpoint string, reqBody any, out any, expectedStatus int) error {
	return s.doJSONStatus(ctx, method, endpoint, reqBody, out, map[int]struct{}{
		expectedStatus: {},
	})
}

func (s *QdrantStore) doJSONStatus(ctx context.Context, method, endpoint string, reqBody any, out any, expectedStatuses map[int]struct{}) error {
	if s.URL == "" {
		return fmt.Errorf("qdrant url is required")
	}
	if s.Collection == "" {
		return fmt.Errorf("qdrant collection is required")
	}

	var bodyBytes []byte
	if reqBody != nil {
		encoded, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyBytes = encoded
	}

	req, err := http.NewRequestWithContext(ctx, method, s.URL+endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.APIKey != "" {
		req.Header.Set("api-key", s.APIKey)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if _, ok := expectedStatuses[resp.StatusCode]; !ok {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func toString(v any) string {
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	default:
		return 0
	}
}
