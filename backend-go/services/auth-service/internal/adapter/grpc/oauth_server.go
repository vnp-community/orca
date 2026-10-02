package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// OAuthInternalMethods are the RPCs only mcp-service may call. They are
// guarded by common/internalcaller (see cmd/server/main.go); the four public
// OAuth RPCs (register, validate, exchange, revoke) take no identity.
var OAuthInternalMethods = []string{
	authv1.AuthService_OAuthIssueAuthCode_FullMethodName,
	authv1.AuthService_OAuthRevokeGrant_FullMethodName,
	authv1.AuthService_OAuthListClientsForTenant_FullMethodName,
	authv1.AuthService_OAuthSetClientStatus_FullMethodName,
	authv1.AuthService_OAuthEnsureClientForTenant_FullMethodName,
}

// OAuthUsecases bundles the authorization-server usecases.
type OAuthUsecases struct {
	Register  *usecase.OAuthRegisterClient
	Validate  *usecase.OAuthValidateAuthorizeRequest
	IssueCode *usecase.OAuthIssueAuthCode
	Exchange  *usecase.OAuthExchangeToken
	Revoke    *usecase.OAuthRevokeToken
	RevokeGr  *usecase.OAuthRevokeGrant
	List      *usecase.OAuthListClientsForTenant
	SetStatus *usecase.OAuthSetClientStatus
	Ensure    *usecase.OAuthEnsureClientForTenant
}

// OAuthServer adds the OAuth RPC group to Server. It is a wrapper rather than
// more fields on Server so the existing constructor (40+ positional
// arguments) and its callers stay untouched; its OAuth* methods shadow the
// Unimplemented ones promoted from the embedded Server.
type OAuthServer struct {
	*Server
	oauth OAuthUsecases
}

func WithOAuth(s *Server, uc OAuthUsecases) *OAuthServer { return &OAuthServer{Server: s, oauth: uc} }

