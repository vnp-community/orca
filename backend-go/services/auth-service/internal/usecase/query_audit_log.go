package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type QueryAuditLogInput struct {
	TenantID  string
	Since     time.Time
	PageToken string
	PageSize  int32
	To        time.Time // zero value = no upper bound
	Action    string    // "" = no filter
	ActorID   string    // "" = no filter
}

type QueryAuditLogOutput struct {
	Entries       []domain.AuditEntry
	NextPageToken string
}

// QueryAuditLog is an admin-console, read-only operation — the log itself
// is never mutated through this or any other usecase method
// (domain.AuditEntry's doc comment).
type QueryAuditLog struct {
	users UserRepository
	audit AuditRepository
	opa   OPAClient
}

func NewQueryAuditLog(users UserRepository, audit AuditRepository, opa OPAClient) *QueryAuditLog {
	return &QueryAuditLog{users: users, audit: audit, opa: opa}
}

func (uc *QueryAuditLog) Execute(ctx context.Context, in QueryAuditLogInput) (QueryAuditLogOutput, error) {
	if _, err := requireAdminActor(ctx, uc.users, uc.opa); err != nil {
		return QueryAuditLogOutput{}, err
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	filter := AuditQueryFilter{
		TenantID: in.TenantID,
		Since:    in.Since,
		To:       in.To,
		Action:   in.Action,
		ActorID:  in.ActorID,
	}
	entries, next, err := uc.audit.Query(ctx, filter, in.PageToken, pageSize)
	if err != nil {
		return QueryAuditLogOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_AUDIT_QUERY_FAILED", "failed to query audit log", err)
	}
	return QueryAuditLogOutput{Entries: entries, NextPageToken: next}, nil
}
