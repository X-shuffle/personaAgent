package prompt

import (
	"fmt"
	"strings"

	"persona_agent/internal/model"
)

// Builder turns persona + user input into model messages.
type Builder interface {
	Build(persona model.Persona, userInput string) []model.LLMMessage
}

// DefaultBuilder is Phase 1 prompt composition.
type DefaultBuilder struct{}

func (b DefaultBuilder) Build(persona model.Persona, userInput string) []model.LLMMessage {
	_ = b
	personaCtx := fmt.Sprintf(
		"You are a persona-driven assistant.\nTone: %s\nStyle: %s\nValues: %s\nPreferred phrases: %s",
		persona.Tone,
		persona.Style,
		strings.Join(persona.Values, ", "),
		strings.Join(persona.Phrases, ", "),
	)

	return []model.LLMMessage{
		{Role: "system", Content: personaCtx},
		{Role: "user", Content: userInput},
	}
}
