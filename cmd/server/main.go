package main

import (
	"context"
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
	"persona_agent/internal/mcp/runtime"
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
		// HTTP 模式下，超时由 .env 中的 LLM_TIMEOUT_SECONDS 控制，便于按环境调参。
		llmClient = llm.NewHTTPClient(llm.HTTPClient{
			Endpoint:                 cfg.LLMEndpoint,
			APIKey:                   cfg.LLMAPIKey,
			Model:                    cfg.LLMModel,
			Provider:                 cfg.LLMProvider,
			ToolPayloadNormalization: cfg.LLMToolPayloadNormalization,
			HTTPClient:               &http.Client{Timeout: time.Duration(cfg.LLMTimeoutSeconds) * time.Second},
			Logger:                   logger,
		})
	} else {
		llmClient = llm.MockClient{}
	}

	mcpManager := runtime.NewManager(logger)
	mcpReport := mcpManager.Start(context.Background(), cfg.ActiveMCPServers)
	defer func() {
		if err := mcpManager.Close(); err != nil {
			logger.Warn("mcp manager close failed", zap.Error(err))
		}
	}()

	memorySvc, ingestSvc := buildMemoryAndIngestionServices(cfg, logger)

	emotionDetector := buildEmotionDetector(cfg, llmClient)

	orch := agent.NewOrchestrator(agent.Orchestrator{
		PersonaProvider:   persona.StaticProvider{Persona: cfg.Persona},
		PromptBuilder:     prompt.DefaultBuilder{},
		MemoryService:     memorySvc,
		EmotionDetector:   emotionDetector,
		ToolCaller:        mcpManager,
		ToolCatalog:       mcpManager,
		ToolMaxExecRounds: cfg.ToolMaxExecRounds,
		LLMClient:         llmClient,
		Logger:            logger,
	})

	mux := http.NewServeMux()
	mux.Handle("/chat", api.ChatHandler{Orchestrator: orch})
	mux.Handle("/ingest", api.IngestHandler{Ingestor: ingestSvc, MaxUploadBytes: cfg.IngestMaxUploadBytes})

	addr := ":" + cfg.Port
	logger.Info(
		"persona_agent listening",
		zap.String("addr", addr),
		zap.String("llm_mode", cfg.LLMMode),
		zap.Int("mcp_servers_total", len(cfg.MCPServers)),
		zap.Int("mcp_servers_active", len(cfg.ActiveMCPServers)),
		zap.Int("mcp_servers_configured", mcpReport.Configured),
		zap.Int("mcp_servers_connected", mcpReport.Connected),
		zap.Int("mcp_servers_failed", len(mcpReport.FailedByServer)),
	)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Fatal("server exited", zap.Error(err))
	}
}

func buildMemoryAndIngestionServices(cfg config.Config, logger *zap.Logger) (memory.Service, ingestion.Service) {
	// 统一使用云端 embedding，不再走本地 hash embedder。
	embedder := memory.HTTPEmbedder{
		Endpoint:    cfg.MemoryEmbedEndpoint,
		APIKey:      cfg.MemoryEmbedAPIKey,
		Model:       cfg.MemoryEmbedModel,
		ExpectedDim: cfg.MemoryVectorDim,
		HTTPClient: &http.Client{
			Timeout: time.Duration(cfg.MemoryEmbedTimeoutSeconds) * time.Second,
		},
	}
	store := memory.NewQdrantStore(cfg.QdrantURL, cfg.QdrantCollection, cfg.QdrantAPIKey, cfg.MemoryVectorDim)

	memorySvc := memory.NewService(store, embedder, logger, cfg.MemoryTopK, 0, cfg.MemorySimilarityThreshold)
	if !cfg.IngestEnabled {
		// 未开启摄入时仍保留记忆检索能力，只关闭 /ingest 的实际写入流程。
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
