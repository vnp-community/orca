package domain

import (
	"errors"
	"strings"
)

var (
	// ErrEmptyEmailDomain is returned when a CompanyEmailDomain is
	// constructed with no domain.
	ErrEmptyEmailDomain = errors.New("domain: email_domain is required")
	// ErrInvalidEmailDomain is returned when the domain still contains "@"
	// or whitespace after normalization — a caller passed a full email
	// address (or something malformed), not a bare domain.
	ErrInvalidEmailDomain = errors.New("domain: email_domain must be a bare domain (no '@', no whitespace)")
)

// CompanyEmailDomain links one email domain (e.g. "vnpay.vn") to the
// company whose users authenticate with it — the multi-tenant SSO follow-up
// to CR-LOGIN-001: a brand-new SSO signup's tenant is resolved from this
// table (ResolveCompanyByEmailDomain), keyed by the domain half of their
// verified email, instead of requiring exactly one company to exist
// deployment-wide.
type CompanyEmailDomain struct {
	EmailDomain string
	CompanyID   string
}

// NormalizeEmailDomain lowercases and trims an email domain for consistent
// storage/lookup — "VnPay.VN" and "vnpay.vn " must resolve to the same row.
// Callers should normalize before both writes (Add) and reads
// (ResolveCompanyID) — see CompanyEmailDomainRepository's doc comment.
func NormalizeEmailDomain(emailDomain string) string {
	return strings.ToLower(strings.TrimSpace(emailDomain))
}

// NewCompanyEmailDomain constructs a CompanyEmailDomain, enforcing that the
// (already-normalized) domain is non-empty and doesn't itself look like a
// full email address. No CreatedAt field — this service doesn't model
// timestamps in its domain types (Company/Department don't either); the
// column exists in SQL only, defaulted by Postgres.
func NewCompanyEmailDomain(emailDomain, companyID string) (CompanyEmailDomain, error) {
	emailDomain = NormalizeEmailDomain(emailDomain)
	if emailDomain == "" {
		return CompanyEmailDomain{}, ErrEmptyEmailDomain
	}
	if strings.ContainsAny(emailDomain, "@ \t\n") {
		return CompanyEmailDomain{}, ErrInvalidEmailDomain
	}
	if companyID == "" {
		return CompanyEmailDomain{}, ErrEmptyID
	}
	return CompanyEmailDomain{EmailDomain: emailDomain, CompanyID: companyID}, nil
}

// EmailDomainFromAddress extracts the domain half of an email address
// (lowercased) — "" if addr has no "@". Shared by
// ResolveCompanyByEmailDomain's usecase and any future caller that needs
// the same extraction.
func EmailDomainFromAddress(addr string) string {
	i := strings.LastIndex(addr, "@")
	if i < 0 || i == len(addr)-1 {
		return ""
	}
	return NormalizeEmailDomain(addr[i+1:])
}
