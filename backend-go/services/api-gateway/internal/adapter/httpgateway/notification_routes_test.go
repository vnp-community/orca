package httpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
)

// fakeNotificationServiceClient implements notificationv1.NotificationServiceClient
// for tests. StreamNotifications is not exercised by mountNotificationRoutes'
// handlers (it's served for real elsewhere, by a WS bridge) so it's
// implemented minimally here just to satisfy the generated interface.
type fakeNotificationServiceClient struct {
	subscribeReq  *notificationv1.SubscribeRequest
	subscribeResp *notificationv1.SubscribeResponse
	subscribeErr  error

	vapidResp *notificationv1.GetVapidPublicKeyResponse
	vapidErr  error

	unregisterReq *notificationv1.UnregisterPushSubscriptionRequest
	unregisterErr error

	listReq  *notificationv1.ListNotificationsRequest
	listResp *notificationv1.ListNotificationsResponse
	listErr  error

	markAsReadReq *notificationv1.MarkAsReadRequest
	markAsReadErr error

	markAllReq *notificationv1.MarkAllAsReadRequest
	markAllErr error

	unreadCountReq  *notificationv1.GetUnreadCountRequest
	unreadCountResp *notificationv1.GetUnreadCountResponse
	unreadCountErr  error
}

func (f *fakeNotificationServiceClient) Subscribe(_ context.Context, in *notificationv1.SubscribeRequest, _ ...grpc.CallOption) (*notificationv1.SubscribeResponse, error) {
	f.subscribeReq = in
	if f.subscribeErr != nil {
		return nil, f.subscribeErr
	}
	return f.subscribeResp, nil
}

func (f *fakeNotificationServiceClient) GetVapidPublicKey(_ context.Context, _ *notificationv1.GetVapidPublicKeyRequest, _ ...grpc.CallOption) (*notificationv1.GetVapidPublicKeyResponse, error) {
	if f.vapidErr != nil {
		return nil, f.vapidErr
	}
	return f.vapidResp, nil
}

func (f *fakeNotificationServiceClient) UnregisterPushSubscription(_ context.Context, in *notificationv1.UnregisterPushSubscriptionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.unregisterReq = in
	if f.unregisterErr != nil {
		return nil, f.unregisterErr
	}
	return &emptypb.Empty{}, nil
}

// GetUnreadCount/ListNotifications/MarkAsRead/MarkAllAsRead: TASK-BE-NOTIF-008's
// 4 authenticated routes — real configurable fakes (request capture +
// response/error), same pattern as Subscribe above.
func (f *fakeNotificationServiceClient) GetUnreadCount(_ context.Context, in *notificationv1.GetUnreadCountRequest, _ ...grpc.CallOption) (*notificationv1.GetUnreadCountResponse, error) {
	f.unreadCountReq = in
	if f.unreadCountErr != nil {
		return nil, f.unreadCountErr
	}
	return f.unreadCountResp, nil
}

func (f *fakeNotificationServiceClient) ListNotifications(_ context.Context, in *notificationv1.ListNotificationsRequest, _ ...grpc.CallOption) (*notificationv1.ListNotificationsResponse, error) {
	f.listReq = in
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResp, nil
}

func (f *fakeNotificationServiceClient) MarkAsRead(_ context.Context, in *notificationv1.MarkAsReadRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.markAsReadReq = in
	if f.markAsReadErr != nil {
		return nil, f.markAsReadErr
	}
	return &emptypb.Empty{}, nil
}

func (f *fakeNotificationServiceClient) MarkAllAsRead(_ context.Context, in *notificationv1.MarkAllAsReadRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.markAllReq = in
	if f.markAllErr != nil {
		return nil, f.markAllErr
	}
	return &emptypb.Empty{}, nil
}

