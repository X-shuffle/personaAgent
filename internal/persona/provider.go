package persona

import (
	"context"

	"persona_agent/internal/model"
)

// Provider returns persona data for a session.
type Provider interface {
	GetPersona(ctx context.Context, sessionID string) (model.Persona, error)
}

// StaticProvider always returns the same persona.
type StaticProvider struct {
	Persona model.Persona
}

func (p StaticProvider) GetPersona(_ context.Context, _ string) (model.Persona, error) {
	return p.Persona, nil
}
