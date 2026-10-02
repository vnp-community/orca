// Package authclient implements usecase.AuthorizationServer over auth-service's
// OAuth* gRPC group. It forwards the caller's tenant/user as gRPC metadata,
// bounds every call with a deadline, and presents the shared internal-caller
// secret that auth-service requires on mcp-service-only methods.
package authclient

import (
	"context"
	"errors"
	"regexp"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// DefaultTimeout bounds each auth-service call (arch/08: deadline on every
// outbound call; 5s default).
const DefaultTimeout = 5 * time.Second

// Dial opens the connection to auth-service. Insecure transport matches every
// other internal dialer in this repo (mesh mTLS is a known cross-cutting gap).
func Dial(addr, internalToken string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(internalToken)))
}

type Client struct {
	api     authv1.AuthServiceClient
	timeout time.Duration
}

func New(api authv1.AuthServiceClient, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{api: api, timeout: timeout}
}

// call prepares the outgoing context: identity metadata plus a deadline.
func (c *Client) call(ctx context.Context) (context.Context, context.CancelFunc) {
	var kv []string
	if v, ok := tenant.TenantID(ctx); ok {
		kv = append(kv, grpcmw.MetadataTenantID, v)
	}
	if v, ok := tenant.UserID(ctx); ok {
		kv = append(kv, grpcmw.MetadataUserID, v)
	}
	if v, ok := tenant.ClientIP(ctx); ok {
		kv = append(kv, grpcmw.MetadataClientIP, v)
	}
	if len(kv) > 0 {
		ctx = metadata.AppendToOutgoingContext(ctx, kv...)
	}
	return context.WithTimeout(ctx, c.timeout)
}

var codePrefix = regexp.MustCompile(`^([A-Z][A-Z0-9_]+): (.*)$`)

// translate turns an auth-service gRPC error into an apperrors.AppError that
// keeps auth's own `CODE` (e.g. OAUTH_INVALID_REDIRECT_URI) so the gateway can
// act on it; transport failures become MCP_UNAVAILABLE / MCP_TIMEOUT.
func translate(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return apperrors.New(apperrors.KindInternal, "MCP_INTERNAL", "authorization server call failed", err)
	}
	switch st.Code() {
	case codes.Unavailable:
		return status.Error(codes.Unavailable, "MCP_UNAVAILABLE: authorization server is unavailable")
	case codes.DeadlineExceeded, codes.Canceled:
		return status.Error(codes.DeadlineExceeded, "MCP_TIMEOUT: authorization server did not answer in time")
	}
	code, msg := "MCP_INTERNAL", st.Message()
	if m := codePrefix.FindStringSubmatch(st.Message()); m != nil {
		code, msg = m[1], m[2]
	}
	kind := apperrors.KindInternal
	switch st.Code() {
	case codes.InvalidArgument:
		kind = apperrors.KindInvalidArgument
	case codes.NotFound:
		kind = apperrors.KindNotFound
	case codes.AlreadyExists:
		kind = apperrors.KindAlreadyExists
	case codes.PermissionDenied:
		kind = apperrors.KindPermissionDenied
	case codes.FailedPrecondition:
		kind = apperrors.KindFailedPrecondition
	case codes.Unauthenticated:
		kind = apperrors.KindUnauthenticated
	}
	return apperrors.New(kind, code, msg, errors.New(st.Code().String()))
}

func (c *Client) ValidateAuthorizeRequest(ctx context.Context, p usecase.AuthorizeParams) (usecase.AuthorizeInfo, error) {
	ctx, cancel := c.call(ctx)
	defer cancel()
	r, err := c.api.OAuthValidateAuthorizeRequest(ctx, &authv1.OAuthValidateAuthorizeRequestRequest{
		ResponseType: p.ResponseType, ClientId: p.ClientID, RedirectUri: p.RedirectURI, Scope: p.Scope,
		CodeChallenge: p.CodeChallenge, CodeChallengeMethod: p.CodeChallengeMethod, Resource: p.Resource,
	})
	if err != nil {
		return usecase.AuthorizeInfo{}, translate(err)
	}
	return usecase.AuthorizeInfo{
		ClientID: r.GetClientId(), ClientName: r.GetClientName(), ClientURI: r.GetClientUri(), RegisteredVia: r.GetRegisteredVia(),
		Scopes: r.GetScopes(), RedirectURI: r.GetRedirectUri(), Resource: r.GetResource(),
	}, nil
}