func (f *fakeNotificationServiceClient) StreamNotifications(_ context.Context, _ *notificationv1.StreamNotificationsRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[notificationv1.NotificationServiceStreamNotificationsResponse], error) {
	return nil, status.Error(codes.Unimplemented, "StreamNotifications is served by the WS bridge, not this REST proxy")
}

// notificationTestRouter mounts mountNotificationRoutes on a bare chi router
// and injects the given identity into every request's context the way the
// real authMiddleware would (see middleware.go's withIdentity), so handlers
// under test read tenant/user the same way they do in production.
func notificationTestRouter(client notificationv1.NotificationServiceClient, identity usecase.Identity) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), identity)))
		})
	})
	mountNotificationRoutes(r, client)
	return r
}

func TestHandleSubscribe_Success(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		subscribeResp: &notificationv1.SubscribeResponse{SubscriptionId: "sub-123"},
	}
	identity := usecase.Identity{TenantID: "tenant-1", UserID: "user-1"}
	router := notificationTestRouter(fake, identity)

	body, err := json.Marshal(subscribeRequestBody{
		Endpoint:  "https://push.example.com/ep",
		P256dhKey: "p256dh-key",
		AuthKey:   "auth-key",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/subscribe", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// user_id must come from identity, never the (absent) request body.
	if fake.subscribeReq.GetUserId() != identity.UserID {
		t.Fatalf("Subscribe called with UserId = %q, want %q", fake.subscribeReq.GetUserId(), identity.UserID)
	}
	if fake.subscribeReq.GetEndpoint() != "https://push.example.com/ep" {
		t.Fatalf("Subscribe called with Endpoint = %q", fake.subscribeReq.GetEndpoint())
	}

	var resp notificationv1.SubscribeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if resp.SubscriptionId != "sub-123" {
		t.Fatalf("SubscriptionId = %q, want %q", resp.SubscriptionId, "sub-123")
	}
}

func TestHandleSubscribe_ChannelPassthrough(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		subscribeResp: &notificationv1.SubscribeResponse{SubscriptionId: "sub-123"},
	}
	router := notificationTestRouter(fake, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	body, err := json.Marshal(subscribeRequestBody{
		Endpoint: "ios-device-token", Channel: "ios", DeviceLabel: "iPhone 15",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/subscribe", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if fake.subscribeReq.GetChannel() != "ios" {
		t.Fatalf("Subscribe called with Channel = %q, want %q", fake.subscribeReq.GetChannel(), "ios")
	}
	if fake.subscribeReq.GetDeviceLabel() != "iPhone 15" {
		t.Fatalf("Subscribe called with DeviceLabel = %q, want %q", fake.subscribeReq.GetDeviceLabel(), "iPhone 15")
	}
}

// TestHandleSubscribe_EmptyChannelStillWorks is a regression guard — a
// request body with no "channel" field (every caller before this task)
// must still succeed exactly as before.
func TestHandleSubscribe_EmptyChannelStillWorks(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		subscribeResp: &notificationv1.SubscribeResponse{SubscriptionId: "sub-123"},
	}
	router := notificationTestRouter(fake, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	body, err := json.Marshal(subscribeRequestBody{
		Endpoint: "https://push.example.com/ep", P256dhKey: "p256dh-key", AuthKey: "auth-key",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/subscribe", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if fake.subscribeReq.GetChannel() != "" {
		t.Fatalf("expected empty Channel to pass through unchanged, got %q", fake.subscribeReq.GetChannel())
	}
}

func TestHandleGetVapidPublicKey_Success(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		vapidResp: &notificationv1.GetVapidPublicKeyResponse{PublicKey: "vapid-public-key"},
	}
	router := notificationTestRouter(fake, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications/vapid-public-key", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp notificationv1.GetVapidPublicKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if resp.PublicKey != "vapid-public-key" {
		t.Fatalf("PublicKey = %q, want %q", resp.PublicKey, "vapid-public-key")
	}
}

// pushTestRouter mounts mountPushRoutes standalone, WITHOUT injecting any
// identity into request context — these routes are unauthenticated by
// design (see mountPushRoutes's doc comment), so a test router for them
// must not simulate authMiddleware the way notificationTestRouter does for
// the authenticated /v1/notifications/* mount. cookieValidator is nil for
// the "genuinely no session" tests below; TestPushRoutes_SoftAuth_* passes
// a fake one to exercise resolveSoftIdentity's cookie-validation fallback.
func pushTestRouter(client notificationv1.NotificationServiceClient, cookieValidator CookieSessionValidator) http.Handler {
	r := chi.NewRouter()
	mountPushRoutes(r, client, cookieValidator)
	return r
}

// fakeCookieValidator is a minimal CookieSessionValidator test double.
type fakeCookieValidator struct {
	identity wscompat.Identity
	err      error
}

func (f *fakeCookieValidator) ValidateCookie(_ context.Context, _ *http.Request) (wscompat.Identity, error) {
	return f.identity, f.err
}

func TestHandlePushUnsubscribe_KnownEndpoint(t *testing.T) {
	fake := &fakeNotificationServiceClient{}
	router := pushTestRouter(fake, nil)

	body, err := json.Marshal(unsubscribeRequestBody{Endpoint: "https://push.example.com/ep"})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/push-unsubscribe", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if fake.unregisterReq.GetEndpoint() != "https://push.example.com/ep" {
		t.Fatalf("UnregisterPushSubscription called with Endpoint = %q", fake.unregisterReq.GetEndpoint())
	}
}

// TestHandlePushUnsubscribe_UnknownEndpoint_StillReturns204 guards
// idempotency at the REST boundary: re-unsubscribing an endpoint that was
// never registered (or already removed) must not surface as an error.
func TestHandlePushUnsubscribe_UnknownEndpoint_StillReturns204(t *testing.T) {
	fake := &fakeNotificationServiceClient{} // UnregisterPushSubscription succeeds unconditionally (no unregisterErr set)
	router := pushTestRouter(fake, nil)

	body, err := json.Marshal(unsubscribeRequestBody{Endpoint: "https://push.example.com/never-subscribed"})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/push-unsubscribe", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (idempotent, not an error); body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

// TestPushRoutes_NoAuthRequired is the route-placement regression test
// TASK-011 calls out explicitly: GET /api/vapid-public-key and POST
// /api/push-subscribe (and push-unsubscribe) must succeed with NO identity
// in request context — guards against these accidentally being remounted
// inside the authenticated group later (BUG-003).
func TestPushRoutes_NoAuthRequired(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		vapidResp:     &notificationv1.GetVapidPublicKeyResponse{PublicKey: "vapid-key"},
		subscribeResp: &notificationv1.SubscribeResponse{SubscriptionId: "sub-1"},
	}
	router := pushTestRouter(fake, nil)

	// GET /api/vapid-public-key — no identity in context at all (unlike
	// notificationTestRouter's tests, this router never injects one).
	req := httptest.NewRequest(http.MethodGet, "/api/vapid-public-key", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/vapid-public-key: status = %d, want %d (push routes must not require auth — regression against BUG-003); body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	subBody, _ := json.Marshal(subscribeRequestBody{Endpoint: "https://push.example.com/ep"})
	req = httptest.NewRequest(http.MethodPost, "/api/push-subscribe", bytes.NewReader(subBody))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/push-subscribe: status = %d, want %d (push routes must not require auth — regression against BUG-003); body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	unsubBody, _ := json.Marshal(unsubscribeRequestBody{Endpoint: "https://push.example.com/ep"})
	req = httptest.NewRequest(http.MethodPost, "/api/push-unsubscribe", bytes.NewReader(unsubBody))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("POST /api/push-unsubscribe: status = %d, want %d (push routes must not require auth — regression against BUG-003); body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}
}

// TestPushRoutes_SoftAuth_NoCookie_StillNoIdentity documents the no-cookie
// half of resolveSoftIdentity's fallback: with a cookieValidator configured
// but the request carrying no valid session, the caller still gets an empty
// identity (never a 401) — same behavior as nil cookieValidator, preserving
// BUG-003's guarantee for a truly anonymous caller.
func TestPushRoutes_SoftAuth_NoCookie_StillNoIdentity(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		vapidResp: &notificationv1.GetVapidPublicKeyResponse{PublicKey: "vapid-key"},
	}
	validator := &fakeCookieValidator{err: errors.New("no session cookie")}
	router := pushTestRouter(fake, validator)

	req := httptest.NewRequest(http.MethodGet, "/api/vapid-public-key", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestPushRoutes_SoftAuth_ValidCookie_ResolvesRealTenant is the live-bug
// regression: GetVapidPublicKey/Subscribe are tenant-scoped usecases
// (tenant.RequireTenantID), but the unauthenticated /api/push-* mount never
// gave them a tenant — NOTIFICATION_NO_TENANT fired even for a genuinely
// logged-in browser whose fetch() sends the session cookie same-origin.
// With a cookieValidator that resolves a real session, the identity must
// now reach the downstream Subscribe call.
func TestPushRoutes_SoftAuth_ValidCookie_ResolvesRealTenant(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		subscribeResp: &notificationv1.SubscribeResponse{SubscriptionId: "sub-1"},
	}
	validator := &fakeCookieValidator{
		identity: wscompat.Identity{TenantID: "tenant-1", UserID: "user-1", Role: "user"},
	}
	router := pushTestRouter(fake, validator)

	body, _ := json.Marshal(subscribeRequestBody{Endpoint: "https://push.example.com/ep"})
	req := httptest.NewRequest(http.MethodPost, "/api/push-subscribe", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	if fake.subscribeReq.GetUserId() != "user-1" {
		t.Fatalf("Subscribe called with UserId = %q, want %q (soft auth should have resolved the cookie's identity)", fake.subscribeReq.GetUserId(), "user-1")
	}
}

func TestHandleSubscribe_GRPCErrorMapsToHTTPStatus(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		subscribeErr: status.Error(codes.InvalidArgument, "endpoint is required"),
	}
	router := notificationTestRouter(fake, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	body, err := json.Marshal(subscribeRequestBody{})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/subscribe", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var respBody errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if respBody.Error.Code != codes.InvalidArgument.String() {
		t.Fatalf("error.code = %q, want %q", respBody.Error.Code, codes.InvalidArgument.String())
	}
}

// --- TASK-BE-NOTIF-008: 4 authenticated CR-NOTIF-001 routes ---

func TestNotificationRoutes_ListNotifications_RequiresAuth(t *testing.T) {
	fake := &fakeNotificationServiceClient{}
	router := notificationTestRouter(fake, usecase.Identity{}) // no identity resolved — mirrors an unauthenticated caller

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if fake.listReq != nil {
		t.Fatal("ListNotifications must not be called when there is no authenticated identity")
	}
}

func TestNotificationRoutes_ListNotifications_PassesQueryParamsThrough(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		listResp: &notificationv1.ListNotificationsResponse{NextCursor: "next"},
	}
	router := notificationTestRouter(fake, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications/?cursor=abc&limit=10&unread_only=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fake.listReq.GetUserId() != "user-1" {
		t.Fatalf("UserId = %q, want %q (must come from identity, not client input)", fake.listReq.GetUserId(), "user-1")
	}
	if fake.listReq.GetCursor() != "abc" || fake.listReq.GetLimit() != 10 || !fake.listReq.GetUnreadOnly() {
		t.Fatalf("query params not passed through correctly: %+v", fake.listReq)
	}
}

func TestNotificationRoutes_MarkAsRead_UsesIdentityUserIDNotClientInput(t *testing.T) {
	fake := &fakeNotificationServiceClient{}
	// The route body has no field for user_id at all (see handleMarkAsRead) —
	// this test documents that guarantee: whatever the caller is, the
	// request that reaches the gRPC client always carries identity.UserID.
	router := notificationTestRouter(fake, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/notif-1/read", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if fake.markAsReadReq.GetUserId() != "user-1" || fake.markAsReadReq.GetNotificationId() != "notif-1" {
		t.Fatalf("unexpected MarkAsRead request: %+v", fake.markAsReadReq)
	}
}

func TestNotificationRoutes_MarkAsRead_ReturnsNoContentOnSuccess(t *testing.T) {
	fake := &fakeNotificationServiceClient{}
	router := notificationTestRouter(fake, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/notif-1/read", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body for 204, got %q", rec.Body.String())
	}
}

func TestNotificationRoutes_MarkAsRead_RequiresAuth(t *testing.T) {
	fake := &fakeNotificationServiceClient{}
	router := notificationTestRouter(fake, usecase.Identity{})

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/notif-1/read", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if fake.markAsReadReq != nil {
		t.Fatal("MarkAsRead must not be called when there is no authenticated identity")
	}
}

func TestNotificationRoutes_MarkAllAsRead_RequiresAuth(t *testing.T) {
	fake := &fakeNotificationServiceClient{}
	router := notificationTestRouter(fake, usecase.Identity{})

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/read-all", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if fake.markAllReq != nil {
		t.Fatal("MarkAllAsRead must not be called when there is no authenticated identity")
	}
}

