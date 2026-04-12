package config

import (
	"os"
	"testing"
)

func setRequiredEmbedEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MEMORY_EMBED_ENDPOINT", "https://ai.gitee.com/v1/embeddings")
}

func TestLoad_MemorySimilarityThreshold(t *testing.T) {
	setRequiredEmbedEnv(t)
	t.Setenv("MEMORY_SIMILARITY_THRESHOLD", "0.77")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.MemorySimilarityThreshold != 0.77 {
		t.Fatalf("expected threshold=0.77, got %v", cfg.MemorySimilarityThreshold)
	}
}

func TestLoad_MemorySimilarityThresholdClamp(t *testing.T) {
	setRequiredEmbedEnv(t)
	original, hadOriginal := os.LookupEnv("MEMORY_SIMILARITY_THRESHOLD")
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.Setenv("MEMORY_SIMILARITY_THRESHOLD", original)
			return
		}
		_ = os.Unsetenv("MEMORY_SIMILARITY_THRESHOLD")
	})

	_ = os.Setenv("MEMORY_SIMILARITY_THRESHOLD", "-1")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.MemorySimilarityThreshold != 0 {
		t.Fatalf("expected threshold=0 after clamp, got %v", cfg.MemorySimilarityThreshold)
	}

	_ = os.Setenv("MEMORY_SIMILARITY_THRESHOLD", "2")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.MemorySimilarityThreshold != 1 {
		t.Fatalf("expected threshold=1 after clamp, got %v", cfg.MemorySimilarityThreshold)
	}
}

func TestLoad_IngestAllowedExtensionsLegacyFallback(t *testing.T) {
	setRequiredEmbedEnv(t)
	t.Setenv("INGEST_ALLOWED_EXTENSIONS", "")
	t.Setenv("INGEST_ALLOWED_EXT", "txt,jsonl")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(cfg.IngestAllowedExtensions) != 2 || cfg.IngestAllowedExtensions[0] != "txt" || cfg.IngestAllowedExtensions[1] != "jsonl" {
		t.Fatalf("expected legacy fallback [txt jsonl], got %v", cfg.IngestAllowedExtensions)
	}
}

func TestLoad_IngestAllowedExtensionsPrimaryPreferred(t *testing.T) {
	setRequiredEmbedEnv(t)
	t.Setenv("INGEST_ALLOWED_EXTENSIONS", "txt,md")
	t.Setenv("INGEST_ALLOWED_EXT", "txt,json")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(cfg.IngestAllowedExtensions) != 2 || cfg.IngestAllowedExtensions[0] != "txt" || cfg.IngestAllowedExtensions[1] != "md" {
		t.Fatalf("expected primary value [txt md], got %v", cfg.IngestAllowedExtensions)
	}
}

func TestLoad_MemoryEmbedEndpointRequired(t *testing.T) {
	t.Setenv("MEMORY_EMBED_ENDPOINT", "")
	_, err := Load()
	if err == nil {
		t.Fatalf("expected endpoint required error")
	}
}

func TestLoad_MemoryEmbedTimeoutDefault(t *testing.T) {
	setRequiredEmbedEnv(t)
	t.Setenv("MEMORY_EMBED_TIMEOUT_SECONDS", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.MemoryEmbedTimeoutSeconds != 15 {
		t.Fatalf("expected timeout default 15, got %d", cfg.MemoryEmbedTimeoutSeconds)
	}
}

func TestLoad_LLMTimeoutDefault(t *testing.T) {
	setRequiredEmbedEnv(t)
	t.Setenv("LLM_TIMEOUT_SECONDS", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.LLMTimeoutSeconds != 20 {
		t.Fatalf("expected llm timeout default 20, got %d", cfg.LLMTimeoutSeconds)
	}
}
