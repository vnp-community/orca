package apiclient

import (
	"context"
	"encoding/json"
	"time"
)

type LoginResult struct {
	JWT       string
	ExpiresAt string // RFC 3339; empty when the gateway sent none
}

// Login calls POST /auth/cli-token.
func Login(ctx context.Context, c *Client, email, password string) (LoginResult, error) {
	var resp struct {
		JWT       string          `json:"jwt"`
		ExpiresAt json.RawMessage `json:"expires_at"`
	}
	if err := c.do(ctx, "POST", "/auth/cli-token", map[string]string{
		"email": email, "password": password,
	}, &resp); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{JWT: resp.JWT, ExpiresAt: normalizeExpiry(resp.ExpiresAt)}, nil
}

// normalizeExpiry reads the expiry in either shape the gateway has used: an RFC 3339 / HTTP-date string, or the
// protobuf Timestamp object {"seconds": ..., "nanos": ...} that encoding/json writes for a *timestamppb.Timestamp.
func normalizeExpiry(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		for _, layout := range []string{time.RFC3339, time.RFC1123} {
			if t, err := time.Parse(layout, s); err == nil {
				return t.UTC().Format(time.RFC3339)
			}
		}
		return ""
	}
	var ts struct {
		Seconds int64 `json:"seconds"`
	}
	if json.Unmarshal(raw, &ts) == nil && ts.Seconds > 0 {
		return time.Unix(ts.Seconds, 0).UTC().Format(time.RFC3339)
	}
	return ""
}
