package httpgateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakePairingAuthServiceClient implements authv1.AuthServiceClient with a
// single configurable canned response/error, mirroring
// fakeAdminAuthServiceClient's shape — the non-pairing methods are unused
// pass-throughs, present only because Go requires every interface method to
// compile.
type fakePairingAuthServiceClient struct {
	initiateResp *authv1.InitiateDevicePairingResponse
	completeResp *authv1.CompleteDevicePairingResponse
	listResp     *authv1.ListPairedDevicesResponse
	err          error

	completeCalled  bool
	lastCompleteReq *authv1.CompleteDevicePairingRequest
	lastUnpairReq   *authv1.UnpairDeviceRequest
}

func (f *fakePairingAuthServiceClient) Login(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.LoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) Logout(ctx context.Context, in *authv1.LogoutRequest, opts ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) ValidateSession(ctx context.Context, in *authv1.ValidateSessionRequest, opts ...grpc.CallOption) (*authv1.ValidateSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) IssueServiceToken(ctx context.Context, in *authv1.IssueServiceTokenRequest, opts ...grpc.CallOption) (*authv1.IssueServiceTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) GetJWKS(ctx context.Context, in *authv1.GetJWKSRequest, opts ...grpc.CallOption) (*authv1.GetJWKSResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) CreateUser(ctx context.Context, in *authv1.CreateUserRequest, opts ...grpc.CallOption) (*authv1.CreateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) UpdateUserRole(ctx context.Context, in *authv1.UpdateUserRoleRequest, opts ...grpc.CallOption) (*authv1.UpdateUserRoleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) RevokeSession(ctx context.Context, in *authv1.RevokeSessionRequest, opts ...grpc.CallOption) (*authv1.RevokeSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) QueryAuditLog(ctx context.Context, in *authv1.QueryAuditLogRequest, opts ...grpc.CallOption) (*authv1.QueryAuditLogResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) DeactivateUser(ctx context.Context, in *authv1.DeactivateUserRequest, opts ...grpc.CallOption) (*authv1.DeactivateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) ReactivateUser(ctx context.Context, in *authv1.ReactivateUserRequest, opts ...grpc.CallOption) (*authv1.ReactivateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) ListSessionsForUser(ctx context.Context, in *authv1.ListSessionsForUserRequest, opts ...grpc.CallOption) (*authv1.ListSessionsForUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) ForceRevokeAllSessionsForUser(ctx context.Context, in *authv1.ForceRevokeAllSessionsForUserRequest, opts ...grpc.CallOption) (*authv1.ForceRevokeAllSessionsForUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) CreateAccessPolicy(ctx context.Context, in *authv1.CreateAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) GetAccessPolicy(ctx context.Context, in *authv1.GetAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) ListAccessPolicies(ctx context.Context, in *authv1.ListAccessPoliciesRequest, opts ...grpc.CallOption) (*authv1.ListAccessPoliciesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) UpdateAccessPolicy(ctx context.Context, in *authv1.UpdateAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) DeleteAccessPolicy(ctx context.Context, in *authv1.DeleteAccessPolicyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) GetAdminStats(ctx context.Context, in *authv1.GetAdminStatsRequest, opts ...grpc.CallOption) (*authv1.GetAdminStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakePairingAuthServiceClient) InitiateDevicePairing(ctx context.Context, in *authv1.InitiateDevicePairingRequest, opts ...grpc.CallOption) (*authv1.InitiateDevicePairingResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.initiateResp, nil
}

func (f *fakePairingAuthServiceClient) CompleteDevicePairing(ctx context.Context, in *authv1.CompleteDevicePairingRequest, opts ...grpc.CallOption) (*authv1.CompleteDevicePairingResponse, error) {
	f.completeCalled = true
	f.lastCompleteReq = in
	if f.err != nil {
		return nil, f.err
	}
	return f.completeResp, nil
}

func (f *fakePairingAuthServiceClient) ListPairedDevices(ctx context.Context, in *authv1.ListPairedDevicesRequest, opts ...grpc.CallOption) (*authv1.ListPairedDevicesResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.listResp, nil
}

func (f *fakePairingAuthServiceClient) UnpairDevice(ctx context.Context, in *authv1.UnpairDeviceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	f.lastUnpairReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &emptypb.Empty{}, nil
}

