package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// AuthorizationUsecases bundles the consent/grant usecases.
type AuthorizationUsecases struct {
	CreateConsent *usecase.CreateConsentRequest
	GetConsent    *usecase.GetConsentRequest
	DecideConsent *usecase.DecideConsent
	ListGrants    *usecase.ListGrants
	RevokeGrant   *usecase.RevokeGrant
	ListClients   *usecase.ListOAuthClients
	SetStatus     *usecase.SetOAuthClientStatus
}

// AuthorizationServer adds the consent/grant RPCs to Server. A wrapper rather
// than new fields on Server keeps Server's constructor and its callers
// untouched; its methods shadow the Unimplemented ones of the embedded Server.
type AuthorizationServer struct {
	*Server
	uc AuthorizationUsecases
}

func WithAuthorization(s *Server, uc AuthorizationUsecases) *AuthorizationServer {
	return &AuthorizationServer{Server: s, uc: uc}
}

// toStatus keeps already-formed gRPC statuses (auth-service transport errors
// such as MCP_UNAVAILABLE) intact instead of flattening them to "internal".
func toStatus(err error) error {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return apperrors.ToGRPCStatus(err)
	}
	if st, ok := status.FromError(err); ok {
		return st.Err()
	}
	return apperrors.ToGRPCStatus(err)
}

func (s *AuthorizationServer) CreateConsentRequest(ctx context.Context, req *mcpv1.CreateConsentRequestRequest) (*mcpv1.CreateConsentRequestResponse, error) {
	out, err := s.uc.CreateConsent.Execute(ctx, usecase.CreateConsentRequestInput{
		AuthorizeParams: usecase.AuthorizeParams{
			ResponseType: req.GetResponseType(), ClientID: req.GetClientId(), RedirectURI: req.GetRedirectUri(), Scope: req.GetScope(),
			CodeChallenge: req.GetCodeChallenge(), CodeChallengeMethod: req.GetCodeChallengeMethod(), Resource: req.GetResource(),
		},
		State: req.GetState(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.CreateConsentRequestResponse{RequestId: out.RequestID, RedirectUrl: out.RedirectURL}, nil
}

func (s *AuthorizationServer) GetConsentRequest(ctx context.Context, req *mcpv1.GetConsentRequestRequest) (*mcpv1.ConsentRequestView, error) {
	v, err := s.uc.GetConsent.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, toStatus(err)
	}
	r := v.Request
	scopes := make([]*mcpv1.ScopeDescriptor, 0, len(v.Scopes))
	for _, d := range v.Scopes {
		scopes = append(scopes, toProtoScope(d))
	}
	return &mcpv1.ConsentRequestView{
		RequestId: r.ID, ClientId: r.ClientID, ClientName: r.ClientName, ClientUri: r.ClientURI, RedirectHost: v.RedirectHost,
		Scopes: scopes, AlreadyGranted: v.AlreadyGranted, TenantId: r.TenantID, IsNewClient: r.IsNewClient,
		RegisteredViaDcr: r.RegisteredViaDCR, ExpiresAt: timestamppb.New(r.ExpiresAt),
	}, nil
}

func (s *AuthorizationServer) DecideConsent(ctx context.Context, req *mcpv1.DecideConsentRequest) (*mcpv1.DecideConsentResponse, error) {
	out, err := s.uc.DecideConsent.Execute(ctx, usecase.DecideConsentInput{
		RequestID: req.GetRequestId(), Decision: req.GetDecision(), Scopes: req.GetScopes(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.DecideConsentResponse{RedirectUrl: out.RedirectURL}, nil
}

func (s *AuthorizationServer) ListGrants(ctx context.Context, req *mcpv1.ListGrantsRequest) (*mcpv1.ListGrantsResponse, error) {
	views, err := s.uc.ListGrants.Execute(ctx, usecase.ListGrantsInput{AllUsers: req.GetAllUsers(), UserID: req.GetUserId()})
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.ListGrantsResponse{Grants: make([]*mcpv1.Grant, 0, len(views))}
	for _, v := range views {
		resp.Grants = append(resp.Grants, toProtoGrant(v))
	}
	return resp, nil
}

func (s *AuthorizationServer) RevokeGrant(ctx context.Context, req *mcpv1.RevokeGrantRequest) (*mcpv1.RevokeGrantResponse, error) {
	if err := s.uc.RevokeGrant.Execute(ctx, usecase.RevokeGrantInput{GrantID: req.GetGrantId(), Admin: req.GetAdmin()}); err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.RevokeGrantResponse{}, nil
}

func (s *AuthorizationServer) ListOAuthClients(ctx context.Context, _ *emptypb.Empty) (*mcpv1.ListOAuthClientsResponse, error) {
	items, err := s.uc.ListClients.Execute(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.ListOAuthClientsResponse{Clients: make([]*mcpv1.OAuthClient, 0, len(items))}
	for _, it := range items {
		resp.Clients = append(resp.Clients, toProtoClient(it))
	}
	return resp, nil
}

func (s *AuthorizationServer) SetOAuthClientStatus(ctx context.Context, req *mcpv1.SetOAuthClientStatusRequest) (*mcpv1.OAuthClient, error) {
	it, err := s.uc.SetStatus.Execute(ctx, req.GetClientId(), req.GetStatus())
	if err != nil {
		return nil, toStatus(err)
	}
	return toProtoClient(it), nil
}

func toProtoGrant(v usecase.GrantView) *mcpv1.Grant {
	g := v.Grant
	out := &mcpv1.Grant{
		Id: g.ID, ClientId: g.ClientID, ClientName: g.ClientName, ClientUri: g.ClientURI, Scopes: g.Scopes,
		CreatedAt: timestamppb.New(g.CreatedAt), Status: g.Status, UserId: g.UserID, UserName: v.UserName,
	}
	if g.LastUsedAt != nil {
		out.LastUsedAt = timestamppb.New(*g.LastUsedAt)
	}
	return out
}

func toProtoClient(it usecase.OAuthClientItem) *mcpv1.OAuthClient {
	out := &mcpv1.OAuthClient{
		ClientId: it.ClientID, Name: it.Name, RedirectUris: it.RedirectURIs, RegisteredVia: it.RegisteredVia, Status: it.Status,
		ActiveGrants: int32(it.ActiveGrants), ClientUri: it.ClientURI,
	}
	if !it.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(it.CreatedAt)
	}
	if it.LastUsedAt != nil {
		out.LastUsedAt = timestamppb.New(*it.LastUsedAt)
	}
	return out
}
