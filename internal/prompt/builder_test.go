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

	msgs := b.Build(persona, nil, "你好")
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

func TestDefaultBuilder_Build_WithMemory(t *testing.T) {
	b := DefaultBuilder{}
	persona := model.Persona{Tone: "warm", Style: "concise"}
	memories := []model.Memory{
		{Content: "User likes morning runs."},
		{Content: "User prefers concise replies."},
	}

	msgs := b.Build(persona, memories, "你记得我吗")
	if !strings.Contains(msgs[0].Content, "Relevant Memory") {
		t.Fatalf("expected memory section, got: %s", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "User likes morning runs.") {
		t.Fatalf("expected first memory content in system prompt")
	}
}
