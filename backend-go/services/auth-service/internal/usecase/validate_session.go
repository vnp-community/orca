package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ValidateSessionOutput mirrors ValidateSessionResponse: an invalid/expired
// session is a normal (valid=false, no error) result, not an RPC error —
// this is the one RPC on literally every browser request's path
// (auth-service.md §8) and must stay cheap to call.
type ValidateSessionOutput struct {
	Valid bool
	User  domain.User
}

// ValidateSession looks up a session by its token hash and checks it isn't
// expired or revoked.
type ValidateSession struct {
	sessions SessionRepository
	users    UserRepository
	clock    Clock
}

func NewValidateSession(sessions SessionRepository, users UserRepository, clock Clock) *ValidateSession {
	return &ValidateSession{sessions: sessions, users: users, clock: clock}
}

func (uc *ValidateSession) Execute(ctx context.Context, sessionToken string) (ValidateSessionOutput, error) {
	if sessionToken == "" {
		return ValidateSessionOutput{}, nil
	}

	tokenHash := domain.HashSessionToken(sessionToken)
	session, err := uc.sessions.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return ValidateSessionOutput{}, nil // not found is "invalid", not an error
	}
	if !session.IsValid(uc.clock.Now()) {
		return ValidateSessionOutput{}, nil
	}

	user, err := uc.users.GetUserByID(ctx, session.UserID)
	if err != nil {
		return ValidateSessionOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_USER_LOOKUP_FAILED", "failed to load session's user", err)
	}
	if !user.IsActive {
		return ValidateSessionOutput{}, nil
	}

	return ValidateSessionOutput{Valid: true, User: user}, nil
}
