package grpcclient

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
)

// withTenantMetadata stamps the caller's already-validated tenant ID onto
// ctx as outbound gRPC metadata, using the same key every service's inbound
// grpcmw.TenantExtractionInterceptor reads (common/grpcmw.MetadataTenantID)
// — mirrors git-gateway-service/internal/adapter/grpcclient's helper of the
// same name exactly, since task-service is in the identical position: an
// internal (not api-gateway) service forwarding what its own inbound
// interceptor already put on this request's context, never inventing or
// re-validating a tenant itself.
func withTenantMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID), nil
}
