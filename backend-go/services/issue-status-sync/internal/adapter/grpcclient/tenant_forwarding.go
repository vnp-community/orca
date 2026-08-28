package grpcclient

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
)

// withTenantMetadata stamps tenantID onto ctx as outbound gRPC metadata,
// using the same key every service's inbound grpcmw.TenantExtractionInterceptor
// reads (common/grpcmw.MetadataTenantID) — same wire contract every other
// grpcclient package in this codebase uses, but sourced from an event's own
// tenant_id rather than an inbound request's validated caller identity:
// issue-status-sync is an async event consumer, not an inbound gRPC
// handler, so there is no request context to forward from (see
// usecase/ports.go's IssueTrackerClient doc comment).
func withTenantMetadata(ctx context.Context, tenantID string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID)
}