func TestNotificationRoutes_GetUnreadCount_RequiresAuth(t *testing.T) {
	fake := &fakeNotificationServiceClient{}
	router := notificationTestRouter(fake, usecase.Identity{})

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications/unread-count", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if fake.unreadCountReq != nil {
		t.Fatal("GetUnreadCount must not be called when there is no authenticated identity")
	}
}

func TestNotificationRoutes_GetUnreadCount_ReturnsCountFromUsecase(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		unreadCountResp: &notificationv1.GetUnreadCountResponse{Count: 9},
	}
	router := notificationTestRouter(fake, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications/unread-count", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fake.unreadCountReq.GetUserId() != "user-1" {
		t.Fatalf("UserId = %q, want %q", fake.unreadCountReq.GetUserId(), "user-1")
	}
	var resp notificationv1.GetUnreadCountResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if resp.GetCount() != 9 {
		t.Fatalf("count = %d, want 9", resp.GetCount())
	}
}

// TestNotificationRoutes_RouteOrderingNoConflict confirms the 4 new
// literal-path routes (/, /read-all, /unread-count — plus the existing
// /subscribe, /vapid-public-key) aren't swallowed by the /{id}/read
// path-param route registered alongside them, and vice versa.
func TestNotificationRoutes_RouteOrderingNoConflict(t *testing.T) {
	fake := &fakeNotificationServiceClient{
		unreadCountResp: &notificationv1.GetUnreadCountResponse{Count: 1},
		vapidResp:       &notificationv1.GetVapidPublicKeyResponse{PublicKey: "pk"},
	}
	router := notificationTestRouter(fake, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/v1/notifications/unread-count", nil))
	if rec1.Code != http.StatusOK || fake.unreadCountReq == nil {
		t.Fatalf("GET /unread-count did not reach handleGetUnreadCount: status=%d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/v1/notifications/vapid-public-key", nil))
	if rec2.Code != http.StatusOK || fake.vapidErr != nil {
		t.Fatalf("GET /vapid-public-key did not reach handleGetVapidPublicKey: status=%d", rec2.Code)
	}

	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, httptest.NewRequest(http.MethodPost, "/v1/notifications/notif-1/read", nil))
	if rec3.Code != http.StatusNoContent || fake.markAsReadReq.GetNotificationId() != "notif-1" {
		t.Fatalf("POST /{id}/read did not reach handleMarkAsRead correctly: status=%d", rec3.Code)
	}
}
