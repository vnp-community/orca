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
// the authenticated /v1/notifications/* mount.
func pushTestRouter(client notificationv1.NotificationServiceClient) http.Handler {
	r := chi.NewRouter()
	mountPushRoutes(r, client)
	return r
}

func TestHandlePushUnsubscribe_KnownEndpoint(t *testing.T) {
	fake := &fakeNotificationServiceClient{}
	router := pushTestRouter(fake)

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
	router := pushTestRouter(fake)

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
	router := pushTestRouter(fake)

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
