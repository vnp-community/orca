package httpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeAdminAuthServiceClient implements authv1.AuthServiceClient with a
// single configurable canned response/error used by whichever RPC the test
// exercises — the login-flow methods (Login, Logout, ValidateSession,
// IssueServiceToken, GetJWKS) are thin unused pass-throughs, present only
// because Go requires every interface method to compile.
type fakeAdminAuthServiceClient struct {
	listUsersResp *authv1.ListUsersResponse
	err           error

	loginResp      *authv1.LoginResponse
	loginErr       error
	lastLoginReq   *authv1.LoginRequest
	loginCallCount int

	updateUserResp         *authv1.UpdateUserResponse
	lastUpdateUserReq      *authv1.UpdateUserRequest
	listSessionsAllResp    *authv1.ListSessionsResponse
	lastListSessionsAllReq *authv1.ListSessionsRequest

	createUserResp    *authv1.CreateUserResponse
	lastCreateUserReq *authv1.CreateUserRequest

	statsResp             *authv1.GetAdminStatsResponse
	queryAuditLogResp     *authv1.QueryAuditLogResponse
	queryAuditLogResps    []*authv1.QueryAuditLogResponse // consumed in order, one per call, for multi-page export tests
	lastQueryAuditLogReq  *authv1.QueryAuditLogRequest
	deactivateUserResp    *authv1.DeactivateUserResponse
	listSessionsResp      *authv1.ListSessionsForUserResponse
	forceRevokeAllResp    *authv1.ForceRevokeAllSessionsForUserResponse
	listPoliciesResp      *authv1.ListAccessPoliciesResponse
	createPolicyResp      *authv1.AccessPolicy
	updatePolicyResp      *authv1.AccessPolicy
	lastCreatePolicyReq   *authv1.CreateAccessPolicyRequest
	lastUpdatePolicyReq   *authv1.UpdateAccessPolicyRequest
	lastDeactivateUserReq *authv1.DeactivateUserRequest
	lastListSessionsReq   *authv1.ListSessionsForUserRequest
	lastForceRevokeAllReq *authv1.ForceRevokeAllSessionsForUserRequest
	lastDeletePolicyReq   *authv1.DeleteAccessPolicyRequest
}

func (f *fakeAdminAuthServiceClient) Login(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.LoginResponse, error) {
	f.loginCallCount++
	f.lastLoginReq = in
	if f.loginErr != nil {
		return nil, f.loginErr
	}
	if f.loginResp != nil {
		return f.loginResp, nil
	}
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeAdminAuthServiceClient) Logout(ctx context.Context, in *authv1.LogoutRequest, opts ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeAdminAuthServiceClient) ValidateSession(ctx context.Context, in *authv1.ValidateSessionRequest, opts ...grpc.CallOption) (*authv1.ValidateSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeAdminAuthServiceClient) IssueServiceToken(ctx context.Context, in *authv1.IssueServiceTokenRequest, opts ...grpc.CallOption) (*authv1.IssueServiceTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeAdminAuthServiceClient) GetJWKS(ctx context.Context, in *authv1.GetJWKSRequest, opts ...grpc.CallOption) (*authv1.GetJWKSResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeAdminAuthServiceClient) CreateUser(ctx context.Context, in *authv1.CreateUserRequest, opts ...grpc.CallOption) (*authv1.CreateUserResponse, error) {
	f.lastCreateUserReq = in
	if f.err != nil {
		return nil, f.err
	}
	if f.createUserResp != nil {
		return f.createUserResp, nil
	}
	return &authv1.CreateUserResponse{}, nil
}

func (f *fakeAdminAuthServiceClient) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.listUsersResp, nil
}

func (f *fakeAdminAuthServiceClient) UpdateUserRole(ctx context.Context, in *authv1.UpdateUserRoleRequest, opts ...grpc.CallOption) (*authv1.UpdateUserRoleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeAdminAuthServiceClient) RevokeSession(ctx context.Context, in *authv1.RevokeSessionRequest, opts ...grpc.CallOption) (*authv1.RevokeSessionResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &authv1.RevokeSessionResponse{}, nil
}

