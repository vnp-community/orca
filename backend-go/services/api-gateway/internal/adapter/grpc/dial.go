// Package grpc holds api-gateway's OUTBOUND gRPC clients — one dialer per
// downstream service this scaffold really calls (usage-service,
// notification-service), plus the helper that attaches resolved
// tenant/user identity onto outbound call metadata. This is the mirror
// image of every other service's internal/adapter/grpc/, which implements
// an INBOUND server; api-gateway has no gRPC server of its own (see
// cmd/server/main.go's doc comment for why).
package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// Dial opens a gRPC client connection to addr.
//
// Uses insecure transport credentials — acceptable for local dev only.
// Production must dial with mTLS client credentials matching every other
// service's inbound mTLS expectation (api-gateway.md §9's "every hop past
// [TLS termination] is internal-cluster mTLS gRPC"); wiring the mesh's
// certificate material into this dialer is tracked as a known gap, not
// silently skipped — see README.md.
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// AttachIdentity stamps the resolved tenant/user identity onto ctx as
// outbound gRPC metadata, using the same metadata keys every service's
// inbound TenantExtractionInterceptor (common/grpcmw) reads — this is the
// "identity propagation" mechanism api-gateway.md §2/§9 describes: the
// gateway resolves identity once from the validated credential and never
// lets a downstream service trust a tenant ID from the request body.
func AttachIdentity(ctx context.Context, id usecase.Identity) context.Context {
	return metadata.AppendToOutgoingContext(ctx,
		grpcmw.MetadataTenantID, id.TenantID,
		grpcmw.MetadataUserID, id.UserID,
	)
}
