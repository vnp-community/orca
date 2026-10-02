// Package mcpscope holds the MCP scope identifiers and the one rule every
// service shares: a role's scope ceiling. Descriptors (labels, risk) belong to
// mcp-service; this package only knows ids, ceilings and set operations.
package mcpscope

import (
	"errors"
	"strings"
)

const (
	Read  = "orca:read"
	Write = "orca:write"
	Exec  = "orca:exec"
	Admin = "orca:admin"
)

// ErrUnknownScope is returned for a scope outside the catalog (maps to MCP_SCOPE_INVALID).
var ErrUnknownScope = errors.New("mcpscope: unknown scope")

// All returns the catalog in a stable order.
func All() []string { return []string{Read, Write, Exec, Admin} }

func known(s string) bool {
	for _, k := range All() {
		if k == s {
			return true
		}
	}
	return false
}

// Parse splits a space-delimited scope string, de-duplicates and orders it by
// catalog order. Unknown scopes are an error, never silently dropped.
func Parse(space string) ([]string, error) { return Normalize(strings.Fields(space)) }

// Normalize de-duplicates and catalog-orders scopes; unknown ones are an error.
func Normalize(scopes []string) ([]string, error) {
	seen := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		if !known(s) {
			return nil, ErrUnknownScope
		}
		seen[s] = true
	}
	out := make([]string, 0, len(seen))
	for _, s := range All() {
		if seen[s] {
			out = append(out, s)
		}
	}
	return out, nil
}

// Format joins scopes with single spaces.
func Format(scopes []string) string { return strings.Join(scopes, " ") }

// Intersect returns the elements of a that are also in b, in a's order.
func Intersect(a, b []string) []string {
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

// Subset reports whether every element of sub is in super.
func Subset(sub, super []string) bool { return len(Intersect(sub, super)) == len(sub) }

// CeilingForRole is the most a role may ever hold. Unknown or empty roles get
// nothing (fail closed). admin is the only role that may hold orca:admin.
func CeilingForRole(role string) []string {
	switch role {
	case "admin":
		return All()
	case "user":
		return []string{Read, Write, Exec}
	default:
		return nil
	}
}

// Implies reports whether granted authorizes required. A fine-grained scope
// "orca:git:write" is implied by "orca:write"; the four coarse scopes never
// imply each other (exec does not include write, admin includes nothing else).
func Implies(granted []string, required string) bool {
	for _, g := range granted {
		if g == required {
			return true
		}
	}
	parts := strings.Split(required, ":")
	if len(parts) == 3 && parts[0] == "orca" {
		coarse := "orca:" + parts[2]
		for _, g := range granted {
			if g == coarse {
				return true
			}
		}
	}
	return false
}
