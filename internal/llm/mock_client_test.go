package llm

import (
	"context"
	"testing"

	"persona_agent/internal/model"
)

func TestMockClientGenerate(t *testing.T) {
	c := MockClient{}
	resp, err := c.Generate(context.Background(), model.LLMRequest{Messages: []model.LLMMessage{{Role: "user", Content: "你好"}}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Text == "" {
		t.Fatal("expected non-empty response")
	}
}
