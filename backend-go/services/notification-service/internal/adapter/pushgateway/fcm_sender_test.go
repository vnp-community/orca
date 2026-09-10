package pushgateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// testFCMCredential builds a real (locally generated, never sent anywhere
// but our own test server) RSA service-account JSON — the exact shape
// golang.org/x/oauth2/google's service-account JWT flow expects — with
// token_uri pointed at tokenServerURL, so FCMSender.Send's real
// google.CredentialsFromJSON → TokenSource.Token() call hits OUR test
// server instead of Google's real oauth2.googleapis.com. This is the
// standard way to test x/oauth2/google's JWT flow without a real network
// call or a fake TokenSource.
func testFCMCredential(t *testing.T, projectID, tokenServerURL string) []byte {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	cred := map[string]string{
		"type":         "service_account",
		"project_id":   projectID,
		"private_key":  string(pemBytes),
		"client_email": "fcm-test@" + projectID + ".iam.gserviceaccount.com",
		"token_uri":    tokenServerURL,
	}
	blob, err := json.Marshal(cred)
	if err != nil {
		t.Fatalf("marshal credential: %v", err)
	}
	return blob
}

// fakeTokenServer returns an httptest.Server standing in for Google's
// OAuth2 token endpoint — accepts any JWT-bearer assertion POST (the
// signature was already produced correctly by x/oauth2/jwt using our own
// generated key; this fake doesn't need to re-verify it) and always
// returns a fixed access token.
func fakeTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fake-fcm-token","token_type":"Bearer","expires_in":3600}`))
	}))
}

func testAndroidSubscription(t *testing.T, endpoint string) domain.PushSubscription {
	t.Helper()
	sub, err := domain.NewPushSubscription("s1", "tenant-1", "user-1", domain.ChannelAndroid, endpoint, nil, nil, "", timeNow())
	if err != nil {
		t.Fatalf("building android subscription: %v", err)
	}
	return sub
}

func TestFCMSender_Send_Success200(t *testing.T) {
	tokenSrv := fakeTokenServer(t)
	defer tokenSrv.Close()
	blob := testFCMCredential(t, "orca-test-project", tokenSrv.URL)

	var gotAuth, gotPath string
	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("authorization")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer fcmSrv.Close()

	sender := NewFCMSender(&fakePushCredentialResolver{blob: blob}, fcmSrv.Client())
	sender.fcmBaseURL = fcmSrv.URL

	err := sender.Send(context.Background(), testAndroidSubscription(t, "device-token-1"), domain.NotificationEvent{Title: "t", Body: "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer fake-fcm-token" {
		t.Fatalf("authorization = %q, want %q", gotAuth, "Bearer fake-fcm-token")
	}
	wantPath := "/v1/projects/orca-test-project/messages:send"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
}

func TestFCMSender_Send_404ReturnsErrDeviceTokenInvalid(t *testing.T) {
	tokenSrv := fakeTokenServer(t)
	defer tokenSrv.Close()
	blob := testFCMCredential(t, "orca-test-project", tokenSrv.URL)

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer fcmSrv.Close()

	sender := NewFCMSender(&fakePushCredentialResolver{blob: blob}, fcmSrv.Client())
	sender.fcmBaseURL = fcmSrv.URL

	err := sender.Send(context.Background(), testAndroidSubscription(t, "device-token-1"), domain.NotificationEvent{})
	if !errors.Is(err, usecase.ErrDeviceTokenInvalid) {
		t.Fatalf("expected usecase.ErrDeviceTokenInvalid, got %v", err)
	}
}

func TestFCMSender_Send_OtherErrorStatusIsGenericError(t *testing.T) {
	tokenSrv := fakeTokenServer(t)
	defer tokenSrv.Close()
	blob := testFCMCredential(t, "orca-test-project", tokenSrv.URL)

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer fcmSrv.Close()

	sender := NewFCMSender(&fakePushCredentialResolver{blob: blob}, fcmSrv.Client())
	sender.fcmBaseURL = fcmSrv.URL

	err := sender.Send(context.Background(), testAndroidSubscription(t, "device-token-1"), domain.NotificationEvent{})
	if err == nil {
		t.Fatal("expected an error for a 503 response")
	}
	if errors.Is(err, usecase.ErrDeviceTokenInvalid) {
		t.Fatal("a 503 must NOT be classified as ErrDeviceTokenInvalid")
	}
}

func TestFCMSender_Send_CredentialResolveFailurePropagates(t *testing.T) {
	sender := NewFCMSender(&fakePushCredentialResolver{err: errors.New("broker unavailable")}, http.DefaultClient)
	err := sender.Send(context.Background(), testAndroidSubscription(t, "device-token-1"), domain.NotificationEvent{})
	if err == nil {
		t.Fatal("expected the credential resolve failure to propagate")
	}
}

func TestFCMSender_Send_MalformedServiceAccountJSONFails(t *testing.T) {
	sender := NewFCMSender(&fakePushCredentialResolver{blob: []byte("not valid json")}, http.DefaultClient)
	err := sender.Send(context.Background(), testAndroidSubscription(t, "device-token-1"), domain.NotificationEvent{})
	if err == nil {
		t.Fatal("expected an error for a malformed service account credential")
	}
	if errors.Is(err, usecase.ErrDeviceTokenInvalid) {
		t.Fatal("a config error must NOT be classified as ErrDeviceTokenInvalid — it isn't a stale token, it's a broken credential")
	}
}
