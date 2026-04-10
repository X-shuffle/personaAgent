package llm

import (
	"context"

	"persona_agent/internal/model"
)

// Client abstracts model generation.
type Client interface {
	Generate(ctx context.Context, req model.LLMRequest) (model.LLMResponse, error)
}
