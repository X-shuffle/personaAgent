package prompt

import (
	"fmt"
	"strings"
	"time"

	"persona_agent/internal/emotion"
	"persona_agent/internal/model"
)

// Builder turns persona + memory + user input into model messages.
type Builder interface {
	Build(persona model.Persona, memories []model.Memory, emotionState model.EmotionState, userInput string) []model.LLMMessage
}

// DefaultBuilder is Phase 2 prompt composition.
type DefaultBuilder struct{}

// Build 组装 system/user 两条消息：注入 persona、一致性约束、情绪与分类记忆上下文。
func (b DefaultBuilder) Build(persona model.Persona, memories []model.Memory, emotionState model.EmotionState, userInput string) []model.LLMMessage {
	_ = b
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.UTC
	}
	personaCtx := fmt.Sprintf(
		"You are a persona-driven assistant.\nTone: %s\nStyle: %s\nValues: %s",
		persona.Tone,
		persona.Style,
		strings.Join(persona.Values, ", "),
	)
	personaCtx += "\n\nPersona consistency rules: Keep tone/style/values consistent across turns. Do not contradict stated values. Keep phrase cues optional and natural; never force them."
	if len(persona.Phrases) > 0 {
		personaCtx += fmt.Sprintf("\nOptional phrase cues: %s\nUse at most one phrase naturally when it fits. Avoid repetition across turns.", strings.Join(persona.Phrases, ", "))
	}

	personaCtx += fmt.Sprintf("\n\nEmotion: %s (intensity=%.2f)",
		emotion.NormalizeLabel(emotionState.Label),
		normalizeIntensity(emotionState.Intensity),
	)
	personaCtx += fmt.Sprintf("\nCurrent time: %s", time.Now().In(loc).Format("2006-01-02 15:04:05 -07:00"))
	personaCtx += "\nWhen user mentions dates/times, reason relative to Current time above and memory timestamps. Do not assume a different current year."

	if len(memories) > 0 {
		summaryItems := make([]string, 0, len(memories))
		episodicItems := make([]string, 0, len(memories))
		for _, m := range memories {
			if strings.TrimSpace(m.Content) == "" {
				continue
			}
			item := m.Content
			if m.Timestamp > 0 {
				item = fmt.Sprintf("%s | %s", time.Unix(m.Timestamp, 0).In(loc).Format("2006-01-02 15:04:05 -07:00"), item)
			}
			if m.Type == model.MemoryTypeSummary {
				summaryItems = append(summaryItems, item)
				continue
			}
			episodicItems = append(episodicItems, item)
		}
		if len(summaryItems) > 0 {
			personaCtx += "\n\nMemory summaries:\n" + strings.Join(summaryItems, "\n")
		}
		if len(episodicItems) > 0 {
			personaCtx += "\n\nEpisodic memories:\n" + strings.Join(episodicItems, "\n")
		}
	}

	return []model.LLMMessage{
		{Role: "system", Content: personaCtx},
		{Role: "user", Content: userInput},
	}
}

// normalizeIntensity 将情绪强度归一化到 [0,1]。
func normalizeIntensity(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
