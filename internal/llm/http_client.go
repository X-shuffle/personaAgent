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

// HTTPClient 封装 OpenAI-compatible chat completions 调用。
// 该客户端在初始化时要求非空 logger，用于统一日志约束与排障。
type HTTPClient struct {
	Endpoint string
	APIKey   string
	Model    string
	// Provider 标识当前上游提供商。
	// 作用：在 normalization=auto 时选择 provider 默认兼容策略。
	Provider string
	// ToolPayloadNormalization 控制是否规范化 tool-call 历史消息。
	// 作用：按 on/off/auto 在发请求前决定是否执行 id 与 arguments 的兼容改写。
	ToolPayloadNormalization string
	HTTPClient               *http.Client
	Logger                   *zap.Logger
}

// NewHTTPClient 在初始化阶段校验 logger，避免运行时再做 nil 分支判断。
func NewHTTPClient(client HTTPClient) HTTPClient {
	if client.Logger == nil {
		panic("llm http client logger is nil")
	}
	return client
}

type chatCompletionsRequest struct {
	Model       string             `json:"model"`
	Messages    []model.LLMMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Tools       []model.LLMTool    `json:"tools,omitempty"`
	ToolChoice  string             `json:"tool_choice,omitempty"`
}

type chatCompletionsResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string              `json:"content"`
			ToolCalls []model.LLMToolCall `json:"tool_calls"`
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

func normalizeToolCallID(raw string) string {
	id := strings.TrimSpace(raw)
	if strings.HasPrefix(id, "fc_") {
		id = strings.TrimPrefix(id, "fc_")
	}
	return id
}

func normalizeToolCallIDsInMessages(messages []model.LLMMessage) []model.LLMMessage {
	if len(messages) == 0 {
		return messages
	}
	out := make([]model.LLMMessage, len(messages))
	copy(out, messages)
	for i := range out {
		out[i].ToolCallID = normalizeToolCallID(out[i].ToolCallID)
		if len(out[i].ToolCalls) == 0 {
			continue
		}
		calls := make([]model.LLMToolCall, len(out[i].ToolCalls))
		copy(calls, out[i].ToolCalls)
		for j := range calls {
			calls[j].ID = normalizeToolCallID(calls[j].ID)
			calls[j].Function.Arguments = normalizeToolCallArguments(calls[j].Function.Arguments)
		}
		out[i].ToolCalls = calls
	}
	return out
}

func normalizeToolCallArguments(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	if normalized, ok := marshalIfValidJSON(trimmed); ok {
		return normalized
	}
	if fixed, ok := salvageJSONObject(trimmed); ok {
		if normalized, ok := marshalIfValidJSON(fixed); ok {
			return normalized
		}
	}
	return raw
}

func marshalIfValidJSON(raw string) (string, bool) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "", false
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func salvageJSONObject(raw string) (string, bool) {
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] != '{' {
			continue
		}
		candidate := strings.TrimSpace(raw[i:])
		if strings.HasSuffix(candidate, "}") {
			return candidate, true
		}
	}
	return "", false
}

// shouldNormalizeToolPayload 决定是否对历史 tool 消息做兼容规范化。
// on/off 为强制策略；auto 按 provider 默认策略。
func (c HTTPClient) shouldNormalizeToolPayload() bool {
	mode := strings.ToLower(strings.TrimSpace(c.ToolPayloadNormalization))
	switch mode {
	case "on":
		return true
	case "off":
		return false
	}

	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	switch provider {
	case "", "generic", "minimax":
		return true
	default:
		return false
	}
}

// Generate 调用上游 chat completions，并解析文本/工具调用结果。
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

	messages := req.Messages
	if c.shouldNormalizeToolPayload() {
		messages = normalizeToolCallIDsInMessages(req.Messages)
	}

	payload := chatCompletionsRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("marshal llm request: %w", err)
	}

	c.Logger.Debug("llm http request",
		zap.String("endpoint", sanitizeEndpointForLog(endpoint)),
		zap.String("model", modelName),
		zap.String("request_body", truncateForLog(string(body), 2048)),
	)

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
	c.Logger.Debug("llm http response",
		zap.Int("status_code", resp.StatusCode),
		zap.String("response_body", truncateForLog(string(raw), 2048)),
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.LLMResponse{}, fmt.Errorf("llm upstream status %d: %s", resp.StatusCode, truncateForLog(string(raw), 1024))
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return model.LLMResponse{}, fmt.Errorf("decode llm response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return model.LLMResponse{}, errors.New("llm returned empty response")
	}

	choice := parsed.Choices[0]
	text := choice.Message.Content
	toolCalls := choice.Message.ToolCalls
	if strings.TrimSpace(text) == "" && len(toolCalls) == 0 {
		return model.LLMResponse{}, errors.New("llm returned empty response")
	}

	return model.LLMResponse{
		Text:         text,
		Model:        parsed.Model,
		ToolCalls:    toolCalls,
		FinishReason: choice.FinishReason,
	}, nil
}
