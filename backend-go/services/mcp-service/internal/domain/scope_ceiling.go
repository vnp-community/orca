package domain

import (
	"errors"
	"strings"
)

// Scope ids and the per-role ceiling (same rule as BE-MCP-SOL-006's
// common/mcpscope; kept local until that package exists).
const (
	ScopeRead  = "orca:read"
	ScopeWrite = "orca:write"
	ScopeExec  = "orca:exec"
	ScopeAdmin = "orca:admin"

	RoleAdmin = "admin"
	RoleUser  = "user"
)

var ErrUnknownScope = errors.New("domain: unknown scope")

// NormalizeScopes de-duplicates and orders scopes by catalog order; an
// unknown scope is an error, never silently dropped.
func NormalizeScopes(scopes []string) ([]string, error) {
	seen := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		if !knownScope(s) {
			return nil, ErrUnknownScope
		}
		seen[s] = true
	}
	out := make([]string, 0, len(seen))
	for _, d := range ScopeCatalog() {
		if seen[d.ID] {
			out = append(out, d.ID)
		}
	}
	return out, nil
}

func knownScope(s string) bool {
	for _, d := range ScopeCatalog() {
		if d.ID == s {
			return true
		}
	}
	return false
}

// ScopeCeilingForRole is the most a role may ever consent to. Unknown or
// empty role gets nothing (fail closed).
func ScopeCeilingForRole(role string) []string {
	switch role {
	case RoleAdmin:
		return []string{ScopeRead, ScopeWrite, ScopeExec, ScopeAdmin}
	case RoleUser:
		return []string{ScopeRead, ScopeWrite, ScopeExec}
	default:
		return nil
	}
}

// ScopesSubset reports whether every element of sub is in super.
func ScopesSubset(sub, super []string) bool {
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

// IntersectScopes returns elements of a that are also in b, in a's order.
func IntersectScopes(a, b []string) []string {
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

// SplitScopeString splits an RFC 6749 space-delimited scope parameter.
func SplitScopeString(space string) []string { return strings.Fields(space) }
