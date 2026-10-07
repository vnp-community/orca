package postgres

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type ContextSourceRepository struct {
    // db connection
}

var _ usecase.ContextSourceRepository = (*ContextSourceRepository)(nil)

func (r *ContextSourceRepository) List(ctx context.Context) ([]domain.ContextSource, error) {
    return nil, nil
}

func (r *ContextSourceRepository) Get(ctx context.Context, key string) (domain.ContextSource, error) {
    return domain.ContextSource{}, nil
}

func (r *ContextSourceRepository) Upsert(ctx context.Context, s domain.ContextSource, expectedVersion int64) (domain.ContextSource, error) {
    return domain.ContextSource{}, nil
}

func (r *ContextSourceRepository) SetStatus(ctx context.Context, key, status string, expectedVersion int64) (domain.ContextSource, error) {
    return domain.ContextSource{}, nil
}
