package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/mcpscope"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// McpUsecases bundles the MCP PAT and principal-resolution usecases.
type McpUsecases struct {
	Issue   *usecase.IssueMcpToken
	List    *usecase.ListMcpTokens
	Revoke  *usecase.RevokeMcpToken
	Resolve *usecase.ResolveMcpPrincipal
}

// McpServer layers the MCP RPCs over any AuthServiceServer (wrapper rather
// than more fields on Server, same reason as OAuthServer).
type McpServer struct {
	authv1.AuthServiceServer
	mcp McpUsecases
}

func WithMcp(inner authv1.AuthServiceServer, uc McpUsecases) *McpServer {
	return &McpServer{AuthServiceServer: inner, mcp: uc}
}

func (s *McpServer) IssueMcpToken(ctx context.Context, req *authv1.IssueMcpTokenRequest) (*authv1.IssueMcpTokenResponse, error) {
	tenantID, _ := tenant.TenantID(ctx)
	userID, _ := tenant.UserID(ctx)
	out, err := s.mcp.Issue.Execute(ctx, usecase.IssueMcpTokenInput{
		TenantID: tenantID, UserID: userID, Name: req.GetName(), Scopes: req.GetScopes(), ExpiresInDays: int(req.GetExpiresInDays()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.IssueMcpTokenResponse{Token: toProtoMcpToken(out.Token), Secret: out.Secret}, nil
}

func (s *McpServer) ListMcpTokens(ctx context.Context, _ *emptypb.Empty) (*authv1.ListMcpTokensResponse, error) {
	tenantID, _ := tenant.TenantID(ctx)
	userID, _ := tenant.UserID(ctx)
	toks, err := s.mcp.List.Execute(ctx, tenantID, userID)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &authv1.ListMcpTokensResponse{Tokens: make([]*authv1.McpTokenInfo, 0, len(toks))}
	for _, t := range toks {
		resp.Tokens = append(resp.Tokens, toProtoMcpToken(t))
	}
	return resp, nil
}

func (s *McpServer) RevokeMcpToken(ctx context.Context, req *authv1.RevokeMcpTokenRequest) (*emptypb.Empty, error) {
	tenantID, _ := tenant.TenantID(ctx)
	userID, _ := tenant.UserID(ctx)
	if err := s.mcp.Revoke.Execute(ctx, tenantID, userID, req.GetJti()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *McpServer) ResolveMcpPrincipal(ctx context.Context, req *authv1.ResolveMcpPrincipalRequest) (*authv1.ResolveMcpPrincipalResponse, error) {
	out, err := s.mcp.Resolve.Execute(ctx, usecase.ResolveMcpPrincipalInput{
		JTI: req.GetJti(), UserID: req.GetUserId(), TenantID: req.GetTenantId(), TokenUse: req.GetTokenUse(),
		FamilyID: req.GetFamilyId(), GrantID: req.GetGrantId(), ClientID: req.GetClientId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.ResolveMcpPrincipalResponse{Active: out.Active, InactiveReason: out.InactiveReason, Role: out.Role}, nil
}

func toProtoMcpToken(t domain.McpToken) *authv1.McpTokenInfo {
	scopes, _ := mcpscope.Parse(t.Scope)
	out := &authv1.McpTokenInfo{
		Jti: t.JTI, Name: t.Name, Scopes: scopes,
		CreatedAt: timestamppb.New(t.CreatedAt), ExpiresAt: timestamppb.New(t.ExpiresAt),
	}
	if t.LastUsedAt != nil {
		out.LastUsedAt = timestamppb.New(*t.LastUsedAt)
	}
	if t.RevokedAt != nil {
		out.RevokedAt = timestamppb.New(*t.RevokedAt)
	}
	return out
}
