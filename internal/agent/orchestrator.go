package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

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

// Orchestrator coordinates persona + memory + prompt + llm.
type Orchestrator struct {
	PersonaProvider persona.Provider
	PromptBuilder   prompt.Builder
	MemoryService   memory.Service
	EmotionDetector emotion.Detector
	LLMClient       llm.Client
	Logger          *zap.Logger
}

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
			if o.Logger != nil {
				o.Logger.Warn("emotion detect failed", zap.String("session_id", sessionID), zap.Error(result.err))
			}
		} else {
			detectedEmotion = result.state
		}
	}
	if memoryCh != nil {
		result := <-memoryCh
		if result.err != nil {
			if o.Logger != nil {
				o.Logger.Warn("memory retrieve failed", zap.String("session_id", sessionID), zap.Error(result.err))
			}
		} else {
			memories = result.memories
		}
	}

	messages := o.PromptBuilder.Build(p, memories, detectedEmotion, message)
	resp, err := o.LLMClient.Generate(ctx, model.LLMRequest{
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUpstreamLLM, err)
	}

	if strings.TrimSpace(resp.Text) == "" {
		return "", fmt.Errorf("%w: empty text", ErrUpstreamLLM)
	}

	if o.MemoryService != nil {
		go func(sessionID, message, responseText string, detectedEmotion model.EmotionState) {
			storeCtx, cancel := context.WithTimeout(context.Background(), storeTurnAsyncTimeout)
			defer cancel()
			if err := o.MemoryService.StoreTurn(storeCtx, sessionID, message, responseText, detectedEmotion); err != nil {
				if o.Logger != nil {
					o.Logger.Warn("memory store failed", zap.String("session_id", sessionID), zap.Error(err))
				}
				return
			}
			if o.Logger != nil {
				o.Logger.Debug("memory store succeeded", zap.String("session_id", sessionID), zap.String("emotion", detectedEmotion.Label))
			}
		}(sessionID, message, resp.Text, detectedEmotion)
	}

	return resp.Text, nil
}
