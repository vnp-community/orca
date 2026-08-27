package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

const touchDebounce = 60 * time.Second // avoid a write on every single request

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

	if session.LastSeenAt == nil || uc.clock.Now().Sub(*session.LastSeenAt) > touchDebounce {
		uc.touchBestEffort(session.TokenHash) // fire-and-forget, does not block the response
	}

	return ValidateSessionOutput{Valid: true, User: user}, nil
}

// touchBestEffort mirrors login.go's appendAuditBestEffort pattern applied
// to a write instead of an audit append — a failed or slow touch must never
// turn a valid session into a failed request, and per auth-service.md §8's
// p99<20ms budget, must not add a synchronous round trip to the hot path at
// all. Uses context.Background() (not the request ctx) so it isn't
// cancelled the instant the RPC handler returns.
func (uc *ValidateSession) touchBestEffort(tokenHash string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = uc.sessions.TouchLastSeen(ctx, tokenHash, uc.clock.Now())
	}()
}
