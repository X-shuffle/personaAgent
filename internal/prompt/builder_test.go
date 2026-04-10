package prompt

import (
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

	msgs := b.Build(persona, "你好")
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
}
