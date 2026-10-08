package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// WebhookNonceTTL is how long a signature stays remembered; it must exceed the accepted clock skew (5 minutes)
// so a signature that passes the time check can never be replayed after its nonce expired.
const WebhookNonceTTL = 10 * time.Minute

// WebhookReplayGuard rejects a webhook whose signature was already accepted. The signature itself is the nonce:
// the same body and timestamp always sign the same.
type WebhookReplayGuard struct {
	store WebhookNonceStore
	now   func() time.Time
}

func NewWebhookReplayGuard(store WebhookNonceStore, now func() time.Time) *WebhookReplayGuard {
	if now == nil {
		now = time.Now
	}
	return &WebhookReplayGuard{store: store, now: now}
}

// FirstSeen reports whether this is the first time the signature is presented for (tenant, source).
func (g *WebhookReplayGuard) FirstSeen(ctx context.Context, tenantID, source, signatureHeader string) (bool, error) {
	sig := strings.TrimPrefix(strings.TrimSpace(signatureHeader), "sha256=")
	sum := sha256.Sum256([]byte(sig))
	return g.store.Remember(ctx, tenantID, source, hex.EncodeToString(sum[:]), g.now().Add(WebhookNonceTTL))
}
