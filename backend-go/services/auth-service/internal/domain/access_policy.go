package domain

import (
	"errors"
	"time"
)

var (
	// ErrEmptyPolicyName is returned when an AccessPolicy is constructed
	// without a name.
	ErrEmptyPolicyName = errors.New("domain: access policy name is required")
	// ErrEmptyPolicyKind is returned when an AccessPolicy is constructed
	// without a kind (e.g. "role-definition", "rate-tier").
	ErrEmptyPolicyKind = errors.New("domain: access policy kind is required")
	// ErrEmptyPolicyDocument is returned when an AccessPolicy is
	// constructed with no document — an access policy that decides
	// nothing is not a valid domain state.
	ErrEmptyPolicyDocument = errors.New("domain: access policy document_json is required")
	// ErrInvalidPolicyVersion is returned when Version is less than 1 —
	// versions are 1-indexed, append-only (auth-service.md:150).
	ErrInvalidPolicyVersion = errors.New("domain: access policy version must be >= 1")
)

// AccessPolicy is one version of an admin-console access-policy document —
// an OPA-bundle-published RBAC/rate-tier definition. Per auth-service.md:150,
// UpdateAccessPolicy never mutates a row in place: every update inserts a
// NEW row with Version = previous + 1, so a policy's full history is always
// reconstructable from auth.access_policies, and OPA bundle sync / audit
// both have a stable version to point at.
type AccessPolicy struct {
	ID           string
	Name         string
	Kind         string // "role-definition" | "rate-tier" | ...
	DocumentJSON string
	Version      int32
	UpdatedBy    string
	UpdatedAt    time.Time
}

// NewAccessPolicy constructs an AccessPolicy, enforcing that every policy
// has an id, name, kind, non-empty document, and a version >= 1.
func NewAccessPolicy(id, name, kind, documentJSON string, version int32, updatedBy string, updatedAt time.Time) (AccessPolicy, error) {
	if id == "" {
		return AccessPolicy{}, ErrEmptyID
	}
	if name == "" {
		return AccessPolicy{}, ErrEmptyPolicyName
	}
	if kind == "" {
		return AccessPolicy{}, ErrEmptyPolicyKind
	}
	if documentJSON == "" {
		return AccessPolicy{}, ErrEmptyPolicyDocument
	}
	if version < 1 {
		return AccessPolicy{}, ErrInvalidPolicyVersion
	}
	return AccessPolicy{
		ID:           id,
		Name:         name,
		Kind:         kind,
		DocumentJSON: documentJSON,
		Version:      version,
		UpdatedBy:    updatedBy,
		UpdatedAt:    updatedAt,
	}, nil
}
