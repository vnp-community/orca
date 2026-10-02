package domain

import (
	"errors"
	"strings"
)

// OAuth/MCP scope identifiers (CONTRACT McpScopeId). The ceiling rule is the
// same one BE-MCP-SOL-006 specifies for common/mcpscope; kept local until
// that package exists so this service has no cross-module dependency on
// unmerged work.
const (
	OAuthScopeRead  = "orca:read"
	OAuthScopeWrite = "orca:write"
	OAuthScopeExec  = "orca:exec"
	OAuthScopeAdmin = "orca:admin"
)

// ErrOAuthUnknownScope is returned for a scope outside the catalog.
var ErrOAuthUnknownScope = errors.New("domain: unknown oauth scope")

// OAuthAllScopes returns the catalog in a stable order.
func OAuthAllScopes() []string {
	return []string{OAuthScopeRead, OAuthScopeWrite, OAuthScopeExec, OAuthScopeAdmin}
}

// ParseOAuthScopes splits an RFC 6749 space-delimited scope string,
// de-duplicates and orders it by catalog order. An unknown scope is an error
// (never silently dropped) so a client cannot believe it holds a scope it
// does not. Empty input yields an empty slice.
func ParseOAuthScopes(space string) ([]string, error) {
	return NormalizeOAuthScopes(strings.Fields(space))
}

// NormalizeOAuthScopes de-duplicates and catalog-orders scopes.
func NormalizeOAuthScopes(scopes []string) ([]string, error) {
	seen := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		if !isKnownOAuthScope(s) {
			return nil, ErrOAuthUnknownScope
		}
		seen[s] = true
	}
	out := make([]string, 0, len(seen))
	for _, s := range OAuthAllScopes() {
		if seen[s] {
			out = append(out, s)
		}
	}
	return out, nil
}

func isKnownOAuthScope(s string) bool {
	for _, k := range OAuthAllScopes() {
		if k == s {
			return true
		}
	}
	return false
}

// FormatOAuthScopes joins scopes with single spaces.
func FormatOAuthScopes(scopes []string) string { return strings.Join(scopes, " ") }

// OAuthScopeCeilingForRole is the most a role may ever be granted. An
// unknown or empty role gets nothing (fail closed).
func OAuthScopeCeilingForRole(role Role) []string {
	switch role {
	case RoleAdmin:
		return OAuthAllScopes()
	case RoleUser:
		return []string{OAuthScopeRead, OAuthScopeWrite, OAuthScopeExec}
	default:
		return nil
	}
}

// IsScopeSubset reports whether every element of sub is in super.
func IsScopeSubset(sub, super []string) bool {
	have := make(map[string]bool, len(super))
	for _, s := range super {
		have[s] = true
	}
	for _, s := range sub {
		if !have[s] {
			return false
		}
	}
	return true
}

// IntersectOAuthScopes returns the elements of a that are also in b, in a's order.
func IntersectOAuthScopes(a, b []string) []string {
	have := make(map[string]bool, len(b))
	for _, s := range b {
		have[s] = true
	}
	out := make([]string, 0, len(a))
	for _, s := range a {
		if have[s] {
			out = append(out, s)
		}
	}
	return out
}
