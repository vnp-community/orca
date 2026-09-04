// Package domain holds auth-service's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework.
package domain

import (
	"errors"
	"strings"
	"time"
)

// Role is a coarse global role — fine-grained authorization is OPA's job,
// not an enum grown over time here. See auth-service.md §4.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

func (r Role) Valid() bool {
	switch r {
	case RoleUser, RoleAdmin:
		return true
	default:
		return false
	}
}

var (
	// ErrEmptyID is returned when a User is constructed without an ID.
	ErrEmptyID = errors.New("domain: id is required")
	// ErrEmptyTenant guards the tenant-isolation invariant — a User with no
	// owning tenant is never a valid domain state.
	ErrEmptyTenant = errors.New("domain: tenant_id is required")
	// ErrEmptyEmail is returned when Email is empty.
	ErrEmptyEmail = errors.New("domain: email is required")
	// ErrInvalidEmail is returned when Email doesn't contain "@" — a
	// minimal syntactic check only; real deliverability validation is out
	// of scope for the domain layer.
	ErrInvalidEmail = errors.New("domain: email must contain '@'")
	// ErrInvalidRole is returned when Role isn't one of the known enum
	// values.
	ErrInvalidRole = errors.New("domain: invalid role")
)

// User is auth-service's identity record. Deliberately carries no password
// material — the constructor takes only what the wire/API surface exposes;
// the bcrypt hash lives alongside the row in adapter/postgres and is never
// part of this struct, so a leaked domain.User value can never leak a
// credential. See auth-service.md §4/§6.
type User struct {
	ID        string
	TenantID  string
	Email     string
	Name      string
	Role      Role
	IsActive  bool
	CreatedAt time.Time
	// SsoProvider is the provider this user most recently authenticated
	// through — empty for an account that has only ever logged in with a
	// local password. Deliberately NOT a NewUser constructor parameter
	// (unlike Role/IsActive): it's a mutable "last used" fact updated by
	// UserRepository.SetSsoProvider on every successful SSO login (see
	// internal/usecase/login_or_provision_sso_user.go), not an identity
	// invariant fixed at creation time.
	SsoProvider SsoProvider
}

// NewUser constructs a User, enforcing the invariants a record must satisfy
// to be meaningful — rejects an empty ID/tenant, an email with no "@", and
// any role outside the closed enum.
func NewUser(id, tenantID, email, name string, role Role, isActive bool, createdAt time.Time) (User, error) {
	if id == "" {
		return User{}, ErrEmptyID
	}
	if tenantID == "" {
		return User{}, ErrEmptyTenant
	}
	if email == "" {
		return User{}, ErrEmptyEmail
	}
	if !strings.Contains(email, "@") {
		return User{}, ErrInvalidEmail
	}
	if !role.Valid() {
		return User{}, ErrInvalidRole
	}
	return User{
		ID:        id,
		TenantID:  tenantID,
		Email:     email,
		Name:      name,
		Role:      role,
		IsActive:  isActive,
		CreatedAt: createdAt,
	}, nil
}
