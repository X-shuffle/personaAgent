package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"persona_agent/internal/agent"
	"persona_agent/internal/model"
)

type testPersonaProvider struct{}

func (p testPersonaProvider) GetPersona(_ context.Context, _ string) (model.Persona, error) {
	return model.Persona{Tone: "warm"}, nil
}

type testPromptBuilder struct{}

func (b testPromptBuilder) Build(_ model.Persona, _ []model.Memory, _ model.EmotionState, input string) []model.LLMMessage {
	return []model.LLMMessage{{Role: "user", Content: input}}
}

type testLLMClient struct {
	fail bool
}

func (c testLLMClient) Generate(_ context.Context, _ model.LLMRequest) (model.LLMResponse, error) {
	if c.fail {
		return model.LLMResponse{}, agent.ErrUpstreamLLM
	}
	return model.LLMResponse{Text: "ok"}, nil
}

func TestChatHandler_BadJSON(t *testing.T) {
	h := ChatHandler{}
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewBufferString("{"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
