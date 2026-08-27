package domain

import (
	"testing"
	"time"
)

func strPtr(s string) *string { return &s }

func TestNewPushSubscription_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		tenantID string
		userID   string
		channel  Channel
		endpoint string
		p256dh   *string
		auth     *string
		wantErr  error
	}{
		{"valid web", "t1", "u1", ChannelWeb, "https://push.example/ep", strPtr("p256dh"), strPtr("auth"), nil},
		{"valid ios", "t1", "u1", ChannelIOS, "device-token", nil, nil, nil},
		{"empty tenant", "", "u1", ChannelWeb, "ep", strPtr("p"), strPtr("a"), ErrEmptyTenant},
		{"empty user", "t1", "", ChannelWeb, "ep", strPtr("p"), strPtr("a"), ErrEmptyTenant},
		{"invalid channel", "t1", "u1", Channel("bogus"), "ep", strPtr("p"), strPtr("a"), ErrInvalidChannel},
		{"empty endpoint", "t1", "u1", ChannelWeb, "", strPtr("p"), strPtr("a"), ErrEmptyEndpoint},
		{"missing web keys", "t1", "u1", ChannelWeb, "ep", nil, nil, ErrMissingWebKeys},
		{"missing auth key only", "t1", "u1", ChannelWeb, "ep", strPtr("p"), nil, ErrMissingWebKeys},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, err := NewPushSubscription("sub-1", tt.tenantID, tt.userID, tt.channel, tt.endpoint, tt.p256dh, tt.auth, "", now)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if sub.Status != SubscriptionActive {
					t.Errorf("expected new subscription to be active, got %s", sub.Status)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
