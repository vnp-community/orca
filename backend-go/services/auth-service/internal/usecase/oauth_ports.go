package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// Sentinel errors OAuthRepository implementations wrap so usecases can tell
// "no such row" and "lost an atomic race" apart from infrastructure failure.
var (
	ErrOAuthClientNotFound       = errors.New("usecase: oauth client not found")
	ErrOAuthClientStatusNotFound = errors.New("usecase: oauth client tenant status not found")
	ErrOAuthCodeNotFound         = errors.New("usecase: oauth authorization code not found")
	ErrOAuthRefreshNotFound      = errors.New("usecase: oauth refresh token not found")
	ErrOAuthFamilyNotFound       = errors.New("usecase: oauth token family not found")
	// ErrOAuthCodeAlreadyUsed: the conditional "used_at IS NULL" update
	// matched no row, i.e. another request consumed the code first.
	ErrOAuthCodeAlreadyUsed = errors.New("usecase: oauth authorization code already used")
	// ErrOAuthRefreshAlreadyUsed is the same race for refresh rotation.
	ErrOAuthRefreshAlreadyUsed = errors.New("usecase: oauth refresh token already used")
)

// OAuthRepository persists the OAuth authorization server's state
// (BE-MCP-SOL-005 §C). Only hashes of codes and refresh tokens cross this
// interface. Lookups by hash run without a tenant (the /token endpoint is
// anonymous); the tenant is read from the row. Tenant-scoped methods take
// tenantID explicitly and bind it in SQL.
type OAuthRepository interface {
	CountOAuthClients(ctx context.Context) (int, error)
	CreateOAuthClient(ctx context.Context, c domain.OAuthClient) error
	GetOAuthClient(ctx context.Context, clientID string) (domain.OAuthClient, error)
	TouchOAuthClientUsed(ctx context.Context, clientID string, at time.Time) error

	// EnsureOAuthClientTenantStatus inserts s if (tenant, client) has no row
	// and returns the stored row either way; an existing row is never changed.
	EnsureOAuthClientTenantStatus(ctx context.Context, s domain.OAuthClientTenantStatus) (domain.OAuthClientTenantStatus, error)
	GetOAuthClientTenantStatus(ctx context.Context, tenantID, clientID string) (domain.OAuthClientTenantStatus, error)
	// SetOAuthClientTenantStatus updates an existing row; ErrOAuthClientStatusNotFound if absent.
	SetOAuthClientTenantStatus(ctx context.Context, s domain.OAuthClientTenantStatus) error
	ListOAuthClientViews(ctx context.Context, tenantID string) ([]domain.OAuthClientView, error)

	// CreateOAuthFamilyWithCode stores the family and its authorization code atomically.
	CreateOAuthFamilyWithCode(ctx context.Context, f domain.OAuthTokenFamily, c domain.OAuthAuthCode) error
	GetOAuthAuthCode(ctx context.Context, codeHash string) (domain.OAuthAuthCode, error)
	GetOAuthFamily(ctx context.Context, familyID string) (domain.OAuthTokenFamily, error)
	// ClaimOAuthAuthCode marks the code used (only if still unused) and
	// stores the family's first refresh token in the same transaction.
	// ErrOAuthCodeAlreadyUsed if the code was already consumed.
	ClaimOAuthAuthCode(ctx context.Context, codeHash string, at time.Time, first domain.OAuthRefreshToken) error

	GetOAuthRefreshToken(ctx context.Context, tokenHash string) (domain.OAuthRefreshToken, error)
	// RotateOAuthRefreshToken marks oldHash used (only if unused) and stores
	// next in the same transaction. ErrOAuthRefreshAlreadyUsed on a lost race.
	RotateOAuthRefreshToken(ctx context.Context, oldHash string, at time.Time, next domain.OAuthRefreshToken) error

	// RevokeOAuthFamily is idempotent; the first reason wins.
	RevokeOAuthFamily(ctx context.Context, familyID, reason string, at time.Time) error
	// RevokeOAuthGrant records the grant as revoked (idempotent) and revokes
	// every family of the grant; returns how many families were newly revoked.
	RevokeOAuthGrant(ctx context.Context, tenantID, grantID, reason string, at time.Time) (int, error)
	RevokeOAuthFamiliesForClient(ctx context.Context, tenantID, clientID, reason string, at time.Time) (int, error)
	IsOAuthGrantRevoked(ctx context.Context, tenantID, grantID string) (bool, error)
}
