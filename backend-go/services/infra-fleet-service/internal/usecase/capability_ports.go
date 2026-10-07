package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type CapabilityProfileStore interface {
	Get(ctx context.Context, tenantID, devServerID string) (domain.CapabilityProfile, bool, error)
	Upsert(ctx context.Context, p domain.CapabilityProfile) (previousFingerprint string, existed bool, err error)
}
