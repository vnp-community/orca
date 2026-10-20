package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// EngineSettingsReviewReader reads the project's review choice from its engine settings. A project without a row has made no choice.
type EngineSettingsReviewReader struct {
	Repo EngineSettingsRepository
}

var _ ReviewSettingsReader = EngineSettingsReviewReader{}

func (r EngineSettingsReviewReader) OutcomeReviewMode(ctx context.Context, projectID string) (domain.OutcomeReviewMode, error) {
	s, ok, err := r.Repo.Get(ctx, projectID)
	if err != nil || !ok {
		return "", err
	}
	return s.OutcomeReview, nil
}
