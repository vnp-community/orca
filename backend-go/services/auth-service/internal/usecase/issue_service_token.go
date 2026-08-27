package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/jwtauth"
)

// serviceTokenJTIBytes is the entropy generateRandomToken reads for a
// service token's "jti" claim — same size Login uses for session tokens
// (see internal/usecase/token.go).
const serviceTokenJTIBytes = 32

type IssueServiceTokenInput struct {
	UserID   string
	Audience string
}

type IssueServiceTokenOutput struct {
	JWT       string
	ExpiresAt time.Time
}

// IssueServiceToken mints a real RS256 JWT (signed via Vault Transit,
// TokenSigner) for an existing user. Per this task's design, "sub" is the
// requested user_id (after verifying it exists), "tenant_id" comes from
// that user's own record, and "aud" is the request's audience verbatim.
//
// KNOWN GAP (see this service's README "Known gaps"): the generated
// IssueServiceTokenRequest carries no caller-identity field, so there is no
// check that the *requester* of a token is itself authorized to mint one
// for the given user_id — this usecase only verifies the target user
// exists, not who is asking. Fixing that needs a proto change outside this
// task's scope.
type IssueServiceToken struct {
	users  UserRepository
	signer TokenSigner
	clock  Clock
	ttl    time.Duration
}

func NewIssueServiceToken(users UserRepository, signer TokenSigner, clock Clock, ttl time.Duration) *IssueServiceToken {
	return &IssueServiceToken{users: users, signer: signer, clock: clock, ttl: ttl}
}

func (uc *IssueServiceToken) Execute(ctx context.Context, in IssueServiceTokenInput) (IssueServiceTokenOutput, error) {
	if in.UserID == "" {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_USER_ID", "user_id is required", nil)
	}
	if in.Audience == "" {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_AUDIENCE", "audience is required", nil)
	}

	user, err := uc.users.GetUserByID(ctx, in.UserID)
	if errors.Is(err, ErrUserNotFound) {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindNotFound, "AUTH_USER_NOT_FOUND", "user not found", err)
	}
	if err != nil {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_LOOKUP_FAILED", "failed to look up user", err)
	}

	jti, err := generateRandomToken(serviceTokenJTIBytes)
	if err != nil {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_JTI_FAILED", "failed to generate token id", err)
	}

	now := uc.clock.Now()
	expiresAt := now.Add(uc.ttl)
	claims := jwtauth.Claims{
		Claims: jwt.Claims{
			Issuer:   jwtauth.Issuer,
			Subject:  user.ID,
			Audience: jwt.Audience{in.Audience},
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(expiresAt),
			ID:       jti,
		},
		TenantID: user.TenantID,
	}

	token, err := uc.signer.Sign(ctx, claims)
	if err != nil {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_SIGN_FAILED", "failed to sign token", err)
	}

	return IssueServiceTokenOutput{JWT: token, ExpiresAt: expiresAt}, nil
}
