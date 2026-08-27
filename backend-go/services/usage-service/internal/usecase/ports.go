// Package usecase holds usage-service's application services and the ports
// they need — defined here, implemented in internal/adapter/*, per the
// Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

// Repository is the persistence port for usage sessions and daily rollups.
// Implemented by internal/adapter/postgres against usage-service's own
// database — see specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule.
type Repository interface {
	// SaveSession persists a session, atomically updates its day's rollup,
	// and durably enqueues event in the SAME transaction (Epic G's
	// transactional-outbox pattern, docs/execution-plan.md — this replaced
	// the previous best-effort direct-publish-after-commit call, which
	// could silently drop the event on a crash between commit and publish).
	// Idempotent on RequestID: calling twice with the same RequestID must
	// not double-count and must not enqueue a duplicate event either.
	SaveSession(ctx context.Context, session domain.UsageSession, event domain.OutboxEvent) error
	GetDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) (domain.DailyUsageRollup, error)
	ListSessions(ctx context.Context, tenantID, userID string, pageToken string, pageSize int32) ([]domain.UsageSession, string, error)
	// RecomputeDailyRollup rebuilds a day's rollup from its sessions —
	// the periodic reconciliation safety net mentioned in usage-service.md,
	// not called on the hot write path.
	RecomputeDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) error
}
