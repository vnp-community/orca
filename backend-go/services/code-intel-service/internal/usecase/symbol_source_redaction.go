package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

// SymbolSourceRedactor inspects source files or paths to redact sensitive information
// or determine whether source display is permitted.
type SymbolSourceRedactor interface {
	RedactSource(ctx context.Context, repo, path string, source *domain.SymbolSource) (*domain.SymbolSource, string, error)
}

// FailClosedSymbolSourceRedactor is the default fail-closed redactor when no redactor is configured.
// It withholds source code and sets source_omitted to "sensitive_path" (fail-closed per H7/H8).
type FailClosedSymbolSourceRedactor struct{}

// NewFailClosedSymbolSourceRedactor creates a new fail-closed redactor.
func NewFailClosedSymbolSourceRedactor() *FailClosedSymbolSourceRedactor {
	return &FailClosedSymbolSourceRedactor{}
}

// RedactSource implements SymbolSourceRedactor, always withholding the source.
func (r *FailClosedSymbolSourceRedactor) RedactSource(ctx context.Context, repo, path string, source *domain.SymbolSource) (*domain.SymbolSource, string, error) {
	return nil, "sensitive_path", nil
}
