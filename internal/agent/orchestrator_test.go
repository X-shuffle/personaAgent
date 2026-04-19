package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"persona_agent/internal/agent/tooling"
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
	lastPersona  model.Persona
	lastMemories []model.Memory
	lastEmotion  model.EmotionState
}

func (f *fakePromptBuilder) Build(persona model.Persona, memories []model.Memory, emotion model.EmotionState, userInput string) []model.LLMMessage {
	f.lastPersona = persona
	f.lastMemories = append([]model.Memory(nil), memories...)
	f.lastEmotion = emotion
	return []model.LLMMessage{{Role: "user", Content: userInput}}
}

type fakeLLMClient struct {
	responses []model.LLMResponse
	err       error
	calls     int
	requests  []model.LLMRequest
}

func (f *fakeLLMClient) Generate(_ context.Context, req model.LLMRequest) (model.LLMResponse, error) {
	f.calls++
	f.requests = append(f.requests, req)
	if f.err != nil {
		return model.LLMResponse{}, f.err
	}
	if len(f.responses) == 0 {
		return model.LLMResponse{}, nil
	}
	if f.calls <= len(f.responses) {
		return f.responses[f.calls-1], nil
	}
	return f.responses[len(f.responses)-1], nil
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

type fakeToolCaller struct {
	mu      sync.Mutex
	result  *mcptypes.CallToolResult
	err     error
	calls   int
	records []toolCallRecord
	callFn  func(serverName, toolName string, args map[string]any) (*mcptypes.CallToolResult, error)
}

type toolCallRecord struct {
	server string
	tool   string
	args   map[string]any
}

func (f *fakeToolCaller) CallTool(_ context.Context, serverName, toolName string, args map[string]any) (*mcptypes.CallToolResult, error) {
	f.mu.Lock()
	f.calls++
	f.records = append(f.records, toolCallRecord{server: serverName, tool: toolName, args: args})
	f.mu.Unlock()

	if f.callFn != nil {
		return f.callFn(serverName, toolName, args)
	}
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &mcptypes.CallToolResult{}, nil
}

type fakeToolCatalog struct {
	catalog map[string][]mcptypes.Tool
}

func (f fakeToolCatalog) ToolCatalog() map[string][]mcptypes.Tool {
	out := make(map[string][]mcptypes.Tool, len(f.catalog))
	for server, tools := range f.catalog {
		cloned := make([]mcptypes.Tool, len(tools))
		copy(cloned, tools)
		out[server] = cloned
	}
	return out
}

func newTestOrchestrator(base Orchestrator) Orchestrator {
	base.Logger = zap.NewNop()
	return NewOrchestrator(base)
}

func TestOrchestratorChat_NativeFinalWithoutToolCall(t *testing.T) {
	llm := &fakeLLMClient{responses: []model.LLMResponse{{Text: "hello"}}}
	toolCaller := &fakeToolCaller{}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider:   fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:     &fakePromptBuilder{},
		ToolCaller:        toolCaller,
		ToolCatalog:       fakeToolCatalog{catalog: map[string][]mcptypes.Tool{"demo_server": {{Name: "weather"}}}},
		ToolMaxExecRounds: 3,
		LLMClient:         llm,
	})

	got, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
	if toolCaller.calls != 0 {
		t.Fatalf("expected no tool calls, got %d", toolCaller.calls)
	}
	if llm.calls != 1 {
		t.Fatalf("expected 1 llm call, got %d", llm.calls)
	}
}

func TestOrchestratorChat_NativeSingleRoundMultiTool(t *testing.T) {
	llm := &fakeLLMClient{responses: []model.LLMResponse{
		{ToolCalls: []model.LLMToolCall{
			{ID: "c1", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "weather"), Arguments: `{"city":"shanghai"}`}},
			{ID: "c2", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "time"), Arguments: `{}`}},
		}},
		{Text: "hello"},
	}}
	toolCaller := &fakeToolCaller{result: &mcptypes.CallToolResult{Content: []mcptypes.Content{mcptypes.TextContent{Type: "text", Text: "ok"}}}}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider:   fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:     &fakePromptBuilder{},
		ToolCaller:        toolCaller,
		ToolCatalog:       fakeToolCatalog{catalog: map[string][]mcptypes.Tool{"demo_server": {{Name: "weather"}, {Name: "time"}}}},
		ToolMaxExecRounds: 3,
		LLMClient:         llm,
	})

	got, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
	if toolCaller.calls != 2 {
		t.Fatalf("expected 2 tool calls, got %d", toolCaller.calls)
	}
	if llm.calls != 2 {
		t.Fatalf("expected 2 llm calls, got %d", llm.calls)
	}
}

