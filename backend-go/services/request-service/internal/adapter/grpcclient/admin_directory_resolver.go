package grpcclient

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// AdminDirectoryResolver lists active admins through auth-service's internal ListActiveTenantAdmins. ListUsers is
// not usable here: it demands an admin actor, and the outbox relay that enriches notifications has a tenant but no
// user. The tenant sent is the one already validated in ctx, never a caller-supplied value. The connection must
// carry the internal-caller token (internalcaller.ClientInterceptor).
type AdminDirectoryResolver struct {
	client authv1.AuthServiceClient
	cache  *directoryCache
	log    *slog.Logger
}

var _ usecase.AdminDirectoryResolver = (*AdminDirectoryResolver)(nil)

func NewAdminDirectoryResolver(client authv1.AuthServiceClient, ttl time.Duration) *AdminDirectoryResolver {
	return &AdminDirectoryResolver{client: client, cache: newDirectoryCache(ttl), log: slog.Default()}
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
	resp, err := r.client.ListActiveTenantAdmins(callCtx, &authv1.ListActiveTenantAdminsRequest{})
	if err != nil {
		return nil, directoryUnavailable("list active tenant admins", err)
	}
	if resp.GetTruncated() {
		r.log.Warn("tenant admin list truncated by auth-service; some admins will not be notified", slog.String("tenant_id", tenantID))
	}
	admins := resp.GetUserIds()
	r.cache.put(key, admins)
	return admins, nil
}
