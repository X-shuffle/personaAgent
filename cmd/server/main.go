package main

import (
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"persona_agent/internal/agent"
	"persona_agent/internal/api"
	"persona_agent/internal/config"
	"persona_agent/internal/llm"
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

	orch := agent.Orchestrator{
		PersonaProvider: persona.StaticProvider{Persona: cfg.Persona},
		PromptBuilder:   prompt.DefaultBuilder{},
		LLMClient:       llmClient,
	}

	mux := http.NewServeMux()
	mux.Handle("/chat", api.ChatHandler{Orchestrator: orch})

	addr := ":" + cfg.Port
	logger.Info("persona_agent listening", zap.String("addr", addr), zap.String("llm_mode", cfg.LLMMode))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Fatal("server exited", zap.Error(err))
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
