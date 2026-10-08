package grpc

import (
	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
)

// SecurityChainConfig holds what the interceptors need. Empty tokens do not disable anything: the guards
// then refuse every call to their methods (fail closed).
type SecurityChainConfig struct {
	GatewayToken string
	ServiceToken string
	Authorize    *usecase.AuthorizeRequestAction
	Limiter      *usecase.RateLimiter
	Gate         *usecase.ConcurrencyGate
	// AfterGuard run once the caller is authenticated and identified, before authorization (the feature
	// flag gate of rf/roll goes here). AfterAuthz run for authorized calls only, before rate limiting
	// (RPC audit goes here). Both keep their order.
	AfterGuard []grpc.UnaryServerInterceptor
	AfterAuthz []grpc.UnaryServerInterceptor
}

// SecurityChain returns the server options placed after grpcmw.ChainUnary (recovery, tenant extraction,
// logging). Order: callers are authenticated before their identity metadata is read, then authorized,
// then rate limited, so a refused call costs no tokens.
func SecurityChain(c SecurityChainConfig) []grpc.ServerOption {
	public, internal := domain.PublicMethods(), domain.InternalMethods()
	unary := []grpc.UnaryServerInterceptor{
		internalcaller.Guard(c.GatewayToken, public...),
		internalcaller.Guard(c.ServiceToken, internal...),
		ActorTypeInterceptor(),
	}
	unary = append(unary, c.AfterGuard...)
	unary = append(unary, RequestAccessInterceptor(c.Authorize))
	unary = append(unary, c.AfterAuthz...)
	unary = append(unary, RateLimitInterceptor(c.Limiter, c.Gate))
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(
			internalcaller.StreamGuard(c.GatewayToken, public...),
			internalcaller.StreamGuard(c.ServiceToken, internal...),
			StreamIdentityInterceptor(),
			StreamAccessInterceptor(c.Authorize),
		),
	}
}
