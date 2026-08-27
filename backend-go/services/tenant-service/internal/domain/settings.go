// Package domain holds tenant-service's entities and the profile-resolution
// merge algorithm. Per specs/backend-go/architecture/03-clean-architecture-guidelines.md,
// this package has zero imports outside stdlib + other domain/ packages —
// no database, no gRPC, no framework.
package domain

// Settings is the layered JSON settings blob shared by Company, Department,
// Team, and UserProfile — the company/department/team/user layers merged by
// ResolveProfile (see profile_resolution.go and tenant-service.md §4).
//
// Modeled as a generic decoded-JSON map (map[string]any, with nested
// map[string]any/[]any/scalars), not a fixed struct: it ports 1:1 from the
// TS system's free-form Settings JSON blob (ProfileResolver.ts) without this
// service needing to know every field name in advance. Adapters own the
// JSON marshal/unmarshal boundary (internal/adapter/postgres,
// internal/adapter/grpc) — domain code only reads/merges already-decoded
// values, per architecture/03's "pure domain, no I/O" rule.
type Settings map[string]any

// emptySettings returns s, or a non-nil empty Settings if s is nil, so
// merge code never needs a repeated nil check once a layer has passed
// through this — nil is a normal, expected value for an unset layer (e.g.
// no department assigned, no user overrides yet).
func emptySettings(s Settings) Settings {
	if s == nil {
		return Settings{}
	}
	return s
}

// asMap extracts a nested settings object from a decoded-JSON value.
// Accepts both the plain map[string]any encoding/json produces and the
// named Settings type callers (tests, in particular) often use directly for
// nested literals — a bare type assertion to map[string]any would fail for
// the latter since Settings is a distinct named type in Go's type system.
func asMap(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		return t, true
	case Settings:
		return map[string]any(t), true
	default:
		return nil, false
	}
}
