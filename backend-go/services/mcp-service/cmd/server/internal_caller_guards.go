package main

import (
	"log/slog"

	"github.com/stablyai/orca-go/common/internalcaller"
	"google.golang.org/grpc"
)

// internalCallerServerOptions guards the gateway-only and registry-internal RPCs (including the stdio-capable
// CallExternalTool) and the StreamEvents stream. An empty token never switches the guard off: internalcaller.Guard
// refuses every call to the listed methods, so a forgotten variable is a refused RPC instead of an open one.
func internalCallerServerOptions(token string, log *slog.Logger) []grpc.ServerOption {
	if token == "" {
		log.Error("MCP_INTERNAL_CALLER_TOKEN is empty: gateway-only governance RPCs, the internal registry RPCs and StreamEvents refuse every call")
	}
	methods := append(append([]string{}, gatewayOnlyMethods...), registryInternalMethods...)
	return []grpc.ServerOption{
		// Unary interceptors never see streams, so StreamEvents needs its own.
		grpc.ChainUnaryInterceptor(internalcaller.Guard(token, methods...)),
		grpc.ChainStreamInterceptor(internalcaller.StreamGuard(token, gatewayOnlyStreamMethods...)),
	}
}
