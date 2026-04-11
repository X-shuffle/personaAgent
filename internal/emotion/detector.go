package emotion

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

// RuleDetector classifies emotion locally with keyword rules.
type RuleDetector struct{}

var (
	sadKeywords     = []string{"难过", "伤心", "失落", "沮丧", "委屈", "心累", "想哭", "低落"}
	angryKeywords   = []string{"生气", "愤怒", "火大", "气死", "烦死", "恼火", "暴躁", "怒"}
	anxiousKeywords = []string{"焦虑", "紧张", "担心", "不安", "害怕", "慌", "压力大", "忐忑"}
	happyKeywords   = []string{"开心", "高兴", "快乐", "兴奋", "满足", "轻松", "放心", "不错"}

	intensifiers = []string{"非常", "特别", "很", "太", "极其", "超级", "真的"}
	negations    = []string{"不", "没", "无", "别"}

	emotionPriority = []string{LabelAngry, LabelAnxious, LabelSad, LabelHappy}
)

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

func (d RuleDetector) Detect(_ context.Context, userInput string) (model.EmotionState, error) {
	_ = d
	text := strings.ToLower(strings.TrimSpace(userInput))
	if text == "" {
		return DefaultEmotion(), nil
	}

	scores := map[string]float64{
		LabelSad:     scoreByKeywords(text, sadKeywords),
		LabelAngry:   scoreByKeywords(text, angryKeywords),
		LabelAnxious: scoreByKeywords(text, anxiousKeywords),
		LabelHappy:   scoreByKeywords(text, happyKeywords),
	}
	intensifierCount := countMatches(text, intensifiers)

	if hasNegatedMatch(text, happyKeywords) {
		scores[LabelHappy] -= 0.8
		scores[LabelSad] += 0.4
	}
	if hasNegatedMatch(text, anxiousKeywords) {
		scores[LabelAnxious] -= 0.6
	}
	if hasNegatedMatch(text, sadKeywords) {
		scores[LabelSad] -= 0.6
	}

	for label, score := range scores {
		if score > 0 && intensifierCount > 0 {
			scores[label] += math.Min(0.4*float64(intensifierCount), 1.2)
		}
	}

	label, topScore, secondScore := pickTopLabel(scores)
	if topScore <= 0 {
		return DefaultEmotion(), nil
	}
	if topScore-secondScore < 0.35 {
		for _, candidate := range emotionPriority {
			if scores[candidate] == topScore {
				label = candidate
				break
			}
		}
	}

	intensity := 0.35 + 0.18*(topScore-1) + 0.08*float64(intensifierCount)
	if strings.Contains(text, "!!!") || strings.Contains(text, "气死") || strings.Contains(text, "崩溃") {
		intensity += 0.12
	}
	intensity = clamp01(intensity)

	return model.EmotionState{Label: label, Intensity: intensity}, nil
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

func scoreByKeywords(text string, keywords []string) float64 {
	score := 0.0
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			score += 1
		}
	}
	return score
}

func countMatches(text string, tokens []string) int {
	count := 0
	for _, token := range tokens {
		count += strings.Count(text, token)
	}
	return count
}

func hasNegatedMatch(text string, keywords []string) bool {
	for _, neg := range negations {
		for _, kw := range keywords {
			if strings.Contains(text, neg+kw) {
				return true
			}
		}
	}
	return false
}

func pickTopLabel(scores map[string]float64) (label string, topScore float64, secondScore float64) {
	label = LabelNeutral
	for _, candidate := range emotionPriority {
		score := scores[candidate]
		if score > topScore {
			secondScore = topScore
			topScore = score
			label = candidate
			continue
		}
		if score > secondScore {
			secondScore = score
		}
	}
	return label, topScore, secondScore
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
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
