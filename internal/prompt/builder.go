package prompt

import (
	"fmt"
	"strings"

	"persona_agent/internal/emotion"
	"persona_agent/internal/model"
)

// Builder turns persona + memory + user input into model messages.
type Builder interface {
	Build(persona model.Persona, memories []model.Memory, emotionState model.EmotionState, userInput string) []model.LLMMessage
}

// DefaultBuilder is Phase 2 prompt composition.
type DefaultBuilder struct{}

func (b DefaultBuilder) Build(persona model.Persona, memories []model.Memory, emotionState model.EmotionState, userInput string) []model.LLMMessage {
	_ = b
	personaCtx := fmt.Sprintf(
		"You are a persona-driven assistant.\nTone: %s\nStyle: %s\nValues: %s\nPreferred phrases: %s",
		persona.Tone,
		persona.Style,
		strings.Join(persona.Values, ", "),
		strings.Join(persona.Phrases, ", "),
	)

	personaCtx += fmt.Sprintf("\n\nEmotion: %s",
		emotion.NormalizeLabel(emotionState.Label),
	)

	if len(memories) > 0 {
		items := make([]string, 0, len(memories))
		for _, m := range memories {
			if strings.TrimSpace(m.Content) == "" {
				continue
			}
			item := m.Content
			items = append(items, item)
		}
		if len(items) > 0 {
			personaCtx += "\n\nMemories:\n" + strings.Join(items, "\n")
		}
	}

	return []model.LLMMessage{
		{Role: "system", Content: personaCtx},
		{Role: "user", Content: userInput},
	}
}