func (c *Client) EnsureClientForTenant(ctx context.Context, clientID string, dcrEnabled bool) (usecase.OAuthClientView, error) {
	ctx, cancel := c.call(ctx)
	defer cancel()
	v, err := c.api.OAuthEnsureClientForTenant(ctx, &authv1.OAuthEnsureClientForTenantRequest{ClientId: clientID, DcrEnabled: dcrEnabled})
	if err != nil {
		return usecase.OAuthClientView{}, translate(err)
	}
	return toView(v), nil
}

func (c *Client) IssueAuthCode(ctx context.Context, in usecase.IssueCodeInput) (string, error) {
	ctx, cancel := c.call(ctx)
	defer cancel()
	r, err := c.api.OAuthIssueAuthCode(ctx, &authv1.OAuthIssueAuthCodeRequest{
		ClientId: in.ClientID, RedirectUri: in.RedirectURI, Scopes: in.Scopes, CodeChallenge: in.CodeChallenge,
		Resource: in.Resource, GrantId: in.GrantID,
	})
	if err != nil {
		return "", translate(err)
	}
	return r.GetCode(), nil
}

func (c *Client) RevokeGrant(ctx context.Context, grantID, reason string) error {
	ctx, cancel := c.call(ctx)
	defer cancel()
	_, err := c.api.OAuthRevokeGrant(ctx, &authv1.OAuthRevokeGrantRequest{GrantId: grantID, Reason: reason})
	return translate(err)
}

func (c *Client) ListClientsForTenant(ctx context.Context) ([]usecase.OAuthClientView, error) {
	ctx, cancel := c.call(ctx)
	defer cancel()
	r, err := c.api.OAuthListClientsForTenant(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, translate(err)
	}
	out := make([]usecase.OAuthClientView, 0, len(r.GetClients()))
	for _, v := range r.GetClients() {
		out = append(out, toView(v))
	}
	return out, nil
}

func (c *Client) SetClientStatus(ctx context.Context, clientID, st string) (usecase.OAuthClientView, error) {
	ctx, cancel := c.call(ctx)
	defer cancel()
	v, err := c.api.OAuthSetClientStatus(ctx, &authv1.OAuthSetClientStatusRequest{ClientId: clientID, Status: st})
	if err != nil {
		return usecase.OAuthClientView{}, translate(err)
	}
	return toView(v), nil
}

func (c *Client) MemberNames(ctx context.Context) (map[string]string, error) {
	ctx, cancel := c.call(ctx)
	defer cancel()
	r, err := c.api.ListTenantMemberDirectory(ctx, &authv1.ListTenantMemberDirectoryRequest{})
	if err != nil {
		return nil, translate(err)
	}
	out := make(map[string]string, len(r.GetMembers()))
	for _, m := range r.GetMembers() {
		name := m.GetName()
		if name == "" {
			name = m.GetEmail()
		}
		out[m.GetId()] = name
	}
	return out, nil
}

func toView(v *authv1.OAuthClientTenantView) usecase.OAuthClientView {
	out := usecase.OAuthClientView{
		ClientID: v.GetClientId(), Name: v.GetClientName(), ClientURI: v.GetClientUri(), RegisteredVia: v.GetRegisteredVia(),
		Status: v.GetStatus(), RedirectURIs: v.GetRedirectUris(),
	}
	if v.GetCreatedAt() != nil {
		out.CreatedAt = v.GetCreatedAt().AsTime()
	}
	if v.GetLastUsedAt() != nil {
		t := v.GetLastUsedAt().AsTime()
		out.LastUsedAt = &t
	}
	return out
}

var _ usecase.AuthorizationServer = (*Client)(nil)
