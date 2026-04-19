package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"persona_agent/internal/agent/ports"
	"persona_agent/internal/agent/tooling"
	"persona_agent/internal/emotion"
	"persona_agent/internal/llm"
	"persona_agent/internal/memory"
	"persona_agent/internal/model"
	"persona_agent/internal/persona"
	"persona_agent/internal/prompt"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrUpstreamLLM        = errors.New("llm upstream error")
	storeTurnAsyncTimeout = 5 * time.Second
)

const (
	defaultToolMaxExecRounds = 3
	toolCallTimeout          = 8 * time.Second
)

// Orchestrator 负责编排一次对话请求的完整链路：
// 人设/记忆/情绪 -> Prompt -> LLM -> 工具调用循环 -> 最终文本。
type Orchestrator struct {
	PersonaProvider   persona.Provider
	PromptBuilder     prompt.Builder
	MemoryService     memory.Service
	EmotionDetector   emotion.Detector
	ToolCaller        ports.ToolCaller
	ToolCatalog       ports.ToolCatalogProvider
	ToolMaxExecRounds int
	LLMClient         llm.Client
	Logger            *zap.Logger
}

// NewOrchestrator 在初始化阶段强制校验 logger，避免运行期出现分支判空。
func NewOrchestrator(o Orchestrator) Orchestrator {
	if o.Logger == nil {
		panic("agent orchestrator logger is nil")
	}
	return o
}

// Chat 处理单轮用户输入，并在需要时触发多轮 tool-calling。
func (o Orchestrator) Chat(ctx context.Context, sessionID, message string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	message = strings.TrimSpace(message)
	if sessionID == "" || message == "" {
		return "", fmt.Errorf("%w: session_id and message are required", ErrInvalidInput)
	}

	p, err := o.PersonaProvider.GetPersona(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("get persona: %w", err)
	}

	detectedEmotion := emotion.DefaultEmotion()
	var memories []model.Memory

	type emotionResult struct {
		state model.EmotionState
		err   error
	}
	var emotionCh chan emotionResult
	if o.EmotionDetector != nil {
		emotionCh = make(chan emotionResult, 1)
		go func() {
			state, detectErr := o.EmotionDetector.Detect(ctx, message)
			emotionCh <- emotionResult{state: state, err: detectErr}
		}()
	}

	type memoryResult struct {
		memories []model.Memory
		err      error
	}
	var memoryCh chan memoryResult
	if o.MemoryService != nil {
		memoryCh = make(chan memoryResult, 1)
		go func() {
			retrieved, retrieveErr := o.MemoryService.Retrieve(ctx, sessionID, message)
			memoryCh <- memoryResult{memories: retrieved, err: retrieveErr}
		}()
	}

	if emotionCh != nil {
		result := <-emotionCh
		if result.err != nil {
			o.Logger.Warn("emotion detect failed", zap.String("session_id", sessionID), zap.Error(result.err))
		} else {
			detectedEmotion = result.state
		}
	}
	if memoryCh != nil {
		result := <-memoryCh
		if result.err != nil {
			o.Logger.Warn("memory retrieve failed", zap.String("session_id", sessionID), zap.Error(result.err))
		} else {
			memories = result.memories
		}
	}

	messages := o.PromptBuilder.Build(p, memories, detectedEmotion, message)
	responseText, err := o.generateChatResponse(ctx, messages)
	if err != nil {
		return "", err
	}

	if o.MemoryService != nil {
		go func(sessionID, message, responseText string, detectedEmotion model.EmotionState) {
			storeCtx, cancel := context.WithTimeout(context.Background(), storeTurnAsyncTimeout)
			defer cancel()
			if err := o.MemoryService.StoreTurn(storeCtx, sessionID, message, responseText, detectedEmotion); err != nil {
				o.Logger.Warn("memory store failed", zap.String("session_id", sessionID), zap.Error(err))
				return
			}
			o.Logger.Debug("memory store succeeded", zap.String("session_id", sessionID), zap.String("emotion", detectedEmotion.Label))
		}(sessionID, message, responseText, detectedEmotion)
	}

	return responseText, nil
}

