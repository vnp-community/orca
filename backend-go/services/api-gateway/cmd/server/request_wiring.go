// Configured by REQUEST_INTERNAL_CALLER_TOKEN
package main

import (
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stablyai/orca-go/common/internalcaller"
)

// dialRequestService dials request-service and presents REQUEST_INTERNAL_CALLER_TOKEN.
func dialRequestService(addr string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler())}
	tok := os.Getenv("REQUEST_INTERNAL_CALLER_TOKEN")
	if tok != "" {
		opts = append(opts, grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(tok)),
			grpc.WithChainStreamInterceptor(internalcaller.StreamClientInterceptor(tok)))
	} else {
		slog.Warn("REQUEST_INTERNAL_CALLER_TOKEN is empty: request-service will reject every call")
	}
	return grpc.NewClient(addr, opts...)
}
