package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"persona_agent/internal/llm"
	"persona_agent/internal/memory"
	"persona_agent/internal/model"
	"persona_agent/internal/persona"
	"persona_agent/internal/prompt"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUpstreamLLM  = errors.New("llm upstream error")
)

// Orchestrator coordinates persona + memory + prompt + llm.
type Orchestrator struct {
	PersonaProvider persona.Provider
	PromptBuilder   prompt.Builder
	MemoryService   memory.Service
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

	var memories []model.Memory
	if o.MemoryService != nil {
		memories, err = o.MemoryService.Retrieve(ctx, sessionID, message)
		if err != nil {
			if o.Logger != nil {
				o.Logger.Warn("memory retrieve failed", zap.String("session_id", sessionID), zap.Error(err))
			}
			memories = nil
		}
	}

	messages := o.PromptBuilder.Build(p, memories, message)
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
		if err := o.MemoryService.StoreTurn(ctx, sessionID, message, resp.Text); err != nil {
			if o.Logger != nil {
				o.Logger.Warn("memory store failed", zap.String("session_id", sessionID), zap.Error(err))
			}
		}
	}

	return resp.Text, nil
}
