package grpcmw

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/tenant"
)

// ClientIdentityInterceptor forwards the identity already on ctx (tenant, user, role, client IP, actor type) as
// outbound gRPC metadata, the same keys TenantExtractionInterceptor reads on the receiving side. Without it every
// adapter has to remember to attach them and the first one that forgets fails with NO_TENANT / NO_USER deep in a flow.
//
// Rules, each with a reason:
//   - It only fills keys that are not already set, so an adapter that deliberately acts as someone else (asUser)
//     keeps control of the identity it sends.
//   - It never invents a tenant: with none on ctx nothing is added and the callee rejects the call (fail-closed).
//   - A non-user actor type is always forwarded; dropping it would turn an agent's call into a person's at the callee.
func ClientIdentityInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(withForwardedIdentity(ctx), method, req, reply, cc, opts...)
	}
}

// ClientIdentityStreamInterceptor is ClientIdentityInterceptor for streaming calls.
func ClientIdentityStreamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(withForwardedIdentity(ctx), desc, cc, method, opts...)
	}
}

// ClientIdentityDialOptions returns the dial options every service-to-service connection should carry.
// Transport credentials and tracing stay with the caller so this package does not decide them.
func ClientIdentityDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(ClientIdentityInterceptor()),
		grpc.WithChainStreamInterceptor(ClientIdentityStreamInterceptor()),
	}
}

func withForwardedIdentity(ctx context.Context) context.Context {
	existing, _ := metadata.FromOutgoingContext(ctx)
	var pairs []string
	add := func(key, value string) {
		if value == "" || len(existing.Get(key)) > 0 {
			return
		}
		pairs = append(pairs, key, value)
	}
	if v, ok := tenant.TenantID(ctx); ok {
		add(MetadataTenantID, v)
	}
	if v, ok := tenant.UserID(ctx); ok {
		add(MetadataUserID, v)
	}
	if v, ok := tenant.Role(ctx); ok {
		add(MetadataRole, v)
	}
	if v, ok := tenant.ClientIP(ctx); ok {
		add(MetadataClientIP, v)
	}
	if actor := tenant.ActorType(ctx); actor != tenant.ActorUser {
		add(MetadataActorType, actor)
	}
	if len(pairs) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

// NewClient is grpc.NewClient with ClientIdentityDialOptions appended: the one way a service dials another service.
// A direct grpc.NewClient is rejected by ci/check-grpc-dials.sh, because a connection without the interceptor
// forgets the caller's identity and fails far from where the dial was written.
func NewClient(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return grpc.NewClient(target, append(opts, ClientIdentityDialOptions()...)...)
}
