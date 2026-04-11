package memory

import (
	"context"

	"persona_agent/internal/model"
)

// NoopService disables memory features while keeping interfaces stable.
type NoopService struct{}

func (NoopService) Retrieve(_ context.Context, _ string, _ string) ([]model.Memory, error) {
	return nil, nil
}

func (NoopService) StoreTurn(_ context.Context, _ string, _ string, _ string, _ model.EmotionState) error {
	return nil
}
