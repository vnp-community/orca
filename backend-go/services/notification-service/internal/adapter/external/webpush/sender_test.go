package webpush

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func testSubscription(t *testing.T, endpoint string) domain.PushSubscription {
	t.Helper()
	_, p256dhKeyB64, authKeyB64 := testSubscriberKeys(t)
	sub, err := domain.NewPushSubscription("sub-1", "tenant-1", "user-1", domain.ChannelWeb, endpoint, &p256dhKeyB64, &authKeyB64, "", time.Now())
	if err != nil {
		t.Fatalf("building subscription: %v", err)
	}
	return sub
}

func TestSender_Send_2xxReturnsNotExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "aes128gcm" {
			t.Errorf("expected Content-Encoding: aes128gcm, got %q", r.Header.Get("Content-Encoding"))
		}
		if r.Header.Get("Authorization") != "vapid t=abc, k=xyz" {
			t.Errorf("Authorization header not forwarded correctly: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	sender := New(nil)
	expired, err := sender.Send(context.Background(), testSubscription(t, srv.URL), "vapid t=abc, k=xyz", []byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Fatal("expected expired=false for a 2xx response")
	}
}

func TestSender_Send_410ReturnsExpiredTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	sender := New(nil)
	expired, err := sender.Send(context.Background(), testSubscription(t, srv.URL), "vapid t=abc, k=xyz", []byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error for a 410 response (this is an expected outcome, not a transport failure): %v", err)
	}
	if !expired {
		t.Fatal("expected expired=true for a 410 response")
	}
}

func TestSender_Send_404ReturnsExpiredTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	sender := New(nil)
	expired, err := sender.Send(context.Background(), testSubscription(t, srv.URL), "vapid t=abc, k=xyz", []byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error for a 404 response: %v", err)
	}
	if !expired {
		t.Fatal("expected expired=true for a 404 response")
	}
}

func TestSender_Send_5xxReturnsError_NotExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	sender := New(nil)
	expired, err := sender.Send(context.Background(), testSubscription(t, srv.URL), "vapid t=abc, k=xyz", []byte("hello"))
	if err == nil {
		t.Fatal("expected an error for a 5xx response")
	}
	if expired {
		t.Fatal("expected expired=false for a 5xx (transient) response — must not be marked expired")
	}
}

func TestSender_Send_NilWebKeys_ReturnsError(t *testing.T) {
	sub := domain.PushSubscription{ID: "sub-1", Channel: domain.ChannelWeb, Endpoint: "https://push.example.com"}
	sender := New(nil)
	if _, err := sender.Send(context.Background(), sub, "vapid t=abc, k=xyz", []byte("hello")); err == nil {
		t.Fatal("expected an error when p256dh/auth keys are nil")
	}
}
