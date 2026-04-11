package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"

	"persona_agent/internal/emotion"
	"persona_agent/internal/model"
)

const (
	defaultPort      = "8080"
	modeMock           = "mock"
	emotionModeRule    = "rule"
	emotionModeLLM     = "llm"
	memoryModeOff      = "off"
	memoryModeInMem    = "inmem"
	memoryModeQdrant   = "qdrant"
)

// Config is runtime configuration.
type Config struct {
	Port        string
	LLMMode     string
	LLMEndpoint string
	LLMAPIKey   string
	LLMModel    string
	LogLevel    string
	Persona     model.Persona

	EmotionDetectorMode         string
	EmotionDetectTimeoutSeconds int

	MemoryMode       string
	MemoryTopK       int
	MemoryVectorDim  int
	QdrantURL        string
	QdrantCollection string
	QdrantAPIKey     string
}

type envConfig struct {
	Port          string `env:"PORT" envDefault:"8080"`
	LLMMode       string `env:"LLM_MODE" envDefault:"mock"`
	LLMEndpoint   string `env:"LLM_ENDPOINT"`
	LLMAPIKey     string `env:"LLM_API_KEY"`
	LLMModel      string `env:"LLM_MODEL" envDefault:"default"`
	LogLevel      string `env:"LOG_LEVEL" envDefault:"info"`
	PersonaTone   string `env:"PERSONA_TONE" envDefault:"warm"`
	PersonaStyle  string `env:"PERSONA_STYLE" envDefault:"concise"`
	PersonaValues string `env:"PERSONA_VALUES" envDefault:"family,patience"`
	PersonaPhrase string `env:"PERSONA_PHRASES" envDefault:"慢慢来,别着急"`

	EmotionDetectorMode         string `env:"EMOTION_DETECTOR_MODE" envDefault:"rule"`
	EmotionDetectTimeoutSeconds int    `env:"EMOTION_DETECT_TIMEOUT_SECONDS" envDefault:"20"`

	MemoryMode       string `env:"MEMORY_MODE" envDefault:"inmem"`
	MemoryTopK       int    `env:"MEMORY_TOP_K" envDefault:"3"`
	MemoryVectorDim  int    `env:"MEMORY_VECTOR_DIM" envDefault:"256"`
	QdrantURL        string `env:"QDRANT_URL"`
	QdrantCollection string `env:"QDRANT_COLLECTION" envDefault:"persona_memories"`
	QdrantAPIKey     string `env:"QDRANT_API_KEY"`
}

func Load() (Config, error) {
	_ = godotenv.Load()

	var e envConfig
	if err := env.Parse(&e); err != nil {
		return Config{}, fmt.Errorf("parse env config: %w", err)
	}

	cfg := Config{
		Port:        strings.TrimSpace(e.Port),
		LLMMode:     strings.TrimSpace(e.LLMMode),
		LLMEndpoint: strings.TrimSpace(e.LLMEndpoint),
		LLMAPIKey:   strings.TrimSpace(e.LLMAPIKey),
		LLMModel:    strings.TrimSpace(e.LLMModel),
		LogLevel:    normalizeLogLevel(e.LogLevel),
		Persona: model.Persona{
			Tone:    strings.TrimSpace(e.PersonaTone),
			Style:   strings.TrimSpace(e.PersonaStyle),
			Values:  splitCSV(e.PersonaValues),
			Phrases: splitCSV(e.PersonaPhrase),
		},
		EmotionDetectorMode:         normalizeEmotionMode(e.EmotionDetectorMode),
		EmotionDetectTimeoutSeconds: e.EmotionDetectTimeoutSeconds,
		MemoryMode:                  normalizeMemoryMode(e.MemoryMode),
		MemoryTopK:                  e.MemoryTopK,
		MemoryVectorDim:             e.MemoryVectorDim,
		QdrantURL:                   strings.TrimSpace(e.QdrantURL),
		QdrantCollection:            strings.TrimSpace(e.QdrantCollection),
		QdrantAPIKey:                strings.TrimSpace(e.QdrantAPIKey),
	}

	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	if cfg.LLMMode == "" {
		cfg.LLMMode = modeMock
	}
	if cfg.Persona.Tone == "" {
		cfg.Persona.Tone = "warm"
	}
	if cfg.Persona.Style == "" {
		cfg.Persona.Style = "concise"
	}
	if len(cfg.Persona.Values) == 0 {
		cfg.Persona.Values = splitCSV("family,patience")
	}
	if len(cfg.Persona.Phrases) == 0 {
		cfg.Persona.Phrases = splitCSV("慢慢来,别着急")
	}
	if cfg.EmotionDetectTimeoutSeconds <= 0 {
		cfg.EmotionDetectTimeoutSeconds = int(emotion.DefaultDetectTimeout / time.Second)
	}
	if cfg.MemoryTopK <= 0 {
		cfg.MemoryTopK = 3
	}
	if cfg.MemoryVectorDim <= 0 {
		cfg.MemoryVectorDim = 256
	}
	if cfg.QdrantCollection == "" {
		cfg.QdrantCollection = "persona_memories"
	}

	return cfg, nil
}

func normalizeLogLevel(level string) string {
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "debug", "info", "warn", "error":
		return level
	default:
		return "info"
	}
}

func normalizeEmotionMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case emotionModeRule, emotionModeLLM:
		return mode
	default:
		return emotionModeRule
	}
}

func normalizeMemoryMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case memoryModeOff, memoryModeInMem, memoryModeQdrant:
		return mode
	default:
		return memoryModeInMem
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
