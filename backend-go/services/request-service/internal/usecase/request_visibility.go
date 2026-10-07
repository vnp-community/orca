package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type DefaultRequestVisibility struct {
	// dependencies like team membership can go here
}

func NewDefaultRequestVisibility() *DefaultRequestVisibility {
	return &DefaultRequestVisibility{}
}

func (v *DefaultRequestVisibility) Filter(ctx context.Context, actor domain.DecisionActor, requests []domain.Request) ([]domain.Request, error) {
	var filtered []domain.Request
	for _, r := range requests {
		// Mock implementation:
		// Admin sees all, Reporter sees theirs.
		if actor.Role == "admin" || r.ReporterID == actor.UserID {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}
