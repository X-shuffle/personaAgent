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

type fakePromptBuilder struct {
	lastMemories []model.Memory
}

func (f *fakePromptBuilder) Build(_ model.Persona, memories []model.Memory, userInput string) []model.LLMMessage {
	f.lastMemories = append([]model.Memory(nil), memories...)
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

type fakeMemoryService struct {
	retrieveResult []model.Memory
	retrieveErr    error
	storeErr       error
	storeCalls     int
	retrieveCalls  int
}

func (f *fakeMemoryService) Retrieve(_ context.Context, _ string, _ string) ([]model.Memory, error) {
	f.retrieveCalls++
	if f.retrieveErr != nil {
		return nil, f.retrieveErr
	}
	return f.retrieveResult, nil
}

func (f *fakeMemoryService) StoreTurn(_ context.Context, _ string, _ string, _ string) error {
	f.storeCalls++
	if f.storeErr != nil {
		return f.storeErr
	}
	return nil
}

func TestOrchestratorChat_OK(t *testing.T) {
	builder := &fakePromptBuilder{}
	memorySvc := &fakeMemoryService{retrieveResult: []model.Memory{{Content: "m1"}}}
	o := Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		LLMClient:       fakeLLMClient{resp: model.LLMResponse{Text: "hello"}},
	}

	got, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
	if memorySvc.retrieveCalls != 1 {
		t.Fatalf("expected retrieve calls=1, got %d", memorySvc.retrieveCalls)
	}
	if memorySvc.storeCalls != 1 {
		t.Fatalf("expected store calls=1, got %d", memorySvc.storeCalls)
	}
	if len(builder.lastMemories) != 1 {
		t.Fatalf("expected builder to receive retrieved memories")
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
	builder := &fakePromptBuilder{}
	o := Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   &fakeMemoryService{},
		LLMClient:       fakeLLMClient{err: errors.New("boom")},
	}
	_, err := o.Chat(context.Background(), "s1", "hi")
	if !errors.Is(err, ErrUpstreamLLM) {
		t.Fatalf("expected ErrUpstreamLLM, got %v", err)
	}
}

func TestOrchestratorChat_RetrieveFailureDegradesGracefully(t *testing.T) {
	builder := &fakePromptBuilder{}
	memorySvc := &fakeMemoryService{retrieveErr: errors.New("fail")}
	o := Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		LLMClient:       fakeLLMClient{resp: model.LLMResponse{Text: "hello"}},
	}

	_, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(builder.lastMemories) != 0 {
		t.Fatalf("expected empty memories when retrieve fails")
	}
}

func TestOrchestratorChat_StoreFailureDegradesGracefully(t *testing.T) {
	builder := &fakePromptBuilder{}
	memorySvc := &fakeMemoryService{storeErr: errors.New("fail")}
	o := Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		LLMClient:       fakeLLMClient{resp: model.LLMResponse{Text: "hello"}},
	}

	_, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if memorySvc.storeCalls != 1 {
		t.Fatalf("expected store to be called")
	}
}