func (f *fakePairingAuthServiceClient) ResolveDeviceSharedSecret(ctx context.Context, in *authv1.ResolveDeviceSharedSecretRequest, opts ...grpc.CallOption) (*authv1.ResolveDeviceSharedSecretResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

var _ authv1.AuthServiceClient = (*fakePairingAuthServiceClient)(nil)

// pairingTestRouter mounts BOTH the authenticated pairing routes and the
// unauthenticated /complete route the way NewRouter does — the authed
// routes behind a fake identity-injecting middleware (mirroring
// testAuthAdminRouter), CompleteDevicePairing outside it, unauthenticated,
// so this test suite can exercise the real route-placement split.
func pairingTestRouter(client authv1.AuthServiceClient) http.Handler {
	r := chi.NewRouter()
	mountUnauthenticatedPairingRoutes(r, client, func(next http.Handler) http.Handler { return next })
	r.Group(func(authed chi.Router) {
		authed.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := withIdentity(r.Context(), usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		mountPairingRoutes(authed, client)
	})
	return r
}

// TestCompleteDevicePairing_NoAuthRequired mirrors
// TestPushRoutes_NoAuthRequired (notification_routes_test.go) — a request
// with no Authorization header/session cookie must reach the handler, not
// authMiddleware's 401 (this router never injects an identity at all).
func TestCompleteDevicePairing_NoAuthRequired(t *testing.T) {
	fake := &fakePairingAuthServiceClient{
		completeResp: &authv1.CompleteDevicePairingResponse{DeviceId: "device-1"},
	}
	router := pairingTestRouter(fake)

	body, err := json.Marshal(completeDevicePairingRequestBody{
		MobilePublicKeyB64: base64.StdEncoding.EncodeToString([]byte("mobile-pubkey")),
		DeviceLabel:        "My Phone",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/paired-devices/pairing-sessions/some-token/complete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (pairing complete must not require auth); body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if !fake.completeCalled {
		t.Fatal("CompleteDevicePairing was not called")
	}
	if fake.lastCompleteReq.GetPairingToken() != "some-token" {
		t.Fatalf("PairingToken = %q, want %q", fake.lastCompleteReq.GetPairingToken(), "some-token")
	}
}

// TestCompleteDevicePairing_ErrorsShareIdenticalShape asserts an invalid,
// expired, and already-used token all produce the identical HTTP status +
// error body shape — the unauthenticated route must not leak which failure
// mode occurred (pairing_routes.go's doc comment).
func TestCompleteDevicePairing_ErrorsShareIdenticalShape(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"invalid token", status.Error(codes.NotFound, "pairing session not found")},
		{"expired token", status.Error(codes.NotFound, "pairing session not found")},
		{"already-used token", status.Error(codes.NotFound, "pairing session not found")},
	}

	var statuses []int
	var bodies []errorBody
	for _, tc := range cases {
		fake := &fakePairingAuthServiceClient{err: tc.err}
		router := pairingTestRouter(fake)

		reqBody, err := json.Marshal(completeDevicePairingRequestBody{
			MobilePublicKeyB64: base64.StdEncoding.EncodeToString([]byte("k")),
		})
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/paired-devices/pairing-sessions/tok/complete", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var body errorBody
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: response body is not the expected JSON shape: %v; body=%s", tc.name, err, w.Body.String())
		}
		statuses = append(statuses, w.Code)
		bodies = append(bodies, body)
	}

	for i := 1; i < len(statuses); i++ {
		if statuses[i] != statuses[0] {
			t.Fatalf("%s: status = %d, want %d (same shape as %q)", cases[i].name, statuses[i], statuses[0], cases[0].name)
		}
		if bodies[i].Error.Code != bodies[0].Error.Code {
			t.Fatalf("%s: error.code = %q, want %q (same shape as %q)", cases[i].name, bodies[i].Error.Code, bodies[0].Error.Code, cases[0].name)
		}
	}
}

func TestHandleInitiateDevicePairing_Success(t *testing.T) {
	fake := &fakePairingAuthServiceClient{
		initiateResp: &authv1.InitiateDevicePairingResponse{PairingToken: "tok-1"},
	}
	router := pairingTestRouter(fake)

	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/paired-devices/pairing-sessions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestHandleListPairedDevices_Success(t *testing.T) {
	fake := &fakePairingAuthServiceClient{
		listResp: &authv1.ListPairedDevicesResponse{Devices: []*authv1.PairedDevice{{Id: "device-1"}}},
	}
	router := pairingTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/paired-devices/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandleUnpairDevice_Success(t *testing.T) {
	fake := &fakePairingAuthServiceClient{}
	router := pairingTestRouter(fake)

	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/paired-devices/device-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}
	if fake.lastUnpairReq.GetDeviceId() != "device-1" {
		t.Fatalf("DeviceId = %q, want %q", fake.lastUnpairReq.GetDeviceId(), "device-1")
	}
}
