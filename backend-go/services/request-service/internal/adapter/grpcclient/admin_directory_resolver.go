package grpcclient

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// maxAdminPages bounds ListUsers paging for one lookup; a tenant with more admins than this gets a truncated list.
const maxAdminPages = 20

// AdminDirectoryResolver lists active admins from auth-service ListUsers. The tenant_id sent is the one already
// validated in ctx, never a caller-supplied value.
type AdminDirectoryResolver struct {
	client authv1.AuthServiceClient
	cache  *directoryCache
}

var _ usecase.AdminDirectoryResolver = (*AdminDirectoryResolver)(nil)

func NewAdminDirectoryResolver(client authv1.AuthServiceClient, ttl time.Duration) *AdminDirectoryResolver {
	return &AdminDirectoryResolver{client: client, cache: newDirectoryCache(ttl)}
}

func (r *AdminDirectoryResolver) ListAdmins(ctx context.Context, tenantID string) ([]string, error) {
	ctxTenant, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if tenantID != ctxTenant {
		return nil, usecase.ErrForbidden
	}
	key := "admins|" + tenantID
	if v, ok := r.cache.get(key); ok {
		return v, nil
	}
	callCtx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	var admins []string
	token := ""
	for page := 0; page < maxAdminPages; page++ {
		resp, err := r.client.ListUsers(callCtx, &authv1.ListUsersRequest{TenantId: tenantID, PageToken: token, PageSize: 200})
		if err != nil {
			return nil, directoryUnavailable("list users", err)
		}
		for _, u := range resp.GetUsers() {
			if u.GetRole() == authv1.Role_ROLE_ADMIN && u.GetIsActive() {
				admins = append(admins, u.GetId())
			}
		}
		if token = resp.GetNextPageToken(); token == "" {
			break
		}
	}
	r.cache.put(key, admins)
	return admins, nil
}
