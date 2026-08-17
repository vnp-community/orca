package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

// RecordUsageSessionInput mirrors the gRPC request 1:1 by design — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the TS->Go mapping stays traceable.
type RecordUsageSessionInput struct {
	ID               string
	Provider         domain.Provider
	WorktreeID       string
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	CostUSD          float64
	StartedAt        int64 // unix seconds, converted at the usecase boundary
	EndedAt          int64
	RequestID        string
}

// RecordUsageSession is usage-service's core write path. TenantID/UserID
// are NOT part of the input struct — they're pulled from context (see
// common/tenant), never trusted from the request body, per
// architecture/05-data-architecture.md's tenant-isolation rule.
type RecordUsageSession struct {
	repo      Repository
	publisher EventPublisher
}

func NewRecordUsageSession(repo Repository, publisher EventPublisher) *RecordUsageSession {
	return &RecordUsageSession{repo: repo, publisher: publisher}
}

func (uc *RecordUsageSession) Execute(ctx context.Context, in RecordUsageSessionInput) (domain.UsageSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.UsageSession{}, apperrors.New(apperrors.KindUnauthenticated, "USAGE_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.UsageSession{}, apperrors.New(apperrors.KindUnauthenticated, "USAGE_NO_USER", "no user in request context", nil)
	}

	session, err := domain.NewUsageSession(
		in.ID, tenantID, userID, in.Provider, in.WorktreeID,
		in.InputTokens, in.OutputTokens, in.CacheReadTokens, in.CacheWriteTokens,
		in.CostUSD, unixToTime(in.StartedAt), unixToTime(in.EndedAt), in.RequestID,
	)
	if err != nil {
		return domain.UsageSession{}, apperrors.New(apperrors.KindInvalidArgument, "USAGE_INVALID_SESSION", err.Error(), err)
	}

	if err := uc.repo.SaveSession(ctx, session); err != nil {
		return domain.UsageSession{}, apperrors.New(apperrors.KindInternal, "USAGE_SAVE_FAILED", "failed to persist usage session", err)
	}

	// Best-effort event publish — usage-service.md notes this can tolerate
	// eventual consistency; a publish failure doesn't fail the write that
	// already committed. In production this goes through the transactional
	// outbox (architecture/05), not a direct call — this scaffold calls the
	// publisher directly to keep the reference implementation simple; see
	// this service's README for the outbox follow-up.
	if uc.publisher != nil {
		if err := uc.publisher.PublishSessionRecorded(ctx, tenantID, session); err != nil {
			return session, fmt.Errorf("session saved but event publish failed: %w", err)
		}
	}

	return session, nil
}

// unixToTime converts a unix-seconds timestamp to time.Time, treating 0 as
// the zero value rather than the unix epoch — the gRPC layer sends 0 for an
// unset EndedAt (a session still in progress), which domain.NewUsageSession
// distinguishes from a real end time via time.Time.IsZero().
func unixToTime(sec int64) time.Time {
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}
