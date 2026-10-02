package domain

import (
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// Ranges that net/netip's classification helpers do not cover. Blocking by
// address class (not by a list of metadata hostnames) keeps cloud metadata
// endpoints (169.254.169.254 link-local, fd00:ec2::254 ULA, 100.100.100.200
// CGNAT) unreachable without naming each provider.
var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",       // "this network"
	"100.64.0.0/10",   // CGNAT, includes Alibaba metadata 100.100.100.200
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"240.0.0.0/4",     // reserved + broadcast
	"64:ff9b::/96",    // NAT64: embeds an IPv4 target
	"2002::/16",       // 6to4: embeds an IPv4 target
	"100::/64",        // discard-only
	"2001:db8::/32",   // documentation
)

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, netip.MustParsePrefix(c))
	}
	return out
}

// IsBlockedIP reports whether an outbound probe/connection to ip is forbidden.
// IPv4-mapped IPv6 is unmapped first so ::ffff:127.0.0.1 cannot bypass the
// check; NAT64/6to4 are blocked wholesale instead of unwrapped (no legitimate
// MCP server lives there).
func IsBlockedIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return true
	}
	ip = ip.Unmap()
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// ExternalURLPolicy is the operator-controlled part of URL validation.
type ExternalURLPolicy struct {
	// HTTPAllowlist lists host:port pairs that may use plain http (dev only).
	HTTPAllowlist []string
	// AllowedPorts defaults to {443}.
	AllowedPorts []int
	// AllowPrivate disables IP blocking for hosts in HTTPAllowlist only.
	// It is derived, never configured on its own.
}

func (p ExternalURLPolicy) ports() []int {
	if len(p.AllowedPorts) == 0 {
		return []int{443}
	}
	return p.AllowedPorts
}

// HTTPAllowed reports whether hostport is on the plain-http allow-list.
func (p ExternalURLPolicy) HTTPAllowed(hostport string) bool {
	for _, a := range p.HTTPAllowlist {
		if strings.EqualFold(a, hostport) {
			return true
		}
	}
	return false
}

// ValidateExternalURL checks the static shape of a server URL (no DNS). It
// returns ErrSSRFBlocked for scheme/port/IP-literal violations and
// ErrServerInvalid for userinfo or embedded credentials. The dial-time check
// in the prober is what actually stops DNS tricks.
func ValidateExternalURL(raw string, p ExternalURLPolicy) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Opaque != "" {
		return nil, ErrServerInvalid("url is not a valid absolute URL")
	}
	if u.User != nil {
		return nil, ErrServerInvalid("url must not contain credentials")
	}
	if u.Fragment != "" {
		return nil, ErrServerInvalid("url must not contain a fragment")
	}
	if _, red := (SecretRedactor{}).Redact(raw); red || sensitiveQuery(u) {
		return nil, ErrServerInvalid("url must not embed a secret; use a header reference")
	}
	host := u.Hostname()
	port := u.Port()
	scheme := strings.ToLower(u.Scheme)
	if port == "" {
		port = map[string]string{"https": "443", "http": "80"}[scheme]
	}
	hostport := net.JoinHostPort(strings.ToLower(host), port)
	switch scheme {
	case "https":
	case "http":
		if !p.HTTPAllowed(hostport) {
			return nil, ErrSSRFBlocked("url must use https")
		}
		return u, nil // allow-listed dev host: skips literal/port checks by design
	default:
		return nil, ErrSSRFBlocked("url must use https")
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return nil, ErrSSRFBlocked("url must use a host name, not an IP address")
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return nil, ErrServerInvalid("url has an invalid port")
	}
	for _, ok := range p.ports() {
		if ok == n {
			return u, nil
		}
	}
	return nil, ErrSSRFBlocked("url port is not allowed")
}

func sensitiveQuery(u *url.URL) bool {
	for k := range u.Query() {
		if sensitiveQueryKeys[strings.ToLower(k)] {
			return true
		}
	}
	return false
}
