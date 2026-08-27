package grpcclient

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
// git-gateway-service is itself an internal (not api-gateway) service, so
// unlike api-gateway's AttachIdentity (internal/adapter/grpc/dial.go, which
// resolves identity fresh from a validated credential) this helper only
// forwards what the inbound TenantExtractionInterceptor already put on this
// request's context — it never invents or re-validates a tenant. Every
// outbound call this package makes to infra-fleet-service must go through
// this helper: infra-fleet-service's usecases call tenant.RequireTenantID
// and fail closed with INFRA_NO_TENANT otherwise.
func withTenantMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID), nil
}
