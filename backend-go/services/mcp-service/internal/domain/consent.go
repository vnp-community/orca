package domain

import (
	"encoding/json"
	"net/url"
	"time"
)

const (
	DecisionApprove = "approve"
	DecisionDeny    = "deny"

	GrantActive  = "active"
	GrantRevoked = "revoked"

	MaxOAuthStateLength = 512

	RevokeReasonUser  = "user_revoked"
	RevokeReasonAdmin = "admin_revoked"
)

// ConsentRequest is a pending (or decided) user decision about whether a
// client may act for them, created when an OAuth client sends the user to
// /oauth/authorize.
type ConsentRequest struct {
	ID               string
	TenantID         string
	UserID           string
	ClientID         string
	ClientName       string
	ClientURI        string
	RedirectURI      string
	Scopes           []string // what the client asked for (already capped by the user's role)
	State            string
	CodeChallenge    string
	Resource         string
	IsNewClient      bool
	RegisteredViaDCR bool
	CreatedAt        time.Time
	ExpiresAt        time.Time
	DecidedAt        *time.Time
	Decision         string
}

// Grant is a user's standing consent for one client (confused-deputy guard:
// consent is per client and user, never shared between clients).
type Grant struct {
	ID                     string
	TenantID               string
	UserID                 string
	ClientID               string
	ClientName             string
	ClientURI              string
	Scopes                 []string
	Status                 string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	LastUsedAt             *time.Time
	RevokedAt              *time.Time
	RevokedBy              string
	RevocationPropagatedAt *time.Time
}

// RedirectHost returns only the host of a redirect URI for display, so the
// consent screen never prints a full URL (which could carry query tricks).
func RedirectHost(redirectURI string) string {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return ""
	}
	return u.Host
}

// BuildAuthorizeRedirect appends the authorization response parameters to the
// client's redirect_uri, preserving any query it already has. iss (RFC 9207)
// lets the client detect a mix-up attack; state is echoed verbatim.
func BuildAuthorizeRedirect(redirectURI string, params map[string]string, state, issuer string) (string, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	if state != "" {
		q.Set("state", state)
	}
	if issuer != "" {
		q.Set("iss", issuer)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Grant event subjects (outbox -> orca.mcp.>).
const (
	SubjectGrantCreated       = "orca.mcp.grant.created"
	SubjectGrantUpdated       = "orca.mcp.grant.updated"
	SubjectGrantRevoked       = "orca.mcp.grant.revoked"
	SubjectClientStatusChange = "orca.mcp.client.status_changed"
)

// NewOutboxEvent builds an outbox record whose payload carries the common
// envelope (event_id, tenant_id, occurred_at, schema_version) plus fields.
func NewOutboxEvent(eventID, subject, tenantID string, at time.Time, fields map[string]any) (OutboxRecord, error) {
	payload := map[string]any{
		"event_id": eventID, "tenant_id": tenantID, "occurred_at": at.UTC().Format(time.RFC3339Nano), "schema_version": 1,
	}
	for k, v := range fields {
		payload[k] = v
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return OutboxRecord{}, err
	}
	return OutboxRecord{ID: eventID, Subject: subject, OccurredAt: at, Version: 1, PayloadJSON: b}, nil
}
