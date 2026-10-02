package domain

import "time"

// OAuthClientStatus is a client's standing inside one tenant.
type OAuthClientStatus string

const (
	OAuthClientAllowed OAuthClientStatus = "allowed"
	OAuthClientBlocked OAuthClientStatus = "blocked"
	// OAuthClientPending: registered by DCR in a tenant that has dynamic
	// registration off; an admin must allow it before any code is issued.
	OAuthClientPending OAuthClientStatus = "pending"
)

func (s OAuthClientStatus) Valid() bool {
	switch s {
	case OAuthClientAllowed, OAuthClientBlocked, OAuthClientPending:
		return true
	}
	return false
}

// OAuthClientTenantStatus is auth.oauth_client_tenant_status.
type OAuthClientTenantStatus struct {
	TenantID  string
	ClientID  string
	Status    OAuthClientStatus
	UpdatedBy string // empty = system
	UpdatedAt time.Time
}

// OAuthClientView joins a registry client with its status in one tenant.
type OAuthClientView struct {
	Client OAuthClient
	Status OAuthClientTenantStatus
}
