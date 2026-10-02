// Package originpolicy decides whether a browser-initiated WebSocket upgrade
// (or other cookie-authenticated cross-origin request) comes from an origin the
// operator trusts.
//
// Why it exists: the edge authenticates browsers with an ambient session
// cookie, so without an Origin check any web page the user visits could open
// /ws and act as them (cross-site WebSocket hijacking). SameSite=Strict
// narrows this but is not a substitute for checking Origin.
package originpolicy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type pattern struct {
	scheme string
	// host is lower-case, may include a port. A leading "*." matches any
	// non-empty subdomain prefix (one or more labels), never the bare domain.
	host     string
	wildcard bool
}

// Policy is an allow-list of origins. The zero value (and a nil *Policy) is
// "not enforced": every origin is accepted, preserving behavior for
// deployments that have not configured an allow-list yet.
type Policy struct {
	patterns []pattern
}

// Parse builds a Policy from a comma-separated list of origins such as
// "https://orca.example.com,https://*.example.com,http://localhost:5173".
// An empty list returns a non-enforcing Policy.
func Parse(csv string) (*Policy, error) {
	p := &Policy{}
	for _, raw := range strings.Split(csv, ",") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		u, err := url.Parse(entry)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Path != "" && u.Path != "/") {
			return nil, fmt.Errorf("originpolicy: %q is not a valid origin (want scheme://host[:port])", entry)
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme != "http" && scheme != "https" {
			return nil, fmt.Errorf("originpolicy: %q has unsupported scheme %q", entry, scheme)
		}
		host := strings.ToLower(u.Host)
		wild := strings.HasPrefix(host, "*.")
		if wild {
			host = strings.TrimPrefix(host, "*.")
		}
		if strings.Contains(host, "*") || host == "" {
			return nil, fmt.Errorf("originpolicy: %q: wildcard is only allowed as a leading \"*.\"", entry)
		}
		p.patterns = append(p.patterns, pattern{scheme: scheme, host: host, wildcard: wild})
	}
	return p, nil
}

// Enforced reports whether the policy rejects anything. False means legacy
// permissive behavior.
func (p *Policy) Enforced() bool {
	return p != nil && len(p.patterns) > 0
}

// Allow reports whether r may proceed. Requests with no Origin header are
// allowed: browsers always send Origin on WebSocket upgrades, so its absence
// means a non-browser client (CLI, native mobile) that cannot be driven by a
// malicious web page and still has to authenticate.
func (p *Policy) Allow(r *http.Request) bool {
	if !p.Enforced() {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Host)
	// Same-origin is always fine and keeps single-host deployments working
	// without listing themselves.
	if strings.EqualFold(host, r.Host) && sameSchemeAsRequest(r, scheme) {
		return true
	}
	for _, pt := range p.patterns {
		if pt.scheme != scheme {
			continue
		}
		if !pt.wildcard && host == pt.host {
			return true
		}
		if pt.wildcard && strings.HasSuffix(host, "."+pt.host) && len(host) > len(pt.host)+1 {
			return true
		}
	}
	return false
}

func sameSchemeAsRequest(r *http.Request, originScheme string) bool {
	reqScheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		reqScheme = "https"
	}
	return reqScheme == originScheme
}
