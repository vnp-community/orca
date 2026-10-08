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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ActorTypeInterceptor reads x-orca-actor-type; tenant.WithActorType normalizes missing or unknown values to user.
func ActorTypeInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(withActorType(ctx), req)
	}
}

func withActorType(ctx context.Context) context.Context {
	v := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(grpcmw.MetadataActorType); len(vals) > 0 {
			v = vals[0]
		}
	}
	return tenant.WithActorType(ctx, v)
}

// RequestAccessInterceptor refuses an RPC the caller may not run against the Request it targets.
func RequestAccessInterceptor(a *usecase.AuthorizeRequestAction) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !domain.IsGuardedMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		ctx, err := a.Authorize(ctx, info.FullMethod, targetID(info.FullMethod, req))
		if err != nil {
			return nil, apperrors.ToGRPCStatus(err)
		}
		return handler(ctx, req)
	}
}

// targetID reads the catalog Field of the request message; "" when the RPC has no locator or the field is empty.
func targetID(fullMethod string, req any) string {
	e, ok := domain.Catalog[fullMethod]
	if !ok || e.Field == "" {
		return ""
	}
	m, ok := req.(proto.Message)
	if !ok {
		return ""
	}
	fd := m.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(e.Field))
	if fd == nil || fd.Kind() != protoreflect.StringKind || fd.IsList() {
		return ""
	}
	return m.ProtoReflect().Get(fd).String()
}
