package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"persona_agent/internal/llm"
	"persona_agent/internal/model"
	"persona_agent/internal/persona"
	"persona_agent/internal/prompt"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUpstreamLLM  = errors.New("llm upstream error")
)

// Orchestrator coordinates persona + prompt + llm.
type Orchestrator struct {
	PersonaProvider persona.Provider
	PromptBuilder   prompt.Builder
	LLMClient       llm.Client
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

	messages := o.PromptBuilder.Build(p, message)
	resp, err := o.LLMClient.Generate(ctx, model.LLMRequest{
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUpstreamLLM, err)
	}

	if strings.TrimSpace(resp.Text) == "" {
		return "", fmt.Errorf("%w: empty text", ErrUpstreamLLM)
	}
	return resp.Text, nil
}
