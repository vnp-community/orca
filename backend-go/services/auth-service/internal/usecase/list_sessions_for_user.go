package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ListSessionsForUser is an admin-console, read-only operation — returns
// every session (revoked or not, expired or not) for a user, for the
// session-inspection view.
type ListSessionsForUser struct {
	users    UserRepository
	sessions SessionRepository
	opa      OPAClient
}

func NewListSessionsForUser(users UserRepository, sessions SessionRepository, opa OPAClient) *ListSessionsForUser {
	return &ListSessionsForUser{users: users, sessions: sessions, opa: opa}
}

func (uc *ListSessionsForUser) Execute(ctx context.Context, userID string) ([]domain.Session, error) {
	if _, err := requireAdminActor(ctx, uc.users, uc.opa); err != nil {
		return nil, err
	}
	if userID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_USER_ID", "user_id is required", nil)
	}

	sessions, err := uc.sessions.ListForUser(ctx, userID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "AUTH_LIST_SESSIONS_FAILED", "failed to list sessions for user", err)
	}
	return sessions, nil
}
