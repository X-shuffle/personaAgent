package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// HTTPEmbedder calls an OpenAI-compatible embeddings endpoint.
// 用于把文本转换为向量，供 Qdrant 相似度检索使用。
type HTTPEmbedder struct {
	// Endpoint 例如 https://ai.gitee.com/v1/embeddings（也支持传 /v1，内部会自动补全）。
	Endpoint    string
	APIKey      string
	Model       string
	// ExpectedDim 用于强校验向量维度，避免写入 Qdrant 时才报维度不匹配。
	ExpectedDim int
	HTTPClient  *http.Client
}

type embeddingsRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type embeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func normalizeEmbeddingsEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("memory embed endpoint is empty")
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid memory embed endpoint: %q", raw)
	}

	switch {
	case u.Path == "" || u.Path == "/":
		u.Path = "/v1/embeddings"
	case strings.HasSuffix(u.Path, "/embeddings"):
		// keep as-is
	case u.Path == "/v1" || u.Path == "/v1/":
		u.Path = "/v1/embeddings"
	}

	return u.String(), nil
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func (e HTTPEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	endpoint, err := normalizeEmbeddingsEndpoint(e.Endpoint)
	if err != nil {
		return nil, err
	}

	hc := e.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}

	modelName := strings.TrimSpace(e.Model)
	if modelName == "" {
		modelName = "bge-large-zh-v1.5"
	}

	payload := embeddingsRequest{
		Input: texts,
		Model: modelName,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal embeddings request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embeddings request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Failover-Enabled", "true")
	if strings.TrimSpace(e.APIKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(e.APIKey))
	}

	resp, err := hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embeddings request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings upstream status %d: %s", resp.StatusCode, truncateForLog(string(raw), 1024))
	}

	var parsed embeddingsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode embeddings response: %w", err)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings size mismatch: got %d vectors for %d texts", len(parsed.Data), len(texts))
	}

	sort.Slice(parsed.Data, func(i, j int) bool { return parsed.Data[i].Index < parsed.Data[j].Index })
	vectors := make([][]float64, 0, len(parsed.Data))
	for i, item := range parsed.Data {
		if item.Index != i {
			return nil, fmt.Errorf("embeddings index mismatch at %d: got %d", i, item.Index)
		}
		if e.ExpectedDim > 0 && len(item.Embedding) != e.ExpectedDim {
			return nil, fmt.Errorf("embedding dimension mismatch at %d: got %d expected %d", i, len(item.Embedding), e.ExpectedDim)
		}
		vectors = append(vectors, item.Embedding)
	}
	return vectors, nil
}
