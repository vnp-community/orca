package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CapabilityProfileStore persists one capability profile per dev server.
type CapabilityProfileStore interface {
	Get(ctx context.Context, tenantID, devServerID string) (domain.CapabilityProfile, bool, error)
	// Upsert reads the previous fingerprint and writes the new profile in one
	// transaction so the caller knows whether the profile changed; it returns
	// domain.ErrNotFound when the dev server is missing for p.TenantID.
	Upsert(ctx context.Context, p domain.CapabilityProfile) (previousFingerprint string, existed bool, err error)
}

// OutboxEnqueuer is the narrow outbox port the capability events use.
type OutboxEnqueuer interface {
	EnqueueOutboxEvent(ctx context.Context, id, tenantID, subject string, now time.Time, version int, payload []byte) error
}

// DevServerTenantLookup resolves a dev server's tenant without a tenant in
// context; used by the post-handshake probe, where a session only knows the
// dev server id.
type DevServerTenantLookup interface {
	TenantIDForDevServer(ctx context.Context, devServerID string) (tenantID string, found bool, err error)
}
