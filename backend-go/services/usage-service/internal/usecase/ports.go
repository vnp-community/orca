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
	// SaveSession persists a session and atomically updates its day's
	// rollup in the same transaction (on-write aggregation, see
	// usage-service.md §6). Idempotent on RequestID: calling twice with the
	// same RequestID must not double-count.
	SaveSession(ctx context.Context, session domain.UsageSession) error
	GetDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) (domain.DailyUsageRollup, error)
	ListSessions(ctx context.Context, tenantID, userID string, pageToken string, pageSize int32) ([]domain.UsageSession, string, error)
	// RecomputeDailyRollup rebuilds a day's rollup from its sessions —
	// the periodic reconciliation safety net mentioned in usage-service.md,
	// not called on the hot write path.
	RecomputeDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) error
}

// EventPublisher is the outbound event port — usage-service publishes
// usage.session.recorded events for anything that wants to react (e.g. a
// future billing/alerting consumer); not required by any other service in
// this system today, included to keep the pattern consistent across
// services per architecture/08-inter-service-communication.md.
type EventPublisher interface {
	PublishSessionRecorded(ctx context.Context, tenantID string, session domain.UsageSession) error
}
