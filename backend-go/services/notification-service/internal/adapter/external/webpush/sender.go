// Package webpush implements usecase.WebPushSender: RFC 8291 message
// encryption + RFC 8292 VAPID auth header, POSTed to a Web Push endpoint.
// VAPID JWT signing is delegated to usecase.VaultSigner (Vault Transit) —
// this package never holds a VAPID private key, per
// notification-service.md §9.
package webpush

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// Sender implements usecase.WebPushSender.
type Sender struct {
	http *http.Client
}

func New(httpClient *http.Client) *Sender {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Sender{http: httpClient}
}

// Send encrypts payload (RFC 8291) for sub and POSTs it to sub.Endpoint
// with vapidAuthHeader as the Authorization header — vapidAuthHeader is
// built by the caller (usecase.DeliverPush, TASK-BE-NOTIF-011) via
// BuildVapidAuthHeader, because that step needs tenantID to call
// VaultSigner, which this type deliberately doesn't hold (see this file's
// package doc comment).
func (s *Sender) Send(ctx context.Context, sub domain.PushSubscription, vapidAuthHeader string, payload []byte) (expired bool, err error) {
	if sub.P256dhKey == nil || sub.AuthKey == nil {
		// domain.NewPushSubscription enforces these are non-nil for
		// Channel == web (ErrMissingWebKeys) — a nil pointer here would
		// mean this Sender was called for a non-web subscription, a
		// caller bug, not a transient send failure.
		return false, fmt.Errorf("webpush: subscription %s has no p256dh/auth key (not a web subscription?)", sub.ID)
	}

	encrypted, err := encrypt(payload, *sub.P256dhKey, *sub.AuthKey)
	if err != nil {
		return false, fmt.Errorf("webpush: encrypt payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(encrypted))
	if err != nil {
		return false, fmt.Errorf("webpush: build request: %w", err)
	}
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", "86400")
	req.Header.Set("Authorization", vapidAuthHeader)

	resp, err := s.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("webpush: post to endpoint: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return false, nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		// The push service says this endpoint no longer exists — the
		// caller (usecase.DeliverPush) must not retry it and should call
		// SubscriptionRepository.MarkExpired (TASK-BE-NOTIF-009).
		return true, nil
	default:
		// Any other non-2xx (429, 5xx, ...) is a transient failure, not
		// "this subscription is dead" — must not be marked expired.
		return false, fmt.Errorf("webpush: push service responded %d", resp.StatusCode)
	}
}
