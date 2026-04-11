package agent

import (
	"context"
	"errors"
	"testing"
	"time"

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
	lastEmotion  model.EmotionState
}

func (f *fakePromptBuilder) Build(_ model.Persona, memories []model.Memory, emotion model.EmotionState, userInput string) []model.LLMMessage {
	f.lastMemories = append([]model.Memory(nil), memories...)
	f.lastEmotion = emotion
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

type fakeEmotionDetector struct {
	state model.EmotionState
	err   error
}

func (f fakeEmotionDetector) Detect(_ context.Context, _ string) (model.EmotionState, error) {
	if f.err != nil {
		return model.EmotionState{}, f.err
	}
	return f.state, nil
}

type fakeMemoryService struct {
	retrieveResult []model.Memory
	retrieveErr    error
	storeErr       error
	storeCalls     int
	retrieveCalls  int
	lastEmotion    model.EmotionState
	storeCh        chan struct{}
	storeCtx       context.Context
}

func (f *fakeMemoryService) Retrieve(_ context.Context, _ string, _ string) ([]model.Memory, error) {
	f.retrieveCalls++
	if f.retrieveErr != nil {
		return nil, f.retrieveErr
	}
	return f.retrieveResult, nil
}

func (f *fakeMemoryService) StoreTurn(ctx context.Context, _ string, _ string, _ string, emotion model.EmotionState) error {
	f.storeCalls++
	f.lastEmotion = emotion
	f.storeCtx = ctx
	if f.storeCh != nil {
		select {
		case f.storeCh <- struct{}{}:
		default:
		}
	}
	if f.storeErr != nil {
		return f.storeErr
	}
	return nil
}

func waitForStore(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async store")
	}
}

func TestOrchestratorChat_OK(t *testing.T) {
	builder := &fakePromptBuilder{}
	memorySvc := &fakeMemoryService{retrieveResult: []model.Memory{{Content: "m1"}}, storeCh: make(chan struct{}, 1)}
	o := Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		EmotionDetector: fakeEmotionDetector{state: model.EmotionState{Label: "sad", Intensity: 0.8}},
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
	waitForStore(t, memorySvc.storeCh)
	if memorySvc.storeCalls != 1 {
		t.Fatalf("expected store calls=1, got %d", memorySvc.storeCalls)
	}
	if len(builder.lastMemories) != 1 {
		t.Fatalf("expected builder to receive retrieved memories")
	}
	if builder.lastEmotion.Label != "sad" {
		t.Fatalf("expected builder emotion sad, got %+v", builder.lastEmotion)
	}
	if memorySvc.lastEmotion.Label != "sad" {
		t.Fatalf("expected stored emotion sad, got %+v", memorySvc.lastEmotion)
	}
}

func TestOrchestratorChat_DetectFailureDegradesGracefully(t *testing.T) {
	builder := &fakePromptBuilder{}
	memorySvc := &fakeMemoryService{storeCh: make(chan struct{}, 1)}
	o := Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		EmotionDetector: fakeEmotionDetector{err: errors.New("fail")},
		LLMClient:       fakeLLMClient{resp: model.LLMResponse{Text: "hello"}},
	}

	_, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	waitForStore(t, memorySvc.storeCh)
	if builder.lastEmotion.Label != "neutral" {
		t.Fatalf("expected neutral fallback, got %+v", builder.lastEmotion)
	}
	if memorySvc.lastEmotion.Label != "neutral" {
		t.Fatalf("expected stored neutral fallback, got %+v", memorySvc.lastEmotion)
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
	memorySvc := &fakeMemoryService{storeErr: errors.New("fail"), storeCh: make(chan struct{}, 1)}
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
	waitForStore(t, memorySvc.storeCh)
	if memorySvc.storeCalls != 1 {
		t.Fatalf("expected store to be called")
	}
}

func TestOrchestratorChat_StoreUsesDetachedContext(t *testing.T) {
	builder := &fakePromptBuilder{}
	memorySvc := &fakeMemoryService{storeCh: make(chan struct{}, 1)}
	o := Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		LLMClient:       fakeLLMClient{resp: model.LLMResponse{Text: "hello"}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	_, err := o.Chat(ctx, "s1", "hi")
	cancel()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	waitForStore(t, memorySvc.storeCh)
	if memorySvc.storeCtx == nil {
		t.Fatal("expected store context to be captured")
	}
	if memorySvc.storeCalls != 1 {
		t.Fatalf("expected async store to run after request cancel")
	}
}
