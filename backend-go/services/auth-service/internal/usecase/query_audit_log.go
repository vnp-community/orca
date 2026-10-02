package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type QueryAuditLogInput struct {
	TenantID string
	Since    time.Time
	To       time.Time // zero value = no upper bound
	// ActorID/Action/Outcome are optional filters (TASK-BE-016) — zero
	// value on each means "no filter," matching AuditRepository.Query's own
	// empty-means-no-filter convention.
	ActorID   string
	Action    string
	Outcome   domain.Outcome
	PageToken string
	PageSize  int32

	// Additive (BE-MCP-SOL-013); zero values keep the legacy behavior.
	ActorType      domain.ActorType
	TargetID       string
	MetadataEquals map[string]string
	NewestFirst    bool
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

	if in.ActorType != "" && !in.ActorType.Valid() {
		return QueryAuditLogOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_AUDIT_INVALID_FILTER", "unknown actor_type", nil)
	}
	for k := range in.MetadataEquals {
		if !AllowedAuditMetadataKeys[k] {
			return QueryAuditLogOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_AUDIT_INVALID_FILTER", "metadata filter key is not allowed", nil)
		}
	}
	if in.NewestFirst && in.PageToken != "" {
		if _, _, err := domain.DecodeAuditKeyset(in.PageToken); err != nil {
			return QueryAuditLogOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_AUDIT_INVALID_FILTER", "invalid page token", err)
		}
	}
	filter := AuditQueryFilter{
		ActorType: in.ActorType, TargetID: in.TargetID, MetadataEquals: in.MetadataEquals, NewestFirst: in.NewestFirst,
		TenantID: in.TenantID,
		Since:    in.Since,
		To:       in.To,
		Action:   in.Action,
		ActorID:  in.ActorID,
		Outcome:  in.Outcome,
	}
	entries, next, err := uc.audit.Query(ctx, filter, in.PageToken, pageSize)
	if err != nil {
		return QueryAuditLogOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_AUDIT_QUERY_FAILED", "failed to query audit log", err)
	}
	return QueryAuditLogOutput{Entries: entries, NextPageToken: next}, nil
}
