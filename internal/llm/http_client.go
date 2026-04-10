package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"persona_agent/internal/model"
)

// HTTPClient calls an OpenAI-compatible chat completion endpoint.
type HTTPClient struct {
	Endpoint   string
	APIKey     string
	Model      string
	HTTPClient *http.Client
	Logger     *zap.Logger
}

type chatCompletionsRequest struct {
	Model       string             `json:"model"`
	Messages    []model.LLMMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
}

type chatCompletionsResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func normalizeChatCompletionsEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("llm endpoint is empty")
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid llm endpoint: %q", raw)
	}

	switch {
	case u.Path == "" || u.Path == "/":
		u.Path = "/v1/chat/completions"
	case strings.HasSuffix(u.Path, "/chat/completions"):
		// keep as-is
	case u.Path == "/v1" || u.Path == "/v1/":
		u.Path = "/v1/chat/completions"
	}

	return u.String(), nil
}

func sanitizeEndpointForLog(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func (c HTTPClient) Generate(ctx context.Context, req model.LLMRequest) (model.LLMResponse, error) {
	endpoint, err := normalizeChatCompletionsEndpoint(c.Endpoint)
	if err != nil {
		return model.LLMResponse{}, err
	}

	hc := c.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}

	modelName := strings.TrimSpace(c.Model)
	if modelName == "" {
		modelName = "default"
	}

	payload := chatCompletionsRequest{
		Model:       modelName,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("marshal llm request: %w", err)
	}

	if c.Logger != nil {
		c.Logger.Debug("llm http request",
			zap.String("endpoint", sanitizeEndpointForLog(endpoint)),
			zap.String("model", modelName),
			zap.String("request_body", truncateForLog(string(body), 2048)),
		)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("create llm request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.APIKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.APIKey))
	}

	resp, err := hc.Do(httpReq)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if c.Logger != nil {
		c.Logger.Debug("llm http response",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response_body", truncateForLog(string(raw), 2048)),
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.LLMResponse{}, fmt.Errorf("llm upstream status %d: %s", resp.StatusCode, truncateForLog(string(raw), 1024))
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return model.LLMResponse{}, fmt.Errorf("decode llm response: %w", err)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return model.LLMResponse{}, errors.New("llm returned empty response")
	}

	return model.LLMResponse{Text: parsed.Choices[0].Message.Content, Model: parsed.Model}, nil
}
