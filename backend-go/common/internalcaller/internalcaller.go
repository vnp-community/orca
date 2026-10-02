// Package internalcaller restricts selected gRPC methods to a trusted sibling
// service. There is no mesh peer-identity interceptor in this codebase yet
// (internal identity is gateway-attached metadata), so this adds a
// shared-secret metadata check as defense in depth on top of the
// NetworkPolicy allow-list; it is not a substitute for mTLS peer identity.
package internalcaller

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MetadataKey carries the shared secret on calls to a guarded method.
const MetadataKey = "x-orca-internal-token"

// Guard is a unary server interceptor that rejects calls to the listed full
// method names unless they carry the expected token. An empty expected token
// denies every guarded call (fail closed) so a missing secret can never
// silently open an internal-only RPC. Other methods pass through untouched.
func Guard(expectedToken string, fullMethods ...string) grpc.UnaryServerInterceptor {
	guarded := make(map[string]bool, len(fullMethods))
	for _, m := range fullMethods {
		guarded[m] = true
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !guarded[info.FullMethod] {
			return handler(ctx, req)
		}
		if expectedToken != "" {
			if md, ok := metadata.FromIncomingContext(ctx); ok {
				for _, got := range md.Get(MetadataKey) {
					if subtle.ConstantTimeCompare([]byte(got), []byte(expectedToken)) == 1 {
						return handler(ctx, req)
					}
				}
			}
		}
		// Generic message: never reveal whether a token was present or close.
		return nil, status.Error(codes.PermissionDenied, "INTERNAL_CALLER_REQUIRED: this method is restricted to internal services")
	}
}

// ClientInterceptor attaches the shared secret to every outgoing unary call.
// Use it only on the connection to the service whose methods are guarded.
func ClientInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, MetadataKey, token)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