func (f *fakeAdminAuthServiceClient) QueryAuditLog(ctx context.Context, in *authv1.QueryAuditLogRequest, opts ...grpc.CallOption) (*authv1.QueryAuditLogResponse, error) {
	f.lastQueryAuditLogReq = in
	if f.err != nil {
		return nil, f.err
	}
	if len(f.queryAuditLogResps) > 0 {
		resp := f.queryAuditLogResps[0]
		f.queryAuditLogResps = f.queryAuditLogResps[1:]
		return resp, nil
	}
	if f.queryAuditLogResp != nil {
		return f.queryAuditLogResp, nil
	}
	return &authv1.QueryAuditLogResponse{}, nil
}

func (f *fakeAdminAuthServiceClient) DeactivateUser(ctx context.Context, in *authv1.DeactivateUserRequest, opts ...grpc.CallOption) (*authv1.DeactivateUserResponse, error) {
	f.lastDeactivateUserReq = in
	if f.err != nil {
		return nil, f.err
	}
	return f.deactivateUserResp, nil
}

func (f *fakeAdminAuthServiceClient) ReactivateUser(ctx context.Context, in *authv1.ReactivateUserRequest, opts ...grpc.CallOption) (*authv1.ReactivateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeAdminAuthServiceClient) ListSessionsForUser(ctx context.Context, in *authv1.ListSessionsForUserRequest, opts ...grpc.CallOption) (*authv1.ListSessionsForUserResponse, error) {
	f.lastListSessionsReq = in
	if f.err != nil {
		return nil, f.err
	}
	return f.listSessionsResp, nil
}

func (f *fakeAdminAuthServiceClient) ForceRevokeAllSessionsForUser(ctx context.Context, in *authv1.ForceRevokeAllSessionsForUserRequest, opts ...grpc.CallOption) (*authv1.ForceRevokeAllSessionsForUserResponse, error) {
	f.lastForceRevokeAllReq = in
	if f.err != nil {
		return nil, f.err
	}
	return f.forceRevokeAllResp, nil
}

func (f *fakeAdminAuthServiceClient) CreateAccessPolicy(ctx context.Context, in *authv1.CreateAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	f.lastCreatePolicyReq = in
	if f.err != nil {
		return nil, f.err
	}
	return f.createPolicyResp, nil
}

func (f *fakeAdminAuthServiceClient) GetAccessPolicy(ctx context.Context, in *authv1.GetAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeAdminAuthServiceClient) ListAccessPolicies(ctx context.Context, in *authv1.ListAccessPoliciesRequest, opts ...grpc.CallOption) (*authv1.ListAccessPoliciesResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.listPoliciesResp, nil
}

func (f *fakeAdminAuthServiceClient) UpdateAccessPolicy(ctx context.Context, in *authv1.UpdateAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	f.lastUpdatePolicyReq = in
	if f.err != nil {
		return nil, f.err
	}
	return f.updatePolicyResp, nil
}

func (f *fakeAdminAuthServiceClient) DeleteAccessPolicy(ctx context.Context, in *authv1.DeleteAccessPolicyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	f.lastDeletePolicyReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &emptypb.Empty{}, nil
}

func (f *fakeAdminAuthServiceClient) GetAdminStats(ctx context.Context, in *authv1.GetAdminStatsRequest, opts ...grpc.CallOption) (*authv1.GetAdminStatsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.statsResp, nil
}

// ListSessions/UpdateUser: added to authv1.AuthServiceClient by SOL-AUTH-04
// (TASK-AUTH-04-01); exercised by admin_routes_test.go's cross-user-sessions
// and update-user tests (TASK-AUTH-04-07).
func (f *fakeAdminAuthServiceClient) ListSessions(ctx context.Context, in *authv1.ListSessionsRequest, opts ...grpc.CallOption) (*authv1.ListSessionsResponse, error) {
	f.lastListSessionsAllReq = in
	if f.err != nil {
		return nil, f.err
	}
	if f.listSessionsAllResp != nil {
		return f.listSessionsAllResp, nil
	}
	return &authv1.ListSessionsResponse{}, nil
}

func (f *fakeAdminAuthServiceClient) UpdateUser(ctx context.Context, in *authv1.UpdateUserRequest, opts ...grpc.CallOption) (*authv1.UpdateUserResponse, error) {
	f.lastUpdateUserReq = in
	if f.err != nil {
		return nil, f.err
	}
	if f.updateUserResp != nil {
		return f.updateUserResp, nil
	}
	return &authv1.UpdateUserResponse{}, nil
}

var _ authv1.AuthServiceClient = (*fakeAdminAuthServiceClient)(nil)

