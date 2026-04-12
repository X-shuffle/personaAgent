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
	defaultPort     = "8080"
	modeMock        = "mock"
	emotionModeRule = "rule"
	emotionModeLLM  = "llm"
)

// Config is runtime configuration.
type Config struct {
	Port        string
	LLMMode     string
	LLMEndpoint string
	LLMAPIKey   string
	LLMModel    string
	// LLMTimeoutSeconds 控制 HTTP 模式下单次 LLM 请求超时（秒）。
	LLMTimeoutSeconds int
	LogLevel    string
	Persona     model.Persona

	EmotionDetectMode           string
	EmotionDetectTimeoutSeconds int

	MemoryTopK                int
	MemoryVectorDim           int
	MemorySimilarityThreshold float64
	// MemoryEmbedEndpoint 是 embedding API 地址，服务启动时要求必填。
	MemoryEmbedEndpoint       string
	MemoryEmbedAPIKey         string
	MemoryEmbedModel          string
	// MemoryEmbedTimeoutSeconds 控制 embedding HTTP 请求超时（秒）。
	MemoryEmbedTimeoutSeconds int
	QdrantURL                 string
	QdrantCollection          string
	QdrantAPIKey              string

	IngestEnabled                   bool
	IngestMaxUploadBytes            int64
	IngestAllowedExtensions         []string
	IngestEmbedBatchSize            int
	IngestSegmentMaxChars           int
	IngestSegmentMergeWindowSeconds int
}