func TestOrchestratorChat_NativeMultiRound(t *testing.T) {
	llm := &fakeLLMClient{responses: []model.LLMResponse{
		{ToolCalls: []model.LLMToolCall{{ID: "r1", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "weather"), Arguments: `{}`}}}},
		{ToolCalls: []model.LLMToolCall{{ID: "r2", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "news"), Arguments: `{}`}}}},
		{Text: "hello"},
	}}
	toolCaller := &fakeToolCaller{result: &mcptypes.CallToolResult{Content: []mcptypes.Content{mcptypes.TextContent{Type: "text", Text: "ok"}}}}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider:   fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:     &fakePromptBuilder{},
		ToolCaller:        toolCaller,
		ToolCatalog:       fakeToolCatalog{catalog: map[string][]mcptypes.Tool{"demo_server": {{Name: "weather"}, {Name: "news"}}}},
		ToolMaxExecRounds: 3,
		LLMClient:         llm,
	})

	got, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
	if toolCaller.calls != 2 {
		t.Fatalf("expected 2 tool calls, got %d", toolCaller.calls)
	}
	if llm.calls != 3 {
		t.Fatalf("expected 3 llm calls, got %d", llm.calls)
	}
}

func TestOrchestratorChat_NativeRespectsMaxRounds(t *testing.T) {
	llm := &fakeLLMClient{responses: []model.LLMResponse{
		{ToolCalls: []model.LLMToolCall{{ID: "r1", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "weather"), Arguments: `{}`}}}},
		{ToolCalls: []model.LLMToolCall{{ID: "r2", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "news"), Arguments: `{}`}}}},
		{ToolCalls: []model.LLMToolCall{{ID: "r3", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "stocks"), Arguments: `{}`}}}},
	}}
	toolCaller := &fakeToolCaller{result: &mcptypes.CallToolResult{Content: []mcptypes.Content{mcptypes.TextContent{Type: "text", Text: "ok"}}}}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider:   fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:     &fakePromptBuilder{},
		ToolCaller:        toolCaller,
		ToolCatalog:       fakeToolCatalog{catalog: map[string][]mcptypes.Tool{"demo_server": {{Name: "weather"}, {Name: "news"}, {Name: "stocks"}}}},
		ToolMaxExecRounds: 2,
		LLMClient:         llm,
	})

	_, err := o.Chat(context.Background(), "s1", "hi")
	if !errors.Is(err, ErrUpstreamLLM) {
		t.Fatalf("expected ErrUpstreamLLM, got %v", err)
	}
	if toolCaller.calls != 2 {
		t.Fatalf("expected 2 tool calls due to max rounds, got %d", toolCaller.calls)
	}
}

func TestOrchestratorChat_NativeInvalidToolArgsDegradesGracefully(t *testing.T) {
	llm := &fakeLLMClient{responses: []model.LLMResponse{
		{ToolCalls: []model.LLMToolCall{{ID: "c1", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "weather"), Arguments: `{invalid`}}}},
		{Text: "hello"},
	}}
	toolCaller := &fakeToolCaller{}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider:   fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:     &fakePromptBuilder{},
		ToolCaller:        toolCaller,
		ToolCatalog:       fakeToolCatalog{catalog: map[string][]mcptypes.Tool{"demo_server": {{Name: "weather"}}}},
		ToolMaxExecRounds: 3,
		LLMClient:         llm,
	})

	got, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
	if toolCaller.calls != 0 {
		t.Fatalf("expected no actual tool call on invalid args, got %d", toolCaller.calls)
	}
}

