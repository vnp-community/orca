package httpgateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// testAdminRouter mounts mountAdminRoutes standalone and injects a test
// Identity into request context the way authMiddleware would (see
// auth_admin_routes_test.go's testAuthAdminRouter, the same pattern).
func testAdminRouter(client authv1.AuthServiceClient) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := withIdentity(r.Context(), usecase.Identity{TenantID: "tenant-1", UserID: "admin-1"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	mountAdminRoutes(r, client)
	return r
}

func TestAdminRoutes_Stats(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{statsResp: &authv1.GetAdminStatsResponse{TotalUsers: 3, ActiveSessions: 5, TotalPolicies: 2}}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	// camelCase, not the raw proto struct's snake_case tags — see
	// admin_routes.go's adminStatsJSON doc comment
	// (specs/backend-go/bugs/missing-v2/ follow-up).
	var resp struct {
		TotalUsers     int32 `json:"totalUsers"`
		ActiveSessions int32 `json:"activeSessions"`
		TotalPolicies  int32 `json:"totalPolicies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, w.Body.String())
	}
	if resp.TotalUsers != 3 || resp.ActiveSessions != 5 || resp.TotalPolicies != 2 {
		t.Fatalf("unexpected stats: totalUsers=%d activeSessions=%d totalPolicies=%d", resp.TotalUsers, resp.ActiveSessions, resp.TotalPolicies)
	}
}

func TestAdminRoutes_ListUsers(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{listUsersResp: &authv1.ListUsersResponse{
		Users: []*authv1.User{{Id: "u1", Email: "a@example.com"}},
	}}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestAdminRoutes_CreateUser(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{}
	router := testAdminRouter(fake)

	body, _ := json.Marshal(createUserRequestBody{Email: "new@example.com", Name: "New", Role: "user"})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/users", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// CreateUser isn't stubbed with a response on fakeAdminAuthServiceClient
	// by default (nil f.err, nil resp field) — this exercises routing +
	// request decoding, not a full round trip; assert it reached the RPC
	// (no 400/404) rather than a specific 2xx body shape.
	if w.Code == http.StatusNotFound || w.Code == http.StatusBadRequest {
		t.Fatalf("unexpected status = %d; body=%s", w.Code, w.Body.String())
	}
}

func TestAdminRoutes_UpdateUserRole(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{}
	router := testAdminRouter(fake)

	body, _ := json.Marshal(updateUserRoleRequestBody{Role: "admin"})
	req := httptest.NewRequest(http.MethodPatch, "/admin/api/users/u1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatalf("expected PATCH /admin/api/users/:id to route, got 404; body=%s", w.Body.String())
	}
}

func TestAdminRoutes_DeactivateUser(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{deactivateUserResp: &authv1.DeactivateUserResponse{
		User: &authv1.User{Id: "u1", IsActive: false},
	}}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/users/u1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if fake.lastDeactivateUserReq.GetUserId() != "u1" {
		t.Fatalf("DeactivateUser called with UserId = %q, want %q", fake.lastDeactivateUserReq.GetUserId(), "u1")
	}
	// userJSON's shape (camelCase, role as a string) — not the raw proto
	// struct's snake_case/numeric-enum tags. See auth_admin_routes.go's
	// userJSON doc comment (specs/backend-go/bugs/missing-v2/ follow-up).
	var user struct {
		IsActive bool   `json:"isActive"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, w.Body.String())
	}
	if user.IsActive {
		t.Fatal("expected returned user to be inactive")
	}
}

func TestAdminRoutes_ListSessions_RequiresUserIDQueryParam(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/sessions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (no cross-user ListAllSessions RPC exists); body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestAdminRoutes_ListSessions_WithUserIDProxiesToListSessionsForUser(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{listSessionsResp: &authv1.ListSessionsForUserResponse{
		Sessions: []*authv1.Session{{Id: "sess-1", UserId: "u1"}},
	}}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/sessions?user_id=u1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if fake.lastListSessionsReq.GetUserId() != "u1" {
		t.Fatalf("ListSessionsForUser called with UserId = %q, want %q", fake.lastListSessionsReq.GetUserId(), "u1")
	}
}

func TestAdminRoutes_RevokeSession(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/sessions/sess-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}
}