type envConfig struct {
	Port           string `env:"PORT" envDefault:"8080"`
	LLMMode        string `env:"LLM_MODE" envDefault:"mock"`
	LLMEndpoint    string `env:"LLM_ENDPOINT"`
	LLMAPIKey      string `env:"LLM_API_KEY"`
	LLMModel       string `env:"LLM_MODEL" envDefault:"default"`
	LLMTimeoutSeconds int `env:"LLM_TIMEOUT_SECONDS" envDefault:"20"`
	LogLevel       string `env:"LOG_LEVEL" envDefault:"info"`
	PersonaTone    string `env:"PERSONA_TONE" envDefault:"warm"`
	PersonaStyle   string `env:"PERSONA_STYLE" envDefault:"concise"`
	PersonaValues  string `env:"PERSONA_VALUES" envDefault:"family,patience"`
	PersonaPhrases string `env:"PERSONA_PHRASES" envDefault:"慢慢来,别着急"`

	EmotionDetectMode           string `env:"EMOTION_DETECTOR_MODE" envDefault:"rule"`
	EmotionDetectTimeoutSeconds int    `env:"EMOTION_DETECT_TIMEOUT_SECONDS" envDefault:"20"`

	MemoryTopK                int     `env:"MEMORY_TOP_K" envDefault:"3"`
	MemoryVectorDim           int     `env:"MEMORY_VECTOR_DIM" envDefault:"256"`
	MemorySimilarityThreshold float64 `env:"MEMORY_SIMILARITY_THRESHOLD" envDefault:"0"`
	MemoryEmbedEndpoint       string  `env:"MEMORY_EMBED_ENDPOINT"`
	MemoryEmbedAPIKey         string  `env:"MEMORY_EMBED_API_KEY"`
	MemoryEmbedModel          string  `env:"MEMORY_EMBED_MODEL" envDefault:"bge-large-zh-v1.5"`
	MemoryEmbedTimeoutSeconds int     `env:"MEMORY_EMBED_TIMEOUT_SECONDS" envDefault:"15"`
	QdrantURL                 string  `env:"QDRANT_URL"`
	QdrantCollection          string  `env:"QDRANT_COLLECTION" envDefault:"persona_memories"`
	QdrantAPIKey              string  `env:"QDRANT_API_KEY"`

	IngestEnabled                   bool   `env:"INGEST_ENABLED" envDefault:"true"`
	IngestMaxUploadBytes            int64  `env:"INGEST_MAX_UPLOAD_BYTES" envDefault:"10485760"`
	IngestAllowedExtensions         string `env:"INGEST_ALLOWED_EXTENSIONS"`
	IngestAllowedExtLegacy          string `env:"INGEST_ALLOWED_EXT"`
	IngestEmbedBatchSize            int    `env:"INGEST_EMBED_BATCH_SIZE" envDefault:"64"`
	IngestSegmentMaxChars           int    `env:"INGEST_SEGMENT_MAX_CHARS" envDefault:"500"`
	IngestSegmentMergeWindowSeconds int    `env:"INGEST_SEGMENT_MERGE_WINDOW_SECONDS" envDefault:"90"`
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
		LLMTimeoutSeconds: e.LLMTimeoutSeconds,
		LogLevel:    normalizeLogLevel(e.LogLevel),
		Persona: model.Persona{
			Tone:    strings.TrimSpace(e.PersonaTone),
			Style:   strings.TrimSpace(e.PersonaStyle),
			Values:  splitCSV(e.PersonaValues),
			Phrases: splitCSV(e.PersonaPhrases),
		},
		EmotionDetectMode:           normalizeEmotionMode(e.EmotionDetectMode),
		EmotionDetectTimeoutSeconds: e.EmotionDetectTimeoutSeconds,
		MemoryTopK:                  e.MemoryTopK,
		MemoryVectorDim:             e.MemoryVectorDim,
		MemorySimilarityThreshold:   e.MemorySimilarityThreshold,
		MemoryEmbedEndpoint:         strings.TrimSpace(e.MemoryEmbedEndpoint),
		MemoryEmbedAPIKey:           strings.TrimSpace(e.MemoryEmbedAPIKey),
		MemoryEmbedModel:            strings.TrimSpace(e.MemoryEmbedModel),
		MemoryEmbedTimeoutSeconds:   e.MemoryEmbedTimeoutSeconds,
		QdrantURL:                   strings.TrimSpace(e.QdrantURL),
		QdrantCollection:            strings.TrimSpace(e.QdrantCollection),
		QdrantAPIKey:                strings.TrimSpace(e.QdrantAPIKey),
		IngestEnabled:               e.IngestEnabled,
		IngestMaxUploadBytes:        e.IngestMaxUploadBytes,
		IngestAllowedExtensions:     splitCSVLower(resolveIngestAllowedExtensions(e.IngestAllowedExtensions, e.IngestAllowedExtLegacy)),
		IngestEmbedBatchSize:        e.IngestEmbedBatchSize,
		IngestSegmentMaxChars:       e.IngestSegmentMaxChars,
		IngestSegmentMergeWindowSeconds: e.IngestSegmentMergeWindowSeconds,
	}

	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	if cfg.LLMMode == "" {
		cfg.LLMMode = modeMock
	}
	if cfg.LLMTimeoutSeconds <= 0 {
		// 防止误配 0/负数导致请求无超时约束。
		cfg.LLMTimeoutSeconds = 20
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
	if cfg.MemorySimilarityThreshold < 0 {
		cfg.MemorySimilarityThreshold = 0
	}
	if cfg.MemorySimilarityThreshold > 1 {
		cfg.MemorySimilarityThreshold = 1
	}
	if cfg.MemoryEmbedModel == "" {
		cfg.MemoryEmbedModel = "bge-large-zh-v1.5"
	}
	if cfg.MemoryEmbedTimeoutSeconds <= 0 {
		cfg.MemoryEmbedTimeoutSeconds = 15
	}
	if cfg.MemoryEmbedEndpoint == "" {
		// embedding 接口是记忆检索与摄入的前置依赖，缺失时直接 fail fast。
		return Config{}, fmt.Errorf("memory embed endpoint is required")
	}
	if cfg.QdrantCollection == "" {
		cfg.QdrantCollection = "persona_memories"
	}
	if cfg.IngestMaxUploadBytes <= 0 {
		cfg.IngestMaxUploadBytes = 10 * 1024 * 1024
	}
	if len(cfg.IngestAllowedExtensions) == 0 {
		cfg.IngestAllowedExtensions = []string{"txt", "json"}
	}
	if cfg.IngestEmbedBatchSize <= 0 {
		cfg.IngestEmbedBatchSize = 64
	}
	if cfg.IngestSegmentMaxChars <= 0 {
		cfg.IngestSegmentMaxChars = 500
	}
	if cfg.IngestSegmentMergeWindowSeconds <= 0 {
		cfg.IngestSegmentMergeWindowSeconds = 90
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

func splitCSVLower(v string) []string {
	parts := splitCSV(v)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.ToLower(p))
	}
	return out
}

func resolveIngestAllowedExtensions(primary, legacy string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}

	legacy = strings.TrimSpace(legacy)
	if legacy != "" {
		return legacy
	}

	return "txt,json"
}
