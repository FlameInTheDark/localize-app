package inference

import (
	"context"
	"encoding/json"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

type Request struct {
	Model       string
	System      string
	Prompt      string
	ImageBase64 string
	MimeType    string
	Schema      json.RawMessage
	Temperature *float64
}

// Client is deliberately provider-neutral. Domain services never depend on an
// HTTP endpoint, an executable, or a provider-specific request shape.
type Client interface {
	Health(context.Context) error
	ListModels(context.Context) ([]domain.ModelInfo, error)
	Generate(context.Context, Request) (string, error)
}
