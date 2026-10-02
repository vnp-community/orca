// Package mcptokens is the gateway-side orchestration of MCP personal access
// tokens (BE-MCP-SOL-006 section F), shared by the REST /v1/auth/mcp-tokens
// routes and the mcp.token.* WS channels so both enforce the same policy:
// tenant maxTokenDays and kill switch come from mcp-service, the hard 90 day
// cap and role ceiling from auth-service. It never sees or stores a secret
// beyond handing the one-time value back to the caller.
package mcptokens

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/mcpscope"
	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

const (
	// HardMaxDays mirrors auth-service's absolute PAT lifetime ceiling.
	HardMaxDays = 90
	// DefaultMaxActivePerUser is MCP_PAT_MAX_PER_USER's default.
	DefaultMaxActivePerUser = 20
)

// AuthAPI / PolicyAPI are the downstream slices used.
type AuthAPI interface {
	IssueMcpToken(ctx context.Context, in *authv1.IssueMcpTokenRequest, opts ...grpc.CallOption) (*authv1.IssueMcpTokenResponse, error)
	ListMcpTokens(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*authv1.ListMcpTokensResponse, error)
	RevokeMcpToken(ctx context.Context, in *authv1.RevokeMcpTokenRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type PolicyAPI interface {
	GetServerInfo(ctx context.Context, in *mcpv1.GetServerInfoRequest, opts ...grpc.CallOption) (*mcpv1.GetServerInfoResponse, error)
}

// Error is a CONTRACT error: Code is an MCP_* identifier.
type Error struct{ Code, Message string }

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func newErr(code, format string, a ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// Token is CONTRACT McpToken (never carries the secret).
type Token struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"createdAt"`
	ExpiresAt  string   `json:"expiresAt"`
	LastUsedAt string   `json:"lastUsedAt,omitempty"`
	Status     string   `json:"status"` // active | revoked | expired
}

// Created is the one response that carries the secret.
type Created struct {
	Token  Token  `json:"token"`
	Secret string `json:"secret"`
}

type Service struct {
	Auth   AuthAPI
	Policy PolicyAPI
	// MaxActivePerUser is MCP_PAT_MAX_PER_USER (0 = default 20).
	MaxActivePerUser int
	// OnRevoked lets the verifier cache drop a token right after it is revoked.
	OnRevoked func(jti string)
	Now       func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

var codedStatus = regexp.MustCompile(`^([A-Z][A-Z0-9_]+): (.*)$`)

// translate converts a downstream gRPC error into a CONTRACT error without
// echoing anything but the (already user-safe) coded message.
func translate(err error) error {
	if err == nil {
		return nil
	}
	var ce *Error
	if errors.As(err, &ce) {
		return ce
	}
	st, ok := status.FromError(err)
	if !ok {
		if errors.Is(err, context.DeadlineExceeded) {
			return newErr("MCP_TIMEOUT", "service did not respond in time")
		}
		return newErr("MCP_INTERNAL", "internal error")
	}
	switch st.Code() {
	case codes.Unavailable:
		return newErr("MCP_UNAVAILABLE", "service temporarily unavailable")
	case codes.DeadlineExceeded:
		return newErr("MCP_TIMEOUT", "service did not respond in time")
	}
	if m := codedStatus.FindStringSubmatch(st.Message()); m != nil {
		switch m[1] {
		case "AUTH_MCP_TOKEN_TOO_LONG":
			return newErr("MCP_TOKEN_TOO_LONG", "%s", m[2])
		case "AUTH_MCP_SCOPE_NOT_ALLOWED":
			return newErr("MCP_SCOPE_NOT_ALLOWED", "%s", m[2])
		case "AUTH_MCP_SCOPE_INVALID":
			return newErr("MCP_SCOPE_INVALID", "%s", m[2])
		case "AUTH_MCP_TOKEN_NOT_FOUND":
			return newErr("MCP_NOT_FOUND", "token not found")
		case "AUTH_MCP_TOKEN_INVALID_NAME":
			return newErr("MCP_INVALID_ARGUMENT", "%s", m[2])
		case "AUTH_MCP_NOT_CONFIGURED":
			return newErr("MCP_DISABLED", "MCP tokens are not enabled")
		}
		if len(m[1]) > 4 && m[1][:4] == "MCP_" {
			return newErr(m[1], "%s", m[2])
		}
	}
	switch st.Code() {
	case codes.NotFound, codes.PermissionDenied:
		return newErr("MCP_NOT_FOUND", "not found")
	case codes.InvalidArgument:
		return newErr("MCP_INVALID_ARGUMENT", "invalid argument")
	}
	return newErr("MCP_INTERNAL", "internal error")
}

// CreateInput is the caller-controlled part; identity comes from the session.
type CreateInput struct {
	Name          string
	Scopes        []string
	ExpiresInDays int
}

func (s *Service) Create(ctx context.Context, id usecase.Identity, in CreateInput) (Created, error) {
	ctx = gatewaygrpc.AttachIdentity(ctx, id)
	info, err := s.Policy.GetServerInfo(ctx, &mcpv1.GetServerInfoRequest{})
	if err != nil {
		return Created{}, translate(err)
	}
	if !info.GetEnabled() {
		return Created{}, newErr("MCP_DISABLED", "MCP is disabled for this organization")
	}
	if info.GetKillSwitch().GetActive() {
		return Created{}, newErr("MCP_KILL_SWITCH_ACTIVE", "MCP access is currently stopped by an administrator")
	}
	scopes, err := mcpscope.Normalize(in.Scopes)
	if err != nil || len(scopes) == 0 {
		return Created{}, newErr("MCP_SCOPE_INVALID", "choose at least one known scope")
	}
	max := HardMaxDays
	if d := int(info.GetMaxTokenDays()); d > 0 && d < max {
		max = d
	}
	if in.ExpiresInDays < 1 || in.ExpiresInDays > max {
		return Created{}, newErr("MCP_TOKEN_TOO_LONG", "maximum is %d days for this organization", max)
	}
	existing, err := s.Auth.ListMcpTokens(ctx, &emptypb.Empty{})
	if err != nil {
		return Created{}, translate(err)
	}
	limit := s.MaxActivePerUser
	if limit <= 0 {
		limit = DefaultMaxActivePerUser
	}
	active := 0
	for _, t := range existing.GetTokens() {
		if toToken(t, s.now()).Status == "active" {
			active++
		}
	}
	if active >= limit {
		return Created{}, newErr("MCP_TOKEN_LIMIT", "you already have %d active tokens; revoke one first", active)
	}
	resp, err := s.Auth.IssueMcpToken(ctx, &authv1.IssueMcpTokenRequest{Name: in.Name, Scopes: scopes, ExpiresInDays: int32(in.ExpiresInDays)})
	if err != nil {
		return Created{}, translate(err)
	}
	return Created{Token: toToken(resp.GetToken(), s.now()), Secret: resp.GetSecret()}, nil
}

func (s *Service) List(ctx context.Context, id usecase.Identity) ([]Token, error) {
	resp, err := s.Auth.ListMcpTokens(gatewaygrpc.AttachIdentity(ctx, id), &emptypb.Empty{})
	if err != nil {
		return nil, translate(err)
	}
	out := make([]Token, 0, len(resp.GetTokens()))
	for _, t := range resp.GetTokens() {
		out = append(out, toToken(t, s.now()))
	}
	return out, nil
}

func (s *Service) Revoke(ctx context.Context, id usecase.Identity, tokenID string) error {
	if _, err := s.Auth.RevokeMcpToken(gatewaygrpc.AttachIdentity(ctx, id), &authv1.RevokeMcpTokenRequest{Jti: tokenID}); err != nil {
		return translate(err)
	}
	if s.OnRevoked != nil {
		s.OnRevoked(tokenID)
	}
	return nil
}

func toToken(t *authv1.McpTokenInfo, now time.Time) Token {
	out := Token{
		ID: t.GetJti(), Name: t.GetName(), Scopes: append([]string{}, t.GetScopes()...),
		CreatedAt: t.GetCreatedAt().AsTime().UTC().Format(time.RFC3339), ExpiresAt: t.GetExpiresAt().AsTime().UTC().Format(time.RFC3339),
		Status: "active",
	}
	if t.GetLastUsedAt() != nil {
		out.LastUsedAt = t.GetLastUsedAt().AsTime().UTC().Format(time.RFC3339)
	}
	switch {
	case t.GetRevokedAt() != nil:
		out.Status = "revoked"
	case !t.GetExpiresAt().AsTime().After(now):
		out.Status = "expired"
	}
	return out
}