func (s *OAuthServer) OAuthRegisterClient(ctx context.Context, req *authv1.OAuthRegisterClientRequest) (*authv1.OAuthRegisterClientResponse, error) {
	out, err := s.oauth.Register.Execute(ctx, usecase.OAuthRegisterClientInput{
		ClientName: req.GetClientName(), ClientURI: req.GetClientUri(), RedirectURIs: req.GetRedirectUris(),
		TokenEndpointAuthMethod: req.GetTokenEndpointAuthMethod(), GrantTypes: req.GetGrantTypes(), ResponseTypes: req.GetResponseTypes(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	c := out.Client
	return &authv1.OAuthRegisterClientResponse{
		ClientId: c.ClientID, ClientIdIssuedAt: c.CreatedAt.Unix(), ClientName: c.ClientName, ClientUri: c.ClientURI,
		RedirectUris: c.RedirectURIs, TokenEndpointAuthMethod: "none", GrantTypes: out.GrantTypes, ResponseTypes: out.ResponseTypes,
	}, nil
}

func (s *OAuthServer) OAuthValidateAuthorizeRequest(ctx context.Context, req *authv1.OAuthValidateAuthorizeRequestRequest) (*authv1.OAuthAuthorizeRequestInfo, error) {
	info, err := s.oauth.Validate.Execute(ctx, usecase.OAuthAuthorizeParams{
		ResponseType: req.GetResponseType(), ClientID: req.GetClientId(), RedirectURI: req.GetRedirectUri(), Scope: req.GetScope(),
		CodeChallenge: req.GetCodeChallenge(), CodeChallengeMethod: req.GetCodeChallengeMethod(), Resource: req.GetResource(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.OAuthAuthorizeRequestInfo{
		ClientId: info.Client.ClientID, ClientName: info.Client.ClientName, ClientUri: info.Client.ClientURI,
		Scopes: info.Scopes, RegisteredVia: info.Client.RegisteredVia, RedirectUri: info.RedirectURI, Resource: info.Resource,
	}, nil
}

func (s *OAuthServer) OAuthIssueAuthCode(ctx context.Context, req *authv1.OAuthIssueAuthCodeRequest) (*authv1.OAuthIssueAuthCodeResponse, error) {
	tenantID, _ := tenant.TenantID(ctx)
	userID, _ := tenant.UserID(ctx)
	out, err := s.oauth.IssueCode.Execute(ctx, usecase.OAuthIssueAuthCodeInput{
		TenantID: tenantID, UserID: userID, ClientID: req.GetClientId(), RedirectURI: req.GetRedirectUri(), Scopes: req.GetScopes(),
		CodeChallenge: req.GetCodeChallenge(), Resource: req.GetResource(), GrantID: req.GetGrantId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.OAuthIssueAuthCodeResponse{Code: out.Code, ExpiresAt: timestamppb.New(out.ExpiresAt)}, nil
}

func (s *OAuthServer) OAuthExchangeToken(ctx context.Context, req *authv1.OAuthExchangeTokenRequest) (*authv1.OAuthTokenResponse, error) {
	out, err := s.oauth.Exchange.Execute(ctx, usecase.OAuthExchangeTokenInput{
		GrantType: req.GetGrantType(), Code: req.GetCode(), RedirectURI: req.GetRedirectUri(), CodeVerifier: req.GetCodeVerifier(),
		ClientID: req.GetClientId(), RefreshToken: req.GetRefreshToken(), Resource: req.GetResource(), Scope: req.GetScope(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.OAuthTokenResponse{
		AccessToken: out.AccessToken, ExpiresIn: int32(out.ExpiresIn), RefreshToken: out.RefreshToken, Scope: out.Scope,
	}, nil
}

func (s *OAuthServer) OAuthRevokeToken(ctx context.Context, req *authv1.OAuthRevokeTokenRequest) (*emptypb.Empty, error) {
	if err := s.oauth.Revoke.Execute(ctx, usecase.OAuthRevokeTokenInput{Token: req.GetToken(), ClientID: req.GetClientId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *OAuthServer) OAuthRevokeGrant(ctx context.Context, req *authv1.OAuthRevokeGrantRequest) (*emptypb.Empty, error) {
	tenantID, userID, err := oauthCaller(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.oauth.RevokeGr.Execute(ctx, usecase.OAuthRevokeGrantInput{
		TenantID: tenantID, GrantID: req.GetGrantId(), Reason: req.GetReason(), ActorUserID: userID,
	}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *OAuthServer) OAuthListClientsForTenant(ctx context.Context, _ *emptypb.Empty) (*authv1.OAuthListClientsResponse, error) {
	tenantID, _, err := oauthCaller(ctx)
	if err != nil {
		return nil, err
	}
	views, err := s.oauth.List.Execute(ctx, tenantID)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &authv1.OAuthListClientsResponse{Clients: make([]*authv1.OAuthClientTenantView, 0, len(views))}
	for _, v := range views {
		resp.Clients = append(resp.Clients, toProtoOAuthClientView(v))
	}
	return resp, nil
}

func (s *OAuthServer) OAuthSetClientStatus(ctx context.Context, req *authv1.OAuthSetClientStatusRequest) (*authv1.OAuthClientTenantView, error) {
	tenantID, userID, err := oauthCaller(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.oauth.SetStatus.Execute(ctx, usecase.OAuthSetClientStatusInput{
		TenantID: tenantID, ClientID: req.GetClientId(), Status: domain.OAuthClientStatus(req.GetStatus()), ActorUserID: userID,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoOAuthClientView(v), nil
}

func (s *OAuthServer) OAuthEnsureClientForTenant(ctx context.Context, req *authv1.OAuthEnsureClientForTenantRequest) (*authv1.OAuthClientTenantView, error) {
	tenantID, _, err := oauthCaller(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.oauth.Ensure.Execute(ctx, usecase.OAuthEnsureClientForTenantInput{
		TenantID: tenantID, ClientID: req.GetClientId(), DCREnabled: req.GetDcrEnabled(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoOAuthClientView(v), nil
}

// oauthCaller returns the tenant and acting user forwarded in gRPC metadata
// by mcp-service; the user may be empty for system-driven calls (reconcile).
func oauthCaller(ctx context.Context) (tenantID, userID string, err error) {
	tenantID, ok := tenant.TenantID(ctx)
	if !ok {
		return "", "", apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, usecase.CodeOAuthAccessDenied, "tenant context is required", nil))
	}
	userID, _ = tenant.UserID(ctx)
	return tenantID, userID, nil
}

func toProtoOAuthClientView(v domain.OAuthClientView) *authv1.OAuthClientTenantView {
	out := &authv1.OAuthClientTenantView{
		ClientId: v.Client.ClientID, ClientName: v.Client.ClientName, ClientUri: v.Client.ClientURI,
		RedirectUris: v.Client.RedirectURIs, RegisteredVia: v.Client.RegisteredVia, Status: string(v.Status.Status),
		CreatedAt: timestamppb.New(v.Client.CreatedAt),
	}
	if v.Client.LastUsedAt != nil {
		out.LastUsedAt = timestamppb.New(*v.Client.LastUsedAt)
	}
	if !v.Status.UpdatedAt.IsZero() {
		out.StatusUpdatedAt = timestamppb.New(v.Status.UpdatedAt)
	}
	return out
}
