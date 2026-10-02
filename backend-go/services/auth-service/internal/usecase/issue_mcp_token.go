package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/common/mcpscope"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type IssueMcpTokenInput struct {
	// TenantID and UserID come from gRPC metadata, never from the request body.
	TenantID, UserID string
	Name             string
	Scopes           []string
	ExpiresInDays    int
}

type IssueMcpTokenOutput struct {
	Token  domain.McpToken
	Secret string // "omp_<jwt>": the only time it exists outside the caller
}

// IssueMcpToken mints a personal access token: an RS256 JWT with
// aud = the MCP resource URL (fixed server-side, never an input), carrying
// scopes already capped by the user's current role. Only SHA-256(secret) is
// stored. Tenant policy (maxTokenDays, per-user quota) is enforced upstream by
// the gateway; the 90 day cap and role ceiling here are the hard second layer.
type IssueMcpToken struct {
	users       UserRepository
	tokens      McpTokenRepository
	audit       AuditRepository
	signer      TokenSigner
	clock       Clock
	resourceURL string
}

func NewIssueMcpToken(users UserRepository, tokens McpTokenRepository, audit AuditRepository, signer TokenSigner, clock Clock, resourceURL string) *IssueMcpToken {
	return &IssueMcpToken{users: users, tokens: tokens, audit: audit, signer: signer, clock: clock, resourceURL: resourceURL}
}

func (uc *IssueMcpToken) Execute(ctx context.Context, in IssueMcpTokenInput) (IssueMcpTokenOutput, error) {
	if uc.resourceURL == "" {
		return IssueMcpTokenOutput{}, errMcp(apperrors.KindFailedPrecondition, CodeMcpNotConfigured, "MCP tokens are not enabled")
	}
	if in.TenantID == "" || in.UserID == "" {
		return IssueMcpTokenOutput{}, errMcp(apperrors.KindUnauthenticated, CodeMcpTokenNotFound, "caller identity is required")
	}
	name := strings.TrimSpace(in.Name)
	if n := len([]rune(name)); n < 1 || n > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return IssueMcpTokenOutput{}, errMcp(apperrors.KindInvalidArgument, CodeMcpTokenInvalidName, "name must be 1-80 characters without control characters")
	}
	scopes, err := mcpscope.Normalize(in.Scopes)
	if err != nil || len(scopes) == 0 {
		return IssueMcpTokenOutput{}, errMcp(apperrors.KindInvalidArgument, CodeMcpScopeInvalid, "scopes must be a non-empty list of known scopes")
	}
	if in.ExpiresInDays < 1 || in.ExpiresInDays > domain.MaxMcpTokenDays {
		return IssueMcpTokenOutput{}, errMcp(apperrors.KindInvalidArgument, CodeMcpTokenTooLong, "expires_in_days must be between 1 and 90")
	}

	user, err := uc.users.GetUserByID(ctx, in.UserID)
	if errors.Is(err, ErrUserNotFound) || (err == nil && (!user.IsActive || user.TenantID != in.TenantID)) {
		return IssueMcpTokenOutput{}, errMcp(apperrors.KindPermissionDenied, CodeMcpScopeNotAllowed, "user is not allowed to create tokens")
	}
	if err != nil {
		return IssueMcpTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_TOKEN_LOOKUP_FAILED", "failed to look up user", err)
	}
	if !mcpscope.Subset(scopes, mcpscope.CeilingForRole(string(user.Role))) {
		return IssueMcpTokenOutput{}, errMcp(apperrors.KindPermissionDenied, CodeMcpScopeNotAllowed, "a requested scope exceeds what your role allows")
	}

	jti, err := generateRandomToken(mcpTokenJTIBytes)
	if err != nil {
		return IssueMcpTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_TOKEN_JTI_FAILED", "failed to generate token id", err)
	}
	now := uc.clock.Now()
	expiresAt := now.Add(time.Duration(in.ExpiresInDays) * 24 * time.Hour)
	// role is deliberately absent: the gateway reads it live per request.
	jwtStr, err := uc.signer.Sign(ctx, jwtauth.Claims{
		Claims: jwt.Claims{
			Issuer: jwtauth.Issuer, Subject: user.ID, Audience: jwt.Audience{uc.resourceURL},
			IssuedAt: jwt.NewNumericDate(now), Expiry: jwt.NewNumericDate(expiresAt), ID: jti,
		},
		TenantID: user.TenantID, Scope: mcpscope.Format(scopes), TokenUse: mcpTokenUsePAT,
	})
	if err != nil {
		return IssueMcpTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_TOKEN_SIGN_FAILED", "failed to sign token", err)
	}
	secret := mcpTokenSecretPrefix + jwtStr
	sum := sha256.Sum256([]byte(secret))
	rec := domain.McpToken{
		JTI: jti, TenantID: user.TenantID, UserID: user.ID, Name: name, Scope: mcpscope.Format(scopes),
		TokenSHA256: hex.EncodeToString(sum[:]), CreatedAt: now, ExpiresAt: expiresAt,
	}
	if err := uc.tokens.CreateMcpToken(ctx, rec); err != nil {
		return IssueMcpTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_TOKEN_RECORD_FAILED", "failed to record token", err)
	}
	if e, err := domain.NewAuditEntry(uuid.NewString(), user.TenantID, user.ID, "mcp_token.created", "", "mcp_token", jti,
		map[string]any{"scopes": rec.Scope, "expires_at": expiresAt.Format(time.RFC3339), "name": name}, domain.OutcomeAllowed, "", now); err == nil {
		_ = uc.audit.Append(ctx, e)
	}
	return IssueMcpTokenOutput{Token: rec, Secret: secret}, nil
}
