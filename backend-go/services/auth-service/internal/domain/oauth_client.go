package domain

import (
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	OAuthRegisteredViaDCR   = "dcr"
	OAuthRegisteredViaAdmin = "admin"

	MaxRedirectURILength  = 2048
	MaxRedirectURIsPerApp = 5
	MaxClientNameRunes    = 100
)

var (
	ErrOAuthInvalidRedirectURI  = errors.New("domain: invalid redirect_uri")
	ErrOAuthInvalidClientName   = errors.New("domain: invalid client_name")
	ErrOAuthInvalidClientURI    = errors.New("domain: invalid client_uri")
	ErrOAuthTooManyRedirectURIs = errors.New("domain: between 1 and 5 redirect_uris are required")
)

// OAuthClient is a deployment-wide registry entry (auth.oauth_clients).
// It deliberately has no tenant: dynamic registration happens before any
// tenant is known, and the row holds only app identity. Per-tenant
// allow/block state lives in OAuthClientTenantStatus.
type OAuthClient struct {
	ClientID      string
	ClientName    string
	ClientURI     string
	RedirectURIs  []string
	RegisteredVia string
	CreatedAt     time.Time
	LastUsedAt    *time.Time
}

func isLoopbackHost(host string) bool {
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return false
}

// ValidateRedirectURI enforces the registration-time rules: https, or http
// only for a loopback host (any port, RFC 8252); no fragment, userinfo,
// wildcard, backslash, whitespace/control chars or ".." path segment; at
// most 2048 bytes.
func ValidateRedirectURI(raw string) error {
	if raw == "" || len(raw) > MaxRedirectURILength || strings.ContainsRune(raw, '*') || strings.ContainsRune(raw, '\\') {
		return ErrOAuthInvalidRedirectURI
	}
	for _, r := range raw {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return ErrOAuthInvalidRedirectURI
		}
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || u.Opaque != "" {
		return ErrOAuthInvalidRedirectURI
	}
	if u.User != nil || u.Fragment != "" || strings.Contains(raw, "#") {
		return ErrOAuthInvalidRedirectURI
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopbackHost(u.Hostname()) {
			return ErrOAuthInvalidRedirectURI
		}
	default:
		return ErrOAuthInvalidRedirectURI
	}
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == ".." {
			return ErrOAuthInvalidRedirectURI
		}
	}
	for _, seg := range strings.Split(u.EscapedPath(), "/") {
		if l := strings.ToLower(seg); l == "%2e%2e" || l == ".%2e" || l == "%2e." {
			return ErrOAuthInvalidRedirectURI
		}
	}
	return nil
}

// ValidateRedirectURIs validates the whole registration list.
func ValidateRedirectURIs(uris []string) error {
	if len(uris) < 1 || len(uris) > MaxRedirectURIsPerApp {
		return ErrOAuthTooManyRedirectURIs
	}
	seen := make(map[string]bool, len(uris))
	for _, u := range uris {
		if err := ValidateRedirectURI(u); err != nil {
			return err
		}
		if seen[u] {
			return ErrOAuthInvalidRedirectURI
		}
		seen[u] = true
	}
	return nil
}

// MatchRedirectURI compares a presented redirect_uri to the registered list
// by exact string equality. The one exception is RFC 8252 section 7.3: for a
// registered http loopback URI the port may differ; scheme, host, path and
// query still have to match exactly. The presented value must itself be valid.
func MatchRedirectURI(registered []string, presented string) bool {
	if ValidateRedirectURI(presented) != nil {
		return false
	}
	p, err := url.Parse(presented)
	if err != nil {
		return false
	}
	for _, reg := range registered {
		if reg == presented {
			return true
		}
		r, err := url.Parse(reg)
		if err != nil || r.Scheme != "http" || p.Scheme != "http" {
			continue
		}
		if !isLoopbackHost(r.Hostname()) || r.Hostname() != p.Hostname() {
			continue
		}
		if r.EscapedPath() == p.EscapedPath() && r.RawQuery == p.RawQuery {
			return true
		}
	}
	return false
}

// SanitizeClientName strips control and invisible format characters
// (including bidi overrides), collapses whitespace, and rejects names that
// are empty, too long, or contain a URL (a phishing vector on the consent
// screen).
func SanitizeClientName(raw string) (string, error) {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsControl(r) && !unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
	}
	name := strings.Join(strings.Fields(b.String()), " ")
	if name == "" || utf8.RuneCountInString(name) > MaxClientNameRunes {
		return "", ErrOAuthInvalidClientName
	}
	if strings.Contains(name, "://") {
		return "", ErrOAuthInvalidClientName
	}
	return name, nil
}

// ValidateClientURI checks the optional client_uri (informational link).
func ValidateClientURI(raw string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > MaxRedirectURILength {
		return ErrOAuthInvalidClientURI
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return ErrOAuthInvalidClientURI
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname())) {
		return ErrOAuthInvalidClientURI
	}
	return nil
}
