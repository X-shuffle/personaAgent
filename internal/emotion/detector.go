package emotion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"persona_agent/internal/llm"
	"persona_agent/internal/model"
)

const (
	DefaultDetectTimeout = 20 * time.Second

	LabelSad     = "sad"
	LabelAngry   = "angry"
	LabelAnxious = "anxious"
	LabelHappy   = "happy"
	LabelNeutral = "neutral"
)

var validLabels = map[string]struct{}{
	LabelSad:     {},
	LabelAngry:   {},
	LabelAnxious: {},
	LabelHappy:   {},
	LabelNeutral: {},
}

// Detector classifies user input into a constrained emotion state.
type Detector interface {
	Detect(ctx context.Context, userInput string) (model.EmotionState, error)
}

// LLMDetector uses the shared llm.Client to classify the current turn emotion.
type LLMDetector struct {
	Client  llm.Client
	Timeout time.Duration
}

func (d LLMDetector) Detect(ctx context.Context, userInput string) (model.EmotionState, error) {
	userInput = strings.TrimSpace(userInput)
	if userInput == "" {
		return DefaultEmotion(), nil
	}
	if d.Client == nil {
		return DefaultEmotion(), fmt.Errorf("emotion llm client is nil")
	}

	timeout := d.Timeout
	if timeout <= 0 {
		timeout = DefaultDetectTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := d.Client.Generate(ctx, model.LLMRequest{
		Messages: []model.LLMMessage{
			{Role: "system", Content: emotionClassifierPrompt},
			{Role: "user", Content: userInput},
		},
		Temperature: 0,
		MaxTokens:   80,
	})
	if err != nil {
		return DefaultEmotion(), err
	}

	state, err := parseEmotionState(resp.Text)
	if err != nil {
		return DefaultEmotion(), err
	}
	return state, nil
}

func DefaultEmotion() model.EmotionState {
	return model.EmotionState{Label: LabelNeutral, Intensity: 0}
}

func NormalizeLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return LabelNeutral
	}
	return label
}

func Guidance(label string) string {
	switch NormalizeLabel(label) {
	case LabelSad:
		return "Respond gently, validate the feeling, and offer calm support."
	case LabelAngry:
		return "Stay calm, avoid escalation, and keep the response steady and concise."
	case LabelAnxious:
		return "Reduce uncertainty, be reassuring, and break advice into clear steps."
	case LabelHappy:
		return "Match the positive tone while staying grounded and helpful."
	default:
		return "Keep the response natural and aligned with the persona."
	}
}

func parseEmotionState(raw string) (model.EmotionState, error) {
	var state model.EmotionState
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &state); err != nil {
		return DefaultEmotion(), fmt.Errorf("decode emotion json: %w", err)
	}

	state.Label = NormalizeLabel(state.Label)
	if _, ok := validLabels[state.Label]; !ok {
		return DefaultEmotion(), fmt.Errorf("invalid emotion label: %q", state.Label)
	}
	if state.Intensity < 0 {
		state.Intensity = 0
	}
	if state.Intensity > 1 {
		state.Intensity = 1
	}
	return state, nil
}

const emotionClassifierPrompt = "You classify the user's current emotion. Return JSON only with schema {\"label\":\"sad|angry|anxious|happy|neutral\",\"intensity\":0..1}. Choose the dominant current emotion from the user's message. Do not add explanations or markdown."
