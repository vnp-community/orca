package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

// SessionRecordedSubject is the event subject a recorded session's outbox
// row publishes under — task-service/project-service consumers, if any
// exist in the future, would subscribe here (see usecase.Repository's doc
// comment: this used to be internal/adapter/eventbus's Subject constant,
// moved here now that this usecase is what builds the outbox row instead
// of calling a publisher directly).
const SessionRecordedSubject = "orca.usage.session.recorded"

// sessionRecordedPayload is SessionRecordedSubject's JSON payload shape.
type sessionRecordedPayload struct {
	SessionID string  `json:"session_id"`
	Provider  string  `json:"provider"`
	CostUSD   float64 `json:"cost_usd"`
}

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
	repo Repository
}

func NewRecordUsageSession(repo Repository) *RecordUsageSession {
	return &RecordUsageSession{repo: repo}
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

	payload, err := json.Marshal(sessionRecordedPayload{SessionID: session.ID, Provider: string(session.Provider), CostUSD: session.CostUSD})
	if err != nil {
		return domain.UsageSession{}, apperrors.New(apperrors.KindInternal, "USAGE_MARSHAL_EVENT_FAILED", "failed to marshal session-recorded event payload", err)
	}
	event := domain.OutboxEvent{
		ID:          uuid.NewString(),
		Subject:     SessionRecordedSubject,
		OccurredAt:  time.Now().UTC(),
		PayloadJSON: payload,
	}

	// Session write and outbox enqueue happen in ONE transaction (Epic G,
	// docs/execution-plan.md) — unlike the previous best-effort
	// direct-publish call, there is no longer a "session saved but event
	// publish failed" partial-success state to report: either both commit,
	// or neither does. common/outbox.Relay (started in cmd/server/main.go)
	// publishes the enqueued row to NATS asynchronously.
	if err := uc.repo.SaveSession(ctx, session, event); err != nil {
		return domain.UsageSession{}, apperrors.New(apperrors.KindInternal, "USAGE_SAVE_FAILED", "failed to persist usage session", err)
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
