package httpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeCliTokenAuthServiceClient implements authv1.AuthServiceClient with a
// single configurable IssueServiceToken response/error and records the last
// request it received — every other RPC is an unused pass-through, present
// only because Go requires every interface method to compile (mirrors
// fakeAdminAuthServiceClient's convention, auth_admin_routes_test.go).
type fakeCliTokenAuthServiceClient struct {
	issueResp    *authv1.IssueServiceTokenResponse
	issueErr     error
	lastIssueReq *authv1.IssueServiceTokenRequest

	listResp    *authv1.ListCliTokensResponse
	listErr     error
	lastListReq *authv1.ListCliTokensRequest

	revokeErr     error
	lastRevokeReq *authv1.RevokeCliTokenRequest
}

func (f *fakeCliTokenAuthServiceClient) IssueServiceToken(ctx context.Context, in *authv1.IssueServiceTokenRequest, opts ...grpc.CallOption) (*authv1.IssueServiceTokenResponse, error) {
	f.lastIssueReq = in
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	return f.issueResp, nil
}

func (f *fakeCliTokenAuthServiceClient) Login(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.LoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) Logout(ctx context.Context, in *authv1.LogoutRequest, opts ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ValidateSession(ctx context.Context, in *authv1.ValidateSessionRequest, opts ...grpc.CallOption) (*authv1.ValidateSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) GetJWKS(ctx context.Context, in *authv1.GetJWKSRequest, opts ...grpc.CallOption) (*authv1.GetJWKSResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) IsServiceTokenRevoked(ctx context.Context, in *authv1.IsServiceTokenRevokedRequest, opts ...grpc.CallOption) (*authv1.IsServiceTokenRevokedResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ListCliTokens(ctx context.Context, in *authv1.ListCliTokensRequest, opts ...grpc.CallOption) (*authv1.ListCliTokensResponse, error) {
	f.lastListReq = in
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResp, nil
}

func (f *fakeCliTokenAuthServiceClient) RevokeCliToken(ctx context.Context, in *authv1.RevokeCliTokenRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	f.lastRevokeReq = in
	if f.revokeErr != nil {
		return nil, f.revokeErr
	}
	return &emptypb.Empty{}, nil
}

func (f *fakeCliTokenAuthServiceClient) CreateUser(ctx context.Context, in *authv1.CreateUserRequest, opts ...grpc.CallOption) (*authv1.CreateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) UpdateUserRole(ctx context.Context, in *authv1.UpdateUserRoleRequest, opts ...grpc.CallOption) (*authv1.UpdateUserRoleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) RevokeSession(ctx context.Context, in *authv1.RevokeSessionRequest, opts ...grpc.CallOption) (*authv1.RevokeSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) QueryAuditLog(ctx context.Context, in *authv1.QueryAuditLogRequest, opts ...grpc.CallOption) (*authv1.QueryAuditLogResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) AppendAuditEntry(ctx context.Context, in *authv1.AppendAuditEntryRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) DeactivateUser(ctx context.Context, in *authv1.DeactivateUserRequest, opts ...grpc.CallOption) (*authv1.DeactivateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ReactivateUser(ctx context.Context, in *authv1.ReactivateUserRequest, opts ...grpc.CallOption) (*authv1.ReactivateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ListSessionsForUser(ctx context.Context, in *authv1.ListSessionsForUserRequest, opts ...grpc.CallOption) (*authv1.ListSessionsForUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ForceRevokeAllSessionsForUser(ctx context.Context, in *authv1.ForceRevokeAllSessionsForUserRequest, opts ...grpc.CallOption) (*authv1.ForceRevokeAllSessionsForUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ForceRevokeSession(ctx context.Context, in *authv1.ForceRevokeSessionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) CreateAccessPolicy(ctx context.Context, in *authv1.CreateAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) GetAccessPolicy(ctx context.Context, in *authv1.GetAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ListAccessPolicies(ctx context.Context, in *authv1.ListAccessPoliciesRequest, opts ...grpc.CallOption) (*authv1.ListAccessPoliciesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) UpdateAccessPolicy(ctx context.Context, in *authv1.UpdateAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) DeleteAccessPolicy(ctx context.Context, in *authv1.DeleteAccessPolicyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) GetAdminStats(ctx context.Context, in *authv1.GetAdminStatsRequest, opts ...grpc.CallOption) (*authv1.GetAdminStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ListTenantMemberDirectory(ctx context.Context, in *authv1.ListTenantMemberDirectoryRequest, opts ...grpc.CallOption) (*authv1.ListTenantMemberDirectoryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) StartSsoLogin(ctx context.Context, in *authv1.StartSsoLoginRequest, opts ...grpc.CallOption) (*authv1.StartSsoLoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) CompleteSsoLogin(ctx context.Context, in *authv1.CompleteSsoLoginRequest, opts ...grpc.CallOption) (*authv1.CompleteSsoLoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

// RefreshSession/UpdateSsoGroupMapping/ListSsoGroupMapping: unused
// pass-throughs, present only so this fake keeps satisfying
// authv1.AuthServiceClient as the interface grows (CR-RBAC-003) — none of
// this file's tests exercise them.
func (f *fakeCliTokenAuthServiceClient) RefreshSession(ctx context.Context, in *authv1.RefreshSessionRequest, opts ...grpc.CallOption) (*authv1.RefreshSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) UpdateSsoGroupMapping(ctx context.Context, in *authv1.UpdateSsoGroupMappingRequest, opts ...grpc.CallOption) (*authv1.UpdateSsoGroupMappingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeCliTokenAuthServiceClient) ListSsoGroupMapping(ctx context.Context, in *authv1.ListSsoGroupMappingRequest, opts ...grpc.CallOption) (*authv1.ListSsoGroupMappingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCliTokenAuthServiceClient) CompleteDevicePairing(ctx context.Context, in *authv1.CompleteDevicePairingRequest, opts ...grpc.CallOption) (*authv1.CompleteDevicePairingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCliTokenAuthServiceClient) ListSessions(ctx context.Context, in *authv1.ListSessionsRequest, opts ...grpc.CallOption) (*authv1.ListSessionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCliTokenAuthServiceClient) UpdateUser(ctx context.Context, in *authv1.UpdateUserRequest, opts ...grpc.CallOption) (*authv1.UpdateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCliTokenAuthServiceClient) InitiateDevicePairing(ctx context.Context, in *authv1.InitiateDevicePairingRequest, opts ...grpc.CallOption) (*authv1.InitiateDevicePairingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCliTokenAuthServiceClient) ListPairedDevices(ctx context.Context, in *authv1.ListPairedDevicesRequest, opts ...grpc.CallOption) (*authv1.ListPairedDevicesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCliTokenAuthServiceClient) UnpairDevice(ctx context.Context, in *authv1.UnpairDeviceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCliTokenAuthServiceClient) ResolveDeviceSharedSecret(ctx context.Context, in *authv1.ResolveDeviceSharedSecretRequest, opts ...grpc.CallOption) (*authv1.ResolveDeviceSharedSecretResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

var _ authv1.AuthServiceClient = (*fakeCliTokenAuthServiceClient)(nil)

// testCliTokenRouter mounts mountCliTokenRoutes standalone and injects a
// test Identity into request context the way authMiddleware would (see
// middleware.go's withIdentity) — mirrors testAuthAdminRouter
// (auth_admin_routes_test.go), targeting mountCliTokenRoutes in isolation.
func testCliTokenRouter(client authv1.AuthServiceClient) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := withIdentity(r.Context(), usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	mountCliTokenRoutes(r, client)
	return r
}

// testCliTokenRouterRealAuth mounts mountCliTokenRoutes behind the REAL
// authMiddleware (no identity injected) — for
// TestIssueCliToken_RequiresAuthentication, which needs an actual 401 from
// authMiddleware itself, not a test double bypassing it.
func testCliTokenRouterRealAuth(client authv1.AuthServiceClient) http.Handler {
	r := chi.NewRouter()
	r.Use(authMiddleware(usecase.NewAuthValidator(&fakeJWKSClient{kid: "irrelevant"}), nil))
	mountCliTokenRoutes(r, client)
	return r
}

func TestIssueCliToken_Success(t *testing.T) {
	client := &fakeCliTokenAuthServiceClient{
		issueResp: &authv1.IssueServiceTokenResponse{
			Jwt:       "signed.jwt.token",
			ExpiresAt: timestamppb.Now(),
		},
	}
	router := testCliTokenRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/auth/cli-tokens/", nil))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if client.lastIssueReq.GetUserId() != "user-1" {
		t.Errorf("UserId sent to gRPC = %q, want %q (the caller's identity)", client.lastIssueReq.GetUserId(), "user-1")
	}
	if client.lastIssueReq.GetAudience() != cliTokenAudience {
		t.Errorf("Audience sent to gRPC = %q, want %q", client.lastIssueReq.GetAudience(), cliTokenAudience)
	}

	var body issueCliTokenResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if body.JWT != "signed.jwt.token" {
		t.Errorf("jwt = %q, want %q", body.JWT, "signed.jwt.token")
	}
}

// TestIssueCliToken_CannotMintForOtherUser is the most important security
// test in this task: a request body claiming a different user_id must be
// completely ignored — the gRPC request actually sent must carry the
// caller's own authenticated identity, never anything from the body.
func TestIssueCliToken_CannotMintForOtherUser(t *testing.T) {
	client := &fakeCliTokenAuthServiceClient{
		issueResp: &authv1.IssueServiceTokenResponse{Jwt: "signed.jwt.token", ExpiresAt: timestamppb.Now()},
	}
	router := testCliTokenRouter(client)

	body := bytes.NewBufferString(`{"user_id":"someone-else"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/auth/cli-tokens/", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if client.lastIssueReq.GetUserId() != "user-1" {
		t.Fatalf("UserId sent to gRPC = %q, want %q (identity.UserID) — a body-supplied user_id must never reach the RPC", client.lastIssueReq.GetUserId(), "user-1")
	}
	if client.lastIssueReq.GetUserId() == "someone-else" {
		t.Fatal("route minted a token using the body's user_id — privilege escalation")
	}
}

func TestIssueCliToken_RequiresAuthentication(t *testing.T) {
	client := &fakeCliTokenAuthServiceClient{}
	router := testCliTokenRouterRealAuth(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/auth/cli-tokens/", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (no cookie/bearer present); body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if client.lastIssueReq != nil {
		t.Error("IssueServiceToken must never be called for an unauthenticated request")
	}
}

func TestIssueCliToken_PropagatesGRPCError(t *testing.T) {
	client := &fakeCliTokenAuthServiceClient{issueErr: status.Error(codes.NotFound, "user not found")}
	router := testCliTokenRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/auth/cli-tokens/", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (mapped from codes.NotFound); body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestListCliToken_ReturnsOnlyCallerOwnTokens(t *testing.T) {
	client := &fakeCliTokenAuthServiceClient{
		listResp: &authv1.ListCliTokensResponse{
			Tokens: []*authv1.CliToken{
				{Jti: "jti-1", Audience: cliTokenAudience, IssuedAt: timestamppb.Now(), ExpiresAt: timestamppb.Now()},
			},
		},
	}
	router := testCliTokenRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/cli-tokens/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if client.lastListReq.GetUserId() != "user-1" {
		t.Errorf("UserId sent to gRPC = %q, want %q (the caller's identity, never a request param)", client.lastListReq.GetUserId(), "user-1")
	}

	var body listCliTokensResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if len(body.Tokens) != 1 || body.Tokens[0].JTI != "jti-1" {
		t.Fatalf("body.Tokens = %+v, want 1 token with jti-1", body.Tokens)
	}
}

func TestRevokeCliToken_Success(t *testing.T) {
	client := &fakeCliTokenAuthServiceClient{}
	router := testCliTokenRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v1/auth/cli-tokens/jti-1", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if client.lastRevokeReq.GetJti() != "jti-1" {
		t.Errorf("Jti sent to gRPC = %q, want %q", client.lastRevokeReq.GetJti(), "jti-1")
	}
	if client.lastRevokeReq.GetUserId() != "user-1" {
		t.Errorf("UserId sent to gRPC = %q, want %q (the caller's identity)", client.lastRevokeReq.GetUserId(), "user-1")
	}
}

// TestRevokeCliToken_CannotRevokeAnotherUsersToken is the second most
// important security test in CR-CLI-002 (after
// TestIssueCliToken_CannotMintForOtherUser): the route always sends the
// CALLER's own identity.UserID as UserId — it never derives ownership
// from the {id}/jti path param — and correctly surfaces auth-service's
// rejection (PermissionDenied, mapped to 403) as an error response, never
// a silent 200/204, when the usecase determines the jti belongs to
// someone else.
func TestRevokeCliToken_CannotRevokeAnotherUsersToken(t *testing.T) {
	client := &fakeCliTokenAuthServiceClient{
		revokeErr: status.Error(codes.NotFound, "issued token not found"), // mirrors RevokeCliToken usecase's real NotFound-not-PermissionDenied choice (revoke_cli_token.go's doc comment)
	}
	router := testCliTokenRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v1/auth/cli-tokens/jti-belongs-to-user-b", nil))

	if rec.Code == http.StatusOK || rec.Code == http.StatusNoContent {
		t.Fatalf("status = %d, want an error status (not 200/204) when the usecase rejects a cross-user revoke", rec.Code)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (mapped from codes.NotFound)", rec.Code, http.StatusNotFound)
	}
	// Even though the path names a jti "belonging to user-b", the route
	// must still have sent the CALLER's own identity, never anything
	// derived from the path or body.
	if client.lastRevokeReq.GetUserId() != "user-1" {
		t.Errorf("UserId sent to gRPC = %q, want %q (the caller's identity, not inferred from the jti)", client.lastRevokeReq.GetUserId(), "user-1")
	}
}

func TestRevokeCliToken_PropagatesGRPCError(t *testing.T) {
	client := &fakeCliTokenAuthServiceClient{revokeErr: status.Error(codes.Internal, "boom")}
	router := testCliTokenRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v1/auth/cli-tokens/jti-1", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
