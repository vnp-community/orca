// Request-service wiring: REQUEST_SERVICE_ADDR selects the target and
// REQUEST_INTERNAL_CALLER_TOKEN is presented on every unary and stream call.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stablyai/orca-go/common/internalcaller"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// dialRequestService dials request-service and presents REQUEST_INTERNAL_CALLER_TOKEN.
func dialRequestService(addr string, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler())}
	opts = append(opts, extra...)
	tok := os.Getenv("REQUEST_INTERNAL_CALLER_TOKEN")
	if tok != "" {
		opts = append(opts, grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(tok)),
			grpc.WithChainStreamInterceptor(internalcaller.StreamClientInterceptor(tok)))
	} else {
		slog.Warn("REQUEST_INTERNAL_CALLER_TOKEN is empty: request-service will reject every call")
	}
	return grpc.NewClient(addr, opts...)
}

// buildRequestClients dials request-service when REQUEST_SERVICE_ADDR is set. An
// empty address is not dialed (grpc.NewClient("") behaviour is unverified): the
// clients stay nil and every Request channel answers REQUEST_UNAVAILABLE.
func buildRequestClients(addr string, logger *slog.Logger, extra ...grpc.DialOption) (requestv1.RequestServiceClient, requestv1.ApprovalServiceClient, *grpc.ClientConn, error) {
	if addr == "" {
		logger.Info("REQUEST_SERVICE_ADDR is empty: request channels answer REQUEST_UNAVAILABLE")
		return nil, nil, nil, nil
	}
	conn, err := dialRequestService(addr, extra...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("dialing request-service: %w", err)
	}
	return requestv1.NewRequestServiceClient(conn), requestv1.NewApprovalServiceClient(conn), conn, nil
}
