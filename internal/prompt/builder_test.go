package prompt

import (
	"strings"
	"testing"

	"persona_agent/internal/model"
)

func TestDefaultBuilder_Build(t *testing.T) {
	b := DefaultBuilder{}
	persona := model.Persona{
		Tone:    "warm",
		Style:   "concise",
		Values:  []string{"family", "patience"},
		Phrases: []string{"慢慢来", "别着急"},
	}

	msgs := b.Build(persona, nil, model.EmotionState{Label: "neutral", Intensity: 0}, "你好")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatalf("expected first role system, got %s", msgs[0].Role)
	}
	if msgs[1].Role != "user" || msgs[1].Content != "你好" {
		t.Fatalf("unexpected user message: %+v", msgs[1])
	}
	if got := msgs[0].Content; got == "" {
		t.Fatal("expected system prompt content")
	}
	if !strings.Contains(msgs[0].Content, "Persona consistency rules:") {
		t.Fatalf("expected persona consistency rules in system prompt, got: %s", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "Use at most one phrase naturally when it fits") {
		t.Fatalf("expected sparse phrase usage guard, got: %s", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "Current time:") {
		t.Fatalf("expected current time anchor in system prompt, got: %s", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "Do not assume a different current year") {
		t.Fatalf("expected date reasoning guard in system prompt, got: %s", msgs[0].Content)
	}
}

func TestDefaultBuilder_Build_WithSummaryMemory(t *testing.T) {
	b := DefaultBuilder{}
	persona := model.Persona{Tone: "warm", Style: "concise"}
	memories := []model.Memory{
		{Type: model.MemoryTypeSummary, Content: "Summary of recent session context: ...", Timestamp: 1712801000},
		{Type: model.MemoryTypeEpisodic, Content: "User prefers concise replies.", Timestamp: 1712801100},
	}

	msgs := b.Build(persona, memories, model.EmotionState{Label: "neutral", Intensity: 0.2}, "继续")
	if !strings.Contains(msgs[0].Content, "Memory summaries:") {
		t.Fatalf("expected summary section, got: %s", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "Episodic memories:") {
		t.Fatalf("expected episodic section, got: %s", msgs[0].Content)
	}
}
