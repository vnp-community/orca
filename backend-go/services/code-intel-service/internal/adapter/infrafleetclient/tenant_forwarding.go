package infrafleetclient

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
)

// withTenantMetadata stamps the caller's tenant ID onto ctx as outbound gRPC metadata.
func withTenantMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID), nil
}
