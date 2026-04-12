package main

import (
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"persona_agent/internal/agent"
	"persona_agent/internal/api"
	"persona_agent/internal/config"
	"persona_agent/internal/emotion"
	"persona_agent/internal/ingestion"
	"persona_agent/internal/llm"
	"persona_agent/internal/memory"
	"persona_agent/internal/persona"
	"persona_agent/internal/prompt"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger, err := newLogger(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	var llmClient llm.Client
	if cfg.LLMMode == "http" {
		llmClient = llm.HTTPClient{Endpoint: cfg.LLMEndpoint, APIKey: cfg.LLMAPIKey, Model: cfg.LLMModel, Logger: logger}
	} else {
		llmClient = llm.MockClient{}
	}

	memorySvc, ingestSvc := buildMemoryAndIngestionServices(cfg)

	emotionDetector := buildEmotionDetector(cfg, llmClient)

	orch := agent.Orchestrator{
		PersonaProvider: persona.StaticProvider{Persona: cfg.Persona},
		PromptBuilder:   prompt.DefaultBuilder{},
		MemoryService:   memorySvc,
		EmotionDetector: emotionDetector,
		LLMClient:       llmClient,
		Logger:          logger,
	}

	mux := http.NewServeMux()
	mux.Handle("/chat", api.ChatHandler{Orchestrator: orch})
	mux.Handle("/ingest", api.IngestHandler{Ingestor: ingestSvc, MaxUploadBytes: cfg.IngestMaxUploadBytes})

	addr := ":" + cfg.Port
	logger.Info("persona_agent listening", zap.String("addr", addr), zap.String("llm_mode", cfg.LLMMode))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Fatal("server exited", zap.Error(err))
	}
}

func buildMemoryAndIngestionServices(cfg config.Config) (memory.Service, ingestion.Service) {
	embedder := memory.NewHashEmbedder(cfg.MemoryVectorDim)
	store := memory.NewQdrantStore(cfg.QdrantURL, cfg.QdrantCollection, cfg.QdrantAPIKey, cfg.MemoryVectorDim)

	memorySvc := memory.NewService(store, embedder, cfg.MemoryTopK, 0, cfg.MemorySimilarityThreshold)
	if !cfg.IngestEnabled {
		return memorySvc, ingestion.NoopService{}
	}

	ingestSvc := ingestion.NewService(embedder, store, ingestion.Config{
		Enabled:            cfg.IngestEnabled,
		SegmentMaxChars:    cfg.IngestSegmentMaxChars,
		MergeWindowSeconds: cfg.IngestSegmentMergeWindowSeconds,
		EmbedBatchSize:     cfg.IngestEmbedBatchSize,
		DefaultSource:      "wechat",
		AllowedExtensions:  cfg.IngestAllowedExtensions,
	})
	return memorySvc, ingestSvc
}

func buildEmotionDetector(cfg config.Config, llmClient llm.Client) emotion.Detector {
	switch cfg.EmotionDetectMode {
	case "llm":
		return emotion.LLMDetector{Client: llmClient, Timeout: time.Duration(cfg.EmotionDetectTimeoutSeconds) * time.Second}
	default:
		return emotion.RuleDetector{}
	}
}

func newLogger(level string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()

	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	cfg.Level = zap.NewAtomicLevelAt(zapLevel)
	return cfg.Build()
}