// testAuthAdminRouter mounts mountAuthAdminRoutes standalone and injects a
// test Identity into request context the way authMiddleware would (see
// middleware.go's withIdentity) — this test targets mountAuthAdminRoutes in
// isolation, not the full NewRouter wiring.
func testAuthAdminRouter(client authv1.AuthServiceClient) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := withIdentity(r.Context(), usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	mountAuthAdminRoutes(r, client)
	return r
}

func TestHandleListUsers_SuccessRoundTrip(t *testing.T) {
	client := &fakeAdminAuthServiceClient{
		listUsersResp: &authv1.ListUsersResponse{
			Users: []*authv1.User{
				{Id: "user-1", TenantId: "tenant-1", Email: "a@example.com", Role: authv1.Role_ROLE_ADMIN},
			},
			NextPageToken: "next-token",
		},
	}
	router := testAuthAdminRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/users", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Users []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"users"`
		NextPageToken string `json:"next_page_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if len(body.Users) != 1 || body.Users[0].ID != "user-1" || body.Users[0].Email != "a@example.com" {
		t.Fatalf("unexpected users in response: %+v", body.Users)
	}
	if body.NextPageToken != "next-token" {
		t.Fatalf("next_page_token = %q, want %q", body.NextPageToken, "next-token")
	}
}

func TestHandleListUsers_PermissionDeniedMapsTo403(t *testing.T) {
	client := &fakeAdminAuthServiceClient{
		err: status.Error(codes.PermissionDenied, "caller is not an admin"),
	}
	router := testAuthAdminRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/users", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Code != codes.PermissionDenied.String() {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, codes.PermissionDenied.String())
	}
}

// TestAuthAdminRoutes_CreateUserForwardsPassword is the end-to-end
// regression guard for the bug's headline symptom ("that account is
// permanently unusable") — POST /v1/auth/users with a password in the body
// must reach CreateUser with that exact plaintext in req.Password.
func TestAuthAdminRoutes_CreateUserForwardsPassword(t *testing.T) {
	client := &fakeAdminAuthServiceClient{
		createUserResp: &authv1.CreateUserResponse{User: &authv1.User{Id: "u1", Email: "new@example.com"}},
	}
	router := testAuthAdminRouter(client)

	body, _ := json.Marshal(createUserRequestBody{Email: "new@example.com", Name: "New", Role: "user", Password: "s3cret-pw"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/auth/users", bytes.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if client.lastCreateUserReq.GetPassword() != "s3cret-pw" {
		t.Fatalf("CreateUser called with Password = %q, want %q", client.lastCreateUserReq.GetPassword(), "s3cret-pw")
	}
}

// TestAuthAdminRoutes_QueryAuditLog_ForwardsActionUserIDAndTo verifies
// GET /admin/api/audit?action=...&userId=...&to=... forwards all three
// filters unchanged into the RPC request ("userId" is the spec's literal
// wire name, mapped onto ActorId).
func TestAuthAdminRoutes_QueryAuditLog_ForwardsActionUserIDAndTo(t *testing.T) {
	client := &fakeAdminAuthServiceClient{}
	router := testAuthAdminRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/audit-log?action=login.fail&userId=X&to=2026-01-01T00:00:00Z", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if client.lastQueryAuditLogReq.GetAction() != "login.fail" {
		t.Fatalf("Action = %q, want %q", client.lastQueryAuditLogReq.GetAction(), "login.fail")
	}
	if client.lastQueryAuditLogReq.GetActorId() != "X" {
		t.Fatalf("ActorId = %q, want %q", client.lastQueryAuditLogReq.GetActorId(), "X")
	}
	wantTo, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	if !client.lastQueryAuditLogReq.GetTo().AsTime().Equal(wantTo) {
		t.Fatalf("To = %v, want %v", client.lastQueryAuditLogReq.GetTo().AsTime(), wantTo)
	}
}

// TestAuthAdminRoutes_QueryAuditLog_MalformedToRejectedBeforeRPC verifies a
// malformed `to` value 400s before the RPC call is ever made.
func TestAuthAdminRoutes_QueryAuditLog_MalformedToRejectedBeforeRPC(t *testing.T) {
	client := &fakeAdminAuthServiceClient{}
	router := testAuthAdminRouter(client)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/audit-log?to=not-a-timestamp", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if client.lastQueryAuditLogReq != nil {
		t.Fatal("expected QueryAuditLog RPC not to be called for a malformed `to` value")
	}
}
