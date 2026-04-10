package agent

import (
	"context"
	"errors"
	"testing"

	"persona_agent/internal/model"
)

type fakePersonaProvider struct {
	persona model.Persona
	err     error
}

func (f fakePersonaProvider) GetPersona(_ context.Context, _ string) (model.Persona, error) {
	if f.err != nil {
		return model.Persona{}, f.err
	}
	return f.persona, nil
}

type fakePromptBuilder struct{}

func (f fakePromptBuilder) Build(_ model.Persona, userInput string) []model.LLMMessage {
	return []model.LLMMessage{{Role: "user", Content: userInput}}
}

type fakeLLMClient struct {
	resp model.LLMResponse
	err  error
}

func (f fakeLLMClient) Generate(_ context.Context, _ model.LLMRequest) (model.LLMResponse, error) {
	if f.err != nil {
		return model.LLMResponse{}, f.err
	}
	return f.resp, nil
}

func TestOrchestratorChat_OK(t *testing.T) {
	o := Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   fakePromptBuilder{},
		LLMClient:       fakeLLMClient{resp: model.LLMResponse{Text: "hello"}},
	}

	got, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
}

func TestOrchestratorChat_InvalidInput(t *testing.T) {
	o := Orchestrator{}
	_, err := o.Chat(context.Background(), "", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestOrchestratorChat_UpstreamError(t *testing.T) {
	o := Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   fakePromptBuilder{},
		LLMClient:       fakeLLMClient{err: errors.New("boom")},
	}
	_, err := o.Chat(context.Background(), "s1", "hi")
	if !errors.Is(err, ErrUpstreamLLM) {
		t.Fatalf("expected ErrUpstreamLLM, got %v", err)
	}
}
