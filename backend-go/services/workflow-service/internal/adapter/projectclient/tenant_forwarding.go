package projectclient

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
)

// withTenantMetadata stamps the caller's already-validated tenant ID onto
// ctx as outbound gRPC metadata — same convention as
// infrafleetclient.withTenantMetadata (this service's other outbound
// client package): forwards what the inbound TenantExtractionInterceptor
// already put on the request's context, never invents or re-validates a
// tenant. project-service's usecases require a tenant in context the same
// way infra-fleet-service's do.
func withTenantMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID), nil
}
