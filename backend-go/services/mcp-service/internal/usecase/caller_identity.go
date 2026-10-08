package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// callerIdentity is the authenticated caller as attached by api-gateway in
// gRPC metadata. Identity is never read from request bodies.
type callerIdentity struct {
	TenantID string
	UserID   string
	Role     string
}

// userCaller requires tenant and user (every consent/grant call acts for a user).
func userCaller(ctx context.Context) (callerIdentity, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return callerIdentity{}, domain.ErrNoTenant(err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return callerIdentity{}, domain.ErrNoTenant(nil)
	}
	role, _ := tenant.Role(ctx)
	return callerIdentity{TenantID: tenantID, UserID: userID, Role: role}, nil
}

// tenantCaller requires only the tenant: internal service callers
// (request-service) may act without an end user. UserID is set when forwarded.
func tenantCaller(ctx context.Context) (callerIdentity, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return callerIdentity{}, domain.ErrNoTenant(err)
	}
	userID, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)
	return callerIdentity{TenantID: tenantID, UserID: userID, Role: role}, nil
}

// requireAdmin mirrors policy/orca-authz/mcp_oauth.rego: only role "admin"
// passes. An absent or unknown role fails closed.
func (c callerIdentity) requireAdmin() error {
	if c.Role != domain.RoleAdmin {
		return domain.ErrNotAdmin()
	}
	return nil
}
