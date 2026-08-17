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
