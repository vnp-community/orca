package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// GetVapidPublicKey is the only VAPID material that ever crosses this
// service's API — the private key never appears in any request or
// response here (§9's headline property).
type GetVapidPublicKey struct {
	repo VapidKeyRepository
}

func NewGetVapidPublicKey(repo VapidKeyRepository) *GetVapidPublicKey {
	return &GetVapidPublicKey{repo: repo}
}

func (uc *GetVapidPublicKey) Execute(ctx context.Context) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err)
	}

	key, err := uc.repo.GetPublicKey(ctx, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNoActiveVapidKey) {
			return "", apperrors.New(apperrors.KindNotFound, "NOTIFICATION_NO_VAPID_KEY", "no active vapid key for tenant", err)
		}
		return "", apperrors.New(apperrors.KindInternal, "NOTIFICATION_VAPID_KEY_FETCH_FAILED", "failed to fetch vapid public key", err)
	}
	return key.PublicKey, nil
}
