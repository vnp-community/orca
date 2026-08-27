package infrafleetclient

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
)

// withTenantMetadata stamps the caller's already-validated tenant ID onto
// ctx as outbound gRPC metadata, using the same key every service's inbound
// grpcmw.TenantExtractionInterceptor reads (common/grpcmw.MetadataTenantID).
//
// workflow-service is itself an internal (not api-gateway) service, so this
// helper only forwards what the inbound TenantExtractionInterceptor already
// put on the request's context — it never invents or re-validates a tenant.
// Every outbound call this package makes to infra-fleet-service must go
// through this helper: infra-fleet-service's usecases call
// tenant.RequireTenantID and fail closed with INFRA_NO_TENANT otherwise. No
// backend-to-backend client in workflow-service forwarded tenant identity
// before this pass — see this package's build instructions.
func withTenantMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID), nil
}
