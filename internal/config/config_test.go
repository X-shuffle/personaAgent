package config

import (
	"os"
	"testing"
)

func TestLoad_MemorySimilarityThreshold(t *testing.T) {
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
