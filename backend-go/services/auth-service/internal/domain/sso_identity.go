package domain

import (
	"errors"
	"time"
)

// SsoProvider is the closed set of identity providers CR-LOGIN-001 supports.
// "oidc" covers any generic/self-hosted OIDC provider (Keycloak included) —
// the frontend labels this one "keycloak", see api-gateway's auth_routes.go
// doc comment for that string-mapping boundary.
type SsoProvider string

const (
	SsoProviderGitHub SsoProvider = "github"
	SsoProviderGoogle SsoProvider = "google"
	SsoProviderOIDC   SsoProvider = "oidc"
)

func (p SsoProvider) Valid() bool {
	switch p {
	case SsoProviderGitHub, SsoProviderGoogle, SsoProviderOIDC:
		return true
	default:
		return false
	}
}

var (
	// ErrEmptyProvider is returned when an SsoIdentity is constructed
	// without a provider.
	ErrEmptyProvider = errors.New("domain: provider is required")
	// ErrInvalidProvider is returned when Provider isn't one of the known
	// enum values.
	ErrInvalidProvider = errors.New("domain: invalid sso provider")
	// ErrEmptySubject is returned when ExternalSubject is empty — an SSO
	// identity that isn't pinned to a stable IdP subject can't be looked
	// up again on the user's next login.
	ErrEmptySubject = errors.New("domain: external_subject is required")
)

// SsoIdentity links one IdP identity (provider + its stable subject) to
// exactly one local User — see auth.sso_identities' UNIQUE(provider,
// external_subject) constraint, which this type's invariants mirror.
type SsoIdentity struct {
	ID              string
	UserID          string
	TenantID        string
	Provider        SsoProvider
	ExternalSubject string
	// EmailAtLink is the email the IdP reported at link time — audit trail
	// only. LoginOrProvisionSsoUser never re-reads it for an auth decision
	// on a returning identity (see that usecase's doc comment): once
	// linked, (provider, external_subject) alone is the login key.
	EmailAtLink string
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

// NewSsoIdentity constructs an SsoIdentity, enforcing the same
// non-empty-ID/tenant invariants domain.NewUser/NewSession use, plus a
// valid provider and a non-empty external subject.
func NewSsoIdentity(id, userID, tenantID string, provider SsoProvider, externalSubject, emailAtLink string, createdAt time.Time) (SsoIdentity, error) {
	if id == "" {
		return SsoIdentity{}, ErrEmptyID
	}
	if userID == "" {
		return SsoIdentity{}, ErrEmptyUser
	}
	if tenantID == "" {
		return SsoIdentity{}, ErrEmptyTenant
	}
	if provider == "" {
		return SsoIdentity{}, ErrEmptyProvider
	}
	if !provider.Valid() {
		return SsoIdentity{}, ErrInvalidProvider
	}
	if externalSubject == "" {
		return SsoIdentity{}, ErrEmptySubject
	}
	return SsoIdentity{
		ID:              id,
		UserID:          userID,
		TenantID:        tenantID,
		Provider:        provider,
		ExternalSubject: externalSubject,
		EmailAtLink:     emailAtLink,
		CreatedAt:       createdAt,
	}, nil
}
