package aiproviderclient

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
)

// withTenantMetadata stamps the caller's already-validated tenant ID onto
// ctx as outbound gRPC metadata — same convention as
// infrafleetclient.withTenantMetadata/projectclient.withTenantMetadata
// (this service's other outbound client packages). ai-provider-service's
// ResolveProvider handler calls tenant.RequireTenantID(ctx) — its
// request's tenant_id field is documented as ignored server-side — so this
// forwarding is required, not optional.
func withTenantMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID), nil
}