func TestOrchestratorChat_NativePartialToolFailureDegradesGracefully(t *testing.T) {
	llm := &fakeLLMClient{responses: []model.LLMResponse{
		{ToolCalls: []model.LLMToolCall{
			{ID: "c1", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "weather"), Arguments: `{}`}},
			{ID: "c2", Type: "function", Function: model.LLMFunctionCall{Name: tooling.EncodeFunctionName("demo_server", "news"), Arguments: `{}`}},
		}},
		{Text: "hello"},
	}}
	toolCaller := &fakeToolCaller{callFn: func(serverName, toolName string, args map[string]any) (*mcptypes.CallToolResult, error) {
		if toolName == "news" {
			return nil, errors.New("tool failed")
		}
		return &mcptypes.CallToolResult{Content: []mcptypes.Content{mcptypes.TextContent{Type: "text", Text: "ok"}}}, nil
	}}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider:   fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:     &fakePromptBuilder{},
		ToolCaller:        toolCaller,
		ToolCatalog:       fakeToolCatalog{catalog: map[string][]mcptypes.Tool{"demo_server": {{Name: "weather"}, {Name: "news"}}}},
		ToolMaxExecRounds: 3,
		LLMClient:         llm,
	})

	got, err := o.Chat(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
	if toolCaller.calls != 2 {
		t.Fatalf("expected both tools attempted, got %d", toolCaller.calls)
	}
}

func TestOrchestratorChat_OK(t *testing.T) {
	builder := &fakePromptBuilder{}
	memorySvc := &fakeMemoryService{retrieveResult: []model.Memory{{Content: "m1"}}, storeCh: make(chan struct{}, 1)}
	llm := &fakeLLMClient{responses: []model.LLMResponse{{Text: "hello"}}}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		EmotionDetector: fakeEmotionDetector{state: model.EmotionState{Label: "sad", Intensity: 0.8}},
		LLMClient:       llm,
	})

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
	llm := &fakeLLMClient{responses: []model.LLMResponse{{Text: "hello"}}}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		EmotionDetector: fakeEmotionDetector{err: errors.New("fail")},
		LLMClient:       llm,
	})

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

func TestOrchestratorChat_PersonaPhrasesSparseGating(t *testing.T) {
	builder := &fakePromptBuilder{}
	sessionID := "s1"
	message := "hi"
	basePersona := model.Persona{Tone: "warm", Phrases: []string{"慢慢来", "别着急"}}

	o := newTestOrchestrator(Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: basePersona},
		PromptBuilder:   builder,
		LLMClient:       &fakeLLMClient{responses: []model.LLMResponse{{Text: "ok"}}},
	})

	_, err := o.Chat(context.Background(), sessionID, message)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	expectInclude := shouldIncludePersonaPhrases(sessionID, message)
	if expectInclude {
		if len(builder.lastPersona.Phrases) != len(basePersona.Phrases) {
			t.Fatalf("expected phrases included, got %v", builder.lastPersona.Phrases)
		}
		return
	}
	if len(builder.lastPersona.Phrases) != 0 {
		t.Fatalf("expected phrases removed by sparse gating, got %v", builder.lastPersona.Phrases)
	}
}

func TestShouldIncludePersonaPhrases_Deterministic(t *testing.T) {
	sessionID := "same-session"
	message := "same-message"
	first := shouldIncludePersonaPhrases(sessionID, message)
	for i := 0; i < 10; i++ {
		if shouldIncludePersonaPhrases(sessionID, message) != first {
			t.Fatal("expected deterministic sparse gating decision")
		}
	}
}

func TestOrchestratorChat_InvalidInput(t *testing.T) {
	o := newTestOrchestrator(Orchestrator{})
	_, err := o.Chat(context.Background(), "", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestOrchestratorChat_UpstreamError(t *testing.T) {
	llm := &fakeLLMClient{err: errors.New("boom")}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   &fakePromptBuilder{},
		MemoryService:   &fakeMemoryService{},
		LLMClient:       llm,
	})
	_, err := o.Chat(context.Background(), "s1", "hi")
	if !errors.Is(err, ErrUpstreamLLM) {
		t.Fatalf("expected ErrUpstreamLLM, got %v", err)
	}
}

func TestOrchestratorChat_RetrieveFailureDegradesGracefully(t *testing.T) {
	builder := &fakePromptBuilder{}
	memorySvc := &fakeMemoryService{retrieveErr: errors.New("fail")}
	llm := &fakeLLMClient{responses: []model.LLMResponse{{Text: "hello"}}}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		LLMClient:       llm,
	})

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
	llm := &fakeLLMClient{responses: []model.LLMResponse{{Text: "hello"}}}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		LLMClient:       llm,
	})

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
	llm := &fakeLLMClient{responses: []model.LLMResponse{{Text: "hello"}}}
	o := newTestOrchestrator(Orchestrator{
		PersonaProvider: fakePersonaProvider{persona: model.Persona{Tone: "warm"}},
		PromptBuilder:   builder,
		MemoryService:   memorySvc,
		LLMClient:       llm,
	})

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
