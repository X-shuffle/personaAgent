package prompt

import (
	"fmt"
	"strings"

	"persona_agent/internal/model"
)

// Builder turns persona + memory + user input into model messages.
type Builder interface {
	Build(persona model.Persona, memories []model.Memory, userInput string) []model.LLMMessage
}

// DefaultBuilder is Phase 2 prompt composition.
type DefaultBuilder struct{}

func (b DefaultBuilder) Build(persona model.Persona, memories []model.Memory, userInput string) []model.LLMMessage {
	_ = b
	personaCtx := fmt.Sprintf(
		"You are a persona-driven assistant.\nTone: %s\nStyle: %s\nValues: %s\nPreferred phrases: %s",
		persona.Tone,
		persona.Style,
		strings.Join(persona.Values, ", "),
		strings.Join(persona.Phrases, ", "),
	)

	if len(memories) > 0 {
		items := make([]string, 0, len(memories))
		for _, m := range memories {
			if strings.TrimSpace(m.Content) == "" {
				continue
			}
			items = append(items, "- "+m.Content)
		}
		if len(items) > 0 {
			personaCtx += "\n\nRelevant Memory:\n" + strings.Join(items, "\n")
		}
	}

	return []model.LLMMessage{
		{Role: "system", Content: personaCtx},
		{Role: "user", Content: userInput},
	}
}
