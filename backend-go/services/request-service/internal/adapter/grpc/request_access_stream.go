package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// contextStream swaps the context of a server stream so identity reaches the handler.
type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s contextStream) Context() context.Context { return s.ctx }

// StreamIdentityInterceptor does for streams what grpcmw.TenantExtractionInterceptor and
// ActorTypeInterceptor do for unary calls (the shared interceptor does not cover streams).
func StreamIdentityInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if v := md.Get(grpcmw.MetadataTenantID); len(v) > 0 && v[0] != "" {
				ctx = tenant.WithTenantID(ctx, v[0])
			}
			if v := md.Get(grpcmw.MetadataUserID); len(v) > 0 && v[0] != "" {
				ctx = tenant.WithUserID(ctx, v[0])
			}
			if v := md.Get(grpcmw.MetadataRole); len(v) > 0 && v[0] != "" {
				ctx = tenant.WithRole(ctx, v[0])
			}
			if v := md.Get(grpcmw.MetadataClientIP); len(v) > 0 && v[0] != "" {
				ctx = tenant.WithClientIP(ctx, v[0])
			}
		}
		return handler(srv, contextStream{ServerStream: ss, ctx: withActorType(ctx)})
	}
}

// StreamAccessInterceptor authorizes stream RPCs; they are admin-only today, so there is no target id.
func StreamAccessInterceptor(a *usecase.AuthorizeRequestAction) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !domain.IsGuardedMethod(info.FullMethod) {
			return handler(srv, ss)
		}
		ctx, err := a.Authorize(ss.Context(), info.FullMethod, "")
		if err != nil {
			return apperrors.ToGRPCStatus(err)
		}
		return handler(srv, contextStream{ServerStream: ss, ctx: ctx})
	}
}
