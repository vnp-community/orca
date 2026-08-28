package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type ListSessionsInput struct {
	PageToken string
	PageSize  int32
}

type ListSessionsOutput struct {
	Sessions      []domain.SessionWithUser
	NextPageToken string
}

// ListSessions is the cross-user, tenant-scoped admin session-dashboard
// usecase — distinct from ListSessionsForUser (single-user scope).
type ListSessions struct {
	users    UserRepository
	sessions SessionRepository
	opa      OPAClient
}

func NewListSessions(users UserRepository, sessions SessionRepository, opa OPAClient) *ListSessions {
	return &ListSessions{users: users, sessions: sessions, opa: opa}
}

func (uc *ListSessions) Execute(ctx context.Context, in ListSessionsInput) (ListSessionsOutput, error) {
	actor, err := requireAdminActor(ctx, uc.users, uc.opa)
	if err != nil {
		return ListSessionsOutput{}, err
	}
	// actor.TenantID, never a caller-supplied tenant_id — multi-tenancy
	// isolation layer 2 (07-security-architecture.md).
	rows, next, err := uc.sessions.ListForTenant(ctx, actor.TenantID, in.PageToken, in.PageSize)
	if err != nil {
		return ListSessionsOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_LIST_SESSIONS_FAILED", "failed to list sessions", err)
	}
	return ListSessionsOutput{Sessions: rows, NextPageToken: next}, nil
}
