package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

const (
	activeAdminScanPageSize = 200
	// activeAdminScanMaxPages bounds one call; a tenant above pageSize*maxPages users gets a truncated answer.
	activeAdminScanMaxPages = 20
)

// ActiveTenantAdmins is the answer of ListActiveTenantAdmins; Truncated means the scan hit its page bound.
type ActiveTenantAdmins struct {
	UserIDs   []string
	Truncated bool
}

// ListActiveTenantAdmins serves internal background callers (request-service's outbox relay) that hold a tenant but no
// acting user, so it cannot reuse ListUsers (admin actor required). The caller is authenticated by the transport's
// internal-caller guard instead, and the tenant comes only from the context, never from a request field, so a caller
// cannot enumerate another tenant. It returns ids only.
type ListActiveTenantAdmins struct {
	users UserRepository
}

func NewListActiveTenantAdmins(users UserRepository) *ListActiveTenantAdmins {
	return &ListActiveTenantAdmins{users: users}
}

func (uc *ListActiveTenantAdmins) Execute(ctx context.Context) (ActiveTenantAdmins, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ActiveTenantAdmins{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_NO_TENANT", "no tenant in request context", err)
	}
	out := ActiveTenantAdmins{UserIDs: []string{}}
	token := ""
	for page := 0; page < activeAdminScanMaxPages; page++ {
		users, next, err := uc.users.ListUsers(ctx, tenantID, token, activeAdminScanPageSize)
		if err != nil {
			return ActiveTenantAdmins{}, apperrors.New(apperrors.KindInternal, "AUTH_LIST_ACTIVE_ADMINS_FAILED", "failed to list tenant admins", err)
		}
		for _, u := range users {
			// The repository scopes by tenant already; re-checking keeps a repository bug from leaking another tenant.
			if u.TenantID == tenantID && u.Role == domain.RoleAdmin && u.IsActive {
				out.UserIDs = append(out.UserIDs, u.ID)
			}
		}
		if token = next; token == "" {
			return out, nil
		}
	}
	out.Truncated = true
	return out, nil
}
