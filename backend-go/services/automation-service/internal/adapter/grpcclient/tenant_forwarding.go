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
// — mirrors workflow-service's own internal/adapter/infrafleetclient's
// identically-named helper for its own outbound hop.
//
// Found by TASK-220's E2E test (run_now_e2e_test.go): WorkflowClient set
// TenantId on the wire request message but never forwarded it via outgoing
// gRPC metadata, so workflow-service's own tenant.RequireTenantID(ctx) (fed
// only by its inbound TenantExtractionInterceptor reading metadata, not
// request fields) always failed closed with WORKFLOW_NO_TENANT — every real
// RunNow dispatch to a live workflow-service was broken, not just the
// already-known "no live workflow-service" gap SOL-033 flagged.
func withTenantMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID), nil
}
