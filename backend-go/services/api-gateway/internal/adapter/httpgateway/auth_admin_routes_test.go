package httpgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
}

func (f *fakeAdminAuthServiceClient) Login(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.LoginResponse, error) {
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
	return nil, status.Error(codes.Unimplemented, "not used by this test")
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
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeAdminAuthServiceClient) QueryAuditLog(ctx context.Context, in *authv1.QueryAuditLogRequest, opts ...grpc.CallOption) (*authv1.QueryAuditLogResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
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