// generateChatResponse 执行 function-calling 主循环：
// 有 tool_calls 就执行工具并回填；无 tool_calls 且有文本就结束。
func (o Orchestrator) generateChatResponse(ctx context.Context, baseMessages []model.LLMMessage) (string, error) {
	catalog := map[string][]mcptypes.Tool{}
	if o.ToolCatalog != nil {
		catalog = o.ToolCatalog.ToolCatalog()
	}
	tools := buildLLMTools(catalog)
	if o.ToolCaller == nil || len(tools) == 0 {
		resp, err := o.LLMClient.Generate(ctx, model.LLMRequest{Messages: baseMessages})
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrUpstreamLLM, err)
		}
		if strings.TrimSpace(resp.Text) == "" {
			return "", fmt.Errorf("%w: empty text", ErrUpstreamLLM)
		}
		return resp.Text, nil
	}

	maxRounds := o.ToolMaxExecRounds
	if maxRounds <= 0 {
		maxRounds = defaultToolMaxExecRounds
	}

	messages := append([]model.LLMMessage(nil), baseMessages...)
	for round := 1; round <= maxRounds; round++ {
		resp, err := o.LLMClient.Generate(ctx, model.LLMRequest{
			Messages:   messages,
			Tools:      tools,
			ToolChoice: "auto",
		})
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrUpstreamLLM, err)
		}

		if len(resp.ToolCalls) == 0 {
			if strings.TrimSpace(resp.Text) == "" {
				return "", fmt.Errorf("%w: empty text", ErrUpstreamLLM)
			}
			return resp.Text, nil
		}

		normalizedToolCalls := tooling.NormalizeToolCalls(resp.ToolCalls)
		messages = append(messages, model.LLMMessage{
			Role:      "assistant",
			ToolCalls: normalizedToolCalls,
		})
		toolMsgs := o.executeNativeToolCalls(ctx, round, resp.ToolCalls)
		messages = append(messages, toolMsgs...)
	}

	return "", fmt.Errorf("%w: max tool rounds exceeded", ErrUpstreamLLM)
}

// executeNativeToolCalls 并行执行一轮工具调用，并按原顺序返回 tool 消息。
func (o Orchestrator) executeNativeToolCalls(ctx context.Context, round int, calls []model.LLMToolCall) []model.LLMMessage {
	results := make([]model.LLMMessage, len(calls))
	var wg sync.WaitGroup
	for i := range calls {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = o.executeSingleNativeToolCall(ctx, round, calls[idx])
		}(i)
	}
	wg.Wait()
	return results
}

func (o Orchestrator) executeSingleNativeToolCall(ctx context.Context, round int, call model.LLMToolCall) model.LLMMessage {
	return tooling.ExecuteSingleCall(ctx, round, call, o.ToolCaller, o.Logger, toolCallTimeout)
}

func buildLLMTools(catalog map[string][]mcptypes.Tool) []model.LLMTool {
	if len(catalog) == 0 {
		return nil
	}
	servers := make([]string, 0, len(catalog))
	for server := range catalog {
		servers = append(servers, server)
	}
	sort.Strings(servers)

	tools := make([]model.LLMTool, 0)
	for _, server := range servers {
		entries := catalog[server]
		sort.Slice(entries, func(i, j int) bool {
			return strings.TrimSpace(entries[i].Name) < strings.TrimSpace(entries[j].Name)
		})
		for _, tool := range entries {
			name := strings.TrimSpace(tool.Name)
			if name == "" {
				continue
			}
			tools = append(tools, model.LLMTool{
				Type: "function",
				Function: model.LLMFunctionSpec{
					Name:        tooling.EncodeFunctionName(server, name),
					Description: strings.TrimSpace(tool.Description),
					Parameters:  extractToolParameters(tool),
				},
			})
		}
	}
	return tools
}

func extractToolParameters(tool mcptypes.Tool) map[string]any {
	if len(tool.RawInputSchema) > 0 {
		var raw map[string]any
		if err := json.Unmarshal(tool.RawInputSchema, &raw); err == nil && len(raw) > 0 {
			return raw
		}
	}

	b, err := json.Marshal(tool.InputSchema)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}

	var params map[string]any
	if err := json.Unmarshal(b, &params); err != nil || len(params) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return params
}

