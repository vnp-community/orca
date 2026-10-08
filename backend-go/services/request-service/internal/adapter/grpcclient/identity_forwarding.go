package grpcclient

import (
	"context"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/metadata"
)

// withTenantMetadata forwards the already-validated tenant to a downstream service.
func withTenantMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID), nil
}

// withIdentityMetadata forwards tenant and acting user; services that gate by membership or
// keep per-user credentials (project-service, issue-tracking) reject calls without the user.
func withIdentityMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	userID, _ := tenant.UserID(ctx)
	if userID == "" {
		return nil, domain.ErrRequestReporterRequired()
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID), nil
}
