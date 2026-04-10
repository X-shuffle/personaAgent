package llm

import (
	"context"
	"strings"

	"persona_agent/internal/model"
)

// MockClient returns deterministic output for local run/tests.
type MockClient struct{}

func (c MockClient) Generate(_ context.Context, req model.LLMRequest) (model.LLMResponse, error) {
	var userMsg string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if strings.EqualFold(req.Messages[i].Role, "user") {
			userMsg = req.Messages[i].Content
			break
		}
	}
	if userMsg == "" {
		userMsg = "你好"
	}
	return model.LLMResponse{
		Text:  "（mock）我收到了你的消息：" + userMsg,
		Model: "mock",
	}, nil
}
