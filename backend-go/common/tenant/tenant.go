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
	tenantIDKey  = &contextKey{"tenant_id"}
	userIDKey    = &contextKey{"user_id"}
	roleKey      = &contextKey{"role"} // NEW
	projectIDKey = &contextKey{"project_id"}
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

// WithProjectID attaches the current request/execution's project scope to
// ctx — added for workflow-service's step-dispatch context (TASK-WF-02-05/
// 06): a wave-dispatched step's ctx is rebuilt from tenantID alone (see
// usecase.Execute's dispatchCtx), so provider/server resolution that needs
// "which project is this step running for" reads it from here rather than
// re-deriving it, once the dispatch path is enriched with it.
func WithProjectID(ctx context.Context, projectID string) context.Context {
	return context.WithValue(ctx, projectIDKey, projectID)
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

// WithRole attaches the caller's role claim to ctx — populated by the
// inbound gRPC interceptor once JWT-role-claim propagation from api-gateway
// lands (tracked at project-service/internal/usecase/authorization.go's
// callerGlobalRole stub). Not called anywhere in this codebase yet — see
// Role's doc comment.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

// Role returns the caller's role and whether one was present. Returns
// ("", false) until the upstream role-claim-propagation gap closes (see
// WithRole's doc comment) — every caller of this function must treat that
// as "unknown role" and fail closed (deny), never as an implicit grant.
func Role(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(roleKey).(string)
	return v, ok && v != ""
}

// ProjectID returns the current project scope and whether one was present —
// see WithProjectID's doc comment.
func ProjectID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(projectIDKey).(string)
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