func TestAdminRoutes_ForceRevokeAllSessions(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{forceRevokeAllResp: &authv1.ForceRevokeAllSessionsForUserResponse{RevokedCount: 4}}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/users/u1/sessions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if fake.lastForceRevokeAllReq.GetUserId() != "u1" {
		t.Fatalf("ForceRevokeAllSessionsForUser called with UserId = %q, want %q", fake.lastForceRevokeAllReq.GetUserId(), "u1")
	}
	var resp authv1.ForceRevokeAllSessionsForUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, w.Body.String())
	}
	if resp.RevokedCount != 4 {
		t.Fatalf("RevokedCount = %d, want 4", resp.RevokedCount)
	}
}

func TestAdminRoutes_ListPolicies(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{listPoliciesResp: &authv1.ListAccessPoliciesResponse{
		Policies: []*authv1.AccessPolicy{{Id: "p1", Name: "policy-1", Version: 1}},
	}}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/policies", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestAdminRoutes_CreatePolicy(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{createPolicyResp: &authv1.AccessPolicy{Id: "p1", Name: "policy-1", Kind: "rate-tier", Version: 1}}
	router := testAdminRouter(fake)

	body, _ := json.Marshal(createPolicyRequestBody{Name: "policy-1", Kind: "rate-tier", DocumentJSON: `{"rps":10}`})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/policies", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	if fake.lastCreatePolicyReq.GetName() != "policy-1" || fake.lastCreatePolicyReq.GetDocumentJson() != `{"rps":10}` {
		t.Fatalf("unexpected CreateAccessPolicyRequest: %+v", fake.lastCreatePolicyReq)
	}
}

func TestAdminRoutes_UpdatePolicy(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{updatePolicyResp: &authv1.AccessPolicy{Id: "p1", Version: 2}}
	router := testAdminRouter(fake)

	body, _ := json.Marshal(updatePolicyRequestBody{DocumentJSON: `{"rps":20}`, ExpectedVersion: 1})
	req := httptest.NewRequest(http.MethodPut, "/admin/api/policies/p1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if fake.lastUpdatePolicyReq.GetId() != "p1" || fake.lastUpdatePolicyReq.GetExpectedVersion() != 1 {
		t.Fatalf("unexpected UpdateAccessPolicyRequest: %+v", fake.lastUpdatePolicyReq)
	}
}

func TestAdminRoutes_UpdatePolicy_VersionConflictMapsToPreconditionFailed(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{err: status.Error(codes.FailedPrecondition, "policy was updated concurrently, refetch and retry")}
	router := testAdminRouter(fake)

	body, _ := json.Marshal(updatePolicyRequestBody{DocumentJSON: `{}`, ExpectedVersion: 1})
	req := httptest.NewRequest(http.MethodPut, "/admin/api/policies/p1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusPreconditionFailed, w.Body.String())
	}
}

func TestAdminRoutes_DeletePolicy(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/policies/p1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}
	if fake.lastDeletePolicyReq.GetId() != "p1" {
		t.Fatalf("DeleteAccessPolicy called with Id = %q, want %q", fake.lastDeletePolicyReq.GetId(), "p1")
	}
}

func TestAdminRoutes_Audit(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{}
	router := testAdminRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/audit", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestAdminRoutes_AuditMatchesV1AuthAuditLog is the contract-regression test
// TASK-006 calls out: /admin/api/audit and /v1/auth/audit-log must resolve
// to the same QueryAuditLog RPC and return byte-identical response shapes
// for the same fake client response, so the two REST surfaces can't drift
// apart from each other later.
func TestAdminRoutes_AuditMatchesV1AuthAuditLog(t *testing.T) {
	resp := &authv1.QueryAuditLogResponse{
		Entries:       []*authv1.AuditEntry{{Id: "e1", Action: "user.created"}},
		NextPageToken: "next",
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/admin/api/audit", nil)
	adminW := httptest.NewRecorder()
	testAdminRouter(&fakeAdminAuthServiceClient{queryAuditLogResp: resp}).ServeHTTP(adminW, adminReq)

	v1Req := httptest.NewRequest(http.MethodGet, "/v1/auth/audit-log", nil)
	v1W := httptest.NewRecorder()
	testAuthAdminRouter(&fakeAdminAuthServiceClient{queryAuditLogResp: resp}).ServeHTTP(v1W, v1Req)

	if adminW.Code != v1W.Code {
		t.Fatalf("/admin/api/audit status %d != /v1/auth/audit-log status %d", adminW.Code, v1W.Code)
	}
	if adminW.Body.String() != v1W.Body.String() {
		t.Fatalf("/admin/api/audit body %q != /v1/auth/audit-log body %q — the two REST surfaces have drifted apart", adminW.Body.String(), v1W.Body.String())
	}
}
