// Package tenant provides the context primitives every service uses to
// carry the validated tenant/user identity through a request. Per
// specs/backend-go/architecture/05-data-architecture.md: tenant scoping is
// "never optional, never inferred from request body" — the ONLY way a
// repository method should learn the current tenant is by pulling it from
// context via this package, after api-gateway (or a gRPC interceptor, see
// common/grpcmw) has already validated it from the caller's JWT/session.
package tenant

import (
	"context"
	"errors"
)

type contextKey struct{ name string }

var (
	tenantIDKey = &contextKey{"tenant_id"}
	userIDKey   = &contextKey{"user_id"}
	roleKey     = &contextKey{"role"}
)

// ErrNoTenant is returned by RequireTenantID when the context carries no
// tenant — a bug (a request reached a repository without going through the
// gRPC auth interceptor), not a normal runtime condition to handle gracefully.
var ErrNoTenant = errors.New("tenant: no tenant_id in context")

// WithTenantID attaches the validated tenant ID to ctx. Called once, by the
// inbound gRPC interceptor (common/grpcmw), never by usecase/domain code.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// WithUserID attaches the validated acting user ID to ctx.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithRole attaches the caller's global role ("admin"/"user", matching
// auth-service's domain.Role values) to ctx — added for CR-DS-006 Phase 2
// (docs/crs/v2/dev-server/CR-DS-006-dev-server-approval-and-grouping.md),
// closing the previously-documented gap every service's callerGlobalRole
// stub described ("no role claim propagates from api-gateway into a
// service's request context yet" — see project-service/annotation-service's
// own authorization code). Empty string means "unknown" — callers must
// treat that as non-admin (fail closed), never as an implicit allow.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

// TenantID returns the tenant ID and whether one was present.
func TenantID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(tenantIDKey).(string)
	return v, ok && v != ""
}

// UserID returns the acting user ID and whether one was present.
func UserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok && v != ""
}

// Role returns the caller's global role and whether one was present. A
// bearer-JWT-authenticated caller (usecase.AuthValidator's path) never
// populates this today — only the cookie/session path
// (authclient.SessionValidator) does, per that gap's own doc comment. Every
// caller of Role must treat ok==false the same as an empty/non-admin role,
// never as "trust it."
func Role(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(roleKey).(string)
	return v, ok && v != ""
}

// RequireTenantID panics-free variant repository methods call at the top of
// every tenant-scoped query — returns ErrNoTenant rather than silently
// running an unscoped query. See production-readiness-checklist.md's
// "every generated query touching a tenant-scoped table takes tenant_id as
// a bound parameter" requirement.
func RequireTenantID(ctx context.Context) (string, error) {
	id, ok := TenantID(ctx)
	if !ok {
		return "", ErrNoTenant
	}
	return id, nil
}
